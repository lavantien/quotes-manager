package server

import (
	"net/http"

	"github.com/lavantien/quotes-manager/internal/quote"
)

// importQuotesForm renders the paste-import form into the root toolbar's
// #form-slot, mirroring the New-quote form.
func (s *Server) importQuotesForm(w http.ResponseWriter, r *http.Request) {
	s.render(w, "quote_import_form", nil)
}

// importQuotes parses pasted canonical-format text, de-duplicates within the
// paste, creates each quote, and re-renders the root list in sorted order with a
// live rail + count refresh, then clears the form.
func (s *Server) importQuotes(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		badRequest(w)
		return
	}
	for _, q := range quote.Dedup(quote.ParseCanonical(r.PostForm.Get("content"))) {
		if _, err := s.store.Create(q); err != nil {
			serverError(w, err)
			return
		}
	}
	if isHTMX(r) {
		rail, err := s.railData(parseQueryID(r, "cat"), parseQueryID(r, "col"))
		if err != nil {
			serverError(w, err)
			return
		}
		s.renderQuoteList(w)
		s.exec(w, "rail_left_oob", rail)
		s.exec(w, "root_count", rail.TotalQuotes)
		s.exec(w, "form_slot_clear", nil)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
