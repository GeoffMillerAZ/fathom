# Reach: one install, a small default surface, and a stranger's first five minutes

Status: accepted 2026-09-27 (ADR-049 decision 3). Vision: `docs/vision/38-proof-layer-roadmap.md`.
Plan: `docs/plans/reach.md` (RH-). Goal G16 (with GL-M1 and GL-M4 from
`docs/plans/governance-learning.md`). Promotes the plugin part of futures F2. Constraints
carried in: law 4 (no tool joins a surface without call-log justification; no net tool
growth), the agent-surface compatibility surface (`internal/mcp/surface.go`, `POST
/rpc/agent`), ADR-014 (surfaces and groups), `fathom plugin render` (IO-25, RP-6), the
stranger check (RP-23), I11 and I37 (docs and stanza match the registry), Q4 (publication is
the owner's call).

## 1. Problem

The landscape's fastest-growing tools installed with one command from a plugin marketplace
and started with a handful of tools; one found agents choose better with a single default
tool. Fathom renders a plugin but has never been installed from it in a second client, its
agent surface is 25 tools (19 lookups, five promoted verbs, `safe_delete_check`), and a stranger's path from nothing to a first proven answer has
never been timed.

## 2. Decisions

1. A default agent surface of at most eight tools; every other tool stays registered and is
   reachable by opting into a named group. Nothing is deleted by this decision (law 4).
2. The plugin bundle is the rendered plugin with the default surface, install-tested in Claude
   Code and one other client (Copilot's Agent Plugins, or Cursor). Publishing it is Q4.
3. The stranger's bar: install to first proven answer in at most 5 minutes, timed by the
   existing stranger check.
4. The stack claim ("leanest in the field") is measured on one shared corpus before it is
   repeated (RH-M5).

## 3. The default surface

Chosen by the answers grep cannot give (ADR-035) and the call log, not by preference. The
proposal, to be confirmed by RH-3's call-log reading before it lands:

| # | Tool | Why default |
|---|---|---|
| 1 | `find_symbol` | the most-called lookup; the entry to everything else |
| 2 | `symbol_context` | definition, callers, callees, tests in one answer |
| 3 | `find_callers` | "who calls this" at typed tier |
| 4 | `blast_radius` | "what breaks": read precision 1.00 on 9 audited calls |
| 5 | `obligations` | "what governs this file", with citations |
| 6 | `verify_plan` | "is it done", the gate |
| 7 | `pack_context` | concern-ranked context under a budget |
| 8 | `search` | grep with a receipt, so the agent never needs to leave |

Groups, opt-in by `.fathom/config.toml` `[surface] groups = [...]` or the plugin's settings:
`review` (review_diff, safe_delete_check, rename_impact, test_impact, localize), `explore` (outline,
repo_map, references, entrypoints, query_symbols, read_source), `memory` (memory_search,
discoveries, staleness_report), `system` (the system-graph verbs when they land), `admin`
(ingest, list_services, capabilities). `ingest` is served to hooks regardless of surface.

**Law 4 check.** The tool count is unchanged; the default shrinks. A tool moves into the
default only when RH-3's call-log reading shows sessions reaching for it through its group at
least 1 in 5 sessions that opted in; a tool leaves the default when fewer than 1 in 20
sessions call it (the ADR-049 revisit trigger).

**Compatibility.** The agent-surface allowlist is in the compatibility surface (scope lists
it as 19 tools; the registry holds 25 since ADR-035's promoted verbs). The change is
breaking for a client that relies on a tool leaving the default: it ships with a `!` commit,
a `### Breaking` line, a migration note naming `[surface] groups = ["all"]` as the one-line
restore, and `POST /rpc/agent?groups=all` for scripted callers.

## 4. The plugin bundle

`fathom plugin render` already writes one tree with two loaders (Agent Plugins 1.0 at the
root, Claude Code under `.claude-plugin/`), ten skills, the hook template and the MCP
registration. The work:

- render the default surface and group settings into both manifests;
- add a `--client cursor` output where Cursor's plugin shape differs;
- an install test per client: a scratch HOME, the rendered tree installed through the
  client's documented local-install path, a session started, the MCP tools listed and one
  tool answered; the test records the client version it passed on;
- `pluginVersion` bumps are driven by a digest of the rendered tree, so a template change
  cannot ship without a bump.

The bundle points at a local daemon (loopback). The first-run path starts one if none is
running, through the same code as `fathom init`.

## 5. The stranger's five minutes

The stranger check (RP-23) already copies the tracked tree to scratch, builds with
`GOWORK=off`, starts a throwaway daemon with a temp store and HOME, runs `fathom init
--harness=none --skills` and `fathom doctor`, and prints elapsed seconds. It gains:

1. a timed first proven answer: after init, ingest a fixture repository (fauxtown's
   checkout-service), run `fathom verify` on a two-row fixture plan with one true row, and stop
   the clock when it exits 0;
2. per-step timings (build, daemon ready, init, ingest, first answer) written to
   `docs/bench/readings/reach/stranger-<date>.md`;
3. the path a stranger follows is the README's Quickstart, executed as written; a step the
   README does not state fails the check.

Five minutes excludes the one-time Go build when a prebuilt binary exists; RH-M1 reports both
(with and without the build) and the threshold applies to the prebuilt path, which is what a
plugin user meets.

## 5.1 Two borrows from the Claude OS comparison (RH-11, RH-12)

The Claude OS comparison (`docs/assessments/2026-09-27-fathom-and-claude-os.md`, ADR-049 amendment 2026-09-29)
found that a friendlier first run is the cheapest way to lower fathom's biggest risk, being powerful but hard to
start with. Two of its borrows are accepted here, both serving goal G16:

- `fathom init --dry-run` prints every file init would write or change and writes nothing. A test holds its list
  equal to the touched set of the real run, so the preview cannot drift from the act (law 15: init writes only
  through `internal/wiring`, and every write leaves a receipt, so the set is the receipt's set).
- `fathom init --demo` wires a throwaway copy of the fauxtown fixture fleet (a temp HOME and store, no download)
  and reaches a first proven answer, so a newcomer sees value before touching a real repository.

Instrument: RH-M1 (stranger time) also times the demo path beside the prebuilt path; the dry-run equality is a test.
Not accepted now: "ready now, deeper later" (a fast first index usable at once while the typed pass runs behind it)
is RH-X4, deferred, because it adds background work before tenancy closes (ADR-049 decision 2). README screenshots
of the console and directory listings wait on F2.

## 6. The stack measurement (RH-M5)

On one shared corpus (this repository plus two fauxtown services), measure peak RSS or
`phys_footprint`, cold index time and on-disk index size for fathom and at least two peers
run locally from their own install instructions. Published as a reading with each tool's
version and command; no claim about the stack is repeated without it.

## 7. Tenancy and cost

The default surface lowers every session's tool-schema bytes (fewer schemas listed). The
install tests run in scratch under the stranger-check pattern, outside the module tree, never
touching the shared daemon.

## 8. Measurement

| Reading | Threshold |
|---|---|
| RH-M1 | stranger time from install to first proven answer at most 5 minutes on the prebuilt path |
| RH-M2 | the plugin installs and answers in Claude Code and one other client |
| RH-M3 | default agent surface at most 8 tools; the call-log reading behind each default and each group |
| RH-M4 | tool-schema bytes per session before and after the default surface |
| RH-M5 | the stack reading against at least two peers on one corpus |

## 9. Jev, weighed

- **Where it could help:** none; surface membership is a call-log count (law 4).
- **Where it cannot:** choosing tools for an agent (the harness does that).
- **Cost and measurement:** not applicable.

## 10. Refused

- Publishing to a marketplace, a brew tap or a registry without the owner's Q4 call.
- Deleting tools to shrink the surface (law 4 governs membership, not existence).
- A hosted install or a remote daemon.
- A stack claim without RH-M5.
- "Ready now, deeper later" before tenancy closes and cold ingest is shown to dominate stranger time (RH-X4, deferred).
