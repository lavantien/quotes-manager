package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lavantien/quotes-manager/internal/canon/analytics"
	"github.com/lavantien/quotes-manager/internal/config"
)

const (
	codeUnknownAnalytic = "unknown_analytic"
	codeSidecarStale    = "sidecar_stale"
	codeConsoleDisabled = "console_disabled"
	codeSQLRejected     = "sql_rejected"
	codeUnknownTable    = "unknown_table"
	codeParquetAbsent   = "parquet_absent"
)

const consoleBodyMaxBytes = 64 << 10

var sidecarTables = []string{
	"node", "edge", "run", "discrepancy",
	"segment_pairs", "dpd_entries", "word_occurrences", "text_stats",
}

func (s *Server) registerAnalyticsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/analytics/catalog", s.apiAnalyticsCatalog)
	mux.HandleFunc("GET /api/v1/analytics/{name}", s.apiAnalyticsAggregate)
	mux.HandleFunc("GET /api/v1/analytics/{name}/csv", s.apiAnalyticsCSV)
	mux.HandleFunc("POST /api/v1/analytics/query", s.apiAnalyticsQuery)
	mux.HandleFunc("GET /api/v1/export", s.apiExportList)
	mux.HandleFunc("GET /api/v1/export/parquet/{table}", s.apiExportParquet)
}

type tableJSON struct {
	Cols []string `json:"cols"`
	Rows [][]any  `json:"rows"`
}

func toTableJSON(t analytics.Table) tableJSON {
	out := tableJSON{Cols: t.Cols, Rows: t.Rows}
	if out.Rows == nil {
		out.Rows = [][]any{}
	}
	return out
}

func (s *Server) analyticsDB(ctx context.Context) (*analytics.DB, error) {
	pool, err := s.duck.State(ctx)
	if err != nil {
		return nil, err
	}
	return analytics.Wrap(pool)
}

func writeAnalyticsError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, analytics.ErrSidecarStale):
		writeErr(w, http.StatusServiceUnavailable, codeSidecarStale,
			"sidecar predates the analytics tables; rerun the export stage")
	case errors.Is(err, analytics.ErrUnknownAggregate):
		writeErr(w, http.StatusNotFound, codeUnknownAnalytic, err.Error())
	default:
		writeError(w, err)
	}
}

func aggregateTakesDivision(name string) bool {
	for _, e := range analytics.Catalog() {
		if e.Name != name {
			continue
		}
		for _, p := range e.Params {
			if p.Name == "division" {
				return true
			}
		}
	}
	return false
}

func (s *Server) apiAnalyticsCatalog(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, analytics.Catalog())
}

func (s *Server) aggregateParams(r *http.Request) (name, division string, limit int, err error) {
	name = r.PathValue("name")
	division = r.URL.Query().Get("division")
	if division != "" && !aggregateTakesDivision(name) {
		return "", "", 0, errInvalidParam("aggregate " + name + " takes no division filter")
	}
	if limit, err = s.queryLimit(r); err != nil {
		return "", "", 0, err
	}
	return name, division, limit, nil
}

func (s *Server) apiAnalyticsAggregate(w http.ResponseWriter, r *http.Request) {
	name, division, limit, err := s.aggregateParams(r)
	if err != nil {
		writeError(w, err)
		return
	}
	d, err := s.analyticsDB(r.Context())
	if err != nil {
		writeAnalyticsError(w, err)
		return
	}
	tbl, err := d.Aggregate(r.Context(), name, division, limit)
	if err != nil {
		writeAnalyticsError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toTableJSON(tbl))
}

func (s *Server) apiAnalyticsCSV(w http.ResponseWriter, r *http.Request) {
	name, division, limit, err := s.aggregateParams(r)
	if err != nil {
		writeError(w, err)
		return
	}
	d, err := s.analyticsDB(r.Context())
	if err != nil {
		writeAnalyticsError(w, err)
		return
	}
	tbl, err := d.Aggregate(r.Context(), name, division, limit)
	if err != nil {
		writeAnalyticsError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name+".csv"))
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, analytics.ToCSV(tbl))
}

func (s *Server) apiAnalyticsQuery(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.Server.SQLConsole {
		writeErr(w, http.StatusForbidden, codeConsoleDisabled,
			"sql console disabled by configuration")
		return
	}
	var body struct {
		SQL   string `json:"sql"`
		Limit int    `json:"limit"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, consoleBodyMaxBytes)).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, codeInvalidParam,
			`body must be json: {"sql": "select ...", "limit": 50}`)
		return
	}
	d, err := s.analyticsDB(r.Context())
	if err != nil {
		writeAnalyticsError(w, err)
		return
	}
	tbl, err := d.Query(r.Context(), body.SQL, body.Limit)
	if err != nil {
		if errors.Is(err, analytics.ErrSidecarStale) {
			writeAnalyticsError(w, err)
			return
		}
		writeErr(w, http.StatusBadRequest, codeSQLRejected, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toTableJSON(tbl))
}

type exportTableJSON struct {
	Name    string `json:"name"`
	Rows    int64  `json:"rows"`
	Parquet bool   `json:"parquet"`
}

type exportListJSON struct {
	Tables []exportTableJSON `json:"tables"`
}

func parquetPath(cfg *config.Config, table string) string {
	return filepath.Join(cfg.Canon.ParquetDir(), table+".parquet")
}

func (s *Server) apiExportList(w http.ResponseWriter, r *http.Request) {
	db, err := s.duck.State(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]exportTableJSON, 0, len(sidecarTables))
	for _, t := range sidecarTables {
		var n int64
		if err := db.QueryRowContext(r.Context(),
			"SELECT COUNT(*) FROM "+t).Scan(&n); err != nil {
			writeError(w, fmt.Errorf("count %s: %w", t, err))
			return
		}
		_, statErr := os.Stat(parquetPath(s.cfg, t))
		out = append(out, exportTableJSON{Name: t, Rows: n, Parquet: statErr == nil})
	}
	writeJSON(w, http.StatusOK, exportListJSON{Tables: out})
}

func (s *Server) apiExportParquet(w http.ResponseWriter, r *http.Request) {
	table := r.PathValue("table")
	if !slices.Contains(sidecarTables, table) {
		writeError(w, errNotFound(codeUnknownTable,
			"table must be one of: "+strings.Join(sidecarTables, ", ")))
		return
	}
	path := parquetPath(s.cfg, table)
	if _, err := os.Stat(path); err != nil {
		writeError(w, errNotFound(codeParquetAbsent,
			"no parquet file for "+table+"; rerun export with parquet enabled"))
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", table+".parquet"))
	http.ServeFile(w, r, path)
}
