# Landing evidence

ADR-081's local receipt path records one clean git tree, framed gate log and command
identity without a store table, signing, transport, gate dispatch or model call.
The wrapper emits the commit, tree and clean state at START and PASS, fails if they
change or either endpoint is dirty, logs the Go environment, and calls the receipt
writer only after PASS. Write and check bind both endpoints to the receipt tree.

Anyone who can run `fathom gate receipt write` as you can create an accepted receipt;
the guard stops editor-tool tampering only; CI is the independent witness.

The repository's enforced command guard denies agent receipt-writer invocations;
the enforced file guard protects its receipt directory. These controls do not
authenticate unsigned evidence. Acceptance stays off unless the project sets
`[gate] accept_receipts = true` and the user sets
`[gate] allow_receipt_acceptance = true`. Either alone does nothing; doctor names
half-set consent. This repository leaves acceptance off.

The checker requires the logged Go version and os/arch to match local `go env` and
`go version`, and requires `GOWORK=off`. Command identity reads gate definitions
from the requested git tree. For HEAD, the tree hash already covers tracked gate
definitions and scripts. Pre-push checks every pushed non-delete local SHA and
falls back to validate if any receipt refuses. CI remains independent.

## Decision models

None applies: source identity, consent and log checks are deterministic and offline.
