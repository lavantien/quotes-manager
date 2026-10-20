# Sutta tables and collection groups task board

Wave goal: ingest the AriyaSavaka curated sutta table from the crawled reddit dataset into the quotes database as a named collection with semantic groups, and ship collection groups as an app feature.

Source of truth: `dumps/ariyasavaka-curated-list.md` (verbatim post body, provenance in `dumps/ariyasavaka-curated-list.json`, commit bb83190). 16 roman-numeral sections in gradual training order, 107 numbered items, 261 sutta markdown links, 44,688 bytes.

Design decisions: suttas are a global registry table; collections gain named ordered groups; quote items and sutta items both carry an optional group id; within a group the document order is the ordering (author's gradual approach encodes similarity and relationship); no length data exists offline so shortest-on-top is honored by the table's own generally-shortest-first structure; seed.sql is the only ingest surface (local db verified identical to seed: 109 quotes, 1 collection, 3 categories); sample collection seeding is removed once AriyaSavaka seeds. The muid is the operative handle for our own CLI: T4 adds `qm query suttas` (registry and per-collection listing, ids feed `qm query segments --muid`) and T5 makes `/collections/{cid}/ids.txt` carry the collection's sutta ids alongside quote ids.

First launch attempt (2026-10-20 18:20) landed all three agents in a web-only toolset with no file or shell tools; one agent probed a host escape through a browser MCP before being stopped, both repos verified clean afterwards. The 18:40 relaunch on the general-purpose type hit the same wall: in this harness every non-fork agent type draws a web-only toolset from its definition. Resolution 19:00: all lanes relaunched as forks (they inherit the parent toolset and model), same working tree, file-disjoint lanes, index.lock retries on commit contention.

| task | lane | files owned | depends | status |
|---|---|---|---|---|
| T1 dataset grab | descoped 2026-10-20 19:35: the dataset repo only ever needed to yield the id plus link table, which is already committed as dumps/ariyasavaka-curated-list.md; the repo was reset to 2b57cfe clean and needs nothing further | none | done |
| T2 sutta package | quotes-manager | internal/sutta/** | dump committed | done 20:20, commit 45aae98: contract delivered, 16 groups and 471 suttas pinned from manual derivation, fuzz 2 real finds fixed, mutation zero survivors, artifact-forced deltas recorded in docs/qa/notes/t2.md |
| T3 store groups+suttas | quotes-manager | internal/store/**, internal/server/server_fakestore_test.go stubs only | none | running, length column delta delivered |
| T4 extract+seed wiring | quotes-manager | internal/cli/extract*.go, internal/seed/**, database/seed.sql | T2 | queued |
| T5 server groups UI | quotes-manager | internal/server/** except fakestore stubs, web/templates/** | T3, T4 seed names | queued |
| T6 docs and artifacts | quotes-manager | README.md, docs/**, project-pictures/** | T4, T5 | queued |
| T7 adversarial review | read-only plus fix slots | none | T3, T4, T5 | queued |

Progress lives in docs/qa/notes/t2.md through t6.md, one note per task, latest state at the bottom.
