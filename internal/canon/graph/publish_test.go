package graph

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lavantien/quotes-manager/internal/config"
)

func tempConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg := config.Defaults()
	cfg.Canon.Paths.DBDir = t.TempDir()
	return cfg
}

func touchDB(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPublishAndCurrentDB(t *testing.T) {
	cfg := tempConfig(t)
	dir := cfg.Canon.Paths.DBDir

	old := touchDB(t, dir, "canon-old.db")
	for _, sidecar := range []string{"-wal", "-shm", "-journal"} {
		if err := os.WriteFile(old+sidecar, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cur := touchDB(t, dir, "canon-run1.db")

	if err := PublishDB(cfg, "run1", cur); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(cfg.Canon.DBPointerFile())
	if err != nil {
		t.Fatal(err)
	}
	var p struct {
		Current string `json:"current"`
		Path    string `json:"path"`
		Updated string `json:"updated"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("pointer is not json: %v", err)
	}
	if p.Current != "run1" || p.Path != cur {
		t.Fatalf("pointer = %+v", p)
	}
	if _, err := time.Parse(time.RFC3339, p.Updated); err != nil {
		t.Fatalf("updated %q is not RFC3339: %v", p.Updated, err)
	}

	runID, path, err := CurrentDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if runID != "run1" || path != cur {
		t.Fatalf("CurrentDB = %q,%q", runID, path)
	}

	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("old db still present: %v", err)
	}
	for _, sidecar := range []string{"-wal", "-shm", "-journal"} {
		if _, err := os.Stat(old + sidecar); !os.IsNotExist(err) {
			t.Fatalf("old db %s sidecar still present", sidecar)
		}
	}
	if _, err := os.Stat(cur); err != nil {
		t.Fatalf("current db removed: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" || len(e.Name()) > 0 && e.Name()[0] == '.' && e.Name() != ".gitkeep" {
			t.Fatalf("temp leftover %q", e.Name())
		}
	}
}

func TestPublishCreatesDataDir(t *testing.T) {
	cfg := tempConfig(t)
	cfg.Canon.Paths.DBDir = filepath.Join(cfg.Canon.Paths.DBDir, "nested", "deeper")
	if err := os.MkdirAll(filepath.Dir(cfg.Canon.Paths.DBDir), 0o755); err != nil {
		t.Fatal(err)
	}
	db := touchDB(t, filepath.Dir(cfg.Canon.Paths.DBDir), "canon-x.db")
	if err := PublishDB(cfg, "x", db); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cfg.Canon.DBPointerFile()); err != nil {
		t.Fatalf("pointer missing: %v", err)
	}
}

func TestPublishOverwritesPointer(t *testing.T) {
	cfg := tempConfig(t)
	dir := cfg.Canon.Paths.DBDir
	first := touchDB(t, dir, "canon-a.db")
	second := touchDB(t, dir, "canon-b.db")
	if err := PublishDB(cfg, "a", first); err != nil {
		t.Fatal(err)
	}
	if err := PublishDB(cfg, "b", second); err != nil {
		t.Fatal(err)
	}
	runID, path, err := CurrentDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if runID != "b" || path != second {
		t.Fatalf("CurrentDB = %q,%q, want b", runID, path)
	}
	if _, err := os.Stat(first); !os.IsNotExist(err) {
		t.Fatal("superseded db must be removed")
	}
}

func TestCurrentDBMissingPointer(t *testing.T) {
	cfg := tempConfig(t)
	if _, _, err := CurrentDB(cfg); err == nil {
		t.Fatal("missing pointer file must error")
	}
}

func TestPublishRejectsInvalidUTF8(t *testing.T) {
	cfg := tempConfig(t)
	db := touchDB(t, cfg.Canon.Paths.DBDir, "canon-u.db")
	if err := PublishDB(cfg, "\xc3", db); err == nil {
		t.Fatal("invalid UTF-8 run id must be rejected, not corrupted")
	}
	if err := PublishDB(cfg, "ok", db+"\xff"); err == nil {
		t.Fatal("invalid UTF-8 path must be rejected, not corrupted")
	}
	if _, err := os.Stat(cfg.Canon.DBPointerFile()); !os.IsNotExist(err) {
		t.Fatal("rejected publishes must not write the pointer")
	}
}

func TestCurrentDBMalformedPointer(t *testing.T) {
	cfg := tempConfig(t)
	if err := os.WriteFile(cfg.Canon.DBPointerFile(), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := CurrentDB(cfg); err == nil {
		t.Fatal("malformed pointer file must error")
	}
}
