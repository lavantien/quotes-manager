# ui1: merged UI polish lane

Task: restore the 1.x quotes workspace identity under one shared slim nav, keep canon pages on their own design, coherent typography and spacing between the two. No new design language.

## durable progress

- 2026-10-17 lane opened. Cron a50f31fe pings main every 20 min.
- 2026-10-17 change set 1 landed as c94689d (nav restyle + grid offset + canon token mapping, evidence committed).
- 2026-10-17 change set 2: min-height wrap hardening, verified, landed.
- git archaeology done. UI-relevant commits since 1.x: 104be70 merged the servers, 7d51ff4 unified static assets, 662b12e introduced the shared nav (web/templates/nav.html + web/static/nav.css), 8ae72bd htmx node swap fix, f29ddbf vendor pins, 6c2a938 comment strip. Diff 662b12e~1..HEAD over web/templates + web/static touches only nav.css (new), nav.html (new), layout.html (+2 lines: nav.css link + nav template call), canon.css (-15: own .nav block removed), app.js (comment strip). app.css, layout.css, all zone/rail templates unchanged since 1.x: the workspace's own CSS identity is intact, the damage is the injected nav.
- 1.x workspace layout (662b12e~1 layout.html): body > main.page > .layout grid, no nav element at all. Zone heads sticky at top 0 inside 100vh scroll zones.
- canon 1.x (sc-graph): own .nav in canon.css styled with canon variables (--card, --border), brand "sc-graph", links texts/search/words/graph/analytics/pipeline/discrepancies. 662b12e deleted that block and moved both shells onto web/templates/nav.html + nav.css with var() fallbacks.
- nav markup pinned by tests: internal/canon/web/web_test.go:460 (nav-brand qm, overview, texts, discrepancies), cmd/qm/main_test.go:153, internal/server/server_test.go:230, playground/serve-smoke/main.go:168. Any nav restyle must keep these assertions green or coordinate .go test edits after the t6fa2 lane frees.
- serve surface confirmed: dist/qm.exe serve --addr --db seeds quotes from embedded SeedSQL automatically; canon pages read data/e2e-live/db via QM_CANON_DB_DIR + pointer.json (real live-run canon db canon-20261017T082157Z.db present, captured 2026-10-17).
- baseline capture script: docs/qa/v2.0.0-notes/ui/capture-baseline.ps1 (serves real binary + real canon db, chrome headless shots, kills and verifies processes gone).

## baseline evidence

9 shots in docs/qa/v2.0.0-notes/ui/01-baseline-*.png (2026-10-17, real dist/qm.exe serve, embedded quotes seed, real live canon db canon-20261017T082157Z via QM_CANON_DB_DIR, chrome headless, OS dark scheme active). Pixel-sampled, not just eyeballed: nav bg #1e2126 on every surface, workspace body #f7f4ef, link glyphs #0000ce (UA default blue).

## defect catalog

Workspace / (01-baseline-workspace*.png):

- D1 foreign slab. 1.x had no top nav at all (body > main.page > 4-zone grid, verified at 662b12e~1). The shared nav renders near-black #1e2126 (nav.css dark-scheme var fallback) with UA-default blue underlined links and a visited-purple brand on the cream paper workspace. It is the only dark element and the only pure blue in the workspace. In light OS scheme it would render cool gray #f5f5f5/#d9d9d9 on warm cream: still foreign.
- D2 grid squeeze. Zones are height:100vh (layout.css) and the nav adds ~44px of normal flow above them: body overflows, the bottom of every zone and rail is cut off (BACKUP section and 4th quote clipped in the shots), a page scrollbar appears in real use.
- D3 sticky stacking. nav is sticky top:0 z-10; zone heads are sticky top:0 z-2 inside their zones, rails sticky top:0. Any body scroll slides heads and rails under the nav.
- D4 tab-wall reading. 10 equal-weight links (brand qm + workspace + 8 canon routes) in one wrapped-flex row read as a wall of tabs; no grouping between surfaces, no active state, brand and workspace both target /.
- D5 typography mismatch. Nav uses UA default 16px links and its own padding rhythm; the workspace's controlled scale is 0.72 to 0.98rem with clamp() paddings.

Canon surfaces (01-baseline-dashboard/texts/text/words/word/graph.png):

- D6 canon design itself is intact (stat cards, tables, pali serif, dark scheme all match sc-graph 1.x; its own .nav block was deleted in 662b12e but nav.css reproduces the same values via fallbacks, and canon.css's global a rule keeps links accent-blue). What changed: brand sc-graph became qm and workspace+overview links were added. The nav works here because it inherited this surface's palette; the incoherence is cross-surface, not within canon.
- D7 the same nav element is styled by var() fallbacks that resolve differently per surface and per OS color scheme, so there is no single deliberate nav design: dark slab on cream (workspace, dark OS), gray on cream (workspace, light OS), blends on canon. No shared type scale between the shells.

Non-defects confirmed as 1.x identity (not damage, leaving alone): empty collection column before selection, duplicates rail, split zone header + toolbar rows, canon /words resolve-first form, canon graph needs-javascript note, graph not auto-drawing without a param (canon.js draws only when a param value is present).

## fix plan (markup and css only, no .go)

- F1 nav.css restyles .nav as one deliberate slim bar driven by per-surface custom properties: background var(--nav-bg, var(--paper)), foreground/rule/link colors resolving to the workspace palette by default and to canon's --card/--fg/--border/--accent on canon pages via a small :root override added to canon.css. Links explicitly styled (color, no underline, hover), killing UA blue/visited purple everywhere. Brand keeps exact markup (tests pin `<nav class="nav">`, `class="nav-brand" href="/">qm</a>`, overview/texts/discrepancies hrefs).
- F2 fixed slim height: --nav-h (2.5rem) declared by nav.css; layout.css switches .zone height/max-height and .rail max-height/sticky-top and .zone__head sticky-top to calc(100vh - var(--nav-h)) / var(--nav-h), restoring the exact 1.x full-viewport grid with zero body overflow and heads pinned under the nav.
- F3 quiet link treatment: 0.85rem, ink-soft, hover ink, generous gap, brand bold. No pills, no tabs, no active-state machinery (matches sc-graph 1.x plain-link nav).
- F4 comment strip in every file touched (layout.css, nav.css, canon.css if touched).
- F5 verification per change set: make test-pkg PKG="./internal/server ./internal/cli", make e2e-hermetic (chromedp console-error gate), after-shots 02-after-*.png compared in this file.

## change set 1: fix(ui) restore the 1.x workspace grid under a slim shared nav

Files: web/static/nav.css (rewritten: --nav-h token, per-surface --nav-* custom props, explicit link styling, no dark-scheme fallback slab), web/static/layout.css (.rail sticky top and max-height, .zone height/max-height moved to calc(100vh - var(--nav-h)); all comments stripped per the touched-file rule), web/static/canon.css (:root maps --nav-bg/rule/link/link-hover/brand onto --card/--border/--accent/--fg; header comment stripped).

DOM-verified on the real binary at 1440x900 (chrome-devtools, server on 127.0.0.1):
- workspace: bodyOverflow = 0 (doc scrollHeight 900 == innerHeight 900; was 44px overflow), zone rect top 40 bottom 900 height 860, zone internal scroll 74330px (cut cards in shots are internal scroll, the 1.x behavior), rail sticky top 40px, zone head sticky top 0 inside its scrollport.
- workspace nav: bg #f7f4ef, links #5c5750 no underline, brand #1c1a17 weight 700, no UA blue link anywhere (probed every anchor).
- canon /dashboard (dark scheme): nav bg #1e2126 (--card), bottom border #3a3f47 (--border), links and brand #7aa7ff (--accent) weight 700: sc-graph 1.x nav values reproduced exactly.
- one cascade bug found and fixed during verification: `.nav-brand` lost to `.nav a` on specificity, now `.nav a.nav-brand`.

Chain on the final state: make test-pkg PKG="./internal/server ./internal/cli" green, make e2e-hermetic green (86-pattern inventory, CSP equality, 122-target crawl, chromedp smoke with console-error gate). After-shots re-captured on the final binary in 02-after-*.png.

## identity and geometry proof

- 00-1x-workspace.png: true 1.x render (worktree build of 662b12e~1 = 657fe00, its qm-1x.exe served on 8793, embedded seed). No nav of any kind, 4-zone grid flush at y=0, cream/ink/bronze palette.
- 02-after-workspace.png vs 00-1x-workspace.png: identical grid, palette, and column rhythm; the only delta is the 40px slim cream nav (bg #f7f4ef, hairline #ddd6c8 rule) above it, links ink-soft 13.6px no underline, brand ink bold.
- geometry: DOM-measured on the real binary (bodyOverflow 0 at 1440x900, zone rect 40 to 900, rail sticky top 40px) and pixel-measured on 02-after-workspace-scrollbars.png (scrollbars enabled, no --hide-scrollbars): the entire x=1439 right edge is #f7f4ef from y=10 to y=890, no page-level scrollbar gutter. Cut cards and the BACKUP fold are internal zone scroll, the 1.x behavior (zone internal scrollHeight 74330px).
- dark-mode coherence: the old nav.css dark fallback (#1e2126 slab on the light-only workspace) was the defect, so nav.css carries no dark block. Each surface keeps its own scheme: the workspace is light-only by 1.x identity (app.css never had a dark variant), canon stays dark-aware through canon.css's own :root media block feeding --nav-* (measured dark: nav #1e2126, rule #3a3f47, links and brand #7aa7ff bold: exactly sc-graph 1.x nav values).

## chain results

- make test-pkg PKG="./internal/server ./internal/cli": green (2x, before and after the brand specificity fix).
- make e2e-hermetic: green (2x, pre-fix and final state; includes the chromedp console-error gate).
- make fmt/lint/typecheck/sloc: n/a, no Go file touched in this change set.

## change set 2: fix(ui) let the nav grow instead of clipping on narrow wrap

nav.css height -> min-height. At 375px the 2-row wrap already fit the 40px box, but 320px wraps to 3 rows and the fixed height would paint row 3 over the workspace. DOM-verified on the rebuilt binary: desktop unchanged (nav 40, layout top 40, bodyOverflow 0, cache-busted reload), 320x800 emulation nav grows to 54.6px with all links inside the box and layout starting below. Chain: make test-pkg ./internal/server ./internal/cli green, make e2e-hermetic green.

## incident: chrome profile dialog on the geometry shot

The first geometry capture (the inline run that produced 02-after-workspace-scrollbars.png pre-commit) launched chrome with a relative `--user-data-dir=dist\ui-geo\profile` while the PowerShell session cwd was the repo root and Start-Process launched chrome with its own default working directory. Chrome resolved the relative profile path against its own cwd, could not create or lock it there, and surfaced a read/write error dialog that reached the user's desktop; the same stall is why that chrome call exceeded its 60s wait and only wrote the png after the dialog cleared. capture-baseline.ps1 (the durable artifact) already used absolute paths, which is why all its 9-shot runs were clean. Fix applied to the script: GetFullPath on the profile and every screenshot output, explicit New-Item of the profile dir before any launch, and the narrow-width and scrollbars variants folded into the standard shot list so no ad hoc chrome launch is needed again.

## cross-surface type rhythm verdict: no change warranted

Measured on both surfaces with the real binary: nav computed font-size 13.6px (0.85rem) and height 40px identical on the workspace and canon pages, same link treatment and gap rhythm; the nav is the only shared element at the seam and its metrics already match across surfaces. Each surface's internal type scale (workspace 0.72 to 1.05rem rail/zone scale with serif quote bodies, canon 15px base with 1.5rem h1) is its own 1.x identity and stays untouched. Forcing them onto one scale would be a new design language, which the task forbids.

## narrow-width verdict and pre-existing overflow note

At 375px the nav wraps to 2 rows inside the cream bar with zero link crossing the content edge (DOM: max link right 326.9 vs content right 348) and grows instead of clipping (min-height fix). At 320px it wraps to 3 rows, nav height 54.6px, all links inside the box, layout below the nav. Captured as ui/02-after-workspace-375.png.

Separate finding, pre-existing and out of lane: at 375px the page itself has 21px horizontal overflow (htmlScrollW 381 vs clientWidth 360) caused by the left rail's min-content width under the 1.x mobile layout (`.layout { grid-template-columns: 1fr }` tracks size to min-content; rail__row icon chains are the wide unbreakable content). That media block and the rail markup are byte-identical to 662b12e~1, so 1.x had the same mobile overflow. Fixing it means redesigning the 1.x mobile rail flow (minmax(0,1fr) would clip icons instead of scrolling the page), which is new design language, out of this lane's scope. Reported to main for the board.

The stuck shot reruns during this set had a plain script bug: the launch line still passed the automatic `$args` (empty) instead of `$chromeArgs` after the profile-path edit, so chrome started argument-less and never exited. Fixed, plus a 120s bound with one retry per shot for the shared box.

## reading log

- read git log/diff 662b12e, 7d51ff4, 104be70 range - locate the shared-nav origin and the exact 1.x baseline
- read web/templates/layout.html nav.html, web/static/app.css layout.css nav.css canon.css - current markup and both design systems
- read internal/canon/web/templates/layout.tmpl dashboard.tmpl - canon shell and dashboard content
- read internal/e2e/compose_test.go hermetic_test.go smoke_test.go, internal/cli/e2e_hermetic_test.go serve.go, cmd/qm/main.go - how to serve the merged app with data and what the console-error gate covers
- read internal/seed/load.go, internal/canon/source/dpd/fixture.go fixture_test.go - seeding is embedded, dpd fixture shape
- read playground/serve-smoke/main.go - reusable serve recipe (QM_CANON_DB_DIR + QM_SEED_FILE)
- grep internal/canon/web templates + server pages for route shapes - /texts/{muid}, /words/{lemma} confirmed
