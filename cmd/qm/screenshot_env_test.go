package main

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/lavantien/quotes-manager/internal/cli"
	"github.com/lavantien/quotes-manager/internal/config"
)

func TestScreenshotEnvServesEveryJourneyShot(t *testing.T) {
	baseURL, closeEnv, err := buildScreenshotEnv(config.Defaults())
	if err != nil {
		t.Fatalf("buildScreenshotEnv: %v", err)
	}
	t.Cleanup(func() {
		if err := closeEnv(); err != nil {
			t.Errorf("closeEnv: %v", err)
		}
	})
	client := &http.Client{Timeout: 10 * time.Second}
	for _, shot := range cli.Journey() {
		resp, err := client.Get(baseURL + shot.Path)
		if err != nil {
			t.Fatalf("%s: %v", shot.Path, err)
		}
		raw, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			t.Fatalf("%s: read body: %v", shot.Path, err)
		}
		body := string(raw)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s status = %d, body:\n%s", shot.Path, resp.StatusCode, body)
		}
		token := waitToken(shot.Wait)
		if token != "" &&
			!strings.Contains(body, `id="`+token+`"`) &&
			!strings.Contains(body, token+`"`) &&
			!strings.Contains(body, `"`+token) {
			t.Errorf("%s wait selector %q not served: no %q id or class in the page", shot.Path, shot.Wait, token)
		}
	}
	for _, tc := range []struct{ path, marker string }{
		{"/texts/mn1", "So I have heard."},
		{"/words?form=dhamma", "the teaching"},
		{"/api/v1/graph/word?token=dhamma", "dpd-root:dh"},
	} {
		resp, err := client.Get(baseURL + tc.path)
		if err != nil {
			t.Fatalf("%s: %v", tc.path, err)
		}
		raw, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			t.Fatalf("%s: read body: %v", tc.path, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s status = %d", tc.path, resp.StatusCode)
		}
		if !strings.Contains(string(raw), tc.marker) {
			t.Errorf("%s missing marker %q", tc.path, tc.marker)
		}
	}
}

func waitToken(wait string) string {
	first, _, _ := strings.Cut(wait, " ")
	first = strings.TrimPrefix(first, "#")
	if _, class, found := strings.Cut(first, "."); found {
		return class
	}
	return first
}
