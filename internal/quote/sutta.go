package quote

import (
	"regexp"
	"strings"
)

// suttaIDPat matches a single sutta identifier (non-capturing alternation):
//   - four main nikayas:   MN 22, AN 1.278-286, SN 12.63, AN 5.34#7.9
//   - Khuddaka sub-books:  KN Snp 2.14, KN Iti 25, KN Ud 8.8
//   - full Vinaya id:      pli-tv-bu-vb-pj1#5.11.20
//   - abbreviated Vinaya:  Tv Vi Bu Pj1
const suttaIDPat = `(?:` +
	`(?:DN|MN|AN|SN)\s+\d+(?:[.-]\d+)*(?:#\d+(?:\.\d+)*)?` +
	`|KN\s+[A-Za-z]+\s+\d+(?:[.-]\d+)*` +
	`|pli-tv[A-Za-z0-9-]+(?:#\d+(?:\.\d+)*)?` +
	`|Tv\s+Vi\s+Bu\s+Pj\d` +
	`)`

var (
	suttaIDRe = regexp.MustCompile(suttaIDPat)

	// citationRe matches a clean citation tail: [attribution,]* SUTTA [( url )].
	citationRe = regexp.MustCompile(`^\s*(?:[^(),]+,\s+)*` + suttaIDPat + `(?:\s*\([^)]*\))?\s*$`)

	// headerRe matches a lone "SUTTA:" line (header-cited quote).
	headerRe = regexp.MustCompile(`^` + suttaIDPat + `:\s*$`)

	// sepRe matches the " - " separating a passage from its inline citation.
	sepRe = regexp.MustCompile(`\s+-\s+`)

	// urlParensRe matches a trailing "( ... )" (e.g. a suttacentral URL).
	urlParensRe = regexp.MustCompile(`\s*\([^)]*\)\s*$`)
)

// CanonicalSuttaID returns the sutta identifier embedded in s.
func CanonicalSuttaID(s string) string {
	return strings.TrimSpace(suttaIDRe.FindString(s))
}

// cleanCitation drops a trailing URL-in-parens and trims whitespace.
func cleanCitation(s string) string {
	return strings.TrimSpace(urlParensRe.ReplaceAllString(strings.TrimSpace(s), ""))
}

// ensureAttribution prefixes "the Buddha, " when a citation carries no
// attribution — i.e. it is just the sutta id with nothing before it.
func ensureAttribution(citation string) string {
	c := strings.TrimSpace(citation)
	if c == CanonicalSuttaID(c) {
		return "the Buddha, " + c
	}
	return c
}

// splitCitation splits a line into (passage, citation, ok). It locates the
// rightmost " - " whose tail is a clean citation; if none qualifies the line is
// not an inline-citation line.
func splitCitation(line string) (passage, citation string, ok bool) {
	idxs := sepRe.FindAllStringIndex(line, -1)
	for i := len(idxs) - 1; i >= 0; i-- {
		tail := line[idxs[i][1]:]
		if citationRe.MatchString(tail) {
			return line[:idxs[i][0]], cleanCitation(tail), true
		}
	}
	return line, "", false
}
