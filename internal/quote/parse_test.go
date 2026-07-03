package quote

import (
	"reflect"
	"testing"
)

func TestSplitCitation(t *testing.T) {
	cases := []struct {
		name         string
		line         string
		wantPassage  string
		wantCitation string
		wantOK       bool
	}{
		{"plain id", `...suffering." - MN 38`, `...suffering."`, "MN 38", true},
		{"attribution", `...couples." - the Buddha, MN 22`, `...couples."`, "the Buddha, MN 22", true},
		{"url stripped", `...discard it." - AN 4.180 ( https://suttacentral.net/an4.180 )`,
			`...discard it."`, "AN 4.180", true},
		{"no closing quote", `the householder left. - MN 87`, `the householder left.`, "MN 87", true},
		{"range id", `...untrue persons." - AN 1.278-286`, `...untrue persons."`, "AN 1.278-286", true},
		{"KN sub-book", `...rhino." - KN Snp 2.14`, `...rhino."`, "KN Snp 2.14", true},
		{"abbrev vinaya", `...couples." - Tv Vi Bu Pj1`, `...couples."`, "Tv Vi Bu Pj1", true},
		{"full vinaya", `...couples." - pli-tv-bu-vb-pj1#5.11.20`, `...couples."`, "pli-tv-bu-vb-pj1#5.11.20", true},
		{"multi-comma attribution", `...diligence." - These were the Buddha's, the Realized One's last words, DN 16`,
			`...diligence."`, "These were the Buddha's, the Realized One's last words, DN 16", true},
		{"mid-text dash stays", `some - text - MN 8`, `some - text`, "MN 8", true},
		{"plain prose", `This is plain prose with no citation.`, `This is plain prose with no citation.`, "", false},
		{"section title", `(A) What is the Buddha's attitude?`, `(A) What is the Buddha's attitude?`, "", false},
		{"prose sutta mention", `...hidden in a jungle (SN 12.65) and think...`, `...hidden in a jungle (SN 12.65) and think...`, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, cit, ok := splitCitation(c.line)
			if ok != c.wantOK {
				t.Fatalf("ok = %v, want %v (passage=%q citation=%q)", ok, c.wantOK, p, cit)
			}
			if ok && (p != c.wantPassage || cit != c.wantCitation) {
				t.Errorf("got (%q, %q), want (%q, %q)", p, cit, c.wantPassage, c.wantCitation)
			}
		})
	}
}

func TestCanonicalSuttaID(t *testing.T) {
	cases := map[string]string{
		"the Buddha, MN 22":        "MN 22",
		"AN 4.180":                 "AN 4.180",
		"SN 55.1:":                 "SN 55.1",
		"Tv Vi Bu Pj1":             "Tv Vi Bu Pj1",
		"pli-tv-bu-vb-pj1#5.11.20": "pli-tv-bu-vb-pj1#5.11.20",
		"KN Snp 2.14":              "KN Snp 2.14",
		"AN 1.278-286":             "AN 1.278-286",
	}
	for in, want := range cases {
		if got := CanonicalSuttaID(in); got != want {
			t.Errorf("CanonicalSuttaID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEnsureAttribution(t *testing.T) {
	cases := map[string]string{
		"MN 8":                              "the Buddha, MN 8",
		"SN 5.2":                            "the Buddha, SN 5.2",
		"pli-tv-bu-vb-pj1#5.11.20":          "the Buddha, pli-tv-bu-vb-pj1#5.11.20",
		"  SN 55.1  ":                       "the Buddha, SN 55.1",
		"the Buddha, MN 22":                 "the Buddha, MN 22",
		"the Buddha to layman Pessa, MN 51": "the Buddha to layman Pessa, MN 51",
		"These were last words, DN 16":      "These were last words, DN 16",
	}
	for in, want := range cases {
		if got := ensureAttribution(in); got != want {
			t.Errorf("ensureAttribution(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestClean(t *testing.T) {
	cases := map[string]string{
		`(1) "In the past..."`: `"In the past..."`,
		`...Jain ascetics.*`:   `...Jain ascetics.`,
		`*Then, secluded...`:   `Then, secluded...`,
		`  spaced  `:           `spaced`,
	}
	for in, want := range cases {
		if got := clean(in); got != want {
			t.Errorf("clean(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseInlineDialogue(t *testing.T) {
	doc := "Intro prose line.\n\n" +
		"\"Is it permanent?\"\n" +
		"\"Impermanent, sir.\"\n" +
		"\"Is it suffering?\"\n" +
		"\"Suffering, sir.\" - SN 22.59\n"
	qs := Parse([]File{{Name: "x.txt", Content: doc}})
	if len(qs) != 1 {
		t.Fatalf("got %d quotes, want 1: %+v", len(qs), qs)
	}
	q := qs[0]
	want := []string{
		`"Is it permanent?"`,
		`"Impermanent, sir."`,
		`"Is it suffering?"`,
		`"Suffering, sir."`,
	}
	if q.SuttaID != "SN 22.59" || q.Citation != "the Buddha, SN 22.59" {
		t.Errorf("SuttaID=%q Citation=%q", q.SuttaID, q.Citation)
	}
	if !reflect.DeepEqual(q.Passages, want) {
		t.Errorf("Passages = %#v", q.Passages)
	}
}

func TestParseNumberedWithAttribution(t *testing.T) {
	doc := "(1) \"I teach suffering.\" - the Buddha, MN 22\n"
	qs := Parse([]File{{Name: "x.txt", Content: doc}})
	if len(qs) != 1 {
		t.Fatalf("got %d quotes, want 1", len(qs))
	}
	q := qs[0]
	if q.SuttaID != "MN 22" || q.Citation != "the Buddha, MN 22" {
		t.Errorf("SuttaID=%q Citation=%q", q.SuttaID, q.Citation)
	}
	if len(q.Passages) != 1 || q.Passages[0] != `"I teach suffering."` {
		t.Errorf("Passages = %#v", q.Passages)
	}
}

func TestParseHeaderCited(t *testing.T) {
	doc := "SN 55.1:\n\n" +
		"\"Now suppose a noble disciple wears rags.\"\n\n" +
		"What four? It's when a noble disciple has confidence.\n\n" +
		".\n"
	qs := Parse([]File{{Name: "x.txt", Content: doc}})
	if len(qs) != 1 {
		t.Fatalf("got %d quotes, want 1: %+v", len(qs), qs)
	}
	q := qs[0]
	want := []string{
		`"Now suppose a noble disciple wears rags."`,
		"What four? It's when a noble disciple has confidence.",
	}
	if q.SuttaID != "SN 55.1" || q.Citation != "the Buddha, SN 55.1" {
		t.Errorf("SuttaID=%q Citation=%q", q.SuttaID, q.Citation)
	}
	if !reflect.DeepEqual(q.Passages, want) {
		t.Errorf("Passages = %#v", q.Passages)
	}
}

func TestParseNarrativeBlock(t *testing.T) {
	doc := "Now a householder's child passed away.\n" +
		"\"Your faculties are unstable.\"\n" +
		"\"Sir, who could think such a thing!\" He left. - MN 87\n"
	qs := Parse([]File{{Name: "x.txt", Content: doc}})
	if len(qs) != 1 || qs[0].SuttaID != "MN 87" {
		t.Fatalf("got %+v", qs)
	}
	if len(qs[0].Passages) != 3 {
		t.Errorf("Passages = %#v", qs[0].Passages)
	}
}

func TestParseProseOnlyYieldsNothing(t *testing.T) {
	doc := "Some prose about AN5.34#7.9.\n\nMore prose with \"quotes\" but no citation.\n"
	qs := Parse([]File{{Name: "x.txt", Content: doc}})
	if len(qs) != 0 {
		t.Fatalf("got %d quotes, want 0: %+v", len(qs), qs)
	}
}

func TestParseDividerEndsHeader(t *testing.T) {
	doc := "AN 10.62:\n\n" +
		"\"A passage.\"\n\n" +
		".\n\n" +
		"\"This is a separate inline quote.\" - MN 8\n"
	qs := Parse([]File{{Name: "x.txt", Content: doc}})
	if len(qs) != 2 {
		t.Fatalf("got %d quotes, want 2: %+v", len(qs), qs)
	}
	if qs[0].SuttaID != "AN 10.62" {
		t.Errorf("first SuttaID = %q", qs[0].SuttaID)
	}
	if qs[1].SuttaID != "MN 8" {
		t.Errorf("second SuttaID = %q", qs[1].SuttaID)
	}
}

func TestParseInlineStanzaBreak(t *testing.T) {
	// A quote split by a blank-line stanza break: the first stanza opens with a
	// quote mark but has no citation; it must be absorbed into the cited stanza.
	doc := "\"First stanza line one.\"\n\n" +
		"Second stanza ends here.\" - SN 5.2\n"
	qs := Parse([]File{{Name: "x.txt", Content: doc}})
	if len(qs) != 1 {
		t.Fatalf("got %d quotes, want 1: %+v", len(qs), qs)
	}
	q := qs[0]
	want := []string{`"First stanza line one."`, `Second stanza ends here."`}
	if q.SuttaID != "SN 5.2" || !reflect.DeepEqual(q.Passages, want) {
		t.Errorf("SuttaID=%q Passages=%#v", q.SuttaID, q.Passages)
	}
}

func TestParseAdjacentCitedQuotesDontMerge(t *testing.T) {
	doc := "\"Quote A.\" - MN 1\n\n\"Quote B.\" - MN 2\n"
	qs := Parse([]File{{Name: "x.txt", Content: doc}})
	if len(qs) != 2 {
		t.Fatalf("got %d quotes, want 2: %+v", len(qs), qs)
	}
	if len(qs[0].Passages) != 1 || len(qs[1].Passages) != 1 {
		t.Errorf("quotes should not merge: %#v", qs)
	}
}

func TestParseEmptyInputs(t *testing.T) {
	for name, in := range map[string][]File{
		"nil":   nil,
		"empty": {},
		"blank": {{Name: "x", Content: ""}},
	} {
		if qs := Parse(in); len(qs) != 0 {
			t.Errorf("%s: Parse = %#v, want no quotes", name, qs)
		}
	}
}

func TestParseDividersOnly(t *testing.T) {
	if qs := Parse([]File{{Name: "x", Content: ".\n\n.\n"}}); len(qs) != 0 {
		t.Errorf("got %d quotes, want 0", len(qs))
	}
}

func TestParseMultiFileOrder(t *testing.T) {
	files := []File{
		{Name: "first.txt", Content: "\"From file one.\" - MN 1\n"},
		{Name: "second.txt", Content: "\"From file two.\" - MN 2\n"},
	}
	qs := Parse(files)
	if len(qs) != 2 {
		t.Fatalf("got %d quotes, want 2", len(qs))
	}
	if qs[0].SuttaID != "MN 1" || qs[1].SuttaID != "MN 2" {
		t.Errorf("order = %s, %s; want MN 1, MN 2", qs[0].SuttaID, qs[1].SuttaID)
	}
	if qs[0].Sources[0] != "first.txt" || qs[1].Sources[0] != "second.txt" {
		t.Errorf("source attribution lost: %#v / %#v", qs[0].Sources, qs[1].Sources)
	}
}

func TestParseHeaderNoPassages(t *testing.T) {
	// A lone header at EOF with no following passages emits nothing.
	if qs := Parse([]File{{Name: "x", Content: "MN 22:\n"}}); len(qs) != 0 {
		t.Errorf("got %d quotes, want 0", len(qs))
	}
}

func TestParseHeaderThenHeader(t *testing.T) {
	// A second header flushes the first (which had no passages) without emitting.
	doc := "MN 1:\n\nMN 2:\n\n\"Absorbed passage.\" - SN 1\n"
	qs := Parse([]File{{Name: "x", Content: doc}})
	if len(qs) != 1 {
		t.Fatalf("got %d quotes, want 1: %+v", len(qs), qs)
	}
	if qs[0].SuttaID != "MN 2" {
		t.Errorf("SuttaID = %q, want MN 2 (second header wins)", qs[0].SuttaID)
	}
}

func TestParseOrphanDroppedAtEOF(t *testing.T) {
	// A quote-opening fragment with no cited terminator is dropped at EOF.
	if qs := Parse([]File{{Name: "x", Content: "\"An orphan with no citation.\n"}}); len(qs) != 0 {
		t.Errorf("got %d quotes, want 0 (orphan dropped)", len(qs))
	}
}

func TestSplitBlocks(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		if got := splitBlocks(""); len(got) != 0 {
			t.Errorf("splitBlocks(\"\") = %#v, want none", got)
		}
	})
	t.Run("no trailing newline", func(t *testing.T) {
		got := splitBlocks("a\nb")
		if len(got) != 1 || len(got[0].lines) != 2 {
			t.Errorf("got %#v, want one 2-line block", got)
		}
	})
	t.Run("CRLF input", func(t *testing.T) {
		got := splitBlocks("a\r\nb")
		if len(got) != 1 || got[0].lines[0] != "a" || got[0].lines[1] != "b" {
			t.Errorf("CRLF handling = %#v", got)
		}
	})
	t.Run("many blank lines", func(t *testing.T) {
		if got := splitBlocks("a\n\n\n\nb"); len(got) != 2 {
			t.Errorf("got %d blocks, want 2", len(got))
		}
	})
}

func TestIsDivider(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  bool
	}{
		{"empty", nil, false},
		{"single", []string{"."}, true},
		{"multi", []string{".", "."}, true},
		{"mixed", []string{".", "x"}, false},
		{"non", []string{"text"}, false},
	}
	for _, c := range cases {
		if got := isDivider(c.lines); got != c.want {
			t.Errorf("isDivider(%v) = %v, want %v", c.lines, got, c.want)
		}
	}
}

func TestStartsQuote(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{`"straight"`, true},
		{"“curly”", true},
		{"plain text", false},
		{`  "indented`, true},
	}
	for _, c := range cases {
		if got := startsQuote(c.line); got != c.want {
			t.Errorf("startsQuote(%q) = %v, want %v", c.line, got, c.want)
		}
	}
}

func TestCleanUnderscoreAndCombined(t *testing.T) {
	cases := map[string]string{
		"_underscore_":      "underscore",
		"*_combined_*":      "combined",
		"(3) *quoted text*": "quoted text",
		"_":                 "",
	}
	for in, want := range cases {
		if got := clean(in); got != want {
			t.Errorf("clean(%q) = %q, want %q", in, got, want)
		}
	}
}
