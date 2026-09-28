# Start measuring: the research-flywheel runbook

A concrete, in-order sequence for turning fathom on in one of your own
projects and getting an honest, evidence-backed answer to "is this actually
helping" within a week — not a screenshot of raw logs, not a guess from
memory.

This doc is the **runbook** — the sequence a developer follows. For what
each command measures, why, and the exact honesty guarantees behind it, see
[`docs/use-cases/research-flywheel.md`](../use-cases/research-flywheel.md)
(the tool-reference doc this one points to rather than duplicates) and
[`docs/design/research-flywheel.md`](../design/research-flywheel.md) (the
full design rationale, for when "why does it say `not measurable`" needs an
answer deeper than this doc gives).

---

## Step 0 — Wire fathom into the repo

If fathom isn't indexing this repo yet, follow
[`docs/adoption/claude-code.md`](claude-code.md) (or
[`docs/adoption/codex.md`](codex.md) for the Codex harness) end to end:
build, run a daemon, register the MCP surface, wire the hook stack. The
research flywheel measures fathom's *own* instrumentation — the call log,
the hooks beat log, the observations file — so there is nothing to measure
until fathom is actually wired and firing.

```sh
fathom init          # in the repo you're adopting fathom into
```

## Step 1 — Verify the wiring, don't assume it

Before you spend a week trusting fathom to instrument itself, confirm the
whole stack is actually live — MCP registration, daemon reachability,
binary freshness, the hook stack installed *and* firing, the CLAUDE.md
stanza, obligations serving:

```sh
fathom doctor
```

A green `fathom doctor` is the precondition for everything below: if the
edit-hook isn't firing, `research report`'s injection-coverage number will
honestly report a low or zero rate — not because fathom isn't helping, but
because it was never wired to try. Fix what `doctor` flags before starting
the clock. (The `fathom-doctor` skill covers reading its output in more
depth than this runbook needs to.) `fathom flywheel` is the companion
gauge for the loops themselves — each feedback loop CLOSED/OPEN/IDLE with
its evidence and remedy — run it once here and again in Step 6.

## Step 2 — Work normally for a week

Do nothing special. Use Claude Code (or your harness of choice) on this repo
the way you normally would — let the edit-hook inject obligations, let
`verify_plan`/`review` run where your workflow already calls for them, file
a `fathom observe` entry whenever something surprises you (a wrong answer, a
recall gap, a confusing tool contract — the `fathom-feedback` skill exists
for exactly this and should be used **the moment** something's off, not
retroactively at the end of the week):

```sh
fathom observe --symptom "…" [--tool T] [--expected …] [--actual …] [--repro …]   # right when you notice it
```

Optionally rate each session on the way out — `fathom rate <1-5> [--note
"…"]` (*experimental*: the human OUTCOME half of the A/B criterion, so
"fewer tokens" is never counted as a win when less got done).

The point of this step is that the evidence the flywheel reports in Step 3
has to come from real usage, not a staged session — a research report run
five minutes after `fathom init` will honestly say there's nothing to
report yet, which is the correct answer, not a bug.

## Step 3 — Collect: run the report

```sh
fathom research report --since 168h   # 168h = 7 days
```

This aggregates fathom's own pre-existing instrumentation — call-log
traffic, injection coverage (obligations actually reaching an edit),
verify-gate catches, staleness signal, observation counts, and any
context-bench artifact on file — into one deterministic snapshot at
`workspace/artifacts/research/report.{md,json}`. It reads **only local
files**; it never queries the live daemon or opens a store, so there is
nothing partial about a fathom-unreachable degradation — every number
either comes from a file on disk or is honestly absent.

## Step 4 — Read what the report actually says

This is the step most worth doing deliberately, not skimming. The report's
whole design point (`docs/design/research-flywheel.md` §2's "never a
fabricated zero" rule) is that every field is either a real, computed
number or a named reason it can't be computed — read both kinds:

- **A real number is citable.** If injection coverage reads 90%+ and the
  verify gate caught something this week, that's a fact you can put in a
  PR description or a standup update with a committed artifact behind it.
- **A `"not measurable: <reason>"` line is not a bug to work around** — it
  names a genuine gap in what fathom can see about itself today. Don't
  paper over it with a claim the report itself refuses to make.
- **"discovery_share_percent" is real but CORRELATIONAL, never causal.**
  Phase P's counting-only PreToolUse observer gives fathom-vs-grep/glob/
  read/shell-search a real denominator for the first time
  (`docs/design/production-tokenomics.md` §4) — but a rising share is not
  proof fathom is being *chosen over* the alternative until the
  pre-registered A/B protocol (design §5) actually runs. Say "correlated
  with," never "caused," until then.
- **"what fathom SPENT" is a cost figure, never a savings figure** (law
  12). Cite the per-channel token *estimate* if you want, but never phrase
  it as money or tokens saved — that claim needs the same A/B. The A/B
  counterfactual is *experimental and still collecting*: the OFF arm ran,
  the ON arm is live, and ≥20 admissible units per arm are needed before
  any savings number is admissible — until then the report will not print
  one, and neither should you.
- **No context-bench artifact found is a real, common, honest state** — it
  means a tokenomics claim ("fathom saves N% tokens") isn't backed by *this
  repo's own evidence* yet, even if it's true elsewhere. Say so if you're
  reporting out, per the `fathom-impact` skill's rule: report what was
  measured, not what you believe.

## Step 5 — Share (optional): build the sanitized bundle

If you want to hand evidence to someone outside this machine — a teammate,
a write-up, anyone who shouldn't see raw repo/service names or free-text
observation notes — build the sanitized twin. Review it before deciding
anything:

```sh
fathom research bundle --since 168h --dry-run   # preview only, writes nothing
```

`--dry-run` prints the exact JSON that would be written and transmits
nothing — read it. When you're satisfied it says what you want it to say:

```sh
fathom research bundle --since 168h
```

This writes two plain local files under
`workspace/artifacts/research/bundle/` (`bundle.json`, `bundle.md`) —
aggregate counts, rates, tool-name frequencies, and verdict distributions
only. Service identities appear as salted hashes, never plaintext; a
closed CUE schema (`schema/research-bundle/schema.cue`) ships with the
binary so a recipient can validate what you send them independently,
without trusting your binary. Nothing is transmitted by this command —
sharing, if you choose to, is an entirely manual step you take afterward.

**Never share the parent `workspace/artifacts/research/` directory.** Share
only the two files under `workspace/artifacts/research/bundle/` (or that
`bundle/` subdirectory in isolation). The parent directory also holds
`report.md` from Step 3, which is **deliberately unsanitized** — it carries
plaintext service names and the local hooks-log path (which embeds your
username on a default `~/.fathom/hooks.jsonl`). The CLI repeats this
warning after every non-`--dry-run` bundle write; this runbook repeats it a
second time because it is the one step in this whole sequence where a
careless `cp -r` or a Finder drag of the wrong folder does real harm.

## Step 6 — Close the loop: file what you found

If Step 4 surfaced a gap — a tool that never got called, an injection rate
lower than expected, a verify-gate catch that should have been three but was
one — file it:

```sh
fathom observe --symptom "…"
fathom flywheel          # which loops are now CLOSED, and what each OPEN one still needs
```

and, if you also found and fixed something in fathom itself while adopting
it, record the fix the way this repo's own dogfood discipline requires
(`docs/guides/fleet-index.md` carries the discipline; the narrative log
with reproductions is `docs/dogfood-observations.md`) — the flywheel's
value compounds only if what it surfaces actually gets acted on, not just
reported.

---

## Step 7 — Observability surfaces (phase Q): spend, traffic, channels, interventions

`fathom research report`/`bundle` (Steps 3-5) are the narrative snapshot;
`fathom metrics {spend,traffic,channels,interventions,series}`
(`docs/design/observability-surfaces.md`) is the queryable substrate
underneath it — grouped, time-ranged, and JSON-able for a dashboard or
script rather than a one-shot report. Neither duplicates the other's
logic: `research report` and the `metrics` verbs share the same underlying
aggregators (`internal/metrics`).

```sh
fathom metrics spend --store workspace/db/fathom.sqlite --service <svc> --since 168h --by layer
fathom metrics traffic --by tool --since 24h
fathom metrics channels --since 24h
fathom metrics interventions --store workspace/db/fathom.sqlite --since 168h
```

See [`docs/use-cases/metrics-surfaces.md`](../use-cases/metrics-surfaces.md)
for the full verb reference, `--json` shapes, and worked examples.

The daemon renders the same substrate as pages: `GET /observatory` (fleet
— a 30-day grade distribution; *experimental*: days without a persisted
snapshot render Absent, so today only until daily snapshots persist),
`/observatory/service?id=<svc>` (per-dimension 30-day series — queried,
discovering, preventing, gating — from real call-log rows; a day with no
rows renders Absent with its reason, never a fabricated 0, and sparklines
never draw across an absent day), `/observatory/impact`, and
`/observatory/brief` (`?format=json|md` where offered). Every tile that
cannot read the call log or store renders Absent, never 0 — the same rule
as the report.

**Turning `FATHOM_METRICS=full` on for a tuning window, then off again.**
The default tier, `basic`, already captures everything phase P specifies
plus O19's compile rejection-reason capture — no code in this phase yet
branches on `full` specifically (payload sampling and per-section
diagnostics beyond what's already unconditional are deferred, named with a
trigger, in `docs/plans/expert-suite-q.md`'s Deferred section). If you do
turn `full` on for a deliberate tuning window:

```sh
export FATHOM_METRICS=full
# ... work normally for the tuning window ...
fathom doctor   # metrics_tier check confirms the tier that was actually active
unset FATHOM_METRICS   # or: export FATHOM_METRICS=basic
```

**No tier of this design spends model tokens to measure model tokens**
(design §3) — `full` costs disk and (once payload sampling ships) privacy
surface, never LLM spend. Turn it off once the tuning window is over; there
is no reason to run `full` continuously today.

**`/statusz` is loopback-only.** Hosted-modules mode (`--modules-token-env`) and
the bearer gate that covered `/statusz` in it were removed by SR-17 (surface
reckoning F-68); like every route (`/healthz`, `/metrics`, `/events`), `/statusz`
answers only a loopback `Host`.

---

## Recap: the whole sequence

```sh
fathom init                              # Step 0 — wire it in
fathom doctor && fathom flywheel         # Step 1 — verify, don't assume
# ... work normally for a week, `fathom observe --symptom "…"` as friction happens ...
fathom research report --since 168h      # Step 3 — collect
# read report.md, decide what's citable and what isn't (Step 4)
fathom research bundle --since 168h --dry-run   # Step 5a — preview
fathom research bundle --since 168h             # Step 5b — write, then choose to share
fathom observe --symptom "…"             # Step 6 — close the loop
```

## Maintained

This doc covers the adoption sequence for `fathom research report` /
`fathom research bundle`, `fathom flywheel`, `fathom rate`, `fathom
observe --symptom`, the observatory pages (fleet, service, impact, brief),
and (Step 7) the `fathom metrics` verb family's
tiering/`/statusz`-gating adoption notes. It intentionally does not
duplicate what each command measures or how the sanitization/honesty
guarantees work — see
[`docs/use-cases/research-flywheel.md`](../use-cases/research-flywheel.md)
and [`docs/use-cases/metrics-surfaces.md`](../use-cases/metrics-surfaces.md)
for that. Update this doc when the sequence itself changes (a new step, a
reordered one, a changed flag); update the use-case docs when a command's
flags, output shape, or sanitization/honesty guarantees change.
