package tenancy

// The wave-B live reading's F5: a call queued for a slot when the
// ladder reaches refuse is re-checked when it wins the slot and refused by
// name (a 503 with Retry-After), never admitted into work the rung refuses.

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestQueuedCallRecheckedOnDequeue(t *testing.T) {
	var level atomic.Uint32
	g := NewGate(GateConfig{Budget: testBudget(t), Mode: ModeEnforce, InFlight: NewInFlight(nil),
		Level: func() Level { return Level(level.Load()) }})
	var held []*Ticket
	for range g.Budget().Slots[ClassTree] {
		tk, ref := g.Admit(context.Background(), Request{Tool: "search", Class: ClassTree, Origin: OriginAgent})
		if ref != nil {
			t.Fatalf("hold: %+v", ref)
		}
		held = append(held, tk)
	}
	type res struct {
		tk  *Ticket
		ref *Refusal
	}
	out := make(chan res, 1)
	go func() {
		tk, ref := g.Admit(context.Background(), Request{Tool: "symbol_context", Class: ClassTree, Origin: OriginAgent})
		out <- res{tk, ref}
	}()
	for g.Queued(ClassTree) == 0 {
		time.Sleep(time.Millisecond)
	}
	level.Store(uint32(LevelRefuse))
	for _, tk := range held {
		tk.Release(OutcomeOK)
	}
	got := <-out
	if got.ref == nil || got.ref.Code != RefusalLevel {
		if got.tk != nil {
			got.tk.Release(OutcomeOK)
		}
		t.Fatalf("a call dequeued after the ladder reached refuse got ticket=%v refusal=%+v, want the level refusal", got.tk != nil, got.ref)
	}
	level.Store(uint32(LevelNormal))
	tk, ref := g.Admit(context.Background(), Request{Tool: "search", Class: ClassTree, Origin: OriginAgent})
	if ref != nil {
		t.Fatalf("the refused waiter leaked its slot: %+v", ref)
	}
	tk.Release(OutcomeOK)
}
