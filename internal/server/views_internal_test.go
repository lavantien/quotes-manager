package server

import (
	"strings"
	"testing"
)

func TestBodyExcerpt(t *testing.T) {
	// Non-truncating paths: short or single-line text comes back as-is.
	for _, c := range []struct{ name, text, want string }{
		{"empty", "", ""},
		{"short single line", "a short line", "a short line"},
		{"multi-line uses first line when short", "first line here\nsecond line", "first line here"},
		{"trims surrounding whitespace", "   padded short   ", "padded short"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := bodyExcerpt(c.text); got != c.want {
				t.Errorf("bodyExcerpt(%q) = %q, want %q", c.text, got, c.want)
			}
		})
	}
	// Truncating paths: result is the first excerptRunes runes of the first
	// (trimmed) line, plus an ellipsis.
	for _, c := range []struct{ name, text string }{
		{"long single line", "this is a much longer single line that exceeds the rune limit"},
		{"multi-line with long first line", "a long first line that surely exceeds the cutoff\nsecond"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := bodyExcerpt(c.text)
			first := c.text
			if i := strings.IndexByte(first, '\n'); i >= 0 {
				first = first[:i]
			}
			first = strings.TrimSpace(first)
			want := string([]rune(first)[:excerptRunes]) + "…"
			if got != want {
				t.Errorf("bodyExcerpt(%q) = %q, want %q", c.text, got, want)
			}
		})
	}
}
