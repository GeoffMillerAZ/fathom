### `fleet` — report, reconcile, receipts, undo (law 15)

```sh
fathom fleet report    [--json | --out ~/.fathom/fleet] [--url U]           # wiring/freshness across every indexed repo
fathom fleet reconcile [--repo R]... [--apply [--rebaseline]] [--url U]      # dry-run by default; --apply: shim repoint + additive template merge + stanza + baseline digests
fathom fleet realization --service S [--json] [--url U]                     # the realization ladder: wired → fresh → queried → preventing → gating, the first unmet rung with its remedy
fathom fleet receipts  [--repo R] [--limit 20] [--json]                      # every write fathom makes in a repo is receipted…
fathom fleet undo      [--last | <receipt-id>] [--repo R] [--json]           # …and reversible
fathom fleet remedy    [--dir ~/.fathom/fleet] [--json]                      # time-to-remedy per repository from the report snapshots (SH-9)
```

`remedy` grades every dated `report-<date>.json` snapshot with the fleet page's
wiring grade: a gap opens at the first snapshot grading a repository below A and
closes at the first later one grading it A again. A gap already in the first
snapshot opened on or before it (its interval is a lower bound); a snapshot that
could not read the call log grades that repository unknown, never unused. The
2026-08-08 baseline (9 of 17 below A, unbounded) is recorded beside the median.

`reconcile --apply` writes stanza regions (the `AGENTS.md` stanza and a root CLAUDE
file's `@AGENTS.md` import, moving an old `CLAUDE.md` stanza — ADR-059) and hooks only through
`internal/wiring` — the same plan → apply → receipt path as `init`, with the
same law-15 refusal for a hand-edited tracked region (blocked and named;
other files still applied). The same ledger (`.fathom/owned.json`) baselines
each harness descriptor's MCP server entry (argv and env/header NAMES, never
values) and every `fathom:owned` skill file: dry-run prints a `DRIFT:` line by
kind and path for a changed entry and never merges it; `--apply` records a
baseline for entries that have none, and `--apply --rebaseline` re-records a
drifted one when the change is yours. Dry-run distinguishes `stale` from `up to date
(will be re-marked with a digest on apply; no prose changes)` for a v1
region awaiting its digest.
