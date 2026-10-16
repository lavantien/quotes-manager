package cli

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lavantien/quotes-manager/internal/canon/graph"
	"github.com/lavantien/quotes-manager/internal/canon/source/dpd"
	"github.com/lavantien/quotes-manager/internal/config"

	_ "github.com/duckdb/duckdb-go/v2"
	_ "github.com/mattn/go-sqlite3"
)

func readCfg(t *testing.T, cfgPath string) *config.Config {
	t.Helper()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.email=e2e@quotes-manager", "-c", "user.name=e2e"}, args...)...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %s in %s: %v: %s", strings.Join(args, " "), dir, err, out.String())
	}
}

func buildHermeticBilara(t *testing.T, path string) {
	t.Helper()
	files := map[string]string{
		"root/pli/ms/sutta/mn/mn1_root-pli-ms.json": `{
	"mn1:0.1": "Middle Discourses 1",
	"mn1:1.1": "Evaṁ me sutaṁ.",
	"mn1:1.1": "duplicate occurrence",
	"mn1:2.1": "Deep indeed."
}`,
		"root/pli/ms/sutta/mn/mn2_root-pli-ms.json": `{
	"mn2:0.1": "Middle Discourses 2",
	"mn2:1.1": "All the defilements."
}`,
		"root/pli/ms/vinaya/pli-tv-bu-pm_root-pli-ms.json": `{
	"pli-tv-bu-pm:0.1": "The monks' rules."
}`,
		"translation/en/sujato/sutta/mn/mn1_translation-en-sujato.json": `{
	"mn1:0.1": "Middle Discourses 1",
	"mn1:1.1": "So I have heard.",
	"mn1:2.1": "Deep indeed."
}`,
		"translation/en/brahmali/sutta/mn/mn2_translation-en-brahmali.json": `{
	"mn2:1.1": "All the defilements."
}`,
		"translation/en/sujato/name/sutta/mn-name_translation-en-sujato.json": `{
	"mn-name:1.mn": "The Middle Discourses",
	"mn-name:2.mn1": "The Root of All Things",
	"mn-name:3.mn2": "All the Defilements"
}`,
		"html/pli/ms/sutta/mn/mn1_html.json": `{
	"mn1:0.1": "<p>",
	"mn1:1.1": "<p>"
}`,
		"reference/pli/ms/sutta/mn/mn1_reference.json": `{
	"mn1:0.1": "MN i 1",
	"mn1:1.1": "MN i 2",
	"mn1:2.1": "MN i 3"
}`,
		"root/pli/ms/sutta/mn/notes.txt": "stray file under a kept cone stays",
		"misc/stray.txt":                 "stray file the prune pass must remove",
	}
	for rel, body := range files {
		p := filepath.Join(path, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitIn(t, path, "init", "-b", "main")
	gitIn(t, path, "add", "-A")
	gitIn(t, path, "commit", "-m", "chore: synthetic bilara corpus")
}

func buildHermeticEditions(t *testing.T, path string) {
	t.Helper()
	htmlPath := filepath.Join(path, "en/sujato/mn/html/Middle-Discourses-sujato-2026-01-02.html")
	if err := os.MkdirAll(filepath.Dir(htmlPath), 0o755); err != nil {
		t.Fatal(err)
	}
	html := `<!doctype html>
<html><body>
<article id="mn1">
<h3 class="sutta-title heading"><span class="sutta-heading acronym">MN 1</span></h3>
<p id="mn1:0.1">Middle Discourses 1</p>
<p id="mn1:1.1">So I have heard.<a epub:type="noteref" href="#note-1" id="noteref-1" role="doc-noteref">1</a></p>
<p id="mn1:2.1">Deep indeed.</p>
</article>
<article id="mn2">
<p id="mn2:1.1">All the defilements.</p>
</article>
</body></html>`
	if err := os.WriteFile(htmlPath, []byte(html), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "last_run_date"), []byte("2026-01-02"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, path, "init", "-b", "main")
	gitIn(t, path, "add", "-A")
	gitIn(t, path, "commit", "-m", "chore: synthetic editions corpus")
}

func writeHermeticConfig(t *testing.T, tmp string) string {
	t.Helper()
	bilaraSrc := filepath.Join(tmp, "bilara-src")
	editionsSrc := filepath.Join(tmp, "editions-src")
	work := filepath.Join(tmp, "work")
	_, dpdTar, err := dpd.WriteFixture(filepath.Join(tmp, "dpd-fixture"))
	if err != nil {
		t.Fatalf("dpd fixture: %v", err)
	}
	cfg := map[string]any{
		"canon": map[string]any{
			"repos": map[string]any{
				"bilara":   map[string]any{"url": fileURL(bilaraSrc), "branch": "main", "dir": "bilara-data"},
				"editions": map[string]any{"url": fileURL(editionsSrc), "branch": "main", "dir": "editions"},
			},
			"paths": map[string]any{
				"data_dir":      filepath.ToSlash(work),
				"db_dir":        filepath.ToSlash(filepath.Join(work, "db")),
				"dist_dir":      filepath.ToSlash(filepath.Join(work, "dist")),
				"manifests_dir": filepath.ToSlash(filepath.Join(work, "docs", "manifests")),
				"reports_dir":   filepath.ToSlash(filepath.Join(work, "docs", "reports")),
			},
			"dpd": map[string]any{"url": fileURL(dpdTar)},
			"e2e": map[string]any{"muids": []string{}},
		},
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(tmp, "config.json")
	if err := os.WriteFile(p, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func fileURL(path string) string {
	return "file:///" + strings.TrimPrefix(filepath.ToSlash(path), "/")
}

func runCLI(t *testing.T, cfgPath string, args ...string) (int, string) {
	t.Helper()
	old := stdout
	stdout = new(bytes.Buffer)
	code := Main(append([]string{"--config", cfgPath}, args...))
	out := stdout.(*bytes.Buffer).String()
	stdout = old
	return code, out
}

func TestE2EHermetic(t *testing.T) {
	tmp := t.TempDir()
	buildHermeticBilara(t, filepath.Join(tmp, "bilara-src"))
	buildHermeticEditions(t, filepath.Join(tmp, "editions-src"))
	cfgPath := writeHermeticConfig(t, tmp)

	if code, out := runCLI(t, cfgPath, "sync"); code != 0 {
		t.Fatalf("sync exit %d: %s", code, out)
	}
	if code, out := runCLI(t, cfgPath, "sync", "--source", "dpd"); code != 0 {
		t.Fatalf("sync dpd exit %d: %s", code, out)
	}
	work := filepath.Join(tmp, "work")
	if _, err := os.Stat(filepath.Join(work, "repos/bilara-data/misc/stray.txt")); !os.IsNotExist(err) {
		t.Errorf("stray file must be pruned, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(work, "docs", "manifests", "sync.json")); err != nil {
		t.Fatalf("manifest missing: %v", err)
	}

	if code, out := runCLI(t, cfgPath, "parse"); code != 0 {
		t.Fatalf("parse exit %d: %s", code, out)
	}
	if code, out := runCLI(t, cfgPath, "load"); code != 0 {
		t.Fatalf("load exit %d: %s", code, out)
	}

	code, out := runCLI(t, cfgPath, "triangulate")
	if code != 1 {
		t.Fatalf("triangulate exit %d (want 1, errors expected): %s", code, out)
	}
	for _, want := range []string{"B004/error", "B001/warn", "B002/error", "D001/info"} {
		if !strings.Contains(out, want) {
			t.Errorf("triangulate output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "D002") {
		t.Errorf("D002 must stay silent while the dpd fixture is present:\n%s", out)
	}

	code, out = runCLI(t, cfgPath, "discrepancies")
	if code != 0 {
		t.Fatalf("discrepancies exit %d: %s", code, out)
	}
	if !strings.Contains(out, "B004 error") || !strings.Contains(out, "rows (run ") {
		t.Fatalf("discrepancies output missing the recorded rows:\n%s", out)
	}

	if code, out := runCLI(t, cfgPath, "export"); code != 0 {
		t.Fatalf("export exit %d: %s", code, out)
	}
	if code, out := runCLI(t, cfgPath, "report"); code != 0 {
		t.Fatalf("report exit %d: %s", code, out)
	}

	assertHermeticGraph(t, cfgPath)
	assertHermeticDuckDB(t, cfgPath, work)
	first := readReport(t, work)
	if first.Provenance["dpd_tag"] != "local" {
		t.Errorf("dpd_tag = %q, want local (fixture via file://)", first.Provenance["dpd_tag"])
	}
	if first.Provenance["dpd_sha256"] == "" || first.Provenance["dpd_fetched"] == "" {
		t.Errorf("dpd provenance incomplete: %v", first.Provenance)
	}
	secondRun(t, cfgPath)
	second := readReport(t, work)
	assertSameFindings(t, first, second)

	code, out = runCLI(t, cfgPath, "all")
	if code != 1 {
		t.Fatalf("all exit %d (want 1, the fixture triangulate errors): %s", code, out)
	}
	for _, want := range []string{"B004/error", "B001/warn"} {
		if !strings.Contains(out, want) {
			t.Errorf("all output missing %q:\n%s", want, out)
		}
	}
}

func secondRun(t *testing.T, cfgPath string) {
	t.Helper()
	for _, cmd := range []string{"parse", "load", "triangulate", "export", "report"} {
		code, out := runCLI(t, cfgPath, cmd)
		if cmd == "triangulate" {
			if code != 1 {
				t.Fatalf("second triangulate exit %d: %s", code, out)
			}
			continue
		}
		if code != 0 {
			t.Fatalf("second %s exit %d: %s", cmd, code, out)
		}
	}
}

func assertHermeticGraph(t *testing.T, cfgPath string) {
	t.Helper()
	runID, dbPath, err := graph.CurrentDB(readCfg(t, cfgPath))
	if err != nil {
		t.Fatalf("pointer: %v", err)
	}
	if runID == "" || dbPath == "" {
		t.Fatalf("pointer = %q/%q", runID, dbPath)
	}
	store, err := graph.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	ctx := t.Context()

	pairs, err := store.Segments(ctx, "mn1", true, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(pairs) == 0 {
		t.Fatal("mn1 must have aligned pairs")
	}
	found := false
	for _, p := range pairs {
		if p.SegKey == "1.1" && p.Pali == "Evaṁ me sutaṁ." && p.En == "So I have heard." && p.Author == "sujato" {
			found = true
		}
	}
	if !found {
		t.Errorf("aligned pair 1.1 not found: %+v", pairs)
	}

	for _, typ := range []string{"headword", "dpd_root", "dpd_form"} {
		var n int
		if err := store.DB().QueryRowContext(ctx,
			`SELECT COUNT(*) FROM node WHERE type = ?`, typ).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			t.Errorf("dpd fixture loaded no %s nodes", typ)
		}
	}
	var derived int
	if err := store.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM edge WHERE type = 'DERIVED_FROM'`).Scan(&derived); err != nil {
		t.Fatal(err)
	}
	if derived == 0 {
		t.Error("dpd fixture loaded no DERIVED_FROM edges")
	}
	var hasWord int
	if err := store.DB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM edge e
		JOIN node s ON s.id = e.src AND s.type = 'segment'
		WHERE e.type = 'HAS_WORD' AND s.key = 'pli:mn1:1.1'`).Scan(&hasWord); err != nil {
		t.Fatal(err)
	}
	if hasWord == 0 {
		t.Error("mn1:1.1 (evam me sutam) must carry HAS_WORD edges into the fixture headwords")
	}
	var d001 int
	if err := store.DB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM discrepancy WHERE "check" = 'D001' AND muid = 'mn2'`).Scan(&d001); err != nil {
		t.Fatal(err)
	}
	if d001 != 1 {
		t.Errorf("D001 mn2 findings = %d, want 1 (fixture-unknown tokens)", d001)
	}

	var covered int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM edge WHERE type = 'COVERED_BY'`).Scan(&covered); err != nil {
		t.Fatal(err)
	}
	if covered == 0 {
		t.Error("E102 must have set at least one COVERED_BY edge (mn1 ~ editions)")
	}
	var source string
	if err := store.DB().QueryRowContext(ctx, `
		SELECT e.data FROM edge e
		JOIN node t ON t.id = e.src AND t.type = 'text' AND t.key = 'mn1'
		WHERE e.type = 'COVERED_BY'`).Scan(&source); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(source, `"similarity":1`) || !strings.Contains(source, "html sujato/mn") {
		t.Errorf("mn1 covers data = %s; want the exact html match", source)
	}

	tree, err := store.Tree(ctx, "vinaya/bu-pm", "text", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree) != 1 || tree[0].Key != "pli-tv-bu-pm" {
		t.Errorf("vinaya/bu-pm tree = %+v", tree)
	}

	code, out := runCLI(t, cfgPath, "query", "stats")
	if code != 0 {
		t.Fatalf("query stats exit %d: %s", code, out)
	}
	if !strings.Contains(out, "node:segment:") || !strings.Contains(out, "node:text:") {
		t.Errorf("stats output missing segment/text counts:\n%s", out)
	}
	code, out = runCLI(t, cfgPath, "discrepancies", "--check", "B004")
	if code != 0 {
		t.Fatalf("discrepancies exit %d: %s", code, out)
	}
	if !strings.Contains(out, "dup keys in root/pli/ms/sutta/mn/mn1_root-pli-ms.json") {
		t.Errorf("B004 row missing:\n%s", out)
	}
}

func assertHermeticDuckDB(t *testing.T, cfgPath, work string) {
	t.Helper()
	cfg := readCfg(t, cfgPath)
	_, dbPath, err := graph.CurrentDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	sdb, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sdb.Close() }()
	ddb, err := sql.Open("duckdb", cfg.Canon.DuckDBFile())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ddb.Close() }()

	parity := func(table string) {
		t.Helper()
		var sn, dn int64
		if err := sdb.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&sn); err != nil {
			t.Fatal(err)
		}
		if err := ddb.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&dn); err != nil {
			t.Fatal(err)
		}
		if sn != dn {
			t.Errorf("duckdb %s = %d; sqlite = %d", table, dn, sn)
		}
	}
	for _, table := range []string{"node", "edge", "run", "discrepancy"} {
		parity(table)
	}

	var en, author, ref string
	var html sql.NullString
	err = ddb.QueryRow(`SELECT en, author, html, reference FROM segment_pairs
		WHERE muid = 'mn1' AND seg_key = '2.1'`).Scan(&en, &author, &html, &ref)
	if err != nil {
		t.Fatal(err)
	}
	if en != "Deep indeed." || author != "sujato" || html.Valid || ref != "MN i 3" {
		t.Errorf("segment_pairs mn1:2.1 = %q/%q/%v/%q", en, author, html, ref)
	}
	var pairs int64
	if err := ddb.QueryRow(`SELECT COUNT(*) FROM segment_pairs`).Scan(&pairs); err != nil {
		t.Fatal(err)
	}
	var pali int64
	if err := sdb.QueryRow(`SELECT COUNT(*) FROM node WHERE type='segment' AND key LIKE 'pli:%'`).Scan(&pali); err != nil {
		t.Fatal(err)
	}
	if pairs != pali {
		t.Errorf("segment_pairs = %d; pali segments = %d", pairs, pali)
	}
}

type reportFields struct {
	Counts   map[string]int64 `json:"counts"`
	Coverage []struct {
		Division string `json:"division"`
		Texts    int    `json:"texts"`
		Covered  int    `json:"covered"`
	} `json:"coverage"`
	Discrepancies     []map[string]any  `json:"discrepancies"`
	DiscrepancyCounts map[string]int    `json:"discrepancy_counts"`
	Provenance        map[string]string `json:"provenance"`
}

func readReport(t *testing.T, work string) reportFields {
	t.Helper()
	date := todayUTC()
	raw, err := os.ReadFile(filepath.Join(work, "docs", "reports", date, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	var f reportFields
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

func assertSameFindings(t *testing.T, a, b reportFields) {
	t.Helper()
	if fmt.Sprint(a.Counts) != fmt.Sprint(b.Counts) {
		t.Errorf("counts differ between runs: %v vs %v", a.Counts, b.Counts)
	}
	if fmt.Sprint(a.DiscrepancyCounts) != fmt.Sprint(b.DiscrepancyCounts) {
		t.Errorf("discrepancy counts differ between runs: %v vs %v", a.DiscrepancyCounts, b.DiscrepancyCounts)
	}
	if fmt.Sprint(a.Coverage) != fmt.Sprint(b.Coverage) {
		t.Errorf("coverage differs between runs: %v vs %v", a.Coverage, b.Coverage)
	}
	if fmt.Sprint(a.Discrepancies) != fmt.Sprint(b.Discrepancies) {
		t.Errorf("discrepancy set differs between runs")
	}
}
