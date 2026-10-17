package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/lavantien/quotes-manager/internal/canon/graph"
	"github.com/lavantien/quotes-manager/internal/config"
)

const liveEnvVar = "QM_E2E_LIVE"

func writeLiveConfig(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
	root := filepath.Join(repoRoot, "data", "e2e-live")
	cfg := config.Defaults()
	cfg.Canon.E2E.Muids = []string{"mn1", "mn2", "pli-tv-bu-pm"}
	cfg.Canon.Dpd.URL = fileURL(filepath.Join(repoRoot, "internal", "canon", "testdata", "dpd", "dpd.db.tar.xz"))
	cfg.Canon.Paths.DataDir = filepath.ToSlash(root)
	cfg.Canon.Paths.DBDir = filepath.ToSlash(filepath.Join(root, "db"))
	cfg.Canon.Paths.DistDir = filepath.ToSlash(filepath.Join(root, "dist"))
	cfg.Canon.Paths.ManifestsDir = filepath.ToSlash(filepath.Join(root, "docs", "manifests"))
	cfg.Canon.Paths.ReportsDir = filepath.ToSlash(filepath.Join(root, "docs", "reports"))
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(root, "config.json")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func liveEnabled() bool {
	return os.Getenv(liveEnvVar) == "1"
}

func TestLiveEnabled(t *testing.T) {
	old, had := os.LookupEnv(liveEnvVar)
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(liveEnvVar, old)
			return
		}
		_ = os.Unsetenv(liveEnvVar)
	})
	for _, tc := range []struct {
		value  string
		set    bool
		expect bool
	}{
		{"", false, false},
		{"0", true, false},
		{"1", true, true},
		{"yes", true, false},
	} {
		if tc.set {
			_ = os.Setenv(liveEnvVar, tc.value)
		} else {
			_ = os.Unsetenv(liveEnvVar)
		}
		if got := liveEnabled(); got != tc.expect {
			t.Errorf("%s=%q: liveEnabled() = %v, want %v", liveEnvVar, tc.value, got, tc.expect)
		}
	}
}

func TestE2ELive(t *testing.T) {
	if !liveEnabled() {
		t.Skipf("live e2e requires %s=1", liveEnvVar)
	}
	cfgPath := writeLiveConfig(t)
	cfg := readCfg(t, cfgPath)

	old := stdout
	buf := new(bytes.Buffer)
	stdout = buf
	syncErr := runSync(t.Context(), cfg, nil)
	stdout = old
	if testing.Verbose() {
		_, _ = buf.WriteTo(os.Stderr)
	}
	if syncErr != nil {
		t.Skipf("live e2e: network unavailable, skipping: %v", syncErr)
	}
	if err := runSync(t.Context(), cfg, []string{"--source", "dpd"}); err != nil {
		t.Fatalf("live sync dpd: %v", err)
	}

	for _, step := range []struct {
		name string
		run  func() error
		fail bool
	}{
		{"parse", func() error { return runParse(t.Context(), cfg, nil) }, true},
		{"load", func() error { return runLoad(t.Context(), cfg, nil) }, true},
		{"triangulate", func() error { return runTriangulate(t.Context(), cfg, nil) }, false},
		{"export", func() error { return runExport(t.Context(), cfg, nil) }, true},
		{"report", func() error { return runReport(t.Context(), cfg, nil) }, true},
	} {
		if err := step.run(); err != nil && step.fail {
			t.Fatalf("live %s: %v", step.name, err)
		}
	}

	_, dbPath, err := graph.CurrentDB(cfg)
	if err != nil {
		t.Fatalf("live pointer: %v", err)
	}
	store, err := graph.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	ctx := t.Context()

	pairs, err := store.Segments(ctx, "mn1", true, 5, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(pairs) == 0 {
		t.Fatal("live mn1 must have aligned pairs")
	}
	if pairs[0].Pali == "" || pairs[0].En == "" {
		t.Errorf("live mn1 pair incomplete: %+v", pairs[0])
	}

	var covered int
	if err := store.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM edge WHERE type = 'COVERED_BY'`).Scan(&covered); err != nil {
		t.Fatal(err)
	}
	if covered == 0 {
		t.Error("live run must record at least one COVERED_BY edge (mn1 editions coverage)")
	}

	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stats["node:segment"] == 0 {
		t.Error("live run loaded no segments")
	}
	if stats["node:headword"] == 0 {
		t.Error("live run loaded no dpd headwords (fixture tarball)")
	}
	var hasWord int64
	if err := store.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM edge WHERE type = 'HAS_WORD'`).Scan(&hasWord); err != nil {
		t.Fatal(err)
	}
	if hasWord == 0 {
		t.Error("live run linked no HAS_WORD edges within the pinned slice")
	}
	if Verbose(t.Context()) || testing.Verbose() {
		fmt.Fprintf(os.Stderr, "live stats: %v\n", stats)
	}

	code, out := runCLI(t, cfgPath, "query", "word", "--form", "evam")
	if code != 0 {
		t.Fatalf("query word --form evam exit %d: %s", code, out)
	}
	var entries []struct {
		Lemma1 string `json:"lemma_1"`
	}
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("query word output not json: %v: %s", err, out)
	}
	if len(entries) == 0 || entries[0].Lemma1 == "" {
		t.Errorf("query word --form evam resolved nothing: %s", out)
	}

	tree, err := store.Tree(ctx, "vinaya/bu-pm", "text", 5, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree) == 0 || tree[0].Key != "pli-tv-bu-pm" {
		t.Errorf("live vinaya/bu-pm tree = %+v", tree)
	}
}
