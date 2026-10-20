package server

import (
	"net/http"
	"sort"
	"strings"

	"github.com/lavantien/quotes-manager/internal/store"
)

func (s *Server) corpusIDs(w http.ResponseWriter, r *http.Request) {
	qs, err := s.store.List()
	if err != nil {
		serverError(w, err)
		return
	}
	writeIDs(w, qs)
}

func (s *Server) collectionIDs(w http.ResponseWriter, r *http.Request) {
	cid, ok := parseID(w, r, "cid")
	if !ok {
		return
	}
	qs, err := s.store.CollectionQuotes(cid)
	if err != nil {
		serverError(w, err)
		return
	}
	items, err := s.store.CollectionSuttaItems(cid)
	if err != nil {
		serverError(w, err)
		return
	}
	muids := make([]string, len(items))
	for i, it := range items {
		muids[i] = it.Muid
	}
	ids := make([]string, len(qs))
	for i, q := range qs {
		ids[i] = q.SuttaID
	}
	writeText(w, idLines(ids)+idLines(muids))
}

func writeIDs(w http.ResponseWriter, qs []store.Quote) {
	ids := make([]string, len(qs))
	for i, q := range qs {
		ids[i] = q.SuttaID
	}
	writeText(w, idLines(ids))
}

func idLines(ids []string) string {
	ids = uniqueSortedIDs(ids)
	if len(ids) == 0 {
		return ""
	}
	return strings.Join(ids, "\n") + "\n"
}

func uniqueSortedIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
