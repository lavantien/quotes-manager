package graph

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lavantien/quotes-manager/internal/canon/model"
)

func buildPublishedStore(t *testing.T, path string) {
	t.Helper()
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
	l := newTestLoader(t, s, 100)
	if _, err := l.EnsureNode(model.NodeText, "mn1", `{"name":"root of all things"}`); err != nil {
		t.Fatal(err)
	}
	if err := l.Flush(); err != nil {
		t.Fatal(err)
	}
	if err := s.Checkpoint(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestOpenROReadsAndRejectsWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pub.db")
	buildPublishedStore(t, path)

	s, err := OpenRO(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	var n int
	if err := s.DB().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM node WHERE key = 'mn1'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("read through ro store = %d nodes, want 1", n)
	}
	if _, err := s.DB().ExecContext(context.Background(),
		`INSERT INTO node (type, key, data) VALUES ('text', 'x', '{}')`); err == nil {
		t.Fatal("write through ro store must fail")
	}
}

func TestOpenRORejectsQuestionMarkPath(t *testing.T) {
	if _, err := OpenRO(filepath.Join("data", "a?b.db")); err == nil {
		t.Fatal("path containing ? must be rejected")
	}
}

func TestCheckpointLeavesSelfContainedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pub.db")
	buildPublishedStore(t, path)

	if wal, err := os.Stat(path + "-wal"); err == nil && wal.Size() > 0 {
		t.Fatalf("-wal still holds %d bytes after checkpoint; ro open would need it", wal.Size())
	}

	s, err := OpenRO(path)
	if err != nil {
		t.Fatalf("ro open on checkpointed publish: %v", err)
	}
	defer func() { _ = s.Close() }()
	var n int
	if err := s.DB().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM node`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("checkpointed store holds %d nodes, want 1", n)
	}
}

func TestPublishDBRetirementBoundedUnderHeldHandle(t *testing.T) {
	cfg := tempConfig(t)
	dir := cfg.Canon.Paths.DBDir

	old := touchDB(t, dir, "canon-old.db")
	if err := PublishDB(cfg, "old", old); err != nil {
		t.Fatal(err)
	}
	cur := touchDB(t, dir, "canon-run2.db")

	held, err := sql.Open("sqlite3", old)
	if err != nil {
		t.Fatal(err)
	}
	if err := held.Ping(); err != nil {
		t.Fatal(err)
	}

	err = PublishDB(cfg, "run2", cur)
	if err != nil && !errors.Is(err, ErrRetirementLag) {
		t.Fatalf("held-handle publish error must be the post-commit retirement lag: %v", err)
	}

	_, got, err := CurrentDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got != cur {
		t.Fatalf("pointer must advance to %q even when retirement lags, got %q", cur, got)
	}
	if err := held.Close(); err != nil {
		t.Fatal(err)
	}
}
