# T6a: adversarial review 1

Scope: whole merge at 2a35188 (+ uncommitted Makefile/memcapmon from T5a, build-only impact). Attacker role: findings only, no fixes. All runtime evidence from `make build` -> dist/qm.exe (102779639 B) driven on scratch ports (18096-18099) with temp dbs under %TEMP%\t6a-dd425ec9; every spawned process killed and verified gone.

## Findings (ranked)

1. Merged tree has no 404 for GET; unknown paths serve the quotes home with 200 (medium-high)
   - Repro: `curl http://127.0.0.1:18099/no-such-page` -> 200, 208463 B quotes home HTML. Same for `/texts/mn1/deep/nested`, `/quotes/import`. `POST /no-such-page` -> 405 `Allow: GET, HEAD`.
   - Cause: cmd/qm/main.go:36-38 builds the root mux from RegisterRoutes only. Quotes `GET /` (internal/server/server.go:40) is a trailing-slash subtree pattern matching every GET path. Canon's `/` 404 fallback lives only in `Routes()` (internal/canon/server/server.go:125), which serveMerged never calls, so `pageNotFound` (pages.go:67) and web/templates/notfound.tmpl are unreachable in the shipped binary.
   - Why it violates the requirement: plan line 93 analyzes the merged-root collision "GET / (quotes index) vs canon's / 404 fallback", i.e. the fallback was part of the merged root contract. Consequences: the merged app has no 404 at all outside /api/ and /static/; the maintained notfound page is dead code; D3's crawl gate (crawl_test.go:54 fails only on 404/5xx) structurally cannot detect dead links because every GET returns 200, and the dead citation links in finding 2 render as silent empty pages instead of 404s.

2. Nikaya `#` fragment citations cross-link to muids that cannot exist (medium)
   - Repro: POST /quotes with text_id `SN 42.8#3`, then home HTML contains `href="/texts/sn42.8%233"`. The canon db keys texts by fragment-free muids (only pli-tv fragments are stripped, internal/quote/sutta.go:42-47), so /texts/sn42.8#3 renders an empty text page (or 503 before load), never the cited sutta.
   - Why: plan line 98 requires "citation links to /texts/{muid} when the mapping succeeds". The mapping succeeds but produces `an5.34#7.9`-style keys (pinned in sutta_test.go "fragment kept"), which the graph store will never contain. The corpus seed has no nikaya-fragment citation, but the UI accepts arbitrary text_id, so any user-entered one yields a dead link.

3. KN `#` fragments silently dropped, inconsistent with nikaya (low)
   - Repro: POST /quotes text_id `KN Dhp 5#2` -> rendered citation id becomes `KN Dhp 5`, href `/texts/dhp5`. The KN branch of suttaIDPat (sutta.go:12) lacks the `#...` suffix the nikaya branch has (sutta.go:11), so the fragment vanishes from display and link while nikaya fragments are preserved. Same-input family, two behaviors.

4. Alias-failing KN ids fall back to garbage external links (low)
   - Repro: POST /quotes text_id `KN An 3` -> href `https://suttacentral.net/knan3` (dead). MuidFromSuttaID correctly errors on the alias, but render.go suttaHref falls back to SuttaURL which compact-lowercases the whole id. Pre-existing SuttaURL behavior (its test pins `knsnp2.14` too), but the merge made it reachable from the quotes UI citation link where an unmappable-id signal would be more correct.

5. `Tv Vi Bu Pj` family maps to muids absent from bilara data (low, latent)
   - Repro: POST /quotes text_id `Tv Vi Bu Pj2` -> href `/texts/tvvibupj2`. Bilara vinaya muids are `pli-tv-*` (the corpus's own `pli-tv-bu-vb-pj1#5.11.20` maps correctly to `/texts/pli-tv-bu-vb-pj1`); `tvvibupj2` exists nowhere in the canon sources. No seed or dump citation uses the abbrev form (grep clean), so this only fires on hand-entered ids.

6. tex/zip removal orphaned editions.ParseBuildDate (low)
   - internal/canon/source/editions/html.go:194-199 + texZipSuffix (line 21) have zero production callers in quotes-manager. Their only source-repo caller was internal/pipeline/texsplit.go:280 (deleted with texsplit). Only html_test.go calls it now: dead exported API certified by its own test. No other tex/zip leftovers found (pipeline, triangulate, report greps clean; remaining ".tex" hits are fixtures mimicking editions zip layout for FindHTML tests, which is intended).

7. Store order vs canonical export order diverge on char_count ties after UI edits (info)
   - Repro: created `MN 10` then `AN 2` with equal-length bodies on the scratch db; home and `qm query quotes` list MN 10 first (store tie-break `char_count, id`, sqlite.go:118) while the canonical sort tie-break is sutta_id (quote.go:107-118), which would put AN 2 first. Seeded data cannot diverge (ids assigned in extract order), so only post-edit corpora reorder. Byte fidelity itself is unaffected (see verifications).

8. Comments persist in newly touched code despite the zero-comments protocol (info)
   - Examples: web/static/app.js applyFragments block (added by D1) carries 4 comment blocks; internal/server/handlers.go, internal/quote/quote.go, internal/config/config.go, internal/canon/web/web.go all carry doc comments; Makefile comments were only removed in T5a's uncommitted working tree. Flagged for the record; no functional impact.

## Verifications that came back clean

- Byte fidelity: `qm query quotes` == exports/shortest-first.md (87165 B, sha equal); live `/export.txt` == same; `qm extract` into temp dirs regenerates database/seed.sql and exports/shortest-first.md byte-identical. `--format json`, `--limit`, `--db`, bad-format exit 1 all correct.
- htmx 4.0.0: vendored sha256 e484d917...d593f matches the Makefile pin; live browser (Playwright) on the quotes home: `window.htmx.version == "4.0.0"`, category rail click swaps #root-zone in place with zero navigation, canon /search input-triggered `morph:innerHTML` swap works, /graph loads cytoscape; zero console errors/warnings on all three pages under CSP `default-src 'self'`; no inline scripts or inline handlers in either UI's templates (grep clean).
- Middleware: cross-origin POST 403 JSON origin_rejected (quotes and canon paths alike), same-origin POST passes; nosniff everywhere; CSP only on text/html; /static/ has Cache-Control public max-age=86400; /static/../go.mod does not leak.
- Config: defaults < file < env < flag precedence all verified live (env beats file on addr, flag beats both); DisallowUnknownFields rejects unknown keys; env coverage is a strict superset of the source repo's 9 SC_GRAPH_* vars; Validate blocks bad addr/limits.
- CLI verbs: full registry prints correctly; serve -addr/-db, coverage -profile/-readme, screenshot -out, fixture -db/-out all preserved (old cmd/extract and cmd/screenshot had no flags to regress); extract honors env-redirected paths.
- API: /api/v1/quotes limit clamp at max, limit<=0 -> default, negative offset -> 400 json, missing quote -> 404 json `not_found`, POST /api/v1/quotes -> canon catch-all 404 `no such api route`, DELETE -> 405 with Allow.
- Test baselines: make test-pkg green on ./internal/quote, ./internal/config, ./cmd/qm.

## Reading log

- read docs/qa/v2.0.0-task-board.md full - fan-out state, splice maps, known debts
- read readme.md head - old app contract
- read plan adopt-all-the-features... lines 93-117 grep - merged shell, cross-link, output-format requirements
- read internal/quote/sutta.go full - id patterns, muid mapping both ways
- read internal/quote/quote.go, render.go, parse_canonical.go full - format shapes and round-trip
- read internal/config/config.go full - defaults/env/validate
- read internal/server/server.go, handlers.go, api_quotes.go full - quotes routes, form contract, api clamps
- read internal/canon/server/server.go, pages.go, middleware.go full - route registration, chain, 404 fallback
- read cmd/qm/main.go full - merged wiring
- read internal/cli/cli.go, serve.go, query.go, qmformat.go, extract.go, coverage.go, screenshot.go full - flag surfaces and renderer
- read internal/canon/source/editions/html.go full + grep ParseBuildDate callers - orphan proof
- read internal/canon/web/web.go + layout.tmpl + nav.html + notfound.tmpl - shared shell
- read web/static/app.js full, grep templates for hx-*/inline JS, grep htmx.min.js for swap styles - htmx 4 surface
- git show 5e2f0f6^:cmd/{extract,screenshot,fixture,server}/main.go - old flag surfaces
- grep source repo C:\Users\lavantien\temp\temp for SC_GRAPH_* envs and ParseBuildDate caller - env contract and orphan origin
- runtime probes 1-5 + Playwright drive per above; artifacts in %TEMP%\t6a-dd425ec9 (scratch, outside repo)
