package store

import (
	"errors"
	"testing"

	"github.com/lavantien/quotes-manager/internal/quote"
)

func mkMergeQuote(sutta, body string) *quote.Quote {
	return quote.New(sutta, "the Buddha, "+sutta, []string{body})
}

func TestMergeQuotesRopointsMemberships(t *testing.T) {
	s := newTestStore(t)
	keep := mustCreate(t, s, mkMergeQuote("MN 1", "keep body"))
	merge := mustCreate(t, s, mkMergeQuote("MN 2", "merge body"))
	other := mustCreate(t, s, mkMergeQuote("MN 3", "other body"))

	// A collection holding merge + other (keep not yet a member).
	cid, err := s.CreateCollection([]int64{merge, other})
	if err != nil {
		t.Fatalf("create collection: %v", err)
	}
	// A category tagging merge.
	catID, err := s.CreateCategory("suffering")
	if err != nil {
		t.Fatalf("create category: %v", err)
	}
	if err := s.SetQuoteCategories(merge, []int64{catID}); err != nil {
		t.Fatalf("set categories: %v", err)
	}

	if err := s.MergeQuotes(keep, []int64{merge}); err != nil {
		t.Fatalf("merge: %v", err)
	}

	if _, err := s.Get(merge); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected merged quote gone, got err=%v", err)
	}
	if _, err := s.Get(keep); err != nil {
		t.Errorf("keep quote gone: %v", err)
	}

	cq, err := s.CollectionQuotes(cid)
	if err != nil {
		t.Fatalf("collection quotes: %v", err)
	}
	if got := idsOf(cq); len(got) != 2 || got[0] != keep || got[1] != other {
		t.Errorf("collection members after merge = %v, want [%d %d]", got, keep, other)
	}
	catQs, err := s.CategoryQuotes(catID)
	if err != nil {
		t.Fatalf("category quotes: %v", err)
	}
	if got := idsOf(catQs); len(got) != 1 || got[0] != keep {
		t.Errorf("category members after merge = %v, want [%d]", got, keep)
	}
}

func TestMergeQuotesSkipsSharedMembership(t *testing.T) {
	// keep and merge both belong to the same collection: the repoint must not
	// collide on the composite PK and must drop merge's row.
	s := newTestStore(t)
	keep := mustCreate(t, s, mkMergeQuote("MN 1", "keep body"))
	merge := mustCreate(t, s, mkMergeQuote("MN 2", "merge body"))
	cid, err := s.CreateCollection([]int64{keep, merge})
	if err != nil {
		t.Fatalf("create collection: %v", err)
	}
	if err := s.MergeQuotes(keep, []int64{merge}); err != nil {
		t.Fatalf("merge: %v", err)
	}
	cq, _ := s.CollectionQuotes(cid)
	if got := idsOf(cq); len(got) != 1 || got[0] != keep {
		t.Errorf("shared-membership collection after merge = %v, want [%d]", got, keep)
	}
}

func TestMergeQuotesMissingIDs(t *testing.T) {
	s := newTestStore(t)
	keep := mustCreate(t, s, mkMergeQuote("MN 1", "keep body"))
	merge := mustCreate(t, s, mkMergeQuote("MN 2", "merge body"))

	if err := s.MergeQuotes(999, []int64{merge}); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing keep: err=%v, want ErrNotFound", err)
	}
	if err := s.MergeQuotes(keep, []int64{999}); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing merge id: err=%v, want ErrNotFound", err)
	}
}

func TestMergeQuotesEmptyIsNoop(t *testing.T) {
	s := newTestStore(t)
	keep := mustCreate(t, s, mkMergeQuote("MN 1", "keep body"))
	if err := s.MergeQuotes(keep, nil); err != nil {
		t.Errorf("empty merge: err=%v", err)
	}
	if _, err := s.Get(keep); err != nil {
		t.Errorf("keep gone after no-op: %v", err)
	}
}

func TestMergeQuotesIgnoresKeepInMergeList(t *testing.T) {
	s := newTestStore(t)
	keep := mustCreate(t, s, mkMergeQuote("MN 1", "keep body"))
	merge := mustCreate(t, s, mkMergeQuote("MN 2", "merge body"))
	if err := s.MergeQuotes(keep, []int64{keep, merge}); err != nil {
		t.Fatalf("merge: %v", err)
	}
	if _, err := s.Get(keep); err != nil {
		t.Errorf("keep gone: %v", err)
	}
	if _, err := s.Get(merge); !errors.Is(err, ErrNotFound) {
		t.Errorf("merge still present: err=%v", err)
	}
}

func TestMergeQuotesRollsBackOnFailure(t *testing.T) {
	// A missing id mid-merge rolls back the whole transaction.
	s := newTestStore(t)
	keep := mustCreate(t, s, mkMergeQuote("MN 1", "keep body"))
	merge := mustCreate(t, s, mkMergeQuote("MN 2", "merge body"))
	cid, err := s.CreateCollection([]int64{merge})
	if err != nil {
		t.Fatalf("create collection: %v", err)
	}
	if err := s.MergeQuotes(keep, []int64{merge, 999}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err=%v, want ErrNotFound", err)
	}
	if _, err := s.Get(merge); err != nil {
		t.Errorf("merge gone after rollback: %v", err)
	}
	cq, _ := s.CollectionQuotes(cid)
	if got := idsOf(cq); len(got) != 1 || got[0] != merge {
		t.Errorf("membership lost after rollback = %v, want [%d]", got, merge)
	}
}

func idsOf(qs []Quote) []int64 {
	out := make([]int64, len(qs))
	for i, q := range qs {
		out[i] = q.ID
	}
	return out
}
