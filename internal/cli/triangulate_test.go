package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lavantien/quotes-manager/internal/config"
)

func TestTriangulateEnvDegrades(t *testing.T) {
	cfg := config.Defaults()
	cfg.Canon.Paths.DataDir = filepath.ToSlash(t.TempDir())

	env, err := triangulateEnv(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if d, ok := env.BilaraNewerThan("sujato"); ok || d != "" {
		t.Fatalf("BilaraNewerThan without a bilara mirror = (%q, %v), want empty false", d, ok)
	}
	if env.LastRunDate != "" {
		t.Fatalf("LastRunDate without an editions mirror = %q, want empty", env.LastRunDate)
	}
	if len(env.EditionsHTML) != 0 {
		t.Fatalf("EditionsHTML without an editions mirror = %d entries, want none", len(env.EditionsHTML))
	}

	if err := os.MkdirAll(cfg.Canon.EditionsDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.Canon.EditionsDir(), "last_run_date"),
		[]byte("2026-01-02"), 0o644); err != nil {
		t.Fatal(err)
	}
	env, err = triangulateEnv(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if env.LastRunDate != "2026-01-02" {
		t.Fatalf("LastRunDate = %q, want the editions last_run_date value", env.LastRunDate)
	}
	if len(env.EditionsHTML) != 0 {
		t.Fatalf("EditionsHTML with no html files = %d entries, want none", len(env.EditionsHTML))
	}
}
