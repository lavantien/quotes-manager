package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/lavantien/quotes-manager/internal/seed"
	"github.com/lavantien/quotes-manager/internal/store"
)

// TestRunRoundTripsAllTables seeds a source DB, dumps it, loads the dump into a
// fresh clone, and asserts every table has identical row counts. Loading the
// dump also proves the SQL escaping is correct (the seed carries curly quotes
// and apostrophes).
func TestRunRoundTripsAllTables(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "main.db")
	outPath := filepath.Join(dir, "fixture.sql")
	clonePath := filepath.Join(dir, "clone.db")

	src, err := store.Open(srcPath)
	if err != nil {
		t.Fatalf("open source: %v", err)
	}
	if err := seed.EnsureSeeded(src.DB()); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := src.Close(); err != nil {
		t.Fatalf("close source: %v", err)
	}

	if err := run(srcPath, outPath); err != nil {
		t.Fatalf("run: %v", err)
	}
	dump, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if len(dump) == 0 {
		t.Fatal("fixture is empty")
	}

	clone, err := store.Open(clonePath)
	if err != nil {
		t.Fatalf("open clone: %v", err)
	}
	defer clone.Close()
	if _, err := clone.DB().Exec(string(dump)); err != nil {
		t.Fatalf("load dump into clone: %v", err)
	}

	ref, err := store.Open(srcPath)
	if err != nil {
		t.Fatalf("reopen source: %v", err)
	}
	defer ref.Close()

	for _, tb := range tables {
		want := count(t, ref.DB(), tb.name)
		got := count(t, clone.DB(), tb.name)
		if got != want {
			t.Errorf("table %s: clone has %d rows, source has %d", tb.name, got, want)
		}
	}
}

func count(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}
