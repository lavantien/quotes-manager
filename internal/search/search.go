// Package search filters quotes by free-text query. It is a pure (no I/O, no
// templates) helper: Terms parses a query, Match applies the OR/substring rule,
// and Filter narrows a []store.Quote to the matching quotes.
package search

import (
	"strings"

	"github.com/lavantien/quotes-manager/internal/store"
)

// Terms parses a raw query into lowercased, trimmed, deduped search terms
// (whitespace-separated). It returns nil for an empty/whitespace query so a
// missing search and an active search stay distinguishable: nil terms means
// Filter is a no-op (every quote shown).
func Terms(q string) []string {
	fields := strings.Fields(strings.ToLower(q))
	if len(fields) == 0 {
		return nil
	}
	out := make([]string, 0, len(fields))
	seen := make(map[string]bool, len(fields))
	for _, f := range fields {
		if !seen[f] {
			seen[f] = true
			out = append(out, f)
		}
	}
	return out
}

// Match reports whether any term appears (case-insensitively) as a substring of
// text. An empty terms slice never matches. Terms are lowercased defensively so
// Match is correct even when called with un-normalized input.
func Match(text string, terms []string) bool {
	if len(terms) == 0 {
		return false
	}
	lt := strings.ToLower(text)
	for _, t := range terms {
		if strings.Contains(lt, strings.ToLower(t)) {
			return true
		}
	}
	return false
}

// Filter returns the quotes whose BodyText+Citation match the given terms under
// the same OR/substring rule as Match, in their original order. With nil/empty
// terms it returns qs unchanged (a search with no query is a no-op).
func Filter(qs []store.Quote, terms []string) []store.Quote {
	if len(terms) == 0 {
		return qs
	}
	out := make([]store.Quote, 0, len(qs))
	for _, q := range qs {
		if Match(q.BodyText+"\n"+q.Citation, terms) {
			out = append(out, q)
		}
	}
	return out
}
