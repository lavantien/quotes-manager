// Package server serves the quotes-manager web UI: an HTMX-driven list of
// draggable quote blocks backed by a store.Store.
package server

import (
	"html/template"
	"log"
	"net/http"

	"github.com/lavantien/quotes-manager/internal/store"
)

// Server is the HTTP server. It renders server-side templates and applies every
// mutation to the store, returning the freshly rendered HTML fragment so the UI
// stays a faithful projection of the database.
type Server struct {
	store store.Store
	tmpl  *template.Template
	mux   *http.ServeMux
}

// New builds a Server backed by the given store.
func New(s store.Store) *Server {
	srv := &Server{store: s, tmpl: mustTemplates()}
	srv.mux = srv.routes()
	return srv
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() *http.ServeMux {
	m := http.NewServeMux()
	m.HandleFunc("GET /", s.index)
	m.HandleFunc("GET /quotes", s.listFragment)
	m.HandleFunc("GET /quotes/new", s.newForm)
	m.HandleFunc("POST /quotes", s.create)
	m.HandleFunc("GET /quotes/{id}/edit", s.editForm)
	m.HandleFunc("POST /quotes/{id}", s.update)
	m.HandleFunc("DELETE /quotes/{id}", s.delete)
	m.HandleFunc("POST /quotes/delete", s.bulkDelete)
	m.HandleFunc("GET /quotes/{id}/copy", s.copyOne)
	m.HandleFunc("GET /export.txt", s.exportAll)
	m.HandleFunc("GET /collections/{cid}", s.collection)
	m.HandleFunc("POST /collections", s.createCollection)
	m.HandleFunc("POST /collections/{cid}/items", s.addCollectionItems)
	m.HandleFunc("DELETE /collections/{cid}", s.deleteCollection)
	m.HandleFunc("POST /collections/{cid}/reorder", s.collectionReorder)
	m.HandleFunc("GET /collections/{cid}/export.txt", s.collectionExport)
	m.HandleFunc("GET /sidebar", s.sidebar)
	m.HandleFunc("GET /categories/{ctid}", s.category)
	m.HandleFunc("POST /categories", s.createCategory)
	m.HandleFunc("POST /categories/{ctid}/rename", s.renameCategory)
	m.HandleFunc("DELETE /categories/{ctid}", s.deleteCategory)
	m.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS()))))
	return m
}

// render writes a template as UTF-8 HTML.
func (s *Server) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("render %q: %v", name, err)
	}
}
