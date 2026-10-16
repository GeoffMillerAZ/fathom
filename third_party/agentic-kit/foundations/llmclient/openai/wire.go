package openai

import (
	"encoding/json"
	"fmt"
	"maps"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// Wire types for the Chat Completions API request body.

// wireRequest is the outbound Chat Completions body.
type wireRequest struct {
	Model    string        `json:"model"`
	Messages []wireMessage `json:"messages"`

	// max_completion_tokens is the modern field (post-Nov-2024).
	// For pre-Nov-2024 model deployments use ProviderParams to override with
	// "max_tokens" instead — or set both; the server ignores max_tokens when
	// max_completion_tokens is present.
	MaxCompletionTokens int `json:"max_completion_tokens,omitempty"`

	// Temperature is omitted entirely when zero (zero = model default per the
	// llmclient contract; sending explicit 0.0 is rejected by some frontier
	// models, and zero is the JSON default so omitempty works correctly here).
	Temperature *float64 `json:"temperature,omitempty"`

	// Stop is the stop sequences array.
	Stop []string `json:"stop,omitempty"`

	// Tools is the tools array; each element is a pre-marshalled JSON object.
	Tools []json.RawMessage `json:"tools,omitempty"`

	// ToolChoice constrains which tool (if any) the model calls.
	ToolChoice *json.RawMessage `json:"tool_choice,omitempty"`

	// ResponseFormat sets structured-output mode.
	ResponseFormat *wireResponseFormat `json:"response_format,omitempty"`

	// ReasoningEffort maps Request.Effort for o-series models ("low",
	// "medium", "high"). Omitted when empty.
	ReasoningEffort string `json:"reasoning_effort,omitempty"`

	// Stream enables SSE streaming.
	Stream bool `json:"stream,omitempty"`

	// StreamOptions sets stream behaviour; used to request usage in the final
	// chunk (include_usage:true). Only sent when Stream is true.
	StreamOptions *wireStreamOptions `json:"stream_options,omitempty"`
}

// wireStreamOptions enables usage reporting in the final stream chunk.
type wireStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// wireMessage is one turn in the messages array.
type wireMessage struct {
	Role       string          `json:"role"`
	Content    json.RawMessage `json:"content,omitempty"`
	ToolCalls  json.RawMessage `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
}

// wireToolCall is one element of the tool_calls array in an assistant message.
type wireToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"` // always "function"
	Function wireFunctionCall `json:"function"`
}

// wireFunctionCall holds the function name and arguments for a tool_call.
type wireFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // raw JSON string
}

// wireToolDef is one element of the tools array.
type wireToolDef struct {
	Type     string          `json:"type"` // always "function"
	Function wireFunctionDef `json:"function"`
}

// wireFunctionDef describes a function tool.
type wireFunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
	Strict      *bool           `json:"strict,omitempty"`
}

// wireResponseFormat holds the response_format object.
type wireResponseFormat struct {
	Type       string               `json:"type"`
	JSONSchema *wireJSONSchemaBlock `json:"json_schema,omitempty"`
}

// wireJSONSchemaBlock holds the json_schema sub-object.
type wireJSONSchemaBlock struct {
	Name   string          `json:"name"`
	Strict bool            `json:"strict"`
	Schema json.RawMessage `json:"schema"`
}

// wireUsage mirrors the API usage object.
type wireUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

func (u wireUsage) toUsage() llmclient.Usage {
	return llmclient.Usage{
		InputTokens:  u.PromptTokens,
		OutputTokens: u.CompletionTokens,
	}
}

// wireResponse is the non-streaming response body.
type wireResponse struct {
	Choices []wireChoice `json:"choices"`
	Usage   wireUsage    `json:"usage"`
}

// wireChoice is one element of the choices array.
type wireChoice struct {
	Message      wireChoiceMessage `json:"message"`
	FinishReason string            `json:"finish_reason"`
}

// wireChoiceMessage is the message field inside a choice.
type wireChoiceMessage struct {
	Content   *string         `json:"content"`
	ToolCalls json.RawMessage `json:"tool_calls,omitempty"`
}

// buildBody maps req to the Chat Completions request JSON. All pre-HTTP
// validation happens here and returns typed llmclient errors.
func (c *Client) buildBody(req llmclient.Request, stream bool) ([]byte, error) {
	model := req.Model
	if model == "" {
		model = c.model
	}
	if model == "" {
		return nil, &llmclient.InvalidRequestError{Message: "openai: no model: Request.Model and Config.DefaultModel are both empty"}
	}

	// Optional caller-supplied model-capability validation.
	if c.capabilityCheck != nil {
		if err := c.capabilityCheck(req); err != nil {
			return nil, err
		}
	}

	// Thinking mapping decision:
	// OpenAI has no standard budget_tokens field. Effort → reasoning_effort
	// is handled below. If Thinking.Type is set and there is no Effort to map
	// from, we cannot silently drop the intent — return a typed error. Callers
	// that want extended reasoning on o-series models should set Request.Effort
	// ("low"/"medium"/"high") which maps to reasoning_effort, or pass
	// ProviderParams directly for provider-specific fields not yet modelled.
	if req.Thinking != nil && req.Thinking.Type != "" {
		return nil, &llmclient.InvalidRequestError{Message: fmt.Sprintf(
			"openai: Request.Thinking.Type %q cannot be mapped to an OpenAI wire field; "+
				"use Request.Effort (\"low\"/\"medium\"/\"high\") for reasoning_effort on o-series models, "+
				"or pass provider-specific params via Request.ProviderParams",
			req.Thinking.Type,
		)}
	}

	if err := llmclient.ValidateProviderParams(req.ProviderParams, "openai"); err != nil {
		return nil, err
	}

	if err := llmclient.ValidateOutputSchema(req.OutputSchema, "openai"); err != nil {
		return nil, err
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = c.maxTokens
	}

	// Build the messages array.
	msgs, err := buildMessages(req)
	if err != nil {
		return nil, err
	}

	w := wireRequest{
		Model:               model,
		Messages:            msgs,
		MaxCompletionTokens: maxTokens,
		Stop:                req.StopSequences,
		Stream:              stream,
	}
	if stream {
		w.StreamOptions = &wireStreamOptions{IncludeUsage: true}
	}

	if req.Temperature != 0 {
		t := req.Temperature
		w.Temperature = &t
	}

	// Effort → reasoning_effort.
	if req.Effort != "" {
		w.ReasoningEffort = req.Effort
	}

	// Map tools.
	for _, t := range req.Tools {
		if len(t.Raw) > 0 {
			// Verbatim passthrough for server-tools and future tool types.
			var obj map[string]json.RawMessage
			if err := json.Unmarshal(t.Raw, &obj); err != nil {
				return nil, &llmclient.InvalidRequestError{Message: fmt.Sprintf(
					"openai: ToolDef.Raw for tool %q is not a valid JSON object: %v", t.Name, err,
				)}
			}
			w.Tools = append(w.Tools, t.Raw)
			continue
		}
		schema := t.InputSchema
		if len(schema) == 0 {
			schema = json.RawMessage(`{"type":"object"}`)
		}
		fd := wireFunctionDef{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  schema,
		}
		if t.Strict {
			v := true
			fd.Strict = &v
		}
		td := wireToolDef{Type: "function", Function: fd}
		raw, err := json.Marshal(td)
		if err != nil {
			return nil, &llmclient.InvalidRequestError{Message: fmt.Sprintf("openai: encoding tool %q: %v", t.Name, err)}
		}
		w.Tools = append(w.Tools, raw)
	}

	// Map ToolChoice.
	tc, err := mapToolChoice(req.ToolChoice)
	if err != nil {
		return nil, err
	}
	w.ToolChoice = tc

	// Map OutputSchema → response_format with OpenAI json_schema wrapper.
	if len(req.OutputSchema) > 0 {
		w.ResponseFormat = &wireResponseFormat{
			Type: "json_schema",
			JSONSchema: &wireJSONSchemaBlock{
				Name:   "response",
				Strict: true,
				Schema: req.OutputSchema,
			},
		}
	}

	body, err := json.Marshal(w)
	if err != nil {
		return nil, &llmclient.InvalidRequestError{Message: fmt.Sprintf("openai: encoding request: %v", err)}
	}

	// Merge ProviderParams VERBATIM into the top-level body object. Keys the
	// adapter already set are overridden by ProviderParams — this is the
	// caller's escape hatch. See the llmclient package doc.
	if len(req.ProviderParams) > 0 {
		body, err = mergeProviderParams(body, req.ProviderParams, "openai")
		if err != nil {
			return nil, err
		}
	}

	return body, nil
}

// buildMessages maps a llmclient.Request to the OpenAI messages array:
//   - If System is non-empty, prepend {"role":"system","content":"..."}.
//   - If Messages is non-empty, map each message; otherwise use Prompt.
func buildMessages(req llmclient.Request) ([]wireMessage, error) {
	var out []wireMessage

	if req.System != "" {
		sysRaw, err := json.Marshal(req.System)
		if err != nil {
			return nil, &llmclient.InvalidRequestError{Message: fmt.Sprintf("openai: encoding system prompt: %v", err)}
		}
		out = append(out, wireMessage{Role: "system", Content: sysRaw})
	}

	msgs := req.Messages
	if len(msgs) == 0 {
		textRaw, err := json.Marshal(req.Prompt)
		if err != nil {
			return nil, &llmclient.InvalidRequestError{Message: fmt.Sprintf("openai: encoding prompt: %v", err)}
		}
		out = append(out, wireMessage{Role: "user", Content: textRaw})
		return out, nil
	}

	for _, m := range msgs {
		wmsgs, err := mapMessage(m)
		if err != nil {
			return nil, err
		}
		out = append(out, wmsgs...)
	}
	return out, nil
}

// mapMessage maps one llmclient.Message to one or more wireMessages.
func mapMessage(m llmclient.Message) ([]wireMessage, error) {
	switch m.Role {
	case llmclient.RoleUser:
		return mapUserMessage(m)
	case llmclient.RoleAssistant:
		return mapAssistantMessage(m)
	default:
		return nil, &llmclient.InvalidRequestError{Message: fmt.Sprintf("openai: unsupported message role %q", m.Role)}
	}
}

// mapUserMessage maps a user-role message. tool_result blocks are emitted as
// separate {"role":"tool",...} messages; remaining blocks form the user message
// content.
func mapUserMessage(m llmclient.Message) ([]wireMessage, error) {
	var toolMsgs []wireMessage
	var contentParts []json.RawMessage

	for _, b := range m.Content {
		if b.Type == llmclient.BlockToolResult {
			contentRaw, err := json.Marshal(b.Content)
			if err != nil {
				return nil, &llmclient.InvalidRequestError{Message: fmt.Sprintf("openai: encoding tool_result content: %v", err)}
			}
			toolMsgs = append(toolMsgs, wireMessage{
				Role:       "tool",
				ToolCallID: b.ToolUseID,
				Content:    contentRaw,
			})
			continue
		}
		part, err := marshalContentPart(b)
		if err != nil {
			return nil, err
		}
		contentParts = append(contentParts, part)
	}

	var out []wireMessage
	if len(contentParts) > 0 {
		raw, err := marshalContent(contentParts)
		if err != nil {
			return nil, err
		}
		out = append(out, wireMessage{Role: "user", Content: raw})
	}
	out = append(out, toolMsgs...)
	return out, nil
}

// mapAssistantMessage maps an assistant-role message. tool_use blocks become
// the tool_calls array; text/image blocks become content.
func mapAssistantMessage(m llmclient.Message) ([]wireMessage, error) {
	var toolCalls []wireToolCall
	var contentParts []json.RawMessage

	for _, b := range m.Content {
		switch b.Type {
		case llmclient.BlockToolUse:
			input := b.Input
			if len(input) == 0 {
				input = json.RawMessage(`{}`)
			}
			toolCalls = append(toolCalls, wireToolCall{
				ID:   b.ID,
				Type: "function",
				Function: wireFunctionCall{
					Name:      b.Name,
					Arguments: string(input),
				},
			})
		case llmclient.BlockUnknown:
			// BlockUnknown in the assistant turn: emit the raw bytes as a text
			// content part so multi-turn history does not drop data. The model
			// will see the raw JSON text on the next turn. OpenAI has no
			// native "unknown" block type; this is the best available fallback.
			if len(b.Raw) > 0 {
				raw, err := json.Marshal(map[string]string{"type": "text", "text": string(b.Raw)})
				if err != nil {
					return nil, &llmclient.InvalidRequestError{Message: fmt.Sprintf("openai: encoding unknown block: %v", err)}
				}
				contentParts = append(contentParts, raw)
			}
		default:
			raw, err := marshalContentPart(b)
			if err != nil {
				return nil, err
			}
			contentParts = append(contentParts, raw)
		}
	}

	msg := wireMessage{Role: "assistant"}
	if len(contentParts) > 0 {
		raw, err := marshalContent(contentParts)
		if err != nil {
			return nil, err
		}
		msg.Content = raw
	}
	if len(toolCalls) > 0 {
		tcRaw, err := json.Marshal(toolCalls)
		if err != nil {
			return nil, &llmclient.InvalidRequestError{Message: fmt.Sprintf("openai: encoding tool_calls: %v", err)}
		}
		msg.ToolCalls = tcRaw
	}
	return []wireMessage{msg}, nil
}

// marshalContent emits a single-element text array as a plain JSON string
// (OpenAI accepts string or array for content; string is more compact for the
// common case). Multi-element or non-text arrays are emitted as arrays.
func marshalContent(parts []json.RawMessage) (json.RawMessage, error) {
	if len(parts) == 1 {
		var typed struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal(parts[0], &typed); err == nil && typed.Type == "text" {
			raw, err := json.Marshal(typed.Text)
			if err != nil {
				return nil, &llmclient.InvalidRequestError{Message: fmt.Sprintf("openai: encoding text: %v", err)}
			}
			return raw, nil
		}
	}
	raw, err := json.Marshal(parts)
	if err != nil {
		return nil, &llmclient.InvalidRequestError{Message: fmt.Sprintf("openai: encoding content parts: %v", err)}
	}
	return raw, nil
}

// marshalContentPart maps one ContentBlock to an OpenAI content-part object.
func marshalContentPart(b llmclient.ContentBlock) (json.RawMessage, error) {
	switch b.Type {
	case llmclient.BlockText:
		raw, err := json.Marshal(map[string]string{"type": "text", "text": b.Text})
		if err != nil {
			return nil, &llmclient.InvalidRequestError{Message: fmt.Sprintf("openai: encoding text block: %v", err)}
		}
		return raw, nil
	case llmclient.BlockImage:
		if b.MediaType == "" || b.Data == "" {
			return nil, &llmclient.InvalidRequestError{Message: "openai: BlockImage requires MediaType and Data"}
		}
		url := "data:" + b.MediaType + ";base64," + b.Data
		raw, err := json.Marshal(map[string]interface{}{
			"type":      "image_url",
			"image_url": map[string]string{"url": url},
		})
		if err != nil {
			return nil, &llmclient.InvalidRequestError{Message: fmt.Sprintf("openai: encoding image block: %v", err)}
		}
		return raw, nil
	case llmclient.BlockUnknown:
		// Raw precedence: emit verbatim for BlockUnknown. This will only be
		// reached for unknown blocks in non-assistant positions.
		if len(b.Raw) > 0 {
			return b.Raw, nil
		}
		return nil, &llmclient.InvalidRequestError{Message: "openai: BlockUnknown content block with empty Raw"}
	default:
		return nil, &llmclient.InvalidRequestError{Message: fmt.Sprintf("openai: unsupported content block type %q as content part", b.Type)}
	}
}

// mapToolChoice maps llmclient.ToolChoice to an OpenAI tool_choice value.
func mapToolChoice(tc llmclient.ToolChoice) (*json.RawMessage, error) {
	switch tc.Mode {
	case llmclient.ToolChoiceAuto:
		return nil, nil // omit entirely — auto is the API default
	case llmclient.ToolChoiceAny:
		raw := json.RawMessage(`"required"`)
		return &raw, nil
	case llmclient.ToolChoiceNone:
		raw := json.RawMessage(`"none"`)
		return &raw, nil
	case llmclient.ToolChoiceTool:
		if tc.Name == "" {
			return nil, &llmclient.InvalidRequestError{Message: `openai: ToolChoice.Mode is "tool" but ToolChoice.Name is empty`}
		}
		obj := map[string]interface{}{
			"type":     "function",
			"function": map[string]string{"name": tc.Name},
		}
		raw, err := json.Marshal(obj)
		if err != nil {
			return nil, &llmclient.InvalidRequestError{Message: fmt.Sprintf("openai: encoding tool_choice: %v", err)}
		}
		r := json.RawMessage(raw)
		return &r, nil
	default:
		// Unknown future mode: pass through as a string.
		raw, err := json.Marshal(string(tc.Mode))
		if err != nil {
			return nil, &llmclient.InvalidRequestError{Message: fmt.Sprintf("openai: encoding tool_choice mode: %v", err)}
		}
		r := json.RawMessage(raw)
		return &r, nil
	}
}

// parseBlocks maps an OpenAI choice message to llmclient.ContentBlocks.
func parseBlocks(choice wireChoiceMessage) ([]llmclient.ContentBlock, error) {
	var blocks []llmclient.ContentBlock

	if choice.Content != nil {
		text := *choice.Content
		if text != "" {
			blocks = append(blocks, llmclient.TextBlock(text))
		}
	}

	if len(choice.ToolCalls) > 0 {
		var rawCalls []json.RawMessage
		if err := json.Unmarshal(choice.ToolCalls, &rawCalls); err != nil {
			return nil, fmt.Errorf("openai: parsing tool_calls array: %w", err)
		}
		var calls []struct {
			ID       string `json:"id"`
			Type     string `json:"type"`
			Function struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			} `json:"function"`
		}
		if err := json.Unmarshal(choice.ToolCalls, &calls); err != nil {
			return nil, fmt.Errorf("openai: parsing tool_calls: %w", err)
		}
		for i, call := range calls {
			b := llmclient.ToolUseBlock(call.ID, call.Function.Name, json.RawMessage(call.Function.Arguments))
			if i < len(rawCalls) {
				b.Raw = rawCalls[i]
			}
			blocks = append(blocks, b)
		}
	}

	return blocks, nil
}

// mapFinishReason maps an OpenAI finish_reason to a llmclient.StopReason.
func mapFinishReason(reason string) llmclient.StopReason {
	switch reason {
	case "stop":
		return llmclient.StopEndTurn
	case "tool_calls":
		return llmclient.StopToolUse
	case "length":
		return llmclient.StopMaxTokens
	case "content_filter":
		// Content filter: map to StopEndTurn; the response may carry text.
		return llmclient.StopEndTurn
	default:
		if reason == "" {
			return llmclient.StopEndTurn
		}
		// Forward-compatible: preserve unknown reasons as-is.
		return llmclient.StopReason(reason)
	}
}

// textOf returns the concatenation of all text blocks.
func textOf(blocks []llmclient.ContentBlock) string {
	var s string
	for _, b := range blocks {
		if b.Type == llmclient.BlockText {
			s += b.Text
		}
	}
	return s
}

// mergeProviderParams merges caller-supplied ProviderParams into the adapter
// body. Params keys OVERRIDE adapter-set keys (documented precedence).
func mergeProviderParams(body, params json.RawMessage, prefix string) ([]byte, error) {
	var base map[string]json.RawMessage
	if err := json.Unmarshal(body, &base); err != nil {
		return nil, &llmclient.InvalidRequestError{Message: fmt.Sprintf(
			"%s: internal: could not re-parse own request body for ProviderParams merge: %v", prefix, err,
		)}
	}
	var overlay map[string]json.RawMessage
	if err := json.Unmarshal(params, &overlay); err != nil {
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
