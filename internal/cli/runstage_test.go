package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/lavantien/quotes-manager/internal/config"
)

func TestStagesList(t *testing.T) {
	want := []string{"sync", "prune", "parse", "load", "triangulate", "export", "report", "all"}
	got := Stages()
	if len(got) != len(want) {
		t.Fatalf("stages = %v, want %v", got, want)
	}
	for i, s := range want {
		if got[i] != s {
			t.Fatalf("stages[%d] = %q, want %q (pipeline order)", i, got[i], want[i])
		}
	}
}

func TestRunStageRejectsNonStage(t *testing.T) {
	c := config.Defaults()
	for _, name := range []string{"", "version", "query", "discrepancies", "serve", "syncx"} {
		if err := RunStage(context.Background(), c, name, nil, &bytes.Buffer{}); err == nil {
			t.Fatalf("RunStage(%q) must reject non-stage commands", name)
		}
	}
}

func TestRunStagePassesArgsThrough(t *testing.T) {
	c := config.Defaults()
	err := RunStage(context.Background(), c, "prune", []string{"--nope"}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("RunStage with unknown flag = %v, want error naming the flag", err)
	}
}

func TestRunStageArglessStagesRejectArgs(t *testing.T) {
	c := config.Defaults()
	for _, name := range []string{"parse", "load", "triangulate", "export", "all"} {
		err := RunStage(context.Background(), c, name, []string{"--garbage"}, &bytes.Buffer{})
		if err == nil || !strings.Contains(err.Error(), "takes no arguments") {
			t.Fatalf("RunStage(%q, --garbage) = %v, want a takes-no-arguments error", name, err)
		}
	}
}

func TestRunStageFlagStagesRejectPositionalArgs(t *testing.T) {
	tmp := t.TempDir()
	buildHermeticBilara(t, filepath.Join(tmp, "bilara-src"))
	buildHermeticEditions(t, filepath.Join(tmp, "editions-src"))
	c, err := config.Load(writeHermeticConfig(t, tmp))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"sync", "prune", "report"} {
		err := RunStage(context.Background(), c, name, []string{"stray"}, &bytes.Buffer{})
		if err == nil || !strings.Contains(err.Error(), "positional") {
			t.Fatalf("RunStage(%q, stray) = %v, want a positional-args error", name, err)
		}
	}
}

func TestRunStageCapturesVerboseOutput(t *testing.T) {
	tmp := t.TempDir()
	buildHermeticBilara(t, filepath.Join(tmp, "bilara-src"))
	buildHermeticEditions(t, filepath.Join(tmp, "editions-src"))
	cfgPath := writeHermeticConfig(t, tmp)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	var syncOut bytes.Buffer
	if err := RunStage(ctx, cfg, "sync", nil, &syncOut); err != nil {
		t.Fatalf("sync stage: %v", err)
	}
	if !bytes.Contains(syncOut.Bytes(), []byte("manifest:")) {
		t.Fatalf("sync output not captured (verbose must be forced):\n%s", syncOut.String())
	}

	var pruneOut bytes.Buffer
	if err := RunStage(ctx, cfg, "prune", nil, &pruneOut); err != nil {
		t.Fatalf("prune stage: %v", err)
	}
	if pruneOut.Len() == 0 {
		t.Fatal("prune output not captured")
	}
}

func TestRunStageRestoresAndSerializesStdout(t *testing.T) {
	tmp := t.TempDir()
	buildHermeticBilara(t, filepath.Join(tmp, "bilara-src"))
	buildHermeticEditions(t, filepath.Join(tmp, "editions-src"))
	cfgPath := writeHermeticConfig(t, tmp)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := RunStage(ctx, cfg, "sync", nil, &bytes.Buffer{}); err != nil {
		t.Fatalf("sync stage: %v", err)
	}

	before := stdout
	var wg sync.WaitGroup
	for i := range 4 {
		wg.Go(func() {
			var buf bytes.Buffer
			if err := RunStage(ctx, cfg, "prune", nil, &buf); err != nil {
				t.Errorf("concurrent prune %d: %v", i, err)
			}
			if buf.Len() == 0 {
				t.Errorf("concurrent prune %d captured nothing", i)
			}
		})
	}
	wg.Wait()
	if stdout != before {
		t.Fatal("stdout writer must be restored after RunStage")
	}
}
