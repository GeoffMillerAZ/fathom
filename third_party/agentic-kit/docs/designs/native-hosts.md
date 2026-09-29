# Native host adapters

Path-enforcing policies also protect adapter metadata, native configuration and
CLAUDE.md/AGENTS.md and installed guard executables (including hardlink aliases)
from the governed file-tool path. Raw parent path components are rejected before
normalization, including in hook cwd. Deny paths use printable ASCII
(protect an ASCII-named ancestor for Unicode paths); project roots are canonicalized.
Case-folded paths and aliases of explicitly named existing targets are checked.
This does not find hardlinks to every descendant of a denied directory, copied bytes,
or filesystem races. This remains a hook observation,
not atomic filesystem confinement; native sandboxing and configuration trust are
separate requirements. Configuration-only policies do not claim path protection.

Status: implemented project-local CLI; live event coverage remains evidence-dependent.
Protocol version: 1. Baseline inspected: 2026-09-13, Claude Code 2.1.270 and
Codex CLI 0.154.0. This is the native-host lane, not an LLM provider adapter.

## Boundary and contract

`kit-new host plan|install|uninstall|doctor|probe|guard` extends the existing command;
the module generator remains unchanged. The adapter installs native hooks and a
managed instruction stanza. It neither replaces host permissions nor configures
trust, sandboxes, credentials, models, global settings, or permission bypasses.
Hooks are partial guardrails, not a sandbox or a proof of task completion.

| Host | Project hook file | Managed instruction file | Baseline gaps |
| --- | --- | --- | --- |
| Claude Code | `.claude/settings.json` | `AGENTS.md`, plus an `@AGENTS.md` import in an existing `CLAUDE.md` | Effective managed/user/CLI overrides and event enforcement need live observation |
| Codex CLI | `.codex/hooks.json` | `AGENTS.md` | Existing-session `write_stdin` and hosted tools do not receive a new local PreToolUse hook |

One instruction file: the stanza lives in `AGENTS.md` for both hosts and the installer never
creates a `CLAUDE.md`, because Claude Code (2.1.277 and later) reads `AGENTS.md` natively only
while no `CLAUDE.md`, `.claude/CLAUDE.md` or `CLAUDE.local.md` exists
([integration](../project-intent/integration.md) section 2). Where the project already has a root `CLAUDE.md`
with no standalone `@AGENTS.md` line outside a code fence, the install plan appends a marked import
block to it (an import, never a second copy of the stanza) and records it in ownership. The
baseline above (2.1.270) predates native `AGENTS.md` loading; that loading is documented, not
observed here, so `nested-instructions`, `startup` and `resume` stay unknown until live evidence.
`.claude/CLAUDE.md` and `CLAUDE.local.md` are not managed; doctor checks only the root file.

Both install `SessionStart` (`startup|resume|compact`), `PreToolUse` (all local tool
names), `PreCompact`, and `Stop` command hooks with a five-second timeout. Absolute,
shell-quoted binary/project paths keep nested working directories on the same
project policy. SessionStart supplies additional context; PreCompact and Stop are
observed but do not assert instruction retention, truthful stopping, or completion.
Nested instruction precedence, startup/resume delivery, compaction retention, and
subagent behavior are distinct live coverage questions, not implied by file presence.

## Policy and handler

The bounded JSON policy uses `version: 1`, optional `deny_tools` (case-sensitive Go
filepath globs against actual host tool names), `deny_paths` (clean project-relative
files/subtrees), `required_controls`, and `instructions`. Lists permit at most 128
unique nonempty entries of 512 bytes; instructions permit 8192 bytes. Inputs and
managed JSON are capped at 1 MiB and 64 nesting levels; duplicate keys, unknown
policy fields, malformed syntax, unknown controls, and path escapes are rejected.
[`schema.cue`](../../cmd/kit-new/schema.cue) is the authoring/interchange contract;
stdlib runtime checks enforce it and the additional path/byte constraints.

The guard pins policy bytes by SHA-256. Missing, changed, malformed, oversized, or
unreadable policy/input returns exit 2 with stderr and a native PreToolUse
`hookSpecificOutput.permissionDecision: "deny"` envelope. It never emits the
unsupported Codex `continue:false` denial. Allowed tools return `{}`, preserving
native permission decisions rather than emitting an unconditional allow.

Path decoding supports Claude Read/Edit/Write/MultiEdit/NotebookEdit and Codex
apply_patch's `tool_input.command`, including Move-to destinations. Symlink
components and paths outside the installed root are rejected. With any denied
path, arbitrary shell/stdin tools and tools without a verified file-path decoder
are denied conservatively; the adapter does not attempt to prove shell programs
safe by searching command text. Tool-only policies allow otherwise unlisted tools.
These rules govern only invocations the host actually delivers. A host user, other
process, configuration override, or unhooked tool can bypass them. Metadata is not
an adversarial security boundary and no cross-process filesystem lock is claimed.

## Installation and ownership

Plan output contains each complete before/after file, existence flags, and a
canonical SHA-256 digest. `install -expect DIGEST` recomputes and rejects a stale
review. Plans can contain existing private settings: do not publish them blindly.
Existing JSON keys and foreign hook entries survive semantically; whitespace is
normalized. Instruction text outside the exact managed stanza survives byte-for-byte.

Per-host `.agentic-kit/hosts/HOST/ownership.json` records original existence/config
digest, exact owned hook entries/stanza and the file holding it (`instructions_path`), the
exact appended `CLAUDE.md` import block when there is one, root, policy digest, and executable
digest. A record with no `instructions_path` predates the one-file layout: its Claude stanza is in
`CLAUDE.md`, and validate and uninstall read it from there.
The executable is caller-owned, regular, executable, at most 128 MiB, and never owned or
deleted. Identical installation is a no-op; replacement requires reviewed uninstall
and reinstall. Unowned policy files, duplicate/changed owned hooks, changed stanzas,
symlinks, special files, and changed policy bytes produce conflicts, not overwrites.

Writes compare every baseline, create exclusive stage files, preserve existing mode
bits, and rename into place. New files use 0600; new directories use 0700. Ownership
is written first and native hook activation last. Ordinary failures attempt rollback
only while generated bytes remain unchanged. This is not a multi-file atomic
transaction: interruption can leave an ownership record and incomplete install.
Doctor rejects it; uninstall recognizes original config plus partial generated files.
Uninstall removes exact generated entries/stanza/policy/ownership only, keeps foreign
changes, and leaves directories and the executable. Ambiguous drift requires human
reconciliation. Exclusive stage leftovers are visible conflicts; no recursive cleanup
or fallback removal is performed.

## Receipts and evidence

Doctor and probe are read-only and identical today: they inspect owned files and
execute installed `--version`/`--help` (plus Codex `features list`) in the project
directory. Each subprocess has a five-second deadline and 64 KiB output cap. Neither
command prompts a model or incurs API spend. Receipts report UTC time (injectable
clock), exact actual host version, help/config digests, tri-state controls, evidence
basis, and `ready`. Unknown host versions and failed/oversized/timed-out probes are
unknown, not certified by resemblance to the baseline.

Configuration digests include native hooks, instructions, policy, ownership, and
Claude local settings and `CLAUDE.md` or Codex project TOML. Doctor also fails a Claude
install whose root `CLAUDE.md` does not import `AGENTS.md`. Doctor checks executable hash/mode,
owned entries, obvious project hook disabling, and Codex's enabled hooks feature.
It does not parse all TOML or establish effective trust/global/managed/CLI settings.
Those remain live-observation scope. `ready` means configuration/runtime checks
passed AND every declared required control is supported. An empty required list
certifies configuration/runtime only, never event enforcement.

Event controls start unknown: startup, resume, nested-instructions, tool, edit,
shell, stdin, hosted-tools, compaction, stop, and hook-failure-closed. Codex stdin and
hosted-tools are statically unsupported; hook-failure-closed is unsupported for both
hosts because timeouts, malformed/disabled hooks, and startup trust gaps can fail open.
Required unsupported, unknown, missing, disabled, failed, or stale coverage cannot pass.

`-observations PROJECT_RELATIVE_JSON` imports separately gathered live evidence:
exact protocol/host/version/config digest, one unique status per control,
`method: "live-host"`, a bounded project-local artifact path and SHA-256, and detail.
Receipt basis is explicitly `supplied-live-observation`: the CLI verifies identity
and artifact integrity, not authenticity, test adequacy, freshness by wall clock, or
that a model actually ran. Config/version/artifact drift invalidates evidence;
static unsupported controls cannot be promoted by supplied evidence. Tests exercising
this transport are not real host observations. Use native process logs and independent
sentinel checks, not an agent's self-report, to justify supported event status.

## Verification and limits

`TestHost*` covers native output shape, startup/resume/compact inputs, nested paths,
edit/shell/stdin policy, patch moves, Stop, malformed/deep/duplicate/oversized input,
missing/drifted policy, symlinks, stale plans, interrupted install/rollback, foreign
ownership preservation, required unknown controls, evidence drift, process cancellation,
output bounds, disabled features, and unknown versions. All are hermetic/offline.
Actual local probes established only Claude `2.1.270 (Claude Code)`, Codex
`codex-cli 0.154.0`, version/help behavior, and Codex `hooks stable true`.
No live session event is certified by this implementation commit.

Official sources verified for this baseline:
[Claude hooks reference](https://code.claude.com/docs/en/hooks) and
[Codex hooks guide](https://learn.chatgpt.com/docs/hooks). Revisit after host upgrades,
new event/tool envelopes, changed trust/precedence, or a reproduced coverage failure.
