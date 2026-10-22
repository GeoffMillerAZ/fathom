### Added

- `fathom gate receipt write|check` records clean-tree gate evidence with a copied log and checks exact tree/command identity, log integrity and the gate's final success line. Pre-push receipt reuse is opt-in and remains off in this repository.
- `fathom gate record-ids` checks duplicate ADR, invariant, future, incident and per-plan task ids in validation, excluding archived plans. Cross-plan checks are opt-in with explicit exceptions for existing shared ids.
