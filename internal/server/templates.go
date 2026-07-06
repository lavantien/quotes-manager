package server

import (
	"html/template"
	"io/fs"

	"github.com/lavantien/quotes-manager/internal/quote"
	"github.com/lavantien/quotes-manager/internal/store"
	"github.com/lavantien/quotes-manager/web"
)

// mustTemplates parses all templates once. The display func renders a quote in
// the on-screen format (italic passages, bolded suttacentral link).
func mustTemplates() *template.Template {
	tmpl := template.New("").Funcs(template.FuncMap{
		"display": func(q store.Quote, terms []string) template.HTML {
			return quote.New(q.SuttaID, q.Citation, splitPassages(q.BodyText)).DisplayHTMLWithTerms(terms)
		},
		"withView": func(q store.Quote, cm map[int64][]store.Category, colm map[int64][]store.Collection, terms []string, draggable bool) quoteView {
			return quoteView{Quote: q, Cats: cm[q.ID], Cols: colm[q.ID], Terms: terms, Draggable: draggable}
		},
		"chipsFor": func(id int64, cats []store.Category) chipsData {
			return chipsData{ID: id, Categories: cats}
		},
		"colChipsFor": func(id int64, cols []store.Collection) colChipsData {
			return colChipsData{ID: id, Collections: cols}
		},
		"colLabel": func(c store.Collection) string { return collectionLabel(c) },
		"add":      func(a, b int) int { return a + b },
	})
	return template.Must(tmpl.ParseFS(web.Templates,
		"templates/layout.html",
		"templates/rail_left.html",
		"templates/rail_right.html",
		"templates/root_zone.html",
		"templates/collection_zone.html",
		"templates/check_zone.html",
		"templates/check_results.html",
		"templates/collection_list.html",
		"templates/quote_list.html",
		"templates/quote_block.html",
		"templates/quote_block_ro.html",
		"templates/quote_form.html",
		"templates/quote_import_form.html",
		"templates/quote_chips.html",
		"templates/quote_collection_chips.html",
		"templates/quote_category_editor.html",
	))
}

// staticFS returns the embedded static assets rooted at web/static.
func staticFS() fs.FS {
	sub, err := fs.Sub(web.Static, "static")
	if err != nil {
		panic(err)
	}
	return sub
}
