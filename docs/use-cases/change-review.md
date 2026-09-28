# Change & PR Review

Review a diff or pull request with automated impact analysis and conformance gates before merging.

---

## When to use this

- You have a PR open and want to know which callers, tests, and files are affected before merging.
- You need to catch public API breakage (added/removed/changed exported symbols) introduced by the diff.
- You want to enforce architectural layering rules or proto naming conventions as a pre-merge gate.
- You want to identify lockfile version divergence between two services that share a dependency.

---

## Tools

| Name | Kind | What it gives you |
|---|---|---|
| `review_diff` | MCP tool (investigation) | **The composed verb** (C1-1, `docs/plans/review-verb.md`): diff impact + obligations/findings + architecture conformance + spec coverage in ONE cited verdict, over an explicit `changed_files` list; pass `since` for its `conservation` dimension, which reports the `surface` plane's exported-API base→head delta (below) |
| `blast_radius` | MCP tool (investigation) | Blast radius of changed files: affected symbols, transitive callers, and related tests (widen with `max_depth:0, max_symbols:0` for the unbounded recall `diff_impact` used to give — SR-25 unregistered that alias) |
| `arch_conformance` | MCP tool (coding) | Call-graph edges that violate declared layering rules |
| `proto_conformance` | MCP tool (coding) | Naming convention violations in `.proto` files |
| `ingest` | MCP tool (shared) | Re-ingest a service in-session to refresh the store before querying |
| `staleness_report` | MCP tool (shared) | Check whether the indexed state has drifted from the current HEAD, plus (`doc_drift`) Markdown files with inline-code or link references that no longer match known symbols or files — the retired `stale_docs` tool's own scan, folded in (SR-28) |
| `fathom review` | CLI | Store-direct CLI mirror of `review_diff`: resolves changed files via `git diff --name-only <ref>`, runs the same engine (used by the non-blocking pre-push summary, `docs/plans/review-verb.md` C3-1); `--budget-seconds N` (with `--no-persist`) prints a named skip and exits 0 on excess. Not yet listed by `fathom help` — `fathom review -h` |
| `findings_near` / `fathom findings near\|status` | MCP tool / CLI | Prior findings within N call-graph hops of a symbol or file (precedent for a review comment), and the findings layer's entry-condition progress — **experimental** until ≥15–20 resolved findings exist |
| `fathom ingest-diff` | CLI | Ingest only files changed since a given git ref, then exit (pre-session or CI use) |

---

## How it enhances Claude Code

Without fathom, an agent reviewing a PR must read changed files and mentally trace impact — it misses callers defined in files it did not read, cannot compare API surfaces across services, and has no conformance gate. With fathom:

1. **One call composes the whole review.** `review_diff` returns diff impact, obligations/findings scoped to the changed files, architecture conformance, and spec coverage as one cited verdict — the agent does not have to remember (or narrate) an 8-step tool sequence.
2. **Impact is grounded in the call graph.** The verdict's `impact` dimension returns the exact set of symbols and tests that transitively depend on the changed files, so the agent knows what to re-test without guessing.
3. **Layering rules gate automatically, honestly.** The `arch_conformance` dimension produces zero findings on a clean diff, and reports `checked:false` with a reason — never a clean-shaped pass — when no rule pack is configured, so the agent can tell "passed" from "wasn't checked."
4. **Findings are durable.** A finding refuted once (`fathom claim refute` or the `obligations` tool's feedback action) stays refuted across every later re-review of the same diff — no re-litigating the same false positive on every push.
5. **Fewer tokens spent on context assembly.** A single `review_diff` call returns exactly the affected nodes and findings; the agent does not need to scan the whole repo or issue five separate calls to find them.

---

## Worked example

A PR modifies `internal/api/handler.go` in `payment-service`, which now calls into `internal/store` directly — a layering violation the service's own `.fathom/rules/archconformance.cue` forbids.

**1. Refresh the store for the changed service.**

```json
{ "name": "ingest", "arguments": { "service": "payment-service" } }
```

Returns `{ "service": "payment-service", "ok": true }`. The store now reflects HEAD.

**2. Call `review_diff` once with the changed files.**

```json
{
  "name": "review_diff",
  "arguments": {
    "service": "payment-service",
    "changed_files": ["internal/api/handler.go"]
  }
}
```

Returns one cited verdict:

```json
{
  "service": "payment-service",
  "changed_files": ["internal/api/handler.go"],
  "dimensions": [
    {
      "name": "impact",
      "checked": true,
      "count": 2,
      "findings": [
        { "kind": "impact", "symbol": "api.RouteHandler", "file": "internal/api/router.go", "message": "transitively depends on changed code", "citation": "internal/api/router.go#L12-L12", "severity": "info" },
        { "kind": "impact", "symbol": "api.TestHandler_Charge", "file": "internal/api/handler_test.go", "message": "transitively depends on changed code", "citation": "internal/api/handler_test.go#L4-L4", "severity": "info" }
      ],
      "tests": ["api.TestHandler_Charge"],
      "files": ["internal/api/handler.go", "internal/api/router.go", "internal/api/handler_test.go"]
    },
    { "name": "obligations", "checked": true, "count": 0, "findings": [] },
    {
      "name": "arch_conformance",
      "checked": true,
      "count": 1,
      "findings": [
        {
          "kind": "arch_violation", "symbol": "api.Handler",
          "file": "internal/api/handler.go", "line": 42,
          "message": "api.Handler → store.Query: package \"api\" must not import \"store\" (deny rule)",
          "citation": "internal/api/handler.go#L42-L42",
          "severity": "error",
          "claim_id": "3f9a…"
        }
      ]
    },
    { "name": "spec_coverage", "checked": true, "count": 0, "findings": [] }
  ],
  "summary": "service payment-service: 1 changed file(s); 4/4 dimensions checked; 3 active finding(s)",
  "findings_persisted": true
}
```

`arch_conformance` caught the layering violation — cited with its reason and rule-pack-derived source — and the two `impact` findings name exactly which caller and test are affected.

**`review_diff` writes to the claims store — deliberately, and only the oracle dimensions.** `findings_persisted:true` reports that the `arch_conformance` and `spec_coverage` findings (the deterministic-oracle dimensions — never `obligations`, which re-serves already-compiled claims, and never `impact` context noise) were persisted as content-addressed claims on the `findings` layer, each anchored with a hashed line citation and a scope fingerprint so the daemon's post-refresh citation sweep and remediation-drift pass verify them instead of falsifying them. The write is idempotent (an identical re-run upserts identical rows) and self-reconciling: a review-minted claim whose finding a later run over the same files no longer emits is flipped to `superseded` — absent = fixed. If the PR author disputes the violation, `fathom claim refute --claim 3f9a… --note "..."` (or the `obligations` tool's `action:"refute"`) suppresses it durably — a later `review_diff` call re-emitting the identical finding carries `status:"refuted"` (excluded from `count`, still listed) instead of re-flagging it.

The `review_change` canned prompt that used to pre-structure this same call is gone
(SR-20, surface reckoning F-78, zero lifetime calls) — call `review_diff` directly, as
above.

**CLI equivalent:**

```sh
fathom review --store workspace/db/fleet.sqlite --service payment-service --since origin/main
```

Resolves changed files via `git diff --name-only origin/main`, runs the identical engine, and prints a short human-readable summary (`--json` for the same payload shape as above). Like the MCP tool, a plain `fathom review` run **persists** its oracle-dimension findings to the claims store; pass `--no-persist` for a pure report with no store write of any kind — that is what the non-blocking pre-push summary (`docs/plans/review-verb.md` C3-1) uses, because a hook must never be an undisclosed writer to the shared store.

---

### Appendix: manual tool-by-tool sequence

Before `review_diff` existed, reviewing a change meant calling each analyzer separately. This sequence still works — useful when you want only one dimension, or when composing your own workflow — and every tool it calls remains independently available.

**Compute the blast radius of the diff.**

```json
{
  "name": "blast_radius",
  "arguments": {
    "service": "payment-service",
    "changed_files": [
      "internal/billing/invoice.go",
      "internal/billing/retry.go",
      "internal/store/txn.go"
    ],
    "max_depth": 0,
    "max_symbols": 0
  }
}
```

`max_depth: 0, max_symbols: 0` widen past `blast_radius`'s bounded default (depth 2, 500
symbols) to the unbounded, full-recall walk the retired `diff_impact` alias always ran —
what a PR review wants. Returns:
```json
{
  "symbols": ["billing.CreateInvoice", "billing.RetryCharge", "store.RunTxn", "store.CommitTxn"],
  "tests":   ["TestCreateInvoice_Idempotent", "TestRetryCharge_Backoff", "TestRunTxn_Rollback"],
  "files":   ["internal/billing/invoice.go", "internal/billing/retry.go", "internal/store/txn.go",
              "internal/api/handler.go"]
}
```

**Check for public API breakage.** The `api_diff` tool that used to compare two independently-indexed service snapshots was unregistered (SR-26, F-20): the calibrated API delta stays in `conserve`'s `surface` plane, which reads the SAME repository's base→head delta instead of two separately-ingested snapshots — no second service ID to maintain. Call `review_diff` with `since` set to the PR's base ref:

```json
{
  "name": "review_diff",
  "arguments": {
    "service": "payment-service",
    "changed_files": ["internal/billing/invoice.go"],
    "since": "origin/main"
  }
}
```

Its `conservation` dimension's `surface` plane finding reports:
```json
{"plane": "surface", "kind": "removed", "key": "api:billing.Invoice", "detail": "func()", "declared": "none"}
```

`billing.Invoice` was removed — a breaking public API change. (`fathom conserve --since origin/main --service payment-service` runs the same check store-direct, outside a review session.)

**Check architectural layering.**

```json
{
  "name": "arch_conformance",
  "arguments": {
    "service": "payment-service",
    "rules": [
      { "from_pkg": "internal/api",     "to_pkg": "internal/billing", "allow": true },
      { "from_pkg": "internal/billing", "to_pkg": "internal/store",   "allow": true },
      { "from_pkg": "internal/api",     "to_pkg": "internal/store",   "allow": false }
    ],
    "deny_by_default": false
  }
}
```

Returns `{ "violations": [], "count": 0 }` — layer rules pass (or a `rule_source`/`checked:false` honesty split when no rules were configured; see the `arch_conformance` row above).

**Check proto naming conventions (if applicable).**

```json
{ "name": "proto_conformance", "arguments": { "service": "payment-service" } }
```

Returns `{ "issues": [], "count": 0 }`. No violations.

**Check doc and stale-docs coverage on changed files (optional).**

```json
{ "name": "staleness_report", "arguments": { "service": "payment-service" } }
```

Returns any Markdown references that no longer resolve to known symbols or files in the
response's `doc_drift` field (the retired `stale_docs` tool's own scan, folded in — SR-28).

---

## Setup & notes

**`review_diff` never accepts inline architecture rules.** Unlike calling `arch_conformance` directly, `review_diff`'s `arch_conformance` dimension always resolves through the service's own `.fathom/rules/archconformance.cue` project rule pack (or reports `checked:false` when none exists) — there is no `rules` argument on `review_diff` itself. If you need to test rules that aren't committed yet, call `arch_conformance` directly with inline `rules`.

**Surface.** `review_diff` and `blast_radius` are in the `investigation` group; the appendix's remaining tools (`arch_conformance`, `proto_conformance`) are in `coding`, and `staleness_report` (which now carries the doc-drift scan) is `shared`. There is no profile switch (`set_profile` was removed, SW-2): stdio `serve-mcp` serves the full registry; over the shared HTTP daemon the path is the scope (bare `POST /rpc` for everything (the group endpoints were deleted — HF-5)). None of these review tools is on the `/rpc/agent` allowlist, but an off-surface call still executes there — logged `off_surface:true`, never refused — so a session registered on `/rpc/agent` can run the whole review.

**Precedent before verdict.** `findings_near { "service", "subject": "api.Handler", "hops": 2 }` (CLI: `fathom findings near api.Handler --store … --service …`) answers "was this flagged here before?" — live findings ranked by call-graph proximity, superseded ones only with `include_superseded`/`--include-superseded` and always labeled. `fathom findings status` reports how far the findings layer is from its entry condition (**experimental** until then).

**The `surface` plane needs `since`.** Without it, `review_diff`'s `conservation` dimension reports `checked:false, reason:"needs a base ref"` — the plane has nothing to diff against. `since` resolves against the service's own indexed corpus root, never the daemon's cwd.

**`blast_radius` does not require a `concern`.** It takes `changed_files` instead. Pass repo-relative paths as returned by `git diff --name-only <base>`.

**CLI alternative for CI.** When no MCP session is running, use the CLI to update the store before running queries:

```sh
fathom ingest-diff --service payment-service --path /path/to/payment-service --base origin/main --store workspace/db/fleet.sqlite
```

This runs `git diff --name-only origin/main`, ingests changed files, and exits. A subsequent MCP session will pick up the updated store.

**Staleness during long sessions.** A `serve-mcp` process's write can contend with a concurrent CLI `ingest-diff` write (SQLite serializes writers, ADR-020) and fail past `busy_timeout`. Use the `ingest` MCP tool instead to refresh in-session. The `staleness_report` tool tells you whether a refresh is needed before you run impact queries.

**`arch_conformance` rules: inline or project rule pack.** Pass layering rules as JSON in the call (`rules: [{from_pkg, to_pkg, allow}]`), or pass none and the tool falls back to the service's own committed rule pack at `.fathom/rules/archconformance.cue` (validated against `schema/rules/archconformance/schema.cue`; loaded from the corpus root recorded at ingest). The response's `rule_source` field reports which source was used — `"inline"`, `"project"`, or `"none"` — and when no rules were available from either source the response carries `checked: false` plus a `reason` instead of a clean-shaped zero-violation answer. A malformed rule pack is a loud error, never silently treated as "no rules". Scope note: rules match function-origin call edges; method symbols carry receiver-typed package prefixes and are a disclosed gap (see the schema's `#Rule` doc).

---

## The `fathom verify` gate — and `fathom review`'s non-blocking summary

Impact analysis answers *what is affected*; the `fathom verify` gate answers *what is proven done*. Before pushing, `lefthook` runs `fathom verify --quiet` (failing rows plus the summary only) over any plan files in `docs/plans/*.md`, checking each `[x]`-claimed task against the index. A claimed task that lacks code evidence blocks the push (exit 1); an operational error (daemon down) warns and allows (exit 2, fail-open). `--strict` (claimed-but-Unknown rows fail too) is the CI form — `scripts/ci.sh` carries it, run by hand: no CI runner exists yet, so treat it as **experimental**. Run `fathom verify` manually with `--plan <file> --service <id>` or pass `--store` to bypass the daemon. See [spec-verification.md](spec-verification.md) for the full gate reference.

The same pre-push hook also runs `fathom review --service fathom --since origin/main --store … --no-persist --budget-seconds 20` (C3-1, `docs/plans/review-verb.md`) — unlike `verify`, this step is **advisory, never blocking**: it always exits 0 and prints the verdict summary so review findings are visible on every push, even when nothing forces you to read them. It is truthfully a *summary*: `--no-persist` means the hook never writes to the shared claims store (persisting findings is a deliberate CLI/MCP act, not a hook side effect), and `--budget-seconds` (which requires `--no-persist` — a bounded run must never be a store writer) turns an over-budget run into a named skip rather than a stall. An operational error there degrades to a single line (fail-open, same doctrine as `verify`'s exit 2), never a raw error dump. Latency: measured ~12s on a pathological 1777-file diff; a normal `--since` diff is far smaller.

---

## Review findings as compiled claims (`fathom report` + `fathom claim`)

Review conclusions can outlive the review session as governed claims (the findings layer — one markdown file per finding under `findings/`, compiled with `fathom compile --layer findings` or the `compile_layer` MCP tool; kind `finding`, tier `sme-stated`). Two CLI verbs close the loop:

- **`fathom report --store DB --service S --analyzer security_rollup --out findings/`** writes one deterministic markdown finding document from the security analyzers' current output (per-category sections with `path:line` lists; secret values are redacted). Written into the corpus dir, it flows through the normal ingest → compile → claims path; the report's own output directory is excluded from the scan, and re-running against an unchanged tree reproduces identical bytes.
- **`fathom report --store DB --service S --analyzer reverse-drift --out findings/`** is the reverse-SDD-drift check: the service's public API surface (`internal/analysis/apisurface`, with `_test.go` symbols excluded — test helpers are not public API) minus what a spec-bindings claim covers (matched by file-path glob against gated plan rows' `{symbol, package}` contracts) = the surface no plan or spec governs. When the service has zero compiled spec-bindings claims the report banners that explicitly — the listing is then the whole surface, not measured drift. Read-only — it reports the current index/claims state and doesn't persist anything itself, but its output can be pointed at `findings/` the same way security_rollup's is if you want ungoverned-surface findings to join the compiled corpus.
- **`fathom claim refute|corroborate --store DB --service S --claim ID --note "..."`** is the triage channel: `refute` flips the claim to `refuted` (suppression with a note — durable across recompiles); `corroborate` records provenance without touching status or tier. Both are store-direct (`--store` required): stop the daemon first when triaging the fleet store.

Once compiled, findings scoped to the files under review surface through the same `obligations` tool (labeled `kind:"finding"`), and `code_drifted:true` flags findings whose scoped code changed since compile — the "did remediation happen?" query. `fathom layers show findings` prints the pack's resolved definition and prompt provenance; see `docs/layers.md` for the full catalog (`docs/use-cases/coding-agents.md`'s Maintained line owns `fathom layers` itself).

---

## The findings loop: external review findings, imported and served

The section above is fathom's OWN findings (`fathom report`, compiled through the `findings` layer). `docs/vision/30-findings-loop.md` (ADR-040) is the other half: an **external** reviewer's findings — a review kit's own portable journal — entering fathom through a governed import that preserves who found what, at which revision, at which confidence, so they are served at the point of work exactly like a governing rule, never mistaken for one.

- **`fathom findings import --store PATH --service S --subject ORG/REPO/COMPONENT --producer NAME --file PATH [--dry-run] [--json]`** validates and imports one review-kit projection batch (`.fathom/config.toml`'s `[findings] subject`, `docs/configuration.md`, must match exactly — no mapping, no import). The receipt (`{schema_version, delivered, duplicate, stale, unclassified, rejected: [{index, finding_id, reason}], truncated, cursors, dry_run}`) is the audit trail; any rejection exits 1. Identity is deterministic (`ClaimID` over `producer/org/repo/component/sourceId`, `PromptVersion "external-import/v1"`) and isolated from `fathom`'s own native-review reconciliation — an imported finding is never touched by `StaleReviewClaims`, and a `fathom`-origin producer is refused outright (the circular-proof rule, vision 30 §2).
- **`fathom findings candidates --store --service [--min-sites 3] [--json]`** derives rule candidates from patterns recurring across ≥3 distinct sites (native and external findings together) — class, normalized shape, member citations, the producers that saw it, a drafted rule sentence and the ADR-038 guard-check family that could enforce it. This is a **report**: fathom never writes a rule. Each candidate's score is sites × the mean CODEOWNERS-ownership weight of its member files (1.0 when no CODEOWNERS), decayed once per disputed (refuted) member (`internal/analysis/findings.CandidatePolicy`); a caller-set cadence cap emits at most that many per call and counts the rest as `deferred`; the drafted rule carries a `promoted_from: <pattern id>, members: N, producers: [...]` tag so lifting it into `CLAUDE.md`/`guard.cue` keeps provenance. `fathom findings status` gains `candidates`, `disagreements` (a `fail` from one producer and a `pass` from another on the same rule/anchor, both served, never silently resolved), `rules_never_fired_30d` (currently-served rules with no fire in 30 days), and `top_fired` (highest served+touched, from the same hooks beat log every other instrument reads).
- **`fathom claim add --store --service --scope GLOB [--scope GLOB ...] --rule "..." [--remedy "..."] [--cite path#Lstart-Lend] [--json]`** is the single-utterance "remember this": mints one content-addressed, human-authored claim (producer `human.review`, tier always capped at `sme-stated` — `--tier` is refused outright, never silently ignored) without waiting for a pattern to recur. Its `PromptVersion` (`human-authored/v1`) is its own reconciliation domain, distinct from native review (`review-verb/v1`) and the governed import (`external-import/v1`); it is served by the pre-edit hook exactly like any other finding.
- **`fathom claim refute --dry-run --store --service --claim ID [--json]`** previews a refutation before it is recorded: `would withhold N served row(s) across N file(s): <first 5 files>` (JSON: `{claim_id, would_withhold, files, truncated}`), computed by walking the claim's own scope glob(s) against the service's indexed files — nothing is written.
- **Served at the edit**: once `[findings] outbox` is set, the session hook imports a changed outbox once per session; the pre-edit hook renders `open findings on this file (N, showing k)` right after the governing rules and the enforcement verdict — class, producer, rule id, remedy, `path#Lstart-Lend`, and whether the anchor's code has moved since it was observed (`anchor_current`). Findings are advisory there; only a rule a human promotes into `CLAUDE.md`/`guard.cue` (§3.6, `findings candidates`) ever enforces.
- **Honesty rules that never change**: a finding served is not a finding verified; an import receipt is not organizational admission; fathom never approves, disputes, tailors, or closes a review-kit finding on its own authority. `docs/adoption/findings-loop.md` is the end-to-end guide (CI and local variants); `docs/plans/findings-loop.md` is the gated plan.

---

## Maintained

This doc covers: `review_diff` (the composed verb, C1-1/C2-1, `docs/plans/review-verb.md`; incl. its `test_impact` block — `affected_tests` and `test_gap`, BD-4 — and its `conservation` dimension's surface-plane finding, the retired api_diff tool's replacement, SR-26) and its `fathom review` CLI mirror (which prints the same block, BD-5) (incl. `--no-persist`, `--budget-seconds`; `fathom review -h`), `blast_radius`, `arch_conformance` (incl. its `.fathom/rules/archconformance.cue` project rule packs and `rule_source`/`checked` response fields), `proto_conformance`, `findings_near`, `fathom findings` (`near`/`status`; `fathom findings import`/`fathom findings candidates` land in another wave, FL-10/FL-23, `docs/plans/findings-loop.md`), `ingest`, `staleness_report`, `fathom ingest-diff`, `fathom conserve`, `fathom verify` (`--quiet`, `--strict`), the pre-push `fathom review` summary step (C3-1), `fathom report`, `fathom claim` (add/refute/corroborate). Update it when these change.
