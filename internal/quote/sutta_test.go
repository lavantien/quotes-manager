package quote

import "testing"

func TestSuttaURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"main nikaya", "MN 22", "https://suttacentral.net/mn22"},
		{"with range", "AN 4.180", "https://suttacentral.net/an4.180"},
		{"khuddaka sub-book", "KN Snp 2.14", "https://suttacentral.net/knsnp2.14"},
		{"full vinaya", "pli-tv-bu-vb-pj1#5.11.20", "https://suttacentral.net/pli-tv-bu-vb-pj1#5.11.20"},
		{"abbreviated vinaya", "Tv Vi Bu Pj1", "https://suttacentral.net/tvvibupj1"},
		{"extracts id from citation", "the Buddha, MN 22", "https://suttacentral.net/mn22"},
		{"surrounding whitespace", "  MN 22  ", "https://suttacentral.net/mn22"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := SuttaURL(c.in); got != c.want {
				t.Errorf("SuttaURL(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestSuttaURLEdgeCases(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"DN nikaya", "DN 16", "https://suttacentral.net/dn16"},
		{"empty", "", "https://suttacentral.net/"},
		{"no match", "no sutta id here", "https://suttacentral.net/"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := SuttaURL(c.in); got != c.want {
				t.Errorf("SuttaURL(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestCanonicalSuttaIDMore(t *testing.T) {
	cases := map[string]string{
		"DN 16":                    "DN 16",
		"DN 16.1.5":                "DN 16.1.5",
		"These last words, DN 16":  "DN 16",
		"AN 5.34#7.9":              "AN 5.34#7.9",
		"KN Iti 25":                "KN Iti 25",
		"pli-tv-bu-vb-pj1":         "pli-tv-bu-vb-pj1",
		"":                         "",
		"no sutta identifier here": "",
		"mn 22":                    "", // regex is case-sensitive
	}
	for in, want := range cases {
		if got := CanonicalSuttaID(in); got != want {
			t.Errorf("CanonicalSuttaID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSplitCitationReverseIteration(t *testing.T) {
	// Multiple " - " separators where the rightmost tail is not a citation: the
	// loop walks left, finds no clean citation, and reports not-ok.
	if _, _, ok := splitCitation(`text - MN 8 - trailing non-citation`); ok {
		t.Errorf("expected ok=false for multi-dash non-citation tail")
	}
}

func TestCleanCitation(t *testing.T) {
	cases := map[string]string{
		"MN 22":                 "MN 22",
		"MN 22 ( https://x/y )": "MN 22",
		"   MN 22   ":           "MN 22",
		"the Buddha, MN 22":     "the Buddha, MN 22",
	}
	for in, want := range cases {
		if got := cleanCitation(in); got != want {
			t.Errorf("cleanCitation(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEnsureAttributionEdgeCases(t *testing.T) {
	cases := map[string]string{
		"":    "the Buddha, ",
		"   ": "the Buddha, ",
	}
	for in, want := range cases {
		if got := ensureAttribution(in); got != want {
			t.Errorf("ensureAttribution(%q) = %q, want %q", in, got, want)
		}
	}
}
