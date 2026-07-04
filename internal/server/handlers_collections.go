package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/lavantien/quotes-manager/internal/quote"
)

// collection renders the full dual-pane page with a collection active in the
// right column (deep-link/refresh friendly), paralleling the in-place pane swap.
func (s *Server) collection(w http.ResponseWriter, r *http.Request) {
	cid, ok := parseID(w, r, "cid")
	if !ok {
		return
	}
	if _, err := s.store.GetCollection(cid); err != nil {
		handleStoreErr(w, err)
		return
	}
	data, err := s.buildPageData(parseQueryID(r, "cat"), cid, parseQueryStr(r, "rq"), parseQueryStr(r, "cq"))
	if err != nil {
		serverError(w, err)
		return
	}
	s.render(w, "page", data)
}

// createCollection spins up a new collection from the checked root quotes and
// makes it the active right-column collection. Returns the collection zone
// (primary), plus out-of-band refreshes of the right rail (new entry) and the
// root zone (membership chips on the added quotes). cat is carried so the root
// zone preserves its current category filter.
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
	s.swapCollectionZone(w, parseQueryID(r, "cat"), cid, "", "", true, true)
}

// addCollectionItems prepends quotes to an existing collection (the legacy
// "add to collection" path) and redirects so it becomes the active collection.
func (s *Server) addCollectionItems(w http.ResponseWriter, r *http.Request) {
	cid, ok := parseID(w, r, "cid")
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		badRequest(w)
		return
	}
	if err := s.store.AddToCollection(cid, parseIDs(r.PostForm["id"])); err != nil {
		handleStoreErr(w, err)
		return
	}
	w.Header().Set("HX-Redirect", fmt.Sprintf("/?col=%d", cid))
	w.WriteHeader(http.StatusOK)
}

// insertCollectionItems inserts the checked root quotes at the given 1-based
// position in an existing collection, then swaps the collection zone (primary)
// with out-of-band refreshes of the right rail (count) and the root zone
// (membership chips). Form fields: id (repeated) + pos.
func (s *Server) insertCollectionItems(w http.ResponseWriter, r *http.Request) {
	cid, ok := parseID(w, r, "cid")
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		badRequest(w)
		return
	}
	pos := 1
	if v := strings.TrimSpace(r.PostForm.Get("pos")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			pos = n
		}
	}
	if err := s.store.InsertAtCollection(cid, parseIDs(r.PostForm["id"]), pos); err != nil {
		handleStoreErr(w, err)
		return
	}
	// Clear cq so the just-inserted quotes are visible (a filtered view would
	// hide them). rq is irrelevant to the collection zone but passed through for
	// consistency; search state does not survive a mutation.
	s.swapCollectionZone(w, parseQueryID(r, "cat"), cid, "", "", true, true)
}

// swapCollectionZone renders the collection zone for cid as the primary swap
// target, optionally with out-of-band refreshes of the right rail and root zone.
func (s *Server) swapCollectionZone(w http.ResponseWriter, catID, colID int64, rq, cq string, oobRail, oobRoot bool) {
	data, err := s.buildPageData(catID, colID, rq, cq)
	if err != nil {
		serverError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	s.exec(w, "collection_zone", data)
	if oobRail {
		s.exec(w, "rail_right_oob", data)
	}
	if oobRoot {
		s.exec(w, "root_zone_oob", data)
	}
}

func (s *Server) renameCollection(w http.ResponseWriter, r *http.Request) {
	cid, ok := parseID(w, r, "cid")
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
	if err := s.store.RenameCollection(cid, name); err != nil {
		handleStoreErr(w, err)
		return
	}
	data, err := s.railData(parseQueryID(r, "cat"), parseQueryID(r, "col"))
	if err != nil {
		serverError(w, err)
		return
	}
	s.render(w, "rail_right", data)
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

// collectionReorder rewrites a collection's positions from a JSON {ids} body
// (drag-and-drop). Returns 204; the client has already reordered the DOM.
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

// --- pane + rail fragment endpoints (htmx in-place swaps) ---

// rootPaneHandler returns the root zone (home or category-filtered) plus an
// out-of-band left rail so the active highlight follows the selection. A pane
// swap intentionally drops the search: a freshly selected active set is a new
// view, not a filtered one.
func (s *Server) rootPaneHandler(w http.ResponseWriter, r *http.Request) {
	data, err := s.buildPageData(parseQueryID(r, "cat"), parseQueryID(r, "col"), "", "")
	if err != nil {
		serverError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	s.exec(w, "root_zone", data)
	s.exec(w, "rail_left_oob", data)
}

// collectionPaneHandler returns the collection zone for ?col= (or the empty
// placeholder when col is absent/0) plus an out-of-band right rail.
func (s *Server) collectionPaneHandler(w http.ResponseWriter, r *http.Request) {
	data, err := s.buildPageData(parseQueryID(r, "cat"), parseQueryID(r, "col"), "", "")
	if err != nil {
		serverError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	s.exec(w, "collection_zone", data)
	s.exec(w, "rail_right_oob", data)
}

func (s *Server) leftRailHandler(w http.ResponseWriter, r *http.Request) {
	data, err := s.railData(parseQueryID(r, "cat"), parseQueryID(r, "col"))
	if err != nil {
		serverError(w, err)
		return
	}
	s.render(w, "rail_left", data)
}

func (s *Server) rightRailHandler(w http.ResponseWriter, r *http.Request) {
	data, err := s.railData(parseQueryID(r, "cat"), parseQueryID(r, "col"))
	if err != nil {
		serverError(w, err)
		return
	}
	s.render(w, "rail_right", data)
}
