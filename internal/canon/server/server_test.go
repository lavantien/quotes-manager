package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lavantien/quotes-manager/internal/canon/graph"
	"github.com/lavantien/quotes-manager/internal/canon/model"
	"github.com/lavantien/quotes-manager/internal/config"

	_ "github.com/duckdb/duckdb-go/v2"
)

func fixtureDB(t testing.TB, cfg *config.Config, runID, mn1Name string) string {
	t.Helper()
	path := cfg.Canon.DBFile(runID)
	s, err := graph.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
	l := graph.NewLoader(s, 100)
	if err := populateFixture(t.Context(), l, s, mn1Name); err != nil {
		_ = l.Flush()
		t.Fatalf("populate fixture: %v", err)
	}
	if err := s.Checkpoint(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := graph.PublishDB(cfg, runID, path); err != nil && !strings.Contains(err.Error(), "retiring") {
		t.Fatal(err)
	}
	return path
}

func sqlOpenDuck(path string) (*sql.DB, error) {
	return sql.Open("duckdb", path)
}

func populateFixture(ctx context.Context, l *graph.Loader, s *graph.Store, mn1Name string) error {
	ord := func(v int64) *int64 { return &v }

	div, err := l.EnsureNode(model.NodeDivision, "mn-div", `{"name":"Middle Length Discourses"}`)
	if err != nil {
		return err
	}
	mn1, err := l.EnsureNode(model.NodeText, "mn1", `{"name":"`+mn1Name+`"}`)
	if err != nil {
		return err
	}
	mn2, err := l.EnsureNode(model.NodeText, "mn2", `{}`)
	if err != nil {
		return err
	}
	if err := l.AddEdge(mn1, div, model.EdgeChildOf, ord(0), "{}"); err != nil {
		return err
	}
	if err := l.AddEdge(mn2, div, model.EdgeChildOf, ord(1), "{}"); err != nil {
		return err
	}

	type seg struct{ segKey, pali, en string }
	texts := map[string]int64{"mn1": mn1, "mn2": mn2}
	segs := map[string][]seg{
		"mn1": {
			{"0.1", "pariyāya", "the relational"},
			{"0.2", "ekodhāna", "settled"},
			{"0.3", "āsavā", ""},
		},
		"mn2": {{"0.1", "sabbāsava", "all the taints"}},
	}
	segIDs := map[string]int64{}
	for _, textKey := range []string{"mn1", "mn2"} {
		for i, sg := range segs[textKey] {
			paliKey := "pli:" + textKey + ":" + sg.segKey
			pid, err := l.EnsureNode(model.NodeSegment, paliKey, `{"text":"`+sg.pali+`"}`)
			if err != nil {
				return err
			}
			segIDs[paliKey] = pid
			if err := l.AddEdge(pid, texts[textKey], model.EdgeChildOf, ord(int64(i)), "{}"); err != nil {
				return err
			}
			if err := l.SetFTS(pid, sg.pali, sg.en); err != nil {
				return err
			}
			if sg.en != "" {
				enKey := "en/sujato:" + textKey + ":" + sg.segKey
				eid, err := l.EnsureNode(model.NodeSegment, enKey, `{"text":"`+sg.en+`"}`)
				if err != nil {
					return err
				}
				if err := l.AddEdge(eid, texts[textKey], model.EdgeChildOf, nil, "{}"); err != nil {
					return err
				}
				if err := l.AddEdge(eid, pid, model.EdgeTranslates, nil, `{"author":"sujato"}`); err != nil {
					return err
				}
			}
		}
	}

	root, err := l.EnsureNode(model.NodeDpdRoot, "dpd-root:dhṛ", `{"root":"dhṛ","root_meaning":"to hold"}`)
	if err != nil {
		return err
	}
	headword := func(lemma, data string) (int64, error) {
		return l.EnsureNode(model.NodeHeadword, "dpd:"+lemma, data)
	}
	dhamma, err := headword("dhamma", `{"lemma_1":"dhamma","pos":"m","meaning_1":"the teaching; nature","root_key":"dhṛ","family_root":"dhṛ"}`)
	if err != nil {
		return err
	}
	dhareti, err := headword("dhāreti", `{"lemma_1":"dhāreti","pos":"v","meaning_1":"bears, carries","root_key":"dhṛ"}`)
	if err != nil {
		return err
	}
	eva, err := headword("eva", `{"lemma_1":"eva","pos":"ind","meaning_1":"thus, in this way"}`)
	if err != nil {
		return err
	}
	if err := l.AddEdge(dhamma, root, model.EdgeDerivedFrom, nil, "{}"); err != nil {
		return err
	}
	if err := l.AddEdge(dhareti, root, model.EdgeDerivedFrom, nil, "{}"); err != nil {
		return err
	}
	form := func(form string, headwords ...string) error {
		raw, err := json.Marshal(headwords)
		if err != nil {
			return err
		}
		_, err = l.EnsureNode(model.NodeDpdForm, "dpd-form:"+form, `{"headwords":`+string(raw)+`}`)
		return err
	}
	if err := form("dhamma", "dhamma"); err != nil {
		return err
	}
	if err := form("evam", "eva"); err != nil {
		return err
	}
	hasWord := func(segKey string, dst int64) error {
		return l.AddEdge(segIDs[segKey], dst, model.EdgeHasWord, nil, `{"forms":["x"]}`)
	}
	for _, link := range []struct {
		segKey string
		dst    int64
	}{
		{"pli:mn1:0.1", dhamma},
		{"pli:mn1:0.2", dhamma},
		{"pli:mn2:0.1", dhamma},
		{"pli:mn1:0.3", dhareti},
		{"pli:mn1:0.1", eva},
	} {
		if err := hasWord(link.segKey, link.dst); err != nil {
			return err
		}
	}

	if _, err := l.EnsureNode(model.NodeEdition, "sujato/en/mn", `{"author":"sujato","collection":"mn"}`); err != nil {
		return err
	}
	if err := l.Flush(); err != nil {
		return err
	}
	if err := s.SetCoversEdge(ctx, "mn1", "sujato/en/mn", `{"tex":"mn.tex"}`); err != nil {
		return err
	}
	runID, err := s.NewRun("bilara1", "editions1", "dpd-tag-1")
	if err != nil {
		return err
	}
	if err := s.FinishRun(runID); err != nil {
		return err
	}
	return s.InsertDiscrepancies(ctx, runID, []model.Discrepancy{
		{Check: model.CheckEmptyValue, Severity: model.Info, Muid: "mn1", Detail: "empty root value at 0.3"},
		{Check: model.CheckDuplicateKeys, Severity: model.Error, Muid: "mn2", NodeID: 42, Detail: "duplicate key"},
	})
}

func newFixtureConfig(t testing.TB) *config.Config {
	t.Helper()
	cfg := config.Defaults()
	cfg.Canon.Paths.DataDir = t.TempDir()
	cfg.Canon.Paths.DBDir = filepath.Join(cfg.Canon.Paths.DataDir, "db")
	if err := os.MkdirAll(cfg.Canon.Paths.DBDir, 0o755); err != nil {
		t.Fatalf("create db dir: %v", err)
	}
	return cfg
}

func newFixtureServer(t testing.TB) (*Server, *config.Config) {
	t.Helper()
	cfg := newFixtureConfig(t)
	fixtureDB(t, cfg, "run1", "Mūlapariyāya")
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	return srv, cfg
}

func getJSON(t *testing.T, h http.Handler, target string, got any) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code >= 400 {
		t.Fatalf("GET %s = %d: %s", target, rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("GET %s content type = %q", target, ct)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), got); err != nil {
		t.Fatalf("GET %s body is not json: %v (%s)", target, err, rec.Body.String())
	}
	return rec
}

func getError(t *testing.T, h http.Handler, target string) (int, apiError) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var env errorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("GET %s error body is not json: %v (%s)", target, err, rec.Body.String())
	}
	return rec.Code, env.Error
}

func TestDBStateNotReadyBeforePublish(t *testing.T) {
	cfg := config.Defaults()
	cfg.Canon.Paths.DataDir = t.TempDir()
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Close() }()
	ctx := t.Context()
	if _, err := srv.db.Store(ctx); !errors.Is(err, ErrNotReady) {
		t.Fatalf("store before publish = %v, want ErrNotReady", err)
	}
	if _, err := srv.db.Run(ctx); !errors.Is(err, ErrNotReady) {
		t.Fatalf("run before publish = %v, want ErrNotReady", err)
	}
}

func TestDBStateResolvesPublishedPointer(t *testing.T) {
	srv, _ := newFixtureServer(t)
	ctx := t.Context()

	runID, err := srv.db.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if runID != "run1" {
		t.Fatalf("run = %q, want run1", runID)
	}
	store, err := srv.db.Store(ctx)
	if err != nil {
		t.Fatal(err)
	}
	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stats["node:text"] != 2 {
		t.Fatalf("node:text = %d, want 2", stats["node:text"])
	}

	if _, err := store.DB().ExecContext(ctx,
		`INSERT INTO node (type, key, data) VALUES ('text', 'x', '{}')`); err == nil {
		t.Fatal("write through the served pool must fail")
	}
}

func TestDBStateSwapOnPointerChange(t *testing.T) {
	srv, cfg := newFixtureServer(t)
	ctx := t.Context()

	first, err := srv.db.Store(ctx)
	if err != nil {
		t.Fatal(err)
	}
	fixtureDB(t, cfg, "run2", "Mūlapariyāya v2")

	if _, newPath, err := graph.CurrentDB(cfg); err != nil || newPath != cfg.Canon.DBFile("run2") {
		t.Fatalf("pointer after second publish = %v, %v", newPath, err)
	}

	runID, err := srv.db.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if runID != "run2" {
		t.Fatalf("run after swap = %q, want run2", runID)
	}
	if err := first.DB().QueryRowContext(ctx, `SELECT 1`).Scan(new(int)); err == nil ||
		!strings.Contains(err.Error(), "closed") {
		t.Fatalf("old pool query after swap = %v, want closed-pool error", err)
	}
}

func TestDBStateRefreshForcesResolve(t *testing.T) {
	cfg := newFixtureConfig(t)
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Close() }()
	if _, err := srv.db.Store(t.Context()); !errors.Is(err, ErrNotReady) {
		t.Fatalf("store before publish = %v, want ErrNotReady", err)
	}
	fixtureDB(t, cfg, "run1", "Mūlapariyāya")
	srv.db.Refresh()
	runID, err := srv.db.Run(t.Context())
	if err != nil || runID != "run1" {
		t.Fatalf("run after refresh = %q, %v", runID, err)
	}
}

func TestWriteErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"not ready", ErrNotReady, http.StatusServiceUnavailable, codeDBNotReady},
		{"invalid param", errInvalidParam("muid is required"), http.StatusBadRequest, codeInvalidParam},
		{"run not found", errNotFound(codeRunNotFound, "no run 9"), http.StatusNotFound, codeRunNotFound},
		{"internal", errors.New("boom"), http.StatusInternalServerError, codeInternal},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			writeError(rec, tc.err)
			var env errorEnvelope
			if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
				t.Fatalf("body is not json: %v (%s)", err, rec.Body.String())
			}
			if rec.Code != tc.status || env.Error.Code != tc.code {
				t.Fatalf("writeError = %d/%s, want %d/%s (body %s)",
					rec.Code, env.Error.Code, tc.status, tc.code, rec.Body.String())
			}
		})
	}
}

func TestMiddlewareRecoverPanic(t *testing.T) {
	srv, _ := newFixtureServer(t)
	srv.log = slogDiscard()
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		panic("boom")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	srv.recoverPanic(panicHandler).ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("api panic status = %d, want 500", rec.Code)
	}
	var env errorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("api panic body is not json: %v (%s)", err, rec.Body.String())
	}
	if env.Error.Code != codeInternal {
		t.Fatalf("api panic code = %s, want %s", env.Error.Code, codeInternal)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/pages/x", nil)
	srv.recoverPanic(panicHandler).ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("page panic status = %d, want 500", rec.Code)
	}
	if !strings.HasPrefix(rec.Body.String(), "internal error") {
		t.Fatalf("page panic body = %q, want plain text", rec.Body.String())
	}
}

func TestRegisterRoutesSharedMux(t *testing.T) {
	srv, _ := newFixtureServer(t)
	m := http.NewServeMux()
	srv.RegisterRoutes(m)

	wantHTML(t, getPage(t, m, "/dashboard"), http.StatusOK, "dashboard")
	wantHTML(t, getPage(t, m, "/texts"), http.StatusOK, "texts")
	wantHTML(t, getPage(t, m, "/words"), http.StatusOK, "words")
	wantHTML(t, getPage(t, m, "/discrepancies"), http.StatusOK, "discrepancies")

	status, body := getError(t, m, "/api/v1/nope")
	if status != http.StatusNotFound || body.Code != codeNotFound {
		t.Fatalf("unknown api route = %d/%s", status, body.Code)
	}

	if rec := getPage(t, m, "/"); rec.Code != http.StatusNotFound {
		t.Fatalf("root on bare canon registration = %d, want 404", rec.Code)
	}
}

func TestMiddlewareOriginCheck(t *testing.T) {
	srv, _ := newFixtureServer(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := srv.checkOrigin(next)

	for _, tc := range []struct {
		name   string
		method string
		host   string
		origin string
		status int
	}{
		{"post same origin localhost", http.MethodPost, "localhost:8080", "http://localhost:8080", http.StatusOK},
		{"post same origin loopback", http.MethodPost, "127.0.0.1:8080", "http://127.0.0.1:8080", http.StatusOK},
		{"post cross origin", http.MethodPost, "127.0.0.1:8080", "http://evil.example", http.StatusForbidden},
		{"post origin host mismatch", http.MethodPost, "localhost:8080", "http://127.0.0.1:8080", http.StatusForbidden},
		{"post no origin", http.MethodPost, "127.0.0.1:8080", "", http.StatusOK},
		{"get cross origin", http.MethodGet, "127.0.0.1:8080", "http://evil.example", http.StatusOK},
		{"post malformed origin", http.MethodPost, "127.0.0.1:8080", "://", http.StatusForbidden},
		{"post null origin", http.MethodPost, "127.0.0.1:8080", "null", http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/api/v1/jobs", nil)
			req.Host = tc.host
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d", rec.Code, tc.status)
			}
		})
	}
}

func TestMiddlewareSecurityHeaders(t *testing.T) {
	srv := &Server{}
	for _, tc := range []struct {
		name        string
		contentType string
		csp         bool
	}{
		{"html gets csp", "text/html; charset=utf-8", true},
		{"json has no csp", "application/json", false},
		{"unset content type has no csp", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tc.contentType != "" {
					w.Header().Set("Content-Type", tc.contentType)
				}
				w.WriteHeader(http.StatusOK)
			})
			rec := httptest.NewRecorder()
			srv.securityHeaders(next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
			if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
				t.Fatalf("nosniff = %q", got)
			}
			_, hasCSP := rec.Header()["Content-Security-Policy"]
			if hasCSP != tc.csp {
				t.Fatalf("csp present = %v, want %v", hasCSP, tc.csp)
			}
			if hasCSP && rec.Header().Get("Content-Security-Policy") != "default-src 'self'" {
				t.Fatalf("csp = %q", rec.Header().Get("Content-Security-Policy"))
			}
		})
	}
}

func TestMiddlewareRequestLogging(t *testing.T) {
	srv := &Server{}
	srv.log = slogDiscard()
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	rec := httptest.NewRecorder()
	srv.logRequests(next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rec.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want passthrough %d", rec.Code, http.StatusTeapot)
	}
}

type headerSpy struct {
	http.ResponseWriter
	codes []int
}

func (s *headerSpy) WriteHeader(code int) {
	s.codes = append(s.codes, code)
	s.ResponseWriter.WriteHeader(code)
}

func TestMiddlewareStatusRecorderCapture(t *testing.T) {
	rec := &statusRecorder{ResponseWriter: httptest.NewRecorder()}
	rec.WriteHeader(http.StatusCreated)
	if rec.status != http.StatusCreated {
		t.Fatalf("status after WriteHeader = %d, want %d", rec.status, http.StatusCreated)
	}
	rec.WriteHeader(http.StatusInternalServerError)
	if rec.status != http.StatusCreated {
		t.Fatalf("status after repeat WriteHeader = %d, want %d", rec.status, http.StatusCreated)
	}

	spy := &headerSpy{ResponseWriter: httptest.NewRecorder()}
	rec = &statusRecorder{ResponseWriter: spy}
	rec.WriteHeader(http.StatusNotFound)
	if _, err := rec.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	if rec.status != http.StatusNotFound {
		t.Fatalf("status after Write = %d, want %d", rec.status, http.StatusNotFound)
	}
	if len(spy.codes) != 1 || spy.codes[0] != http.StatusNotFound {
		t.Fatalf("forwarded headers = %v, want exactly [%d]", spy.codes, http.StatusNotFound)
	}

	spy = &headerSpy{ResponseWriter: httptest.NewRecorder()}
	rec = &statusRecorder{ResponseWriter: spy}
	if _, err := rec.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	if rec.status != http.StatusOK {
		t.Fatalf("implicit status = %d, want %d", rec.status, http.StatusOK)
	}
	if len(spy.codes) != 1 || spy.codes[0] != http.StatusOK {
		t.Fatalf("implicit forwarded headers = %v, want exactly [%d]", spy.codes, http.StatusOK)
	}
}

type logSink struct {
	records []slog.Record
}

func (s *logSink) Enabled(context.Context, slog.Level) bool { return true }
func (s *logSink) Handle(_ context.Context, r slog.Record) error {
	s.records = append(s.records, r)
	return nil
}
func (s *logSink) WithAttrs([]slog.Attr) slog.Handler { return s }
func (s *logSink) WithGroup(string) slog.Handler      { return s }

func loggedStatus(t *testing.T, s *logSink) []int64 {
	t.Helper()
	var out []int64
	for _, r := range s.records {
		if r.Message != "request" {
			continue
		}
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == "status" {
				out = append(out, a.Value.Int64())
			}
			return true
		})
	}
	return out
}

func TestMiddlewareLoggedStatusValues(t *testing.T) {
	srv := &Server{}

	sink := &logSink{}
	srv.log = slog.New(sink)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("x"))
	})
	srv.logRequests(next).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))
	if got := loggedStatus(t, sink); len(got) != 1 || got[0] != http.StatusNotFound {
		t.Fatalf("logged status = %v, want [%d]", got, http.StatusNotFound)
	}

	sink = &logSink{}
	srv.log = slog.New(sink)
	next = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	srv.logRequests(next).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))
	if got := loggedStatus(t, sink); len(got) != 1 || got[0] != http.StatusOK {
		t.Fatalf("implicit logged status = %v, want [%d]", got, http.StatusOK)
	}
}

func TestHookTrackerSignalsIdle(t *testing.T) {
	h := newHookTracker()

	start := time.Now()
	h.wait(300 * time.Millisecond)
	if elapsed := time.Since(start); elapsed >= 150*time.Millisecond {
		t.Fatalf("fresh tracker wait took %v, want immediate", elapsed)
	}

	h.begin()
	h.end()
	start = time.Now()
	h.wait(300 * time.Millisecond)
	if elapsed := time.Since(start); elapsed >= 150*time.Millisecond {
		t.Fatalf("wait after end took %v, want immediate", elapsed)
	}
}

func TestServeSurfacesListenerError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()

	cfg := config.Defaults()
	cfg.Canon.Paths.DataDir = t.TempDir()
	cfg.Server.Addr = ln.Addr().String()
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	withFakeExec(t, srv, func(context.Context, io.Writer) error {
		time.Sleep(150 * time.Millisecond)
		return nil
	})
	if _, err := srv.startJob("sync", nil); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx) }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Serve must surface the listener error, got nil")
		}
		if !strings.Contains(err.Error(), "server:") {
			t.Fatalf("Serve error = %v, want the wrapped listener error", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("Serve did not shut down")
	}
}

func TestServerRoutesUnknownAPIPath(t *testing.T) {
	srv, _ := newFixtureServer(t)
	h := srv.Routes()
	status, body := getError(t, h, "/api/v1/nope")
	if status != http.StatusNotFound || body.Code != codeNotFound {
		t.Fatalf("unknown api path = %d/%s", status, body.Code)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stats", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("wrong method status = %d, want 404", rec.Code)
	}
}

func TestParamsLimit(t *testing.T) {
	srv, _ := newFixtureServer(t)
	for _, tc := range []struct {
		raw    string
		want   int
		errMsg string
	}{
		{"", 50, ""},
		{"10", 10, ""},
		{"0", 50, ""},
		{"-5", 50, ""},
		{"99999", 500, ""},
		{"501", 500, ""},
		{"abc", 0, "limit"},
		{"1e3", 0, "limit"},
	} {
		req := httptest.NewRequest(http.MethodGet, "/x?limit="+tc.raw, nil)
		got, err := srv.queryLimit(req)
		if tc.errMsg != "" {
			if err == nil || !strings.Contains(err.Error(), tc.errMsg) {
				t.Fatalf("limit %q err = %v, want %s", tc.raw, err, tc.errMsg)
			}
			continue
		}
		if err != nil {
			t.Fatalf("limit %q err = %v", tc.raw, err)
		}
		if got != tc.want {
			t.Fatalf("limit %q = %d, want %d", tc.raw, got, tc.want)
		}
	}
}

func TestParamsOffsetAndEnumAndFlag(t *testing.T) {
	for _, tc := range []struct {
		raw    string
		want   int
		errMsg string
	}{
		{"", 0, ""},
		{"5", 5, ""},
		{"-1", 0, "offset"},
		{"x", 0, "offset"},
	} {
		req := httptest.NewRequest(http.MethodGet, "/x?offset="+tc.raw, nil)
		got, err := queryOffset(req)
		if tc.errMsg != "" {
			if err == nil || !strings.Contains(err.Error(), tc.errMsg) {
				t.Fatalf("offset %q err = %v, want %s", tc.raw, err, tc.errMsg)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Fatalf("offset %q = %d, %v, want %d", tc.raw, got, err, tc.want)
		}
	}

	langReq := func(raw string) *http.Request {
		return httptest.NewRequest(http.MethodGet, "/x?lang="+raw, nil)
	}
	if got, err := queryEnum(langReq(""), "lang", "pali", "pali", "en"); err != nil || got != "pali" {
		t.Fatalf("lang default = %q, %v", got, err)
	}
	if got, err := queryEnum(langReq("en"), "lang", "pali", "pali", "en"); err != nil || got != "en" {
		t.Fatalf("lang en = %q, %v", got, err)
	}
	if _, err := queryEnum(langReq("fr"), "lang", "pali", "pali", "en"); err == nil {
		t.Fatal("lang fr must reject")
	}

	flagReq := func(target string) *http.Request {
		return httptest.NewRequest(http.MethodGet, target, nil)
	}
	for _, tc := range []struct {
		target string
		want   bool
		errMsg string
	}{
		{"/x", false, ""},
		{"/x?with_en", true, ""},
		{"/x?with_en=true", true, ""},
		{"/x?with_en=0", false, ""},
		{"/x?with_en=yes", false, "with_en"},
	} {
		got, err := queryFlag(flagReq(tc.target), "with_en")
		if tc.errMsg != "" {
			if err == nil || !strings.Contains(err.Error(), tc.errMsg) {
				t.Fatalf("flag %q err = %v, want %s", tc.target, err, tc.errMsg)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Fatalf("flag %q = %v, %v, want %v", tc.target, got, err, tc.want)
		}
	}

	runReq := httptest.NewRequest(http.MethodGet, "/x?run=7", nil)
	if got, err := queryInt64(runReq, "run", 0); err != nil || got != 7 {
		t.Fatalf("run = %d, %v", got, err)
	}
	badRun := httptest.NewRequest(http.MethodGet, "/x?run=abc", nil)
	if _, err := queryInt64(badRun, "run", 0); err == nil {
		t.Fatal("malformed run must reject")
	}
}

func TestDuckStateAbsentSidecar(t *testing.T) {
	cfg := config.Defaults()
	cfg.Canon.Paths.DataDir = t.TempDir()
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Close() }()
	if _, err := srv.duck.State(t.Context()); !errors.Is(err, ErrNotReady) {
		t.Fatalf("duck state without sidecar = %v, want ErrNotReady", err)
	}
}

func TestDuckStateLifecycle(t *testing.T) {
	cfg := config.Defaults()
	cfg.Canon.Paths.DataDir = t.TempDir()
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Close() }()
	ctx := t.Context()

	writeSidecar := func(createSQL string) {
		t.Helper()
		db, err := sqlOpenDuck(cfg.Canon.DuckDBFile())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(createSQL); err != nil {
			_ = db.Close()
			t.Fatalf("create sidecar: %v", err)
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}

	writeSidecar(`CREATE TABLE t1 (x INTEGER)`)
	db, err := srv.duck.State(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM t1`).Scan(&n); err != nil {
		t.Fatalf("query sidecar: %v", err)
	}

	if err := srv.duck.Quiesce(); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.duck.State(ctx); !errors.Is(err, ErrNotReady) {
		t.Fatalf("state while rebuilding = %v, want ErrNotReady", err)
	}
	if err := os.Remove(cfg.Canon.DuckDBFile()); err != nil {
		t.Fatal(err)
	}
	writeSidecar(`CREATE TABLE t2 (y INTEGER)`)
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(cfg.Canon.DuckDBFile(), future, future); err != nil {
		t.Fatal(err)
	}
	srv.duck.Reopen()
	db, err = srv.duck.State(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM t2`).Scan(&n); err != nil {
		t.Fatalf("query rebuilt sidecar: %v", err)
	}
	if _, err := db.QueryContext(ctx, `SELECT * FROM t1`); err == nil {
		t.Fatal("old table must be gone after rebuild")
	}
}

func writeDuckSidecar(t *testing.T, cfg *config.Config) {
	t.Helper()
	db, err := sqlOpenDuck(cfg.Canon.DuckDBFile())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE t1 (x INTEGER)`); err != nil {
		_ = db.Close()
		t.Fatalf("create sidecar: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestDuckStateQuiesceDrainsBriefReader(t *testing.T) {
	cfg := config.Defaults()
	cfg.Canon.Paths.DataDir = t.TempDir()
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Close() }()
	writeDuckSidecar(t, cfg)

	db, err := srv.duck.State(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	conn, err := db.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	time.AfterFunc(300*time.Millisecond, func() { _ = conn.Close() })

	done := make(chan error, 1)
	go func() { done <- srv.duck.Quiesce() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("quiesce must drain a brief reader, got %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("quiesce must be bounded, not hang")
	}
	srv.duck.Reopen()
}

func TestDuckStateQuiesceFailsOnHeldReader(t *testing.T) {
	cfg := config.Defaults()
	cfg.Canon.Paths.DataDir = t.TempDir()
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Close() }()
	writeDuckSidecar(t, cfg)

	db, err := srv.duck.State(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	conn, err := db.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	done := make(chan error, 1)
	go func() { done <- srv.duck.Quiesce() }()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "in-flight") {
			t.Fatalf("held reader must fail the quiesce, got %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("quiesce must fail fast, not hang on a held reader")
	}
	srv.duck.Reopen()
}

func TestServeGracefulShutdown(t *testing.T) {
	cfg := config.Defaults()
	cfg.Canon.Paths.DataDir = t.TempDir()
	cfg.Server.Addr = "127.0.0.1:0"
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	go func() {
		done <- srv.Serve(ctx)
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve returned %v, want nil", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("Serve did not shut down")
	}
}

func slogDiscard() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
