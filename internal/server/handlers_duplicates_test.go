package server_test

import (
	"net/http"
	"regexp"
	"testing"

	"github.com/lavantien/quotes-manager/internal/quote"
	"github.com/lavantien/quotes-manager/internal/server"
	"github.com/lavantien/quotes-manager/internal/store"
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

func quoteWithText(id int64, passage string) store.Quote {
	q := quote.New("MN 10", "the Buddha, MN 10", []string{passage})
	return store.Quote{
		ID: id, SuttaID: q.SuttaID, Citation: q.Citation,
		BodyMD: q.BodyMD(), BodyText: q.BodyText(), LineCount: q.LineCount(), CharCount: q.CharCount(),
	}
}

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

	containsBody(t, body, `id="root-zone"`)
	containsBody(t, body, `id="left-rail"`)
	containsBody(t, body, `id="right-rail"`)
	containsBody(t, body, "id=\"quote-"+rep+"\"")
	containsBody(t, body, ">107 blocks<")
	lacksBody(t, body, `MN 22 <span class="rail__count">3</span>`)

	after := do(t, srv, "GET", "/", "").Body.String()
	lacksBody(t, after, `MN 22 <span class="rail__count">3</span>`)
}

func TestMergeDuplicatesUnknownRep(t *testing.T) {
	srv := server.New(storetest.CloneFixture(t))
	rec := do(t, srv, "POST", "/duplicates/9999/merge", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestMergeDuplicatesSingletonRepNotFound(t *testing.T) {
	fs := newFake(quoteWithText(1, "one wholly distinct passage"), quoteWithText(2, "two entirely different wording"))
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", "/duplicates/1/merge", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if len(fs.quotes) != 2 {
		t.Errorf("store has %d quotes, want 2", len(fs.quotes))
	}
}

func TestMergeDuplicatesNonRepresentativeMemberNotFound(t *testing.T) {
	fs := newFake(
		quoteWithText(1, "same body text"),
		quoteWithText(2, "same body text"),
		quoteWithText(3, "same body text"),
	)
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", "/duplicates/2/merge", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if len(fs.quotes) != 3 {
		t.Errorf("store has %d quotes, want 3", len(fs.quotes))
	}
}

func TestMergeDuplicatesStoreError(t *testing.T) {
	assert500(t, newServer(t, failingStore{}), "POST", "/duplicates/1/merge", "")
}
