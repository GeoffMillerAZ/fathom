# Reach: the plan (vision 38, ADR-049)

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/reach.md` proves it.
Design: `docs/design/reach.md` (section numbers below are its). Goal G16 (RH-M1..M3 here;
GL-M1 and GL-M4 in `docs/plans/governance-learning.md`). Promotes the plugin part of futures
F2; publishing stays the owner's Q4 call. Builds on `fathom plugin render` (IO-25, RP-6) and
the stranger check (RP-23).

## Wave 1: the default surface, the bundle, the stranger's five minutes

- [ ] The call-log reading behind the default: per tool, sessions that called it and answers grep could not give, over 30 days {id: RH-3, oracle: lexical, path: docs/bench/readings/reach/surface-calllog.md, pattern: "^Default surface proposal"}
- [ ] A default agent surface of at most eight tools and named opt-in groups (`review`, `explore`, `memory`, `system`, `admin`); no tool deleted (§3) {id: RH-1, symbol: defaultAgentSurface, package: internal/mcp, reachable_from: TestDefaultAgentSurfaceAtMostEight, needs: "RH-3"}
- [ ] `[surface] groups` in `.fathom/config.toml` and `POST /rpc/agent?groups=` select groups; `all` restores every tool {id: RH-2, symbol: surfaceGroupsFromConfig, package: internal/mcp, reachable_from: TestSurfaceGroupsOptIn, needs: "RH-1"}
- [ ] The breaking change recorded: `!` commit, `### Breaking`, and a migration note naming the one-line restore {id: RH-4, oracle: lexical, path: docs/upgrades/default-surface.md, pattern: "groups = ..all", needs: "RH-2"}
- [x] `plugin render` writes the default surface and group settings into both manifests; `pluginVersion` is driven by a digest of the rendered tree {id: RH-5, symbol: renderPluginSurface, package: cmd/fathom, reachable_from: TestPluginRenderCarriesDefaultSurface, needs: "RH-1"}
- [ ] Claude Code install test: a scratch HOME, the rendered tree installed by the documented local path, a session lists the tools and one answers {id: RH-6, oracle: lexical, path: docs/bench/readings/reach/install-claude-code.md, pattern: "^Installed and answered on", needs: "RH-5"}
- [ ] A second client install test (Copilot Agent Plugins or Cursor), same shape {id: RH-7, oracle: lexical, path: docs/bench/readings/reach/install-second-client.md, pattern: "^Installed and answered on", needs: "RH-5"}
- [x] A `--client cursor` render where Cursor's plugin shape differs {id: RH-8, symbol: renderCursorPlugin, package: cmd/fathom, reachable_from: TestPluginRenderCursor, needs: "RH-5"}
- [x] The stranger check times a first proven answer: ingest a fauxtown service, verify a two-row fixture plan, stop the clock on exit 0; per-step timings written as a reading (§5) {id: RH-9, oracle: lexical, path: scripts/stranger-check.sh, pattern: "first proven answer"}
- [ ] The README's Quickstart is executed as written by the stranger check; a step it does not state fails the check {id: RH-10, oracle: lexical, path: scripts/stranger-check.sh, pattern: "Quickstart", needs: "RH-9"}

**RH-6/RH-7 (2026-09-28):** left open — both need a live install into an actual
Claude Code / Cursor (or Copilot) session, which spends LLM budget on a nested
agent session (`docs/intent/autonomy.md`: ask first) and, for the second
client, a client this environment does not have installed. Not implemented as
a shortcut that only pings the MCP endpoint directly — that would not be the
"Installed and answered on ..." claim the row's own oracle names.

**RH-10 (2026-09-28), judged wrong as written:** the README's `## Quickstart`
section today is the CONTRIBUTOR gate (`task`, `task doctor`, `task validate`,
`task deploy`, `lefthook install`), not an adopter path — `task deploy` signs
with the `fathom-dev` identity and installs/restarts a real daemon under the
default launchd label, `task validate` runs the full `-race` gate (minutes,
not seconds). Executing it "as written" inside a headless stranger-check would
sign with a real identity and touch a real daemon/`~/.fathom`-shaped install
outside a throwaway HOME — exactly what this repo's own conventions (and the
worker instructions this row was implemented under) forbid. The row's premise
now points at the wrong section; `docs/adoption/README.md`'s 15-minute path
(`fathom init --skills`) is the actual stranger path RH-9 already exercises.
Left `[ ]` rather than silently reinterpreting "the README's Quickstart" to
mean a different section, or fabricating a lighter check that would not
actually prove "executed as written".

## Refused (recorded so they are not re-proposed)

- [-] Publish to a marketplace, a brew tap or a registry {id: RH-X1, disposition: deferred, reason: "the owner's Q4 call (F2); licence questions Q29 and Q30 come first", trigger: "Q4 answered for a public release"}
- [-] Delete tools to shrink the surface {id: RH-X2, disposition: refused, reason: "law 4 governs membership; the rest stay registered behind groups"}
- [-] A hosted install or remote daemon {id: RH-X3, disposition: refused, reason: "scope non-goal: no hosted anything"}

## Measurements (op)

- [ ] (op) RH-M1: stranger time from install to first proven answer at most 5 minutes on the prebuilt path (with and without the build reported) {id: RH-M1, oracle: lexical, path: docs/bench/readings/reach/stranger.md, pattern: "^RH-M1 verdict: PASS", needs: "RH-9,RH-10"}
- [ ] (op) RH-M2: the plugin installs and answers in Claude Code and one other client {id: RH-M2, oracle: lexical, path: docs/bench/readings/reach/install.md, pattern: "^RH-M2 verdict: PASS", needs: "RH-6,RH-7"}
- [ ] (op) RH-M3: default agent surface at most 8 tools, with the call-log reading behind each default and each group {id: RH-M3, oracle: lexical, path: docs/bench/readings/reach/surface.md, pattern: "^RH-M3 verdict: PASS", needs: "RH-1"}
- [ ] (op) RH-M4: tool-schema bytes per session before and after the default surface {id: RH-M4, oracle: lexical, path: docs/bench/readings/reach/surface.md, pattern: "^RH-M4 verdict: PASS", needs: "RH-1"}
- [ ] (op) RH-M5: the stack reading: peak memory, cold index time and index size for fathom and at least two peers on one corpus {id: RH-M5, oracle: lexical, path: docs/bench/readings/reach/stack.md, pattern: "^RH-M5 verdict: (PASS|FAIL)"}
