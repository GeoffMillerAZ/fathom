# Project Intent — examples

## 1. Future-proofing: good and bad

Each pair: the documented future, then the two moves. The good move is free; the bad
move pays now for later.

**Multi-tenancy (plausible, later).**
- Bad: a `Tenant` interface, tenant-scoped repositories, a tenant column on every
  table, tenant-aware middleware, "for when we go hosted".
- Good: primary keys are opaque ids, not `(user_email)`; nothing in the schema assumes
  exactly one household. Zero extra code. A tenant boundary is later a column and a
  filter, not a rewrite.

**Second storage backend (speculative).**
- Bad: a `Storage` interface with one implementation, a factory, and a config key.
- Good: nothing. Speculative futures never justify a free option. Call SQLite directly.
  If a second backend ever becomes plausible, the interface is extracted in an
  afternoon from one real implementation, which is the only way good interfaces are
  extracted anyway.

**Historical replay of events (likely, next; a documented `Revisit when` on the
messaging decision).**
- Bad: build Kafka now "because replay is coming".
- Good: say "trigger fired: decision 0004 (in-memory bus) lists durable replay as a
  revisit condition" and reopen the decision in the design session. The outcome may
  still be "not yet", recorded with the new evidence.

**Windows port (plausible, someday).**
- Bad: a platform abstraction layer over every filesystem call.
- Good: the one function that does the platform-specific copy takes a path and returns
  an error; nothing else knows it is APFS. Same code, one seam, no layer.

**A team-shared config layer (likely, later).**
- Bad: implementing org → team → person cascade resolution, publishing, and review.
- Good: configuration is already unified from any number of files, so a second file
  from another source composes with no new mechanism. Do not add a `team:` field until
  the feature is a goal.

## 2. When not to write a decision record

- Naming, formatting, file layout inside one package.
- Any choice a type, test, or lint already enforces and whose name explains it.
- A dependency bump with no API change.
- Picking one of two equivalent libraries for a leaf concern.
- Anything reversible in an hour with no data or API consequence.
- A rejected idea nobody will propose again.
- Implementation detail of a decision already recorded (put it in the code comment).

Litmus test: would a future reader, lacking this record, reasonably rebuild the system
differently? If no, do not record.

## 3. A trigger firing, end to end

User: "Let's design offline synchronization."

Agent, before designing: reads `docs/intent/README.md`; the routing table classes this
as a new subsystem; loads `scope.md`, `invariants.md`, `principles.md`, the decisions
index, and grep-matches `futures.md` for "sync", "offline", "multi-device". Reports:

> Principle P3 says interactive actions never wait on the network when local state
> suffices. Future F2 (multi-device, plausible, later) is affected. Decision 0006
> (single SQLite file, no server) lists "Revisit when: state must be reconciled across
> more than one device" as a trigger. **That trigger has fired.** Recommend reopening
> 0006 in this session rather than designing sync around it. Not in scope today:
> conflict UI, account system, hosted relay (all `Do not build` on F2).

Then the design. Then `decide`: a record superseding 0006 if the outcome changed the
storage model, or a record with `Status: deferred` if the outcome was "not yet", with
the new condition.

## 4. A minimal decision record

```markdown
# 0007: Drift is compared on semantic hashes, never raw bytes

**Status:** accepted
**Type:** architecture

## Context
cfprefsd rewrites plists in binary with arbitrary key order; Electron apps rewrite
JSON wholesale. Raw-byte comparison flags every app launch as drift (incident
2026-06-14, 41 false drift reports in one day).

## Decision
Every managed file has a per-format canonicalizer; drift is `semantic(live) !=
semantic(last-applied)`. Raw hashes are kept only for adopt hints and dedup.

## Alternatives rejected
- **Raw byte hash** — drowns in noise. Wins if: every managed format were opaque.
- **Ignore-list of volatile keys** — grows forever, and is per app. Wins if: only
  one or two formats were ever managed.

## Consequences
A canonicalizer per format (JSON, JSONC, plist, TOML, YAML, INI, opaque). Unknown
formats fall back to raw and are noisy by design.

## Revisit when
- A managed format has no deterministic canonical form and its drift noise exceeds
  the 7-day ignored-report SLO.
```

## 5. A rejected proposal as a record

```markdown
# 0009: Do not add a background enforce daemon in v0

**Status:** rejected
**Type:** architecture

## Context
Proposed: a daemon that rewrites drifted files automatically.

## Decision
Rejected for v0. Enforcement is foreground only.

## Rationale
A background writer fighting an app for its own config file is the original
incident (read-only symlink farm) in a new costume. The notify pipeline has no
track record yet.

## Revisit when
- The notify pipeline has run 60 days without a false drift report, and only for
  units the tool fully owns; never for app-owned key-mode files.
```
