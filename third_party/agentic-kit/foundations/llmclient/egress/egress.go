package llmegress

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// Transformer is the egress seam: every outbound text-bearing field passes
// through TransformOutbound before the inner client sees it. Implementations
// rewrite text that must not leave the trust boundary verbatim — tokenize it
// (GateFromVault), redact it (GateFromDLP), or apply any caller-defined
// policy.
//
// FAIL CLOSED: a non-nil error from TransformOutbound aborts the entire call
// with a typed *TransformError; the inner client is never invoked with text
// the Transformer did not approve. Returning the input unchanged with a nil
// error is an explicit approval, not a fallback.
//
// Transformers used with a concurrently-used client must be safe for
// concurrent use.
type Transformer interface {
	TransformOutbound(ctx context.Context, text string) (string, error)
}

// TransformerFunc adapts a plain function to the Transformer interface.
type TransformerFunc func(ctx context.Context, text string) (string, error)

// TransformOutbound calls f(ctx, text).
func (f TransformerFunc) TransformOutbound(ctx context.Context, text string) (string, error) {
	return f(ctx, text)
}

// TransformError is the typed failure WithEgress returns when the Transformer
// rejects an outbound field. It is the fail-closed signal: when a call fails
// with a TransformError, the inner client was NOT invoked and nothing left
// the process.
//
// Field is the Go path of the offending request field, e.g. "Prompt",
// "System", "Messages[2].Content[0].Text", "Messages[1].Content[0].Content"
// (a tool_result), or — under TransformToolJSON —
// "Messages[0].Content[1].Input" / "Messages[0].Content[2].Raw".
type TransformError struct {
	// Field is the Go path of the request field whose transformation failed.
	Field string

	// Err is the Transformer's error.
	Err error
}

// Error implements the error interface, naming the request field whose
// outbound transformation was rejected and the underlying cause.
func (e *TransformError) Error() string {
	return fmt.Sprintf("llmegress: transform %s: %v", e.Field, e.Err)
}

// Unwrap returns the Transformer's underlying error so errors.Is/As reach it.
func (e *TransformError) Unwrap() error { return e.Err }

// Option configures WithEgress.
type Option func(*egressClient)

// TransformToolJSON opts tool_use Input and BlockUnknown Raw into the
// outbound transformation. They are EXCLUDED by default because they are
// structured JSON and blind string transformation corrupts structured JSON: a
// redaction label inserted mid-document breaks parsing, and a substitution
// inside a JSON string silently changes tool semantics. Opt in only when your
// Transformer is JSON-shape-safe; see the package doc.
func TransformToolJSON() Option {
	return func(c *egressClient) { c.toolJSON = true }
}

// egressClient is the LLMClient wrapper returned by WithEgress. It holds no
// mutable state after construction.
type egressClient struct {
	inner    llmclient.LLMClient
	t        Transformer
	toolJSON bool
}

var _ llmclient.LLMClient = (*egressClient)(nil)

// WithEgress wraps inner so that every outbound text-bearing field of every
// request — Prompt, System, and each message block's Text and tool_result
// Content — is passed through t before inner sees it. Any Transformer error
// aborts the call with a typed *TransformError and inner is not invoked
// (fail closed). The request inner receives is a deep copy; the caller's
// Request, Messages, and blocks are never mutated. See the package doc for
// the tool-JSON policy, streaming semantics, and the recommended composition
// order (WithEgress goes OUTSIDE WithRetry).
//
// inner and t must not be nil; WithEgress panics otherwise — a nil seam is a
// construction-time wiring bug, not a runtime condition to limp through.
func WithEgress(inner llmclient.LLMClient, t Transformer, opts ...Option) llmclient.LLMClient {
	if inner == nil {
		panic("llmegress: WithEgress: inner must not be nil")
	}
	if t == nil {
		panic("llmegress: WithEgress: transformer must not be nil")
	}
	c := &egressClient{inner: inner, t: t}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Complete transforms every outbound text field of req and delegates the
// gated deep copy to the inner client. A Transformer failure returns a
// *TransformError without invoking the inner client. The response is inbound
// traffic and passes through verbatim.
func (c *egressClient) Complete(ctx context.Context, req llmclient.Request) (llmclient.Response, error) {
	gated, err := c.transformRequest(ctx, req)
	if err != nil {
		return llmclient.Response{}, err
	}
	return c.inner.Complete(ctx, gated)
}

// Stream transforms every outbound text field of req and delegates the gated
// deep copy to the inner client. A Transformer failure is a pre-stream
// failure: it returns (nil, *TransformError) and the inner client is never
// invoked. The returned channel is the inner client's channel — inbound
// chunks are not touched (see the package doc, "Streaming and inbound
// traffic").
func (c *egressClient) Stream(ctx context.Context, req llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	gated, err := c.transformRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	return c.inner.Stream(ctx, gated)
}

// transformRequest returns a deep copy of req with every outbound text field
// transformed. On any Transformer error it returns a zero Request and a
// *TransformError naming the offending field.
func (c *egressClient) transformRequest(ctx context.Context, req llmclient.Request) (llmclient.Request, error) {
	out := req // value copy; every reference-typed field is replaced below

	var err error
	if out.Prompt, err = c.transformText(ctx, "Prompt", req.Prompt); err != nil {
		return llmclient.Request{}, err
	}
	if out.System, err = c.transformText(ctx, "System", req.System); err != nil {
		return llmclient.Request{}, err
	}

	out.StopSequences = slices.Clone(req.StopSequences)
	out.Tools = cloneTools(req.Tools)

	if len(req.Messages) > 0 {
		msgs := make([]llmclient.Message, len(req.Messages))
		for i, m := range req.Messages {
			blocks, err := c.transformBlocks(ctx, i, m.Content)
			if err != nil {
				return llmclient.Request{}, err
			}
			msgs[i] = llmclient.Message{Role: m.Role, Content: blocks}
		}
		out.Messages = msgs
	}
	return out, nil
}

// transformBlocks transforms each block of one message into a fresh slice,
// leaving the input untouched.
func (c *egressClient) transformBlocks(ctx context.Context, msgIdx int, blocks []llmclient.ContentBlock) ([]llmclient.ContentBlock, error) {
	if len(blocks) == 0 {
		return nil, nil
	}
	out := make([]llmclient.ContentBlock, len(blocks))
	for j, b := range blocks {
		nb, err := c.transformBlock(ctx, msgIdx, j, b)
		if err != nil {
			return nil, err
		}
		out[j] = nb
	}
	return out, nil
}

// transformBlock gates one content block BY FIELD, not by declared block
// type, so a mis-tagged block cannot smuggle text past the gate: Text and
// Content (the string fields) are always transformed; Input and Raw (the
// JSON fields) follow the tool-JSON policy; ID/Name/ToolUseID are protocol
// identifiers and pass through.
func (c *egressClient) transformBlock(ctx context.Context, msgIdx, blockIdx int, b llmclient.ContentBlock) (llmclient.ContentBlock, error) {
	out := b // value copy; json.RawMessage fields are re-cloned below

	path := func(field string) string {
		return fmt.Sprintf("Messages[%d].Content[%d].%s", msgIdx, blockIdx, field)
	}

	var err error
	if out.Text, err = c.transformText(ctx, path("Text"), b.Text); err != nil {
		return llmclient.ContentBlock{}, err
	}
	if out.Content, err = c.transformText(ctx, path("Content"), b.Content); err != nil {
		return llmclient.ContentBlock{}, err
	}
	if out.Input, err = c.transformJSON(ctx, path("Input"), b.Input); err != nil {
		return llmclient.ContentBlock{}, err
	}
	if out.Raw, err = c.transformJSON(ctx, path("Raw"), b.Raw); err != nil {
		return llmclient.ContentBlock{}, err
	}
	return out, nil
}

// transformText passes one text field through the Transformer. Empty fields
// skip the Transformer entirely: nothing leaves, so there is nothing to gate.
func (c *egressClient) transformText(ctx context.Context, field, text string) (string, error) {
	if text == "" {
		return "", nil
	}
	out, err := c.t.TransformOutbound(ctx, text)
	if err != nil {
		return "", &TransformError{Field: field, Err: err}
	}
	return out, nil
}

// transformJSON applies the tool-JSON policy to one raw-JSON field: by
// default the bytes are cloned untouched (structured JSON is not blindly
// string-transformed — see the package doc); under TransformToolJSON the
// JSON text is passed through the Transformer like any other field.
func (c *egressClient) transformJSON(ctx context.Context, field string, raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	if !c.toolJSON {
		return bytes.Clone(raw), nil
	}
	out, err := c.t.TransformOutbound(ctx, string(raw))
	if err != nil {
		return nil, &TransformError{Field: field, Err: err}
	}
	return json.RawMessage(out), nil
}

// cloneTools deep-copies the tool definitions (including InputSchema bytes)
// without transforming them — tool definitions are the caller's own authored
// text and schema, not user data; see the package doc.
func cloneTools(tools []llmclient.ToolDef) []llmclient.ToolDef {
	if len(tools) == 0 {
		return nil
	}
	out := make([]llmclient.ToolDef, len(tools))
	for i, t := range tools {
		t.InputSchema = bytes.Clone(t.InputSchema)
		out[i] = t
	}
	return out
}
