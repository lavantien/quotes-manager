package server

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/lavantien/quotes-manager/internal/quote"
	"github.com/lavantien/quotes-manager/internal/search"
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
	Duplicates  []duplicateGroup             // near-duplicate groups (>= 2 members each)
	TotalQuotes int                          // size of the whole library (home count)
	ActiveCatID int64
	ActiveColID int64
}

// duplicateGroup is one cluster of near-duplicate quotes, for the Duplicates
// section of the left rail. Representative is the first (shortest) member and
// is the jump target; Label is its text id, falling back to a body excerpt.
type duplicateGroup struct {
	Representative int64
	Label          string
	Count          int
}

// rootPane is the editable left column: every quote (home) or a category's quotes.
type rootPane struct {
	Quotes     []store.Quote
	Count      int
	Title      string
	IsCategory bool
	CategoryID int64
	ExportURL  string
	Query      string   // raw ?rq= search, echoed into the input's value
	Terms      []string // parsed Query, used by the display func to highlight
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
	Query     string   // raw ?cq= search, echoed into the input's value
	Terms     []string // parsed Query, used by the display func to highlight
	Searching bool     // true when Terms is non-empty (disables reorder/gaps)
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
// rendering, so both chip rows show without an N+1 lookup per block. Terms drives
// highlight; Drivable switches off the collection block's drag affordance while a
// collection search is active (reordering a filtered subset is ambiguous).
type quoteView struct {
	Quote     store.Quote
	Cats      []store.Category
	Cols      []store.Collection
	Terms     []string
	Draggable bool
}

// collectionLabel returns a collection's display name: its stored Name, or the
// autonumbered fallback "Col {id}" when it has none.
func collectionLabel(c store.Collection) string {
	if c.Name != "" {
		return c.Name
	}
	return fmt.Sprintf("Col %d", c.ID)
}

// buildPageData loads the rails, both membership maps, and the root/collection
// panes for the given active category/collection and search queries. An unknown
// active id clears that pane (treated as inactive) rather than erroring, so a
// stale query param never 404s the whole page. rq/cq filter the root/collection
// panes respectively; empty queries are no-ops.
func (s *Server) buildPageData(catID, colID int64, rq, cq string) (pageData, error) {
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

	root, err := s.buildRootPane(catID, rq)
	if err != nil {
		return data, err
	}
	data.Root = root

	dups, total, err := s.buildDuplicates()
	if err != nil {
		return data, err
	}
	data.Duplicates, data.TotalQuotes = dups, total

	if colID > 0 {
		c, err := s.store.GetCollection(colID)
		switch {
		case err == nil:
			qs, qerr := s.store.CollectionQuotes(colID)
			if qerr != nil {
				return data, qerr
			}
			cTerms := search.Terms(cq)
			qs = search.Filter(qs, cTerms)
			data.Collection = collectionPane{
				Active:    true,
				ID:        colID,
				Name:      collectionLabel(c),
				Count:     len(qs),
				Quotes:    qs,
				ExportURL: fmt.Sprintf("/collections/%d/export.txt", colID),
				Query:     cq,
				Terms:     cTerms,
				Searching: len(cTerms) > 0,
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
// home, ordered by char_count) narrowed by the root search query rq.
func (s *Server) buildRootPane(catID int64, rq string) (rootPane, error) {
	rTerms := search.Terms(rq)
	if catID <= 0 {
		qs, err := s.store.List()
		if err != nil {
			return rootPane{}, err
		}
		qs = search.Filter(qs, rTerms)
		return rootPane{Quotes: qs, Count: len(qs), Title: "Quotes", ExportURL: "/export.txt", Query: rq, Terms: rTerms}, nil
	}
	c, err := s.store.GetCategory(catID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return rootPane{Quotes: nil, Count: 0, Title: "Quotes", ExportURL: "/export.txt", Query: rq, Terms: rTerms}, nil
		}
		return rootPane{}, err
	}
	qs, err := s.store.CategoryQuotes(catID)
	if err != nil {
		return rootPane{}, err
	}
	qs = search.Filter(qs, rTerms)
	return rootPane{
		Quotes:     qs,
		Count:      len(qs),
		Title:      fmt.Sprintf("#%s", c.Name),
		IsCategory: true,
		CategoryID: catID,
		ExportURL:  fmt.Sprintf("/categories/%d/export.txt", catID),
		Query:      rq,
		Terms:      rTerms,
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
	dups, total, err := s.buildDuplicates()
	if err != nil {
		return pageData{}, err
	}
	return pageData{
		Categories:  cats,
		Collections: cols,
		Duplicates:  dups,
		TotalQuotes: total,
		ActiveCatID: catID,
		ActiveColID: colID,
	}, nil
}

// buildDuplicates loads the whole library and groups near-duplicate quotes by
// word-level Jaccard similarity. It returns the duplicate groups (each >= 2
// members) plus the total library size. An empty groups slice means nothing is
// duplicated.
func (s *Server) buildDuplicates() ([]duplicateGroup, int, error) {
	qs, err := s.store.List()
	if err != nil {
		return nil, 0, err
	}
	items := make([]quote.DupItem, len(qs))
	byID := make(map[int64]store.Quote, len(qs))
	for i, q := range qs {
		items[i] = quote.DupItem{ID: q.ID, Text: q.BodyText}
		byID[q.ID] = q
	}
	groups := quote.GroupDuplicates(items, quote.DefaultDuplicateThreshold)
	out := make([]duplicateGroup, 0, len(groups))
	for _, ids := range groups {
		rep := ids[0]
		label := byID[rep].SuttaID
		if label == "" {
			label = bodyExcerpt(byID[rep].BodyText)
		}
		out = append(out, duplicateGroup{Representative: rep, Label: label, Count: len(ids)})
	}
	return out, len(qs), nil
}

// bodyExcerpt returns a short, single-line preview of a quote's body for use as
// a fallback label when a quote has no text id. It keeps the first runeRunes of
// the first passage line and trails with an ellipsis when truncated.
const excerptRunes = 24

func bodyExcerpt(text string) string {
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		text = text[:i]
	}
	text = strings.TrimSpace(text)
	r := []rune(text)
	if len(r) <= excerptRunes {
		return text
	}
	return string(r[:excerptRunes]) + "…"
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
