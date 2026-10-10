### `resources` — what the daemon takes from this machine

```sh
fathom resources [--json] [--since 24h] [--days N] [--id <state-dir id>] [--url <base>]
```

One report, rendered from files so it works when the daemon is down, wedged or
crash-looping (RT-48, design §13.2): the budget and marks the daemon runs under (from
its ring header, else config), its phys_footprint and its children's now and over the
window (peak with its level, an hourly sparkline), lifecycle (starts, unclean and
over-budget exits, counted exits in 30 min, the kill switch — a ledger with no start rows
is disclosed as not recording starts and unclean exits, never "0"), quarantined services, host
pressure, swap and volume free space, in-flight work by class with the oldest call,
the hooks' shared breaker, scratch daemons (the `daemon ps` census), disk (with RT-162's
24 h disk-write judgment while the daemon answers; the ring keeps only the process's
bytes-written counter and says the judgment is not kept), and the last diagnostic capture. When the daemon answers, the figures come from its `/statusz`
tenancy block (the same projection); when it does not, the report says so and shows the
ring's last sample with its age — never nothing. A ring sample older than 5 min (the
daemon stopped sampling) is shown with its age but never judged as the daemon's state
now. Every line names its instrument.

`--json` also carries `footprint.hourly` (the window's peak footprint per hour, oldest
first, `-1` for an hour with no sample — never `0`) and `ladder` (each transition in the
window with its time and level). The console's Operate page renders this report as its
**Tenancy** panel (RT-121): the budget and its marks, the footprint per hour as a share
of the budget (an hour with no sample hatched), the ladder's transitions on a timeline,
lifecycle, in-flight work by class, the scratch census and the disk classes; the page's
JSON carries the verb's output as the `fathom resources --json` section.

The ring is `~/.fathom/daemon/<id>/resources.ring`: one fixed-size file (271,520 B)
the governor writes in place — a sample a minute for 24 h plus every ladder transition,
each slot checksummed (a torn slot is skipped and counted). A scratch daemon's ring is
removed with its record.

`--since` reads the ring, which holds 24 h: a longer window is the ring's whole 24 h, not
more. For history use `--days N` (1..8, otherwise exit 2): it adds the last N UTC days from
`resources.days` beside the ring (RT-184; one row a day, 8 kept, rolled by the governor on
its sampler tick and rewritten at most once a minute). Each row carries the sample count,
the max footprint (and with children), shed minutes, refuse minutes with each refuse
window's start, length and entering trigger (memory, host trigger, or the swap-volume disk
floor), exit rungs and unclean exits. Exit rungs and unclean exits are read from the
lifecycle ledger (`lifecycle.jsonl` and `.1`) at read time; a day the ledger does not
reach back to shows `unknown`, never `0`. One more line gives RT-M1's verdict over the
last seven complete UTC days (today excluded): `PASS`, `FAIL` naming the criterion and day,
or `INCOMPLETE` naming the missing, thin (under 1,296 of 1,440 samples) or ledger-blind days.
`--json` adds `days` and `verdict` (`state`, `criterion`, `day`, `detail`, `days`); the
reading is `fathom resources --days 8 --json`, field `verdict.state`. The exit code is
unchanged by `--days`.

Exit `0` within budget; `1` over budget, the ladder at refuse or exit, crash-looping
(3 counted exits in 30 min), stopped by the breaker, or scratch daemons leaked (the
daemons `daemon reap --apply` would kill; a record-less daemon whose parent is alive is
listed as kept, not leaked); `2` a
ring, ledger or config is unreadable, or there is no sample at all to judge. `--id`
picks another state directory (a scratch daemon's `addr-<port>-<pid>`); the default is
the record serving `--url`, else the managed daemon.

The session primer (`fathom prime`, the SessionStart hook) carries one line from the
same files — at most 160 B, inside the primer's 3,200 B cap — only when the daemon is
not normal: stopped by the breaker, shed or refusing, in safe mode, crash-looping, or
holding a quarantined service (RT-51).
