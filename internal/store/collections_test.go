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
