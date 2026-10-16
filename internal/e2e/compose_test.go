package e2e

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	canonserver "github.com/lavantien/quotes-manager/internal/canon/server"
	"github.com/lavantien/quotes-manager/internal/config"
	"github.com/lavantien/quotes-manager/internal/seed"
	"github.com/lavantien/quotes-manager/internal/server"
	"github.com/lavantien/quotes-manager/internal/store"
)

type mergedApp struct {
	cfg   *config.Config
	store *store.SQLiteStore
	canon *canonserver.Server
	root  *http.ServeMux
	h     http.Handler
	srv   *httptest.Server
}

func newMergedApp(cfg *config.Config, seeded bool) (*mergedApp, error) {
	st, err := store.Open(cfg.Quotes.DBPath)
	if err != nil {
		return nil, err
	}
	if seeded {
		if err := seed.EnsureSeeded(st.DB()); err != nil {
			_ = st.Close()
			return nil, err
		}
	}
	canonSrv, err := canonserver.New(cfg)
	if err != nil {
		_ = st.Close()
		return nil, err
	}
	root := http.NewServeMux()
	server.New(st, server.WithLimits(cfg.Server.DefaultLimit, cfg.Server.MaxLimit)).RegisterRoutes(root)
	canonSrv.RegisterRoutes(root)
	return &mergedApp{cfg: cfg, store: st, canon: canonSrv, root: root, h: canonSrv.Chain(root)}, nil
}

func (a *mergedApp) close() error {
	if a.srv != nil {
		a.srv.Close()
	}
	return errors.Join(a.canon.Close(), a.store.Close())
}

func (a *mergedApp) server() *httptest.Server {
	if a.srv == nil {
		a.srv = httptest.NewServer(a.h)
	}
	return a.srv
}

type fetched struct {
	status int
	header http.Header
	body   string
}

func (a *mergedApp) fetch(method, path, contentType, body, origin string) (fetched, error) {
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, a.server().URL+path, rdr)
	if err != nil {
		return fetched{}, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	resp, err := a.server().Client().Do(req)
	if err != nil {
		return fetched{}, fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fetched{}, fmt.Errorf("%s %s body: %w", method, path, err)
	}
	return fetched{status: resp.StatusCode, header: resp.Header, body: string(raw)}, nil
}

func mustFetch(t *testing.T, f fetched, err error) fetched {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func mergedAppFor(t *testing.T, seeded bool) *mergedApp {
	t.Helper()
	tmp := t.TempDir()
	cfgPath, err := writeMergedConfig(tmp, filepath.Join(tmp, "quotes.db"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	app, err := newMergedApp(cfg, seeded)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = app.close()
		_ = os.RemoveAll(tmp)
	})
	return app
}

func (a *mergedApp) get(t *testing.T, path string) fetched {
	t.Helper()
	f, err := a.fetch(http.MethodGet, path, "", "", "")
	return mustFetch(t, f, err)
}

func (a *mergedApp) postForm(t *testing.T, path string, form url.Values, origin string) fetched {
	t.Helper()
	f, err := a.fetch(http.MethodPost, path, "application/x-www-form-urlencoded", form.Encode(), origin)
	return mustFetch(t, f, err)
}

func (a *mergedApp) postJSON(t *testing.T, path, body string) fetched {
	t.Helper()
	f, err := a.fetch(http.MethodPost, path, "application/json", body, "")
	return mustFetch(t, f, err)
}

func decodeJSON(t *testing.T, f fetched, dst any) {
	t.Helper()
	if ct := f.header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content-type = %q, want application/json (%s)", ct, f.body)
	}
	if err := json.Unmarshal([]byte(f.body), dst); err != nil {
		t.Fatalf("json body: %v (%s)", err, f.body)
	}
}
