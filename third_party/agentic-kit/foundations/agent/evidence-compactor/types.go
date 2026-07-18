// Package evidencecompactor implements the evidence-preserving compactor for
// foundations/agent/conversation. It satisfies the widened Compactor
// signature: func(ctx context.Context, dropped []llmclient.Message)
// (llmclient.Message, bool, error).
//
// Design: PROJECTION, not replacement (VISION belief 3)
//
// The summary produced by this compactor is a PROJECTION of the dropped
// ground truth — not a replacement. Ground truth is preserved first:
//
//  1. The dropped messages are journalled verbatim (JSONL via jsonl-log)
//     to the caller-supplied io.Writer BEFORE any LLM call.
//  2. An LLM call generates the summary from a versioned prompt template.
//     The template version, prompt hash, and params hash are embedded in
//     the summary message as a structured marker so the summary is
//     traceable back to the journal.
//  3. On LLM failure the compactor returns (false, err) — never a lossy
//     fallback. The journal still holds the ground truth.
//
// Honesty disclaimer (MUST be kept visible):
// The summary's CLAIMS are only as good as the model that produced them.
// The GUARANTEE this package provides is preservation (the journal holds
// every dropped message verbatim) and provenance (the prompt hash and
// template version are embedded in the summary marker). Summary fidelity
// is the model's concern, not ours.
//
// Concurrency contract: a *Compactor is safe for concurrent use. The
// injected LLMClient and io.Writer must themselves be safe for concurrent
// use; a *jsonllog.Writer satisfies this. The injected clock (now func())
// is called with no lock held — a plain time.Now is safe.
//
// CUE contract: schema.cue owns the #JournalRecord shape; this file is
// the Go counterpart and must stay in sync with that schema.
package evidencecompactor

import (
	"encoding/json"
	"time"
)

// SummaryTemplateVersion is the version string of the package's versioned
// prompt template used to generate compaction summaries. It is shipped with
// the package so every caller gets the same prompt and version without
// configuration.
//
// Template parameters (map[string]any keys):
//
//	"messages" — a JSON-encoded array of the dropped messages (string).
//
// Bump SummaryTemplateVersion on any change to summaryTemplateText that
// would alter model output. PromptHash is derived deterministically from
// the rendered text at call time; this Version feeds into the marker for
// coarse provenance.
const SummaryTemplateVersion = "v1.0.0"

// summaryTemplateText is the Go text/template source for the summary prompt.
// It references exactly one parameter: {{.messages}}.
const summaryTemplateText = `You are a conversation archivist. The following messages were dropped from
an active conversation window to make room for new context. Produce a concise
summary that preserves ALL factual claims, decisions, and conclusions from
the dropped messages so they remain available as context for the ongoing
conversation.

Requirements:
- Start with "COMPACTION SUMMARY:" on its own line.
- Use bullet points for distinct facts or decisions.
- Preserve specific values (numbers, names, IDs, dates) exactly.
- Do NOT interpret, evaluate, or add information not in the messages.
- The summary is a PROJECTION of ground truth preserved in the journal;
  it supplements but never replaces the journal record.

Dropped messages (oldest first):
{{.messages}}`

// JournalBlock is the portable representation of one content block stored
// in the journal record. Fields are tagged omitempty so each block type
// populates only its relevant subset.
//
// CUE mirror: #JournalBlock in schema.cue.
type JournalBlock struct {
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`
}

// JournalMessage is the portable representation of one llmclient.Message
// stored in the journal record.
//
// CUE mirror: #JournalMessage in schema.cue.
type JournalMessage struct {
	Role    string         `json:"role"`
	Content []JournalBlock `json:"content"`
}

// JournalRecord is the record written to the caller-supplied io.Writer
// (via jsonl-log) before any LLM summarisation call. Its presence in the
// journal guarantees that the dropped ground truth survives a crash between
// journalling and the LLM call.
//
// The record is written with PromptHash and ParamsHash empty; the caller
// may append a follow-up record with those fields set after the LLM call
// completes, but the ground-truth guarantee depends only on this first
// record existing.
//
// CUE mirror: #JournalRecord in schema.cue.
type JournalRecord struct {
	Kind            string           `json:"kind"` // always "compaction-dropped"
	At              string           `json:"at"`   // RFC 3339 UTC
	DroppedCount    int              `json:"dropped_count"`
	TemplateVersion string           `json:"template_version"`
	ParamsHash      string           `json:"params_hash"`
	PromptHash      string           `json:"prompt_hash,omitempty"` // set in the post-call record
	Messages        []JournalMessage `json:"messages"`
}

// SummaryMarker is the structured provenance marker embedded as a JSON
// comment at the start of every summary message text block. It links the
// summary back to the journal record and the prompt that produced it so
// the summary is traceable to its ground truth.
//
// The marker is serialised as a single JSON object on one line enclosed in
// [COMPACTION: ... ] brackets so it survives being sent to a model as
// context without being mistaken for a command.
type SummaryMarker struct {
	TemplateVersion string `json:"template_version"`
	PromptHash      string `json:"prompt_hash"`
	ParamsHash      string `json:"params_hash"`
	JournaledAt     string `json:"journaled_at"` // RFC 3339 UTC of the journal record
	DroppedCount    int    `json:"dropped_count"`
}

// FormatMarker serialises m as the structured prefix embedded at the start
// of the summary text: "[COMPACTION: <json>]\n\n". The format is chosen to
// be parseable, model-readable (a labelled JSON block), and easy to strip
// for display.
func FormatMarker(m SummaryMarker) (string, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return "[COMPACTION: " + string(b) + "]\n\n", nil
}

// Config configures a Compactor. All fields are required unless noted.
type Config struct {
	// System is an optional system prompt sent with the summary LLM call.
	// Leave empty for the model's default behaviour.
	System string

	// Model is the model ID to use for summary generation. An empty string
	// uses the LLMClient's default model.
	Model string

	// MaxTokens is the max-tokens cap for the summary call. Zero uses the
	// client default.
	MaxTokens int
}

// clockFunc is the injectable clock seam. nil defaults to time.Now at
// call site (documented seam: kits default to time.Now when nil).
type clockFunc = func() time.Time
