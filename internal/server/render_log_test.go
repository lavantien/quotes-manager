package server_test

import (
	"bytes"
	"errors"
	"log"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/lavantien/quotes-manager/internal/server"
)

type failingWriteRecorder struct {
	httptest.ResponseRecorder
}

func (f *failingWriteRecorder) Write([]byte) (int, error) { return 0, errors.New("write refused") }

func doFailingWrite(t *testing.T, srv *server.Server, method, target string, hdrs ...string) *failingWriteRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(""))
	for i := 0; i+1 < len(hdrs); i += 2 {
		req.Header.Set(hdrs[i], hdrs[i+1])
	}
	rec := &failingWriteRecorder{}
	srv.ServeHTTP(rec, req)
	return rec
}

func captureLog(t *testing.T, run func()) string {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	run()
	return buf.String()
}

func TestRenderLogsExecuteFailure(t *testing.T) {
	srv := newServer(t, newFake(sampleQuote(1)))
	out := captureLog(t, func() {
		doFailingWrite(t, srv, "GET", "/", "")
	})
	if !strings.Contains(out, `render "page"`) {
		t.Errorf("render error not logged, log = %q", out)
	}
}

func TestExecLogsExecuteFailure(t *testing.T) {
	srv := newServer(t, newFake(sampleQuote(1)))
	out := captureLog(t, func() {
		doFailingWrite(t, srv, "DELETE", "/quotes/1", "HX-Request", "true")
	})
	if !strings.Contains(out, `render "rail_left_oob"`) {
		t.Errorf("exec error not logged, log = %q", out)
	}
}
