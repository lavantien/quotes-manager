// Package store persists quotes in SQLite.
package store

import (
	"errors"

	"github.com/lavantien/quotes-manager/internal/quote"
)

// Quote is a persisted quote row.
type Quote struct {
	ID        int64
	SortOrder int64
	SuttaID   string
	Citation  string
	BodyMD    string
	BodyText  string
	LineCount int
	CharCount int
	Sources   []string
}

// ErrNotFound is returned when a quote id does not exist.
var ErrNotFound = errors.New("quote not found")

// Store is the persistence interface for quotes.
type Store interface {
	List() ([]Quote, error)                // ordered by sort_order, then id
	Get(id int64) (Quote, error)           // ErrNotFound if missing
	Create(q *quote.Quote) (int64, error)  // assigns sort_order = max + 1
	Update(id int64, q *quote.Quote) error // re-derives body/count fields
	Delete(id int64) error                 // ErrNotFound if missing
	DeleteMany(ids []int64) error          // empty slice is a no-op
	Reorder(orderedIDs []int64) error      // single transaction; ErrNotFound on unknown id
	Close() error
}
