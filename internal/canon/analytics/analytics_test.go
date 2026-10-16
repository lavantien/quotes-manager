package analytics

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/lavantien/quotes-manager/internal/canon/export"
	"github.com/lavantien/quotes-manager/internal/canon/graph"
	"github.com/lavantien/quotes-manager/internal/canon/model"
	"github.com/lavantien/quotes-manager/internal/config"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func buildSidecar(t *testing.T) *config.Config {
	t.Helper()
	cfg := config.Defaults()
	cfg.Canon.Paths.DataDir = t.TempDir()
	cfg.Canon.Paths.DBDir = t.TempDir()

	dbPath := cfg.Canon.DBFile("anrun")
	store, err := graph.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}
	loader := graph.NewLoader(store, 100)
	node := func(typ model.NodeType, key, data string) int64 {
		t.Helper()
		id, err := loader.EnsureNode(typ, key, data)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	edge := func(src, dst int64, typ model.EdgeType, data string) {
		t.Helper()
		if err := loader.AddEdge(src, dst, typ, nil, data); err != nil {
			t.Fatal(err)
		}
	}

	divMN := node(model.NodeDivision, "mn", `{}`)
	divSN := node(model.NodeDivision, "sn", `{}`)
	edition := node(model.NodeEdition, "en/sujato/mn", `{}`)
	text := func(key, name string, parent int64) int64 {
		t.Helper()
		id := node(model.NodeText, key, `{"name":`+quoteJSON(name)+`}`)
		edge(id, parent, model.EdgeChildOf, "")
		return id
	}
	mn1 := text("mn1", "Mūlapariyāya", divMN)
	mn2 := text("mn2", "Sabbāsava", divMN)
	sn1 := text("sn1", "Saddhammasamādāna", divSN)
	edge(mn1, edition, model.EdgeCoveredBy, `{"similarity":1}`)

	dhamma := node(model.NodeHeadword, "dpd:dhamma",
		`{"lemma_1":"dhamma","root_key":"dhṛ","family_root":"dhṛ"}`)
	eva := node(model.NodeHeadword, "dpd:eva", `{"lemma_1":"eva"}`)
	bhikkhu := node(model.NodeHeadword, "dpd:bhikkhu",
		`{"lemma_1":"bhikkhu","root_key":"bhikkh","family_root":"bhikkh"}`)

	seg := func(key string, parent int64) int64 {
		t.Helper()
		id := node(model.NodeSegment, key, `{"text":"t"}`)
		edge(id, parent, model.EdgeChildOf, "")
		return id
	}
	p111 := seg("pli:mn1:1.1", mn1)
	p121 := seg("pli:mn1:2.1", mn1)
	p211 := seg("pli:mn2:1.1", mn2)
	s111 := seg("pli:sn1:1.1", sn1)
	s121 := seg("pli:sn1:2.1", sn1)
	seg("pli:sn1:3.1", sn1)
	en1 := node(model.NodeSegment, "en/sujato:mn1:1.1", `{"text":"e"}`)
	edge(en1, p111, model.EdgeTranslates, `{"author":"sujato"}`)

	word := func(src, dst int64, form string) {
		t.Helper()
		edge(src, dst, model.EdgeHasWord, `{"forms":[`+quoteJSON(form)+`]}`)
	}
	word(p111, dhamma, "dhamma")
	word(p121, dhamma, "dhamma")
	word(p111, eva, "evam")
	word(p211, bhikkhu, "bhikkhu")
	word(s111, dhamma, "dhamma")
	word(s121, bhikkhu, "bhikkhu")

	if err := loader.Flush(); err != nil {
		t.Fatal(err)
	}
	runID, err := store.NewRun("bc", "ec", "tag")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.InsertDiscrepancies(context.Background(), runID, []model.Discrepancy{
		{Check: model.CheckEmptyValue, Severity: model.Warn, Muid: "mn1", Detail: "d1"},
		{Check: model.CheckEmptyValue, Severity: model.Warn, Muid: "mn2", Detail: "d2"},
		{Check: model.CheckDpdUnknownForms, Severity: model.Info, Muid: "sn1", Detail: "d3"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.FinishRun(runID); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := graph.PublishDB(cfg, "anrun", dbPath); err != nil {
		t.Fatal(err)
	}
	if _, err := export.Rebuild(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func quoteJSON(s string) string {
	return `"` + s + `"`
}

func openAnalytics(t *testing.T, cfg *config.Config) *DB {
	t.Helper()
	d, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func TestOpenMissingFileFails(t *testing.T) {
	cfg := config.Defaults()
	cfg.Canon.Paths.DataDir = t.TempDir()
	if _, err := Open(cfg); err == nil {
		t.Fatal("opening a sidecar that does not exist must fail")
	}
}

func TestWrapSharesThePool(t *testing.T) {
	cfg := buildSidecar(t)
	pool, err := sql.Open("duckdb", cfg.Canon.DuckDBFile()+"?access_mode=read_only")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pool.Close() }()
	d, err := Wrap(pool)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Aggregate(context.Background(), "top-headwords", "", 1); err != nil {
		t.Fatalf("aggregate through wrapped pool: %v", err)
	}

	stale, err := sql.Open("duckdb", cfg.Canon.DuckDBFile()+"2")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stale.Exec(staleSidecarDDL); err != nil {
		t.Fatal(err)
	}
	if err := stale.Close(); err != nil {
		t.Fatal(err)
	}
	ro, err := sql.Open("duckdb", cfg.Canon.DuckDBFile()+"2?access_mode=read_only")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ro.Close() }()
	if _, err := Wrap(ro); !errors.Is(err, ErrSidecarStale) {
		t.Fatalf("wrap on stale sidecar = %v; want ErrSidecarStale", err)
	}
}

const staleSidecarDDL = `
CREATE TABLE node (id BIGINT, type VARCHAR, key VARCHAR, data VARCHAR);
CREATE TABLE edge (id BIGINT, src BIGINT, dst BIGINT, type VARCHAR, ord BIGINT, data VARCHAR);
CREATE TABLE run (id BIGINT, bilara_commit VARCHAR, editions_commit VARCHAR, dpd_tag VARCHAR, started VARCHAR, finished VARCHAR);
CREATE TABLE discrepancy (id BIGINT, run_id BIGINT, "check" VARCHAR, severity VARCHAR, muid VARCHAR, node_id BIGINT, detail VARCHAR);
CREATE TABLE segment_pairs (pali_id BIGINT, muid VARCHAR, seg_key VARCHAR, pali VARCHAR, en VARCHAR, author VARCHAR, html VARCHAR, reference VARCHAR);
CREATE TABLE dpd_entries (id BIGINT, lemma_1 VARCHAR, pos VARCHAR, meaning_1 VARCHAR, root VARCHAR, family_root VARCHAR);
`

func TestOpenRejectsStaleSidecar(t *testing.T) {
	cfg := config.Defaults()
	cfg.Canon.Paths.DataDir = t.TempDir()
	ddb, err := sql.Open("duckdb", cfg.Canon.DuckDBFile())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ddb.Exec(staleSidecarDDL); err != nil {
		t.Fatal(err)
	}
	if err := ddb.Close(); err != nil {
		t.Fatal(err)
	}

	_, err = Open(cfg)
	if !errors.Is(err, ErrSidecarStale) {
		t.Fatalf("open on stale sidecar = %v; want ErrSidecarStale", err)
	}
}

func TestOpenSetsReadOnlyEngine(t *testing.T) {
	cfg := buildSidecar(t)
	d := openAnalytics(t, cfg)
	if _, err := d.db.Exec(`CREATE TABLE smuggled (x INT)`); err == nil {
		t.Fatal("read-only connection accepted a write")
	}
}

func TestClampLimit(t *testing.T) {
	for _, tc := range []struct{ in, want int }{
		{0, defaultRows},
		{-3, defaultRows},
		{1, 1},
		{defaultRows, defaultRows},
		{maxRows, maxRows},
		{maxRows + 1, maxRows},
		{1_000_000, maxRows},
	} {
		if got := clampLimit(tc.in); got != tc.want {
			t.Errorf("clampLimit(%d) = %d; want %d", tc.in, got, tc.want)
		}
	}
}

func TestToCSV(t *testing.T) {
	tbl := Table{
		Cols: []string{"a", "b"},
		Rows: [][]any{
			{int64(1), nil},
			{"x,y", `he said "hi"`},
		},
	}
	want := "a,b\n1,\n\"x,y\",\"he said \"\"hi\"\"\"\n"
	if got := ToCSV(tbl); got != want {
		t.Errorf("ToCSV = %q; want %q", got, want)
	}
}
