### `daemon` — install, inspect, stop and remove the serve-http agent

```sh
fathom daemon install [--label dev.fathom.serve-http] [--store PATH] [--addr :7575] [--bin PATH] [--force]
fathom daemon install --reconcile [--label NAME] [--adopt]
fathom daemon status  [--url http://localhost:7575] [--json]
fathom daemon stop    [--for 2h] [--reason TEXT] [--url URL] [--label NAME]
fathom daemon start   [--label NAME]
fathom daemon pause   [--reason TEXT] [--url URL]
fathom daemon resume
fathom daemon uninstall [--label NAME]
fathom daemon ps   [--json]
fathom daemon reap [--apply] [--grace 5s]
fathom daemon deploy-intent [--label NAME] [--binary PATH]
```

`deploy-intent` appends a `deploy` row (the running pid from the daemon
record, the new binary's hash) to the label's lifecycle ledger; `task deploy`
runs it before its kickstart so the breaker does not count the kickstart. It
never fails a deploy: with no recorded daemon it says so and exits 0.

`status --json`: `{platform, label?, plist?, plist_schema?, posture_drift?,
posture_remedy?, switch?, signing?, url, reachable, detail?, remedy?,
exit_code}` — exit 1 when not reachable, same as the text path.

**The kill switch (responsible tenant §9, RT-3..RT-6).** `stop` writes
`~/.fathom/daemon/disabled.json {by, reason, since, until?, addr}` first (`addr` is `--url`'s host:port; `pause` records it in `paused.json` the same way, and hooks go quiet only for that address), then
finds the managed pid from the daemon record — cross-checked against the live
process (still a `serve-http`, started no later than the record: a reused pid
is never signalled) or, without a record, the port's listener when it is a
fathom `serve-http` — sends SIGTERM, waits 10 s, sends SIGKILL, and reports
success only when the process is gone **and** the port is closed, printing
the proof (`pid 75018 gone after 3.1 s (SIGTERM); :7575 closed; launchd will
not relaunch: disabled.json`). Exit 1 when either fails. The managed daemon
(`serve-http --managed`, or a launchd job whose label is the `Label` of an
installed fathom `serve-http` agent — the label need not contain "fathom")
reads the
switch as its first action — before config, store or bind — and exits 0 in
under 50 ms; with `--for` it exits 75 (retried later) until `until`, then
starts normally. An empty file (`touch ~/.fathom/daemon/disabled.json`, the
"no binary available" switch) reads "disabled, reason unknown". A scratch
(unmanaged) `serve-http` ignores both files. `start` removes the switch,
appends an `operator_reset` row to `~/.fathom/daemon/<label>/lifecycle.jsonl`
(the breaker window restarts) and kickstarts the label. `pause` writes
`paused.json`; the managed daemon re-reads it every 2 s, stays up and
enters safe mode (logged): tree and fleet refused with the reason,
auto-refresh, the console feeders and the decision port off (reads answer
from the index as it stands and their envelope says `refresh: deferred (safe
mode)`), point and index served. `resume` removes it. While the
switch is set, `status`, doctor and the session primer name `fathom daemon
start` (with the switch's reason) as the remedy instead of a kickstart, and
`task deploy` installs the binary, then stops with that message instead of
waiting on a health check the switch will fail. A switch file that exists
but cannot be read still applies, named unreadable (I38). **Under an installed schema-1 plist** (`KeepAlive
true`) exit 0 does not stay down: `stop` says launchd will relaunch every
10 s, each start exiting in < 50 ms, and prints `launchctl bootout` and the
`--reconcile` line (or, when the plist's label would not be recognised as
the managed daemon, says each relaunch will NOT see the switch). From an agent shell (`CLAUDECODE` or `CODEX_*` set)
every `launchctl` line is printed, never run — running it from an agent's
Bash tool wedges.

**Plist schema 2 (§8.1, RT-13).** `install` writes `KeepAlive
{SuccessfulExit: false}` (exit 0 stays down; crashes and signals relaunch),
`ThrottleInterval 60`, `ProcessType Background`, `LowPriorityIO`,
`LowPriorityBackgroundIO`, `Nice 5`, `ExitTimeOut 20`, `serve-http
--managed`, and a `<!-- fathom-plist-schema: 2 -->` marker. `install
--reconcile` brings an installed plist there in place: it rewrites only
those keys (and inserts `--managed` after `serve-http`), keeps the label,
paths, store, call log, `WorkingDirectory`, the environment and every
comment byte for byte, writes a backup
`~/.fathom/daemon/<label>/<label>.plist.bak.<UTC ts>` (never beside the live
plist, where launchd could load it) and a receipt at
`~/.fathom/daemon/<label>/reconcile-<ts>.json`, needs `--adopt` for a plist
fathom did not write, and refuses unless the plist's binary answers
`version --json` with `tenancy_contract >= 1` (order: `task deploy`,
confirm, reconcile, reload). Doctor's `daemon_posture` row and `status`
WARN on drift with the exact command. `status` also prints the running
daemon's codesigning identity — read from the live process (`codesign -dv
<pid>`), since a deploy replaces the file under it, or from the plist's
binary on disk when none is running, and says which — and names an ad-hoc
one (RT-148). Adoption by the self-restart watcher exits 75, so schema 2
relaunches it; that is what `tenancy_contract` 1 promises.

`ps` joins the daemon records (`~/.fathom/daemon/<id>/record.json`, schema 2,
`<id>` = the launchd label when managed, else `addr-<port>-<pid>`; the legacy
`~/.fathom/daemons/<host>-<port>.json` is still read for this release) with a
live census of every `serve-http` process of any fathom executable, record or
not. Columns: pid, addr, managed, age, RSS, idle (from the record's 60 s
heartbeat; `?` without one), parent alive, unlinked executable, record state.
Verdicts: `ok`, `orphaned` (unmanaged, parent gone), `idle` (unmanaged, no
request for 1 h), `unlinked`, `no-record`, `stale-record` (the record's process
is gone or its pid was reused). The managed daemon is never judged orphaned or
idle; a record-less process on the managed address — the installed plist's
`--addr`, else `[daemon].addr`, else `127.0.0.1:7575` — counts as managed. A
daemon with a request in flight (an open `/events` stream) is never idle.

`reap` is a **dry run by default**: it prints what it would kill and prune.
With `--apply` it kills only unmanaged daemons whose verdict is not `ok`
(SIGTERM, `--grace`, SIGKILL), re-checking each pid against a fresh census
first (a reused pid is skipped), never the managed daemon, never its own
process tree, and never a `no-record` daemon whose parent is alive (its record
lives under another HOME — stranger-check's, a test suite's — and its own
bounds apply); it removes their records and prunes stale ones whose pid is
still gone at prune time.

Records do not accumulate: an unmanaged daemon removes its record on its
bounded exits and on SIGTERM, SIGINT or SIGHUP (exiting 128+signo), and a
daemon recording itself prunes the records of scratch daemons whose pid is
gone (a SIGKILL or a crash). The managed daemon's record is kept while it is
down, so verbs still find its call log.

macOS/launchd. `install` writes `~/Library/LaunchAgents/<label>.plist` with
**your** resolved binary (symlinks followed, so launchd runs the stable
installed copy rather than a `bin/` build output a rebuild would truncate),
store at `~/.fathom/db/fathom.sqlite`, call log at
`~/.fathom/logs/mcp-calls.jsonl`, and a PATH carrying your Go toolchain —
without `go` the daemon's auto-refresh degrades Go services to syntax-only
edges, so `install` says so when it cannot find one. It refuses to stack a
second agent on the same port, naming the one already there and how to keep
or replace it. `uninstall` leaves the store and logs alone.

`status` prints the installed label and whether the daemon is actually
serving — an installed agent that is not serving is worse than none, because
everything downstream assumes it.

Every "the daemon is down" message in fathom (doctor's remedy, the
SessionStart line, `init`'s closing note) names the label **discovered on
this machine**, never a fixed one. Off macOS, all of them fall back to
`fathom serve-http` under your own supervisor.
