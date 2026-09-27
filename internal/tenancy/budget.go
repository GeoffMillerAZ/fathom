package tenancy

// budget.go — the budget (design §1, §2, §3.1; RT-29, RT-177). One number,
// phys_footprint, from which every mark, slot count, per-call ceiling and
// deadline is derived; each derivation is named in a Note so `resources`,
// doctor and the startup line can say where a number came from (P22).

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// IEC byte units.
const (
	KiB int64 = 1 << 10
	MiB int64 = 1 << 20
	GiB int64 = 1 << 30
)

// Class is a cost class (§3.1). The admission gate, the producer governor
// and the in-flight registry share it.
type Class uint8

const (
	ClassPoint Class = iota
	ClassIndex
	ClassTree
	ClassFleet
	ClassIngest
	ClassEgress
	numClasses
)

// Classes lists every class in order.
var Classes = []Class{ClassPoint, ClassIndex, ClassTree, ClassFleet, ClassIngest, ClassEgress}

func (c Class) String() string {
	switch c {
	case ClassPoint:
		return "point"
	case ClassIndex:
		return "index"
	case ClassTree:
		return "tree"
	case ClassFleet:
		return "fleet"
	case ClassIngest:
		return "ingest"
	case ClassEgress:
		return "egress"
	}
	return "class(" + strconv.Itoa(int(c)) + ")"
}

// Config is the resolved [tenancy] table (§1): user scope and env only
// (internal/config.ResolveTenancyConfig). A zero field means "the default".
type Config struct {
	Budget    int64 // [tenancy].budget; 0 = auto
	EnvBudget int64 // FATHOM_TENANCY_BUDGET; wins for managed and unmanaged alike

	ShedAt, RefuseAt, ExitAt float64 // × budget; 0 = 0.75, 1.00, 2.00
	ExitDisabled             bool    // exit_enabled = false (or FATHOM_TENANCY_EXIT=off)
	GCHint                   string  // "exit" | "budget" | "off" | an IEC size; "" = exit
	Admission                string  // "observe" | "enforce" | "off"; "" = observe
	GOMAXPROCS               int     // 0 = auto: max(2, NumCPU/2)
	ChildParallelism         int     // 0 = 2
	NotifyOff                bool

	Host      HostConfig
	Breaker   BreakerSettings
	Unmanaged UnmanagedConfig
	Caches    CacheConfig
}

// HostConfig is [tenancy.host]. Pressure names: "warn", "critical" or
// "off"; "" = the default.
type HostConfig struct {
	ShedOnPressure     string        // "" = warn
	RefuseOnPressure   string        // "" = critical
	RefuseOnSwapGrowth int64         // 0 = 2 GiB
	SwapGrowthWindow   time.Duration // 0 = 5 min
	MinSwapVolumeFree  int64         // 0 = 5 GiB
	MinStoreVolumeFree int64         // 0 = 2 GiB
}

// BreakerSettings is [tenancy.breaker] (§2.3); zero fields take the
// defaults (3 in 30 min → safe mode, 5 in 60 min → stop, warm-up 120 s).
type BreakerSettings struct {
	SafeAfter  int
	SafeWindow time.Duration
	StopAfter  int
	StopWindow time.Duration
	Warmup     time.Duration
}

// UnmanagedConfig is [tenancy.unmanaged] (§10.1).
type UnmanagedConfig struct {
	IdleExit time.Duration
	TTL      time.Duration
	Budget   int64 // 0 = 512 MiB
}

// CacheConfig is [tenancy.caches] (§5).
type CacheConfig struct {
	Fraction float64       // 0 = 0.25 of the budget
	ShedTrim float64       // 0 = 0.50 of the cache budget
	IdleDrop time.Duration // 0 = 30 min
}

// HostFacts are the machine facts the budget derives from.
type HostFacts struct {
	RAM     int64 // hw.memsize; 0 = unknown
	NumCPU  int
	Managed bool // §10.1: the operator's supervised daemon
}

// HostTriggers are the resolved host-side rung triggers (§2.1). A zero
// pressure level never triggers.
type HostTriggers struct {
	ShedOnPressure     PressureLevel
	RefuseOnPressure   PressureLevel
	SwapGrowth         int64
	SwapGrowthWindow   time.Duration
	MinSwapVolumeFree  int64
	MinStoreVolumeFree int64
}

// Budget is the resolved budget: marks in bytes of phys_footprint and the
// derived admission shape.
type Budget struct {
	Budget, Shed, Refuse, Exit int64
	GCHint                     int64 // SetMemoryLimit target; meaningless when GCHintOff
	GCHintOff                  bool
	Caches                     int64
	// CacheShedTrim is the fraction of Caches the memo registry trims to
	// at shed; CacheIdleDrop the idle time after which it drops (§5).
	CacheShedTrim float64
	CacheIdleDrop time.Duration
	ExitEnabled   bool
	Managed       bool

	Slots    map[Class]int
	PerCall  map[Class]int64 // byte ceiling per ticket; 0 = streamed (ingest)
	Deadline map[Class]time.Duration

	Host             HostTriggers
	GOMAXPROCS       int // 0 = leave the runtime's (unmanaged daemons)
	ChildParallelism int
	Admission        string
	Breaker          BreakerSettings
}

// Note names one derivation: which key, its value, and where it came from.
type Note struct {
	Key, Value, Derivation string
}

// Defaults (§1).
const (
	defaultShedAt            = 0.75
	defaultRefuseAt          = 1.00
	defaultExitAt            = 2.00
	defaultUnmanagedBudget   = 512 * MiB
	defaultSwapGrowth        = 2 * GiB
	defaultSwapGrowthWindow  = 5 * time.Minute
	defaultMinSwapVolumeFree = 5 * GiB
	defaultMinStoreFree      = 2 * GiB
	defaultCacheFraction     = 0.25
	defaultCacheShedTrim     = 0.50
	defaultCacheIdleDrop     = 30 * time.Minute
	defaultChildParallelism  = 2
	budgetFloor              = 1 * GiB
	budgetCeiling            = 4 * GiB
)

// ResolveBudget derives the budget from config and host facts. Pure; the
// notes name every derivation, sorted by key.
func ResolveBudget(cfg Config, host HostFacts) (Budget, []Note) {
	var notes []Note
	note := func(key, value, how string) { notes = append(notes, Note{Key: key, Value: value, Derivation: how}) }

	b := Budget{Managed: host.Managed}
	switch {
	case cfg.EnvBudget > 0:
		b.Budget = cfg.EnvBudget
		note("budget", FormatBytes(b.Budget), "FATHOM_TENANCY_BUDGET")
	case !host.Managed:
		b.Budget = cfg.Unmanaged.Budget
		how := "[tenancy.unmanaged].budget (configured) — an unmanaged serve-http never takes the managed budget"
		if b.Budget <= 0 {
			b.Budget = defaultUnmanagedBudget
			how = "unmanaged default 512 MiB ([tenancy.unmanaged].budget) — an unmanaged serve-http never takes the managed budget"
		}
		note("budget", FormatBytes(b.Budget), how)
	case cfg.Budget > 0:
		b.Budget = cfg.Budget
		note("budget", FormatBytes(b.Budget), "[tenancy].budget (configured)")
	case host.RAM <= 0:
		b.Budget = budgetFloor
		note("budget", FormatBytes(b.Budget), "auto: RAM unknown, the 1 GiB floor")
	default:
		b.Budget = clamp(host.RAM/8, budgetFloor, budgetCeiling)
		note("budget", FormatBytes(b.Budget), fmt.Sprintf("auto: clamp(RAM %s / 8, 1 GiB, 4 GiB)", FormatBytes(host.RAM)))
	}

	shedAt, refuseAt, exitAt := orF(cfg.ShedAt, defaultShedAt), orF(cfg.RefuseAt, defaultRefuseAt), orF(cfg.ExitAt, defaultExitAt)
	b.Shed = int64(float64(b.Budget) * shedAt)
	note("shed", FormatBytes(b.Shed), fmt.Sprintf("%.2f × budget", shedAt))
	b.Refuse = int64(float64(b.Budget) * refuseAt)
	note("refuse", FormatBytes(b.Refuse), fmt.Sprintf("%.2f × budget", refuseAt))
	b.Exit = int64(float64(b.Budget) * exitAt)
	exitHow := fmt.Sprintf("%.2f × budget", exitAt)
	if host.RAM > 0 && b.Exit > host.RAM/4 {
		b.Exit = host.RAM / 4
		exitHow += fmt.Sprintf(", capped at RAM/4 (%s)", FormatBytes(host.RAM/4))
	}
	if b.Exit < b.Refuse {
		b.Exit = b.Refuse
		exitHow += ", raised to the refuse mark"
	}
	b.ExitEnabled = !cfg.ExitDisabled
	if !b.ExitEnabled {
		exitHow += "; exit rung disabled (exit_enabled = false): shed and refuse only, an over-budget daemon keeps running"
	}
	note("exit", FormatBytes(b.Exit), exitHow)

	switch h := strings.TrimSpace(cfg.GCHint); {
	case h == "" || strings.EqualFold(h, "exit"):
		b.GCHint = b.Exit
		note("gc_hint", FormatBytes(b.GCHint), "the exit mark (gc_hint = exit): the ladder bounds memory, the hint only keeps the heap from outrunning the exit mark")
	case strings.EqualFold(h, "budget"):
		b.GCHint = b.Budget
		note("gc_hint", FormatBytes(b.GCHint), "the budget (gc_hint = budget)")
	case strings.EqualFold(h, "off"):
		b.GCHintOff = true
		note("gc_hint", "off", "gc_hint = off")
	default:
		if n, err := ParseSize(h); err == nil {
			b.GCHint = n
			note("gc_hint", FormatBytes(n), "gc_hint (configured size)")
		} else {
			b.GCHint = b.Exit
			note("gc_hint", FormatBytes(b.GCHint), fmt.Sprintf("the exit mark: gc_hint %q is not exit|budget|off|an IEC size", h))
		}
	}

	frac := orF(cfg.Caches.Fraction, defaultCacheFraction)
	b.Caches = int64(float64(b.Budget) * frac)
	note("caches", FormatBytes(b.Caches), fmt.Sprintf("%.2f × budget", frac))
	b.CacheShedTrim = orF(cfg.Caches.ShedTrim, defaultCacheShedTrim)
	b.CacheIdleDrop = orD(cfg.Caches.IdleDrop, defaultCacheIdleDrop)

	// §3.1: slots and ceilings are derived, not configured.
	perGiB := float64(b.Budget) / float64(GiB)
	b.Slots = map[Class]int{
		ClassPoint:  max(1, int(16*perGiB)),
		ClassIndex:  max(2, min(host.NumCPU-2, int(b.Budget/(512*MiB)))),
		ClassTree:   max(1, int(b.Budget/(2*GiB))),
		ClassFleet:  1,
		ClassIngest: 1,
		ClassEgress: 2,
	}
	note("slots", formatSlots(b.Slots), "point 16×budget/GiB; index max(2, min(NumCPU−2, budget/512 MiB)); tree max(1, budget/2 GiB); fleet 1; ingest 1 machine-wide; egress 2")
	b.PerCall = map[Class]int64{
		ClassPoint:  8 * MiB,
		ClassIndex:  64 * MiB,
		ClassTree:   b.Budget / 16,
		ClassFleet:  b.Budget / 8,
		ClassIngest: 0,
		ClassEgress: 8 * MiB,
	}
	note("per_call", formatPerCall(b.PerCall), "point 8 MiB; index 64 MiB; tree budget/16; fleet budget/8; ingest streamed; egress 8 MiB")
	b.Deadline = map[Class]time.Duration{
		ClassPoint:  10 * time.Second,
		ClassIndex:  30 * time.Second,
		ClassTree:   60 * time.Second,
		ClassFleet:  120 * time.Second,
		ClassIngest: 0,
		ClassEgress: 0,
	}
	note("deadline", "point 10s; index 30s; tree 60s; fleet 120s", "server deadlines (§3.1); ingest detached; egress the port's own timeout")

	b.Host = HostTriggers{
		ShedOnPressure:     pressureTrigger(cfg.Host.ShedOnPressure, PressureWarn),
		RefuseOnPressure:   pressureTrigger(cfg.Host.RefuseOnPressure, PressureCritical),
		SwapGrowth:         orI(cfg.Host.RefuseOnSwapGrowth, defaultSwapGrowth),
		SwapGrowthWindow:   orD(cfg.Host.SwapGrowthWindow, defaultSwapGrowthWindow),
		MinSwapVolumeFree:  orI(cfg.Host.MinSwapVolumeFree, defaultMinSwapVolumeFree),
		MinStoreVolumeFree: orI(cfg.Host.MinStoreVolumeFree, defaultMinStoreFree),
	}
	note("host", fmt.Sprintf("shed on pressure %s; refuse on pressure %s, swap growth %s in %s, swap volume free < %s",
		b.Host.ShedOnPressure, b.Host.RefuseOnPressure, FormatBytes(b.Host.SwapGrowth), b.Host.SwapGrowthWindow, FormatBytes(b.Host.MinSwapVolumeFree)),
		"[tenancy.host]; the swap-used fraction is never a trigger")

	if host.Managed {
		if cfg.GOMAXPROCS > 0 {
			b.GOMAXPROCS = cfg.GOMAXPROCS
			note("gomaxprocs", strconv.Itoa(b.GOMAXPROCS), "[tenancy].gomaxprocs (configured)")
		} else {
			b.GOMAXPROCS = max(2, host.NumCPU/2)
			note("gomaxprocs", strconv.Itoa(b.GOMAXPROCS), fmt.Sprintf("auto: max(2, NumCPU %d / 2) for a managed daemon", host.NumCPU))
		}
	} else {
		note("gomaxprocs", "runtime default", "unmanaged daemons keep the runtime's GOMAXPROCS")
	}
	b.ChildParallelism = cfg.ChildParallelism
	if b.ChildParallelism <= 0 {
		b.ChildParallelism = defaultChildParallelism
	}
	b.Admission = cfg.Admission
	if b.Admission == "" {
		b.Admission = "observe"
	}
	b.Breaker = cfg.Breaker

	sort.SliceStable(notes, func(i, j int) bool { return notes[i].Key < notes[j].Key })
	return b, notes
}

func pressureTrigger(s string, def PressureLevel) PressureLevel {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "":
		return def
	case "warn":
		return PressureWarn
	case "critical":
		return PressureCritical
	}
	return PressureUnknown // "off": never triggers
}

func clamp(v, lo, hi int64) int64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// CheckMarks reports a ladder whose rungs are out of order: the effective
// refuse fraction (configured or the default) below the effective shed
// fraction. ResolveBudget itself never fails; config resolution calls this
// so a misordered ladder is a named error, not a silent inversion.
func CheckMarks(cfg Config) error {
	shedAt, refuseAt := orF(cfg.ShedAt, defaultShedAt), orF(cfg.RefuseAt, defaultRefuseAt)
	if refuseAt < shedAt {
		return fmt.Errorf("[tenancy] refuse_at %.2f is below shed_at %.2f: the rungs are ordered (shed ≤ refuse)", refuseAt, shedAt)
	}
	return nil
}

func orF(v, d float64) float64 {
	if v > 0 {
		return v
	}
	return d
}

func orI(v, d int64) int64 {
	if v > 0 {
		return v
	}
	return d
}

func orD(v, d time.Duration) time.Duration {
	if v > 0 {
		return v
	}
	return d
}

func formatSlots(m map[Class]int) string {
	parts := make([]string, 0, len(Classes))
	for _, c := range Classes {
		parts = append(parts, fmt.Sprintf("%s %d", c, m[c]))
	}
	return strings.Join(parts, ", ")
}

func formatPerCall(m map[Class]int64) string {
	parts := make([]string, 0, len(Classes))
	for _, c := range Classes {
		if m[c] == 0 {
			parts = append(parts, c.String()+" streamed")
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %s", c, FormatBytes(m[c])))
	}
	return strings.Join(parts, ", ")
}

var sizePattern = regexp.MustCompile(`^([0-9]+(?:\.[0-9]+)?)(B|KiB|MiB|GiB|TiB)$`)

// ParseSize parses an IEC size ("512MiB", "1.5GiB"); the same syntax as
// [daemon].memory_limit and GOMEMLIMIT.
func ParseSize(s string) (int64, error) {
	m := sizePattern.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, fmt.Errorf("size %q: want an IEC size such as 512MiB or 4GiB", s)
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, fmt.Errorf("size %q: %w", s, err)
	}
	mult := map[string]int64{"B": 1, "KiB": KiB, "MiB": MiB, "GiB": GiB, "TiB": 1 << 40}[m[2]]
	n := int64(v * float64(mult))
	if n <= 0 {
		return 0, fmt.Errorf("size %q: must be greater than zero", s)
	}
	return n, nil
}

// FormatBytes renders n in the largest IEC unit, one decimal when inexact.
func FormatBytes(n int64) string {
	switch {
	case n < 0:
		return "unknown"
	case n >= GiB:
		if n%GiB == 0 {
			return fmt.Sprintf("%d GiB", n/GiB)
		}
		return fmt.Sprintf("%.1f GiB", float64(n)/float64(GiB))
	case n >= MiB:
		if n%MiB == 0 {
			return fmt.Sprintf("%d MiB", n/MiB)
		}
		return fmt.Sprintf("%.1f MiB", float64(n)/float64(MiB))
	case n >= KiB:
		return fmt.Sprintf("%d KiB", n/KiB)
	}
	return fmt.Sprintf("%d B", n)
}
