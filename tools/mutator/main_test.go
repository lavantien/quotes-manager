package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const sampleSrc = `package sample

func Less(a, b int) bool { return a < b }
func Eq(a, b int) bool   { return a == b }
func Both(a, b bool) bool { return a && b }
`

func writeSample(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "sample")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "s.go"), []byte(sampleSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestDiscoverFindsOperators(t *testing.T) {
	dir := writeSample(t)
	ms, err := discover([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	ops := map[string]bool{}
	for _, m := range ms {
		ops[m.oldOp+"->"+m.newOp] = true
	}
	for _, want := range []string{"<->>=", "==->!=", "&&->||"} {
		if !ops[want] {
			t.Fatalf("missing flip %s in %v", want, ops)
		}
	}
}

func TestReplaceAt(t *testing.T) {
	b := []byte("a == b")
	got := replaceAt(b, 2, "==", "!=")
	if string(got) != "a != b" {
		t.Fatalf("replace failed: %q", got)
	}
	if replaceAt(b, 2, "!=", "==") != nil {
		t.Fatal("mismatched old op must be rejected")
	}
	if replaceAt(b, 100, "==", "!=") != nil {
		t.Fatal("out of range must be rejected")
	}
}

func TestPkgFor(t *testing.T) {
	if got := pkgFor("internal/store/store.go"); got != "github.com/lavantien/quotes-manager/internal/store" {
		t.Fatalf("pkgFor = %q", got)
	}
}

func TestClampParallel(t *testing.T) {
	cases := []struct {
		name        string
		requested   int
		mutantCount int
		procs       int
		want        int
	}{
		{"zero uses procs", 0, 100, 8, 8},
		{"negative uses procs", -3, 100, 4, 4},
		{"capped at slots", 32, 100, 32, 8},
		{"procs capped too", 0, 100, 64, 8},
		{"floored at one", 0, 100, 0, 1},
		{"trimmed to mutants", 8, 3, 8, 3},
		{"explicit under cap", 4, 100, 8, 4},
	}
	for _, c := range cases {
		if got := clampParallel(c.requested, c.mutantCount, c.procs); got != c.want {
			t.Errorf("%s: clampParallel(%d, %d, %d) = %d, want %d",
				c.name, c.requested, c.mutantCount, c.procs, got, c.want)
		}
	}
}

func TestCapSelfMemory(t *testing.T) {
	if err := capSelfMemory(); err != nil {
		t.Fatalf("capSelfMemory: %v", err)
	}
	if err := capSelfMemory(); err != nil {
		t.Fatalf("capSelfMemory second call: %v", err)
	}
}

func writeHangModule(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":            "module hang\n",
		"hang/hang.go":      "package hang\n",
		"hang/hang_test.go": "package hang\n\nimport \"testing\"\n\nfunc TestHang(t *testing.T) {\n\tfor {\n\t}\n}\n",
	}
	for name, src := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestTestPkgTimesOutHungTests(t *testing.T) {
	dir := writeHangModule(t)
	start := time.Now()
	if got := testPkgTimeout(dir, "./hang", "", 2*time.Second); got != outcomeKilled {
		t.Fatalf("hung test outcome = %v, want killed", got)
	}
	if elapsed := time.Since(start); elapsed > 60*time.Second {
		t.Fatalf("hung run took %v to die, want the ~2s timeout plus compile", elapsed)
	}
}
