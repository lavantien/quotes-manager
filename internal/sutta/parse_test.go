package sutta

import (
	"strings"
	"testing"
)

func parseOne(t *testing.T, content string) Table {
	t.Helper()
	tab, err := ParseFile("t.md", content)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	return tab
}

func TestParseHeadingVariants(t *testing.T) {
	content := "**I. First.**\n" +
		"1. a ([MN 1](https://suttacentral.net/mn1))\n" +
		"**II. Second (deep).** ([MN 2](https://suttacentral.net/mn2))\n" +
		"**III. Third** tail ([MN 3](https://suttacentral.net/mn3))\n"
	tab := parseOne(t, content)
	wantNames := []string{"First", "Second (deep)", "Third"}
	wantFirst := []string{"mn1", "mn2", "mn3"}
	if len(tab.Groups) != 3 {
		t.Fatalf("groups = %d, want 3", len(tab.Groups))
	}
	for i, g := range tab.Groups {
		if g.Name != wantNames[i] {
			t.Errorf("group %d name = %q, want %q", i+1, g.Name, wantNames[i])
		}
		if len(g.Suttas) != 1 || g.Suttas[0].Muid != wantFirst[i] {
			t.Errorf("group %d suttas = %+v, want %s", i+1, g.Suttas, wantFirst[i])
		}
	}
}

func TestParseNonHeadingBoldLines(t *testing.T) {
	content := "**Curated List.**\n" +
		"nothing here\n" +
		"**I. Real.**\n" +
		"**6\\. Right Effort** ([AN 4.14](https://suttacentral.net/an4.14))\n" +
		"**8\\. Right Composure**\n" +
		"1. ([AN 4.41](https://suttacentral.net/an4.41))\n"
	tab := parseOne(t, content)
	if len(tab.Groups) != 1 {
		t.Fatalf("groups = %d, want 1: bold lines without a roman numeral prefix are not headings", len(tab.Groups))
	}
	g := tab.Groups[0]
	if g.Name != "Real" || len(g.Suttas) != 2 {
		t.Fatalf("group = %q with %d suttas, want Real with 2 (an4.14, an4.41 fold in)", g.Name, len(g.Suttas))
	}
	if g.Suttas[0].Muid != "an4.14" || g.Suttas[1].Muid != "an4.41" {
		t.Errorf("suttas = %+v, want an4.14 then an4.41", g.Suttas)
	}
}

func TestParseIgnoresNonSuttaLinks(t *testing.T) {
	content := "**I. G.**\n" +
		"[v](https://www.youtube.com/watch?v=1)\n" +
		"[a](https://www.accesstoinsight.org/ptf/dhamma/index.html)\n" +
		"[d](https://suttacentral.net/dhp?view=normal&lang=en)\n" +
		"[j](https://suttacentral.net/pitaka/sutta/minor/kn/ja?lang=en)\n" +
		"[vi](https://suttacentral.net/pli-tv-vi)\n" +
		"[e](https://suttacentral.net/editions?lang=en)\n"
	tab := parseOne(t, content)
	if len(tab.Groups) != 1 || len(tab.Groups[0].Suttas) != 0 {
		t.Fatalf("captured %+v, want one empty group: whole-collection and non-sutta links are ignored", tab.Groups)
	}
}

func TestParseLocaleFirstURL(t *testing.T) {
	tab := parseOne(t, "**I. G.**\n[x](https://suttacentral.net/en/mn26)\n")
	if len(tab.Groups[0].Suttas) != 1 || tab.Groups[0].Suttas[0].Muid != "mn26" {
		t.Fatalf("suttas = %+v, want mn26 behind the en locale segment", tab.Groups[0].Suttas)
	}
}

func TestParseLeadingZeros(t *testing.T) {
	tab := parseOne(t, "**I. G.**\n[x](https://suttacentral.net/mn007/en)\n[y](https://suttacentral.net/an2.007)\n")
	if tab.Groups[0].Suttas[0].Muid != "mn7" {
		t.Errorf("muid = %q, want mn7", tab.Groups[0].Suttas[0].Muid)
	}
	if tab.Groups[0].Suttas[1].Muid != "an2.7" {
		t.Errorf("muid = %q, want an2.7", tab.Groups[0].Suttas[1].Muid)
	}
}

func TestParseAnchorAndQueryStrippedFromMuid(t *testing.T) {
	tab := parseOne(t, "**I. G.**\n[x](https://suttacentral.net/mn26?view=normal#5.9.1)\n")
	s := tab.Groups[0].Suttas[0]
	if s.Muid != "mn26" {
		t.Errorf("muid = %q, want mn26", s.Muid)
	}
	if s.Link != "https://suttacentral.net/mn26?view=normal#5.9.1" {
		t.Errorf("link = %q, want the full original url", s.Link)
	}
}

func TestParseEntitiesDecodedInLink(t *testing.T) {
	tab := parseOne(t, "**I. G.**\n[x](https://suttacentral.net/mn1/en/sujato?lang=en&amp;layout=plain)\n")
	if got := tab.Groups[0].Suttas[0].Link; got != "https://suttacentral.net/mn1/en/sujato?lang=en&layout=plain" {
		t.Errorf("link = %q, want entities decoded", got)
	}
}

func TestParseRangeUndottedInheritsChapter(t *testing.T) {
	tab := parseOne(t, "**I. G.**\n[x](https://suttacentral.net/an2.130-140)\n[y](https://suttacentral.net/mn1-10)\n")
	g := tab.Groups[0]
	if len(g.Suttas) != 21 {
		t.Fatalf("suttas = %d, want 21 (11 + 10)", len(g.Suttas))
	}
	if g.Suttas[0].Muid != "an2.130" || g.Suttas[10].Muid != "an2.140" {
		t.Errorf("an2 range ends = %q, %q, want an2.130, an2.140", g.Suttas[0].Muid, g.Suttas[10].Muid)
	}
	if g.Suttas[11].Muid != "mn1" || g.Suttas[20].Muid != "mn10" {
		t.Errorf("mn range ends = %q, %q, want mn1, mn10", g.Suttas[11].Muid, g.Suttas[20].Muid)
	}
	if g.Suttas[5].Display != "AN 2.135" {
		t.Errorf("expanded display = %q, want AN 2.135", g.Suttas[5].Display)
	}
}

func TestParseRangeCapBoundary(t *testing.T) {
	tab := parseOne(t, "**I. G.**\n[x](https://suttacentral.net/mn1-1000)\n")
	if len(tab.Groups[0].Suttas) != 1000 {
		t.Fatalf("suttas = %d, want exactly 1000 at the cap", len(tab.Groups[0].Suttas))
	}
	_, err := ParseFile("t.md", "**I. G.**\n[x](https://suttacentral.net/mn1-1001)\n")
	if err == nil {
		t.Fatal("span of 1001 must be a hard error")
	}
	if !strings.Contains(err.Error(), "span 1001 over cap 1000") {
		t.Errorf("error %q must name the span and the cap", err)
	}
}

func TestParseRangeErrors(t *testing.T) {
	cases := []string{
		"**I. G.**\n[x](https://suttacentral.net/an2.130-3.5)\n",
		"**I. G.**\n[x](https://suttacentral.net/mn10-1)\n",
		"**I. G.**\n[x](https://suttacentral.net/mn99999999999999999999-2)\n",
		"**I. G.**\n[x](https://suttacentral.net/mn2-99999999999999999999)\n",
	}
	for _, content := range cases {
		_, err := ParseFile("t.md", content)
		if err == nil {
			t.Errorf("range %q must be a hard error", content)
			continue
		}
		if strings.Contains(content, "99999") && !strings.Contains(err.Error(), "malformed endpoints") {
			t.Errorf("error %q must name malformed endpoints", err)
		}
	}
}

func TestParseUnknownFamilyIsError(t *testing.T) {
	_, err := ParseFile("t.md", "**I. G.**\n1. fine\n2. ok\n[x](https://suttacentral.net/xy99)\n")
	if err == nil {
		t.Fatal("unknown family with digits must be a hard error")
	}
	if !strings.Contains(err.Error(), "xy99") {
		t.Errorf("error %q must name the url", err)
	}
	if !strings.Contains(err.Error(), "line 4") {
		t.Errorf("error %q must name the offending line", err)
	}
}

func TestParseDhammatalks(t *testing.T) {
	content := "**I. G.**\n" +
		"[a](https://www.dhammatalks.org/suttas/KN/Snp/StNp4.2.html)\n" +
		"[b](https://www.dhammatalks.org/suttas/KN/Snp/StNp4.html)\n" +
		"[c](https://www.dhammatalks.org/suttas/KN/Snp/StNp4_2.html)\n"
	tab := parseOne(t, content)
	g := tab.Groups[0]
	want := []string{"snp4.2", "snp4"}
	if len(g.Suttas) != 2 {
		t.Fatalf("suttas = %d, want 2: the StNp4_2 url collapses to snp4.2 and dedup keeps the first", len(g.Suttas))
	}
	for i, s := range g.Suttas {
		if s.Muid != want[i] {
			t.Errorf("muid %d = %q, want %q", i, s.Muid, want[i])
		}
	}
	if got := g.Suttas[0].Display; got != "a" {
		t.Errorf("snp4.2 display = %q, want the first link text a", got)
	}
}

func TestParseDedupWithinTableFirstWins(t *testing.T) {
	content := "**I. G.**\n[x](https://suttacentral.net/mn1?first)\n" +
		"**II. H.**\n[y](https://suttacentral.net/mn1?second)\n"
	tab := parseOne(t, content)
	if len(tab.Groups[0].Suttas) != 1 || len(tab.Groups[1].Suttas) != 0 {
		t.Fatalf("dedup failed: %+v", tab.Groups)
	}
	if got := tab.Groups[0].Suttas[0].Link; got != "https://suttacentral.net/mn1?first" {
		t.Errorf("link = %q, want the first occurrence", got)
	}
}

func TestParseMalformedMarkdownIsProse(t *testing.T) {
	tab := parseOne(t, "**I. G.**\n[x](https://suttacentral.net/mn1\n[y] (https://suttacentral.net/mn2)\n")
	if len(tab.Groups[0].Suttas) != 0 {
		t.Fatalf("malformed links must be prose, got %+v", tab.Groups[0].Suttas)
	}
}

func TestParseLinksBeforeFirstHeadingIgnored(t *testing.T) {
	tab := parseOne(t, "[x](https://suttacentral.net/mn107)\nintro\n**I. G.**\n")
	if len(tab.Groups) != 1 || len(tab.Groups[0].Suttas) != 0 {
		t.Fatalf("preamble links must be ignored, got %+v", tab.Groups)
	}
}

func TestParseEmptyNameHeadingIsProse(t *testing.T) {
	tab, err := ParseFile("t.md", "**C. **0\n")
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(tab.Groups) != 0 {
		t.Fatalf("groups = %d, want 0: a heading needs name text, degenerate bold matches are prose", len(tab.Groups))
	}
	tab, err = ParseFile("t.md", "**I. .**\n[x](https://suttacentral.net/mn1)\n")
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(tab.Groups) != 0 {
		t.Fatalf("empty-name headings must not open groups: %+v", tab.Groups)
	}
}

func TestParseEmptyLinkTextDerivesDisplay(t *testing.T) {
	tab := parseOne(t, "**I. G.**\n[](https://suttacentral.net/mn26)\n")
	g := tab.Groups[0]
	if len(g.Suttas) != 1 {
		t.Fatalf("suttas = %d, want 1", len(g.Suttas))
	}
	if g.Suttas[0].Muid != "mn26" || g.Suttas[0].Display != "MN 26" {
		t.Errorf("sutta = %+v, want mn26 with derived display MN 26", g.Suttas[0])
	}
}

func TestParseEmptyContent(t *testing.T) {
	tab, err := ParseFile("e.md", "")
	if err != nil {
		t.Fatalf("empty content must parse: %v", err)
	}
	if tab.Name != "e.md" || len(tab.Groups) != 0 {
		t.Fatalf("table = %+v, want named empty table", tab)
	}
}

func TestParseFilesOrderAndErrorPropagation(t *testing.T) {
	tables, err := ParseFiles([]File{
		{Name: "a.md", Content: "**I. A.**\n[x](https://suttacentral.net/mn1)\n"},
		{Name: "b.md", Content: "**I. B.**\n[y](https://suttacentral.net/mn2)\n"},
	})
	if err != nil {
		t.Fatalf("ParseFiles: %v", err)
	}
	if len(tables) != 2 || tables[0].Name != "a.md" || tables[1].Name != "b.md" {
		t.Fatalf("tables = %+v, want both in order", tables)
	}
	_, err = ParseFiles([]File{
		{Name: "a.md", Content: "**I. A.**\n[x](https://suttacentral.net/mn1)\n"},
		{Name: "bad.md", Content: "**I. B.**\n[y](https://suttacentral.net/xy99)\n"},
	})
	if err == nil || !strings.Contains(err.Error(), "bad.md") {
		t.Fatalf("error %v must carry the offending file name", err)
	}
}
