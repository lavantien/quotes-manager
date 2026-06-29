# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

[0.2.0]: https://github.com/lavantien/quotes-manager/releases/tag/v0.2.0
[0.1.0]: https://github.com/lavantien/quotes-manager/releases/tag/v0.1.0
