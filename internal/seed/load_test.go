package seed

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func quoteCount(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM quotes").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func assertSortOrderMatchesID(t *testing.T, db *sql.DB) {
	t.Helper()
	var bad int
	if err := db.QueryRow("SELECT COUNT(*) FROM quotes WHERE sort_order <> id OR sort_order = 0").Scan(&bad); err != nil {
		t.Fatal(err)
	}
	if bad != 0 {
		t.Errorf("%d rows have sort_order != id or unset", bad)
	}
}

// TestEnsureSeededFreshDB loads the canonical seed into an empty database.
func TestEnsureSeededFreshDB(t *testing.T) {
	db := openDB(t)
	if err := EnsureSeeded(db); err != nil {
		t.Fatal(err)
	}
	if n := quoteCount(t, db); n == 0 {
		t.Error("fresh DB has no seeded quotes")
	}
	assertSortOrderMatchesID(t, db)
}

// TestEnsureSeededOnEmptyPrecreatedTable mirrors the server flow: store.Open
// creates an empty table (with sort_order) before EnsureSeeded runs. EnsureSeeded
// must still recognize this as fresh and load the seed.
func TestEnsureSeededOnEmptyPrecreatedTable(t *testing.T) {
	db := openDB(t)
	if _, err := db.Exec(`CREATE TABLE quotes (
		id INTEGER PRIMARY KEY, sutta_id TEXT, citation TEXT, body_md TEXT,
		body_text TEXT, line_count INTEGER, char_count INTEGER, sources TEXT,
		sort_order INTEGER NOT NULL DEFAULT 0)`); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSeeded(db); err != nil {
		t.Fatal(err)
	}
	if n := quoteCount(t, db); n == 0 {
		t.Error("empty pre-created table was not seeded")
	}
}

// TestEnsureSeededLegacyPreservesData: a database created by the old seed.sql
// (109 rows, no sort_order) must keep its rows and gain sort_order = id, without
// being dropped.
func TestEnsureSeededLegacyPreservesData(t *testing.T) {
	db := openDB(t)
	if _, err := db.Exec(`CREATE TABLE quotes (
		id INTEGER PRIMARY KEY, sutta_id TEXT, citation TEXT, body_md TEXT,
		body_text TEXT, line_count INTEGER, char_count INTEGER, sources TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO quotes (id, sutta_id, citation, body_md, body_text, line_count, char_count, sources)
		VALUES (5, 'X', 'X', 'x', 'x', 1, 1, 's')`); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSeeded(db); err != nil {
		t.Fatal(err)
	}
	assertSortOrderMatchesID(t, db)
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM quotes WHERE id = 5 AND sort_order = 5 AND sutta_id = 'X'").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("legacy row not preserved/migrated: n=%d", n)
	}
}

func TestEnsureSeededIdempotent(t *testing.T) {
	db := openDB(t)
	if err := EnsureSeeded(db); err != nil {
		t.Fatal(err)
	}
	n := quoteCount(t, db)
	if err := EnsureSeeded(db); err != nil {
		t.Fatal(err)
	}
	if got := quoteCount(t, db); got != n {
		t.Errorf("second EnsureSeeded changed count %d -> %d", n, got)
	}
}

// TestEnsureSeededDoesNotReseedAfterDelete: once seeded, deleting everything
// must not cause a later EnsureSeeded to resurrect the canonical quotes.
func TestEnsureSeededDoesNotReseedAfterDelete(t *testing.T) {
	db := openDB(t)
	if err := EnsureSeeded(db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("DELETE FROM quotes"); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSeeded(db); err != nil {
		t.Fatal(err)
	}
	if n := quoteCount(t, db); n != 0 {
		t.Errorf("EnsureSeeded resurrected %d deleted quotes", n)
	}
}
