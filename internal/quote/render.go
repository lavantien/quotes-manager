package quote

import "strings"

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
