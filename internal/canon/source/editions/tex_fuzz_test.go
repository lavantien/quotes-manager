package editions

import (
	"testing"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

func FuzzTexNormalize(f *testing.F) {
	seeds := []string{
		"",
		"plain text",
		"a % comment\nb",
		`50\% off`,
		`\emph{x} \textbf{y}`,
		`\footnote{gone \emph{deep}}`,
		"``q'' 's' --- -- ----",
		"`'",
		"`''",
		"-'--`",
		"a \\\\ b \\par c \\newline d",
		"café",
		"a\r\nb\rc",
		`a\,b\;c\:d\!e f~g\~{}h`,
		`\emph{a'}'b`,
		`x\footnote{open`,
		"\\\\",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		n1 := Normalize(s)
		if !utf8.ValidString(n1) {
			t.Fatalf("invalid UTF-8: %q -> %q", s, n1)
		}
		if !norm.NFC.IsNormalString(n1) {
			t.Fatalf("not NFC: %q -> %q", s, n1)
		}
		if n2 := Normalize(n1); n2 != n1 {
			t.Fatalf("not idempotent:\nin   %q\nout  %q\nout2 %q", s, n1, n2)
		}
	})
}
