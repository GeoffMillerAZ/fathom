package tenancy

import (
	"strings"
	"testing"
	"time"
)

// RT-29 (§2, §3.1): the budget resolves from RAM and config —
// clamp(RAM/8, 1–4 GiB) — and every mark, slot count, per-call ceiling and
// deadline is derived from it, each derivation named in a note.
func TestResolveBudget_PerRAMClass(t *testing.T) {
	cases := []struct {
		ram                 int64
		cpus                int
		budget, exit        int64
		point, index, tree  int
		treeCeil, fleetCeil int64
	}{
		// 4 GiB RAM: RAM/8 = 512 MiB, clamped up to 1 GiB; RAM/4 = 1 GiB
		// caps the exit mark at the refuse mark.
		{4 * GiB, 4, 1 * GiB, 1 * GiB, 16, 2, 1, 64 * MiB, 128 * MiB},
		// 8 GiB: 1 GiB budget, exit min(2 GiB, 2 GiB).
		{8 * GiB, 8, 1 * GiB, 2 * GiB, 16, 2, 1, 64 * MiB, 128 * MiB},
		// 16 GiB: 2 GiB budget, exit 4 GiB.
		{16 * GiB, 10, 2 * GiB, 4 * GiB, 32, 4, 1, 128 * MiB, 256 * MiB},
		// 32 GiB (the incident machine): 4 GiB budget, exit 8 GiB, §3.1's table.
		{32 * GiB, 12, 4 * GiB, 8 * GiB, 64, 8, 2, 256 * MiB, 512 * MiB},
		// 128 GiB: clamped down to 4 GiB; exit 8 GiB (RAM/4 = 32 GiB).
		{128 * GiB, 24, 4 * GiB, 8 * GiB, 64, 8, 2, 256 * MiB, 512 * MiB},
	}
	var notes []Note
	for _, tc := range cases {
		var b Budget
		b, notes = ResolveBudget(Config{}, HostFacts{RAM: tc.ram, NumCPU: tc.cpus, Managed: true})
		if b.Budget != tc.budget {
			t.Errorf("RAM %d GiB: budget %d, want %d", tc.ram/GiB, b.Budget, tc.budget)
		}
		if b.Shed != tc.budget*3/4 || b.Refuse != tc.budget {
			t.Errorf("RAM %d GiB: shed %d refuse %d, want 0.75× and 1.0× of %d", tc.ram/GiB, b.Shed, b.Refuse, tc.budget)
		}
		if b.Exit != tc.exit {
			t.Errorf("RAM %d GiB: exit %d, want %d", tc.ram/GiB, b.Exit, tc.exit)
		}
		if b.GCHint != b.Exit || b.GCHintOff {
			t.Errorf("RAM %d GiB: the GC hint defaults to the exit mark, got %d (off %v)", tc.ram/GiB, b.GCHint, b.GCHintOff)
		}
		if b.Caches != tc.budget/4 {
			t.Errorf("RAM %d GiB: caches %d, want budget/4", tc.ram/GiB, b.Caches)
		}
		if b.Slots[ClassPoint] != tc.point || b.Slots[ClassIndex] != tc.index || b.Slots[ClassTree] != tc.tree ||
			b.Slots[ClassFleet] != 1 || b.Slots[ClassIngest] != 1 || b.Slots[ClassEgress] != 2 {
			t.Errorf("RAM %d GiB: slots %v", tc.ram/GiB, b.Slots)
		}
		if b.PerCall[ClassPoint] != 8*MiB || b.PerCall[ClassIndex] != 64*MiB ||
			b.PerCall[ClassTree] != tc.treeCeil || b.PerCall[ClassFleet] != tc.fleetCeil {
			t.Errorf("RAM %d GiB: per-call %v", tc.ram/GiB, b.PerCall)
		}
		if b.Deadline[ClassPoint] != 10*time.Second || b.Deadline[ClassIndex] != 30*time.Second ||
			b.Deadline[ClassTree] != 60*time.Second || b.Deadline[ClassFleet] != 120*time.Second {
			t.Errorf("RAM %d GiB: deadlines %v", tc.ram/GiB, b.Deadline)
		}
		if !b.ExitEnabled {
			t.Errorf("exit is enabled by default")
		}
		// Every derivation is named.
		for _, key := range []string{"budget", "shed", "refuse", "exit", "gc_hint", "caches", "slots", "per_call", "deadline", "gomaxprocs"} {
			if n, ok := noteFor(notes, key); !ok || n.Derivation == "" {
				t.Errorf("RAM %d GiB: no named derivation for %q in %v", tc.ram/GiB, key, notes)
			}
		}
	}

	// The §3.1 envelope at 4 GiB: 64×8 + 8×64 + 2×256 + 1×512 MiB = 2 GiB.
	b, _ := ResolveBudget(Config{}, HostFacts{RAM: 32 * GiB, NumCPU: 12, Managed: true})
	env := int64(0)
	for _, c := range []Class{ClassPoint, ClassIndex, ClassTree, ClassFleet} {
		env += int64(b.Slots[c]) * b.PerCall[c]
	}
	if env != 2*GiB {
		t.Fatalf("the admitted envelope at a 4 GiB budget is %d, want 2 GiB", env)
	}

	// Config overrides: an explicit budget, marks and gc hint.
	b, notes = ResolveBudget(Config{Budget: 3 * GiB, ShedAt: 0.5, RefuseAt: 0.9, ExitAt: 1.5, GCHint: "budget", GOMAXPROCS: 6},
		HostFacts{RAM: 64 * GiB, NumCPU: 16, Managed: true})
	if b.Budget != 3*GiB || b.Shed != 3*GiB/2 || b.Refuse != int64(float64(3*GiB)*refuse09) || b.Exit != int64(float64(3*GiB)*exit15) {
		t.Fatalf("configured marks: %+v", b)
	}
	if b.GCHint != b.Budget || b.GOMAXPROCS != 6 {
		t.Fatalf("gc_hint=budget and gomaxprocs=6: %d %d", b.GCHint, b.GOMAXPROCS)
	}
	if n, _ := noteFor(notes, "budget"); !strings.Contains(n.Derivation, "configured") {
		t.Fatalf("a configured budget is named as configured: %q", n.Derivation)
	}
	// An IEC gc hint and "off".
	b, _ = ResolveBudget(Config{GCHint: "6GiB"}, HostFacts{RAM: 32 * GiB, NumCPU: 8, Managed: true})
	if b.GCHint != 6*GiB {
		t.Fatalf("gc_hint 6GiB: %d", b.GCHint)
	}
	b, _ = ResolveBudget(Config{GCHint: "off"}, HostFacts{RAM: 32 * GiB, NumCPU: 8, Managed: true})
	if !b.GCHintOff {
		t.Fatalf("gc_hint off")
	}
	// exit_enabled = false keeps shed/refuse only, disclosed.
	b, notes = ResolveBudget(Config{ExitDisabled: true}, HostFacts{RAM: 32 * GiB, NumCPU: 8, Managed: true})
	if b.ExitEnabled {
		t.Fatalf("exit_enabled=false")
	}
	if n, _ := noteFor(notes, "exit"); !strings.Contains(n.Derivation, "disabled") {
		t.Fatalf("a disabled exit rung is disclosed: %q", n.Derivation)
	}
	// GOMAXPROCS auto: max(2, NumCPU/2).
	for cpus, want := range map[int]int{1: 2, 2: 2, 4: 2, 12: 6} {
		b, _ = ResolveBudget(Config{}, HostFacts{RAM: 32 * GiB, NumCPU: cpus, Managed: true})
		if b.GOMAXPROCS != want {
			t.Errorf("NumCPU %d: GOMAXPROCS %d, want %d", cpus, b.GOMAXPROCS, want)
		}
	}
	// Unknown RAM falls back to the 1 GiB floor, named.
	b, notes = ResolveBudget(Config{}, HostFacts{NumCPU: 8, Managed: true})
	if b.Budget != 1*GiB {
		t.Fatalf("unknown RAM: budget %d", b.Budget)
	}
	if n, _ := noteFor(notes, "budget"); !strings.Contains(n.Derivation, "RAM unknown") {
		t.Fatalf("unknown RAM is named: %q", n.Derivation)
	}
}

// RT-177 (§10.1): an unmanaged serve-http resolves its own
// [tenancy.unmanaged] budget (512 MiB by default) through the same ladder,
// so a leaked scratch daemon cannot grow to the managed size.
func TestResolveBudget_UnmanagedClass(t *testing.T) {
	b, notes := ResolveBudget(Config{Budget: 4 * GiB}, HostFacts{RAM: 32 * GiB, NumCPU: 12, Managed: false})
	if b.Managed {
		t.Fatalf("unmanaged budget marked managed")
	}
	if b.Budget != 512*MiB {
		t.Fatalf("unmanaged budget %d, want 512 MiB (the managed [tenancy].budget does not apply)", b.Budget)
	}
	if b.Exit != 1*GiB || b.Refuse != 512*MiB || b.Shed != 384*MiB {
		t.Fatalf("unmanaged marks %+v", b)
	}
	if !b.ExitEnabled {
		t.Fatalf("the unmanaged exit rung is on: a leaked scratch daemon must stop growing")
	}
	if n, _ := noteFor(notes, "budget"); !strings.Contains(n.Derivation, "unmanaged") {
		t.Fatalf("unmanaged budget derivation named: %q", n.Derivation)
	}
	if b.GOMAXPROCS != 0 {
		t.Fatalf("GOMAXPROCS is set for managed daemons only, got %d", b.GOMAXPROCS)
	}
	// [tenancy.unmanaged].budget configures it.
	b, _ = ResolveBudget(Config{Unmanaged: UnmanagedConfig{Budget: 256 * MiB}}, HostFacts{RAM: 32 * GiB, NumCPU: 12})
	if b.Budget != 256*MiB {
		t.Fatalf("configured unmanaged budget %d", b.Budget)
	}
	// The env override (FATHOM_TENANCY_BUDGET) is the process's own and
	// wins for either kind.
	b, _ = ResolveBudget(Config{EnvBudget: 2 * GiB}, HostFacts{RAM: 32 * GiB, NumCPU: 12})
	if b.Budget != 2*GiB {
		t.Fatalf("env budget %d", b.Budget)
	}
	// Slots derive from the smaller budget: point 16 × 0.5 = 8, tree 1.
	b, _ = ResolveBudget(Config{}, HostFacts{RAM: 32 * GiB, NumCPU: 12})
	if b.Slots[ClassPoint] != 8 || b.Slots[ClassTree] != 1 || b.PerCall[ClassTree] != 32*MiB {
		t.Fatalf("unmanaged slots %v per-call %v", b.Slots, b.PerCall)
	}
}

var refuse09, exit15 = 0.9, 1.5

func noteFor(notes []Note, key string) (Note, bool) {
	for _, n := range notes {
		if n.Key == key {
			return n, true
		}
	}
	return Note{}, false
}
