package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

const legacySchemaSQL = `CREATE TABLE quotes (
    id          INTEGER PRIMARY KEY,
    sutta_id    TEXT    NOT NULL,
    citation    TEXT    NOT NULL,
    body_md     TEXT    NOT NULL,
    body_text   TEXT    NOT NULL,
    line_count  INTEGER NOT NULL,
    char_count  INTEGER NOT NULL,
    sources     TEXT    NOT NULL
);
CREATE TABLE collections (
    id   INTEGER PRIMARY KEY,
    name TEXT NOT NULL DEFAULT ''
);
CREATE TABLE collection_items (
    collection_id INTEGER NOT NULL,
    quote_id      INTEGER NOT NULL,
    position      INTEGER NOT NULL,
    PRIMARY KEY (collection_id, quote_id)
);
CREATE TABLE categories (
    id   INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE
);
CREATE TABLE category_items (
    category_id INTEGER NOT NULL,
    quote_id    INTEGER NOT NULL,
    PRIMARY KEY (category_id, quote_id)
);`

func openLegacyStore(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "quotes.db")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open legacy: %v", err)
	}
	if _, err := db.Exec(legacySchemaSQL); err != nil {
		t.Fatalf("legacy schema: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO quotes (id, sutta_id, citation, body_md, body_text, line_count, char_count, sources)
		VALUES (1, 'MN 1', 'the Buddha, MN 1', '*"a"* - **the Buddha, MN 1**', '"a"', 1, 3, 'x');
		INSERT INTO collections (id, name) VALUES (1, 'old');
		INSERT INTO collection_items (collection_id, quote_id, position) VALUES (1, 1, 1);`); err != nil {
		t.Fatalf("legacy data: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close legacy: %v", err)
	}
	return path
}

func TestOpenMigratesLegacySchema(t *testing.T) {
	path := openLegacyStore(t)
	s, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	for _, table := range []string{"suttas", "collection_groups", "collection_sutta_items"} {
		var n int
		if err := s.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Errorf("table %s missing after migration", table)
		}
	}
	var hasGroupID int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('collection_items') WHERE name = 'group_id'").Scan(&hasGroupID); err != nil {
		t.Fatal(err)
	}
	if hasGroupID != 1 {
		t.Error("collection_items.group_id missing after migration")
	}
	qs, err := s.List()
	if err != nil || len(qs) != 1 || qs[0].SuttaID != "MN 1" {
		t.Errorf("legacy quote lost after migration: %v %v", qs, err)
	}
	cols, err := s.CollectionQuotes(1)
	if err != nil || len(cols) != 1 || cols[0].ID != 1 {
		t.Errorf("legacy membership lost after migration: %v %v", cols, err)
	}
}

func TestListSuttasEmpty(t *testing.T) {
	s := newTestStore(t)
	got, err := s.ListSuttas()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("suttas = %+v, want empty", got)
	}
}

func mustCollection(t *testing.T, s *SQLiteStore) int64 {
	t.Helper()
	cid, err := s.CreateCollection(nil)
	if err != nil {
		t.Fatalf("create collection: %v", err)
	}
	return cid
}

func TestCreateAndListCollectionGroups(t *testing.T) {
	s := newTestStore(t)
	cid := mustCollection(t, s)

	g1, err := s.CreateCollectionGroup(cid, "senses restraint")
	if err != nil {
		t.Fatal(err)
	}
	g2, err := s.CreateCollectionGroup(cid, "wakefulness")
	if err != nil {
		t.Fatal(err)
	}
	if g1 == 0 || g2 == 0 || g1 == g2 {
		t.Errorf("group ids = %d %d", g1, g2)
	}
	groups, err := s.ListCollectionGroups(cid)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 {
		t.Fatalf("groups = %+v, want 2", groups)
	}
	if groups[0].ID != g1 || groups[0].Name != "senses restraint" || groups[0].Count != 0 {
		t.Errorf("group 1 = %+v", groups[0])
	}
	if groups[1].ID != g2 || groups[1].Name != "wakefulness" || groups[1].Count != 0 {
		t.Errorf("group 2 = %+v", groups[1])
	}
}

func TestCreateCollectionGroupAllowsDuplicateNames(t *testing.T) {
	s := newTestStore(t)
	cid := mustCollection(t, s)
	if _, err := s.CreateCollectionGroup(cid, "same"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateCollectionGroup(cid, "same"); err != nil {
		t.Fatalf("duplicate group name rejected: %v", err)
	}
	groups, _ := s.ListCollectionGroups(cid)
	if len(groups) != 2 || groups[0].Name != "same" || groups[1].Name != "same" {
		t.Errorf("groups = %+v", groups)
	}
}

func TestCreateCollectionGroupEmptyName(t *testing.T) {
	s := newTestStore(t)
	cid := mustCollection(t, s)
	if _, err := s.CreateCollectionGroup(cid, ""); !errors.Is(err, ErrEmptyName) {
		t.Errorf("err = %v, want ErrEmptyName", err)
	}
	if _, err := s.CreateCollectionGroup(cid, "   "); !errors.Is(err, ErrEmptyName) {
		t.Errorf("err = %v, want ErrEmptyName", err)
	}
}

func TestCreateCollectionGroupUnknownCollection(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.CreateCollectionGroup(999, "x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestRenameCollectionGroup(t *testing.T) {
	s := newTestStore(t)
	cid := mustCollection(t, s)
	g, _ := s.CreateCollectionGroup(cid, "old")

	if err := s.RenameCollectionGroup(cid, g, "new"); err != nil {
		t.Fatal(err)
	}
	groups, _ := s.ListCollectionGroups(cid)
	if groups[0].Name != "new" {
		t.Errorf("name = %q, want new", groups[0].Name)
	}
}

func TestRenameCollectionGroupNotFound(t *testing.T) {
	s := newTestStore(t)
	cid := mustCollection(t, s)
	g, _ := s.CreateCollectionGroup(cid, "old")
	other := mustCollection(t, s)

	if err := s.RenameCollectionGroup(cid, 999, "x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown group: err = %v, want ErrNotFound", err)
	}
	if err := s.RenameCollectionGroup(999, g, "x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown collection: err = %v, want ErrNotFound", err)
	}
	if err := s.RenameCollectionGroup(other, g, "x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("group of another collection: err = %v, want ErrNotFound", err)
	}
}

func TestDeleteCollectionGroup(t *testing.T) {
	s := newTestStore(t)
	cid := mustCollection(t, s)
	g1, _ := s.CreateCollectionGroup(cid, "one")
	g2, _ := s.CreateCollectionGroup(cid, "two")

	if err := s.DeleteCollectionGroup(cid, g1); err != nil {
		t.Fatal(err)
	}
	groups, _ := s.ListCollectionGroups(cid)
	if len(groups) != 1 || groups[0].ID != g2 {
		t.Errorf("groups after delete = %+v", groups)
	}
}

func TestDeleteCollectionGroupNotFound(t *testing.T) {
	s := newTestStore(t)
	cid := mustCollection(t, s)
	g, _ := s.CreateCollectionGroup(cid, "one")
	other := mustCollection(t, s)

	if err := s.DeleteCollectionGroup(cid, 999); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown group: err = %v, want ErrNotFound", err)
	}
	if err := s.DeleteCollectionGroup(999, g); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown collection: err = %v, want ErrNotFound", err)
	}
	if err := s.DeleteCollectionGroup(other, g); !errors.Is(err, ErrNotFound) {
		t.Errorf("group of another collection: err = %v, want ErrNotFound", err)
	}
}

func TestReorderCollectionGroups(t *testing.T) {
	s := newTestStore(t)
	cid := mustCollection(t, s)
	g1, _ := s.CreateCollectionGroup(cid, "one")
	g2, _ := s.CreateCollectionGroup(cid, "two")
	g3, _ := s.CreateCollectionGroup(cid, "three")

	if err := s.ReorderCollectionGroups(cid, []int64{g3, g1, g2}); err != nil {
		t.Fatal(err)
	}
	groups, _ := s.ListCollectionGroups(cid)
	want := []int64{g3, g1, g2}
	for i, g := range groups {
		if g.ID != want[i] {
			t.Errorf("pos %d = %d, want %d", i, g.ID, want[i])
		}
	}
	rows, err := s.db.Query("SELECT id FROM collection_groups WHERE collection_id = ? ORDER BY position ASC", cid)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var got []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		got = append(got, id)
	}
	if !equalInts(got, want) {
		t.Errorf("stored order = %v, want %v", got, want)
	}
	for i, id := range got {
		var pos int
		if err := s.db.QueryRow("SELECT position FROM collection_groups WHERE id = ?", id).Scan(&pos); err != nil {
			t.Fatal(err)
		}
		if pos != i+1 {
			t.Errorf("group %d stored position = %d, want %d", id, pos, i+1)
		}
	}
}

func TestReorderCollectionGroupsNotFound(t *testing.T) {
	s := newTestStore(t)
	cid := mustCollection(t, s)
	g1, _ := s.CreateCollectionGroup(cid, "one")

	if err := s.ReorderCollectionGroups(999, []int64{g1}); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown collection: err = %v, want ErrNotFound", err)
	}
	err := s.ReorderCollectionGroups(cid, []int64{g1, 999})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown group: err = %v, want ErrNotFound", err)
	}
	groups, _ := s.ListCollectionGroups(cid)
	if len(groups) != 1 || groups[0].ID != g1 {
		t.Errorf("partial reorder changed state: %+v", groups)
	}
}

func TestListCollectionGroupsEmptyForUnknownCollection(t *testing.T) {
	s := newTestStore(t)
	groups, err := s.ListCollectionGroups(999)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 0 {
		t.Errorf("groups = %+v, want empty", groups)
	}
}
