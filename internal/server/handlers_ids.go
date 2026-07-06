package server

import (
	"net/http"
	"sort"
	"strings"

	"github.com/lavantien/quotes-manager/internal/store"
)

// corpusIDs writes the corpus's text ids (the sutta_id of every quote), deduped
// and sorted, one per line — a plain-text counterpart to /export.txt for quick
// "which suttas are covered" extraction.
func (s *Server) corpusIDs(w http.ResponseWriter, r *http.Request) {
	qs, err := s.store.List()
	if err != nil {
		serverError(w, err)
		return
	}
	writeIDs(w, qs)
}

// collectionIDs writes the active collection's text ids, deduped and sorted,
// paralleling /collections/{cid}/export.txt.
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
	writeIDs(w, qs)
}

// writeIDs writes the deduped, sorted, one-per-line sutta_ids of qs as UTF-8
// plain text. Empty sutta_ids are dropped so the output never has blank lines;
// the body ends with a trailing newline when non-empty and is empty otherwise.
func writeIDs(w http.ResponseWriter, qs []store.Quote) {
	ids := make([]string, len(qs))
	for i, q := range qs {
		ids[i] = q.SuttaID
	}
	ids = uniqueSortedIDs(ids)
	body := strings.Join(ids, "\n")
	if len(ids) > 0 {
		body += "\n"
	}
	writeText(w, body)
}

// uniqueSortedIDs returns the non-empty, deduplicated, lexicographically
// (byte-order) sorted subset of ids. Sorting is case-sensitive on purpose so
// the canonical sutta forms (e.g. "MN 22", "AN 5.34") group predictably.
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
