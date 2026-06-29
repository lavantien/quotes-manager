package quote

import "testing"

func TestDedupMergesSources(t *testing.T) {
	q1 := newQuote("AN 8.53", "AN 8.53", []string{"Gotami quote."}, "sacredness-and-profanity.txt")
	q2 := newQuote("AN 8.53", "AN 8.53", []string{"Gotami quote."}, "stream-entry-for-lay-buddhists.txt")
	q3 := newQuote("MN 22", "the Buddha, MN 22", []string{"Different text."}, "x.txt")
	out := Dedup([]*Quote{q1, q2, q3})
	if len(out) != 2 {
		t.Fatalf("got %d quotes, want 2", len(out))
	}
	if out[0].SuttaID != "AN 8.53" {
		t.Errorf("first kept SuttaID = %q", out[0].SuttaID)
	}
	if len(out[0].Sources) != 2 {
		t.Errorf("merged sources = %#v", out[0].Sources)
	}
}

func TestDedupKeepsDifferentExcerpts(t *testing.T) {
	a := newQuote("MN 22", "MN 22", []string{"Bhikkhus, that one can engage..."}, "a")
	b := newQuote("MN 22", "the Buddha, MN 22", []string{"Mendicants, that one can engage..."}, "b")
	out := Dedup([]*Quote{a, b})
	if len(out) != 2 {
		t.Fatalf("got %d quotes, want 2 (different excerpts)", len(out))
	}
}

func TestSortByCharCountShortestFirst(t *testing.T) {
	long := newQuote("X", "X", []string{"this is a much longer passage than the others"}, "a")
	mid := newQuote("Y", "Y", []string{"medium length here"}, "a")
	short := newQuote("Z", "Z", []string{"hi"}, "a")
	qs := []*Quote{long, mid, short}
	SortByCharCount(qs)
	for i := 1; i < len(qs); i++ {
		if qs[i].CharCount() < qs[i-1].CharCount() {
			t.Fatalf("not sorted ascending at %d: %d > %d", i, qs[i-1].CharCount(), qs[i].CharCount())
		}
	}
	if qs[0] != short {
		t.Errorf("shortest not first")
	}
}
