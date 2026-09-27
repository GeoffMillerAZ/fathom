package main

import (
	"context"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// TestTenancyRefusalsReadTheGate (RT-52 with RT-53 integrated): once the
// admission gate exists the tenancy block's refusals line counts it —
// refused and would-refuse, with the mode — and says "not counted" only
// when this process has no gate (a CLI, or the ring).
func TestTenancyRefusalsReadTheGate(t *testing.T) {
	if got := tenancyRefusals(nil); !strings.Contains(got, "not counted") {
		t.Fatalf("no gate: %q", got)
	}
	b, _ := tenancy.ResolveBudget(tenancy.Config{}, tenancy.HostFacts{RAM: 16 * tenancy.GiB, NumCPU: 8, Managed: true})
	g := tenancy.NewGate(tenancy.GateConfig{Budget: b, Mode: tenancy.ModeObserve, InFlight: tenancy.NewInFlight(nil),
		Level: func() tenancy.Level { return tenancy.LevelNormal }})
	var held []*tenancy.Ticket
	for range b.Slots[tenancy.ClassFleet] + 2 {
		tk, _ := g.Admit(context.Background(), tenancy.Request{Tool: "list_services", Class: tenancy.ClassFleet, Origin: tenancy.OriginHook})
		held = append(held, tk)
	}
	for _, tk := range held {
		if tk != nil {
			tk.Release(tenancy.OutcomeOK)
		}
	}
	got := tenancyRefusals(g)
	if !strings.Contains(got, "refused 0") || !strings.Contains(got, "would refuse 2") || !strings.Contains(got, "observe") {
		t.Fatalf("gate counts not read: %q", got)
	}
}
