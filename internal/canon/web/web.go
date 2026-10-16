package web

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

//go:embed templates assets
var content embed.FS

const (
	PageDashboard     = "dashboard"
	PageTexts         = "texts"
	PageText          = "text"
	PageSearch        = "search"
	PageWords         = "words"
	PageWord          = "word"
	PageGraph         = "graph"
	PageAnalytics     = "analytics"
	PagePipeline      = "pipeline"
	PageDiscrepancies = "discrepancies"
	PageNotFound      = "notfound"
)

const (
	PartialTreeBranch      = "tree_branch"
	PartialSegmentsList    = "segments_list"
	PartialSearchResults   = "search_results"
	PartialWordEntries     = "word_entries"
	PartialUsageRefs       = "usage_refs"
	PartialJobStatus       = "job_status"
	PartialJobLog          = "job_log"
	PartialPipelineLive    = "pipeline_live"
	PartialDiscrepancyRows = "discrepancy_rows"
	PartialStatsGrid       = "stats_grid"
	PartialBars            = "bars"
	PartialPager           = "pager"
)

var pageNames = []string{
	PageDashboard, PageTexts, PageText, PageSearch, PageWords, PageWord,
	PageGraph, PageAnalytics, PagePipeline, PageDiscrepancies, PageNotFound,
}

var partialNames = []string{
	PartialTreeBranch, PartialSegmentsList, PartialSearchResults,
	PartialWordEntries, PartialUsageRefs, PartialJobStatus, PartialJobLog,
	PartialPipelineLive, PartialDiscrepancyRows, PartialStatsGrid,
	PartialBars, PartialPager,
}

func Pages() []string { return append([]string(nil), pageNames...) }

func Partials() []string { return append([]string(nil), partialNames...) }

var (
	initOnce sync.Once
	pageTpls map[string]*template.Template
	partials *template.Template
	assetsFS fs.FS
	initErr  error
)

func Init() error {
	initOnce.Do(build)
	return initErr
}

func build() {
	assetsFS, initErr = fs.Sub(content, "assets")
	if initErr != nil {
		initErr = fmt.Errorf("web: assets: %w", initErr)
		return
	}
	fm := template.FuncMap{"urlescape": urlescape, "intcomma": intcomma}
	base := template.New("base").Funcs(fm)
	files := []string{"templates/layout.tmpl"}
	for _, n := range partialNames {
		files = append(files, "templates/"+n+".tmpl")
	}
	for _, f := range files {
		if base, initErr = base.ParseFS(content, f); initErr != nil {
			initErr = fmt.Errorf("web: parse %s: %w", f, initErr)
			return
		}
	}
	partials = base
	pageTpls = make(map[string]*template.Template, len(pageNames))
	for _, n := range pageNames {
		clone, err := base.Clone()
		if err != nil {
			initErr = fmt.Errorf("web: clone for %s: %w", n, err)
			return
		}
		t, err := clone.ParseFS(content, "templates/"+n+".tmpl")
		if err != nil {
			initErr = fmt.Errorf("web: parse page %s: %w", n, err)
			return
		}
		pageTpls[n] = t
	}
}

func Render(w io.Writer, name string, data any) error {
	if err := Init(); err != nil {
		return err
	}
	t, ok := pageTpls[name]
	if !ok {
		return fmt.Errorf("web: unknown page template %q", name)
	}
	return t.ExecuteTemplate(w, "layout", data)
}

func RenderPartial(w io.Writer, name string, data any) error {
	if err := Init(); err != nil {
		return err
	}
	if partials.Lookup(name) == nil {
		return fmt.Errorf("web: unknown partial template %q", name)
	}
	return partials.ExecuteTemplate(w, name, data)
}

func Assets() fs.FS {
	if err := Init(); err != nil {
		return nil
	}
	return assetsFS
}

func urlescape(s string) template.URL { return template.URL(url.PathEscape(s)) }

func intcomma(n int64) string {
	s := strconv.FormatInt(n, 10)
	sign := ""
	if strings.HasPrefix(s, "-") {
		sign, s = "-", s[1:]
	}
	var b strings.Builder
	b.WriteString(sign)
	for i := 0; i < len(s); i++ {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

type Page struct {
	Title string
}

type StatRow struct {
	Label string
	Value int64
}

type StatsGrid struct {
	Rows []StatRow
}

type Bar struct {
	Label string
	Value int64
}

type BarChart struct {
	Bars []Bar
}

type BarRow struct {
	Label  string
	Value  int64
	Y      int
	W      int
	ValueX int
}

const (
	barLabelX = 180
	barMaxW   = 380
	rowH      = 20
)

func (c BarChart) Height() int { return rowH * len(c.Bars) }

func (c BarChart) Rows() []BarRow {
	var max int64
	for _, b := range c.Bars {
		if b.Value > max {
			max = b.Value
		}
	}
	rows := make([]BarRow, 0, len(c.Bars))
	for i, b := range c.Bars {
		w := 0
		if b.Value > 0 && max > 0 {
			w = int(float64(b.Value) / float64(max) * barMaxW)
		}
		rows = append(rows, BarRow{Label: b.Label, Value: b.Value, Y: i * rowH, W: w, ValueX: barLabelX + w + 6})
	}
	return rows
}

type Pager struct {
	Path   string
	Query  url.Values
	Offset int64
	Limit  int64
	Total  int64
}

func (p *Pager) HasPrev() bool { return p != nil && p.Offset > 0 }

func (p *Pager) HasNext() bool { return p != nil && p.Offset+p.Limit < p.Total }

func (p *Pager) Start() int64 {
	if p == nil || p.Total == 0 {
		return 0
	}
	return p.Offset + 1
}

func (p *Pager) End() int64 {
	if p == nil {
		return 0
	}
	e := p.Offset + p.Limit
	if p.Limit <= 0 || e > p.Total {
		e = p.Total
	}
	return e
}

func (p *Pager) PrevHref() template.URL {
	if p == nil {
		return ""
	}
	return template.URL(p.href(p.Offset - p.Limit))
}

func (p *Pager) NextHref() template.URL {
	if p == nil {
		return ""
	}
	return template.URL(p.href(p.Offset + p.Limit))
}

func (p *Pager) href(offset int64) string {
	q := url.Values{}
	for k, vs := range p.Query {
		for _, v := range vs {
			q.Add(k, v)
		}
	}
	q.Set("offset", strconv.FormatInt(offset, 10))
	q.Set("limit", strconv.FormatInt(p.Limit, 10))
	return p.Path + "?" + q.Encode()
}

type TreeBranch struct {
	Key        string
	Label      string
	Type       string
	Muid       string
	Expandable bool
	Children   []*TreeBranch
}

type SegmentToken struct {
	Text   string
	IsWord bool
}

type SegmentRow struct {
	Muid   string
	SegKey string
	Pali   string
	En     string
	Author string
	Tokens []SegmentToken
}

type WordEntryView struct {
	Lemma      string
	POS        string
	Meaning    string
	Root       string
	FamilyRoot string
	Segments   int64
}

type UsageRef struct {
	Muid   string
	SegKey string
}

type JobView struct {
	ID        int64
	Stage     string
	State     string
	Err       string
	Started   string
	Finished  string
	StatusURL string
	Polling   bool
}

type LogLineView struct {
	Seq  int64
	Text string
}

type JobLogView struct {
	Lines   []LogLineView
	NextSeq int64
	LogURL  string
	Polling bool
}

type DiscrepancyRow struct {
	ID       int64
	Check    string
	Severity string
	Muid     string
	Detail   string
}

type RunView struct {
	ID       int64
	Label    string
	Started  string
	Finished string
}

type CoverRowView struct {
	Division string
	Texts    int64
	Covered  int64
}

func (c CoverRowView) Percent() int {
	if c.Texts <= 0 {
		return 0
	}
	return int(100 * c.Covered / c.Texts)
}

type DashboardPage struct {
	Page
	Stats        StatsGrid
	Coverage     []CoverRowView
	CoverageBars *BarChart
	Run          *RunView
	Job          *JobView
}

type TextsPage struct {
	Page
	Divisions []*TreeBranch
}

type TextPage struct {
	Page
	Muid          string
	Heading       string
	Segments      []SegmentRow
	SegmentsPager *Pager
}

type SearchPage struct {
	Page
	Text    string
	Lang    string
	Results []SegmentRow
}

type WordsPage struct {
	Page
	Form    string
	Entries []WordEntryView
}

type WordPage struct {
	Page
	Lemma     string
	Entries   []WordEntryView
	Total     int64
	Refs      []UsageRef
	RefsPager *Pager
	GraphURL  string
}

type GraphDataset struct {
	Name        string
	Label       string
	APIPath     string
	Param       string
	Placeholder string
	Layout      string
	Options     []string
}

func (d GraphDataset) OptionsListID() string { return "graph-options-" + d.Name }

type GraphPage struct {
	Page
	Datasets []GraphDataset
	Dataset  string
	Param    string
}

func (p GraphPage) SelectedDataset() *GraphDataset {
	for i := range p.Datasets {
		if p.Datasets[i].Name == p.Dataset {
			return &p.Datasets[i]
		}
	}
	if len(p.Datasets) > 0 {
		return &p.Datasets[0]
	}
	return &GraphDataset{}
}

type Analytic struct {
	Name        string
	Description string
}

func (p AnalyticsPage) CSVLink() template.URL { return template.URL(p.CSVURL) }

type AnalyticsPage struct {
	Page
	Catalog        []Analytic
	Selected       string
	Division       string
	Limit          int
	Cols           []string
	Rows           [][]string
	Chart          *BarChart
	CSVURL         string
	ConsoleEnabled bool
	SQL            string
	ConsoleCols    []string
	ConsoleRows    [][]string
	ConsoleErr     string
}

type PipelinePage struct {
	Page
	Stages []string
	Job    *JobView
	Log    *JobLogView
}

type DiscrepanciesPage struct {
	Page
	RunID    int64
	Severity string
	Check    string
	Rows     []DiscrepancyRow
	Pager    *Pager
}

type NotFoundPage struct {
	Page
	Path string
}
