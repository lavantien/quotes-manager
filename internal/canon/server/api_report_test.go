package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func reportTestServer(t testing.TB, root string, tree map[string]map[string]string) *Server {
	t.Helper()
	cfg := newFixtureConfig(t)
	cfg.Canon.Paths.ReportsDir = root
	for date, files := range tree {
		dir := filepath.Join(root, date)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for name, content := range files {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	return srv
}

func TestAPIReportList(t *testing.T) {
	root := filepath.Join(t.TempDir(), "reports")
	srv := reportTestServer(t, root, map[string]map[string]string{
		"2026-10-05": {"report.md": "# a\n", "report.json": "{}", "a.txt": "x"},
		"2026-10-07": {"report.md": "# b\n"},
		"2026-09-30": {},
	})
	for _, junk := range []string{"not-a-date", "2026-1-2", "2026-02-30", "2026-01-02x"} {
		if err := os.MkdirAll(filepath.Join(root, junk), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "README"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "2026-10-05", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}

	var got struct {
		Reports []struct {
			Date  string   `json:"date"`
			Files []string `json:"files"`
		} `json:"reports"`
	}
	getJSON(t, srv.Routes(), "/api/v1/report", &got)
	want := []struct {
		date  string
		files []string
	}{
		{"2026-10-07", []string{"report.md"}},
		{"2026-10-05", []string{"a.txt", "report.json", "report.md"}},
		{"2026-09-30", []string{}},
	}
	if len(got.Reports) != len(want) {
		t.Fatalf("listing = %+v, want %v", got.Reports, want)
	}
	for i, w := range want {
		g := got.Reports[i]
		if g.Date != w.date {
			t.Fatalf("listing[%d] = %s, want %s (newest first)", i, g.Date, w.date)
		}
		if g.Files == nil {
			t.Fatalf("listing[%d] files = null, want []", i)
		}
		if len(g.Files) != len(w.files) {
			t.Fatalf("listing[%s] files = %v, want %v", g.Date, g.Files, w.files)
		}
		for j := range w.files {
			if g.Files[j] != w.files[j] {
				t.Fatalf("listing[%s] files = %v, want %v", g.Date, g.Files, w.files)
			}
		}
	}
}

func TestAPIReportListMissingDir(t *testing.T) {
	srv := reportTestServer(t, filepath.Join(t.TempDir(), "absent"), nil)
	var got struct {
		Reports []json.RawMessage `json:"reports"`
	}
	rec := getJSON(t, srv.Routes(), "/api/v1/report", &got)
	if got.Reports == nil || len(got.Reports) != 0 {
		t.Fatalf("missing reports dir must list an empty array, got %v", got.Reports)
	}
	if rec.Body.String() != "{\"reports\":[]}\n" {
		t.Fatalf("body = %q, want an empty json array", rec.Body.String())
	}
}

func TestAPIReportDownload(t *testing.T) {
	root := filepath.Join(t.TempDir(), "reports")
	srv := reportTestServer(t, root, map[string]map[string]string{
		"2026-10-05": {"report.md": "# hello\n", "report.json": `{"a":1}`, "data.bin": "\x00\x01"},
	})
	h := srv.Routes()
	for _, tc := range []struct {
		file, ctype, body string
	}{
		{"report.md", "text/markdown; charset=utf-8", "# hello\n"},
		{"report.json", "application/json", `{"a":1}`},
		{"data.bin", "application/octet-stream", "\x00\x01"},
	} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/report/2026-10-05/"+tc.file, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s = %d: %s", tc.file, rec.Code, rec.Body.String())
		}
		if ct := rec.Header().Get("Content-Type"); ct != tc.ctype {
			t.Fatalf("%s content type = %q, want %q", tc.file, ct, tc.ctype)
		}
		if rec.Body.String() != tc.body {
			t.Fatalf("%s body = %q, want %q", tc.file, rec.Body.String(), tc.body)
		}
	}
}

func TestAPIReportDownloadNotFound(t *testing.T) {
	root := filepath.Join(t.TempDir(), "reports")
	srv := reportTestServer(t, root, map[string]map[string]string{
		"2026-10-05": {"report.md": "# a\n"},
	})
	h := srv.Routes()
	for _, target := range []string{
		"/api/v1/report/2026-10-05/missing.md",
		"/api/v1/report/2020-01-01/report.md",
	} {
		status, apiErr := getError(t, h, target)
		if status != http.StatusNotFound || apiErr.Code != codeReportNotFound {
			t.Fatalf("GET %s = %d %s, want 404 %s", target, status, apiErr.Code, codeReportNotFound)
		}
	}
}

func TestAPIReportEncodedTraversal(t *testing.T) {
	root := filepath.Join(t.TempDir(), "reports")
	srv := reportTestServer(t, root, map[string]map[string]string{
		"2026-10-05": {"report.md": "# x\n"},
	})
	h := srv.Routes()
	for _, p := range []string{
		"/api/v1/report/2026-10-05/..%2F..%2Fcanary",
		"/api/v1/report/2026-10-05/report.md%00",
		"/api/v1/report/2026-10-05/%2e%2e%2fcanary",
	} {
		rec := getPage(t, h, p)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("GET %s = %d (%q), want 400", p, rec.Code, rec.Body.String())
		}
	}
}

func TestAPIReportReservedNames(t *testing.T) {
	root := filepath.Join(t.TempDir(), "reports")
	srv := reportTestServer(t, root, map[string]map[string]string{
		"2026-10-05": {"report.md": "# a\n"},
	})
	h := srv.Routes()
	for _, name := range []string{"NUL", "nul.md", "CON", "con.txt", "PRN", "AUX", "COM1", "com9.log", "LPT1"} {
		target := "/api/v1/report/2026-10-05/" + name
		status, apiErr := getError(t, h, target)
		if status != http.StatusBadRequest {
			t.Fatalf("GET %s = %d: %s, want 400", target, status, apiErr.Code)
		}
	}
}

func TestAPIReportTraversal(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "reports")
	srv := reportTestServer(t, root, map[string]map[string]string{
		"2026-10-05": {"report.md": "# a\n"},
	})
	const canary = "canary secret payload"
	if err := os.WriteFile(filepath.Join(tmp, "canary.txt"), []byte(canary), 0o644); err != nil {
		t.Fatal(err)
	}
	h := srv.Routes()
	for _, tc := range []struct {
		name     string
		target   string
		statuses []int
	}{
		{"encoded dot-dot slash", "/api/v1/report/2026-10-05/..%2F..%2Fcanary.txt", []int{http.StatusBadRequest}},
		{"encoded dots", "/api/v1/report/2026-10-05/%2E%2E%2Fcanary.txt", []int{http.StatusBadRequest}},
		{"encoded backslash", "/api/v1/report/2026-10-05/canary.txt%5C..%5C..%5Ccanary.txt", []int{http.StatusBadRequest}},
		{"windows drive", "/api/v1/report/2026-10-05/C:%5Ccanary.txt", []int{http.StatusBadRequest}},
		{"extra segment", "/api/v1/report/2026-10-05/a/b", []int{http.StatusNotFound}},
		{"leading dot file", "/api/v1/report/2026-10-05/.canary.txt", []int{http.StatusBadRequest}},
		{"dot-dot file", "/api/v1/report/2026-10-05/..", []int{http.StatusTemporaryRedirect, http.StatusMovedPermanently, http.StatusBadRequest, http.StatusNotFound}},
		{"date traversal", "/api/v1/report/..%2F..%2Fcanary.txt", []int{http.StatusNotFound}},
		{"short month", "/api/v1/report/2026-1-2/report.md", []int{http.StatusBadRequest}},
		{"day out of range", "/api/v1/report/2026-10-32/report.md", []int{http.StatusBadRequest}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.target, nil))
			allowed := false
			for _, s := range tc.statuses {
				if rec.Code == s {
					allowed = true
				}
			}
			if !allowed {
				t.Fatalf("GET %s = %d: %s, want one of %v", tc.target, rec.Code, rec.Body.String(), tc.statuses)
			}
			if rec.Body.String() == canary {
				t.Fatalf("GET %s served the canary", tc.target)
			}
		})
	}
}
