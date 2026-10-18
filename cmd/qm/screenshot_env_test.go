package main

import (
	"io"
	"net/http"
	"net/url"
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
		if len(shot.Steps) > 0 {
			continue
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
		{"/quotes/new", `class="quote-form"`},
		{"/quotes/import/form", `id="import-form"`},
		{"/quotes/1/edit", `class="quote-form"`},
		{"/analytics?aggregate=top-headwords", "download csv"},
		{"/analytics?aggregate=top-headwords", "sql console"},
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
	checkResp, err := client.PostForm(baseURL+"/check", url.Values{
		"ids": {"MN 22\nMN 51\nXX 9.9"},
	})
	if err != nil {
		t.Fatalf("/check: %v", err)
	}
	checkRaw, err := io.ReadAll(checkResp.Body)
	_ = checkResp.Body.Close()
	if err != nil {
		t.Fatalf("/check read: %v", err)
	}
	for _, marker := range []string{"2 of 3 found.", "is-found", "is-missing"} {
		if !strings.Contains(string(checkRaw), marker) {
			t.Errorf("/check missing marker %q:\n%s", marker, string(checkRaw))
		}
	}
	dresp, err := client.Get(baseURL + "/discrepancies")
	if err != nil {
		t.Fatalf("/discrepancies: %v", err)
	}
	draw, err := io.ReadAll(dresp.Body)
	_ = dresp.Body.Close()
	if err != nil {
		t.Fatalf("/discrepancies read: %v", err)
	}
	if !strings.Contains(string(draw), `<tr class="sev-`) {
		t.Errorf("/discrepancies renders no finding rows:\n%s", string(draw))
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
