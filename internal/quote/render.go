package quote

import (
	"html"
	"html/template"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

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
			b.WriteString("  ")
		} else {
			b.WriteString(" - **")
			b.WriteString(q.Citation)
			b.WriteString("**")
		}
	}
	return b.String()
}

func (q *Quote) DisplayHTML() template.HTML { return q.DisplayHTMLWithTerms(nil) }

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
	href, external := suttaHref(id)
	b.WriteString(`<a class="sutta-link" href="`)
	b.WriteString(href)
	if external {
		b.WriteString(`" target="_blank" rel="noopener"><strong>`)
	} else {
		b.WriteString(`"><strong>`)
	}
	b.WriteString(highlight(id, terms))
	b.WriteString(`</strong></a>`)
	return b.String()
}

func suttaHref(id string) (string, bool) {
	muid, err := MuidFromSuttaID(id)
	if err != nil {
		return SuttaURL(id), true
	}
	return "/texts/" + url.PathEscape(muid), false
}

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
	sort.Slice(parts, func(i, j int) bool { return len(parts[i]) > len(parts[j]) })
	return regexp.MustCompile("(?i)(?:" + strings.Join(parts, "|") + ")")
}

const (
	dotSeparator = ".  \n.  \n."

	dotSeparatorGap = "\n\n\n"
)

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
