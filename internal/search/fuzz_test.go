package search

import (
	"reflect"
	"strings"
	"testing"
)

func FuzzSearchParse(f *testing.F) {
	for _, s := range []string{
		"dukkha",
		`"multi word" phrase`,
		"-negated",
		`-"quoted neg"`,
		"--double",
		`""`,
		`-"-`,
		`"unclosed phrase`,
		"Mn 22",
		"sabbe sattā",
		"sabbe sattā",
		"\ufeffbom",
		"   ",
		"- ",
		"-\"- x y -z",
		strings.Repeat("term ", 32) + "-tail",
		"",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		q := Parse(s)
		if again := Parse(s); !reflect.DeepEqual(q, again) {
			t.Fatalf("Parse not deterministic on %q: %+v vs %+v", s, q, again)
		}
		low := strings.ToLower(s)
		for _, term := range append(append([]string{}, q.Pos...), q.Neg...) {
			if term == "" {
				t.Fatalf("empty term from %q: %+v", s, q)
			}
			if term != strings.ToLower(term) {
				t.Fatalf("term %q not lowercased from %q", term, s)
			}
			if !strings.Contains(low, term) {
				t.Fatalf("term %q not a substring of the lowercased query %q", term, low)
			}
		}
		if want := len(q.Neg) == 0; Match(low, q) != want {
			t.Fatalf("Match(lower(query)) = %v, want %v for %q and %+v", !want, want, s, q)
		}
	})
}
