# Self-healing fleet — fathom does the wiring, and can undo it

> Phase U of `docs/vision/20-ambient-fathom.md`. Answers the owner's ask:
> *"instead of just advising you on commands on how to update it a bit per
> project, maybe it can execute what needs done on your behalf. We want to
> make sure we're not destroying other configs people may have by injecting
> the configs we need."*
>
> **Status:** design. Builds on the existing `fathom fleet reconcile`
> (`cmd/fathom/reconcile.go`: `analyzeWiring`, `repointStaleCommands`) and
> the existing `<!-- fathom:begin -->` / `<!-- fathom:end -->` stanza
> markers in `cmd/fathom/init.go`.

---

## 1. The failure this fixes

The observatory has, for weeks, correctly reported that 9 of 17
repositories are graded below A, and correctly printed
`fathom fleet reconcile --apply` beside each one. Nobody ran it.

That is not a discipline problem. **Advice that requires a human to switch
context, find a terminal, and type a command is advice that does not
happen** — and every unwired repository is one whose sessions sit outside
the denominator of every impact number fathom reports. The measurement
program is being degraded by an unclicked button.

The fix is a button. The design work is entirely in making the button safe.

---

## 2. Law 15, and why it is not a judgement call

> **Law 15.** When fathom modifies a file it did not create, it writes only
> inside a region it marked as its own. If that region has drifted from
> what fathom last wrote, reconciliation **stops and reports the drift**;
> it never merges, never overwrites, and never "helpfully" resolves. Every
> write produces a receipt sufficient to undo it.

Fathom's writes land in `CLAUDE.md`, `AGENTS.md`, `lefthook.yml`,
`.claude/settings.json` — files that carry a human's own configuration and
*other tools'* configuration alongside fathom's.

The asymmetry is the whole argument:

- Cost of a **false stop**: a human reads a diff and clicks again. Seconds.
- Cost of a **false merge**: someone's hand-tuned hook configuration is
  silently destroyed, possibly across seventeen repositories in one
  action, possibly discovered weeks later.

Those are not comparable, so no cleverness is warranted. Stop and report.

---

## 3. The ownership model

Three tiers, and fathom's authority differs at each.

### Tier 1 — files fathom created (full ownership)

`.fathom/config.toml`, `.fathom/layers/**`, `.fathom/guard.cue`,
`dist/skills/fathom-*`. Fathom may rewrite these wholly. Even here it
writes a receipt, because "fathom created it" is a claim that can be wrong
after a year of history.

### Tier 2 — marked regions in shared files (bounded ownership)

`CLAUDE.md`, `AGENTS.md`, `README.md`. Fathom owns exactly the bytes
between its markers:

```
<!-- fathom:begin v3 sha256:8c1f… -->
…fathom's stanza…
<!-- fathom:end -->
```

Two additions to the existing marker format, both load-bearing:

- **`v3`** — the template version that produced this region, so
  reconciliation knows whether a rewrite is an upgrade or a no-op.
- **`sha256:…`** — the digest of the region's body **as fathom last wrote
  it**. On reconcile, fathom re-hashes the current body. A mismatch means a
  human edited inside fathom's region, and law 15 applies: **stop, report,
  show both versions, offer "keep mine" / "take fathom's" as an explicit
  human choice.** Never a merge.

Text outside the markers is not read, not parsed, not touched.

### Tier 3 — structured files owned by other tools (surgical, keyed)

`lefthook.yml`, `.claude/settings.json`, `.gitignore`. There is nowhere to
put a comment marker in JSON, and a YAML comment is too fragile to trust
with someone's CI configuration. So ownership is **keyed, and the key is
recorded out of band**:

- Fathom adds only entries whose identity it can state exactly — a
  lefthook command named `fathom-verify`, a hook entry whose command
  contains the fathom shim path, a `.gitignore` line inside a
  fathom-labelled block.
- The set of keys fathom has added to each file is recorded in
  `.fathom/owned.json`, written at the same instant as the edit.
- On reconcile, fathom reads `.fathom/owned.json` and touches **only**
  those keys. A key present in the file but absent from `owned.json` is
  someone else's and is invisible to fathom.
- A key present in `owned.json` but whose current value differs from what
  fathom wrote is **drift**: stop and report.

`.fathom/owned.json` is the receipt ledger and the ownership map at once.
Losing it does not cause damage; it causes fathom to consider itself the
owner of nothing, and to propose additions that already exist — which
surfaces as a no-op diff, not as a duplicate. (This is exactly the failure
mode of the `fathom init` hook duplication incident, and this file is its
structural fix.)

---

## 4. The three-phase apply

Every mutation, whether from the CLI or the UI, goes through the same
pipeline. The UI has no privileged path — law 10.

```
plan   →  a typed diff: per file, per region/key, before → after,
          plus a per-item verdict: apply | skip (already current) |
          BLOCKED (drift, with both versions attached)

apply  →  executes only items with verdict `apply`; a single BLOCKED item
          does NOT block the others — it is reported and skipped

receipt → appended to .fathom/receipts.jsonl: timestamp, fathom version,
          template versions, per-file before-digest and after-digest, and
          the full prior content of every region it replaced
```

**`plan` is always computed, even when the caller asked to apply.** There
is no code path that mutates without first producing a diff. This is what
makes the UI's "show me first" and the CLI's `--apply` the same operation
with a different number of steps.

### Blocked is a first-class outcome

A `BLOCKED` item is not an error and does not fail the run. It renders as:

> `CLAUDE.md` — fathom's stanza was edited by hand (region digest does not
> match what fathom wrote on 2026-07-14). Fathom will not merge. Your
> version and fathom's are shown below; choose one.

Choosing "take fathom's" writes a receipt containing the human's version,
so the choice is still reversible.

---

## 5. Undo

```
fathom fleet receipts                 # what fathom has written, newest first
fathom fleet undo <receipt-id>        # restore every file in that receipt
fathom fleet undo --last              # the most recent apply
```

Undo is a *restore from receipt*, not an inverse operation — the receipt
carries the full prior content of every region touched, so undo does not
need to reason about what the change meant. It refuses if a file has
changed since the receipt was written (drift again, same rule), reporting
which file and why.

Receipts are append-only, local, and never leave the machine (law 13). They
are pruned by count, not by age, so an undo is always available for the
last N applies regardless of how long ago they happened.

---

## 6. The UI

On `/observatory` and `/observatory/service`, each remedy currently renders
as a command string. It becomes:

```
┌─ planning · grade C ──────────────────────────────────────┐
│  Missing: CLAUDE.md stanza, verify pre-push hook           │
│                                                            │
│  [ Preview changes ]                                       │
└────────────────────────────────────────────────────────────┘
```

`Preview changes` runs `plan` and renders the typed diff — additions in
full, drift blocks side by side. Only then does an `Apply` button appear,
and it applies exactly what was previewed (the plan is passed back by
digest; if the repository changed between preview and apply, the apply is
refused and the preview re-run).

**Fleet-level apply exists but is deliberately awkward:** it requires
previewing every repository's diff first, and it applies them one at a
time with per-repository receipts, so a bad outcome is bounded to one
repository and undoable individually.

### Safety properties of the UI path

- **Loopback only.** The mutating endpoints bind to loopback exclusively
  and refuse a request whose `Host` header is not a loopback name. Law 13
  is unchanged: nothing here is reachable off the machine.
- **POST with an origin check.** A page served by the daemon can call it; a
  random page in another browser tab cannot. `Sec-Fetch-Site` plus an
  `Origin` allowlist, and no cookies anywhere, so there is no ambient
  authority to abuse.
- **Only registered repositories.** The apply endpoint takes a service id,
  resolves it through the registry, and refuses any path not already in
  the fleet. There is no way to point it at an arbitrary directory.
- **Never automatic.** No timer, no session hook, no "while I'm here" —
  vision 20 §4 X1. The button is the consent.

---

## 7. Time-to-remedy: the instrument that proves this worked

This phase is only worth building if it changes behaviour, so it ships with
its own measurement, pre-registered here before the code exists:

- **Instrument:** for each repository, the interval between fathom first
  reporting a wiring gap and that gap being closed.
- **Baseline (measured 2026-08-08, before this phase):** 9 of 17
  repositories below grade A, remedy printed continuously since at least
  2026-07-30. Time-to-remedy is unbounded for all 9.
- **Success:** median time-to-remedy under one week, and grade-A count
  above 14 of 17, within 30 days of this shipping.
- **Kill:** if the buttons ship and grade-A count has not moved after 30
  days, the problem was never friction, and the remaining work — deeper
  dashboards, richer advice — should be cancelled rather than continued.

That last line matters. The premise of this entire phase is that friction
is the blocker. If the premise is wrong, better to learn it from a cheap
button than from a year of dashboards.

---

## 8. Test obligations

- A hand-edited fathom region is **never** merged; the run reports BLOCKED
  and the file on disk is byte-identical afterwards.
- Content outside markers survives an apply byte-for-byte, including
  trailing whitespace and line endings.
- A `lefthook.yml` carrying three non-fathom commands keeps all three, in
  order, across an apply that adds one fathom command.
- A `.claude/settings.json` with unrelated hooks keeps them; fathom's own
  entry is added exactly once even when apply runs twice (the duplication
  regression).
- `undo --last` restores every file in the receipt to its exact prior
  bytes, and refuses when any of them has changed since.
- The apply endpoint refuses: a non-loopback `Host`, a cross-site `Origin`,
  a service id not in the registry, and a plan digest that no longer
  matches the repository.
- A fleet-wide apply across a fixture fleet where one repository is drifted
  applies the others and reports exactly one BLOCKED.

---

## 9. Explicitly out of scope

- Any automatic or scheduled application (vision 20 X1).
- Reconciling a repository that is not a registered fathom service.
- Merging, three-way or otherwise. Law 15.
- Anything reachable off loopback. Law 13.
