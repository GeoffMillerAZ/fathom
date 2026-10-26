# Pinned command scanner qualification

GT5 design/implementation research, 2026-10-24. Receiver: `command-guard` selected
snapshot inspection. [GT-C3a](../designs/governed-command-tools.md#gt-c3a-gitleaks-stdin-qualification-profile)
records the exact profile and acceptance, not broad secrets/PII absence.

## Sources and decisions

Inspected Gitleaks `v8.30.1`, tag commit
`83d9cd684c87d95d656c1458ef04895a7f1cbd8e`, through GitHub's API. An independent
critic inspected the same source and the final combination of checks.

| Source | Finding | Receiver decision |
|---|---|---|
| [stdin command](https://github.com/gitleaks/gitleaks/blob/v8.30.1/cmd/stdin.go) | Config initializes against cwd; source is stdin; errors may terminate before report. | Private empty cwd and environment, embedded-default config only, no implicit config/ignore discovery. |
| [root command and summary](https://github.com/gitleaks/gitleaks/blob/v8.30.1/cmd/root.go) | Stdout reports are supported; default exit 1 conflates findings/errors; scanned-byte and completion messages are human logs. | Fixed findings exit 3 and a version-specific strict log/report/exit/deadline contract; unknown message holds. |
| [source reader](https://github.com/gitleaks/gitleaks/blob/v8.30.1/sources/file.go) | MIME classification may skip application data before yielding any detector fragment. | Exact supplied/observed byte equality rejects skipped nonempty input. Byte equality alone is insufficient. |
| [detector](https://github.com/gitleaks/gitleaks/blob/v8.30.1/detect/detect.go) | Bytes count before rule evaluation; fragment errors can be logged without a final error. | Require normal completion plus no extra/error/partial messages; adapter owns cancellation, internal timeout disabled. |
| [report fields](https://github.com/gitleaks/gitleaks/blob/v8.30.1/report/finding.go) | Redacted reports still contain metadata and arbitrary strings. | Bound both channels in private memory; expose fixed categories/counts only, never raw reports/errors. |
| [configuration](https://github.com/gitleaks/gitleaks/blob/v8.30.1/config/config.go) | Custom TOML can extend other paths and change native allowlists. | No custom/transitive config in this profile. Guard regex rules remain the configurable org/PII criterion path. |

The critic initially described archive skipping more broadly than source evidence
established for stdin's empty filename. That claim was corrected before the design
was accepted. The confirmed case is MIME-based application skipping. An unrelated
[upstream detection report](https://github.com/gitleaks/gitleaks/issues/2170) was closed
after a synthetic token matched an intentional stopword; it is not a release defect.

## Direct native observations before integration

Downloaded the official Darwin arm64 release archive to a private scratch directory,
verified the GitHub release digest, extracted only its regular executable member and
ran against synthetic bytes with private cwd/HOME/TMP and minimized environment.
No PATH, installed hook, global config, credentials or provider calls changed.

| Synthetic input | Input bytes | Exit / report | Observed scanned bytes |
|---|---:|---|---:|
| Public plain text | 14 | 0 / empty array | 14 |
| Independently assembled fictitious generic credential | 45 | 3 / one finding | 45 |
| Text-only PDF signature and prose | 40 | 0 / empty array | 0 |
| Empty text | 0 | 0 / empty array | 0 |

The PDF result is a concrete false-completion counterexample for an exit-only wrapper.
The adapter must hold it. Version output was exactly `8.30.1` with no stderr.

Archive SHA-256:
`b40ab0ae55c505963e365f271a8d3846efbc170aa17f2607f13df610a9aeb6a5`.
Executable SHA-256:
`ba52fb1bfabbcde42f032afad3d6e0b19dff8ed105229a16e7caa338bbc0e84f`.
[Release asset](https://github.com/gitleaks/gitleaks/releases/download/v8.30.1/gitleaks_8.30.1_darwin_arm64.tar.gz).
These establish the retrieved artifact's identity through GitHub HTTPS metadata;
they are not an independent signature attestation or qualification of another host.
Integration, fault-injection, CUE/Go and gate receipts remain separate evidence.
