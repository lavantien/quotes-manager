package server

import (
	"bytes"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/lavantien/quotes-manager/internal/canon/graph"
	"github.com/lavantien/quotes-manager/internal/canon/jobs"
	"github.com/lavantien/quotes-manager/internal/canon/web"
	"github.com/lavantien/quotes-manager/internal/cli"
)

func (s *Server) registerPageRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /dashboard", s.pageDashboard)
	mux.HandleFunc("GET /texts", s.pageTexts)
	mux.HandleFunc("GET /texts/branch", s.pageTextsBranch)
	mux.HandleFunc("GET /texts/{muid}", s.pageText)
	mux.HandleFunc("GET /search", s.pageSearch)
	mux.HandleFunc("GET /words", s.pageWords)
	mux.HandleFunc("GET /words/{lemma}", s.pageWord)
	mux.HandleFunc("GET /graph", s.pageGraph)
	mux.HandleFunc("GET /analytics", s.pageAnalytics)
	mux.HandleFunc("POST /analytics", s.pageAnalyticsConsole)
	mux.HandleFunc("GET /pipeline", s.pagePipeline)
	mux.HandleFunc("POST /pipeline", s.pagePipelineStart)
	mux.HandleFunc("GET /discrepancies", s.pageDiscrepancies)
	mux.HandleFunc("GET /partials/job/{id}", s.pageJobStatus)
}

func (s *Server) renderPage(w http.ResponseWriter, status int, name string, data any) {
	var buf bytes.Buffer
	if err := web.Render(&buf, name, data); err != nil {
		s.log.Error("render page", "template", name, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

func (s *Server) renderPartial(w http.ResponseWriter, name string, data any) {
	var buf bytes.Buffer
	if err := web.RenderPartial(&buf, name, data); err != nil {
		s.log.Error("render partial", "template", name, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

func pageNotReady(w http.ResponseWriter) {
	http.Error(w, "no published database yet; run the load stage first",
		http.StatusServiceUnavailable)
}

func (s *Server) pageInternalError(w http.ResponseWriter, err error) {
	s.log.Error("page handler", "error", err)
	http.Error(w, "internal error", http.StatusInternalServerError)
}

func (s *Server) pageNotFound(w http.ResponseWriter, r *http.Request) {
	s.renderPage(w, http.StatusNotFound, web.PageNotFound,
		web.NotFoundPage{Page: web.Page{Title: "not found"}, Path: r.URL.Path})
}

func (s *Server) pageDashboard(w http.ResponseWriter, r *http.Request) {
	page := web.DashboardPage{Page: web.Page{Title: "dashboard"}}
	ctx := r.Context()

	if stats, err := s.coreStats(ctx); err == nil {
		page.Stats.Rows = statsRows(stats.(map[string]int64))
	} else if !errors.Is(err, ErrNotReady) {
		s.pageInternalError(w, err)
		return
	}
	if cov, err := s.coverageByDivision(ctx); err == nil {
		page.Coverage = cov
		page.CoverageBars = coverageBars(cov)
	} else if !errors.Is(err, ErrNotReady) {
		s.pageInternalError(w, err)
		return
	}
	if runs, err := s.coreRuns(ctx); err == nil {
		page.Run = runView(runs.([]runJSON))
	} else if !errors.Is(err, ErrNotReady) {
		s.pageInternalError(w, err)
		return
	}
	page.Job = s.jobView(s.latestJob())
	s.renderPage(w, http.StatusOK, web.PageDashboard, page)
}

func (s *Server) pageTexts(w http.ResponseWriter, r *http.Request) {
	page := web.TextsPage{Page: web.Page{Title: "texts"}}
	divisions, err := s.divisionBranches(r.Context())
	if err != nil && !errors.Is(err, ErrNotReady) {
		s.pageInternalError(w, err)
		return
	}
	page.Divisions = divisions
	s.renderPage(w, http.StatusOK, web.PageTexts, page)
}

func (s *Server) pageTextsBranch(w http.ResponseWriter, r *http.Request) {
	node := r.URL.Query().Get("node")
	if node == "" {
		http.Error(w, "node is required", http.StatusBadRequest)
		return
	}
	limit, err := s.queryLimit(r)
	if err != nil {
		http.Error(w, "limit must be an integer", http.StatusBadRequest)
		return
	}
	v, err := s.coreTree(r.Context(), node, "text", limit, 0)
	if err != nil {
		if errors.Is(err, ErrNotReady) {
			pageNotReady(w)
			return
		}
		s.pageInternalError(w, err)
		return
	}
	branch := &web.TreeBranch{
		Key: node, Type: "division", Label: node, Children: textBranches(v.([]treeNodeJSON)),
	}
	s.renderPartial(w, web.PartialTreeBranch, branch)
}

func (s *Server) pageText(w http.ResponseWriter, r *http.Request) {
	muid := r.PathValue("muid")
	limit, err := s.queryLimit(r)
	if err != nil {
		http.Error(w, "limit must be an integer", http.StatusBadRequest)
		return
	}
	offset, err := queryOffset(r)
	if err != nil {
		http.Error(w, "offset must be an integer >= 0", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	store, err := s.db.Store(ctx)
	if err != nil {
		if errors.Is(err, ErrNotReady) {
			pageNotReady(w)
			return
		}
		s.pageInternalError(w, err)
		return
	}
	v, err := s.coreSegments(ctx, muid, false, limit, offset)
	if err != nil {
		s.pageInternalError(w, err)
		return
	}
	rows := v.([]segmentJSON)
	total, err := countPaliSegments(ctx, store, muid)
	if err != nil {
		s.pageInternalError(w, err)
		return
	}
	page := web.TextPage{
		Page:     web.Page{Title: muid},
		Muid:     muid,
		Heading:  s.textHeading(ctx, muid),
		Segments: segmentRows(rows, s.cfg.Canon.Dpd.MinTokenLength),
		SegmentsPager: &web.Pager{
			Path: "/texts/" + pathEscape(muid), Offset: int64(offset), Limit: int64(limit), Total: total,
		},
	}
	s.renderPage(w, http.StatusOK, web.PageText, page)
}

func (s *Server) pageSearch(w http.ResponseWriter, r *http.Request) {
	page := web.SearchPage{Page: web.Page{Title: "search"}}
	page.Text = r.URL.Query().Get("text")
	lang, err := queryEnum(r, "lang", "pali", "pali", "en")
	if err != nil {
		http.Error(w, "lang must be one of: pali, en", http.StatusBadRequest)
		return
	}
	page.Lang = lang
	limit, err := s.queryLimit(r)
	if err != nil {
		http.Error(w, "limit must be an integer", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(page.Text) != "" {
		v, err := s.coreSearch(r.Context(), page.Text, lang, limit, 0)
		if err != nil && !errors.Is(err, ErrNotReady) {
			s.pageInternalError(w, err)
			return
		}
		if err == nil {
			page.Results = searchRows(v.([]segmentJSON))
		}
	}
	if htmxRequest(r) {
		s.renderPartial(w, web.PartialSearchResults, page.Results)
		return
	}
	s.renderPage(w, http.StatusOK, web.PageSearch, page)
}

func (s *Server) pageWords(w http.ResponseWriter, r *http.Request) {
	page := web.WordsPage{Page: web.Page{Title: "words"}, Form: r.URL.Query().Get("form")}
	if page.Form != "" {
		v, err := s.coreWordForm(r.Context(), page.Form)
		if err != nil {
			if errors.Is(err, ErrNotReady) {
				pageNotReady(w)
				return
			}
			s.pageInternalError(w, err)
			return
		}
		page.Entries = wordEntryViews(v.([]graph.WordEntry))
	}
	s.renderPage(w, http.StatusOK, web.PageWords, page)
}

func (s *Server) pageWord(w http.ResponseWriter, r *http.Request) {
	lemma := r.PathValue("lemma")
	limit, err := s.queryLimit(r)
	if err != nil {
		http.Error(w, "limit must be an integer", http.StatusBadRequest)
		return
	}
	offset, err := queryOffset(r)
	if err != nil {
		http.Error(w, "offset must be an integer >= 0", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	if _, err := s.db.Store(ctx); err != nil {
		if errors.Is(err, ErrNotReady) {
			pageNotReady(w)
			return
		}
		s.pageInternalError(w, err)
		return
	}
	v, err := s.coreWordHeadword(ctx, lemma, true, limit, offset)
	if err != nil {
		s.pageInternalError(w, err)
		return
	}
	usage := v.(headwordUsageJSON)
	entries, err := s.headwordEntry(ctx, lemma)
	if err != nil {
		s.pageInternalError(w, err)
		return
	}
	page := web.WordPage{
		Page:     web.Page{Title: lemma},
		Lemma:    lemma,
		Entries:  entries,
		Total:    usage.Segments,
		Refs:     usageRefs(usage.Refs),
		GraphURL: "/api/v1/graph/word?token=" + queryEscape(lemma),
		RefsPager: &web.Pager{
			Path: "/words/" + pathEscape(lemma), Offset: int64(offset), Limit: int64(limit),
			Total: usage.Segments,
		},
	}
	s.renderPage(w, http.StatusOK, web.PageWord, page)
}

func (s *Server) pageGraph(w http.ResponseWriter, r *http.Request) {
	datasets, err := s.graphDatasets(r.Context())
	if err != nil && !errors.Is(err, ErrNotReady) {
		s.pageInternalError(w, err)
		return
	}
	page := web.GraphPage{
		Page:     web.Page{Title: "graph"},
		Datasets: datasets,
		Dataset:  r.URL.Query().Get("dataset"),
		Param:    r.URL.Query().Get("param"),
	}
	s.renderPage(w, http.StatusOK, web.PageGraph, page)
}

func (s *Server) pageAnalytics(w http.ResponseWriter, r *http.Request) {
	page := s.analyticsPage(r)
	if page.Selected != "" {
		s.fillAnalyticsResult(r, page)
	}
	s.renderPage(w, http.StatusOK, web.PageAnalytics, page)
}

func (s *Server) pageAnalyticsConsole(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.Server.SQLConsole {
		http.Redirect(w, r, "/analytics", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "form body must be urlencoded", http.StatusBadRequest)
		return
	}
	page := s.analyticsPage(r)
	page.SQL = r.PostFormValue("sql")
	if page.SQL != "" {
		d, err := s.analyticsDB(r.Context())
		if err == nil {
			tbl, qerr := d.Query(r.Context(), page.SQL, page.Limit)
			if qerr != nil {
				page.ConsoleErr = qerr.Error()
			} else {
				page.ConsoleCols = tbl.Cols
				page.ConsoleRows = tableStrings(tbl.Rows)
			}
		} else {
			page.ConsoleErr = sidecarMessage(err)
		}
	}
	s.renderPage(w, http.StatusOK, web.PageAnalytics, page)
}

func (s *Server) pagePipeline(w http.ResponseWriter, _ *http.Request) {
	page := web.PipelinePage{Page: web.Page{Title: "pipeline"}, Stages: cli.Stages()}
	if j := s.latestJob(); j != nil {
		page.Job = s.jobView(j)
		page.Log = s.jobLogView(j)
	}
	s.renderPage(w, http.StatusOK, web.PagePipeline, page)
}

func (s *Server) pagePipelineStart(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "form body must be urlencoded", http.StatusBadRequest)
		return
	}
	j, err := s.startJob(r.PostFormValue("stage"), nil)
	if err != nil {
		if errors.Is(err, jobs.ErrBusy) {
			if htmxRequest(r) {
				if active := s.latestJob(); active != nil {
					s.renderPipelineLive(w, active)
					return
				}
			}
			http.Redirect(w, r, "/pipeline", http.StatusSeeOther)
			return
		}
		http.Error(w, "stage must be one of: "+strings.Join(cli.Stages(), ", "), http.StatusBadRequest)
		return
	}
	if htmxRequest(r) {
		s.renderPipelineLive(w, j)
		return
	}
	http.Redirect(w, r, "/pipeline", http.StatusSeeOther)
}

func (s *Server) renderPipelineLive(w http.ResponseWriter, j *jobs.Job) {
	type live struct {
		Job *web.JobView
		Log *web.JobLogView
	}
	s.renderPartial(w, web.PartialPipelineLive, live{Job: s.jobView(j), Log: s.jobLogView(j)})
}

func (s *Server) pageJobStatus(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "no such job", http.StatusNotFound)
		return
	}
	j, ok := s.jobs.Get(id)
	if !ok {
		http.Error(w, "no such job", http.StatusNotFound)
		return
	}
	s.renderPartial(w, web.PartialJobStatus, s.jobView(j))
}

func (s *Server) pageDiscrepancies(w http.ResponseWriter, r *http.Request) {
	page := web.DiscrepanciesPage{Page: web.Page{Title: "discrepancies"}}
	limit, err := s.queryLimit(r)
	if err != nil {
		http.Error(w, "limit must be an integer", http.StatusBadRequest)
		return
	}
	offset, err := queryOffset(r)
	if err != nil {
		http.Error(w, "offset must be an integer >= 0", http.StatusBadRequest)
		return
	}
	run, err := queryInt64(r, "run", 0)
	if err != nil {
		http.Error(w, "run must be an integer", http.StatusBadRequest)
		return
	}
	page.RunID = run
	page.Severity = r.URL.Query().Get("severity")
	page.Check = r.URL.Query().Get("check")

	v, err := s.coreDiscrepancies(r.Context(), run, page.Severity, page.Check, limit, offset)
	if err == nil {
		res := v.(discrepanciesJSON)
		for _, row := range res.Rows {
			page.Rows = append(page.Rows, web.DiscrepancyRow{
				ID: row.ID, Check: row.Check, Severity: row.Severity,
				Muid: derefStr(row.Muid), Detail: row.Detail,
			})
		}
		query := url.Values{}
		if run != 0 {
			query.Set("run", strconv.FormatInt(run, 10))
		}
		if page.Severity != "" {
			query.Set("severity", page.Severity)
		}
		if page.Check != "" {
			query.Set("check", page.Check)
		}
		page.Pager = &web.Pager{
			Path: "/discrepancies", Query: query,
			Offset: int64(offset), Limit: int64(limit), Total: res.Total,
		}
	} else if !errors.Is(err, ErrNotReady) && !isStatusError(err) {
		s.pageInternalError(w, err)
		return
	}
	s.renderPage(w, http.StatusOK, web.PageDiscrepancies, page)
}
