package server_test

import (
	"net/http"
	"regexp"
	"testing"

	"github.com/lavantien/quotes-manager/internal/server"
	"github.com/lavantien/quotes-manager/internal/store/storetest"
)

var focusQuoteRe = regexp.MustCompile(`data-action="focus-quote" data-id="(\d+)"`)

func focusQuoteID(body string) string {
	m := focusQuoteRe.FindStringSubmatch(body)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// TestMergeDuplicatesHTTP drives the merge endpoint against the clone-of-main
// fixture: the seed's MN 22 trio surfaces as one duplicate group, the merge
// endpoint folds its members into the representative, and the response refreshes
// the root column and both rails.
func TestMergeDuplicatesHTTP(t *testing.T) {
	srv := server.New(storetest.CloneFixture(t))

	home := do(t, srv, "GET", "/", "").Body.String()
	rep := focusQuoteID(home)
	if rep == "" {
		t.Fatal("expected a duplicate group on the fixture")
	}

	rec := do(t, srv, "POST", "/duplicates/"+rep+"/merge", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()

	// The root column and both rails are refreshed.
	containsBody(t, body, `id="root-zone"`)
	containsBody(t, body, `id="left-rail"`)
	containsBody(t, body, `id="right-rail"`)
	// The representative survives; the two merged-away members are gone, so the
	// corpus is now 109 - 2 = 107 blocks.
	containsBody(t, body, "id=\"quote-"+rep+"\"")
	containsBody(t, body, ">107 blocks<")
	// The trio no longer reads as a 3-member group in the refreshed rail.
	lacksBody(t, body, `MN 22 <span class="rail__count">3</span>`)

	// A fresh GET / no longer lists the trio as a duplicate group.
	after := do(t, srv, "GET", "/", "").Body.String()
	lacksBody(t, after, `MN 22 <span class="rail__count">3</span>`)
}

// TestMergeDuplicatesUnknownRep returns 404 when the id is not a current
// duplicate-group representative.
func TestMergeDuplicatesUnknownRep(t *testing.T) {
	srv := server.New(storetest.CloneFixture(t))
	rec := do(t, srv, "POST", "/duplicates/9999/merge", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}
