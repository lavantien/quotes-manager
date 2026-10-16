package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lavantien/quotes-manager/internal/canon/graph"
	"github.com/lavantien/quotes-manager/internal/canon/model"
	"github.com/lavantien/quotes-manager/internal/config"
)

func runMain(t *testing.T, cfgPath string, args ...string) (int, string) {
	t.Helper()
	old := stdout
	stdout = new(bytes.Buffer)
	code := Main(append([]string{"--config", cfgPath}, args...))
	out := stdout.(*bytes.Buffer).String()
	stdout = old
	return code, out
}

func buildWordGraph(t *testing.T, cfg *config.Config) {
	t.Helper()
	dbPath := cfg.Canon.DBFile("wordtest")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatal(err)
	}
	store, err := graph.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}
	loader := graph.NewLoader(store, 100)
	hw, err := loader.EnsureNode(model.NodeHeadword, "dpd:dhamma",
		`{"lemma_1":"dhamma","pos":"m","meaning_1":"the teaching","root_key":"dhṛ","family_root":"dhṛ"}`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loader.EnsureNode(model.NodeDpdForm, "dpd-form:dhamma", `{"headwords":["dhamma"]}`); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"pli:mn1:1.1", "pli:mn2:1.1"} {
		seg, err := loader.EnsureNode(model.NodeSegment, key, `{"text":"dhamma"}`)
		if err != nil {
			t.Fatal(err)
		}
		if err := loader.AddEdge(seg, hw, model.EdgeHasWord, nil, `{"forms":["dhamma"]}`); err != nil {
			t.Fatal(err)
		}
	}
	if err := loader.Flush(); err != nil {
		t.Fatal(err)
	}
	runID, err := store.NewRun("bc", "ec", "v0.4.20260926")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.FinishRun(runID); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := graph.PublishDB(cfg, "wordtest", dbPath); err != nil {
		t.Fatal(err)
	}
}

func wordQueryConfig(t *testing.T) (*config.Config, string) {
	t.Helper()
	tmp := t.TempDir()
	work := filepath.Join(tmp, "work")
	raw := `{
  "canon": {
    "paths": {
      "data_dir": "` + filepath.ToSlash(work) + `",
      "db_dir": "` + filepath.ToSlash(filepath.Join(work, "db")) + `",
      "dist_dir": "` + filepath.ToSlash(filepath.Join(work, "dist")) + `",
      "manifests_dir": "` + filepath.ToSlash(filepath.Join(work, "docs", "manifests")) + `",
      "reports_dir": "` + filepath.ToSlash(filepath.Join(work, "docs", "reports")) + `"
    }
  }
}`
	cfgPath := filepath.Join(tmp, "config.json")
	if err := os.WriteFile(cfgPath, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	return cfg, cfgPath
}

func buildSegmentsGraph(t *testing.T, cfg *config.Config) {
	t.Helper()
	dbPath := cfg.Canon.DBFile("segtest")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatal(err)
	}
	store, err := graph.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}
	loader := graph.NewLoader(store, 100)
	text, err := loader.EnsureNode(model.NodeText, "mn1", `{}`)
	if err != nil {
		t.Fatal(err)
	}
	ord := func(v int64) *int64 { return &v }
	for i, pali := range []string{"pariyāya", "ekodhāna", "āsavā"} {
		seg, err := loader.EnsureNode(model.NodeSegment, fmt.Sprintf("pli:mn1:0.%d", i+1), `{"text":"`+pali+`"}`)
		if err != nil {
			t.Fatal(err)
		}
		if err := loader.AddEdge(seg, text, model.EdgeChildOf, ord(int64(i)), "{}"); err != nil {
			t.Fatal(err)
		}
	}
	if err := loader.Flush(); err != nil {
		t.Fatal(err)
	}
	runID, err := store.NewRun("bc", "ec", "v0.4.20260926")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.FinishRun(runID); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := graph.PublishDB(cfg, "segtest", dbPath); err != nil {
		t.Fatal(err)
	}
}

func TestQueryWordForm(t *testing.T) {
	cfg, cfgPath := wordQueryConfig(t)
	buildWordGraph(t, cfg)

	code, out := runMain(t, cfgPath, "query", "word", "--form", "dhammā")
	if code != 0 {
		t.Fatalf("query word --form exit %d: %s", code, out)
	}
	var entries []struct {
		Lemma1   string `json:"lemma_1"`
		POS      string `json:"pos"`
		Meaning1 string `json:"meaning_1"`
		Root     string `json:"root"`
		Segments int64  `json:"segments"`
	}
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("output not json: %v\n%s", err, out)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %s", out)
	}
	e := entries[0]
	if e.Lemma1 != "dhamma" || e.POS != "m" || e.Meaning1 != "the teaching" || e.Root != "dhṛ" || e.Segments != 2 {
		t.Errorf("entry = %+v; want dhamma with 2 segments", e)
	}

	code, out = runMain(t, cfgPath, "query", "word", "--form", "zzzz")
	if code != 0 || !strings.Contains(out, "[]") {
		t.Errorf("unknown form = %d/%s; want exit 0 with []", code, out)
	}
}

func TestQueryWordHeadword(t *testing.T) {
	cfg, cfgPath := wordQueryConfig(t)
	buildWordGraph(t, cfg)

	code, out := runMain(t, cfgPath, "query", "word", "--headword", "dhamma")
	if code != 0 {
		t.Fatalf("query word --headword exit %d: %s", code, out)
	}
	var usage struct {
		Headword string   `json:"headword"`
		Segments int64    `json:"segments"`
		Refs     []string `json:"refs"`
	}
	if err := json.Unmarshal([]byte(out), &usage); err != nil {
		t.Fatalf("output not json: %v\n%s", err, out)
	}
	if usage.Headword != "dhamma" || usage.Segments != 2 || usage.Refs != nil {
		t.Errorf("usage = %+v; want 2 segments without refs", usage)
	}

	code, out = runMain(t, cfgPath, "query", "word", "--headword", "dhamma", "--segments", "--limit", "1")
	if code != 0 {
		t.Fatalf("query word --headword --segments exit %d: %s", code, out)
	}
	usage = struct {
		Headword string   `json:"headword"`
		Segments int64    `json:"segments"`
		Refs     []string `json:"refs"`
	}{}
	if err := json.Unmarshal([]byte(out), &usage); err != nil {
		t.Fatal(err)
	}
	if usage.Segments != 2 || len(usage.Refs) != 1 || usage.Refs[0] != "mn1:1.1" {
		t.Errorf("limited usage = %+v; want 2 total with 1 ref", usage)
	}
}

func TestQueryTreeInvalidTypesFails(t *testing.T) {
	cfg, cfgPath := wordQueryConfig(t)
	buildWordGraph(t, cfg)

	if code, out := runMain(t, cfgPath, "query", "tree", "--division", "mn", "--types", "bogus"); code == 0 {
		t.Fatalf("query tree with an invalid types filter must exit non-zero: %s", out)
	}
	if code, out := runMain(t, cfgPath, "query", "tree"); code == 0 {
		t.Fatalf("query tree without --division must exit non-zero: %s", out)
	}
	code, out := runMain(t, cfgPath, "query", "tree", "--division", "mn", "--types", "all")
	if code != 0 || !strings.Contains(out, "[]") {
		t.Fatalf("query tree over a division-less graph = %d/%s; want exit 0 with []", code, out)
	}
}

func TestQuerySearchValidation(t *testing.T) {
	cfg, cfgPath := wordQueryConfig(t)
	buildWordGraph(t, cfg)

	if code, out := runMain(t, cfgPath, "query", "search", "--text", "   ", "--lang", "en"); code == 0 {
		t.Fatalf("query search with blank text must exit non-zero: %s", out)
	}
	code, out := runMain(t, cfgPath, "query", "search", "--text", "dhamma", "--lang", "en")
	if code != 0 || !strings.Contains(out, "[]") {
		t.Fatalf("query search over an empty fts table = %d/%s; want exit 0 with []", code, out)
	}
}

func TestStagesFailWithoutArtifacts(t *testing.T) {
	_, cfgPath := wordQueryConfig(t)
	for _, cmd := range []string{"parse", "load", "export"} {
		if code, out := runMain(t, cfgPath, cmd); code == 0 {
			t.Fatalf("%s without artifacts must exit non-zero: %s", cmd, out)
		}
	}
}

func TestReportRejectsPositionalArgs(t *testing.T) {
	cfg, cfgPath := wordQueryConfig(t)
	buildWordGraph(t, cfg)

	if code, out := runMain(t, cfgPath, "report", "stray"); code == 0 {
		t.Fatalf("report with a positional must exit non-zero: %s", out)
	}
	if code, out := runMain(t, cfgPath, "report", "--out", filepath.ToSlash(filepath.Join(t.TempDir(), "r")), "stray"); code == 0 {
		t.Fatalf("report with flags plus a positional must exit non-zero: %s", out)
	}
}

func TestReportVerbosePrintsResolvedDir(t *testing.T) {
	cfg, cfgPath := wordQueryConfig(t)
	buildWordGraph(t, cfg)

	custom := filepath.Join(t.TempDir(), "custom-report")
	code, out := runMain(t, cfgPath, "-v", "report", "--out", filepath.ToSlash(custom))
	if code != 0 {
		t.Fatalf("report --out exit %d: %s", code, out)
	}
	if !strings.Contains(out, filepath.ToSlash(custom)) {
		t.Fatalf("verbose report line must print the --out dir:\n%s", out)
	}

	code, out = runMain(t, cfgPath, "-v", "report")
	if code != 0 {
		t.Fatalf("report exit %d: %s", code, out)
	}
	if !strings.Contains(out, cfg.Canon.ReportsDirFor(todayUTC())) {
		t.Fatalf("verbose report line must print the dated reports dir:\n%s", out)
	}
}

func TestQueryWordFlagValidation(t *testing.T) {
	cfg, cfgPath := wordQueryConfig(t)
	buildWordGraph(t, cfg)

	code, _ := runMain(t, cfgPath, "query", "word")
	if code != 1 {
		t.Errorf("bare word exit = %d; want 1", code)
	}
	if code, _ := runMain(t, cfgPath, "query", "word", "--form", "a", "--headword", "b"); code != 1 {
		t.Error("both --form and --headword must be a usage error")
	}
}

func TestQuerySegmentsOffset(t *testing.T) {
	cfg, cfgPath := wordQueryConfig(t)
	buildSegmentsGraph(t, cfg)

	code, out := runMain(t, cfgPath, "query", "segments", "--muid", "mn1", "--limit", "1", "--offset", "1")
	if code != 0 {
		t.Fatalf("query segments offset exit %d: %s", code, out)
	}
	var rows []struct {
		Muid   string
		SegKey string
	}
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("output not json: %v\n%s", err, out)
	}
	if len(rows) != 1 || rows[0].Muid != "mn1" || rows[0].SegKey != "0.2" {
		t.Fatalf("offset window = %s, want only the mn1 0.2 row", out)
	}

	if code, _ := runMain(t, cfgPath, "query", "segments", "--muid", "mn1", "--offset", "-1"); code != 1 {
		t.Error("negative offset must be a usage error")
	}
}
