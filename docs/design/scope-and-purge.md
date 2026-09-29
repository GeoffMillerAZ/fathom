# Scope and purge — reversible suppression, irreversible removal, both provable

> Phase V of `docs/vision/20-ambient-fathom.md`. Answers the owner's ask:
> *"we should have a way to turn off certain paths in the fleet. It would
> be great if that could be done from the UI. And it could be a temporary
> turn off, and there should also be a purge option in case something gets
> indexed that shouldn't be."*
>
> **Status:** design, REVISED 2026-08-08 after adversarial review found
> three ways the "provable purge" guarantee was false as first written
> (§3.5, §3.2's physical row, §3.3's two corrections). REVISED AGAIN
> 2026-08-13 after the first BUILD was held for rework over six verified
> defects (V1-V6): the verdict is a tri-state carrying the raw-bytes
> probe's outcome (§3.3), the ingest-time exclusion is SERVICE-SCOPED by
> construction (§3.5), the read chokepoint is the store's own per-call
> gate rather than an open-time decorator (§2.2), and suppression state
> resolves through an injectable path ($FATHOM_SUPPRESSIONS) with no
> init()-armed globals, so the validate gate is machine-independent.
>
> Builds on the existing `Store` primitives
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

At **one** place: the query layer — concretely, INSIDE the store
backends' own content-bearing read methods, which every surface (MCP
tools, CLI, HTTP, the observatory's counts, the cross-repo platform
store) funnels through. Two properties are load-bearing, both from the
2026-08-13 rework:

- **Per-call state.** The gate re-reads the suppression file on every
  read (stat + reload on mtime/size change), because the daemon opens
  its store once and holds it for the process lifetime — a filter
  installed at open time is inert in the only deployment that exists
  (V5). A suppression written after open filters the very next query.
- **No constructor can opt out.** The filter is part of the read
  methods, not a decorator a constructor installs, so every exported
  store constructor returns a filtering store; an audit test derives the
  constructor list from the store package's own source and fails a new
  constructor by default (V3 — the hand-listed audit missed
  MountReadOnly and eleven surfaces bypassed the filter).

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
| `file_stats` (RT-76) | one row per discovered file: path, size, language, exclusion — an oversize or unreadable file has a row and no fingerprint | delete by `(service_id, path)`, resolved from its own `path` column |
| `service_summary` (RT-76) | per-service counts and languages | delete the service's row; derived again at first use |
| `observation_calls` (RT-77) | callee keys of each observation's calls | cascades with `observations` (delete trigger); cleared wholly after a VACUUM and re-derived |
| `doc_units` (KR-1, KR-8) | one row per indexed document section: path, heading path, line range | delete by `(service_id, path)`, resolved from its own `path` column |
| `docs_fts` (KR-1, KR-8) | the section text and its FTS5 index terms | delete by `(service_id, path)`, resolved from its own `path` column (independently of `doc_units`), then `optimize`: deleting an FTS5 row leaves its terms in the index segments until a merge, and a raw-bytes probe finds them |
| `doc_links` (KR-6) | doc span to code target links | delete by the purged units' ids: it has no path column of its own |
| result cache (`internal/resultcache`) | recent tool answers | **invalidate wholly** — it is keyed by query, not by ref, so selective removal is not possible and a full flush is the only sound option |
| call log | tool name, service, outcome | retain — query text is never logged, so no content is present |
| session ledger | counts only | retain — no content |
| `~/.fathom/briefs/*.md` | rendered aggregates | retain — counts only, no content |
| `<repo>/.fathom/memories.jsonl` | anchored memories: free-form claim text that may QUOTE the purged content | delete memories whose anchors match the pattern; disclose the count (added with phase W — review finding W4: a purge that leaves a memory quoting the purged secret has not purged it) |
| **SQLite free pages and the `-wal`** | the deleted rows' raw bytes | `secure_delete=ON` for the deleting connection, then `wal_checkpoint(TRUNCATE)` and `VACUUM` (or `VACUUM INTO` a replacement — the pattern `module_pack.go:190` already uses) |

The last row was missing from the first draft and is the one that matters
most for §3.4's own named use case. The store is WAL-mode SQLite with no
`secure_delete`, no checkpoint and no VACUUM (`store/sqlite.go:70-78`).
Reproduced against this repo's own store during review: after
`PurgeSource` the query count is zero **while the canary bytes remain** in
`fleet.sqlite` (store closed) and in `fleet.sqlite-wal` (store held open —
the live daemon case). A purge of a leaked credential would report
VERIFIED while `strings fleet.sqlite` still returned the key, and the
residue would propagate into every backup taken afterwards.

The correct standard already exists in-program — agentic-kit's handoff-h3
sets `secure_delete` plus a post-migration `VACUUM` with a raw-bytes
assertion — and simply was not applied here.

The result cache row is the one that would have been missed. A purge that
leaves a cached answer containing the purged content is a purge that
failed, and because the cache is query-keyed there is no clever partial
answer: flush it.

### 3.3 Verification is part of the operation

**A purge that reports success without verifying is theatre.** So the run
has four phases and the third is not optional:

```
1. resolve   → expand the pattern against EACH LOCATION'S OWN path column,
               not a single ref list; show the exact file list and
               per-table row counts; require confirmation
2. execute   → delete across every location in §3.2, in one transaction
               where the storage allows it, on a secure_delete connection
3. verify    → a scan DERIVED INDEPENDENTLY of step 1, plus a raw-bytes
               probe of the store file; every count must be zero
4. report    → per-location before/after counts, and the verification
               result, written to ~/.fathom/purges.jsonl
```

Two corrections from review, both about proof that cannot falsify itself:

**Resolution must not come from one ref list.** `ListSourceRefs` returns
observations ∪ fingerprints only (`store/store.go:184-189`). Any row
anchored by a different key is never resolved, never deleted, and never
checked — `knowledge_cards` has no `source_ref` at all (and no per-source
delete exists; only `RemoveService`'s whole-service loop), and
`claim_links.target_ref` uses the `path#Lstart-Lend` grammar while
`anchor_text` stores verbatim cited source. Purging a service whose
`secrets/` had previously been `.fathomignore`'d would resolve zero refs,
delete nothing, and write "verified" over a knowledge card holding an LLM
summary of the file. So resolution reads `observations.source_ref`,
`fingerprints.source_ref`, `doc_summaries.source_ref`,
`knowledge_cards.file`, `typed_edges.from_file`, and `ParseCiteRef` over
`claim_links.target_ref`.

**Verification must not be derived from the deletion.** Re-querying "the
resolved refs" can only confirm the deletion it was derived from (law 14).
Step 3 re-derives its scan from each table's own path column and adds a
raw-bytes probe of the store file, so it can detect residue a SQL
re-query structurally cannot see.

If verify finds a survivor, the run **fails loudly** and names the table.
It does not retry and it does not report partial success — a partial purge
that reports success is worse than a failed one, because the user stops
looking.

**The verdict is a tri-state, and the probe's outcome is part of the type
(V1, 2026-08-13).** The first build printed VERIFIED whenever the SQL
re-scan was clean — even when the raw-bytes probe never ran (every needle
skipped as still-live under another service, an empty needle set, a read
error falling through). The rebuilt run has exactly three outcomes,
computed in one function with no boolean to shortcut through:

| Verdict | Meaning | CLI exit |
|---|---|---|
| `verified` | SQL zero everywhere AND the probe ran clean | 0 |
| `sql-zero-bytes-unchecked` | SQL zero, but the probe did not run — named reason | 2 |
| `failed` | a survivor, raw residue, or a wired derived store erroring | 1 |

`RawProbe.Ran` is earned, never assumed: needles > 0, at least one file
scanned, no read error. "Bytes unchecked" is not success and the report
says so in words.

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

### 3.5 Purge writes an INGEST-time exclusion, not just a suppression

The first draft of this section said purge writes a suppression "otherwise
the next ingest re-indexes exactly what was just deleted". **That is
unreachable by its own mechanism**, and adversarial review of this document
caught it before anything was built:

- §2.2 fixes the suppression filter at the **query layer** and explicitly
  forbids a second site.
- §3.2 deletes the `fingerprints` row, which *re-arms* extraction — the
  hash gate no longer knows the file was seen.
- `autorefresh.ensure` re-ingests on git-HEAD drift with no human action,
  and CLAUDE.md instructs every session to call `ingest`.

So the bytes come back within one commit, while `purge history` and the §4
panel keep reporting "verified" against a store where the rows have
returned. A query-layer filter can hide them; it cannot stop them being
written.

**The correction, as revised 2026-08-13: purge appends the pattern to
`~/.fathom/ignore.d/<service>`** (override the directory with
`$FATHOM_IGNORE_D`), which `connector.LoadIgnoreScopedFor` unions at
`Discover` FOR THAT SERVICE ALONE — user-scope, local, additive-only
(law 13), applied at *write* time, and SERVICE-SCOPED BY CONSTRUCTION.
The first build wrote the bare pattern into the shared `~/.fathom/ignore`,
which unions into EVERY repo: `purge --service S --all` put a bare `**`
there and destroyed the whole fleet index on the next refresh, and even a
narrow glob silently stopped that path indexing everywhere (V2, V4). A
pattern in a per-service file cannot reach another service; the writer
(`connector.AppendServiceIgnore`) refuses a fleet-wide request loudly;
and the purge report DISCLOSES the exclusion's scope in words.

**§2.2 is therefore restated as two chokepoints, not one:**

| Chokepoint | Applies at | Prevents |
|---|---|---|
| `Discover` (`~/.fathom/ignore`) | ingest / write | content entering the store |
| the query layer | read | content leaving the store |

Suppression alone still uses only the read chokepoint — it is reversible
and must not change what gets indexed. Purge uses both, because a removal
that a background refresh undoes is not a removal.

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
- **No promise of erasure beyond fathom's own store.** On a copy-on-write
  filesystem, with Time Machine or any snapshotting backup in play, older
  copies of the store file may survive a purge. §3.1's guarantee is scoped
  to the live store and its derived indexes; the report says so rather
  than implying more.
