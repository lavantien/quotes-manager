package quote

import (
	"regexp"
	"strings"
)

// boldTailRe matches the canonical " - **citation**" suffix BodyMD appends to the
// final passage line.
var boldTailRe = regexp.MustCompile(`\s+-\s+\*\*(.+)\*\*\s*$`)

// ParseCanonical parses the canonical text format — quotes rendered by BodyMD
// and separated by the three-dot divider (see RenderExportFile) — back into
// quotes. It is the inverse of the export: each block's italic passage lines and
// bold citation tail are recovered. A block whose last line carries a plain
// " - clean-citation" tail (no bold markers) is accepted as a fallback for
// lightly edited pastes; blocks without any recognizable citation are skipped.
// The result is in document order and is not de-duplicated.
func ParseCanonical(text string) []*Quote {
	var out []*Quote
	for _, block := range splitCanonicalBlocks(text) {
		if q, ok := parseCanonicalBlock(block); ok {
			out = append(out, q)
		}
	}
	return out
}

// splitCanonicalBlocks divides text on runs of '.'-only divider lines (the dot
// separator), returning the trimmed non-empty quote blocks between them.
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
	if m := boldTailRe.FindStringSubmatch(block); m != nil {
		citation = strings.TrimSpace(m[1])
		body = block[:len(block)-len(m[0])]
	} else {
		// Fallback: a plain " - clean-citation" tail on the last line.
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

// parsePassageLines strips the '*' italics wrapping and the trailing two-space
// markdown break BodyMD adds to each passage line.
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
