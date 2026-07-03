package quote

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

func TestCleanText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"lowercases", "Hello WORLD", "hello world"},
		{"strips punctuation", "Hello, world! How's it going?", "hello world hows it going"},
		{"keeps digits", "MN 22 verse 3:14", "mn 22 verse 314"},
		{"keeps whitespace between words", "one, two; three.", "one two three"},
		{"does not collapse whitespace", "a  b", "a  b"},
		{"strips punctuation joined to words", "a!b?c", "abc"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := CleanText(tc.in); got != tc.want {
				t.Errorf("CleanText(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestWords(t *testing.T) {
	set := Words("Hello, hello world! WORLD")
	want := map[string]struct{}{"hello": {}, "world": {}}
	if len(set) != len(want) {
		t.Fatalf("Words returned %d unique words, want %d: %v", len(set), len(want), set)
	}
	for w := range want {
		if _, ok := set[w]; !ok {
			t.Errorf("Words missing %q", w)
		}
	}
}

func TestJaccard(t *testing.T) {
	set := func(words ...string) map[string]struct{} {
		m := make(map[string]struct{}, len(words))
		for _, w := range words {
			m[w] = struct{}{}
		}
		return m
	}
	tests := []struct {
		name string
		a, b map[string]struct{}
		want float64
	}{
		{"identical non-empty", set("a", "b", "c"), set("a", "b", "c"), 1.0},
		{"disjoint", set("a", "b"), set("c", "d"), 0.0},
		{"subset half", set("a", "b"), set("a", "b", "c", "d"), 0.5},
		{"both empty", set(), set(), 0.0},
		{"one empty", set("a", "b"), set(), 0.0},
		{"four of five", set("a", "b", "c", "d"), set("a", "b", "c", "d", "e"), 0.8},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Jaccard(tc.a, tc.b)
			if abs(got-tc.want) > 1e-9 {
				t.Errorf("Jaccard = %v, want %v", got, tc.want)
			}
			if rev := Jaccard(tc.b, tc.a); abs(got-rev) > 1e-12 {
				t.Errorf("Jaccard not symmetric: %v vs %v", got, rev)
			}
		})
	}
}

func TestGroupDuplicates(t *testing.T) {
	mk := func(id int64, text string) DupItem { return DupItem{ID: id, Text: text} }
	items := func(xs ...DupItem) []DupItem { return xs }

	t.Run("empty input", func(t *testing.T) {
		if got := GroupDuplicates(nil, DefaultDuplicateThreshold); len(got) != 0 {
			t.Errorf("expected no groups, got %v", got)
		}
	})

	t.Run("singletons excluded", func(t *testing.T) {
		is := items(mk(1, "alpha beta"), mk(2, "completely different words here"))
		if got := GroupDuplicates(is, DefaultDuplicateThreshold); len(got) != 0 {
			t.Errorf("expected no groups for non-duplicates, got %v", got)
		}
	})

	t.Run("exact pair grouped in input order", func(t *testing.T) {
		is := items(mk(1, "the quick brown fox"), mk(2, "the quick brown fox"))
		got := GroupDuplicates(is, DefaultDuplicateThreshold)
		if len(got) != 1 || len(got[0]) != 2 {
			t.Fatalf("expected one group of 2, got %v", got)
		}
		if !reflect.DeepEqual(got[0], []int64{1, 2}) {
			t.Errorf("expected [1 2], got %v", got[0])
		}
	})

	t.Run("threshold boundary not grouped", func(t *testing.T) {
		// Jaccard exactly 0.8 (4 shared / 5 union) must NOT group (> threshold).
		is := items(mk(1, "a b c d"), mk(2, "a b c d e"))
		if got := GroupDuplicates(is, DefaultDuplicateThreshold); len(got) != 0 {
			t.Errorf("expected no group at Jaccard==threshold, got %v", got)
		}
	})

	t.Run("above threshold grouped", func(t *testing.T) {
		// 5 shared / 6 union ~= 0.833 > 0.8.
		is := items(mk(1, "a b c d e"), mk(2, "a b c d e f"))
		got := GroupDuplicates(is, DefaultDuplicateThreshold)
		if len(got) != 1 || len(got[0]) != 2 {
			t.Fatalf("expected one group of 2, got %v", got)
		}
	})

	t.Run("transitive chaining via a bridge", func(t *testing.T) {
		// A~B and B~C are > 0.8; A~C is ~0.667. Union-find chains all three.
		is := items(
			mk(1, "w1 w2 w3 w4 w5 w6 w7 w8 w9 a"), // A
			mk(2, "w1 w2 w3 w4 w5 w6 w7 w8 w9 b"), // B (bridge)
			mk(3, "w1 w2 w3 w4 w5 w6 w7 w8 b c"),  // C
		)
		got := GroupDuplicates(is, DefaultDuplicateThreshold)
		if len(got) != 1 || len(got[0]) != 3 {
			t.Fatalf("expected one chained group of 3, got %v", got)
		}
		if !reflect.DeepEqual(got[0], []int64{1, 2, 3}) {
			t.Errorf("expected [1 2 3], got %v", got[0])
		}
	})

	t.Run("MN 22 seed trio collapses to one group", func(t *testing.T) {
		is := items(
			mk(1, "Bhikkhus, that one can engage in sexual pleasures without sexual desires, without perceptions of sexual desire, without thoughts of sexual desire: that is impossible."),
			mk(2, "Mendicants, that one can engage in sexual pleasures without sexual desires, without perceptions of sexual desire, without thoughts of sexual desire: that is impossible."),
			mk(3, "Bhikkhus, that one can engage in sensual pleasures without sensual desires, without perceptions of sensual desire, without thoughts of sensual desire: that is impossible."),
		)
		got := GroupDuplicates(is, DefaultDuplicateThreshold)
		if len(got) != 1 || len(got[0]) != 3 {
			t.Fatalf("expected one group of 3 for the MN 22 trio, got %v", got)
		}
		if !reflect.DeepEqual(got[0], []int64{1, 2, 3}) {
			t.Errorf("expected [1 2 3], got %v", got[0])
		}
	})

	t.Run("only duplicated ids surfaced", func(t *testing.T) {
		is := items(
			mk(1, "one two three four five six seven eight nine ten"),
			mk(2, "one two three four five six seven eight nine eleven"),
			mk(3, "completely different unique words here"),
		)
		got := GroupDuplicates(is, DefaultDuplicateThreshold)
		if len(got) != 1 || len(got[0]) != 2 {
			t.Fatalf("expected one group of 2, got %v", got)
		}
		for _, ids := range got {
			for _, id := range ids {
				if id == 3 {
					t.Errorf("singleton id 3 must not appear in any group")
				}
			}
		}
	})
}

func TestJaccardProperty(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	vocab := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	randSet := func() map[string]struct{} {
		n := 1 + r.Intn(len(vocab))
		m := make(map[string]struct{}, n)
		for i := 0; i < n; i++ {
			m[vocab[r.Intn(len(vocab))]] = struct{}{}
		}
		return m
	}
	for i := 0; i < 2000; i++ {
		a, b := randSet(), randSet()
		j := Jaccard(a, b)
		if j < 0 || j > 1 {
			t.Fatalf("Jaccard %v out of [0,1]", j)
		}
		if abs(Jaccard(b, a)-j) > 1e-12 {
			t.Fatalf("Jaccard not symmetric")
		}
	}
}

func TestGroupDuplicatesPartitionInvariant(t *testing.T) {
	r := rand.New(rand.NewSource(2))
	for iter := 0; iter < 200; iter++ {
		n := 2 + r.Intn(15)
		items := make([]DupItem, n)
		for i := range items {
			items[i] = DupItem{ID: int64(i + 1), Text: randomText(r)}
		}
		groups := GroupDuplicates(items, DefaultDuplicateThreshold)
		seen := make(map[int64]bool)
		for _, g := range groups {
			if len(g) < 2 {
				t.Fatalf("group with fewer than 2 members: %v", g)
			}
			for _, id := range g {
				if seen[id] {
					t.Fatalf("id %d in more than one group", id)
				}
				seen[id] = true
			}
		}
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func randomText(r *rand.Rand) string {
	vocab := []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta", "eta", "theta"}
	n := 1 + r.Intn(6)
	var b strings.Builder
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(vocab[r.Intn(len(vocab))])
	}
	return b.String()
}
