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
