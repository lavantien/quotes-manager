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

// TestEnsureSeededFreshDB loads the canonical seed into an empty database.
func TestEnsureSeededFreshDB(t *testing.T) {
	db := openDB(t)
	if err := EnsureSeeded(db); err != nil {
		t.Fatal(err)
	}
	if n := quoteCount(t, db); n == 0 {
		t.Error("fresh DB has no seeded quotes")
	}
}

// TestEnsureSeededOnEmptyPrecreatedTable mirrors the server flow: store.Open
// creates an empty table before EnsureSeeded runs. EnsureSeeded must still
// recognize this as fresh and load the seed.
func TestEnsureSeededOnEmptyPrecreatedTable(t *testing.T) {
	db := openDB(t)
	if _, err := db.Exec(`CREATE TABLE quotes (
		id INTEGER PRIMARY KEY, sutta_id TEXT, citation TEXT, body_md TEXT,
		body_text TEXT, line_count INTEGER, char_count INTEGER, sources TEXT)`); err != nil {
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
// (rows already present, no app_meta marker) must keep its rows untouched,
// without being dropped or re-seeded.
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
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM quotes WHERE id = 5 AND sutta_id = 'X'").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("legacy row not preserved: n=%d", n)
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

// TestEnsureSeededSeedsSampleCategories: on a fresh database where the
// categories tables already exist (as store.Open provisions them before
// seeding), EnsureSeeded tags a few quotes with example categories so the
// sidebar and chip rows are non-empty out of the box.
func TestEnsureSeededSeedsSampleCategories(t *testing.T) {
	db := openDB(t)
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS categories (id INTEGER PRIMARY KEY, name TEXT NOT NULL UNIQUE COLLATE NOCASE);
		CREATE TABLE IF NOT EXISTS category_items (category_id INTEGER NOT NULL, quote_id INTEGER NOT NULL, PRIMARY KEY (category_id, quote_id))`); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSeeded(db); err != nil {
		t.Fatal(err)
	}
	var cats, items int
	if err := db.QueryRow("SELECT COUNT(*) FROM categories").Scan(&cats); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM category_items").Scan(&items); err != nil {
		t.Fatal(err)
	}
	if cats == 0 || items == 0 {
		t.Errorf("expected sample categories/tags, got cats=%d items=%d", cats, items)
	}
}

// TestEnsureSeededSeedsSampleCollection: on a fresh database where the
// collections tables already exist (as store.Open provisions them before
// seeding), EnsureSeeded creates one sample collection holding the two shortest
// quotes, so the collection column and membership chips are non-empty on a fresh
// install.
func TestEnsureSeededSeedsSampleCollection(t *testing.T) {
	db := openDB(t)
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS collections (id INTEGER PRIMARY KEY, name TEXT NOT NULL DEFAULT '');
		CREATE TABLE IF NOT EXISTS collection_items (collection_id INTEGER NOT NULL, quote_id INTEGER NOT NULL, position INTEGER NOT NULL, PRIMARY KEY (collection_id, quote_id))`); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSeeded(db); err != nil {
		t.Fatal(err)
	}
	var cols, items int
	if err := db.QueryRow("SELECT COUNT(*) FROM collections").Scan(&cols); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM collection_items").Scan(&items); err != nil {
		t.Fatal(err)
	}
	if cols != 1 || items != 2 {
		t.Errorf("expected 1 sample collection with 2 items, got cols=%d items=%d", cols, items)
	}
}

// TestSeedSampleCollectionIdempotent: re-entering the seeding path must not
// create a second sample collection.
func TestSeedSampleCollectionIdempotent(t *testing.T) {
	db := openDB(t)
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS collections (id INTEGER PRIMARY KEY, name TEXT NOT NULL DEFAULT '');
		CREATE TABLE IF NOT EXISTS collection_items (collection_id INTEGER NOT NULL, quote_id INTEGER NOT NULL, position INTEGER NOT NULL, PRIMARY KEY (collection_id, quote_id))`); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSeeded(db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("DELETE FROM app_meta WHERE key = 'seeded'"); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSeeded(db); err != nil {
		t.Fatal(err)
	}
	var cols int
	if err := db.QueryRow("SELECT COUNT(*) FROM collections").Scan(&cols); err != nil {
		t.Fatal(err)
	}
	if cols != 1 {
		t.Errorf("re-seed duplicated the sample collection: cols=%d", cols)
	}
}

// TestSeedCategoriesIdempotent: re-entering the seeding path (e.g. recovery
// from a partial failure) must not duplicate category tags.
func TestSeedCategoriesIdempotent(t *testing.T) {
	db := openDB(t)
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS categories (id INTEGER PRIMARY KEY, name TEXT NOT NULL UNIQUE COLLATE NOCASE);
		CREATE TABLE IF NOT EXISTS category_items (category_id INTEGER NOT NULL, quote_id INTEGER NOT NULL, PRIMARY KEY (category_id, quote_id))`); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSeeded(db); err != nil {
		t.Fatal(err)
	}
	var first int
	if err := db.QueryRow("SELECT COUNT(*) FROM category_items").Scan(&first); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("DELETE FROM app_meta WHERE key = 'seeded'"); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSeeded(db); err != nil {
		t.Fatal(err)
	}
	var second int
	if err := db.QueryRow("SELECT COUNT(*) FROM category_items").Scan(&second); err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Errorf("re-seed changed category_items %d -> %d", first, second)
	}
}

// TestEnsureSeededClosedDB: a closed database surfaces the first Exec error
// rather than hanging or panicking.
func TestEnsureSeededClosedDB(t *testing.T) {
	db := openDB(t)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSeeded(db); err == nil {
		t.Error("EnsureSeeded on a closed DB should error")
	}
}

// TestTableExistsError drives the helper's error path via a closed connection.
func TestTableExistsError(t *testing.T) {
	db := openDB(t)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := tableExists(db, "quotes"); err == nil {
		t.Error("tableExists on a closed DB should error")
	}
}

// TestMetaValueMissingTable: querying a key before the app_meta table exists
// yields a non-ErrNoRows error (surfaced, not swallowed as empty).
func TestMetaValueMissingTable(t *testing.T) {
	db := openDB(t)
	if _, err := metaValue(db, "seeded"); err == nil {
		t.Error("metaValue on a missing app_meta table should error")
	}
}

// TestSetMetaMissingTable: the upsert fails when app_meta does not exist yet.
func TestSetMetaMissingTable(t *testing.T) {
	db := openDB(t)
	if err := setMeta(db, "seeded", "1"); err == nil {
		t.Error("setMeta on a missing app_meta table should error")
	}
}

// TestSeedCategoriesMalformedTable: a categories table lacking the name column
// makes seedCategories' INSERT fail.
func TestSeedCategoriesMalformedTable(t *testing.T) {
	db := openDB(t)
	if _, err := db.Exec("CREATE TABLE categories (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	if err := seedCategories(db); err == nil {
		t.Error("seedCategories should error when the name column is missing")
	}
}

// TestSeedSampleCollectionMissingItems: collections exists but collection_items
// does not, so the item INSERT fails.
func TestSeedSampleCollectionMissingItems(t *testing.T) {
	db := openDB(t)
	if _, err := db.Exec("CREATE TABLE collections (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	if err := seedSampleCollection(db); err == nil {
		t.Error("seedSampleCollection should error when collection_items is missing")
	}
}
