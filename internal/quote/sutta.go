package quote

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

const suttaIDPat = `(?:` +
	`(?:DN|MN|AN|SN)\s+\d+(?:[.-]\d+)*(?:#\d+(?:\.\d+)*)?` +
	`|KN\s+[A-Za-z]+\s+\d+(?:[.-]\d+)*` +
	`|pli-tv[A-Za-z0-9-]+(?:#\d+(?:\.\d+)*)?` +
	`|Tv\s+Vi\s+Bu\s+Pj\d` +
	`)`

var (
	suttaIDRe = regexp.MustCompile(suttaIDPat)

	citationRe = regexp.MustCompile(`^\s*(?:[^(),]+,\s+)*` + suttaIDPat + `(?:\s*\([^)]*\))?\s*$`)

	headerRe = regexp.MustCompile(`^` + suttaIDPat + `:\s*$`)

	sepRe = regexp.MustCompile(`\s+-\s+`)

	urlParensRe = regexp.MustCompile(`\s*\([^)]*\)\s*$`)

	muidNikayaRe       = regexp.MustCompile(`^(dn|mn|an|sn)(\d.*)$`)
	muidAbbrevVinayaRe = regexp.MustCompile(`^tvvibu(pj\d)$`)
	muidSubBookRe      = regexp.MustCompile(`^([a-z]+)(\d.*)$`)
)

func CanonicalSuttaID(s string) string {
	return strings.TrimSpace(suttaIDRe.FindString(s))
}

func MuidFromSuttaID(id string) (string, error) {
	c := CanonicalSuttaID(id)
	if c == "" {
		return "", fmt.Errorf("no sutta id in %q", id)
	}
	if strings.HasPrefix(c, "pli-tv") {
		if i := strings.IndexByte(c, '#'); i >= 0 {
			c = c[:i]
		}
		return c, nil
	}
	if rest, ok := strings.CutPrefix(c, "KN "); ok {
		if muidNikayaRe.MatchString(compactLower(rest)) {
			return "", fmt.Errorf("kn sub-book %q aliases a main nikaya muid", rest)
		}
		c = rest
	}
	return compactLower(c), nil
}

func DisplaySuttaID(muid string) (string, error) {
	if m := muidNikayaRe.FindStringSubmatch(muid); m != nil {
		return strings.ToUpper(m[1]) + " " + m[2], nil
	}
	if m := muidAbbrevVinayaRe.FindStringSubmatch(muid); m != nil {
		return "Tv Vi Bu Pj" + m[1][2:], nil
	}
	if strings.HasPrefix(muid, "pli-tv") {
		return muid, nil
	}
	if m := muidSubBookRe.FindStringSubmatch(muid); m != nil {
		return "KN " + titleFirst(m[1]) + " " + m[2], nil
	}
	return "", fmt.Errorf("no display id for muid %q", muid)
}

func compactLower(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if !unicode.IsSpace(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func titleFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	return string(unicode.ToUpper(r[0])) + string(r[1:])
}

func SuttaURL(id string) string {
	sig := strings.ToLower(strings.ReplaceAll(CanonicalSuttaID(id), " ", ""))
	return "https://suttacentral.net/" + sig
}

func cleanCitation(s string) string {
	return strings.TrimSpace(urlParensRe.ReplaceAllString(strings.TrimSpace(s), ""))
}

func ensureAttribution(citation string) string {
	c := strings.TrimSpace(citation)
	if c == CanonicalSuttaID(c) {
		return "the Buddha, " + c
	}
	return c
}

func splitCitation(line string) (passage, citation string, ok bool) {
	idxs := sepRe.FindAllStringIndex(line, -1)
	for i := len(idxs) - 1; i >= 0; i-- {
		tail := line[idxs[i][1]:]
		if citationRe.MatchString(tail) {
			return line[:idxs[i][0]], cleanCitation(tail), true
		}
	}
	return line, "", false
}
