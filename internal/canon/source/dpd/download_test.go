package dpd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func hexSum(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func destFile(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "dpd", "dpd.db.tar.xz")
}

func destDirFiles(t *testing.T, destPath string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Dir(destPath))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func readFile(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func redirectChain(t *testing.T, payload []byte, tag string) *httptest.Server {
	t.Helper()
	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/download/"+tag+"/dpd.db.tar.xz" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(payload)
	}))
	t.Cleanup(final.Close)
	chain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, final.URL+"/download/"+tag+"/dpd.db.tar.xz", http.StatusFound)
	}))
	t.Cleanup(chain.Close)
	return chain
}

func TestFetchTagFromMiddleHop(t *testing.T) {
	payload := []byte("xz-bytes")
	tag := "v0.4.20260926"
	signed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/signed-assets/169565896" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(payload)
	}))
	t.Cleanup(signed.Close)
	middle := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/releases/download/"+tag+"/dpd.db.tar.xz" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, signed.URL+"/signed-assets/169565896", http.StatusFound)
	}))
	t.Cleanup(middle.Close)
	latest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, middle.URL+"/releases/download/"+tag+"/dpd.db.tar.xz", http.StatusFound)
	}))
	t.Cleanup(latest.Close)

	pv, err := Fetch(context.Background(), FetchOpts{
		URL:      latest.URL + "/releases/latest/download/dpd.db.tar.xz",
		MaxBytes: 1 << 20,
		Timeout:  10 * time.Second,
	}, destFile(t))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if pv.Tag != tag {
		t.Fatalf("Provenance.Tag = %q, want %q captured from the middle hop", pv.Tag, tag)
	}
}

func TestFetchDirectURLCarriesTag(t *testing.T) {
	tag := "v0.9.20260101"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/releases/download/"+tag+"/dpd.db.tar.xz" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("direct"))
	}))
	t.Cleanup(srv.Close)

	pv, err := Fetch(context.Background(), FetchOpts{
		URL:      srv.URL + "/releases/download/" + tag + "/dpd.db.tar.xz",
		MaxBytes: 1 << 20,
		Timeout:  10 * time.Second,
	}, destFile(t))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if pv.Tag != tag {
		t.Fatalf("Provenance.Tag = %q, want %q from the direct final URL", pv.Tag, tag)
	}
}

func TestFetchRedirectCapturesTagAndHash(t *testing.T) {
	payload := []byte("xz-magic-bytes-here")
	srv := redirectChain(t, payload, "v0.4.20260926")
	dest := destFile(t)

	pv, err := Fetch(context.Background(), FetchOpts{
		URL:      srv.URL + "/releases/latest/download/dpd.db.tar.xz",
		MaxBytes: 1 << 20,
		Timeout:  10 * time.Second,
	}, dest)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if pv.Tag != "v0.4.20260926" {
		t.Errorf("Provenance.Tag = %q, want v0.4.20260926", pv.Tag)
	}
	if pv.SHA256 != hexSum(payload) {
		t.Errorf("Provenance.SHA256 = %q, want %q", pv.SHA256, hexSum(payload))
	}
	if pv.Size != int64(len(payload)) {
		t.Errorf("Provenance.Size = %d, want %d", pv.Size, len(payload))
	}
	if pv.Fetched.IsZero() {
		t.Error("Provenance.Fetched is zero")
	}
	if got := readFile(t, dest); string(got) != string(payload) {
		t.Errorf("downloaded bytes = %q, want %q", got, payload)
	}
}

func TestFetchDirectURLWithoutTag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "plain")
	}))
	t.Cleanup(srv.Close)
	dest := destFile(t)

	pv, err := Fetch(context.Background(), FetchOpts{
		URL: srv.URL + "/asset", MaxBytes: 1 << 20, Timeout: 10 * time.Second,
	}, dest)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if pv.Tag != "" {
		t.Errorf("Provenance.Tag = %q, want empty for a tag-less final URL", pv.Tag)
	}
}

func TestFetchNon2xxFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "gone", http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	dest := destFile(t)

	_, err := Fetch(context.Background(), FetchOpts{
		URL: srv.URL, MaxBytes: 1 << 20, Timeout: 10 * time.Second,
	}, dest)
	if err == nil {
		t.Fatal("Fetch on 404: expected error")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error should carry the status, got %v", err)
	}
	if files := destDirFiles(t, dest); len(files) != 0 {
		t.Errorf("failed fetch left files behind: %v", files)
	}
}

func TestFetchOversizeFails(t *testing.T) {
	payload := []byte("0123456789")
	srv := redirectChain(t, payload, "v0.1.0")
	dest := destFile(t)

	_, err := Fetch(context.Background(), FetchOpts{
		URL: srv.URL + "/x", MaxBytes: int64(len(payload) - 1), Timeout: 10 * time.Second,
	}, dest)
	if err == nil {
		t.Fatal("Fetch over MaxBytes: expected hard error")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("error should name the oversize cause, got %v", err)
	}
	if files := destDirFiles(t, dest); len(files) != 0 {
		t.Errorf("oversize fetch left files behind: %v", files)
	}
}

func TestFetchPinnedSHA(t *testing.T) {
	payload := []byte("pinned-body")
	sum := hexSum(payload)
	srv := redirectChain(t, payload, "v0.2.0")

	cases := []struct {
		name  string
		pin   string
		ok    bool
		srvFn func(t *testing.T) *httptest.Server
	}{
		{"match", sum, true, nil},
		{"mismatch", strings.Repeat("0", 64), false, nil},
		{"case-insensitive match", strings.ToUpper(sum), true, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := srv
			if tc.srvFn != nil {
				server = tc.srvFn(t)
			}
			dest := destFile(t)
			pv, err := Fetch(context.Background(), FetchOpts{
				URL: server.URL + "/x", PinnedSHA256: tc.pin,
				MaxBytes: 1 << 20, Timeout: 10 * time.Second,
			}, dest)
			if tc.ok {
				if err != nil {
					t.Fatalf("Fetch: %v", err)
				}
				if !strings.EqualFold(pv.SHA256, tc.pin) {
					t.Errorf("SHA256 %q does not match pin %q", pv.SHA256, tc.pin)
				}
				return
			}
			if err == nil {
				t.Fatal("Fetch with wrong pin: expected error")
			}
			if !strings.Contains(err.Error(), "sha256 mismatch") {
				t.Errorf("error should name the mismatch, got %v", err)
			}
			if files := destDirFiles(t, dest); len(files) != 0 {
				t.Errorf("mismatched fetch left files behind: %v", files)
			}
		})
	}
}

func TestFetchFileURL(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.tar.xz")
	payload := []byte("local asset bytes")
	if err := os.WriteFile(src, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	dest := destFile(t)

	fileURL := "file:///" + filepath.ToSlash(src)
	pv, err := Fetch(context.Background(), FetchOpts{URL: fileURL, MaxBytes: 1 << 20}, dest)
	if err != nil {
		t.Fatalf("Fetch file://: %v", err)
	}
	if pv.Tag != "local" {
		t.Errorf("Provenance.Tag = %q, want local", pv.Tag)
	}
	if pv.SHA256 != hexSum(payload) || pv.Size != int64(len(payload)) {
		t.Errorf("provenance = %+v, want sha %s size %d", pv, hexSum(payload), len(payload))
	}
	if got := readFile(t, dest); string(got) != string(payload) {
		t.Errorf("copied bytes = %q, want %q", got, payload)
	}
}

func TestFetchFileURLOversizeAndPin(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.tar.xz")
	payload := []byte("tiny")
	if err := os.WriteFile(src, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	urlStr := "file:///" + filepath.ToSlash(src)
	badPin := FetchOpts{URL: urlStr, PinnedSHA256: strings.Repeat("f", 64), MaxBytes: 1 << 20}
	if _, err := Fetch(context.Background(), badPin, destFile(t)); err == nil {
		t.Error("file:// with wrong pin: expected error")
	}
	oversize := FetchOpts{URL: urlStr, MaxBytes: 2}
	if _, err := Fetch(context.Background(), oversize, destFile(t)); err == nil {
		t.Error("file:// over MaxBytes: expected error")
	}
}

func TestFetchBadInputs(t *testing.T) {
	dest := destFile(t)
	cases := []struct {
		name string
		opts FetchOpts
		want string
	}{
		{"zero max bytes", FetchOpts{URL: "http://x/y", MaxBytes: 0}, "MaxBytes"},
		{"negative max bytes", FetchOpts{URL: "http://x/y", MaxBytes: -5}, "MaxBytes"},
		{"unsupported scheme", FetchOpts{URL: "ftp://example.com/dpd.db.tar.xz", MaxBytes: 1 << 20}, "scheme"},
		{"unparsable url", FetchOpts{URL: "://\x7f", MaxBytes: 1 << 20}, "URL"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Fetch(context.Background(), tc.opts, dest)
			if err == nil {
				t.Fatalf("Fetch(%s): expected error", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestFetchTagRegexp(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"https://github.com/o/r/releases/download/v0.4.20260926/dpd.db.tar.xz", "v0.4.20260926"},
		{"https://github.com/o/r/releases/tag/v0.4.20260926/", "v0.4.20260926"},
		{"https://release-assets.example.com/download/v1.2.3/dpd.db.tar.xz?token=x", "v1.2.3"},
		{"https://example.com/assets/dpd.db.tar.xz?sig=1", ""},
		{"https://example.com/download//dpd.db.tar.xz", ""},
		{"https://example.com/no/tag/here/dpd.db.tar.xz", "here"},
	}
	for _, tc := range cases {
		got := ""
		if m := tagRe.FindStringSubmatch(tc.url); m != nil {
			got = m[1]
		}
		if got != tc.want {
			t.Errorf("tagRe(%q) = %q, want %q", tc.url, got, tc.want)
		}
	}
}

func TestFetchFileURLForms(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "odd name.tar.xz")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	slash := filepath.ToSlash(src)
	forms := []string{
		"file:///" + slash,
		"file://localhost/" + slash,
		"file:///" + strings.ReplaceAll(slash, " ", "%20"),
	}
	if _, err := Fetch(context.Background(), FetchOpts{URL: "file:" + slash, MaxBytes: 1 << 20}, destFile(t)); err == nil {
		t.Error("opaque file: form must be rejected, use file:///path")
	}
	for _, u := range forms {
		if _, err := Fetch(context.Background(), FetchOpts{URL: u, MaxBytes: 1 << 20}, destFile(t)); err != nil {
			t.Errorf("Fetch(%q): %v", u, err)
		}
	}
	bad := "file://otherhost/" + filepath.ToSlash(src)
	if _, err := Fetch(context.Background(), FetchOpts{URL: bad, MaxBytes: 1 << 20}, destFile(t)); err == nil {
		t.Errorf("Fetch(%q): expected host rejection", bad)
	}
}

func TestTrimDriveSlash(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/C:/Users/x/dpd.db", "C:/Users/x/dpd.db"},
		{"/c:/users/x/dpd.db", "c:/users/x/dpd.db"},
		{"/Z:", "Z:"},
		{"C:/Users/x", "C:/Users/x"},
		{"/1:/x", "/1:/x"},
		{"/Cxy", "/Cxy"},
		{"/ab", "/ab"},
		{"/:/x", "/:/x"},
		{"/", "/"},
		{"//", "//"},
		{"///", "///"},
		{"", ""},
		{"/C:", "C:"},
		{"/-:/x", "/-:/x"},
		{"relative", "relative"},
	}
	for _, tc := range cases {
		if got := trimDriveSlash(tc.in); got != tc.want {
			t.Errorf("trimDriveSlash(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestIsDriveLetter(t *testing.T) {
	cases := []struct {
		c    byte
		want bool
	}{
		{'A', true},
		{'Z', true},
		{'M', true},
		{'a', true},
		{'z', true},
		{'m', true},
		{'@', false},
		{'[', false},
		{'`', false},
		{'{', false},
		{'0', false},
		{':', false},
		{'/', false},
		{'-', false},
	}
	for _, tc := range cases {
		if got := isDriveLetter(tc.c); got != tc.want {
			t.Errorf("isDriveLetter(%q) = %v, want %v", tc.c, got, tc.want)
		}
	}
}

func TestFetchTimeoutApplies(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		_, _ = fmt.Fprint(w, "late")
	}))
	t.Cleanup(srv.Close)
	start := time.Now()
	_, err := Fetch(context.Background(), FetchOpts{
		URL: srv.URL, MaxBytes: 1 << 20, Timeout: 100 * time.Millisecond,
	}, destFile(t))
	if err == nil {
		t.Fatal("Fetch with a 100ms timeout against a slow server: expected error")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("timeout did not apply, took %v", elapsed)
	}
}
