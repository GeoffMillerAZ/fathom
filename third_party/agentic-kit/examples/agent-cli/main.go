// Command agent-cli is the hello-world agent for agentic-kit: a small,
// runnable CLI that proves the whole stack end-to-end and is meant to be
// COPIED as the starting point for new projects (see README.md and
// docs/adopting.md).
//
// It composes, bottom-up:
//
//   - foundations/llmclient              — the LLM seam (MockClient in mock mode,
//     anthropic.Client in live mode) plus WithRetry
//   - foundations/llmclient/replay       — record/replay cassettes for hermetic runs
//   - foundations/llmclient/instrument   — per-call telemetry + cost middleware
//   - foundations/agent/tool-registry    — three real tools: read_file, list_dir,
//     word_count (the first two confined to the working directory), each
//     declaring its Effect (all read-only here)
//   - foundations/agent/evidence-compactor — opt-in with -context-budget: the
//     conversation Compactor that journals every dropped message verbatim to
//     -compaction-journal BEFORE summarising (practices §1)
//   - foundations/telemetry/{event,sink} — JSONL telemetry events
//   - foundations/cost/cost-tracker      — token/USD accounting
//   - kits/agent-harness                 — the governed agent loop tying it together
//     (which itself drives foundations/agent/conversation, so transcript
//     invariants hold by construction)
//
// Modes:
//
//	agent-cli -mode mock                                          # no key, no network
//	agent-cli -mode replay -cassette testdata/mock-session.jsonl  # hermetic replay
//	agent-cli -mode live                                          # real API (ANTHROPIC_API_KEY)
//	agent-cli -mode live -record -cassette my-session.jsonl       # record while live
//
// Run it FROM THIS DIRECTORY (examples/agent-cli): the tools are confined to
// the working directory and the default prompt asks about testdata/sample.txt.
//
// Exit codes: 0 = run completed; 1 = startup or run failure (including
// governance stops such as max_turns); 2 = usage error. A run that stops on
// budget, max_tokens or max_turns also prints its Handoff — what was tried,
// the last tool, the checkpoint to resume from — on stderr.
//
// Concurrency: a single-goroutine CLI — one Run per process; nothing here is
// shared across goroutines.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	evidencecompactor "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/agent/evidence-compactor"
	toolregistry "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/agent/tool-registry"
	costtracker "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/cost/cost-tracker"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient/anthropic"
	llminstrument "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient/instrument"
	llmreplay "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient/replay"
	jsonllog "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/persistence/jsonl-log"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/sink"
	agentharness "github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/agent-harness"
)

const (
	// apiKeyEnv names the environment variable consulted in live mode. The
	// key is read once, handed to the anthropic client config, and NEVER
	// printed, logged, or recorded (cassette entries hold domain structs
	// only — no headers, no keys).
	apiKeyEnv = "ANTHROPIC_API_KEY"

	// defaultModel is used when -model is not given. It is also the key the
	// cost tracker prices against (see examplePriceTable).
	defaultModel = "claude-opus-4-8"

	// systemPrompt must stay byte-stable: requests are hashed into replay
	// cassettes, so editing it invalidates testdata/mock-session.jsonl —
	// regenerate with `go generate ./...` (see the directive at mockScript).
	systemPrompt = "You are a careful file assistant. Answer questions about files in the working directory using the provided tools. Keep the final answer to one short sentence."

	// defaultPrompt is a task the three tools can actually answer.
	defaultPrompt = "How many words are in testdata/sample.txt? Use your tools."

	// telemetrySource attributes the llm_call events emitted by the
	// instrument middleware. The harness stamps its own events with
	// agentharness.TelemetrySource; both share one RunID per run.
	telemetrySource = "examples/agent-cli"

	// costLabel is the cost tracker bucket for this program's spend.
	costLabel = "agent-cli"

	// maxReadBytes caps read_file output so a huge file cannot blow up the
	// model's context window (or your bill).
	maxReadBytes = 64 * 1024

	// maxTokensPerCall caps every model response. Like systemPrompt it is
	// hashed into cassettes — change it and regenerate.
	maxTokensPerCall = 1024
)

// examplePriceTable prices the models this example might route to.
//
// THE NUMBERS ARE ILLUSTRATIVE (Anthropic list prices, USD per million
// tokens, as of mid-2026). costtracker ships NO prices by design: provider
// prices change without notice, and a stale table silently produces wrong
// costs. Treat this as config — verify against the provider's pricing page
// before trusting any report. Models absent from the table still record
// their tokens, just with zero cost (see llminstrument.WithCost).
func examplePriceTable() costtracker.PriceTable {
	return costtracker.PriceTable{
		"claude-opus-4-8":   {InputPerMTok: 5.00, OutputPerMTok: 25.00, CacheReadPerMTok: 0.50, CacheWritePerMTok: 6.25},
		"claude-sonnet-4-6": {InputPerMTok: 3.00, OutputPerMTok: 15.00, CacheReadPerMTok: 0.30, CacheWritePerMTok: 3.75},
		"claude-haiku-4-5":  {InputPerMTok: 1.00, OutputPerMTok: 5.00, CacheReadPerMTok: 0.10, CacheWritePerMTok: 1.25},
	}
}

// ---------------------------------------------------------------------------
// Tools
// ---------------------------------------------------------------------------

// confine validates a model-supplied path against the working-directory
// sandbox. It fails closed with clear, model-readable errors:
//
//   - absolute paths are rejected outright;
//   - any ".." element is rejected — no traversal, not even segments that
//     would cancel out ("a/../b"): nothing legitimate needs them and the
//     rule is trivial to explain to the model.
//
// The cleaned path is then only ever used through an *os.Root, which
// enforces the same boundary at the OS level — including symlink escapes,
// which string checks alone cannot catch.
func confine(p string) (string, error) {
	if strings.TrimSpace(p) == "" {
		return "", errors.New("path must not be empty")
	}
	if filepath.IsAbs(p) {
		return "", fmt.Errorf("absolute path %q rejected: paths must be relative to the working directory", p)
	}
	for part := range strings.SplitSeq(filepath.ToSlash(p), "/") {
		if part == ".." {
			return "", fmt.Errorf("path %q rejected: %q traversal would escape the working directory", p, "..")
		}
	}
	return filepath.Clean(p), nil
}

// readFile returns the contents of one regular file under root, capped at
// maxReadBytes (a truncation marker is appended when the cap bites).
func readFile(root *os.Root, path string) (string, error) {
	clean, err := confine(path)
	if err != nil {
		return "", err
	}
	st, err := root.Stat(clean)
	if err != nil {
		return "", err
	}
	if st.IsDir() {
		return "", fmt.Errorf("%q is a directory; use list_dir", path)
	}
	f, err := root.Open(clean)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// Read one byte past the cap so truncation is detected without ever
	// slurping the whole file.
	data, err := io.ReadAll(io.LimitReader(f, maxReadBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxReadBytes {
		return string(data[:maxReadBytes]) + fmt.Sprintf("\n[output truncated at %d bytes]", maxReadBytes), nil
	}
	return string(data), nil
}

// listDir returns the sorted entries of one directory under root,
// directories suffixed with "/". An empty path lists the working directory.
func listDir(root *os.Root, path string) (string, error) {
	if path == "" {
		path = "."
	}
	clean, err := confine(path)
	if err != nil {
		return "", err
	}
	f, err := root.Open(clean)
	if err != nil {
		return "", err
	}
	defer f.Close()
	entries, err := f.ReadDir(-1)
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return "(empty directory)", nil
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		names = append(names, name)
	}
	slices.Sort(names)
	return strings.Join(names, "\n"), nil
}

// newRegistry builds the tool table. Every tool follows the same recipe: an
// llmclient.ToolDef built with toolregistry.ObjectSchema (what the model
// sees) plus a Handler (what runs). Handler errors come back to the model as
// is_error tool_results — the model reads them and adapts; they are never Go
// errors and never end the run.
func newRegistry(root *os.Root) *toolregistry.Registry {
	reg := toolregistry.New(toolregistry.Options{
		// One governance knob shown here: every tool execution is bounded.
		// Per-tool overrides go on Tool.Timeout; argument-aware allow/deny
		// policies go on Options.Policy.
		DefaultTimeout: 10 * time.Second,
	})

	reg.MustRegister(toolregistry.Tool{
		Def: llmclient.ToolDef{
			Name:        "read_file",
			Description: "Read a text file relative to the working directory. Output is capped at 64 KiB.",
			InputSchema: toolregistry.ObjectSchema(
				toolregistry.Prop{Name: "path", Type: "string", Description: "Relative path of the file to read.", Required: true},
			),
		},
		// Every tool declares its Effect — the MCP annotation vocabulary —
		// so a harness under EffectStrict can dispatch the reads without a
		// human token and gate anything undeclared or destructive.
		Effect: &toolregistry.Effect{ReadOnlyHint: true, IdempotentHint: true},
		Handler: func(ctx context.Context, input json.RawMessage) (string, error) {
			var in struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return "", fmt.Errorf("invalid input: %w", err)
			}
			return readFile(root, in.Path)
		},
	})

	reg.MustRegister(toolregistry.Tool{
		Def: llmclient.ToolDef{
			Name:        "list_dir",
			Description: "List a directory relative to the working directory. Directories are suffixed with '/'. Omit path to list the working directory itself.",
			InputSchema: toolregistry.ObjectSchema(
				toolregistry.Prop{Name: "path", Type: "string", Description: "Relative path of the directory to list. Defaults to '.'."},
			),
		},
		Effect: &toolregistry.Effect{ReadOnlyHint: true, IdempotentHint: true},
		Handler: func(ctx context.Context, input json.RawMessage) (string, error) {
			var in struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return "", fmt.Errorf("invalid input: %w", err)
			}
			return listDir(root, in.Path)
		},
	})

	reg.MustRegister(toolregistry.Tool{
		Def: llmclient.ToolDef{
			Name:        "word_count",
			Description: "Count the whitespace-separated words in the given text. Pure: no filesystem access.",
			InputSchema: toolregistry.ObjectSchema(
				toolregistry.Prop{Name: "text", Type: "string", Description: "The text to count words in.", Required: true},
			),
		},
		Effect: &toolregistry.Effect{ReadOnlyHint: true, IdempotentHint: true},
		Handler: func(ctx context.Context, input json.RawMessage) (string, error) {
			// A pointer field distinguishes "absent" from "empty string", so
			// a missing required argument is a clear error, not a silent 0.
			var in struct {
				Text *string `json:"text"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return "", fmt.Errorf("invalid input: %w", err)
			}
			if in.Text == nil {
				return "", errors.New(`input field "text" is required`)
			}
			return strconv.Itoa(len(strings.Fields(*in.Text))), nil
		},
	})

	return reg
}

// ---------------------------------------------------------------------------
// Mock session
// ---------------------------------------------------------------------------

// The committed cassette is the mock session recorded through the real
// RecordingClient. Regenerate it after changing the script, the system
// prompt, the tool schemas, or testdata/sample.txt:
//
//go:generate go run . -mode mock -record -cassette testdata/mock-session.jsonl

// mockScript is the deterministic two-tool-turn session served by -mode mock:
// the scripted "model" reads testdata/sample.txt, counts its words with
// word_count, then answers in text. The tools genuinely execute on every
// run — only the model side is scripted. Usage numbers are synthetic but
// stable so the telemetry and cost pipelines report something real.
func mockScript() []llmclient.MockEntry {
	return []llmclient.MockEntry{
		llmclient.MockEntryWithResponse(llmclient.Response{
			Content: []llmclient.ContentBlock{
				llmclient.TextBlock("I'll read the file first."),
				llmclient.ToolUseBlock("toolu_mock_1", "read_file", json.RawMessage(`{"path":"testdata/sample.txt"}`)),
			},
			StopReason: llmclient.StopToolUse,
			Usage:      llmclient.Usage{InputTokens: 640, OutputTokens: 46},
		}),
		llmclient.MockEntryWithResponse(llmclient.Response{
			Content: []llmclient.ContentBlock{
				llmclient.TextBlock("Now I'll count the words."),
				llmclient.ToolUseBlock("toolu_mock_2", "word_count", json.RawMessage(`{"text":"The quick brown fox jumps over the lazy dog.\n"}`)),
			},
			StopReason: llmclient.StopToolUse,
			Usage:      llmclient.Usage{InputTokens: 712, OutputTokens: 58},
		}),
		llmclient.MockEntryWithResponse(llmclient.Response{
			Text:       "testdata/sample.txt contains 9 words.",
			StopReason: llmclient.StopEndTurn,
			Usage:      llmclient.Usage{InputTokens: 801, OutputTokens: 21},
		}),
	}
}

// ---------------------------------------------------------------------------
// Client construction
// ---------------------------------------------------------------------------

// baseClient builds the innermost client for the chosen mode. run wraps the
// result (record -> retry -> telemetry -> cost), so every mode exercises the
// exact same middleware stack.
func baseClient(mode, cassette, model, key string) (llmclient.LLMClient, error) {
	switch mode {
	case "mock":
		return llmclient.NewMockClient(mockScript()...), nil

	case "replay":
		entries, err := llmreplay.LoadFile(cassette)
		if err != nil {
			return nil, err
		}
		// MatchByRequestHash is the fidelity check: every request this run
		// makes must hash identically to the recorded one, so any drift in
		// the prompt, system prompt, tool schemas, or tool RESULTS (the
		// tools re-execute under replay!) is a loud error, never silence.
		return llmreplay.NewReplayClient(entries, llmreplay.MatchByRequestHash()), nil

	case "live":
		// The key was resolved by the caller (so tests can inject an empty
		// environment); it lives only inside the client from here on.
		c, err := anthropic.New(anthropic.Config{APIKey: key, DefaultModel: model})
		if err != nil {
			return nil, err
		}
		return c, nil
	}
	return nil, fmt.Errorf("unknown mode %q", mode)
}

// ---------------------------------------------------------------------------
// Testable main
// ---------------------------------------------------------------------------

// run is the testable entry point (the same pattern as cmd/endpoint-guard):
// flags in, exit code out, all I/O through the injected writers, environment
// through the injected getenv — so tests never touch the real process state.
func run(args []string, stdout, stderr io.Writer, getenv func(string) string) int {
	fs := flag.NewFlagSet("agent-cli", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		mode     = fs.String("mode", "mock", "client mode: mock, replay, or live")
		cassette = fs.String("cassette", "", "cassette path (required for -mode replay; the target of -record)")
		record   = fs.Bool("record", false, "record the session to -cassette via llmreplay.RecordingClient (mock or live mode)")
		prompt   = fs.String("prompt", defaultPrompt, "the user task")
		model    = fs.String("model", defaultModel, "model name (also the cost-table key)")
		maxTurns = fs.Int("max-turns", 8, "maximum model calls per run")
		jsonOut  = fs.Bool("json", false, "emit the RunResult as JSON on stdout")
		verbose  = fs.Bool("verbose", false, "stream telemetry events (JSONL) and the cost report to stderr")

		contextBudget     = fs.Int("context-budget", 0, "token budget for the transcript window the model sees (0 = no windowing); requires -compaction-journal")
		compactionJournal = fs.String("compaction-journal", "", "JSONL file the evidence-compactor appends every dropped message to, verbatim, before summarising (required with -context-budget)")
	)
	if err := fs.Parse(args); err != nil {
		return 2 // flag already printed the error
	}

	// ---- validate flag combinations (usage errors exit 2) -----------------
	switch *mode {
	case "mock", "replay", "live":
	default:
		fmt.Fprintf(stderr, "agent-cli: invalid -mode %q (want mock, replay, or live)\n", *mode)
		return 2
	}
	if *mode == "replay" && *cassette == "" {
		fmt.Fprintln(stderr, "agent-cli: -mode replay requires -cassette")
		return 2
	}
	if *record && *mode == "replay" {
		fmt.Fprintln(stderr, "agent-cli: -record cannot be combined with -mode replay (the replay IS the cassette)")
		return 2
	}
	if *record && *cassette == "" {
		fmt.Fprintln(stderr, "agent-cli: -record requires -cassette")
		return 2
	}
	if *contextBudget < 0 {
		fmt.Fprintf(stderr, "agent-cli: -context-budget must not be negative, got %d\n", *contextBudget)
		return 2
	}
	if *contextBudget > 0 && *compactionJournal == "" {
		fmt.Fprintln(stderr, "agent-cli: -context-budget requires -compaction-journal (dropped messages must land somewhere before they are summarised)")
		return 2
	}

	// ---- tools: confined to the working directory -------------------------
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "agent-cli: getwd: %v\n", err)
		return 1
	}
	root, err := os.OpenRoot(cwd)
	if err != nil {
		fmt.Fprintf(stderr, "agent-cli: open working directory: %v\n", err)
		return 1
	}
	defer root.Close()
	registry := newRegistry(root)

	// ---- base client per mode ----------------------------------------------
	var key string
	if *mode == "live" {
		key = getenv(apiKeyEnv)
		if key == "" {
			fmt.Fprintf(stderr, "agent-cli: live mode needs an Anthropic API key: set %s and re-run (the key is never printed or recorded)\n", apiKeyEnv)
			return 1
		}
	}
	base, err := baseClient(*mode, *cassette, *model, key)
	if err != nil {
		fmt.Fprintf(stderr, "agent-cli: %v\n", err)
		return 1
	}

	// ---- optional cassette recording ---------------------------------------
	if *record {
		if dir := filepath.Dir(*cassette); dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				fmt.Fprintf(stderr, "agent-cli: create cassette dir: %v\n", err)
				return 1
			}
		}
		f, err := os.Create(*cassette)
		if err != nil {
			fmt.Fprintf(stderr, "agent-cli: create cassette: %v\n", err)
			return 1
		}
		defer f.Close()
		// llmreplay.DefaultScrubber redacts secret-shaped text before any
		// entry hits disk, but cassettes are still sensitive artifacts:
		// review one before sharing it. testdata/mock-session.jsonl is
		// purely synthetic, which is why it is safe to commit.
		base = llmreplay.NewRecordingClient(base, f)
		fmt.Fprintf(stderr, "agent-cli: recording cassette to %s\n", *cassette)
	}

	// ---- observability -------------------------------------------------------
	// One JSONL sink serves both the client middleware (llm_call events) and
	// the harness (per-turn events). Without -verbose it encodes to
	// io.Discard, so the composition is byte-for-byte identical in every
	// mode — quiet runs differ only in where the lines go.
	telemetryOut := io.Discard
	if *verbose {
		telemetryOut = stderr
	}
	snk := sink.NewJSONLSink(telemetryOut)
	tracker := costtracker.New()

	// ---- the documented client composition -----------------------------------
	// WithCost(WithTelemetry(WithRetry(base))) — retry sits INNERMOST so
	// telemetry and cost observe one logical call (not one record per
	// attempt), and cost sits outside telemetry so measured durations
	// exclude bookkeeping. See foundations/llmclient/instrument's doc.go for
	// the full rationale.
	client := llminstrument.WithCost(
		llminstrument.WithTelemetry(
			llmclient.WithRetry(base, llmclient.RetryConfig{
				MaxAttempts: 3,
				BaseDelay:   500 * time.Millisecond,
				Jitter:      true,
			}),
			snk, telemetrySource, time.Now,
		),
		tracker, examplePriceTable(), costLabel,
	)

	// ---- the harness -----------------------------------------------------------
	// Deps left nil are features switched off, not omissions: Budget
	// (governance/budget-guard) for hard pre-call spend caps, Detector
	// (security/injection-detector) to screen tool results, Journal
	// (investigation/investigation-journal) for a human-readable run trace.
	// Wire them exactly like Sink when your agent needs them.
	cfg := agentharness.Config{
		MaxTurns:         *maxTurns,
		System:           systemPrompt,
		Model:            *model,
		MaxTokensPerCall: maxTokensPerCall,
		ContextBudget:    *contextBudget,
	}

	// ---- evidence-preserving compaction (opt-in) --------------------------------
	// With -context-budget the model sees a windowed view of the transcript.
	// The evidence-compactor makes that window a PROJECTION over ground
	// truth, never a replacement: every dropped message is appended verbatim
	// to -compaction-journal BEFORE the summary call, and the summary the
	// model sees carries a [COMPACTION: …] provenance marker pointing back
	// at it. The summary call rides the same governed client as every other
	// call, so it is retried, instrumented and costed identically.
	// WithEvidenceCompactor refuses a budget-less config — a compactor that
	// can never run is exactly the claimed-but-not-wired drift it exists to
	// remove.
	if *contextBudget > 0 {
		if dir := filepath.Dir(*compactionJournal); dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				fmt.Fprintf(stderr, "agent-cli: create compaction journal dir: %v\n", err)
				return 1
			}
		}
		jf, err := os.OpenFile(*compactionJournal, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			fmt.Fprintf(stderr, "agent-cli: open compaction journal: %v\n", err)
			return 1
		}
		defer jf.Close()
		ec, err := evidencecompactor.New(
			evidencecompactor.Config{Model: *model, MaxTokens: maxTokensPerCall},
			client, jsonllog.NewWriter(jf), time.Now,
		)
		if err != nil {
			fmt.Fprintf(stderr, "agent-cli: %v\n", err)
			return 1
		}
		if cfg, err = agentharness.WithEvidenceCompactor(cfg, ec); err != nil {
			fmt.Fprintf(stderr, "agent-cli: %v\n", err)
			return 1
		}
	}

	h, err := agentharness.New(
		cfg,
		agentharness.Deps{
			Client:   client,
			Registry: registry,
			Sink:     snk,
			Now:      time.Now,
		},
	)
	if err != nil {
		fmt.Fprintf(stderr, "agent-cli: %v\n", err)
		return 1
	}

	result, runErr := h.Run(context.Background(), *prompt)

	// ---- render -------------------------------------------------------------------
	// RunResult is ALWAYS meaningful — on error Stop is "error" and Messages
	// still carries the transcript so far — so render first, report after.
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			fmt.Fprintf(stderr, "agent-cli: encode result: %v\n", err)
			return 1
		}
	} else {
		fmt.Fprintf(stdout, "stop=%s turns=%d tool_calls=%d input_tokens=%d output_tokens=%d\n\n%s\n",
			result.Stop, result.Turns, result.ToolCalls,
			result.Usage.InputTokens, result.Usage.OutputTokens, result.FinalText)
	}

	// ---- handoff ------------------------------------------------------------------
	// A run that stopped on budget, max_tokens or max_turns ended before its
	// task did. The harness hands over a typed receipt — what was tried, the
	// last tool, the checkpoint to resume from — instead of a bare stop
	// cause; print it where the operator (or the next run's caller) will see
	// it. In -json mode it is already inside the RunResult above.
	if result.Handoff != nil && !*jsonOut {
		fmt.Fprintf(stderr, "agent-cli: handoff: %s\n", result.Handoff.Summary)
	}

	// ---- cost report on exit ---------------------------------------------------
	// Always printed in live mode (real money), and under -verbose in any
	// mode (the mock session carries synthetic usage so the pipeline is
	// visible end-to-end).
	if *mode == "live" || *verbose {
		fmt.Fprint(stderr, tracker.Report())
	}

	if runErr != nil {
		fmt.Fprintf(stderr, "agent-cli: %v\n", runErr)
		return 1
	}
	if result.Stop != agentharness.StopCompleted {
		fmt.Fprintf(stderr, "agent-cli: run stopped without completing: %s\n", result.Stop)
		return 1
	}
	return 0
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, os.Getenv))
}
