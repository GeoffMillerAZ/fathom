# Fathom configuration for app-kit

This guide owns repo-local agent-configuration maintenance. Root [intent](../intent/README.md)
and [scope](../intent/scope.md) remain authoritative; Fathom supplies evidence, not UI taste.

## Feature selection

| Feature | Selection and purpose | Evidence boundary |
|---|---|---|
| Symbol/call-graph discovery | Use for where/who-calls/impact; literal config/doc comparisons use exact search | Fresh source and call edges are not runtime success |
| Guards | Six narrow kit-self groups from corrected shipped v0.2 pack | Advisory pre-edit guidance, not denial or policy enforcement |
| Config custodian | Four falsifiable checks for retired guidance, real task command, pointers and served tools | A clean audit is configuration evidence, not host loading |
| Semantic layer | Stage ui-appkit v2 after policy correction, with current living corpus and bounded serving | Not evaluated or compiled; no fabricated precision/recall pass |
| Module bindings | Explicit empty selection until an applicable mounted, evaluated module is chosen | Installing a layer is not binding a module |
| Plan gates | Legacy claims repaired; CLI implementation rows pin specific tests | Strict verification proves claimed implementation, not broad outcome certification |
| Continuity and feedback | Point to current files/tests and scoped failures; use existing local instruments | No extra telemetry, model story generation or global settings introduced |
| Dialects | Reuse existing Go/CUE/JS/Markdown extraction; investigate a concrete templ gap separately | Dialects are upstream extractor code, not installable semantic packs |
| Architecture rules/review | Use when exact dependency edges and a planted violation prove the rule | Do not install an unverified glob rule and call the boundary enforced |
| Answer budgets | Explicit bounded queries first; tune shared budgets only after measuring useful recall | A smaller response that omits needed evidence is not an optimization |

Do not install every pack. The agentic-kit runtime pack includes different kit-specific
laws; following its intent practice does not adopt those laws into app-kit. Provider
packs and speculative per-technology layers are not a new backlog.

## Reproduce the repo-local checks

```sh
fathom doctor --harness codex --service app-kit
fathom doctor --harness claude --service app-kit
fathom config audit --repo .
fathom plans status --service app-kit
go test ./cmd/appkit-mcp -run '^TestFathom' -count=1
```

Use `fathom fleet reconcile --repo .` dry-run before the repo-scoped `--apply`.
Reconcile refreshes only owned regions/skills and safe hook additions, with a reversible
receipt. Read the actual write scope; do not run unscoped fleet changes or restart the
shared daemon. Current activation receipt: `7fa69ddc8d69`.

For no-hook vehicles, use `fathom prime` and explicit obligations/guard checks. A manual
hook probe proves that command path only. Actual host-invoked pre/post output is stronger;
neither alone proves every Claude/Codex event or a restarted session loaded its config.

## Semantic-layer admission and economics

[ADR-0072](../adr/0072-stabilize-and-resume-on-demand.md) chooses ordinary source indexing
and the existing MCP `stabilization` document route for [demand-driven continuation](stabilization-and-resumption.md).
Keep mutable readiness in the CUE inventory and execution priority in scope. No new
semantic layer or LLM compilation is part of landing; admit one only after a measured
retrieval gap and a separately bounded evaluation. A thread note is a pointer, not the spec.

The shipped v2 manifest excludes historical ADRs, private assets, assessment claims and
plans; current living docs carry the applicable decisions. The prompt preserves
originals, profile choices, scanner distinctions and target evidence limits.
Structural tests reject known retired instructions and local/shipped guard drift.

Keep `evaled: false` until labeled positive/negative/scope-trap fixtures pass the actual
real-model precision/recall gate with model, prompt and corpus identities recorded.
Do not bypass the eval gate or treat an empty result as success. Installing/staging is
not compiling or serving. Estimate compilation before requesting a bounded spend;
do not rewrite daemon profiles or global configuration to make it run.

Already-served compiled obligations have stale citations. Follow the current source
and explicit guard, retaining the drift warning; new config must not relabel them fresh.
Recompilation/claim triage needs separately reviewed evidence and authority.

## Maintenance loop

When intent changes, review both agent routers, MCP-served docs and the distributed
Fathom pack in the same change. Run config audit and the focused tests; record failures
as incidents/observations with a real reproduction. Update owning goals without
claiming runtime/consumer completion from static symbols. Retire high-noise guards
after observing no useful catches in the pack's recorded window.

## Activation and maintenance evidence (2026-09-13–14)

- Reconcile dry-run now reports one repo, zero drift; both host doctors have zero failures.
  Forty-one owned entries match. Nine shipped skills are current for each host. No
  global host file or running daemon was changed. O9 required a reviewed local baseline
  refresh after the tool recorded pre-refresh skill hashes; the upstream bug remains open.
- Six actual guards validate and have live pre/post-edit output in this session. O11
  records the separate capability-counter bug: valid quoted CUE labels report zero in
  the installed binary. A generic Fathom source repair now shares the semantic CUE parser
  across CLI/config-panel diagnostics and distinguishes invalid/unreadable from absent.
  Positive/negative integration tests and all three affected packages pass with race
  detection; an isolated candidate reports all six guards. It was not installed or deployed.
  Verification used baseline `3909c716` for unrelated concurrent discovery/harness edits;
  receipt: `sextant/workspace/tmp/guard-diagnostic-check.48Q6Gg/receipt.json`.
- Four config checks hold in the repo. An isolated synthetic Git fixture at
  `/tmp/appkit-fathom-config-canary.9rAUGN` rejects all four planted violations (exit 1)
  then holds all four after repair (exit 0). No real agent config was deliberately broken.
- Seven focused Fathom tests, the full Go suite, `go vet ./...`, and focused MCP/CLI/viz
  race checks pass. The scoped intent checker covers 88 docs / 516 relative links /
  33 ADRs / six goals / nine futures; it is not the full intent-doctor or host-loading test.
- Strict plan verification reports 62 rows: 45 claimed and verified, six in progress,
  eleven deferred. Static reachability is not runtime, consumer, release or G1–G6 completion.
- Intent audit recognizes all ten sensitive surfaces and reports no unparsed headings.
  Broader conditions remain human-judged. Historical test-pinning and exported-guard
  provenance warnings remain; curated pack guards are not auto-exported intent guards.
  A sampled index/audit disagreement never justifies weakening a real test claim.

The root-executable exclusion once hid `appkit/`; INC-12 and its source-root canary
now cover that observed regression. Re-ingest and query a known source symbol after
ignore changes. Bounded reference results no longer include the excluded root binary.

Remaining work: real semantic evaluation/serving, further historical evidence mapping,
and only a demonstrated dialect gap worth contributing. This is not a claim that every
optional feature is active or useful. Fathom's current generated stanza has an outdated
blanket Codex SessionStart claim; actual harness capabilities and doctor govern whether
the no-hook `prime` fallback is needed. Do not hand-edit an owned marked region to hide drift.
