H4's tests-to-run line is fast enough to ride the edit hook, but only through a warm daemon at the managed budget. Over 50 one-file edits replayed from this repository's recent commits, the daemon call the hook would make answered every edit, with a p95 of 37 ms against a 2 s bar. Two limits come with that answer, and RO-43 should design for both. The first call after the daemon builds its call graph took 8.4 to 9.6 s, which RO-43 must report as `tests: not computed (deadline)`. A scratch (unmanaged) daemon at its 512 MiB default answered none of the 50 edits. `fathom tests select` itself does not go through the daemon: it opens the store in process and took 4.4 to 5.1 s per edit. So a pane that MOD-X2 builds on the CLI as it stands would not meet its own 2 s trigger.

RO-M13 verdict: PASS

# RO-M13: H4's speed reading, MOD-X2's trigger

Taken 2026-10-19 between 07:35Z and 07:43Z. The binary was `bin/fathom` built in the worker clone at 0201a1d1 (v0.15.6), and the store was a scratch ingest of that commit (service `fathom`, 257 s). A parallel wave worker (wave stat1, item `stat`) shared the host for the whole run. The load average stayed under 8 throughout, so no re-run was needed.

## What the hook would call

The premise needed grounding in two places.

- `fathom tests select` has no daemon path. `runTestsSelect` (cmd/fathom/tests_select.go) opens the store with `store.Open`, lists every symbol, builds the call graph and shells out to `git diff` on each run. The daemon path that exists is the one RO-43 names as its reach, "the reach the pre-edit blast trailer counts". That trailer is `fetchBlastRadius` (cmd/fathom/edithook_blast.go): `POST <url>/rpc`, `tools/call blast_radius {service, changed_files: [file]}`, a new client for each call, with the edit hook's `X-Fathom-Origin: edit-hook` header. The hook resolves `<url>` from `--url`, whose default is `FATHOM_HTTP` (`defaultFathomURL`). The harness sends that exact request with `FATHOM_HTTP` pointed at the scratch daemon. This call is file-scoped: it answers for the edited file, not for the edit's changed symbols alone.
- The section's Start says RO-M13 makes no daemon call. Its own text does make one, and the result depends on the daemon's tenancy budget. An unmanaged `serve-http` never takes the managed budget (internal/tenancy/budget.go `ResolveBudget`): its default is 512 MiB. The managed daemon, the one the owner's hooks call, has no `[tenancy]` budget in its config or LaunchAgent, so it takes the auto budget, clamp(RAM / 8, 1 GiB, 4 GiB), which is 4 GiB on this 32 GiB host. The verdict run sets `FATHOM_TENANCY_BUDGET=4GiB` to match the managed daemon. The 512 MiB run is reported separately.

## Method

These rules were fixed before any timing:

- **Edits.** The design names no rule, so the brief's applies. The edits are the 50 most recent non-merge commits on `main`, in `git log` order, whose changed files include exactly one non-test `.go` file (other changed files are allowed) and whose file still exists at HEAD. Because of that rule, 13 of the 50 are `internal/buildinfo/buildinfo.go` version bumps.
- **Daemon.** A scratch `serve-http` ran on a free loopback port (56996, never 7575) with a scratch store and a scratch `FATHOM_HOME` under `$W`, a scratch directory outside the repository. Its daemon record landed there, not in `~/.fathom`. Auto-refresh stayed off, the unmanaged default, so the store watermark never moved during a series. Timing waited out the daemon's 2-minute tenancy warm-up.
- **Per series.** One warm-up call is reported apart, because it builds the graph that later calls reuse from the memo. Then each of the 50 edits gets one call, timed end to end from the client (curl `time_total`, connect included).
- **Statistics.** Percentiles are nearest-rank.
- **Context series.** This series is not the verdict. It runs `fathom tests select --since <commit>^ --json` over the first 10 edits, in a scratch worktree checked out at each commit against the same store, and times wall clock from exec to exit.

## Numbers

| Series | Answered | p50 | p95 | max | warm-up (first) call | load start → end |
|---|---:|---:|---:|---:|---:|---|
| Daemon, managed budget 4 GiB (verdict) | 50 of 50 | 26.1 ms | 37.2 ms | 39.3 ms | 9,534.9 ms | 2.93 → 3.18 |
| Daemon, unmanaged default 512 MiB | 0 of 50 | 2.4 ms (error) | 2.7 ms (error) | 2,492.6 ms (error) | 8,371.5 ms | 3.33 → 3.64 |
| `fathom tests select` CLI, in process, 10 edits | 10 of 10 | 4,692.0 ms | 5,053.4 ms | 5,053.4 ms | n/a | 3.80 → 4.52 |

Notes on the table:

- **512 MiB run.** After the warm-up call, the daemon went from normal to refuse (`footprint+children 764.8 MiB ≥ refuse mark 512 MiB`). Every call after that returned `client.ListAllSymbols: over budget: tree producer halted at refuse`, which the hook would turn into silence. This is O1073's failure on the blast path.
- **Trial run.** An earlier run at 512 MiB was mistimed: it started inside the warm-up window by a harness error. It answered 3 of 50 edits, at 9.4 to 10.0 s each, and refused the rest.
- **First-call cost.** In all three daemon starts, the first call took 8.4 to 9.6 s. Later calls reuse the graph from the memo under the store watermark. A re-ingest moves the watermark, and the cache drops after 30 minutes idle, so the next call after either costs a cold build. That call misses both this 2 s bar and the trailer's own 500 ms `blastRadiusTimeout` (O1260).
- **Reach.** `blast_radius` named 0 to 167 tests per edit, at its default depth of 2. `tests select` named 11 to 1,332 ranked tests on the same edits. RO-43's cap of 5 reads the former.

The 50 edits, verdict series (ms, then the `tests` and `total_symbols` the answer carried):

| # | Commit | File | ms | Tests | Symbols |
|---:|---|---|---:|---:|---:|
| 1 | `c6a5737e` | `internal/decide/securitytemplate.go` | 27.8 | 20 | 39 |
| 2 | `bc321d39` | `cmd/fathom/daemon_signing.go` | 26.3 | 11 | 28 |
| 3 | `c4c52c45` | `cmd/fathom/daemon_signing.go` | 25.1 | 11 | 28 |
| 4 | `eb3b84a0` | `cmd/fathom/verbhelp.go` | 26.7 | 87 | 112 |
| 5 | `27a5cf29` | `cmd/fathom/reconcile.go` | 25.0 | 22 | 77 |
| 6 | `714f6f7d` | `cmd/fathom/readoutcmd_replay.go` | 27.4 | 9 | 133 |
| 7 | `8db6c4c9` | `cmd/fathom/readoutcmd_replay.go` | 26.9 | 9 | 133 |
| 8 | `674b29b1` | `internal/readout/claims.go` | 26.6 | 1 | 37 |
| 9 | `1762acbe` | `internal/buildinfo/buildinfo.go` | 29.0 | 66 | 148 |
| 10 | `f93ea21b` | `cmd/fathom/console_plansdata.go` | 36.8 | 167 | 1333 |
| 11 | `7bb41a4e` | `cmd/fathom/testrunhook.go` | 39.3 | 14 | 85 |
| 12 | `cf585a99` | `internal/buildinfo/buildinfo.go` | 26.9 | 66 | 148 |
| 13 | `088683b8` | `internal/verify/operational.go` | 25.2 | 5 | 26 |
| 14 | `09ccb490` | `internal/buildinfo/buildinfo.go` | 26.1 | 66 | 148 |
| 15 | `08a67106` | `cmd/fathom/verify.go` | 26.8 | 107 | 214 |
| 16 | `688331f4` | `cmd/fathom/verify.go` | 27.0 | 107 | 214 |
| 17 | `777fe438` | `cmd/fathom/plans.go` | 25.8 | 102 | 166 |
| 18 | `78157704` | `internal/buildinfo/buildinfo.go` | 26.2 | 66 | 148 |
| 19 | `48423b19` | `internal/buildinfo/buildinfo.go` | 26.2 | 66 | 148 |
| 20 | `02673c14` | `internal/buildinfo/buildinfo.go` | 27.0 | 66 | 148 |
| 21 | `7cf3c858` | `cmd/fathom/testrunhook.go` | 38.8 | 14 | 85 |
| 22 | `b59097d6` | `internal/scenariobench/types.go` | 26.2 | 0 | 44 |
| 23 | `a7d18c2f` | `internal/buildinfo/buildinfo.go` | 26.8 | 66 | 148 |
| 24 | `9cf4bcd0` | `internal/intent/futures.go` | 24.7 | 13 | 30 |
| 25 | `2dfcab5e` | `cmd/fathom/intentexport.go` | 25.1 | 15 | 47 |
| 26 | `ebf07bb5` | `cmd/fathom/genplancue.go` | 25.9 | 87 | 107 |
| 27 | `c6226ce1` | `internal/buildinfo/buildinfo.go` | 26.2 | 66 | 148 |
| 28 | `4646b700` | `cmd/fathom/verify.go` | 26.4 | 107 | 214 |
| 29 | `61676841` | `internal/mcp/platform_tools.go` | 23.8 | 9 | 41 |
| 30 | `f6bf2904` | `internal/buildinfo/buildinfo.go` | 25.7 | 66 | 148 |
| 31 | `7b26f21b` | `internal/testisolate/testisolate.go` | 24.7 | 27 | 106 |
| 32 | `93f1eb30` | `internal/buildinfo/buildinfo.go` | 36.7 | 66 | 148 |
| 33 | `dc64537c` | `internal/mcp/semantic_tools.go` | 25.4 | 3 | 42 |
| 34 | `7ea258db` | `internal/fusion/typedcalls.go` | 26.3 | 41 | 113 |
| 35 | `71e6383c` | `internal/buildinfo/buildinfo.go` | 25.6 | 66 | 148 |
| 36 | `d0667c8d` | `cmd/fathom/portholder.go` | 23.5 | 3 | 9 |
| 37 | `516d8fb2` | `internal/buildinfo/buildinfo.go` | 25.8 | 66 | 148 |
| 38 | `f3915040` | `internal/analysis/impact/testgap.go` | 24.4 | 13 | 33 |
| 39 | `db8d2ba5` | `cmd/fathom/tests_select_certifying.go` | 24.6 | 8 | 23 |
| 40 | `89fae5ed` | `internal/mcp/autorefresh.go` | 26.0 | 58 | 242 |
| 41 | `6a4372c0` | `internal/mcp/verify_plan_tool.go` | 25.4 | 47 | 96 |
| 42 | `c3c5bd4b` | `internal/mcp/platform_tools.go` | 24.8 | 9 | 41 |
| 43 | `b97e6e8c` | `internal/mcp/semantic_tools.go` | 26.8 | 3 | 42 |
| 44 | `efd8adc7` | `internal/buildinfo/buildinfo.go` | 37.2 | 66 | 148 |
| 45 | `8d36740b` | `cmd/fathom/doctor.go` | 27.9 | 165 | 314 |
| 46 | `3808007b` | `internal/mcp/observedby.go` | 24.5 | 0 | 5 |
| 47 | `7feb168e` | `cmd/indexbench/langroutes.go` | 24.7 | 9 | 24 |
| 48 | `45073f2d` | `internal/mcp/search_tool.go` | 24.8 | 19 | 83 |
| 49 | `3c8785a4` | `internal/fusion/jenkinsrun.go` | 24.1 | 4 | 55 |
| 50 | `8df0a2d7` | `internal/store/machinelocal.go` | 24.6 | 1 | 6 |

Context series, `tests select` in process (ms, then the length of `ranked_tests`): 1 4,998.7 (1332), 2 4,546.7 (636), 3 5,005.2 (620), 4 4,606.8 (617), 5 4,410.3 (648), 6 4,692.0 (623), 7 4,756.3 (655), 8 4,606.5 (620), 9 5,053.4 (1246), 10 5,038.6 (11).

## What this means

- **RO-43 may proceed.** Its daemon call fits the 2 s bar by about fifty times while the graph is warm. It still has to name the cold call as `tests: not computed (deadline)`, because after every re-ingest the first call takes about 9 s on this repository.
- **Scratch daemons.** Any daemon running at the unmanaged 512 MiB default (scratch daemons, adopters without config) serves no tests line at all on a repository this size. That is the tenancy budget fix this section waits on (O1073, O1014).
- **MOD-X2.** Its trigger reads "tests select reads p95 under 2 s". The CLI does not meet that bar (p95 5.1 s), and the daemon path does. A person's hint or pane should therefore call the daemon (O1259).

## Harness

The scripts live in `$W/harness/` (`$W` the scratch directory, `$CLONE` the worker clone). Every command ran through `wx stat1 speed '<command>'`, which exports the scratch `FATHOM_HOME`.

```bash
go build -o bin/fathom ./cmd/fathom
bin/fathom ingest --service fathom --path . --store $W/store/fathom.sqlite
$W/harness/pick.sh > $W/harness/edits.txt
python3 -c "import socket;s=socket.socket();s.bind((\"127.0.0.1\",0));print(s.getsockname()[1])" > $W/port
$W/harness/start.sh            # unmanaged default, 512 MiB
sleep 125; FATHOM_HTTP=http://127.0.0.1:$(cat $W/port) $W/harness/run.sh $W/harness/run2.txt
kill $(cat $W/daemon.pid); $W/harness/start.sh 4GiB
sleep 125; FATHOM_HTTP=http://127.0.0.1:$(cat $W/port) $W/harness/run.sh $W/harness/run3.txt
git worktree add -q --detach $W/wt HEAD; $W/harness/cli.sh $W/harness/cli2.txt
```

`pick.sh`:

```bash
git log --no-merges --format='C %H' --name-only main | awk '
  /^C /{ if (h!="" && cnt==1) print h, f; h=$2; cnt=0; f=""; next }
  /\.go$/ && !/_test\.go$/ { cnt++; f=$0 }
  END{ if (h!="" && cnt==1) print h, f }' |
while read -r h f; do
  [ -f "$f" ] || continue
  echo "$h $f"; n=$((n+1)); [ "$n" -ge 50 ] && break
done
```

`start.sh`:

```bash
P=$(cat $W/port)
[ -n "${1:-}" ] && export FATHOM_TENANCY_BUDGET=$1 || unset FATHOM_TENANCY_BUDGET
nohup $CLONE/bin/fathom serve-http --addr 127.0.0.1:$P \
  --store $W/store/fathom.sqlite --call-log $W/logs/mcp-calls.jsonl --exit-with-parent=false --ttl 3h --idle-exit 0 \
  > $W/logs/daemon-${1:-default}.log 2>&1 < /dev/null & echo $! > $W/daemon.pid; disown
```

`run.sh` (the request is `fetchBlastRadius`'s):

```bash
call() {
  local body; body=$(jq -cn --arg f "$1" '{jsonrpc:"2.0",id:1,method:"tools/call",params:{name:"blast_radius",arguments:{service:"fathom",changed_files:[$f]}}}')
  local t; t=$(curl -s -o "$H/resp.json" -w '%{http_code} %{time_total}' -X POST "$URL/rpc" \
    -H 'Content-Type: application/json' -H 'X-Fathom-Origin: edit-hook' \
    -H "X-Fathom-Caller: pid=$$;ppid=$PPID;origin=edit-hook;session=ro-m13;test=0;cwd=$PWD" --data "$body")
  ...   # ms = time_total*1000; "error" when the JSON-RPC answer carries .error; tests and total_symbols from the result
}
echo "load_start $(sysctl -n vm.loadavg)" > "$OUT"
echo "warmup $(call cmd/fathom/main.go)" >> "$OUT"
while read -r h f; do i=$((i+1)); echo "$i ${h:0:8} $f $(call "$f")" >> "$OUT"; done < "$H/edits.txt"
echo "load_end $(sysctl -n vm.loadavg)" >> "$OUT"
```

`cli.sh` (context series):

```bash
while read -r h f; do
  i=$((i+1)); git -C "$W/wt" checkout -q --detach "$h"
  s=$(now); (cd "$W/wt" && "$BIN" tests select --since "$h^" --service fathom --store "$W/store/fathom.sqlite" --json > "$H/cli.json" 2>"$H/cli.err"); rc=$?; e=$(now)
  n=$(jq -r '(.ranked_tests // []) | length' "$H/cli.json" 2>/dev/null)
  echo "$i ${h:0:8} $f $(awk -v s="$s" -v e="$e" 'BEGIN{printf "%.1f", (e-s)*1000}') rc=$rc n=$n" >> "$OUT"
done < <(head -n 10 "$H/edits.txt")
```

The scratch daemons (PIDs 31182, 34191 and 37093; 29946 had already exited with its parent) were stopped by PID. The store, home and worktree were then removed.
