# agent-cli — the hello-world agent

The smallest **runnable** agent that proves the whole agentic-kit stack
end-to-end, and the recommended starting point to copy for new projects.

One binary, three modes, the exact same composition in every mode:

```
agent-harness (the governed loop)
 ├── tool-registry        three real tools: read_file, list_dir, word_count
 ├── conversation         transcript invariants (driven by the harness)
 └── llmclient            WithCost(WithTelemetry(WithRetry(<base>)))
       ├── mock           scripted MockClient            (no key, no network)
       ├── replay         ReplayClient over a cassette   (no key, no network)
       └── live           anthropic.Client               (ANTHROPIC_API_KEY)
      plus: telemetry/sink (JSONL events), cost-tracker (token/USD report),
            llmreplay.RecordingClient (-record writes a cassette)
```

The two filesystem tools are **confined to the working directory**: absolute
paths and `..` traversal are rejected with model-readable errors, the byte
checks are backed by `os.Root` (so symlink escapes are caught too), and
`read_file` output is capped at 64 KiB.

## Running it

Run from this directory — the tools sandbox to the working directory and the
default prompt asks about `testdata/sample.txt`:

```sh
cd examples/agent-cli

# 1. Mock: deterministic scripted session. No key, no network.
go run .

# 2. Replay: serve the committed cassette; every request is hash-matched
#    against the recording, so any drift fails loudly.
go run . -mode replay -cassette testdata/mock-session.jsonl

# 3. Live: the real Anthropic API. Refuses to start without the key;
#    prints a cost report on exit. The key is never printed or recorded.
export ANTHROPIC_API_KEY=sk-ant-...
go run . -mode live

# Record a live session into a cassette for later replay:
go run . -mode live -record -cassette my-session.jsonl
```

From the repo root, `task smoke-live` runs the live smoke (and prints `SKIP`
when `ANTHROPIC_API_KEY` is unset).

### Flags

| Flag         | Default                            | Meaning                                                |
| ------------ | ---------------------------------- | ------------------------------------------------------ |
| `-mode`      | `mock`                             | `mock`, `replay`, or `live`                            |
| `-cassette`  | —                                  | cassette path (required for replay; target of -record) |
| `-record`    | `false`                            | record the session via `llmreplay.RecordingClient`     |
| `-prompt`    | "How many words are in testdata/sample.txt? …" | the user task                               |
| `-model`     | `claude-opus-4-8`                  | model name (also the cost-table key)                   |
| `-max-turns` | `8`                                | max model calls per run                                |
| `-json`      | `false`                            | emit the full `RunResult` (incl. transcript) as JSON   |
| `-verbose`   | `false`                            | stream JSONL telemetry + cost report to stderr         |
| `-context-budget` | `0`                           | token budget for the transcript window (0 = off); requires `-compaction-journal` |
| `-compaction-journal` | —                         | JSONL file the evidence-compactor appends dropped messages to, verbatim, before summarising |

A run that stops on `max_turns`, `budget` or `max_tokens` prints its
**handoff** on stderr — `agent-cli: handoff: run stopped on max_turns after 1
turn(s); last tool read_file; tried [read_file]; resume from checkpoint:…` —
the typed `RunResult.Handoff` record the harness emits, so the next run (or
the operator) knows what was tried and where to resume.

Exit codes: `0` run completed, `1` startup/run failure (including governance
stops such as `max_turns`), `2` usage error.

## What each mode demonstrates

- **mock** — `llmclient.MockClient` scripts the *model* side of a two-tool-turn
  loop (read the file → count the words → answer). The tools genuinely
  execute on every run; only the completions are scripted. This is the shape
  of every hermetic agent test in the repo.
- **replay** — `llmreplay.ReplayClient` with `MatchByRequestHash()`: the n-th
  call gets the n-th recorded entry **and** must hash identically to the
  recorded request. Because the tools re-execute under replay, this proves
  prompt, system prompt, tool schemas, and tool *results* are all
  byte-faithful to the recording. Change any of them and the run fails with
  a hash mismatch instead of lying.
- **live** — `anthropic.New`, wrapped exactly per the documented composition
  `WithCost(WithTelemetry(WithRetry(...)))`, with a `costtracker.Report()` on
  exit. Prices in `examplePriceTable` are illustrative config — verify
  against the provider's pricing page before trusting a report.

## The committed cassette

`testdata/mock-session.jsonl` is the mock session recorded through the real
`RecordingClient` (synthetic text only, scrubbed on write — safe to commit).
Regenerate it after changing the script, system prompt, tool schemas, or
`testdata/sample.txt`:

```sh
go generate .
```

`TestCommittedCassetteIsCurrent` keeps it honest: a stale cassette fails the
test suite.

## Copying this as a new project

1. Copy this directory into your new repo and rename the module/package.
2. While developing against a local checkout of agentic-kit, point your
   `go.work` (or a `replace` directive) at it — see
   [docs/adopting.md](../../docs/adopting.md) for the full pattern.
3. Replace the three tools with your own (`newRegistry` is the template:
   `ObjectSchema` def + handler; handler errors become `is_error`
   tool_results, never Go errors).
4. Keep the testable-`run()` shape and the mock/replay tests: your agent
   stays testable with no key and no network from day one.
5. Switch on the governance seams as you need them — `Deps.Budget`
   (budget-guard), `Deps.Detector` (injection-detector), `Deps.Journal`
   (investigation-journal) — they wire in exactly like `Deps.Sink`.
