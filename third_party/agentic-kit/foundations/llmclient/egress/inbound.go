// Package llmegress — inbound detokenization.
//
// Asymmetry:
//
//   - Outbound (WithEgress + GateFromVault): fail-closed. Any error aborts
//     the call; the inner client is never invoked with un-gated text.
//   - Inbound (WithInboundDetokenize + DetokenizeText): best-effort. A token
//     that cannot be resolved (unknown, access denied, parse error) is left
//     verbatim in the response text. Detokenization NEVER turns a successful
//     response into an error.
//
// Recommended composition so the model response flows through both layers:
//
//	client := WithInboundDetokenize(
//	        WithEgress(base, gate, …),
//	        vault, namespace, principal, purpose)
//
// Response path: model → egress (verbatim passthrough) → inbound-detokenize
// (restore) → caller.
package llmegress

import (
	"context"
	"regexp"
	"strings"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/security/tokenization"
)

// tokenRe matches the opaque token strings emitted by GateFromVault:
// "tok_<class>_<hex>" where class is one or more word chars and hex is one or
// more lowercase hex digits. This intentionally matches the full token value
// as a single unit — no sub-group expansion.
var tokenRe = regexp.MustCompile(`tok_[a-zA-Z0-9]+_[0-9a-fA-F]+`)

// DetokenizeText scans text for token strings matching tokenRe, resolves each
// via vault.Detokenize, and returns the text with resolved tokens replaced by
// their original values. namespace is set on each parsed Token before the
// detokenize call (Namespace is not encoded in the opaque token string).
//
// Best-effort: any error from tokenization.Parse or vault.Detokenize leaves
// the original match text unchanged. DetokenizeText never returns an error —
// a response that arrived successfully is never aborted by detokenization
// failure. Unknown tokens, denied principals, and parse errors all silently
// pass the token through verbatim.
func DetokenizeText(vault tokenization.Vault, namespace string, principal tokenization.Principal, purpose, text string) string {
	locs := tokenRe.FindAllStringIndex(text, -1)
	if len(locs) == 0 {
		return text
	}
	var b strings.Builder
	last := 0
	for _, loc := range locs {
		b.WriteString(text[last:loc[0]])
		match := text[loc[0]:loc[1]]
		tok, err := tokenization.Parse(match)
		if err != nil {
			// Not a parseable token — write verbatim.
			b.WriteString(match)
			last = loc[1]
			continue
		}
		tok.Namespace = namespace
		resolved, err := vault.Detokenize(tok, principal, purpose)
		if err != nil {
			// Unknown token, access denied, or any vault error — write verbatim.
			b.WriteString(match)
			last = loc[1]
			continue
		}
		b.WriteString(resolved)
		last = loc[1]
	}
	b.WriteString(text[last:])
	return b.String()
}

// inboundClient wraps an LLMClient and applies DetokenizeText to every
// text-bearing field of Complete responses. Stream is passed through unchanged
// because a token can span chunk boundaries; per-chunk detokenization would
// corrupt partial tokens. Streaming detokenization is out of scope for v1 —
// the primary consumer (deterministic query planning) uses Complete.
type inboundClient struct {
	inner     llmclient.LLMClient
	vault     tokenization.Vault
	namespace string
	principal tokenization.Principal
	purpose   string
}

var _ llmclient.LLMClient = (*inboundClient)(nil)

// WithInboundDetokenize wraps inner so that every Complete response has its
// text-bearing fields detokenized before being returned to the caller. It is
// the inbound counterpart of WithEgress: where WithEgress tokenizes sensitive
// values on the way out, WithInboundDetokenize restores them on the way back.
//
// Detokenization is best-effort: unresolvable tokens are left verbatim; the
// call never fails due to detokenization. Only Complete responses are touched;
// Stream is forwarded unchanged — see the inboundClient doc for rationale.
//
// inner and vault must not be nil; namespace must not be empty. A nil seam or
// empty namespace is a wiring bug, not a runtime condition — WithInboundDetokenize
// panics to surface it at construction time.
func WithInboundDetokenize(inner llmclient.LLMClient, vault tokenization.Vault, namespace string, principal tokenization.Principal, purpose string) llmclient.LLMClient {
	if inner == nil {
		panic("llmegress: WithInboundDetokenize: inner must not be nil")
	}
	if vault == nil {
		panic("llmegress: WithInboundDetokenize: vault must not be nil")
	}
	if namespace == "" {
		panic("llmegress: WithInboundDetokenize: namespace must not be empty")
	}
	return &inboundClient{
		inner:     inner,
		vault:     vault,
		namespace: namespace,
		principal: principal,
		purpose:   purpose,
	}
}

// Complete delegates to the inner client and, on success, detokenizes the
// response's text-bearing fields (Response.Text and each ContentBlock.Text).
// An error from the inner client is returned as-is; detokenization failure
// never converts a successful response into an error.
//
// Fields detokenized:
//   - Response.Text — the concatenated text convenience field.
//   - ContentBlock.Text — text blocks in the assistant response (BlockText).
//
// Fields NOT detokenized (not assistant text output):
//   - ContentBlock.Content — tool_result output; that is inbound user data,
//     not model output, and was not tokenized by GateFromVault on the way out.
//   - ContentBlock.Input, ContentBlock.Raw — structured JSON; token strings
//     inside JSON would be corrupted by blind string replacement.
//   - Identifiers (ID, Name, ToolUseID), StopReason, StopSequence, Usage —
//     not text content.
func (c *inboundClient) Complete(ctx context.Context, req llmclient.Request) (llmclient.Response, error) {
	resp, err := c.inner.Complete(ctx, req)
	if err != nil {
		return resp, err
	}

	resp.Text = DetokenizeText(c.vault, c.namespace, c.principal, c.purpose, resp.Text)

	for i := range resp.Content {
		if resp.Content[i].Text != "" {
			resp.Content[i].Text = DetokenizeText(c.vault, c.namespace, c.principal, c.purpose, resp.Content[i].Text)
		}
	}

	return resp, nil
}

// Stream forwards the inner client's stream channel unchanged. Per-chunk
// detokenization is not safe because a single token string (e.g.
// "tok_pii_3f…") can be split across consecutive TextDelta chunks; naively
// scanning each chunk independently would leave partial token fragments in the
// output. Full streaming detokenization requires buffering and boundary
// detection — out of scope for v1. Callers that need detokenized streaming
// output should collect the full text from the channel and call DetokenizeText
// on the assembled string.
func (c *inboundClient) Stream(ctx context.Context, req llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	return c.inner.Stream(ctx, req)
}
