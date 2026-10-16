package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lavantien/quotes-manager/internal/canon/jobs"
)

func withFakeExec(t testing.TB, srv *Server, exec func(ctx context.Context, out io.Writer) error) {
	t.Helper()
	prev := func(ctx context.Context, stage string, args []string, out io.Writer) error {
		return srv.defaultStageExec(ctx, stage, args, out)
	}
	srv.setStageExec(func(ctx context.Context, _ string, _ []string, out io.Writer) error {
		return exec(ctx, out)
	})
	t.Cleanup(func() { srv.setStageExec(prev) })
}

func postBody(t testing.TB, h http.Handler, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func waitJobState(t *testing.T, h http.Handler, id int64, want jobs.State) jobs.JobSnapshot {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		var snap jobs.JobSnapshot
		getJSON(t, h, fmt.Sprintf("/api/v1/jobs/%d", id), &snap)
		if snap.State == want {
			return snap
		}
		if snap.State == jobs.StateDone || snap.State == jobs.StateError ||
			snap.State == jobs.StateCanceled {
			t.Fatalf("job reached %s (%s), want %s", snap.State, snap.Error, want)
		}
		if time.Now().After(deadline) {
			t.Fatalf("job stuck in %s, want %s", snap.State, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestAPIJobsLifecycle(t *testing.T) {
	srv, _ := newFixtureServer(t)
	release := make(chan struct{})
	withFakeExec(t, srv, func(_ context.Context, out io.Writer) error {
		<-release
		_, _ = io.WriteString(out, "line one\nline two\n")
		return nil
	})
	h := srv.Routes()

	rec := postBody(t, h, "/api/v1/jobs", `{"stage":"export"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("start job = %d: %s", rec.Code, rec.Body.String())
	}
	var snap jobs.JobSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("start body = %s", rec.Body.String())
	}
	if snap.ID != 1 || snap.Stage != "export" {
		t.Fatalf("snapshot = %+v", snap)
	}
	if snap.State != jobs.StateQueued && snap.State != jobs.StateRunning {
		t.Fatalf("initial state = %s", snap.State)
	}

	rec = postBody(t, h, "/api/v1/jobs", `{"stage":"sync"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("second start = %d: %s", rec.Code, rec.Body.String())
	}
	var env errorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil || env.Error.Code != codeJobBusy {
		t.Fatalf("busy body = %s", rec.Body.String())
	}

	var list []jobs.JobSnapshot
	getJSON(t, h, "/api/v1/jobs", &list)
	if len(list) != 1 || list[0].ID != 1 {
		t.Fatalf("list = %+v", list)
	}

	close(release)
	done := waitJobState(t, h, 1, jobs.StateDone)
	if done.Error != "" {
		t.Fatalf("done snapshot error = %q", done.Error)
	}
	if done.Started == "" || done.Finished == "" {
		t.Fatalf("done snapshot timestamps = %+v", done)
	}

	var lines []jobs.JobLogLine
	getJSON(t, h, "/api/v1/jobs/1/logs", &lines)
	if len(lines) != 2 || lines[0].Seq != 1 || lines[0].Text != "line one" ||
		lines[1].Text != "line two" {
		t.Fatalf("logs = %+v", lines)
	}
	getJSON(t, h, "/api/v1/jobs/1/logs?after=1", &lines)
	if len(lines) != 1 || lines[0].Text != "line two" {
		t.Fatalf("logs after 1 = %+v", lines)
	}
	getJSON(t, h, "/api/v1/jobs/1/logs?after=99", &lines)
	if len(lines) != 0 {
		t.Fatalf("logs after 99 = %+v", lines)
	}

	rec = postBody(t, h, "/api/v1/jobs/1/cancel", "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("cancel done job = %d: %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil || env.Error.Code != codeNotCancelable {
		t.Fatalf("not cancelable body = %s", rec.Body.String())
	}
}

func TestAPIJobsArgsReachExecutor(t *testing.T) {
	srv, _ := newFixtureServer(t)
	var mu sync.Mutex
	var gotStage string
	var gotArgs []string
	prev := func(ctx context.Context, stage string, args []string, out io.Writer) error {
		return srv.defaultStageExec(ctx, stage, args, out)
	}
	srv.setStageExec(func(_ context.Context, stage string, args []string, _ io.Writer) error {
		mu.Lock()
		gotStage, gotArgs = stage, args
		mu.Unlock()
		return nil
	})
	t.Cleanup(func() { srv.setStageExec(prev) })
	h := srv.Routes()

	rec := postBody(t, h, "/api/v1/jobs", `{"stage":"prune","args":["--source","bilara"]}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("start with args = %d: %s", rec.Code, rec.Body.String())
	}
	waitJobState(t, h, 1, jobs.StateDone)
	mu.Lock()
	defer mu.Unlock()
	if gotStage != "prune" || len(gotArgs) != 2 || gotArgs[0] != "--source" || gotArgs[1] != "bilara" {
		t.Fatalf("executor saw stage %q args %v", gotStage, gotArgs)
	}
}

func TestAPIJobsArgsLimit(t *testing.T) {
	srv, _ := newFixtureServer(t)
	withFakeExec(t, srv, func(context.Context, io.Writer) error { return nil })
	h := srv.Routes()

	build := func(n int) string {
		args := make([]string, n)
		for i := range args {
			args[i] = "--flag" + strconv.Itoa(i)
		}
		raw, err := json.Marshal(struct {
			Stage string   `json:"stage"`
			Args  []string `json:"args"`
		}{"sync", args})
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}

	rec := postBody(t, h, "/api/v1/jobs", build(maxJobArgs))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("%d args = %d: %s", maxJobArgs, rec.Code, rec.Body.String())
	}
	waitJobState(t, h, 1, jobs.StateDone)

	rec = postBody(t, h, "/api/v1/jobs", build(maxJobArgs+1))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("%d args = %d: %s", maxJobArgs+1, rec.Code, rec.Body.String())
	}
	var env errorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil || env.Error.Code != codeArgsInvalid {
		t.Fatalf("args limit body = %s", rec.Body.String())
	}
}

func TestAPIJobsUnknownFlagArgFailsJob(t *testing.T) {
	srv, _ := newFixtureServer(t)
	h := srv.Routes()

	rec := postBody(t, h, "/api/v1/jobs", `{"stage":"prune","args":["--nope"]}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("start = %d: %s", rec.Code, rec.Body.String())
	}
	snap := waitJobState(t, h, 1, jobs.StateError)
	if !strings.Contains(snap.Error, "nope") {
		t.Fatalf("error snapshot = %+v, want the flag named", snap)
	}
}

func TestAPIJobsReportOutConfined(t *testing.T) {
	srv, _ := newFixtureServer(t)
	withFakeExec(t, srv, func(context.Context, io.Writer) error { return nil })
	h := srv.Routes()

	outside := t.TempDir()
	under := filepath.Join(srv.cfg.Canon.Paths.ReportsDir, "custom")
	for _, body := range []string{
		fmt.Sprintf(`{"stage":"report","args":["--out",%q]}`, filepath.ToSlash(outside)),
		`{"stage":"report","args":["--out","../escape"]}`,
		`{"stage":"report","args":["--out=../escape"]}`,
		`{"stage":"report","args":["--out"]}`,
	} {
		rec := postBody(t, h, "/api/v1/jobs", body)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("outside --out body %s = %d: %s", body, rec.Code, rec.Body.String())
		}
		var env errorEnvelope
		if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil || env.Error.Code != codeArgsInvalid {
			t.Fatalf("outside --out body %s = %s", body, rec.Body.String())
		}
	}

	rec := postBody(t, h, "/api/v1/jobs",
		fmt.Sprintf(`{"stage":"report","args":["--out",%q]}`, filepath.ToSlash(under)))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("under-root --out = %d: %s", rec.Code, rec.Body.String())
	}
	waitJobState(t, h, 1, jobs.StateDone)

	rec = postBody(t, h, "/api/v1/jobs", `{"stage":"prune","args":["--source","bilara"]}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("non-report args = %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAPIJobsInvalidRequests(t *testing.T) {
	srv, _ := newFixtureServer(t)
	withFakeExec(t, srv, func(context.Context, io.Writer) error { return nil })
	h := srv.Routes()

	for _, tc := range []struct {
		name   string
		body   string
		status int
		code   string
	}{
		{"malformed json", `{`, http.StatusBadRequest, codeInvalidParam},
		{"empty body", ``, http.StatusBadRequest, codeInvalidParam},
		{"wrong shape", `["sync"]`, http.StatusBadRequest, codeInvalidParam},
		{"args wrong type", `{"stage":"sync","args":"--x"}`, http.StatusBadRequest, codeInvalidParam},
		{"trailing data", `{"stage":"sync"} trailing`, http.StatusBadRequest, codeInvalidParam},
		{"second object", `{"stage":"sync"}{"stage":"sync"}`, http.StatusBadRequest, codeInvalidParam},
		{"unknown stage", `{"stage":"serve"}`, http.StatusBadRequest, codeStageInvalid},
		{"empty stage", `{"stage":""}`, http.StatusBadRequest, codeStageInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := postBody(t, h, "/api/v1/jobs", tc.body)
			if rec.Code != tc.status {
				t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
			}
			var env errorEnvelope
			if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil || env.Error.Code != tc.code {
				t.Fatalf("body = %s", rec.Body.String())
			}
		})
	}

	for _, target := range []string{
		"/api/v1/jobs/999", "/api/v1/jobs/999/logs", "/api/v1/jobs/999/cancel",
		"/api/v1/jobs/abc", "/api/v1/jobs/abc/logs",
	} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		if strings.HasSuffix(target, "/cancel") {
			req.Method = http.MethodPost
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("GET %s = %d: %s", target, rec.Code, rec.Body.String())
		}
		var env errorEnvelope
		if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil || env.Error.Code != codeJobNotFound {
			t.Fatalf("%s body = %s", target, rec.Body.String())
		}
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/jobs/1/logs?after=x", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("logs after=x = %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAPIJobsCancelRunningJob(t *testing.T) {
	srv, _ := newFixtureServer(t)
	withFakeExec(t, srv, func(ctx context.Context, _ io.Writer) error {
		<-ctx.Done()
		return context.Canceled
	})
	h := srv.Routes()

	rec := postBody(t, h, "/api/v1/jobs", `{"stage":"sync"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("start = %d: %s", rec.Code, rec.Body.String())
	}

	rec = postBody(t, h, "/api/v1/jobs/1/cancel", "")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("cancel = %d: %s", rec.Code, rec.Body.String())
	}
	var snap jobs.JobSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("cancel body = %s", rec.Body.String())
	}
	canceled := waitJobState(t, h, 1, jobs.StateCanceled)
	if canceled.ID != 1 {
		t.Fatalf("canceled snapshot = %+v", canceled)
	}
}

func TestAPIJobsErrorState(t *testing.T) {
	srv, _ := newFixtureServer(t)
	withFakeExec(t, srv, func(context.Context, io.Writer) error {
		return errors.New("sync failed: network")
	})
	h := srv.Routes()

	rec := postBody(t, h, "/api/v1/jobs", `{"stage":"sync"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("start = %d: %s", rec.Code, rec.Body.String())
	}
	snap := waitJobState(t, h, 1, jobs.StateError)
	if !strings.Contains(snap.Error, "sync failed") {
		t.Fatalf("error snapshot = %+v", snap)
	}
	var list []jobs.JobSnapshot
	getJSON(t, h, "/api/v1/jobs", &list)
	if len(list) != 1 || list[0].State != jobs.StateError {
		t.Fatalf("list = %+v", list)
	}
}

func TestJobsFinishHookReopensSidecar(t *testing.T) {
	cfg := newFixtureConfig(t)
	fixtureDB(t, cfg, "run1", "Mūlapariyāya")
	writeSidecarTable(t, cfg, "t1")
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })

	ctx := t.Context()
	if _, err := srv.duck.State(ctx); err != nil {
		t.Fatal(err)
	}
	duckStamp := func() time.Time {
		srv.duck.mu.Lock()
		defer srv.duck.mu.Unlock()
		return srv.duck.modTime
	}
	if duckStamp().IsZero() {
		t.Fatal("test premise: opening the sidecar must stamp the mtime")
	}

	release := make(chan struct{})
	withFakeExec(t, srv, func(_ context.Context, _ io.Writer) error {
		<-release
		return nil
	})
	h := srv.Routes()
	if rec := postBody(t, h, "/api/v1/jobs", `{"stage":"sync"}`); rec.Code != http.StatusAccepted {
		t.Fatalf("start sync job = %d: %s", rec.Code, rec.Body.String())
	}
	close(release)
	waitJobState(t, h, 1, jobs.StateDone)
	if duckStamp().IsZero() {
		t.Fatal("non-rebuilding stage must not drop the duck mtime")
	}

	if rec := postBody(t, h, "/api/v1/jobs", `{"stage":"export"}`); rec.Code != http.StatusAccepted {
		t.Fatalf("start export job = %d: %s", rec.Code, rec.Body.String())
	}
	if err := srv.duck.Quiesce(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(cfg.Canon.DuckDBFile()); err != nil {
		t.Fatal(err)
	}
	writeSidecarTable(t, cfg, "t2")
	waitJobState(t, h, 2, jobs.StateDone)
	deadline := time.Now().Add(5 * time.Second)
	for !duckStamp().IsZero() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if !duckStamp().IsZero() {
		t.Fatal("finish hook must drop the duck mtime for export stages")
	}

	db, err := srv.duck.State(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.QueryContext(ctx, `SELECT COUNT(*) FROM t2`); err != nil {
		t.Fatalf("query rebuilt sidecar: %v", err)
	}
	if _, err := db.QueryContext(ctx, `SELECT * FROM t1`); err == nil {
		t.Fatal("old table must be gone after the hook reopened the pool")
	}
}

func TestServeCancelsRunningJobOnShutdown(t *testing.T) {
	cfg := newFixtureConfig(t)
	fixtureDB(t, cfg, "run1", "Mūlapariyāya")
	cfg.Server.Addr = "127.0.0.1:0"
	cfg.Server.ShutdownTimeoutSeconds = 5
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	withFakeExec(t, srv, func(ctx context.Context, _ io.Writer) error {
		<-ctx.Done()
		return context.Canceled
	})
	h := srv.Routes()
	if rec := postBody(t, h, "/api/v1/jobs", `{"stage":"sync"}`); rec.Code != http.StatusAccepted {
		t.Fatalf("start = %d: %s", rec.Code, rec.Body.String())
	}

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	go func() {
		done <- srv.Serve(ctx)
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve returned %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("Serve did not shut down")
	}
	for _, j := range srv.jobs.List() {
		if j.State() != jobs.StateCanceled {
			t.Fatalf("job after shutdown = %s, want canceled", j.State())
		}
	}
}
