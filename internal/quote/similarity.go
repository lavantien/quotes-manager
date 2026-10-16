package quote

import (
	"sort"
	"strings"
	"unicode"
)

const DefaultDuplicateThreshold = 0.8

type DupItem struct {
	ID   int64
	Text string
}

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

func Words(s string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, w := range strings.Fields(CleanText(s)) {
		set[w] = struct{}{}
	}
	return set
}

func Jaccard(a, b map[string]struct{}) float64 {
	if len(a)+len(b) == 0 {
		return 0
	}
	inter := 0
	for w := range a {
		if _, ok := b[w]; ok {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	return float64(inter) / float64(union)
}

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
		u.parent[x] = u.parent[u.parent[x]]
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
