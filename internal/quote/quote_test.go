package quote

import "testing"

func TestNew(t *testing.T) {
	q := New("MN 22", "the Buddha, MN 22", []string{"one", "two"})
	if q.SuttaID != "MN 22" || q.Citation != "the Buddha, MN 22" {
		t.Errorf("fields = %+v", q)
	}
	if len(q.Passages) != 2 || q.Passages[0] != "one" || q.Passages[1] != "two" {
		t.Errorf("passages = %#v", q.Passages)
	}
	if len(q.Sources) != 0 {
		t.Errorf("sources = %#v, want empty", q.Sources)
	}
	if q.LineCount() != 2 || q.CharCount() != 6 {
		t.Errorf("derived = line %d char %d", q.LineCount(), q.CharCount())
	}
}

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

func TestSortByCharCountTieBreakers(t *testing.T) {
	t.Run("sutta id breaks ties", func(t *testing.T) {
		bb := newQuote("BBB", "BBB", []string{"xx"}, "a")
		aaa := newQuote("AAA", "AAA", []string{"yy"}, "a")
		qs := []*Quote{bb, aaa}
		SortByCharCount(qs)
		if qs[0].SuttaID != "AAA" || qs[1].SuttaID != "BBB" {
			t.Errorf("tie-break order = %s, %s; want AAA, BBB", qs[0].SuttaID, qs[1].SuttaID)
		}
	})
	t.Run("body text breaks sutta-id ties", func(t *testing.T) {
		high := newQuote("AAA", "AAA", []string{"zz"}, "a")
		low := newQuote("AAA", "AAA", []string{"aa"}, "a")
		qs := []*Quote{high, low}
		SortByCharCount(qs)
		if qs[0].BodyText() != "aa" || qs[1].BodyText() != "zz" {
			t.Errorf("body-text tie-break = %q, %q; want aa, zz", qs[0].BodyText(), qs[1].BodyText())
		}
	})
}

func TestCharCountCountsRunes(t *testing.T) {
	// Pāli diacritics are multibyte in UTF-8 but one rune each; the sort key
	// must count runes, not bytes.
	q := New("AN 1", "AN 1", []string{"āīū"})
	if q.CharCount() != 3 {
		t.Errorf("CharCount = %d, want 3 (rune count, not bytes)", q.CharCount())
	}
}

func TestBodyTextDirect(t *testing.T) {
	if got := (&Quote{Passages: []string{"a", "b"}}).BodyText(); got != "a\nb" {
		t.Errorf("BodyText = %q, want %q", got, "a\nb")
	}
	if got := (&Quote{}).BodyText(); got != "" {
		t.Errorf("empty BodyText = %q, want empty", got)
	}
}

func TestDedupEmpty(t *testing.T) {
	if got := Dedup(nil); len(got) != 0 {
		t.Errorf("Dedup(nil) = %#v, want empty", got)
	}
	if got := Dedup([]*Quote{}); len(got) != 0 {
		t.Errorf("Dedup(empty) = %#v, want empty", got)
	}
}

func TestDedupCollapsesWhitespace(t *testing.T) {
	// Passages that differ only by runs of whitespace dedup together.
	a := newQuote("MN 1", "MN 1", []string{"foo bar"}, "a")
	b := newQuote("MN 2", "MN 2", []string{"foo\t bar"}, "b")
	out := Dedup([]*Quote{a, b})
	if len(out) != 1 {
		t.Fatalf("got %d quotes, want 1 (whitespace-collapsed dup)", len(out))
	}
	if len(out[0].Sources) != 2 {
		t.Errorf("merged sources = %#v", out[0].Sources)
	}
}

func TestCollapseWhitespace(t *testing.T) {
	cases := map[string]string{
		"normal":        "normal",
		"":              "",
		"double  space": "double space",
		"tab\tsep":      "tab sep",
		"new\nline":     "new line",
		"car\rret":      "car ret",
		"non break":     "non break",
		"  leading":     " leading",
	}
	for in, want := range cases {
		if got := collapseWhitespace(in); got != want {
			t.Errorf("collapseWhitespace(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMergeSourcesDedup(t *testing.T) {
	q := &Quote{Sources: []string{"a"}}
	q.mergeSources("b", "a", "c")
	want := []string{"a", "b", "c"}
	if len(q.Sources) != len(want) {
		t.Fatalf("Sources = %#v, want %#v", q.Sources, want)
	}
	for i := range want {
		if q.Sources[i] != want[i] {
			t.Errorf("Sources[%d] = %q, want %q", i, q.Sources[i], want[i])
		}
	}
}
