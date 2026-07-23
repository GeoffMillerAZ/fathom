# Spec-driven development with fathom — the definitive guide

The full SDD loop: author a spec/plan in fathom's grammar, implement with an
agent (or by hand), prove every claimed row against the code index, and let
a blocking gate — not a human's memory — catch over-claiming before it
leaves the workstation. You should not need to read any other doc to follow
this one; where a topic has a deeper reference elsewhere, it's linked, not
required.

This loop is **git-native and agent-agnostic**: `fathom verify` is a CLI
command backed by lefthook and CI, not a Claude-Code-specific hook. It works
identically whether the plan was written by a human, Claude Code, Codex, or
any other agent — see
[docs/adoption/claude-code.md](claude-code.md) or
[docs/adoption/codex.md](codex.md) first if you haven't wired fathom's MCP
surface into your agent yet; nothing in this guide requires that wiring,
but the discovery tools it recommends for grounding a plan (`pack_context`,
`test_impact`) are faster over MCP than the CLI.

Every command below is verified against this repo's own source
(`cmd/fathom/verify.go`, `internal/verify/*.go`, ADR-018,
`docs/design/sdd-reconciler.md`) and the worked example throughout is
**this repo's own live adoption program** — real plan rows, real
`fathom verify` output, generated with this worktree's own build.

---

## 1. The stance, in one paragraph

Fathom's SDD doctrine (ADR-018, `docs/design/coding-agent-excellence.md`
§4): **specs are IDs, code cites them, plans carry contracts, and "done" is
a verifiable proposition — all deterministic, no LLM anywhere in the verify
path.** State is *computed*, never stored: `verify.VerifyPlan` is a pure
function of (plan text, live index) with no clock and no persisted process
state. The one thing that IS authored, not derived, is *intent* — a `[-]`
skipped task's `disposition` and `reason` say why a human or agent chose not
to do something; everything else (pass/fail, coverage, readiness) the gate
computes fresh every run. This is deliberately **not** a state machine: no
lifecycle enum, no transition engine, no separate SDD binary or repo — see
ADR-018's "Rejected" list for what was tried and discarded.

---

## 2. Spec references — the ID layer

A spec is any stable bracketed ID (`[US-042]`, `[EP-013]`, `[ADR-017]`)
declared in a doc and cited in code comments, commit messages, or plan
titles. `internal/sdd.ExtractRefs` harvests every citation at ingest time;
four MCP tools serve the resulting matrix (spec → implementing symbols/files
→ tests):

| Tool | What it answers |
|---|---|
| `spec_refs` | Every occurrence of spec IDs in a service's source, deterministically sorted (`{specs: {spec_id: [{file, line}]}, total_refs, files_scanned}`); default pattern covers `[US/EP/FR/NFR-N]`, `MN.N`, and `doc N` |
| `spec_coverage` | For each declared spec ID: which symbols implement it, which specs are uncovered, and which referenced IDs are orphans (cited but never declared) |
| `spec_status` | `"covered"` / `"uncovered"` for a list of spec IDs — the cheap yes/no check |
| `traceability` | Extracts spec-ID references from source and builds the symbol → spec-ID map directly |

Convention for fleet repos: new work that implements a documented decision
cites that decision's ID at the primary implementation site — one citation
at the load-bearing symbol, not scattered across every file it touches.

This ID layer is optional — a plan task with no bracketed spec ID and no
`{…}` contract still parses and reports (as `no-contract`/`Unknown`, §3
below); spec IDs sharpen the picture, they aren't required to gate.

---

## 3. Plan files and the contract grammar

Plans live at `docs/plans/<slug>.md` — ordinary Markdown checklists, the
format a human or an agent in plan mode already writes, with an optional
**acceptance contract** per line. Full grammar reference:
[docs/plans/README.md](../plans/README.md); parsed by
`verify.ParseMarkdownPlan` (`internal/verify/markdown.go`).

### Checkbox semantics

| Checkbox | Meaning | Gated? |
|---|---|---|
| `[ ]` | Open — work not yet claimed done | Never (fails only under `--require-all`) |
| `[x]` | Claimed done — **must verify against the index** | Always |
| `[~]` | In progress — not done, not claimed | Never (fails only under `--require-all`) |
| `[-]` | Skipped or abandoned — exempt; refine with `disposition:` | Never |

Checking a box is a **claim**. The entire point of the gate is that claims
get checked, not trusted.

### Annotation keys

One `{key: value, …}` block, on the checkbox line itself (see the
load-bearing placement rule below):

| Key | Meaning | Default |
|---|---|---|
| `symbol` | Claimed symbol that must exist in the index | required for a symbol contract |
| `package` | Repo-relative directory the symbol's file must live under — disambiguates a bare name that exists in several packages. If the filter empties the match set, `symbol_present` fails `claimed-but-absent`; if several distinct symbols with that name remain in the directory, resolution stays ambiguous — never first-match-wins | unset (no filter) |
| `reachable_from` | An entrypoint that must statically reach `symbol` | unset |
| `min_tier` | Minimum evidence tier (`verified`, `pre-scan`, `doc-claimed`, `sme-stated`, `assumed`) | `verified` |
| `skip_test` | `true` exempts the symbol from the "a `Test*` must statically reach it" requirement | `false` |
| `allow_contradiction` | `true` lets contradictions on the symbol pass | `false` |
| `id` | Author-owned stable task ID, overriding the positional `md-NNN` (which renumbers when lines are inserted). Use it in every plan that will be diffed/gated across edits — the Stop hook matches tasks by ID | positional |
| `disposition` | `refused` \| `deferred` — types *why* a `[-]` line is skipped; requires `reason` | unset |
| `reason` | Free-text justification for a disposition or skip (double-quote a value containing commas) | unset |
| `trigger` | Revival condition for a `deferred` task — recorded and surfaced, **never machine-evaluated in v0** | unset |
| `skip` | Justification for `skip_test: true` (machine-visible; linted when absent) | unset |

The zero value of every key is the **strictest** contract; loosen
explicitly. `verify.LintPlan` reports v2 convention violations (duplicate
IDs, dispositions without a reason, unjustified skips) as **warnings** on
stderr — they never change the exit code in v0.

**Annotation placement is load-bearing.** The parser is line-based: an
annotation on a continuation (wrapped) line is silently ignored, and the
task verifies as `no-contract` instead of carrying its intended contract.
Keep the checkbox, title, spec IDs, and `{…}` block on one line, however
long.

**Honesty boundary.** `test_reaches` means a `Test*` function **statically
reaches** the symbol via the call graph — never that the test passes or
even runs. `runtime_pass` is always `Unknown` until CI/coverage evidence is
ingested as its own tier (the `cmd` oracle class, §9 below).

### Live worked example — this program's own contract

`docs/plans/adoption-docs.md` is the plan governing the very guides this
paragraph belongs to. Its row for this document:

```markdown
- [x] docs/adoption/spec-driven-development.md — the SDD loop with this program as the worked example {id: AD-3, symbol: cmdVerify, package: cmd/fathom, skip_test: true, skip: "docs row anchored to the verify CLI at the loop's center"}
```

Read that annotation the way the gate does: `symbol: cmdVerify,
package: cmd/fathom` claims that `cmdVerify` (the function backing this very
`fathom verify` command) exists under `cmd/fathom` — a docs-completion task
anchored to the code it documents, rather than to a symbol this doc itself
doesn't produce. `skip_test: true` exempts it from "a `Test*` must reach
it," with `skip:` naming why: it's a documentation row, not a code row, and
`cmdVerify`'s own tests are the real coverage.

---

## 4. Author a plan grounded in the real index

Before writing `reachable_from` or a `package:` filter, ground it against
the live index rather than guessing:

- `pack_context` — pre-pack ranked evidence (from a concern-scoped query)
  into a token budget; the fastest way to confirm a symbol name and its
  package before citing it in a contract.
- `test_impact` — `{tests: [string]}`: the `Test*` functions that
  transitively reach a symbol via the call graph. If this comes back empty
  for a task you're about to write `skip_test: true` on, that's the moment
  to decide honestly whether the gap is real or the task needs a test
  written first.

This is what "grounded" means in practice: `reachable_from` should name a
real entrypoint the tool actually returned, not one remembered from a
previous session (the index can drift — `staleness_report` before trusting
a stale memory of the codebase, per the companion adoption guides).

Once the annotation cites real evidence, the round-trip CUE shadow
(`fathom gen-plan-cue`, §10) is one way to sanity-check parsing before
committing the plan — it fails loudly (exit 2) on any structural loss.

---

## 5. Implement with an agent

Nothing in this step is SDD-specific — it's the same coding loop the
adoption guides describe (obligations injected on every edit, discovery
through the index instead of grep). The only SDD-relevant discipline here:
**check a box only when the work is actually done**, because the next step
checks your work, not your word.

---

## 6. `fathom verify` proves the rows

```sh
fathom verify --service <id> --plan docs/plans/<slug>.md \
  [--store <path> | --url http://localhost:7575] \
  [--strict] [--require-all] [--concern <id>] [--json] [--quiet-pass]
```

| Flag | Default | Meaning |
|---|---|---|
| `--plan` | required, repeatable | One or more Markdown plan files |
| `--service` | required | Service ID to verify against |
| `--store` | `""` | Direct mode — bypasses the daemon, opens a local store (what CI uses) |
| `--url` | daemon URL | HTTP mode — queries the shared `serve-http` daemon (what an interactive session uses) |
| `--strict` | `false` | Claimed-but-`Unknown` tasks also gate (recommended for CI, not pre-push) |
| `--require-all` | `false` | **Wave gate** (ADR-018 §2): additionally fail on any open/in-progress task — a wave only advances when every task is claimed-and-verified or explicitly skipped |
| `--concern` | `""` | Optional concern for contradiction scoping |
| `--json` | `false` | Machine-readable report instead of human text |
| `--quiet-pass` | `false` | Suppress output on a passing gate |

**Exit codes are the whole contract:** `0` — every claimed task verifies.
`1` — a claimed `[x]` task **failed** verification — this is the block.
`2` — operational error (daemon unreachable, service not indexed, or a
genuinely empty index) — **fail-open**, never a false "all claims false."
The HTTP path probes `list_services` before verifying anything for exactly
this reason: a service with zero symbols would otherwise make every claim
read as "not found," which is an infrastructure problem wearing an
evidence-shaped costume.

### Real run — this repo's own plan

`docs/plans/activation.md` (Phase A — Activation) is a live, currently
gating plan in this repo. Running the actual verify command against this
worktree's index:

```sh
$ bin/fathom verify --service fathom --store workspace/db/fathom.sqlite --plan docs/plans/activation.md
✓ [A1-1] server_stats surface_traffic block: per-surface {calls, last_ts} from the call-log counters, covered by fake-backend test
✓ [A1-2] CLAUDE.md registration URL corrected to /rpc/agent + restart caveat; adoption-clock t0 stamp convention documented in the plan + vision doc
✓ [A2-1] Compile idempotency guard: same (service, layer, watermark, prompt identity) → refuse with citation; --force logged escape; unit-tested against a fake store
✓ [A2-2] scripts/recompile-fleet.sh: 11 service·packs, claude-cli lane default, per-pack failure isolation, completion stamp
✓ [A2-3] (op) Recompile fired across the fleet; stamp exists; post-compile: obligations claims fresh at HEAD for all 9 services, adr-rules + spec-bindings have >0 production claims on fathom
✓ [A3-1] Expert-module portability proven: agentic-kit + app-kit obligations compiles required zero code changes beyond flags — else this row stays open and the portability defect is fixed first
✓ [A4-1] Catalog de-theater: eval vs production columns in docs/layers.md + layers CLI verb reading production truth from the store

7 tasks total; 5 claimed: 5 verified, 0 failing
```
Exit code `0`. Five `[x]` rows verified against real symbols
(`registerServerStats`, `cmdCompile`, `cmdLayersList`); two rows (`A2-3`,
`A3-1`) stay `[ ]` — open, operator-gated work, correctly excluded from the
claimed count and *not* gating a normal push.

Add `--require-all` to that exact command and the exit code flips to `1` —
the wave gate refuses to pass while `A2-3`/`A3-1` are open, even though
nothing claimed is failing. That's the difference between the ordinary
pre-push gate (§7, gates only what's claimed) and the wave gate (gates the
whole plan's completeness).

### Real run — catching an over-claim

The historical record: the very first run of this gate, against
`h1-coding-agent-excellence.md`, caught its own author (an AI agent)
over-claiming — the exact failure mode the gate exists for
(`docs/plans/README.md`'s "Recorded demo"):

```
✗ [md-001] Adoption instrument repaired: … {symbol: session_stats, skip_test: true}
    symbol_present: "session_stats" not found in the index
…
15 tasks total; 9 claimed: 6 verified, 3 failing        → exit 1, push blocked
```

`session_stats` turned out to be a Python function inside a bash heredoc —
not an index-provable symbol. The claim was rewritten honestly rather than
argued with.

You can reproduce the failure mode yourself with this worktree's binary
against a two-line scratch plan (never commit a plan like this — it's
purely illustrative):

```sh
$ cat > /tmp/demo-plan.md <<'EOF'
- [x] Ghost function is wired {id: DEMO-1, symbol: TotallyMadeUpFunctionName, package: cmd/fathom}
- [x] verify CLI exists {id: DEMO-2, symbol: cmdVerify, package: cmd/fathom, skip_test: true, skip: "demo row"}
EOF
$ bin/fathom verify --service fathom --store workspace/db/fathom.sqlite --plan /tmp/demo-plan.md; echo "exit: $?"
✗ [DEMO-1] Ghost function is wired
    symbol_present: "TotallyMadeUpFunctionName" not found under "cmd/fathom" in the index
✓ [DEMO-2] verify CLI exists

2 tasks total; 2 claimed: 1 verified, 1 failing
exit: 1
```

DEMO-2 (a real symbol, honestly annotated) verifies; DEMO-1 (a claim about
code that doesn't exist) fails with the exact missing symbol named, and the
process exit code is `1` — this is what blocks a `git push` in the next
section.

---

## 7. The pre-push gate blocks over-claiming

This repo's own `lefthook.yml` pre-push block (generalize `<service>` and
the plan glob for your repo):

```yaml
pre-push:
  commands:
    fathom-verify:
      run: |
        [ -x bin/fathom ] || exit 0
        shopt -s nullglob
        plans=()
        for f in docs/plans/*.md; do
          [ "$(basename "$f")" = "README.md" ] && continue   # READMEs describe, plans claim
          plans+=("$f")
        done
        [ ${#plans[@]} -eq 0 ] && exit 0
        rc=0
        for f in "${plans[@]}"; do
          bin/fathom verify --service <service> --plan "$f"
          code=$?
          if [ $code -eq 1 ]; then rc=1; fi
          if [ $code -eq 2 ]; then
            echo "fathom verify: operational error — allowing push (fail-open)"
          fi
        done
        exit $rc
```

`fathom init` prints this exact snippet as a manual step (it deliberately
never edits `lefthook.yml` itself — that file is hand-curated). In CI, add
`--strict` and build a throwaway store rather than depending on the daemon:

```sh
fathom ingest --service <svc> --path . --store /tmp/ci-store.sqlite
fathom verify --strict --store /tmp/ci-store.sqlite --service <svc> --plan docs/plans/<slug>.md
```

CI is the fail-**closed** environment — "daemon down" is never a valid CI
state, so exit `2` should be treated as a build failure there, not warned
past. Pre-push stays fail-**open** on exit `2` by design
(`docs/design/coding-agent-excellence.md` §5.3): a gate that blocks a push
because a `launchd` job happened to be down gets disabled within a week,
and that failure mode is worse than an occasional unverified push slipping
through on a bad day.

---

## 8. Computed progress: `plans status` / `plans next` *(landing this wave)*

E3 (`docs/plans/new-use-cases.md`) adds two read-only CLI verbs that turn
the same evidence `verify` already computes into rollups, rather than
hand-tracked progress notes:

- **`fathom plans status`** — a per-plan rollup of
  `{proven, claimed-unproven, open, deferred-with-trigger}` counts, computed
  by the existing verify engine, never asserted by hand.
- **`fathom plans next`** — the ordered work frontier: open/unproven rows
  with their `{symbol, package}` anchors, in a shape an agent can consume
  directly to pick up the next task.

Both are CLI-only in v0 (no new MCP tools). **These verbs are not yet built
in this worktree** — this section documents the committed design
(`docs/plans/new-use-cases.md`'s E3 rows) so the shape is right when they
land; treat the exact flags as provisional until `fathom plans status
--help` exists. Regression timelines (progress *over time*, not just current
state) depend on a separate claim-events design landing first and are
explicitly deferred past this wave.

---

## 9. Archival lifecycle: `plans archive` / `plans restore` *(landing this wave)*

E4 (`docs/plans/new-use-cases.md`) — also not yet built in this worktree,
documented here for the same reason as §8:

- **`fathom plans archive <plan>`** refuses unless (a) every row is proven
  or dispositioned, (b) the plan's task IDs appear in `CHANGELOG.md` or
  `ADR.md` (a lexical absorption check — the work has to be recorded
  somewhere durable before its plan file disappears from the gate's view),
  and (c) every anchor still resolves. On success, the plan file moves to
  `docs/plans/archive/`, a tombstone records the evidence, and the pre-push
  gate scopes to active plans only — bounding the gate's cost as the number
  of historical plans grows without bound.
- **`fathom plans restore`** must exist before the first real archive runs
  — an archive with no restore path is a one-way door, and E4's design
  explicitly refuses to ship one without the other.

Claims persistence beyond archival (a restored plan's claims still meaning
something) depends on the compile idempotency work (A2,
`docs/plans/activation.md`) already having produced production claims —
already shipped in this worktree, so archival's own dependency is clear the
moment `plans archive` lands.

---

## 10. Binding specs to code: the `spec-bindings` layer pack

Where the plan grammar (§3) makes a task's contract *checkable*, the
`spec-bindings` semantic layer pack makes an open task's binding
requirement **visible at edit time** — the same PostToolUse channel that
serves `obligations`, feeding one more `kind`-labeled claim into the same
injected payload:

```sh
fathom layers show spec-bindings          # full resolved definition
fathom compile --service <id> --layer spec-bindings --dry-run=false
```

- **Corpus:** `docs/plans/**` excluding `README.md` (the grammar reference
  itself carries no gated tasks to bind).
- **Extraction discipline:** only task lines whose annotation states a
  `package:` value are extracted; a `disposition: deferred`/`refused` task
  or an unannotated line is skipped, never guessed at.
- **Completed tasks compile too, by design** — a `[x]` task's binding still
  describes the package's contract (the work was done *to* that rule), so
  it serves exactly like an open task's binding. The honest caveat: a
  long-lived plan file accumulates completed rows, and a stale completed
  binding is historical noise at edit time by the time the package has
  moved on — retire individual claims with `fathom claim refute`, or prune
  old plans out of the corpus with a project-local `.fathom/layers/`
  override's `corpus.exclude`.

This is what turns "there's an open task about this file somewhere" into
"here's the binding requirement, right now, while you're editing it" —
without a second source of truth: the plan file stays the one place a
binding is authored, the layer just makes it visible where it matters.

---

## 11. Oracle classes — what a gate check is actually allowed to ask

A gate check is a conjunction of criteria answered by named **oracles**
(`docs/design/sdd-reconciler.md` §5); the moment a check needs OR/negation
logic, that logic becomes a Go oracle rather than a prose rule a human has
to interpret consistently. v0 ships exactly one class wired:

| Class | Status | What it answers |
|---|---|---|
| `index` | **shipped** | `symbol_present`, `tier`, `test_reaches`, `edge_wired`, `no_contradiction`, `spec_covered` — everything `verify` checks today |
| `cmd` | future | A pinned command's exit code → `runtime_pass` (the one thing `index` explicitly cannot answer — a reached test is not a passing test) |
| `lexical` | future | agentic-kit's evidence-signature `ABSENT_PROVEN` class — deterministic, Model-empty pattern absence (already shipped for one narrow case: the authorship-trailer parser, `docs/plans/new-use-cases.md` E2, is a `lexical`-class oracle in practice even though the general class isn't wired into `verify` yet) |
| `eval` | future | agentic-kit's eval-runner behavioral verdicts |
| `human` | future | `sme-stated` ratification — a person's explicit sign-off as its own evidence tier |

Only `index` gates anything today. This is why `runtime_pass` is `Unknown`
everywhere in this guide: no `cmd`-class oracle is wired yet to turn "a
test reaches this code" into "this test passed." When it lands, it's
additive — existing `index`-only contracts keep meaning exactly what they
mean today.

---

## 12. The CUE shadow (optional, informational)

`fathom gen-plan-cue --plan docs/plans/<slug>.md --service <id>` emits a
**generated-only** CUE document (`schema/plan`'s `#Plan`) and proves the
emission is lossless by re-parsing its own output and comparing task-for-
task with the Markdown parse — any divergence is exit `2`. Nobody authors
this CUE by hand; it exists so a plan's structure has a typed, machine-
checkable shadow without changing what anyone writes. Real output against
this repo's own `activation.md`:

```sh
$ bin/fathom gen-plan-cue --plan docs/plans/activation.md --service fathom
// Code generated by fathom gen-plan-cue; DO NOT EDIT.
// CUE is the typed shadow of the Markdown plan (ADR-018 §3).
package plan

service: "fathom"
tasks: [
	{
		id: "A1-1"
		title: "server_stats surface_traffic block: per-surface {calls, last_ts} from the call-log counters, covered by fake-backend test"
		state: "claimed"
		expect: {
			symbol: "registerServerStats"
			package: "internal/mcp"
		}
	},
	...
]
```

---

## 13. Session-granularity enforcement: the Stop hook (Claude Code only)

One piece of this loop *is* Claude-Code-specific: the **Stop** hook
(`fathom stop-hook`, ADR-018 §4) diffs which plan tasks were newly claimed
`[x]` **during this session** against the file's state at git HEAD, and
re-verifies just those against the index — advisory by default,
`{"decision":"block"}` with `--block`. It's the strongest in-session form
of "done means proven," because it fires before the session even ends, not
just before `git push`. See
[docs/adoption/claude-code.md §7.3](claude-code.md#73-stop--stop-hook-done-means-proven-session-granularity)
for the wiring; Codex has no Stop-hook equivalent, so on that harness the
pre-push gate (§7 above, identical either way) is the earliest enforcement
point — see
[docs/adoption/codex.md §6](codex.md#6-honest-capability-table) for the
full honest comparison.

---

## You know it works when…

```sh
# 1. Build is current
bin/fathom version
git rev-parse HEAD

# 2. A plan with at least one [x] contract exists
ls docs/plans/*.md
```

- [ ] `bin/fathom verify --service <svc> --plan docs/plans/<file>.md` exits
      `0` on a clean plan, printing `N tasks total; M claimed: M verified, 0
      failing`.
- [ ] Checking a box for a symbol that doesn't exist and re-running `verify`
      fails with the exact missing-symbol citation (`symbol_present:
      "<name>" not found under "<package>" in the index`) and exit code `1`
      — never a silent pass.
- [ ] `fathom verify --require-all --service <svc> --plan <file>` fails
      (exit `1`) while any task is still `[ ]`/`[~]`, and passes once every
      task is claimed-and-verified or explicitly `[-]`-skipped with a
      `disposition`/`reason`.
- [ ] `git push` in the repo runs lefthook's pre-push hooks, including the
      `fathom-verify` command from §7, and a deliberately broken claim
      blocks the push locally, before CI even sees it.
- [ ] `fathom gen-plan-cue --plan <file> --service <svc>` exits `0` (the
      round-trip check passed) rather than `2`.

If every box above is checked, the full loop — author, implement, prove,
gate — is live and honest.

---

## Maintained

This doc covers: the `docs/plans/*.md` grammar (`{id, symbol, package,
reachable_from, min_tier, skip_test, allow_contradiction, disposition,
reason, trigger, skip}`), `fathom verify` (`--strict`/`--require-all`/
`--store`/`--json`/`--quiet-pass`), the lefthook pre-push gate, the
Stop hook (`fathom stop-hook`/`--block`), `fathom gen-plan-cue`, the
`spec-bindings` layer pack, the oracle-class registry
(`docs/design/sdd-reconciler.md` §5), and the MCP tools `spec_refs`/
`spec_coverage`/`spec_status`/`traceability`/`pack_context`/`test_impact`/
`verify_plan`. `fathom plans status`/`next`/`archive`/`restore` are
documented from the committed E3/E4 design
(`docs/plans/new-use-cases.md`) and marked landing-this-wave until they
ship — update this doc's examples to real CLI output the moment they do,
and update this line whenever any of the above change.
