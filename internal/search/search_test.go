package search

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"

	"github.com/lavantien/quotes-manager/internal/store"
)

func TestTerms(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"whitespace only", "   \t\n ", nil},
		{"single", "buddha", []string{"buddha"}},
		{"lowercases", "Buddha MN", []string{"buddha", "mn"}},
		{"trims each field", "  buddha   mn  ", []string{"buddha", "mn"}},
		{"dedupes case-insensitively (stable order)", "buddha Buddha BUDDHA truth", []string{"buddha", "truth"}},
		{"keeps repeated distinct terms once", "mn mn an", []string{"mn", "an"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Terms(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Terms(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestMatch(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		terms []string
		want  bool
	}{
		{"nil terms never match", "anything", nil, false},
		{"empty terms never match", "anything", []string{}, false},
		{"single term in body", "the buddha spoke", []string{"buddha"}, true},
		{"multi-word term as substring", "the buddha, mn 22", []string{"mn 22"}, true},
		{"or any term matches", "alpha omega", []string{"beta", "omega"}, true},
		{"or none match", "alpha", []string{"beta", "gamma"}, false},
		{"case insensitive", "The Buddha", []string{"buddha"}, true},
		{"substring inside punctuation", `"buddha,"`, []string{"buddha"}, true},
		{"lowercases terms defensively", "the buddha", []string{"Buddha"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Match(tc.text, tc.terms); got != tc.want {
				t.Errorf("Match(%q, %v) = %v, want %v", tc.text, tc.terms, got, tc.want)
			}
		})
	}
}

func TestFilter(t *testing.T) {
	qs := []store.Quote{
		{ID: 1, BodyText: "the buddha spoke", Citation: "the Buddha, MN 22"},
		{ID: 2, BodyText: "a quiet forest", Citation: "a monk, AN 3"},
		{ID: 3, BodyText: "truth and renewal", Citation: "the Buddha, Iti"},
	}
	t.Run("nil terms returns input unchanged", func(t *testing.T) {
		if got := Filter(qs, nil); !reflect.DeepEqual(got, qs) {
			t.Errorf("Filter(nil terms) = %v, want %v", got, qs)
		}
	})
	t.Run("empty terms returns input unchanged", func(t *testing.T) {
		if got := Filter(qs, []string{}); !reflect.DeepEqual(got, qs) {
			t.Errorf("Filter(empty terms) = %v, want %v", got, qs)
		}
	})
	t.Run("keeps matches in order", func(t *testing.T) {
		got := Filter(qs, Terms("buddha"))
		want := []store.Quote{qs[0], qs[2]}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Filter(buddha) = %+v, want %+v", got, want)
		}
	})
	t.Run("or across terms", func(t *testing.T) {
		got := Filter(qs, Terms("forest truth"))
		want := []store.Quote{qs[1], qs[2]}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Filter(forest truth) = %+v, want %+v", got, want)
		}
	})
	t.Run("match in citation only", func(t *testing.T) {
		got := Filter(qs, Terms("iti"))
		if len(got) != 1 || got[0].ID != 3 {
			t.Errorf("Filter(iti) = %+v, want [ID=3] (citation-only match)", got)
		}
	})
	t.Run("no matches returns empty", func(t *testing.T) {
		if got := Filter(qs, Terms("zzz")); len(got) != 0 {
			t.Errorf("Filter(zzz) = %+v, want empty", got)
		}
	})
	t.Run("empty input no panic", func(t *testing.T) {
		if got := Filter(nil, Terms("x")); len(got) != 0 {
			t.Errorf("Filter(nil, x) = %+v, want empty", got)
		}
	})
}

// --- properties ---

func TestTermsIdempotent(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	words := []string{"buddha", "mn", "an", "truth", "forest", "Buddha", "MN", "", "  "}
	for i := 0; i < 200; i++ {
		q := randomQuery(rng, words)
		once := Terms(q)
		twice := Terms(strings.Join(once, " "))
		if !reflect.DeepEqual(once, twice) {
			t.Fatalf("Terms not idempotent for %q:\n once  = %v\n twice = %v", q, once, twice)
		}
	}
}

func TestFilterSupersetWhenTermsGrow(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	qs := []store.Quote{
		{ID: 1, BodyText: "buddha truth", Citation: ""},
		{ID: 2, BodyText: "forest monk", Citation: ""},
		{ID: 3, BodyText: "buddha forest", Citation: ""},
		{ID: 4, BodyText: "ocean", Citation: ""},
	}
	pool := []string{"buddha", "truth", "forest", "monk", "ocean"}
	for i := 0; i < 400; i++ {
		a := pickTerms(rng, pool, 1, 3)
		b := pickTerms(rng, pool, 1, 4)
		if !subset(toSet(a), toSet(b)) {
			continue
		}
		fa := ids(Filter(qs, a))
		fb := ids(Filter(qs, b))
		// more (or equal) terms => OR can only match more, never fewer.
		if !subsetInt(fa, fb) {
			t.Fatalf("Filter(%v)=%v not subset of Filter(%v)=%v", a, fa, b, fb)
		}
	}
}

func TestMatchOrderInvariant(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	terms := []string{"buddha", "truth", "forest"}
	text := "the buddha in a forest"
	for i := 0; i < 50; i++ {
		dup := append([]string(nil), terms...)
		rng.Shuffle(len(dup), func(i, j int) { dup[i], dup[j] = dup[j], dup[i] })
		if Match(text, dup) != true {
			t.Fatalf("Match(%q, %v) = false, want true", text, dup)
		}
	}
	if Match("xyz", []string{"alpha", "beta"}) || Match("xyz", []string{"beta", "alpha"}) {
		t.Fatal("negative match must be order-independent")
	}
}

// --- helpers ---

func randomQuery(rng *rand.Rand, words []string) string {
	n := 1 + rng.Intn(5)
	parts := make([]string, n)
	for i := range parts {
		parts[i] = words[rng.Intn(len(words))]
	}
	return strings.Join(parts, " ")
}

func pickTerms(rng *rand.Rand, from []string, min, max int) []string {
	n := min + rng.Intn(max-min+1)
	terms := make([]string, n)
	for i := range terms {
		terms[i] = from[rng.Intn(len(from))]
	}
	return terms
}

func toSet(s []string) map[string]bool {
	m := make(map[string]bool, len(s))
	for _, v := range s {
		m[strings.ToLower(v)] = true
	}
	return m
}

func subset(a, b map[string]bool) bool {
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

func ids(qs []store.Quote) map[int64]bool {
	m := make(map[int64]bool, len(qs))
	for _, q := range qs {
		m[q.ID] = true
	}
	return m
}

func subsetInt(a, b map[int64]bool) bool {
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}
