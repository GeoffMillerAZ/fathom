package evidencecompactor

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	jsonllog "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/persistence/jsonl-log"
	prompttemplate "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/prompt/prompt-template"
)

// summaryTemplate is the package-level versioned prompt template. It is
// initialised once at package load time; a parse failure here is a
// programming error in the package itself and panics.
var summaryTemplate = prompttemplate.Template{
	Name:    "evidence-compactor-summary",
	Version: SummaryTemplateVersion,
	Text:    summaryTemplateText,
}

// Compactor is an evidence-preserving implementation of the
// conversation.Options.Compactor signature. It journals dropped messages
// verbatim before calling the LLM, embedding provenance in the returned
// summary message so summaries are traceable to ground truth.
//
// Construct with New. A *Compactor is safe for concurrent use.
type Compactor struct {
	cfg     Config
	client  llmclient.LLMClient
	journal *jsonllog.Writer
	now     clockFunc
}

// New returns a *Compactor ready to use as conversation.Options.Compactor.
//
//   - client must be non-nil: it receives the summary LLM call.
//   - journal must be non-nil: every compaction writes the dropped messages
//     here as a JournalRecord (JSONL line) BEFORE calling the LLM. The
//     caller owns the underlying io.Writer; no file I/O is performed by
//     this package.
//   - now may be nil: a nil clock defaults to time.Now (documented seam).
//   - cfg carries optional model/system/maxTokens overrides.
func New(cfg Config, client llmclient.LLMClient, journal *jsonllog.Writer, now clockFunc) (*Compactor, error) {
	if client == nil {
		return nil, fmt.Errorf("evidencecompactor: client must be non-nil")
	}
	if journal == nil {
		return nil, fmt.Errorf("evidencecompactor: journal must be non-nil")
	}
	if now == nil {
		now = time.Now
	}
	return &Compactor{cfg: cfg, client: client, journal: journal, now: now}, nil
}

// Compact satisfies the conversation.Options.Compactor signature:
//
//	func(ctx context.Context, dropped []llmclient.Message) (llmclient.Message, bool, error)
//
// Ordering guarantee: dropped messages are journalled FIRST, then the LLM
// call is made. A crash between journalling and the LLM call leaves the
// ground truth in the journal but produces no summary — the caller's
// WindowCtx returns with err=nil and ok=false at the compactor's request
// (the error path only fires on LLM failure, not journal-only crash, since
// a crashed process cannot return anything).
//
// On LLM failure: returns (false, err) — the caller's WindowCtx aborts
// compaction and returns the window over-budget but intact. No messages
// are lost.
//
// Summary marker: the returned message's first (and only) text block begins
// with "[COMPACTION: <json>]\n\n" — a structured JSON marker encoding the
// template version, prompt hash, params hash, journal timestamp, and
// dropped count. This marker survives round-trips through model context and
// can be stripped for display.
func (c *Compactor) Compact(ctx context.Context, dropped []llmclient.Message) (llmclient.Message, bool, error) {
	if len(dropped) == 0 {
		return llmclient.Message{}, false, nil
	}

	at := c.now().UTC()
	atStr := at.Format(time.RFC3339)

	// Serialise dropped messages for both the journal and the prompt.
	journalMsgs := toJournalMessages(dropped)
	messagesJSON, err := json.Marshal(journalMsgs)
	if err != nil {
		return llmclient.Message{}, false, fmt.Errorf("evidencecompactor: serialise dropped messages: %w", err)
	}

	// Render the versioned prompt template (also computes prompt + params hashes).
	params := map[string]any{
		"messages": string(messagesJSON),
	}
	rendered, err := prompttemplate.Render(summaryTemplate, params)
	if err != nil {
		return llmclient.Message{}, false, fmt.Errorf("evidencecompactor: render summary prompt: %w", err)
	}

	// Step 1: journal the dropped messages BEFORE the LLM call.
	// Crash between here and the LLM call = ground truth preserved, no summary.
	rec := JournalRecord{
		Kind:            "compaction-dropped",
		At:              atStr,
		DroppedCount:    len(dropped),
		TemplateVersion: rendered.TemplateVersion,
		ParamsHash:      rendered.ParamsHash,
		Messages:        journalMsgs,
	}
	if err := c.journal.Append(rec); err != nil {
		return llmclient.Message{}, false, fmt.Errorf("evidencecompactor: journal dropped messages: %w", err)
	}

	// Step 2: call the LLM to generate the summary.
	req := llmclient.Request{
		Model:     c.cfg.Model,
		System:    c.cfg.System,
		MaxTokens: c.cfg.MaxTokens,
		Messages: []llmclient.Message{
			{
				Role:    llmclient.RoleUser,
				Content: []llmclient.ContentBlock{llmclient.TextBlock(rendered.Text)},
			},
		},
	}
	resp, err := c.client.Complete(ctx, req)
	if err != nil {
		// LLM failure → abort compaction. Ground truth is safe in the journal.
		return llmclient.Message{}, false, fmt.Errorf("evidencecompactor: summary LLM call failed: %w", err)
	}

	summaryText := resp.Text
	if summaryText == "" {
		// Empty response — decline rather than insert a blank summary.
		return llmclient.Message{}, false, nil
	}

	// Build the structured marker (links this summary to journal + prompt).
	marker := SummaryMarker{
		TemplateVersion: rendered.TemplateVersion,
		PromptHash:      rendered.PromptHash,
		ParamsHash:      rendered.ParamsHash,
		JournaledAt:     atStr,
		DroppedCount:    len(dropped),
	}
	markerStr, err := FormatMarker(marker)
	if err != nil {
		// Marker serialisation failure is a programming error; decline rather
		// than insert an unprovenance'd summary.
		return llmclient.Message{}, false, fmt.Errorf("evidencecompactor: format summary marker: %w", err)
	}

	// Return the summary with the marker prepended. Role is ignored by
	// conversation.Window / WindowCtx — placement is role-safe by the caller.
	summaryMsg := llmclient.Message{
		Role: llmclient.RoleAssistant,
		Content: []llmclient.ContentBlock{
			llmclient.TextBlock(markerStr + summaryText),
		},
	}
	return summaryMsg, true, nil
}

// toJournalMessages converts []llmclient.Message to []JournalMessage for
// stable JSON serialisation. Unknown block types (BlockUnknown, future
// inventions) are stored with their type tag; the Raw json.RawMessage
// field is not stored (unknown blocks rarely carry text content this
// package could summarise anyway).
func toJournalMessages(msgs []llmclient.Message) []JournalMessage {
	out := make([]JournalMessage, len(msgs))
	for i, m := range msgs {
		jm := JournalMessage{Role: string(m.Role)}
		jm.Content = make([]JournalBlock, len(m.Content))
		for j, b := range m.Content {
			jm.Content[j] = JournalBlock{
				Type:      string(b.Type),
				Text:      b.Text,
				ID:        b.ID,
				Name:      b.Name,
				ToolUseID: b.ToolUseID,
				Content:   b.Content,
				IsError:   b.IsError,
			}
		}
		out[i] = jm
	}
	return out
}
