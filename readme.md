# quotes-manager

A small Go 1.26 pipeline that distills the sutta quotes embedded in the essay
dumps (`dumps/*.txt`) into one canonical format, loads them into SQLite, and
exports a shortest-first text file. This is the seed step for a future
Go + SQLite quotes-manager web application.

## Directory layout

```
dumps/                         source essays (input, hand-written)
  discerning-truth-from-deception.txt   (prose only — no sutta quotes)
  sacredness-and-profanity.txt          (sutta quotes, inline-cited)
  stream-entry-for-lay-buddhists.txt    (sutta quotes, inline + header-cited)
internal/quote/                parser, normalizer, renderer, seed emitter (+ tests)
cmd/extract/                   CLI: reads dumps/ -> writes database/ + exports/
database/
  seed.sql                     generated schema + inserts (committed)
  quotes.db                    generated SQLite database (gitignored)
exports/
  shortest-first.md            generated export, shortest-first (committed)
go.mod
readme.md
changelog.md
.gitignore
```

`database/seed.sql` and `exports/shortest-first.md` are generated — regenerate
with `go run ./cmd/extract`. `database/quotes.db` is gitignored; populate it
from the seed with `sqlite3 database/quotes.db < database/seed.sql`. Never
hand-edit generated files.

## Canonical quote format

Every extracted quote is normalized into one format and written to **both** the
database (`body_md`) and `exports/shortest-first.md`:

```
*"first passage*  
*second passage*  
*last passage"* - **the Buddha, MN 22**
```

- Each passage line is wrapped in italics (`*…*`).
- Lines 1..n−1 end with two spaces (a Markdown line break); there are **no
  blank lines** between passages of the same quote.
- The last line ends with ` - **<citation>**`, outside the italics.
- `<citation>` keeps the **full attribution as found in the source**
  (`the Buddha, MN 22`, `the Buddha to layman Pessa, MN 51`, `layman Siha,
  AN 8.12`). Any quote recorded without an attribution is attributed to
  **the Buddha** (e.g. `the Buddha, AN 4.180`, `the Buddha, SN 55.1`);
  suttacentral URLs in `( … )` are dropped.
- Source curly quotes (`“ ”`) and Pāli diacritics are preserved.

### Separator in the text export

Consecutive quotes in `exports/shortest-first.md` are divided by:

```
.  
.  
.
```

(two blank lines before and after the divider; the first two dots carry two
trailing spaces).

## Extraction rules

The dumps quote suttas in several formats; all are reduced to the canonical form
above (`internal/quote`).

- **Inline-cited** — a block whose last line ends with ` - <citation>`. Covers
  single-line quotes, multi-line dialog, and narrative-framed passages.
- **Header-cited** — a lone `SUTTA:` line (e.g. `SN 55.1:`, `MN 13:`); every
  following block becomes the quote's passages until the next header or a `.`
  divider. Such quotes include any framing narrative the essay placed between the
  header and the divider (e.g. `MN 13`).
- **Verse with stanza breaks** — a quote may span several blank-separated blocks;
  leading blocks that open with `“` but carry no citation are absorbed into the
  next cited block (e.g. `SN 5.2`).

Per-line cleanup: a leading `(N)` numbering marker and stray `*` / `_` Markdown
artifacts are stripped; the ` - <citation>` tail of the closing line is removed
(it is rendered separately). A citation with no attribution (just the sutta id,
as with all header-cited quotes) is normalized to `the Buddha, <id>`.

Sutta-ID forms recognized: `(DN|MN|AN|SN) N[.N…][-N][#…]`,
`KN <sub> N[…]`, `pli-tv-…#…`, and the abbreviated Vinaya `Tv Vi Bu Pj1`.

### De-duplication, ordering, and counting

- **De-dup** by normalized passage text (whitespace collapsed). Source-file lists
  are merged; the first-seen citation / sutta id is kept.
- **Order** shortest-first by rune count of the concatenated passages (stable,
  with deterministic tie-breakers on sutta id then body text). Row `id` equals
  the shortest-first rank.
- Five quotes recur across both essay files (e.g. `AN 8.53`, `MN 117`,
  `SN 20.7`) and are collapsed to one row each.

## SQLite schema

```sql
CREATE TABLE quotes (
    id          INTEGER PRIMARY KEY,     -- shortest-first rank
    sutta_id    TEXT    NOT NULL,        -- canonical id, e.g. "MN 22"
    citation    TEXT    NOT NULL,        -- full kept citation
    body_md     TEXT    NOT NULL,        -- canonical italicized format
    body_text   TEXT    NOT NULL,        -- plain passages joined by newlines
    line_count  INTEGER NOT NULL,
    char_count  INTEGER NOT NULL,        -- rune count of passages (sort key)
    sources     TEXT    NOT NULL         -- ';'-joined dump files
);
```

Indexes on `char_count` and `sutta_id`. Current seed: **109 quotes**,
char counts from 51 to 5032.

## Regenerate

```sh
go test ./...                 # parser / renderer / seed tests
go run ./cmd/extract          # writes database/seed.sql + exports/shortest-first.md
sqlite3 database/quotes.db < database/seed.sql   # (re)populate the database
```

## Notes

- `discerning-truth-from-deception.txt` is prose only; it mentions suttas inline
  (e.g. `AN5.34#7.9`) but contains no citation-terminated block quotes, so it
  contributes **zero** quotes.
- The Vinaya "black snake" passage appears as `Tv Vi Bu Pj1` in one essay and
  `pli-tv-bu-vb-pj1#5.11.20` in the other with identical text; de-duplication
  collapses them to one row, keeping the first-seen `pli-tv-…` id.
