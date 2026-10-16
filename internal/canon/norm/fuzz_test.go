package norm

import (
	"testing"
	"unicode"
	"unicode/utf8"
)

func FuzzPaliTokenizer(f *testing.F) {
	for _, s := range []string{
		"evaṁ me sutaṁ", "Dhammā", "…pe…", "evaṁ pe sutaṁ", "an1.1-10",
		"āīūṭḍṅṇñḷṃṁṛ", "ĀĪŪṬḌṄṆÑḶṂṀṚ", "evā", "ṁ", "Ā",
		"", "plain ASCII 123", "İstanbul", "Ǆǅǆ", "αβγ", "中文字",
		"000נּ00",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		folded := FoldPali(s)
		if again := FoldPali(folded); again != folded {
			t.Fatalf("FoldPali not idempotent: FoldPali(%q) = %q, refold = %q", s, folded, again)
		}

		minLen := len(s) % 8
		tokens := TokenizePali(s, minLen)
		unfiltered := TokenizePali(s, 0)
		ui := 0
		for _, tok := range tokens {
			if tok == "" {
				t.Fatalf("TokenizePali(%q, %d) emitted an empty token", s, minLen)
			}
			if tok == "pe" {
				t.Fatalf("TokenizePali(%q, %d) emitted the elision marker", s, minLen)
			}
			if got := utf8.RuneCountInString(tok); got < minLen {
				t.Fatalf("token %q holds %d runes, below minLen %d", tok, got, minLen)
			}
			if FoldPali(tok) != tok {
				t.Fatalf("token %q of %q is not a fold fixpoint", tok, s)
			}
			for _, r := range tok {
				if r < utf8.RuneSelf {
					if r < 'a' || r > 'z' {
						t.Fatalf("token %q of %q holds non a-z ASCII %q", tok, s, r)
					}
					continue
				}
				if !unicode.IsLetter(r) && !unicode.IsMark(r) {
					t.Fatalf("token %q of %q holds non-letter non-mark %q", tok, s, r)
				}
				if r != unicode.ToLower(r) {
					t.Fatalf("token %q of %q holds unfolded cased rune %q", tok, s, r)
				}
				if _, foldedAway := paliFold[r]; foldedAway {
					t.Fatalf("token %q of %q holds unfoldable diacritic %q", tok, s, r)
				}
			}
			for ui < len(unfiltered) && unfiltered[ui] != tok {
				ui++
			}
			if ui == len(unfiltered) {
				t.Fatalf("token %q of %q missing from unfiltered pass %q", tok, s, unfiltered)
			}
			ui++
		}
	})
}
