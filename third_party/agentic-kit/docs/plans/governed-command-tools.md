# Governed command tools: delivery and qualification

Request (Geoff, 2026-10-23): "over time we replace more and more of the risky commands with versions that make the commands much more safe when possible"

Authority: [0039](../intent/decisions/0039-governed-command-tools-preserve-evidence-and-authority.md).
Full contract: [GT-C1..7](../designs/governed-command-tools.md), extending [CG](command-governance.md).
The existing W1 slot admits the first controlled inspection unit, not installation.
Deferred rows preserve the full requested end state. They are not fulfilled by the
first increment and must be admitted/qualified before the overall goal can close.
Owner: kit maintainer. Budget/review: 0039. No paid inference or third slot.

- [x] GT-1: whole-selected-input scan before rooted exact-content read/search [GT-C2] {id: GT1, symbol: run, package: cmd/command-guard}
- [x] GT-2: exact independent required controls and strict bounded input [GT-C2] {id: GT2, symbol: validate, package: cmd/command-guard, needs: GT1}
- [x] GT-3: CLI disclosure/error/limit/privacy paths and useful guide [GT-C2] {id: GT3, symbol: execute, package: cmd/command-guard, needs: GT1}
- [-] GT-4: native Windows/macOS/Linux path and resource qualification [GT-C7] {id: GT4, disposition: deferred, reason: "fixed file profiles have native witnesses; other native surfaces remain unqualified", trigger: "qualify remaining Git, special-file, cancellation, scanner and host-control profiles with retained failures"}
- [x] GT-4a: fixed-roster native file/scanner qualification driver [NQ-1..5] {id: GT4A, symbol: execute, package: cmd/command-qualify, needs: GT1}
- [~] GT-5: pinned scanner adapters and complete declared disclosure coverage [GT-C3] {id: GT5, symbol: runWithScanner, package: cmd/command-guard, needs: GT5A}
- [x] GT-5a: fixed Gitleaks stdin text profile and negative qualification [GT-C3a] {id: GT5A, symbol: scanSelectedWithRunner, package: cmd/command-guard, needs: GT1}
- [x] GT-6: controlled Git status/object diff/history recipes [GT-G1..5] {id: GT6, symbol: runGit, package: cmd/command-guard, needs: GT1}
- [-] GT-7: bounded isolated build/test/check recipes [GT-C1] {id: GT7, disposition: deferred, reason: "candidate code execution needs its host boundary", trigger: "receiver supplies trusted recipes and isolation/cancellation proof"}
- [x] GT-8: authenticated layered config and explain/status [GT-C5, HC-1..4] {id: GT8, symbol: check, package: cmd/harnessctl, needs: GT1}
- [-] GT-9: native presence and exact maintenance grants [GT-C5] {id: GT9, disposition: deferred, reason: "macOS SDK source is built but live protected-key presence is unqualified; other host profiles are unimplemented", trigger: "issuer binds qualified native auth to exact finite grant on selected host"}
- [~] GT-9a: protected receiver transaction and native signing seam [0043/HMT; evidence: docs/evidence/command-tools/2026-10-24-maintenance/README.md; native consent/interception remain unqualified] {id: GT9A, symbol: maintenanceTransition, package: cmd/harnessctl, needs: GT8}
- [-] GT-10: durable arm/lockdown/maintenance/rearm/recovery state [GT-C5] {id: GT10, disposition: deferred, reason: "selected synthetic receiver exists under GT9A; installed hook authority and host qualification remain absent", trigger: "GT8 and GT9 qualify actual dispatcher per-boundary expiry/revocation and crash probes"}
- [-] GT-11: agent-hooks/cavecrew economic integration and sampling [GT-C5] {id: GT11, disposition: deferred, reason: "GT11A qualifies the strict peer/current receiver; installed migration and actual host interception remain absent", trigger: "admit compatibility migration and actual host interception tests"}
- [x] GT-11a: strict spawn evaluation and controlled advice sampling [0044/HSE; native interception remains separate] {id: GT11A, symbol: runHooksWithDeps, package: cmd/harnessctl, needs: GT8}
- [-] GT-12: local and stronger decision adapters with escalation [GT-C3] {id: GT12, disposition: deferred, reason: "no live classifier or residency/calibration qualification", trigger: "criterion owner declares eligible endpoints and cohort/budget bars"}
- [~] GT-12a: concern site implemented; hosted native observations pending [MC-0..4; evidence: docs/evidence/command-tools/2026-10-24-classifier/README.md] {id: GT12A, symbol: classifySelectedContext, package: cmd/command-guard, needs: GT1}
- [~] GT-13: faithful compact views and exact observation cache [GT-C4] {id: GT13, symbol: encode, package: cmd/command-guard, needs: GT13A}
- [x] GT-13a: lossless tuple views and scoped byte/task baseline [CV-1..5] {id: GT13A, symbol: compactInspect, package: cmd/command-guard, needs: GT1}
- [x] GT-13b: exact invocation-local scanner observations with truthful process/reuse coverage [OR-1..6] {id: GT13B, symbol: scanSelectedWithContextDeps, package: cmd/command-guard, needs: GT5A}
- [-] GT-14: optional revision-bound Fathom enrichment [GT-C3] {id: GT14, disposition: deferred, reason: "inspection has no evidence service dependency", trigger: "consumer needs source facts and proves unavailable/stale engine behavior"}
- [-] GT-15: privacy-bounded journal/flywheel and harness catalog [GT-C6] {id: GT15, disposition: deferred, reason: "no passive history collection or auto policy promotion", trigger: "independent goals/scope retention owner and sanitized lesson corpus are supplied"}
- [-] GT-16: exact patch/quarantine/send effects and recovery [GT-C1] {id: GT16, disposition: deferred, reason: "no write/network effect adapter admitted", trigger: "named destination defines authority preview replay and reconciliation"}
- [-] GT-17: controlled rollout and measured task economics [GT-C7] {id: GT17, disposition: deferred, reason: "synthetic probes do not prove production interception or savings", trigger: "host owner admits installation and predeclares task baseline and acceptance bars"}

GT1..3 completion needs focused adversarial tests, guide-only binary trial, native
host observations, CUE vet, `task validate`, independent review and strict plan proof.
Native observations qualify only their actual host. Cross-builds do not close GT4.
GT4A's [NQ evidence](../evidence/command-tools/2026-10-24-native/README.md) includes
18-case native core observations on hosted Linux/amd64, Windows/amd64 and
macOS/arm64, plus the 27-case scanner profile on the owned Mac. Dual-CUE actual
reports, race/vet, retained false-oracle/schema failures, independent reviews and
strict isolated proof qualify this driver. It does not qualify other GT4 surfaces.
No GT row grants permission to install or disarm this machine's hooks.
GT13a's [CV profile](../designs/command-compact.md) qualified lossless selected-row
layouts: full gate, 34 native cases, 66 actual report/CUE checks, ten negative shapes,
independent security/guide reviews and strict isolated proof. [Scoped evidence](../evidence/command-tools/2026-10-24-compact/README.md)
records synthetic byte savings and added encoding work. Exact observation caching and
real task economics remain GT13/17. INC-21 repairs file-inspection failure output caps.
GT8's [0041/HC contract](../designs/harness-control.md) qualifies a protected central
registry, authenticated snapshots and bounded advisory preference composition: full
gate, 47 native Mac cases/report-CUE checks, independent reviews, helper-source race/vet
and strict isolated-store proof. [Evidence](../evidence/command-tools/2026-10-24-harness/README.md)
retains exact identities and repaired failures. Native approval, durable leases and
hook interception remain GT9..11; selected-source authentication is not live revocation.
GT6's [0040 contract](../designs/command-git.md) qualifies the three fixed recipes
within the protected-registration profile: full gate, 39 native Mac cases, actual
report/CUE checks, independent security/guide trials and strict isolated-store proof.
[Scoped evidence](../evidence/command-tools/2026-10-24-git/README.md) retains source
identities, repaired failures and support limits. GT4 still owns other native hosts;
GT6 does not qualify arbitrary metadata, Git commands or coherent live status.
GT5a is the first versioned scanner profile, not closure of GT5's broader declared
scanner/PII/format coverage or GT4's native platform matrix. Unsupported or skipped
input holds rather than inheriting this text profile's qualification.
GT5a reopened for OR-A1 and requalified with explicit process-start witnesses plus
red/green tests. Production Start errors were already explicit; INC-24 is not a
demonstrated source-controlled bypass. GT13b's [OR qualification](../evidence/command-tools/2026-10-24-reuse/README.md)
includes bounded immutable copies, exact logical/physical counters, linear mutation
witnesses, dual-CUE real reports, native baseline/paired trials, independent reviews
and guide-only use. First native qualification failed an ambiguous default CUE
branch; corrected absent/present coverage and default regressions pass both versions.
Full validation and fresh isolated strict proof cover the eight claimed command rows.
Broader GT13 cache, GT4 native hosts and GT17 accepted-task economics remain open.
GT5a passed full validation, independent security/guide review, 14 native Mac cases
and strict isolated-store proof. [Evidence](../evidence/command-tools/2026-10-24/README.md)
binds the source, actual binary, pinned scanner and remaining qualification limits.
GT1..3 passed the full gate, independent security/guide trials and strict isolated-store
proof before being claimed. [Scoped evidence](../evidence/command-tools/2026-10-23/README.md)
retains code identities, native entrypoint results and the remaining limits.
GT11A's [0044/HSE](../designs/harness-spawn-economics.md) passes ordinary and combined
full gates, independent source/guide review, actual pinned process failures, 63 owned
Mac CLI cases, 198 dual-CUE checks and strict matching-build plan proof. Required
denials persist at advice rates0/partial/full and under synthetic maintenance.
INC-25 repairs failed preparation interfaces. [Evidence](../evidence/command-tools/2026-10-24-spawn/README.md)
separates measured process latency from installed interception, live consent and
production economics. GT9..11/15/17 and the full goal remain open.

Before claiming a row, ground its current symbol/test paths and run:

```sh
fathom verify --service agentic-kit --plan docs/plans/governed-command-tools.md --strict
```

An unavailable daemon is operational unknown, not proof. The normal pre-push gate
has a separate isolated-store check; retain the actual receipt and scope.

## Full-objective reconciliation

Portable risky-command replacements: GT1/4/6/7/16. Deterministic secrets/PII/scanners:
GT2/5. Optional local/advanced judgment and confidence/explanation: GT12. RTK-inspired
output and performance: GT3/13/17. Optional Fathom: GT14. Global/project/organization/
team/personal policy: GT8. Arm/disarm, native authentication and recovery: GT9/10.
Tokenomics/cavecrew compatibility and hook rates: GT11. Journal/flywheel rates and
reviewed improvement: GT15. Guidance/skills and research accompany each applicable
unit. Missing any of these keeps the full goal incomplete.
