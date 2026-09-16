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
- [x] Call-log entries carry origin so kill clocks and adoption numbers separate agent choice from plumbing {id: GI-6, symbol: WithOrigin, package: internal/mcp, reachable_from: TestAnsweredBeatCarriesDisplacedCount}

## Grammar hardening (today's kit-corpus lessons)

- [x] Annotation parser takes the TRAILING brace block, so prose braces in titles no longer hijack contracts; documented one-annotation-per-line stays {id: GI-7, symbol: parseAnnotation, package: internal/verify, reachable_from: TestBind_DriftedModule_FlagsRules}

## Deferred

- [-] PreToolUse grep-hint revival {id: GI-8, disposition: refused, reason: "closed by a sharper PreToolUse mechanism, not implemented as originally proposed — docs/design/guarded-edits.md (phase O, owner-commissioned 2026-07-29) ships the PreToolUse channel GI-8 asked for, but scoped to configured critical scopes/commands (.fathom/guard.cue) with governing-rule injection, not an evidence-gated grep-hint suggestion. The grep-miss-taxonomy evidence gate this row's trigger named was never reached and is superseded by guarded-edits.md's own §2.4 doctor/instrumentation loop (dead-guard warnings, guarded-edit coverage in `fathom research report`) as the ongoing falsifiability check for this channel."}
