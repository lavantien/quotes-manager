# v2.0.0 quality baselines

Full-chain quality baseline for the v2.0.0 absorption: code HEAD 197c975, docs-only commits after, date 2026-10-17. Every stage ran through its make target on the remediated tree (post t6a/t6b wave, live-gate fix, UI restoration, E2 screenshots). Logs live in docs/qa/v2.0.0-notes/, full stage history in t5v3.md. Test targets run wrapped with GOMEMLIMIT bounds and the jobwrap supervisor.

## stage results

| stage | headline numbers | wall | log |
| --- | --- | --- | --- |
| 1 test | 30/30 test packages, 956 top-level + 678 subtest passes, 0 fail | 45s | t5v3-test.log |
| 2 memtest | 3/3 targeted tests under GOMEMLIMIT=512MiB | ~77s | t5v3-memtest.log |
| 3 static | fmt, lint, typecheck, sloc all green | 25s | t5v3-static.log |
| 4 fuzz | 25/25 targets, 0 FAIL, 8,937,664 execs | 10m16s | t5v3-fuzz.log |
| 5 mutate | see mutation coverage below | | |
| 6 e2e-hermetic | internal/e2e 7/7 top-level PASS (crawl 7.81s, chromedp ui smoke 6.59s), cli TestE2EHermetic PASS, 0 fail 0 skip | 45s | t5v3-e2e-hermetic.log |
| 7 e2e live | TestE2ELive PASS, 7041 findings matching the C1 reference, live graph 3344 segments | 18s | t5v3-e2e-live.log |

## mutation coverage

Zero-survivor verdicts cover all 14 mutated packages. Verdicts transfer across runs because the package code, its test suite, and tools/mutator are byte-identical from each decisive run to HEAD 197c975 (verified per package from git history; tools/mutator unchanged since f26b34b 01:26; zero code commits after HEAD). Only internal/cli changed after its last verdict (01a1ad8 19:35 post-dates t6fa's 18:48), so it alone was re-run fresh.

| packages | verdict | evidence |
| --- | --- | --- |
| norm, bilara, dpd, triangulate, canon/server, jobs, analytics, export, search | survived=0 in the 14-package run (mutants=1504, survivors all in server+cli) | t4-chain-mutate2.log, 11:14 |
| store, editions | survived=0 (mutants=295) | t6fb-mutate2.log, 15:39 |
| quote, server | survived=0 (mutants=426) | t6fa-mutate4.log, 18:48 |
| cli | survived=0 (slots=2 mutants=166 killed=161 invalid=5), fresh delta run at 2 slots x 2GiB under the 8 GB accumulation cap, own-lane footprint max 3.39 GB | t5v3-mutate8.log, 07:24:55 |

Full rerun history with the cap restructure and foreign-burst evidence: t5v3.md findings.

## live e2e notes
