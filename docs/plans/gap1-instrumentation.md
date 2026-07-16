# Gap-1 — channel instrumentation: denominator, liveness, origin (docs/vision/04 Gap 1)

The flagship injection channel was dark 2026-07-11→14 and nothing noticed;
the call log records effects, not activity, and carries no origin — so
coverage has no denominator and gate/hook plumbing pollutes every demand
number. Composes agentic-kit v0.7.0 `telemetry/heartbeat`.

## Beats (the denominator)

- [x] Edit-hook beats every invocation to the hooks beat log (FATHOM_HOOKS_LOG, default ~/.fathom/hooks.jsonl — hooks fire in every fleet repo, so the log is central): kind seen on entry then answered (payload or clean) or error — silent-exit contract unchanged (beat failures are swallowed) {id: GI-1, symbol: emitBeat, package: cmd/fathom}
- [x] Session-hook beats kind fired on every run {id: GI-2, symbol: cmdSessionHook, package: cmd/fathom}
- [x] Injection coverage becomes computable: answered/seen ratio surfaced by fathom instrument {id: GI-3, symbol: cmdInstrument, package: cmd/fathom}

## Liveness (the watchdog)

- [x] fathom instrument runs heartbeat.CheckLiveness with the standing expectations (session-hook.fired implies edit-hook.seen within 7d; edit-hook.seen implies edit-hook.answered within 1d) and prints alarms; exit 0 always — advisory data, never a gate {id: GI-4, symbol: cmdInstrument, package: cmd/fathom, skip_test: true, skip: "cmd wrapper over kit CheckLiveness; projections are kit-tested; wiring test covers cmdInstrument"}

## Origin (clean demand numbers)

- [x] Every hook/CLI HTTP call sends X-Fathom-Origin (edit-hook, session-hook, stop-hook, verify-cli); the daemon call log records it {id: GI-5, symbol: originHeader, package: cmd/fathom}
- [x] Call-log entries carry origin so kill clocks and adoption numbers separate agent choice from plumbing {id: GI-6, symbol: WithOrigin, package: internal/mcp}

## Grammar hardening (today's kit-corpus lessons)

- [x] Annotation parser takes the TRAILING brace block, so prose braces in titles no longer hijack contracts; documented one-annotation-per-line stays {id: GI-7, symbol: parseAnnotation, package: internal/verify}

## Deferred

- [-] PreToolUse grep-hint revival {id: GI-8, disposition: deferred, reason: "rejected pending evidence", trigger: "grep-miss taxonomy shows an index-answerable share at or above 30 percent"}
