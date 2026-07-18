// Package structured composes llmclient's OutputSchema seam into a typed
// strict-JSON completion: set the schema, call the client, and classify the
// two silent failure modes — truncation and malformed output — as
// errors.Is-matchable sentinels, so callers implement policy (skip the
// section, abort the run, retry bigger) instead of string-parsing symptoms.
//
// Motivating failure (fathom, 2026-07): an obligations compile hit
// StopMaxTokens mid-JSON and surfaced as "invalid character 'd' after object
// key:value pair" — a parse error that was really a budget error, aborting an
// 88-section corpus for one truncated section.
//
// Scope: text-JSON contract only. A StopToolUse response is out of scope. The
// open StopReason set is respected — only StopMaxTokens is classified; every
// other reason flows to decode.
//
// Concurrency: Complete is a pure composition over the caller's client and
// carries no state; it is as safe for concurrent use as the client it wraps.
package structured

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	llmclient "github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

// ErrTruncated reports that generation stopped at the max-token budget: the
// text is a prefix, not a malformed document. Retry with a larger budget or
// skip the unit — do not "fix" the JSON.
var ErrTruncated = errors.New("structured: response truncated at max_tokens")

// ErrMalformed reports that a complete (non-truncated) response failed the
// strict decode into the target — unknown fields included, so schema drift
// between prompt and target type surfaces here rather than passing silently.
var ErrMalformed = errors.New("structured: response does not decode into the target")

// Complete sets req.OutputSchema to schema, runs the request on c, and
// strictly decodes the response text into out (a non-nil pointer).
// Classification contract:
//
//   - StopReason == StopMaxTokens        → the Response and ErrTruncated
//   - complete but undecodable/unknown   → the Response and ErrMalformed
//     fields
//   - otherwise                          → the Response and nil
//
// The Response is returned in every case so callers keep usage/cost
// accounting even for failed units. A schema already present on req is an
// error — the caller's intent would be ambiguous.
func Complete(ctx context.Context, c llmclient.LLMClient, req llmclient.Request, schema json.RawMessage, out any) (llmclient.Response, error) {
	if c == nil {
		return llmclient.Response{}, errors.New("structured: client must not be nil")
	}
	if len(schema) == 0 {
		return llmclient.Response{}, errors.New("structured: schema must not be empty")
	}
	if out == nil {
		return llmclient.Response{}, errors.New("structured: out must not be nil")
	}
	if len(req.OutputSchema) != 0 {
		return llmclient.Response{}, errors.New("structured: req.OutputSchema already set — pass the schema through Complete's schema argument only")
	}
	req.OutputSchema = schema

	resp, err := c.Complete(ctx, req)
	if err != nil {
		return resp, err
	}
	if resp.StopReason == llmclient.StopMaxTokens {
		return resp, fmt.Errorf("structured: %d byte prefix: %w", len(resp.Text), ErrTruncated)
	}

	dec := json.NewDecoder(bytes.NewReader([]byte(resp.Text)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return resp, fmt.Errorf("structured: strict decode: %v: %w", err, ErrMalformed)
	}
	return resp, nil
}
