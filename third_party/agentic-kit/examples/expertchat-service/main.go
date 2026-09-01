// Command expertchat-service is a thin, runnable example wiring
// kits/expertchat to a fathom module host. It is a COMPOSITION example, not
// a new server (docs/plans/gke-expertchat.md, GKE-EC-6 / H5): the governed
// answer loop itself lives entirely in kits/expertchat — see that package's
// doc comment for the full normative contract this composes.
//
// Two retriever modes:
//
//	expertchat-service -query "..."                                 # -retriever demo (default): a tiny built-in
//	                                                                 # corpus, no server needed
//	expertchat-service -query "..." -retriever http -url $URL       # a real module host's /rpc/modules surface
//	                                                                 # (docs/plans/gke-module-host.md, Phase G —
//	                                                                 # not yet built as of this writing; -url may
//	                                                                 # also come from EXPERTCHAT_MODULE_HOST_URL)
//
// Two LLM modes, the same seam examples/agent-cli uses:
//
//	-llm mock   (default; scripted MockClient — no key, no network)
//	-llm live   (real Anthropic client — ANTHROPIC_API_KEY, never printed or logged)
//
// Exit codes: 0 = a CitedAnswer was composed and printed; 3 = expertchat
// refused (a typed, non-error outcome — see kits/expertchat.Refusal); 1 =
// an operational failure (bad retriever/model call); 2 = usage error.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient/anthropic"
	expertchat "github.com/geoffmilleraz/agentic-kit/kits/expertchat"
)

const (
	// apiKeyEnv names the environment variable consulted in -llm live mode.
	// The key is read once, handed to the anthropic client config, and NEVER
	// printed, logged, or recorded.
	apiKeyEnv = "ANTHROPIC_API_KEY"

	// moduleHostURLEnv is the default source for -url.
	moduleHostURLEnv = "EXPERTCHAT_MODULE_HOST_URL"

	defaultModel = "claude-opus-4-8"

	defaultQuery = "What does the governed answer loop refuse to do?"
)

// demoCorpus is the built-in evidence corpus used by -retriever demo, so
// this example runs end-to-end with zero setup: no server, no key. It is
// small and self-describing on purpose — it demonstrates the loop's own
// shape, not a real fathom module.
func demoCorpus() []expertchat.EvidenceItem {
	return []expertchat.EvidenceItem{
		{
			Module: "agentic-kit", ModuleVersion: "v1",
			Watermark: "manifest:0000000000000000000000000000000000000000000000000000000000000000",
			ClaimID:   "refusal-contract",
			Tier:      "doc-claimed",
			Status:    expertchat.StatusFresh,
			SourceRef: "kits/expertchat/refusal.go",
			Anchor:    "Run returns a *Refusal wrapped as an error whenever it will not answer",
			Text:      "expertchat.Run refuses rather than answering from the model's own prior whenever retrieval returns no eligible evidence, evidence below the caller's tier threshold, or (with RequireFresh set) only stale evidence.",
		},
		{
			Module: "agentic-kit", ModuleVersion: "v1",
			Watermark: "manifest:0000000000000000000000000000000000000000000000000000000000000000",
			ClaimID:   "citation-contract",
			Tier:      "doc-claimed",
			Status:    expertchat.StatusFresh,
			SourceRef: "kits/expertchat/citedanswer.go",
			Anchor:    "NewCitedAnswer... fails when citations is empty",
			Text:      "A CitedAnswer with zero citations cannot be constructed through the public API: NewCitedAnswer rejects an empty citation slice with ErrNoCitations.",
		},
	}
}

// newRetriever builds the Retriever for the chosen mode.
func newRetriever(mode, url, module string) (expertchat.Retriever, error) {
	switch mode {
	case "demo":
		return &expertchat.FakeRetriever{Items: demoCorpus()}, nil
	case "http":
		if url == "" {
			return nil, fmt.Errorf("-retriever http requires -url (or %s)", moduleHostURLEnv)
		}
		return &expertchat.HTTPRetriever{URL: url, Module: module}, nil
	}
	return nil, fmt.Errorf("unknown -retriever %q (want demo or http)", mode)
}

// mockScript is the deterministic scripted model response used by -llm mock:
// it cites whichever claim ids the demo corpus actually offers, so the mock
// path exercises the exact same citation-validation logic as -llm live would.
func mockScript(items []expertchat.EvidenceItem) []llmclient.MockEntry {
	tokens := ""
	for i, it := range items {
		if i > 0 {
			tokens += ", "
		}
		tokens += it.Module + "#" + it.ClaimID
	}
	return []llmclient.MockEntry{
		llmclient.MockEntryOK("ANSWER: The governed answer loop refuses to answer without eligible, citable evidence — it never falls back to the model's own prior.\nCITES: " + tokens),
	}
}

// newLLMClient builds the llmclient.LLMClient for the chosen mode.
func newLLMClient(mode, model, key string, demoItems []expertchat.EvidenceItem) (llmclient.LLMClient, error) {
	switch mode {
	case "mock":
		return llmclient.NewMockClient(mockScript(demoItems)...), nil
	case "live":
		return anthropic.New(anthropic.Config{APIKey: key, DefaultModel: model})
	}
	return nil, fmt.Errorf("unknown -llm %q (want mock or live)", mode)
}

// run is the testable entry point (the cmd/endpoint-guard / examples/agent-cli
// pattern): flags in, exit code out, all I/O through the injected writers,
// environment through the injected getenv.
func run(args []string, stdout, stderr io.Writer, getenv func(string) string) int {
	fs := flag.NewFlagSet("expertchat-service", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		query         = fs.String("query", defaultQuery, "the question to ask")
		retrieverMode = fs.String("retriever", "demo", "retriever mode: demo (built-in corpus, no server) or http")
		url           = fs.String("url", "", "module-host JSON-RPC URL (retriever=http); defaults to $"+moduleHostURLEnv)
		module        = fs.String("module", "", "scope the http retriever to one module; empty searches every mounted module")
		llmMode       = fs.String("llm", "mock", "LLM mode: mock (no key, no network) or live ($"+apiKeyEnv+")")
		model         = fs.String("model", defaultModel, "model name (live mode only)")
		tierThreshold = fs.String("tier-threshold", "", "minimum evidence tier required to cite (empty = no floor)")
		requireFresh  = fs.Bool("require-fresh", false, "refuse rather than cite stale evidence")
		jsonOut       = fs.Bool("json", false, "print the result as JSON instead of Render()'d text")
	)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *query == "" {
		fmt.Fprintln(stderr, "expertchat-service: -query must not be empty")
		return 2
	}

	hostURL := *url
	if hostURL == "" {
		hostURL = getenv(moduleHostURLEnv)
	}
	retriever, err := newRetriever(*retrieverMode, hostURL, *module)
	if err != nil {
		fmt.Fprintf(stderr, "expertchat-service: %v\n", err)
		return 2
	}

	var key string
	if *llmMode == "live" {
		key = getenv(apiKeyEnv)
		if key == "" {
			fmt.Fprintf(stderr, "expertchat-service: -llm live needs an Anthropic API key: set %s and re-run (the key is never printed or recorded)\n", apiKeyEnv)
			return 1
		}
	}
	client, err := newLLMClient(*llmMode, *model, key, demoCorpus())
	if err != nil {
		fmt.Fprintf(stderr, "expertchat-service: %v\n", err)
		return 1
	}

	deps := expertchat.Deps{
		Retriever: retriever,
		Client:    client,
		Model:     *model,
		Policy: expertchat.Policy{
			// scoring.Tier is a true string ALIAS (not a defined type), so a
			// plain string assigns directly — see foundations/review/scoring.
			TierThreshold: *tierThreshold,
			RequireFresh:  *requireFresh,
		},
		Now: time.Now,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	answer, runErr := expertchat.Run(ctx, deps, *query)
	if runErr != nil {
		var refusal *expertchat.Refusal
		if errors.As(runErr, &refusal) {
			if *jsonOut {
				enc := json.NewEncoder(stdout)
				enc.SetIndent("", "  ")
				_ = enc.Encode(refusal)
			} else {
				fmt.Fprintf(stdout, "REFUSED (%s): %s\n", refusal.Reason, refusal.Detail)
			}
			return 3
		}
		fmt.Fprintf(stderr, "expertchat-service: %v\n", runErr)
		return 1
	}

	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		out := struct {
			Text      string                `json:"text"`
			Citations []expertchat.Citation `json:"citations"`
		}{Text: answer.Text(), Citations: answer.Citations()}
		if err := enc.Encode(out); err != nil {
			fmt.Fprintf(stderr, "expertchat-service: encode result: %v\n", err)
			return 1
		}
	} else {
		fmt.Fprintln(stdout, answer.Render())
	}
	return 0
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, os.Getenv))
}
