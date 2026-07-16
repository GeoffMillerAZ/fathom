package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestWithOrigin_RoundTripAndLogEntry(t *testing.T) {
	ctx := WithOrigin(context.Background(), "edit-hook")
	if got := originFrom(ctx); got != "edit-hook" {
		t.Fatalf("originFrom = %q, want edit-hook", got)
	}
	if got := originFrom(context.Background()); got != "" {
		t.Fatalf("zero ctx origin = %q, want empty", got)
	}
	// Empty origin must not allocate a value.
	if ctx2 := WithOrigin(context.Background(), ""); originFrom(ctx2) != "" {
		t.Fatal("empty origin must stay empty")
	}

	// The logger records origin in the JSONL entry (GI-6).
	var buf bytes.Buffer
	l := NewCallLogger(&buf)
	l.log("obligations", "fathom", "shared", "companion", "", "edit-hook", false, 42*time.Millisecond, nil)
	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("entry not JSON: %v", err)
	}
	if entry["origin"] != "edit-hook" {
		t.Errorf("origin = %v, want edit-hook (line: %s)", entry["origin"], buf.String())
	}
	// Agent-choice calls omit the field entirely.
	buf.Reset()
	l.log("find_symbol", "fathom", "shared", "", "", "", false, time.Millisecond, nil)
	if strings.Contains(buf.String(), "origin") {
		t.Errorf("empty origin must be omitted: %s", buf.String())
	}
}
