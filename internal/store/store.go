package store

import (
	"errors"
	"sort"

	"github.com/lavantien/quotes-manager/internal/quote"
)

type Quote struct {
	ID        int64    `json:"id"`
	SuttaID   string   `json:"sutta_id"`
	Citation  string   `json:"citation"`
	BodyMD    string   `json:"body_md"`
	BodyText  string   `json:"body_text"`
	LineCount int      `json:"line_count"`
	CharCount int      `json:"char_count"`
	Sources   []string `json:"sources"`
}

type Collection struct {
	ID    int64
	Name  string
	Count int
}

type Sutta struct {
	Muid    string
	Display string
	Link    string
	Length  int
}

type Group struct {
	ID    int64
	Name  string
	Count int
}

type SuttaItem struct {
	Muid     string
	Display  string
	Link     string
	Length   int
	Position int
	GroupID  int64
}

type QuoteItem struct {
	Quote
	GroupID int64
}

type Category struct {
	ID    int64
	Name  string
	Count int
}

var ErrNotFound = errors.New("quote not found")

func SortShortestFirst(items []SuttaItem) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if (a.Length > 0) != (b.Length > 0) {
			return a.Length > 0
		}
		if a.Length != b.Length {
			return a.Length < b.Length
		}
		return a.Position < b.Position
	})
}

var ErrDuplicate = errors.New("duplicate")

var ErrEmptyName = errors.New("empty name")

const MaxGroupNameChars = 100

var ErrNameTooLong = errors.New("name too long")

var ErrReorderMismatch = errors.New("reorder list does not match the collection")

var ErrUnsupportedDump = errors.New("unsupported dump version")

type Store interface {
	List() ([]Quote, error)
	Get(id int64) (Quote, error)
	Create(q *quote.Quote) (int64, error)
	Update(id int64, q *quote.Quote) error
	Delete(id int64) error
	DeleteMany(ids []int64) error
	MergeQuotes(keep int64, merge []int64) error
	ListCollections() ([]Collection, error)
	CreateCollection(quoteIDs []int64) (int64, error)
	AddToCollection(id int64, quoteIDs []int64) error
	InsertAtCollection(id int64, quoteIDs []int64, pos int) error
	GetCollection(id int64) (Collection, error)
	RenameCollection(id int64, name string) error
	CollectionQuotes(id int64) ([]Quote, error)
	QuoteCollectionMap() (map[int64][]Collection, error)
	ReorderCollection(id int64, orderedQuoteIDs []int64) error
	DeleteCollection(id int64) error
	ListSuttas() ([]Sutta, error)
	ListCollectionGroups(id int64) ([]Group, error)
	CreateCollectionGroup(id int64, name string) (int64, error)
	RenameCollectionGroup(id, groupID int64, name string) error
	DeleteCollectionGroup(id, groupID int64) error
	ReorderCollectionGroups(id int64, orderedGroupIDs []int64) error
	CollectionQuoteItems(id int64) ([]QuoteItem, error)
	CollectionSuttaItems(id int64) ([]SuttaItem, error)
	SetQuoteGroup(id, quoteID, groupID int64) error
	SetSuttaGroup(id int64, muid string, groupID int64) error
	AddSuttaToCollection(id int64, muid, display, link string) error
	RemoveSuttaFromCollection(id int64, muid string) error
	ReorderCollectionSuttas(id int64, orderedMuids []string) error
	SetSuttaLengths(lengths map[string]int) error
	ListCategories() ([]Category, error)
	CreateCategory(name string) (int64, error)
	GetCategory(id int64) (Category, error)
	RenameCategory(id int64, name string) error
	DeleteCategory(id int64) error
	CategoryQuotes(id int64) ([]Quote, error)
	SetQuoteCategories(quoteID int64, categoryIDs []int64) error
	QuoteCategoryMap() (map[int64][]Category, error)
	Export() (*Dump, error)
	Import(*Dump) error
	Close() error
}
