# T4b notes: close the 14 server/cli mutation survivors

- 2026-10-17 task start. Mission: kill or provably-remove the 14 survivors from the clean-shadow mutate run (t4.md line 21), then rerun `make mutate MUTATE_PKGS=internal/server,internal/cli` to survived=0.
- Survivor triage after reading every site (line numbers re-located in current tree, zero drift):

| site | flip | observable behavior | closure |
| --- | --- | --- | --- |
| internal/server/api_quotes.go:62 `end-start` cap | - -> + | make cap is allocation-only; rows content and JSON identical; end+start >= 0 so no panic path differs | REMOVE (equivalent mutant, restructure to `make(..., 0, end)`) |
| internal/server/handlers.go:425 `cat > 0` | > -> <= | delete-HTMX root_count then always shows rail.TotalQuotes instead of the active category's count | kill: TestDeleteHTMXRootCountTracksCategoryFilter |
| internal/server/handlers.go:426 `qerr == nil` | == -> != | category count ignored on success, used on error | kill: same test |
| internal/server/handlers.go:521 `err != nil \|\| id <= 0` (parseID) | \|\| -> && | parses-ok-but-nonpositive ids (0, -3) accepted as valid | kill: TestUpdateRejectsNonPositiveID |
| internal/server/handlers_duplicates.go:30 `len(g) > 1 && g[0] == repID` | && -> \|\| | singleton group of repID no longer 404s | kill: TestMergeDuplicatesSingletonRepNotFound |
| internal/server/server.go:95 (render) err != nil | != -> == | render failure goes unlogged (success logs noise) | kill: TestRenderLogsExecuteFailure |
| internal/server/templates.go:27 `add` func a+b | + -> - | insert-gap data-pos positions go negative/off-by-N | kill: TestCollectionViewInsertGapPositions |
| internal/server/views.go:303 (exec) err != nil | != -> == | exec failure goes unlogged | kill: TestExecLogsExecuteFailure |
| internal/cli/coverage.go:25 `fs.NArg() > 0` | > -> <= | no-arg invocation rejected with `unexpected argument ""` | kill: TestRunCoverageNoPositionalArgsSucceeds |
| internal/cli/coverage.go:47 markers `&&` | && -> \|\| | one-sided marker readme reported as "already current" instead of "no coverage markers" | kill: TestRunCoverageOneSidedMarkerReportsNoMarkers |
| internal/cli/coverage.go:56 WriteFile err != nil | != -> == | badge-updated notice skipped on success | kill: TestRunCoverageReportsBadgeUpdated |
| internal/cli/screenshot.go:96 `bin != ""` ExecPath guard | != -> == | which allocator options get built | REMOVE (equivalent mutant, see below) |
| internal/cli/screenshot.go:111 chromedp.Do err != nil | != -> == | successful nav returns (nil, nil); failed nav proceeds to shot | kill: TestCapturePageRunsNavigateThenShot + TestCapturePageNavigateErrorSkipsShot (check moved into capturePage seam) |
| internal/cli/screenshot.go:126 `runtime.GOOS == "windows"` | == -> != | windows install-path scan gated on the wrong OS | kill: TestFindBrowserPathScansWindowsInstallPaths (check moved into findBrowserPath(goos, stat) seam) |

- Equivalence arguments for the 2 removals:
  - api_quotes.go: invariant = the make capacity argument affects allocation only. The observable value of `rows` (length, elements, JSON encoding) is independent of the cap; `end+start` is always >= 0 (both operands are bounded by total >= 0), so no makeslice panic path differs. Restructured to `make([]store.Quote, 0, end)`, a flip-free bound that is still >= the final length.
  - screenshot.go: invariant = chromedp maps an empty execPath to its built-in browser search. chromedp@v0.21.0 allocate.go:61-62 `if ep.execPath == "" { ep.execPath = findExecPath() }` runs in setupExecAllocator, which NewExecAllocator uses; `chromedp.ExecPath("")` sets execPath = "" because exec.LookPath("") fails, so appending `ExecPath(browserPath())` when the path is empty is output-identical to appending nothing. Guard deleted, single unconditional append.
- Test seams added (plain, no new deps): capturePage(ctx, navigate, shot) factors the Do-then-Run error contract out of captureScreenshot; findBrowserPath(goos, stat) factors the OS gate and install-path scan out of browserPath; both keep behavior byte-identical (cancellation order in browserContext preserved: timeout, browser, alloc).

Reading log:
- read docs/qa/v2.0.0-task-board.md full - protocol, history, slot rules
- read docs/qa/v2.0.0-notes/t4.md full - survivor list verbatim, kill-verification standard
- read tools/mutator/main.go full - flip set, kill criteria, package scoping
- read internal/server/api_quotes.go full - survivor 1 site (make cap)
- read internal/cli/coverage.go full - survivors 9-11 sites
- read internal/server/handlers.go 340-400,395-544 - delete htmx branch (425/426), parseID (521), badRequest
- read internal/server/handlers_duplicates.go full - survivor 5 site
- read internal/server/server.go 1-99 - routes, render (95)
- read internal/server/templates.go 1-57 - add func (27), template list
- read internal/server/views.go 195-306 - buildRootPane/railData semantics, exec (303)
- read internal/cli/screenshot.go full - captureScreenshot/browserPath (96/111/126), screenshotCapture seam
- read internal/cli/coverage_test.go full - existing coverage tests, stdout swap pattern
- read internal/cli/screenshot_test.go full - existing browserPath tests, capture stubs
- read internal/coverbadge/coverbadge.go full - Pct/RenderBadge/marker semantics for the one-sided-marker case
- read internal/cli/discrepancies.go 1-45 - printLine/stdout var
- grep web/templates add/root_count - insert-gap data-pos contract, root_count span shape
- read web/templates/root_zone.html full - root_count fragment bytes
- read internal/server/server_test.go 1-120 - newServer/do/sampleQuote helpers
- read internal/server/server_fakestore_test.go 1-130,340-460 - fake List/Delete/MergeQuotes/CategoryQuotes/tags
- read internal/server/views.go 195-269 - railData.TotalQuotes = full library size
- read internal/server/server_collections_test.go 1-55 - fakeWithCollection (collection of 2)
- read internal/server/handlers_duplicates_test.go full - existing merge tests, fixture style
- read internal/server/server_categories_test.go 1-30 - fakeWithCategory
- grep chromedp@v0.21.0 allocate.go ExecPath/findExecPath/setupExecAllocator - equivalence proof for guard removal
- grep chromedp@v0.21.0 action.go/screenshot.go - Action[T]/Run/Do signatures for the seam
- read Makefile grep test-pkg/lint-pkg/mutate - chain entry points
