package server_test

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/lavantien/quotes-manager/internal/quote"
	"github.com/lavantien/quotes-manager/internal/store"
)

type fakeStore struct {
	quotes      []store.Quote
	nextID      int64
	collections []store.Collection
	items       map[int64][]int64
	categories  []store.Category
	tags        map[int64][]int64
}

func newFake(quotes ...store.Quote) *fakeStore {
	maxID := int64(0)
	for _, q := range quotes {
		if q.ID > maxID {
			maxID = q.ID
		}
	}
	return &fakeStore{
		quotes: append([]store.Quote{}, quotes...),
		nextID: maxID,
		items:  map[int64][]int64{},
		tags:   map[int64][]int64{},
	}
}

func (f *fakeStore) List() ([]store.Quote, error) {
	out := make([]store.Quote, len(f.quotes))
	copy(out, f.quotes)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CharCount != out[j].CharCount {
			return out[i].CharCount < out[j].CharCount
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (f *fakeStore) Get(id int64) (store.Quote, error) {
	for _, q := range f.quotes {
		if q.ID == id {
			return q, nil
		}
	}
	return store.Quote{}, store.ErrNotFound
}

func (f *fakeStore) Create(q *quote.Quote) (int64, error) {
	f.nextID++
	row := store.Quote{
		ID: f.nextID, SuttaID: q.SuttaID, Citation: q.Citation,
		BodyMD: q.BodyMD(), BodyText: q.BodyText(), LineCount: q.LineCount(),
		CharCount: q.CharCount(), Sources: append([]string(nil), q.Sources...),
	}
	f.quotes = append(f.quotes, row)
	return row.ID, nil
}

func (f *fakeStore) Update(id int64, q *quote.Quote) error {
	for i := range f.quotes {
		if f.quotes[i].ID == id {
			f.quotes[i].SuttaID = q.SuttaID
			f.quotes[i].Citation = q.Citation
			f.quotes[i].BodyMD = q.BodyMD()
			f.quotes[i].BodyText = q.BodyText()
			f.quotes[i].LineCount = q.LineCount()
			f.quotes[i].CharCount = q.CharCount()
			return nil
		}
	}
	return store.ErrNotFound
}

func (f *fakeStore) Delete(id int64) error {
	for i, q := range f.quotes {
		if q.ID == id {
			f.quotes = append(f.quotes[:i], f.quotes[i+1:]...)
			return nil
		}
	}
	return store.ErrNotFound
}

func (f *fakeStore) DeleteMany(ids []int64) error {
	keep := f.quotes[:0]
	drop := make(map[int64]bool)
	for _, id := range ids {
		drop[id] = true
	}
	for _, q := range f.quotes {
		if !drop[q.ID] {
			keep = append(keep, q)
		}
	}
	f.quotes = keep
	return nil
}

func (f *fakeStore) MergeQuotes(keep int64, merge []int64) error {
	if !f.has(keep) {
		return store.ErrNotFound
	}
	for _, id := range merge {
		if id == keep {
			continue
		}
		if !f.has(id) {
			return store.ErrNotFound
		}
	}
	drop := make(map[int64]bool, len(merge))
	for _, id := range merge {
		drop[id] = true
	}
	keepList := f.quotes[:0]
	for _, q := range f.quotes {
		if !drop[q.ID] {
			keepList = append(keepList, q)
		}
	}
	f.quotes = keepList
	return nil
}

func (f *fakeStore) has(id int64) bool {
	for _, q := range f.quotes {
		if q.ID == id {
			return true
		}
	}
	return false
}

func (f *fakeStore) Export() (*store.Dump, error) {
	return &store.Dump{Version: store.DumpVersion, Quotes: append([]store.Quote{}, f.quotes...)}, nil
}

func (f *fakeStore) Import(d *store.Dump) error {
	if d == nil || d.Version != store.DumpVersion {
		return store.ErrUnsupportedDump
	}
	f.quotes = append([]store.Quote{}, d.Quotes...)
	return nil
}

func (f *fakeStore) ListCollections() ([]store.Collection, error) {
	return append([]store.Collection{}, f.collections...), nil
}

func (f *fakeStore) CreateCollection(quoteIDs []int64) (int64, error) {
	var cid int64 = 1
	if n := len(f.collections); n > 0 {
		cid = f.collections[n-1].ID + 1
	}
	f.collections = append(f.collections, store.Collection{ID: cid, Count: len(quoteIDs)})
	f.items[cid] = append([]int64{}, quoteIDs...)
	return cid, nil
}

func (f *fakeStore) GetCollection(id int64) (store.Collection, error) {
	for _, c := range f.collections {
		if c.ID == id {
			return c, nil
		}
	}
	return store.Collection{}, store.ErrNotFound
}

func (f *fakeStore) CollectionQuotes(id int64) ([]store.Quote, error) {
	var out []store.Quote
	for _, qid := range f.items[id] {
		for _, q := range f.quotes {
			if q.ID == qid {
				out = append(out, q)
			}
		}
	}
	return out, nil
}

func (f *fakeStore) DeleteCollection(id int64) error {
	for i, c := range f.collections {
		if c.ID == id {
			f.collections = append(f.collections[:i], f.collections[i+1:]...)
			delete(f.items, id)
			return nil
		}
	}
	return store.ErrNotFound
}

func (f *fakeStore) ReorderCollection(cid int64, orderedQuoteIDs []int64) error {
	members, ok := f.items[cid]
	if !ok {
		return store.ErrNotFound
	}
	belong := make(map[int64]bool, len(members))
	for _, qid := range members {
		belong[qid] = true
	}
	for _, qid := range orderedQuoteIDs {
		if !belong[qid] {
			return store.ErrNotFound
		}
	}
	f.items[cid] = append([]int64{}, orderedQuoteIDs...)
	return nil
}

func (f *fakeStore) AddToCollection(cid int64, quoteIDs []int64) error {
	members, ok := f.items[cid]
	if !ok {
		return store.ErrNotFound
	}
	belong := make(map[int64]bool, len(members))
	for _, qid := range members {
		belong[qid] = true
	}
	seen := make(map[int64]bool)
	var add []int64
	for _, qid := range quoteIDs {
		if qid <= 0 || seen[qid] || belong[qid] {
			continue
		}
		seen[qid] = true
		add = append(add, qid)
	}
	f.items[cid] = append(add, members...)
	for i := range f.collections {
		if f.collections[i].ID == cid {
			f.collections[i].Count = len(f.items[cid])
		}
	}
	return nil
}

func (f *fakeStore) RenameCollection(id int64, name string) error {
	for i := range f.collections {
		if f.collections[i].ID == id {
			f.collections[i].Name = name
			return nil
		}
	}
	return store.ErrNotFound
}

func (f *fakeStore) InsertAtCollection(cid int64, quoteIDs []int64, pos int) error {
	members, ok := f.items[cid]
	if !ok {
		return store.ErrNotFound
	}
	belong := make(map[int64]bool, len(members))
	for _, qid := range members {
		belong[qid] = true
	}
	seen := make(map[int64]bool)
	var add []int64
	for _, qid := range quoteIDs {
		if qid <= 0 || seen[qid] || belong[qid] {
			continue
		}
		seen[qid] = true
		add = append(add, qid)
	}
	if pos < 1 {
		pos = 1
	} else if pos > len(members)+1 {
		pos = len(members) + 1
	}
	out := make([]int64, 0, len(members)+len(add))
	out = append(out, members[:pos-1]...)
	out = append(out, add...)
	out = append(out, members[pos-1:]...)
	f.items[cid] = out
	for i := range f.collections {
		if f.collections[i].ID == cid {
			f.collections[i].Count = len(out)
		}
	}
	return nil
}

func (f *fakeStore) QuoteCollectionMap() (map[int64][]store.Collection, error) {
	out := make(map[int64][]store.Collection)
	for _, c := range f.collections {
		for _, qid := range f.items[c.ID] {
			out[qid] = append(out[qid], c)
		}
	}
	return out, nil
}

func (f *fakeStore) ListCategories() ([]store.Category, error) {
	out := make([]store.Category, 0, len(f.categories))
	for _, c := range f.categories {
		out = append(out, store.Category{ID: c.ID, Name: c.Name, Count: f.catCount(c.ID)})
	}
	sort.SliceStable(out, func(i, j int) bool {
		li, lj := strings.ToLower(out[i].Name), strings.ToLower(out[j].Name)
		if li != lj {
			return li < lj
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (f *fakeStore) CreateCategory(name string) (int64, error) {
	for _, c := range f.categories {
		if strings.EqualFold(c.Name, name) {
			return 0, fmt.Errorf("%w: category %q", store.ErrDuplicate, name)
		}
	}
	var id int64 = 1
	if n := len(f.categories); n > 0 {
		id = f.categories[n-1].ID + 1
	}
	f.categories = append(f.categories, store.Category{ID: id, Name: name})
	return id, nil
}

func (f *fakeStore) GetCategory(id int64) (store.Category, error) {
	for _, c := range f.categories {
		if c.ID == id {
			return store.Category{ID: c.ID, Name: c.Name, Count: f.catCount(c.ID)}, nil
		}
	}
	return store.Category{}, store.ErrNotFound
}

func (f *fakeStore) RenameCategory(id int64, name string) error {
	for i, c := range f.categories {
		if c.ID == id {
			for _, other := range f.categories {
				if other.ID != id && strings.EqualFold(other.Name, name) {
					return fmt.Errorf("%w: category %q", store.ErrDuplicate, name)
				}
			}
			f.categories[i].Name = name
			return nil
		}
	}
	return store.ErrNotFound
}

func (f *fakeStore) DeleteCategory(id int64) error {
	for i, c := range f.categories {
		if c.ID == id {
			f.categories = append(f.categories[:i], f.categories[i+1:]...)
			for qid, cids := range f.tags {
				f.tags[qid] = removeID(cids, id)
			}
			return nil
		}
	}
	return store.ErrNotFound
}

func (f *fakeStore) CategoryQuotes(id int64) ([]store.Quote, error) {
	var out []store.Quote
	for _, q := range f.quotes {
		if containsID(f.tags[q.ID], id) {
			out = append(out, q)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CharCount != out[j].CharCount {
			return out[i].CharCount < out[j].CharCount
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (f *fakeStore) SetQuoteCategories(quoteID int64, categoryIDs []int64) error {
	found := false
	for _, q := range f.quotes {
		if q.ID == quoteID {
			found = true
			break
		}
	}
	if !found {
		return store.ErrNotFound
	}
	known := make(map[int64]bool, len(f.categories))
	for _, c := range f.categories {
		known[c.ID] = true
	}
	seen := make(map[int64]bool)
	var ids []int64
	for _, cid := range categoryIDs {
		if cid <= 0 || seen[cid] {
			continue
		}
		if !known[cid] {
			return store.ErrNotFound
		}
		seen[cid] = true
		ids = append(ids, cid)
	}
	f.tags[quoteID] = ids
	return nil
}

func (f *fakeStore) QuoteCategoryMap() (map[int64][]store.Category, error) {
	out := make(map[int64][]store.Category)
	for qid, cids := range f.tags {
		var cs []store.Category
		for _, cid := range cids {
			for _, c := range f.categories {
				if c.ID == cid {
					cs = append(cs, store.Category{ID: c.ID, Name: c.Name})
				}
			}
		}
		sort.SliceStable(cs, func(i, j int) bool {
			return strings.ToLower(cs[i].Name) < strings.ToLower(cs[j].Name)
		})
		if len(cs) > 0 {
			out[qid] = cs
		}
	}
	return out, nil
}

func (f *fakeStore) catCount(cid int64) int {
	n := 0
	for _, cids := range f.tags {
		if containsID(cids, cid) {
			n++
		}
	}
	return n
}

func containsID(xs []int64, x int64) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func removeID(xs []int64, x int64) []int64 {
	out := xs[:0]
	for _, v := range xs {
		if v != x {
			out = append(out, v)
		}
	}
	return out
}

func (f *fakeStore) Close() error { return nil }

type failingStore struct{}

var errStoreFails = errors.New("store unavailable")

func (failingStore) List() ([]store.Quote, error)                 { return nil, errStoreFails }
func (failingStore) Get(int64) (store.Quote, error)               { return store.Quote{}, errStoreFails }
func (failingStore) Create(*quote.Quote) (int64, error)           { return 0, errStoreFails }
func (failingStore) Update(int64, *quote.Quote) error             { return errStoreFails }
func (failingStore) Delete(int64) error                           { return errStoreFails }
func (failingStore) DeleteMany([]int64) error                     { return errStoreFails }
func (failingStore) MergeQuotes(int64, []int64) error             { return errStoreFails }
func (failingStore) Export() (*store.Dump, error)                 { return nil, errStoreFails }
func (failingStore) Import(*store.Dump) error                     { return errStoreFails }
func (failingStore) ListCollections() ([]store.Collection, error) { return nil, errStoreFails }
func (failingStore) CreateCollection([]int64) (int64, error)      { return 0, errStoreFails }
func (failingStore) AddToCollection(int64, []int64) error         { return errStoreFails }
func (failingStore) InsertAtCollection(int64, []int64, int) error { return errStoreFails }
func (failingStore) GetCollection(int64) (store.Collection, error) {
	return store.Collection{}, errStoreFails
}
func (failingStore) RenameCollection(int64, string) error          { return errStoreFails }
func (failingStore) CollectionQuotes(int64) ([]store.Quote, error) { return nil, errStoreFails }
func (failingStore) QuoteCollectionMap() (map[int64][]store.Collection, error) {
	return nil, errStoreFails
}
func (failingStore) ReorderCollection(int64, []int64) error    { return errStoreFails }
func (failingStore) DeleteCollection(int64) error              { return errStoreFails }
func (failingStore) ListCategories() ([]store.Category, error) { return nil, errStoreFails }
func (failingStore) CreateCategory(string) (int64, error)      { return 0, errStoreFails }
func (failingStore) GetCategory(int64) (store.Category, error) {
	return store.Category{}, errStoreFails
}
func (failingStore) RenameCategory(int64, string) error                    { return errStoreFails }
func (failingStore) DeleteCategory(int64) error                            { return errStoreFails }
func (failingStore) CategoryQuotes(int64) ([]store.Quote, error)           { return nil, errStoreFails }
func (failingStore) SetQuoteCategories(int64, []int64) error               { return errStoreFails }
func (failingStore) QuoteCategoryMap() (map[int64][]store.Category, error) { return nil, errStoreFails }
func (failingStore) Close() error                                          { return nil }

type failList struct{ *fakeStore }

func (failList) List() ([]store.Quote, error) { return nil, errStoreFails }

type failCatMap struct{ *fakeStore }

func (failCatMap) QuoteCategoryMap() (map[int64][]store.Category, error) {
	return nil, errStoreFails
}

type failColMap struct{ *fakeStore }

func (failColMap) QuoteCollectionMap() (map[int64][]store.Collection, error) {
	return nil, errStoreFails
}

type failListCats struct{ *fakeStore }

func (failListCats) ListCategories() ([]store.Category, error) { return nil, errStoreFails }

type failListCols struct{ *fakeStore }

func (failListCols) ListCollections() ([]store.Collection, error) { return nil, errStoreFails }

type failCatQuotes struct{ *fakeStore }

func (failCatQuotes) CategoryQuotes(int64) ([]store.Quote, error) { return nil, errStoreFails }

type failCreateCategory struct{ *fakeStore }

func (failCreateCategory) CreateCategory(string) (int64, error) { return 0, errStoreFails }

type failSetQuoteCategories struct{ *fakeStore }

func (failSetQuoteCategories) SetQuoteCategories(int64, []int64) error { return errStoreFails }
