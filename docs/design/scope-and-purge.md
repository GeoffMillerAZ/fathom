# Scope and purge — reversible suppression, irreversible removal, both provable

> Phase V of `docs/vision/20-ambient-fathom.md`. Answers the owner's ask:
> *"we should have a way to turn off certain paths in the fleet. It would
> be great if that could be done from the UI. And it could be a temporary
> turn off, and there should also be a purge option in case something gets
> indexed that shouldn't be."*
>
> **Status:** design. Builds on the existing `Store` primitives
> (`PurgeSource`, `PurgeService`, `RemoveService`, `DeleteFingerprint`,
> `ListSourceRefs` in `internal/store/store.go`) and `.fathomignore`.

---

## 1. Two operations, deliberately not one

The ask names them together and they must be built apart, because they have
opposite risk profiles and opposite failure modes.

| | **Suppress** | **Purge** |
|---|---|---|
| What it does | hides matching content from answers | deletes it from storage |
| Reversible | yes, instantly | **no** |
| Takes effect | next query | on completion of the run |
| Needs re-ingest | no | no (it is the inverse) |
| Time-boxable | yes (`--until`) | meaningless |
| Failure if wrong | an answer is missing | data is gone |
| Proof required | that the filter applies | that **nothing remains anywhere** |

Conflating them produces the worst of both: a "delete" that leaves copies,
or a "hide" that destroys data. The verbs stay separate and the UI never
puts them on the same button.

**Why suppression is not just `.fathomignore`.** `.fathomignore` is an
*ingest-time* decision, committed to the repository, affecting everyone,
taking effect only on re-ingest. Suppression is a *query-time* decision,
local, instant, optionally temporary, and reversible without re-indexing
60,782 symbols. They coexist: `.fathomignore` is the durable policy,
suppression is the operational control.

---

## 2. Suppression

### 2.1 Model

```
suppression := {
  id           // stable, generated
  service      // "" means fleet-wide
  pattern      // gitignore-syntax path glob, matched against source_ref
  reason       // required, free text — a suppression with no reason is a mystery in three weeks
  created_at
  expires_at   // nil = indefinite
  created_by   // "ui" | "cli" | "config"
}
```

Stored in `~/.fathom/suppressions.jsonl` — user-central, never in the
repository, never shared (law 13). A suppression is an operational fact
about *this machine's* index, not a policy about the code.

### 2.2 Where the filter applies

At **one** place: the query layer, immediately before results are returned,
applied to every surface uniformly — MCP tools, CLI, HTTP, and the
observatory's own counts. One chokepoint is the only way to be sure there
is no bypass, and it is testable by enumerating the surfaces.

Suppressed content is filtered, and **the fact of filtering is disclosed**:

```json
{ "symbols": [...], "suppressed": { "count": 14, "reason": "vendored corpus (expires 2026-08-15)" } }
```

A silent filter is indistinguishable from a recall bug, and this program
has spent real time chasing recall bugs. Every response that dropped
something says so. This is the same value-or-named-reason discipline the
observatory uses (law 3), applied to results.

### 2.3 Expiry

An expired suppression stops filtering at the next query — no daemon
timer, no sweep, expiry is evaluated at read time so it cannot be missed
while the daemon is down. Expired entries are retained in the file, marked
expired, so the history of "what was hidden, when, and why" survives.

### 2.4 Verbs

```
fathom scope list [--service S]
fathom scope suppress '<glob>' --reason '<why>' [--service S] [--until 7d]
fathom scope unsuppress <id>
fathom scope preview '<glob>' [--service S]     # what WOULD be hidden, count + sample
```

`preview` exists because a glob that matches more than you think is the
main way this feature hurts someone. The UI always previews before
suppressing; the CLI offers it.

---

## 3. Purge

### 3.1 The requirement, stated as a claim to be proven

> After a purge of pattern `P` in service `S`, no content originating from
> a path matching `P` in `S` is retrievable through any fathom surface, and
> no copy remains in any store fathom writes to.

The second clause is the hard one, and the reason purge cannot be
`DELETE FROM observations`.

### 3.2 Every place a copy can be

Enumerated from the live schema (2026-08-08) — this list is the design, and
a new table that can hold content **must** be added to it in the same
commit that creates the table:

| Location | Holds | Purge action |
|---|---|---|
| `observations` | symbols, facts, snippets | delete by `(service_id, source_ref)` |
| `typed_edges` | call/reference edges | delete rows citing the ref on either end |
| `typed_edge_fingerprints` | per-package edge hashes | delete for affected packages |
| `doc_summaries` | extracted doc text | delete by `(service_id, source_ref)` |
| `fingerprints` | content hash per file | delete — otherwise the hash gate treats the file as already-ingested and re-ingest silently skips it |
| `knowledge_cards` | curated per-service knowledge | delete rows citing the ref |
| `semantic_claims` | compiled layer claims | delete claims whose evidence cites the ref |
| `claim_links` | claim → evidence edges | cascade with the claims |
| `claim_events` | append-only claim history | **see §3.4 — not deleted by default** |
| `compile_runs` | layer compile metadata | retain (metadata, no content) |
| vector index (`internal/vector`) | embeddings of content | delete vectors for affected refs |
| result cache (`internal/resultcache`) | recent tool answers | **invalidate wholly** — it is keyed by query, not by ref, so selective removal is not possible and a full flush is the only sound option |
| call log | tool name, service, outcome | retain — query text is never logged, so no content is present |
| session ledger | counts only | retain — no content |
| `~/.fathom/briefs/*.md` | rendered aggregates | retain — counts only, no content |

The result cache row is the one that would have been missed. A purge that
leaves a cached answer containing the purged content is a purge that
failed, and because the cache is query-keyed there is no clever partial
answer: flush it.

### 3.3 Verification is part of the operation

**A purge that reports success without verifying is theatre.** So the run
has four phases and the third is not optional:

```
1. resolve   → expand the pattern against ListSourceRefs; show the exact
               file list and per-table row counts; require confirmation
2. execute   → delete across every location in §3.2, in one transaction
               where the storage allows it
3. verify    → re-query every location for the resolved refs and for a
               content probe; count must be zero everywhere
4. report    → per-location before/after counts, and the verification
               result, written to ~/.fathom/purges.jsonl
```

If verify finds a survivor, the run **fails loudly** and names the table.
It does not retry and it does not report partial success — a partial purge
that reports success is worse than a failed one, because the user stops
looking.

### 3.4 `claim_events` — the one deliberate retention

`claim_events` is an append-only evidence log; deleting from it breaks the
integrity of every claim that references it, and its rows carry claim ids
and outcomes rather than source content.

Default: **retained**, and the purge report says so explicitly, naming the
count. This is a disclosed retention, not a silent one.

`--include-history` deletes them too, refuses unless `--force` is also
given, and states in the report that claim history integrity is now broken
for the affected claims. Someone purging a leaked credential needs this
door to exist; nobody should walk through it by accident.

### 3.5 Purge implies suppression

A purge automatically writes a suppression for the same pattern, reason
`"purged <date>"`. Otherwise the next ingest re-indexes exactly what was
just deleted, which is the most predictable possible way for this feature
to betray someone. The suppression is visible and removable like any other
— the point is that the default is safe.

### 3.6 Verbs

```
fathom purge '<glob>' --service S --reason '<why>'   # resolve + confirm + execute + verify
fathom purge '<glob>' --service S --dry-run          # resolve only
fathom purge --service S --all                       # every row for the service (RemoveService)
fathom purge history                                 # what has been purged, when, and whether verified
```

---

## 4. The UI

A **Scope** panel on `/observatory/service`, and only there — this is a
per-repository operation and a fleet-wide purge button is an invitation to
an accident.

```
┌─ Scope — conjure ─────────────────────────────────────────────┐
│                                                               │
│  Active suppressions                                          │
│   • vendor/**            expires in 3 days   [remove]         │
│     "third-party corpus, re-evaluating"                       │
│                                                               │
│  Hide paths from answers                                      │
│   [ vendor/**                        ]  [ Preview ]           │
│   Preview: 214 files, 3,981 symbols would be hidden           │
│   Reason  [                          ]                        │
│   For     ( ) until I remove it  (•) 7 days                   │
│   [ Hide these paths ]                                        │
│                                                               │
│  ── Danger zone ───────────────────────────────────────────── │
│  Permanently remove indexed content                           │
│  This deletes data. It cannot be undone.                      │
│  [ Open purge… ]                                              │
└───────────────────────────────────────────────────────────────┘
```

Purge sits behind a second click, requires the resolved file list to have
been displayed, requires a typed reason, and requires typing the service
name to confirm. Then it shows the verification result — per table, before
and after — because the user's actual question is "is it really gone?" and
the interface should answer it rather than assert it.

The mutating endpoints inherit every safety property from
`docs/design/self-healing-fleet.md` §6: loopback-only, origin-checked,
POST, registry-resolved service ids, no ambient authority.

---

## 5. Test obligations

- Suppression filters **every** surface: a table test enumerating MCP
  tools, CLI verbs and HTTP routes asserts a suppressed ref appears in
  none of them. A new surface added without being in that list fails the
  test.
- Every filtered response discloses the count and reason; a silent drop
  fails.
- Expiry is evaluated at read time — a suppression expiring while the
  daemon is stopped is inactive on the next query, with no sweep.
- `preview` counts match what `suppress` then hides, exactly.
- Purge across a fixture service removes rows from **every** location in
  §3.2, proven per-table, and the verify phase reports zero for each.
- A purge with an artificially-surviving row (injected by the test after
  execute) **fails**, names the table, and does not report success.
- Purge flushes the result cache: an answer cached before the purge is not
  served after it.
- Purge writes a suppression; a subsequent ingest does not re-surface the
  content.
- `claim_events` retention is disclosed with a count by default and deleted
  only under `--include-history --force`.
- The UI purge endpoint refuses without: a resolved-plan digest, a reason,
  and a matching service-name confirmation.

---

## 6. What this does not do

- No cross-machine propagation. Suppressions and purges are local (law 13).
- No content-based matching ("purge anything that looks like a key"). Paths
  only — a heuristic that decides what is secret will be wrong in both
  directions, and being wrong about deletion is unrecoverable.
- No undo for purge. That is what suppression is for; if you might want it
  back, do not purge.
