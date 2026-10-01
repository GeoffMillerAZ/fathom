### `assess` — a priced second opinion on what shipped (explicit, off by default)

```sh
fathom assess [--session S | (latest stamped)] [--project DIR] [--interventions] [--dry-run] [--json] [--max-tokens 40000]
fathom assess spend [--since 720h] [--by label|day|model] [--json]
```

A model judges one session's **on-disk work products** as typed decision-port
questions (noul and choice), never its transcript. The unit lane (DO-3) sends
the commits landed in the session's beat window, the plan rows they flipped,
and the verify results recorded there. It asks `coherent`, `claims_backed`
(only when a row flipped), `tests_accompany`, and a five-level `quality`
choice. `--interventions` (IR-3) sends only the retro rows the mechanics
cannot decide: a guarded or governed file that was later committed or left
modified. Each such file gets one `complied` / `departed` / `not_applicable` /
`cannot_tell` question, with the guidance re-derived from disk and its
provenance shown, and the file's diff inside the window (an untracked new file
is diffed against `/dev/null`). Untouched files, unstamped firings, and files
with no change inside the window are named and never asked.

A live run needs four things. The first is the `assess` capability (`fathom
capabilities set assess on`, default off), which fails **closed** on a
malformed config. The second is the pre-flight token cap. The third is a
`[decide]` provider (`llm` resolves your llm profile, or `typesafe`), and the
fourth is the `assess` site; both are read from your user config and the
assessed repository's `.fathom/config.toml`, not your working directory's.
`--dry-run` needs none of them: it prints exactly what would be sent, after
DLP redaction with the extended credential catalog, the redaction count and
the estimated tokens, and spends nothing. Every run prints its own token cost,
priced through the compile meter. An unpriced model reads "cost unknown",
never zero, and `assess spend` marks any total with an unknown-cost run
(unpriced, no usage reported, or failed) as a lower bound.

Verdicts are banded: proceed means a verdict, confirm means leaning (not
counted), unsure means undecided. They are labelled **uncalibrated** until a
question set passes the adjudication bar, and they stay on
`~/.fathom/assess-ledger.jsonl`. IR-4 (pre-registered 2026-09-25) isolates the
spend: assess runs its judge with `FATHOM_CHANNELS=off` and a
`FATHOM_SPEND_LABEL`, and a labelled session-ledger row is never an arm, bet
or flywheel unit. `assess spend` totals the ledger through the `metrics
spend` aggregation. Design: `docs/design/assess-lanes.md` (ADR-046).
