package sutta

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const rangeCap = 1000

var families = map[string]bool{
	"mn":   true,
	"an":   true,
	"sn":   true,
	"dn":   true,
	"snp":  true,
	"dhp":  true,
	"ud":   true,
	"iti":  true,
	"thag": true,
	"thig": true,
	"khp":  true,
	"ja":   true,
	"vv":   true,
}

var (
	slugRe        = regexp.MustCompile(`^([a-z]+)(\d+(?:\.\d+)?)(?:-(\d+(?:\.\d+)?))?$`)
	pliTvRe       = regexp.MustCompile(`^pli-tv-[a-z0-9-]*\d[a-z0-9-]*$`)
	localeRe      = regexp.MustCompile(`^[a-z]{2,3}$`)
	dhammatalksRe = regexp.MustCompile(`StNp(\d+(?:\.\d+)?)(?:_(\d+))?\.html`)
)

type suttaRef struct {
	muids    []string
	link     string
	expanded bool
}

func parseSuttaURL(raw string) (suttaRef, bool, error) {
	decoded := decodeEntities(raw)
	u, err := url.Parse(decoded)
	if err != nil {
		return suttaRef{}, false, nil
	}
	host := strings.TrimPrefix(u.Host, "www.")
	switch host {
	case "suttacentral.net":
		return parseSuttacentralPath(u.Path, decoded)
	case "dhammatalks.org":
		m := dhammatalksRe.FindStringSubmatch(u.Path)
		if m == nil {
			return suttaRef{}, false, nil
		}
		muid := "snp" + normalizeNum(m[1])
		if m[2] != "" {
			muid += "." + normalizeNum(m[2])
		}
		return suttaRef{muids: []string{muid}, link: decoded}, true, nil
	default:
		return suttaRef{}, false, nil
	}
}

func parseSuttacentralPath(path, raw string) (suttaRef, bool, error) {
	segs := strings.Split(strings.Trim(path, "/"), "/")
	if segs[0] == "" {
		return suttaRef{}, false, nil
	}
	candidate := segs[0]
	if len(segs) > 1 && localeRe.MatchString(candidate) {
		candidate = segs[1]
	}
	if strings.HasPrefix(candidate, "pli-tv") {
		if !pliTvRe.MatchString(candidate) {
			return suttaRef{}, false, nil
		}
		return suttaRef{muids: []string{candidate}, link: raw}, true, nil
	}
	m := slugRe.FindStringSubmatch(candidate)
	if m == nil {
		return suttaRef{}, false, nil
	}
	family := m[1]
	if !families[family] {
		return suttaRef{}, false, fmt.Errorf("unknown sutta family in url %q", raw)
	}
	start := normalizeNum(m[2])
	if m[3] == "" {
		return suttaRef{muids: []string{family + start}, link: raw}, true, nil
	}
	muids, err := expandRange(family, start, normalizeNum(m[3]))
	if err != nil {
		return suttaRef{}, false, fmt.Errorf("range in url %q: %w", raw, err)
	}
	return suttaRef{muids: muids, link: raw, expanded: true}, true, nil
}

func KnownMuid(muid string) bool {
	if pliTvRe.MatchString(muid) {
		return true
	}
	m := slugRe.FindStringSubmatch(muid)
	return m != nil && families[m[1]]
}

func decodeEntities(s string) string {
	return strings.NewReplacer("&lt;", "<", "&gt;", ">", "&amp;", "&").Replace(s)
}

func normalizeNum(s string) string {
	parts := strings.Split(s, ".")
	for i, p := range parts {
		t := strings.TrimLeft(p, "0")
		if t == "" {
			t = "0"
		}
		parts[i] = t
	}
	return strings.Join(parts, ".")
}

func expandRange(family, start, end string) ([]string, error) {
	startCh, startN, errStart := splitNum(start)
	endCh, endN, errEnd := splitNum(end)
	if errStart != nil || errEnd != nil {
		return nil, fmt.Errorf("malformed endpoints %q-%q", start, end)
	}
	if endCh == nil && startCh != nil {
		c := *startCh
		endCh = &c
	}
	if (startCh == nil) != (endCh == nil) {
		return nil, fmt.Errorf("chapter mismatch %q-%q", start, end)
	}
	if startCh != nil && *endCh != *startCh {
		return nil, fmt.Errorf("chapter mismatch %q-%q", start, end)
	}
	if endN < startN {
		return nil, fmt.Errorf("reversed %q-%q", start, end)
	}
	span := endN - startN + 1
	if span > rangeCap {
		return nil, fmt.Errorf("span %d over cap %d", span, rangeCap)
	}
	out := make([]string, 0, span)
	if startCh == nil {
		for n := startN; n <= endN; n++ {
			out = append(out, family+strconv.Itoa(n))
		}
		return out, nil
	}
	chapter := strconv.Itoa(*startCh)
	for n := startN; n <= endN; n++ {
		out = append(out, family+chapter+"."+strconv.Itoa(n))
	}
	return out, nil
}

func splitNum(s string) (*int, int, error) {
	chapter, num, found := strings.Cut(s, ".")
	if !found {
		n, err := strconv.Atoi(s)
		if err != nil {
			return nil, 0, err
		}
		return nil, n, nil
	}
	c, err := strconv.Atoi(chapter)
	if err != nil {
		return nil, 0, err
	}
	n, err := strconv.Atoi(num)
	if err != nil {
		return nil, 0, err
	}
	return &c, n, nil
}
