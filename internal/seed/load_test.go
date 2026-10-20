package seed

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/lavantien/quotes-manager/internal/store"

	_ "github.com/mattn/go-sqlite3"
)

func openStoreDB(t *testing.T) *sql.DB {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st.DB()
}

func countRows(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestEnsureSeededFreshDB(t *testing.T) {
	db := openStoreDB(t)
	if err := EnsureSeeded(db); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, db, "quotes"); n == 0 {
		t.Error("fresh DB has no seeded quotes")
	}
}

func TestEnsureSeededOnEmptyPrecreatedTables(t *testing.T) {
	db := openStoreDB(t)
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM quotes").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("store.Open must provision empty tables, has %d quotes", n)
	}
	if err := EnsureSeeded(db); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, db, "quotes"); n == 0 {
		t.Error("empty pre-created tables were not seeded")
	}
}

func TestEnsureSeededLegacyPreservesData(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)
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
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM quotes WHERE id = 5 AND sutta_id = 'X'").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("legacy row not preserved: n=%d", n)
	}
}

func TestEnsureSeededIdempotent(t *testing.T) {
	db := openStoreDB(t)
	if err := EnsureSeeded(db); err != nil {
		t.Fatal(err)
	}
	n := countRows(t, db, "quotes")
	if err := EnsureSeeded(db); err != nil {
		t.Fatal(err)
	}
	if got := countRows(t, db, "quotes"); got != n {
		t.Errorf("second EnsureSeeded changed count %d -> %d", n, got)
	}
}

func TestEnsureSeededDoesNotReseedAfterDelete(t *testing.T) {
	db := openStoreDB(t)
	if err := EnsureSeeded(db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("DELETE FROM quotes"); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSeeded(db); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, db, "quotes"); n != 0 {
		t.Errorf("EnsureSeeded resurrected %d deleted quotes", n)
	}
}

func TestEnsureSeededSeedsSampleCategories(t *testing.T) {
	db := openStoreDB(t)
	if err := EnsureSeeded(db); err != nil {
		t.Fatal(err)
	}
	if cats := countRows(t, db, "categories"); cats == 0 {
		t.Error("expected sample categories")
	}
	if items := countRows(t, db, "category_items"); items == 0 {
		t.Error("expected sample category tags")
	}
}

func TestEnsureSeededSeedsSuttaCollection(t *testing.T) {
	db := openStoreDB(t)
	if err := EnsureSeeded(db); err != nil {
		t.Fatal(err)
	}
	var name string
	var cols int
	if err := db.QueryRow("SELECT COUNT(*), MAX(name) FROM collections GROUP BY name").Scan(&cols, &name); err != nil {
		t.Fatal(err)
	}
	if cols != 1 || name != "AriyaSavaka" {
		t.Errorf("collections = %d rows, name %q, want exactly one AriyaSavaka", cols, name)
	}
	for table, want := range map[string]int{
		"suttas":                 471,
		"collection_groups":      16,
		"collection_sutta_items": 471,
	} {
		if got := countRows(t, db, table); got != want {
			t.Errorf("%s = %d rows, want %d", table, got, want)
		}
	}
	var grouped int
	if err := db.QueryRow("SELECT COUNT(*) FROM collection_sutta_items WHERE group_id IS NOT NULL").Scan(&grouped); err != nil {
		t.Fatal(err)
	}
	if grouped != 471 {
		t.Errorf("grouped membership rows = %d, want 471", grouped)
	}
}

func TestEnsureSeededClosedDB(t *testing.T) {
	db := openStoreDB(t)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSeeded(db); err == nil {
		t.Error("EnsureSeeded on a closed DB should error")
	}
}

func TestTableExistsError(t *testing.T) {
	db := openStoreDB(t)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := tableExists(db, "quotes"); err == nil {
		t.Error("tableExists on a closed DB should error")
	}
}

func TestMetaValueMissingTable(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := metaValue(db, "seeded"); err == nil {
		t.Error("metaValue on a missing app_meta table should error")
	}
}

func TestSetMetaMissingTable(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := setMeta(db, "seeded", "1"); err == nil {
		t.Error("setMeta on a missing app_meta table should error")
	}
}

func TestSeedCategoriesMalformedTable(t *testing.T) {
	db := openStoreDB(t)
	if _, err := db.Exec("DROP TABLE categories; CREATE TABLE categories (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	if err := seedCategories(db); err == nil {
		t.Error("seedCategories should error when the name column is missing")
	}
}
