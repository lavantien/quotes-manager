package quote

import (
	"strings"
	"testing"
)

func TestBodyMDMultiPassage(t *testing.T) {
	q := newQuote("SN 12.35", "the Buddha, SN 12.35",
		[]string{`"aaaaa`, "bbbbb", `ccccc"`}, "x")
	got := q.BodyMD()
	want := "*\"aaaaa*  \n*bbbbb*  \n*ccccc\"* - **the Buddha, SN 12.35**"
	if got != want {
		t.Errorf("BodyMD =\n%q\nwant\n%q", got, want)
	}
}

func TestBodyMDSinglePassage(t *testing.T) {
	q := newQuote("MN 38", "MN 38", []string{`" Consciousness."`}, "x")
	got := q.BodyMD()
	want := `*" Consciousness."* - **MN 38**`
	if got != want {
		t.Errorf("BodyMD = %q, want %q", got, want)
	}
}

func mustContain(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Errorf("missing %q in:\n%s", sub, s)
	}
}

func TestDisplayHTML(t *testing.T) {
	t.Run("single passage with attribution", func(t *testing.T) {
		q := New("MN 22", "the Buddha, MN 22", []string{`"hi"`})
		got := string(q.DisplayHTML())
		mustContain(t, got, `<em>&#34;hi&#34;</em>`)
		mustContain(t, got, `the Buddha, `)
		mustContain(t, got, `<a class="sutta-link" href="https://suttacentral.net/mn22" target="_blank" rel="noopener"><strong>MN 22</strong></a>`)
		if strings.Contains(got, "<strong>the Buddha") {
			t.Errorf("attribution must not be bolded: %s", got)
		}
	})
	t.Run("multi-passage verses joined by br", func(t *testing.T) {
		q := New("KN Snp 2.14", "the Buddha, KN Snp 2.14", []string{`"line one`, `line two"`})
		got := string(q.DisplayHTML())
		mustContain(t, got, `<em>&#34;line one</em><br><em>line two&#34;</em>`)
		mustContain(t, got, `href="https://suttacentral.net/knsnp2.14"`)
	})
	t.Run("multi-comma attribution only bolds the id", func(t *testing.T) {
		c := "These were the Buddha's, the Realized One's last words, DN 16"
		q := New("DN 16", c, []string{`"last words"`})
		got := string(q.DisplayHTML())
		if strings.Contains(got, "<strong>These were") || strings.Contains(got, "<strong>the Realized") {
			t.Errorf("attribution bolded: %s", got)
		}
		mustContain(t, got, `<strong>DN 16</strong>`)
	})
	t.Run("bare id citation links without attribution", func(t *testing.T) {
		q := New("MN 38", "MN 38", []string{`"x"`})
		got := string(q.DisplayHTML())
		mustContain(t, got, `<strong>MN 38</strong>`)
		mustContain(t, got, `href="https://suttacentral.net/mn38"`)
	})
}

func TestRenderExportFileSeparator(t *testing.T) {
	a := newQuote("A", "A", []string{`"a"`}, "x")
	b := newQuote("B", "B", []string{`"b"`}, "x")
	got := RenderExportFile([]*Quote{a, b})
	sep := dotSeparatorGap + dotSeparator + dotSeparatorGap
	if !strings.Contains(got, sep) {
		t.Errorf("missing dot separator %q in:\n%s", sep, got)
	}
	if !strings.HasPrefix(got, "*\"a\"*") || !strings.HasSuffix(strings.TrimSpace(got), "**B**") {
		t.Errorf("unexpected export layout:\n%s", got)
	}
}

func TestBodyMDEmptyPassages(t *testing.T) {
	q := New("MN 22", "the Buddha, MN 22", nil)
	if got := q.BodyMD(); got != "" {
		t.Errorf("BodyMD(nil passages) = %q, want empty", got)
	}
	if got := string(q.DisplayHTML()); got != "" {
		t.Errorf("DisplayHTML(nil passages) = %q, want empty", got)
	}
}

func TestDisplayHTMLNoSuttaID(t *testing.T) {
	// A citation with no recognizable sutta id: the whole citation is emitted as
	// escaped text, with no suttacentral link.
	q := New("", "a sage of old", []string{`"wisdom"`})
	got := string(q.DisplayHTML())
	mustContain(t, got, `<em>&#34;wisdom&#34;</em>`)
	mustContain(t, got, " — a sage of old")
	if strings.Contains(got, "sutta-link") {
		t.Errorf("unexpected sutta link for citation without id: %s", got)
	}
}

func TestDisplayHEscape(t *testing.T) {
	q := New("MN 22", "the Buddha, MN 22", []string{"<script>alert(1)</script> & more"})
	got := string(q.DisplayHTML())
	mustContain(t, got, `&lt;script&gt;alert(1)&lt;/script&gt;`)
	if strings.Contains(got, "<script>") {
		t.Errorf("unescaped script tag: %s", got)
	}
	mustContain(t, got, "&amp; more")
}

func TestRenderExportFileEmptyAndSingle(t *testing.T) {
	if got := RenderExportFile(nil); got != "\n" {
		t.Errorf("RenderExportFile(nil) = %q, want %q", got, "\n")
	}
	got := RenderExportFile([]*Quote{newQuote("A", "A", []string{`"a"`}, "x")})
	want := "*\"a\"* - **A**\n"
	if got != want {
		t.Errorf("RenderExportFile(single) = %q, want %q", got, want)
	}
}

func mustNotContain(t *testing.T, s, sub string) {
	t.Helper()
	if strings.Contains(s, sub) {
		t.Errorf("unexpected %q in:\n%s", sub, s)
	}
}

func TestDisplayHTMLWithTermsNilEqualsDisplayHTML(t *testing.T) {
	q := New("MN 22", "the Buddha, MN 22", []string{`"hi"`, "second passage"})
	if string(q.DisplayHTMLWithTerms(nil)) != string(q.DisplayHTML()) {
		t.Errorf("DisplayHTMLWithTerms(nil) must equal DisplayHTML()")
	}
	// An empty (but non-nil) terms slice must also be a no-op.
	if string(q.DisplayHTMLWithTerms([]string{})) != string(q.DisplayHTML()) {
		t.Errorf("DisplayHTMLWithTerms([]) must equal DisplayHTML()")
	}
}

func TestDisplayHTMLWithTermsWrapsBodyMatch(t *testing.T) {
	q := New("MN 22", "the Buddha, MN 22", []string{"the Buddha spoke"})
	got := string(q.DisplayHTMLWithTerms([]string{"buddha"}))
	mustContain(t, got, "<mark>Buddha</mark>") // original case preserved
	mustNotContain(t, got, "<script>")
}

func TestDisplayHTMLWithTermsWrapsCitationID(t *testing.T) {
	q := New("MN 22", "the Buddha, MN 22", []string{`"x"`})
	got := string(q.DisplayHTMLWithTerms([]string{"mn 22"}))
	mustContain(t, got, "<strong><mark>MN 22</mark></strong>")
}

func TestDisplayHTMLWithTermsEscapesNonMatchedScript(t *testing.T) {
	q := New("MN 22", "the Buddha, MN 22", []string{"<script>alert(1)</script> Buddhadatta"})
	got := string(q.DisplayHTMLWithTerms([]string{"buddhadatta"}))
	mustContain(t, got, "&lt;script&gt;alert(1)&lt;/script&gt;")
	mustContain(t, got, "<mark>Buddhadatta</mark>")
	mustNotContain(t, got, "<script>")
}

func TestDisplayHTMLWithTermsEscapesMatchedScript(t *testing.T) {
	q := New("MN 22", "the Buddha, MN 22", []string{"<script>x</script>"})
	got := string(q.DisplayHTMLWithTerms([]string{"<script>"}))
	mustContain(t, got, "<mark>&lt;script&gt;</mark>")
	mustNotContain(t, got, "<script>")
}

func TestDisplayHTMLWithTermsNoMatchNoMark(t *testing.T) {
	q := New("MN 22", "the Buddha, MN 22", []string{`"calm"`})
	got := string(q.DisplayHTMLWithTerms([]string{"zzz"}))
	mustNotContain(t, got, "<mark>")
	if got != string(q.DisplayHTML()) {
		t.Errorf("no-match highlight must equal DisplayHTML()")
	}
}

func TestDisplayHTMLWithTermsMultipleTermsOR(t *testing.T) {
	q := New("MN 22", "the Buddha, MN 22", []string{"the Buddha spoke"})
	got := string(q.DisplayHTMLWithTerms([]string{"buddha", "mn 22"}))
	mustContain(t, got, "<mark>Buddha</mark>")
	mustContain(t, got, "<strong><mark>MN 22</mark></strong>")
}

func TestDisplayHTMLWithTermsMultiPassageOnlyMatching(t *testing.T) {
	q := New("MN 22", "the Buddha, MN 22", []string{"first passage", "the Buddha spoke"})
	got := string(q.DisplayHTMLWithTerms([]string{"buddha"}))
	mustContain(t, got, "<em>first passage</em>")
	mustContain(t, got, "<em>the <mark>Buddha</mark> spoke</em>")
}

func TestHighlight(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		terms []string
		want  string
	}{
		{"no terms is plain escape", "a&b<c>", nil, "a&amp;b&lt;c&gt;"},
		{"empty terms is plain escape", "a&b", []string{}, "a&amp;b"},
		{"preserves original case", "Buddha", []string{"buddha"}, "<mark>Buddha</mark>"},
		{"longest term preferred", "buddha", []string{"bud", "buddha"}, "<mark>buddha</mark>"},
		{"empty term ignored", "abc", []string{"", "a"}, "<mark>a</mark>bc"},
		{"only empty terms -> no mark", "abc", []string{""}, "abc"},
		{"marks every occurrence", "a a a", []string{"a"}, "<mark>a</mark> <mark>a</mark> <mark>a</mark>"},
		{"escaped ampersand in gap", "a & b", []string{"b"}, "a &amp; <mark>b</mark>"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := highlight(tc.in, tc.terms); got != tc.want {
				t.Errorf("highlight(%q, %v) = %q, want %q", tc.in, tc.terms, got, tc.want)
			}
		})
	}
}
