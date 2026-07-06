package server

import (
	"net/http"

	"github.com/lavantien/quotes-manager/internal/quote"
)

// mergeDuplicates folds a duplicate group into its representative. It recomputes
// the groups (they are derived, not stored), finds the one whose representative
// is the requested id, and merges every other member into it via
// store.MergeQuotes. The response refreshes the root column and both rails so
// the merged-away blocks vanish and the Duplicates section recomputes.
func (s *Server) mergeDuplicates(w http.ResponseWriter, r *http.Request) {
	repID, ok := parseID(w, r, "repID")
	if !ok {
		return
	}
	qs, err := s.store.List()
	if err != nil {
		serverError(w, err)
		return
	}
	items := make([]quote.DupItem, len(qs))
	for i, q := range qs {
		items[i] = quote.DupItem{ID: q.ID, Text: q.BodyText}
	}
	var merge []int64
	for _, g := range quote.GroupDuplicates(items, quote.DefaultDuplicateThreshold) {
		if len(g) > 1 && g[0] == repID {
			merge = g[1:]
			break
		}
	}
	if merge == nil {
		http.NotFound(w, r)
		return
	}
	if err := s.store.MergeQuotes(repID, merge); err != nil {
		handleStoreErr(w, err)
		return
	}
	cat, col := parseQueryID(r, "cat"), parseQueryID(r, "col")
	page, err := s.buildPageData(cat, col, parseQueryStr(r, "rq"), parseQueryStr(r, "cq"))
	if err != nil {
		serverError(w, err)
		return
	}
	rail, err := s.railData(cat, col)
	if err != nil {
		serverError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	s.exec(w, "root_zone_oob", page)
	s.exec(w, "rail_left_oob", rail)
	s.exec(w, "rail_right_oob", rail)
}
