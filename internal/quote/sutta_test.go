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
