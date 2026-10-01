### `decide` — the decision port's log

```sh
fathom decide log [--site S] [--page N] [--per-page 50] [--readings DIR] [--call-log P] [--url U] [--json]
```

The provider and its sites (`[decide]`), the ruling log — `decisions.jsonl`
beside the resolved call log: site, kind, choice, confidence, latency, applied,
and a failed call's error — newest first, and the table of the newest
`docs/bench/readings/decision-port-calibration-*.md` under `--readings` or any
registered service's root (parsed, never recomputed: the log carries no gold
labels). A missing log or reading is named, never zero.
