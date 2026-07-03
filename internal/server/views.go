package server

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/lavantien/quotes-manager/internal/store"
)

// pageData drives the full dual-pane page and every zone/rail fragment. The root
// column (home or a category filter) and the collection column (the active
// collection, or a placeholder) render side by side; two rails flank them.
type pageData struct {
	Root        rootPane
	Collection  collectionPane
	Categories  []store.Category
	Collections []store.Collection
	CatMap      map[int64][]store.Category   // quote_id -> its categories
	ColMap      map[int64][]store.Collection // quote_id -> its collections
	ActiveCatID int64
	ActiveColID int64
}

// rootPane is the editable left column: every quote (home) or a category's quotes.
type rootPane struct {
	Quotes     []store.Quote
	Count      int
	Title      string
	IsCategory bool
	CategoryID int64
	ExportURL  string
}

// collectionPane is the read-only right column: the active collection's quotes
// (with insert-gap affordances) or, when Active is false, an empty placeholder.
type collectionPane struct {
	Active    bool
	ID        int64
	Name      string
	Count     int
	Quotes    []store.Quote
	ExportURL string
}

// formData drives the 3-field quote create/edit form.
type formData struct {
	ID          int64
	Content     string
	Attribution string
	TextID      string
	Action      string
	SubmitLabel string
}

// chipsData drives a quote's category chip row.
type chipsData struct {
	ID         int64
	Categories []store.Category
}

// colChipsData drives a quote's collection-membership chip row.
type colChipsData struct {
	ID          int64
	Collections []store.Collection
}

// editorData drives the inline category checkbox editor for a quote.
type editorData struct {
	ID    int64
	Items []categoryItem
}

type categoryItem struct {
	Category store.Category
	Checked  bool
}

// quoteView bundles a quote with its categories and collections for block
// rendering, so both chip rows show without an N+1 lookup per block.
type quoteView struct {
	Quote store.Quote
	Cats  []store.Category
	Cols  []store.Collection
}

// collectionLabel returns a collection's display name: its stored Name, or the
// autonumbered fallback "Collection {id}" when it has none.
func collectionLabel(c store.Collection) string {
	if c.Name != "" {
		return c.Name
	}
	return fmt.Sprintf("Collection %d", c.ID)
}

// buildPageData loads the rails, both membership maps, and the root/collection
// panes for the given active category/collection. An unknown active id clears
// that pane (treated as inactive) rather than erroring, so a stale query param
// never 404s the whole page.
func (s *Server) buildPageData(catID, colID int64) (pageData, error) {
	data := pageData{ActiveCatID: catID, ActiveColID: colID}

	cats, err := s.store.ListCategories()
	if err != nil {
		return data, err
	}
	cols, err := s.store.ListCollections()
	if err != nil {
		return data, err
	}
	catMap, err := s.store.QuoteCategoryMap()
	if err != nil {
		return data, err
	}
	colMap, err := s.store.QuoteCollectionMap()
	if err != nil {
		return data, err
	}
	data.Categories, data.Collections, data.CatMap, data.ColMap = cats, cols, catMap, colMap

	root, err := s.buildRootPane(catID)
	if err != nil {
		return data, err
	}
	data.Root = root

	if colID > 0 {
		c, err := s.store.GetCollection(colID)
		switch {
		case err == nil:
			qs, qerr := s.store.CollectionQuotes(colID)
			if qerr != nil {
				return data, qerr
			}
			data.Collection = collectionPane{
				Active:    true,
				ID:        colID,
				Name:      collectionLabel(c),
				Count:     c.Count,
				Quotes:    qs,
				ExportURL: fmt.Sprintf("/collections/%d/export.txt", colID),
			}
		case errors.Is(err, store.ErrNotFound):
			data.ActiveColID = 0
		default:
			return data, err
		}
	}
	return data, nil
}

// buildRootPane loads the root column quotes for the given category filter (0 =
// home, ordered by char_count).
func (s *Server) buildRootPane(catID int64) (rootPane, error) {
	if catID <= 0 {
		qs, err := s.store.List()
		if err != nil {
			return rootPane{}, err
		}
		return rootPane{Quotes: qs, Count: len(qs), Title: "Quotes", ExportURL: "/export.txt"}, nil
	}
	c, err := s.store.GetCategory(catID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return rootPane{Quotes: nil, Count: 0, Title: "Quotes", ExportURL: "/export.txt"}, nil
		}
		return rootPane{}, err
	}
	qs, err := s.store.CategoryQuotes(catID)
	if err != nil {
		return rootPane{}, err
	}
	return rootPane{
		Quotes:     qs,
		Count:      len(qs),
		Title:      fmt.Sprintf("#%s", c.Name),
		IsCategory: true,
		CategoryID: catID,
		ExportURL:  fmt.Sprintf("/categories/%d/export.txt", catID),
	}, nil
}

// railData loads just the rails (categories + collections) plus the active
// cat/col, enough to render either rail fragment without reloading the panes.
func (s *Server) railData(catID, colID int64) (pageData, error) {
	cats, err := s.store.ListCategories()
	if err != nil {
		return pageData{}, err
	}
	cols, err := s.store.ListCollections()
	if err != nil {
		return pageData{}, err
	}
	return pageData{
		Categories:  cats,
		Collections: cols,
		ActiveCatID: catID,
		ActiveColID: colID,
	}, nil
}

// exec executes one template into w without touching headers, so a handler can
// compose a fragment response from several templates (a primary target plus
// out-of-band swaps) that may each take a different data object. The caller sets
// the Content-Type once before the first exec.
func (s *Server) exec(w http.ResponseWriter, name string, data any) {
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("render %q: %v", name, err)
	}
}
