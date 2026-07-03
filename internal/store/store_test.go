package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/lavantien/quotes-manager/internal/quote"
)

func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "quotes.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func mustCreate(t *testing.T, s *SQLiteStore, q *quote.Quote) int64 {
	t.Helper()
	id, err := s.Create(q)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return id
}

func TestCreateListRoundTrip(t *testing.T) {
	s := newTestStore(t)
	id := mustCreate(t, s, quote.New("MN 22", "the Buddha, MN 22", []string{`"hi"`}))

	got, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	q := got[0]
	if q.ID != id {
		t.Errorf("ID = %d, want %d", q.ID, id)
	}
	if q.BodyMD != `*"hi"* - **the Buddha, MN 22**` {
		t.Errorf("BodyMD = %q", q.BodyMD)
	}
	if q.BodyText != `"hi"` || q.LineCount != 1 || q.CharCount != 4 {
		t.Errorf("derived = body %q line %d char %d", q.BodyText, q.LineCount, q.CharCount)
	}
}

func TestListOrderedByCharCount(t *testing.T) {
	s := newTestStore(t)
	long := mustCreate(t, s, quote.New("Long", "Long", []string{"abcdef"})) // 6 runes
	short := mustCreate(t, s, quote.New("Short", "Short", []string{"ab"}))  // 2 runes
	mid := mustCreate(t, s, quote.New("Mid", "Mid", []string{"abcd"}))      // 4 runes
	got, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	want := []int64{short, mid, long}
	for i, q := range got {
		if q.ID != want[i] {
			t.Errorf("pos %d ID = %d (char_count %d), want %d", i, q.ID, q.CharCount, want[i])
		}
	}
}

func TestListCharCountTieBreaksByID(t *testing.T) {
	s := newTestStore(t)
	a := mustCreate(t, s, quote.New("A", "A", []string{"xx"})) // 2 runes, lower id
	b := mustCreate(t, s, quote.New("B", "B", []string{"yy"})) // 2 runes, higher id
	got, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != a || got[1].ID != b {
		t.Errorf("tie-break order = %+v, want [%d %d]", got, a, b)
	}
}

func TestGet(t *testing.T) {
	s := newTestStore(t)
	id := mustCreate(t, s, quote.New("MN 22", "the Buddha, MN 22", []string{`"hi"`}))

	q, err := s.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if q.SuttaID != "MN 22" {
		t.Errorf("SuttaID = %q", q.SuttaID)
	}
	if _, err := s.Get(id + 999); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing Get err = %v, want ErrNotFound", err)
	}
}

func TestUpdateRecomputesDerived(t *testing.T) {
	s := newTestStore(t)
	id := mustCreate(t, s, quote.New("MN 22", "the Buddha, MN 22", []string{`"hi"`}))

	if err := s.Update(id, quote.New("DN 16", "the Buddha, DN 16", []string{`"one"`, `two`})); err != nil {
		t.Fatal(err)
	}
	q, err := s.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if q.SuttaID != "DN 16" || q.Citation != "the Buddha, DN 16" {
		t.Errorf("citation fields = %+v", q)
	}
	if q.LineCount != 2 || q.CharCount != 8 {
		t.Errorf("derived = line %d char %d", q.LineCount, q.CharCount)
	}
	wantBodyMD := "*\"one\"*  \n*two* - **the Buddha, DN 16**"
	if q.BodyMD != wantBodyMD {
		t.Errorf("BodyMD = %q, want %q", q.BodyMD, wantBodyMD)
	}
	if err := s.Update(id+999, quote.New("X", "X", []string{"x"})); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing Update err = %v, want ErrNotFound", err)
	}
}

func TestDeleteOne(t *testing.T) {
	s := newTestStore(t)
	id := mustCreate(t, s, quote.New("X", "X", []string{"x"}))

	if err := s.Delete(id); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.List(); len(got) != 0 {
		t.Errorf("after delete, len = %d", len(got))
	}
	if err := s.Delete(id); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing Delete err = %v, want ErrNotFound", err)
	}
}

func TestDeleteMany(t *testing.T) {
	s := newTestStore(t)
	a := mustCreate(t, s, quote.New("A", "A", []string{"a"}))
	b := mustCreate(t, s, quote.New("B", "B", []string{"b"}))
	c := mustCreate(t, s, quote.New("C", "C", []string{"c"}))

	if err := s.DeleteMany(nil); err != nil {
		t.Errorf("empty DeleteMany err = %v", err)
	}
	if err := s.DeleteMany([]int64{a, b}); err != nil {
		t.Fatal(err)
	}
	got, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != c {
		t.Errorf("after bulk delete = %+v", got)
	}
}

func TestSourcesRoundTrip(t *testing.T) {
	s := newTestStore(t)
	q := quote.New("X", "X", []string{"x"})
	q.Sources = []string{"a.txt", "b.txt"}
	id := mustCreate(t, s, q)

	got, err := s.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Sources) != 2 || got.Sources[0] != "a.txt" || got.Sources[1] != "b.txt" {
		t.Errorf("Sources = %#v", got.Sources)
	}
}

// TestOpenMigratesCollectionsName opens a database built with the pre-0.6.0
// collections schema (id only) and asserts Open adds the name column, preserving
// existing rows with the empty-string default.
func TestOpenMigratesCollectionsName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "quotes.db")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	// Legacy schema: collections had only an id column.
	if _, err := db.Exec("CREATE TABLE collections (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO collections (id) VALUES (7)"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	s, err := Open(path)
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	// The name column is now present and defaults to "" for the legacy row.
	cols, err := s.ListCollections()
	if err != nil {
		t.Fatal(err)
	}
	if len(cols) != 1 || cols[0].ID != 7 || cols[0].Name != "" {
		t.Errorf("legacy row after migration = %+v", cols)
	}
	// And the migrated column is writable.
	if err := s.RenameCollection(7, "keepsakes"); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetCollection(7)
	if got.Name != "keepsakes" {
		t.Errorf("name = %q, want keepsakes", got.Name)
	}
}
