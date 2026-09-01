# Project Intent — monorepos, nesting, and the repository charter

Sibling to [README.md](README.md). A single-product repository's intent is about the
product. A repository that holds several products, or components with different
evolution paths, also needs intent about the **container**: why these things live
together, what belongs, how the parts relate, and which parts are expected to leave or
arrive. None of the nine living docs owns that, so a monorepo gets a tenth, conditional
one: the **repository charter**, `docs/intent/repo.md`.

## 1. Does this repo need a charter?

Yes if any holds: more than one product or audience; components with their own release
cadence, version, or consumers; nested modules; a documented expectation that some part
will be extracted, merged in, or retired; more than one `docs/intent/` directory. A
single product with many packages does not: `vision.md` and `scope.md` cover it. When in
doubt, write the component map (§2 row 3) and see whether the evolution column has any
value other than "stays". If not, no charter.

## 2. The charter (`repo.md`, ≤ 120 lines, rarely changes)

1. **Why one repository.** The reason the parts live together, stated as what would be
   lost by splitting (one gate, one audit, one compatibility promise, atomic
   cross-component change). Not "convenience".
2. **What belongs here, what does not.** The membership rule and the exit rule. A
   package belongs when it has a named consumer; it leaves when it has none.
3. **Component map.** One row per component:
   `component | kind | intent | consumers | evolution`.
   *Kind*: product, kit, library, tooling, example, contract. *Intent*: `root` or the
   path of its own `docs/intent/`. *Evolution* uses a fixed vocabulary:
   `stays` · `split when: <trigger>` · `merge into <target> when: <trigger>` ·
   `retire when: <trigger>` · `inbound from <repo> when: <trigger>`.
4. **Dependency direction.** Which tiers may import which. The one sentence a new
   package must not violate, plus what enforces it (a lint, a test, or nothing).
5. **Adding, promoting, extracting, retiring.** The four procedures, each a short
   checklist naming the artifacts that must change together (tracking row, compat row,
   catalog, changelog, tag cadence).
6. **Ecosystem.** Downstream consumers, upstream doctrine sources, and shared decisions
   inherited from another repository (cite them as `<repo> D13`, never renumber them).

The charter is the free-option principle applied to repository shape: a component kept
extractable at zero cost (own module or self-contained directory, no upward imports,
own tests and docs) preserves the split without building it. Separate CI, duplicated
gates, or a premature repo split are the "prepare" move, and are refused.

## 3. Which components get their own `docs/intent/`

A component earns its own intent directory only when it passes the **own-intent test**:
it has its own audience (consumers who do not consume the rest), its own release cadence
or version, or its own roadmap. Everything else is covered by the root: a foundation
package's intent is its package doc and `schema.cue`, not a directory of prose.

A component with its own intent holds only what differs from root: usually `vision.md`
(if it is a product), `scope.md`, `futures.md`, `decisions/`, a short `domain.md` for
local vocabulary, and `incidents.md` when it has its own. It never holds `autonomy.md`
(inherited) and rarely `principles.md`. Its router is the template router with a first
line pointing up: "Root charter and shared invariants: `../../docs/intent/README.md`."

## 4. Routing across levels

Both Claude Code and Codex load a nested `CLAUDE.md` / `AGENTS.md` when work touches
its directory. That is the mechanism; the practice adds three rules:

- The **root stanza** (integration.md §2) stays in the root files unchanged.
- A component with its own intent adds the **nested stanza** (integration.md §2b,
  three lines) to `<component>/CLAUDE.md` and `<component>/AGENTS.md`.
- Skills **walk up** from the working directory to the nearest `docs/intent/README.md`,
  then also read the root router's `invariants.md` and `repo.md`. Nearest router for
  scope and decisions; root for invariants and the container. Never the siblings.

Load moments: the charter is read when adding, moving, extracting, or retiring a
component, and on first entry into a component directory. Ordinary work inside a
component never loads it.

## 5. Decision scoping

- A root decision binds every component. A component decision may **narrow** a root
  decision or invariant; it may never contradict one. If it must, the change is a root
  decision: reopen at root with `intent-decide`, then narrow below.
- Numbering is per intent directory. Cross-references are path-qualified
  (`kits/expertchat/docs/intent/decisions/0003`), never bare numbers.
- Ecosystem decisions inherited from another repository keep that repository's id and
  are cited with the repository name. Two repositories each having a "D13" is fine as
  long as no citation is bare.

## 6. Evolution futures

Split, merge, retire, and inbound expectations live in the charter's evolution column,
not in `futures.md`, because they are properties of the map. `futures.md` carries one
pointer entry when a split or merge is likely enough to deserve `Do not build` /
`Preserve` lines. Triggers are observable, like every other condition in the practice:
"a consumer outside the fleet pins this module independently", "release cadence has
diverged for two tags", "zero consumers at the next wave end". `Drop if` for an
extraction is usually "the trigger has not fired within N tags".

## 7. The level above the repository

An ecosystem of repositories (a kit, its consumers, a design system, a tool that
indexes all of them) has intent too: shared decisions, shared invariants, the fleet's
own map. The practice does not require a fleet repository, but it gives the pattern:
one repository is designated the home of a shared decision series (a design kit's
`docs/DECISIONS.md`, say), every other repository's charter names it under
**Ecosystem**, and the user-level agent instructions stay a router only. The moment two
repositories carry their own copy of a shared decision, one of them is wrong.

## 8. Doctor checks for monorepos

`intent-doctor` adds three checks when `repo.md` exists or more than one
`docs/intent/README.md` is present: (21) the charter exists and its component map lists
every directory that has its own intent; (22) every component with its own intent
carries the nested stanza in both agent files; (23) no component directory holds an
`autonomy.md`, and each component's living docs are within budget.

## 9. Examples

- **A foundation package asks for its own intent.** No. It has no audience of its own;
  its package doc, `schema.cue`, and the root invariants are its intent. Adding a
  directory would be per-package ceremony for 190 packages.
- **A format handler needs a dependency the kit forbids.** Make it a nested module with
  its own `go.mod`, listed in `go.work`, tagged on its own cadence, with no imports from
  sibling kits. The charter marks it `split when: a consumer outside the fleet pins it
  independently`. It is extractable in an afternoon and nothing was built for the split.
- **A component decision says "we may call the network in tests here".** Contradicts a
  root invariant. Not a component decision; reopen at root, where the answer is no.
- **A product's roadmap item depends on another product in the same repo.** Both
  products' `futures.md` entries name the dependency by path; the charter's map does
  not change. If the dependency is an import, the dependency-direction rule decides.

## 10. Considered and rejected

- **One root intent directory with per-component sections.** Every doc blows its
  budget by the third product, and routing degrades to "read everything".
- **Intent per package.** Ceremony with no reader; the own-intent test exists to
  refuse this.
- **A generated component map.** The mechanical part (package list, classification,
  counts) is already lint-enforced where it matters (`compat-coverage`,
  `catalog-fresh`); the evolution intent cannot be generated, and a generator would
  overwrite it.
- **Putting the charter in `vision.md`.** Vision is the product's; the charter is the
  container's. A product that later moves to its own repository takes its vision with
  it and leaves the charter behind. Keeping them apart makes that move free.
