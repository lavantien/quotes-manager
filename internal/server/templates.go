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
		"display": func(q store.Quote) template.HTML {
			return quote.New(q.SuttaID, q.Citation, splitPassages(q.BodyText)).DisplayHTML()
		},
		"withCats": func(q store.Quote, m map[int64][]store.Category) quoteView {
			return quoteView{Quote: q, Cats: m[q.ID]}
		},
		"chipsFor": func(id int64, cats []store.Category) chipsData {
			return chipsData{ID: id, Categories: cats}
		},
	})
	return template.Must(tmpl.ParseFS(web.Templates,
		"templates/layout.html",
		"templates/index.html",
		"templates/quote_list.html",
		"templates/quote_block.html",
		"templates/quote_block_ro.html",
		"templates/quote_form.html",
		"templates/sidebar.html",
		"templates/quote_chips.html",
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
