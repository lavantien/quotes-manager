package quote

import (
	"regexp"
	"strings"
)

var boldTailRe = regexp.MustCompile(`(?s:.*)(\s+-\s+\*\*(.+)\*\*\s*)$`)

func ParseCanonical(text string) []*Quote {
	var out []*Quote
	for _, block := range splitCanonicalBlocks(text) {
		if q, ok := parseCanonicalBlock(block); ok {
			out = append(out, q)
		}
	}
	return out
}

func splitCanonicalBlocks(text string) []string {
	var blocks []string
	var cur strings.Builder
	flush := func() {
		if s := strings.TrimSpace(cur.String()); s != "" {
			blocks = append(blocks, s)
		}
		cur.Reset()
	}
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimRight(raw, "\r")
		if strings.TrimSpace(line) == "." {
			flush()
			continue
		}
		if cur.Len() > 0 {
			cur.WriteByte('\n')
		}
		cur.WriteString(line)
	}
	flush()
	return blocks
}

func parseCanonicalBlock(block string) (*Quote, bool) {
	body, citation := block, ""
	if loc := boldTailRe.FindStringSubmatchIndex(block); loc != nil {
		citation = strings.TrimSpace(block[loc[4]:loc[5]])
		body = block[:loc[2]]
	} else {
		lines := strings.Split(body, "\n")
		passage, cit, ok := splitCitation(lines[len(lines)-1])
		if !ok {
			return nil, false
		}
		citation = ensureAttribution(cit)
		lines[len(lines)-1] = passage
		body = strings.Join(lines, "\n")
	}
	passages := parsePassageLines(body)
	if len(passages) == 0 || strings.TrimSpace(citation) == "" {
		return nil, false
	}
	sutta := CanonicalSuttaID(citation)
	if sutta == "" {
		sutta = strings.TrimSpace(citation)
	}
	return New(sutta, ensureAttribution(citation), passages), true
}

func parsePassageLines(body string) []string {
	var out []string
	for _, raw := range strings.Split(body, "\n") {
		s := strings.TrimSpace(raw)
		s = strings.TrimPrefix(s, "*")
		s = strings.TrimSuffix(s, "*")
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}
