package server_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/lavantien/quotes-manager/internal/quote"
	"github.com/lavantien/quotes-manager/internal/store"
)

// quoteRow builds a store.Quote with an arbitrary body and citation, for search
// tests that need distinguishable text (sampleQuote bodies are all identical).
func quoteRow(id int64, sutta, citation, body string) store.Quote {
	q := quote.New(sutta, citation, []string{body})
	return store.Quote{
		ID: id, SuttaID: q.SuttaID, Citation: q.Citation,
		BodyMD: q.BodyMD(), BodyText: q.BodyText(), LineCount: q.LineCount(), CharCount: q.CharCount(),
	}
}

func containsBody(t *testing.T, body, sub string) {
	t.Helper()
	if !strings.Contains(body, sub) {
		t.Errorf("missing %q in response", sub)
	}
}

func lacksBody(t *testing.T, body, sub string) {
	t.Helper()
	if strings.Contains(body, sub) {
		t.Errorf("unexpected %q in response", sub)
	}
}

// --- deep-link (GET /) filtering ---

func TestSearchRootFiltersHome(t *testing.T) {
	srv := newServer(t, newFake(
		quoteRow(1, "MN 22", "the Buddha, MN 22", "the buddha sat calmly"),
		quoteRow(2, "AN 5", "a monk, AN 5", "a quiet forest monastery"),
	))
	body := do(t, srv, "GET", "/?rq=buddha", "").Body.String()
	containsBody(t, body, `id="quote-1"`)
	lacksBody(t, body, `id="quote-2"`)
	containsBody(t, body, "<mark>buddha</mark>")
}

func TestSearchRootEmptyShowsAll(t *testing.T) {
	srv := newServer(t, newFake(
		quoteRow(1, "MN 22", "the Buddha, MN 22", "calm"),
		quoteRow(2, "AN 5", "a monk, AN 5", "forest"),
	))
	body := do(t, srv, "GET", "/?rq=", "").Body.String()
	containsBody(t, body, `id="quote-1"`)
	containsBody(t, body, `id="quote-2"`)
	lacksBody(t, body, "<mark>")
}

func TestSearchRootORAndCaseInsensitive(t *testing.T) {
	srv := newServer(t, newFake(
		quoteRow(1, "MN 1", "MN 1", "alpha"),
		quoteRow(2, "MN 2", "MN 2", "beta"),
		quoteRow(3, "MN 3", "MN 3", "Alpha beta gamma"),
	))
	// Terms "alpha" and "gamma" (OR), uppercased to prove case-insensitivity.
	body := do(t, srv, "GET", "/?rq=ALPHA%20gamma", "").Body.String()
	containsBody(t, body, `id="quote-1"`) // matches "alpha"
	containsBody(t, body, `id="quote-3"`) // matches both
	lacksBody(t, body, `id="quote-2"`)    // matches neither
}

func TestSearchRootCategoryScoped(t *testing.T) {
	fs, cid := fakeWithCategory(t) // sample quotes 1 and 2 are tagged
	srv := newServer(t, fs)
	// "human" matches every sample body, but quote 3 is outside the active
	// category so it must be absent even though its body matches.
	body := do(t, srv, "GET", fmt.Sprintf("/?cat=%d&rq=human", cid), "").Body.String()
	containsBody(t, body, `id="quote-1"`)
	containsBody(t, body, `id="quote-2"`)
	lacksBody(t, body, `id="quote-3"`)
}

func TestSearchRootNoMatchEmptyState(t *testing.T) {
	srv := newServer(t, newFake(quoteRow(1, "MN 22", "the Buddha, MN 22", "calm")))
	body := do(t, srv, "GET", "/?rq=zzznotpresent", "").Body.String()
	containsBody(t, body, "No matches.")
}

func TestSearchRootInputPrefilled(t *testing.T) {
	srv := newServer(t, newFake(quoteRow(1, "MN 22", "the Buddha, MN 22", "the buddha")))
	body := do(t, srv, "GET", "/?rq=buddha", "").Body.String()
	containsBody(t, body, `name="rq"`)
	containsBody(t, body, `value="buddha"`)
}

func TestSearchRootXSS(t *testing.T) {
	srv := newServer(t, newFake(
		quoteRow(1, "MN 22", "the Buddha, MN 22", "<script>alert(1)</script> calm"),
	))
	body := do(t, srv, "GET", "/?rq=%3Cscript%3E", "").Body.String()
	lacksBody(t, body, "<script>alert")             // never a raw script tag
	containsBody(t, body, "&lt;script&gt;")         // escaped somewhere (highlight and/or value)
	containsBody(t, body, `value="&lt;script&gt;"`) // input value is escaped too
}

func TestSearchCollectionDeepLink(t *testing.T) {
	fs := newFake(
		quoteRow(1, "MN 22", "the Buddha, MN 22", "the buddha spoke"),
		quoteRow(2, "AN 5", "a monk, AN 5", "a forest monk"),
	)
	cid, err := fs.CreateCollection([]int64{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	srv := newServer(t, fs)
	body := do(t, srv, "GET", fmt.Sprintf("/?col=%d&cq=buddha", cid), "").Body.String()
	containsBody(t, body, `id="col-quote-1"`)
	lacksBody(t, body, `id="col-quote-2"`)
}

// --- fragment endpoints (htmx swaps) ---

func TestSearchRootFragment(t *testing.T) {
	srv := newServer(t, newFake(
		quoteRow(1, "MN 22", "the Buddha, MN 22", "the buddha spoke"),
		quoteRow(2, "AN 5", "a monk, AN 5", "a quiet forest"),
	))
	body := do(t, srv, "GET", "/search/root?rq=buddha&cat=0", "").Body.String()
	containsBody(t, body, `<div id="quote-list"`) // primary swap target
	containsBody(t, body, `id="root-count"`)      // OOB count refresh
	lacksBody(t, body, `<section id="root-zone"`) // not a full zone (focus safety)
	lacksBody(t, body, `name="rq"`)               // input not re-rendered (focus safety)
	lacksBody(t, body, `id="quote-2"`)            // filtered
}

func TestSearchCollectionFragment(t *testing.T) {
	fs := newFake(
		quoteRow(1, "MN 22", "the Buddha, MN 22", "the buddha spoke"),
		quoteRow(2, "AN 5", "a monk, AN 5", "a forest monk"),
	)
	cid, err := fs.CreateCollection([]int64{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	srv := newServer(t, fs)
	body := do(t, srv, "GET", fmt.Sprintf("/search/collection?cq=buddha&col=%d", cid), "").Body.String()
	containsBody(t, body, `<div id="collection-list"`)
	containsBody(t, body, `id="collection-count"`)
	lacksBody(t, body, `<section id="collection-zone"`)
	lacksBody(t, body, "insert-gap")       // gaps hidden while searching
	lacksBody(t, body, "data-reorder")     // reorder disabled while searching
	lacksBody(t, body, `draggable="true"`) // blocks not draggable while searching
	containsBody(t, body, `id="col-quote-1"`)
	lacksBody(t, body, `id="col-quote-2"`)
}

// --- search is cleared by a pane swap / not preserved by mutations ---

func TestPaneSwapClearsRootSearch(t *testing.T) {
	fs, cid := fakeWithCategory(t)
	srv := newServer(t, fs)
	body := do(t, srv, "GET", fmt.Sprintf("/pane/root?cat=%d&rq=human", cid), "").Body.String()
	containsBody(t, body, `name="rq"`) // input still rendered
	lacksBody(t, body, "<mark>")       // but the search is dropped
	lacksBody(t, body, `value="human"`)
}

func TestCreateDoesNotPreserveSearch(t *testing.T) {
	srv := newServer(t, newFake(quoteRow(1, "MN 22", "the Buddha, MN 22", "the buddha")))
	body := do(t, srv, "POST", "/quotes?rq=buddha",
		"content=the buddha sat&attribution=the%20Buddha&text_id=MN%2022",
		"Content-Type", "application/x-www-form-urlencoded",
		"HX-Request", "true").Body.String()
	lacksBody(t, body, "<mark>") // create re-renders the full active set, no filter
}

// --- regression: collection list keeps gaps + drag when NOT searching ---

func TestCollectionListGapsAndDragWhenNotSearching(t *testing.T) {
	fs, cid := fakeWithCollection(t)
	srv := newServer(t, fs)
	body := do(t, srv, "GET", fmt.Sprintf("/?col=%d", cid), "").Body.String()
	containsBody(t, body, "insert-gap")
	containsBody(t, body, `draggable="true"`)
	containsBody(t, body, "data-reorder")
}
