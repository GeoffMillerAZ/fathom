# Registered Git reads: scoped qualification

GT6, 2026-10-24. [0040](../../../intent/decisions/0040-git-reads-require-registered-metadata.md)
and [GT-G1..5](../../../designs/command-git.md) define the supported profile.
This closes the three fixed read recipes, not the broader command/hook program.

| Evidence | Result and boundary |
|---|---|
| [Source identities](source-pins.json) | Exact command, tests, controls, guide, skill and owning design bytes, based on `e22ca0d`. The tested native binary is independently identified below. |
| [Native entrypoints](native-trial.json) | 39/39 owned synthetic cases passed on macOS arm64, Go 1.26.3, Git 2.54.0. No real project history, credentials, provider or hook installation. |
| [Actual report shapes](report-schema-trial.json) | 38 nonempty native reports passed `cue vet ... -d '#GitReport'`; the remaining tiny-budget case correctly emitted no bytes and exit 2. |
| [Security review](security-review.md) | Independent differential/sharp-edges review plus a narrow final integration reread. Failures were repaired rather than waived. |
| [Guide trial](guide-trial.md) | Independent README/schema-only setup succeeded, then copied revised examples were retested against the final binary. |
| Tests/gate | Focused race tests passed (73.4% statement coverage, not behavioral assurance). `task validate` passed with format, vet, CUE, constitution and full race suites. |
| Plan proof | Fresh private-store ingestion and strict candidate verification: five claimed GT rows verified, zero failures. This is static symbol/test reachability, separate from native/test execution. |
| Other hosts | Windows amd64 and Linux amd64 cross-builds passed. Neither was executed natively; Git execution on Windows currently returns unsupported. GT4 stays open. |

Native binary SHA-256:
`ad12b985b0019711f64fc8fd7930d57b165fa023cc7d4d5aced6609afabd3be4`.
Git executable SHA-256:
`90f3250b0bd80bc9d6026e1f4a10d175e04a8e1302fdaf662acb28b9bf5ebc24`.
The optional Gitleaks cases use the previously qualified 8.30.1 profile and scanner
identity in [the scanner receipt](../2026-10-24/README.md).

The native corpus covers ordinary/packed/linked/bare/unborn/SHA-256 contexts,
literal leading-dash/space selectors, bound history and same-HEAD comparison,
signature-header structure and empty messages, omitted large-source checks,
late regex/Gitleaks findings, ignored/submodule exclusions, conflicts, binary/
symlink/gitlink/oversize/missing objects, metadata redirection, includes, alternates,
promisor/replacement state, ambient helper poisoning and tiny output budgets.
Every case retained a stable before/after content/mode/path inventory. Planted
filter/fsmonitor/helper witnesses stayed absent in the tested paths. Those observations
do not prove that no transient write, child process or network operation could occur.

Raw controls, reports, synthetic fixtures, trial script, source snapshot, guide results,
failed pre-fix cases and logs are retained privately under the maintainer's dated
`2026-10-24-command-git` evidence directory. Public receipts omit host paths and raw
source bodies. One-shot elapsed times are corpus observations, not a benchmark,
production savings estimate, memory ceiling or hard latency guarantee.

## Remaining limits

The operator must protect registered metadata, directory resolution chains and
executables from candidate writes. Path/hash checks cannot establish that authority.
Metadata enumeration holds above 65,536 entries; otherwise healthy larger repositories
are unsupported. Filesystem preparation is cooperative with the deadline and cannot
interrupt a blocked OS call. Live status is non-atomic, ignores specified classes
and cannot establish clean/absent state. History's lookahead is a Git-reported valid
OID, not independently read older content. A signature-shaped header is not verified.
Scanner success is neither secret/PII absence nor permission to disclose to a receiver.

No protected library roots changed, so this unit does not claim downstream consumer
runtime coverage. Broader scanners, native platforms, authenticated policy/hook
controls, models, caches, Fathom enrichment, mutation and rollout remain in GT4/5/7..17.
Prose/HITLMAN remains the flagship priority under 0038, not completed by this reader.
