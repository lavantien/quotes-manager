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
		{"khuddaka sub-book with fragment", "KN Dhp 5#2", "https://suttacentral.net/kndhp5#2"},
		{"main nikaya with fragment", "SN 42.8#3", "https://suttacentral.net/sn42.8#3"},
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
		"KN Dhp 5#2":               "KN Dhp 5#2",
		"pli-tv-bu-vb-pj1":         "pli-tv-bu-vb-pj1",
		"":                         "",
		"no sutta identifier here": "",
		"mn 22":                    "",
	}
	for in, want := range cases {
		if got := CanonicalSuttaID(in); got != want {
			t.Errorf("CanonicalSuttaID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMuidFromSuttaID(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"main nikaya", "MN 22", "mn22"},
		{"digha nikaya", "DN 16", "dn16"},
		{"samyutta with vaggas", "SN 12.63", "sn12.63"},
		{"anguttara with nipata", "AN 4.77", "an4.77"},
		{"range id", "AN 1.278-286", "an1.278-286"},
		{"nikaya fragment dropped from muid", "AN 5.34#7.9", "an5.34"},
		{"deep section", "DN 16.1.5", "dn16.1.5"},
		{"kn sub-book", "KN Snp 2.14", "snp2.14"},
		{"kn fragment dropped from muid", "KN Dhp 5#2", "dhp5"},
		{"kn itivuttaka", "KN Iti 25", "iti25"},
		{"kn udana", "KN Ud 8.8", "ud8.8"},
		{"kn dhammapada", "KN Dhp 20", "dhp20"},
		{"full vinaya", "pli-tv-bu-vb-pj1", "pli-tv-bu-vb-pj1"},
		{"full vinaya fragment dropped", "pli-tv-bu-vb-pj1#5.11.20", "pli-tv-bu-vb-pj1"},
		{"extracts id from citation", "the Buddha, MN 22", "mn22"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := MuidFromSuttaID(c.in)
			if err != nil {
				t.Fatalf("MuidFromSuttaID(%q): %v", c.in, err)
			}
			if got != c.want {
				t.Errorf("MuidFromSuttaID(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestMuidFromSuttaIDUnmappable(t *testing.T) {
	for _, in := range []string{
		"",
		"   ",
		"no sutta id here",
		"mn 22",
		"KN An 5",
		"KN Sn 12.63",
		"KN An 5#2",
		"Tv Vi Bu Pj1",
		"Tv Vi Bu Pj2",
	} {
		if got, err := MuidFromSuttaID(in); err == nil {
			t.Errorf("MuidFromSuttaID(%q) = %q, want an error", in, got)
		}
	}
}

func TestMuidFromSuttaIDExcludesAbbreviatedVinaya(t *testing.T) {
	for _, in := range []string{"Tv Vi Bu Pj1", "Tv Vi Bu Pj2"} {
		muid, err := MuidFromSuttaID(in)
		if err == nil {
			t.Errorf("MuidFromSuttaID(%q) = %q, want the abbreviated vinaya family excluded from muid mapping", in, muid)
		}
		if got := SuttaURL(in); got != "https://suttacentral.net/"+compactLower(in) {
			t.Errorf("SuttaURL(%q) = %q, want the pre-merge 1.x fallback format", in, got)
		}
	}
}

func TestDisplaySuttaID(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"main nikaya", "mn22", "MN 22"},
		{"range id", "an1.278-286", "AN 1.278-286"},
		{"fragment kept", "an5.34#7.9", "AN 5.34#7.9"},
		{"deep section", "dn16.1.5", "DN 16.1.5"},
		{"kn sub-book", "snp2.14", "KN Snp 2.14"},
		{"kn itivuttaka", "iti25", "KN Iti 25"},
		{"kn udana", "ud8.8", "KN Ud 8.8"},
		{"kn dhammapada", "dhp20", "KN Dhp 20"},
		{"full vinaya", "pli-tv-bu-vb-pj1", "pli-tv-bu-vb-pj1"},
		{"abbreviated vinaya", "tvvibupj1", "Tv Vi Bu Pj1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := DisplaySuttaID(c.in)
			if err != nil {
				t.Fatalf("DisplaySuttaID(%q): %v", c.in, err)
			}
			if got != c.want {
				t.Errorf("DisplaySuttaID(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestDisplaySuttaIDUnmappable(t *testing.T) {
	for _, in := range []string{
		"",
		"MN1",
		"Snp2.14",
		"abc",
		"1.2",
		"22mn",
	} {
		if got, err := DisplaySuttaID(in); err == nil {
			t.Errorf("DisplaySuttaID(%q) = %q, want an error", in, got)
		}
	}
}

func TestSuttaIDMuidRoundTrip(t *testing.T) {
	familyReps := []struct {
		family string
		id     string
	}{
		{"dn", "DN 16"},
		{"mn", "MN 22"},
		{"sn", "SN 12.63"},
		{"an", "AN 4.77"},
		{"an range", "AN 1.278-286"},
		{"dn deep", "DN 16.1.5"},
		{"kn snp", "KN Snp 2.14"},
		{"kn iti", "KN Iti 25"},
		{"kn ud", "KN Ud 8.8"},
		{"kn dhp", "KN Dhp 20"},
		{"vinaya full", "pli-tv-bu-vb-pj1"},
	}
	for _, c := range familyReps {
		t.Run(c.family, func(t *testing.T) {
			muid, err := MuidFromSuttaID(c.id)
			if err != nil {
				t.Fatalf("MuidFromSuttaID(%q): %v", c.id, err)
			}
			got, err := DisplaySuttaID(muid)
			if err != nil {
				t.Fatalf("DisplaySuttaID(%q): %v", muid, err)
			}
			if got != c.id {
				t.Errorf("round trip %q -> %q -> %q", c.id, muid, got)
			}
		})
	}
}

func TestSuttaIDMuidRoundTripDropsVinayaFragment(t *testing.T) {
	muid, err := MuidFromSuttaID("pli-tv-bu-vb-pj1#5.11.20")
	if err != nil {
		t.Fatal(err)
	}
	if muid != "pli-tv-bu-vb-pj1" {
		t.Fatalf("muid = %q, want the fragment dropped", muid)
	}
	got, err := DisplaySuttaID(muid)
	if err != nil {
		t.Fatal(err)
	}
	if got != "pli-tv-bu-vb-pj1" {
		t.Fatalf("display = %q, want the bare full vinaya id", got)
	}
}

func TestSplitCitationReverseIteration(t *testing.T) {
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
