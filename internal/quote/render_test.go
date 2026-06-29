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
