# Project Intent — standalone projects and monorepos

The same practice works at both sizes: each decision has one home and each task loads
only the intent that governs it. A monorepo adds a repository charter, not a second
product backlog. [The model](README.md) and [agent integration](integration.md) apply.

## 1. Choose the boundary before creating documents

Use a root intent directory for a standalone product. Add a charter when there are
independent audiences, release cadences, nested products, or extraction/inbound plans.
Many packages alone do not justify intent per package.

A component earns its own intent when it has its own audience, cadence, or roadmap.
Create only its differences: a router, local scope and vocabulary, and pointers to
existing vision and decisions. Add local principles, invariants, futures or incidents
only when they carry useful local content. Omitted documents are explicitly inherited
or routed elsewhere. Never create a second autonomy policy granting wider permissions.

## 2. Ownership by level

| Concern | Repository root | Component |
|---|---|---|
| Purpose | why the components belong together; integration outcomes | audience, product promise, experience |
| Goals | shared contracts and cross-component integration | one canonical product backlog with exit tests |
| Invariants | shared constraints with an explicit area of applicability | additional constraints; link inherited ones |
| Decisions | anything changing a shared boundary or promise | choices whose consequences stay local |
| Authority | contributor policy, release/commit rules, delegation boundaries | inherit; may name stricter local restrictions |
| Compatibility | shared formats and integration promises | component API/config/artifact promises |
| Incidents | failures of repository-wide mechanisms | local failure and durable outcome, or link its journal |

A product-specific rule in a root ADR does not automatically apply to unrelated
products. Cite the record's scope. A component cannot silently override a rule that
does apply; an authorized change first updates the owning root decision.

## 3. The charter: repo.md, at most 120 lines

Keep these facts here, linking mechanical inventories rather than copying counts:

- Why one repository: what splitting would lose.
- Membership and exit rules, with named consumers.
- Component map: `component | kind | intent | consumers | evolution`.
- Dependency direction and its actual enforcement.
- Add/promote/extract/retire procedure and coordinated compatibility changes.
- Ecosystem: upstream doctrine, downstream consumers, inherited decision identities.

Kind: product, kit, library, tooling, example, contract. Intent is `root` or a
repository-relative path. Evolution starts with `stays`, `split when:`,
`merge into <target> when:`, `retire when:`, or `inbound from <repo> when:`.
A map lists every owned intent scope. Vendored examples and upstream routers are
dependencies, excluded from the owned component map.

## 4. Route from affected paths

The working directory is a hint. An agent launched at root can edit a product several
levels below it. First identify affected paths, then:

1. Read the root router and applicable shared invariants once.
2. For each target, find its nearest intent router and relevant intervening ancestors.
3. Load that router's task-specific documents; load the charter on first entry or a
   membership/dependency change. Do not load sibling intent for a local task.
4. For a cross-component change, read both sides of the changed contract and root
   ownership. Record the boundary decision once, with references from the consumers.
5. After compaction, consult source versions if they changed; do not recite the tree.

Markdown links resolve relative to their file. Map paths and instruction paths marked
repository-relative resolve from the checkout root. Validate both; do not count bare
`ADR-0003` references as unambiguous across repositories.

Native instruction discovery differs by harness and launch directory. Explicitly
read the target's area instructions when not already present. A root launch must
still reach the nested router. See [integration](integration.md#2b-the-nested-stanza-a-component-with-its-own-intent-monorepos-only).

## 5. Scope and decision maintenance

Root scope carries integration outcomes and links to component goals; it does not
repeat their task counts. Root vision describes the platform or portfolio; a kit's
vision travels with the kit. Root futures keep only cross-component directions.

Retain existing ADR homes and numbering. Local decisions may have their own index,
or a component router can select root records by path. Do not create duplicate records
to give every folder a numbered series. A root-scoped change is recorded at root;
a purely local change stays local. Inherited external records retain their source IDs.

When scope is owner-directed, make the authorized decisions and label them accepted
design. Keep implementation evidence separate: `implemented`, `partial`,
`accepted design`, `deferred`, and `unverified` describe different states.
A baseline assessment is dated evidence, not a second living backlog.

## 6. Proof and economics

A file's existence proves existence. A control needs separate evidence that it was
configured, invoked on the relevant path, and refused the seeded violation. A skill
can guide judgment; it cannot establish that a hook ran or an approval was enforced.

Use deterministic checks for links, document budgets, index membership, stanza parity,
and pinned dependency integrity. Their successful exit proves only those checks.
Use an intent audit for assumptions, semantic contradictions and fired triggers.
Never interpret a missing prerequisite or unexamined scope as a pass.

A trivial task loads no intent. Ordinary component work loads shared constraints plus
local context. Measure bytes or tokens actually loaded and avoid duplicating global,
root and component instructions. A short router is a budget control, not a summary of
every document. Existing history over budget is named debt; do not mass-rewrite it.

## 7. Distribution and updates

A sibling checkout or personal symlink is convenient but is not a reproducible team
dependency. A consumer may vendor a bounded practice/skills snapshot with source URL,
base commit and exact file digests. Label uncommitted source changes as a working-tree
snapshot; never claim a release pin for them. Keep the license and source paths.

Updates are explicit reviewed diffs. Check local integrity before replacement, preserve
local authoring outside the snapshot, and refuse collisions. Offline verification must
work without the upstream checkout. Do not auto-pull policy on session start or import
upstream project intent as consumer policy. A snapshot carries the practice, not the
upstream product's goals or autonomy. This is independent of runtime Go vendoring.

Both agents use the same skill bodies. Prefer repository-local installation for team
reproducibility; a machine-wide install remains a separate explicit choice. Host wiring
must be tested at its actual entrypoint, not inferred from installed files.

## 8. Doctor and audit

Check the root, the requested component(s), and their owning map entries. A fleet-wide
doctor additionally enumerates all owned routers. Check nested stanzas, parent links,
budgets and declared inheritance. Do not fail a component for lacking inherited
autonomy or a duplicated changelog. Accept the repository's documented ADR header
style and external decision pointers; do not compare all index links with local files.

Audit the two routing scenarios: launched at root touching one component, and touching
a shared contract between two components. Reject duplicate goal ownership, omitted
ancestor constraints, unresolved overrides, and a vendored router mistaken for a product.
Report structural results separately from live host checks and content judgments.
