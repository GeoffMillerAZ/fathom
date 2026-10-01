### `metrics` — observability surfaces

```sh
fathom metrics spend         --store P [--service S] [--module LAYER] [--by layer|day|model] [--since DUR | --from RFC3339 --to RFC3339] [--root D] [--json]
fathom metrics traffic       [--service S] [--by surface|tool|hour|day|caller|tool-hour] [--since DUR | --from … --to …] [--call-log P] [--url U] [--json]
fathom metrics channels      [--since DUR | --from … --to …] [--hooks-log P] [--json]
fathom metrics interventions [--since DUR | --from … --to …] [--store P] [--call-log P] [--hooks-log P] [--json]
fathom metrics series        <metric> --bucket hour|day [--since DUR] [--root D] [--json]
fathom metrics rollup        [--date YYYY-MM-DD] [--root D] [--store P] [--call-log P] [--hooks-log P] [--json]
fathom metrics rotate        [--call-log P] [--keep-days N] [--apply]
fathom metrics impact        [--call-log P] [--url U] [--json]
fathom metrics catches       [--kind K]... [--service S] [--days N] [--page N] [--per-page 20] [--call-log P] [--url U] [--json]
fathom metrics sessions      [--arm on|off] [--repo R] [--q TEXT] [--sort at|turns|first_edit|tokens|fathom_calls|catches] [--asc] [--page N] [--per-page 50] [--json]
fathom metrics sessions      --id SESSION [--channel C]... [--page N] [--per-page 500] [--json]
fathom metrics load          [--call-log P] [--url U] [--json]
fathom metrics instrument    [--log PATH] [--window DUR] | --adoption [...]   # SR-31: same as `fathom instrument`
fathom metrics flywheel      [--service S] [--url U] [--json]                # SR-31: same as `fathom flywheel`
fathom metrics retro         [--session S] [--project DIR] [--write] [--json] # SR-31: same as `fathom retro`
fathom metrics rate          <1-5> [--note "..."] [--session S] [--repo R] | --list  # SR-31: same as `fathom rate`
fathom metrics arm           off --for D --sessions N [--note ...] | on      # SR-31: same as `fathom arm`
```

Surface reckoning SR-31 (`docs/design/surface-reckoning.md` §4): `instrument`,
`flywheel`, `retro`, `rate` and `arm` are subverbs of `metrics` now — the
observability verb was already their natural home. The five top-level verbs
stay fully dispatched (hidden from `fathom help` only) for one release, so
existing hooks and skills keep working; prefer the `metrics` form in new
scripts.

Design: `docs/design/observability-surfaces.md`. `traffic --by day` groups by UTC
day and `--by caller` by caller class — agent, edit hook, gate, refresh, plumbing or
unattributed, from each row's origin and surface — so plumbing never passes for
adoption; `--by tool-hour` groups by UTC hour and tool (`"<hour> <tool>"`), the Operate
page's per-tool per-hour p95 beside its Tenancy panel. `impact`, `catches` and `sessions` print the console's Evidence projections
(vision 35, `internal/console`): the four questions with each arm as a range and "not
instrumented" instead of a zero; one timeline of verify refusals (the verify ledger),
conserve losses and integrity catches (`fathom review` rows in the call log) and guard
refusals (the edit hook's `denied` beats, read from the hooks log and its rotated
generation `hooks.jsonl.1`), each kind absent with its reason when its instrument
recorded nothing to count from in the filtered scope (`--service`, `--days`) and an
error when its source exists and cannot be read; and the session table (one row per
session) or one session's replay, never more than 2,000 events per response. `load`
prints the Operate page's load: calls, p50, p95 and error rate per UTC day per surface
(a day with no row absent, never zero) and the tools by caller class. With no call log,
or an unreadable one, `traffic --json` carries `state` (`absent` or `error`) and
`reason` in place of every count, and `sessions` / `load` print their counts as absent
or error values — never a zero.
