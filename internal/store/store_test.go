package store

import (
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

func TestCreateAssignsIncreasingSortOrder(t *testing.T) {
	s := newTestStore(t)
	for range 3 {
		mustCreate(t, s, quote.New("X", "X", []string{"a"}))
	}
	got, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	for i, q := range got {
		if q.SortOrder != int64(i+1) {
			t.Errorf("pos %d SortOrder = %d, want %d", i, q.SortOrder, i+1)
		}
	}
}

func TestListOrderedBySortOrder(t *testing.T) {
	s := newTestStore(t)
	a := mustCreate(t, s, quote.New("A", "A", []string{"a"}))
	b := mustCreate(t, s, quote.New("B", "B", []string{"b"}))
	c := mustCreate(t, s, quote.New("C", "C", []string{"c"}))

	if err := s.Reorder([]int64{c, a, b}); err != nil {
		t.Fatal(err)
	}
	got, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	want := []int64{c, a, b}
	for i, q := range got {
		if q.ID != want[i] {
			t.Errorf("pos %d ID = %d, want %d", i, q.ID, want[i])
		}
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

func TestReorderRejectsUnknownID(t *testing.T) {
	s := newTestStore(t)
	a := mustCreate(t, s, quote.New("A", "A", []string{"a"}))
	if err := s.Reorder([]int64{a, 9999}); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown id err = %v, want ErrNotFound", err)
	}
	// Failed reorder must not leave partial state: order unchanged.
	got, _ := s.List()
	if len(got) != 1 || got[0].ID != a {
		t.Errorf("partial reorder changed state: %+v", got)
	}
}

func TestReorderPersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "q.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]int64, 3)
	for i, name := range []string{"A", "B", "C"} {
		ids[i] = mustCreate(t, s, quote.New(name, name, []string{"x"}))
	}
	want := []int64{ids[2], ids[0], ids[1]}
	if err := s.Reorder(want); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s2.Close() }()
	got, err := s2.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i, q := range got {
		if q.ID != want[i] {
			t.Errorf("pos %d ID = %d, want %d", i, q.ID, want[i])
		}
	}
}
