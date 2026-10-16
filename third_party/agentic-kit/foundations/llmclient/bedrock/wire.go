package bedrock

import (
	"encoding/json"
	"fmt"
	"maps"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// Wire types for the Anthropic-on-Bedrock request body. The body is the public
// Anthropic Messages body with two differences: the model is NOT in the body
// (it is the {modelId} URL path segment) and "anthropic_version" IS in the body
// (the fixed Bedrock token "bedrock-2023-05-31"). "stream" is also absent —
// streaming is selected by the endpoint (/invoke vs
// /invoke-with-response-stream), not a body flag. Field declaration order is
// the serialization order; the golden wire-shape tests assert it.
type wireRequest struct {
	AnthropicVersion string        `json:"anthropic_version"`
	MaxTokens        int           `json:"max_tokens"` // REQUIRED by Bedrock: never omitted.
	System           any           `json:"system,omitempty"`
	Messages         []wireMessage `json:"messages"`
	Temperature      *float64      `json:"temperature,omitempty"` // omitted when zero.
	StopSequences    []string      `json:"stop_sequences,omitempty"`
	// Tools holds pre-marshaled tool entries. Typed ToolDefs are marshalled
	// via wireTool; ToolDefs with Raw set are included verbatim.
	Tools        []json.RawMessage `json:"tools,omitempty"`
	ToolChoice   *wireToolChoice   `json:"tool_choice,omitempty"`
	Thinking     *wireThinking     `json:"thinking,omitempty"`
	OutputConfig *wireOutputConfig `json:"output_config,omitempty"`
}

// wireOutputConfig is the structured-outputs "output_config" object. Bedrock's
// InvokeModel body is the same Anthropic Messages body, so the mapping is
// identical to the public Anthropic API:
//
//	"output_config": {"format": {"type": "json_schema", "schema": <OutputSchema>}}
type wireOutputConfig struct {
	Format wireOutputFormat `json:"format"`
}

// wireOutputFormat is the "format" sub-object of output_config.
type wireOutputFormat struct {
	Type   string          `json:"type"`
	Schema json.RawMessage `json:"schema"`
}

// wireThinking is the Anthropic-on-Bedrock "thinking" object. Bedrock's
// InvokeModel body is the same Anthropic Messages body, so thinking passes
// through identically to the public API.
type wireThinking struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens,omitempty"`
}

type wireMessage struct {
	Role string `json:"role"`
	// Content holds pre-marshaled blocks so BlockUnknown raw JSON passes
	// through byte-verbatim.
	Content []json.RawMessage `json:"content"`
}

type wireCacheControl struct {
	Type string `json:"type"` // always "ephemeral"
}

func ephemeral() *wireCacheControl { return &wireCacheControl{Type: "ephemeral"} }

type wireTextBlock struct {
	Type         string            `json:"type"`
	Text         string            `json:"text"`
	CacheControl *wireCacheControl `json:"cache_control,omitempty"`
}

type wireImageSource struct {
	Type      string `json:"type"`       // "base64"
	MediaType string `json:"media_type"` // e.g. "image/png"
	Data      string `json:"data"`       // base64 payload
}

type wireImageBlock struct {
	Type         string            `json:"type"` // "image"
	Source       wireImageSource   `json:"source"`
	CacheControl *wireCacheControl `json:"cache_control,omitempty"`
}

type wireToolUseBlock struct {
	Type         string            `json:"type"`
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Input        json.RawMessage   `json:"input"`
	CacheControl *wireCacheControl `json:"cache_control,omitempty"`
}

type wireToolResultBlock struct {
	Type         string            `json:"type"`
	ToolUseID    string            `json:"tool_use_id"`
	Content      string            `json:"content"`
	IsError      bool              `json:"is_error,omitempty"`
	CacheControl *wireCacheControl `json:"cache_control,omitempty"`
}

type wireTool struct {
	Name         string            `json:"name"`
	Description  string            `json:"description,omitempty"`
	InputSchema  json.RawMessage   `json:"input_schema"`
	Strict       *bool             `json:"strict,omitempty"`
	CacheControl *wireCacheControl `json:"cache_control,omitempty"`
}

type wireToolChoice struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

// wireUsage mirrors the Bedrock/Anthropic usage object.
type wireUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

func (u wireUsage) toUsage() llmclient.Usage {
	return llmclient.Usage{
		InputTokens:              u.InputTokens,
		OutputTokens:             u.OutputTokens,
		CacheCreationInputTokens: u.CacheCreationInputTokens,
		CacheReadInputTokens:     u.CacheReadInputTokens,
	}
}

// mergeUsage overlays non-zero fields of src onto dst. Streams report usage
// incrementally (input/cache fields on message_start, output tokens on
// message_delta); merging keeps the latest non-zero value of each field.
func mergeUsage(dst *llmclient.Usage, src wireUsage) {
	if src.InputTokens != 0 {
		dst.InputTokens = src.InputTokens
	}
	if src.OutputTokens != 0 {
		dst.OutputTokens = src.OutputTokens
	}
	if src.CacheCreationInputTokens != 0 {
		dst.CacheCreationInputTokens = src.CacheCreationInputTokens
	}
	if src.CacheReadInputTokens != 0 {
		dst.CacheReadInputTokens = src.CacheReadInputTokens
	}
}

// wireResponse is the non-streaming InvokeModel response body. Bedrock returns
// the Anthropic message object; the model id is echoed but not needed.
type wireResponse struct {
	Content      []json.RawMessage `json:"content"`
	StopReason   string            `json:"stop_reason"`
	StopSequence string            `json:"stop_sequence"`
	Usage        wireUsage         `json:"usage"`
}

// buildBody maps req to the Anthropic-on-Bedrock request JSON and returns the
// resolved model id (for the URL path). All validation that can fail before
// HTTP happens here and returns typed llmclient errors.
func (c *Client) buildBody(req llmclient.Request) (body []byte, model string, err error) {
	model = req.Model
	if model == "" {
		model = c.model
	}
	if model == "" {
		return nil, "", &llmclient.InvalidRequestError{Message: "bedrock: no model: Request.Model and Config.DefaultModel are both empty"}
	}

	// Optional caller-supplied model-capability validation. Nil = permissive.
	if c.capabilityCheck != nil {
		if err := c.capabilityCheck(req); err != nil {
			return nil, "", err
		}
	}

	// Effort is not a first-class Bedrock/Anthropic-on-Bedrock API field.
	// Bedrock's InvokeModel body is the Anthropic Messages body; Anthropic has
	// no stable effort knob. Reject explicitly so intent is never silently lost.
	if req.Effort != "" {
		return nil, "", &llmclient.InvalidRequestError{Message: fmt.Sprintf(
			"bedrock: Request.Effort %q is not supported by this adapter; use Request.ProviderParams to pass experimental effort fields verbatim",
			req.Effort,
		)}
	}

	if err := llmclient.ValidateProviderParams(req.ProviderParams, "bedrock"); err != nil {
		return nil, "", err
	}

	if err := llmclient.ValidateOutputSchema(req.OutputSchema, "bedrock"); err != nil {
		return nil, "", err
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = c.maxTokens
	}

	if err := validateCacheHints(req); err != nil {
		return nil, "", err
	}

	w := wireRequest{
		AnthropicVersion: anthropicVersionValue,
		MaxTokens:        maxTokens,
		StopSequences:    req.StopSequences,
	}

	if req.System != "" {
		if req.SystemCacheHint {
			w.System = []wireTextBlock{{Type: "text", Text: req.System, CacheControl: ephemeral()}}
		} else {
			w.System = req.System
		}
	}

	msgs := req.Messages
	if len(msgs) == 0 {
		// v1 back-compat: Prompt becomes a single user text message.
		msgs = []llmclient.Message{{
			Role:    llmclient.RoleUser,
			Content: []llmclient.ContentBlock{llmclient.TextBlock(req.Prompt)},
		}}
	}
	w.Messages = make([]wireMessage, 0, len(msgs))
	for _, m := range msgs {
		wm := wireMessage{Role: string(m.Role)}
		for _, b := range m.Content {
			raw, err := marshalBlock(b)
			if err != nil {
				return nil, "", err
			}
			wm.Content = append(wm.Content, raw)
		}
		w.Messages = append(w.Messages, wm)
	}

	if req.Temperature != 0 {
		t := req.Temperature
		w.Temperature = &t
	}

	for _, t := range req.Tools {
		if len(t.Raw) > 0 {
			// ToolDef.Raw wins: emit the verbatim JSON object as the tool entry.
			// Validate it is a JSON object before including it. CacheHint is
			// ignored — cache_control cannot be injected into verbatim JSON.
			var obj map[string]json.RawMessage
			if err := json.Unmarshal(t.Raw, &obj); err != nil {
				return nil, "", &llmclient.InvalidRequestError{Message: fmt.Sprintf(
					"bedrock: ToolDef.Raw for tool %q is not a valid JSON object: %v", t.Name, err,
				)}
			}
			w.Tools = append(w.Tools, t.Raw)
			continue
		}
		schema := t.InputSchema
		if len(schema) == 0 {
			schema = json.RawMessage(`{"type":"object"}`)
		}
		wt := wireTool{Name: t.Name, Description: t.Description, InputSchema: schema}
		if t.Strict {
			v := true
			wt.Strict = &v
		}
		if t.CacheHint {
			wt.CacheControl = ephemeral()
		}
		wtRaw, err := json.Marshal(wt)
		if err != nil {
			return nil, "", &llmclient.InvalidRequestError{Message: fmt.Sprintf("bedrock: encoding tool %q: %v", t.Name, err)}
		}
		w.Tools = append(w.Tools, wtRaw)
	}

	tc, err := mapToolChoice(req.ToolChoice)
	if err != nil {
		return nil, "", err
	}
	w.ToolChoice = tc

	// Thinking passes through to Bedrock's InvokeModel body identically to the
	// public Anthropic API — both use the same Anthropic Messages body format.
	if req.Thinking != nil && req.Thinking.Type != "" {
		w.Thinking = &wireThinking{
			Type:         req.Thinking.Type,
			BudgetTokens: req.Thinking.BudgetTokens,
		}
	}

	// OutputSchema maps to output_config.format identically to the public
	// Anthropic API — Bedrock's InvokeModel body is the same Messages body.
	if len(req.OutputSchema) > 0 {
		w.OutputConfig = &wireOutputConfig{
			Format: wireOutputFormat{
				Type:   "json_schema",
				Schema: req.OutputSchema,
			},
		}
	}

	body, err = json.Marshal(w)
	if err != nil {
		return nil, "", &llmclient.InvalidRequestError{Message: fmt.Sprintf("bedrock: encoding request: %v", err)}
	}

	// Merge ProviderParams VERBATIM into the top-level body object. Keys the
	// adapter already set are overridden by ProviderParams — the caller's
	// escape hatch. See the llmclient package doc.
	if len(req.ProviderParams) > 0 {
		body, err = mergeProviderParams(body, req.ProviderParams, "bedrock")
		if err != nil {
			return nil, "", err
		}
	}

	return body, model, nil
}

// validateCacheHints enforces the 4-breakpoint cache_control limit locally.
// CacheHint on BlockUnknown blocks is ignored (Raw is emitted verbatim) and not
// counted.
func validateCacheHints(req llmclient.Request) error {
	n := 0
	if req.System != "" && req.SystemCacheHint {
		n++
	}
	for _, t := range req.Tools {
		if t.CacheHint {
			n++
		}
	}
	for _, m := range req.Messages {
		for _, b := range m.Content {
			if b.CacheHint && b.Type != llmclient.BlockUnknown {
				n++
			}
		}
	}
	if n > maxCacheBreakpoints {
		return &llmclient.InvalidRequestError{Message: fmt.Sprintf(
			"bedrock: %d cache breakpoints requested across tools+system+blocks; at most %d per request",
			n, maxCacheBreakpoints,
		)}
	}
	return nil
}

// mapToolChoice maps llmclient.ToolChoice to the wire form. The zero value
// (auto) is omitted. Unknown future modes pass through verbatim.
func mapToolChoice(tc llmclient.ToolChoice) (*wireToolChoice, error) {
	switch tc.Mode {
	case llmclient.ToolChoiceAuto:
		return nil, nil
	case llmclient.ToolChoiceAny, llmclient.ToolChoiceNone:
		return &wireToolChoice{Type: string(tc.Mode)}, nil
	case llmclient.ToolChoiceTool:
		if tc.Name == "" {
			return nil, &llmclient.InvalidRequestError{Message: `bedrock: ToolChoice.Mode is "tool" but ToolChoice.Name is empty`}
		}
		return &wireToolChoice{Type: "tool", Name: tc.Name}, nil
	default:
		return &wireToolChoice{Type: string(tc.Mode), Name: tc.Name}, nil
	}
}

// marshalBlock maps one ContentBlock to its wire JSON.
//
// Raw precedence rule: when b.Raw is non-empty it is emitted VERBATIM and all
// typed fields are ignored for wire encoding. This gives lossless round-trip
// fidelity for blocks received from the provider (which carry their original
// wire JSON in Raw). Callers who edit typed fields of a received block MUST
// clear Raw to force re-serialisation from the updated typed fields.
//
// BlockUnknown always requires Raw (it has no typed representation).
func marshalBlock(b llmclient.ContentBlock) (json.RawMessage, error) {
	// Raw wins when non-empty — verbatim round-trip for any block type.
	if len(b.Raw) > 0 {
		return b.Raw, nil
	}

	var cc *wireCacheControl
	if b.CacheHint {
		cc = ephemeral()
	}
	switch b.Type {
	case llmclient.BlockText:
		return json.Marshal(wireTextBlock{Type: "text", Text: b.Text, CacheControl: cc})
	case llmclient.BlockImage:
		if b.MediaType == "" || b.Data == "" {
			return nil, &llmclient.InvalidRequestError{Message: "bedrock: BlockImage requires MediaType and Data"}
		}
		return json.Marshal(wireImageBlock{
			Type:         "image",
			Source:       wireImageSource{Type: "base64", MediaType: b.MediaType, Data: b.Data},
			CacheControl: cc,
		})
	case llmclient.BlockToolUse:
		input := b.Input
		if len(input) == 0 {
			input = json.RawMessage(`{}`)
		}
		return json.Marshal(wireToolUseBlock{Type: "tool_use", ID: b.ID, Name: b.Name, Input: input, CacheControl: cc})
	case llmclient.BlockToolResult:
		return json.Marshal(wireToolResultBlock{Type: "tool_result", ToolUseID: b.ToolUseID, Content: b.Content, IsError: b.IsError, CacheControl: cc})
	case llmclient.BlockUnknown:
		// BlockUnknown has no typed representation; Raw is required.
		return nil, &llmclient.InvalidRequestError{Message: "bedrock: BlockUnknown content block with empty Raw"}
	default:
		return nil, &llmclient.InvalidRequestError{Message: fmt.Sprintf("bedrock: unsupported content block type %q", b.Type)}
	}
}

// parseBlocks maps inbound content blocks to ContentBlocks. Anything other than
// text / tool_use / tool_result is preserved verbatim as BlockUnknown.
//
// All blocks — typed or unknown — record their verbatim provider JSON in Raw,
// so callers can round-trip them back unmodified (citations annotations,
// server-tool payloads, and any other provider extras are preserved).
func parseBlocks(raw []json.RawMessage) ([]llmclient.ContentBlock, error) {
	blocks := make([]llmclient.ContentBlock, 0, len(raw))
	for i, r := range raw {
		var head struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(r, &head); err != nil {
			return nil, fmt.Errorf("content block %d: %w", i, err)
		}
		switch head.Type {
		case "text":
			var tb struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(r, &tb); err != nil {
				return nil, fmt.Errorf("text block %d: %w", i, err)
			}
			b := llmclient.TextBlock(tb.Text)
			b.Raw = r
			blocks = append(blocks, b)
		case "tool_use":
			var tu struct {
				ID    string          `json:"id"`
				Name  string          `json:"name"`
				Input json.RawMessage `json:"input"`
			}
			if err := json.Unmarshal(r, &tu); err != nil {
				return nil, fmt.Errorf("tool_use block %d: %w", i, err)
			}
			b := llmclient.ToolUseBlock(tu.ID, tu.Name, tu.Input)
			b.Raw = r
			blocks = append(blocks, b)
		case "tool_result":
			var tr struct {
				ToolUseID string          `json:"tool_use_id"`
				Content   json.RawMessage `json:"content"`
				IsError   bool            `json:"is_error"`
			}
			if err := json.Unmarshal(r, &tr); err != nil {
				return nil, fmt.Errorf("tool_result block %d: %w", i, err)
			}
			b := llmclient.ToolResultBlock(tr.ToolUseID, flattenToolResultContent(tr.Content), tr.IsError)
			b.Raw = r
			blocks = append(blocks, b)
		default:
			blocks = append(blocks, llmclient.ContentBlock{Type: llmclient.BlockUnknown, Raw: r})
		}
	}
	return blocks, nil
}

// flattenToolResultContent maps the tool_result content (a string or an array
// of text blocks) to the flat string the llmclient contract uses.
func flattenToolResultContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var arr []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &arr); err == nil {
		var out string
		for _, e := range arr {
			if e.Type == "text" {
				out += e.Text
			}
		}
		return out
	}
	return string(raw)
}

// textOf returns the in-order concatenation of the text blocks in blocks.
func textOf(blocks []llmclient.ContentBlock) string {
	var s string
	for _, b := range blocks {
		if b.Type == llmclient.BlockText {
			s += b.Text
		}
	}
	return s
}

// mergeProviderParams merges the caller-supplied ProviderParams JSON object
// into the already-marshalled adapter body. The merge is a top-level key
// overlay: params keys OVERRIDE adapter-set keys (the documented precedence).
// Both inputs are valid JSON objects (validated before this call).
func mergeProviderParams(body, params json.RawMessage, prefix string) ([]byte, error) {
	var base map[string]json.RawMessage
	if err := json.Unmarshal(body, &base); err != nil {
		return nil, &llmclient.InvalidRequestError{Message: fmt.Sprintf(
			"%s: internal: could not re-parse own request body for ProviderParams merge: %v", prefix, err,
		)}
	}
	var overlay map[string]json.RawMessage
	if err := json.Unmarshal(params, &overlay); err != nil {
		// Should not happen — ValidateProviderParams already passed — but be safe.
		return nil, &llmclient.InvalidRequestError{Message: fmt.Sprintf(
			"%s: Request.ProviderParams is not a valid JSON object: %v", prefix, err,
		)}
	}
	maps.Copy(base, overlay)
	merged, err := json.Marshal(base)
	if err != nil {
		return nil, &llmclient.InvalidRequestError{Message: fmt.Sprintf(
			"%s: encoding merged request body: %v", prefix, err,
		)}
	}
	return merged, nil
}
