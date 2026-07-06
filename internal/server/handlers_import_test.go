package server_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/lavantien/quotes-manager/internal/quote"
	"github.com/lavantien/quotes-manager/internal/server"
	"github.com/lavantien/quotes-manager/internal/store/storetest"
)

func TestImportQuotesForm(t *testing.T) {
	srv := server.New(storetest.CloneFixture(t))
	body := do(t, srv, "GET", "/quotes/import/form", "").Body.String()
	containsBody(t, body, `name="content"`)
	containsBody(t, body, `hx-post="/quotes/import"`)
}

func TestImportQuotesHTTP(t *testing.T) {
	srv := server.New(storetest.CloneFixture(t))

	payload := quote.RenderExportFile([]*quote.Quote{
		quote.New("MN 22", "the Buddha, MN 22", []string{"imported alpha one", "imported alpha two"}),
		quote.New("AN 5.34", "the Buddha, AN 5.34", []string{"imported beta single"}),
	})
	form := url.Values{"content": {payload}}.Encode()

	rec := do(t, srv, "POST", "/quotes/import", form,
		"Content-Type", "application/x-www-form-urlencoded", "HX-Request", "true")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	resp := rec.Body.String()

	containsBody(t, resp, "imported alpha one")
	containsBody(t, resp, "imported beta single")
	containsBody(t, resp, `id="quote-list"`) // primary swap
	containsBody(t, resp, `id="left-rail"`)  // OOB rail refresh
	containsBody(t, resp, ">111 blocks<")    // 109 seed + 2 imported
	// The form-slot is cleared (empty OOB div, no textarea left).
	if strings.Contains(resp, `name="content"`) {
		t.Errorf("import form should be cleared after submit")
	}
}

func TestImportQuotesDedupsWithinPaste(t *testing.T) {
	srv := server.New(storetest.CloneFixture(t))

	// The same quote twice in one paste: de-duplicated to a single new row.
	dup := quote.RenderExportFile([]*quote.Quote{
		quote.New("MN 22", "the Buddha, MN 22", []string{"duplicate body text"}),
		quote.New("MN 22", "the Buddha, MN 22", []string{"duplicate body text"}),
	})
	form := url.Values{"content": {dup}}.Encode()
	rec := do(t, srv, "POST", "/quotes/import", form,
		"Content-Type", "application/x-www-form-urlencoded", "HX-Request", "true")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	containsBody(t, rec.Body.String(), ">110 blocks<") // 109 + 1
}

func TestImportQuotesStoreError(t *testing.T) {
	payload := quote.RenderExportFile([]*quote.Quote{
		quote.New("MN 22", "the Buddha, MN 22", []string{"some body"}),
	})
	form := url.Values{"content": {payload}}.Encode()
	assert500(t, newServer(t, failingStore{}), "POST", "/quotes/import", form,
		"Content-Type", "application/x-www-form-urlencoded", "HX-Request", "true")
}

func TestImportQuotesRailDataError(t *testing.T) {
	// Create succeeds on the fake, but the post-create rail refresh fails.
	payload := quote.RenderExportFile([]*quote.Quote{
		quote.New("MN 22", "the Buddha, MN 22", []string{"some body"}),
	})
	form := url.Values{"content": {payload}}.Encode()
	assert500(t, newServer(t, failListCats{newFake()}), "POST", "/quotes/import", form,
		"Content-Type", "application/x-www-form-urlencoded", "HX-Request", "true")
}

func TestImportQuotesNonHTMXRedirects(t *testing.T) {
	srv := server.New(storetest.CloneFixture(t))
	payload := quote.RenderExportFile([]*quote.Quote{
		quote.New("MN 22", "the Buddha, MN 22", []string{"redirect body"}),
	})
	form := url.Values{"content": {payload}}.Encode()
	rec := do(t, srv, "POST", "/quotes/import", form,
		"Content-Type", "application/x-www-form-urlencoded")
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", rec.Code)
	}
}
