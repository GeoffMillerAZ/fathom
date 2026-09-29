package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

func responsesInvalid(field string) error {
	return &llmclient.InvalidRequestError{Message: "openai responses: unsupported or invalid " + field}
}

// responsesReservedParams are owned by the normalized Request mapping. Optional
// provider extensions may pass through ProviderParams, but they cannot alter
// the selected lane, storage posture, normalized conversation, tools, or model.
var responsesReservedParams = map[string]struct{}{
	"model": {}, "input": {}, "max_output_tokens": {}, "stream": {}, "store": {},
	"instructions": {}, "temperature": {}, "reasoning": {}, "text": {},
	"tools": {}, "tool_choice": {},
	"previous_response_id": {}, "conversation": {}, "prompt": {}, "truncation": {},
	"parallel_tool_calls": {}, "max_tool_calls": {}, "background": {}, "stream_options": {},
}

func responsesProviderParams(raw json.RawMessage) (map[string]json.RawMessage, error) {
	if err := llmclient.ValidateProviderParams(raw, "openai responses"); err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, nil
	}
	var params map[string]json.RawMessage
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, responsesInvalid("ProviderParams")
	}
	for key := range params {
		if _, reserved := responsesReservedParams[key]; reserved {
			return nil, responsesInvalid("ProviderParams." + key)
		}
	}
	return params, nil
}

// buildResponsesBody preserves conversation order and disables implicit server
// storage. ProviderParams is limited to trusted optional provider extensions.
func (c *Client) buildResponsesBody(req llmclient.Request, stream bool) ([]byte, error) {
	if req.Thinking != nil {
		return nil, responsesInvalid("Thinking; use Effort")
	}
	if len(req.StopSequences) != 0 {
		return nil, responsesInvalid("StopSequences")
	}
	if req.SystemCacheHint {
		return nil, responsesInvalid("SystemCacheHint")
	}
	if req.MaxTokens < 0 {
		return nil, responsesInvalid("MaxTokens")
	}
	params, err := responsesProviderParams(req.ProviderParams)
	if err != nil {
		return nil, err
	}
	if err := llmclient.ValidateOutputSchema(req.OutputSchema, "openai responses"); err != nil {
		return nil, err
	}
	model := req.Model
	if model == "" {
		model = c.model
	}
	if model == "" {
		return nil, responsesInvalid("Model")
	}
	if c.capabilityCheck != nil {
		resolved := req
		resolved.Model = model
		if err := c.capabilityCheck(resolved); err != nil {
			return nil, err
		}
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = c.maxTokens
	}
	input := make([]any, 0)
	msgs := req.Messages
	if len(msgs) == 0 {
		msgs = []llmclient.Message{{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{llmclient.TextBlock(req.Prompt)}}}
	}
	for _, msg := range msgs {
		if msg.Role != llmclient.RoleUser && msg.Role != llmclient.RoleAssistant {
			return nil, responsesInvalid("message role")
		}
		for _, b := range msg.Content {
			if b.CacheHint {
				return nil, responsesInvalid("ContentBlock.CacheHint")
			}
			switch b.Type {
			case llmclient.BlockText:
				input = append(input, map[string]any{"role": msg.Role, "content": b.Text})
			case llmclient.BlockImage:
				if msg.Role != llmclient.RoleUser {
					return nil, responsesInvalid("assistant image")
				}
				input = append(input, map[string]any{"role": msg.Role, "content": []any{map[string]any{"type": "input_image", "image_url": "data:" + b.MediaType + ";base64," + b.Data}}})
			case llmclient.BlockToolUse:
				if msg.Role != llmclient.RoleAssistant || b.ID == "" || b.Name == "" || !json.Valid(b.Input) {
					return nil, responsesInvalid("function call")
				}
				input = append(input, map[string]any{"type": "function_call", "call_id": b.ID, "name": b.Name, "arguments": string(b.Input)})
			case llmclient.BlockToolResult:
				if msg.Role != llmclient.RoleUser || b.ToolUseID == "" {
					return nil, responsesInvalid("function call output")
				}
				output := b.Content
				if b.IsError {
					output = "Error: " + output
				}
				input = append(input, map[string]any{"type": "function_call_output", "call_id": b.ToolUseID, "output": output})
			case llmclient.BlockUnknown:
				if err := llmclient.ValidateProviderParams(b.Raw, "openai responses block"); err != nil || len(b.Raw) == 0 {
					return nil, responsesInvalid("unknown block Raw")
				}
				input = append(input, b.Raw)
			default:
				return nil, responsesInvalid("content block type")
			}
		}
	}
	body := map[string]any{"model": model, "input": input, "max_output_tokens": maxTokens, "stream": stream, "store": false}
	if req.System != "" {
		body["instructions"] = req.System
	}
	if req.Temperature != 0 {
		body["temperature"] = req.Temperature
	}
	if req.Effort != "" {
		body["reasoning"] = map[string]any{"effort": req.Effort}
	}
	if len(req.OutputSchema) > 0 {
		body["text"] = map[string]any{"format": map[string]any{"type": "json_schema", "name": "response", "strict": true, "schema": req.OutputSchema}}
	}
	var defs []any
	for _, t := range req.Tools {
		if len(t.Raw) > 0 {
			if err := llmclient.ValidateProviderParams(t.Raw, "openai responses tool"); err != nil {
				return nil, err
			}
			defs = append(defs, t.Raw)
			continue
		}
		if t.CacheHint {
			return nil, responsesInvalid("ToolDef.CacheHint")
		}
		if t.Name == "" {
			return nil, responsesInvalid("ToolDef.Name")
		}
		schema := t.InputSchema
		if len(schema) == 0 {
			schema = json.RawMessage(`{"type":"object"}`)
		}
		if err := llmclient.ValidateOutputSchema(schema, "openai responses tool"); err != nil {
			return nil, err
		}
		// Explicit false preserves the common ToolDef default: Responses otherwise
		// normalizes omitted strict into strict schemas, changing optional fields.
		defs = append(defs, map[string]any{"type": "function", "name": t.Name, "description": t.Description, "parameters": schema, "strict": t.Strict})
	}
	if len(defs) > 0 {
		body["tools"] = defs
	}
	switch req.ToolChoice.Mode {
	case llmclient.ToolChoiceAuto:
	case llmclient.ToolChoiceAny:
		body["tool_choice"] = "required"
	case llmclient.ToolChoiceNone:
		body["tool_choice"] = "none"
	case llmclient.ToolChoiceTool:
		if req.ToolChoice.Name == "" {
			return nil, responsesInvalid("ToolChoice.Name")
		}
		body["tool_choice"] = map[string]any{"type": "function", "name": req.ToolChoice.Name}
	default:
		return nil, responsesInvalid("ToolChoice.Mode")
	}
	for k, v := range params {
		body[k] = v
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, responsesInvalid("request JSON")
	}
	return encoded, nil
}

type responsesWire struct {
	Status string            `json:"status"`
	Output []json.RawMessage `json:"output"`
	Error  *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Incomplete *struct {
		Reason string `json:"reason"`
	} `json:"incomplete_details"`
	Usage *struct {
		Input   int `json:"input_tokens"`
		Output  int `json:"output_tokens"`
		Details struct {
			Cached int `json:"cached_tokens"`
		} `json:"input_tokens_details"`
	} `json:"usage"`
}

func (c *Client) responsesError(message string) error {
	return &llmclient.ServerError{Message: c.redact("openai responses: " + message)}
}

func (c *Client) parseResponse(raw []byte) (llmclient.Response, error) {
	var w responsesWire
	if err := json.Unmarshal(raw, &w); err != nil {
		return llmclient.Response{}, c.responsesError("malformed response JSON")
	}
	if w.Error != nil {
		return llmclient.Response{}, c.responsesError(w.Error.Code + ": " + w.Error.Message)
	}
	resp := llmclient.Response{StopReason: llmclient.StopEndTurn}
	if w.Usage == nil || w.Usage.Input < 0 || w.Usage.Output < 0 || w.Usage.Details.Cached < 0 || w.Usage.Details.Cached > w.Usage.Input {
		return resp, c.responsesError("missing or invalid usage")
	}
	// InputTokens follows the common non-cached-input convention used by the
	// cost middleware; cached input must not be billed again at the full rate.
	resp.Usage = llmclient.Usage{InputTokens: w.Usage.Input - w.Usage.Details.Cached, OutputTokens: w.Usage.Output, CacheReadInputTokens: w.Usage.Details.Cached}
	for _, rawItem := range w.Output {
		blocks, refusal, err := c.parseResponseItem(rawItem)
		if err != nil {
			return resp, err
		}
		resp.Content = append(resp.Content, blocks...)
		if refusal {
			resp.StopReason = llmclient.StopRefusal
		}
	}
	if len(resp.ToolCalls()) > 0 && resp.StopReason != llmclient.StopRefusal {
		resp.StopReason = llmclient.StopToolUse
	}
	switch w.Status {
	case "completed":
	case "incomplete":
		if w.Incomplete == nil {
			return resp, c.responsesError("missing incomplete_details")
		}
		switch w.Incomplete.Reason {
		case "max_output_tokens":
			resp.StopReason = llmclient.StopMaxTokens
		case "content_filter":
			resp.StopReason = llmclient.StopRefusal
		default:
			return resp, c.responsesError("unknown incomplete reason: " + w.Incomplete.Reason)
		}
	default:
		return resp, c.responsesError("nonterminal or unknown response status: " + w.Status)
	}
	resp.Text = textOf(resp.Content)
	return resp, nil
}

func (c *Client) parseResponseItem(raw json.RawMessage) ([]llmclient.ContentBlock, bool, error) {
	var item struct {
		Type      string `json:"type"`
		CallID    string `json:"call_id"`
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
		Status    string `json:"status"`
		Content   []struct {
			Type    string `json:"type"`
			Text    string `json:"text"`
			Refusal string `json:"refusal"`
		} `json:"content"`
	}
	if json.Unmarshal(raw, &item) != nil || item.Type == "" {
		return nil, false, c.responsesError("malformed output item")
	}
	var blocks []llmclient.ContentBlock
	refusal := false
	switch item.Type {
	case "message":
		for _, part := range item.Content {
			switch part.Type {
			case "output_text":
				blocks = append(blocks, llmclient.TextBlock(part.Text))
			case "refusal":
				refusal = true
				blocks = append(blocks, llmclient.TextBlock(part.Refusal))
			default:
				return nil, false, c.responsesError("unsupported message content type: " + part.Type)
			}
		}
	case "function_call":
		if item.Status == "incomplete" {
			// Preserve partial arguments for inspection, never dispatch them as
			// an executable function call after an output limit.
			return []llmclient.ContentBlock{{Type: llmclient.BlockUnknown, Raw: append(json.RawMessage(nil), raw...)}}, false, nil
		}
		if item.Status != "completed" {
			return nil, false, c.responsesError("function call is not completed")
		}
		if item.CallID == "" || item.Name == "" || !json.Valid([]byte(item.Arguments)) {
			return nil, false, c.responsesError("malformed function call")
		}
		blocks = append(blocks, llmclient.ToolUseBlock(item.CallID, item.Name, json.RawMessage(item.Arguments)))
	default:
		blocks = append(blocks, llmclient.ContentBlock{Type: llmclient.BlockUnknown, Raw: append(json.RawMessage(nil), raw...)})
	}
	return blocks, refusal, nil
}

func (c *Client) completeResponses(ctx context.Context, req llmclient.Request) (llmclient.Response, error) {
	body, err := c.buildResponsesBody(req, false)
	if err != nil {
		return llmclient.Response{}, err
	}
	resp, err := c.do(ctx, c.completeClient, body, "application/json")
	if err != nil {
		return llmclient.Response{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return llmclient.Response{}, c.mapHTTPError(resp)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
	if ctx.Err() != nil {
		return llmclient.Response{}, ctx.Err()
	}
	if err != nil {
		return llmclient.Response{}, c.responsesError(fmt.Sprintf("read body: %v", err))
	}
	if len(raw) > maxBodyBytes {
		return llmclient.Response{}, c.responsesError("response exceeds byte cap")
	}
	return c.parseResponse(raw)
}
