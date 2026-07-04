package quote

import (
	"html"
	"html/template"
	"regexp"
	"sort"
	"strings"
)

// BodyMD renders the canonical singular format. Every passage line is wrapped in
// italics; lines are joined with a two-space markdown break (no blank lines
// between passages); the final line carries the bold citation:
//
//	*"passage one*
//	*passage two*
//	*last passage"* - **the Buddha, MN 22**
func (q *Quote) BodyMD() string {
	var b strings.Builder
	last := len(q.Passages) - 1
	for i, p := range q.Passages {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteByte('*')
		b.WriteString(p)
		b.WriteByte('*')
		if i < last {
			b.WriteString("  ") // markdown line break
		} else {
			b.WriteString(" - **")
			b.WriteString(q.Citation)
			b.WriteString("**")
		}
	}
	return b.String()
}

// DisplayHTML renders the quote body for the web UI: passages italicized, the
// citation following an em dash with the attribution kept verbatim and only the
// sutta id bolded and linked to suttacentral (opens in a new tab). This is the
// on-screen format; BodyMD remains the canonical/export format.
// DisplayHTML renders the quote body for the web UI: passages italicized, the
// citation following an em dash with the attribution kept verbatim and only the
// sutta id bolded and linked to suttacentral (opens in a new tab). This is the
// on-screen format; BodyMD remains the canonical/export format.
func (q *Quote) DisplayHTML() template.HTML { return q.DisplayHTMLWithTerms(nil) }

// DisplayHTMLWithTerms is DisplayHTML with search terms highlighted: every
// (case-insensitive) occurrence of any term inside a passage or the citation is
// wrapped in <mark>. With nil/empty terms it is byte-identical to DisplayHTML.
func (q *Quote) DisplayHTMLWithTerms(terms []string) template.HTML {
	var b strings.Builder
	last := len(q.Passages) - 1
	for i, p := range q.Passages {
		if i > 0 {
			b.WriteString("<br>")
		}
		b.WriteString("<em>")
		b.WriteString(highlight(p, terms))
		b.WriteString("</em>")
		if i == last {
			b.WriteString(q.citationHTMLWithTerms(terms))
		}
	}
	return template.HTML(b.String())
}

// citationHTMLWithTerms renders the citation tail: the attribution followed by
// the sutta id as a bold suttacentral link. Search terms are highlighted in both
// the prefix and the id; the <a>/<strong> markup is emitted here, only the inner
// text is highlighted/escaped. With no terms it is plain escaping.
func (q *Quote) citationHTMLWithTerms(terms []string) string {
	c := q.Citation
	loc := suttaIDRe.FindStringIndex(c)
	var b strings.Builder
	b.WriteString(" — ")
	if loc == nil {
		b.WriteString(highlight(c, terms))
		return b.String()
	}
	prefix, id := c[:loc[0]], c[loc[0]:loc[1]]
	b.WriteString(highlight(prefix, terms))
	b.WriteString(`<a class="sutta-link" href="`)
	b.WriteString(SuttaURL(id))
	b.WriteString(`" target="_blank" rel="noopener"><strong>`)
	b.WriteString(highlight(id, terms))
	b.WriteString(`</strong></a>`)
	return b.String()
}

// highlight HTML-escapes in and wraps every case-insensitive occurrence of any
// term in <mark>...</mark>, preserving the original case of the matched text.
// With no usable terms it is plain html.EscapeString. Nothing is emitted
// unescaped: the matched substring is itself escaped before wrapping.
func highlight(in string, terms []string) string {
	re := termsRegexp(terms)
	if re == nil {
		return html.EscapeString(in)
	}
	var b strings.Builder
	last := 0
	for _, m := range re.FindAllStringIndex(in, -1) {
		b.WriteString(html.EscapeString(in[last:m[0]]))
		b.WriteString("<mark>")
		b.WriteString(html.EscapeString(in[m[0]:m[1]]))
		b.WriteString("</mark>")
		last = m[1]
	}
	b.WriteString(html.EscapeString(in[last:]))
	return b.String()
}

// termsRegexp compiles a case-insensitive alternation of the literal terms
// (regexp.QuoteMeta'd so user input can never inject regex metacharacters),
// longest-first so a longer term wins over a shorter prefix at the same spot.
// Empty terms are skipped; it returns nil when no term is usable.
func termsRegexp(terms []string) *regexp.Regexp {
	parts := make([]string, 0, len(terms))
	for _, t := range terms {
		if t == "" {
			continue
		}
		parts = append(parts, regexp.QuoteMeta(t))
	}
	if len(parts) == 0 {
		return nil
	}
	sort.Slice(parts, func(i, j int) bool {
		if len(parts[i]) != len(parts[j]) {
			return len(parts[i]) > len(parts[j])
		}
		return parts[i] < parts[j]
	})
	return regexp.MustCompile("(?i)(?:" + strings.Join(parts, "|") + ")")
}

const (
	// dotSeparator is the three-dot inter-quote divider used in the text export.
	dotSeparator = ".  \n.  \n."
	// dotSeparatorGap wraps the divider with two blank lines on each side.
	dotSeparatorGap = "\n\n\n"
)

// RenderExportFile joins sorted quotes with the dot separator. Each quote is
// rendered in the canonical format; quotes are separated by two blank lines, the
// three-dot divider, and two more blank lines.
func RenderExportFile(qs []*Quote) string {
	var b strings.Builder
	for i, q := range qs {
		if i > 0 {
			b.WriteString(dotSeparatorGap)
			b.WriteString(dotSeparator)
			b.WriteString(dotSeparatorGap)
		}
		b.WriteString(q.BodyMD())
	}
	b.WriteByte('\n')
	return b.String()
}
