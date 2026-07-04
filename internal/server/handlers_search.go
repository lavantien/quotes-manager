package server

import "net/http"

// searchRoot re-renders just the root quote list (primary swap target
// #quote-list) narrowed by the root search query ?rq= against the active
// category (or home when cat=0), plus an out-of-band refresh of the root-zone
// block count. Only the list is swapped, so the toolbar's search input retains
// focus between keystrokes.
func (s *Server) searchRoot(w http.ResponseWriter, r *http.Request) {
	data, err := s.buildPageData(parseQueryID(r, "cat"), 0, parseQueryStr(r, "rq"), "")
	if err != nil {
		serverError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	s.exec(w, "quote_list", data)
	s.exec(w, "root_count", data.Root.Count)
}

// searchCollection re-renders just the collection list (primary swap target
// #collection-list) narrowed by ?cq= against the active collection, plus an
// out-of-band refresh of the collection count. While a search is active the list
// drops its insert-gaps and drag-reorder affordances. Only the list is swapped,
// so the input retains focus.
func (s *Server) searchCollection(w http.ResponseWriter, r *http.Request) {
	data, err := s.buildPageData(0, parseQueryID(r, "col"), "", parseQueryStr(r, "cq"))
	if err != nil {
		serverError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	s.exec(w, "collection_list", data)
	s.exec(w, "collection_count", data)
}
