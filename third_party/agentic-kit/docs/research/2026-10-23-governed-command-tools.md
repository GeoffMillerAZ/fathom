# Governed command tools: research and decisions

Checked 2026-10-23. Primary-source research extends the earlier
[command assessment](../assessments/2026-10-23-command-governance.md).
Two independent research passes covered output/scanning/file execution and policy/
hook/authentication/platform controls. No private transcript audit, live inference,
installation, upstream code copy or global configuration change was performed.
Firecrawl search returned an authentication error; web research supplied the fallback.
Current implementation evidence is separate from these upstream descriptions.

## Findings and disposition

| ID | Primary source and observed feature/limit | Decision for agentic-kit |
|---|---|---|
| GT-R1 | [RTK](https://github.com/rtk-ai/rtk) documents command-specific compression/grouping and savings reports. Its repository distinguishes filtering from a separately licensed compression engine. | Learn structural projection and measurement patterns; copy no implementation/license assumption. Measure our accepted-task economics, not upstream advertised savings. |
| GT-R2 | [RTK advisory GHSA-fvvm-949w-qj4w](https://github.com/advisories/GHSA-fvvm-949w-qj4w), published 2026-05-20, identifies project-local filters able to tamper with output before 0.32.0. | Treat reducer bytes as a trust boundary. No silent repository-defined filters. Security/coverage checks precede an explicitly lossy view; missing evidence stays visible. |
| GT-R3 | [Gitleaks README](https://github.com/gitleaks/gitleaks) distinguishes git/dir/stdin scopes, redact, configuration, decoding/archive/size controls and finding/error exits. | Prefer a pinned stdin adapter for exact bounded snapshots; parse normalized report plus process outcome, not exit alone. Do not equate secrets scanning with PII detection or exhaustive absence. |
| GT-R4 | [Go traversal-resistant APIs](https://go.dev/blog/osroot), [os.Root](https://pkg.go.dev/os#Root) constrain rooted access while documenting special filesystem/platform limits. | Use opened root handles, bounded regular-file reads and exact content identities. Compare opened identity before disclosure; no sandbox or hardlink sensitivity claim. |
| GT-R5 | [os/exec](https://pkg.go.dev/os/exec) uses argv and documents executable lookup and cancellation behavior. | No shell-string interpolation or PATH/CWD-selected trusted binaries. Future child execution needs pinned recipe/environment, bounded channels, process-tree cancellation and actual isolation where required. |
| GT-R6 | [Git diff](https://git-scm.com/docs/git-diff), [Git configuration](https://git-scm.com/docs/git-config), [Git environment](https://git-scm.com/docs/git) describe external diff/textconv and ambient configuration behavior. | Git inspection needs fixed arguments, disabled helpers and scrubbed config/environment. Qualify actual read recipe and hostile repo cases; do not generalize to arbitrary Git. |
| GT-R7 | [Cedar authorization](https://docs.cedarpolicy.com/auth/authorization.html) has default deny and forbid override, but skips errored policies. | A mandatory floor requires complete expected policies and no relevant diagnostic errors before a receiver can use allow. No new general evaluator or naive allow adapter. |
| GT-R8 | [OPA config](https://www.openpolicyagent.org/docs/configuration), [policy tests](https://www.openpolicyagent.org/docs/policy-testing), [decision logs](https://www.openpolicyagent.org/docs/management-decision-logs) provide bundle/trust/test/log controls. | External policy integration is optional and explicitly trust-configured. Signed bundles and sensitive log controls need receiving deployment proof. |
| GT-R9 | [Claude Code hooks](https://code.claude.com/docs/en/hooks) describe event matching, blocking semantics, failure and timeout behavior. | Prove exact installed/invoked events and negative paths for each pinned host. Hook availability, configured failure policy and actual interception are different facts. |
| GT-R10 | [Apple LocalAuthentication](https://developer.apple.com/documentation/localauthentication), [Authorization Services](https://developer.apple.com/library/archive/documentation/Security/Conceptual/AuthenticationAndAuthorizationGuide/Authorization/Authorization.html) distinguish authentication and authorization/platform limits. | Bind native presence to an exact trusted maintenance grant. Do not store passwords or present authentication as a grant over arbitrary later changes. |
| GT-R11 | [Windows UserConsentVerifier](https://learn.microsoft.com/en-us/uwp/api/windows.security.credentials.ui.userconsentverifier), [AppContainer](https://learn.microsoft.com/en-us/windows/win32/secauthz/appcontainer-isolation) describe separate consent and isolation capabilities. | Keep Windows adapters native and explicit; no portable biometric-success fiction or subprocess sandbox claim. |
| GT-R12 | [Linux Landlock](https://docs.kernel.org/userspace-api/landlock.html), [seccomp](https://kernel.org/doc/html/latest/userspace-api/seccomp_filter.html) describe different restrictions and limits. | Capability report distinguishes supported/enforced/unknown. No cross-platform common sandbox abstraction based on a filter alone. |
| GT-R13 | [Ollama structured outputs](https://docs.ollama.com/capabilities/structured-outputs) requires validating returned JSON against its schema. | Existing decision port supplies typed evidence. Shape is not accuracy, calibration, residency or permission. Unknown/error holds required review; no hosted fallback for local-only policy. |

## Owner synthesis

The stronger product is a **governed operation vocabulary plus operator controls**.
Replacing text command names alone leaves ambient authority and disclosure intact.
Keep useful in-process read/search first, then qualify actual Git and bounded check
recipes. Mutations and network effects need their own exact destination gates.

Existing CG contracts already cover many requested ideas. The uplift adds explicit
lossy-output integrity, scanner report/error semantics, mandatory policy diagnostic
handling, native authentication versus grant binding, and task-level economic proof.
No scanner, classifier, source index or biometric prompt can stand in for these.

The complete requirement mapping and acceptance live in the [delivery specification](../designs/governed-command-tools.md)
and [GT work order](../plans/governed-command-tools.md). Deferred rows remain required
for the original full objective. Neither this research nor a first command increment
closes the broader goal. [0039](../intent/decisions/0039-governed-command-tools-preserve-evidence-and-authority.md)
records admission and rejected alternatives.
