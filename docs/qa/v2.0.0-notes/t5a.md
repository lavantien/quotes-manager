# T5a notes: repo-scope the memory monitor

- 2026-10-17 task start. Mission: memcapmon supervision must never match or kill processes from other repos (the 12:58:08 caro-ai-pvp kill came from name matching across the whole box), and the orphan gap on plain test targets must close via the dist jobwrap artifact.
- memcapmon changes (tools/memcapmon/main.go, all windows-tagged):
  - new `-root <absolute path>`: supervision becomes exclusively tree-scoped, zero name matching. A process is supervised iff its own executable image path is under root, or it is a descendant of such a process via parent-PID walk over the point-in-time snapshot. Relative -root is rejected at startup. The monitor's own ancestor chain stays excluded exactly as before (protectedSetWithSelf inside supervisedRooted).
  - image paths come from the OS: QueryFullProcessImageNameW over the same PROCESS_QUERY_LIMITED_INFORMATION handle the ws query already opens; command lines are never parsed. Access-denied leaves exe empty, which can never satisfy underRoot, so protected/system processes are unsupervisable by construction.
  - new `-list`: one snapshot, supervised set printed as `pid\tws\tname\texe` sorted by pid, plus a total line on stderr, then exit. Works in both modes.
  - pure functions added for the rule: normalizePath (backslash->slash + lowercase), underRoot (separator-boundary prefix test, so C:\repo\qm2 and C:\repo\qm.exe never fall under root C:\repo\qm), rootedSet (children adjacency + DFS from image-under-root seeds, revisit-guarded), supervisedRooted (rootedSet minus self chain). collectSupervised unifies root mode and legacy name mode for both the loop and -list; protectedSet wrapper deleted (unused after the refactor, its test already targeted protectedSetWithSelf).
  - kill path unchanged: total supervised ws over cap -> pickLargest -> killTree over the snapshot, survivor check after.
- Makefile changes:
  - WRAP is now the built artifact `$(CURDIR)/dist/tools/jobwrap$(EXE)` (EXE block moved above it) instead of `go run ./tools/jobwrap`; rule `$(WRAP): $(wildcard tools/jobwrap/*.go) $(wildcard internal/canon/winjob/*.go)` builds it once, make skips it while fresh, clean removes it with dist.
  - test and test-pkg now run under `$(CGO_ENV) $(WRAP) go test ...` (they were the only spawning targets without the wrapper), so a reaped wrapper takes its whole tree down instead of stranding orphan test binaries. Every WRAP-using target (test, test-pkg, race, fuzz, fuzz-one, memtest, e2e-hermetic, e2e) gained the `$(WRAP)` prerequisite.
  - monitor target now passes `-root $(CURDIR)`, so the standard invocation is already repo-scoped: `CGO_ENABLED=1 go run ./tools/memcapmon -cap 16GB -poll 2s -for 0 -root C:/Users/lavantien/dev/github/quotes-manager`.
  - per the zero-comments rule every standalone Makefile comment was removed; the `##` help annotations stay because the help target parses them as data. The removed prose (fuzz worker pinning rationale, UCRT toolchain note, vendor pin policy) now lives only in git history, flag for a readme pass if anyone needs it back.
- TDD: tests first (TestUnderRoot, TestRootedSetIncludesDescendantsExcludesForeign, TestSupervisedRootedExcludesSelfChain), red run logged at t5a-red.log (undefined underRoot/rootedSet/supervisedRooted, unknown exe field), then implementation, fresh green `make test-pkg PKG=./tools/memcapmon -count=1` 1.356s logged at t5a-green.log. Expectations were derived from the supervision spec, not from the implementation.
- Chain green in order after the final restructure: make test-pkg ./tools/memcapmon -count=1 (1.356s), make test-pkg ./tools/memcapmon ./tools/jobwrap (4.623s, kill-on-close test included), make fmt (no diffs), make lint (0 issues), make typecheck, make sloc. Logs: t5a-green/-lint/-typecheck/-sloc/-tools-tests/-wrap.log.
- Functional proof (t5a-proof-list.log, t5a-proof-monitor.log, t5a-proof-namemode.log):
  - built dist/tools/jobwrap.exe via the new rule (3356160 bytes), ran `make test-pkg PKG=./internal/cli` under it, and a foreign decoy `go run` sleeper with a GOROOT image and no repo ancestry alive at snapshot time.
  - monitor `-list -root <repo>` listed exactly 3 pids: jobwrap.exe (repo image) plus its two go.exe descendants (one from the GOTOOLCHAIN module cache path, ancestry-caught). Decoy pid 59588 alive during the snapshot and absent. An independent oracle (Win32_Process parent-PID walk, image-under-root test) classified every listed pid as repo-origin: 0 foreign. Positive controls: wrap pid and go-child pid both present; negative control: decoy leak count 0.
  - `make monitor MONITOR_FOR=6s` boots in root mode and exits at deadline (t5a-proof-monitor.log).
  - name-mode regression contrast: with no -root, -list still sweeps by name, and the live box showed caro-ai-pvp's server.test.exe processes plus a no-ancestry decoy go.exe as supervised (first capture 13:16, rows included 4 foreign server.test.exe and 2 foreign make.exe). Old mode intact, new mode closes the boundary.
- Known limitations, recorded not fixed: procInfo.cmd is still never populated, so name mode remains effectively name-only (pre-existing, left unchanged because the -match contract is untouched); PPID staleness within one snapshot can in principle graft a reused pid onto a repo tree, the same point-in-time hazard killTree already carries.
- Deviations from the brief: none functional. The Makefile comment strip is the zero-comments rule applied to a touched file, with the ## help annotations kept as parser data. Adversarial pass was self-performed (edge cases above) rather than fanned out, per slot discipline on this fan-out; the lead's chain defines done here.
- Monitor executions used: two -list runs, one 6s make monitor run, one -list in name mode. No long-lived monitor launched, monitor stays DOWN by design.

Reading log:
- read tools/memcapmon/main.go full - existing name-match supervision, killTree, protectedSet, snapshot
- read tools/memcapmon/main_test.go full - existing pure-function test style, pwsh-based killTree test
- read tools/jobwrap/jobwrap.go + jobwrap_other.go full - job object wrap primitive, non-windows passthrough
- read tools/jobwrap/jobwrap_test.go full - kill-on-close test harness pattern
- read Makefile full - WRAP definition, spawning targets, monitor target, vendor-assets self-modifying pins
- read docs/qa/v2.0.0-notes/t4b.md 1-40 - notes house format
- glob internal/canon/winjob/*.go - wrap rule prerequisites
- grep golang.org/x/sys zsyscall_windows.go - confirmed QueryFullProcessImageName(proc, flags, exeName, size)
