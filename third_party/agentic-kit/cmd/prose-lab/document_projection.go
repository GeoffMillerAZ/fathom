package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/internal/jsonwire"
)

// documentTextProjection identifies the exact Anthropic non-answer metadata
// admitted by this command-local view. The lab client has already retained and
// priced the original HTTP response before this view reaches the prose engine.
const documentTextProjection = "anthropic-thinking-text-projection/v1"
const maxDocumentThinkingBlockBytes = 64 << 10

type documentTextClient struct{ inner llmclient.LLMClient }

func (c *documentTextClient) Complete(ctx context.Context, req llmclient.Request) (llmclient.Response, error) {
	resp, err := c.inner.Complete(ctx, req)
	if err != nil {
		return resp, err
	}
	if err = ctx.Err(); err != nil {
		return llmclient.Response{}, err
	}
	kept := make([]llmclient.ContentBlock, 0, len(resp.Content))
	for _, block := range resp.Content {
		metadata, err := documentThinkingMetadata(block)
		if err != nil {
			return llmclient.Response{}, err
		}
		if !metadata {
			kept = append(kept, block)
		}
	}
	if strings.TrimSpace(resp.Text) == "" {
		return llmclient.Response{}, errors.New("document response has no text answer")
	}
	resp.Content = kept // Text, usage, stop reason and all other response fields stay exact.
	return resp, nil
}
func (c *documentTextClient) Stream(context.Context, llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	return nil, errors.New("document study does not stream")
}

// Unknown blocks of any other type remain verbatim. The engine/reviewer refuses
// them; no generic unknown-block exemption is created here.
func documentThinkingMetadata(block llmclient.ContentBlock) (bool, error) {
	if block.Type != llmclient.BlockUnknown {
		return false, nil
	}
	if len(block.Raw) == 0 || len(block.Raw) > maxDocumentThinkingBlockBytes {
		return false, nil
	}
	object, err := jsonwire.Object(block.Raw)
	if err != nil {
		return false, nil
	}
	var kind string
	if err = json.Unmarshal(object["type"], &kind); err != nil {
		return false, nil
	}
	if kind != "thinking" && kind != "redacted_thinking" {
		return false, nil
	}
	if block.Text != "" || block.MediaType != "" || block.Data != "" || block.ID != "" || block.Name != "" || len(block.Input) != 0 || block.ToolUseID != "" || block.Content != "" || block.IsError || block.CacheHint {
		return false, errors.New("thinking block has non-raw content")
	}
	if kind == "thinking" {
		if err = exactObject(block.Raw, "type", "thinking", "signature"); err != nil {
			return false, err
		}
		var value struct {
			Type      string `json:"type"`
			Thinking  string `json:"thinking"`
			Signature string `json:"signature"`
		}
		// The provider's default omitted display returns an empty thinking
		// string with an opaque signature. Field presence/type still matter.
		if err = jsonwire.DecodeStrict(block.Raw, &value); err != nil || value.Type != "thinking" || value.Signature == "" {
			return false, errors.New("malformed thinking metadata")
		}
	} else {
		if err = exactObject(block.Raw, "type", "data"); err != nil {
			return false, err
		}
		var value struct {
			Type string `json:"type"`
			Data string `json:"data"`
		}
		if err = jsonwire.DecodeStrict(block.Raw, &value); err != nil || value.Type != "redacted_thinking" || value.Data == "" {
			return false, errors.New("malformed redacted thinking metadata")
		}
	}
	return true, nil
}
