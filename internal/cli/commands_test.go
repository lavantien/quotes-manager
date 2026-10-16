package cli

import (
	"sort"
	"testing"
)

func TestPipelineCommandsRegistered(t *testing.T) {
	want := []string{
		"all", "discrepancies", "export", "load", "parse", "prune",
		"query", "report", "serve", "sync", "triangulate", "version",
	}
	got := Names()
	sort.Strings(got)
	if len(got) != len(want) {
		t.Fatalf("registered commands = %v; want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("registered commands = %v; want %v", got, want)
		}
	}
}

func TestMirrorSourceRejectsUnknown(t *testing.T) {
	if _, err := mirrorSource("nope"); err == nil {
		t.Error("unknown --source must error")
	}
	for _, ok := range []string{"bilara", "editions", "dpd", "all"} {
		if _, err := mirrorSource(ok); err != nil {
			t.Errorf("mirrorSource(%q): %v", ok, err)
		}
	}
}
