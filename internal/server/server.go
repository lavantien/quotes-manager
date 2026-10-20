package server

import (
	"bytes"
	"html/template"
	"log"
	"net/http"

	canonweb "github.com/lavantien/quotes-manager/internal/canon/web"
	"github.com/lavantien/quotes-manager/internal/store"
)

type Server struct {
	store        store.Store
	tmpl         *template.Template
	mux          *http.ServeMux
	defaultLimit int
	maxLimit     int
}

func New(s store.Store, opts ...Option) *Server {
	def, max := limitsFromConfig()
	srv := &Server{store: s, tmpl: mustTemplates(), defaultLimit: def, maxLimit: max}
	for _, o := range opts {
		o(srv)
	}
	srv.mux = srv.routes()
	return srv
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() *http.ServeMux {
	m := http.NewServeMux()
	s.RegisterRoutes(m)
	return m
}

func (s *Server) RegisterRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /", s.rootIndex)
	m.HandleFunc("GET /quotes", s.listFragment)
	m.HandleFunc("GET /quotes/new", s.newForm)
	m.HandleFunc("GET /quotes/import/form", s.importQuotesForm)
	m.HandleFunc("POST /quotes/import", s.importQuotes)
	m.HandleFunc("POST /quotes", s.create)
	m.HandleFunc("GET /quotes/{id}/edit", s.editForm)
	m.HandleFunc("POST /quotes/{id}", s.update)
	m.HandleFunc("DELETE /quotes/{id}", s.delete)
	m.HandleFunc("POST /quotes/delete", s.bulkDelete)
	m.HandleFunc("GET /quotes/{id}/copy", s.copyOne)
	m.HandleFunc("GET /quotes/{id}/categories", s.quoteChips)
	m.HandleFunc("GET /quotes/{id}/categories/edit", s.editQuoteCategories)
	m.HandleFunc("POST /quotes/{id}/categories", s.setQuoteCategories)
	m.HandleFunc("GET /export.txt", s.exportAll)
	m.HandleFunc("GET /backup.json", s.backup)
	m.HandleFunc("POST /restore", s.restore)
	m.HandleFunc("GET /collections/{cid}", s.collection)
	m.HandleFunc("POST /collections", s.createCollection)
	m.HandleFunc("POST /collections/{cid}/items", s.addCollectionItems)
	m.HandleFunc("POST /collections/{cid}/insert", s.insertCollectionItems)
	m.HandleFunc("POST /collections/{cid}/rename", s.renameCollection)
	m.HandleFunc("DELETE /collections/{cid}", s.deleteCollection)
	m.HandleFunc("POST /collections/{cid}/reorder", s.collectionReorder)
	m.HandleFunc("GET /collections/{cid}/export.txt", s.collectionExport)
	m.HandleFunc("POST /collections/{cid}/groups", s.createGroup)
	m.HandleFunc("POST /collections/{cid}/groups/{gid}/rename", s.renameGroup)
	m.HandleFunc("DELETE /collections/{cid}/groups/{gid}", s.deleteGroup)
	m.HandleFunc("POST /collections/{cid}/groups/{gid}/move", s.moveGroup)
	m.HandleFunc("POST /collections/{cid}/items/{id}/group", s.setQuoteGroup)
	m.HandleFunc("POST /collections/{cid}/suttas", s.addSutta)
	m.HandleFunc("DELETE /collections/{cid}/suttas/{muid}", s.removeSutta)
	m.HandleFunc("POST /collections/{cid}/suttas/{muid}/group", s.setSuttaGroup)
	m.HandleFunc("GET /ids.txt", s.corpusIDs)
	m.HandleFunc("GET /collections/{cid}/ids.txt", s.collectionIDs)
	m.HandleFunc("GET /api/v1/quotes", s.apiQuotesListHandler)
	m.HandleFunc("GET /api/v1/quotes/{id}", s.apiQuoteGetHandler)
	m.HandleFunc("GET /pane/root", s.rootPaneHandler)
	m.HandleFunc("GET /pane/collection", s.collectionPaneHandler)
	m.HandleFunc("GET /pane/check", s.checkPaneHandler)
	m.HandleFunc("POST /check", s.checkIdsHandler)
	m.HandleFunc("GET /search/root", s.searchRoot)
	m.HandleFunc("GET /search/collection", s.searchCollection)
	m.HandleFunc("GET /rail/left", s.leftRailHandler)
	m.HandleFunc("GET /rail/right", s.rightRailHandler)
	m.HandleFunc("POST /duplicates/{repID}/merge", s.mergeDuplicates)
	m.HandleFunc("GET /categories/{ctid}", s.category)
	m.HandleFunc("GET /categories/{ctid}/export.txt", s.categoryExport)
	m.HandleFunc("POST /categories", s.createCategory)
	m.HandleFunc("POST /categories/{ctid}/rename", s.renameCategory)
	m.HandleFunc("DELETE /categories/{ctid}", s.deleteCategory)
	m.Handle("GET /static/", cacheStatic(http.StripPrefix("/static/", http.FileServer(http.FS(staticFS())))))
}

func cacheStatic(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		h.ServeHTTP(w, r)
	})
}

func (s *Server) rootIndex(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/":
		s.index(w, r)
	case "/favicon.ico":
		w.WriteHeader(http.StatusNoContent)
	default:
		s.renderNotFound(w, r)
	}
}

func (s *Server) renderNotFound(w http.ResponseWriter, r *http.Request) {
	var buf bytes.Buffer
	page := canonweb.NotFoundPage{Page: canonweb.Page{Title: "not found"}, Path: r.URL.Path}
	if err := canonweb.Render(&buf, canonweb.PageNotFound, page); err != nil {
		serverError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write(buf.Bytes())
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("render %q: %v", name, err)
	}
}
