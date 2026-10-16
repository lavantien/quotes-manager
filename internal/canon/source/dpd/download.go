package dpd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type FetchOpts struct {
	URL          string
	PinnedSHA256 string
	MaxBytes     int64
	Timeout      time.Duration
}

type Provenance struct {
	Tag     string
	SHA256  string
	Size    int64
	Fetched time.Time
}

var tagRe = regexp.MustCompile(`/(?:tag|download)/([^/]+)/`)

func Fetch(ctx context.Context, opts FetchOpts, destPath string) (Provenance, error) {
	if opts.MaxBytes <= 0 || opts.MaxBytes == math.MaxInt64 {
		return Provenance{}, fmt.Errorf("dpd fetch: MaxBytes must be a positive int64 below MaxInt64, got %d", opts.MaxBytes)
	}
	u, err := url.Parse(opts.URL)
	if err != nil {
		return Provenance{}, fmt.Errorf("dpd fetch: bad URL %q: %w", opts.URL, err)
	}
	if destPath == "" {
		return Provenance{}, fmt.Errorf("dpd fetch: empty destPath")
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return Provenance{}, fmt.Errorf("dpd fetch: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(destPath), ".dpd-fetch-*")
	if err != nil {
		return Provenance{}, fmt.Errorf("dpd fetch: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		if tmpName != "" {
			_ = os.Remove(tmpName)
		}
	}()

	var size int64
	var sum, tag string
	switch u.Scheme {
	case "http", "https":
		tag, size, sum, err = fetchHTTP(ctx, opts, tmp)
	case "file":
		tag, size, sum, err = copyFileURL(u, tmp, opts.MaxBytes)
	default:
		err = fmt.Errorf("dpd fetch: unsupported URL scheme %q", u.Scheme)
	}
	if err != nil {
		return Provenance{}, err
	}

	pv := Provenance{Tag: tag, Size: size, SHA256: sum, Fetched: time.Now().UTC()}
	if opts.PinnedSHA256 != "" && !strings.EqualFold(sum, opts.PinnedSHA256) {
		return Provenance{}, fmt.Errorf("dpd fetch: sha256 mismatch: pinned %s, got %s", opts.PinnedSHA256, sum)
	}
	if err := tmp.Sync(); err != nil {
		return Provenance{}, fmt.Errorf("dpd fetch: sync %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return Provenance{}, fmt.Errorf("dpd fetch: close %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, destPath); err != nil {
		return Provenance{}, fmt.Errorf("dpd fetch: rename to %s: %w", destPath, err)
	}
	tmpName = ""
	return pv, nil
}

func fetchHTTP(ctx context.Context, opts FetchOpts, tmp *os.File) (tag string, size int64, sum string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, opts.URL, nil)
	if err != nil {
		return "", 0, "", fmt.Errorf("dpd fetch: %w", err)
	}
	client := &http.Client{
		Timeout: opts.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if m := tagRe.FindStringSubmatch(req.URL.String()); m != nil {
				tag = m[1]
			}
			return nil
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, "", fmt.Errorf("dpd fetch %s: %w", opts.URL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", 0, "", fmt.Errorf("dpd fetch %s: unexpected status %s", opts.URL, resp.Status)
	}
	finalURL := resp.Request.URL.String()
	if tag == "" {
		if m := tagRe.FindStringSubmatch(finalURL); m != nil {
			tag = m[1]
		}
	}
	size, sum, err = copyLimited(resp.Body, tmp, opts.MaxBytes)
	if err != nil {
		return "", 0, "", fmt.Errorf("dpd fetch %s: %w", opts.URL, err)
	}
	return tag, size, sum, nil
}

func copyFileURL(u *url.URL, tmp *os.File, max int64) (tag string, size int64, sum string, err error) {
	if u.Opaque != "" {
		return "", 0, "", fmt.Errorf("dpd fetch: file URL must use the file:///path form, got %q", u.String())
	}
	if u.Host != "" && u.Host != "localhost" {
		return "", 0, "", fmt.Errorf("dpd fetch: file URL host %q unsupported", u.Host)
	}
	p := trimDriveSlash(u.Path)
	f, err := os.Open(filepath.FromSlash(p))
	if err != nil {
		return "", 0, "", fmt.Errorf("dpd fetch: %w", err)
	}
	defer func() { _ = f.Close() }()
	size, sum, err = copyLimited(f, tmp, max)
	if err != nil {
		return "", 0, "", fmt.Errorf("dpd fetch %s: %w", p, err)
	}
	return "local", size, sum, nil
}

func trimDriveSlash(p string) string {
	if len(p) >= 3 && p[0] == '/' && isDriveLetter(p[1]) && p[2] == ':' {
		return p[1:]
	}
	return p
}

func isDriveLetter(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
}

func copyLimited(src io.Reader, dst io.Writer, max int64) (int64, string, error) {
	h := sha256.New()
	n, err := io.Copy(dst, io.TeeReader(io.LimitReader(src, max+1), h))
	if err != nil {
		return 0, "", err
	}
	if n > max {
		return 0, "", fmt.Errorf("payload exceeds %d bytes (read %d and stopped)", max, n)
	}
	return n, hex.EncodeToString(h.Sum(nil)), nil
}
