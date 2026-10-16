package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/lavantien/quotes-manager/internal/canon/export"
	"github.com/lavantien/quotes-manager/internal/config"
)

func writeSidecarTable(t testing.TB, cfg *config.Config, table string) {
	t.Helper()
	db, err := sqlOpenDuck(cfg.Canon.DuckDBFile())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE ` + table + ` (x INTEGER)`); err != nil {
		_ = db.Close()
		t.Fatalf("create sidecar: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
}

func fixtureSidecarServer(t testing.TB) (*Server, *config.Config) {
	t.Helper()
	cfg := newFixtureConfig(t)
	fixtureDB(t, cfg, "run1", "Mūlapariyāya")
	if _, err := export.Rebuild(context.Background(), cfg); err != nil {
		t.Fatalf("rebuild sidecar: %v", err)
	}
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	return srv, cfg
}

func TestAPIAnalyticsCatalog(t *testing.T) {
	srv, _ := fixtureSidecarServer(t)
	h := srv.Routes()

	var entries []struct {
		Name string `json:"name"`
	}
	getJSON(t, h, "/api/v1/analytics/catalog", &entries)
	if len(entries) < 6 || entries[0].Name != "top-headwords" {
		t.Fatalf("catalog = %+v", entries)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/analytics/catalog", nil))
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("catalog = %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
}

func TestAPIAnalyticsAggregate(t *testing.T) {
	srv, _ := fixtureSidecarServer(t)
	h := srv.Routes()

	var tbl tableJSON
	getJSON(t, h, "/api/v1/analytics/top-headwords", &tbl)
	if len(tbl.Cols) != 3 || tbl.Cols[0] != "headword" {
		t.Fatalf("cols = %+v", tbl.Cols)
	}
	if len(tbl.Rows) != 3 {
		t.Fatalf("rows = %+v", tbl.Rows)
	}
	if tbl.Rows[0][0] != "dhamma" {
		t.Fatalf("top row = %+v", tbl.Rows[0])
	}

	var filtered tableJSON
	getJSON(t, h, "/api/v1/analytics/top-headwords?division=mn-div", &filtered)
	if len(filtered.Rows) != 3 {
		t.Fatalf("division-filtered rows = %+v", filtered.Rows)
	}

	var limited tableJSON
	getJSON(t, h, "/api/v1/analytics/top-headwords?limit=1", &limited)
	if len(limited.Rows) != 1 {
		t.Fatalf("limited rows = %+v", limited.Rows)
	}

	var empty tableJSON
	getJSON(t, h, "/api/v1/analytics/top-headwords?division=none", &empty)
	if empty.Rows == nil || len(empty.Rows) != 0 {
		t.Fatalf("empty rows = %+v", empty.Rows)
	}

	status, env := getError(t, h, "/api/v1/analytics/nope")
	if status != http.StatusNotFound || env.Code != codeUnknownAnalytic {
		t.Fatalf("unknown aggregate = %d/%s", status, env.Code)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/api/v1/analytics/vocab-overlap?division=mn-div", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("division on division-less aggregate = %d: %s", rec.Code, rec.Body.String())
	}
	var env2 errorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env2); err != nil || env2.Error.Code != codeInvalidParam {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestAPIAnalyticsCSV(t *testing.T) {
	srv, _ := fixtureSidecarServer(t)
	h := srv.Routes()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/analytics/top-headwords/csv", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("csv = %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Fatalf("csv content type = %q", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, `filename="top-headwords.csv"`) {
		t.Fatalf("csv disposition = %q", cd)
	}
	if !strings.HasPrefix(rec.Body.String(), "headword,occurrences,texts\n") {
		t.Fatalf("csv body = %q", rec.Body.String())
	}

	status, env := getError(t, h, "/api/v1/analytics/nope/csv")
	if status != http.StatusNotFound || env.Code != codeUnknownAnalytic {
		t.Fatalf("csv unknown aggregate = %d/%s", status, env.Code)
	}
}

func TestAPIAnalyticsSidecarStale(t *testing.T) {
	cfg := newFixtureConfig(t)
	fixtureDB(t, cfg, "run1", "Mūlapariyāya")
	writeSidecarTable(t, cfg, "word_occurrences")
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	h := srv.Routes()

	status, env := getError(t, h, "/api/v1/analytics/top-headwords")
	if status != http.StatusServiceUnavailable || env.Code != codeSidecarStale {
		t.Fatalf("stale sidecar = %d/%s", status, env.Code)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/analytics/query",
		strings.NewReader(`{"sql":"SELECT 1 AS x"}`))
	srv.cfg.Server.SQLConsole = true
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("query on stale sidecar = %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAPIAnalyticsQueryConsole(t *testing.T) {
	srv, _ := fixtureSidecarServer(t)
	srv.cfg.Server.SQLConsole = false
	h := srv.Routes()

	rec := postBody(t, h, "/api/v1/analytics/query", `{"sql":"SELECT 1 AS x"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("disabled console = %d: %s", rec.Code, rec.Body.String())
	}
	var env errorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil || env.Error.Code != codeConsoleDisabled {
		t.Fatalf("disabled body = %s", rec.Body.String())
	}

	srv.cfg.Server.SQLConsole = true
	var tbl tableJSON
	req := httptest.NewRequest(http.MethodPost, "/api/v1/analytics/query",
		strings.NewReader(`{"sql":"SELECT 42 AS answer","limit":5}`))
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusOK {
		t.Fatalf("query = %d: %s", rec2.Code, rec2.Body.String())
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &tbl); err != nil {
		t.Fatal(err)
	}
	if len(tbl.Cols) != 1 || tbl.Cols[0] != "answer" || fmt.Sprint(tbl.Rows[0][0]) != "42" {
		t.Fatalf("query table = %+v", tbl)
	}

	for _, sql := range []string{
		`{"sql":"DELETE FROM node"}`,
		`{"sql":"UPDATE node SET id = 1"}`,
		`{"sql":"SELECT 1; DROP TABLE node"}`,
		`{"sql":""}`,
		`{"sql":"WITH x AS (SELECT 1) SELECT * FROM x LIMIT 2; SELECT 2"}`,
	} {
		rec := postBody(t, h, "/api/v1/analytics/query", sql)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("gate %s = %d: %s", sql, rec.Code, rec.Body.String())
		}
		var env errorEnvelope
		if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil || env.Error.Code != codeSQLRejected {
			t.Fatalf("gate body for %s = %s", sql, rec.Body.String())
		}
	}

	rec = postBody(t, h, "/api/v1/analytics/query", `{"sql":`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed console body = %d: %s", rec.Code, rec.Body.String())
	}

	rec = postBody(t, h, "/api/v1/analytics/query", `{"sql":"SELECT * FROM no_such_table"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("engine error = %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAPIExportListAndParquet(t *testing.T) {
	srv, _ := fixtureSidecarServer(t)
	h := srv.Routes()

	var list exportListJSON
	getJSON(t, h, "/api/v1/export", &list)
	if len(list.Tables) != 8 {
		t.Fatalf("table count = %d", len(list.Tables))
	}
	if list.Tables[0].Name != "node" || list.Tables[0].Rows == 0 {
		t.Fatalf("node row = %+v", list.Tables[0])
	}
	if list.Tables[6].Name != "word_occurrences" || list.Tables[6].Rows == 0 {
		t.Fatalf("word_occurrences row = %+v", list.Tables[6])
	}
	for _, tbl := range list.Tables {
		if !tbl.Parquet {
			t.Fatalf("table %s has no parquet after rebuild", tbl.Name)
		}
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/export/parquet/node", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("parquet node = %d: %s", rec.Code, rec.Body.String())
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, `filename="node.parquet"`) {
		t.Fatalf("parquet disposition = %q", cd)
	}
	if b := rec.Body.Bytes(); len(b) < 4 || string(b[:4]) != "PAR1" {
		t.Fatalf("parquet magic missing, starts %q", string(b[:min(4, len(b))]))
	}

	status, env := getError(t, h, "/api/v1/export/parquet/nope")
	if status != http.StatusNotFound || env.Code != codeUnknownTable {
		t.Fatalf("unknown table = %d/%s", status, env.Code)
	}
	status, env = getError(t, h, "/api/v1/export/parquet/"+strings.ReplaceAll("../node", "/", "%2F"))
	if status != http.StatusNotFound || env.Code != codeUnknownTable {
		t.Fatalf("traversal name = %d/%s", status, env.Code)
	}

	if err := os.Remove(parquetPath(srv.cfg, "node")); err != nil {
		t.Fatal(err)
	}
	status, env = getError(t, h, "/api/v1/export/parquet/node")
	if status != http.StatusNotFound || env.Code != codeParquetAbsent {
		t.Fatalf("absent parquet = %d/%s", status, env.Code)
	}
}

func TestAPIExportNotReady(t *testing.T) {
	srv, _ := newFixtureServer(t)
	h := srv.Routes()
	status, env := getError(t, h, "/api/v1/export")
	if status != http.StatusServiceUnavailable || env.Code != codeDBNotReady {
		t.Fatalf("export without sidecar = %d/%s", status, env.Code)
	}
}
