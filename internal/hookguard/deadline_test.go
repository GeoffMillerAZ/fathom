package hookguard

import (
	"context"
	"testing"
	"time"
)

func TestHookDeadlineBudget(t *testing.T) {
	start := time.Now()
	for _, tc := range []struct {
		timeoutSec int
		want       time.Duration
	}{
		{3, 2 * time.Second},        // Codex SessionEnd 3 s → 2 s
		{5, 4 * time.Second},        // Claude Code 5 s hooks → 4 s
		{10, 8 * time.Second},       // 10 s hooks → 8 s (20% > 1 s)
		{15, 12 * time.Second},      // doctor --brief
		{60, 48 * time.Second},      // 20% dominates
		{1, 500 * time.Millisecond}, // degenerate: never zero or negative
		{0, 0},                      // no descriptor: no deadline
	} {
		d := HookDeadline(start, tc.timeoutSec)
		if tc.want == 0 {
			if !d.IsZero() {
				t.Errorf("timeout %ds: want no deadline, got %v", tc.timeoutSec, d.Budget())
			}
			continue
		}
		if got := d.Budget(); got != tc.want {
			t.Errorf("timeout %ds: budget %v, want %v", tc.timeoutSec, got, tc.want)
		}
		if !d.At().Equal(start.Add(tc.want)) {
			t.Errorf("timeout %ds: at %v, want start+%v", tc.timeoutSec, d.At(), tc.want)
		}
	}
}

func TestHookDeadlineContextAndHeader(t *testing.T) {
	d := HookDeadline(time.Now(), 5)
	ctx, cancel := d.Context(context.Background())
	defer cancel()
	dl, ok := ctx.Deadline()
	if !ok || !dl.Equal(d.At()) {
		t.Fatalf("ctx deadline %v ok=%v, want %v", dl, ok, d.At())
	}
	ms := d.HeaderValue()
	if ms == "" || ms == "0" {
		t.Fatalf("header value %q, want positive ms", ms)
	}
	// A spent deadline says 0 and reports spent.
	spent := HookDeadline(time.Now().Add(-10*time.Second), 5)
	if !spent.Spent() || spent.HeaderValue() != "0" {
		t.Fatalf("spent deadline: spent=%v header=%q", spent.Spent(), spent.HeaderValue())
	}
}
