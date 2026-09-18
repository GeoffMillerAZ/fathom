# The findings loop: an external review kit's findings, served at the edit

Design: `docs/vision/30-findings-loop.md` (ADR-040). Plan:
`docs/plans/findings-loop.md`. This page is the adopter's walkthrough — what
the loop is, how it reaches a session, and what to do when it doesn't.

## What this is, in one paragraph

A review finding and a governing rule are the same shape of fact: a claim
about a place in the code, made by a named producer with a stated method,
that must be re-checked when that place moves. fathom already stores,
drift-tracks, tiers and serves that shape for its OWN findings
(`docs/use-cases/change-review.md`'s "Review findings as compiled claims").
The findings loop is the other half: an **external** reviewer's
findings — a review kit's own portable journal — entering fathom through a
governed import that preserves who found what, at which revision, at which
confidence; the open findings on a file are served to the agent *before* it
edits that file, beside the governing rules; the next review closes them by
evidence, never by silence; and a finding that keeps recurring becomes a
candidate rule a human lifts into `CLAUDE.md` or `guard.cue`. fathom is the
review kit's first projection adapter, never its only home, and never the
organizational record — see "What fathom never does" below before you trust
this loop with anything you can't afford to lose.

## The two variants

**CI.** A GitHub Action step runs `leadsman review` (the review kit's own
CLI — a peer process, never a fathom dependency, ADR-040 §2) and commits its
projection outbox back to the branch:

```sh
leadsman review --target fathom --outbox .reviewkit/outbox/projection.json
git add .reviewkit/outbox/projection.json && git commit -m "chore(reviewkit): update findings outbox" && git push
```

See `docs/adoption/github-action.md` §7 for the full workflow step. This
gets the review kit's findings OUT of the CI runner and into the repo; it
does not by itself import anything.

**Local.** Run the review kit directly, then import — no CI required:

```sh
leadsman review --target fathom --outbox .reviewkit/outbox/projection.json
fathom findings import --store PATH --service S \
  --subject org/repo/component --producer reviewkit.system-review \
  --file .reviewkit/outbox/projection.json --json
```

Or skip the manual import: once `.fathom/config.toml`'s `[findings]
outbox` names the same file (`docs/configuration.md`), starting a session
imports a changed outbox automatically (see below) — commit the outbox (CI
variant) OR leave it as a local, gitignored file a developer refreshes
before a session (local variant); either reaches the next session the same
way.

## What the agent sees

1. **The session brief** (`docs/configuration.md`'s findings-loop section):
   when `[findings] outbox` is set and its digest has changed since the
   last import (a cursor kept under fathom's state dir, one attempt per
   session), the SessionStart/SubagentStart hook runs the import and the
   brief carries the receipt or the failure reason:
   `findings outbox: imported 3 (0 duplicate, 1 stale)` or
   `findings outbox: import failed — subject mismatch (…)`. Never silent.
2. **Before an edit**, the pre-edit hook renders a bounded block right
   after the governing rules and the enforcement verdict:
   `open findings on this file (N, showing k)` — for each: class,
   producer, rule id, the one-line rule, the remedy, `path#Lstart-Lend`,
   status, and whether the code has moved since the finding was observed
   (`anchor_current`). Capped at 3 findings and 1,200 bytes; superseded and
   refuted findings never appear; `omitted: N-k` is always rendered when
   the cap trims the list.
3. **After an edit**, the same block renders again ONLY when the edited
   region overlaps a finding's cited lines (`finding_touched`) — an edit
   somewhere else in a large file gets no repeated noise.
4. **In the prompt hook**, one line: `N open findings across the files
   this prompt names` — inside the same byte deadline every other
   prompt-hook line already respects (ADR-036).

Findings are **advisory** at every one of these points. Only a rule a human
promotes into `CLAUDE.md`/`.fathom/guard.cue` (via `fathom findings
candidates`, then a real edit to that file) ever enforces — a finding
served is a fact about what was observed, never a gate.

## The receipt

`fathom findings import`'s response is the audit trail — read it, don't
just check its exit code:

```json
{
  "schema_version": 1,
  "delivered": 3,
  "duplicate": 1,
  "stale": 0,
  "unclassified": 0,
  "rejected": [
    {"index": 4, "finding_id": "…", "reason": "conflicting-replay"}
  ],
  "truncated": false,
  "cursors": [{"producer": "reviewkit.system-review", "latest_source_version": 7}],
  "dry_run": false
}
```

- `delivered` — events that landed as a fresh or refreshed claim.
- `duplicate` — the same `(producer, sourceId, sourceVersion, payloadDigest)`
  seen before; nothing was written.
- `stale` — an event whose `sourceVersion` is older than the recorded
  cursor; counted, not applied.
- `unclassified` — decoded but not mapped to any known lifecycle transition
  (a forward-compat guard against a future event kind this fathom version
  does not know).
- `rejected` — per-event rejections, each with the batch index and the
  reason (see Troubleshooting below); the rest of the batch still applies.
- `truncated` — the batch itself was cut short (a size/count budget), named
  rather than silently dropping trailing events.
- Exit code **1** on ANY `rejected` entry, even when other events in the
  same batch delivered fine — a receipt with rejections is never a clean
  exit.
- `--dry-run` runs every validation and returns the SAME receipt shape
  without writing anything — use it before trusting a new producer or a
  freshly wired subject.

## The honesty rules (vision 30 §5) — read before you trust a number

- **A finding served is not a finding verified.** The block names who
  observed it and at which revision; `anchor_current` says whether the code
  has moved since. Neither fathom nor the agent re-derives the finding —
  it is exactly as reliable as its producer.
- **An import receipt is not organizational admission.** A refusal is not
  "no findings exist"; an outbox that failed to sync is reported in the
  brief with the reason, never silently skipped.
- **fathom does not become the organizational record.** Retention,
  revocation, restore drills, access isolation across trust boundaries, and
  named operational owners remain the review kit's own (ADR-0098). The
  receipts and cursors here are what would let that review happen — they
  are not a claim that it has.
- **No model sits in this path.** Import, serve, `candidates`, and
  `review-key` are all deterministic and replayable from the review kit's
  own archive.
- **Tier never rises on import.** A `model` producer caps at `sme-stated`;
  a `deterministic` producer caps at `pre-scan` unless its cap is
  explicitly `verified` with a stated oracle; a `human` producer caps at
  `sme-stated`. The served tier is always `min(event tier, producer cap)`.

## What fathom never does

fathom never approves, disputes, tailors, or closes a review-kit finding
on its own authority. A review-kit acceptance is never translated into a
fathom `refute`; `fathom claim refute|corroborate` stays a human verdict
recorded against fathom's own claim, never a proxy for a review-kit
disposition. Authority never flows backwards: a projected finding cannot
re-enter the review kit as independent evidence, and a fathom-native
finding the review kit imported into ITS journal cannot be projected back
into fathom — the importer refuses any producer whose declared `origin` is
`fathom` (see Troubleshooting: circular producer).

## Troubleshooting

**No subject provisioned.** `fathom findings import` refuses before it
reads the store at all: `.fathom/config.toml`'s `[findings] subject` must
equal the event's own subject, exactly. Fix: uncomment and set `subject`
(`docs/configuration.md`) to the review kit's own `org/repo/component`
string for this repository — check the review kit's own subject
provisioning if you don't know it. No mapping means no import, by design;
fathom never guesses which subject an unlabeled outbox belongs to.

**Unknown producer.** The event names a producer id not in the catalog
(`schema/semantic/producers.cue`) or not in this repo's own `[findings]
producers` allow-list. Fix: confirm the producer id is spelled exactly as
the catalog declares it, and add it to `producers` in `.fathom/
config.toml` if it is a trusted source you have not allow-listed yet. An
unlisted producer's events are refused, never silently accepted at a lower
tier.

**Circular producer.** The event names a producer whose declared `origin`
is `fathom` — i.e., the review kit is trying to hand back a finding fathom
itself originated. Refused with `circular: native finding already exists
as <claim_id>`, by design (the authority-never-flows-backwards rule
above). This is not a bug to work around; it means the review kit's own
pipeline picked up a fathom-native finding somewhere it shouldn't have.

**Stale.** The event's `sourceVersion` is older than what this repo's
cursor already recorded for that `(producer, sourceId)`. Counted in the
receipt, never applied, never an error — this is the expected shape of
re-importing an outbox that has already been (partially) imported.

**Duplicate.** The exact same `(producer, sourceId, sourceVersion,
payloadDigest)` as a prior import. A no-op, counted separately from
`stale` so you can tell "already current" from "already superseded" at a
glance.

**Conflicting replay.** The SAME `sourceVersion` as the recorded cursor,
but a DIFFERENT `payloadDigest` — the review kit's own archive and this
repo's cursor disagree about what that version's content was. Rejected
(exit 1), never silently overwritten: re-derive which payload is actually
current (the review kit's own archive is authoritative — ADR-0098) and
re-import from there. This is the shape a hand-edited or corrupted outbox
file produces; it should never happen from an untouched `leadsman review`
output.

**A disagreement (not a rejection).** Two producers reporting opposite
outcomes for the same rule and anchor (`fail` from one, `pass` from
another) are BOTH served, each labeled with its own producer — fathom
never resolves the disagreement for you. `fathom findings status` counts
these under `disagreements`; read both and decide, the same way you would
read two human reviewers who disagree.

## See also

- `docs/configuration.md` — the `[findings]` config section and the
  session-sync mechanism.
- `docs/use-cases/change-review.md` — "The findings loop" section, and how
  this relates to fathom's OWN native findings (`fathom report`).
- `docs/layers.md` — the `findings` layer's external-producer section
  (tier caps, the `external-import/v1` reconciliation domain).
- `docs/usage.md` — the `findings import`/`findings candidates` CLI
  reference.
- `.claude/skills/fathom-findings/SKILL.md` — the in-session skill covering
  import, inspect, and refute/corroborate.
