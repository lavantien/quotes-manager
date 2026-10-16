package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/lavantien/quotes-manager/internal/canon/graph"
	"github.com/lavantien/quotes-manager/internal/canon/model"
	"github.com/lavantien/quotes-manager/internal/config"
	"github.com/lavantien/quotes-manager/internal/quote"
	"github.com/lavantien/quotes-manager/internal/store"
	"github.com/lavantien/quotes-manager/internal/store/storetest"
)

func TestQuoteFromMuidPairsGolden(t *testing.T) {
	pairs := []graph.Pair{
		{Muid: "mn1", SegKey: "0.1", Pali: "pariyāya", En: "the relational", Author: "sujato"},
		{Muid: "mn1", SegKey: "0.2", Pali: "ekodhāna"},
		{Muid: "mn1", SegKey: "0.3", Pali: "āsavā", En: "the taints", Author: "sujato"},
	}
	q, err := quoteFromMuidPairs(pairs)
	if err != nil {
		t.Fatal(err)
	}
	want := "*the relational*  \n*ekodhāna*  \n*the taints* - **MN 1**"
	if q.BodyMD() != want {
		t.Fatalf("body =\n%s\nwant\n%s", q.BodyMD(), want)
	}
	if q.SuttaID != "MN 1" || q.Citation != "MN 1" {
		t.Fatalf("ids = %q/%q, want MN 1 both", q.SuttaID, q.Citation)
	}
}

func TestQuoteFromMuidPairsEmptyAndUnmappable(t *testing.T) {
	if q, err := quoteFromMuidPairs(nil); err != nil || q != nil {
		t.Fatalf("no pairs = %v, %v; want nil quote without error", q, err)
	}
	bad := []graph.Pair{{Muid: "MN1", SegKey: "0.1", Pali: "x"}}
	if _, err := quoteFromMuidPairs(bad); err == nil {
		t.Fatal("unmappable muid must error")
	}
}

func TestQuotesFromHitsOnePerHit(t *testing.T) {
	hits := []graph.Pair{
		{Muid: "mn2", SegKey: "0.1", Pali: "sabbāsava", En: "all the taints"},
		{Muid: "snp2.14", SegKey: "1.1", Pali: "brahmacariya"},
	}
	qs, err := quotesFromHits(hits)
	if err != nil {
		t.Fatal(err)
	}
	if len(qs) != 2 {
		t.Fatalf("quotes = %d, want one per hit", len(qs))
	}
	if qs[0].BodyMD() != "*all the taints* - **MN 2**" {
		t.Fatalf("hit 0 body = %q", qs[0].BodyMD())
	}
	if qs[1].BodyMD() != "*brahmacariya* - **KN Snp 2.14**" {
		t.Fatalf("hit 1 body = %q", qs[1].BodyMD())
	}
}

func TestQuoteFromRowRegeneratesStoredBodyMD(t *testing.T) {
	rows, err := storetest.CloneFixture(t).List()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 100 {
		t.Fatalf("fixture rows = %d, want the full seed corpus", len(rows))
	}
	for _, r := range rows {
		if got := quoteFromRow(r).BodyMD(); got != r.BodyMD {
			t.Fatalf("quote %d (%s): regenerated body =\n%s\nstored =\n%s", r.ID, r.SuttaID, got, r.BodyMD)
		}
	}
}

func TestRowsToQuotesMatchCanonicalExportFile(t *testing.T) {
	rows, err := storetest.CloneFixture(t).List()
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("..", "..", "exports", "shortest-first.md"))
	if err != nil {
		t.Fatal(err)
	}
	if got := quote.RenderExportFile(rowsToQuotes(rows)); got != string(want) {
		t.Fatalf("qm rendering of the fixture diverged from exports/shortest-first.md")
	}
}

func quotesTestDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "quotes.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	seed := []*quote.Quote{
		quote.New("MN 22", "the Buddha, MN 22", []string{"what I teach is just suffering and the cessation of suffering"}),
		quote.New("AN 4.77", "the Buddha, AN 4.77", []string{"one thing", "when developed", "is of great fruit"}),
		quote.New("KN Snp 2.14", "the Buddha, KN Snp 2.14", []string{"let the intelligent person live a celibate life"}),
	}
	for _, q := range seed {
		if _, err := s.Create(q); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func quotesCLIConfig(t *testing.T, dbPath string) string {
	t.Helper()
	raw := `{"quotes": {"db_path": "` + filepath.ToSlash(dbPath) + `"}}`
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(cfgPath, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	return cfgPath
}

func TestQueryQuotesQMIsDefault(t *testing.T) {
	cfgPath := quotesCLIConfig(t, quotesTestDB(t))
	code, out := runMain(t, cfgPath, "query", "quotes")
	if code != 0 {
		t.Fatalf("query quotes exit %d: %s", code, out)
	}
	want := quote.RenderExportFile([]*quote.Quote{
		quote.New("AN 4.77", "the Buddha, AN 4.77", []string{"one thing", "when developed", "is of great fruit"}),
		quote.New("KN Snp 2.14", "the Buddha, KN Snp 2.14", []string{"let the intelligent person live a celibate life"}),
		quote.New("MN 22", "the Buddha, MN 22", []string{"what I teach is just suffering and the cessation of suffering"}),
	})
	if out != want {
		t.Fatalf("default output =\n%s\nwant\n%s", out, want)
	}
}

func TestQueryQuotesJSONFormat(t *testing.T) {
	cfgPath := quotesCLIConfig(t, quotesTestDB(t))
	code, out := runMain(t, cfgPath, "query", "quotes", "--format", "json")
	if code != 0 {
		t.Fatalf("query quotes --format json exit %d: %s", code, out)
	}
	var rows []store.Quote
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("output not json: %v\n%s", err, out)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}
	if rows[0].SuttaID != "AN 4.77" || rows[0].BodyMD != "*one thing*  \n*when developed*  \n*is of great fruit* - **the Buddha, AN 4.77**" {
		t.Fatalf("row 0 = %+v", rows[0])
	}
	if rows[2].SuttaID != "MN 22" || rows[2].CharCount != 61 {
		t.Fatalf("row 2 = %+v, want MN 22 last at 61 runes", rows[2])
	}
}

func TestQueryQuotesTextFilter(t *testing.T) {
	cfgPath := quotesCLIConfig(t, quotesTestDB(t))
	code, out := runMain(t, cfgPath, "query", "quotes", "--text", "celibate")
	if code != 0 {
		t.Fatalf("query quotes --text exit %d: %s", code, out)
	}
	want := "*let the intelligent person live a celibate life* - **the Buddha, KN Snp 2.14**\n"
	if out != want {
		t.Fatalf("filtered output = %q, want %q", out, want)
	}

	code, out = runMain(t, cfgPath, "query", "quotes", "--text", "suffering -cessation")
	if code != 0 {
		t.Fatalf("query quotes negated exit %d: %s", code, out)
	}
	if out != "" {
		t.Fatalf("negated output = %q, want nothing", out)
	}
}

func TestQueryQuotesEmptyAndLimit(t *testing.T) {
	cfgPath := quotesCLIConfig(t, quotesTestDB(t))
	code, out := runMain(t, cfgPath, "query", "quotes", "--text", "zzzznope")
	if code != 0 || out != "" {
		t.Fatalf("empty result = %d/%q, want exit 0 printing nothing", code, out)
	}

	code, out = runMain(t, cfgPath, "query", "quotes", "--limit", "1")
	if code != 0 {
		t.Fatalf("limited exit %d: %s", code, out)
	}
	want := "*one thing*  \n*when developed*  \n*is of great fruit* - **the Buddha, AN 4.77**\n"
	if out != want {
		t.Fatalf("limited output = %q, want only the shortest quote", out)
	}
}

func TestQueryQuotesDBOverride(t *testing.T) {
	cfgPath := quotesCLIConfig(t, filepath.Join(t.TempDir(), "default.db"))
	overridden := quotesTestDB(t)
	code, out := runMain(t, cfgPath, "query", "quotes", "--db", overridden, "--text", "celibate")
	if code != 0 {
		t.Fatalf("query quotes --db exit %d: %s", code, out)
	}
	if !bytes.Contains([]byte(out), []byte("KN Snp 2.14")) {
		t.Fatalf("--db override ignored, output = %s", out)
	}
}

func TestQueryQuotesBadFormat(t *testing.T) {
	cfgPath := quotesCLIConfig(t, quotesTestDB(t))
	if code, _ := runMain(t, cfgPath, "query", "quotes", "--format", "xml"); code != 1 {
		t.Fatalf("bad format exit = %d, want 1", code)
	}
	if code, _ := runMain(t, cfgPath, "query", "quotes", "stray"); code != 1 {
		t.Fatalf("positional arg exit = %d, want 1", code)
	}
}

func buildQMSegmentsGraph(t *testing.T, cfg *config.Config) {
	t.Helper()
	dbPath := cfg.Canon.DBFile("qmtest")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatal(err)
	}
	st, err := graph.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Migrate(); err != nil {
		t.Fatal(err)
	}
	loader := graph.NewLoader(st, 100)
	text, err := loader.EnsureNode(model.NodeText, "mn1", `{}`)
	if err != nil {
		t.Fatal(err)
	}
	ord := func(v int) *int64 { return int64ptr(int64(v)) }
	type seg struct {
		key, pali, en string
	}
	for i, sg := range []seg{
		{"pli:mn1:0.1", "pariyāya", "the relational"},
		{"pli:mn1:0.2", "ekodhāna", "settled"},
		{"pli:mn1:0.3", "āsavā", ""},
	} {
		pid, err := loader.EnsureNode(model.NodeSegment, sg.key, `{"text":"`+sg.pali+`"}`)
		if err != nil {
			t.Fatal(err)
		}
		if err := loader.AddEdge(pid, text, model.EdgeChildOf, ord(i), "{}"); err != nil {
			t.Fatal(err)
		}
		if err := loader.SetFTS(pid, sg.pali, sg.en); err != nil {
			t.Fatal(err)
		}
		if sg.en != "" {
			eid, err := loader.EnsureNode(model.NodeSegment, "en/sujato:"+sg.key, `{"text":"`+sg.en+`"}`)
			if err != nil {
				t.Fatal(err)
			}
			if err := loader.AddEdge(eid, pid, model.EdgeTranslates, nil, "{}"); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := loader.Flush(); err != nil {
		t.Fatal(err)
	}
	runID, err := st.NewRun("bc", "ec", "v0.4.20260926")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishRun(runID); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	if err := graph.PublishDB(cfg, "qmtest", dbPath); err != nil {
		t.Fatal(err)
	}
}

func int64ptr(v int64) *int64 { return &v }

func TestQuerySegmentsQM(t *testing.T) {
	cfg, cfgPath := wordQueryConfig(t)
	buildQMSegmentsGraph(t, cfg)

	code, out := runMain(t, cfgPath, "query", "segments", "--muid", "mn1", "--format", "qm")
	if code != 0 {
		t.Fatalf("query segments --format qm exit %d: %s", code, out)
	}
	want := "*the relational*  \n*settled*  \n*āsavā* - **MN 1**\n"
	if out != want {
		t.Fatalf("qm segments = %q, want %q", out, want)
	}

	code, out = runMain(t, cfgPath, "query", "segments", "--muid", "mn1", "--with-en", "--format", "qm")
	if code != 0 {
		t.Fatalf("query segments --with-en --format qm exit %d: %s", code, out)
	}
	want = "*the relational*  \n*settled* - **MN 1**\n"
	if out != want {
		t.Fatalf("qm with-en segments = %q, want %q", out, want)
	}
}

func TestQuerySegmentsJSONDefaultUnchanged(t *testing.T) {
	cfg, cfgPath := wordQueryConfig(t)
	buildQMSegmentsGraph(t, cfg)

	code, out := runMain(t, cfgPath, "query", "segments", "--muid", "mn1")
	if code != 0 {
		t.Fatalf("query segments exit %d: %s", code, out)
	}
	want := `[{"Muid":"mn1","SegKey":"0.1","Pali":"pariyāya","En":"the relational","Author":"sujato"},` +
		`{"Muid":"mn1","SegKey":"0.2","Pali":"ekodhāna","En":"settled","Author":"sujato"},` +
		`{"Muid":"mn1","SegKey":"0.3","Pali":"āsavā","En":"","Author":""}]` + "\n"
	if out != want {
		t.Fatalf("default segments output = %q, want %q", out, want)
	}
}

func TestQuerySearchQM(t *testing.T) {
	cfg, cfgPath := wordQueryConfig(t)
	buildQMSegmentsGraph(t, cfg)

	code, out := runMain(t, cfgPath, "query", "search", "--text", "relational", "--format", "qm")
	if code != 0 {
		t.Fatalf("query search --format qm exit %d: %s", code, out)
	}
	want := "*the relational* - **MN 1**\n"
	if out != want {
		t.Fatalf("qm search = %q, want %q", out, want)
	}
}

func TestQuerySearchJSONDefaultUnchanged(t *testing.T) {
	cfg, cfgPath := wordQueryConfig(t)
	buildQMSegmentsGraph(t, cfg)

	code, out := runMain(t, cfgPath, "query", "search", "--text", "settled")
	if code != 0 {
		t.Fatalf("query search exit %d: %s", code, out)
	}
	want := `[{"Muid":"mn1","SegKey":"0.2","Pali":"ekodhāna","En":"settled","Author":"sujato"}]` + "\n"
	if out != want {
		t.Fatalf("default search output = %q, want %q", out, want)
	}
}

func TestQuerySegmentsBadFormat(t *testing.T) {
	cfg, cfgPath := wordQueryConfig(t)
	buildQMSegmentsGraph(t, cfg)
	if code, _ := runMain(t, cfgPath, "query", "segments", "--muid", "mn1", "--format", "yaml"); code != 1 {
		t.Fatalf("bad segments format exit = %d, want 1", code)
	}
}
