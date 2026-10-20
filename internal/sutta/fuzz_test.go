package sutta

import "testing"

func FuzzParseTables(f *testing.F) {
	f.Add("**I. A.**\n1. [MN 1](https://suttacentral.net/mn1/en/sujato?lang=en&amp;layout=plain)\n")
	f.Add("**II. B** ([AN 2.1-10](https://suttacentral.net/an2.1-10))\n")
	f.Add("**III. C.**\n[StNp 4.2](https://www.dhammatalks.org/suttas/KN/Snp/StNp4_2.html)\n")
	f.Add("[xy99](https://suttacentral.net/xy99)")
	f.Add("**IV. D.**\n[v](https://suttacentral.net/pli-tv-bu-vb-pj3#1.1)\n")
	f.Add("")
	f.Add("**I. ")
	f.Fuzz(func(t *testing.T, content string) {
		tab, err := ParseFile("fuzz.md", content)
		if err != nil {
			return
		}
		seenTable := make(map[string]bool)
		for _, g := range tab.Groups {
			if g.Name == "" {
				t.Fatalf("unnamed group rendered from %q", content)
			}
			seenGroup := make(map[string]bool)
			for _, s := range g.Suttas {
				if s.Muid == "" || s.Display == "" || s.Link == "" {
					t.Fatalf("empty sutta field %+v from %q", s, content)
				}
				if seenGroup[s.Muid] {
					t.Fatalf("duplicate muid %s within group from %q", s.Muid, content)
				}
				seenGroup[s.Muid] = true
				if seenTable[s.Muid] {
					t.Fatalf("duplicate muid %s within table from %q", s.Muid, content)
				}
				seenTable[s.Muid] = true
			}
		}
	})
}
