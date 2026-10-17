# T6fb: remediation of t6a findings 6, 7, 8 (editions + config + store + app.js)

Scope: internal/canon/source/editions/**, internal/config/**, internal/store/** including storetest, web/static/app.js. Baseline before any edit: `make test-pkg` green on all three packages (editions 2.137s, config cached, store cached).

## Fix 1: t6a F7, store listing order diverged from the canonical export order on ties

The store ordered quotes by (char_count, id) in List (sqlite.go) and CategoryQuotes (sqlite.go) while the canonical comparator quote.SortByCharCount orders by (char_count, sutta_id, body_text). Equal-length quotes created via the UI therefore listed in insertion order but exported in sutta order.

Failing-first evidence (store_test.go, before the fix, all three failed):

- TestListCharCountTieBreaksBySuttaID: MN 10 (id 1) + AN 2 (id 2), equal char_count, inserted MN first. Old order: MN 10, AN 2. Want: AN 2, MN 10.
- TestListSuttaIDTieBreaksByBodyText: SN 1.1 "zz" (id 1) + SN 1.1 "aa" (id 2). Old order: zz, aa. Want: aa, zz.
- TestCategoryQuotesCanonicalTieBreak: same AN 2/MN 10 pair behind a category. Old order: MN 10, AN 2.

Fix: both ORDER BY clauses now use the full canonical key `char_count ASC, sutta_id ASC, body_text ASC`, byte-order TEXT comparisons matching Go string compares. The third key (body_text) is included because the canonical comparator has it; aligning on only the two-key shorthand would leave same-sutta ties diverging. CollectionQuotes keeps its curated ci.position order and the membership map queries keep their own keys: neither is a canonical listing.

Consumers: server handlers and `qm query quotes` render straight from store.List()/CategoryQuotes() with no re-sort, so both surfaces inherit the fix. Cross-check: `make test-pkg ./internal/cli` green (97s, includes TestRowsToQuotesMatchCanonicalExportFile, the fixture byte-identity pin against exports/shortest-first.md). `./internal/server` fails only dev-t6fa's in-flight untracked notfound_test.go (the t6a F1 404 work); no server failure traces to the store change.

Old TestListCharCountTieBreaksByID (pinned insertion order) was replaced by the canonical tests above.

Commit: 0c1dbc3 `fix: break quote listing ties by sutta id and body like the canonical export`. Comments stripped from sqlite.go, store.go, store_test.go (zero-comments protocol, touched files).

## Fix 2: t6a F6, dead tex leftovers

Reference gate before deletion (grep over **/*.go): ParseBuildDate hits = html.go definition + html_test.go test only, zero production callers (the source-repo caller lived in the deleted internal/pipeline/texsplit.go); texZipSuffix hits = html.go only; tex_zip_suffix hits = config_test.go rejection sample only. No live caller contradicted deadness, so deletion proceeded.

Deleted: editions.ParseBuildDate, the texZipSuffix const, TestParseBuildDate. config.go never carried a tex key (its Editions struct is authors/sparse_dirs/keep_regexes), so no config field or validation existed to remove. TestLoadRejectsTexKeys (strict-decode regression pin naming tex keys) folded into TestLoadRejectsUnknownKeys with an added `{"canon": {"editions": {"nope": 1}}}` case, preserving the editions-section strict-decode guard without the tex reference.

Gate after deletion (grep over **/*.go): ParseBuildDate, texZipSuffix, tex_zip_suffix = zero hits repo-wide. Two `-tex.zip` strings remain in internal/e2e/hermetic_test.go:81 and internal/canon/server/e2e_test.go:90: fixture zip names written into temp editions trees to exercise the sparse-checkout keep_regexes prune, outside my ownership and unrelated to the deleted symbols.

Commit: 5e425c6 `refactor: delete the orphaned tex zip date parser`.

## Fix 3: t6a F8 partial, comments in app.js and config.go

web/static/app.js: 12 comment blocks stripped (33 lines), code byte-identical otherwise, `node --check` clean. internal/config/config.go: verified already comment-free; its only `//` tokens are https URL string literals, so no edit was needed. config_test.go (touched by fix 2) verified comment-free.

Commit: 6c2a938 `refactor: strip comments from app.js`.

## Mutation survivors and closure

First scoped run (t6fb-mutate.log): mutants=295 killed=264 invalid=27 survived=4, rc=1. All four in config.go Validate:

- config.go:368 || -> && (2 of the 3 occurrences): the table only tested "bilara no authors", so flipping the pairs (root_lang,root_edition) or (root_edition,trans_lang) to && still errored on the authors case. No case emptied one lang field alone.
- config.go:381 || -> && (occurrence 1): only "dpd zero timeout" exercised the line; the flipped pair (max_tarball,max_db) still errored on the timeout case. No case zeroed max_tarball_bytes alone.
- config.go:388 != -> ==: the existing "dpd sha256 short" case is 63 chars, an odd length, so under the flipped guard it still failed validation via hex.DecodeString's ErrLength; "not hex" (64 chars) errored through the flipped branch itself. Distinguishing the length check needs an even-length, valid-hex, non-64 string.

Closure (2d11e4f): 6 new TestValidateCanonFailures cases — bilara empty root lang / root edition / trans lang each alone, dpd zero max tarball / max db each alone, dpd sha256 even length under 64 (hex64[:62]). Re-applied-mutation verification for all 5 flip sites on the two lines plus the 388 site: each exact operator flip applied, the named subtest(s) failed, restore confirmed byte-exact by an empty git diff, package green after each restore. Expected kills per site: 368 occ1 by empty-root-lang (+root-edition), occ2 by root-edition (+trans-lang), occ3 by trans-lang (+no-authors), 381 occ1 by zero-max-tarball (+zero-max-db), occ2 by zero-max-db (+zero-timeout), 388 by the 62-char case alone.

Decisive rerun (t6fb-mutate2.log, finished 15:39:53 local): slots=8 mutants=295 killed=268 invalid=27 survived=0. Green. The agent was killed before the detached rerun finished (notes stalled 15:22, no final report landed), so main verified the log tail against the full 295/295 progress and landed this file plus both mutate logs.

## Chain

- baseline test-pkg (3 packages): green
- failing-first run: 3/3 new store tests failed, then green after the fix
- make test-pkg on the three packages: green (re-run green after the kill-test commit)
- make fmt, make lint (0 issues), make typecheck, make sloc: green (re-run green after the kill-test commit)
- cross-check ./internal/cli: green; ./internal/server failures are dev-t6fa's in-flight 404 tests
- make mutate MUTATE_PKGS=internal/canon/source/editions,internal/config,internal/store: first run survived=4, closed above; decisive rerun survived=0
- all commits through the pre-commit hooks, no --no-verify, no Co-Authored-By

## Reading log

- read docs/qa/v2.0.0-notes/t6a.md, t6b.md full - owned findings
- read docs/qa/v2.0.0-task-board.md lines 104-110 - dispatch contract
- read internal/canon/source/editions/html.go full + grep ParseBuildDate/texZipSuffix repo-wide - deadness proof
- read internal/canon/source/editions/html_test.go full - test to delete, fixture layout
- read internal/config/config.go full + grep // - tex key absence, comment-free proof
- read internal/config/config_test.go lines 240-319 - TestLoadRejectsTexKeys fold site
- read internal/store/sqlite.go full - ORDER BY sites
- read internal/store/store.go, store_test.go, export.go full - interface contract, tie-break test, dump ordering
- read internal/store/storetest/fixture.go full - fixture mechanics
- read internal/store/categories_test.go lines 160-199 - CategoryQuotes order pin (no tie, unaffected)
- read internal/quote/quote.go lines 90-118 - canonical comparator
- grep internal/quote/quote_test.go tie pins - canonical key confirmation
- read internal/cli/qmformat_test.go lines 40-99 + grep qmformat.go List usage - byte-fidelity consumer, no re-sort
- grep internal/server List()/CategoryQuotes() callers - consumers inherit the order
- read web/static/app.js full - comment inventory
- read Makefile full - chain targets
