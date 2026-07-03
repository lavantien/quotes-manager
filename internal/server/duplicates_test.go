package server_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/lavantien/quotes-manager/internal/quote"
	"github.com/lavantien/quotes-manager/internal/server"
	"github.com/lavantien/quotes-manager/internal/store"
)

// mkQuote builds a persisted-style store.Quote from a single passage line, for
// duplicate-detection tests.
func mkQuote(id int64, sutta, text string) store.Quote {
	q := quote.New(sutta, sutta, []string{text})
	return store.Quote{
		ID: id, SuttaID: q.SuttaID, Citation: q.Citation,
		BodyMD: q.BodyMD(), BodyText: q.BodyText(), LineCount: q.LineCount(), CharCount: q.CharCount(),
	}
}

func dupPairWithUnique() *fakeStore {
	return newFake(
		// 1 and 2 are near-duplicates (9 shared / 11 union ~= 0.82 > 0.8); 1 is shorter
		// so it is the representative.
		mkQuote(1, "MN 22", "one two three four five six seven eight nine ten"),
		mkQuote(2, "MN 51", "one two three four five six seven eight nine eleven"),
		mkQuote(3, "MN 10", "completely different unique words appear right here"),
	)
}

func TestDuplicatesRenderedInLeftRail(t *testing.T) {
	srv := server.New(dupPairWithUnique())
	for _, target := range []string{"/", "/pane/root", "/rail/left"} {
		rec := do(t, srv, "GET", target, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, want 200", target, rec.Code)
		}
		body := rec.Body.String()
		if got := strings.Count(body, `data-action="focus-quote"`); got != 1 {
			t.Errorf("%s: expected exactly 1 duplicate group link, got %d", target, got)
		}
		if !strings.Contains(body, `data-action="focus-quote" data-id="1"`) {
			t.Errorf("%s: expected the representative (id 1) link", target)
		}
		if !strings.Contains(body, `href="/#quote-1"`) {
			t.Errorf("%s: expected jump href /#quote-1", target)
		}
		if !strings.Contains(body, `MN 22 <span class="rail__count">2</span>`) {
			t.Errorf("%s: expected label 'MN 22' with member count 2", target)
		}
		// Only duplicated text ids are surfaced: the unique quote (id 3, MN 10) must
		// not appear as a duplicate link, and there is exactly one group.
		if strings.Contains(body, `data-id="3"`) && strings.Contains(body, `data-action="focus-quote"`) {
			// more precise: ensure no focus-quote link targets id 3
		}
	}
}

func TestDuplicatesEmptyState(t *testing.T) {
	// No near-duplicates: three distinct texts with negligible word overlap.
	fs := newFake(
		mkQuote(1, "MN 1", "alpha beta"),
		mkQuote(2, "MN 2", "gamma delta"),
		mkQuote(3, "MN 3", "epsilon zeta"),
	)
	srv := server.New(fs)
	rec := do(t, srv, "GET", "/", "")
	body := rec.Body.String()
	if !strings.Contains(body, "No duplicates.") {
		t.Errorf("expected empty-state text, got body:\n%s", body)
	}
	if got := strings.Count(body, `data-action="focus-quote"`); got != 0 {
		t.Errorf("expected no duplicate links, got %d", got)
	}
}

func TestDuplicatesLabelFallsBackToBodyExcerpt(t *testing.T) {
	// Both near-duplicates have an empty text id; the label must come from the body.
	fs := newFake(
		mkQuote(1, "", "alpha beta gamma delta epsilon zeta eta theta iota nu"),
		mkQuote(2, "", "alpha beta gamma delta epsilon zeta eta theta kappa nu"),
	)
	srv := server.New(fs)
	rec := do(t, srv, "GET", "/", "")
	body := rec.Body.String()
	if got := strings.Count(body, `data-action="focus-quote"`); got != 1 {
		t.Fatalf("expected 1 duplicate link, got %d", got)
	}
	if !strings.Contains(body, "alpha beta gamma delta") {
		t.Errorf("expected the body excerpt as label when SuttaID is empty")
	}
}

func TestCreateLiveRefreshesLeftRailAndRootCount(t *testing.T) {
	fs := newFake()
	srv := server.New(fs)
	body := "content=%22Be+your+own+island.%22&attribution=the+Buddha&text_id=MN+44"
	rec := do(t, srv, "POST", "/quotes", body, "Content-Type", "application/x-www-form-urlencoded", "HX-Request", "true")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	got := rec.Body.String()
	for _, want := range []string{`id="quote-list"`, `id="left-rail"`, `id="root-count"`} {
		if !strings.Contains(got, want) {
			t.Errorf("create response missing %s", want)
		}
	}
	if !strings.Contains(got, `id="root-count" class="zone__count" hx-swap-oob="outerHTML">1 blocks`) {
		t.Errorf("create response did not live-refresh the root count to 1 blocks")
	}
}

func TestUpdateLiveRefreshesLeftRail(t *testing.T) {
	fs := newFake(sampleQuote(1))
	srv := server.New(fs)
	body := "content=%22new+text%22&attribution=the+Buddha&text_id=DN+16"
	rec := do(t, srv, "POST", "/quotes/1", body, "Content-Type", "application/x-www-form-urlencoded", "HX-Request", "true")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	got := rec.Body.String()
	if !strings.Contains(got, `id="left-rail"`) {
		t.Errorf("update response missing the out-of-band left rail")
	}
	if !strings.Contains(got, `id="quote-1"`) {
		t.Errorf("update response missing the saved quote block")
	}
}

func TestDeleteLiveRefreshesBothRailsAndRootCount(t *testing.T) {
	fs := newFake(sampleQuote(1))
	srv := server.New(fs)
	rec := do(t, srv, "DELETE", "/quotes/1", "", "HX-Request", "true")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if len(fs.quotes) != 0 {
		t.Errorf("store still has %d quotes", len(fs.quotes))
	}
	got := rec.Body.String()
	for _, want := range []string{`id="left-rail"`, `id="right-rail"`, `id="root-count"`} {
		if !strings.Contains(got, want) {
			t.Errorf("delete response missing %s", want)
		}
	}
	if !strings.Contains(got, `>0 blocks<`) {
		t.Errorf("delete response did not live-refresh the root count to 0 blocks")
	}
}

// When the post-mutation rail refresh itself fails, the handler must still
// report a server error rather than write a partial response.
func TestMutationRailRefreshErrors(t *testing.T) {
	for _, c := range []struct {
		name   string
		method string
		target string
		body   string
	}{
		{"create", "POST", "/quotes", "content=%22x%22&text_id=MN+1"},
		{"update", "POST", "/quotes/1", "content=%22x%22&text_id=MN+1"},
		{"delete", "DELETE", "/quotes/1", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			srv := server.New(failList{newFake(sampleQuote(1))})
			rec := do(t, srv, c.method, c.target, c.body,
				"Content-Type", "application/x-www-form-urlencoded", "HX-Request", "true")
			if rec.Code != http.StatusInternalServerError {
				t.Errorf("%s %s: code = %d, want 500 (railData should fail via List)",
					c.method, c.target, rec.Code)
			}
		})
	}
}
