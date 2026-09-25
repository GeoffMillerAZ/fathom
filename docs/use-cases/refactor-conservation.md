# Refactor Conservation

Run a refactor under a conservation law: fathom computes what a change must
keep (the capability inventory, certified behaviour, the rules bound to the
moved code), proves that the proof itself was not tampered with, and reports
what was lost, what was declared, and what could not be checked. fathom
never writes the change and never grades it — planes and missing inputs,
stated plainly; a Review Kit requirement judges adequacy (vision 34,
ADR-044).

---

## When to use this

- An agent (or you) is about to retire, move, or rename a symbol, a route, a
  wire field, or a whole package, and you want the loss made explicit before
  it lands, not discovered later.
- You want the pre-push hook to print a conservation summary on every push
  without blocking it, and `--strict` to block it in CI once you trust the
  signal.
- You want a gate-ready plan generated for a large retirement instead of
  hand-writing one: a pinned baseline, ratcheted absence rows scoped to
  today's callers, and the certificates the change must not silently break.
- You want to know, before you trust an "it's all gone" claim, whether the
  index backing it is actually at HEAD.

Not for: making the change (fathom proves; it never transforms code — see
`docs/intent/scope.md`), an ordinal safety score (there is no such thing
here — see I44), or absence proofs for what the Go compiler already proves
(an unused import; `gone: pattern`/`gone: callers` cover what the compiler
does not).

---

## Tools

| Name | Kind | What it gives you |
|---|---|---|
| `fathom conserve --since <ref>` (`--json`, `--strict`, `--plan FILE`) | CLI | The conservation report: every plane's verdict, findings, and missing inputs, `fathom.conserve/v1` |
| `fathom refactor plan <target>` (`--out`, `--service`, `--store`) | CLI | Writes a gate-ready plan for retiring one symbol, import, or package: baseline, ratcheted absence rows, certified_by rows, a closing conserve instruction |
| `fathom refactor status --plan FILE` (`--json`) | CLI | Each ratchet row's live count against `max:`, its count at the baseline, and drift landed on main since |
| `fathom review --since <ref>` / `review_diff` (`since` arg) | CLI / MCP tool | The `integrity` and `conservation` dimensions folded into the same review a change already gets |
| `fathom verify --since <ref>` | CLI | Self-declared rows and deleted rows rendered across the whole base..head range (I45), not just the last edit |
| `fathom verify` (`gone:` plan key) | CLI | Proves an import, module, lexical pattern, or kept-but-unused symbol is actually gone — `Unknown`, never falsely proven, on a stale or partial index |
| `fathom report evidence` (`--service`, `--json`) | CLI | Per-service evidence density: typed edge share, edge tier per language, certificate density, inventory counts, receipt re-derivation — ranks no languages |

---

## 1. Pin a baseline and generate the plan

```sh
fathom refactor plan OldHandler --service myservice --store workspace/db/fleet.sqlite --out docs/plans/retire-old-handler.md
```

This writes a `Baseline: <HEAD sha>` header, one ratcheted `gone: pattern`
row per package that still calls `OldHandler` today (`max:` seeded at
today's own count, scoped to that package), a `certified_by:` row for every
live-certified test that reaches it, the final unscoped `gone: callers
OldHandler` row proving the whole corpus, and a closing instruction to run
`fathom conserve --since <baseline> --strict`. Nothing here transforms
code — the plan describes what must become true; you (or the agent) make it
true.

## 2. Make the change

Delete, move, or rename as the plan implies. Commit as many times as you
like — every check below reads base..head as a whole, so a rename in one
commit and its citation update in another are still caught together.

## 3. Watch the ratchets tighten, not loosen

```sh
fathom refactor status --plan docs/plans/retire-old-handler.md --json
```

Each row's `current` count must move toward zero relative to `baseline`;
`drift` is `max(0, current-baseline)` — references that landed on the
branch (or on main, if you rebase) since the baseline, so a ratchet never
silently loosens just because someone else's commit added a call the plan
never anticipated.

## 4. Conserve, then verify

```sh
fathom conserve --since <baseline> --service myservice --store workspace/db/fleet.sqlite --strict
fathom verify --service myservice --plan docs/plans/retire-old-handler.md --since <baseline> --store workspace/db/fleet.sqlite
```

`conserve` reports one line per plane:

- **`conserved`** — checked, nothing undeclared lost.
- **`losses`** — at least one finding not declared before the range began.
- **`unproven`** — an input the plane needed was missing (index not at
  HEAD, a typed package failed, a test run unbound to a commit, an
  unsupported language) — **never** read as `conserved` (I44); the missing
  input is named so you know what to fix before trusting the plane.

Planes today: `integrity` (a certifying or enforcing test deleted, renamed,
weakened, skipped, or its golden data/marker moved — informational `edited`
and clean `renamed` findings never fail `--strict` on their own),
`surface`/`certificates`/`governance` (the conservation ledger: inventory
delta, lost certificate declarations, orphaned governance claims),
`absence`/`resurrection` (a new caller of a `Deprecated:` symbol; a deleted
function body reappearing under another name — the deterministic
Blindness detector), each named in `docs/design/refactor-conservation.md`.

A **declaration** (`retires:`/`surface_change:` on a plan row, a loosened
`max:`, a skip) excuses a finding only when it was present **at base**,
before the range started. One made **inside** the range is reported
`declared: self` and excuses nothing (I45) — the agent that made the change
cannot grade its own row, generalized from O56's single-commit check to the
whole base..head range.

## 5. Prove absence the compiler doesn't

```markdown
- [x] The old auth path is gone {id: RM-1, gone: callers OldHandler, package: internal/auth}
- [x] No file imports the legacy client {id: RM-2, gone: import example.com/pkg/legacy}
```

`fathom verify` evaluates `gone:` rows directly: `import`/`module` (no
Go file/`go.mod` require left), `pattern "<regex>"` (zero lexical matches,
`max`/`min`/`eq` bound the count), `callers <Symbol>` (zero typed
referents, test referents counted separately unless `tests: allowed`). A
row proves only when the index is at HEAD, the typed pass has zero failed
packages in scope, and no build-tag-excluded file references the target —
otherwise `Unknown`, never a false "gone" from a stale or partial index.

## 6. The pre-push warning and the Review Kit pack

`lefthook.yml`'s `refactor-conserve` step runs `fathom conserve --since
"$since"` (the branch's own upstream, falling back to `origin/main`) on
every push, **as a warning only** — it never blocks (`fathom verify` above
is the gate). CI runs the same command with `--strict` to actually fail the
build. Review Kit's `refactoring-conservation` pack (adapter
`fathom-conserve`, over `fathom conserve --json --since <base> --service
<service>` — `--service` is required, so the adapter is built with the
sounding's own configured Service rather than left to a directory-name
guess) is armed at **warn** on this repository — its `refactoring.no-undeclared-loss`
requirement fails on any finding whose `declared` is not `prior`; the
`refactoring.uncovered-behaviour` requirement is advisory (typed-judgment
lane, never a gate by itself).

## 7. Evidence density, honestly ranked never

```sh
fathom report evidence --service myservice --json
```

Typed share of call edges, the edge tier per language touched
(`typed`/`scip`/`file-resolved`/`syntax`), certificate density, inventory
item counts, and whether a sampled receipt re-derives from a bare
checkout — one row per service, never reordered by which language's
tooling happens to be more mature (I44's honesty extends here: better
tooling is not "conserved better").

## 8. Pack it for a reviewer

`review_diff --since <ref>` (or `fathom review --since <ref>`) folds the
same `integrity` and `conservation` findings into the review a change
already gets, so a reviewer — human or Review Kit — sees guarded-test
losses and conservation planes beside blast radius, obligations, and
architecture in one pass instead of a second invocation.

---

## Setup & notes

- **Cost scales with the change, not the repo** (P19): base-side facts come
  from `git show <base>:<path>`/`git ls-tree <base>` over the changed files
  only — there is no second full ingest of the base commit.
- **Certificates bind to a commit** (O114, ADR-043 amended 2026-09-24): a
  certificate is `live` only for a passing run bound to a commit after
  which nothing it reaches changed. Existing certificates read `unproven`
  until re-attested — `task tests:attest` does that for this repository.
- **A rename is not a loss.** `conserve.Resurrected` only flags a body that
  reappears under another name when the range (or an earlier one) actually
  **claimed** the original retired (a `retires:`/`gone: import` row); an
  ordinary, undeclared rename reads `resurrection`/`moved`, informational,
  never a loss.
- **CV-M4 fixed (wave 3, 2026-09-24):** the Go `api:` surface delta now
  applies apidiff's own struct/interface/alias compatibility rules
  (`internal/analysis/inventory/apicompat.go`, built from the same
  per-file `go/ast` parse, no `go/types` pass) instead of a whole-text
  diff — an `interface{}`↔`any` modernization or a compatible additive
  struct field is no longer a false `changed`, and a type-alias
  introduction (`type X T` → `type X = T`) is caught. Measured 0.960
  precision / 1.000 recall over the full 50-pair bar
  (`docs/bench/readings/cv-m4-apidiff-full50-2026-09-24.md`); `surface`
  plane `api:` findings can be trusted at this precision, with one named
  residual gap — a type alias declared in an IMPORTED package (not the
  changed package itself) is still invisible, since resolving another
  package's own declaration is out of this fix's stated scope.
- **Open measurement:** `conserve.Resurrected`'s false-positive rate on a
  genuine, unseeded large removal can run high when two long-lived,
  independently-written implementations converge in shape (24 spurious
  findings replaying ADR-025's own DuckDB removal,
  `docs/bench/readings/cv-m5-blindness-replay-2026-09-24.md`) — read a
  `resurrection` finding on a big deletion as a prompt to look, not as
  proof of copied code.
- **`fathom refactor plan`/`status` and `gone:` row proving are the newest
  surfaces here** — read `docs/plans/refactor-conservation.md` for exactly
  which rows are proven versus still open (stage 4's `(op)` measurement
  rows, the decision-port rows in §G, and the cross-repo test-kit/Review
  Kit rows in §H are tracked there, not claimed here).

---

## Maintained

This doc covers: `fathom conserve` (`--since`, `--json`, `--strict`,
`--plan`), `fathom refactor plan` (`--out`, `--service`, `--store`),
`fathom refactor status` (`--json`), `fathom review` (`--since`) and
`review_diff` (`since`), `fathom verify` (`--since`, the `gone:` plan key),
`fathom report evidence` (`--service`, `--json`). Update it when these
change.
