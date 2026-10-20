# T7a adversarial review notes

Reviewer A, independent of reviewer B. Scope git diff bb83190..HEAD, read-only over code, findings proven at a real boundary where possible.

## findings

1. major, groupless collections hide suttas in the ui. internal/server/views.go:181 falls into the flat branch when a collection has no groups and drops suttas entirely, web/templates/collection_list.html collection_flat renders quotes only. Live probe: POST /collections then POST /collections/2/suttas muid=mn26 returns 200 and ids.txt lists mn26, but GET /collections/2 shows no suttacentral.net/mn26 anchor and prints No quotes here. After creating one group the anchor appears. A user following create collection then add sutta sees nothing. Fix direction: render sutta rows in the flat branch or reuse the grouped renderer with the implicit ungrouped section when sutta items exist.

2. minor, store reorder accepts partial and duplicate id lists. internal/store/sqlite_collections.go:129 ReorderCollectionGroups and :307 ReorderCollectionSuttas rewrite only the ids they receive. Probe playground/t7a_reorder: ReorderCollectionGroups(cid, [last id]) returns nil and leaves two groups at position 1, ListCollectionGroups then returns order a,c,b. No in-tree caller passes a partial list today, moveGroup builds the full list, so the hole is latent in a public interface method. Fix direction: verify inside the tx that the id set equals the collection's current set, or stage positions to negative values before writing final ones.

3. minor, unbounded group name length server-side. internal/server/handlers_groups.go:273 validGroupName rejects control characters only and the route has no body cap, the only MaxBytesReader in the app is the backup restore at internal/server/handlers_backup.go:33. Live probe: POST /collections/2/groups with a 200000 character name returns 200 and the zone echo reaches 404448 bytes. The template maxlength=100 is trivially bypassed. Fix direction: cap the name in validGroupName with the constant in the config hub, or add a body limit to mutating routes.

4. minor, ungrouped placement disagrees between surfaces. internal/cli/suttas.go:147 renders the ungrouped sutta block first, internal/server/views.go:254 renders the ungrouped section last. The same collection orders its sections differently on qm query suttas versus the web app. Fix direction: align the cli to ungrouped last.

5. nit, collection search ignores suttas. views.go:169 filters quote items only while the search box in web/templates/collection_zone.html:20 implies collection-wide search, sutta sections always render in full.

6. nit, registry upsert never updates display or link. sqlite_collections.go:276 INSERT OR IGNORE keeps the first row, re-adding a muid with a corrected link stays silent.

7. nit, Import trusts dump group ids. internal/store/export.go Import writes item group ids without checking they exist in the collection's groups, a hand-edited backup yields dangling ids that render as ungrouped with a select showing no matching option.

8. nit, moveGroup is read then write across two requests, two concurrent moves can lose one swap. SQLite keeps each transaction consistent, worst case is a lost reorder in a single-user local app.

9. nit, extract seeds tables on any parsed dumps/*.md even with zero groups, then RenderSeedSQL still emits the deletes and the collection insert, a stray markdown file in dumps/ would reset collections on the next reseed. Guard on total groups greater than zero.

## what was attacked and held

Requirement fidelity re-derived from dumps/ariyasavaka-curated-list.md with an independent python parser in playground/t7a_rederive.py: 16 sections I to XVI, sizes 86, 60, 20, 12, 14, 2, 6, 5, 4, 1, 24, 228, 1, 2, 1, 5, total 471, first mn26, last ud1.10, seed item order in database/seed.sql equals document order, first occurrence wins dedup across sections. My 3 initial divergences were probe bugs: range slugs failed my family gate, chapter inheritance for an2.130-140 means an2.130 through an2.140, and the digitless whole-collection slug dhp is deliberately ignored. All resolved by url semantics without reading the implementation.

Also held: the dump is CRLF free, control-character group names rejected with 400 at the real boundary, the legacy group_id migration is an idempotent PRAGMA plus ALTER TABLE read directly, DeleteCollectionGroup nulls members in both item tables, DeleteCollection cascades suttas and groups, dump v2 round-trips lengths and accepts v1, e2e literals 471, 17 sections and the group names trace to the artifact, SortShortestFirst is stable with known lengths ascending, unknowns after and position ties.

## reading log

read git log and diff bb83190..HEAD stat and per-file - enumerate the wave surface
read internal/sutta/slug.go full - family gating, range expansion, normalize
read internal/sutta/parse.go full - heading and link extraction, dedup policy
read internal/sutta/seed.go full - delete ordering, id assignment, single collection render
read internal/store/sqlite_collections.go full - group crud, membership, reorder, reads
read internal/store/sqlite.go diff - schema, group id migration, count sums, cascade
read internal/store/export.go diff - dump v2 shape, import path
read internal/store/store.go grep SortShortestFirst and types - ordering comparator
read internal/server/handlers_groups.go full - 8 routes, validation, prompt decode
read internal/server/views.go full - pane assembly, section building, filtering
read internal/server/server.go full - route table, body caps
read internal/server/handlers_ids.go full - ids.txt composition
read web/templates/collection_zone.html and collection_list.html full - flat versus grouped rendering
read internal/cli/suttas.go full - lengths verb, query formats, ungrouped placement
read internal/cli/extract.go and internal/seed/load.go diffs - seed assembly, sample removal
read internal/e2e/groups_test.go full - expectation independence
read playground/t7a_rederive.py, t7a_diff.py, t7a_reorder/main.go - my own probes
grep dumps/ariyasavaka-curated-list.md sections I and XII - artifact cross checks
regex-count database/seed.sql - generated seed verification
