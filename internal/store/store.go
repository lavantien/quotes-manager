// Package store persists quotes in SQLite.
package store

import (
	"errors"

	"github.com/lavantien/quotes-manager/internal/quote"
)

// Quote is a persisted quote row.
type Quote struct {
	ID        int64
	SuttaID   string
	Citation  string
	BodyMD    string
	BodyText  string
	LineCount int
	CharCount int
	Sources   []string
}

// Collection is a numbered, read-only subset of quotes curated from home.
type Collection struct {
	ID    int64
	Count int
}

// Category is a named tag applied to one or more quotes. Count is the number of
// quotes tagged with it (populated by ListCategories for sidebar rendering).
type Category struct {
	ID    int64
	Name  string
	Count int
}

// ErrNotFound is returned when a quote id does not exist.
var ErrNotFound = errors.New("quote not found")

// ErrDuplicate is returned when a uniqueness constraint (e.g. a category name)
// is violated.
var ErrDuplicate = errors.New("duplicate")

// Store is the persistence interface for quotes.
type Store interface {
	List() ([]Quote, error)                                    // ordered by char_count, then id
	Get(id int64) (Quote, error)                               // ErrNotFound if missing
	Create(q *quote.Quote) (int64, error)                      // char_count is the rune-count sort key
	Update(id int64, q *quote.Quote) error                     // re-derives body/count fields
	Delete(id int64) error                                     // ErrNotFound if missing
	DeleteMany(ids []int64) error                              // empty slice is a no-op
	ListCollections() ([]Collection, error)                    // ordered by id
	CreateCollection(quoteIDs []int64) (int64, error)          // new numbered collection; returns its id
	AddToCollection(id int64, quoteIDs []int64) error          // prepends to top; ErrNotFound on unknown cid
	GetCollection(id int64) (Collection, error)                // ErrNotFound if missing
	CollectionQuotes(id int64) ([]Quote, error)                // ordered by collection position
	ReorderCollection(id int64, orderedQuoteIDs []int64) error // single tx; ErrNotFound on bad cid / non-member
	DeleteCollection(id int64) error                           // ErrNotFound if missing
	Close() error
}
