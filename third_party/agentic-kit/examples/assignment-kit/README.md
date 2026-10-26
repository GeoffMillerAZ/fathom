# Assignment Kit reference workflows

Run from the repository root:

```sh
go run ./examples/assignment-kit
go test -race ./examples/assignment-kit
```

The program emits a JSON wrapper with `synthetic`, `evidence_claim` and a `snapshots` object keyed by named case. Every observation, source artifact, policy, subject, brief revision and calibration claim here is synthetic. These reports prove composition behavior in memory, not production evidence or permission to run a command.

| Snapshot | Expected status | What it demonstrates |
|---|---|---|
| `command/complete` | `complete` | Input and privacy requirements both pass. |
| `command/missing-required-unknown` | `continue` | Missing privacy evidence projects unknown; one pass cannot complete. |
| `investigation/complete-stretch-failed` | `complete` | Failed optional summary remains visible without blocking required results. |
| `investigation/unresolved-ambiguity` | `blocked` | Open blocking source concern holds completion. |
| `investigation/resolved-complete` | `complete` | Retained resolution artifact clears the concern. |
| `investigation/no-progress-pivot` | `pivot-required` | Two reported attempts without progress recommend a search revision. |
| `investigation/ambiguity-escalation` | `needs-review` | Two reported ambiguities recommend human review. |
| `investigation/exhausted-budget-stop` | `stopped` | Equality with the exact search budget is terminal, not success. |

The command qualification assignment has two separate required goals: exact proposal input and argument binding, and privacy. The investigation has three separate required goals: independent expected question coverage, retained source support, and counterevidence disposition. Its summary is a stretch goal. Each criterion binds an exact Catalog Kit stable key and `NodeDigest` of the complete synthetic definition, including the rule in its synopsis. Changing or moving a catalog definition invalidates the old assignment pins.

The examples author `Scope.RequiredGoalIDs` as explicit receiver rosters in each workflow. They do not derive scope from the goals being checked. Tests plant omitted privacy and counterevidence goals and prove that those rosters refuse them. An adopter must independently qualify source truth, artifact retention, policy ownership, and intent revisions. Assignment Kit never runs commands, searches, contacts reviewers, reserves spend, or grants an effect.
