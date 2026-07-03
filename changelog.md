# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.6.0] - 2026-07-03

### Added
- **Dual-pane workspace.** The UI is now four zones side by side: a left rail
  (Home + Categories), the root text column (home or a category filter), the
  collection column (the active collection), and a right rail (Collections). A
  thin, two-half header sits atop each text column showing only a name and a
  count. Selecting a category or collection swaps just that pane in place via
  htmx (with an out-of-band rail refresh); the URL carries `?cat=` / `?col=` for
  deep-linking.
- **Named, renameable collections.** Collections carry a `name` (default empty →
  rendered as "Collection {id}"); inline ✎ rename in the right rail commits via
  `POST /collections/{cid}/rename` (`store.RenameCollection`). Names are not
  unique. Pre-0.6.0 databases are migrated with an idempotent `ALTER TABLE` that
  adds `collections.name`.
- **Insert at a precise index.** Checking one or more root quotes reveals
  insert-gap affordances (a `+` marker) between every pair of collection blocks;
  clicking one inserts the selection at that 1-based position, shifting later
  items down (`store.InsertAtCollection`, `POST /collections/{cid}/insert`).
  Duplicates are skipped.
- **Collection membership on root blocks.** Each root quote now shows a second
  chip row for the collections it belongs to (`store.QuoteCollectionMap`),
  distinct from the category chips; clicking one activates that collection.
- A fresh database is seeded with one sample collection (the two shortest
  quotes) so the collection column and membership chips are non-empty out of the
  box and the README screenshot is illustrative.

### Changed
- The sidebar is split into two nav rails; the top bar is now a thin split strip
  (name + count only), with per-column toolbars (root: select-all / +New /
  bulk-delete / Copy all; collection: Copy all / Delete).
- Collection creation from a selection now makes the new collection active and
  swaps the collection zone in place instead of redirecting.
- `web/static` styles are split into `app.css` (typography/components) and
  `layout.css` (the four-zone grid, rails, zones, insert gaps).

## [0.5.0] - 2026-07-01

### Added
- **Categories.** Named, reusable tags for quotes — managed independently of
  collections. Create, rename, and delete them from the sidebar; names are
  unique (case-insensitive). Backed by the `categories` and `category_items`
  tables and `store.ListCategories`, `CreateCategory`, `GetCategory`,
  `RenameCategory`, `DeleteCategory`, `CategoryQuotes`, `SetQuoteCategories`,
  and `QuoteCategoryMap`.
- **Per-quote categories.** Each quote block shows its categories as chips and
  an inline ✎ editor (`GET /quotes/{id}/categories/edit`) that toggles any
  combination of categories and can create a new one inline; saving commits the
  full set via `POST /quotes/{id}/categories` (`store.SetQuoteCategories`).
- **Category view.** `GET /categories/{ctid}` filters the list to a category's
  quotes (in home order) with `GET /categories/{ctid}/export.txt` for Copy all —
  read-only, mirroring the collection view.
- **Sidebar layout.** The home page is now a sticky left sidebar (Collections +
  Categories, each with counts and inline manage affordances) beside the quote
  list. Deleting a category untaggs its quotes; deleting a quote clears its tags.

### Changed
- `store.Delete`/`DeleteMany` now cascade `category_items` alongside
  `collection_items` in the same transaction.
- A fresh database is seeded with three example categories (`suffering`,
  `renunciation`, `right view`) tagging the shortest quotes, so the sidebar and
  chip rows are non-empty out of the box.

## [0.4.0] - 2026-06-30

### Added
- **Choose a target collection.** The "Add to collection" control on home now
  includes a dropdown: add selected quotes to an **existing** collection (new
  items prepended on top; duplicates skipped) via `POST /collections/{cid}/items`,
  or pick **+ New collection** for the previous create flow. Backed by
  `store.AddToCollection`.
- **Block-count badge.** Each page (home and collection) shows its block count
  beside the title.
- **Coverage badge.** `make coverage` runs the suite with `-coverpkg=./...` and
  `cmd/coverage` (backed by `internal/coverbadge`) refreshes a shields.io
  test-coverage badge in the README between markers. The percentage is parsed
  from the real Go cover profile — it is never hand-set.
- **Home screenshot.** `make screenshot` runs `cmd/screenshot`, which serves the
  seeded app in-process and captures `docs/home.png` via chromedp (system Chrome
  or Edge; `QUOTES_BROWSER` overrides). The README embeds it at the top.

### Changed
- Home is now ordered by `char_count` (shortest-first). A newly added quote
  slots into its sorted place automatically — `List` orders by `char_count, id`
  and `create` re-renders the full list.

### Removed
- Home drag-reorder and the entire `sort_order` machinery (column + index,
  `Quote.SortOrder`, `store.Reorder`, the `POST /quotes/reorder` route, and the
  `internal/seed` sort_order migration). Collections still support manual
  drag-reorder. Legacy databases carrying a leftover `sort_order` column keep
  working — the column is simply ignored.

## [0.3.0] - 2026-06-29

### Added
- Numbered **collections**: select quotes on home and "Add to collection" to
  create a new numbered collection. Collections appear as a nav between the title
  and the action buttons.
- Collection views (`GET /collections/{cid}`) render the same block layout,
  copyable (copy-one and copy-all via `/collections/{cid}/export.txt`) and
  drag-to-reorder (`POST /collections/{cid}/reorder`), but read-only for content
  — no +New, edit, or delete — since home remains the sole source of truth. Each
  collection has a "Delete collection" button (`DELETE /collections/{cid}`).
- `internal/store`: `collections` and `collection_items` tables; `Collection`
  type; `ListCollections`, `CreateCollection`, `GetCollection`,
  `CollectionQuotes`, `DeleteCollection`. Deleting a quote on home also removes
  it from every collection.

## [0.2.0] - 2026-06-29

### Added
- Quotes-manager web application. `cmd/server` serves a single-binary Go +
  SQLite app: drag-to-reorder quote blocks persisted in real time, inline
  create/edit (3-field form: content, attribution, text ID), delete, bulk
  delete, copy-one, and copy-all (canonical export with the dot separator).
- `internal/store`: a `database/sql` SQLite store (CRUD + reorder) ordered by a
  new user-owned `sort_order` column.
- `internal/seed`: idempotent `EnsureSeeded` — loads the canonical seed on a
  fresh database, adds `sort_order` to a legacy database, and never re-seeds a
  database a user has emptied. The committed `database/seed.sql` is embedded via
  the new `database` package.
- `internal/server`: HTMX-driven handlers and server-rendered templates. Each
  quote block renders with italic passages and a bolded suttacentral link
  (`quote.DisplayHTML`, `quote.SuttaURL`) opening in a new tab.
- `web/`: hand-written templates, a warm-paper serif theme (`app.css`), and a
  small `app.js` for drag-and-drop, bulk select, and clipboard copy. htmx is
  vendored. Assets are embedded via the `web` package.
- `quote.SuttaURL`, `quote.New`, and `quote.DisplayHTML` helpers in
  `internal/quote`.
- A `Makefile` for the common commands (`make test|vet|fmt|run|extract|seed|tidy|clean`),
  exporting `CGO_ENABLED=1`.

### Changed
- `quotes` schema gains a `sort_order INTEGER` column (user-owned order; the
  canonical `id`/`char_count` ranking is preserved).

### Fixed
- Clipboard copy buttons now write within the user gesture via `ClipboardItem`
  (with a `writeText` fallback), so rapid copy operations no longer lose
  transient activation.

### Notes
- Run with `CGO_ENABLED=1` (mattn/go-sqlite3). `cmd/extract` is unchanged and
  still regenerates the canonical seed artifacts.

## [0.1.0] - 2026-06-29

### Added
- Go 1.26 module `github.com/lavantien/quotes-manager`.
- `internal/quote` package: a block-based parser (inline-cited, header-cited,
  narrative-framed, and verse-with-stanza-break quotes), sutta-id/citation
  recognition across all Nikaya / Vinaya forms, the canonical renderer, and the
  SQLite seed emitter — with table-driven tests.
- `cmd/extract`: reads `dumps/*.txt`, normalizes every quote into one canonical
  format, de-duplicates, sorts shortest-first, and writes `database/seed.sql`
  and `exports/shortest-first.md`.
- Generated `database/seed.sql` and `exports/shortest-first.md` — 109 unique
  sutta quotes (char counts 51–5032).
- README documenting the canonical format, separator, schema, extraction rules,
  and regenerate steps.

### Notes
- Quotes are drawn from `sacredness-and-profanity.txt` and
  `stream-entry-for-lay-buddhists.txt`. `discerning-truth-from-deception.txt` is
  prose only and contributes no quotes.
- Unattributed quotes (including all header-cited ones) are normalized to
  "the Buddha".

[0.6.0]: https://github.com/lavantien/quotes-manager/releases/tag/v0.6.0
[0.5.0]: https://github.com/lavantien/quotes-manager/releases/tag/v0.5.0
[0.4.0]: https://github.com/lavantien/quotes-manager/releases/tag/v0.4.0
[0.3.0]: https://github.com/lavantien/quotes-manager/releases/tag/v0.3.0
[0.2.0]: https://github.com/lavantien/quotes-manager/releases/tag/v0.2.0
[0.1.0]: https://github.com/lavantien/quotes-manager/releases/tag/v0.1.0
