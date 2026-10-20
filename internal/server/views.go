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

type pageData struct {
	Root        rootPane
	Collection  collectionPane
	Categories  []store.Category
	Collections []store.Collection
	CatMap      map[int64][]store.Category
	ColMap      map[int64][]store.Collection
	Duplicates  []duplicateGroup
	TotalQuotes int
	ActiveCatID int64
	ActiveColID int64
	View        string
}

type duplicateGroup struct {
	Representative int64
	Label          string
	Count          int
}

type rootPane struct {
	Quotes     []store.Quote
	Count      int
	Title      string
	IsCategory bool
	CategoryID int64
	ExportURL  string
	Query      string
	Terms      []string
	FlashID    int64
}

type collectionPane struct {
	Active      bool
	ID          int64
	Name        string
	Count       int
	QuoteCount  int
	Quotes      []store.Quote
	Groups      []store.Group
	HasSections bool
	Sections    []groupSection
	ExportURL   string
	Query       string
	Terms       []string
	Searching   bool
}

type groupSection struct {
	Group  store.Group
	Quotes []quoteRow
	Suttas []store.SuttaItem
}

type quoteRow struct {
	Item store.QuoteItem
	Pos  int
}

type formData struct {
	ID          int64
	Content     string
	Attribution string
	TextID      string
	Action      string
	SubmitLabel string
	Categories  []store.Category
}

type chipsData struct {
	ID         int64
	Categories []store.Category
}

type colChipsData struct {
	ID          int64
	Collections []store.Collection
}

type editorData struct {
	ID    int64
	Items []categoryItem
}

type categoryItem struct {
	Category store.Category
	Checked  bool
}

type quoteView struct {
	Quote     store.Quote
	Cats      []store.Category
	Cols      []store.Collection
	Terms     []string
	Draggable bool
}

func collectionLabel(c store.Collection) string {
	if c.Name != "" {
		return c.Name
	}
	return fmt.Sprintf("Col %d", c.ID)
}

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
			groups, err := s.store.ListCollectionGroups(colID)
			if err != nil {
				return data, err
			}
			qItems, err := s.store.CollectionQuoteItems(colID)
			if err != nil {
				return data, err
			}
			sItems, err := s.store.CollectionSuttaItems(colID)
			if err != nil {
				return data, err
			}
			cQuery := search.Parse(cq)
			qItems = filterQuoteItems(qItems, cQuery)
			pane := collectionPane{
				Active:     true,
				ID:         colID,
				Name:       collectionLabel(c),
				Count:      len(qItems) + len(sItems),
				QuoteCount: len(qItems),
				ExportURL:  fmt.Sprintf("/collections/%d/export.txt", colID),
				Query:      cq,
				Terms:      search.HighlightTerms(cQuery),
				Searching:  !cQuery.IsZero(),
			}
			if len(groups) > 0 || len(sItems) > 0 {
				pane.Groups = groups
				pane.HasSections = true
				pane.Sections = buildGroupSections(groups, qItems, sItems)
			} else {
				pane.Quotes = quoteItemsToQuotes(qItems)
			}
			data.Collection = pane
		case errors.Is(err, store.ErrNotFound):
			data.ActiveColID = 0
		default:
			return data, err
		}
	}
	return data, nil
}

func filterQuoteItems(items []store.QuoteItem, query search.Query) []store.QuoteItem {
	qs := quoteItemsToQuotes(items)
	keep := make(map[int64]bool, len(qs))
	for _, q := range search.Filter(qs, query) {
		keep[q.ID] = true
	}
	out := make([]store.QuoteItem, 0, len(items))
	for _, it := range items {
		if keep[it.ID] {
			out = append(out, it)
		}
	}
	return out
}

func quoteItemsToQuotes(items []store.QuoteItem) []store.Quote {
	out := make([]store.Quote, len(items))
	for i, it := range items {
		out[i] = it.Quote
	}
	return out
}

func buildGroupSections(groups []store.Group, qItems []store.QuoteItem, sItems []store.SuttaItem) []groupSection {
	sections := make([]groupSection, len(groups))
	for i, g := range groups {
		sections[i] = groupSection{Group: g}
	}
	ungrouped := groupSection{Group: store.Group{Name: "Ungrouped"}}
	pos := 0
	for _, it := range qItems {
		pos++
		target := &ungrouped
		for i := range sections {
			if sections[i].Group.ID == it.GroupID {
				target = &sections[i]
				break
			}
		}
		target.Quotes = append(target.Quotes, quoteRow{Item: it, Pos: pos})
	}
	for _, it := range sItems {
		target := &ungrouped
		for i := range sections {
			if sections[i].Group.ID == it.GroupID {
				target = &sections[i]
				break
			}
		}
		target.Suttas = append(target.Suttas, it)
	}
	for i := range sections {
		store.SortShortestFirst(sections[i].Suttas)
	}
	store.SortShortestFirst(ungrouped.Suttas)
	ungrouped.Group.Count = len(ungrouped.Quotes) + len(ungrouped.Suttas)
	return append(sections, ungrouped)
}

func (s *Server) buildRootPane(catID int64, rq string) (rootPane, error) {
	rQuery := search.Parse(rq)
	if catID <= 0 {
		qs, err := s.store.List()
		if err != nil {
			return rootPane{}, err
		}
		qs = search.Filter(qs, rQuery)
		return rootPane{Quotes: qs, Count: len(qs), Title: "Quotes", ExportURL: "/export.txt", Query: rq, Terms: search.HighlightTerms(rQuery)}, nil
	}
	c, err := s.store.GetCategory(catID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return rootPane{Quotes: nil, Count: 0, Title: "Quotes", ExportURL: "/export.txt", Query: rq, Terms: search.HighlightTerms(rQuery)}, nil
		}
		return rootPane{}, err
	}
	qs, err := s.store.CategoryQuotes(catID)
	if err != nil {
		return rootPane{}, err
	}
	qs = search.Filter(qs, rQuery)
	return rootPane{
		Quotes:     qs,
		Count:      len(qs),
		Title:      fmt.Sprintf("#%s", c.Name),
		IsCategory: true,
		CategoryID: catID,
		ExportURL:  fmt.Sprintf("/categories/%d/export.txt", catID),
		Query:      rq,
		Terms:      search.HighlightTerms(rQuery),
	}, nil
}

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

func (s *Server) exec(w http.ResponseWriter, name string, data any) {
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("render %q: %v", name, err)
	}
}
