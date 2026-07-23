# Phase A — Activation (vision 11 §3.A)

Turn on what's built. Supply-side only — no new machinery. Law 6: every
operator activation is stamped. Law 8 gates nothing here (these are not
adoption kills).

Design (normative):
- **A1 instrument repair.** The call log's `surface` field is derived from
  the request path; hook/CLI clients hit bare `/rpc` by design and are
  distinguished by `origin` tags (already shipped). The remaining gaps:
  (1) `server_stats` exposes a `surface_traffic` block — per-surface call
  count + last-call timestamp — so "is anything flowing on /rpc/agent?" is
  answerable from the daemon (doctrine 7); (2) the adoption-clock t0 stamp
  mechanism: `workspace/logs/adoption-clock-t0.txt`, written by the operator
  or by the first qualifying non-fleet-repo session (law 8), never by code
  silently; (3) CLAUDE.md documents the true registration URL (/rpc/agent).
- **A2 idempotency guard.** `fathom compile` refuses a non-dry-run compile
  when the store's latest compile_run for (service, layer) has the same
  corpus watermark AND same prompt identity — with `--force` as the logged
  escape. Prevents the flag-day from double-writing superseded claims (75%
  of stored claims are already superseded from same-watermark reruns).
  Only a CLEANLY completed run (not refused, not budget-stopped, zero
  failed sections) makes a rerun idempotent — a stopped/partial run's own
  documented remedy is "re-run to resume", which the guard must not force
  through `--force`. **Scoping note (shared with D3's eval-gate,
  docs/plans/expert-depth.md):** both guards live in `cmd/fathom`'s direct
  `--store` lane only — the lane the flag-day runbook
  (`scripts/recompile-fleet.sh`) actually uses, with the local store
  already open for the pre-flight reads. The HTTP/daemon `compile_layer`
  path (`internal/mcp/semantic_tools.go`) is UNGUARDED today: guarding it
  means teaching the daemon handler the same store pre-flight and an
  escape-note plumb-through over JSON-RPC, deferred until a daemon-side
  compile consumer exists (today's only spending caller is the operator
  CLI). Passing `--force`/`--unsafe-skip-eval-gate` without `--store` is
  refused loudly, never silently ignored.
- **A2/A3 runbook.** `scripts/recompile-fleet.sh`: obligations × every fleet
  service + adr-rules × fathom + spec-bindings × fathom (11 service·packs,
  ≤$0.60 metered / $0 claude-cli lane), stamping
  `workspace/logs/recompile-stamp.txt` on completion. Operator-fired.
- **A4 catalog de-theater.** docs/layers.md and `fathom config layers` show
  `eval` (pass/fail + scores) and `production` (live-store claim count +
  last compile date) as separate columns; production data read from the
  store, never hand-maintained.

## Tasks

- [x] server_stats surface_traffic block: per-surface {calls, last_ts} from the call-log counters, covered by fake-backend test {id: A1-1, symbol: registerServerStats, package: internal/mcp}
- [x] CLAUDE.md registration URL corrected to /rpc/agent + restart caveat; adoption-clock t0 stamp convention documented in the plan + vision doc {id: A1-2, symbol: registerServerStats, package: internal/mcp, skip_test: true, skip: "docs-only row anchored to the feature it documents; A1-1's test is the code coverage"}
- [x] Compile idempotency guard: same (service, layer, watermark, prompt identity) → refuse with citation; --force logged escape; unit-tested against a fake store {id: A2-1, symbol: cmdCompile, package: cmd/fathom}
- [x] scripts/recompile-fleet.sh: 11 service·packs, claude-cli lane default, per-pack failure isolation, completion stamp {id: A2-2, symbol: cmdCompile, package: cmd/fathom, skip_test: true, skip: "operator shell script exercising the gated CLI; A2-1 covers the code path; script syntax-checked by bash -n in CI"}
- [ ] (op) Recompile fired across the fleet; stamp exists; post-compile: obligations claims fresh at HEAD for all 9 services, adr-rules + spec-bindings have >0 production claims on fathom {id: A2-3, symbol: cmdCompile, package: cmd/fathom, skip_test: true, skip: "operator-fired activation; evidence is the stamp file + store row counts, recorded here on completion"}
- [ ] Expert-module portability proven: agentic-kit + app-kit obligations compiles required zero code changes beyond flags — else this row stays open and the portability defect is fixed first {id: A3-1, symbol: CompileLayer, package: pkg/client, skip_test: true, skip: "op-gated proof row; evidence recorded from A2-3's run"}
- [x] Catalog de-theater: eval vs production columns in docs/layers.md + layers CLI verb reading production truth from the store {id: A4-1, symbol: cmdLayersList, package: cmd/fathom}
