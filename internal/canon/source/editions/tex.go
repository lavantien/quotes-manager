package editions

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

var (
	crlfRe  = regexp.MustCompile("\r\n?")
	dqRe    = regexp.MustCompile("``|''")
	sqRe    = regexp.MustCompile("[`']")
	dashRe  = regexp.MustCompile("-{2,}")
	spaceRe = regexp.MustCompile("[ \t]+")
)

func Normalize(src string) string {
	src = crlfRe.ReplaceAllString(src, "\n")
	out := foldTeX(src)
	out = norm.NFC.String(out)
	out = foldQuotes(out)
	out = dashRe.ReplaceAllString(out, "-")
	out = spaceRe.ReplaceAllString(out, " ")
	out = strings.ReplaceAll(out, string(rune(0xFEFF)), "")
	return strings.TrimSpace(out)
}

func foldQuotes(s string) string {
	for {
		next := sqRe.ReplaceAllString(dqRe.ReplaceAllString(s, `"`), "'")
		if next == s {
			return s
		}
		s = next
	}
}

func foldTeX(src string) string {
	rs := []rune(src)
	var b strings.Builder
	b.Grow(len(src))
	var argDepth []int
	for i := 0; i < len(rs); {
		c := rs[i]
		switch {
		case c == '\\':
			i = foldEscape(&b, rs, i, &argDepth)
		case c == '%':
			for i < len(rs) && rs[i] != '\n' {
				i++
			}
		case c == '~':
			b.WriteRune(' ')
			i++
		case c == '{':
			if len(argDepth) > 0 {
				argDepth[len(argDepth)-1]++
			}
			b.WriteRune(c)
			i++
		case c == '}' && len(argDepth) > 0:
			if argDepth[len(argDepth)-1] == 0 {
				argDepth = argDepth[:len(argDepth)-1]
			} else {
				argDepth[len(argDepth)-1]--
				b.WriteRune(c)
			}
			i++
		default:
			b.WriteRune(c)
			i++
		}
	}
	return b.String()
}

func foldEscape(b *strings.Builder, rs []rune, i int, argDepth *[]int) int {
	if i+1 >= len(rs) {
		return i + 1
	}
	switch n := rs[i+1]; {
	case n == '%':
		b.WriteString(`\%`)
		return i + 2
	case n == '&' || n == '#' || n == '$' || n == '_' || n == '{' || n == '}':
		b.WriteRune(n)
		return i + 2
	case n == '\\':
		j := skipHSpace(rs, i+2)
		if j < len(rs) && rs[j] == '*' {
			j = skipHSpace(rs, j+1)
		}
		if end, ok := optionalArgEnd(rs, j); ok {
			j = skipHSpace(rs, end)
		}
		b.WriteRune('\n')
		return j
	case n == ',' || n == ';' || n == ':' || n == '!':
		b.WriteRune(' ')
		return i + 2
	case n == '~':
		b.WriteRune(' ')
		if i+3 < len(rs) && rs[i+2] == '{' && rs[i+3] == '}' {
			return i + 4
		}
		return i + 2
	case unicode.IsLetter(n):
		return foldMacro(b, rs, i, argDepth)
	default:
		return i + 1
	}
}

func foldMacro(b *strings.Builder, rs []rune, i int, argDepth *[]int) int {
	j := i + 1
	for j < len(rs) && unicode.IsLetter(rs[j]) {
		j++
	}
	word := string(rs[i+1 : j])
	k := skipHSpace(rs, j)
	if k < len(rs) && rs[k] == '*' {
		k = skipHSpace(rs, k+1)
	}
	switch {
	case word == "par" || word == "newline":
		b.WriteRune('\n')
		return k
	case word == "footnote":
		p := skipTeXSpace(rs, k)
		if end, ok := optionalArgEnd(rs, p); ok {
			p = skipTeXSpace(rs, end)
		}
		if p < len(rs) && rs[p] == '{' {
			return groupEnd(rs, p)
		}
		return k
	case word == "begin" || word == "end":
		p := skipTeXSpace(rs, k)
		if p < len(rs) && rs[p] == '{' {
			return groupEnd(rs, p)
		}
		return k
	case k < len(rs) && rs[k] == '{':
		*argDepth = append(*argDepth, 0)
		return k + 1
	default:
		if p := skipTeXSpace(rs, k); p < len(rs) && rs[p] == '{' {
			*argDepth = append(*argDepth, 0)
			return p + 1
		}
		return k
	}
}

func skipHSpace(rs []rune, i int) int {
	for i < len(rs) && (rs[i] == ' ' || rs[i] == '\t') {
		i++
	}
	return i
}

func skipTeXSpace(rs []rune, i int) int {
	for i < len(rs) {
		switch rs[i] {
		case ' ', '\t', '\n':
			i++
		case '%':
			for i < len(rs) && rs[i] != '\n' {
				i++
			}
		default:
			return i
		}
	}
	return i
}

func optionalArgEnd(rs []rune, i int) (int, bool) {
	if i >= len(rs) || rs[i] != '[' {
		return i, false
	}
	for j := i; j < len(rs); j++ {
		switch rs[j] {
		case ']':
			return j + 1, true
		case '\n':
			return i, false
		}
	}
	return i, false
}

func groupEnd(rs []rune, open int) int {
	depth := 0
	for i := open; i < len(rs); {
		switch rs[i] {
		case '\\':
			run := 0
			j := i
			for j < len(rs) && rs[j] == '\\' {
				run++
				j++
			}
			if run%2 == 1 {
				j++
			}
			i = j
		case '%':
			for i < len(rs) && rs[i] != '\n' {
				i++
			}
		case '{':
			depth++
			i++
		case '}':
			depth--
			i++
			if depth == 0 {
				return i
			}
		default:
			i++
		}
	}
	return len(rs)
}
