package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/lavantien/quotes-manager/internal/canon/analytics"
	"github.com/lavantien/quotes-manager/internal/canon/graph"
	"github.com/lavantien/quotes-manager/internal/canon/jobs"
	"github.com/lavantien/quotes-manager/internal/canon/norm"
	"github.com/lavantien/quotes-manager/internal/canon/web"
)

func htmxRequest(r *http.Request) bool { return r.Header.Get("HX-Request") == "true" }

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func isStatusError(err error) bool {
	var se *statusError
	return errors.As(err, &se)
}

func queryEscape(s string) string { return url.QueryEscape(s) }
func pathEscape(s string) string  { return url.PathEscape(s) }

func statsRows(stats map[string]int64) []web.StatRow {
	rows := make([]web.StatRow, 0, len(stats))
	for _, k := range slices.Sorted(maps.Keys(stats)) {
		rows = append(rows, web.StatRow{Label: k, Value: stats[k]})
	}
	return rows
}

func (s *Server) coverageByDivision(ctx context.Context) ([]web.CoverRowView, error) {
	store, err := s.db.Store(ctx)
	if err != nil {
		return nil, err
	}
	sums, err := store.CoverageSummary(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]web.CoverRowView, 0, len(sums))
	for _, d := range sums {
		out = append(out, web.CoverRowView{Division: d.Division, Texts: d.Texts, Covered: d.Covered})
	}
	return out, nil
}

func coverageBars(cov []web.CoverRowView) *web.BarChart {
	if len(cov) == 0 {
		return nil
	}
	chart := &web.BarChart{}
	for _, c := range cov {
		chart.Bars = append(chart.Bars, web.Bar{Label: c.Division, Value: c.Covered})
	}
	return chart
}

func runView(runs []runJSON) *web.RunView {
	if len(runs) == 0 {
		return nil
	}
	r := runs[0]
	label := derefStr(r.DpdTag)
	if label == "" {
		label = derefStr(r.BilaraCommit)
	}
	return &web.RunView{
		ID: r.ID, Label: label, Started: derefStr(r.Started), Finished: derefStr(r.Finished),
	}
}

func nodeKeys(ctx context.Context, store *graph.Store, typ string) ([]string, error) {
	rows, err := store.DB().QueryContext(ctx,
		`SELECT key FROM node WHERE type = ? ORDER BY key`, typ)
	if err != nil {
		return nil, fmt.Errorf("list %s nodes: %w", typ, err)
	}
	defer func() { _ = rows.Close() }()
	out := []string{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		out = append(out, key)
	}
	return out, rows.Err()
}

func (s *Server) divisionBranches(ctx context.Context) ([]*web.TreeBranch, error) {
	store, err := s.db.Store(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := store.DB().QueryContext(ctx, `
		SELECT key, COALESCE(json_extract(data, '$.name'), '')
		FROM node WHERE type = 'division' ORDER BY key`)
	if err != nil {
		return nil, fmt.Errorf("list divisions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []*web.TreeBranch{}
	for rows.Next() {
		var key, name string
		if err := rows.Scan(&key, &name); err != nil {
			return nil, err
		}
		if name == "" {
			name = key
		}
		out = append(out, &web.TreeBranch{Key: key, Label: name, Type: "division", Expandable: true})
	}
	return out, rows.Err()
}

func dataName(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	name, _ := m["name"].(string)
	return name
}

func textBranches(nodes []treeNodeJSON) []*web.TreeBranch {
	out := make([]*web.TreeBranch, 0, len(nodes))
	for _, n := range nodes {
		label := dataName(n.Data)
		if label == "" {
			label = n.Key
		}
		out = append(out, &web.TreeBranch{
			Key: n.Key, Label: label, Type: string(n.Type), Muid: n.Key,
		})
	}
	return out
}

func (s *Server) textHeading(ctx context.Context, muid string) string {
	store, err := s.db.Store(ctx)
	if err != nil {
		return muid
	}
	var name string
	err = store.DB().QueryRowContext(ctx,
		`SELECT COALESCE(json_extract(data, '$.name'), '') FROM node
		 WHERE type = 'text' AND key = ?`, muid).Scan(&name)
	if err != nil || name == "" {
		return muid
	}
	return name
}

func countPaliSegments(ctx context.Context, store *graph.Store, muid string) (int64, error) {
	var n int64
	err := store.DB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM node p
		JOIN edge e ON e.src = p.id AND e.type = 'CHILD_OF'
		JOIN node t ON t.id = e.dst AND t.type = 'text'
		WHERE p.type = 'segment' AND p.key LIKE 'pli:' || ? || ':%'`, muid,
	).Scan(&n)
	return n, err
}

func tokenizeSegment(pali string, minLen int) []web.SegmentToken {
	var toks []web.SegmentToken
	runStart, gapStart := -1, 0
	flushRun := func(end int) {
		run := pali[runStart:end]
		if len(norm.TokenizePali(run, minLen)) == 1 {
			toks = append(toks, web.SegmentToken{Text: run, IsWord: true})
		} else {
			toks = append(toks, web.SegmentToken{Text: run})
		}
	}
	flushGap := func(end int) {
		if end > gapStart {
			toks = append(toks, web.SegmentToken{Text: pali[gapStart:end]})
		}
	}
	for i, r := range pali {
		if unicode.IsLetter(r) || unicode.In(r, unicode.Mn, unicode.Mc, unicode.Me) {
			if runStart < 0 {
				flushGap(i)
				runStart = i
			}
			continue
		}
		if runStart >= 0 {
			flushRun(i)
			runStart = -1
			gapStart = i
		}
	}
	if runStart >= 0 {
		flushRun(len(pali))
	} else {
		flushGap(len(pali))
	}
	return toks
}

func segmentRows(pairs []segmentJSON, minLen int) []web.SegmentRow {
	out := make([]web.SegmentRow, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, web.SegmentRow{
			Muid: p.Muid, SegKey: p.SegKey, Pali: p.Pali, En: p.En, Author: p.Author,
			Tokens: tokenizeSegment(p.Pali, minLen),
		})
	}
	return out
}

func searchRows(pairs []segmentJSON) []web.SegmentRow {
	out := make([]web.SegmentRow, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, web.SegmentRow{
			Muid: p.Muid, SegKey: p.SegKey, Pali: p.Pali, En: p.En, Author: p.Author,
		})
	}
	return out
}

func wordEntryViews(entries []graph.WordEntry) []web.WordEntryView {
	out := make([]web.WordEntryView, 0, len(entries))
	for _, e := range entries {
		out = append(out, web.WordEntryView{
			Lemma: e.Lemma1, POS: e.POS, Meaning: e.Meaning1,
			Root: e.Root, FamilyRoot: e.FamilyRoot, Segments: e.Segments,
		})
	}
	return out
}

func usageRefs(refs []string) []web.UsageRef {
	out := []web.UsageRef{}
	for _, ref := range refs {
		muid, segKey, ok := strings.Cut(ref, ":")
		if !ok {
			continue
		}
		out = append(out, web.UsageRef{Muid: muid, SegKey: segKey})
	}
	return out
}

func (s *Server) headwordEntry(ctx context.Context, lemma string) ([]web.WordEntryView, error) {
	store, err := s.db.Store(ctx)
	if err != nil {
		return nil, err
	}
	var v web.WordEntryView
	err = store.DB().QueryRowContext(ctx, `
		SELECT COALESCE(json_extract(n.data, '$.pos'), ''),
			COALESCE(json_extract(n.data, '$.meaning_1'), ''),
			COALESCE(json_extract(n.data, '$.root_key'), ''),
			COALESCE(json_extract(n.data, '$.family_root'), ''),
			(SELECT COUNT(*) FROM edge e WHERE e.dst = n.id AND e.type = 'HAS_WORD')
		FROM node n
		WHERE n.type = 'headword' AND n.key = ?`, "dpd:"+lemma,
	).Scan(&v.POS, &v.Meaning, &v.Root, &v.FamilyRoot, &v.Segments)
	if errors.Is(err, sql.ErrNoRows) {
		return []web.WordEntryView{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("headword %s: %w", lemma, err)
	}
	v.Lemma = lemma
	return []web.WordEntryView{v}, nil
}

func (s *Server) graphDatasets(ctx context.Context) ([]web.GraphDataset, error) {
	var divisions, roots []string
	if store, err := s.db.Store(ctx); err == nil {
		if divisions, err = nodeKeys(ctx, store, "division"); err != nil {
			return nil, err
		}
		if roots, err = nodeKeys(ctx, store, "dpd_root"); err != nil {
			return nil, err
		}
		for i, r := range roots {
			roots[i] = strings.TrimPrefix(r, "dpd-root:")
		}
	} else if !errors.Is(err, ErrNotReady) {
		return nil, err
	}
	return []web.GraphDataset{
		{
			Name: "word", Label: "word neighborhood", APIPath: "/api/v1/graph/word",
			Param: "token", Placeholder: "dhamma", Layout: "concentric",
		},
		{
			Name: "root", Label: "root family", APIPath: "/api/v1/graph/root",
			Param: "root", Placeholder: "root key", Layout: "concentric", Options: roots,
		},
		{
			Name: "tree", Label: "division tree", APIPath: "/api/v1/graph/tree",
			Param: "division", Placeholder: "division key", Layout: "breadthfirst",
			Options: divisions,
		},
		{
			Name: "coverage", Label: "edition coverage", APIPath: "/api/v1/graph/coverage",
			Param: "division", Placeholder: "division key", Layout: "breadthfirst",
			Options: divisions,
		},
	}, nil
}

func (s *Server) latestJob() *jobs.Job {
	all := s.jobs.List()
	if len(all) == 0 {
		return nil
	}
	return all[len(all)-1]
}

func (s *Server) jobView(j *jobs.Job) *web.JobView {
	if j == nil {
		return nil
	}
	snap := j.Snapshot()
	return &web.JobView{
		ID: snap.ID, Stage: snap.Stage, State: string(snap.State), Err: snap.Error,
		Started: snap.Started, Finished: snap.Finished,
		StatusURL: "/partials/job/" + strconv.FormatInt(snap.ID, 10),
		Polling:   jobActive(snap.State),
	}
}

func (s *Server) jobLogView(j *jobs.Job) *web.JobLogView {
	if j == nil {
		return nil
	}
	snap := j.Snapshot()
	view := &web.JobLogView{
		LogURL:  "/api/v1/jobs/" + strconv.FormatInt(j.ID(), 10) + "/logs",
		Polling: jobActive(snap.State),
	}
	for _, line := range s.jobs.LogsAfter(j.ID(), 0) {
		view.Lines = append(view.Lines, web.LogLineView{Seq: line.Seq, Text: line.Text})
	}
	if n := len(view.Lines); n > 0 {
		view.NextSeq = view.Lines[n-1].Seq + 1
	}
	return view
}

func (s *Server) analyticsPage(r *http.Request) *web.AnalyticsPage {
	q := r.URL.Query()
	limit, err := s.queryLimit(r)
	if err != nil {
		limit = s.cfg.Server.DefaultLimit
	}
	page := &web.AnalyticsPage{
		Page:           web.Page{Title: "analytics"},
		Division:       q.Get("division"),
		Limit:          limit,
		ConsoleEnabled: s.cfg.Server.SQLConsole,
	}
	if page.Selected = q.Get("aggregate"); page.Selected == "" && r.Method == http.MethodPost {
		page.Selected = r.PostFormValue("aggregate")
	}
	for _, e := range analytics.Catalog() {
		page.Catalog = append(page.Catalog, web.Analytic{Name: e.Name, Description: e.Desc})
	}
	return page
}

func (s *Server) fillAnalyticsResult(r *http.Request, page *web.AnalyticsPage) {
	d, err := s.analyticsDB(r.Context())
	if err != nil {
		s.log.Error("analytics page", "error", err)
		return
	}
	tbl, err := d.Aggregate(r.Context(), page.Selected, page.Division, page.Limit)
	if err != nil {
		s.log.Error("analytics page", "aggregate", page.Selected, "error", err)
		return
	}
	page.Cols = tbl.Cols
	page.Rows = tableStrings(tbl.Rows)
	page.Chart = barChart(page.Rows)
	q := url.Values{}
	if page.Division != "" {
		q.Set("division", page.Division)
	}
	q.Set("limit", strconv.Itoa(page.Limit))
	page.CSVURL = "/api/v1/analytics/" + pathEscape(page.Selected) + "/csv?" + q.Encode()
}

func tableStrings(rows [][]any) [][]string {
	out := make([][]string, 0, len(rows))
	for _, row := range rows {
		fields := make([]string, len(row))
		for i, v := range row {
			if v != nil {
				fields[i] = fmt.Sprint(v)
			}
		}
		out = append(out, fields)
	}
	return out
}

const chartBarsMax = 10

func barChart(rows [][]string) *web.BarChart {
	chart := &web.BarChart{}
	for _, row := range rows {
		if len(row) < 2 {
			continue
		}
		v, err := strconv.ParseInt(strings.TrimSpace(row[1]), 10, 64)
		if err != nil {
			continue
		}
		chart.Bars = append(chart.Bars, web.Bar{Label: row[0], Value: v})
		if len(chart.Bars) == chartBarsMax {
			break
		}
	}
	if len(chart.Bars) == 0 {
		return nil
	}
	return chart
}

func sidecarMessage(err error) string {
	if errors.Is(err, analytics.ErrSidecarStale) {
		return "sidecar predates the analytics tables; rerun the export stage"
	}
	if errors.Is(err, ErrNotReady) {
		return "no duckdb sidecar yet; run the export stage first"
	}
	return err.Error()
}

func jobActive(state jobs.State) bool {
	return state == jobs.StateQueued || state == jobs.StateRunning
}
