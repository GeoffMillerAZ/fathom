package llmreplay_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	llmtest "github.com/geoffmilleraz/agentic-kit/foundations/llmclient/llm-test"
	llmreplay "github.com/geoffmilleraz/agentic-kit/foundations/llmclient/replay"
)

// kindSplit routes Complete and Stream to two separate ReplayClients. The
// conformance harness does not reveal which method a check will call, and a
// cassette entry replays only through its recorded kind, so each scripted
// behaviour is recorded through BOTH methods and replay is routed by method —
// exactly how a downstream test records a cassette for the method it uses.
type kindSplit struct {
	complete *llmreplay.ReplayClient
	stream   *llmreplay.ReplayClient
}

func (s *kindSplit) Complete(ctx context.Context, req llmclient.Request) (llmclient.Response, error) {
	return s.complete.Complete(ctx, req)
}

func (s *kindSplit) Stream(ctx context.Context, req llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	return s.stream.Stream(ctx, req)
}

// newReplayFromMock records the scripted MockClient behaviour through both
// Complete and Stream into an in-memory cassette, then returns a replay-backed
// client over that cassette. This is the record→replay fidelity bridge the
// conformance run exercises.
func newReplayFromMock(t *testing.T, entry func() llmclient.MockEntry) llmclient.LLMClient {
	t.Helper()

	mock := llmclient.NewMockClient(entry(), entry())
	var buf bytes.Buffer
	rec := llmreplay.NewRecordingClient(mock, &buf)

	// Complete probe. A scripted failure (e.g. a stream-error entry) is a
	// legitimate recording — the error itself lands in the cassette.
	_, _ = rec.Complete(context.Background(), llmclient.Request{Prompt: "record"})

	// Stream probe, drained fully so the stream entry is appended.
	ch, err := rec.Stream(context.Background(), llmclient.Request{Prompt: "record"})
	if err != nil {
		t.Fatalf("recording Stream: %v", err)
	}
	for range ch {
	}

	entries, err := llmreplay.Load(&buf)
	if err != nil {
		t.Fatalf("Load recorded cassette: %v", err)
	}
	var completes, streams []llmreplay.Entry
	for _, e := range entries {
		switch e.Kind {
		case llmreplay.KindComplete:
			completes = append(completes, e)
		case llmreplay.KindStream:
			streams = append(streams, e)
		default:
			t.Fatalf("unexpected entry kind %q in recorded cassette", e.Kind)
		}
	}
	return &kindSplit{
		complete: llmreplay.NewReplayClient(completes),
		stream:   llmreplay.NewReplayClient(streams),
	}
}

// TestReplayClientConformance proves record→replay fidelity end to end: every
// behaviour is recorded from MockClient into a cassette, and the replayed
// cassette passes the full v2 conformance suite — including tool use and
// mid-stream errors — exactly as the live client would.
func TestReplayClientConformance(t *testing.T) {
	llmtest.RunConformance(t, llmtest.Harness{
		NewText: func(t *testing.T, text string) llmclient.LLMClient {
			return newReplayFromMock(t, func() llmclient.MockEntry {
				return llmclient.MockEntryOK(text)
			})
		},
		NewToolUse: func(t *testing.T, id, name string, input json.RawMessage) llmclient.LLMClient {
			return newReplayFromMock(t, func() llmclient.MockEntry {
				return llmclient.MockEntryToolUse(id, name, input)
			})
		},
		NewStreamError: func(t *testing.T, partial string, err error) llmclient.LLMClient {
			return newReplayFromMock(t, func() llmclient.MockEntry {
				return llmclient.MockEntryStreamErr(partial, err)
			})
		},
	}, llmtest.Capabilities{ToolUse: true, MidStreamError: true})
}
