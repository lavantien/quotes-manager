package server

import (
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

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	data, err := s.buildPageData(parseQueryID(r, "cat"), parseQueryID(r, "col"), parseQueryStr(r, "rq"), parseQueryStr(r, "cq"))
	if err != nil {
		serverError(w, err)
		return
	}
	s.render(w, "page", data)
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
	catMap, err := s.store.QuoteCategoryMap()
	if err != nil {
		serverError(w, err)
		return
	}
	colMap, err := s.store.QuoteCollectionMap()
	if err != nil {
		serverError(w, err)
		return
	}
	s.render(w, "quote_list", pageData{Root: rootPane{Quotes: qs}, CatMap: catMap, ColMap: colMap})
}

func (s *Server) listFragment(w http.ResponseWriter, r *http.Request) {
	s.renderQuoteList(w)
}

// parseQueryID reads an optional positive int64 query parameter, returning 0
// when it is absent or invalid.
func parseQueryID(r *http.Request, key string) int64 {
	v := r.URL.Query().Get(key)
	if v == "" {
		return 0
	}
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil || id <= 0 {
		return 0
	}
	return id
}

// parseQueryStr reads an optional trimmed string query parameter (the search
// box value), returning "" when absent. Lowercasing/splitting happens in
// search.Terms.
func parseQueryStr(r *http.Request, key string) string {
	return strings.TrimSpace(r.URL.Query().Get(key))
}

// category renders the full dual-pane page with the root column filtered to a
// category. Kept as a deep-link/refresh-friendly full-page route alongside the
// in-place htmx pane swaps.
func (s *Server) category(w http.ResponseWriter, r *http.Request) {
	ctid, ok := parseID(w, r, "ctid")
	if !ok {
		return
	}
	if _, err := s.store.GetCategory(ctid); err != nil {
		handleStoreErr(w, err)
		return
	}
	data, err := s.buildPageData(ctid, parseQueryID(r, "col"), parseQueryStr(r, "rq"), parseQueryStr(r, "cq"))
	if err != nil {
		serverError(w, err)
		return
	}
	s.render(w, "page", data)
}

func (s *Server) createCategory(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		badRequest(w)
		return
	}
	name := strings.TrimSpace(r.PostForm.Get("name"))
	if name == "" {
		badRequest(w)
		return
	}
	if _, err := s.store.CreateCategory(name); err != nil {
		handleStoreErr(w, err)
		return
	}
	s.renderLeftRail(w, r)
}

func (s *Server) renameCategory(w http.ResponseWriter, r *http.Request) {
	ctid, ok := parseID(w, r, "ctid")
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		badRequest(w)
		return
	}
	name := strings.TrimSpace(r.PostForm.Get("name"))
	if name == "" {
		badRequest(w)
		return
	}
	if err := s.store.RenameCategory(ctid, name); err != nil {
		handleStoreErr(w, err)
		return
	}
	s.renderLeftRail(w, r)
}

func (s *Server) deleteCategory(w http.ResponseWriter, r *http.Request) {
	ctid, ok := parseID(w, r, "ctid")
	if !ok {
		return
	}
	if err := s.store.DeleteCategory(ctid); err != nil {
		handleStoreErr(w, err)
		return
	}
	w.Header().Set("HX-Redirect", "/")
	w.WriteHeader(http.StatusOK)
}

// renderLeftRail writes the left-rail fragment (Home + Categories). Active
// cat/col come from the request query so the highlight survives a rail swap.
func (s *Server) renderLeftRail(w http.ResponseWriter, r *http.Request) {
	data, err := s.railData(parseQueryID(r, "cat"), parseQueryID(r, "col"))
	if err != nil {
		serverError(w, err)
		return
	}
	s.render(w, "rail_left", data)
}

// renderQuoteChips writes the chip row for a quote from the current memberships.
func (s *Server) renderQuoteChips(w http.ResponseWriter, id int64) {
	m, err := s.store.QuoteCategoryMap()
	if err != nil {
		serverError(w, err)
		return
	}
	s.render(w, "quote_chips", chipsData{ID: id, Categories: m[id]})
}

// quoteChips returns a quote's read-only chip row (used to cancel the editor).
func (s *Server) quoteChips(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	if _, err := s.store.Get(id); err != nil {
		handleStoreErr(w, err)
		return
	}
	s.renderQuoteChips(w, id)
}

// editQuoteCategories returns the inline checkbox editor with a quote's current
// categories pre-checked.
func (s *Server) editQuoteCategories(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	if _, err := s.store.Get(id); err != nil {
		handleStoreErr(w, err)
		return
	}
	all, err := s.store.ListCategories()
	if err != nil {
		serverError(w, err)
		return
	}
	m, err := s.store.QuoteCategoryMap()
	if err != nil {
		serverError(w, err)
		return
	}
	current := make(map[int64]bool, len(m[id]))
	for _, c := range m[id] {
		current[c.ID] = true
	}
	items := make([]categoryItem, len(all))
	for i, c := range all {
		items[i] = categoryItem{Category: c, Checked: current[c.ID]}
	}
	s.render(w, "quote_category_editor", editorData{ID: id, Items: items})
}

// resolveCategoryID returns the id of name, creating the category if needed. A
// name that already exists (case-insensitively) resolves to the existing id so
// typing a known name simply selects it.
func (s *Server) resolveCategoryID(name string) (int64, error) {
	if id, err := s.store.CreateCategory(name); err == nil {
		return id, nil
	} else if !errors.Is(err, store.ErrDuplicate) {
		return 0, err
	}
	cats, err := s.store.ListCategories()
	if err != nil {
		return 0, err
	}
	for _, c := range cats {
		if strings.EqualFold(c.Name, name) {
			return c.ID, nil
		}
	}
	return 0, store.ErrDuplicate
}

// setQuoteCategories replaces a quote's categories from the editor submission
// (checked ids plus an optional new name) and returns the fresh chip row plus an
// out-of-band refresh of the left rail so category counts stay in sync.
func (s *Server) setQuoteCategories(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		badRequest(w)
		return
	}
	ids := parseIDs(r.PostForm["id"])
	if newName := strings.TrimSpace(r.PostForm.Get("new_name")); newName != "" {
		cid, err := s.resolveCategoryID(newName)
		if err != nil {
			handleStoreErr(w, err)
			return
		}
		ids = append(ids, cid)
	}
	if err := s.store.SetQuoteCategories(id, ids); err != nil {
		handleStoreErr(w, err)
		return
	}
	m, err := s.store.QuoteCategoryMap()
	if err != nil {
		serverError(w, err)
		return
	}
	rail, err := s.railData(parseQueryID(r, "cat"), parseQueryID(r, "col"))
	if err != nil {
		serverError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	s.exec(w, "quote_chips", chipsData{ID: id, Categories: m[id]})
	s.exec(w, "rail_left_oob", rail)
}

// categoryExport renders a category's quotes as the plain-text export format.
func (s *Server) categoryExport(w http.ResponseWriter, r *http.Request) {
	ctid, ok := parseID(w, r, "ctid")
	if !ok {
		return
	}
	qs, err := s.store.CategoryQuotes(ctid)
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
		// Re-render the whole list so the new quote is placed in char_count order,
		// then live-refresh the left rail (Duplicates + category counts) and the
		// root-zone block count via out-of-band swaps.
		rail, err := s.railData(parseQueryID(r, "cat"), parseQueryID(r, "col"))
		if err != nil {
			serverError(w, err)
			return
		}
		s.renderQuoteList(w)
		s.exec(w, "rail_left_oob", rail)
		s.exec(w, "root_count", rail.TotalQuotes)
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
		// Re-render the saved block, then live-refresh the left rail so the
		// Duplicates section tracks the new body text.
		rail, err := s.railData(parseQueryID(r, "cat"), parseQueryID(r, "col"))
		if err != nil {
			serverError(w, err)
			return
		}
		s.renderQuoteBlock(w, updated)
		s.exec(w, "rail_left_oob", rail)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// renderQuoteBlock writes a single editable block with its current category and
// collection chips, used after an inline edit saves.
func (s *Server) renderQuoteBlock(w http.ResponseWriter, q store.Quote) {
	catMap, err := s.store.QuoteCategoryMap()
	if err != nil {
		serverError(w, err)
		return
	}
	colMap, err := s.store.QuoteCollectionMap()
	if err != nil {
		serverError(w, err)
		return
	}
	s.render(w, "quote_block", quoteView{Quote: q, Cats: catMap[q.ID], Cols: colMap[q.ID]})
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
	if isHTMX(r) {
		// hx-swap="delete" removes the block; the response carries out-of-band
		// refreshes of both rails (category + collection counts) and the root-zone
		// block count, all of which can change when a quote is removed.
		cat, col := parseQueryID(r, "cat"), parseQueryID(r, "col")
		rail, err := s.railData(cat, col)
		if err != nil {
			serverError(w, err)
			return
		}
		displayed := rail.TotalQuotes
		if cat > 0 {
			if qs, qerr := s.store.CategoryQuotes(cat); qerr == nil {
				displayed = len(qs)
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		s.exec(w, "rail_left_oob", rail)
		s.exec(w, "rail_right_oob", rail)
		s.exec(w, "root_count", displayed)
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
	if errors.Is(err, store.ErrDuplicate) {
		http.Error(w, "name already in use", http.StatusConflict)
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
