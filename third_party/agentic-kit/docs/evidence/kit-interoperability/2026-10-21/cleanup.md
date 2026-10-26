# Recoverable contribution cleanup

Date: 2026-10-21. Tested source: 3f554730207dd975d2e4b5ec36989cae31d2bbda.
This supersedes the preservation state in the earlier
[contribution inventory](../../catalog-kit/2026-10-21/contributions.md).
That audit and decision 0016 record where the valuable work landed and why weaker
historical drafts were superseded. The subsequent independent audit found no
additional useful inactive delta. Active implementation was retained and committed.

## Recovery artifact

Local backup: `/Volumes/m2ext/backups/agentic-kit/2026-10-21-workspace-cleanup-8xkoq2p9`.
It contains an independent Git mirror made without hard links, a portable bundle,
a separate restore-check clone and four complete worktree archives. Private or
ignored files stay in this local backup, never in the public repository.

`all-refs.bundle` SHA-256:
`e751be7bc131e29320a9f7ef937b2092ca0230050a9d98e2c11f9d3ad2884b30`.

Before pruning, the backup verified all 27 original refs, all four stash commits
and 84 unreachable commits in the mirror and separate bundle restore. Preserved
refs make those otherwise unreachable objects recoverable. File-name, bytes,
symlink-target and mode inventories of each worktree matched its archive,
including ignored/untracked content and excluding only the worktree `.git`
pointer. Each source was checked again before removal. Activity checks reported
no open files or command errors. The manifest and cleanup receipt retain the
exact inventories, object IDs, archive hashes and operations.

| Archive | Worktree HEAD | Archive SHA-256 |
|---|---|---|
| `gw-cli-wire` | `f8ec57feb6ef5a13347c9e15b2081896742d3902` | `77e8f8d7805f95a3db8f869ba2d27c641ea48723da24a75c3c2f452ebed08c6d` |
| `gw-sourced-scenario` | `5256950985fa1b667b0613aa97ec99123ba6336d` | `49aab1d1c65e3db180dedd155d97940b25e823def6fe5896ecaa644babd0e56e` |
| `gw-workflow-cli` | `1b294515fd6f6eb8103218bae7894fe2560b8489` | `8ce8f7bbf47ff98d8f2d0b2b877d7861998221de46b613f43d21c6c009d6befc` |
| `evidence-reuse` | `3b5b3c86fb5103177c11571afc80e6615c74c39f` | `285291f2c7c05033527219e38e14b803579b6a1443fbe3d72bca69a490b0ad15` |

Recovery never requires overwriting the primary checkout. For example:

```sh
git clone /Volumes/m2ext/backups/agentic-kit/2026-10-21-workspace-cleanup-8xkoq2p9/all-refs.bundle /path/to/new/recovery-checkout
git -C /path/to/new/recovery-checkout branch recovered-draft <recorded-commit>
mkdir /path/to/new/recovery-files
tar -xzf /Volumes/m2ext/backups/agentic-kit/2026-10-21-workspace-cleanup-8xkoq2p9/gw-workflow-cli.tar.gz -C /path/to/new/recovery-files
```

Inspect `manifest.json` for each original ref, stash commit and file inventory.
The backup is local recovery evidence, not an off-machine backup guarantee.

## Removed inactive worktrees and local refs

- `/Volumes/m2ext/code/geoffmilleraz/agentic-kit-preserved-drafts.BDvb4I/gw-cli-wire`.
- `/Volumes/m2ext/code/geoffmilleraz/agentic-kit-preserved-drafts.BDvb4I/gw-sourced-scenario`.
- `/Volumes/m2ext/code/geoffmilleraz/agentic-kit-preserved-drafts.BDvb4I/gw-workflow-cli`.
- `/Volumes/m2ext/code/geoffmilleraz/agentic-kit/.claude/worktrees/evidence-reuse`.

Eight superseded local branches were removed only after matching their exact
heads against the verified backup:

- `docs/intent-excellence-assessment` at `fac9fe736614dd8dfd52c098fa2a9f0c14960ba7`.
- `evidence-producer` at `26392123d1bd099ff837ee77ec50fd304bbd30e2`.
- `operational-reuse` at `a72939b3669864360529d2e005a02a5128f63f51`.
- `integrate/catalog-kit-2026-10-21` at `23a243c23682952bfc6ebcabf90e6b5774ec8110`.
- `worktree-evidence-reuse` at `3b5b3c86fb5103177c11571afc80e6615c74c39f`.
- `worktree-gw-cli-wire` at `f8ec57feb6ef5a13347c9e15b2081896742d3902`.
- `worktree-gw-sourced-scenario` at `5256950985fa1b667b0613aa97ec99123ba6336d`.
- `worktree-gw-workflow-cli` at `1b294515fd6f6eb8103218bae7894fe2560b8489`.

Retain the active `improve/kit-interoperability-2026-10-21` branch through
fast-forward publication, then remove it by ordinary merged-branch deletion.
The two empty worktree-parent directories were removed with `rmdir` and worktree
metadata was pruned. No unrelated directory, cache or consumer checkout was removed.

Four superseded stash entries were cleared after a final exact-object comparison:

- `47cdaa8cf9d90831721e7bda9b1daa9b99998c63`: On main: integration-2026-10-21-preserve-fathom-wiring.
- `01d24ac3b2edba253302bb0639db92d373a3b69f`: WIP on fix/doctor-trust-dedupe: f4f0076 release: 0.9.0 - token-economics hooks for Claude Code and Codex (cmd/agent-hooks).
- `9ea3c77ec2940dae5ea2797d251c527e30de9e54`: On worktree-gw-sourced-scenario: temporary prose and HITL dependency overlays.
- `ade08f0b894c7b0ac27f16e45d063e9bc2ce581e`: On worktree-gw-workflow-cli: gw13-cli-only-before-baseline.

## Remote refs and publication

The three superseded remote branches were atomically deleted with explicit
expected-SHA leases. A fresh fetch and `ls-remote` matched the recorded heads
before deletion; the guards would refuse a moved ref. Normal pre-push hooks passed
source-backed plan proof (44.56 s) and full validation (26.68 s). Push exit 0,
74.09 s overall; log: `/Volumes/m2ext/tmp/agentic-kit-remote-prune.log`.

| Remote branch | Removed HEAD |
|---|---|
| `agent-tokenomics` | `8af29393862e15c30364ab6b2ae81685bff6e6ce` |
| `codex/catalog-kit-2026-10-20` | `89fbe33acc4efd6d29de34357f6353cb13d3ca08` |
| `integrate/catalog-kit-2026-10-21` | `23a243c23682952bfc6ebcabf90e6b5774ec8110` |

Post-deletion `ls-remote` and pruned fetch showed only unchanged main, ready for
fast-forward publication. The final all-state GitHub inventory still contained
the same eleven reviewed PRs and zero open PRs. No tag was moved or removed.

The recovery receipt is `/Volumes/m2ext/backups/agentic-kit/2026-10-21-workspace-cleanup-8xkoq2p9/cleanup-receipt.json`.
The final live-state acceptance is a clean primary checkout on main, one registered
worktree, no stashes and no superseded local or remote branches. Disposable proof
snapshots outside the checkout remain evidence, not registered worktrees. Original
consumer edits and private prerequisite fixtures remain untouched by this cleanup.
