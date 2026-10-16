package web

import (
	"net/url"
	"strings"
	"testing"
)

const canary = "zq7unmark"

func FuzzWebEscape(f *testing.F) {
	for _, s := range []string{
		`<script>alert(1)</script>`,
		`"><img src=x onerror=alert(1)>`,
		`<a href="javascript:alert(1)">x</a>`,
		"āīūṭḍṅñḷṃ",
		"dhamma 1.03",
		"",
		"pli:mn1:0.1",
		`a"b<c>d&e'f`,
		"]]>", "<!--", "%20%3C<script>", "\x00\x01\x02",
		`>0`, `"`, `">`, `<`, `a<b`,
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		hostile := canary + raw + canary
		pages := []struct {
			name string
			data any
		}{
			{PageText, TextPage{
				Page: Page{Title: hostile}, Muid: hostile, Heading: hostile,
				Segments: []SegmentRow{{
					Muid: hostile, SegKey: hostile, Pali: hostile, En: hostile, Author: hostile,
					Tokens: []SegmentToken{{Text: hostile, IsWord: true}, {Text: " ", IsWord: false}},
				}},
				SegmentsPager: &Pager{Path: "/texts/x", Query: mapSlice(hostile), Offset: 0, Limit: 10, Total: 1},
			}},
			{PageWord, WordPage{
				Page: Page{Title: hostile}, Lemma: hostile,
				Entries: []WordEntryView{{Lemma: hostile, POS: hostile, Meaning: hostile, Root: hostile, FamilyRoot: hostile, Segments: 1}},
				Total:   1, Refs: []UsageRef{{Muid: hostile, SegKey: hostile}},
				RefsPager: &Pager{Path: "/words/x", Offset: 0, Limit: 10, Total: 1},
				GraphURL:  "/api/v1/graph/word?token=x",
			}},
			{PageSearch, SearchPage{
				Page: Page{Title: hostile}, Text: hostile,
				Results: []SegmentRow{{Muid: hostile, SegKey: hostile, Pali: hostile, En: hostile}},
			}},
			{PageWords, WordsPage{
				Page: Page{Title: hostile}, Form: hostile,
				Entries: []WordEntryView{{Lemma: hostile, Meaning: hostile}},
			}},
			{PageDiscrepancies, DiscrepanciesPage{
				Page: Page{Title: hostile}, RunID: 1, Severity: hostile, Check: hostile,
				Rows:  []DiscrepancyRow{{ID: 1, Check: hostile, Severity: hostile, Muid: hostile, Detail: hostile}},
				Pager: &Pager{Path: "/discrepancies", Query: mapSlice(hostile), Offset: 0, Limit: 10, Total: 1},
			}},
			{PageTexts, TextsPage{Page: Page{Title: hostile}, Divisions: []*TreeBranch{
				{
					Key: hostile, Label: hostile, Type: hostile, Muid: hostile, Expandable: true,
					Children: []*TreeBranch{{Key: hostile, Label: hostile, Type: hostile}},
				},
			}}},
		}
		partials := []struct {
			name string
			data any
		}{
			{PartialSegmentsList, []SegmentRow{{
				Muid: hostile, SegKey: hostile, Pali: hostile, En: hostile,
				Tokens: []SegmentToken{{Text: hostile, IsWord: true}},
			}}},
			{PartialSearchResults, []SegmentRow{{Muid: hostile, SegKey: hostile, Pali: hostile, En: hostile}}},
			{PartialWordEntries, []WordEntryView{{Lemma: hostile, Meaning: hostile, POS: hostile, Root: hostile}}},
			{PartialUsageRefs, []UsageRef{{Muid: hostile, SegKey: hostile}}},
			{PartialDiscrepancyRows, []DiscrepancyRow{{ID: 1, Check: hostile, Severity: hostile, Muid: hostile, Detail: hostile}}},
			{PartialTreeBranch, TreeBranch{Key: hostile, Label: hostile, Type: hostile, Muid: hostile, Expandable: true}},
		}
		dangerous := strings.Contains(hostile, `<`)
		for _, p := range pages {
			var b strings.Builder
			if err := Render(&b, p.name, p.data); err != nil {
				t.Fatalf("render page %s: %v", p.name, err)
			}
			if dangerous && strings.Contains(b.String(), hostile) {
				t.Fatalf("page %s leaked hostile %q raw", p.name, hostile)
			}
		}
		for _, p := range partials {
			var b strings.Builder
			if err := RenderPartial(&b, p.name, p.data); err != nil {
				t.Fatalf("render partial %s: %v", p.name, err)
			}
			if dangerous && strings.Contains(b.String(), hostile) {
				t.Fatalf("partial %s leaked hostile %q raw", p.name, hostile)
			}
		}
	})
}

func mapSlice(v string) url.Values {
	return url.Values{"q": {v}}
}
