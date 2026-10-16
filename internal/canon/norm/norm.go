package norm

import (
	"strings"

	"golang.org/x/text/unicode/norm"
)

func NFC(s string) (string, bool) {
	out := norm.NFC.String(s)
	return out, out != s
}

var punctFold = map[rune]rune{
	0x2018: '\'',
	0x2019: '\'',
	0x201B: '\'',
	0x201C: '"',
	0x201D: '"',
	0x201F: '"',
	0x2010: '-',
	0x2011: '-',
	0x2012: '-',
	0x2013: '-',
	0x2014: '-',
	0x2015: '-',
	0x2212: '-',
	0x00A0: ' ',
}

var dashFold = map[rune]rune{
	0x2010: '-',
	0x2011: '-',
	0x2012: '-',
	0x2013: '-',
	0x2014: '-',
	0x2015: '-',
	0x2043: '-',
	0x2212: '-',
	0xFE58: '-',
	0xFE63: '-',
}

func FoldDashes(s string) string {
	return strings.Map(func(r rune) rune {
		if f, ok := dashFold[r]; ok {
			return f
		}
		return r
	}, s)
}

func Fold(s string) string {
	s = norm.NFC.String(s)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.ReplaceAll(s, `\%`, "%")
	s = strings.ReplaceAll(s, "…", "...")
	s = strings.Map(func(r rune) rune {
		if f, ok := punctFold[r]; ok {
			return f
		}
		return r
	}, s)

	var b strings.Builder
	b.Grow(len(s))
	inRun := false
	write := func(r rune) {
		if inRun {
			b.WriteByte(' ')
			inRun = false
		}
		b.WriteRune(r)
	}
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' {
			inRun = true
			continue
		}
		write(r)
	}
	if inRun {
		b.WriteByte(' ')
	}
	return strings.TrimSpace(b.String())
}
