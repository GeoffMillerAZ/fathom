# Incidents

Every incident yields an invariant, a guard, a lint check, a test, or a decision. Open
records are ≤ 10 lines; closed ones compress to one line. The kit's generic anti-patterns
are the ledger in `docs/practices.md`; only specific, dated failures go here.

Classes: gate-bypassed · wrong-done-claim · intent-violated · platform-fact ·
contract-broken · other. Record with `intent-incident`.

## Open

### INC-6 2026-08-25 `[Unreleased]` carried 22 entries across a tag
What: `v0.7.0` was tagged 2026-07-16 without rolling `[Unreleased]`; entries grew to
500 words each because rationale and evidence had no other home.
Evidence: `CHANGELOG.md` before 64db8a4; `git log -S` attribution per entry.
Would have caught it: a release ritual that moves the block before the tag; a budget
per line.
Class: gate-bypassed
Outcome: rolled retroactively (64db8a4); ritual in `project-intent/changes.md` §6;
`intent-release` skill. Pending: the 12 remaining long entries stay by rule (§9) until
the next tag, when `intent-prune` collapses them.

## Closed

- INC-1 (pre-2026-07) A hook channel died silently for four days while its metric read
  zero → `heartbeat.CheckExistence`/`ExistenceExpectation`/`ExistenceAlarm`; the
  telemetry guard in `fathom-pack/guard-pack.cue` ("every measurement channel has an
  existence check"); I19. Class: platform-fact.
- INC-2 2026-07-16 `bedrock/awscred` let explicit credentials fall through to ambient
  `~/.aws` state and partial pairs sign as the default profile → typed
  `*PartialCredentialsError`, explicit pairs short-circuit, `credential_process` timeout
  and stderr cap ("hardened same-day after critique"). Class: contract-broken.
- INC-3 2026-07 `docs/first-agent.md` claimed `turns=2`; the CLI produced 3 →
  `TestFirstAgentWalkthrough` golden-pins the doc's mock-mode output. Class:
  wrong-done-claim.
- INC-4 2026-07 `WithCost` emitted zero-cost events silently; `MockClient` returned
  zero usage → conformance suite catches both; mock defaults to 1 token. Class:
  contract-broken.
- INC-5 2026-07 H8 Stage-A security self-review found the docs overclaiming a
  `CallTool` pre-call pin check → docs corrected in the same change; TRACKING row H8-1
  governs the gap (`questions.md` Q3). Class: wrong-done-claim.
