package export

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/lavantien/quotes-manager/internal/canon/graph"
	"github.com/lavantien/quotes-manager/internal/canon/model"
	"github.com/lavantien/quotes-manager/internal/config"
)

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg := config.Defaults()
	cfg.Canon.Paths.DataDir = t.TempDir()
	cfg.Canon.Paths.DBDir = t.TempDir()
	return cfg
}

func buildGraphDB(t *testing.T, cfg *config.Config) {
	t.Helper()
	dbPath := cfg.Canon.DBFile("testrun")
	store, err := graph.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}
	loader := graph.NewLoader(store, 100)
	seg := func(key, text string) int64 {
		id, err := loader.EnsureNode(model.NodeSegment, key, `{"text":`+quoteJSON(text)+`}`)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	pali1 := seg("pli:mn1:1.1", "pali one")
	pali2 := seg("pli:mn1:2.1", "pali two")
	en1 := seg("en/sujato:mn1:1.1", "en one")
	seg("html:mn1:1.1", "<p>")
	seg("reference:mn1:1.1", "MN i 1")
	if _, err := loader.EnsureNode(model.NodeHeadword, "dpd:dhamma",
		`{"lemma_1":"dhamma","pos":"m","meaning_1":"the teaching","root_key":"dhṛ","family_root":"dhṛ"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := loader.EnsureNode(model.NodeHeadword, "dpd:eva",
		`{"lemma_1":"eva","pos":"ind","meaning_1":"thus"}`); err != nil {
		t.Fatal(err)
	}
	text, err := loader.EnsureNode(model.NodeText, "mn1", "{}")
	if err != nil {
		t.Fatal(err)
	}
	o1, o2 := int64(0), int64(1)
	if err := loader.AddEdge(pali1, text, model.EdgeChildOf, &o1, ""); err != nil {
		t.Fatal(err)
	}
	if err := loader.AddEdge(pali2, text, model.EdgeChildOf, &o2, ""); err != nil {
		t.Fatal(err)
	}
	if err := loader.AddEdge(en1, pali1, model.EdgeTranslates, nil, `{"author":"sujato"}`); err != nil {
		t.Fatal(err)
	}
	if err := loader.Flush(); err != nil {
		t.Fatal(err)
	}
	runID, err := store.NewRun("bc", "ec", "v0.4.20260926")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.InsertDiscrepancies(context.Background(), runID, []model.Discrepancy{
		{Check: model.CheckEmptyValue, Severity: model.Info, Muid: "mn1", Detail: "empty values: 1"},
		{Check: model.CheckDuplicateKeys, Severity: model.Error, Muid: "mn2", Detail: "dup keys"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.FinishRun(runID); err != nil {
		t.Fatal(err)
	}
	runID2, err := store.NewRun("bc2", "ec2", "v0.4.20260927")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.FinishRun(runID2); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := graph.PublishDB(cfg, "testrun", dbPath); err != nil {
		t.Fatal(err)
	}
}

func quoteJSON(s string) string {
	return `"` + s + `"`
}

func TestRebuildMirrorsAndDenormalizes(t *testing.T) {
	cfg := testConfig(t)
	buildGraphDB(t, cfg)

	counts, err := Rebuild(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	ddb, err := sql.Open("duckdb", cfg.Canon.DuckDBFile())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ddb.Close() }()

	var nodes, edges int64
	if err := ddb.QueryRow(`SELECT COUNT(*) FROM node`).Scan(&nodes); err != nil {
		t.Fatal(err)
	}
	if err := ddb.QueryRow(`SELECT COUNT(*) FROM edge`).Scan(&edges); err != nil {
		t.Fatal(err)
	}
	if nodes != counts["node"] || edges != counts["edge"] {
		t.Errorf("counts disagree: appender %v vs duckdb node=%d edge=%d", counts, nodes, edges)
	}
	if nodes != 8 || edges != 3 {
		t.Errorf("node=%d edge=%d; want 8 and 3", nodes, edges)
	}

	var paliID int64
	var muid, segKey, pali, en, author, html, ref string
	err = ddb.QueryRow(`SELECT pali_id, muid, seg_key, pali, en, author, html, reference
		FROM segment_pairs WHERE muid = 'mn1' AND seg_key = '1.1'`).
		Scan(&paliID, &muid, &segKey, &pali, &en, &author, &html, &ref)
	if err != nil {
		t.Fatal(err)
	}
	if pali != "pali one" || en != "en one" || author != "sujato" || html != "<p>" || ref != "MN i 1" {
		t.Errorf("segment pair = %d/%s/%s/%s/%s/%s/%s/%s", paliID, muid, segKey, pali, en, author, html, ref)
	}

	var en2, author2, html2 any
	if err := ddb.QueryRow(`SELECT en, author, html FROM segment_pairs WHERE seg_key = '2.1'`).
		Scan(&en2, &author2, &html2); err != nil {
		t.Fatal(err)
	}
	if en2 != nil || author2 != nil || html2 != nil {
		t.Errorf("untranslated pair partners = %v/%v/%v; want NULLs", en2, author2, html2)
	}

	var discrepancies int64
	if err := ddb.QueryRow(`SELECT COUNT(*) FROM discrepancy`).Scan(&discrepancies); err != nil {
		t.Fatal(err)
	}
	if discrepancies != 2 {
		t.Errorf("discrepancies = %d; want 2", discrepancies)
	}
	var detail string
	var nodeID any
	if err := ddb.QueryRow(`SELECT detail, node_id FROM discrepancy WHERE muid = 'mn1'`).Scan(&detail, &nodeID); err != nil {
		t.Fatal(err)
	}
	if detail != "empty values: 1" || nodeID != nil {
		t.Errorf("discrepancy = %q/%v", detail, nodeID)
	}
	var runs, tags int64
	if err := ddb.QueryRow(
		`SELECT COUNT(*), COUNT(DISTINCT dpd_tag) FROM run WHERE finished IS NOT NULL`).Scan(&runs, &tags); err != nil {
		t.Fatal(err)
	}
	if runs != 2 || tags != 2 {
		t.Errorf("finished runs = %d with %d tags; want 2 and 2", runs, tags)
	}

	entries, err := ddb.Query(`SELECT lemma_1, pos, meaning_1, root, family_root
		FROM dpd_entries ORDER BY lemma_1`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = entries.Close() }()

	var rows []struct {
		Lemma1, POS, Meaning1 string
		Root, Family          any
	}
	for entries.Next() {
		var r struct {
			Lemma1, POS, Meaning1 string
			Root, Family          any
		}
		if err := entries.Scan(&r.Lemma1, &r.POS, &r.Meaning1, &r.Root, &r.Family); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, r)
	}
	if err := entries.Err(); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("dpd_entries rows = %d; want 2", len(rows))
	}
	if rows[0].Lemma1 != "dhamma" || rows[0].POS != "m" || rows[0].Meaning1 != "the teaching" ||
		rows[0].Root != "dhṛ" || rows[0].Family != "dhṛ" {
		t.Errorf("dhamma entry = %+v", rows[0])
	}
	if rows[1].Lemma1 != "eva" || rows[1].Root != nil {
		t.Errorf("eva entry = %+v; want NULL root", rows[1])
	}
}

func TestRebuildRecreatesFromScratch(t *testing.T) {
	cfg := testConfig(t)
	buildGraphDB(t, cfg)
	if _, err := Rebuild(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	ddb, err := sql.Open("duckdb", cfg.Canon.DuckDBFile())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ddb.Exec(`INSERT INTO run VALUES (999, NULL, NULL, NULL, NULL, NULL)`); err != nil {
		t.Fatal(err)
	}
	if err := ddb.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Rebuild(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	ddb, err = sql.Open("duckdb", cfg.Canon.DuckDBFile())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ddb.Close() }()
	var runs int64
	if err := ddb.QueryRow(`SELECT COUNT(*) FROM run`).Scan(&runs); err != nil {
		t.Fatal(err)
	}
	if runs != 2 {
		t.Errorf("runs after rebuild = %d; want 2 (rebuild must start clean)", runs)
	}
	if _, err := os.Stat(cfg.Canon.DuckDBFile()); err != nil {
		t.Fatal(err)
	}
}
