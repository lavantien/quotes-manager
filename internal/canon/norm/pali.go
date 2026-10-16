package norm

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

var paliFold = map[rune]rune{
	'ā': 'a', 'ī': 'i', 'ū': 'u',
	'ṭ': 't', 'ḍ': 'd',
	'ṅ': 'n', 'ñ': 'n', 'ṇ': 'n', 'ḷ': 'l',
	'ṃ': 'm', 'ṁ': 'm', 'ṛ': 'r',
	'Ā': 'a', 'Ī': 'i', 'Ū': 'u',
	'Ṭ': 't', 'Ḍ': 'd',
	'Ṅ': 'n', 'Ñ': 'n', 'Ṇ': 'n', 'Ḷ': 'l',
	'Ṃ': 'm', 'Ṁ': 'm', 'Ṛ': 'r',
}

func FoldPali(s string) string {
	for {
		next := foldPaliOnce(s)
		if next == s {
			return s
		}
		s = next
	}
}

func foldPaliOnce(s string) string {
	s, _ = NFC(s)
	s = strings.Map(func(r rune) rune {
		if f, ok := paliFold[r]; ok {
			return f
		}
		if unicode.In(r, unicode.Mn, unicode.Mc, unicode.Me) {
			return -1
		}
		return r
	}, s)
	return strings.ToLower(s)
}

func TokenizePali(s string, minLen int) []string {
	var tokens []string
	start := -1
	for i, r := range s {
		if unicode.IsLetter(r) || unicode.In(r, unicode.Mn, unicode.Mc, unicode.Me) {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			tokens = appendToken(tokens, s[start:i], minLen)
			start = -1
		}
	}
	if start >= 0 {
		tokens = appendToken(tokens, s[start:], minLen)
	}
	return tokens
}

func appendToken(tokens []string, run string, minLen int) []string {
	tok := FoldPali(run)
	if tok == "" || tok == "pe" || utf8.RuneCountInString(tok) < minLen {
		return tokens
	}
	return append(tokens, tok)
}
