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
