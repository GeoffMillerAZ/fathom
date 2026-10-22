# Multi-machine development

Decision: [ADR-078](../../ADR.md#adr-078-multi-machine-development-every-machine-its-own-loopback-fathom-git-moves-code-receipts-move-evidence-snapshots-move-the-index-placement-is-not-fathoms-2026-10-22), accepted by the owner-architect, 2026-10-22.
Plan: `docs/plans/multi-machine.md` (MM-). Operator contract: `docs/guides/multi-machine.md`.

## Problem

A controller can send builds, tests or agent tasks to a helper, but a code answer must describe the checkout that helper edits. A central index answers about some other checkout. The daemon has no authentication; loopback is its boundary (I14, ADR-069 decisions 9–10). A local daemon and fail-open hooks do not depend on a working cross-machine network. Every machine therefore serves only its own checkouts, and a helper starts a daemon only when the job needs local fathom answers.

## The three movers

```text
controller checkout                   helper task-owned checkout
        | -------- git code -------------------> |
        | <------- git branch or bundle -------- |
        | <------- attested receipts + logs ----- |
local loopback daemon                 local loopback daemon (if needed)
        | -------- index snapshot -------------> |  future: IXS phase 1
                 operator carries files
                 no daemon-to-daemon calls
```

Code travels on git over ssh or in bundles. Evidence travels as attested receipts and gate logs with source identity. Indexes will travel as exported index snapshots once their plan is unparked, never as a copied live store. Fathom computes the identity and reads operator-named artifacts; the operator's existing carriers do all uploading, fetching, signing and transfer. No transfer port is added.

## What exists today

- [`requireLoopbackHost`](../../cmd/fathom/admission_http.go#L151) rejects a non-loopback Host and a foreign Origin. This is the boundary of an unauthenticated local daemon, not a remote access mechanism.
- The [`test_runs` envelope](../../internal/store/sqlite_testevidence.go#L34) already stores run digest, signer tier, watermark, commit SHA, dirty state and race state (ADR-067 decision 11). It does not yet hold all of D4's platform, host-label, toolchain, tree and dirty-diff identity.
- [Index snapshots](../plans/index-snapshots.md) already have a parked plan: `export snapshot` and `ingest --cache-from` are IXS phase 1, with cold/cache parity at IXS-M0b. Naming a helper cold start does not implement these commands.
- [Cloud sessions](../adoption/cloud-sessions.md) are the prior single-helper case: one fresh environment, its own checkout, index and loopback daemon. Their environment-specific setup is prior art; this contract requires project-local MCP registration.
- [Agent waves](../guides/agent-waves.md) already define isolated worker environments and a base-commit brief. This design extends the operator recipe across machines without changing the dispatch boundary.

## What changes now

Ship the operator guide and `fathom-offload` skill. Each job owns a checkout at an exact recorded commit, scratch outside every repository and Go module, and a result file. Set both `TMPDIR` and `GOTMPDIR` there. The helper reports source identity before and after work, exact commands and exit codes, and artifact sha256 sums. The controller checks every sum and the input identity before integration, calls `review_diff` with `since` equal to that base commit, and runs strict verification on a fresh scratch ingest of integrated HEAD.

The host label comes from the operator, never from a machine hostname. Until receipt columns exist, git and shell stamp identity in the result file; the guide gives the commands. Dirty status and the diff digest are both recorded: `git diff --binary` does not include staged or untracked content, so preserve that evidence explicitly if present. Returned code must be committed on the job branch.

Heavy jobs first inspect load with the orchestrator's own tools and cap concurrency. Existing sessions and servers are never stopped or restarted. MCP registration stays in the helper job's project; no global agent settings are changed. No operational store, credentials, private keys or build cache directory is transferred.

## What waits for RT-M1

D4's receipt identity columns wait for RT-M1 PASS under ADR-049 decision 2's table freeze. D3 names helper-machine cold start as a consumer of IXS phase 1, after RT-M1 PASS and IXS-M0b parity; export metadata itself is a table. The index-snapshots plan remains parked until its existing start conditions hold. No row in it changes here.

[F92](../intent/futures/f92-f97.md) records static job cost evidence: measured peak memory and duration from resource-bearing receipts could inform an external orchestrator. Promote only after at least 30 attested runs carry resource fields AND an orchestrator asks for placement evidence in writing; drop if none asks within six months (2027-04-22). This does not build resource collection or placement.

## Non-goals

Fathom does not schedule, place or monitor work across machines, or read host load. Load is ephemeral telemetry, not a governed code fact; background sampling meets the freeze and network calls meet I14. A terminal multiplexer's agent list or operator-run ssh with `uptime`, `vm_stat` and `df` belongs to the orchestrator.

There is no central shared daemon, daemon-to-daemon RPC, transport, credential handling or synchronization. Threads remain per-machine `~/.fathom/threads.jsonl`; the controller owns the ledger and helpers return result files. Observations remain repository files moved by git. MM-M1 remains open until a real remote round trip is read; this design is not that reading.

## Decision models

None applies. Every fact here (source identity, receipt match, snapshot key) is computed, and placement is out of scope (ADR-078 D5).
