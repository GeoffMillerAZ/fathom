# Context bill — unchanged hook context once per session

Grammar: v2. Prefix: CB-. Baseline reading: `docs/bench/readings/context-bill/2026-10-22.md`.
Scope: hook-written files only; no daemon tables, caches, jobs, tenancy policy or model calls.

- [x] (op) Record the $0 baseline reading. {id: CB-M1, oracle: lexical, path: docs/bench/readings/context-bill/2026-10-22.md, pattern: "^CB-M1 bill: [0-9]+ bytes"}
- [x] Post-edit and post-bash dedupe: in primed Claude Code and Codex contexts, serve new or changed rule bullets and count distinct rules already shown; unchanged complete payloads emit nothing within the 30-minute and 10-suppression backstops. SessionStart compact or resume and pre-compact reset the record. {id: CB-1, symbol: dedupeHookRules, package: cmd/fathom, reachable_from: cmdEditHook}
- [x] Cwd-changed dedupe: suppress only consecutive successfully delivered primers with equal repository roots and content digests in a primed Claude Code session, within the backstops; A → B → A re-emits A. {id: CB-2, symbol: cwdChangedPrimerForSession, package: cmd/fathom, reachable_from: cmdSessionHookEvent}
- [ ] Measure SubagentStart before and after; retain one primer per fresh subagent and cut only sections demonstrably carried by its own SessionStart or instructions. {id: CB-3, oracle: lexical, path: docs/bench/readings/context-bill/2026-10-22.md, pattern: "^CB-3 measurement:"}
- [ ] (op) Re-read seven days of edit-heavy Claude Code sessions (at least 50 post-edit answers per session): per channel (`post-edit`, `cwd-changed`), require `suppressed_bytes / (emitted_bytes + suppressed_bytes) >= 0.40`; zero first-touch suppressions (the first injection for a key in a session is never suppressed); zero dedupe state errors that suppressed output. Use `hook-dedupe` rows' channel, harness, session, emitted/suppressed kind, bytes, suppressed_bytes, hashed key, first_touch and state_error; emitted_bytes is the sum of emitted rows' bytes. Count post-edit answers from `edit-hook/answered` with phase=post. Missing audit evidence is not a passing zero. Record method and evidence before the verdict. {id: CB-M2, oracle: lexical, path: docs/bench/readings/context-bill/after-dedupe.md, pattern: "^CB-M2 verdict: (PASS|FAIL)"}

Dedupe state: at most one size-bounded file per session under the existing fathom home hook-state mechanism; existing GC prunes files older than seven days. Read/write errors and absent SessionStart evidence emit the full injection. `[hooks] dedupe = false` disables the optimization (default on; a user false wins). Rule delivery is per context: Claude Code uses session plus agent_id; Codex uses session plus transcript_path when present, otherwise session alone. Both reset on pre-compact; other harnesses emit rules in full until their context and compaction signals are proven. Cwd primers remain Claude Code only. Rule identity includes its rendered bullet digest, so text, tag and staleness changes re-emit immediately. Each rule re-emits after 30 minutes or 10 consecutive suppressions; at capacity, the oldest rule entries are evicted first. Full emits commit only after stdout succeeds; empty payloads take no entry. Emitted and suppressed bytes are separate fields on the hook-dedupe rows.

Verify before claiming any row: `fathom verify --service fathom --plan docs/plans/context-bill.md`.

## Decision models

None applies; dedupe is a digest comparison.
