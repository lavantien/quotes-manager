package web

import (
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/lavantien/quotes-manager/web"
)

func TestMain(m *testing.M) {
	var wg sync.WaitGroup
	errs := make(chan error, 64)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := Init(); err != nil {
				errs <- err
			}
			if err := Init(); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func repDashboard() DashboardPage {
	return DashboardPage{
		Page: Page{Title: "dashboard"},
		Stats: StatsGrid{Rows: []StatRow{
			{Label: "node:text", Value: 1234567},
			{Label: "edge:HAS_WORD", Value: 9876543},
		}},
		Coverage: []CoverRowView{
			{Division: "mn", Texts: 152, Covered: 149},
			{Division: "pli-tv", Texts: 0, Covered: 0},
		},
		CoverageBars: &BarChart{Bars: []Bar{{Label: "mn", Value: 149}, {Label: "an", Value: 92}}},
		Run:          &RunView{ID: 7, Label: "all", Started: "2026-10-07 09:00", Finished: "2026-10-07 09:03"},
		Job:          &JobView{ID: 7, Stage: "load", State: "done", StatusURL: "/pipeline/status"},
	}
}

func repTexts() TextsPage {
	return TextsPage{
		Page: Page{Title: "texts"},
		Divisions: []*TreeBranch{
			{Key: "pli-tv", Label: "pli-tv", Type: "collection", Expandable: true},
			{Key: "mn", Label: "mn", Type: "division", Children: []*TreeBranch{
				{Key: "mn1", Label: "mulapariyaya", Type: "text", Muid: "mn1"},
			}},
		},
	}
}

func repText() TextPage {
	return TextPage{
		Page:    Page{Title: "mn1"},
		Muid:    "mn1",
		Heading: "mulapariyayasutta",
		Segments: []SegmentRow{{
			Muid: "mn1", SegKey: "0.1", En: "root case", Author: "sujato",
			Tokens: []SegmentToken{
				{Text: "dhamma 1.03", IsWord: true},
				{Text: " ca ", IsWord: false},
				{Text: "āīūṭḍṅñḷṃ", IsWord: true},
			},
		}},
		SegmentsPager: &Pager{Path: "/texts/mn1", Offset: 0, Limit: 1, Total: 3},
	}
}

func repSearch() SearchPage {
	return SearchPage{
		Page:    Page{Title: "search"},
		Text:    "dhamma",
		Lang:    "pali",
		Results: []SegmentRow{{Muid: "mn1", SegKey: "0.1", Pali: "dhamma ca", En: "mind and"}},
	}
}

func repWords() WordsPage {
	return WordsPage{
		Page: Page{Title: "words"},
		Form: "dhamma",
		Entries: []WordEntryView{{
			Lemma: "dhamma 1.03", POS: "masculine", Meaning: "mind; object",
			Root: "dhṛ", FamilyRoot: "dhamma", Segments: 1234567,
		}},
	}
}

func repWord() WordPage {
	return WordPage{
		Page:  Page{Title: "dhamma 1.03"},
		Lemma: "dhamma 1.03",
		Entries: []WordEntryView{{
			Lemma: "dhamma 1.03", POS: "masculine", Meaning: "mind; object",
			Root: "dhṛ", Segments: 999,
		}},
		Total:     999,
		Refs:      []UsageRef{{Muid: "mn1", SegKey: "0.1"}},
		RefsPager: &Pager{Path: "/words/dhamma 1.03", Offset: 0, Limit: 1, Total: 999},
		GraphURL:  "/api/v1/graph/word?token=dhamma",
	}
}

func repGraph() GraphPage {
	return GraphPage{
		Page: Page{Title: "graph"}, Dataset: "word", Param: "dhamma",
		Datasets: []GraphDataset{
			{Name: "word", Label: "word neighborhood", APIPath: "/api/v1/graph/word", Param: "token", Placeholder: "token, e.g. dhamma", Layout: "concentric"},
			{Name: "root", Label: "root family", APIPath: "/api/v1/graph/root", Param: "root", Placeholder: "root key", Layout: "concentric"},
			{Name: "tree", Label: "division tree", APIPath: "/api/v1/graph/tree", Param: "division", Placeholder: "division key", Layout: "breadthfirst", Options: []string{"mn", "an"}},
		},
	}
}

func repAnalytics(console bool) AnalyticsPage {
	return AnalyticsPage{
		Page:     Page{Title: "analytics"},
		Catalog:  []Analytic{{Name: "top-headwords", Description: "by occurrence count"}, {Name: "coverage-by-division"}},
		Selected: "top-headwords", Division: "mn", Limit: 20,
		Cols: []string{"headword", "n"}, Rows: [][]string{{"dhamma 1.03", "999"}},
		Chart:          &BarChart{Bars: []Bar{{Label: "dhamma 1.03", Value: 999}, {Label: "anicca", Value: 400}}},
		CSVURL:         "/api/v1/analytics/top-headwords.csv",
		ConsoleEnabled: console, SQL: "select 1",
		ConsoleCols: []string{"1"}, ConsoleRows: [][]string{{"1"}},
	}
}

func repPipeline() PipelinePage {
	return PipelinePage{
		Page:   Page{Title: "pipeline"},
		Stages: []string{"sync", "load", "triangulate", "all"},
		Job:    &JobView{ID: 3, Stage: "load", State: "running", StatusURL: "/pipeline/status", Polling: true},
		Log: &JobLogView{
			Lines:   []LogLineView{{Seq: 7, Text: "manifest: 2 files"}},
			NextSeq: 8, LogURL: "/pipeline/log", Polling: true,
		},
	}
}

func repDiscrepancies() DiscrepanciesPage {
	return DiscrepanciesPage{
		Page: Page{Title: "discrepancies"}, RunID: 7, Severity: "error", Check: "D001",
		Rows: []DiscrepancyRow{
			{ID: 11, Check: "D001", Severity: "error", Muid: "mn1", Detail: "unknown form xyz"},
			{ID: 12, Check: "B002", Severity: "info", Detail: "no muid"},
		},
		Pager: &Pager{Path: "/discrepancies", Query: url.Values{"severity": {"error"}, "check": {"D001"}}, Offset: 0, Limit: 50, Total: 120},
	}
}

func render(t *testing.T, name string, data any) string {
	t.Helper()
	var b strings.Builder
	if err := Render(&b, name, data); err != nil {
		t.Fatalf("render %s: %v", name, err)
	}
	return b.String()
}

func renderPartial(t *testing.T, name string, data any) string {
	t.Helper()
	var b strings.Builder
	if err := RenderPartial(&b, name, data); err != nil {
		t.Fatalf("render partial %s: %v", name, err)
	}
	return b.String()
}

func TestTemplateNameSets(t *testing.T) {
	if got, want := len(Pages()), 11; got != want {
		t.Fatalf("Pages() = %d entries, want %d: %v", got, want, Pages())
	}
	if got, want := len(Partials()), 12; got != want {
		t.Fatalf("Partials() = %d entries, want %d: %v", got, want, Partials())
	}
}

func TestRenderPages(t *testing.T) {
	cases := []struct {
		name    string
		data    any
		markers []string
		absent  []string
	}{
		{PageDashboard, repDashboard(), []string{`class="stats-grid"`, "1,234,567", "9,876,543", "mn", "98%", "run 7", "badge-done"}, nil},
		{PageTexts, repTexts(), []string{`hx-get="/texts/branch?node=pli-tv"`, `href="/texts/mn1"`}, nil},
		{PageText, repText(), []string{`href="/words?form=dhamma%201.03"`, `href="/words?form=%C4%81%C4%AB%C5%AB%E1%B9%AD%E1%B8%8D%E1%B9%85%C3%B1%E1%B8%B7%E1%B9%83"`, `id="0.1"`, "sujato"}, nil},
		{PageSearch, repSearch(), []string{`hx-swap="morph:innerHTML"`, `href="/texts/mn1#0.1"`}, nil},
		{PageWords, repWords(), []string{`href="/words/dhamma%201.03"`, "masculine"}, nil},
		{PageWord, repWord(), []string{`href="/texts/mn1#0.1"`, `data-graph-url="/api/v1/graph/word?token=dhamma"`, "999 segments", "showing 1 to 1 of 999"}, nil},
		{PageGraph, repGraph(), []string{`data-api="/api/v1/graph/word"`, `data-layout="concentric"`, `data-layout="breadthfirst"`, `id="graph-options-tree"`}, nil},
		{PageAnalytics, repAnalytics(true), []string{"download csv", "sql console", `<textarea`, "top-headwords"}, nil},
		{PageAnalytics, repAnalytics(false), []string{"download csv"}, []string{"sql console"}},
		{PagePipeline, repPipeline(), []string{`value="sync"`, `value="triangulate"`, "by design", `hx-get="/pipeline/status"`, `hx-trigger="every 2s"`, `data-seq="7"`, `data-next-seq="8"`}, nil},
		{PageDiscrepancies, repDiscrepancies(), []string{`href="/texts/mn1"`, "sev-error", `href="/discrepancies?check=D001&amp;limit=50&amp;offset=50&amp;severity=error"`}, nil},
		{PageNotFound, NotFoundPage{Page: Page{Title: "not found"}, Path: "/nope"}, []string{"does not exist"}, nil},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out := render(t, tc.name, tc.data)
			if !strings.Contains(out, "</html>") {
				t.Fatalf("page %s render incomplete:\n%s", tc.name, out)
			}
			for _, m := range tc.markers {
				if !strings.Contains(out, m) {
					t.Errorf("page %s missing marker %q in:\n%s", tc.name, m, out)
				}
			}
			for _, m := range tc.absent {
				if strings.Contains(out, m) {
					t.Errorf("page %s must not contain %q", tc.name, m)
				}
			}
		})
	}
}

func TestRenderPartials(t *testing.T) {
	cases := []struct {
		name    string
		data    any
		markers []string
	}{
		{PartialStatsGrid, repDashboard().Stats, []string{`class="stats-grid"`, "1,234,567"}},
		{PartialBars, BarChart{Bars: []Bar{{Label: "mn", Value: 200}, {Label: "an", Value: 100}}}, []string{"<svg", `width="380"`, `width="190"`, "200"}},
		{PartialTreeBranch, repTexts().Divisions[0], []string{`hx-get="/texts/branch?node=pli-tv"`, `hx-swap="outerHTML"`}},
		{PartialSegmentsList, repText().Segments, []string{`href="/words?form=dhamma%201.03"`, "root case"}},
		{PartialSearchResults, repSearch().Results, []string{`href="/texts/mn1#0.1"`}},
		{PartialWordEntries, repWords().Entries, []string{`href="/words/dhamma%201.03"`, "masculine"}},
		{PartialUsageRefs, repWord().Refs, []string{`href="/texts/mn1#0.1"`, "mn1:0.1"}},
		{PartialJobStatus, JobView{ID: 1, Stage: "triangulate", State: "error", Err: "boom"}, []string{"badge-error", "on purpose"}},
		{PartialJobLog, repPipeline().Log, []string{`data-log-url="/pipeline/log"`, `data-seq="7"`, "manifest: 2 files"}},
		{PartialDiscrepancyRows, repDiscrepancies().Rows, []string{`href="/texts/mn1"`, "unknown form xyz"}},
		{PartialPager, repDiscrepancies().Pager, []string{"showing 1 to 50 of 120"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out := renderPartial(t, tc.name, tc.data)
			for _, m := range tc.markers {
				if !strings.Contains(out, m) {
					t.Errorf("partial %s missing marker %q in:\n%s", tc.name, m, out)
				}
			}
		})
	}
}

func TestPartialsEmptyStates(t *testing.T) {
	if out := renderPartial(t, PartialSearchResults, []SegmentRow{}); !strings.Contains(out, "no results") {
		t.Errorf("empty search results must say so, got:\n%s", out)
	}
	if out := renderPartial(t, PartialPager, (*Pager)(nil)); strings.TrimSpace(out) != "" {
		t.Errorf("nil pager must render nothing, got:\n%s", out)
	}
	if out := renderPartial(t, PartialBars, BarChart{}); strings.Contains(out, "<svg") {
		t.Errorf("empty bar chart must render no svg, got:\n%s", out)
	}
}

func TestUnknownTemplates(t *testing.T) {
	err := Render(&strings.Builder{}, "nope", nil)
	if err == nil || !strings.Contains(err.Error(), `unknown page template "nope"`) {
		t.Fatalf("Render(nope) = %v, want unknown page template error", err)
	}
	err = RenderPartial(&strings.Builder{}, "nope", nil)
	if err == nil || !strings.Contains(err.Error(), `unknown partial template "nope"`) {
		t.Fatalf("RenderPartial(nope) = %v, want unknown partial template error", err)
	}
	if err := Render(&strings.Builder{}, PartialBars, nil); err == nil {
		t.Fatal("Render of a partial name must error")
	}
	if err := RenderPartial(&strings.Builder{}, PageDashboard, nil); err == nil {
		t.Fatal("RenderPartial of a page name must error")
	}
}

func TestIntcomma(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0"},
		{1, "1"},
		{12, "12"},
		{123, "123"},
		{1234, "1,234"},
		{12345, "12,345"},
		{123456, "123,456"},
		{1234567, "1,234,567"},
		{-1234567, "-1,234,567"},
		{999999999, "999,999,999"},
	}
	for _, tc := range cases {
		if got := intcomma(tc.in); got != tc.want {
			t.Errorf("intcomma(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPager(t *testing.T) {
	p := &Pager{Path: "/discrepancies", Query: url.Values{"severity": {"error"}}, Offset: 0, Limit: 50, Total: 120}
	if !p.HasNext() || p.HasPrev() {
		t.Fatalf("first page flags wrong: next=%v prev=%v", p.HasNext(), p.HasPrev())
	}
	if got, want := p.Start(), int64(1); got != want {
		t.Errorf("Start() = %d, want %d", got, want)
	}
	if got, want := p.End(), int64(50); got != want {
		t.Errorf("End() = %d, want %d", got, want)
	}
	if got, want := string(p.NextHref()), "/discrepancies?limit=50&offset=50&severity=error"; got != want {
		t.Errorf("NextHref() = %q, want %q", got, want)
	}
	p.Offset = 100
	if !p.HasPrev() || p.HasNext() {
		t.Fatalf("last page flags wrong: next=%v prev=%v", p.HasNext(), p.HasPrev())
	}
	if got, want := string(p.PrevHref()), "/discrepancies?limit=50&offset=50&severity=error"; got != want {
		t.Errorf("PrevHref() = %q, want %q", got, want)
	}
	var nilPager *Pager
	if nilPager.HasPrev() || nilPager.HasNext() || nilPager.Start() != 0 || nilPager.End() != 0 {
		t.Error("nil pager methods must be inert")
	}
}

func TestBarChartGeometry(t *testing.T) {
	c := BarChart{Bars: []Bar{{Label: "a", Value: 100}, {Label: "b", Value: 50}, {Label: "c", Value: 0}, {Label: "d", Value: -5}}}
	if got, want := c.Height(), 80; got != want {
		t.Errorf("Height() = %d, want %d", got, want)
	}
	rows := c.Rows()
	wantW := []int{380, 190, 0, 0}
	for i, w := range wantW {
		if rows[i].W != w {
			t.Errorf("rows[%d].W = %d, want %d", i, rows[i].W, w)
		}
		if rows[i].ValueX != 180+w+6 {
			t.Errorf("rows[%d].ValueX = %d, want %d", i, rows[i].ValueX, 180+w+6)
		}
	}
	if rows[0].Y != 0 || rows[1].Y != 20 {
		t.Errorf("row Y offsets wrong: %d %d", rows[0].Y, rows[1].Y)
	}
}

func TestEscapeHostileData(t *testing.T) {
	const js = `<script>alert(1)</script>`
	const attr = `"><img src=x onerror=alert(1)>`
	const pali = `āīūṭḍṅñḷṃ`

	t.Run("script in word page", func(t *testing.T) {
		wp := repWord()
		wp.Lemma = js
		wp.Entries[0].Lemma = js
		wp.Entries[0].Meaning = js
		out := render(t, PageWord, wp)
		if strings.Contains(out, js) {
			t.Fatalf("raw script tag leaked:\n%s", out)
		}
		if !strings.Contains(out, "&lt;script&gt;") {
			t.Errorf("script must render escaped as text, got:\n%s", out)
		}
	})

	t.Run("attr breakout in words page", func(t *testing.T) {
		wp := repWords()
		wp.Form = attr
		wp.Entries[0].Lemma = attr
		out := render(t, PageWords, wp)
		if strings.Contains(out, attr) {
			t.Fatalf("raw attr breakout string leaked:\n%s", out)
		}
		if !strings.Contains(out, `href="/words/%22%3E%3Cimg%20src`) {
			t.Errorf("lemma href must be path-escaped, got:\n%s", out)
		}
	})

	t.Run("pali unicode roundtrip", func(t *testing.T) {
		tp := repText()
		tp.Segments[0].Tokens = []SegmentToken{{Text: pali, IsWord: true}}
		out := render(t, PageText, tp)
		if !strings.Contains(out, ">"+pali+"</a>") {
			t.Errorf("pali token must survive text escaping, got:\n%s", out)
		}
		enc := url.PathEscape(pali)
		if enc == pali || !strings.Contains(out, enc) {
			t.Errorf("pali token href must be url-escaped as %q, got:\n%s", enc, out)
		}
	})

	t.Run("hostile across surfaces", func(t *testing.T) {
		for _, hostile := range []string{js, attr, `a"b<c>d&e'f`, "]]>", "\x00nul", ">0", `"`, "<x"} {
			tp := repText()
			tp.Muid = hostile
			tp.Heading = hostile
			tp.Segments[0].Pali = hostile
			tp.Segments[0].En = hostile
			tp.Segments[0].Tokens = []SegmentToken{{Text: hostile, IsWord: true}}
			out := render(t, PageText, tp)
			if strings.Contains(hostile, "<") && strings.Contains(out, hostile) {
				t.Errorf("hostile %q leaked raw into text page:\n%s", hostile, out)
			}
			sp := repSearch()
			sp.Text = hostile
			sp.Results[0].Pali = hostile
			sp.Results[0].En = hostile
			out = render(t, PageSearch, sp)
			if strings.Contains(hostile, "<") && strings.Contains(out, hostile) {
				t.Errorf("hostile %q leaked raw into search page:\n%s", hostile, out)
			}
		}
	})
}

func TestNoExternalAssetReferences(t *testing.T) {
	reps := []struct {
		name string
		data any
	}{
		{PageDashboard, repDashboard()},
		{PageTexts, repTexts()},
		{PageText, repText()},
		{PageSearch, repSearch()},
		{PageWords, repWords()},
		{PageWord, repWord()},
		{PageGraph, repGraph()},
		{PageAnalytics, repAnalytics(true)},
		{PagePipeline, repPipeline()},
		{PageDiscrepancies, repDiscrepancies()},
		{PageNotFound, NotFoundPage{Page: Page{Title: "not found"}, Path: "/x"}},
	}
	for _, rep := range reps {
		out := render(t, rep.name, rep.data)
		for _, m := range []string{`src="/static/htmx.min.js"`, `src="/static/cytoscape.min.js"`, `src="/static/canon.js"`, `href="/static/canon.css"`, `href="/static/nav.css"`} {
			if !strings.Contains(out, m) {
				t.Errorf("page %s missing vendored asset %q", rep.name, m)
			}
		}
		for _, m := range []string{`<nav class="nav">`, `class="nav-brand" href="/">qm</a>`, `href="/dashboard">overview</a>`, `href="/texts">texts</a>`, `href="/discrepancies">discrepancies</a>`} {
			if !strings.Contains(out, m) {
				t.Errorf("page %s missing shared nav marker %q", rep.name, m)
			}
		}
		if strings.Contains(out, "sc-graph") {
			t.Errorf("page %s still carries the old sc-graph brand", rep.name)
		}
		if i := strings.Index(out, `="http://`); i >= 0 {
			t.Errorf("page %s references an external http asset near %q", rep.name, out[i:i+60])
		}
		if i := strings.Index(out, `="https://`); i >= 0 {
			t.Errorf("page %s references an external https asset near %q", rep.name, out[i:i+60])
		}
	}
}

func TestCanonAssetBudgets(t *testing.T) {
	sub, err := fs.Sub(web.Static, "static")
	if err != nil {
		t.Fatal(err)
	}
	for name, budget := range map[string]int{"canon.css": 400, "canon.js": 300} {
		raw, err := fs.ReadFile(sub, name)
		if err != nil {
			t.Fatalf("asset %s: %v", name, err)
		}
		if len(raw) == 0 {
			t.Errorf("asset %s is empty", name)
		}
		if strings.Contains(string(raw), "http://") || strings.Contains(string(raw), "https://") {
			t.Errorf("asset %s references a remote url", name)
		}
		if n := strings.Count(string(raw), "\n"); n > budget {
			t.Errorf("%s has %d lines, budget is %d", name, n, budget)
		}
	}
}

func TestRenderConcurrent(t *testing.T) {
	reps := []struct {
		name string
		data any
	}{
		{PageDashboard, repDashboard()},
		{PageText, repText()},
		{PageWord, repWord()},
		{PagePipeline, repPipeline()},
		{PageDiscrepancies, repDiscrepancies()},
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		for _, rep := range reps {
			rep := rep
			wg.Add(1)
			go func() {
				defer wg.Done()
				var b strings.Builder
				if err := Render(&b, rep.name, rep.data); err != nil {
					t.Errorf("concurrent render %s: %v", rep.name, err)
				}
			}()
		}
	}
	wg.Wait()
}
