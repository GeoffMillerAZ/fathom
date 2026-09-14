package indexbench

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// replay.go is the bench pyramid's L1 layer (ADR-032, docs/design/
// bench-pyramid.md §3.2): for each read-only tool, a committed list of
// (tool, args) against a pinned fixture store with golden bytes, result
// count and a latency budget.
//
// L0 asks whether the index is RIGHT. L1 asks what the right answer COSTS —
// the question the context bench spent $15 and three weeks failing to see,
// because a payload the agent re-reads at 28k tokens a turn is invisible
// under a cache-read baseline both arms pay. O23's `displaced: 65` would
// have been a red number here three weeks before a human noticed it.
//
// The entries go through mcp.Server.HandleRequest — the real JSON-RPC
// dispatch, the real handlers, the real envelope — not the query functions
// underneath. The whole point is to pin the bytes an AGENT sees, and the
// envelope, the elision filters and the output-schema wrapping are all part
// of that number. It stops short of HTTP only because a socket would make
// the daemon's availability part of a gate that must pass offline.

// Dispatcher is the JSON-RPC entry point a replay drives. *mcp.Server
// implements it.
type Dispatcher interface {
	HandleRequest(ctx context.Context, raw []byte) (resp []byte, hasResp bool)
}

// ReplayEntry is one replayed call: the authored half (everything up to
// BudgetMS) is hand-written in the committed list; the measured half is
// rewritten by ReplayGolden.
type ReplayEntry struct {
	// ID names the entry in a failure message.
	ID string `json:"id"`
	// Tool is the MCP tool name.
	Tool string `json:"tool"`
	// Args is the tools/call arguments object.
	Args map[string]any `json:"args"`
	// CountField is the result field whose length is ResultCount — authored
	// per entry rather than guessed, because "the biggest array in the
	// response" is a heuristic that silently starts counting a different
	// thing when a tool gains a field.
	CountField string `json:"count_field,omitempty"`
	// BudgetMS is the latency budget for this call. Zero means unbudgeted.
	BudgetMS int64 `json:"budget_ms,omitempty"`

	// Bytes is the size of the text block the agent reads — the canonical
	// copy of the result, before the JSON-RPC envelope.
	Bytes int `json:"bytes"`
	// ResultCount is len(result[CountField]).
	ResultCount int `json:"result_count"`
	// Displaced is the obligations payload's displaced count: served-eligible
	// claims the cap dropped. Any non-zero value is a failing test.
	Displaced int `json:"displaced"`
	// ElapsedMS is wall-clock time for the call. Like the oracle's timings it
	// is non-deterministic, so ZeroReplayTimings clears it before a golden is
	// written or compared; it is checked against BudgetMS instead.
	ElapsedMS int64 `json:"elapsed_ms,omitempty"`
}

// ZeroReplayTimings clears every non-deterministic field so entries can be
// written as, or compared against, a committed golden.
func ZeroReplayTimings(entries []ReplayEntry) {
	for i := range entries {
		entries[i].ElapsedMS = 0
	}
}

// ReplayGolden runs every entry through the dispatcher and returns the
// entries with their measured fields filled in (bench-pyramid BP-15). The
// authored fields are copied through untouched, so the result can be written
// straight back over the committed list.
func ReplayGolden(ctx context.Context, d Dispatcher, entries []ReplayEntry) ([]ReplayEntry, error) {
	out := make([]ReplayEntry, 0, len(entries))
	for i, e := range entries {
		req, err := json.Marshal(map[string]any{
			"jsonrpc": "2.0",
			"id":      i + 1,
			"method":  "tools/call",
			"params":  map[string]any{"name": e.Tool, "arguments": e.Args},
		})
		if err != nil {
			return nil, fmt.Errorf("indexbench: replay %s: %w", e.ID, err)
		}
		started := time.Now()
		raw, ok := d.HandleRequest(ctx, req)
		elapsed := time.Since(started)
		if !ok {
			return nil, fmt.Errorf("indexbench: replay %s: no response", e.ID)
		}
		text, err := resultText(raw)
		if err != nil {
			return nil, fmt.Errorf("indexbench: replay %s: %w", e.ID, err)
		}
		e.Bytes = len(text)
		e.ElapsedMS = elapsed.Milliseconds()
		e.ResultCount, e.Displaced = countAndDisplaced(text, e.CountField)
		out = append(out, e)
	}
	return out, nil
}

// resultText pulls the canonical text block out of a JSON-RPC response — the
// exact string an MCP client hands the model.
func resultText(raw []byte) (string, error) {
	var resp struct {
		Error  json.RawMessage `json:"error"`
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if len(resp.Error) > 0 {
		return "", fmt.Errorf("tool returned an error: %s", resp.Error)
	}
	for _, c := range resp.Result.Content {
		if c.Type == "text" {
			return c.Text, nil
		}
	}
	return "", fmt.Errorf("no text content in response")
}

// countAndDisplaced reads the counted field's length and the obligations
// payload's displaced count out of a result. A missing field counts zero —
// a tool that stops returning its primary array shows up as a count drop,
// which is what the gate is for.
func countAndDisplaced(text, countField string) (count, displaced int) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(text), &obj); err != nil {
		return 0, 0
	}
	if countField != "" {
		if raw, ok := obj[countField]; ok {
			var arr []json.RawMessage
			if err := json.Unmarshal(raw, &arr); err == nil {
				count = len(arr)
			}
		}
	}
	if raw, ok := obj["displaced"]; ok {
		_ = json.Unmarshal(raw, &displaced)
	}
	return count, displaced
}

// NewFixtureServer builds the pinned store L1 replays against: the fixture
// fleet ingested, the fixture semantic layer seeded, and a real mcp.Server
// over it.
func NewFixtureServer(ctx context.Context) (*mcp.Server, error) {
	c, st, err := NewFauxtownIndex(ctx)
	if err != nil {
		return nil, err
	}
	if err := SeedFixtureClaims(ctx, st); err != nil {
		return nil, err
	}
	return mcp.NewServer(c), nil
}

// ReplayRow is VB-7's anchor name (docs/plans/verbs.md) for one replayed
// entry — literally ReplayEntry, which predates the plan row's own naming.
// A type alias keeps the two names interchangeable rather than forking the
// shape a call log and every golden already commit to.
type ReplayRow = ReplayEntry

// PayloadReplay is VB-7's `task validate` entry point (ADR-035 decision 5):
// it reads the committed (service, tool, args) list at
// internal/indexbench/testdata/l1-replay.json, replays every entry against
// a fresh fixture server the way NewFixtureServer/ReplayGolden already do,
// and returns the measured rows. root is accepted for signature parity
// with IndexOracleGoldens — L1 needs no staged go/types module (the
// fixture server never loads one), so it is unused.
func PayloadReplay(root string) ([]ReplayRow, error) {
	_ = root
	ctx := context.Background()
	var entries []ReplayRow
	if err := readGolden(l1ReplayPath, &entries); err != nil {
		return nil, err
	}
	srv, err := NewFixtureServer(ctx)
	if err != nil {
		return nil, err
	}
	return ReplayGolden(ctx, srv, entries)
}
