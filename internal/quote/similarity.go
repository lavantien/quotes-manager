package quote

import (
	"sort"
	"strings"
	"unicode"
)

// DefaultDuplicateThreshold is the word-level Jaccard score above which two
// quotes are treated as near-duplicates.
const DefaultDuplicateThreshold = 0.8

// DupItem pairs a stable identifier with the text used to judge near-duplicates.
type DupItem struct {
	ID   int64
	Text string
}

// CleanText lowercases s and drops every rune that is not a letter, digit, or
// whitespace, leaving text ready for word-level comparison. Whitespace is
// preserved (not collapsed) so callers can tokenize with strings.Fields.
func CleanText(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Words returns the set of unique words in s after cleaning (lowercase, no
// punctuation). The result has no duplicates and no ordering.
func Words(s string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, w := range strings.Fields(CleanText(s)) {
		set[w] = struct{}{}
	}
	return set
}

// Jaccard returns the word-level Jaccard similarity |a∩b| / |a∪b| of two word
// sets. It returns 0 when both sets are empty (so empty input never pairs).
func Jaccard(a, b map[string]struct{}) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	// Iterate the smaller set to count the intersection.
	if len(b) < len(a) {
		a, b = b, a
	}
	inter := 0
	for w := range a {
		if _, ok := b[w]; ok {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

// GroupDuplicates returns groups of item IDs whose text is near-duplicate:
// two items are joined when their word-level Jaccard similarity is strictly
// greater than threshold, and groups grow transitively via a disjoint set
// (union-find). Only groups with at least two members are returned; a quote
// with no near-duplicate is never surfaced. Each group is sorted by input
// order (so the first id is the representative), and groups are returned in
// representative order.
func GroupDuplicates(items []DupItem, threshold float64) [][]int64 {
	n := len(items)
	sets := make([]map[string]struct{}, n)
	for i, it := range items {
		sets[i] = Words(it.Text)
	}
	uf := newUnionFind(n)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if Jaccard(sets[i], sets[j]) > threshold {
				uf.union(i, j)
			}
		}
	}

	members := make(map[int][]int64)
	repIdx := make(map[int]int)
	for i, it := range items {
		root := uf.find(i)
		if _, ok := repIdx[root]; !ok {
			repIdx[root] = i
		}
		members[root] = append(members[root], it.ID)
	}

	type group struct {
		rep int
		ids []int64
	}
	kept := make([]group, 0, len(members))
	for root, ids := range members {
		if len(ids) >= 2 {
			kept = append(kept, group{rep: repIdx[root], ids: ids})
		}
	}
	sort.Slice(kept, func(i, j int) bool { return kept[i].rep < kept[j].rep })

	out := make([][]int64, len(kept))
	for i, g := range kept {
		out[i] = g.ids
	}
	return out
}

// unionFind is a standard disjoint set with path compression and union by
// arbitrary parent assignment, used only by GroupDuplicates.
type unionFind struct {
	parent []int
}

func newUnionFind(n int) *unionFind {
	p := make([]int, n)
	for i := range p {
		p[i] = i
	}
	return &unionFind{parent: p}
}

func (u *unionFind) find(x int) int {
	for u.parent[x] != x {
		u.parent[x] = u.parent[u.parent[x]] // path compression
		x = u.parent[x]
	}
	return x
}

func (u *unionFind) union(a, b int) {
	ra, rb := u.find(a), u.find(b)
	if ra != rb {
		u.parent[ra] = rb
	}
}
