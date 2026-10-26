# 0040: Git reads require registered metadata

Status: accepted
Type: security
Date: 2026-10-24

## Context

GT6 extends 0039's command tools with status, immutable object comparison and history.
Existing quality-gate Git code supplies useful read safeguards, but also write recipes
and ambient non-Git environment. It is not a ready-made general safety runner.
Git can read gitfile redirection, config includes and alternates before producing any
result. Candidate-owned administrative metadata is outside a fixed-argv guarantee.

## Decision

- Admit the [GT6 registration and three read recipes](../../designs/command-git.md)
  for the kit's synthetic review receiver, inside the existing W1 slot and budget.
  No new library, dependency, service, installation, hook or third incubation.
- Require operator-protected registration of canonical root, admin/common/object/index
  topology and exact configuration files before any repository command. Override
  discovery with explicit Git context. A local registration is caller data, not proof
  of organization authority or OS protection; GT8 retains that separate responsibility.
- Use one pinned executable snapshot, minimal environment, fixed read-only commands,
  finite buffers and a whole-invocation budget. Reject unqualified includes, alternates,
  shallow/promisor and repository extensions. Keep replacement/lazy fetch/helpers off.
- Preserve all three use cases. Status reports live observations, with checked HEAD/
  index/config drift and explicit non-atomic worktree semantics. Object diff reads
  pinned commit/tree/blob identities. History reports only the bounded selected walk.
- Scan complete selected content and the exact source-data representation before any
  result is displayed. Optional Gitleaks remains required when configured. Git IDs,
  paths and commit metadata are explicit source-data disclosure, never diagnostics.
  They are not authorization or proof of truth, absence, attribution or a clean repo.
- Reuse narrow behavior from quality-gate and the existing guard, not write-tree,
  hash-object writes, caller index overrides or an arbitrary Git argv surface.
  Native platform qualification and no-write/hostile cases remain required evidence.

## Rejected alternatives

- Treat `rev-parse` as safe discovery in an untrusted `.git`: redirection happens
  before the result. Wins if: metadata was independently registered/protected first.
- Rebuild Git or copy its whole object database per query: high cost and another
  parser without a present need. Wins if: a measured missing isolation boundary
  requires an independently admitted snapshot adapter, not another universal engine.
- Call a quiet status or stable pre/post hashes an atomic clean-workspace proof:
  ignored/unreported paths and change-then-restore races remain. Wins if: an owner
  supplies a qualified frozen snapshot and independently complete scope.
- Use Git pretty log, filters, textconv or user-selected flags for convenience:
  adds callback/format ambiguity. Wins if: a separately versioned recipe proves the
  exact required capability without weakening this read profile.

## Revisit when

An unregistered metadata source is accessed; a helper/network/write witness fires;
source state drifts without being labelled; omitted content escapes checks; a native
host fails; false holds or costs exceed the declared bars; or a second consumer
establishes a shared extraction need. Review 2026-11-21 or next release under 0039.
