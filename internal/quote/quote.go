// Package quote extracts, normalizes, and renders sutta quotes from the
// raw essay dumps into a single canonical format.
package quote

import (
	"slices"
	"sort"
	"strings"
	"unicode/utf8"
)

// Quote is a single normalized sutta quote.
type Quote struct {
	SuttaID  string   // canonical id, e.g. "MN 22", "KN Snp 2.14"
	Citation string   // full kept citation, e.g. "the Buddha, MN 22"
	Passages []string // normalized passage lines (smart quotes preserved)
	Sources  []string // dump files this quote appeared in
}

func newQuote(sutta, citation string, passages []string, source string) *Quote {
	return &Quote{
		SuttaID:  sutta,
		Citation: citation,
		Passages: passages,
		Sources:  []string{source},
	}
}

// New constructs a Quote for callers outside the parser (e.g. the web form).
// Sources is empty; suttaID and citation are taken as-is, so callers should
// pre-normalize them (CanonicalSuttaID, default attribution) before calling.
func New(suttaID, citation string, passages []string) *Quote {
	return &Quote{SuttaID: suttaID, Citation: citation, Passages: passages}
}

// BodyText is the plain passages joined by newlines (no markdown, no citation).
func (q *Quote) BodyText() string {
	return strings.Join(q.Passages, "\n")
}

// LineCount is the number of passage lines.
func (q *Quote) LineCount() int { return len(q.Passages) }

// CharCount is the rune count of all passages concatenated (the sort key).
func (q *Quote) CharCount() int {
	n := 0
	for _, p := range q.Passages {
		n += utf8.RuneCountInString(p)
	}
	return n
}

// dedupKey is the normalized passage text used to detect duplicates.
func (q *Quote) dedupKey() string {
	var b strings.Builder
	for _, p := range q.Passages {
		b.WriteString(collapseWhitespace(p))
	}
	return b.String()
}

func collapseWhitespace(s string) string {
	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		switch r {
		case ' ', '\t', '\n', '\r', ' ':
			if !prevSpace {
				b.WriteRune(' ')
			}
			prevSpace = true
		default:
			b.WriteRune(r)
			prevSpace = false
		}
	}
	return b.String()
}

// Dedup collapses quotes with identical passage text, merging their source lists
// and keeping the first-seen citation / sutta id.
func Dedup(qs []*Quote) []*Quote {
	seen := make(map[string]int)
	out := make([]*Quote, 0, len(qs))
	for _, q := range qs {
		key := q.dedupKey()
		if idx, ok := seen[key]; ok {
			out[idx].mergeSources(q.Sources...)
			continue
		}
		seen[key] = len(out)
		out = append(out, q)
	}
	return out
}

func (q *Quote) mergeSources(srcs ...string) {
	for _, s := range srcs {
		if !slices.Contains(q.Sources, s) {
			q.Sources = append(q.Sources, s)
		}
	}
}

// SortByCharCount sorts shortest-first (stable, with deterministic tie-breakers
// on sutta id and body text).
func SortByCharCount(qs []*Quote) {
	sort.SliceStable(qs, func(i, j int) bool {
		a, b := qs[i], qs[j]
		if a.CharCount() != b.CharCount() {
			return a.CharCount() < b.CharCount()
		}
		if a.SuttaID != b.SuttaID {
			return a.SuttaID < b.SuttaID
		}
		return a.BodyText() < b.BodyText()
	})
}
