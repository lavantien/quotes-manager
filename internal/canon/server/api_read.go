package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/lavantien/quotes-manager/internal/canon/graph"
	"github.com/lavantien/quotes-manager/internal/canon/model"
	"github.com/lavantien/quotes-manager/internal/config"
)

func (s *Server) registerReadRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/version", s.apiVersion)
	mux.HandleFunc("GET /api/v1/stats", s.apiStats)
	mux.HandleFunc("GET /api/v1/segments", s.apiSegments)
	mux.HandleFunc("GET /api/v1/search", s.apiSearch)
	mux.HandleFunc("GET /api/v1/tree", s.apiTree)
	mux.HandleFunc("GET /api/v1/word/form", s.apiWordForm)
	mux.HandleFunc("GET /api/v1/word/headword", s.apiWordHeadword)
	mux.HandleFunc("GET /api/v1/runs", s.apiRuns)
	mux.HandleFunc("GET /api/v1/discrepancies", s.apiDiscrepancies)
}

func (s *Server) serveResult(w http.ResponseWriter, v any, err error) {
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

type segmentJSON struct {
	Muid   string `json:"muid"`
	SegKey string `json:"seg_key"`
	Pali   string `json:"pali"`
	En     string `json:"en"`
	Author string `json:"author"`
}

func segmentsToJSON(pairs []graph.Pair) []segmentJSON {
	out := make([]segmentJSON, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, segmentJSON(p))
	}
	return out
}

type treeNodeJSON struct {
	Key   string `json:"key"`
	Type  string `json:"type"`
	Ord   *int64 `json:"ord"`
	Depth int    `json:"depth"`
	Data  any    `json:"data"`
}

type versionJSON struct {
	Version string `json:"version"`
	Schema  int    `json:"schema"`
	Run     string `json:"run"`
}

type headwordUsageJSON struct {
	Headword string   `json:"headword"`
	Segments int64    `json:"segments"`
	Refs     []string `json:"refs"`
}

type runJSON struct {
	ID             int64   `json:"id"`
	BilaraCommit   *string `json:"bilara_commit"`
	EditionsCommit *string `json:"editions_commit"`
	DpdTag         *string `json:"dpd_tag"`
	Started        *string `json:"started"`
	Finished       *string `json:"finished"`
}

type discrepancyJSON struct {
	ID       int64   `json:"id"`
	Check    string  `json:"check"`
	Severity string  `json:"severity"`
	Muid     *string `json:"muid"`
	Detail   string  `json:"detail"`
}

type discrepanciesJSON struct {
	Rows  []discrepancyJSON `json:"rows"`
	Total int64             `json:"total"`
}

func (s *Server) coreVersion(ctx context.Context) (any, error) {
	v := versionJSON{Version: config.Version, Schema: config.SchemaVersion}
	run, err := s.db.Run(ctx)
	if errors.Is(err, ErrNotReady) {
		return v, nil
	}
	if err != nil {
		return nil, err
	}
	v.Run = run
	return v, nil
}

func (s *Server) coreStats(ctx context.Context) (any, error) {
	store, err := s.db.Store(ctx)
	if err != nil {
		return nil, err
	}
	return store.Stats(ctx)
}

func (s *Server) coreSegments(ctx context.Context, muid string, withEn bool, limit, offset int) (any, error) {
	store, err := s.db.Store(ctx)
	if err != nil {
		return nil, err
	}
	pairs, err := store.Segments(ctx, muid, withEn, limit, offset)
	if err != nil {
		return nil, err
	}
	return segmentsToJSON(pairs), nil
}

func (s *Server) coreSearch(ctx context.Context, text, lang string, limit, offset int) (any, error) {
	store, err := s.db.Store(ctx)
	if err != nil {
		return nil, err
	}
	pairs, err := store.SearchFTS(ctx, text, lang, limit, offset)
	if err != nil {
		return nil, err
	}
	return segmentsToJSON(pairs), nil
}

func (s *Server) coreTree(ctx context.Context, division, types string, limit, offset int) (any, error) {
	store, err := s.db.Store(ctx)
	if err != nil {
		return nil, err
	}
	nodes, err := store.Tree(ctx, division, types, limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]treeNodeJSON, 0, len(nodes))
	for _, n := range nodes {
		var data any
		if n.Data != "" {
			if err := json.Unmarshal([]byte(n.Data), &data); err != nil {
				return nil, fmt.Errorf("tree node %s data: %w", n.Key, err)
			}
		}
		out = append(out, treeNodeJSON{
			Key: n.Key, Type: string(n.Type), Ord: n.Ord, Depth: n.Depth, Data: data,
		})
	}
	return out, nil
}

func (s *Server) coreWordForm(ctx context.Context, token string) (any, error) {
	store, err := s.db.Store(ctx)
	if err != nil {
		return nil, err
	}
	return store.ResolveForm(ctx, token)
}

func (s *Server) coreWordHeadword(ctx context.Context, lemma string, withSegments bool, limit, offset int) (any, error) {
	store, err := s.db.Store(ctx)
	if err != nil {
		return nil, err
	}
	segments, refs, err := store.HeadwordUsage(ctx, lemma, withSegments, limit, offset)
	if err != nil {
		return nil, err
	}
	if refs == nil {
		refs = []string{}
	}
	return headwordUsageJSON{Headword: lemma, Segments: segments, Refs: refs}, nil
}

func (s *Server) coreRuns(ctx context.Context) (any, error) {
	store, err := s.db.Store(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := store.DB().QueryContext(ctx, `
		SELECT id, bilara_commit, editions_commit, dpd_tag, started, finished
		FROM run ORDER BY id DESC`)
	if err != nil {
		return nil, fmt.Errorf("runs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []runJSON{}
	for rows.Next() {
		var r runJSON
		var bilara, editions, dpdTag, started, finished sql.NullString
		if err := rows.Scan(&r.ID, &bilara, &editions, &dpdTag, &started, &finished); err != nil {
			return nil, err
		}
		r.BilaraCommit = nullStrPtr(bilara)
		r.EditionsCommit = nullStrPtr(editions)
		r.DpdTag = nullStrPtr(dpdTag)
		r.Started = nullStrPtr(started)
		r.Finished = nullStrPtr(finished)
		out = append(out, r)
	}
	return out, rows.Err()
}

func nullStrPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}

func (s *Server) coreDiscrepancies(ctx context.Context, run int64, severity, check string, limit, offset int) (any, error) {
	store, err := s.db.Store(ctx)
	if err != nil {
		return nil, err
	}
	runID := run
	if runID == 0 {
		var latest sql.NullInt64
		if err := store.DB().QueryRowContext(ctx, `SELECT MAX(id) FROM run`).Scan(&latest); err != nil {
			return nil, fmt.Errorf("latest run: %w", err)
		}
		if !latest.Valid {
			return nil, errNotFound(codeRunNotFound, "no run rows in the current database")
		}
		runID = latest.Int64
	}
	var exists int
	if err := store.DB().QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM run WHERE id = ?)`, runID,
	).Scan(&exists); err != nil {
		return nil, fmt.Errorf("run %d: %w", runID, err)
	}
	if exists == 0 {
		return nil, errNotFound(codeRunNotFound, fmt.Sprintf("run %d not found", runID))
	}

	filter := ` WHERE run_id = ?`
	params := []any{runID}
	if severity != "" {
		filter += ` AND severity = ?`
		params = append(params, severity)
	}
	if check != "" {
		filter += ` AND "check" = ?`
		params = append(params, check)
	}

	var total int64
	if err := store.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM discrepancy`+filter, params...,
	).Scan(&total); err != nil {
		return nil, fmt.Errorf("count discrepancies: %w", err)
	}

	rows, err := store.DB().QueryContext(ctx, `
		SELECT id, "check", severity, muid, detail
		FROM discrepancy`+filter+`
		ORDER BY id
		LIMIT ? OFFSET ?`,
		append(append([]any{}, params...), limit, offset)...)
	if err != nil {
		return nil, fmt.Errorf("discrepancies: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := discrepanciesJSON{Rows: []discrepancyJSON{}, Total: total}
	for rows.Next() {
		var d discrepancyJSON
		var muid sql.NullString
		if err := rows.Scan(&d.ID, &d.Check, &d.Severity, &muid, &d.Detail); err != nil {
			return nil, err
		}
		d.Muid = nullStrPtr(muid)
		out.Rows = append(out.Rows, d)
	}
	return out, rows.Err()
}

func (s *Server) apiVersion(w http.ResponseWriter, r *http.Request) {
	v, err := s.coreVersion(r.Context())
	s.serveResult(w, v, err)
}

func (s *Server) apiStats(w http.ResponseWriter, r *http.Request) {
	v, err := s.coreStats(r.Context())
	s.serveResult(w, v, err)
}

func (s *Server) apiSegments(w http.ResponseWriter, r *http.Request) {
	muid, err := requireParam(r, "muid")
	if err != nil {
		writeError(w, err)
		return
	}
	withEn, err := queryFlag(r, "with_en")
	if err != nil {
		writeError(w, err)
		return
	}
	limit, err := s.queryLimit(r)
	if err != nil {
		writeError(w, err)
		return
	}
	offset, err := queryOffset(r)
	if err != nil {
		writeError(w, err)
		return
	}
	v, err := s.coreSegments(r.Context(), muid, withEn, limit, offset)
	s.serveResult(w, v, err)
}

func (s *Server) apiSearch(w http.ResponseWriter, r *http.Request) {
	text, err := requireParam(r, "text")
	if err != nil {
		writeError(w, err)
		return
	}
	if strings.TrimSpace(text) == "" {
		writeError(w, errInvalidParam("text must not be blank"))
		return
	}
	lang, err := queryEnum(r, "lang", "pali", "pali", "en")
	if err != nil {
		writeError(w, err)
		return
	}
	limit, err := s.queryLimit(r)
	if err != nil {
		writeError(w, err)
		return
	}
	offset, err := queryOffset(r)
	if err != nil {
		writeError(w, err)
		return
	}
	v, err := s.coreSearch(r.Context(), text, lang, limit, offset)
	s.serveResult(w, v, err)
}

func (s *Server) apiTree(w http.ResponseWriter, r *http.Request) {
	division, err := requireParam(r, "division")
	if err != nil {
		writeError(w, err)
		return
	}
	types, err := queryEnum(r, "types", "all", "segment", "text", "all")
	if err != nil {
		writeError(w, err)
		return
	}
	limit, err := s.queryLimit(r)
	if err != nil {
		writeError(w, err)
		return
	}
	offset, err := queryOffset(r)
	if err != nil {
		writeError(w, err)
		return
	}
	v, err := s.coreTree(r.Context(), division, types, limit, offset)
	s.serveResult(w, v, err)
}

func (s *Server) apiWordForm(w http.ResponseWriter, r *http.Request) {
	token, err := requireParam(r, "token")
	if err != nil {
		writeError(w, err)
		return
	}
	v, err := s.coreWordForm(r.Context(), token)
	s.serveResult(w, v, err)
}

func (s *Server) apiWordHeadword(w http.ResponseWriter, r *http.Request) {
	lemma, err := requireParam(r, "lemma")
	if err != nil {
		writeError(w, err)
		return
	}
	withSegments, err := queryFlag(r, "segments")
	if err != nil {
		writeError(w, err)
		return
	}
	limit, err := s.queryLimit(r)
	if err != nil {
		writeError(w, err)
		return
	}
	offset, err := queryOffset(r)
	if err != nil {
		writeError(w, err)
		return
	}
	v, err := s.coreWordHeadword(r.Context(), lemma, withSegments, limit, offset)
	s.serveResult(w, v, err)
}

func (s *Server) apiRuns(w http.ResponseWriter, r *http.Request) {
	v, err := s.coreRuns(r.Context())
	s.serveResult(w, v, err)
}

func (s *Server) apiDiscrepancies(w http.ResponseWriter, r *http.Request) {
	run, err := queryInt64(r, "run", 0)
	if err != nil {
		writeError(w, err)
		return
	}
	severity, err := queryEnum(r, "severity", "",
		string(model.Info), string(model.Warn), string(model.Error))
	if err != nil {
		writeError(w, err)
		return
	}
	check := r.URL.Query().Get("check")
	limit, err := s.queryLimit(r)
	if err != nil {
		writeError(w, err)
		return
	}
	offset, err := queryOffset(r)
	if err != nil {
		writeError(w, err)
		return
	}
	v, err := s.coreDiscrepancies(r.Context(), run, severity, check, limit, offset)
	s.serveResult(w, v, err)
}
