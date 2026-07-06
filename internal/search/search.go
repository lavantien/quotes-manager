// Package search filters quotes by a free-text query. Parse recognizes the AND
// of required terms, quoted phrase substrings, and -negation; Match applies the
// rule; Filter narrows a []store.Quote. It is a pure helper (no I/O, no
// templates).
package search

import (
	"strings"
	"unicode"

	"github.com/lavantien/quotes-manager/internal/store"
)

// Query is a parsed search query. Pos are required substrings (bare words and
// quoted phrases — all must appear); Neg are forbidden substrings (none may
// appear). A zero Query means "no active search": Filter is a no-op.
type Query struct {
	Pos []string
	Neg []string
}

// IsZero reports whether the query is empty (no active search).
func (q Query) IsZero() bool {
	return len(q.Pos) == 0 && len(q.Neg) == 0
}

// Parse tokenizes a query. Bare words are required (AND); a leading '-' marks a
// term or quoted phrase as negation; double quotes group a multi-word phrase.
// Tokens are lowercased and deduped within Pos and Neg, preserving first-seen
// order.
func Parse(s string) Query {
	var out Query
	seenPos := map[string]bool{}
	seenNeg := map[string]bool{}
	for _, tok := range tokenize(s) {
		if tok.neg {
			if !seenNeg[tok.text] {
				seenNeg[tok.text] = true
				out.Neg = append(out.Neg, tok.text)
			}
		} else if !seenPos[tok.text] {
			seenPos[tok.text] = true
			out.Pos = append(out.Pos, tok.text)
		}
	}
	return out
}

// Match reports whether text satisfies q (case-insensitive): every Pos must be
// a substring and no Neg may be a substring. A zero Query matches everything.
func Match(text string, q Query) bool {
	lt := strings.ToLower(text)
	for _, p := range q.Pos {
		if !strings.Contains(lt, p) {
			return false
		}
	}
	for _, n := range q.Neg {
		if strings.Contains(lt, n) {
			return false
		}
	}
	return true
}

// Filter returns the quotes whose BodyText+Citation satisfy q, in their
// original order. A zero Query returns qs unchanged (a search with no query is
// a no-op).
func Filter(qs []store.Quote, q Query) []store.Quote {
	if q.IsZero() {
		return qs
	}
	out := make([]store.Quote, 0, len(qs))
	for _, qq := range qs {
		if Match(qq.BodyText+"\n"+qq.Citation, q) {
			out = append(out, qq)
		}
	}
	return out
}

// HighlightTerms returns the substrings to highlight for q — the Pos terms
// (words and phrases), never the Neg terms — so the renderer's term highlighter
// reflects the active query without having to understand its syntax.
func HighlightTerms(q Query) []string {
	if len(q.Pos) == 0 {
		return nil
	}
	out := make([]string, len(q.Pos))
	copy(out, q.Pos)
	return out
}

// token is a single parsed query token: its match text (lowercased) and whether
// it was negated with a leading '-'.
type token struct {
	text string
	neg  bool
}

// tokenize scans s into tokens, honoring double-quoted phrases and a leading
// '-' negation marker. Quoting lets a phrase carry spaces; everything else is
// split on whitespace.
func tokenize(s string) []token {
	var out []token
	r := []rune(s)
	i := 0
	for i < len(r) {
		for i < len(r) && unicode.IsSpace(r[i]) {
			i++
		}
		if i >= len(r) {
			break
		}
		neg := false
		if r[i] == '-' {
			neg = true
			i++
		}
		var b strings.Builder
		if i < len(r) && r[i] == '"' {
			i++ // opening quote
			for i < len(r) && r[i] != '"' {
				b.WriteRune(r[i])
				i++
			}
			if i < len(r) {
				i++ // closing quote
			}
		} else {
			for i < len(r) && !unicode.IsSpace(r[i]) {
				b.WriteRune(r[i])
				i++
			}
		}
		if text := strings.ToLower(strings.TrimSpace(b.String())); text != "" {
			out = append(out, token{text: text, neg: neg})
		}
	}
	return out
}
