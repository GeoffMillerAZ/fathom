### Added

- `fathom gate receipt write|check` records clean-tree gate evidence and checks exact tree/command identity, log integrity and the gate's final success line. The gate wrapper publishes receipts after PASS; logs bind both endpoints to the clean tree and carry the Go environment. Receipt acceptance requires compatible local Go/platform, `GOWORK=off`, project opt-in and user consent; pre-push checks every pushed local SHA. Acceptance remains off in this repository.
- `fathom gate record-ids` checks duplicate ADR, invariant, future, incident (including bold entries) and active-plan task ids in validation, excluding archived plans. This repository enables cross-plan checks with explicit existing-collision exceptions; stale exceptions fail.
