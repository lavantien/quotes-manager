# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

| Version | Date | Type | Change |
|---------|------|------|--------|
| [0.7.1] | 2026-07-03 | Changed | Streamlined the README (tighter prose, less decorative formatting, no emdashes) and converted the CHANGELOG from nested version headings into a single version table. |
| [0.7.0] | 2026-07-03 | Added | Near-duplicate detection in `internal/quote`: word-level Jaccard similarity over cleaned passage text plus a disjoint set that groups quotes whose pairwise similarity exceeds `0.8`, transitively. `quote.GroupDuplicates` returns only clusters of two or more, grouped by content, not text id. Fully unit-tested (table + property) and pinned to the seed's `MN 22` trio. |
| [0.7.0] | 2026-07-03 | Added | Duplicates rail listing each cluster's representative (shortest) text id with a member-count chip, with a body-excerpt fallback when a quote has no text id. |
| [0.7.0] | 2026-07-03 | Added | Jump to duplicate: clicking a group switches the root column to Home (when a category filter is active), scrolls the representative into view with a brief highlight, and falls back to a `/#quote-{id}` anchor when JS is off. |
| [0.7.0] | 2026-07-03 | Changed | Independent panel scrolling: the root and collection text columns are now each their own viewport-height scroll container; the single-column mobile layout still scrolls as one page. |
| [0.7.0] | 2026-07-03 | Changed | Live-refresh on every edit: creating, editing, or deleting a quote appends out-of-band swaps so the left rail (Duplicates + category counts), the right rail (collection counts on delete), and the root-zone "N blocks" header stay current without a full reload. |
| [0.7.0] | 2026-07-03 | Changed | Test coverage rises to 90.7% of statements; the new similarity code, `buildDuplicates`, and `bodyExcerpt` are at 100%. |
| [0.7.0] | 2026-07-03 | Fixed | `cmd/coverage` conflated "no coverage markers" with "badge already current", so an idempotent `make coverage` falsely reported the README had no markers. The two cases are now distinguished. |
| [0.6.0] | 2026-07-03 | Added | Dual-pane workspace: four zones side by side (left rail with Home + Categories, root text column, collection column, right rail with Collections), each text column headed by a thin name + count strip. Selecting a category or collection swaps just that pane via htmx with an out-of-band rail refresh; the URL carries `?cat=` or `?col=`. |
| [0.6.0] | 2026-07-03 | Added | Named, renameable collections with a `name` (empty renders as "Collection {id}") and inline rename via `POST /collections/{cid}/rename` (`store.RenameCollection`). Pre-0.6.0 databases are migrated with an idempotent `ALTER TABLE` that adds `collections.name`. |
| [0.6.0] | 2026-07-03 | Added | Insert at a precise index: checking root quotes reveals `+` insert-gap markers between collection blocks; clicking one inserts the selection at that 1-based position via `store.InsertAtCollection` and `POST /collections/{cid}/insert`, shifting later items down and skipping duplicates. |
| [0.6.0] | 2026-07-03 | Added | Collection membership on root blocks: each root quote shows a second chip row for its collections (`store.QuoteCollectionMap`), distinct from category chips; clicking one activates that collection. |
| [0.6.0] | 2026-07-03 | Added | A fresh database is seeded with one sample collection (the two shortest quotes) so the collection column and membership chips are non-empty out of the box. |
| [0.6.0] | 2026-07-03 | Added | Test coverage for `internal/quote` in full and `internal/store`, `internal/server`, `internal/seed`, `internal/coverbadge`, and the `cmd/*` CLIs to 90%+ of statements (90.1% reported). Store error paths driven by closed-DB and broken-schema harnesses; server handlers by a failing-store table; `cmd/screenshot`'s capture step is injectable and runs in-process. |
| [0.6.0] | 2026-07-03 | Changed | Sidebar split into two nav rails; top bar is a thin name + count strip with per-column toolbars (root: select-all, New, bulk-delete, Copy all; collection: Copy all, Delete). |
| [0.6.0] | 2026-07-03 | Changed | Creating a collection from a selection now makes it active and swaps the collection zone in place instead of redirecting. |
| [0.6.0] | 2026-07-03 | Changed | `web/static` styles split into `app.css` (typography/components) and `layout.css` (the four-zone grid, rails, zones, insert gaps). |
| [0.6.0] | 2026-07-03 | Changed | `cmd/*` CLIs had testable logic extracted from `main`/`run` (`cmd/extract` `generate`/`report`, `cmd/coverage` `run`/`parseFlags`, `cmd/server` `serve`, `cmd/screenshot` `runWith`) with unit tests added; behavior unchanged. |
| [0.6.0] | 2026-07-03 | Fixed | Coverage badge undercounted: `coverbadge.Pct` summed every line of the merged profile, but `go test -coverpkg=./...` writes one profile per binary, so a block instrumented by N binaries counted N times. Blocks are now keyed by `file:start,end` and OR-ed, matching `go tool cover`; the badge reads 90.1%. |
| [0.5.0] | 2026-07-01 | Added | Categories: named, reusable tags managed independently of collections. Create, rename, delete from the sidebar; names unique (case-insensitive). Backed by `categories` and `category_items` and `store.ListCategories`, `CreateCategory`, `GetCategory`, `RenameCategory`, `DeleteCategory`, `CategoryQuotes`, `SetQuoteCategories`, `QuoteCategoryMap`. |
| [0.5.0] | 2026-07-01 | Added | Per-quote categories: each block shows its categories as chips with an inline editor (`GET /quotes/{id}/categories/edit`) that toggles any combination and can create one inline; saving commits via `POST /quotes/{id}/categories` (`store.SetQuoteCategories`). |
| [0.5.0] | 2026-07-01 | Added | Category view `GET /categories/{ctid}` filters the list to a category's quotes in home order with `GET /categories/{ctid}/export.txt` for Copy all; read-only, mirroring the collection view. |
| [0.5.0] | 2026-07-01 | Added | Sidebar layout: home page is a sticky left sidebar (Collections + Categories, each with counts and inline manage) beside the quote list. Deleting a category untaggs its quotes; deleting a quote clears its tags. |
| [0.5.0] | 2026-07-01 | Changed | `store.Delete`/`DeleteMany` cascade `category_items` alongside `collection_items` in the same transaction. |
| [0.5.0] | 2026-07-01 | Changed | A fresh database seeds three example categories (`suffering`, `renunciation`, `right view`) tagging the shortest quotes. |
| [0.4.0] | 2026-06-30 | Added | Choose a target collection: the "Add to collection" control on home now has a dropdown to add selected quotes to an existing collection (prepended, duplicates skipped) via `POST /collections/{cid}/items` (`store.AddToCollection`), or pick "+ New collection" for the create flow. |
| [0.4.0] | 2026-06-30 | Added | Block-count badge beside the title on home and collection pages. |
| [0.4.0] | 2026-06-30 | Added | Coverage badge: `make coverage` runs the suite with `-coverpkg=./...` and `cmd/coverage` (backed by `internal/coverbadge`) refreshes a shields.io test-coverage badge in the README between markers; the percentage is parsed from the real Go cover profile. |
| [0.4.0] | 2026-06-30 | Added | Home screenshot: `make screenshot` runs `cmd/screenshot`, which serves the seeded app in-process and captures `docs/home.png` via chromedp (system Chrome or Edge; `QUOTES_BROWSER` overrides). |
| [0.4.0] | 2026-06-30 | Changed | Home is ordered by `char_count` (shortest-first). A newly added quote slots into its sorted place via `List` (`char_count, id`) and `create` re-renders the full list. |
| [0.4.0] | 2026-06-30 | Removed | Home drag-reorder and the `sort_order` machinery (column + index, `Quote.SortOrder`, `store.Reorder`, `POST /quotes/reorder`, the `internal/seed` sort_order migration). Collections still support manual drag-reorder. Legacy databases with a leftover `sort_order` column keep working; the column is ignored. |
| [0.3.0] | 2026-06-29 | Added | Numbered collections: select quotes on home and "Add to collection" to create a numbered collection; collections appear as nav between the title and action buttons. |
| [0.3.0] | 2026-06-29 | Added | Collection views (`GET /collections/{cid}`) render the same block layout, copyable (copy-one and copy-all via `/collections/{cid}/export.txt`) and drag-to-reorder (`POST /collections/{cid}/reorder`), but read-only for content: no New, edit, or delete. Each has a "Delete collection" button (`DELETE /collections/{cid}`). |
| [0.3.0] | 2026-06-29 | Added | `internal/store`: `collections` and `collection_items` tables; `Collection` type; `ListCollections`, `CreateCollection`, `GetCollection`, `CollectionQuotes`, `DeleteCollection`. Deleting a quote on home also removes it from every collection. |
| [0.2.0] | 2026-06-29 | Added | Web application: `cmd/server` serves a single-binary Go + SQLite app with drag-to-reorder blocks persisted in real time, inline create/edit (3-field form: content, attribution, text ID), delete, bulk delete, copy-one, and copy-all (canonical export with the dot separator). |
| [0.2.0] | 2026-06-29 | Added | `internal/store`: a `database/sql` SQLite store (CRUD + reorder) ordered by a new user-owned `sort_order` column. |
| [0.2.0] | 2026-06-29 | Added | `internal/seed`: idempotent `EnsureSeeded` loads the canonical seed on a fresh database, adds `sort_order` to a legacy database, and never re-seeds a user-emptied database; the committed `database/seed.sql` is embedded via the new `database` package. |
| [0.2.0] | 2026-06-29 | Added | `internal/server`: HTMX handlers and server-rendered templates; each block renders italic passages and a bolded suttacentral link (`quote.DisplayHTML`, `quote.SuttaURL`) opening in a new tab. |
| [0.2.0] | 2026-06-29 | Added | `web/`: hand-written templates, a warm-paper serif theme (`app.css`), and a small `app.js` for drag-and-drop, bulk select, and clipboard copy; htmx vendored; assets embedded via the `web` package. |
| [0.2.0] | 2026-06-29 | Added | `quote.SuttaURL`, `quote.New`, and `quote.DisplayHTML` helpers in `internal/quote`. |
| [0.2.0] | 2026-06-29 | Added | A `Makefile` for common commands (`test`, `vet`, `fmt`, `run`, `extract`, `seed`, `tidy`, `clean`), exporting `CGO_ENABLED=1`. |
| [0.2.0] | 2026-06-29 | Changed | `quotes` schema gains a `sort_order INTEGER` column (user-owned order; the canonical `id`/`char_count` ranking is preserved). |
| [0.2.0] | 2026-06-29 | Fixed | Clipboard copy buttons now write within the user gesture via `ClipboardItem` (with a `writeText` fallback), so rapid copy operations no longer lose transient activation. |
| [0.2.0] | 2026-06-29 | Notes | Run with `CGO_ENABLED=1` (mattn/go-sqlite3). `cmd/extract` is unchanged and still regenerates the canonical seed artifacts. |
| [0.1.0] | 2026-06-29 | Added | Go 1.26 module `github.com/lavantien/quotes-manager`. |
| [0.1.0] | 2026-06-29 | Added | `internal/quote` package: a block-based parser (inline-cited, header-cited, narrative-framed, verse-with-stanza-break), sutta-id/citation recognition across all Nikaya/Vinaya forms, the canonical renderer, and the SQLite seed emitter, with table-driven tests. |
| [0.1.0] | 2026-06-29 | Added | `cmd/extract`: reads `dumps/*.txt`, normalizes every quote into one canonical format, de-duplicates, sorts shortest-first, and writes `database/seed.sql` and `exports/shortest-first.md`. |
| [0.1.0] | 2026-06-29 | Added | Generated `database/seed.sql` and `exports/shortest-first.md`: 109 unique sutta quotes (char counts 51 to 5032). |
| [0.1.0] | 2026-06-29 | Added | README documenting the canonical format, separator, schema, extraction rules, and regenerate steps. |
| [0.1.0] | 2026-06-29 | Notes | Quotes are drawn from `sacredness-and-profanity.txt` and `stream-entry-for-lay-buddhists.txt`; `discerning-truth-from-deception.txt` is prose only and contributes no quotes. |
| [0.1.0] | 2026-06-29 | Notes | Unattributed quotes (including all header-cited ones) are normalized to "the Buddha". |

[0.7.1]: https://github.com/lavantien/quotes-manager/releases/tag/v0.7.1
[0.7.0]: https://github.com/lavantien/quotes-manager/releases/tag/v0.7.0
[0.6.0]: https://github.com/lavantien/quotes-manager/releases/tag/v0.6.0
[0.5.0]: https://github.com/lavantien/quotes-manager/releases/tag/v0.5.0
[0.4.0]: https://github.com/lavantien/quotes-manager/releases/tag/v0.4.0
[0.3.0]: https://github.com/lavantien/quotes-manager/releases/tag/v0.3.0
[0.2.0]: https://github.com/lavantien/quotes-manager/releases/tag/v0.2.0
[0.1.0]: https://github.com/lavantien/quotes-manager/releases/tag/v0.1.0
