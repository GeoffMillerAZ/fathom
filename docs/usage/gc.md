### `gc` — disk retention by class

```sh
fathom gc [--apply] [--json] [--workspace <dir>] [--call-log <path>] [--url <daemon>]
```

A dry run by default (RT-125, design §12.1): it lists what each class would reclaim —
and changes nothing — until `--apply`. Evidence is archived, never deleted:

| Class | Policy |
|---|---|
| `call-log` | the serving daemon's writer rotates the live log at 64 MiB (renames only: live → `<log>.1`, the previous `.1` → `<log>.<ts>.pending`) and gzips pending generations in the background; gc never renames a live log — over 64 MiB it is reported, and `--apply` rolls up and gzips pending generations to `<log>.<ts>.jsonl.gz` under the lock the daemon's pass shares. Archives (including `metrics rotate`'s `<log>.archive-<date>`) are never deleted, and past 1 GiB they are reported |
| `ledgers` | the verify, session and intent ledgers and `decisions.jsonl` over 32 MiB are reported and kept whole — their readers read only the live file, so rotating them would hide history |
| `ledger-unregistered` | `fathom gc` only (PE-29, design progress-and-economics §11): rows of the verify and intent ledgers whose `repo_root` is not a registered service root of the daemon at `--url` (scratch clones, worktrees, test runs; PE-23's rule). The dry run counts rows and bytes per unregistered root; `--apply` moves them, byte-identical, to `$FATHOM_HOME/archive/<ledger>-unregistered-<YYYY-MM-DD>.jsonl` (appended when the day's file exists), updates `archive/SHA256SUMS` and adds one row to the table in `archive/README.md`. Rows are moved, never deleted, and a row appended during the swap stays live; restore with `cat <archive> >> <ledger>`. A row naming no root stays live. When the registered roots cannot be read, or the registry lists none, nothing is planned and one line says why. Neither the daemon's daily pass nor `doctor` plans this class |
| `session-stamps` | older than 30 days: deleted |
| `daemon-records` | `~/.fathom/daemon/<id>/record.json` (and legacy `~/.fathom/daemons`) whose pid is not running: deleted (the directory and its lifecycle ledger stay) |
| `diag` | captures past 10 / 200 MiB per daemon: deleted, oldest first |
| `tmp` | `$TMPDIR` `fathom-edit-set*`, `fathom-*-test-*`, `fathom-stranger-check*` older than a day: deleted; entries of the hooks' caches (`fathom-filechanged`, `fathom-toplevel-cache`, `fathom-ident-cache`, `fathom-guard-stamps`, `fathom-bash-doubt`) older than a day: deleted (the directories stay) |
| `scenariobench-worktrees` | `workspace/artifacts/scenariobench/worktrees` beyond the newest 5 runs and older than 14 days (a `.pin` file exempts one): deleted |
| `workspace-bins` | `workspace/tmp/fathom-bin-*` beyond the newest 3: deleted |
| `wazero-cache` | `~/.fathom/cache/wazero/`: another wazero version's directory, and this version's compiled grammar code unwritten for 30 days: deleted (only the managed daemon writes it; the next compile recreates what is used) |

Exit `0`, or `1` when any class could not be read or acted on (each error named).

The managed daemon applies the `session-stamps`, `daemon-records`, `diag` and `tmp` classes
itself once a day (RT-181): never in its first 10 minutes, never while work is in flight or
its ladder is above normal, one `gc (daily)` line in its log per run. The workspace classes
stay with this verb. `VACUUM` of a store is not offered (deferred, RT-D10).

The serving daemon also bounds what it writes while it runs (RT-126..RT-128): the call
log rotates at 64 MiB the same way (console tails read the rotated generation
`<log>.1`, so traffic history survives a rotation); `fathom-http.log`/`.err` rotate on
their own 60 s ticker (no longer paused by `FATHOM_SCAVENGE=off`), streaming the prior
generation to `.1` — above 256 MiB they are truncated without a copy and one line names
the discarded bytes; and the managed daemon runs `wal_checkpoint(TRUNCATE)` after 5 idle
minutes, at most hourly (`journal_size_limit` is 64 MiB).
