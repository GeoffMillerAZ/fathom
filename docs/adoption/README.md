# Adoption guides

fathom indexes your repositories once and answers *where is this*, *who calls
it* and *what does this change break* from the index instead of from a grep
sweep, and it gates "done" on evidence rather than on an agent's say-so.

To install fathom and start its daemon, follow
[docs/early-testers.md](../early-testers.md). This page is the short path for
wiring a repository once fathom runs, an account of what is proven and what is
not as of v0.11.0, and the index of the adoption guides.

## Wire a repository

```sh
git clone https://github.com/GeoffMillerAZ/fathom.git && cd fathom

# 1. build: no sibling checkouts; agentic-kit and app-kit are mirrored in third_party/
CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=14.0 go build -o bin/fathom ./cmd/fathom   # macOS
# CGO_ENABLED=0 go build -o bin/fathom ./cmd/fathom                                # Linux (pure Go)

# 2. the daemon, in its own terminal (any OS; launchd for daily macOS use: early-testers.md)
mkdir -p ~/.fathom/db
bin/fathom serve-http --store ~/.fathom/db/fathom.sqlite --idle-exit=0 --ttl=0 --auto-refresh=true

# 3. register the MCP server with your agent, once (Claude Code shown; other
#    harnesses: other-harnesses.md)
claude mcp add --transport http --scope user fathom http://127.0.0.1:7575/rpc/agent

# 4. wire a repository, from its root, with the built binary on your PATH
cd /path/to/your/repo && fathom init --skills   # --harness codex|gemini-cli|cursor|copilot|agents|none
fathom doctor                                   # the checklist; every SKIP says why
```

`--skills` installs the ten shipped skills into `.claude/skills` and links
`.agents/skills` (the Agent Skills standard path that Codex, Gemini CLI,
Cursor and Copilot read) to it. `fathom plugin render` packages the same ten
with the hook template and the MCP registration as one plugin tree.

`fathom doctor` answers "is this actually working". An uncheckable state is
`SKIP` with a reason, never a hopeful `PASS`. In v0.11.0 a correct fresh
install still fails three rows that are not wiring faults: `lefthook_gates`
(no `lefthook.yml` with a verify step), `obligations_served` (the rules layer
is not compiled yet; compiling spends LLM tokens) and `surface_traffic` (no
tool call has reached the daemon yet).

## What is proven

- **The verify gate.** `fathom verify` re-checks every `[x]` in a plan against
  the index: the symbol exists and a test statically reaches it. The figure
  for what it has caught is the `verify gate: figure:` line of `fathom research
  report`, computed from the verify ledger with its window and denominators.
  On the author's machines on 2026-09-02 it read: *the verify gate refused 171
  of 3,426 recorded runs over 317 plans in 21 repos between 2026-08-15 and
  2026-09-02*. That is one adopter's ledger; run the verb rather than quote the
  sentence.
- **The index answers.** The default surface serves eight tools
  (`find_symbol`, `symbol_context`, `blast_radius`, `review_diff`,
  `obligations`, `verify_plan`, `search`, `read_source`). How far each
  language and tool format extracts is the generated board in
  [docs/languages.md](../languages.md): a level is claimed only from a
  committed reading against an independent oracle. L4 (typed edges) outside
  Go and CUE needs that language's own SCIP indexer, imported with `fathom
  ingest --scip`.
- **The gate knows when it cannot see.** A verdict records which binary built
  the store it read, so a daemon behind your checkout reads `measured-stale`,
  a coverage gap, instead of producing failures that look real. An instrument
  that reports a clean result over a broken reading is the bug class this
  project treats as most serious.
- **Answers a stranger can check.** Every SARIF file fathom writes names the
  commit it was produced at and fingerprints each cited excerpt, so it can be
  checked with git alone. `fathom receipt verify` re-derives a saved Go answer
  from a bare checkout, offline ([usage/receipt.md](../usage/receipt.md)).
- **The harness is data, not Claude Code literals** (ADR-029). One seam
  (`internal/harness`) carries every harness's hook table, envelopes and
  vehicles; `fathom harness capabilities --json` says, per harness and per
  capability, which vehicle is used and whether a session has verified it.

## What is not proven

- **No savings claim.** fathom measures what it spends (`fathom research
  report`), never what it saves. The A/B that would support a savings claim has
  not produced an admissible OFF arm, so any "fathom saved N tokens" inferred
  from this repository is unsupported. A number that implies a saving is a bug
  worth reporting.
- **One adopter so far.** Every measurement here comes from one developer's
  machines, mostly macOS. Linux is covered by the CI gate and a cloud-session
  reading, not by daily use.
- **Harness coverage.** Claude Code and Codex are verified by live sessions
  (Codex with codex-cli 0.153.1 on 2026-09-12). Gemini CLI, Cursor and Copilot
  are registered and wired but no session has verified them; `fathom harness
  list` shows the state of each.
- **The change bench is a method, not a score.** The replicated and public
  runs spend LLM tokens and have not been run; read `docs/bench/` as an
  instrument.
- **The intent corpus is young.** `fathom intent audit` reads what a
  repository declares about itself and lists what nothing proves. It found
  real defects in fathom at once. Treat it as promising, not settled.
- **Experimental surfaces**, off or opt-in by default: expert modules,
  findings, semantic layers, the intent corpus. `fathom capabilities` shows
  which are operating here and why the rest are off.
- **Known limits** of this release are listed in
  [early-testers.md](../early-testers.md#known-limits).

## How to evaluate it

1. **Try to make an instrument lie.** Break a file's permissions, point a
   config at a path that does not exist, delete a test a plan row names. Every
   check should say "I could not look" rather than report a clean result.
2. **Judge the always-loaded cost.** fathom injects a session primer and
   per-edit rules. Measured on 1,918 edit payloads, the median was 131 tokens
   ([measurements/2026-08-27-obligation-ceiling.md](../measurements/2026-08-27-obligation-ceiling.md));
   `fathom research report` prints yours.
3. **Adopt it somewhere it has not run.** A language, layout or workflow this
   repository has not seen is worth more than another pass over Go.
4. **Read the doctrine and argue with it.** `docs/intent/principles.md` and
   `docs/intent/invariants.md` (each invariant with its enforcement rung). The
   rules that nothing enforces are the ones most likely to be wrong.

## The shape of the thing

| Layer | What it is |
|---|---|
| `internal/connector` → `fusion` → `store` | ingest: discover, extract, fuse, persist (SQLite, pure Go) |
| `internal/mcp` | the MCP server: 25 tools, 8 on the default `/rpc/agent` surface |
| `internal/harness` | every harness as data: descriptors, codecs, vehicles, verification state |
| `cmd/fathom` | the CLI, the hooks, the daemon, the observatory |
| `docs/intent/` | what the repository promises about itself, and how each promise is enforced |
| `docs/plans/` | work orders whose `[x]` is gated by `fathom verify` |

Architecture: [`../architecture.md`](../architecture.md).
Every command: [`../usage.md`](../usage.md).
Per-concern how-tos: [`../use-cases/`](../use-cases/).

## The guides

| Guide | Covers |
|---|---|
| [claude-code.md](claude-code.md) | Claude Code, build through verify (the canonical guide) |
| [codex.md](codex.md) | Codex: registration, the user-global hooks, the capability gaps |
| [other-harnesses.md](other-harnesses.md) | the registry-generated matrix: Gemini CLI, Cursor, Copilot, the generic `AGENTS.md` tier |
| [cloud-sessions.md](cloud-sessions.md) | Claude Code on the web and Codex cloud: a committed setup script, a VM-local daemon |
| [spec-driven-development.md](spec-driven-development.md) | plans whose boxes `fathom verify` gates |
| [github-rulesets.md](github-rulesets.md) | making `fathom verify` and compiled rules required GitHub checks |
| [github-action.md](github-action.md) | `fathom review --format github`: one deterministic PR comment and a check |
| [findings-loop.md](findings-loop.md) | an external review kit's findings, imported and served at the edit |
