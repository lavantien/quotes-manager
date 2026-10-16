package norm

import (
	"strings"
	"testing"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

func TestNFC(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		changed bool
	}{
		{"", "", false},
		{"abc", "abc", false},
		{"mn1 plain text", "mn1 plain text", false},
		{"ā", "ā", true},
		{"ā", "ā", false},
		{"ṁ", "ṁ", true},
		{"Evaṁ sutaṁ", "Evaṁ sutaṁ", false},
	}
	for _, tt := range tests {
		got, changed := NFC(tt.in)
		if got != tt.want || changed != tt.changed {
			t.Errorf("NFC(%q) = %q, %v; want %q, %v", tt.in, got, changed, tt.want, tt.changed)
		}
	}
}

func TestNFCMatchesXText(t *testing.T) {
	for _, s := range []string{
		"āīūṭḍṅṇḷṃṁ",
		"āï",
		"plain",
		"“quotes” — …",
	} {
		want := norm.NFC.String(s)
		got, changed := NFC(s)
		if got != want {
			t.Errorf("NFC(%q) = %q; want %q", s, got, want)
		}
		if changed != (got != s) {
			t.Errorf("NFC(%q) changed = %v; want %v", s, changed, got != s)
		}
	}
}

func TestFold(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"plain passthrough", "Middle Discourses 1", "Middle Discourses 1"},
		{"double curly quotes", "“Mendicants!”", `"Mendicants!"`},
		{"single curly quotes", "‘Venerable sir,’", `'Venerable sir,'`},
		{"apostrophe", "they’ve", "they've"},
		{"en dash", "a–b", "a-b"},
		{"em dash", "sutaṁ—", "sutaṁ-"},
		{"horizontal bar", "a―b", "a-b"},
		{"minus sign", "a−b", "a-b"},
		{"ellipsis", "fire … ", "fire ..."},
		{"escaped percent unwraps", "50\\% more", "50% more"},
		{"bare percent untouched", "50% more", "50% more"},
		{"crlf folds to space", "a\r\nb", "a b"},
		{"lone cr folds to space", "a\rb\rc", "a b c"},
		{"space run collapse", "a  \t b", "a b"},
		{"nbsp folds to space", "a b", "a b"},
		{"newline collapses to space", "a\n\n b", "a b"},
		{"space run around newline collapses", "a \n b", "a b"},
		{"trim ends", "  evaṁ  ", "evaṁ"},
		{"trim trailing newline", "idamavoca\n", "idamavoca"},
		{"nfd input composed", "ā", "ā"},
		{"markup untouched", "*em* _root_ **bold** #counter", "*em* _root_ **bold** #counter"},
		{"pe marker untouched", "…pe…", "...pe..."},
		{
			"composite",
			"“Evaṁ … me—”\r\n",
			"\"Evaṁ ... me-\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Fold(tt.in); got != tt.want {
				t.Errorf("Fold(%q) = %q; want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFoldIdempotent(t *testing.T) {
	inputs := []string{
		"", "plain", "a\r\n\r\nb", " “x”\r\n", "ā̄ …\t–",
		"a  b", "\r", "\r\n", "\n\n\n", " \t \t ",
		"…—–‘’“”",
	}
	for _, in := range inputs {
		once := Fold(in)
		twice := Fold(once)
		if once != twice {
			t.Errorf("Fold not idempotent: Fold(%q) = %q, Fold(Fold) = %q", in, once, twice)
		}
	}
}

func TestFoldProperties(t *testing.T) {
	for _, in := range []string{"ā—“”…\r\n ", "nested “‘quotes’”"} {
		out := Fold(in)
		if !utf8.ValidString(out) {
			t.Errorf("Fold(%q) produced invalid UTF-8: %q", in, out)
		}
		if out != norm.NFC.String(out) {
			t.Errorf("Fold(%q) is not NFC: %q", in, out)
		}
		for _, bad := range []string{"‘", "’", "“", "”", "–", "—", "…", "\r", " "} {
			if strings.Contains(out, bad) {
				t.Errorf("Fold(%q) still contains %q: %q", in, bad, out)
			}
		}
	}
}
