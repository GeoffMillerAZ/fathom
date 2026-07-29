# Research flywheel: fathom proving its own impact

**Status:** shipped, bundle schema v2. **Owns:** `fathom research report`,
`fathom research bundle`, `internal/research/bundle`,
`schema/research-bundle/`.

**Critic review (2026-07-28) — six findings, all fixed in this pass** (see
§5's per-guarantee mechanism and §4's data layout for the concrete changes):
Finding 1/4/6 (client-controlled Tool/Surface/bench-Arm strings reached the
bundle verbatim — proven end-to-end), Finding 2 (the salt lived inside the
shareable artifacts directory, next to the unsanitized report), Finding 3
(the bundle fabricated all-zero groups for instruments it never actually
read), Finding 5 (the latency percentile was floor-index, not nearest-rank,
biasing p95 low on small samples).

**The owner's ask, verbatim:** "get the research data we need to prove it's
working well... can fathom self collect this?... make an excellent
flywheel... maybe a cli for people to publish the findings if they want. It
would need to be sanitized and safe to share... local data/logs to learn
from our local projects too."

**No new MCP tool.** Doctrine 4 (docs/vision/11-dial-to-eleven.md §2):
"consolidate, don't grow." Everything here is CLI + local files, reading
instrumentation fathom already writes for other reasons.

## 1. What already existed (inventory, not invention)

Before this feature, fathom already collected five independent local
evidence streams, each built for a different reason, never aggregated:

| Stream | Writer | Location | Shape |
|---|---|---|---|
| MCP call log | `internal/mcp/calllog.go` (`CallLogger`) | `workspace/logs/mcp-calls.jsonl` (repo-scoped; the daemon's own default, configurable via `[daemon].call_log`) | JSONL, one row per `tools/call`: `ts, tool, service, group, surface, via, origin, off_surface, claims_attached, verdict_outcome, latency_ms, error` |
| Hooks beat log | `cmd/fathom/beats.go` (`emitBeat`) | `~/.fathom/hooks.jsonl` or `$FATHOM_HOOKS_LOG` (**per-user**, not per-repo — no service field) | JSONL, `heartbeat.Beat{channel, kind, at, meta}`; the edit-hook emits `seen`/`answered{emitted}`/`error` |
| Friction log | `cmd/fathom/observe.go` (`fathom observe`) | `.fathom/observations.md` (repo-scoped) | Append-only Markdown, `### O<N> — <symptom>` entries with symptom/tool/expected/actual/repro/root-cause/fix/status/impact — **no timestamp field** |
| Adoption gauge | `cmd/fathom/instrument_adoption.go` (`fathom instrument --adoption`) | reads the call log + observations.md | A snapshot report: surface traffic, top tools, zero-call tools, claims-attach hit rate, verify-gate catches (with a predates-instrumentation split), open-observation count |
| Bench artifacts | `scripts/context-bench.sh` / `internal/contextbench` | `workspace/artifacts/context-bench/results-<ts>.json` (repo-scoped, produced by the fathom repo's own falsifiability runs) | `RunResults{generated_at, results[], summary{arm: {task_count, success_rate, total_tokens, mean_tokens, mean_wall_ms}}}` |

`fathom doctor` layers a sixth, *live* signal on top (`obligations_served`,
`surface_traffic` via `server_stats`) by querying the daemon directly —
useful for "is fathom wired right now," not for a point-in-time evidence
report, so the research flywheel deliberately does not depend on it (§3).

**The gap this feature closes:** each stream answers a narrow question in
isolation. Nobody had asked, in one command, "over the last week, did
fathom's forced channels actually fire, did the gate actually catch
anything, and is there bench evidence backing the tokenomics story" — nor
had anyone built a sanitized way to let someone else see the answer.

## 2. The metrics catalog

Every row below states what is measured, from which stream, and — where the
signal genuinely doesn't exist yet — says so honestly instead of printing a
zero.

| Metric | Computable today? | Source | Honesty note |
|---|---|---|---|
| Injection coverage (share of edit/write events receiving an obligations payload) | **Yes**, but **per-user, not per-repo** | hooks beat log, `edit-hook` channel, `seen`/`answered` | The beat log carries no service dimension; `--service` cannot scope this number. Stated explicitly in every report. |
| Obligations served (payload actually emitted) | **Yes** | hooks beat log, `answered` with `meta.emitted=true` | Subset of "answered" — an answered-but-empty hook run doesn't count. |
| Obligations evicted | **No** | — | No eviction event is logged anywhere in fathom today. Printed as `"not measurable: <reason>"`, never a 0. |
| Fathom-share of discovery actions (fathom tool calls vs grep/read in the same transcript) | **No**, structurally | — | Fathom's call log only sees fathom's own tool calls; it has zero visibility into an agent's Bash/Read/Grep calls in the same session. The *numerator* (agent-surface fathom calls) is reported as context; the *share* is explicitly `"not measurable"`. |
| Verify verdict distribution + gate catches | **Yes**, where instrumented | call log, `verify_plan`/`review` rows carrying `verdict_outcome` | Rows from a binary built before T5-7 (no `verdict_outcome`) are counted separately as "predates instrumentation," never folded into a zero (the repo's existing honesty convention, `instrument_adoption.go`'s `gateCatchStats`). |
| Staleness / re-anchor events | **Partially** | call log, count of `tool=="staleness_report"` calls | A real, possibly-zero count when the log is readable. "Re-anchor" as a distinct event has no instrumentation anywhere in fathom — reported `"not measurable"`. |
| Per-surface call traffic | **Yes** | call log, `surface` field, windowed | |
| Observe-entry counts by category | **Partially** | `.fathom/observations.md`, entries grouped by their `Tool` field as a category proxy | Entries carry no timestamp, so counts are **not** windowed by `--since` — the whole file's history, every time. Stated explicitly. |
| Token/turn deltas (when a bench has run) | **Yes, when an artifact exists** | `workspace/artifacts/context-bench/results-*.json`, most recent file | If no bench has ever run in this repo, `"not measurable: no context-bench results artifact found"`. When one exists, its own `generated_at` is surfaced so a reader can judge staleness against the report window themselves — the report does not filter bench results by `--since`. |
| Latency (p50/p95) | **Yes** | call log, `latency_ms`, windowed | |

The unifying law (docs/vision/11-dial-to-eleven.md §2, law 3, "never
silent"): **an absent instrument prints a named reason, never a zero.** Every
field above that can be `nil`/absent is paired with a sibling `*_note`
string that is *always* populated — a present value with an empty note is a
real measurement; a `nil` value always has a reason attached.

## 3. Design choice: local files only, never the live daemon

`fathom research report` and `fathom research bundle` read **only local
files** — the call log, the hooks beat log, `.fathom/observations.md`, and a
bench results file if one exists. They never open the store, never query
`serve-http`, never call an MCP tool.

This is a deliberate simplification of the "if the daemon is unreachable,
degrade honestly per-metric" requirement: rather than building per-metric
daemon-fallback logic, the report has **no daemon dependency to fall back
from**. Every number it prints comes from a file already on disk, so daemon
reachability cannot affect it at all — the honest degradation is total, not
partial. This also satisfies "read-only over the logs/stores it consumes;
must not touch any live store as a writer" trivially: there is no store
handle anywhere in this feature.

The cost of this choice: numbers that only the live daemon can see —
`server_stats`'s in-memory, process-lifetime `surface_traffic` counters, or
`obligations_served`'s live probe — are out of scope here. `fathom doctor`
already covers that ground for "is fathom wired right now"; this feature
covers "what happened over a window," which is what the call log's own
persistent JSONL already exists to answer.

## 4. Data layout

All flywheel output is gitignored, under `workspace/` (the repo's own
convention), split by scope:

| Path | Scope | Written by | Notes |
|---|---|---|---|
| `workspace/logs/mcp-calls.jsonl` | repo (the daemon this repo's sessions register against) | `serve-http` (pre-existing) | Read-only input to this feature |
| `~/.fathom/hooks.jsonl` | **per-user**, cross-repo | the edit-hook (pre-existing) | Read-only input; see §2's coverage caveat |
| `.fathom/observations.md` | repo | `fathom observe` (pre-existing) | Read-only input |
| `workspace/artifacts/context-bench/results-*.json` | repo | `scripts/context-bench.sh` (pre-existing) | Read-only input |
| `workspace/artifacts/research/report.md` + `report.json` | repo | `fathom research report` | **New.** Overwritten each run — a snapshot, not an accumulating log. Markdown and JSON carry identical data. **Deliberately unsanitized**: carries plaintext service names, the hooks-log path (which embeds the local username on a default `~/.fathom/hooks.jsonl`), and other repo-identifying detail. Never share this file or its parent directory. |
| `workspace/artifacts/research/bundle/bundle.json` + `bundle.md` | repo | `fathom research bundle` | **New.** Same overwrite discipline. Lives in its own `bundle/` subdirectory (critic Finding 2) containing **only** these two files — so sharing that subdirectory in isolation is safe, but the *parent* `workspace/artifacts/research/` directory (which also holds `report.md`) must never be shared as a whole. The CLI prints this distinction after every write. |
| `workspace/state/research.salt` | repo | `fathom research bundle` (first run) | **New. Relocated in this pass (critic Finding 2)** — previously lived at `workspace/artifacts/research/.salt`, inside the same tree as the unsanitized report and the shareable bundle, where a naive "share the research folder" tar/zip/Finder copy would silently include it (an invisible dotfile) and make every service hash ever shared reversible by dictionary attack. `LoadOrCreateSalt` migrates an existing repo's legacy salt to the new path (preserving its value, so the hash lineage survives) and deletes the old file. A 32-byte random value, generated once, `0o600`, never read by any other command, never included in a bundle. Deleting it silently starts a new hash lineage — old and new bundles from the same repo become uncorrelated by service identity, which is safe (just less useful for trend-tracking) rather than unsafe. |

Nothing here is centralized or fleet-wide by design choice — the salt, the
report, and the bundle are all repo-scoped so that one repo's evidence never
mixes with another's, even though the hooks beat log itself (§2) is
unavoidably per-user today. The salt now also sits in a structurally
different subtree (`workspace/state/`) from anything meant to be shared
(`workspace/artifacts/research/`), so "share this directory" and "share the
salt" can no longer be the same mistake.

## 5. The privacy model (bundle only; the report is never shared automatically)

`fathom research report` is **not** sanitized — it is a full local artifact
meant to be read by the person running it, in the repo it was run in, and it
may legitimately be repo-identifying (its own `--service` flag names the
repo). It is never transmitted by fathom and the command prints nothing
suggesting it should be shared.

`fathom research bundle` is the opt-in, sanitized, shareable artifact. Its
guarantees, and the mechanism that enforces each one:

1. **Allowlist, not blocklist, enforced twice.**
   - **Go-side:** `internal/research/bundle.Bundle` is a plain struct with a
     fixed, small set of typed fields (aggregate counts, rates, durations,
     tool-name frequencies, verdict distributions, fathom's own
     version/commit, `GOOS`/`GOARCH`, and the report window). The *only*
     constructor is `bundle.Build(Input) Bundle`, and `Input` itself has no
     free-text field — a call-log `error` string, an observation's
     `symptom` text, or a file path has no field to be copied into, so it
     cannot reach the output type by construction. `Build` never accepts or
     copies a generic `map[string]any` from source data.
   - **CUE-side:** `schema/research-bundle/schema.cue`'s `#Bundle` is a
     **closed** struct — a stray field introduced by a future edit (e.g.
     someone adding a `RecentErrors []string` to `Bundle` without updating
     the schema) fails `Validate` at build time, before the file is ever
     written. `cmdResearchBundle` calls `Validate` and refuses to write (or
     print, on `--dry-run`) anything that fails it.
   - **Value shape, not just field shape (critic Finding 1, fixed).** Being
     typed `string` in Go is not the same as being fathom-controlled: a
     call-log `Tool`/`Surface` value is *client-controlled* —
     `internal/mcp/dispatch.go` logs the raw *requested* tool name even when
     it fails to resolve to a real tool, and the daemon that logs it is
     always-on and unauthenticated by design (bare `/rpc` serves the full
     registry to any local process). The critic proved this end-to-end: a
     fake path string placed in a call-log row's `tool` field reached a
     written, schema-*passing* bundle. Fixed at three independent layers:
     (a) `cmd/fathom/research_bundle.go`'s `sanitizeLabel` constrains every
     Tool/Surface/bench-Arm value to `^[a-z0-9_]{1,64}$` (Surface also
     allows `""`, its legitimate bare/unscoped value) before it ever reaches
     `Input`, bucketing anything else into the sentinel `"unrecognized"`;
     (b) `schema/research-bundle/schema.cue`'s `#Label` definition enforces
     the identical pattern, so a recipient's own independent validation
     catches a hostile value even if (a) had a bug; (c) adversarial fixtures
     (`TestBuild_HostileToolSurfaceArm_RejectedByValidate`,
     `TestAggregateBundleInput_SanitizesHostileToolSurfaceArm`,
     `TestCmdResearchBundle_Adversarial_SecretsNeverLeak`) seed hostile
     Tool/Surface/Arm rows — including a `|`-and-newline-carrying string
     that would otherwise break out of a rendered Markdown table cell
     (Finding 6) — and assert they never survive. Because every surviving
     value now matches `#Label`, it structurally cannot contain a
     table-breaking character, closing Finding 6 as a consequence rather
     than a separate fix.
2. **Never included:** file paths, repo/service names, symbol names, source
   code, commit SHAs of the *user's* repos, free-text observation notes,
   environment variables, usernames, hostnames. (Fathom's own version/commit
   — the binary that produced the bundle — is explicitly allowed and
   distinct from "the user's repo's commit.")
3. **Service identities as salted hashes.** A service/repo name that had
   call-log traffic in the window is never named in the bundle — it appears
   only as `sha256(salt + "\x00" + name)`, hex-encoded
   (`internal/research/bundle/hash.go`). The salt is generated once per
   repo (`workspace/state/research.salt`, `0o600` — **relocated outside the
   shareable artifacts tree in this pass, critic Finding 2**; see §4),
   reused across runs (so the *same* service hashes the *same* way, keeping
   per-service counts comparable across bundles), and is itself never a
   `Bundle` field — there is no code path by which it could appear in
   output. The hash construction itself (`sha256(salt + "\x00" + service)`,
   a 32-byte random salt, `0o600`) was judged sound by the critic and is
   unchanged; only its storage location moved.
4. **CUE schema ships with the binary and the repo**, so a recipient can
   validate a shared `bundle.json` independently, without trusting the
   sender's fathom binary:
   `schema/research-bundle/schema.cue` + `testdata/valid.cue` +
   `testdata/invalid.cue` (commented negatives, asserted in
   `internal/research/bundle/schema_test.go` — the repo's standard
   Go-asserted-negatives convention, never live-vetted).
5. **Review-first, no network I/O, ever.** `--dry-run` prints the exact
   bytes that would be written and exits — nothing is written, nothing is
   transmitted. The non-dry-run path writes two plain local files, into
   their own `bundle/` subdirectory (§4), and prints "these are plain local
   files... nothing was transmitted anywhere... sharing is entirely your own
   manual choice" **plus an explicit warning to share only the two bundle
   files (or the `bundle/` subdirectory in isolation) and never the parent
   `workspace/artifacts/research/` directory, which also holds the
   deliberately unsanitized `report.md`** (critic Finding 2c). The same
   warning is in `docs/use-cases/research-flywheel.md`. `fathom research
   bundle` contains zero HTTP client code, zero socket code — there is
   nothing in this feature capable of phoning home even if someone wanted
   it to.
6. **Never a fabricated zero, on the shared artifact too (critic Finding 3,
   fixed).** The local report already distinguished "not measurable" from a
   real zero (§2); the bundle did not — with no hooks log, it carried
   `injection_seen:0`/`injection_answered:0`/`obligations_served:0` as bare
   integers, indistinguishable from "measured zero coverage." Fixed by
   restructuring the call-traffic/injection/verify-gate fields from
   top-level scalars into three optional pointer groups
   (`Bundle.CallTraffic`/`Bundle.Injection`/`Bundle.VerifyGate`, bundle
   schema bumped `v1`→`v2`): a group is `nil` — self-describing, no separate
   `*_read` boolean needed — when its instrument was never actually read,
   and present (even with every value at zero) when it was. `Build` gates
   emission on `Input.CallLogRead`/`Input.HooksLogRead`, set by
   `cmd/fathom/research_bundle.go` from the same `readCallLog`/
   `readBeatLog` return values the local report already uses honestly.
7. **Adversarial tests, byte-level.** Both `internal/research/bundle`'s and
   `cmd/fathom`'s test suites seed fake-but-realistic secret-shaped strings
   (an API-key shape, an absolute path, an email, a hostname, an AWS-style
   access key) — and, per Finding 1, hostile Tool/Surface/bench-Arm values —
   into *every* consumed source — a call-log `error` field, a call-log
   `service`/`tool`/`surface` field, and every free-text field of an
   `.fathom/observations.md` entry — run the real aggregation + `Build` +
   render pipeline, and assert byte-for-byte that none of the seeded strings
   appear in either output file
   (`cmd/fathom/research_bundle_test.go`'s
   `TestCmdResearchBundle_Adversarial_SecretsNeverLeak`,
   `internal/research/bundle/adversarial_test.go`). A positive control
   (the hash *does* appear, the real counts *are* nonzero, an
   `"unrecognized"` tool/surface bucket *does* appear) rules out a vacuous
   pass. Salt stability, non-leakage, and legacy-path migration are covered
   separately in `hash_test.go`.

## 6. The flywheel loop

```
        ┌─────────────────────────────────────────────────────────┐
        │  fathom's own instrumentation (pre-existing, unchanged)   │
        │  call log · hooks beat log · observe · bench artifacts   │
        └───────────────────────────┬───────────────────────────────┘
                                     │ read-only, local files only
                                     ▼
                     fathom research report --since 7d
                                     │
                 workspace/artifacts/research/report.{md,json}
                                     │
              ┌──────────────────────┼──────────────────────┐
              ▼                      ▼                      ▼
   what's genuinely       what a claim can't      what to fix next —
   working (cite it)      support yet (retract/    file via `fathom
                          soften it — coding-      observe`, close the
                          agent-excellence.md's    loop when the fix
                          own §0.5 revision is     lands
                          the precedent)
                                     │
                                     ▼
                 fathom research bundle [--dry-run]
                                     │
           workspace/artifacts/research/bundle/{bundle.md,bundle.json}
                                     │
                    a human reviews it, then — and only
                    then — chooses whether to share it
```

This ties directly to doctrine law 6
(docs/vision/11-dial-to-eleven.md, "activation is stamped, never
prose-documented"): running `fathom research report` *is* the stamp — the
report's own `generated_at` field, and the file's mtime under
`workspace/artifacts/research/`, are the evidence that a measurement
actually happened at a point in time, the same discipline
`workspace/logs/*.stamp` files serve for the activation program. A
`docs/plans/*.md` item that claims "we measured the flywheel's impact" is
provable the same way any other claim is: point at the committed evidence
(here, a report artifact + its generation timestamp), not at prose asserting
it happened.

The loop is intentionally **not automatic**: nothing schedules
`research report` or `research bundle`, and nothing uploads a bundle
anywhere. The flywheel turns because someone runs it, reads it, and acts on
what it says — exactly the `fathom-impact` skill's existing honesty
discipline (§7), now with one more instrument to draw on.

## 7. Command reference

```sh
# Collect: aggregate the last 7 days of local evidence into one report.
fathom research report [--service NAME] [--since 168h] [--root .]
                        [--call-log PATH] [--hooks-log PATH] [--bench-dir DIR]
                        [--out DIR] [--json] [--no-write]
# -> workspace/artifacts/research/report.md, report.json

# Share (opt-in): build the sanitized bundle; review before doing anything with it.
fathom research bundle [--service NAME] [--since 168h] [--root .]
                        [--call-log PATH] [--hooks-log PATH] [--bench-dir DIR]
                        [--out DIR] [--dry-run]
# -> workspace/artifacts/research/bundle/bundle.md, bundle/bundle.json (or stdout, --dry-run)
```

`--since` uses Go's `flag.Duration` (matches `instrument.go`'s existing
`--window` flag) — e.g. `--since 720h` for 30 days. `--service` scopes the
call-log-derived numbers (surface traffic, verify gate, staleness, latency);
injection coverage stays per-user regardless (§2, §4).

## 8. What this deliberately does not do

- No new MCP tool (doctrine 4).
- No scheduling, no cron, no CI wiring — a human runs these commands.
- No aggregation across repos — each report/bundle is one repo's evidence.
  (Cross-repo trend-watching, if ever wanted, is a fold over multiple
  `report.json` files someone else's tooling can do — not this feature's
  job.)
- No attempt to infer causation from correlation (the `fathom-impact`
  skill's rule 3 applies here too: a high injection-coverage rate means
  hooks fired often, not that they changed an outcome).
