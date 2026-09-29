package llmclient

import (
	"context"
	"encoding/json"
	"fmt"
)

// Role identifies the author of a Message.
type Role string

// The two roles a Message may carry. (System prompts are not messages — they
// travel on Request.System.)
const (
	// RoleUser identifies a message authored by the caller (the end user or
	// the calling application on the user's behalf).
	RoleUser Role = "user"
	// RoleAssistant identifies a message authored by the model.
	RoleAssistant Role = "assistant"
)

// BlockType discriminates the variants of ContentBlock.
type BlockType string

// Known block types. BlockUnknown is the verbatim passthrough for provider
// block types this package does not model.
const (
	// BlockText identifies a plain-text content block.
	BlockText BlockType = "text"
	// BlockImage identifies an image content block.
	BlockImage BlockType = "image"
	// BlockToolUse identifies a tool-invocation content block emitted by the
	// model.
	BlockToolUse BlockType = "tool_use"
	// BlockToolResult identifies a tool-result content block sent back to
	// the model.
	BlockToolResult BlockType = "tool_result"
	// BlockUnknown is the verbatim passthrough for provider block types this
	// package does not model.
	BlockUnknown BlockType = "unknown"
)

// MaxCacheBreakpoints is the most cache breakpoints (Request.SystemCacheHint
// + ToolDef.CacheHint + ContentBlock.CacheHint, with CacheHint on
// BlockUnknown blocks ignored) one request may carry. The semantics are
// Anthropic's — see the package doc, "Prompt caching". Both the anthropic
// adapter and MockClient reject requests exceeding it with a typed
// InvalidRequestError before any work is done.
const MaxCacheBreakpoints = 4

// ContentBlock is a tagged union: Type selects which of the variant fields are
// meaningful. Exactly one variant's fields are populated per block.
//
//   - BlockText: Text.
//   - BlockToolUse: ID, Name, Input — a tool invocation requested by the model.
//   - BlockToolResult: ToolUseID, Content, IsError — a tool's output, sent back
//     by the caller in a user message.
//   - BlockUnknown: Raw — the provider's block JSON preserved VERBATIM for
//     block types this package does not model (e.g. thinking blocks), so
//     multi-turn round-trips never drop data. Clients must echo Raw back to
//     the provider untouched.
//
// Use the TextBlock, ToolUseBlock, and ToolResultBlock constructors rather
// than struct literals where possible.
type ContentBlock struct {
	// Type selects the variant. See the BlockType constants.
	Type BlockType

	// Text is the text content (BlockText).
	Text string

	// MediaType is the IANA media type of an image block (BlockImage), e.g.
	// "image/png", "image/jpeg", "image/gif", "image/webp".
	MediaType string
	// Data is the base64-encoded image bytes (BlockImage), no data: prefix.
	Data string

	// ID is the provider-assigned tool-use identifier (BlockToolUse).
	ID string
	// Name is the tool being invoked (BlockToolUse).
	Name string
	// Input is the tool's arguments as raw JSON (BlockToolUse).
	Input json.RawMessage

	// ToolUseID links a result back to the tool_use block that requested it
	// (BlockToolResult).
	ToolUseID string
	// Content is the tool's output (BlockToolResult).
	Content string
	// IsError marks the tool execution as failed (BlockToolResult); the model
	// sees the Content as an error message.
	IsError bool

	// Raw is the provider's verbatim block JSON (BlockUnknown).
	Raw json.RawMessage

	// CacheHint requests a provider cache breakpoint at this block. Anthropic
	// semantics: at most 4 breakpoints per request; prefixes below a
	// model-specific minimum (~1k–4k tokens) are silently not cached — verify
	// via Usage.CacheReadInputTokens. See the package doc, "Prompt caching".
	CacheHint bool
}

// TextBlock returns a BlockText ContentBlock containing s.
func TextBlock(s string) ContentBlock {
	return ContentBlock{Type: BlockText, Text: s}
}

// ImageBlock returns a BlockImage ContentBlock from a base64-encoded image.
// mediaType is e.g. "image/png"; data is the base64 payload without a prefix.
func ImageBlock(mediaType, data string) ContentBlock {
	return ContentBlock{Type: BlockImage, MediaType: mediaType, Data: data}
}

// ToolUseBlock returns a BlockToolUse ContentBlock for the given tool-use id,
// tool name, and raw-JSON input.
func ToolUseBlock(id, name string, input json.RawMessage) ContentBlock {
	return ContentBlock{Type: BlockToolUse, ID: id, Name: name, Input: input}
}

// ToolResultBlock returns a BlockToolResult ContentBlock answering the
// tool_use block identified by toolUseID.
func ToolResultBlock(toolUseID, content string, isError bool) ContentBlock {
	return ContentBlock{Type: BlockToolResult, ToolUseID: toolUseID, Content: content, IsError: isError}
}

// Message is one turn of a conversation.
type Message struct {
	// Role is the author: RoleUser or RoleAssistant.
	Role Role

	// Content is the ordered list of content blocks in this turn.
	Content []ContentBlock
}

// ToolDef declares one tool the model may invoke.
//
// # Provider passthrough (Raw)
//
// Raw, when non-empty, is emitted VERBATIM as the tool entry in the outbound
// wire request body; the typed fields (Name, Description, InputSchema,
// CacheHint, Strict) are ignored for wire serialisation. Use this to pass
// server tools (web_search, code_execution, memory) and other provider-native
// tool types that the kit does not model structurally. Raw must be a valid JSON
// object.
//
// Name is still required even when Raw is set: tool registries and the harness
// use Name for routing and bookkeeping. Set Name to the tool's canonical
// identifier (matching the "name" field inside the Raw JSON) so registry lookups
// work correctly.
//
// CacheHint on a Raw tool is ignored — cache_control cannot be injected into
// verbatim JSON without parsing it.
//
// Strict on a Raw tool is ignored — the caller owns the raw JSON and must
// include "strict":true inside Raw if desired.
type ToolDef struct {
	// Name is the tool's identifier, referenced by tool_use blocks. Required
	// even when Raw is set — used by tool registries for routing/bookkeeping.
	Name string

	// Description tells the model when and how to use the tool.
	Description string

	// InputSchema is the JSON Schema (raw JSON) describing the tool's input.
	InputSchema json.RawMessage

	// Strict, when true, sets "strict":true on the tool wire object. The
	// provider treats this as a structured-outputs hint, constraining tool-input
	// generation to the declared InputSchema. Ignored when Raw is set (the caller
	// owns the raw JSON).
	Strict bool

	// CacheHint requests a provider cache breakpoint at this tool definition.
	// Same Anthropic semantics as ContentBlock.CacheHint: max 4 breakpoints
	// per request, silent no-op below the minimum cacheable prefix — verify
	// via Usage.CacheReadInputTokens. Ignored when Raw is set.
	CacheHint bool

	// Raw, when non-empty, overrides the typed fields for wire serialisation:
	// the adapter emits Raw VERBATIM as the tool JSON object. Use for
	// server-side tools (web_search, code_execution, etc.) and future tool
	// types not yet modeled by the kit. Must be a valid JSON object. Name is
	// still required for registry routing even when Raw is set.
	Raw json.RawMessage
}

// ToolChoiceMode selects how the model may use the declared tools.
type ToolChoiceMode string

// Tool-choice modes. The zero value (ToolChoiceAuto) lets the model decide.
const (
	// ToolChoiceAuto (the zero value) lets the model decide whether to call a
	// tool or answer directly.
	ToolChoiceAuto ToolChoiceMode = ""
	// ToolChoiceAny forces the model to call some tool.
	ToolChoiceAny ToolChoiceMode = "any"
	// ToolChoiceNone forbids tool calls even when Tools are declared.
	ToolChoiceNone ToolChoiceMode = "none"
	// ToolChoiceTool forces the model to call the specific tool named in
	// ToolChoice.Name.
	ToolChoiceTool ToolChoiceMode = "tool"
)

// ToolChoice constrains tool selection for a request. The zero value means
// auto: the model decides whether and which tool to call.
type ToolChoice struct {
	// Mode is the constraint; the zero value is auto.
	Mode ToolChoiceMode

	// Name is the tool to force when Mode is ToolChoiceTool; ignored otherwise.
	Name string
}

// ThinkingConfig requests extended thinking from a model that supports it.
// Set Type to "enabled" to activate, "disabled" to suppress, or "auto" to
// let the provider decide. The zero value (empty Type) means "unset" and
// produces no thinking wire field.
//
// Note: the Type field is an open string, not a Go enum — providers may add
// new values and the contract must not break on forward evolution.
type ThinkingConfig struct {
	// Type is the thinking mode: "enabled", "disabled", "auto", or a future
	// provider-specific value.
	Type string

	// BudgetTokens is the maximum number of tokens the model may spend on
	// thinking, when Type is "enabled". Zero means use the provider default.
	BudgetTokens int
}

// Request is the input to any LLM call. It supports both the v1 single-turn
// form (Prompt) and the v2 conversation form (Messages).
//
// Prompt/Messages mapping: when Messages is empty, clients treat Prompt as a
// single user message — i.e. the request behaves exactly like
// Messages: []Message{{Role: RoleUser, Content: []ContentBlock{TextBlock(Prompt)}}}.
// When Messages is non-empty, it is the conversation and Prompt is ignored.
//
// # Temperature-on-frontier hazard
//
// Current frontier models (e.g. Claude 3.7+ with extended thinking enabled)
// reject requests that set Temperature when thinking is active, returning a
// 400 InvalidRequestError. If you set both Request.Temperature and
// Request.Thinking with Type "enabled", the provider will refuse the call.
// The kit does not enforce this combination locally because the constraint is
// model-specific and model-catalog-driven validation is the correct seam
// (see Config.CapabilityCheck). Until catalog-driven validation is wired,
// callers should leave Temperature at zero when thinking is enabled.
type Request struct {
	// Model names the model to use. Empty means the client's configured
	// default. Rung.Apply fills this from a Ladder rung when empty.
	Model string

	// Prompt is the v1 single-turn text. Used only when Messages is empty;
	// see the type doc for the exact mapping.
	Prompt string

	// Messages is the conversation, oldest first. When non-empty it takes
	// precedence over Prompt.
	Messages []Message

	// MaxTokens caps the response length. Zero means use the model default.
	MaxTokens int

	// System is an optional system prompt / preamble.
	System string

	// SystemCacheHint requests a provider cache breakpoint after the system
	// prompt. Same Anthropic semantics as ContentBlock.CacheHint.
	SystemCacheHint bool

	// Temperature controls sampling randomness. Zero uses the model default.
	//
	// HAZARD: current frontier models return 400 when Temperature is set and
	// thinking is enabled simultaneously. See the Request type doc.
	Temperature float64

	// StopSequences are optional sequences where generation should halt.
	StopSequences []string

	// Tools declares the tools the model may call.
	Tools []ToolDef

	// ToolChoice constrains tool selection. The zero value means auto.
	ToolChoice ToolChoice

	// Thinking requests extended thinking from the model. A nil value means
	// "unset" — no thinking field is sent. Set Thinking.Type to "enabled" to
	// activate; see ThinkingConfig for the full contract.
	//
	// Provider mapping:
	//   - Anthropic: maps to the "thinking" wire object.
	//   - Bedrock (Anthropic-on-Bedrock): the Converse API does not expose a
	//     thinking knob; the InvokeModel body is the same Anthropic Messages
	//     body and DOES support thinking — set it here and it passes through.
	Thinking *ThinkingConfig

	// Effort is an open-ended string hint for models that accept an effort or
	// reasoning-effort knob (e.g. "low", "medium", "high"). Where no provider
	// knob exists, the adapter returns a typed InvalidRequestError instead of
	// silently dropping the field.
	//
	// Provider mapping:
	//   - Anthropic: output_config.effort; low, medium, high, xhigh, max.
	//     Model-specific availability belongs in Config.CapabilityCheck.
	//   - OpenAI: reasoning_effort, or reasoning.effort in the Responses lane.
	//   - Bedrock: not mapped; returns InvalidRequestError.
	//   - MockClient: records the hint without simulating model behavior.
	Effort string

	// ProviderParams is a JSON object merged VERBATIM into the outbound
	// provider wire request body at the top level, AFTER the adapter's own
	// field mapping. Keys the adapter already set are OVERRIDDEN by
	// ProviderParams — this is the caller's escape hatch for provider-specific
	// fields not yet modeled by the kit (e.g. experimental betas, future
	// parameters). With great power comes great responsibility: the caller
	// owns the semantics, the serialisation, and any resulting errors from the
	// provider. The adapter does not validate the merged content beyond
	// confirming it is a JSON object (or empty/nil — which is a no-op).
	//
	// Streaming and Complete honour ProviderParams identically. The v2
	// canonical hash drops ProviderParams when nil or empty (zero-value
	// dropping), so existing cassettes are unaffected when the field is not
	// used.
	//
	// Adapter constraints still apply: Anthropic rejects raw output_config
	// when typed Effort or OutputSchema is also set, rather than silently
	// overriding either. Use typed fields together or raw output_config alone.
	ProviderParams json.RawMessage

	// OutputSchema is a JSON Schema object (draft 2020-12 or compatible) that
	// constrains the response format. When non-empty, adapters map it to the
	// provider's structured-outputs surface:
	//
	//   - Anthropic: output_config: {format: {type: "json_schema", schema: <OutputSchema>}}
	//   - Bedrock (Anthropic-on-Bedrock): identical body, same mapping.
	//
	// OutputSchema must be a valid JSON object (not an array, string, or null).
	// The adapter validates this before any HTTP and returns a typed
	// InvalidRequestError on failure.
	//
	// # Belt-and-suspenders design
	//
	// OutputSchema constrains the provider's decoding — the provider attempts
	// to emit JSON conforming to the schema. This is NOT a guarantee: providers
	// may emit non-conforming output under error conditions or model limitations.
	// Callers SHOULD validate the parsed artifact against the originating CUE
	// definition (via cue vet or equivalent) for the "contract that validates
	// the artifact is the same one that constrained generation" guarantee.
	//
	// # CUE→schema→constrained generation loop
	//
	// The toolchains/jsonschema package (DeriveFromDir/DeriveFromBytes) derives
	// a JSON Schema from a CUE definition and returns it as []byte. Cast to
	// json.RawMessage and set it here:
	//
	//	schema, _ := jsonschemagen.DeriveFromBytes(cueSrc, jsonschemagen.Options{})
	//	req.OutputSchema = json.RawMessage(schema)
	//
	// See the end-to-end test in toolchains/jsonschema for the full loop.
	//
	// # Streaming
	//
	// OutputSchema passes through to streaming requests identically — the same
	// output_config.format mapping applies to Stream as to Complete.
	//
	// # Cassette stability
	//
	// The v2 canonical hash drops OutputSchema when nil or empty, so existing
	// cassettes recorded without this field are unaffected.
	OutputSchema json.RawMessage
}

// StopReason reports why the model stopped generating. It is an OPEN set:
// providers add reasons over time, so never switch exhaustively over it —
// handle the constants you know and treat anything else as "stopped for a
// reason this build does not recognise".
type StopReason string

// Stop reasons known today. The set is open; see the StopReason doc.
const (
	// StopEndTurn: the model finished its turn naturally.
	StopEndTurn StopReason = "end_turn"
	// StopToolUse: the model is requesting tool calls. This is a SUCCESSFUL
	// completion — never an error, never retried.
	StopToolUse StopReason = "tool_use"
	// StopMaxTokens: generation hit the MaxTokens cap; output is truncated.
	StopMaxTokens StopReason = "max_tokens"
	// StopSequenceHit: generation hit one of Request.StopSequences; the
	// matched sequence is in Response.StopSequence.
	StopSequenceHit StopReason = "stop_sequence"
	// StopRefusal: the model declined to continue for safety reasons.
	StopRefusal StopReason = "refusal"
	// StopPauseTurn: the provider paused a long-running turn; resume by
	// sending the turn back.
	StopPauseTurn StopReason = "pause_turn"
	// StopContextExceeded: the conversation no longer fits the model's
	// context window.
	StopContextExceeded StopReason = "model_context_window_exceeded"
)

// Usage reports token accounting for one call, as reported by the backend.
// Zero values mean "not reported".
type Usage struct {
	// InputTokens is the token count of the request.
	InputTokens int

	// OutputTokens is the token count of the response.
	OutputTokens int

	// CacheCreationInputTokens counts input tokens written to the provider's
	// prompt cache by this call.
	CacheCreationInputTokens int

	// CacheReadInputTokens counts input tokens served from the provider's
	// prompt cache. This is the ground truth for whether CacheHint worked.
	CacheReadInputTokens int
}

// Response is the output of a Complete call.
//
// Usage is embedded, so the v1 fields Response.InputTokens and
// Response.OutputTokens keep working via field promotion.
type Response struct {
	// Text is the concatenation of the response's text blocks, in order. It
	// remains a populated field (not just derivable from Content) so v1
	// callers keep working.
	Text string

	// Content is the full ordered assistant content: text blocks, tool_use
	// blocks (with provider-assigned IDs), and unknown blocks preserved
	// verbatim.
	Content []ContentBlock

	// StopReason reports why generation stopped. Open set — do not switch
	// exhaustively.
	StopReason StopReason

	// StopSequence is the matched stop sequence when StopReason is
	// StopSequenceHit; empty otherwise.
	StopSequence string

	// Attempts is the number of tries that were made, including the final one.
	// Populated by WithRetry; 1 means no retries were necessary.
	Attempts int

	// Rung is the zero-based index of the Ladder rung that produced this
	// response. Zero when no Ladder is in use.
	Rung int

	// Usage is embedded: InputTokens / OutputTokens / cache counters are
	// promoted onto Response.
	Usage
}

// ToolCalls returns the tool_use blocks of the response, in order. Empty when
// the model did not request any tools.
func (r Response) ToolCalls() []ContentBlock {
	var calls []ContentBlock
	for _, b := range r.Content {
		if b.Type == BlockToolUse {
			calls = append(calls, b)
		}
	}
	return calls
}

// AsMessage returns the response as an assistant Message for appending to
// Request.Messages in a multi-turn loop. Content is passed through verbatim —
// including BlockUnknown blocks — so round-trips never drop data. The slice is
// copied; the blocks (and any json.RawMessage they hold) are shared.
func (r Response) AsMessage() Message {
	m := Message{Role: RoleAssistant}
	if len(r.Content) > 0 {
		m.Content = make([]ContentBlock, len(r.Content))
		copy(m.Content, r.Content)
	}
	return m
}

// ValidateProviderParams checks that pp is either empty/nil or a valid JSON
// object. prefix is the adapter name (e.g. "anthropic", "bedrock", "mock")
// used in the error message. Returns a typed *InvalidRequestError on failure.
// Adapters call this in buildBody before any wire encoding; MockClient calls
// it in validateRequest so the mock→real seam stays pinned.
//
// "Empty" means len(pp) == 0 — both nil and the zero-length empty slice/string
// are no-ops. The JSON literal "null" is rejected: it is syntactically valid
// JSON but semantically not an object, and a caller that explicitly writes
// `null` is almost certainly making a mistake.
func ValidateProviderParams(pp json.RawMessage, prefix string) error {
	if len(pp) == 0 {
		return nil
	}
	// json.Unmarshal("null", &map) succeeds with a nil map in Go, so we must
	// check for the literal "null" explicitly before attempting the decode.
	if string(pp) == "null" {
		return &InvalidRequestError{Message: fmt.Sprintf(
			"%s: Request.ProviderParams must be a JSON object or empty/nil; got JSON null (use nil or omit the field to mean \"no params\")",
			prefix,
		)}
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(pp, &obj); err != nil {
		return &InvalidRequestError{Message: fmt.Sprintf(
			"%s: Request.ProviderParams must be a JSON object (got %q); %v",
			prefix, string(pp), err,
		)}
	}
	return nil
}

// ValidateOutputSchema checks that schema is either empty/nil or a valid JSON
// object. prefix is the adapter name used in the error message. Returns a
// typed *InvalidRequestError on failure. Adapters call this in buildBody
// alongside ValidateProviderParams.
//
// "Empty" means len(schema) == 0. The JSON literal "null" is rejected: it is
// syntactically valid JSON but semantically not an object, and a caller that
// explicitly writes `null` is almost certainly making a mistake.
func ValidateOutputSchema(schema json.RawMessage, prefix string) error {
	if len(schema) == 0 {
		return nil
	}
	if string(schema) == "null" {
		return &InvalidRequestError{Message: fmt.Sprintf(
			"%s: Request.OutputSchema must be a JSON object or empty/nil; got JSON null",
			prefix,
		)}
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(schema, &obj); err != nil {
		return &InvalidRequestError{Message: fmt.Sprintf(
			"%s: Request.OutputSchema must be a JSON object (got %q); %v",
			prefix, string(schema), err,
		)}
	}
	return nil
}

// concatText returns the in-order concatenation of the text blocks in blocks.
func concatText(blocks []ContentBlock) string {
	var s string
	for _, b := range blocks {
		if b.Type == BlockText {
			s += b.Text
		}
	}
	return s
}

// StreamChunk is one event on a Stream channel. See the package doc,
// "Streaming semantics", for the full contract. Per chunk, at most one of
// TextDelta / Block / (StopReason+Usage) / Err is meaningful:
//
//   - TextDelta != "": an incremental piece of text.
//   - Block != nil: a complete non-text content block (tool_use blocks are
//     buffered by the client and delivered whole — no partial-JSON deltas).
//   - StopReason != "": the terminal success chunk; Usage is non-nil here.
//   - Err != nil: the terminal failure chunk.
//
// After a terminal chunk the channel closes.
type StreamChunk struct {
	// TextDelta is an incremental piece of response text.
	TextDelta string

	// Block is a complete non-text content block (e.g. a tool_use block).
	Block *ContentBlock

	// StopReason is set on the terminal success chunk.
	StopReason StopReason

	// Usage is set (non-nil) on the terminal success chunk.
	Usage *Usage

	// Err is set on the terminal failure chunk. No chunk follows it.
	Err error
}

// LLMClient is the single interface through which all model calls are made.
// Implementations must be safe for concurrent use.
type LLMClient interface {
	// Complete sends a request and waits for the full response.
	Complete(ctx context.Context, req Request) (Response, error)

	// Stream sends a request and returns a channel of stream chunks. The
	// returned error covers pre-stream failures only; after that, all
	// outcomes (including mid-stream failures) arrive as chunks and the
	// producer closes the channel exactly once. Callers must drain the
	// channel until it closes. See the package doc, "Streaming semantics".
	Stream(ctx context.Context, req Request) (<-chan StreamChunk, error)
}
