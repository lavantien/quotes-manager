package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lavantien/quotes-manager/internal/canon/model"
	"github.com/lavantien/quotes-manager/internal/config"
)

func testConfig(t *testing.T) (*config.Config, string) {
	t.Helper()
	tmp := t.TempDir()
	cfg := config.Defaults()
	cfg.Canon.Paths.DataDir = tmp
	cfg.Canon.Paths.DBDir = filepath.Join(tmp, "db")
	cfg.Canon.Paths.DistDir = filepath.Join(tmp, "dist")
	cfg.Canon.Paths.ManifestsDir = filepath.Join(tmp, "docs", "manifests")
	cfg.Canon.Paths.ReportsDir = filepath.Join(tmp, "docs", "reports")
	cfg.Canon.E2E.Muids = nil
	return cfg, tmp
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeBilaraTree(t *testing.T, root string) {
	t.Helper()
	writeFile(t, filepath.Join(root, "root/pli/ms/sutta/mn/mn1_root-pli-ms.json"), `{
		"mn1:0.1": "Middle Discourses 1",
		"mn1:1.1": "So I have heard.",
		"mn1:1.1": "duplicate occurrence",
		"mn1:1.2": "",
		"mn1:2.1": "Deep indeed."
	}`)
	writeFile(t, filepath.Join(root, "root/pli/ms/sutta/mn/mn2_root-pli-ms.json"), `{
		"mn2:0.1": "Middle Discourses 2",
		"mn2:1.1": "All the defilements."
	}`)
	writeFile(t, filepath.Join(root, "root/pli/ms/vinaya/pli-tv-bu-pm_root-pli-ms.json"), `{
		"pli-tv-bu-pm:0.1": "The monks' rules."
	}`)
	writeFile(t, filepath.Join(root, "translation/en/sujato/sutta/mn/mn1_translation-en-sujato.json"), `{
		"mn1:0.1": "Middle Discourses 1",
		"mn1:1.1": "So I have heard.",
		"mn1:1.2": "At one time.",
		"mn1:2.1": "Deep indeed."
	}`)
	writeFile(t, filepath.Join(root, "translation/en/sujato/name/sutta/mn-name_translation-en-sujato.json"), `{
		"mn-name:1.mn": "The Middle Discourses",
		"mn-name:2.mn1": "The Root of All Things",
		"mn-name:3.mn2": "All the Defilements"
	}`)
	writeFile(t, filepath.Join(root, "html/pli/ms/sutta/mn/mn1_html.json"), `{
		"mn1:0.1": "<span class='evam'>",
		"mn1:1.1": "<p>",
		"mn1:1.2": "</p>"
	}`)
	writeFile(t, filepath.Join(root, "reference/pli/ms/sutta/mn/mn1_reference.json"), `{
		"mn1:0.1": "MN i 1",
		"mn1:1.1": "MN i 2",
		"mn1:1.2": "MN i 3",
		"mn1:2.1": "MN i 4"
	}`)
	writeFile(t, filepath.Join(root, "_publication/en/sujato/mn1_publication.json"), "{}")
	writeFile(t, filepath.Join(root, "README.md"), "synthetic")
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var out []string
	for l := range strings.SplitSeq(strings.TrimRight(string(raw), "\n"), "\n") {
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

func TestParseBilaraArtifacts(t *testing.T) {
	cfg, _ := testConfig(t)
	root := cfg.Canon.BilaraDir()
	writeBilaraTree(t, root)

	stats, err := Parse(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Files != 6 {
		t.Errorf("files = %d; want 6 segment files", stats.Files)
	}
	if stats.Names != 3 {
		t.Errorf("names = %d; want 3", stats.Names)
	}

	rootLines := readLines(t, filepath.Join(cfg.Canon.ParsedDir(), "root.ndjson"))
	if len(rootLines) != 7 {
		t.Fatalf("root.ndjson lines = %d; want 7", len(rootLines))
	}
	var first segLine
	if err := json.Unmarshal([]byte(rootLines[0]), &first); err != nil {
		t.Fatal(err)
	}
	if first.SegKey != "0.1" || first.Text != "Middle Discourses 1" || first.Stream != "root" {
		t.Errorf("first root record = %+v", first)
	}

	trans := readLines(t, filepath.Join(cfg.Canon.ParsedDir(), "translation.ndjson"))
	if len(trans) != 4 {
		t.Fatalf("translation.ndjson lines = %d; want 4", len(trans))
	}
	var tl segLine
	if err := json.Unmarshal([]byte(trans[0]), &tl); err != nil {
		t.Fatal(err)
	}
	if tl.Author != "sujato" {
		t.Errorf("translation author = %q; want sujato", tl.Author)
	}

	raw, err := os.ReadFile(filepath.Join(cfg.Canon.ParsedDir(), "meta.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "null") {
		t.Error("meta.json must not contain null values")
	}
	var meta struct {
		Files map[string]struct {
			DupKeys     []string `json:"dup_keys"`
			NFCChanged  bool     `json:"nfc_changed"`
			EmptyValues int      `json:"empty_values"`
			Muid        string   `json:"muid"`
			Stream      string   `json:"stream"`
		} `json:"files"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatal(err)
	}
	if len(meta.Files) != 6 {
		t.Fatalf("meta files = %d; want 6", len(meta.Files))
	}
	mn1 := meta.Files["root/pli/ms/sutta/mn/mn1_root-pli-ms.json"]
	if len(mn1.DupKeys) != 1 || mn1.DupKeys[0] != "mn1:1.1" {
		t.Errorf("mn1 dup keys = %v", mn1.DupKeys)
	}
	if mn1.EmptyValues != 1 {
		t.Errorf("mn1 empty values = %d; want 1", mn1.EmptyValues)
	}
	if meta.Files["html/pli/ms/sutta/mn/mn1_html.json"].Stream != "html" {
		t.Error("html meta stream not recorded")
	}

	var nameRec struct {
		Muid string `json:"muid"`
		Name string `json:"name"`
		Lang string `json:"lang"`
	}
	if err := json.Unmarshal([]byte(readLines(t, filepath.Join(cfg.Canon.ParsedDir(), "names.ndjson"))[0]), &nameRec); err != nil {
		t.Fatal(err)
	}
	if nameRec.Muid != "mn" || nameRec.Name != "The Middle Discourses" || nameRec.Lang != "en" {
		t.Errorf("first name record = %+v", nameRec)
	}
}

func TestParseBilaraQuarantinesForeignMuids(t *testing.T) {
	cfg, _ := testConfig(t)
	root := cfg.Canon.BilaraDir()
	writeBilaraTree(t, root)
	writeFile(t, filepath.Join(root, "root/pli/ms/sutta/an/an1/an1.1-2_root-pli-ms.json"), `{
		"an1.1:0.1": "Ones 1",
		"an1.1:1.1": "Mind is the forerunner.",
		"an1.2:1.1": "Not by hatred.",
		"mn2:9.9": "stray foreign record",
		"introduction": "colonless key"
	}`)
	writeFile(t, filepath.Join(root, "translation/en/sujato/sutta/an/an1/an1.1-2_translation-en-sujato.json"), `{
		"an1.1:0.1": "Ones 1",
		"an1.1:1.1": "Mind is the forerunner.",
		"an1.2:1.1": "Not by hatred."
	}`)

	if _, err := Parse(t.Context(), cfg); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"root", "translation"} {
		for _, l := range readLines(t, filepath.Join(cfg.Canon.ParsedDir(), s+".ndjson")) {
			bad := (strings.Contains(l, `"muid":"mn2"`) && strings.Contains(l, `"seg_key":"9.9"`)) ||
				strings.Contains(l, `"seg_key":"introduction"`)
			if bad {
				t.Errorf("quarantined record leaked into %s.ndjson: %s", s, l)
			}
		}
	}
	rootLines := readLines(t, filepath.Join(cfg.Canon.ParsedDir(), "root.ndjson"))
	anLines := 0
	for _, l := range rootLines {
		if strings.Contains(l, `"muid":"an1.`) {
			anLines++
		}
	}
	if anLines != 3 {
		t.Errorf("an1 grouped records = %d; want 3 (an1.1 x2 + an1.2)", anLines)
	}

	raw, err := os.ReadFile(filepath.Join(cfg.Canon.ParsedDir(), "meta.json"))
	if err != nil {
		t.Fatal(err)
	}
	var meta struct {
		Files map[string]struct {
			RecordMuids      []string `json:"record_muids"`
			PrefixMismatches []string `json:"prefix_mismatches"`
		} `json:"files"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatal(err)
	}
	grouped := meta.Files["root/pli/ms/sutta/an/an1/an1.1-2_root-pli-ms.json"]
	if fmt.Sprint(grouped.RecordMuids) != fmt.Sprint([]string{"an1.1", "an1.2"}) {
		t.Errorf("grouped record_muids = %v; want [an1.1 an1.2]", grouped.RecordMuids)
	}
	if fmt.Sprint(grouped.PrefixMismatches) != fmt.Sprint([]string{"mn2:9.9", "introduction"}) {
		t.Errorf("grouped prefix_mismatches = %v; want [mn2:9.9 introduction]", grouped.PrefixMismatches)
	}
	mn1 := meta.Files["root/pli/ms/sutta/mn/mn1_root-pli-ms.json"]
	if fmt.Sprint(mn1.RecordMuids) != fmt.Sprint([]string{"mn1"}) || len(mn1.PrefixMismatches) != 0 {
		t.Errorf("mn1 record_muids/mismatches = %v/%v", mn1.RecordMuids, mn1.PrefixMismatches)
	}
}

func TestParseBilaraEmptyWalkFails(t *testing.T) {
	cfg, _ := testConfig(t)
	writeFile(t, filepath.Join(cfg.Canon.BilaraDir(), "misc/empty.txt"), "nothing kept")
	if _, err := Parse(t.Context(), cfg); err == nil {
		t.Fatal("parse of an empty bilara corpus must fail, not build a silent empty graph")
	}
}

func TestParseFilterLimitsCorpus(t *testing.T) {
	cfg, _ := testConfig(t)
	writeBilaraTree(t, cfg.Canon.BilaraDir())
	cfg.Canon.E2E.Muids = []string{"mn1"}

	if _, err := Parse(t.Context(), cfg); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(cfg.Canon.ParsedDir(), "meta.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "mn2") || strings.Contains(string(raw), "pli-tv-bu-pm") {
		t.Error("filtered muids must not appear in meta.json")
	}
	for _, s := range []string{"root", "translation", "html", "reference"} {
		for _, l := range readLines(t, filepath.Join(cfg.Canon.ParsedDir(), s+".ndjson")) {
			if strings.Contains(l, "mn2") || strings.Contains(l, "pli-tv-bu-pm") {
				t.Errorf("filtered muid leaked into %s.ndjson: %s", s, l)
			}
		}
	}
}

func writeHtmlEdition(t *testing.T, cfg *config.Config, name, body string) {
	t.Helper()
	writeFile(t, filepath.Join(cfg.Canon.EditionsDir(), "en", "sujato", "mn", "html", name), body)
}

func TestParseEditionsHtmlSegments(t *testing.T) {
	cfg, _ := testConfig(t)
	writeBilaraTree(t, cfg.Canon.BilaraDir())
	writeHtmlEdition(t, cfg, "sujato-mn-2026-01-02.html", `<html><body>
<section id="mn1:0.1">Middle Discourses 1</section>
<section id="mn1:1.1">So   I   have heard.</section>
<section id="mn2:1.1">All the defilements.</section>
</body></html>`)

	stats, err := Parse(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if stats.HtmlSegments != 3 {
		t.Fatalf("html segments = %d; want 3", stats.HtmlSegments)
	}
	lines := readLines(t, filepath.Join(cfg.Canon.ParsedDir(), htmlSegmentsFile))
	if len(lines) != 3 {
		t.Fatalf("html-editions.ndjson lines = %d; want 3", len(lines))
	}
	var got []model.HTMLSegment
	for _, l := range lines {
		var s model.HTMLSegment
		if err := json.Unmarshal([]byte(l), &s); err != nil {
			t.Fatal(err)
		}
		got = append(got, s)
	}
	if got[0].Author != "sujato" || got[0].Muid != "mn1" || got[0].SegKey != "0.1" || got[0].Text != "Middle Discourses 1" {
		t.Errorf("first html segment = %+v", got[0])
	}
	if got[1].Text != "So I have heard." {
		t.Errorf("whitespace not folded: %q", got[1].Text)
	}
	if got[2].Muid != "mn2" || got[2].SegKey != "1.1" {
		t.Errorf("third html segment = %+v", got[2])
	}

	raw, err := os.ReadFile(filepath.Join(cfg.Canon.Paths.ManifestsDir, "editions-discovery.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"html editions: 1", "| sujato | mn | 2026-01-02 | sujato-mn-2026-01-02.html |"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("discovery report missing %q:\n%s", want, raw)
		}
	}
}

func TestParseEditionsAbsentDegrades(t *testing.T) {
	cfg, _ := testConfig(t)
	writeBilaraTree(t, cfg.Canon.BilaraDir())

	stats, err := Parse(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if stats.HtmlSegments != 0 {
		t.Errorf("html segments = %d; want 0 without an editions tree", stats.HtmlSegments)
	}
	if _, err := os.Stat(filepath.Join(cfg.Canon.ParsedDir(), htmlSegmentsFile)); !os.IsNotExist(err) {
		t.Errorf("html-editions.ndjson must not exist without an editions tree (stat err = %v)", err)
	}
	raw, err := os.ReadFile(filepath.Join(cfg.Canon.Paths.ManifestsDir, "editions-discovery.md"))
	if err != nil {
		t.Fatalf("discovery report missing: %v", err)
	}
	if !strings.Contains(string(raw), "html editions: 0") {
		t.Errorf("discovery report = %s", raw)
	}
}

func TestParseRefreshesManifestHtmls(t *testing.T) {
	cfg, _ := testConfig(t)
	writeBilaraTree(t, cfg.Canon.BilaraDir())
	writeHtmlEdition(t, cfg, "sujato-mn-2026-01-02.html",
		`<section id="mn1:1.1">So I have heard.</section>`)
	m := Manifest{}
	m.Updated = "2026-01-01T00:00:00Z"
	m.Bilara.Commit = "bc"
	m.Htmls = []HtmlEdition{{Author: "sujato", Collection: "mn", FileName: "old-2025-01-01.html", BuildDate: "2025-01-01"}}
	if err := WriteManifest(cfg, m); err != nil {
		t.Fatal(err)
	}

	if _, err := Parse(t.Context(), cfg); err != nil {
		t.Fatal(err)
	}
	back, err := ReadManifest(cfg)
	if err != nil {
		t.Fatal(err)
	}
	want := []HtmlEdition{{Author: "sujato", Collection: "mn", FileName: "sujato-mn-2026-01-02.html", BuildDate: "2026-01-02"}}
	if fmt.Sprint(back.Htmls) != fmt.Sprint(want) {
		t.Errorf("manifest htmls = %v; want %v", back.Htmls, want)
	}
	if back.Bilara.Commit != "bc" || back.Updated != "2026-01-01T00:00:00Z" {
		t.Errorf("manifest fields clobbered: %+v", back)
	}
}
