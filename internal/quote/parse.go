package quote

import (
	"regexp"
	"strings"
)

// File is a named input document.
type File struct {
	Name    string
	Content string
}

// numPrefixRe strips a leading "(N) " numbering marker from a passage line.
var numPrefixRe = regexp.MustCompile(`^\(\d+\)\s+`)

// Parse extracts quotes from each file, preserving the given order.
func Parse(files []File) []*Quote {
	var out []*Quote
	for _, f := range files {
		out = append(out, parseFile(f.Name, f.Content)...)
	}
	return out
}

// parseFile recognizes three structures inside a document:
//   - a "." divider block: ends the current quote context.
//   - a lone "SUTTA:" header: starts a header-cited quote; every following block
//     becomes its passages until the next header, divider, or EOF.
//   - any block whose last line ends with an inline " - <citation>": an
//     inline-cited quote (covers multi-line dialog and narrative framing).
//
// A quote may span several blank-separated blocks (e.g. verse with stanza
// breaks): any preceding blocks that open a quote ("…") but carry no citation
// are absorbed as fragments into the next cited block.
func parseFile(name, content string) []*Quote {
	var quotes []*Quote

	var pendingSutta, pendingCitation string // active header-cited quote
	var pendingPassages []string             // its accumulated passages
	var orphans []rawBlock                   // uncited quote-opening blocks

	flushHeader := func() {
		if pendingSutta != "" && len(pendingPassages) > 0 {
			quotes = append(quotes, newQuote(pendingSutta, ensureAttribution(pendingCitation), pendingPassages, name))
		}
		pendingSutta, pendingCitation, pendingPassages = "", "", nil
	}

	for _, blk := range splitBlocks(content) {
		// A header-cited quote absorbs everything until a divider or new header.
		if pendingSutta != "" {
			switch {
			case isDivider(blk.lines):
				flushHeader()
				continue
			case len(blk.lines) == 1 && headerRe.MatchString(blk.lines[0]):
				flushHeader()
				pendingSutta = CanonicalSuttaID(blk.lines[0])
				pendingCitation = pendingSutta
				continue
			default:
				for _, ln := range blk.lines {
					pendingPassages = append(pendingPassages, clean(ln))
				}
				continue
			}
		}

		switch {
		case isDivider(blk.lines):
			orphans = nil
		case len(blk.lines) == 1 && headerRe.MatchString(blk.lines[0]):
			orphans = nil
			pendingSutta = CanonicalSuttaID(blk.lines[0])
			pendingCitation = pendingSutta
		default:
			last := blk.lines[len(blk.lines)-1]
			passage, citation, ok := splitCitation(last)
			if !ok {
				if startsQuote(blk.lines[0]) {
					orphans = append(orphans, blk) // fragment awaiting a cited terminator
				} else {
					orphans = nil // prose: abandon any unfinished fragments
				}
				continue
			}
			run := append(orphans, blk)
			orphans = nil
			passages := make([]string, 0)
			for bi, b := range run {
				for li, ln := range b.lines {
					if bi == len(run)-1 && li == len(b.lines)-1 {
						passages = append(passages, clean(passage))
					} else {
						passages = append(passages, clean(ln))
					}
				}
			}
			quotes = append(quotes, newQuote(CanonicalSuttaID(citation), ensureAttribution(citation), passages, name))
		}
	}
	flushHeader()
	return quotes
}

type rawBlock struct {
	lines []string
}

// splitBlocks splits content into maximal runs of consecutive non-blank lines.
func splitBlocks(content string) []rawBlock {
	var blocks []rawBlock
	var cur []string
	flush := func() {
		if len(cur) > 0 {
			blocks = append(blocks, rawBlock{cur})
			cur = nil
		}
	}
	for ln := range strings.SplitSeq(content, "\n") {
		if strings.TrimSpace(ln) == "" {
			flush()
			continue
		}
		cur = append(cur, strings.TrimSpace(ln))
	}
	flush()
	return blocks
}

// isDivider reports whether a block consists solely of "." lines.
func isDivider(lines []string) bool {
	if len(lines) == 0 {
		return false
	}
	for _, ln := range lines {
		if strings.TrimSpace(ln) != "." {
			return false
		}
	}
	return true
}

// startsQuote reports whether a line opens a quotation (straight or curly).
func startsQuote(line string) bool {
	t := strings.TrimSpace(line)
	return strings.HasPrefix(t, "\"") || strings.HasPrefix(t, "“")
}

// clean normalizes a single passage line: drops "(N) " numbering, trims
// whitespace, and strips stray markdown "*" / "_" artifacts from the edges.
func clean(s string) string {
	s = numPrefixRe.ReplaceAllString(s, "")
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "*_")
	return strings.TrimSpace(s)
}
