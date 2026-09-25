# Theme-guide memory v2: method preparation, not runtime acceptance

## Bounded task

- Packet: R3 method-only preparation within the resumed adoption/style-guide work.
- Need: enact the owner's prospective memory direction without reclassifying failed
  v1 RSS results or pretending isolate metrics establish exact app-owned allocation.
- Deliverable: [ADR-0073](../adr/0073-separate-memory-metrics-from-attribution.md),
  [fixed contract](fixtures/styleguide-performance/memory-v2-contract.md), independent
  pure metrics subcheck and adversarial synthetic tests. Existing acquisition is v1.
- Inputs: app-kit main `403db1c`; initially clean worktree, existing stash untouched.
  Selected local CDP definitions and their hash are recorded in the contract.
- Authority: kit files, synthetic tests and reviewed root commit/push only. No
  consumer, dependency, daemon, global config, installed binary or real-data change.
- Acceptance: finite/consistent raw measurement validation, independent per-domain
  boundaries, complete process-cost reporting, missing-evidence refusal, no claim of
  attribution or complete memory acceptance. Historical v1 sources remain unchanged.
- Cost cap: two small-model tasks (implementation and review), one initial synthetic
  test round plus at most two repair rounds; zero browser acquisitions. Stop if
  missing evidence requires expanding the runtime collector. Preserve failed bytes.
- Return: reusable metric/checker controls in the kit; no readiness promotion. Human
  A/B, Fathom's parked guide and Conjure's concurrent adoption remain separate.

## Why this is not a memory pass

Heap use is isolate-wide. Native navigation may replace target/session/isolate IDs;
DOM counters have no documented per-frame attribution guarantee. Deduplication avoids
one obvious overcount, but does not prove ownership or absence of detached retention.
The new output explicitly keeps attribution unverified and both completion flags false.
Chrome RSS is reported in full, not blank-adjusted and not unique physical memory.

The three independent growth budgets are prospective4MiB limits after twenty native
cycles. The JS budget preserves v1's limit; embedder GC heap and backing storage are
additional separately checked domains, never a sum claimed to be disjoint.

## Verification

Private evidence root: `/Volumes/m2ext/appkit-evidence/appkit-memory-v2.3hw44n`.
The first frozen evaluator/test revision passed35 tests. Review still found real
fixture gaps: fractional monotonic clocks, preview selector queries, malformed
target/GC/session entries, loopback origin scope and future endpoint samples.
Those were corrected together; first bytes remain in `first-inputs.tar`.
The final joint v2/v1 suite passed70/70 with no skips, exit0. A final unchanged
replay retained stdout/stderr; no browser, host process sampling or real data ran.

```sh
/opt/homebrew/Cellar/node/26.3.0/bin/node --test \
  docs/assessments/fixtures/styleguide-performance/memory-v2-metrics.test.mjs \
  docs/assessments/fixtures/styleguide-performance/memory-check.test.mjs
```

| Retained item | SHA-256 |
|---|---|
| First inputs (`first-inputs.tar`) | `0ab38e79c45227e6accf39242c686b28e235fce097d9c0dcfe7d40987f019470` |
| Final inputs (`final-inputs.tar`: v2 module/test/contract and v1 module/test) | `be6c429184b76e2cbe7f4cdfded7d0212971af3cbc1ae6509cb29ce1f3cbdbaa` |
| V2 module | `588027022beaca143f35bb2ab50a6fee4dcf7d407be14c19961849cbaf76b192` |
| V2 tests | `7782c1b19bf0e0d1a5b6714921502f3a2328376e96c0732308b144fe840cc1b2` |
| Final test stdout (`verified.stdout`) | `d370157ef48eaa2deef3eb1d9a21f8786ef58b369c2ed2b19645f6e96bcbd5a5` |
| Final test stderr (empty) | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` |
| Selected Node26.3.0 executable (identity checked, not archived) | `56694c81b093cc8da273fa017cf91765b3653e5f64f16727976ffaa87b2b6b31` |

Independent small-model review covered the contract and evaluator; the main agent
reviewed the consolidated repairs and unchanged v1 source boundary. A retained
`verify-docs.mjs` check resolves205 local links with no missing target and verifies
the ADR index plus scope149/ADR57-line budgets. Its result `docs-check.json` SHA is
`4dd184dcd192566a69f315fab2cb1a79202353332a6176692791db5d5f1c1cd4`;
the check script SHA is `8b609e3b63483f80fd586a675dc749d01527f8c14546b1e6a70481c9f7bda399`.
This evidence-summary prose was added after that documentation check.
`git diff --check` passes. V1 collector, sampler, transport, guard, contract and
checker/test have no diff from `403db1c`. An initial external-volume read returned
`Operation not permitted`; one unchanged retry succeeded, without configuration changes.
No v2 browser run, app optimization, consumer adoption or gate promotion is claimed.

## Next action and stopping rule

The contract specifies the missing frame/context/loader/session/isolate evidence and
independent control requirements. Resume only a selected single-cell method check;
stop at the first unknown-target/containment/identity failure and preserve it. Do not
repeat the twelve-cell matrix merely because the numerical subcheck is available.

## Decision-register follow-up

Commit `81af294` passed its then-existing pre-push gates but omitted the D73 row/body
in `docs/DECISIONS.md`. That second register supplies MCP guidance. The final handoff
inspection caught it; the existing consistency test had not run in those gates.
INC-58 records the escape. No metric tests or historical receipts were changed.

Added repository `decision-index` to pre-push, running the existing
`TestDecisionsIndexCoversEveryADR`. Executing that actual hook against the missing row
exits1 with the exact missing filename; after adding D73's row/body it exits0.
The repair also updates identical maintainer instructions in CLAUDE.md/AGENTS.md.

| Artifact in the same private evidence root | SHA-256 |
|---|---|
| `index-red-inputs.tar` | `98503b4e960d1ae78dfc199eec450e85f25cf6fba4322d5157d8e4492aa4de86` |
| `index-red.stdout` | `0104521fa086f9176c1c95c2b9a2fc1e29a9a1ade7e1de138418fb7ea76bbd26` |
| `index-green-inputs.tar` | `da7fe2ceaf5b600e4203ecb19b1f25f9a348bc600ac860b5357735fb93ca13ce` |
| `index-green.stdout` | `bc9fed7afe74a5a3a370eeb95ec6ec286df9e0fa2c080a79bea1c7b3e8715444` |

Scoped intent-doctor checks confirm router80, scope149, incidents148 and Unreleased60
lines; no owned living intent doc exceeds its budget. A closed chart incident was
compressed without changing its evidence or outcome (longer text remains in81af294).
One optional blank list separator was removed from Unreleased. No goal or failure
was removed, and no full host-configuration/intent audit is implied.
The full `go test ./cmd/appkit-mcp -count=1` suite also passes (exit0,8.656s).
Its retained `mcp-suite.stdout` SHA is `77258db690a7a823554a78606ffbe5fdfa7390251e023fb0c3440a0d18d4e927`;
stderr is empty. Selected intent/maintainer stanzas match across both agent files.
