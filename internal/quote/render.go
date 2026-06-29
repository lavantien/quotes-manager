package quote

import (
	"html"
	"html/template"
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
func (q *Quote) DisplayHTML() template.HTML {
	var b strings.Builder
	last := len(q.Passages) - 1
	for i, p := range q.Passages {
		if i > 0 {
			b.WriteString("<br>")
		}
		b.WriteString("<em>")
		b.WriteString(html.EscapeString(p))
		b.WriteString("</em>")
		if i == last {
			b.WriteString(q.citationHTML())
		}
	}
	return template.HTML(b.String())
}

// citationHTML renders the citation tail: the attribution (escaped) followed by
// the sutta id as a bold suttacentral link. If no sutta id is recognized the
// whole citation is emitted as escaped text.
func (q *Quote) citationHTML() string {
	c := q.Citation
	loc := suttaIDRe.FindStringIndex(c)
	var b strings.Builder
	b.WriteString(" — ")
	if loc == nil {
		b.WriteString(html.EscapeString(c))
		return b.String()
	}
	prefix, id := c[:loc[0]], c[loc[0]:loc[1]]
	b.WriteString(html.EscapeString(prefix))
	b.WriteString(`<a class="sutta-link" href="`)
	b.WriteString(SuttaURL(id))
	b.WriteString(`" target="_blank" rel="noopener"><strong>`)
	b.WriteString(html.EscapeString(id))
	b.WriteString(`</strong></a>`)
	return b.String()
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
