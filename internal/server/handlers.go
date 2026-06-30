package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/lavantien/quotes-manager/internal/quote"
	"github.com/lavantien/quotes-manager/internal/store"
)

type pageData struct {
	Quotes      []store.Quote
	Collections []store.Collection
	View        viewSpec
	Count       int
}

// viewSpec describes which view is rendered (home or a collection) and drives the
// layout: home allows +New and the selection toolbar; a collection is read-only
// (copyable) with a delete-collection button.
type viewSpec struct {
	IsCollection bool
	CollectionID int64
	Title        string
	ExportURL    string
}

func (v viewSpec) CanNew() bool { return !v.IsCollection }

type formData struct {
	ID          int64
	Content     string
	Attribution string
	TextID      string
	Action      string
	SubmitLabel string
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	qs, err := s.store.List()
	if err != nil {
		serverError(w, err)
		return
	}
	cols, err := s.store.ListCollections()
	if err != nil {
		serverError(w, err)
		return
	}
	s.render(w, "page", pageData{Quotes: qs, Collections: cols, Count: len(qs), View: viewSpec{Title: "Quotes", ExportURL: "/export.txt"}})
}

// renderQuoteList re-renders the full (char_count-sorted) quote list as an
// HTMX fragment. Used after a create so the new quote lands in sorted order
// instead of being appended/prepended to the DOM.
func (s *Server) renderQuoteList(w http.ResponseWriter) {
	qs, err := s.store.List()
	if err != nil {
		serverError(w, err)
		return
	}
	s.render(w, "quote_list", pageData{Quotes: qs})
}

func (s *Server) listFragment(w http.ResponseWriter, r *http.Request) {
	s.renderQuoteList(w)
}

func (s *Server) collection(w http.ResponseWriter, r *http.Request) {
	cid, ok := parseID(w, r, "cid")
	if !ok {
		return
	}
	if _, err := s.store.GetCollection(cid); err != nil {
		handleStoreErr(w, err)
		return
	}
	qs, err := s.store.CollectionQuotes(cid)
	if err != nil {
		serverError(w, err)
		return
	}
	cols, err := s.store.ListCollections()
	if err != nil {
		serverError(w, err)
		return
	}
	s.render(w, "page", pageData{
		Quotes:      qs,
		Collections: cols,
		Count:       len(qs),
		View: viewSpec{
			IsCollection: true,
			CollectionID: cid,
			Title:        fmt.Sprintf("Collection %d", cid),
			ExportURL:    fmt.Sprintf("/collections/%d/export.txt", cid),
		},
	})
}

func (s *Server) createCollection(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		badRequest(w)
		return
	}
	cid, err := s.store.CreateCollection(parseIDs(r.PostForm["id"]))
	if err != nil {
		serverError(w, err)
		return
	}
	w.Header().Set("HX-Redirect", fmt.Sprintf("/collections/%d", cid))
	w.WriteHeader(http.StatusOK)
}

func (s *Server) deleteCollection(w http.ResponseWriter, r *http.Request) {
	cid, ok := parseID(w, r, "cid")
	if !ok {
		return
	}
	if err := s.store.DeleteCollection(cid); err != nil {
		handleStoreErr(w, err)
		return
	}
	w.Header().Set("HX-Redirect", "/")
	w.WriteHeader(http.StatusOK)
}

func (s *Server) collectionExport(w http.ResponseWriter, r *http.Request) {
	cid, ok := parseID(w, r, "cid")
	if !ok {
		return
	}
	qs, err := s.store.CollectionQuotes(cid)
	if err != nil {
		serverError(w, err)
		return
	}
	quotes := make([]*quote.Quote, len(qs))
	for i, q := range qs {
		quotes[i] = quote.New(q.SuttaID, q.Citation, splitPassages(q.BodyText))
	}
	writeText(w, quote.RenderExportFile(quotes))
}

func (s *Server) newForm(w http.ResponseWriter, r *http.Request) {
	s.render(w, "quote_form", formData{Action: "/quotes", SubmitLabel: "Add quote"})
}

func (s *Server) editForm(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	q, err := s.store.Get(id)
	if err != nil {
		handleStoreErr(w, err)
		return
	}
	s.render(w, "quote_form", formData{
		ID:          q.ID,
		Content:     q.BodyText,
		Attribution: attributionOf(q.Citation, q.SuttaID),
		TextID:      q.SuttaID,
		Action:      fmt.Sprintf("/quotes/%d", q.ID),
		SubmitLabel: "Save",
	})
}

func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		badRequest(w)
		return
	}
	if _, err := s.store.Create(buildQuote(r.PostForm)); err != nil {
		serverError(w, err)
		return
	}
	if isHTMX(r) {
		// Re-render the whole list so the new quote is placed in char_count order.
		s.renderQuoteList(w)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		badRequest(w)
		return
	}
	if err := s.store.Update(id, buildQuote(r.PostForm)); err != nil {
		handleStoreErr(w, err)
		return
	}
	updated, err := s.store.Get(id)
	if err != nil {
		serverError(w, err)
		return
	}
	if isHTMX(r) {
		s.render(w, "quote_block", updated)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	if err := s.store.Delete(id); err != nil {
		handleStoreErr(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) bulkDelete(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		badRequest(w)
		return
	}
	if err := s.store.DeleteMany(parseIDs(r.PostForm["id"])); err != nil {
		serverError(w, err)
		return
	}
	w.Header().Set("HX-Redirect", "/")
	w.WriteHeader(http.StatusOK)
}

func (s *Server) collectionReorder(w http.ResponseWriter, r *http.Request) {
	cid, ok := parseID(w, r, "cid")
	if !ok {
		return
	}
	var body struct {
		IDs []int64 `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w)
		return
	}
	if err := s.store.ReorderCollection(cid, body.IDs); err != nil {
		handleStoreErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) copyOne(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	q, err := s.store.Get(id)
	if err != nil {
		handleStoreErr(w, err)
		return
	}
	writeText(w, q.BodyMD)
}

func (s *Server) exportAll(w http.ResponseWriter, r *http.Request) {
	qs, err := s.store.List()
	if err != nil {
		serverError(w, err)
		return
	}
	quotes := make([]*quote.Quote, len(qs))
	for i, q := range qs {
		quotes[i] = quote.New(q.SuttaID, q.Citation, splitPassages(q.BodyText))
	}
	writeText(w, quote.RenderExportFile(quotes))
}

// buildQuote composes a quote.Quote from the 3-field form: content becomes
// passages (one per non-empty line), an empty attribution defaults to "the
// Buddha", and the citation is "<attribution>, <textId>".
func buildQuote(f url.Values) *quote.Quote {
	attribution := strings.TrimSpace(f.Get("attribution"))
	textID := strings.TrimSpace(f.Get("text_id"))
	if attribution == "" {
		attribution = "the Buddha"
	}
	citation := textID
	if attribution != "" && textID != "" {
		citation = attribution + ", " + textID
	}
	sutta := quote.CanonicalSuttaID(textID)
	if sutta == "" {
		sutta = textID
	}
	return quote.New(sutta, citation, splitPassages(f.Get("content")))
}

// splitPassages breaks body text into one passage per non-empty trimmed line.
func splitPassages(content string) []string {
	var out []string
	for _, line := range strings.Split(content, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

// attributionOf recovers the attribution from a citation by stripping the
// trailing sutta id. It returns "" when the citation is just the id.
func attributionOf(citation, sutta string) string {
	if sutta == "" || citation == sutta {
		return ""
	}
	rest := strings.TrimSuffix(citation, sutta)
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(rest), ","))
}

func parseID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id <= 0 {
		badRequest(w)
		return 0, false
	}
	return id, true
}

func parseIDs(vals []string) []int64 {
	out := make([]int64, 0, len(vals))
	for _, v := range vals {
		if id, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil && id > 0 {
			out = append(out, id)
		}
	}
	return out
}

func isHTMX(r *http.Request) bool { return r.Header.Get("HX-Request") == "true" }

func writeText(w http.ResponseWriter, s string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(s))
}

func handleStoreErr(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		notFound(w)
		return
	}
	serverError(w, err)
}

func serverError(w http.ResponseWriter, err error) {
	log.Printf("server error: %v", err)
	http.Error(w, "internal server error", http.StatusInternalServerError)
}

func badRequest(w http.ResponseWriter) {
	http.Error(w, "bad request", http.StatusBadRequest)
}

func notFound(w http.ResponseWriter) {
	http.Error(w, "not found", http.StatusNotFound)
}
