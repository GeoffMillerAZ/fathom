// Package llm provides knowledge-card enrichment for fathom symbols.
//
// An Enricher holds an injected llmclient.LLMClient and a llmsec.Redactor.
// For every SymbolNode it builds a concise prompt, runs the DLP gate over the
// prompt text, calls the model, and parses the plain-text response into a
// KnowledgeCard (Summary + Tags).
//
// All LLM calls are deterministic in tests because the client is injected —
// pass a llmclient.NewMockClient(...) to get a network-free, reproducible run.
package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/llmsec"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// KnowledgeCard is the enriched summary produced for a single symbol.
type KnowledgeCard struct {
	// Symbol is the symbol name (mirrors SymbolNode.Name).
	Symbol string

	// File is the source file path (mirrors SymbolNode.FilePath).
	File string

	// Summary is a one-to-two sentence human-readable description of the symbol.
	Summary string

	// Tags is a flat list of topic labels parsed from the model response.
	Tags []string
}

// Enricher enriches SymbolNodes with LLM-generated KnowledgeCards.
// Create one with NewEnricher; the zero value is not usable.
type Enricher struct {
	client   llmclient.LLMClient
	redactor *llmsec.Redactor
}

// NewEnricher creates an Enricher backed by the given LLMClient and Redactor.
// Both arguments are required and must not be nil.
func NewEnricher(client llmclient.LLMClient, redactor *llmsec.Redactor) *Enricher {
	if client == nil {
		panic("llm.NewEnricher: client must not be nil")
	}
	if redactor == nil {
		panic("llm.NewEnricher: redactor must not be nil")
	}
	return &Enricher{client: client, redactor: redactor}
}

// Enrich builds a prompt from sym, redacts it through the DLP gate, calls the
// LLM, and parses the response into a KnowledgeCard.
func (e *Enricher) Enrich(ctx context.Context, sym core.SymbolNode) (KnowledgeCard, error) {
	prompt := buildPrompt(sym)

	// DLP gate: strip credentials / secrets before they leave the process.
	safePrompt, _ := e.redactor.Redact(prompt)

	req := llmclient.Request{
		Prompt:    safePrompt,
		MaxTokens: 256,
		System:    systemPrompt,
	}

	resp, err := e.client.Complete(ctx, req)
	if err != nil {
		return KnowledgeCard{}, fmt.Errorf("llm.Enrich %q: %w", sym.Name, err)
	}

	card := parseResponse(resp.Text)
	card.Symbol = sym.Name
	card.File = sym.FilePath
	return card, nil
}

// EnrichBatch enriches a slice of symbols sequentially. It returns an error on
// the first failure; successfully enriched cards up to that point are
// discarded to avoid returning a partial result without a clear signal.
func (e *Enricher) EnrichBatch(ctx context.Context, syms []core.SymbolNode) ([]KnowledgeCard, error) {
	cards := make([]KnowledgeCard, 0, len(syms))
	for _, sym := range syms {
		card, err := e.Enrich(ctx, sym)
		if err != nil {
			return nil, fmt.Errorf("llm.EnrichBatch: %w", err)
		}
		cards = append(cards, card)
	}
	return cards, nil
}

// systemPrompt is the system-level instruction sent with every enrichment call.
const systemPrompt = `You are a code documentation assistant. Given a Go symbol name and signature, produce:
1. A one-to-two sentence summary starting with "Summary: ".
2. A comma-separated list of topic tags on a line starting with "Tags: ".
Keep the output concise and technical. Do not include any other content.`

// buildPrompt constructs the user prompt from a SymbolNode.
func buildPrompt(sym core.SymbolNode) string {
	var sb strings.Builder
	sb.WriteString("Symbol: ")
	sb.WriteString(sym.Name)
	if sym.Kind != "" {
		sb.WriteString(" (")
		sb.WriteString(string(sym.Kind))
		sb.WriteString(")")
	}
	if sym.Signature != "" {
		sb.WriteString("\nSignature: ")
		sb.WriteString(sym.Signature)
	}
	if sym.FilePath != "" {
		sb.WriteString("\nFile: ")
		sb.WriteString(sym.FilePath)
	}
	sb.WriteString("\n\nDescribe this symbol.")
	return sb.String()
}

// parseResponse extracts Summary and Tags from the model's plain-text output.
// The expected format is:
//
//	Summary: <one or two sentences>
//	Tags: tag1, tag2, tag3
//
// Lines that do not match either prefix are appended to the summary. An empty
// or malformed response results in an empty Summary and no Tags — no error is
// returned; the caller can inspect the card fields.
func parseResponse(text string) KnowledgeCard {
	var card KnowledgeCard
	var summaryParts []string

	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		switch {
		case strings.HasPrefix(lower, "summary:"):
			summaryParts = append(summaryParts, strings.TrimSpace(line[len("summary:"):]))
		case strings.HasPrefix(lower, "tags:"):
			raw := strings.TrimSpace(line[len("tags:"):])
			if raw != "" {
				for _, tag := range strings.Split(raw, ",") {
					tag = strings.TrimSpace(tag)
					if tag != "" {
						card.Tags = append(card.Tags, tag)
					}
				}
			}
		default:
			summaryParts = append(summaryParts, line)
		}
	}

	card.Summary = strings.Join(summaryParts, " ")
	return card
}
