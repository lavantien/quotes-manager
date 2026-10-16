package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/lavantien/quotes-manager/internal/config"
)

var sharedApp struct {
	once sync.Once
	app  *mergedApp
	tmp  string
	err  error
}

func mergedShared(t *testing.T) *mergedApp {
	t.Helper()
	sharedApp.once.Do(func() {
		sharedApp.tmp, sharedApp.err = buildSharedApp()
	})
	if sharedApp.err != nil {
		t.Fatalf("shared merged fixture: %v", sharedApp.err)
	}
	return sharedApp.app
}

func buildSharedApp() (string, error) {
	tmp, err := os.MkdirTemp("", "qm-e2e-*")
	if err != nil {
		return "", err
	}
	cfgPath, err := writeMergedConfig(tmp, filepath.Join(tmp, "quotes.db"))
	if err != nil {
		_ = os.RemoveAll(tmp)
		return "", err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		_ = os.RemoveAll(tmp)
		return "", err
	}
	app, err := newMergedApp(cfg, true)
	if err != nil {
		_ = os.RemoveAll(tmp)
		return "", err
	}
	if err := runCanonPipeline(app); err != nil {
		_ = app.close()
		_ = os.RemoveAll(tmp)
		return "", err
	}
	sharedApp.app = app
	return tmp, nil
}

func TestMain(m *testing.M) {
	code := m.Run()
	if sharedApp.app != nil {
		_ = sharedApp.app.close()
		_ = os.RemoveAll(sharedApp.tmp)
	}
	os.Exit(code)
}

type statusBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func wantStatusBody(t *testing.T, f fetched, wantStatus int, wantCode string) {
	t.Helper()
	if f.status != wantStatus {
		t.Fatalf("status = %d, want %d (%s)", f.status, wantStatus, clip(f.body))
	}
	var env statusBody
	if err := json.Unmarshal([]byte(f.body), &env); err != nil {
		t.Fatalf("error envelope json: %v (%s)", err, f.body)
	}
	if env.Error.Code != wantCode {
		t.Fatalf("error code = %q, want %q (%s)", env.Error.Code, wantCode, f.body)
	}
}

func clip(s string) string {
	if len(s) > 500 {
		return s[:500] + "..."
	}
	return s
}
