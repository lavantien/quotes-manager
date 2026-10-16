package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lavantien/quotes-manager/internal/canon/graph"
	"github.com/lavantien/quotes-manager/internal/canon/model"
)

func TestWriteManifestDeterministic(t *testing.T) {
	cfg, _ := testConfig(t)
	build := func() string {
		m := Manifest{}
		m.Updated = "2026-01-02T03:04:05Z"
		m.Bilara.Commit = "bc"
		m.Bilara.Date = "2026-01-02"
		m.Editions.Commit = "ec"
		m.Htmls = []HtmlEdition{
			{Author: "sujato", Collection: "mn", FileName: "sujato-mn-2026-01-02.html", BuildDate: "2026-01-02"},
			{Author: "sujato", Collection: "mn", FileName: "sujato-mn-2026-01-01.html", BuildDate: "2026-01-01"},
			{Author: "brahmali", Collection: "pli-tv-vi", FileName: "brahmali-vi-2026-02-02.html", BuildDate: "2026-02-02"},
		}
		if err := WriteManifest(cfg, m); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(cfg.Canon.ManifestFile())
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	first := build()
	second := build()
	if first != second {
		t.Error("manifest writes must be byte-identical for equal inputs")
	}
	if strings.Index(first, `"sujato"`) < strings.Index(first, `"brahmali"`) {
		t.Error("htmls must be sorted by author first")
	}
	if !strings.Contains(first, `"htmls": [`) {
		t.Errorf("htmls must marshal as an array, got:\n%s", first)
	}

	back, err := ReadManifest(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Htmls) != 3 || back.Bilara.Commit != "bc" || back.Editions.Commit != "ec" {
		t.Errorf("round trip = %+v", back)
	}
}

func TestWriteManifestEmptyHtmlsArray(t *testing.T) {
	cfg, _ := testConfig(t)
	if err := WriteManifest(cfg, Manifest{}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(cfg.Canon.ManifestFile())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"htmls": []`) {
		t.Errorf("nil htmls must render as an empty array, got:\n%s", string(raw))
	}
}

func TestReportWritesArtifacts(t *testing.T) {
	cfg, _ := testConfig(t)
	writeParsedCorpus(t, cfg)
	writeTestManifest(t, cfg)

	res, err := Load(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	store, err := graph.Open(res.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	runID, err := store.NewRun("bc", "ec", "v0.4.20260926")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.InsertDiscrepancies(t.Context(), runID, []model.Discrepancy{
		{Check: model.CheckKeysetRootTranslation, Severity: model.Warn, Muid: "mn2", Detail: "untranslated by sujato: mn2:1.1"},
		{Check: model.CheckDuplicateKeys, Severity: model.Error, Detail: "dup keys"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.FinishRun(runID); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	outDir := filepath.Join(cfg.Canon.Paths.ReportsDir, "custom")
	if err := Report(t.Context(), cfg, outDir); err != nil {
		t.Fatal(err)
	}
	md, err := os.ReadFile(filepath.Join(outDir, "report.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(md)
	for _, want := range []string{"| mn | 2 | 0 |", "| vinaya/bu-pm | 1 | 0 |", "B001/warn", "B004/error", "bilara_commit: bc"} {
		if !strings.Contains(text, want) {
			t.Errorf("report.md missing %q:\n%s", want, text)
		}
	}
	if _, err := os.Stat(filepath.Join(outDir, "report.json")); err != nil {
		t.Fatal(err)
	}

	if err := Report(t.Context(), cfg, ""); err != nil {
		t.Fatal(err)
	}
	date := time.Now().UTC().Format(reportDateLayout)
	if _, err := os.Stat(filepath.Join(cfg.Canon.ReportsDirFor(date), "report.md")); err != nil {
		t.Fatal(err)
	}
}
