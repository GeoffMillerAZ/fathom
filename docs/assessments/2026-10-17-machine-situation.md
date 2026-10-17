# Machines, sessions and ports: what fathom should and should not understand

Owner-architect assessment, 2026-10-17, from the owner's brainstorm. The question was whether fathom should
understand local and remote machines (herdr's remote sessions, Claude Code and Codex over ssh) and help coordinate
several projects that collide on ports and other resources. It was read at fathom `f6bf2904` by five read-only
agents: the intent, the existing capability, the herdr and agent-client boundary, prior art, and a skeptic. The
records it produced are futures F82 to F85, one Preserve line on F15 and one non-goal note.

## Verdict

Fathom understands the *work situation*: what the code declares, which lines of work exist, what each one changes,
and whether what is running is the code being read. It does not manage the *machine situation*. Allocating ports,
supervising or reaping processes, managing sessions, multiplexing terminals and isolating containers are jobs other
tools already do well. They also cross a non-goal that is never revisited: fathom never watches, schedules, retries,
spawns or kicks off (`docs/intent/scope/non-goals.md`).

Four concepts pass that line. Each is read-only, uses the tools fathom already has and carries its own kill bar:

| Future | Concept | Value | Why only fathom |
|---|---|---|---|
| F82 | Line overlap: two live lines of work change the same file or the same symbol's callers | High | worklines plus `blast_radius` forecast a semantic conflict from git and the index. Session managers show where an agent is, not what its code touches. |
| F83 | Declared resources: ports, databases, sockets and directories as facts from config and code | Medium-high | Cross-repository facts give collisions, a runtime contract served through `obligations`, and runtime impact in `review_diff`. Allocators do not know the code. |
| F85 | The host cause behind an operational verdict | Medium | Exit 2, a dead gate or a failing doctor names a wedged volume, the disk floor, a held lease or memory pressure. Today unversioned bash guards these. |
| F84 | Listener provenance: which checkout and commit a running local server came from | Medium | It joins a listening pid, its working directory, the checkout and the commit. No tool found does this. It extends the port-holder join beyond fathom's own port. |

Free option: F15 now preserves a host id on ledger rows and receipts when they are touched, so readings from several
machines stay joinable. Nothing else about remote machines is built. A remote session runs its own fathom on that
host (Linux binaries ship since v0.13.0), and herdr already owns the list of machines.

## What already exists

- **Fathom:**
  - the holder of its own port, `cmd/fathom/portholder.go`;
  - host pressure, swap and swap-volume free, `cmd/fathom/doctor_tenancy.go` `checkHostPressure`;
  - its own footprint budget, `internal/tenancy`;
  - worklines, every unlanded line classed read-only (`internal/worklines`, ADR-060);
  - threads and prime;
  - a gate lease that queues concurrent pushes (`scripts/gate-lease.sh`);
  - the cloud-session path, `docs/plans/cloud-sessions.md`;
  - the planned worktree warm index, IXS-14.
- **What fathom lacks:** it has no view of other projects' ports, no model of live sessions, and no idea of another
  host.
- **herdr 0.9.3:**
  - workspaces, panes and agents with screen-scraped states;
  - the Claude or Codex session id and the cwd per agent;
  - `api snapshot` with a versioned schema;
  - a worktree list;
  - saved ssh machines and `--remote`;
  - metadata tokens another tool may report into its UI.
  
  It has no port awareness.
- **Claude Code:**
  - `session_id`, `cwd` and `agent_id` on every hook;
  - `WorktreeCreate`, `CwdChanged` and `SessionEnd`;
  - worktree and repository fields in the statusline;
  - `CLAUDE_CODE_REMOTE` and the Remote Control session variable.
  
  Hooks carry no hostname or ports.
- **Prior art:**
  - portless (vercel-labs; 0.15.8 adopted by the owner 2026-10-17) gives named `.localhost` URLs, injects `PORT`,
    is worktree-aware, lists its routes (`portless list`) and prunes orphaned dev servers (`portless prune`);
  - wtdev derives a port per worktree from a hash;
  - Conductor, claude-squad, Sculptor and container-use run agents in parallel and show their state;
  - Crystal is deprecated and Vibe Kanban is shutting down.
  
  None of them reports which resources an agent holds, and none joins a code index to a running process.

## Rejected, so it is not re-proposed

- **A port allocator or naming proxy.** portless owns this. The owner's port collisions are best solved today by
  adopting it, not by fathom.
- **A launcher, supervisor or orphan reaper, or a meter of the host's other tenants.** These cross the dispatch
  non-goal, and P23 meters fathom's own footprint only.
- **A session dashboard or registry.** The clients own sessions and spend (non-goals).
- **A new MCP tool for any of this.** No tool joins a surface without call-log justification.
- **The daemon reading or writing herdr's state.** If it ever happens, a hook inside a herdr pane does it. Writing
  herdr metadata is a display seam and waits on the mods readings (ADR-072).
- **Daemon federation or remote control across machines.** F3 and F15 already hold this.

## Decision models

None applies. All four concepts are deterministic joins over git, the index, config facts and one-shot process
reads. Stage 0's E1 and E2 (`docs/bench/readings/decide/`) found no fast-model answer better than the deterministic
or majority baseline on comparable questions, so no judged site is proposed. The provider-`none` fallback is the
whole design.
