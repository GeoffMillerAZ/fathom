# Start here

fathom indexes your repositories once and answers *where is this*, *who calls
it*, *what does this change break* from the index instead of from a grep
sweep — and it gates "done" on evidence rather than on an agent's say-so.

This page is the 15-minute path and an honest account of what is proven and
what is not. The full guides are
[claude-code.md](claude-code.md) (canonical, build-through-verify),
[codex.md](codex.md), [other-harnesses.md](other-harnesses.md) (the
registry-generated adoption matrix: Gemini CLI, the generic `AGENTS.md`
tier, and what is verified vs. not), and
[spec-driven-development.md](spec-driven-development.md).

## Get it running

```sh
git clone <fathom repo> && cd fathom

# 1. build — no sibling checkouts needed, everything private is mirrored in-tree
CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=14.0 go build -o bin/fathom ./cmd/fathom   # macOS
# CGO_ENABLED=1 go build -o bin/fathom ./cmd/fathom                                # Linux

# 2. the daemon (macOS; elsewhere run `fathom serve-http` under your supervisor)
bin/fathom daemon install
bin/fathom daemon status          # must say "reachable"

# 3. register the MCP server with your agent, once (per harness — the matrix in
#    other-harnesses.md carries each harness's own line; Claude Code's is:)
claude mcp add --transport http --scope user fathom http://127.0.0.1:7575/rpc/agent

# 4. wire a repo — from that repo's root
cd /path/to/your/repo && fathom init --skills   # --harness codex|gemini-cli|agents|none for the others
fathom doctor                     # the checklist; every SKIP says why
```

`--skills` (bare) writes the nine shipped skills to `.agents/skills` (the
Agent Skills standard path — Codex, Gemini CLI, Cursor, Copilot, …) and
`.claude/skills` (Claude Code); `fathom plugin render` packages the same
nine with the hook template and the MCP registration as one plugin tree
carrying both the Claude Code and the Agent Plugins 1.0 manifests.

`fathom doctor` is the answer to "is this actually working". It never guesses:
an uncheckable state is `SKIP` with a reason, never a hopeful `PASS`.

## What is actually proven

- **The verify gate.** `fathom verify` re-checks every `[x]` in a plan against
  the index — the symbol exists, a test statically reaches it. The one figure
  for what it has caught is the `verify gate: figure:` line of
  `fathom research report`, computed from the verify ledger with its window
  and denominators in the sentence (PC-13). On the author's machine on
  2026-09-02 it read: *the verify gate refused 139 of 2,762 recorded runs over
  197 plans in 10 repos between 2026-08-15 and 2026-09-02* (a refusal is one
  run whose verdict carried at least one failing claimed row — never a sum of
  failing rows across runs). Run the verb; do not quote this sentence
  onward. This is the part that has earned its keep.
- **The index answers.** `find_symbol`, `find_callers`, `references`,
  `repo_map` over 20-odd languages. Ordinary, and it works.

## What is NOT proven, and where you should push hardest

- **No savings claim is admissible.** fathom measures what it SPENDS
  (`fathom research report`), never what it saves. The A/B that would license
  a savings claim is one-armed — the OFF arm has zero admissible sessions —
  so any "fathom saved N tokens" you infer is unsupported by this repo's own
  evidence. If you see a number that implies a saving, that is a bug worth
  reporting.
- **Single adopter until now.** Every measurement here comes from one
  developer on one machine. You are the second, which is the point.
- **The intent/evidence plane (ADR-026/027) is days old.** `fathom intent
  audit` reads what a repo declares about itself and lists what nothing
  proves. It found real defects in fathom immediately — including that its own
  reporting layer reported an unreadable file as zero findings. Treat it as
  promising, not settled.
- **Experimental surfaces**, off or opt-in by default: expert modules,
  findings, anchored memory, semantic layers, the intent corpus. `fathom
  doctor`'s `capabilities` row names which are operating.

## What would make your review most useful

1. **Try to make an instrument lie.** Break a file's permissions, point a
   config at a path that does not exist, delete a test a plan row names. Every
   check is supposed to say "I could not look" rather than report a clean
   result. Where that fails, it is the bug class this project cares most about.
2. **Judge the always-loaded cost.** fathom injects a session primer and
   per-edit obligations. `fathom research report` prints what that costs in
   tokens. Is it worth it? Say so plainly if not.
3. **Adopt it somewhere I have never run it.** A language, layout or workflow
   this repo has not seen is worth more than another pass over Go.
4. **Read the doctrine and argue with it.** `docs/intent/principles.md` (laws
   1–15) and `docs/intent/invariants.md` (each with an honest enforcement
   rung). The rules that are `stated` — nothing enforces them — are the ones
   most likely to be wrong.

## The shape of the thing

| Layer | What it is |
|---|---|
| `internal/connector` → `fusion` → `store` | ingest: discover, extract, fuse, persist (SQLite, pure Go) |
| `internal/mcp` | the agent surface — 17 tools |
| `cmd/fathom` | the CLI, the hooks, the daemon, the observatory |
| `docs/intent/` | what the repo promises about itself, and how each promise is enforced |
| `docs/plans/` | work orders whose `[x]` is gated by `fathom verify` |

Architecture: [`../architecture.md`](../architecture.md).
Every command: [`../usage.md`](../usage.md).
Per-concern how-tos: [`../use-cases/`](../use-cases/).
