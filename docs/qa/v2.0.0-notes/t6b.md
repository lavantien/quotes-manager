# T6b: adversarial review 2 (spec vs implementation)

Angle: read each surface's contract (approved plan, readme, config schema, sc-graph source at ~/temp/temp), then verify the implementation honors it exactly. Attacker only; no fixes.

## Log

- 2026-10-17 13:0x: read the approved plan, task board, internal/config/config.go, internal/cli (cli.go, serve.go, extract.go, coverage.go, screenshot.go, fixture.go, query.go, qmformat.go, all.go, runstage.go, sync.go), internal/quote (sutta.go, quote.go, render.go), internal/server (server.go, api_quotes.go), internal/canon/server (server.go, middleware.go, pages.go, params.go, errors.go), internal/canon/web/web.go, cmd/qm/main.go, web templates + app.js, d2/d3 notes, e2e/routes_test.go. Compared old cmd/ flags from git (5e2f0f6~1) and sc-graph flags/env/stages from the source repo.
- 2026-10-17 13:1x: built dist/qm-t6b.exe (sqlite_fts5, CGO), ran the real artifact: verb registry (16 verbs), version, config file strictness + env-over-file, query quotes byte-identity vs exports/shortest-first.md (sha256 equal), query edge paths (limit, no-match, bad format, missing canon pointer), live server on 127.0.0.1:8791 with temp db: full route probe (quotes pages, canon pages at root, /api/v1 including quotes list/get/404 and catch-all 404), CSP headers on html only, citation cross-link hrefs for all 109 corpus quotes (zero suttacentral fallbacks, vinaya fragment stripped), origin fix (localhost-origin POST passes, evil-origin POST 403), static bucket unity (app.css/canon.css/nav.css/htmx/cytoscape from one /static/), api limit clamps and bad-limit 400 shape. Killed the server, verified the process gone.
- 2026-10-17 13:2x: verified vendored asset pins byte-exact against Makefile sha256s (htmx 4.0.0 = npm dist-tag next, newest 4.x; cytoscape 3.34.4 = npm latest). Swept the tree for tex/zip leftovers: zero live-code hits for texsplit/FindZips/ZipInfo/trigram/etc; remaining ".tex" mentions are test fixtures (prune-ignore proofs, ignore-non-html proofs) plus a config rejection test for tex keys. Ran make-test-pkg equivalents for ./internal/quote and ./internal/config: green.

## Verified contracts (no finding)

- Config hub: sections/defaults/file/env precedence, DisallowUnknownFields (live-probed), QM env surface one-to-one with sc-graph's SC_GRAPH set (DATA_DIR, DIST_DIR, MANIFESTS_DIR, REPORTS_DIR, DPD_URL, DPD_SHA256, SERVER_SQL_CONSOLE->CANON_SERVER_SQL_CONSOLE, EXPORT_PARQUET, BATCH_SIZE, SIMILARITY_MIN, EMPTY_WARN_PER_MUID, E2E_MUIDS) plus new QM_QUOTES_*/QM_SERVER_(ADDR,timeouts)/QM_CANON_(paths,server knobs). Addr default 127.0.0.1:8080.
- CLI: 16 verbs match the plan set; quotes flags preserved (coverage -profile/-readme, fixture -db/-out, serve --addr/--db, screenshot --out now config-defaulted to pictures_dir/home.png); canon verb flags byte-identical to sc-graph; stage list and all-chain identical to sc-graph.
- qm format: `qm query quotes` output is byte-identical (sha256) to exports/shortest-first.md; segments/search default json; quotes defaults qm; bad format errors; empty result prints nothing exit 0.
- Routes: merged tree live-probed; 86-pattern inventory test matches the registered surface; /api/v1/quotes clamps + 404 shapes use the canon envelope; /api/ catch-all 404 for GET/POST; unknown non-API path renders the quotes index (deliberate quotes SPA behavior).
- Middleware: one chain (panic recovery innermost -> security headers -> origin check -> logging) over both surfaces; CSP default-src 'self' only on text/html; origin fix compares u.Host == r.Host (localhost vs 127.0.0.1 same-origin POST passes live, cross-origin 403).
- htmx: only external scripts, no inline handlers; every hx- token used by either template set exists in the vendored 4.0.0 bundle; morph:innerHTML supported and used by canon search.tmpl; asset sha256 pins match the Makefile.
- Cross-links: all 109 corpus citations render internal /texts/{muid} links; pli-tv fragment stripped; no suttacentral fallback left in the corpus.

## Findings

1. [low] `qm query quotes` on a db path that does not exist (in an existing dir) silently creates a fully-schema'd empty db (store.Open runs CREATE TABLE IF NOT EXISTS, internal/store/sqlite.go:16,61), prints nothing, exits 0. Repro: `QM_QUOTES_DB_PATH=%TEMP%\new.db qm query quotes` -> rc 0, len 0, file now exists. Contract angle: the qm format exists so an agent can paste output straight into a reply; a typo'd path or wrong cwd yields plausible success indistinguishable from a no-match, plus a write side effect from a nominally read-only verb. The canon query subcommands contrast correctly: they fail loudly when pointer.json is missing (verified live: `qm query: query: read pointer: open database\canon\pointer.json: ...`). Same-verb-family failure semantics are inconsistent. Missing parent dir does error cleanly (`unable to open database file`).
2. [info] Plan wording vs implementation on static caching: the plan says "the unified route keeps canon's cacheStatic immutable caching", but sc-graph's cacheStatic sets `public, max-age=86400` (source pages.go:86, one day, not immutable) and the merged tree serves /static/ through the quotes-side cacheStatic with the byte-identical value (live-verified on app.css/canon.css/nav.css/htmx/cytoscape). Implementations agree with each other; only the plan's word "immutable" is wrong. E3 readme wording should say one-day caching.
3. [info] The shared query FlagSet accepts irrelevant flags silently: `qm query segments --db foo` parses and --db is ignored (--db only affects the quotes subcommand), and `qm query tree --format qm` parses and --format is ignored (only segments/search honor it; the usage string implies it applies broadly). Inherited sc-graph pattern widened by the two new flags. Cosmetic, no behavior break.
4. [info] .gitignore still whitelists `!docs/home.png` and both docs/home.png and project-pictures/home.png are tracked while the Makefile screenshot target now writes project-pictures/home.png. Retirement is queued E2/E3 work already on the board; the ignore exception should die with the file.
5. [info, unverifiable here] Cross-link targets like `/texts/an1.278-286` or `/texts/snp2.14` resolve only if the published canon graph contains that exact muid; in this tree (no published canon db) every /texts/{muid} is the 503 not-ready page. The mapping contract itself (round-trip, KN strip, vinaya fragment strip, collision errors both ways) is fully implemented and tested; target existence is a data property that needs a live sync to verify, which the verify chain's e2e stage covers.

## Verdict

No high-severity breakage found. Every explicit contract I could drive (config precedence and env one-to-one mapping, CLI verb/flag preservation both families, stage list, qm byte fidelity incl. sha256-equality with exports/shortest-first.md, merged route inventory vs live behavior, middleware order and origin fix, CSP and inline-script absence, htmx 4.0.0 pin and attribute/morph support, cross-link mapping over the whole corpus, tex/zip removal completeness, notfound back-link, verify chain composition) holds against the plan and the sc-graph source. The 5 findings above are 1 low usability/semantics gap and 4 informational notes.

## Reading log

- read docs/qa/v2.0.0-task-board.md full - contract sources
- read plan adopt-all-the-features-functionalities-tender-bumblebee.md full - the merge contract
- read internal/config/config.go full - env/precedence contract
- grep sc-graph internal/config/config.go env surface - one-to-one check
- read internal/cli/{cli,serve,extract,coverage,screenshot,fixture,query,qmformat,all,runstage,sync}.go full - flag contracts
- git show 5e2f0f6~1 cmd/{server,coverage,fixture,extract,screenshot} - old flag surface
- read internal/quote/{sutta,quote,render}.go full - mapping + renderer contracts
- read internal/server/{server,api_quotes}.go full - route + api contracts
- read internal/canon/server/{server,middleware,pages,params,errors}.go full - merged shell contract
- read internal/canon/web/web.go full - shared nav single-sourcing
- read cmd/qm/main.go full - composition wiring
- read web/templates/{nav,layout}.html, web/static/app.js full - htmx/CSP surface
- read internal/e2e/routes_test.go full - inventory pin
- read docs/qa/v2.0.0-notes/d2.md full - qm format design contract
- grep Makefile pins + Get-FileHash web/static - asset pin verification
- npm registry queries - latest-first verification
- live: dist/qm-t6b.exe subcommands + server on :8791 (killed, verified gone)
