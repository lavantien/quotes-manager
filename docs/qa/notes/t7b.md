# T7b adversarial review notes

Reviewer B, independent of reviewer A. Read-only over code, probes under playground/t7b only. Evidence reproduced live on 2026-10-20 against dist/qm.exe built from HEAD (3b1a035).

## findings

### F1 minor: server accepts unbounded group names

internal/server/handlers_groups.go:273 validGroupName only rejects empty names and control characters. web/templates/collection_zone.html:26 caps the input at 100 characters client side only. Repro: POST /collections/1/groups with name of 5000 x characters returns 200 and stores the name (playground/t7b/html_probe.ps1, output create-5000-chars: 200, stored-long-name=True). Fix direction: reject names over the same 100 character limit server side.

### F2 minor: add sutta accepts fabricated muids from unknown families

internal/server/handlers_groups.go:256 normalizes through quote.MuidFromSuttaID only when it succeeds, then quote/sutta.go:69 DisplaySuttaID renders any letters-plus-digits token as a khuddaka sub-book, so unknown families pass. Repro: POST /collections/1/suttas with muid zz9 returns 200, the registry gains zz9, and it appears in ids.txt (playground/t7b/http_probe.ps1, add-bad-family: 200, ids last=zz9). The seed pipeline rejects unknown families as a hard error, so the two ingest surfaces disagree. Fix direction: validate the family against the known set in suttaRef, or document the route as intentionally permissive.

### F3 minor process: comments added to a touched file

internal/store/export.go carried 13 comment lines before the wave and carries 20 after, the diff added 7. The protocol requires zero comments and removal on touch. Fix direction: strip them in the fix slot.

## attacked and could not break

Ingest integrity: I re-derived the whole table from dumps/ariyasavaka-curated-list.md with an independent parser (playground/t7b/parse_probe.go) written from the artifact alone. It yields 471 suttas, 16 groups, sizes 86 60 20 12 14 2 6 5 4 1 24 228 1 2 1 5, first mn26, last ud1.10, skipping only the whole-collection dhp and pitaka tree links. Per muid comparison against the live database shows zero group mismatches and zero set differences. The dhp members use dotless muids (dhp209) which matches both the artifact slugs and the graph db (dhp209 length 8). Duplicate handling is first occurrence wins and pinned correctly: an2.9 sits in group II, the MN 152 label links mn145 so no mn152 row exists. Displays keep the author's label when text and slug disagree (an3.64 renders as AN 3.36) and derive per member inside ranges.

Range edge cases that shaped the probe, all handled by the implementation: dotless range heads (dhp209-220 expands to 12 members), multi hyphen vinaya slugs kept whole (pli-tv-bu-vb-pj1, pli-tv-bu-vb-pj3), inline links on section heading lines (groups VI to XVI), and the escaped 6. and 8. pseudo headings folding into group XI.

Dump round trip: playground/t7b/roundtrip_probe.go copies the live database, exports, imports, exports again, and the two dumps are byte identical (109 quotes, 1 collection, 471 suttas, 16 groups, 471 sutta items, 408 lengths). A version 1 dump imports with groups and suttas dropped and quotes kept. Version 3 and nil dumps return ErrUnsupportedDump.

Http lifecycle: playground/t7b/http_probe.ps1 drove the 8 new routes on a scratch database through a real server on 127.0.0.1:8093. Create 200, empty name 400, control character name 400, rename by form and by percent decoded HX-Prompt both 200, move up and down 200, assign sutta to group 200, missing group 404, missing collection 404, remove sutta 200, delete group 200, rename deleted group 404. The collection page renders sections in position order with the ungrouped section (id group-0) last, 18 sections at the time of the probe including the two my probe created. ids.txt carries quote ids then deduped sorted muids. The server process died on stop and the port freed.

Length data path: the live database holds 408 known lengths and exactly 63 unknowns, 60 an1 muids plus ja360, ja402, vv51. an1.505 proves the boundary case: its english file exists (an1.394-574_translation-en-sujato.json) but every key sits under the an1.505-514 range, so the muid itself has zero english segments and stays unknown, which is honest rather than a miss.

Test honesty: no skipped tests in the diff. internal/sutta/parse_real_test.go:70 hardcodes the sizes, first, last, and exact first link from the manual derivation, and my independent derivation reproduces the same numbers, which cross validates the pins. internal/e2e/groups_test.go pins 471 items and the first and last group names from the seeded artifact. The fakeStore is an in-memory state machine asserting handler interaction, not a delegate of the real store. Mutation logs check out: store 235 mutants 219 killed 16 invalid 0 survived, server 216 mutants 208 killed 8 invalid 0 survived (docs/qa/logs/store-mutation-gate.log, server-mutation-t5.log).

Probe artifacts, not findings: my percent encoded NUL body was double encoded by the client so it tested a literal string, the fuzz corpus already covers the real byte. My ids sorted check sorted the whole file across its two independently sorted segments. My first html probe guessed a class name that does not exist.

## reading log

read dumps/ariyasavaka-curated-list.md lines 1-70, XII slice, bold heading list - independent re-derivation of sections, items, links, ranges
read internal/server/handlers_groups.go full - route payload parsing, validation, prompt decoding
read internal/store/export.go full - dump v2 shape, import trust boundaries, transaction ordering
read internal/quote/sutta.go grep MuidFromSuttaID, DisplaySuttaID - normalization and display fallback rules
read web/templates/collection_zone.html grep classes and form - client side name cap, toolbar wiring
read web/templates/collection_list.html grep group markup - section ids, ungrouped section, selects
read internal/server/server_fakestore_test.go grep SetSuttaGroup - fake semantics spot check
read internal/sutta/parse_real_test.go grep pins - expectation provenance
read internal/e2e/groups_test.go grep pins - seeded view expectations
read docs/qa/logs/store-mutation-gate.log, server-mutation-t5.log tail - survivor verdicts
