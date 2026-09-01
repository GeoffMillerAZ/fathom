# catalog — a kit for building catalogs that cannot lie

**Status:** adopted design, owner-commissioned 2026-07-30 ("can the agentic
kit be uplifted to help be a kit for incredible, hierarchical (dynamic depth
with no max?) catalogs with groups and sections and whatever you think an
amazing catalog would be. including aid tools and compute and libs and
determinism"). Scoped by the kit's own D13 rule: two consumers make it kit
business.

## 1. The evidence that this is real, not speculative

The fleet already maintains at least four catalogs, each rebuilding the same
machinery badly and independently:

| Catalog | Depth today | How it drifts |
|---|---|---|
| a licensed component corpus (657 components, 10,452 variants) | 4 levels + 3 facet axes (framework × version × mode) | indexed separately, search reimplemented |
| this kit's own package catalog (187 packages) | 2 levels | generated, but the count lives in three files that must agree by hand |
| a consumer's MCP tool catalog (87 tools) | 2 levels (group, tool) | hand-maintained; it silently drifted until an audit reconciled 83/83 |
| a consumer's layer catalog | 1 level + eval/production status columns | status columns hand-copied from run output |

The recurring failure is not "we lack a tree structure." It is that **every
one of these catalogs is capable of lying** — claiming an entry that no
longer exists, omitting one that does, or reporting a count nobody checked.
A catalog's whole value is being trusted without verification, so a catalog
that can lie is worse than no catalog.

**Therefore the kit's contribution is not the tree. It is the drift gate.**
Everything else here exists to make that gate possible.

## 2. What an amazing catalog is (the model)

Four orthogonal ideas, deliberately separated because conflating them is why
hand-rolled catalogs get stuck:

1. **Hierarchy — arbitrary depth, no maximum.** A node has children; that is
   the whole rule. Sections, groups, subgroups, items nest as deeply as the
   domain wants. Depth is data, never a type. (`Section > Category >
   Subcategory > Component` is four; a taxonomy that later needs six must not
   require a schema change.)
2. **Facets — the axes an item varies along.** One component × framework ×
   version × theme = many variants of *one* item. Facets are not tree levels;
   modeling them as levels is the mistake that makes catalogs explode. An
   item carries a facet map; a variant is an item pinned on those axes.
3. **Derived views — computed, never stored.** Counts, rollups, "N packages
   in M groups", coverage-by-facet: all recomputed on read from the node set.
   No second source of truth (the same principle that governs derived
   patterns in the consumer's findings design).
4. **Provenance and freshness per node.** Where an entry came from (source
   path, extractor, watermark) and whether it is still true. A node without
   provenance cannot participate in the drift gate, so provenance is
   required, not optional.

## 3. Determinism (a contract, not an aspiration)

- **Stable node identity.** A node's ID is the slash-joined path of its
  ancestors' slugs, and a `stable_key` may be declared to survive renames;
  the ID is what everything else references, so reorganizing a catalog must
  not silently orphan links.
- **Byte-stable rendering.** Same node set → same output bytes, always:
  sorted children (by explicit `order` then slug), sorted facet keys,
  injected clock, no map iteration in any renderer. A generated catalog file
  that churns on every run cannot be diffed, and a catalog nobody diffs is a
  catalog nobody audits.
- **Content-addressed node hash** over (id, title, facets, provenance) so
  "did this entry change?" is answerable without reading the whole tree.

## 4. The drift gate (the reason this kit exists)

A catalog is built from a **source of truth** — a directory scan, a
`go list`, a tool registry, a corpus index. The gate is one function:

```go
func Verify(built Catalog, published Catalog) Drift
```

`Drift` names, per node: **missing** (published claims it, source lacks it),
**extra** (source has it, published omits it), **changed** (hash differs),
**moved** (same `stable_key`, different path). It is a value, not a bool —
"clean" is an empty Drift with a count, so a passing gate can still print
what it checked.

Consumers wire it where their existing gates already are: a test that fails
when `docs/catalog.md` disagrees with a source scan; a pre-push check; a
`doctor` row. **A published catalog with no drift test is documentation, and
this kit's position is that documentation is not a catalog.**

## 5. Query and search (honest, pluggable, no embeddings)

- **Traversal:** by path, by ancestor, by facet predicate, by depth range.
- **Lexical search:** BM25 over title/synopsis/facet values, deterministic
  tie-breaking, the score exposed in results — a caller that cannot see the
  score cannot judge the match.
- **Ranker seam:** `Ranker` is an interface. A semantic/vector ranker is a
  consumer's choice, injected, never a kit dependency — the kit's standing
  evidence-gated position on embeddings applies unchanged.
- **Empty is an answer.** A no-match returns "no match" with what was
  searched, never a silent nearest-neighbour. (The kit's refusal doctrine,
  applied to search.)

## 6. Aid tools (the surfaces that make it usable)

- **CLI helpers** a consumer can wrap: `build`, `verify`, `render`, `query` —
  provided as library functions plus a thin optional `cmd/` example, so a
  consumer's own binary owns its verb names.
- **Renderers:** markdown (the generated-doc case, with a DO-NOT-EDIT banner
  and the regeneration command in it — the convention this kit already
  follows), JSON (machine consumers), and a compact tree for terminals.
- **MCP shape:** a description of the tool contract (`catalog_search`,
  `catalog_get`, `catalog_tree`) that a consumer can register on its own
  server. The kit ships the shape and the handlers-as-functions; it does not
  ship a server (the kit does not own anyone's surface).
- **CUE schema** for authored catalogs, since a hand-written taxonomy wants
  validation, plus loaders for the generated case.

## 7. Compute

Rollups, coverage-by-facet, depth histograms, orphan detection (a node whose
parent path does not resolve), and duplicate-slug detection — all pure
functions over the node set, all tested table-driven, all recomputed rather
than cached. Cost is linear in nodes; catalogs in this fleet are thousands of
nodes, so no index is warranted and none is built until a consumer measures a
problem.

## 8. Non-goals

No storage engine (a catalog is built from a source of truth and rendered; if
someone needs it persisted, they persist the rendered form). No network, no
registry, no publishing protocol. No embeddings in the kit. No UI. No
ownership of consumer verb names or MCP surfaces. No cross-catalog
federation in v1 — the consumer's module-host federation is the precedent to
follow if it is ever wanted, on evidence.

## 9. Deliverables

1. `foundations/catalog` — the model (arbitrary-depth `Node`, facets,
   provenance), stable IDs, content hashes.
2. Deterministic renderers (markdown / JSON / tree) with byte-stability tests.
3. `Verify` + the `Drift` value type, with the four drift kinds.
4. Traversal + BM25 search + the `Ranker` seam; empty-is-an-answer.
5. Compute: rollups, coverage-by-facet, orphan and duplicate detection.
6. CUE schema for authored catalogs + loaders for generated ones.
7. The MCP tool shape (handlers as functions, no server).
8. Docs: package doc as README (kit convention), a worked example building a
   small catalog from a directory scan and gating it.

## 10. First consumers (the D13 justification, named)

- this kit's own 187-package catalog — replaces a bespoke generator and gets
  a real drift gate instead of three hand-agreed counts;
- a consumer's 87-tool MCP catalog — the one that has already drifted once;
- the licensed component corpus (4 levels, 3 facet axes) — the shape that
  proves facets are not levels.

Three consumers, all in-fleet, all currently hand-rolling. That is the bar
this kit has to clear before it is written, and it clears it.
