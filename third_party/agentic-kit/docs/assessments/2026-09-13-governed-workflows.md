# Governed workflow delivery — 2026-09-13

The assessment's highest-value changes are now working compositions, not just a
larger pattern catalog. [Acceptance and static wiring](../plans/2026-09-governed-workflows.md)
remain separate from executed gates, consumer adoption and live coverage below.
The [2026-09-14 follow-up](2026-09-14-closure.md) closes Unicode wire-decoding gaps
and separates remaining cross-repo asks from this completed implementation.

## Delivered boundaries

| Path | What is enforced | What remains caller-owned |
|---|---|---|
| Engineering profile | scope, effect gate, single-attempt reservations, truncation refusal, independent completion checks | honest executor accounting, tool/destination authority and the completion oracle |
| Sourced prose | exact versioned byte evidence, per-claim review, arithmetic/coverage, correction invalidation, redaction-safe export map | source truth/access, richer semantic reviewers and actual publication |
| HITL | exact operation/actor/version/expiry, atomic local decision consumption, no resend after ambiguous outcome | transaction durability and authoritative destination reconciliation |
| Providers/MCP | terminal-authoritative Responses tools, typed effort/schema, descriptive family, opt-in metadata pin verification | provider assertions, actual independence, remote metadata/dispatch race |
| Native hosts | reversible ownership-aware configuration, strict delivered-path denials, version/config-bound receipts | native trust, permissions, unhooked tools, hook-failure behavior |
| Lesson transfer | bounded reviewed files and independent receiver checks | local acceptance, reproduction, persistence and revocation knowledge |

The non-Go front door is `kit-workflow prose check|export` and `lesson
validate|adoption-check`. Local JSON assertions are not authenticated source or
publication authority. Start with [sourced prose](../adopting-sourced-prose.md),
[portable workflows](../adopting-workflows.md), or [native hosts](../adopting-hosts.md).

## Verification and adoption

Root integration through `8b25230` passed `task validate` in a stable isolated
worktree; final code through `387a7e8` also passed in the actual root workspace:
race tests, formatting, vet, CUE fixtures, constitution checks and nested-module
commands. PDF/XLSX nested modules have no tests. Actual JSON/CUE tests
now cover sourced artifacts/exports, CLI envelopes, scenario reports, host receipts
and lesson payloads/adoption receipts.
Independent host review reproduced three bypass/preservation defects before repair;
its independent regressions pass after repair (INC-11).
The closure review also reproduced malformed Go principal bytes becoming a different
identity during JSON copying (INC-12). Shared pre-copy/hash validation now rejects
them; red/green regressions cover generation, checking, export and nested provenance,
while valid Unicode identities remain byte-exact. No exported signature changed.
The repair passed `task validate` and uncached focused race tests for the prose kit,
scenario and CLI. Read-only `go list -deps -test ./...` checks of all four consumer
snapshots listed below (vendor mode for the two delivered consumers, readonly for the two
isolated checks) show no import of the changed prose package; their pins are unchanged.
Fathom verifies all 13 implementation rows; operational GW-14 is a linked delivery
record, not a substituted symbol proof. The final [intent-doctor](2026-09-13-intent-health.md)
checks structural wiring separately from the content audit and executed gates.

| Consumer snapshot | Result against the new kit |
|---|---|
| Throughline `a007075`, pinned/vendored kit `188c53c` | Actual default-vendor full gate and focused race tests pass; structured findings and SQLite encrypted records use the kit. Other vendor dependencies are unchanged. |
| resume_as_code `7a6c56a`, pinned/vendored kit `8b25230` | Actual default-vendor `test:all`, `go:lint`, focused race and receipt CUE pass; frozen-pin vendor freshness passes. First-class effort/schema and request-scoped family replace duplicated glue; the independent lesson receiver records adoption and refuses late drift/revocation. |
| Fathom `68a5348`, kit `8b25230` | Build passes; full tests fail only `TestNarrowProfileSchemaBytes` (34,821 bytes vs upper bound 34,000). Same failure/value with baseline kit `3db571b`. |
| growing-gardens-bi `7503dd1`, kit `8b25230` | Build passes; 14 finance tests require ignored `tmp/financials.json`. Same missing-fixture failures with baseline kit `3db571b`; other reached packages pass. No private financial data was copied. |

Fathom and growing-gardens-bi were checked in disposable clean clones, not modified.
Their failures are pre-existing dispositions, not compatibility passes. No gate was
relaxed; no new dependency, public release, tag or deployment was made.

Throughline is delivered on actual branch `feat/governed-kit-delivery`;
resume_as_code on `kit-governed-adoption`. Their main branches are unchanged.
Consumer evidence lives in Throughline's `docs/upgrades/governed-kit-delivery.md` and
resume_as_code's `cmd/llm/testdata/lesson-transfer/README.md` / ADR-021. The latter's
receipt binds the actual adapter, test and receiver policy bytes. In agentic-kit and
resume_as_code, the seeded false-pass regression fails when only the classification check is removed
in a disposable snapshot; valid evidence cannot conceal a wrong outcome.

## Native observation and cost

Two synthetic Claude invocations reported $0.0461686 and $0.014327: $0.0604956 total
against the owner's $10 ceiling. Native startup/resume and Read/Edit/Write/Bash/Stop
events were observed, including explicit denied hooks and unchanged private sentinels.
The workspace was untrusted: a public Edit was denied by native permissions despite
an allowing hook. No trust bypass or global configuration change was used.

Codex receipts currently establish configuration/runtime only. A version/help probe
does not prove live tool enforcement. Hosted tools, existing-session stdin and general
hook-failure-closed behavior cannot be promoted by an imported evidence assertion.
The four actual consumer installs are deliberately configuration-only/advisory; strict
path policy would deny arbitrary shell and is an explicit workload choice.

These are agent-assisted setup and synthetic control observations, not measured human
usability, semantic accuracy, production reliability or reduced operating cost.
The fresh synthetic consumer reached the offline scenario in 105 seconds and both
configuration/runtime receipts in 142 seconds with warm tooling. Sanitized native
events, exact costs, sentinel hashes and scoped receipts are [committed with an offline
verifier](../evidence/native-hosts/2026-09-13/README.md). Original private installation
plans remain local; the pinned guard binary must stay in place until uninstalled.

## Fathom judgment after implementation

Useful, with limits. Indexed definitions, cross-consumer references, obligations and
static plan reachability shortened navigation and made omissions visible. They did not
catch mismatched actual JSON/CUE shapes, native path bypasses, UTF-8 identity
normalization or stale vendored copies; executed boundary tests and independent
consumer gates did.

Priority improvements: make operational timeouts distinct from failed proof; suppress
stale contradictory obligations (a served legacy bypass instruction conflicted with
the current no-bypass rule); deduplicate structured/text results; explain result caps
and incomplete cross-service coverage; show runtime test receipts separately from
symbol reachability. Its own schema-size failure also offers a concrete regression
case for measured tool-surface cost. No quantitative token/time saving is claimed.
This closure reused prior consumer/live receipts and two narrowly tasked reviewers;
no additional paid verification calls were needed. A package-name reference query
required `regex: true` despite the advertised string-query contract: avoidable friction
worth fixing in Fathom's tool description/validation agreement.

## Next value tests, not more infrastructure

Use a real attended prose workload to compare accepted supported claims, false blocks
and review effort against its existing process. Expand native coverage only with
normal trust and bounded live observations. Reuse the first lesson in a genuinely new
failure before claiming reduced recurrence. Production persistence adapters should be
pulled by an actual transactional store, not prebuilt as a generic framework.

Compatibility stays explicit: throughline's experimental guardrails risk is accepted
under [0010](../intent/decisions/0010-pinned-experimental-adoption.md); public release,
1.0 and corpus-provenance ratification remain separate owner decisions.
