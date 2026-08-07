# 15 — Reach: obligation packs, language packs, and the dependency plane

> Status: FOR REVIEW · Date: 2026-08-07 · Non-binding, companion to 14.
> Answers three owner directives: (1) make agents actually follow UI/house
> standards at edit time (the CLAUDE.md-is-ignored problem); (2) excellent
> support for all major languages/tools — C++ named concretely (an external
> adopter's shop, Conan dependencies); (3) whether fathom should own a full
> hermetic transitive-dependency mapping/hashing library, and the
> build-vs-adopt question re: Bazel.

## 1. The UI-standards ask — fathom already is this tool; the gap is packs, not mechanism

The owner's pain: UI work happens in sessions that ignore the paid component
corpus and the app-kit doctrine *despite CLAUDE.md describing it*. That is
not a discipline failure — it is the exact failure mode fathom's thesis
names: passive prose is read once and paged out; **delivered-by-configuration
survives**. The mechanism exists end to end today: the PostToolUse edit-hook
injects glob-scoped obligations at the moment a matching file is touched;
the PreToolUse guard can *refuse* (a `package.json` under a no-Node surface
is a refusal, not advice); layer packs under `.fathom/layers/` serve rules
now (platform-engineering serves 4 from its platform-standards pack today).

What's missing is the **product form: obligation packs as first-class
shippables** — `fathom layers install ui-appkit`:

- A premade pack, authored once where the doctrine lives (app-kit owns its
  own pack — the kit ships its law with its code), installed into every
  consuming repo by `fleet reconcile` (14 §7).
- Rule shape, glob-scoped (`portal/**`, `**/*.templ`, `ui/**`, `**/*.css`):
  *search the appkit MCP corpus before writing markup; kit role utilities,
  never raw palette colors; the `@source` line for every rendered package;
  no `package.json` (guard-refused); dashboards start from the licensed
  shells*. Every rule cites its ADR/decision, so the injection is an
  obligation with provenance, not a style nag.
- The **snowball clause**: the review verb (or Stop hook) flags new UI
  components authored in an app whose shape matches the kit's component
  contract — "uplift candidate: this belongs in appkit/ui/" — advisor
  recommends, gate decides (law 11). That is the contribute-back loop made
  mechanical: one app's work becomes every app's component because the
  tool *notices*, not because a human remembers.
- Packs generalize far beyond UI: security-emissions-last, no-raw-CIDR,
  testing standards — any org doctrine becomes an installable, versioned,
  eval-gated pack. **The pack catalog is a product surface**, and the
  expert-module readiness ladder (draft → eval-passed → servable →
  published) is already its packaging and governance.

## 2. Language and platform reach — the coverage program

External signal: a real adopter's shop hit missing/thin C++. Today's
extractors cover Go/JS (+Python, Kubernetes, CUE symbols). The program:

- **A language-pack contract** over the tree-sitter extraction core
  (already the current dev direction): each pack declares what it emits —
  symbols, reference/call edges, test-reachability semantics, doc
  conventions, **and the ecosystem's dependency manifests** — plus a
  calibration-corpus entry with a recall gate against the grep-oracle
  (the existing `calibration/` machinery, generalized). A language without
  a calibration row isn't supported; it's tolerated — the distinction is
  printed, never blurred (law 3).
- **Tier 1** (symbols + edges + tests + deps): Go, TypeScript/JavaScript,
  Python, **C++**, Java, Rust, C#. **Tier 2** (symbols + deps): Swift,
  Kotlin, Ruby, PHP. **Config planes** as first-class packs: CUE (with the
  O3 evidence tier — `vet_clean`/`export_golden` — so CUE-first repos stop
  skip-testing everything), Terraform/HCL, Kubernetes YAML, proto, SQL.
- **C++ specifically**: `compile_commands.json` awareness for real include
  graphs, header/TU dedup, and **Conan as the dependency source of truth**
  (`conan graph info --format=json` → the transitive graph; lockfiles →
  input digests). vcpkg and CMake FetchContent follow the same adapter
  seam. The adopter's shop is the design-partner corpus: their friction
  reports become the calibration fixtures.
- Premade packs for major platforms = §1's obligation packs × §2's
  language packs. The matrix is the catalog; each cell ships eval-gated or
  states its tier honestly.

## 3. The dependency plane — the Bazel decision

Decomposed: (a) does fathom need hermetic transitive dep mapping + input
hashing? (b) should the library cover every major ecosystem? (c) build,
or adopt Bazel/kin?

- **What fathom already has**: hash-gated ingest (content-addressed corpus
  fingerprints), citation anchor hashes, drift sweeping. Fathom's "has the
  concept changed" question is answered by *citation* hashing, not build
  graphs — that stays as-is. What fathom lacks is **dependency-aware
  impact**: `dep_drift`, `package_deps`, `cross_repo_blast_radius` exist
  as tools with zero lifetime calls, waiting for a real dep plane.
- **What the platform side already decided** (platform-engineering): ci-kit
  specs the depgraph engine + canonical input hashing (`DeriveKey`,
  type-tagged length-prefixed, keyEpoch) and already adopted **bazel-remote
  as the $0 REAPI cache** — the Bazel *ecosystem* is in use at the protocol
  layer, which is the correct reuse.
- **The decision, recommended**: one shared library, **born where its first
  paying consumer is** — the depgraph/input-hash engine lands inside ci-kit
  per its existing spec, built kit-first (D13) behind a clean port:
  `DepGraph(ecosystem, root) → nodes + edges + lockDigests` and
  `InputKey(node) → canonical hash`, with per-ecosystem adapters (go.mod,
  npm/pnpm, pip/uv, cargo, **conan**, maven/gradle, vcpkg). fathom becomes
  its second consumer (dep-aware impact + ingest keys); when a third
  appears, it graduates to its own repo. Building it "in fathom first"
  would invert ownership: fathom needs the *answers*, ci-kit needs the
  *engine*.
- **Bazel verdict: reference semantics and protocol reuse, never a
  dependency.** Bazel is open source but Java (Buck2 is Rust) and is a
  build *system* — adopting it means adopting BUILD-file worldview
  repo-wide, which contradicts the recognize-don't-rewrite adoption
  ladder. What we take: its action-key/content-addressing canonicalization
  rules as the reference implementation, REAPI (taken), bazel-remote
  (taken). What we build: the embeddable per-ecosystem graph+hash library —
  the one part none of Bazel/Buck2/Please expose cleanly, and the part
  three of our products need.
- **Adjacent concepts worth naming now**: Nix derivation hashing as the
  hermetic-env reference (devbox is already Nix-backed); SLSA provenance
  predicates (image-kit has them); tree-sitter as the universal parse
  plane (taken, §2); **SCIP/LSIF emission as a watch row** — emitting SCIP
  would make fathom's index consumable by other tools while governance
  stays ours (differentiation-compatible; evaluate, don't commit);
  OpenTelemetry semantic conventions for the observatory's metric naming
  (weigh at 14 §8 rung 2).

## 4. What else the current vision is missing (honest sweep)

- **Harness parity**: the hook stack is Claude-Code-shaped; Codex parity
  is claimed in adoption docs — it needs its own calibration row, not an
  assumption. Other harnesses (Cursor, Windsurf-class) are watch rows.
- **Windows/WSL posture**: undeclared. Declare it (WSL-supported,
  native-Windows deferred) rather than discover it in an adopter's shop.
- **Nested corpus roots**: the crossplane-platform accident shows sub-tree
  services need first-class containment design (parent/child, not
  accident) — folded into 14 §7's registration hygiene.
- **LLM provider breadth for compile**: `providers` exists; the compile
  path's non-Anthropic coverage needs the same honest tier labeling as
  languages.
- **Air-gapped distribution**: expert modules already carry provenance and
  portability — the air-gap story should be *stated* as a supported
  profile, since local-first is the differentiator.

## 5. Activation additions (extends 14 §10)

7. Author the `ui-appkit` obligation pack in app-kit; install into the
   UI-active repos via reconcile; verify with one deliberate violation
   (a raw `bg-indigo-600` edit should inject the role-token rule; a
   `package.json` write under a no-Node surface should refuse).
8. C++ language pack behind the calibration gate, Conan adapter first;
   the external shop as design partner — their observations feed the
   fixtures.
9. Depgraph/input-hash library kickoff inside ci-kit's phase-3 plan
   (platform-engineering), port-first, with fathom's dep-aware impact
   tools as consumer #2.
10. SCIP emission spike: one week, emit-and-diff against an SCIP indexer
    on the calibration corpus; adopt or file the refusal with reasons.
