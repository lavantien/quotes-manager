package norm

import (
	"reflect"
	"strings"
	"testing"
)

func TestFoldPaliCollapsesStackedMarksAndCGJ(t *testing.T) {
	if got := FoldPali("a" + strings.Repeat("̄", 31)); got != "a" {
		t.Fatalf("31 stacked macrons = %q, want a", got)
	}
	if got := FoldPali("a" + strings.Repeat("̄", 32)); got != "a" {
		t.Fatalf("32 stacked macrons = %q, want a", got)
	}
	if got := FoldPali("a" + strings.Repeat("̄", 31)); FoldPali(got) != got {
		t.Fatal("stacked-mark fold must reach its fixpoint")
	}
	if got := FoldPali("a͏b"); got != "ab" {
		t.Fatalf("literal CGJ = %q, want ab", got)
	}
	if got := FoldPali("ñ"); got != "n" {
		t.Fatalf("decomposed tilde = %q, want n", got)
	}
}

func TestFoldPaliDropsNonPaliStrayMarks(t *testing.T) {
	if got := FoldPali("da̖o"); got != "dao" {
		t.Fatalf("stray grave below = %q, want dao", got)
	}
}

func TestTokenizePaliMarksJoinTheRun(t *testing.T) {
	if got := TokenizePali("evāme", 1); len(got) != 1 || got[0] != "evame" {
		t.Fatalf("interior macron = %v, want [evame]", got)
	}
	if got := TokenizePali("dā̖ḍo", 1); len(got) != 1 || got[0] != "dado" {
		t.Fatalf("blocked composition = %v, want [dado]", got)
	}
	if got := TokenizePali("eva ̈me", 1); len(got) != 2 || got[0] != "eva" || got[1] != "me" {
		t.Fatalf("space-separated stays split: %v", got)
	}
	if got := TokenizePali("̄̆", 1); len(got) != 0 {
		t.Fatalf("mark-only run must fold to nothing, got %v", got)
	}
	if got := TokenizePali("̈eva", 1); len(got) != 1 || got[0] != "eva" {
		t.Fatalf("leading mark attaches to the word: %v", got)
	}
}

func TestFoldPali(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"ascii passthrough", "dhamma", "dhamma"},
		{"macron a", "ā", "a"},
		{"macron i", "ī", "i"},
		{"macron u", "ū", "u"},
		{"dot below t", "ṭ", "t"},
		{"dot below d", "ḍ", "d"},
		{"dot above n", "ṅ", "n"},
		{"tilde n", "ñ", "n"},
		{"dot below n", "ṇ", "n"},
		{"dot below l", "ḷ", "l"},
		{"dot below m", "ṃ", "m"},
		{"dot above m", "ṁ", "m"},
		{"dot below r", "ṛ", "r"},
		{"uppercase macron a", "Ā", "a"},
		{"uppercase macron i", "Ī", "i"},
		{"uppercase macron u", "Ū", "u"},
		{"uppercase dot below t", "Ṭ", "t"},
		{"uppercase dot below d", "Ḍ", "d"},
		{"uppercase dot above n", "Ṅ", "n"},
		{"uppercase tilde n", "Ñ", "n"},
		{"uppercase dot below n", "Ṇ", "n"},
		{"uppercase dot below l", "Ḷ", "l"},
		{"uppercase dot below m", "Ṃ", "m"},
		{"uppercase dot above m", "Ṁ", "m"},
		{"uppercase dot below r", "Ṛ", "r"},
		{"decomposed macron folds", "ā", "a"},
		{"decomposed anusvara folds", "ṁ", "m"},
		{"decomposed uppercase macron folds", "Ā", "a"},
		{"word lowercase", "dhammo", "dhammo"},
		{"word initial capital", "Dhammā", "dhamma"},
		{"sentence case", "Evaṁ", "evam"},
		{"all caps", "EVAṀ", "evam"},
		{"both anusvara forms agree on m", "saṃsāra", "samsara"},
		{"dot-above anusvara form", "saṁsāra", "samsara"},
		{"retroflex cluster", "viññāṇa", "vinnana"},
		{"already folded stays", "samsara", "samsara"},
		{"mixed non pali letters pass through", "ångström", "ångström"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FoldPali(tt.in); got != tt.want {
				t.Errorf("FoldPali(%q) = %q; want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFoldPaliIdempotent(t *testing.T) {
	inputs := []string{
		"", "plain", "Dhammā", "EVAṀ", "ā", "ṁ", "Ā",
		"İ", "ß", "Ǆǅǆ", "ǅ", "ångström", "αΒγ",
		"āṁāṁ", "ĀṂĀṀ", "…—‘’“”", "ā̄", "i̇",
	}
	for _, in := range inputs {
		once := FoldPali(in)
		twice := FoldPali(once)
		if once != twice {
			t.Errorf("FoldPali not idempotent: FoldPali(%q) = %q, refold = %q", in, once, twice)
		}
	}
	for _, want := range []string{"", "a", "m", "dhamma", "samsara", "vinnana"} {
		if got := FoldPali(want); got != want {
			t.Errorf("FoldPali(%q) = %q; want fixpoint %q", want, got, want)
		}
	}
}

func TestTokenizePali(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		minLen int
		want   []string
	}{
		{"empty input", "", 2, nil},
		{"no letters", "123 !? \t—", 2, nil},
		{"plain words", "evaṁ me sutaṁ", 2, []string{"evam", "me", "sutam"}},
		{"punctuation splits", "dhamma, saddhamma.", 2, []string{"dhamma", "saddhamma"}},
		{"curly quotes", "“evaṁ”", 2, []string{"evam"}},
		{"digits are separators", "an1.1-10", 2, []string{"an"}},
		{"counter digits dropped", "#12 idamavoca", 2, []string{"idamavoca"}},
		{"roman numerals kept", "i ii iii", 1, []string{"i", "ii", "iii"}},
		{"roman numerals kept at minLen 2", "ii iii iv", 2, []string{"ii", "iii", "iv"}},
		{"minLen keeps boundary", "a ab abc", 2, []string{"ab", "abc"}},
		{"minLen drops shorter", "ab abc abcd", 3, []string{"abc", "abcd"}},
		{"minLen one keeps singles", "a b", 1, []string{"a", "b"}},
		{"pe dropped between words", "evaṁ pe sutaṁ", 2, []string{"evam", "sutam"}},
		{"pe alone dropped", "…pe…", 2, nil},
		{"uppercase pe dropped", "Pe", 2, nil},
		{"pe inside longer token kept", "peyyāla", 2, []string{"peyyala"}},
		{"pe as suffix kept", "supe", 2, []string{"supe"}},
		{"already folded input", "evam me", 2, []string{"evam", "me"}},
		{"newline and tab split", "dhamma\nca\tsaddhamma", 2, []string{"dhamma", "ca", "saddhamma"}},
		{"decomposed input", "evā me", 2, []string{"eva", "me"}},
		{"turkish dotted capital i folds plain", "İ", 1, []string{"i"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TokenizePali(tt.in, tt.minLen)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TokenizePali(%q, %d) = %q; want %q", tt.in, tt.minLen, got, tt.want)
			}
		})
	}
}
