---
name: command-safety-audit
description: Audit selected command proposals or harness safety, and qualify command-guard, optional concern classifiers or harnessctl adoption, including native command-qualify profiles. Use for requested command-frequency, policy/hook safety or governed-tool adoption work; not ordinary shell or Git use.
---

# Command safety audit

Read [the guide](../../../docs/command-safety.md) for CLI usage, privacy and exit
codes, and [the design](../../../docs/designs/command-governance.md) when proposing
changes beyond the offline reference. Use `cmd/command-lab` only for explicit files
the requester has authorized. Create an independent expected-source manifest;
never discover private transcripts, install/disarm hooks, invoke a provider or run
the proposed command as part of this skill. Keep raw transcripts and control files
private. Audit output is closed-category counts, not a safety certificate.

For a command-frequency request, run `audit` and report the declared source set,
coverage, fixed failure counts and proposal/candidate distinction. An incomplete
run cannot support a complete-history claim. For a proposed operation, run
`assess` with independently supplied scope, policy and request. Report 0 as
advisory, 1 as blocked and 2 as incomplete; `noActionAuthority: true` always.

For a coding harness review, first identify its documented goals and actual
policy owners. Compare instructions, tool grants, hooks and failure behavior with
observed invocation and negative cases. Separate configuration, observed
enforcement, adoption and benefit. Record exact findings and small patch proposals
against the owning files. Do not write fixes, change policy floors or tune hooks
unless that separate edit is authorized. Test the controlled path before claiming
enforcement. If a hook fails, repair it through its owner; never bypass its gate.

For controlled file-inspection adoption, read the [command-guard guide](../../../cmd/command-guard/README.md)
and [delivery contract](../../../docs/designs/governed-command-tools.md). Run its
synthetic fixtures first. Use explicit authorized file selection, exact content hashes
and independently supplied required rules/layers. Protect controls outside candidate
write authority before a real controlled disclosure use. Do not silently discover private
files, install hooks, waive a finding or substitute a model verdict. Explain the exact
configured scan scope and omitted rows. Exit 0 means no configured findings in selected
bytes, not absence of secrets/PII, authorization or platform qualification.

For the optional Gitleaks profile, require both independent scanner scope and protected
policy with exact executable/version/hash. Use the guide's fixed profile; no implicit
download, installation or custom scanner arguments. Check attempted, actually invoked,
completed and observed-byte receipts separately. Findings block, while skipped bytes,
unknown log/report shapes, timeout and unavailable prerequisites remain incomplete.
The scanner's embedded default rules do not establish PII coverage. Keep raw scanner
reports private and never treat a model or a clean scanner exit as an override.

For requested contextual classifier adoption, read the
[classifier guide](../../../docs/command-classifiers.md) and
[MC contract](../../../docs/designs/command-concern-classifier.md). Start with
`files-loopback-fixture/1`, which performs no inference. Select trusted
`-tags command_classifier` guard bytes; ordinary builds hold selected classifier
controls as unsupported. Help identifies the compiled capability. Require independently
selected public/synthetic input pins and an exact final policy-file pin in protected
scope. Protect both control files outside candidate write authority. The selected
site runs after mandatory checks and may add a hold only. Unknown, malformed,
refused, late or failed answers withhold all source rows. Check context binding,
attempt/completion/byte counts and the synthetic-peer marker without publishing raw
answers, source hashes or endpoint/model strings. A loopback hop and model assertion
do not qualify residency, confidentiality, accuracy, confidence or billing. Do not
call an installed model, discover private inputs, install a runtime or enable hooks
as incidental qualification. Real runtime/private-input promotion requires a
separately admitted deployment and measured cohort, egress/logging and resource bars.
Selected classifier controls on Git reads are unsupported and must hold before
Git/model execution. A favorable model answer cannot waive a deterministic floor.

For requested native qualification, follow the
[command-qualify guide](../../../cmd/command-qualify/README.md) and
[NQ scope](../../../docs/designs/command-native-qualification.md). Run the fixed
profile against explicitly trusted pinned binaries and owned synthetic fixtures.
The driver executes those binaries with ordinary host authority; it installs nothing.
Record source revision, runner/executable pins, actual OS/architecture, profile and
ordered case results. Retain failures. Missing capabilities or unrun cases stay unknown
and cannot be skipped to obtain a pass. A pass covers that roster on that host only.
Cross-builds, CUE shape checks and authored instructions do not prove native execution,
other host profiles, general sensitive-data absence or installed enforcement.

For signed harness-configuration adoption, read the
[harnessctl guide](../../../cmd/harnessctl/README.md) and
[HC contract](../../../docs/designs/harness-control.md). Qualify with fresh toy
registration and independent negative cases first. Protect the registration, enrolled
key roster and dispatcher outside candidate write authority; source filenames/hints
cannot establish an issuer. Inspect fixed codes and expected/loaded/authenticated
counts without dumping source bodies. Configuration success does not authenticate
the current actor, arm a hook, issue a maintenance grant or prove rollback-safe dispatch.
Do not enroll keys, read private keys, override clocks or install/disarm as part of
this diagnostic. Numeric failure ordinals are operator metadata, not public telemetry.

For requested maintenance-receiver qualification, read the
[maintenance guide](../../../cmd/harnessctl/maintenance.md) and
[HMT contract](../../../docs/designs/harness-maintenance.md). Select explicit
`harness_maintenance` build bytes and qualify synthetic fixtures first. The protected
operator-selected controller/pin, enrolled native approver and immutable receiver
BINDING are separate from HC8 source-policy keys. Recovery cannot replace the issuer
or reset nonce history. Require a protected preparation namespace, finite exact
advisory delta and current mandatory floors. `stamp`, `plan-check`, SDK compilation
and DEMO preview cannot qualify native consent. Do not trigger real `approve`, enroll
keys or install/disarm hooks as incidental testing. Ambiguous state/output requires
inspection, not replay; recovery creates locked state only, and rearm requires fresh
native authority. Report stored-signature verification separately from this command's
native process/signature counters. Live authentication and installed dispatch remain
separate qualification, even when synthetic lifecycle tests pass.

For requested governed spawn-check qualification, read the
[HSE guide](../../../docs/harness-spawn-economics.md) and its linked contract.
Use the strict `agent-hooks evaluate-spawn` peer and explicit maintenance-build
`harnessctl hooks evaluate` against synthetic protected fixtures first. Keep
model/effort/tier/fork checks at full selection; sample only optional advice
observations. Legacy `agent-hooks hook` retains fallback/denial-loop behavior and
cannot substitute for this peer. Require exact selected policy/evaluator pins,
current HC8/HMT authority, bounded native input and post-child revalidation.
Report fixed outcomes and actual process/write counters without dumping prompts,
model names, IDs, hashes or effective values. Native `hooks run` is deny or silence,
does no advice observation work and grants no action authority. Test actual host
delivery/failure/denial before adoption; do not install hooks, alter global policy,
enroll keys or request authentication as incidental qualification.

For registered Git-read adoption, use the guide's `git-inspect` controls and
qualify optional `--format compact` with the [CV grammar](../../../docs/designs/command-compact.md).
Decode by returned version/layout, preserve all scanner/omission/freshness fields and
compare independent task answers against ordinary JSON. Never equate fewer bytes with
tokens, cost or better reviews; no cache or disclosure bypass is provided. Then use the
[GT6 contract](../../../docs/designs/command-git.md). Register protected canonical
admin/common metadata and configuration before any repository process; do not use
candidate Git discovery to establish that trust. Qualify with owned synthetic
repositories first. A success discloses selected source data, including selectors
and exact commit identities. A hold discloses none. Read omission and older-history
states; status is non-atomic and cannot prove clean or absent. A tiny valid output
budget can yield exit 2 with no report bytes. Keep unsupported metadata/configuration
and missing objects incomplete, never fetch or relax controls to obtain a pass.
