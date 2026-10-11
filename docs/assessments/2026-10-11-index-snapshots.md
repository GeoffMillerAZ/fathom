# Index snapshots and environment daemons

Owner-architect assessment, 2026-10-11, revised after critique and renamed by the naming decision of the same day;
summarized into the repository 2026-10-11. Read at fathom `c39875f3` and platform-engineering `84f9d41`; adapted to
the records at `65e5acfa`. Companions: `2026-10-11-reviews-and-incidents.md` and
`2026-10-11-making-the-vision-happen.md`. The design, with its glossary, diagrams, data model and phases, is
`docs/design/index-snapshots.md`; this file is the reading that led to it.
Decisions it drove: ADR-069 decisions 7, 9 and 10 (accepted) and 11 to 15 (decided in shape, parked), F3's
amended triggers, F27's edits, F70, F72, F73, F74, G37's IXS-M0 clause and the provisional G38, the scope
boundary for index snapshots, plan `docs/plans/index-snapshots.md` (IXS-). The owner-gated items are in section 10.2.

Conventions as in `2026-10-11-reviews-and-incidents.md`: plain paths = fathom; `pe:` = platform-engineering; `rk:` =
`pe:kits/review-kit/`; [J] marks a judgment (a *hypothesis*); sourced rows without [J] are *measured* at the read
commit; external facts carry a URL. Measurement ids are the plan's `IXS-` ids. Phases 0 to 8 are the design's
(`docs/design/index-snapshots.md` section 12). Section 14 lists the architect disagreements decided here.

## The words

The owner called this "fathom workspaces". The naming decision of 2026-10-11 replaced the word, because `workspace/`
is fathom's own gitignored scratch directory and the word means a mutable place to work in Go, Terraform, npm,
Bazel and VS Code, while this feature is immutable files served read-only. The terms used below:

| Term | Means |
|---|---|
| index snapshots | the capability: CI exports an index per service per commit, the operator publishes and signs it, and others import or serve it read-only |
| index snapshot (`payments@d4e5f6`) | one service's fathom store at one commit, built from a clean checkout, copied with `VACUUM INTO`, machine-local fields cleared, its cache key recorded inside. A conforming store, not an expert module and not a SCIP index. Fathom's existing word (I56, F27, ADR-068) |
| cache key | repository identity (root-commit id plus subpath), commit, fathom version, ExtractorEpoch, TypedEdgeEpoch, format version, config digest, env.lock digest |
| lineup, lineup entry, lineup change | the committed `service@commit` entries an environment runs (`prod.lineup.cue`), written by the operator's deploy job and read by fathom once at start; a lineup change is a rolling update to a new daemon, never a fathom verb |
| environment daemon (`fathom-<env>`, spoken "fathom-prod") | a long-running `serve-http --read-only --lineup <file>` holding every index snapshot its lineup names; one per lineup, run by the operator |
| composed store | the store an environment daemon builds from its lineup's files at start; the only file it writes |
| read-only sidecar | the same read-only mode run for one job (a review runner, an incident agent) over loopback |
| local daemon | the developer's writable daemon, unchanged |
| answer basis | on every answer: each `service@commit` read, its cache key, the lineup and digest when one is served, and one state of `exact`, `differs`, `unchecked` or `uncommitted`; `behind N` is computed on the caller's side |
| snapshot list | an operator-named file of the commits that have index snapshots |
| `--cache-from` ingest | start an index from an index snapshot; a wrong cache costs time, never correctness, once parity is proven |
| fact sheet | per service at a commit: concern facts stored at ingest, joins computed at request time, receipts and coverage; no grades |

The set of environment daemons has no name: no component acts on it, and a name would invite a control plane.

## 1. Verdict

**Yes, if index snapshots are files CI exports and the operator serves, not a server fathom operates, and with
one new subsystem: code reads over an imported index snapshot.**

- CI builds an index per service at the merge commit, from a clean checkout, and `fathom export snapshot` writes
  it with its cache key. The operator publishes and signs it.
- Whoever needs it reads it: a developer's local daemon (for dependencies), a read-only sidecar beside a review
  runner or an incident agent, or an environment daemon behind the operator's own authenticating proxy (that
  last one waits on the owner, section 10.2).
- The developer's own repository stays on the local daemon (uncommitted bytes, latency, offline).
- Branches are `--cache-from` ingests (start from the merge-base's index snapshot, ingest the head; the
  fingerprint gate skips unchanged extraction). No ref column until a measurement says this is not enough.
- Deploy state is a declaration (a GitOps repository indexed as a service, a provenance file the operator names,
  or a lineup). Fathom never keeps a deploy ledger.
- Instances connect by shipping index snapshots, never by query-time calls between daemons, until one of F3's
  amended triggers fires (ADR-069 decision 9, section 7).

What exists: the store is portable (ADR-017, ADR.md:405-418), and read-only mounting is proven (GKE-MH-1,
`store.MountReadOnly`, internal/store/conformance.go:201; mounted at cmd/fathom/servehttp_lifecycle.go:103-120).

What does not: **federation over mounted stores covers obligations only.** GKE-MH-3's symbol is
`FederatedObligations` (docs/plans/gke-module-host.md:73); the module backend feeds only the `obligations` tool
(internal/mcp/semantic_tools.go:602-610), and the module tools that read more were removed by SR-17
(docs/plans/surface-reckoning.md:33). `find_symbol`, `symbol_context`, `blast_radius` and `cross_repo_callers`
read the daemon's own store through `newFleetAdapter(c)` (cmd/fathom/servehttp_store.go:181). `PlatformStore` has
no product caller (`OpenPlatform` is defined at internal/store/platform_store.go:41 and called by none). So an
index snapshot of a dependency answers no code query today. Change 10 (section 9) builds that, sized L and gated.

**The binding technical risk is epoch churn.** `ExtractorEpoch` is 62 (internal/fusion/extractor_epoch.go:693),
was 39 at v0.9.0 and 52 at v0.10.0, and changed in 29 commits since 2026-09-11 (25 since 10-01). Every bump
re-extracts every file. A shared cache only hits when producer and consumer lock the same fathom release.

**The binding intent conflicts** are four, and all are the owner's (section 10.2). The hosted non-goal
(docs/intent/scope/non-goals.md:17-20) and vision 38's "Hosted anything" refusal
(docs/vision/38-proof-layer-roadmap.md:178) have a revisit trigger (an external written ask plus law 9 hardening)
that cannot fire from the owner's own teams. Law 9 makes "auth and read-only enforcement ... launch requirements
for *hosted*" (docs/vision/12-governed-knowledge-engine.md:109, G5 at :196; vision 13 says "read-only surface,
bearer auth", docs/vision/13-expert-suite.md:172), while the bearer-token path was deleted (GKE-MH-7 refused under
SR-17, docs/plans/gke-module-host.md:94). And three futures drop on a written single-user acceptance (F15, F27,
F30). Phases 0 to 4 need none of these; phase 5, the environment daemon, needs all of them.

## 2. What an index snapshot is, and what serves it

| Part | Definition | Owner |
|---|---|---|
| Index snapshot | one SQLite file per service at a commit (docs/spec/store-format-v0.md:29-37), built from a clean checkout, copied with `VACUUM INTO`, machine-local fields cleared (change 11), cache key in an additive `export_meta` table | fathom exports; the operator's publish job publishes and signs |
| Cache key | the components in "The words" above | fathom records and prints its components; the publish job keys the artifact |
| Lineup | a committed CUE file of `service@commit` entries for one environment, each with the snapshot's path, sha256 and cache key, traced to the GitOps declaration that named it | the operator's deploy job writes it; fathom reads it once at start (`schema/lineup/`, change 15) |
| Serving | local daemon (writable, with imports), read-only sidecar, environment daemon; the last two are `serve-http --read-only` (change 4) | fathom provides the mode; the operator runs it |
| Expert layers | compiled claims and maps inside the store (claims are content-addressed, internal/semantic/compile.go:987-988) | fathom; the spend hold applies |

Repository identity = the repository's root-commit id plus a repo-relative subpath for monorepo services [J]. It is
host-free (ADR-011; ADR-068 decision 11 makes repository identity optional). Today the service id is the stanza
name or the directory basename (cmd/fathom/serviceident.go:21-55), which collides across organizations' services;
the lineup maps root-commit id to a local service name.

## 3. Branches, worktrees, merge-ahead and deploy state

### 3.1 The ref model

| Ref | Built by | How | Stored as | Lifetime |
|---|---|---|---|---|
| main at each merge, release tags | merge CI | clean-checkout ingest `--cache-from` the previous index snapshot, then `export snapshot` | index snapshot | the last 20, plus deployed commits under a retention cap (section 12) |
| PR head | PR CI | `--cache-from` the merge-base's index snapshot, ingest head; changed files re-extract | scratch store, never published | the CI run |
| Local branch or worktree | developer daemon | its own service (I30 unchanged; `CheckReroot`, internal/worktree/worktree.go:61), `--cache-from` the merge-base's index snapshot | local store | developer |
| Uncommitted bytes | developer daemon | ADR-041 decision 4 unchanged (ADR.md:2286-2292); anchors read `uncommitted` (internal/receipt/anchor.go:33-50) | local store | session |
| Deployed commit per environment | not fathom | typed `--deployed/--previous` per call (ADR-058 decision 4); a GitOps repository indexed as a service (flux L3, docs/languages.md:194; Argo L3, :302); image digest to commit from an operator-named SLSA provenance file on image-kit's **bake path only**: the apko path "records no build provenance" (pe:docs/adr/0115-image-kit-ci-signs-keyless-or-by-key-without-a-public-log.md:51-56); a lineup the deploy job writes; I56 | nothing stored | request, or one daemon's life |
| Release preview | tag CI | `review --since <deployed>` over `deployed..candidate` | report | run |

Why no ref column now [J]: a `--cache-from` ingest already reuses everything the fingerprint gate can reuse. A
copied store misses the stat gate (`pr.FileStat == st` fails, since mtime and inode differ,
internal/fusion/stream.go:126-129), so it re-reads and hashes every file, but skips extraction when the
fingerprint matches (internal/fusion/stream.go:125-139). It therefore does strictly more work than a warm ingest.
Typed edges are replaced per package (internal/store/sqlite_schema.go:206-226) and the typed-package cache is keyed
by a hash over repo-relative paths (internal/fusion/typedcalls.go:1565-1570). An overlay store saves the copy and
the re-hash; it costs a store-format change on the compatibility surface. Measure first (IXS-M1, IXS-M5), and prove
parity first (IXS-M0b, section 11).

### 3.2 The answer basis

Every answer from an index snapshot carries a basis: each `service@commit` it read, its cache key, the lineup and
its digest when one is served, and one state:

- `exact`: the caller named an expectation and every service read matches it;
- `differs`: the caller named an expectation and a service read is at another commit, or the daemon serves another
  lineup digest; the basis names what was served;
- `unchecked`: the caller named no expectation; the usual state for an agent over MCP, and not a failure;
- `uncommitted`: a local daemon answered over bytes not yet committed (ADR-041 decision 4; the anchor state of
  the same name).

A caller names an expectation by the `X-Fathom-Expect-Lineup` or `X-Fathom-Expect-Commit` header, or the CLI's
`--expect-commit`; never through MCP `_meta`, which an agent cannot set (internal/mcp/protocol.go:14;
ADR.md:4620-4621), and never as a tool parameter. `behind N` is not computed by a daemon: it needs the caller's
commit, a git graph and the list of commits that have index snapshots, and a read-only daemon holds no checkout,
while listing a registry is a fetch (I56, docs/intent/invariants.md:85). The caller's own git computes it
(`git rev-list --count <basis commit>..<caller commit>`) with a snapshot list the operator names. The deploy gate
accepts `exact`, or `differs` when its own per-service comparison passes; exploratory reads accept any state.

### 3.3 Can fathom track branches, worktrees and deploy states?

| Track | Today | With this design |
|---|---|---|
| Branches | `worklines` classifies landed, at-risk, idle, active, read-only (internal/worklines/worklines.go:1-25; SV-1, SV-2) | unchanged, plus per-branch services started `--cache-from` on demand |
| Worktrees | refused as re-roots (I30), folded to main's rules at the edit (cmd/fathom/edithook_dest.go:185-196) | unchanged; a worktree is its own service, started `--cache-from` |
| Deploy states | none; `--deployed` is designed, unbuilt (docs/design/incident-maps.md:210-218) | resolved per request from declarations, or read once from a lineup at a daemon's start; nothing stored |

### 3.4 Is merge-ahead awareness valuable?

[J] Low to medium, unproven. Worth it only as three narrow answers:

1. Two open PRs touch the same contract key or concern fact (a merge-order hazard).
2. A release preview: the concern delta of `deployed..candidate` ("what this deploy changes in risk").
3. Incident suspects exclude merged-but-undeployed commits (the `--deployed` range already does this).

(2) and (3) need no new state. (1) is a forge bot that intersects `blast_radius` across open PRs; fathom stores
nothing. It lives in the operator's CI (a plain job), not in ci-kit, whose scope is "a library and a set of ports",
`cmd/ci` deliberately empty (pe:kits/ci-kit/README.md:3-6). Gate (1) on **IXS-M6**: at least 3 reverts per 100 PRs
traced to concurrently open PRs with intersecting blast radius. Not worth building: an index of every branch.

## 4. Which serving mode, for whom

| Situation | Winner | Why |
|---|---|---|
| Editing your own repository | local daemon | sees uncommitted bytes; edit-hook latency budget; offline; privacy of uncommitted work |
| Reading a platform dependency at the commit prod runs | its index snapshot, **imported** into the local daemon (`import snapshot`, change 10) | the file travels, nothing to keep fresh locally; vision 07 R1's "you never hold the data" stance (docs/vision/07-questions-and-risks.md:26). A `--module` mount alone serves only its obligations |
| First session on a new repository, cloud session | `--cache-from` the merge-base's index snapshot | cold ingest 299 s vs warm 76 s on fathom itself (docs/bench/readings/efficiency-wave/typed-cold-cost.md:149,152); cloud setup budget 180 s (F27) |
| Cross-repo blast radius before merging a library change | local daemon with imported index snapshots (change 10), or an environment daemon | needs many services at deployed commits; `cross_repo_callers` reads only the daemon's own store (cmd/fathom/servehttp_store.go:181) |
| Incident time | a read-only sidecar or the environment daemon serving prod's lineup | no uncommitted bytes, exact commits, and the basis says so |
| Costly judged layers | an index snapshot carrying compiled claims | compile once per section digest, not per developer |

`serve-mcp --url` already fronts a running daemon over stdio (CS-2, docs/plans/cloud-sessions.md:22). With
loopback-only admission (cmd/fathom/admission_http.go:141-155) the target must be local or tunnelled.

## 5. CI/CD pre-processing and the freshness gate

### 5.1 Pipeline

1. **PR CI.** `--cache-from` the merge-base's index snapshot, ingest head, run `review --since <merge-base>`, diff
   concern facts (the ratchet, `2026-10-11-reviews-and-incidents.md` section 5), emit SARIF as a projection.
2. **Merge CI.** Clean-checkout ingest `--cache-from` the previous index snapshot; `fathom export snapshot`
   copies it with `VACUUM INTO`, clears machine-local fields (change 11) and records the cache key; the operator
   publishes and signs it (ADR-068 decision 7: fathom holds no key). Interim publisher: a plain GitHub Actions job
   (`2026-10-11-making-the-vision-happen.md` section 2).
3. **Image build.** On image-kit's bake path, SLSA provenance binds image digest to commit (pe ADR-0115 D4); the
   apko path has no provenance, so its deploy commit comes from the GitOps declaration. The index snapshot can ride
   as an OCI referrer of the image digest
   (https://github.com/opencontainers/distribution-spec/blob/main/spec.md#listing-referrers).
4. **CD gate.** Review Kit `admit` checks the four conditions in `2026-10-11-reviews-and-incidents.md` section 5.
   No fathom gate verb (ADR-063 O2). Fail-open on any fathom operational failure, with a recorded exception for a
   rollback to an already-evaluated commit and for a pdk/risk T3 break-glass (pe:pdk/risk/model.go:14); WAE-M7
   reads the latency the gate adds per deploy and counts fail-open admissions.
5. **Post-deploy.** The GitOps commit that moved prod is itself an indexed fact. When an environment daemon
   exists (phase 5), the deploy job also writes the lineup, and a lineup change rolls the daemon to a new pod.
   Nothing is pushed into fathom.

### 5.2 What "not stale" means

| Level | Meaning | Check |
|---|---|---|
| exact | every service the answer read is at the commit the caller expected | the answer basis; `review --expect-index-watermark` (cmd/fathom/review.go:64) |
| complete | every required fact kind at a version the bundle accepts | `version --json` plus a fact-kind list in the index snapshot (new) |
| evaluated | Review Kit recorded an evaluation at that commit | Review Kit |
| dependency-fresh | each dependency's lineup entry is the commit its declaration names | the deploy job resolves entries from declarations. GKE-MH-6's per-module staleness library exists (internal/modulehost), but its `module_staleness` tool was removed by SR-17 (docs/plans/surface-reckoning.md:33), so a freshness column needs new wiring (change 12) |

The pre-push gate builds a scratch store on every push (lefthook.yml:110-114), but it ingests the working tree
(`--path .`, lefthook.yml:114), not a clean checkout, so it produces no index snapshot; it can host the IXS-M0
timing reading only.

## 6. Team cache sharing and costly-layer sharing

### 6.1 Key, trust, invalidation

| Aspect | Design | Evidence |
|---|---|---|
| Key | the cache key in "The words" | epoch salts: internal/fusion/extractor_epoch.go:693, internal/fusion/typedcalls.go:1565-1570 |
| Wrong cache | intended to cost time, never correctness: `FingerprintHash` is epoch-salted, so a mismatched file is re-extracted. Gated cross-file passes, route-ref and typed edges may still carry results computed against the cached index's other files, so this is unproven until IXS-M0b reads parity | internal/fusion/extractor_epoch.go:697-700; internal/fusion/ingest.go:215-236 |
| Trust | the operator signs (cosign, ssh-keygen); fathom checks only the file's sha256 and recorded cache key against the lineup entry; a consumer may spot-re-extract k random files and compare fingerprints | ADR-068 decision 7 |
| Machine-local fields | **spec drift, a defect to fix before any index snapshot is exported.** The spec says `service_meta.path` is the only machine-local field (docs/spec/store-format-v0.md:160-163), but `ingest_state.path` is absolute (internal/store/ingeststate.go:49-52) and `file_stats` carries `mtime_ns` and `ino` (internal/store/derived.go:165-171) | change 11 (IXS-0): the spec lists them and `export snapshot` clears them; `fathom remap` (cmd/fathom/remap.go:19) re-points the root |
| `$HOME` state | suppressions, call logs, ledgers live outside the store | internal/store/scoped.go:16-24; keyed per lineup before any daemon serves a second team (change 7) |
| Invalidation | any key component changes: a new index snapshot; consumers fall back to the newest one with a matching fathom version | |
| Release lock | env-kit `env.lock` names the fathom release for CI and developers | without it, IXS-M2's hit rate is near zero [J] |
| Fallback lever | per-extractor epoch salts (a TypeScript change stops invalidating Go) | only if IXS-M2 fails (F73) |

Format: one file per service. Transport is the operator's: an OCI registry with referrers is the recommended
recipe, since image-kit already signs. Fathom never fetches (I56, docs/intent/invariants.md:85); a `--cache-from`
file or an imported index snapshot is read only from a path the operator names in that invocation.

### 6.2 Costly layers

- Compiled claims are content-addressed by section digest (`reproKey`, internal/semantic/compile.go:987-988)
  and carried when sections move (internal/semantic/compile.go:410-421).
- The obligations layer's corpus in this repository took 106 commits in the 14 days to 2026-10-11, so compiled
  claims fall behind within days at most, and raw-section fallback is the steady state here
  (`docs/bench/readings/self-use/2026-10-11-compiled-layer-staleness.md`, hygiene note G10). That reading also
  corrects the first note: the 2026-10-11 dry run's 24 of 103 sections do not show staleness from docs edits.
- Design: compile at a release tag (or on a weekly budget), once per unique section digest, across index
  snapshots; ship the claims inside the index snapshot; compute per-snapshot freshness as a view by running
  `Sweep` with `fetch` bound to `git show <commit>:` (internal/semantic/sweep.go:96). Never write status into a
  base.
- The spend hold (2026-09-29) applies: no compile spend until the owner lifts it (phase 8).

## 7. Networked expertise across services

**Ship files, do not call daemons.** A local daemon or an environment daemon imports the index snapshots of the
services it depends on, at their deployed commits. Query-time RPC between live daemons is not built (ADR-069
decision 9).

Why [J], from the owner-architect's answer of 2026-10-11 to the owner's question about live per-service instances:
an incident is exactly when a dependency or its host is degraded, so a daemon that must call another daemon fails
then; a snapshot at a commit never changes, while a live index can re-ingest in the middle of an incident;
cross-service joins run in one process instead of over N network hops; there are fewer servers to upgrade and
authenticate, with ExtractorEpoch moving 29 times in a month; and today's cross-service tools work only inside one
store, so imported snapshots make every tool work unchanged. The `/rpc` registry is also not a security boundary
(internal/mcp/server.go:121-122, "never security (bare /rpc is the full registry regardless)").

Query-time federation is reconsidered when any one of F3's amended triggers fires: (a) a service's imported
dependency closure exceeds one reader's measured budget (IXS-M7: daemon RSS or disk); (b) a dependency's owner
refuses in writing to let its snapshot leave its boundary; (c) an answer class needs on-demand compute that one
process holding the imported snapshots cannot run, and a reading shows the precomputed fact sheet misses it.

| Capability | Today | Needs |
|---|---|---|
| Federated obligations across mounted stores | GKE-MH-3 shipped (`FederatedObligations`) | nothing |
| Code queries (`find_symbol`, `symbol_context`, `blast_radius`) over a dependency's index snapshot | none: they read the daemon's own store (cmd/fathom/servehttp_store.go:181) | change 10 |
| Symbol-level cross-repo callers | `cross_repo_callers` matches symbols across services in the daemon's store (internal/mcp/platform_tools.go:351) | change 10, so imported services are in that store |
| Versioned cross-repo links (route, contract key, pool arithmetic) | SG-2 tables frozen behind RT-M1 (docs/plans/system-graph.md:20); routes stored as file-tier edges (internal/fusion/routeref.go:14-21) | RT-M1 PASS (first amended window 2026-10-12 00:00Z to 2026-10-19 00:00Z, docs/plans/responsible-tenant.md:250), then SG-2; `MatchLinks` is already a pure join (internal/syslink/link.go:48) |
| Unmount a module cleanly | GKE-MH-2 open (docs/plans/gke-module-host.md:69) | not needed: a lineup change starts a new daemon |

"Expert knowledge" here means the compiled layers, maps and fact sheets inside each index snapshot.

## 8. What else index snapshots are good for

1. **Rollback gates and incident replay**: keep the index snapshot of every deployed commit under a retention cap
   (section 12), so a rollback's prod side and "what did the code at that deployed commit do" need no cold
   ingest. Not evidence: a compliance or point-in-time claim leaves as an ADR-068 receipt re-derived at that
   commit, never as the snapshot file (ADR-068 rejects "exporting the store or an index snapshot as evidence";
   ADR-069 decision 15).
2. **Upgrade and CVE reach per deployed image** [J]: ADR-050 decision 4 already reads `pkg:` facts locally and
   vulnerability reach from an OSV file the user supplies (ADR.md:2927); an image-kit SBOM as that input per
   deployed image is new and unbuilt.
3. **Deprecation campaigns**: who still calls X across the organization's services at deployed commits (needs
   change 10).
4. **Review bots** without a per-PR cold ingest.
5. **Cloud and onboarding sessions** (F27, F30) start from an index snapshot.
6. **Chaos targeting** [J]: the fact sheet names where to inject (pods whose liveness reaches a database);
   Gremlin or LitmusChaos executes.
7. **Agent handoff**: a background agent starts from the same commit-pinned index snapshot the reviewer used.
8. **Test selection** at test-function level inside depgraph packages (cmd/fathom/tests_select.go:75).

## 9. What fathom must change, and what is new

| # | Change | Where | Size |
|---|---|---|---|
| 1 | `version --json` adds `extractor_epoch`, `typed_edge_epoch`, `format_version`, fact-kind versions | cmd/fathom/version.go:80-90 (today: version, revision, dirty, tenancy_contract) | XS |
| 2 | `read_source` takes `ref:` and serves `git show <commit>:path` | internal/mcp/readsource.go:170-188 | S |
| 3 | The answer basis (`exact`, `differs`, `unchecked`, `uncommitted`) and the expectation headers; `behind N` caller-side only | MCP envelope | S |
| 4 | `serve-http --read-only`, in four parts: (a) open snapshots with `store.MountReadOnly` (mode=ro, internal/store/conformance.go:201); (b) auto-refresh off and the refresh queue nil (today a drifted read queues a re-ingest, internal/mcp/autorefresh.go:31,227, wired at cmd/fathom/servehttp_store.go:167-171); (c) refuse `ingest` (which accepts any absolute path, internal/mcp/extra_tools.go:453-459), refresh-file, compile and claim writes, remove `POST /reload`, turn off the wiring apply and undo endpoints, and hold the decision port at provider `none`; (d) hard-gate or disable bare `/rpc`, because `strictSurfaces` gates only named surfaces and bare `/rpc` serves the full registry (internal/mcp/dispatch.go:376-381; internal/mcp/server.go:121-122). Fathom never binds a non-loopback address. The network form (an authenticating proxy in the same pod) waits on the owner (section 10.2) | cmd/fathom serve-http, internal/mcp | M |
| 5 | `ingest --cache-from <file>` copies a named index snapshot, remaps it, and ingests; a parity test proves a cache-from ingest equals a cold one on fixtures | cmd/fathom ingest; ingest-diff runs a full ingest today (cmd/fathom/ingestdiff.go:110-120) | S |
| 6 | Findings importer hashes at `SourceRevision` | internal/findings/external/importer.go:256; the disk read is pkg/client/findings.go:56-66 | S |
| 7 | Lineup-keyed `$HOME` state | internal/store/scoped.go:16-24 | M, only before a second team uses an environment daemon |
| 8 | A fact-kind list in the store | observations, no table | S |
| 9 | Overlay store `(service, ref, merge-base)` | store format v0, additive | L, gated on IXS-M5 (F70) |
| 10 | **Code reads over an imported index snapshot.** Option A [J, chosen]: `fathom import snapshot <file>` merges a snapshot's service into a writable daemon store under the lineup's service name, so every code tool works unchanged. Option B: fleet federation for code tools over mounted stores, as `FederatedObligations` does for obligations. Checked against: store format v0 stays additive (A writes the same tables; B adds none); the compatibility surface lost `module_query` under SR-17, so B re-adds a federated read path that was deleted for zero use | cmd/fathom, internal/store | L, gated on IXS-M1 KEEP and a named consumer of a cross-repo read |
| 11 | Machine-local fields: the spec and the code agree before any index snapshot is exported (the spec lists `ingest_state.path`, `file_stats.mtime_ns`, `file_stats.ino`, and `export snapshot` clears them) | docs/spec/store-format-v0.md:160-163; internal/store/ingeststate.go:49-52; internal/store/derived.go:165-171 | S |
| 12 | A per-service freshness column wired from GKE-MH-6's library (its tool is gone) | internal/modulehost; the observatory | S |
| 13 | `fathom export snapshot`: `VACUUM INTO` a new file, clear machine-local fields, verify `application_id` and `user_version`, record the cache key in `export_meta`, print it; never upload or sign | cmd/fathom export (beside `export scip`) | S |
| 14 | `serve-http --read-only --lineup <file>`: validate the lineup, refuse an entry whose sha256 or cache key differs, compose at start, report ready only when every entry loads | cmd/fathom | M (one entry in phase 3, several in phase 5) |
| 15 | The lineup schema, `schema/lineup/` (fathom is the reader) | schema/ | XS |

New outside fathom: the publish job (a plain GitHub Actions job first, `2026-10-11-making-the-vision-happen.md`
section 2), the deploy job that writes the lineup, the environment-daemon recipe (docs only), and Review Kit's
`index_commit` check.

## 10. Intent conflicts: what landed, and what awaits the owner

### 10.1 Landed with ADR-069 (owner-architect)

| Record | Change |
|---|---|
| F3 (docs/intent/futures/f01-f10.md) | amended: importing an index snapshot is not federation; query-time federation's triggers (a) to (c) added; RPC fan-out not built before one fires |
| F27 (docs/intent/futures/f23-f32.md) | Do not build: no sync, store, registry or fetch operated by fathom; Preserve: the watermark is the cache key's commit component; Promote when: or an owner-named team consumer and IXS-M1 KEEP |
| F70, F72, F73, F74 | new futures (overlays, versioned cross-repo joins, per-extractor salts, the environment daemon) |
| I30, ADR-041 decision 4, I56 | unchanged: worktrees stay their own services; index snapshots come from operator-named paths |
| G37 | carries IXS-M0 only; index snapshots get their own goal (provisionally G38) when the owner starts phase 1 |
| Scope boundaries | index snapshots and environment daemons: designed and parked |

### 10.2 Awaiting the owner (U1)

U1 is the owner's open question of 2026-10-11: a self-hosted, single-organization, read-only environment daemon.
Answering yes narrows the hosted non-goal, meets law 9 through the operator's authenticating proxy, reinterprets
P9, overrides F8's drop trigger and answers Q5 qualified. "Read-only" then means: never writes an index snapshot or
any operator file, and writes only its own composed store, at start. U1 gates phase 5 and F74; phases 0 to 4 do not
need it. None of the texts below is applied. Each is the proposed edit, adapted to the final names.

| Record | Conflict | Proposed edit |
|---|---|---|
| Non-goal "No hosted multi-tenant expert backend" (docs/intent/scope/non-goals.md:17-20) | its trigger (an external written ask) cannot fire from the owner's own teams | "No hosted backend operated by fathom, and no multi-tenant or writable daemon. In scope (ADR-069): index snapshots CI builds at a commit, published by the operator, read by a local daemon, a read-only sidecar, or an operator-run single-organization environment daemon behind the operator's authenticating proxy (law 9 as amended); fathom never binds a non-loopback address and never fetches. Revisit when: a second organization asks in writing to share one daemon." |
| Vision 38 section 7 "Hosted anything" (docs/vision/38-proof-layer-roadmap.md:178) | same | "Hosted anything operated by fathom, multi-tenant, or writable (the narrowed scope non-goal, ADR-069). An index snapshot read by the operator's own local daemon, read-only sidecar or single-organization environment daemon is the born-portable strategy, not hosting." |
| Law 9 (vision 12:103-113, G5 at :196) and vision 13:172 | auth is a launch requirement; the bearer path was deleted (SR-17) | amend law 9: for an operator-run, single-organization, read-only environment daemon serving index snapshots, auth may be the operator's authenticating proxy in the same pod over loopback; proven at the proxy-plus-daemon pair (IXS-M8: every unauthenticated request refused) with the daemon unable to write (IXS-M3). G5 gains the same note; vision 13:172 gains "superseded in part: the hosted modules surface was deleted (SR-17), and auth for a read-only daemon is the operator's proxy" |
| P9 (docs/intent/principles.md:35-36) | P9 orders read-only before writable and names auth; it does not waive auth | reinterpreted: auth may be proven at the operator's proxy in front of a read-only daemon (law 9 as amended; IXS-M8). A writable hosted surface stays refused |
| F8 (docs/intent/futures/f01-f10.md:94-102) | promote needs an auth row GKE-MH-7 refused; drop-if is tied to vision 07 R1 ("FATAL at this stage", docs/vision/07-questions-and-risks.md:26) | Preserve: `strictSurfaces` for named surfaces plus a hard gate on bare `/rpc` in `--read-only` mode. Promote when: IXS-M3 passes (the hostile sweep, including `/reload`, `/route` and the wiring routes) and one consumer's read-only sidecar session is recorded with a law 6 stamp file; a network-reachable environment daemon also needs IXS-M8; its environment form is F74. Drop if: no such session within 60 days of IXS-3 landing, then `--read-only` and the recipe are deleted under a `gone:` row. R1's trigger is overridden by the owner's ask, not fired |
| Q5 (docs/intent/questions.md:16-19) | blocks F8's network form | "Accepted for a loopback daemon on one workstation, while a second user or networked serving remains possible; when it comes, network reach goes through the operator's authenticating proxy in front of `serve-http --read-only` (law 9 as amended, ADR-069)." Deliberately not an unqualified single-user acceptance |
| F15, F27, F30 drop-if (f11-f22.md:57; f23-f32.md:64-66, :101) | each drops when "single-user is accepted in writing" | after Q5 is answered as above, each gains "(Q5 was answered qualified, keeping a second user and networked serving possible, so this has not fired; ADR-069.)" |
| Assumption 2 (docs/intent/domain/assumptions.md:10-12) | the single-user assumption | gains Q5's qualified answer |

Also the owner's, outside U1: whether a lineup change is attach under IM-X2 (docs/plans/incident-maps.md:68). The
recommendation is no: nothing is pushed into fathom, nothing about a deploy is stored, and the start log line is an
operational log, never a served timeline (`docs/design/index-snapshots.md` section 2). A yes means phase 5 waits on
a new ADR under IM-X2's reopen clause.

## 11. Phases

The design's phases, each with its own start trigger and a kill that can fire after it starts
(`docs/design/index-snapshots.md` section 12). Index snapshots are parked: the owner prioritizes them when ready,
no earlier than war1 landing and RT-M1 reading.

| Phase | Build | Exit | Kill | Needs U1 |
|---|---|---|---|---|
| 0 | **IXS-M0** reading only: on 20 commits in CI from a clean checkout, wall time of cold, warm and cache-from ingests | IXS-M0 recorded | cache-from median above 0.6x cold: F27 stays deferred, phases 1 and 2 do not start | no |
| 1 | changes 5 and 13 (war1 brings 1, 2, 6 and IXS-0) | **IXS-M0b**: across 10 commits, a digest over the portable derived tables of a cache-from ingest equals a cold ingest | a mismatch unfixed after two waves: `--cache-from` and `export snapshot` deleted under a `gone:` row | no |
| 2 | the publish recipe (a plain GitHub Actions job); `env.lock` names the fathom release | **IXS-M1** (cache-from at most 1.3x warm, median of 20 merges, same CI, fathom and one platform-engineering repository); **IXS-M2** (same-release hit rate at least 0.5 over 30 days); **IXS-M5**; IXS-M7's CI cells | cache-from above 0.6x cold, or hit rate under 0.2: F27 stays unpromoted, the job stops | no |
| 3 | changes 3, 4, 8, 15, and 14 for a one-entry lineup | **IXS-M3**: the hostile sweep leaves snapshot bytes unchanged and calls no provider; one read-only sidecar session recorded with a law 6 stamp file | no such session within 60 days: deleted with a `gone:` row, the SR-17 precedent | no |
| 4 | change 10, `import snapshot` | a named consumer's session reads an imported service with its basis | no such session within 60 days of change 10 landing: deleted under a `gone:` row | no |
| 5 | the environment daemon: change 14 for several entries, changes 7 and 12, the recipe | **IXS-M8** (every unauthenticated request through the proxy-plus-daemon pair refused), **IXS-M9** (a gate or incident session recorded with a law 6 stamp file within 60 days of first deploy), **IXS-M4** (a second team for four weeks) | IXS-M8 fails, IXS-M9 CUT, or no second team within 90 days: F74 drops | **yes** |
| 6 | versioned cross-repo links (F72) | at least one true-positive cross-repo finding | none within 90 days of SG-2 landing: F72 drops | no |
| 7 | the overlay store (F70) | an overlay-store ADR | its saving under IXS-M5's bar on 20 later PRs: removed | no |
| 8 | shared compiled layers | precision per judged check; cost per true positive | a Wilson lower bound under 0.90 or cost over the owner's bar: removed | no |

Why the IXS-M1 bar is against warm: warm is 76.1/299.3 = 25.4% of cold (typed-cold-cost.md:149,152), and a
cache-from ingest misses the stat gate and re-hashes every file, so it cannot beat warm. An earlier "at most 25% of
cold" bar was below what the design can reach.

## 12. Cost model

Inputs measured on fathom itself (45k symbols): cold ingest 299.3 s wall at 1.81 GB peak RSS; warm 76.1 s at
2.09 GB; store 154.8 MB (docs/bench/readings/efficiency-wave/typed-cold-cost.md:149-156). Figures for many services
are [J] illustrations at 50 services, 10 merges and 4 deploys per service per workday (20 a week); services smaller
than fathom cost less, unmeasured.

| Cost | Per service per event | Illustration at 50 services | Status and gate |
|---|---|---|---|
| CI wall per merge | cache-from at most 99 s (the 1.3x warm bar); cold 299 s | at most 13.8 CI-hours per workday with the cache; 41.6 cold | IXS-M1 |
| CI runner memory | about 2.1 GB peak | runner class of at least 4 GiB | RT-M10, the per-RAM-class feasibility reading (docs/plans/responsible-tenant.md:259), is open and gates the runner class |
| Storage, last 20 per service | 155 MB per index snapshot | 155 GB steady | compression and cross-commit dedup unmeasured (IXS-M7) |
| Storage, every deployed commit (owner ruling D13) | 155 MB per deploy | 155 GB a week, about 8 TB a year uncapped | **capped**: deployed commits kept 90 days after a later lineup moved off them, then one per service per month, about 2 TB at this rate |
| Environment daemon memory per index snapshot | unmeasured | unmeasured | IXS-M7 reads daemon RSS with 1, 5 and 20 snapshots under a fixed query mix before phase 5 |
| Developer machine | one index snapshot per imported dependency, same sizes | n/a | same retention as CI |

## 13. Risks

| Risk | Mitigation |
|---|---|
| Epoch churn voids sharing | lock the release in env.lock; per-extractor salts only if IXS-M2 fails |
| A cache-from ingest differs from a cold one | IXS-M0b parity blocks publishing on any mismatch |
| Code is intellectual property; configs can carry secrets | read-only, ADR-065 withholding, the registry's read access matching the forge's; never a multi-tenant daemon |
| The proxy's `Host` rewrite removes the DNS-rebinding defence | same-pod loopback only; IXS-M8 hostile reading; the mutating and spending routes are off in read-only mode |
| A write path survives `--read-only` (bare `/rpc`, auto-refresh, `/reload`, the wiring routes, the observatory) | change 4's four-part definition; IXS-M3 sweeps all of them |
| Bus factor of one running a service (docs/vision/07-questions-and-risks.md:26) | fathom operates nothing; the daemon is stateless and rebuilt from index snapshots at each start |
| Commoditised hosted context (Augment remote mode indexes default branches only, https://docs.augmentcode.com/context-services/mcp/overview.md) | offer joined, governed, commit-pinned facts with a basis, not symbol lookup |
| A shared base corrupted by one session's uncommitted bytes | index snapshots are never written; uncommitted state stays local |
| Storage grows without bound | D13 under the retention cap above |
| `$HOME` state leaks across teams on one daemon | change 7 before phase 5 |
| Building a hosted mode nobody uses, again (SR-17) | phase 3's and phase 5's kills delete it with a `gone:` row |

## 14. Disagreements resolved here (owner-architect)

| # | Question | Platform | Boundary | Decision | Why (one line) |
|---|---|---|---|---|---|
| 1 | Non-goal | replace with "no multi-tenant SaaS operated by fathom" | append a note: a shared index is not hosting | the narrowing goes to the owner (U1, section 10.2); build order sidecar first, environment daemon after U1, IXS-M3, IXS-M8 and a named service | the owner's ask cannot fire the old trigger, and a note alone leaves vision 38 refusing it |
| 2 | Refs | overlay store plus a `ref:` resolver | per-ref ingest from a cache, no ref column | `--cache-from` now, parity proven; overlay gated on IXS-M5 and IXS-M1 | a ref column touches the compatibility surface before anyone measured the cheap path |
| 3 | Answer label | exact / ancestor+n / overlay / dirty | watermark only | the answer basis `exact` / `differs` / `unchecked` / `uncommitted`; `behind N` caller-side with a snapshot list | a daemon has no git graph and may not list a registry (I56); an agent cannot name an expectation through `_meta` |
| 4 | Deploy state | a CD-attested record ingested; ADR-058 amended | not fathom; per-call parameter, GitOps, provenance | per-call parameter, GitOps service, operator-named provenance file (bake path), or a lineup read at start; nothing stored | a stored deploy record is IM-X2's attach under another name |
| 5 | Worktrees | overlay; amend I30 | own service, I30 kept | I30 kept | no measured need, and INC-3 is why I30 exists |
| 6 | Repository identity | forge URL plus path | root-commit id | root-commit id plus subpath; the forge URL is configuration | host-free (ADR-011) and survives renames and mirrors |
| 7 | Transport | OCI referrers, image-kit signs | CI artifact | the operator's publish job owns it; OCI referrers is the recommended recipe | fathom stays transport-agnostic and fetch-free (I56) |
| 8 | Per-extractor epochs | now, as mitigation | only if IXS-M2 fails | measure first (F73) | a locked release may suffice; salts are a cross-cutting change |
| 9 | Merge-ahead | overlay notices | a bot gated on IXS-M6 | a plain CI bot outside ci-kit, gated | no stored state for an unmeasured benefit; ci-kit's scope is library and ports |
| 10 | Across services | a list of services plus cross-repo joins early | gated on RT-M1 and SG-2 | `import snapshot` when a consumer names a cross-repo read; versioned links after RT-M1; no query-time RPC until F3's triggers | mounted stores federate obligations only; code reads need the import |
| 11 | A UI for this | freshness board, incident views | no UI | a freshness column in the existing observatory only (change 12) | the staleness library exists but its tool is gone; an incident view is the command-center non-goal |
| 12 | F27 promote | the owner names a team consumer | IXS-M1 and IXS-M2 KEEP | naming opens the measurement; IXS-M1 KEEP promotes | demand plus proof, both recorded |

## 15. The goal held for phase 1 (provisionally G38)

G37's 2027-01-31 kill would otherwise fire on parked work, so index snapshots leave G37 except for IXS-M0. Their
goal is held here, not in `docs/intent/scope/`, and is opened in the change that starts phase 1, under the next free
goal id at that time (G38 is free on 2026-10-11; parallel record writers collide on ids, so grep every branch
first). Its `review` date is the start date plus 120 days. The text to land:

```markdown
**G38. Index snapshots make a cold start cheap and answer for what an environment runs, or are cut (ADR-069).**
Done when: `docs/plans/index-snapshots.md` IXS-M0b, IXS-M1, IXS-M2 and IXS-M3 each read KEEP or CUT, and, once
U1 is answered yes, IXS-M8 and IXS-M9 too; a CUT removes what it killed in the same change. Kill: at the review
date, if IXS-M1 is unread or CUT, the publish recipe and `--cache-from` are deleted under a `gone:` row, F27 stays
deferred, and phases 3 to 8 do not start; an unread IXS-M8 or IXS-M9 at that date drops F74. Every reading is $0
in model spend.
{bet: index-snapshots, instrument: "fathom plans status", threshold: "IXS-M0b, IXS-M1, IXS-M2 and IXS-M3 each read KEEP or CUT (IXS-M8 and IXS-M9 too after U1), a CUT removing what it killed", review: <start date plus 120 days>}
```
