# T6fa: confirmed-findings remediation (t6a F1, F2-F5, F8 partial; t6b G1)

Scope: cmd/qm, internal/server, internal/quote, internal/cli only. Failing-test-first on every fix, then minimal code, then the chain (test-pkg on the 4 pkgs, e2e-hermetic, fmt, lint, typecheck, sloc, scoped mutate).

## Fix 1: merged tree 404 (t6a F1)

- Requirement: unknown GET -> 404 with canon's notfound template; known routes byte-unchanged; POST unknown stays 405.
- Where: internal/server RegisterRoutes registers `GET /` -> rootIndex dispatcher (server.go). Path "/" runs the old index handler untouched; "/favicon.ico" returns 204; anything else renders web/templates/notfound.tmpl via internal/canon/web (PageNotFound + NotFoundPage, mirroring canon's pageNotFound) with 404. The dispatch lives in internal/server (not cmd/qm) because internal/e2e compose_test.go builds the merged mux itself with server.RegisterRoutes + canon RegisterRoutes, so the fix must ship inside the quotes side to reach both wirings; the 86-pattern inventory test pins `GET /` as a single pattern, so a separate fallback route was not an option.
- POST /no-such-page still returns 405 with Allow: GET, HEAD from ServeMux method-mismatch (no `/` catch-all is registered, matching the current merged behavior the task said to keep).
- Favicon decision: TestChromedpUISmoke fails on any browser console error, and the browser's implicit /favicon.ico fetch turned into a 404 console entry once the soft-200 died. The app ships no icon, the inventory forbids a new pattern, so rootIndex answers /favicon.ico with 204 No Content: no console error, no body, no route-table change.
- Evidence: internal/server/notfound_test.go (404 markers + no home leak, home still served at / with queries, favicon 204, POST 405) all red before the dispatcher landed; cmd/qm TestServeMergedUnknownPaths red against the real serveMerged server, green after.
- e2e-hermetic rerun green: route inventory (86 patterns untouched), crawl gate, chromedp smoke, merged hermetic suite.

## Fix 2: cross-link family (t6a F2-F5)

Decisions applied exactly as specified:

- (a) A subsection fragment is a URL fragment, never path-encoded: MuidFromSuttaID strips the `#...` tail for every family before mapping; suttaHref appends the raw fragment after the muid, so "SN 42.8#3" renders href /texts/sn42.8#3 (old output was /texts/sn42.8%233).
- (b) KN citations with fragments behave identically: the KN branch of suttaIDPat gained the same `(?:#\d+(?:\.\d+)*)?` suffix the nikaya branch has, so "KN Dhp 5#2" now canonicalizes whole (display keeps the fragment), muid drops it (dhp5), href /texts/dhp5#2. Previously the fragment was silently dropped from display and link.
- (c) Alias-failing and unmappable ids fall back to suttacentral.net formatted exactly as the pre-merge 1.x code did. Format derived from git history, not invented: a645157 (feat(quote): add SuttaURL helper) pins "the canonical id, lowercased with spaces removed, prefixed with the suttacentral base", and the 1.x renderer at 3e5c0cd~1 linked SuttaURL(id) for every citation; SuttaURL is unchanged since. So KN An 3 -> https://suttacentral.net/knan3, Tv Vi Bu Pj2 -> https://suttacentral.net/tvvibupj2, external with target=_blank rel=noopener. No code change was needed for the format itself; only the mapping exclusions changed who falls back.
- (d) The Tv/Vi/Bu/Pj family is excluded from muid mapping (abbrevVinayaIDRe guard in MuidFromSuttaID, pinned by TestMuidFromSuttaIDExcludesAbbreviatedVinaya). Rationale: bilara keys vinaya texts as pli-tv-* muids, so the tvvibupjN muids this family produced exist nowhere in the canon data and every such link was structurally dead; the suttacentral fallback is the only live target for the abbreviated form.
- pli-tv ids keep their fragment-stripped muid (pinned since before) and now also get the fragment appended in the href (/texts/pli-tv-bu-vb-pj1#5.11.20), uniform with (a). The old special-cased pli-tv branch collapsed into the universal strip + compactLower (pli-tv ids are already lowercase and space-free, so behavior is identical).
- The old "fragment kept in muid" pins were updated to the new requirement: MuidFromSuttaID("AN 5.34#7.9") = an5.34, render pins /texts/an5.34#7.9.
- Corpus invariant: TestSeedCorpusCrossLinksInternally parses exports/shortest-first.md (109 quotes), asserts every citation still maps to an internal muid and renders an internal /texts/ href with zero suttacentral fallbacks. Green, including the pli-tv-bu-vb-pj1#5.11.20 corpus citation.
- checkIdsHandler/runCheck use CanonicalSuttaID only and corpus ids canonicalize through the same pattern on both sides, so the KN pattern extension cannot desync the membership check.

## Fix 3: read verbs must not create databases (t6b G1)

- runQueryQuotes (internal/cli/qmformat.go) os.Stat's the resolved db path (config path or --db override) before store.Open and fails with `quotes: database <path> does not exist: <cause>` naming the path; Main maps that to exit 1; no file is ever created because store.Open is unreachable when the stat fails. The canon query subcommands already fail loudly on the missing pointer (pinned in the same test via a temp DBDir). serve/extract/fixture keep their create semantics (untouched).
- Evidence: TestQueryQuotesMissingDBFailsWithoutCreating red before (exit 0, silent empty db created), green after; asserts error text contains the path and the file still does not exist after both the direct call and the full cli run.

## Fix 4: comments (t6a F8, scoped)

- Every comment stripped from internal/server/handlers.go (19 doc blocks and inline comments, 47 lines). All other touched files were written comment-free per the zero-comments protocol.

## Chain

- make test-pkg PKG="./cmd/qm ./internal/server ./internal/quote ./internal/cli": ok x4 (baseline captured green before any change).
- make e2e-hermetic: ok (internal/e2e incl. route inventory, crawl, chromedp smoke; internal/cli hermetic pass).
- make fmt, make lint (0 issues), make typecheck, make sloc: green.
- make mutate MUTATE_PKGS=internal/quote,internal/server,internal/cli: decisive verdict run 4 (t6fa-mutate4.log, 2026-10-17 ~18:00-18:48) `slots=8 mutants=426 killed=393 invalid=33 survived=0`, MUTATE_EXIT=0. Three earlier attempts died without verdicts and their logs are kept as evidence: t6fa-mutate.log (silent death window, 200/426), t6fa-mutate2.log (stopped on the inverted live-gate defect, 280/426), t6fa-mutate3.log (Go toolchain update to 1.27.2, 380/426). The gate fix (3b09ff5) landed between runs 2 and 3; runs 3 and 4 spawned zero network git children, confirming the isolation.

## Mutate rerun (dev-t6fa2)

- First attempt killed silently in the 15:41-16:32 death window at progress 200/426: no verdict, no process left. Log preserved untouched at t6fa-mutate.log as evidence.
- Second attempt (16:36-17:2x) stopped by main's order at progress 280/426 after the inverted live-gate defect was confirmed; no verdict, whole tree verified gone, log preserved at t6fa-mutate2.log. 16 orphan mutate-* shadow dirs (~2 GB, each holding a cold bilara clone the defect caused) swept from TEMP.
- Run 3 (launched ~17:02 after the gate fix, into t6fa-mutate3.log) died during the user's Go toolchain update to 1.27.2: log frozen at progress 380/426, no verdict, zero mutator/go processes left, task failed out with exit 1. No resume flag in tools/mutator, so the run restarts whole. No git children ever appeared in run 3 (gate fix holding): only hermetic local-fixture commits (e2e@quotes-manager synthetic corpus). No orphan shadows left behind.
- Decisive run 4 relaunched post-all-clear (~18:00-18:48), output to t6fa-mutate4.log, verdict `slots=8 mutants=426 killed=393 invalid=33 survived=0` MUTATE_EXIT=0. Task closed.

## Live gate defect (dev-t6fa2)

- Defect (found by main during the second mutate run): internal/cli/e2e_live_test.go gated TestE2ELive on `QM_E2E_LIVE == "0"`, so an unset env (make test, test-pkg, fuzz, mutate) RAN the live test: writeLiveConfig keeps config.Defaults() production bilara+editions URLs and runSync really clones/fetches them under data/e2e-live inside each mutator shadow tree (24 concurrent git.exe children observed, 100-200 MB per shadow). It also contaminated verdicts: runSync network failure hits t.Skipf, so any mutant detectable only through TestE2ELive's assertions survived on network hiccups.
- Fix (3b09ff5, failing-first): gate extracted into liveEnabled() carrying the old semantics, TestLiveEnabled pinned the required semantics (unset -> false, "0" -> false, "1" -> true, "yes" -> false) and ran red on the two defect cases, then the minimal flip to `== "1"` plus `if !liveEnabled() { t.Skip }` went green. TestE2ELive now skips in 0.00s with env unset. make e2e (QM_E2E_LIVE=1) unchanged by construction.
- f5b60c2 added the make test-one target (named-test runs through make; also the survivor-kill vehicle). First recipe draft lost the `$` anchor to make expansion; corrected to `^$(TEST)$$` per the fuzz targets' pattern.
- Isolation sweep: QM_E2E_LIVE has exactly 2 gates. internal/canon/server/e2e_test.go:594 TestServerLiveSmoke skips when unset (not inverted) but accepts any non-empty non-"0" value, looser than the exactly-1 convention; reported to main, outside this lane. All other sync call sites in tests are hermetic (local fixtures or local git remotes via file:// URLs). Zero unset-env network reaches remain.
- Chain for the fix: test-one TestLiveEnabled red then green, test-one TestE2ELive skip proof, test-pkg ./internal/cli green (70.8s), fmt, lint-pkg 0 issues, typecheck, sloc.

## Reading log

- read docs/qa/v2.0.0-notes/t6a.md, t6b.md full - owned findings and evidence
- read internal/server/{server,handlers,handlers_check,templates}.go full - route table, root handler, check ids, template embed
- read internal/quote/{sutta,render,parse_canonical}.go full + {sutta,render,parse_canonical}_test.go full - id patterns, muid mapping, href rendering, corpus pins
- read cmd/qm/main.go + main_test.go full - merged wiring and live probes
- read internal/cli/{cli,query,qmformat}.go + qmformat_test.go full - verb exit codes, quotes read verb, test harness
- read internal/canon/server/{server,pages}.go + internal/canon/web/web.go + notfound.tmpl full - fallback semantics, page rendering contract
- read internal/e2e/{compose,routes,merged,crawl,smoke,shared}_test.go full - merged composition, 86-pattern pin, crawl gate, console-error gate
- git show a645157 (SuttaURL origin), 3e5c0cd~1:internal/quote/render.go, log of render.go/sutta.go - 1.x fallback format provenance
- grep config.go graph/publish.go - Canon.Paths.DBDir / DBPointerFile for a hermetic canon dir in the cli test

dev-t6fa2 reading log:

- read docs/qa/v2.0.0-notes/t6fa.md full - inherited completed work, decisions, chain state
- read docs/qa/v2.0.0-notes/t6fa-mutate.log full - killed first attempt, frozen at 200/426
- read docs/qa/v2.0.0-notes/t4b.md full - survivor-closure standard and kill-verification discipline
- read docs/qa/v2.0.0-task-board.md 103-114 - protocol, silent-death window, t6fb landing
- grep Makefile mutate/fuzz/fuzz-one/test/test-pkg + read 60-135 - recipe vars, $$ escaping pattern
- read internal/cli/e2e_live_test.go full - inverted gate at the TestE2ELive skip, writeLiveConfig defaults
- grep repo QM_E2E_LIVE + config.Defaults() in *_test.go - isolation sweep for unset-env network reaches
- grep canon/server/e2e_test.go e2eLiveEnv context - second gate, skip-when-unset, looser than =1
- read internal/cli/sync_dpd_test.go 1-60 + canon/sync/sync_test.go 60-90,140-188 - hermetic fixture/fileURL remotes confirmed
- read t6fa-mutate2.log tail - second attempt frozen at 280/426, evidence preserved
