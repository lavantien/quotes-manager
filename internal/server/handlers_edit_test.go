package server_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/lavantien/quotes-manager/internal/server"
	"github.com/lavantien/quotes-manager/internal/store/storetest"
)

var firstQuoteRe = regexp.MustCompile(`id="quote-(\d+)"`)

func firstQuoteID(body string) int64 {
	m := firstQuoteRe.FindStringSubmatch(body)
	if m == nil {
		return -1
	}
	n, _ := strconv.ParseInt(m[1], 10, 64)
	return n
}

// TestUpdateReSortsAndFlashes edits the shortest quote (seed id 1) to be the
// longest, then asserts the response re-renders the full sorted list with the
// edited block no longer first and a flash marker on it.
func TestUpdateReSortsAndFlashes(t *testing.T) {
	srv := server.New(storetest.CloneFixture(t))

	longBody := strings.Repeat("a", 6000) // longer than the seed max char_count (5032)
	form := url.Values{"content": {longBody}, "attribution": {"the Buddha"}, "text_id": {"MN 1"}}.Encode()
	rec := do(t, srv, "POST", "/quotes/1", form,
		"Content-Type", "application/x-www-form-urlencoded", "HX-Request", "true")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()

	containsBody(t, body, `id="quote-list"`)   // full list re-rendered
	containsBody(t, body, `data-flash-id="1"`) // flash marker on the edited quote
	containsBody(t, body, `id="quote-1"`)      // the edited quote is present
	containsBody(t, body, `id="left-rail"`)    // rail refreshed
	if first := firstQuoteID(body); first == 1 {
		t.Errorf("quote 1 should not be first after becoming the longest; first id = %d", first)
	}
}
