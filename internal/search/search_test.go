package search

import (
	"sort"
	"strings"
	"testing"

	"github.com/lavantien/quotes-manager/internal/store"
)

func row(sutta, body string) store.Quote {
	return store.Quote{SuttaID: sutta, BodyText: body, Citation: sutta}
}

func TestParse(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want Query
	}{
		{"empty", "", Query{}},
		{"whitespace", "   \t  ", Query{}},
		{"single word", "buddha", Query{Pos: []string{"buddha"}}},
		{"lowercased", "BUDDHA", Query{Pos: []string{"buddha"}}},
		{"and of two words", "buddha suffering", Query{Pos: []string{"buddha", "suffering"}}},
		{"phrase", `"the buddha"`, Query{Pos: []string{"the buddha"}}},
		{"phrase and word", `"right view" buddha`, Query{Pos: []string{"right view", "buddha"}}},
		{"negation word", "-suffering", Query{Neg: []string{"suffering"}}},
		{"negation phrase", `-"the buddha"`, Query{Neg: []string{"the buddha"}}},
		{"mixed", `buddha -suffering "right view"`, Query{Pos: []string{"buddha", "right view"}, Neg: []string{"suffering"}}},
		{"dedupe positives", "buddha buddha", Query{Pos: []string{"buddha"}}},
		{"dedupe negatives", "-a -a", Query{Neg: []string{"a"}}},
		{"lone dash", "-", Query{}},
		{"dash separated by space is a no-op", "- foo", Query{Pos: []string{"foo"}}},
		{"hyphenated word stays one token", "foo-bar", Query{Pos: []string{"foo-bar"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Parse(c.in); !queriesEqual(got, c.want) {
				t.Errorf("Parse(%q) = %+v, want %+v", c.in, got, c.want)
			}
		})
	}
}

func TestIsZero(t *testing.T) {
	if (Query{}).IsZero() == false {
		t.Error("empty query should be zero")
	}
	if (Query{Pos: []string{"a"}}).IsZero() {
		t.Error("pos query should not be zero")
	}
	if (Query{Neg: []string{"a"}}).IsZero() {
		t.Error("neg query should not be zero")
	}
}

func TestMatch(t *testing.T) {
	const hay = "the buddha teaches suffering"
	cases := []struct {
		name string
		q    Query
		want bool
	}{
		{"empty matches all", Query{}, true},
		{"single present", Query{Pos: []string{"buddha"}}, true},
		{"single absent", Query{Pos: []string{"xyz"}}, false},
		{"and both present", Query{Pos: []string{"buddha", "teaches"}}, true},
		{"and one absent", Query{Pos: []string{"buddha", "xyz"}}, false},
		{"phrase present", Query{Pos: []string{"the buddha"}}, true},
		{"phrase absent", Query{Pos: []string{"the xyz"}}, false},
		{"negation present excludes", Query{Pos: []string{"buddha"}, Neg: []string{"suffering"}}, false},
		{"negation absent keeps", Query{Pos: []string{"buddha"}, Neg: []string{"xyz"}}, true},
		{"only negation present excludes", Query{Neg: []string{"suffering"}}, false},
		{"only negation absent keeps", Query{Neg: []string{"xyz"}}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Match(hay, c.q); got != c.want {
				t.Errorf("Match(%+v) = %v, want %v", c.q, got, c.want)
			}
		})
	}
}

func TestMatchCaseInsensitive(t *testing.T) {
	if !Match("The BUDDHA Sat", Query{Pos: []string{"buddha"}}) {
		t.Error("expected case-insensitive match")
	}
}

func TestFilter(t *testing.T) {
	qs := []store.Quote{
		row("MN 1", "the buddha teaches"),
		row("MN 2", "a quiet forest"),
		row("MN 3", "the buddha and a forest"),
	}
	t.Run("zero query returns input unchanged", func(t *testing.T) {
		if got := Filter(qs, Query{}); len(got) != len(qs) {
			t.Errorf("got %d quotes, want %d", len(got), len(qs))
		}
	})
	t.Run("and keeps only matches in order", func(t *testing.T) {
		got := Filter(qs, Query{Pos: []string{"buddha", "forest"}})
		want := []string{"MN 3"}
		if !sameOrder(ids(got), want) {
			t.Errorf("got %v, want %v", ids(got), want)
		}
	})
	t.Run("negation drops matches", func(t *testing.T) {
		got := Filter(qs, Query{Pos: []string{"buddha"}, Neg: []string{"forest"}})
		// MN 1 has buddha, no forest -> kept; MN 3 has both -> dropped.
		if !sameOrder(ids(got), []string{"MN 1"}) {
			t.Errorf("got %v, want [MN 1]", ids(got))
		}
	})
}

func TestHighlightTerms(t *testing.T) {
	got := HighlightTerms(Parse(`buddha -suffering "right view"`))
	if !sameSet(got, []string{"buddha", "right view"}) {
		t.Errorf("HighlightTerms = %v, want {buddha, right view}", got)
	}
	if HighlightTerms(Query{}) != nil {
		t.Error("zero query should yield no highlight terms")
	}
}

// --- properties ---

func TestParseStable(t *testing.T) {
	// Re-parsing a query reconstructed from Parse's output (phrases re-quoted)
	// yields the same Query.
	for _, in := range []string{
		`buddha suffering`, `"the buddha" -suffering`, `alpha -beta "gamma delta"`,
		`-x y`, `foo-bar baz`,
	} {
		q1 := Parse(in)
		var parts []string
		for _, p := range q1.Pos {
			if strings.ContainsAny(p, " \t") {
				parts = append(parts, `"`+p+`"`)
			} else {
				parts = append(parts, p)
			}
		}
		for _, n := range q1.Neg {
			if strings.ContainsAny(n, " \t") {
				parts = append(parts, `-"`+n+`"`)
			} else {
				parts = append(parts, "-"+n)
			}
		}
		q2 := Parse(strings.Join(parts, " "))
		if !queriesEqual(q1, q2) {
			t.Errorf("Parse not stable for %q: %+v vs %+v", in, q1, q2)
		}
	}
}

func TestAddingPositiveNarrows(t *testing.T) {
	qs := sampleQuotes()
	for _, extra := range []string{"forest", "buddha", "xyz"} {
		base := Query{Pos: []string{"the"}}
		wider := Filter(qs, base)
		narrow := Filter(qs, Query{Pos: append(append([]string{}, base.Pos...), extra)})
		if !subsetIDs(narrow, wider) {
			t.Errorf("adding positive %q did not narrow: %v not subset of %v", extra, ids(narrow), ids(wider))
		}
	}
}

func TestAddingNegativeNarrows(t *testing.T) {
	qs := sampleQuotes()
	for _, extra := range []string{"forest", "xyz"} {
		base := Query{Pos: []string{"the"}}
		wider := Filter(qs, base)
		narrow := Filter(qs, Query{Pos: base.Pos, Neg: []string{extra}})
		if !subsetIDs(narrow, wider) {
			t.Errorf("adding negative %q did not narrow: %v not subset of %v", extra, ids(narrow), ids(wider))
		}
	}
}

func TestFilterOrderInvariant(t *testing.T) {
	qs := sampleQuotes()
	a := Filter(qs, Query{Pos: []string{"buddha", "forest"}})
	b := Filter(qs, Query{Pos: []string{"forest", "buddha"}})
	if !sameOrder(ids(a), ids(b)) {
		t.Errorf("positive order changed results: %v vs %v", ids(a), ids(b))
	}
}

// --- helpers ---

func sampleQuotes() []store.Quote {
	return []store.Quote{
		row("MN 1", "the buddha teaches suffering"),
		row("MN 2", "a quiet forest monastery"),
		row("MN 3", "the buddha walked to the forest"),
		row("MN 4", "suffering and the deep forest"),
	}
}

func queriesEqual(a, b Query) bool { return sliceEq(a.Pos, b.Pos) && sliceEq(a.Neg, b.Neg) }

func sliceEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	x, y := append([]string{}, a...), append([]string{}, b...)
	sort.Strings(x)
	sort.Strings(y)
	return sliceEq(x, y)
}

func ids(qs []store.Quote) []string {
	out := make([]string, len(qs))
	for i, q := range qs {
		out[i] = q.SuttaID
	}
	return out
}

func sameOrder(a, b []string) bool { return sliceEq(a, b) }

func subsetIDs(small, big []store.Quote) bool {
	set := map[string]bool{}
	for _, q := range big {
		set[q.SuttaID] = true
	}
	for _, q := range small {
		if !set[q.SuttaID] {
			return false
		}
	}
	return true
}
