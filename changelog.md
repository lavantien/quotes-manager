# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

[0.1.0]: https://github.com/lavantien/quotes-manager/releases/tag/v0.1.0
