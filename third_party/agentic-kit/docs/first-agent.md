# First agent: zero to governed in 10 minutes

This walkthrough gets you from an empty directory to a working agent that
answers a question using tools, then shows the one-line switch to a real
model. Time budget: about 10 minutes to a working mock run; another minute
to go live with a real key.

No prior knowledge of CUE, agentic frameworks, or Anthropic APIs is assumed.

---

## Prerequisites

- Go 1.26 or later (`go version`)
- The `agentic-kit` module: `git clone https://github.com/geoffmilleraz/agentic-kit`
- No external dependencies are needed for the mock path (the module has zero
  external Go dependencies). For the live path you need an Anthropic API key
  in `ANTHROPIC_API_KEY`.

---

## The 10-minute path

The fastest route is to copy `examples/agent-cli`, which is the committed
hello-world agent. It already wires the full stack. Read its `main.go` first —
every line is commented. Then either work from there or use the snippet below.

### Option A: copy and run agent-cli (fastest)

```sh
cd examples/agent-cli
go run . -mode mock
```

You should see output like:

```
stop=completed turns=3 tool_calls=2 input_tokens=2153 output_tokens=125

testdata/sample.txt contains 9 words.
```

That is a governed agent run. No API key was needed. The model side was scripted
by `MockClient`; the tools (`read_file`, `list_dir`, `word_count`) executed for
real.

### Option B: build the minimal agent from scratch

Create a new directory outside the repo and put this in `main.go`. Every type
and function here is real — verified against the package sources:

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "strings"
    "time"

    toolregistry "github.com/geoffmilleraz/agentic-kit/foundations/agent/tool-registry"
    "github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
    agentharness "github.com/geoffmilleraz/agentic-kit/kits/agent-harness"
)

func main() {
    // 1. Register a tool: echo_words counts words in the given text.
    reg := toolregistry.New(toolregistry.Options{})
    reg.MustRegister(toolregistry.Tool{
        Def: llmclient.ToolDef{
            Name:        "echo_words",
            Description: "Count the words in the given text.",
            InputSchema: toolregistry.ObjectSchema(
                toolregistry.Prop{
                    Name: "text", Type: "string", Required: true,
                    Description: "Text to count words in.",
                },
            ),
        },
        Handler: func(_ context.Context, input json.RawMessage) (string, error) {
            var in struct {
                Text string `json:"text"`
            }
            if err := json.Unmarshal(input, &in); err != nil {
                return "", err
            }
            return fmt.Sprintf("%d words", len(strings.Fields(in.Text))), nil
        },
    })

    // 2. Script the model side with MockClient so no key is needed.
    //    Two entries: one tool-use turn, one final-text turn.
    client := llmclient.NewMockClient(
        llmclient.MockEntryWithResponse(llmclient.Response{
            Content: []llmclient.ContentBlock{
                llmclient.TextBlock("Let me count the words."),
                llmclient.ToolUseBlock("tu_1", "echo_words",
                    json.RawMessage(`{"text":"Hello governed world"}`),
                ),
            },
            StopReason: llmclient.StopToolUse,
        }),
        llmclient.MockEntryOK("The phrase has 3 words."),
    )

    // 3. Build the governed harness.
    h, err := agentharness.New(
        agentharness.Config{
            MaxTurns:         4,
            System:           "You are a helpful assistant with one tool.",
            MaxTokensPerCall: 1024,
        },
        agentharness.Deps{
            Client:   client,
            Registry: reg,
            Now:      time.Now,
        },
    )
    if err != nil {
        panic(err)
    }

    // 4. Run.
    result, err := h.Run(context.Background(), "How many words are in 'Hello governed world'?")
    if err != nil {
        panic(err)
    }
    fmt.Printf("stop=%s turns=%d\n%s\n", result.Stop, result.Turns, result.FinalText)
}
```

This compiles with `go run . -mod=mod` from a directory that imports
`github.com/geoffmilleraz/agentic-kit`. Illustrative output (this block is
NOT machine-pinned — unlike the agent-cli block above, which
`TestFirstAgentWalkthrough` diffs against the live CLI on every gate run,
this one describes a snippet you write yourself, so treat the exact numbers
as indicative):

```
stop=completed turns=2
The phrase has 3 words.
```

---

## Switching to the real Anthropic client

Replace the `MockClient` line with:

```go
import "github.com/geoffmilleraz/agentic-kit/foundations/llmclient/anthropic"

c, err := anthropic.New(anthropic.Config{
    DefaultModel: "claude-opus-4-8",
    // APIKey is read from ANTHROPIC_API_KEY when empty.
})
if err != nil {
    panic(err)
}
client := c
```

Then set your key and run:

```sh
ANTHROPIC_API_KEY=sk-ant-... go run .
```

No other change. The harness, the tools, and the governance wiring are
identical in both modes. That is the point of the seam.

---

## What the governance defaults protect you from

### Budget cap

Wire `agentharness.Deps.Budget` with a `budgetguard.Guard` to enforce
per-minute request counts, per-day request counts, and a per-day USD ceiling.
The guard is **fail-closed**: if it cannot determine that a charge is within
budget, the call does not happen and `RunResult.Stop` is `"budget"`. Without a
wired guard, requests are unlimited — fine for development, not for production.

```go
import budgetguard "github.com/geoffmilleraz/agentic-kit/foundations/governance/budget-guard"

budget := budgetguard.New(budgetguard.Config{
    MaxReqPerMin: 10,
    MaxReqPerDay: 200,
    MaxUSDPerDay: 1.00,
}, time.Now)

// Pass it to Deps:
agentharness.Deps{Budget: budget, ...}
```

When the day's budget is exhausted, `Run` returns a `nil` Go error and
`RunResult.Stop == agentharness.StopBudget` — a governance stop, not a crash.

### Injection screening

`Config.InjectionAction` defaults to `"block"` when `Deps.Detector` is wired.
Every tool result is scanned before it re-enters the conversation. A result
that matches a prompt-injection rule is replaced with an `is_error` block that
tells the model what happened; the raw payload never re-enters the
conversation. Without a wired `Detector`, screening is off — also fine for
development, but tool results from untrusted external sources should be
screened in production.

### Journal

Wire `agentharness.Deps.Journal` with an `investigationjournal.Journal` to
record a step-by-step run trace — `run-started`, one `turn-completed` per
model call, `run-ended` — at minimum. The journal is append-only, available
for inspection after the run on `RunResult.Final`, and can be persisted via
the journal's JSONL sink. Without it, the governed loop still runs; you just
have no trace.

---

## Where to go next

| Doc | What it covers |
|-----|---------------|
| [`docs/adopting.md`](adopting.md) | How to build a new project on the kit: recommended layout, the Config/Deps wiring pattern, observability |
| [`docs/catalog.md`](catalog.md) | Every package in one line — find the right foundation for your use case |
| [`docs/practices.md`](practices.md) | Agentic anti-patterns and which kit packages prevent them |
| [`AGENTS.md`](../AGENTS.md) | Hard rules, package shape, the gate (`task validate`) |
| `scenarios/governed-investigation/` | Seeded end-to-end blueprint: evidence → investigation → finding |
| `scenarios/governed-docextract-hitl/` | Seeded end-to-end blueprint: docextract + harness + hitl composing together |

### A note on code blocks in this document

Every code block above was verified by hand against the package sources in
this repository at the time of writing (`kits/agent-harness/types.go`,
`kits/agent-harness/harness.go`, `foundations/llmclient/mock.go`,
`foundations/agent/tool-registry/registry.go`,
`foundations/agent/tool-registry/schema.go`,
`foundations/llmclient/anthropic/config.go`). They reflect the actual exported
API and will compile against the current module. The examples/agent-cli
directory contains a fully runnable version of the same pattern if you want
something to diff against.
