# Governance that learns: every client's rules, every client's hooks, measured rules, dated facts, reviewer output

Status: accepted 2026-09-27 (ADR-049 decision 9, ADR-051 decision 5 for U4). Vision:
`docs/vision/38-proof-layer-roadmap.md`. Plan: `docs/plans/governance-learning.md` (GL-).
Goals G16 (rules formats and hooks) and G21. Constraints carried in: ADR-016 and I27
(compiled claims cite and scope), ADR-029 and vision 25 (the harness port; a harness's inject
tier ships unverified until a live session verifies it), ADR-037 and ADR-038 (governance
bites at the edit), ADR-040 (findings loop authority), SU-19 / ADR-041 decision 7 (validity
windows), PC-21 (the SARIF emitter), law 15, invariant I9 (hooks fail open).

## 1. Problem

Fathom serves the rule that governs a file, with its citation, at the edit, and can block a
violating edit. Four gaps limit it:

1. It reads rules only from `CLAUDE.md`, `AGENTS.md` and its own corpus; teams write rules in
   `.cursor/rules`, Copilot instructions, Kiro steering, `BUGBOT.md` and `GEMINI.md`.
2. It serves them through Claude Code and Codex; Cursor and Copilot are descriptor rows,
   Gemini CLI is unverified.
3. Rules never learn. A rule ignored a hundred times is served a hundred and first time. The
   signals to learn from are empty today: 0 memories, 0 human claim verdicts, 0 live
   certificate declarations on the reference machine.
4. Findings reach reviewers only as gate SARIF. Review bots guess at impact that fathom
   computed.

## 2. Decisions

1. Rules files of every major client are sources, with `off`, `warn`, `error` levels.
2. One rule engine, one hook adapter per client on the harness port.
3. A signal source first (hook outcomes as events), then rule effectiveness (U4), then
   promotion and retirement under published thresholds, announced, never silent.
4. Validity windows are activated and served everywhere a fact ends; `as_of` reaches every
   verb that reads claims, memories, learned-rule states or doc links.
5. SARIF for obligations, review findings and contract drift, and a check-run payload,
   emitted by verbs; layered diffs by blast radius in `review_diff`.

## 3. Rules readers

| Client | Files | Shape | Maps to |
|---|---|---|---|
| Claude Code | `CLAUDE.md`, nested `CLAUDE.md` | prose | exists |
| Codex, many | `AGENTS.md`, nested | prose | exists |
| Cursor | `.cursor/rules/*.mdc` | frontmatter `description`, `globs`, `alwaysApply`, body | a rule per file; `globs` become the claim scope; `alwaysApply` is repo scope |
| Copilot | `.github/copilot-instructions.md`, `.github/instructions/*.instructions.md` (`applyTo:` frontmatter) | prose, glob-scoped | `applyTo` becomes the scope |
| Kiro | `.kiro/steering/*.md` (`inclusion: always|fileMatch|manual`, `fileMatchPattern`) | prose | `fileMatch` becomes the scope; `manual` is compiled but not served unasked |
| Cursor Bugbot | `BUGBOT.md`, nested | review rules | scoped to its directory; served to review verbs, not the edit hook by default |
| Gemini CLI | `GEMINI.md`, nested | prose | as `AGENTS.md` |

Each reader is a `RulesSource` (`Detect`, `Read`) feeding the existing compile path, so every
compiled rule still passes I27 (citation inside its section, scope matches indexed files, not
over-broad). A source carries a **level**: `.fathom/config.toml` `[rules.sources]` maps each
source id to `off`, `warn` (served with a note) or `error` (eligible for the PreToolUse deny,
ADR-038). Default: the client's own files at `warn`, `CLAUDE.md`/`AGENTS.md` unchanged.
Fathom never writes to these files.

## 4. Hooks for every client

The rule engine is one; each client is a codec on the harness port (`internal/harness`):

| Client | Status today | Work |
|---|---|---|
| Claude Code | verified | none |
| Codex | verified (user-global hooks printed, not written) | none |
| Gemini CLI | ships `unverified` | a live session's beats verify it (ADR-029's rule) |
| Cursor | descriptor row; `.cursor/hooks.json` flat shape not addressed by the json-hook editor | a codec for the flat snake_case shape; `fathom harness hooks --harness cursor` prints entries; a live session verifies |
| Copilot | descriptor row | a codec for its hook file; a live session verifies |

A client counts toward G16's "hooks live in at least 3 clients" only when its descriptor
reads `verified` from a real session's beats (GL-M4), never from a codec existing.

## 5. The signal source and rule effectiveness (U4)

Every hook decision already passes through the edit and PreToolUse hooks. GL-10 records, per
served rule and per event, one of: `served`, `complied` (the edit landed and the rule's
checkable condition holds after it), `violated` (the condition fails after the edit),
`blocked` (the deny fired), plus the existing `refuted`/`corroborated` claim verdicts. Rows go
to a bounded, rotated JSONL under `~/.fathom/` (the call-log precedent), never the transcript.

**Correction 2026-09-29 (ADR-056, `docs/plans/judgment-foundations.md` JF-1).** As implemented
(`cmd/fathom/ruleoutcome_track.go`, SR-61), `violated` means the SAME claim topped the file's served
set on the file's previous resolved turn, and `complied` means the top claim changed. Rules are
served by glob scope, so for a prose rule with no checkable condition these two events measure scope
persistence and churn, not compliance: a repeat edit of the same file reads `violated` whether or not
the edit complied. Every GL-10 reading built on them (the classes below, GL-11's counts) is read as
"still served" and "top changed" until the events are renamed `still_served` and `top_changed` and
`violated` is kept only for a rule with a checkable condition. Compliance for prose rules is not
measured; judged compliance is refused (ADR-056, `docs/design/judged-telemetry.md` section 5).

**Rule effectiveness** is a projection: per rule, counts and rates over a window, with n.
Classes, each with a computed remedy shown as a proposal:

| Class | Condition (30-day window, n at least 20 served) | Proposed remedy |
|---|---|---|
| ignored | violated / served at least 0.5 and never blocked | move to a hook (deny) or rewrite with the violating example |
| never triggered | served 0 times in 90 days | prune or re-scope |
| effective | complied / served at least 0.9 | keep |
| contested | refuted at least 3 times | human review |

## 6. Rules that learn

A candidate rule (a compiled claim at `candidate` status, or a finding-derived rule) is
**promoted** to `active` when corroborations reach at least 3 from at least 2 distinct
sessions and refutes are 0 over 30 days. An active rule is **retired** (validity window
closed, status `retired`, not deleted) when refutes reach at least 3 with refutes greater than
corroborations, or when it is `never triggered` for 180 days. The thresholds live in
`.fathom/learning.cue` (repository-authored, published, reviewable); the defaults above ship
in the binary. Every promotion or retirement is announced in the next session's attention line
and in `fathom intent audit`, with the signals that moved it. Nothing learns until GL-10's
signal source has 30 days of data; GL-M2 is not read before that.

### 6.1 Owner decisions (GL-13/GL-14, attempt 2, worker w-gl-w2-rest)

Two implementation questions §6 left open were decided so GL-13/GL-14 could
proceed:

1. **Lifecycle is an additive, orthogonal axis, never an overload of
   Status.** `candidate` | `active` | `retired`, default `active`, lives
   on `core.SemanticClaim.Lifecycle` (store column `semantic_claims.
   lifecycle`, same additive-migration convention as every other claim
   column since SU-19). It is judged INDEPENDENTLY of ClaimStatus
   (fresh/stale/contradicted/superseded/refuted) — a claim can be
   `ClaimFresh` and `LifecycleCandidate` at once. `""` (a claim written
   before this field existed) reads as already active
   (`ClaimLifecycle.EffectiveLifecycle()`), never as unknown or candidate
   — no compile path mints a claim at `candidate` yet (v1: every compiled
   claim is minted active, exactly as before this axis existed); the state
   exists for a future producer (a finding-derived rule) and for
   `applyLearning` (internal/semantic) to promote/retire out of.
   Retirement is terminal and durable: a recompile that re-emits the same
   claim_id with the compiler's default (unset) lifecycle must not
   silently resurrect a retired rule — `PutClaims`' `ON CONFLICT` guards
   `lifecycle` exactly like it already guards `status` for `refuted`.

2. **Corroborate/refute identity.** `fathom claim corroborate`/`refute`
   record the verdict's identity as the harness session id when the hook
   env carries one (the same registry-driven resolution `fathom rate`
   uses), else the literal `human:cli` — never `""`, since §6's promotion
   rule needs a distinct, attributable identity per corroboration to count
   "from at least 2 distinct sessions". The identity lands in both
   `ClaimEvent.AssertedBy` (`claim-add:<identity>`, the existing "kind:id"
   convention) and GL-10's rule-outcome signal row (its `session` field) —
   one identity, two readers, never a second channel.

## 7. Validity windows everywhere a fact ends (N8.2)

`semantic_claims` already has `valid_from`/`valid_to` (SU-19) and `obligations` answers
`as_of`. The work is activation and serving:

- every path that ends a claim sets `valid_to` (compile reconciliation, sweep, refute,
  retirement), audited by a test that no claim-ending path deletes;
- memories, rules-file rules, learned-rule states and doc links gain the same two columns,
  additively;
- `as_of` is accepted by `memory_search`, `staleness_report`, `obligations` (exists) and the
  review verbs, answering "what did we know on date D".

## 8. Reviewer output

- **SARIF.** The PC-21 emitter (`verify.RenderSARIF`) is reused with new rule vocabularies:
  obligations (a governing rule touched by a diff, at its level), review findings, and
  contract drift (`docs/design/system-graph.md` §7). One run per verb, deterministic order.
- **Check payload.** A verb writes a GitHub check-run JSON body (`name`, `conclusion`,
  `output.summary`, `output.annotations[]`, capped at 50 annotations per call as the API
  allows) to stdout or a file. Fathom does not post it; the user's CI does (the F14 repository
  and any forge app stay futures).
- **Layered diffs.** `review_diff` groups changed files into layers by blast radius (leaf
  changes, then shared callees, then cross-service providers), ordered for review, with no
  model.

## 9. Tenancy and cost

Rules readers run at compile (not at request). The signal log is append-only, rotated at
16 MiB, read by the projection in a bounded window. Validity columns are additive. SARIF and
check payloads are rendered from verb output already computed.

## 10. Measurement

| Reading | Threshold |
|---|---|
| GL-M1 | rules read from at least 6 client formats on fixtures, each rule citing its source line |
| GL-M2 | after 30 days of signal: promotions and retirements under the published thresholds, fewer than 1 in 10 promotions refuted within 30 days |
| GL-M3 | `as_of` answers for claims, memories and learned-rule states on a seeded history |
| GL-M4 | hooks verified live in at least 3 clients (descriptor state `verified` from real beats) |
| GL-M5 | SARIF or the check payload consumed by one external reviewer (a code-scanning upload or a review bot reading it) |

## 11. Jev, weighed

- **Where it could help:** choosing among candidate rewrites of an ignored rule that code or an
  llm produced. Jev cannot generate text; it can only choose among candidates it is handed
  (corrected 2026-09-29, ADR-056; the earlier wording, "proposing a rewrite", assumed generation).
- **Where it cannot:** promotion, retirement, levels or compliance. Those are counts against
  published thresholds, and judged compliance is refused (ADR-049 decision 7, ADR-056).
- **Cost:** a call per ignored rule per review, bounded by `[decide]`.
- **The measurement:** a blind review of 20 candidate rewrites against the originals; keep only
  if at least 14 are preferred. Not before GL-M2.

## 12. Refused

- Silent learning: every state change is announced with its signals.
- Writing to any client's rules file.
- Counting a hook codec as "live" without a verified session.
- Posting to a forge or running a review service.
- Auto-applying a remedy (ADR-051).
- Learning from prose inference over transcripts (ADR-041 decision 8's findings learning rule stands).
