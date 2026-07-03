package store

import (
	"errors"
	"testing"

	"github.com/lavantien/quotes-manager/internal/quote"
)

func TestCreateCollectionAndList(t *testing.T) {
	s := newTestStore(t)
	q1 := mustCreate(t, s, quote.New("MN 1", "the Buddha, MN 1", []string{`"a"`}))
	q2 := mustCreate(t, s, quote.New("MN 2", "the Buddha, MN 2", []string{`"b"`}))

	cid, err := s.CreateCollection([]int64{q1, q2})
	if err != nil {
		t.Fatal(err)
	}
	if cid == 0 {
		t.Error("collection id = 0")
	}

	cols, err := s.ListCollections()
	if err != nil {
		t.Fatal(err)
	}
	if len(cols) != 1 || cols[0].ID != cid || cols[0].Count != 2 {
		t.Errorf("collections = %+v", cols)
	}

	qs, err := s.CollectionQuotes(cid)
	if err != nil {
		t.Fatal(err)
	}
	if len(qs) != 2 || qs[0].ID != q1 || qs[1].ID != q2 {
		t.Errorf("collection quotes = %+v", qs)
	}
}

func TestCollectionOrderPreserved(t *testing.T) {
	s := newTestStore(t)
	q1 := mustCreate(t, s, quote.New("A", "A", []string{"a"}))
	q2 := mustCreate(t, s, quote.New("B", "B", []string{"b"}))
	q3 := mustCreate(t, s, quote.New("C", "C", []string{"c"}))

	cid, _ := s.CreateCollection([]int64{q3, q1, q2})
	qs, _ := s.CollectionQuotes(cid)
	want := []int64{q3, q1, q2}
	for i, q := range qs {
		if q.ID != want[i] {
			t.Errorf("pos %d = %d, want %d", i, q.ID, want[i])
		}
	}
}

func TestListCollectionsOrderedByID(t *testing.T) {
	s := newTestStore(t)
	q := mustCreate(t, s, quote.New("A", "A", []string{"a"}))
	c1, _ := s.CreateCollection([]int64{q})
	c2, _ := s.CreateCollection([]int64{q})
	c3, _ := s.CreateCollection([]int64{q})
	cols, _ := s.ListCollections()
	got := []int64{cols[0].ID, cols[1].ID, cols[2].ID}
	if got[0] != c1 || got[1] != c2 || got[2] != c3 {
		t.Errorf("order = %v, want %v %v %v", got, c1, c2, c3)
	}
}

func TestCreateEmptyCollection(t *testing.T) {
	s := newTestStore(t)
	cid, err := s.CreateCollection(nil)
	if err != nil {
		t.Fatal(err)
	}
	cols, _ := s.ListCollections()
	if len(cols) != 1 || cols[0].ID != cid || cols[0].Count != 0 {
		t.Errorf("empty collection = %+v", cols)
	}
	qs, _ := s.CollectionQuotes(cid)
	if len(qs) != 0 {
		t.Errorf("empty collection quotes = %+v", qs)
	}
}

func TestGetCollectionNotFound(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.GetCollection(999); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestDeleteCollection(t *testing.T) {
	s := newTestStore(t)
	q := mustCreate(t, s, quote.New("A", "A", []string{"a"}))
	cid, _ := s.CreateCollection([]int64{q})

	if err := s.DeleteCollection(cid); err != nil {
		t.Fatal(err)
	}
	cols, _ := s.ListCollections()
	if len(cols) != 0 {
		t.Errorf("collection not deleted: %+v", cols)
	}
	// Deleting a quote that was in the (now deleted) collection still works.
	if err := s.Delete(q); err != nil {
		t.Errorf("delete quote after collection delete: %v", err)
	}
}

func TestDeleteCollectionNotFound(t *testing.T) {
	s := newTestStore(t)
	if err := s.DeleteCollection(999); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestDeleteQuoteRemovesFromCollection(t *testing.T) {
	s := newTestStore(t)
	q1 := mustCreate(t, s, quote.New("A", "A", []string{"a"}))
	q2 := mustCreate(t, s, quote.New("B", "B", []string{"b"}))
	cid, _ := s.CreateCollection([]int64{q1, q2})

	if err := s.Delete(q1); err != nil {
		t.Fatal(err)
	}
	qs, _ := s.CollectionQuotes(cid)
	if len(qs) != 1 || qs[0].ID != q2 {
		t.Errorf("collection still references deleted quote: %+v", qs)
	}
	cols, _ := s.ListCollections()
	if cols[0].Count != 1 {
		t.Errorf("count = %d, want 1", cols[0].Count)
	}
}

func TestDeleteManyRemovesFromCollection(t *testing.T) {
	s := newTestStore(t)
	q1 := mustCreate(t, s, quote.New("A", "A", []string{"a"}))
	q2 := mustCreate(t, s, quote.New("B", "B", []string{"b"}))
	q3 := mustCreate(t, s, quote.New("C", "C", []string{"c"}))
	cid, _ := s.CreateCollection([]int64{q1, q2, q3})

	if err := s.DeleteMany([]int64{q1, q3}); err != nil {
		t.Fatal(err)
	}
	qs, _ := s.CollectionQuotes(cid)
	if len(qs) != 1 || qs[0].ID != q2 {
		t.Errorf("after bulk delete collection = %+v", qs)
	}
}

func TestReorderCollection(t *testing.T) {
	s := newTestStore(t)
	q1 := mustCreate(t, s, quote.New("A", "A", []string{"a"}))
	q2 := mustCreate(t, s, quote.New("B", "B", []string{"b"}))
	q3 := mustCreate(t, s, quote.New("C", "C", []string{"c"}))
	cid, _ := s.CreateCollection([]int64{q1, q2, q3})

	if err := s.ReorderCollection(cid, []int64{q3, q1, q2}); err != nil {
		t.Fatal(err)
	}
	qs, _ := s.CollectionQuotes(cid)
	want := []int64{q3, q1, q2}
	for i, q := range qs {
		if q.ID != want[i] {
			t.Errorf("pos %d = %d, want %d", i, q.ID, want[i])
		}
	}
}

func TestReorderCollectionNotFound(t *testing.T) {
	s := newTestStore(t)
	if err := s.ReorderCollection(999, []int64{1}); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestReorderCollectionUnknownQuote(t *testing.T) {
	s := newTestStore(t)
	q1 := mustCreate(t, s, quote.New("A", "A", []string{"a"}))
	q2 := mustCreate(t, s, quote.New("B", "B", []string{"b"})) // not a member
	cid, _ := s.CreateCollection([]int64{q1})

	err := s.ReorderCollection(cid, []int64{q1, q2})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	// Failed reorder must not leave partial state: q1 still first.
	qs, _ := s.CollectionQuotes(cid)
	if len(qs) != 1 || qs[0].ID != q1 {
		t.Errorf("partial reorder changed state: %+v", qs)
	}
}

func TestAddToCollectionPrependsOnTop(t *testing.T) {
	s := newTestStore(t)
	q1 := mustCreate(t, s, quote.New("A", "A", []string{"a"}))
	q2 := mustCreate(t, s, quote.New("B", "B", []string{"b"}))
	q3 := mustCreate(t, s, quote.New("C", "C", []string{"c"}))
	cid, _ := s.CreateCollection([]int64{q1, q2})

	if err := s.AddToCollection(cid, []int64{q3}); err != nil {
		t.Fatal(err)
	}
	qs, _ := s.CollectionQuotes(cid)
	want := []int64{q3, q1, q2}
	if len(qs) != len(want) {
		t.Fatalf("len = %d, want %d (%+v)", len(qs), len(want), qs)
	}
	for i, q := range qs {
		if q.ID != want[i] {
			t.Errorf("pos %d = %d, want %d", i, q.ID, want[i])
		}
	}
}

func TestAddToCollectionPreservesExistingOrder(t *testing.T) {
	s := newTestStore(t)
	q1 := mustCreate(t, s, quote.New("A", "A", []string{"a"}))
	q2 := mustCreate(t, s, quote.New("B", "B", []string{"b"}))
	q3 := mustCreate(t, s, quote.New("C", "C", []string{"c"}))
	q4 := mustCreate(t, s, quote.New("D", "D", []string{"d"}))
	cid, _ := s.CreateCollection([]int64{q3, q1, q2}) // manual order

	if err := s.AddToCollection(cid, []int64{q4}); err != nil {
		t.Fatal(err)
	}
	qs, _ := s.CollectionQuotes(cid)
	want := []int64{q4, q3, q1, q2} // new on top, existing order unchanged
	for i, q := range qs {
		if q.ID != want[i] {
			t.Errorf("pos %d = %d, want %d", i, q.ID, want[i])
		}
	}
}

func TestAddToCollectionSkipsDuplicates(t *testing.T) {
	s := newTestStore(t)
	q1 := mustCreate(t, s, quote.New("A", "A", []string{"a"}))
	q2 := mustCreate(t, s, quote.New("B", "B", []string{"b"}))
	q3 := mustCreate(t, s, quote.New("C", "C", []string{"c"}))
	cid, _ := s.CreateCollection([]int64{q1, q2})

	// q1 is already a member; q3 is new. q3 lands on top; q1 is not duplicated.
	if err := s.AddToCollection(cid, []int64{q1, q3, q1}); err != nil {
		t.Fatal(err)
	}
	qs, _ := s.CollectionQuotes(cid)
	want := []int64{q3, q1, q2}
	if len(qs) != len(want) {
		t.Fatalf("len = %d, want %d (%+v)", len(qs), len(want), qs)
	}
	for i, q := range qs {
		if q.ID != want[i] {
			t.Errorf("pos %d = %d, want %d", i, q.ID, want[i])
		}
	}
}

func TestAddToCollectionUnknownCollection(t *testing.T) {
	s := newTestStore(t)
	q1 := mustCreate(t, s, quote.New("A", "A", []string{"a"}))
	if err := s.AddToCollection(999, []int64{q1}); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// collectionIDs is a small helper pulling the ordered id list out of []Quote.
func collectionIDs(qs []Quote) []int64 {
	out := make([]int64, len(qs))
	for i, q := range qs {
		out[i] = q.ID
	}
	return out
}

func assertOrder(t *testing.T, qs []Quote, want ...int64) {
	t.Helper()
	got := collectionIDs(qs)
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i, id := range want {
		if got[i] != id {
			t.Errorf("pos %d = %d, want %d", i, got[i], id)
		}
	}
}

func TestRenameCollection(t *testing.T) {
	s := newTestStore(t)
	q := mustCreate(t, s, quote.New("A", "A", []string{"a"}))
	cid, _ := s.CreateCollection([]int64{q})

	if err := s.RenameCollection(cid, "Favorites"); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetCollection(cid)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Favorites" {
		t.Errorf("name = %q, want Favorites", got.Name)
	}
	// Names are not unique: two collections may share a name.
	cid2, _ := s.CreateCollection(nil)
	if err := s.RenameCollection(cid2, "Favorites"); err != nil {
		t.Fatalf("rename to duplicate name: %v", err)
	}
	cols, _ := s.ListCollections()
	if len(cols) != 2 || cols[0].Name != "Favorites" || cols[1].Name != "Favorites" {
		t.Errorf("collections = %+v", cols)
	}
}

func TestRenameCollectionNotFound(t *testing.T) {
	s := newTestStore(t)
	if err := s.RenameCollection(999, "x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestInsertAtCollectionShiftsDown(t *testing.T) {
	s := newTestStore(t)
	q1 := mustCreate(t, s, quote.New("A", "A", []string{"a"}))
	q2 := mustCreate(t, s, quote.New("B", "B", []string{"b"}))
	q3 := mustCreate(t, s, quote.New("C", "C", []string{"c"}))
	q4 := mustCreate(t, s, quote.New("D", "D", []string{"d"}))
	cid, _ := s.CreateCollection([]int64{q1, q2, q3})

	if err := s.InsertAtCollection(cid, []int64{q4}, 2); err != nil {
		t.Fatal(err)
	}
	assertOrder(t, mustCollectionQuotes(t, s, cid), q1, q4, q2, q3)
}

func TestInsertAtCollectionMultiplePreservesOrder(t *testing.T) {
	s := newTestStore(t)
	q1 := mustCreate(t, s, quote.New("A", "A", []string{"a"}))
	q2 := mustCreate(t, s, quote.New("B", "B", []string{"b"}))
	q3 := mustCreate(t, s, quote.New("C", "C", []string{"c"}))
	q4 := mustCreate(t, s, quote.New("D", "D", []string{"d"}))
	q5 := mustCreate(t, s, quote.New("E", "E", []string{"e"}))
	cid, _ := s.CreateCollection([]int64{q1, q2, q3})

	if err := s.InsertAtCollection(cid, []int64{q4, q5}, 2); err != nil {
		t.Fatal(err)
	}
	assertOrder(t, mustCollectionQuotes(t, s, cid), q1, q4, q5, q2, q3)
}

func TestInsertAtCollectionSkipsDuplicates(t *testing.T) {
	s := newTestStore(t)
	q1 := mustCreate(t, s, quote.New("A", "A", []string{"a"}))
	q2 := mustCreate(t, s, quote.New("B", "B", []string{"b"}))
	q3 := mustCreate(t, s, quote.New("C", "C", []string{"c"}))
	q4 := mustCreate(t, s, quote.New("D", "D", []string{"d"}))
	cid, _ := s.CreateCollection([]int64{q1, q2, q3})

	// q2 is already a member -> skipped; q4 is new and lands at pos 1, shifting all.
	if err := s.InsertAtCollection(cid, []int64{q2, q4, q2}, 1); err != nil {
		t.Fatal(err)
	}
	assertOrder(t, mustCollectionQuotes(t, s, cid), q4, q1, q2, q3)
}

func TestInsertAtCollectionClampsHighAppends(t *testing.T) {
	s := newTestStore(t)
	q1 := mustCreate(t, s, quote.New("A", "A", []string{"a"}))
	q2 := mustCreate(t, s, quote.New("B", "B", []string{"b"}))
	q3 := mustCreate(t, s, quote.New("C", "C", []string{"c"}))
	cid, _ := s.CreateCollection([]int64{q1, q2})

	// pos far beyond the end clamps to count+1 -> append.
	if err := s.InsertAtCollection(cid, []int64{q3}, 99); err != nil {
		t.Fatal(err)
	}
	assertOrder(t, mustCollectionQuotes(t, s, cid), q1, q2, q3)
}

func TestInsertAtCollectionClampsLowPrepends(t *testing.T) {
	s := newTestStore(t)
	q1 := mustCreate(t, s, quote.New("A", "A", []string{"a"}))
	q2 := mustCreate(t, s, quote.New("B", "B", []string{"b"}))
	q3 := mustCreate(t, s, quote.New("C", "C", []string{"c"}))
	cid, _ := s.CreateCollection([]int64{q1, q2})

	// pos < 1 clamps to 1 -> prepend.
	if err := s.InsertAtCollection(cid, []int64{q3}, 0); err != nil {
		t.Fatal(err)
	}
	assertOrder(t, mustCollectionQuotes(t, s, cid), q3, q1, q2)
}

func TestInsertAtCollectionEmptyCollection(t *testing.T) {
	s := newTestStore(t)
	q1 := mustCreate(t, s, quote.New("A", "A", []string{"a"}))
	q2 := mustCreate(t, s, quote.New("B", "B", []string{"b"}))
	cid, _ := s.CreateCollection(nil)

	if err := s.InsertAtCollection(cid, []int64{q1, q2}, 5); err != nil {
		t.Fatal(err)
	}
	assertOrder(t, mustCollectionQuotes(t, s, cid), q1, q2)
}

func TestInsertAtCollectionNoOpWhenAllMembers(t *testing.T) {
	s := newTestStore(t)
	q1 := mustCreate(t, s, quote.New("A", "A", []string{"a"}))
	q2 := mustCreate(t, s, quote.New("B", "B", []string{"b"}))
	cid, _ := s.CreateCollection([]int64{q1, q2})

	if err := s.InsertAtCollection(cid, []int64{q1, q2}, 1); err != nil {
		t.Fatal(err)
	}
	assertOrder(t, mustCollectionQuotes(t, s, cid), q1, q2)
}

func TestInsertAtCollectionUnknownCollection(t *testing.T) {
	s := newTestStore(t)
	q1 := mustCreate(t, s, quote.New("A", "A", []string{"a"}))
	if err := s.InsertAtCollection(999, []int64{q1}, 1); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestQuoteCollectionMap(t *testing.T) {
	s := newTestStore(t)
	q1 := mustCreate(t, s, quote.New("A", "A", []string{"a"}))
	q2 := mustCreate(t, s, quote.New("B", "B", []string{"b"}))
	q3 := mustCreate(t, s, quote.New("C", "C", []string{"c"}))
	c1, _ := s.CreateCollection([]int64{q1, q2})
	if err := s.RenameCollection(c1, "first"); err != nil {
		t.Fatal(err)
	}
	c2, _ := s.CreateCollection([]int64{q2, q3}) // q2 is in both

	m, err := s.QuoteCollectionMap()
	if err != nil {
		t.Fatal(err)
	}
	if len(m[q1]) != 1 || m[q1][0].ID != c1 || m[q1][0].Name != "first" {
		t.Errorf("q1 map = %+v", m[q1])
	}
	if len(m[q2]) != 2 {
		t.Errorf("q2 should be in 2 collections, got %+v", m[q2])
	}
	if len(m[q3]) != 1 || m[q3][0].ID != c2 {
		t.Errorf("q3 map = %+v", m[q3])
	}
	if _, ok := m[9999]; ok {
		t.Error("unrelated quote should be absent from the map")
	}
}

func mustCollectionQuotes(t *testing.T, s *SQLiteStore, cid int64) []Quote {
	t.Helper()
	qs, err := s.CollectionQuotes(cid)
	if err != nil {
		t.Fatal(err)
	}
	return qs
}
