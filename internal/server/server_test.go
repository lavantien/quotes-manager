package server_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lavantien/quotes-manager/internal/quote"
	"github.com/lavantien/quotes-manager/internal/server"
	"github.com/lavantien/quotes-manager/internal/store"
)

func newServer(t *testing.T, s store.Store) *server.Server {
	t.Helper()
	return server.New(s)
}

func do(t *testing.T, srv *server.Server, method, target, body string, hdrs ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	for i := 0; i+1 < len(hdrs); i += 2 {
		req.Header.Set(hdrs[i], hdrs[i+1])
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func sampleQuote(id int64) store.Quote {
	q := quote.New("MN 22", "the Buddha, MN 22", []string{`"Human beings are shady."`})
	return store.Quote{
		ID: id, SuttaID: q.SuttaID, Citation: q.Citation,
		BodyMD: q.BodyMD(), BodyText: q.BodyText(), LineCount: q.LineCount(), CharCount: q.CharCount(),
	}
}

func TestIndexListsQuotesAndLinksSuttacentral(t *testing.T) {
	srv := newServer(t, newFake(sampleQuote(1)))
	rec := do(t, srv, "GET", "/", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Human beings are shady") {
		t.Error("index missing quote body")
	}
	if !strings.Contains(body, `href="/texts/mn22"`) {
		t.Error("index missing suttacentral link")
	}
	if !strings.Contains(body, `<strong>MN 22</strong>`) {
		t.Error("index missing bolded sutta id")
	}
}

func TestIndexShowsBlockCount(t *testing.T) {
	srv := newServer(t, newFake(sampleQuote(1), sampleQuote(2)))
	rec := do(t, srv, "GET", "/", "")
	if !strings.Contains(rec.Body.String(), "2 blocks") {
		t.Error("index missing block count badge")
	}
}

func TestCreateAddsQuote(t *testing.T) {
	fs := newFake()
	srv := newServer(t, fs)
	body := "content=%22Be+your+own+island.%22&attribution=the+Buddha&text_id=MN+44"
	rec := do(t, srv, "POST", "/quotes", body, "Content-Type", "application/x-www-form-urlencoded", "HX-Request", "true")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if len(fs.quotes) != 1 {
		t.Fatalf("store has %d quotes, want 1", len(fs.quotes))
	}
	q := fs.quotes[0]
	if q.SuttaID != "MN 44" {
		t.Errorf("SuttaID = %q", q.SuttaID)
	}
	if q.Citation != "the Buddha, MN 44" {
		t.Errorf("Citation = %q", q.Citation)
	}
	if !strings.Contains(rec.Body.String(), `href="/texts/mn44"`) {
		t.Error("create response missing link to new quote")
	}
}

func TestCreateDefaultsAttribution(t *testing.T) {
	fs := newFake()
	srv := newServer(t, fs)
	body := "content=%22x%22&text_id=SN+22.59"
	do(t, srv, "POST", "/quotes", body, "Content-Type", "application/x-www-form-urlencoded", "HX-Request", "true")

	if len(fs.quotes) != 1 {
		t.Fatal("no quote created")
	}
	if want := "the Buddha, SN 22.59"; fs.quotes[0].Citation != want {
		t.Errorf("Citation = %q, want %q", fs.quotes[0].Citation, want)
	}
}

func TestUpdateChangesQuote(t *testing.T) {
	fs := newFake(sampleQuote(1))
	srv := newServer(t, fs)
	body := "content=%22new+text%22&attribution=the+Buddha&text_id=DN+16"
	rec := do(t, srv, "POST", "/quotes/1", body, "Content-Type", "application/x-www-form-urlencoded", "HX-Request", "true")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if fs.quotes[0].SuttaID != "DN 16" || fs.quotes[0].Citation != "the Buddha, DN 16" {
		t.Errorf("not updated: %+v", fs.quotes[0])
	}
}

func TestUpdateNotFound(t *testing.T) {
	srv := newServer(t, newFake())
	body := "content=x&text_id=MN+1"
	rec := do(t, srv, "POST", "/quotes/999", body, "Content-Type", "application/x-www-form-urlencoded")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestDeleteOne(t *testing.T) {
	fs := newFake(sampleQuote(1))
	srv := newServer(t, fs)
	rec := do(t, srv, "DELETE", "/quotes/1", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if len(fs.quotes) != 0 {
		t.Errorf("store still has %d quotes", len(fs.quotes))
	}
}

func TestBulkDelete(t *testing.T) {
	fs := newFake(sampleQuote(1), sampleQuote(2), sampleQuote(3))
	srv := newServer(t, fs)
	body := "id=1&id=3"
	rec := do(t, srv, "POST", "/quotes/delete", body, "Content-Type", "application/x-www-form-urlencoded")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if len(fs.quotes) != 1 || fs.quotes[0].ID != 2 {
		t.Errorf("after bulk delete = %+v", fs.quotes)
	}
}

func TestCreateRendersSortedList(t *testing.T) {
	fs := newFake()
	srv := newServer(t, fs)
	body := "content=%22Be+your+own+island.%22&attribution=the+Buddha&text_id=MN+44"
	rec := do(t, srv, "POST", "/quotes", body, "Content-Type", "application/x-www-form-urlencoded", "HX-Request", "true")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), `id="quote-list"`) {
		t.Error("create should re-render the full quote list, not a single block")
	}
}

func TestExportTxt(t *testing.T) {
	srv := newServer(t, newFake(sampleQuote(1), sampleQuote(2)))
	rec := do(t, srv, "GET", "/export.txt", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q", ct)
	}

	if !strings.Contains(rec.Body.String(), ".  \n.  \n.") {
		t.Error("export missing dot separator")
	}
}

func TestCopyOne(t *testing.T) {
	srv := newServer(t, newFake(sampleQuote(1)))
	rec := do(t, srv, "GET", "/quotes/1/copy", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	want := `*"Human beings are shady."* - **the Buddha, MN 22**`
	if b, _ := io.ReadAll(rec.Body); string(b) != want {
		t.Errorf("copy = %q, want %q", string(b), want)
	}
}

func TestCopyOneNotFound(t *testing.T) {
	srv := newServer(t, newFake())
	rec := do(t, srv, "GET", "/quotes/999/copy", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestEditFormPrefilled(t *testing.T) {
	srv := newServer(t, newFake(sampleQuote(1)))
	rec := do(t, srv, "GET", "/quotes/1/edit", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `value="MN 22"`) {
		t.Error("edit form missing text_id value")
	}
	if !strings.Contains(body, `Human beings are shady`) {
		t.Error("edit form missing content")
	}

	if !strings.Contains(body, `id="quote-1"`) {
		t.Error("edit form missing id=quote-1 handle")
	}
}

func TestNavRendered(t *testing.T) {
	srv := newServer(t, newFake())
	rec := do(t, srv, "GET", "/", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, m := range []string{
		`href="/static/nav.css"`,
		`<nav class="nav">`,
		`class="nav-brand" href="/">qm</a>`,
		`href="/dashboard">overview</a>`,
		`href="/texts">texts</a>`,
		`href="/search">search</a>`,
		`href="/words">words</a>`,
		`href="/graph">graph</a>`,
		`href="/analytics">analytics</a>`,
		`href="/pipeline">pipeline</a>`,
		`href="/discrepancies">discrepancies</a>`,
	} {
		if !strings.Contains(body, m) {
			t.Errorf("page missing nav marker %q", m)
		}
	}
}

func TestStaticAssetsServed(t *testing.T) {
	srv := newServer(t, newFake())
	for _, p := range []string{"/static/app.css", "/static/app.js", "/static/htmx.min.js"} {
		rec := do(t, srv, "GET", p, "")
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d", p, rec.Code)
		}
	}
	if rec := do(t, srv, "GET", "/static/app.css", ""); !strings.Contains(rec.Body.String(), "--paper") {
		t.Error("app.css not served correctly")
	}
	for _, p := range []string{"/static/app.css", "/static/htmx.min.js", "/static/canon.css", "/static/cytoscape.min.js"} {
		if cc := do(t, srv, "GET", p, "").Header().Get("Cache-Control"); cc != "public, max-age=86400" {
			t.Errorf("%s: cache-control = %q", p, cc)
		}
	}
}

func TestHandlerStoreErrors(t *testing.T) {
	srv := newServer(t, failingStore{})
	cases := []struct {
		method, target, body, ct string
	}{
		{"GET", "/", "", ""},
		{"GET", "/quotes", "", ""},
		{"GET", "/quotes/new", "", ""},
		{"POST", "/quotes", "content=x&text_id=MN+1", "application/x-www-form-urlencoded"},
		{"POST", "/quotes/1", "content=x&text_id=MN+1", "application/x-www-form-urlencoded"},
		{"DELETE", "/quotes/1", "", ""},
		{"POST", "/quotes/delete", "id=1", "application/x-www-form-urlencoded"},
		{"GET", "/quotes/1/copy", "", ""},
		{"GET", "/quotes/1/edit", "", ""},
		{"GET", "/quotes/1/categories", "", ""},
		{"GET", "/quotes/1/categories/edit", "", ""},
		{"GET", "/export.txt", "", ""},
		{"GET", "/collections/1", "", ""},
		{"POST", "/collections", "id=1", "application/x-www-form-urlencoded"},
		{"POST", "/collections/1/items", "id=2", "application/x-www-form-urlencoded"},
		{"POST", "/collections/1/insert", "id=2&pos=1", "application/x-www-form-urlencoded"},
		{"POST", "/collections/1/rename", "name=x", "application/x-www-form-urlencoded"},
		{"DELETE", "/collections/1", "", ""},
		{"POST", "/collections/1/reorder", `{"ids":[1]}`, "application/json"},
		{"GET", "/collections/1/export.txt", "", ""},
		{"GET", "/ids.txt", "", ""},
		{"GET", "/collections/1/ids.txt", "", ""},
		{"GET", "/categories/1", "", ""},
		{"GET", "/categories/1/export.txt", "", ""},
		{"POST", "/categories", "name=x", "application/x-www-form-urlencoded"},
		{"POST", "/categories/1/rename", "name=x", "application/x-www-form-urlencoded"},
		{"DELETE", "/categories/1", "", ""},
		{"POST", "/quotes/1/categories", "id=1", "application/x-www-form-urlencoded"},
		{"GET", "/rail/left", "", ""},
		{"GET", "/rail/right", "", ""},
		{"GET", "/pane/root", "", ""},
		{"GET", "/pane/collection", "", ""},
		{"GET", "/pane/check", "", ""},
		{"POST", "/check", "ids=MN+1", "application/x-www-form-urlencoded"},
	}
	for _, c := range cases {
		rec := do(t, srv, c.method, c.target, c.body, "Content-Type", c.ct)
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("%s %s: code = %d, want 500", c.method, c.target, rec.Code)
		}
	}
}

func TestNewForm(t *testing.T) {
	srv := newServer(t, newFake())
	rec := do(t, srv, "GET", "/quotes/new", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `hx-post="/quotes"`) {
		t.Error("new form should post to /quotes")
	}
	if !strings.Contains(body, "Add quote") {
		t.Error("new form missing submit label")
	}
}

func TestListFragment(t *testing.T) {
	srv := newServer(t, newFake())
	rec := do(t, srv, "GET", "/quotes", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `id="quote-list"`) {
		t.Error("list fragment should render the quote-list container")
	}
}

func TestCreateNonHTMXRedirects(t *testing.T) {
	srv := newServer(t, newFake())
	rec := do(t, srv, "POST", "/quotes", "content=%22x%22&text_id=MN+1",
		"Content-Type", "application/x-www-form-urlencoded")
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Errorf("Location = %q, want /", loc)
	}
}

func TestUpdateNonHTMXRedirects(t *testing.T) {
	srv := newServer(t, newFake(sampleQuote(1)))
	rec := do(t, srv, "POST", "/quotes/1", "content=%22x%22&text_id=MN+1",
		"Content-Type", "application/x-www-form-urlencoded")
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Errorf("Location = %q, want /", loc)
	}
}

func TestIndexUnknownCategoryEmptyRoot(t *testing.T) {
	srv := newServer(t, newFake(sampleQuote(1)))
	rec := do(t, srv, "GET", "/?cat=999", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func assert500(t *testing.T, srv *server.Server, method, target, body string, hdrs ...string) {
	t.Helper()
	if rec := do(t, srv, method, target, body, hdrs...); rec.Code != http.StatusInternalServerError {
		t.Errorf("%s %s: code = %d, want 500", method, target, rec.Code)
	}
}

func TestBuildPageDataDeepErrors(t *testing.T) {
	for _, c := range []struct {
		name  string
		store store.Store
	}{
		{"ListCollections fails", failListCols{newFake(sampleQuote(1))}},
		{"QuoteCategoryMap fails", failCatMap{newFake(sampleQuote(1))}},
		{"QuoteCollectionMap fails", failColMap{newFake(sampleQuote(1))}},
		{"List fails in buildRootPane", failList{newFake(sampleQuote(1))}},
	} {
		t.Run(c.name, func(t *testing.T) {
			assert500(t, newServer(t, c.store), "GET", "/", "")
		})
	}
}

func TestRenderQuoteListDeepErrors(t *testing.T) {
	for _, c := range []struct {
		name  string
		store store.Store
	}{
		{"QuoteCategoryMap fails", failCatMap{newFake(sampleQuote(1))}},
		{"QuoteCollectionMap fails", failColMap{newFake(sampleQuote(1))}},
	} {
		t.Run(c.name, func(t *testing.T) {
			assert500(t, newServer(t, c.store), "GET", "/quotes", "")
		})
	}
}

func TestRenderQuoteBlockDeepErrors(t *testing.T) {
	for _, c := range []struct {
		name  string
		store store.Store
	}{
		{"QuoteCategoryMap fails", failCatMap{newFake(sampleQuote(1))}},
		{"QuoteCollectionMap fails", failColMap{newFake(sampleQuote(1))}},
	} {
		t.Run(c.name, func(t *testing.T) {
			assert500(t, newServer(t, c.store), "POST", "/quotes/1", "content=%22x%22&text_id=MN+1",
				"Content-Type", "application/x-www-form-urlencoded", "HX-Request", "true")
		})
	}
}

func TestBuildRootPaneCategoryQuotesError(t *testing.T) {
	fs, cid := fakeWithCategory(t)
	assert500(t, newServer(t, failCatQuotes{fs}), "GET", fmt.Sprintf("/categories/%d", cid), "")
}

func TestRailDataDeepError(t *testing.T) {
	assert500(t, newServer(t, failListCols{newFake(sampleQuote(1))}), "GET", "/rail/left", "")
}

func TestEditQuoteCategoriesDeepErrors(t *testing.T) {
	for _, c := range []struct {
		name  string
		store store.Store
	}{
		{"ListCategories fails", failListCats{newFake(sampleQuote(1))}},
		{"QuoteCategoryMap fails", failCatMap{newFake(sampleQuote(1))}},
	} {
		t.Run(c.name, func(t *testing.T) {
			assert500(t, newServer(t, c.store), "GET", "/quotes/1/categories/edit", "")
		})
	}
}

func TestSetQuoteCategoriesDeepErrors(t *testing.T) {
	for _, c := range []struct {
		name  string
		store store.Store
	}{
		{"QuoteCategoryMap fails after set", failCatMap{newFake(sampleQuote(1))}},
		{"railData fails after set", failListCols{newFake(sampleQuote(1))}},
	} {
		t.Run(c.name, func(t *testing.T) {
			assert500(t, newServer(t, c.store), "POST", "/quotes/1/categories", "id=",
				"Content-Type", "application/x-www-form-urlencoded")
		})
	}
}

func TestSwapCollectionZoneBuildError(t *testing.T) {
	assert500(t, newServer(t, failCatMap{newFake(sampleQuote(1))}), "POST", "/collections", "id=1",
		"Content-Type", "application/x-www-form-urlencoded")
}

func TestRenameCollectionRailError(t *testing.T) {
	fs, cid := fakeWithCollection(t)
	assert500(t, newServer(t, failListCols{fs}),
		"POST", fmt.Sprintf("/collections/%d/rename", cid), "name=x",
		"Content-Type", "application/x-www-form-urlencoded")
}

func TestRemainingHandlerDeepErrors(t *testing.T) {
	t.Run("quoteChips QuoteCategoryMap fails", func(t *testing.T) {
		assert500(t, newServer(t, failCatMap{newFake(sampleQuote(1))}), "GET", "/quotes/1/categories", "")
	})
	t.Run("railData ListCategories fails", func(t *testing.T) {
		assert500(t, newServer(t, failListCats{newFake(sampleQuote(1))}), "GET", "/rail/left", "")
	})
	t.Run("update railData fails after Update", func(t *testing.T) {
		assert500(t, newServer(t, failListCats{newFake(sampleQuote(1))}), "POST", "/quotes/1",
			"content=%22x%22&text_id=MN+1", "Content-Type", "application/x-www-form-urlencoded", "HX-Request", "true")
	})
	t.Run("collection buildPageData fails", func(t *testing.T) {
		fs, cid := fakeWithCollection(t)
		assert500(t, newServer(t, failCatMap{fs}), "GET", fmt.Sprintf("/collections/%d", cid), "")
	})
	t.Run("category buildPageData fails", func(t *testing.T) {
		fs, cid := fakeWithCategory(t)
		assert500(t, newServer(t, failCatMap{fs}), "GET", fmt.Sprintf("/categories/%d", cid), "")
	})
}

func TestRenderLeftRailError(t *testing.T) {
	assert500(t, newServer(t, failListCats{newFake(sampleQuote(1))}),
		"POST", "/categories", "name=x", "Content-Type", "application/x-www-form-urlencoded")
}
