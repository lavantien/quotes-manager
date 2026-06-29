package server_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/lavantien/quotes-manager/internal/quote"
	"github.com/lavantien/quotes-manager/internal/server"
	"github.com/lavantien/quotes-manager/internal/store"
)

// fakeStore is an in-memory store.Store for handler tests.
type fakeStore struct {
	quotes  []store.Quote
	nextID  int64
	maxSort int64
}

func newFake(quotes ...store.Quote) *fakeStore {
	maxID := int64(0)
	maxSort := int64(0)
	for _, q := range quotes {
		if q.ID > maxID {
			maxID = q.ID
		}
		if q.SortOrder > maxSort {
			maxSort = q.SortOrder
		}
	}
	if maxID == 0 {
		maxID = 0
	}
	return &fakeStore{quotes: append([]store.Quote{}, quotes...), nextID: maxID, maxSort: maxSort}
}

func (f *fakeStore) List() ([]store.Quote, error) {
	out := make([]store.Quote, len(f.quotes))
	copy(out, f.quotes)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].SortOrder != out[j].SortOrder {
			return out[i].SortOrder < out[j].SortOrder
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (f *fakeStore) Get(id int64) (store.Quote, error) {
	for _, q := range f.quotes {
		if q.ID == id {
			return q, nil
		}
	}
	return store.Quote{}, store.ErrNotFound
}

func (f *fakeStore) Create(q *quote.Quote) (int64, error) {
	f.nextID++
	f.maxSort++
	row := store.Quote{
		ID: f.nextID, SortOrder: f.maxSort, SuttaID: q.SuttaID, Citation: q.Citation,
		BodyMD: q.BodyMD(), BodyText: q.BodyText(), LineCount: q.LineCount(),
		CharCount: q.CharCount(), Sources: append([]string(nil), q.Sources...),
	}
	f.quotes = append(f.quotes, row)
	return row.ID, nil
}

func (f *fakeStore) Update(id int64, q *quote.Quote) error {
	for i := range f.quotes {
		if f.quotes[i].ID == id {
			f.quotes[i].SuttaID = q.SuttaID
			f.quotes[i].Citation = q.Citation
			f.quotes[i].BodyMD = q.BodyMD()
			f.quotes[i].BodyText = q.BodyText()
			f.quotes[i].LineCount = q.LineCount()
			f.quotes[i].CharCount = q.CharCount()
			return nil
		}
	}
	return store.ErrNotFound
}

func (f *fakeStore) Delete(id int64) error {
	for i, q := range f.quotes {
		if q.ID == id {
			f.quotes = append(f.quotes[:i], f.quotes[i+1:]...)
			return nil
		}
	}
	return store.ErrNotFound
}

func (f *fakeStore) DeleteMany(ids []int64) error {
	keep := f.quotes[:0]
	drop := make(map[int64]bool)
	for _, id := range ids {
		drop[id] = true
	}
	for _, q := range f.quotes {
		if !drop[q.ID] {
			keep = append(keep, q)
		}
	}
	f.quotes = keep
	return nil
}

func (f *fakeStore) Reorder(orderedIDs []int64) error {
	for i, id := range orderedIDs {
		found := false
		for j := range f.quotes {
			if f.quotes[j].ID == id {
				f.quotes[j].SortOrder = int64(i + 1)
				found = true
			}
		}
		if !found {
			return store.ErrNotFound
		}
	}
	return nil
}

func (f *fakeStore) Close() error { return nil }

// --- helpers ---

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
		ID: id, SortOrder: id, SuttaID: q.SuttaID, Citation: q.Citation,
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
	if !strings.Contains(body, `href="https://suttacentral.net/mn22"`) {
		t.Error("index missing suttacentral link")
	}
	if !strings.Contains(body, `<strong>MN 22</strong>`) {
		t.Error("index missing bolded sutta id")
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
	if !strings.Contains(rec.Body.String(), `href="https://suttacentral.net/mn44"`) {
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

func TestReorder(t *testing.T) {
	fs := newFake(sampleQuote(1), sampleQuote(2), sampleQuote(3))
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", "/quotes/reorder", `{"ids":[3,1,2]}`, "Content-Type", "application/json")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	want := []int64{3, 1, 2}
	got, _ := fs.List()
	for i, q := range got {
		if q.ID != want[i] {
			t.Errorf("pos %d ID = %d, want %d", i, q.ID, want[i])
		}
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
	// The dot divider (".  \n.  \n.") joins consecutive quotes in the export.
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
}
