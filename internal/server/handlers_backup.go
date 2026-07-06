package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/lavantien/quotes-manager/internal/store"
)

// backup returns the whole database as a JSON attachment the user can save and
// later restore. It is the portable backup format (store.Dump).
func (s *Server) backup(w http.ResponseWriter, r *http.Request) {
	dump, err := s.store.Export()
	if err != nil {
		serverError(w, err)
		return
	}
	body, err := json.MarshalIndent(dump, "", "  ")
	if err != nil {
		serverError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="quotes-backup.json"`)
	_, _ = w.Write(append(body, '\n'))
}

// restore replaces every data table with the posted JSON dump. The whole-DB
// replace is destructive, so the client confirms first; on success the page
// reloads to reflect the restored state.
func (s *Server) restore(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	var dump store.Dump
	if err := json.NewDecoder(r.Body).Decode(&dump); err != nil {
		badRequest(w)
		return
	}
	if err := s.store.Import(&dump); err != nil {
		if errors.Is(err, store.ErrUnsupportedDump) {
			badRequest(w)
			return
		}
		serverError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
