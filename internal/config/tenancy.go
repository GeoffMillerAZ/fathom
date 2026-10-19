package config

// tenancy.go — RT-30 (docs/design/responsible-tenant.md §1): the [tenancy]
// table is a machine contract. It is read from the user layer and env
// only; a repository's [tenancy] validates (a copied block is not a load
// failure) but is ignored and named. Precedence: env > user > defaults.
// [daemon].memory_limit / FATHOM_MEMORY_LIMIT migrate to the GC hint.

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// recordTenancyProvenance records "tenancy.<key>" (nested tables dotted)
// for every key the user layer sets.
func recordTenancyProvenance(prov Provenance, m map[string]any, prefix, path string) {
	for k, v := range m {
		if sub, ok := v.(map[string]any); ok {
			recordTenancyProvenance(prov, sub, prefix+"."+k, path)
			continue
		}
		prov[prefix+"."+k] = SourceInfo{SourceUser, path}
	}
}

// TenancyNoteKind types a resolution note, so a reader (doctor's
// tenancy_scope) judges by kind, never by matching the note's words.
type TenancyNoteKind string

const (
	NoteProjectIgnored TenancyNoteKind = "project_ignored" // a repository's [tenancy] was ignored
	NoteEnvOverride    TenancyNoteKind = "env_override"    // an env variable overrides a key
	NoteMigration      TenancyNoteKind = "migration"       // [daemon].memory_limit is the GC hint
	NoteConflict       TenancyNoteKind = "conflict"        // memory_limit and gc_hint both set
	NoteAdmissionOff   TenancyNoteKind = "admission_off"   // admission is off (the escape hatch, §1)
)

// TenancyNote is one typed resolution note.
type TenancyNote struct {
	Kind TenancyNoteKind
	Text string
}

// ResolveTenancyConfig resolves [tenancy] from the loaded user layer and
// env (getenv; nil = os.Getenv). notes name every non-default source: the
// ignored repository table, the memory_limit migration, an env override.
// An env value that does not parse is an error naming the variable, never
// a silent default.
func ResolveTenancyConfig(l Loaded, getenv func(string) string) (tenancy.Config, []string, error) {
	cfg, typed, err := ResolveTenancyConfigNotes(l, getenv)
	var notes []string
	for _, n := range typed {
		notes = append(notes, n.Text)
	}
	return cfg, notes, err
}

// ResolveTenancyConfigNotes is ResolveTenancyConfig with typed notes.
func ResolveTenancyConfigNotes(l Loaded, getenv func(string) string) (tenancy.Config, []TenancyNote, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	var cfg tenancy.Config
	var notes []TenancyNote
	note := func(k TenancyNoteKind, text string) { notes = append(notes, TenancyNote{k, text}) }
	t := l.userTenancy
	var errs []string
	size := func(m map[string]any, key string, dst *int64) {
		s, ok := stringField(m, key)
		if !ok || s == "auto" {
			return
		}
		n, err := tenancy.ParseSize(s)
		if err != nil {
			errs = append(errs, fmt.Sprintf("[tenancy] %s: %v", key, err))
			return
		}
		*dst = n
	}
	dur := func(m map[string]any, key string, dst *time.Duration) {
		s, ok := stringField(m, key)
		if !ok {
			return
		}
		d, err := time.ParseDuration(s)
		if err != nil {
			errs = append(errs, fmt.Sprintf("[tenancy] %s: %v", key, err))
			return
		}
		*dst = d
	}
	num := func(m map[string]any, key string, dst *float64) {
		if v, ok := numberField(m, key); ok {
			*dst = v
		}
	}
	integer := func(m map[string]any, key string, dst *int) {
		if v, ok := m[key].(int64); ok {
			*dst = int(v)
		}
	}
	str := func(m map[string]any, key string, dst *string) {
		if v, ok := stringField(m, key); ok {
			*dst = v
		}
	}
	sub := func(key string) map[string]any {
		m, _ := t[key].(map[string]any)
		return m
	}

	size(t, "budget", &cfg.Budget)
	num(t, "shed_at", &cfg.ShedAt)
	num(t, "refuse_at", &cfg.RefuseAt)
	num(t, "exit_at", &cfg.ExitAt)
	if v, ok := t["exit_enabled"].(bool); ok {
		cfg.ExitDisabled = !v
	}
	str(t, "gc_hint", &cfg.GCHint)
	str(t, "admission", &cfg.Admission)
	integer(t, "gomaxprocs", &cfg.GOMAXPROCS) // "auto" stays 0
	integer(t, "child_parallelism", &cfg.ChildParallelism)
	if v, ok := t["notify"].(bool); ok {
		cfg.NotifyOff = !v
	}
	h := sub("host")
	str(h, "shed_on_pressure", &cfg.Host.ShedOnPressure)
	str(h, "refuse_on_pressure", &cfg.Host.RefuseOnPressure)
	size(h, "refuse_on_swap_growth", &cfg.Host.RefuseOnSwapGrowth)
	dur(h, "swap_growth_window", &cfg.Host.SwapGrowthWindow)
	size(h, "min_swap_volume_free", &cfg.Host.MinSwapVolumeFree)
	size(h, "min_store_volume_free", &cfg.Host.MinStoreVolumeFree)
	// allowance: 0 is a value here (the old rule), not "the default", so it
	// maps to AllowanceOff and the zero field keeps meaning the default.
	if v, ok := numberField(h, "allowance"); ok {
		cfg.Host.Allowance = v
		if v <= 0 {
			cfg.Host.Allowance = tenancy.AllowanceOff
		}
	}
	br := sub("breaker")
	integer(br, "safe_after", &cfg.Breaker.SafeAfter)
	dur(br, "safe_window", &cfg.Breaker.SafeWindow)
	integer(br, "stop_after", &cfg.Breaker.StopAfter)
	dur(br, "stop_window", &cfg.Breaker.StopWindow)
	dur(br, "warmup", &cfg.Breaker.Warmup)
	u := sub("unmanaged")
	dur(u, "idle_exit", &cfg.Unmanaged.IdleExit)
	dur(u, "ttl", &cfg.Unmanaged.TTL)
	size(u, "budget", &cfg.Unmanaged.Budget)
	c := sub("caches")
	num(c, "fraction", &cfg.Caches.Fraction)
	num(c, "shed_trim", &cfg.Caches.ShedTrim)
	dur(c, "idle_drop", &cfg.Caches.IdleDrop)

	if l.ProjectTenancyIgnored {
		note(NoteProjectIgnored, fmt.Sprintf("[tenancy] in %s ignored: budgets are read at user scope only (~/.fathom/config.toml); a repository cannot raise this machine's ceiling", l.ProjectPath))
	}

	if v := strings.TrimSpace(getenv("FATHOM_TENANCY_BUDGET")); v != "" {
		n, err := tenancy.ParseSize(v)
		if err != nil {
			errs = append(errs, fmt.Sprintf("FATHOM_TENANCY_BUDGET: %v", err))
		} else {
			cfg.EnvBudget = n
			note(NoteEnvOverride, "FATHOM_TENANCY_BUDGET = "+v+" overrides the configured budget")
		}
	}
	// Migration (§1): the old soft limit keeps its meaning — the GC hint.
	oldKey := l.Config.Daemon.MemoryLimit
	oldSrc := "[daemon].memory_limit"
	if v := getenv("FATHOM_MEMORY_LIMIT"); v != "" {
		oldKey, oldSrc = v, "FATHOM_MEMORY_LIMIT"
	}
	if err := tenancy.CheckMarks(cfg); err != nil {
		errs = append(errs, err.Error())
	}
	// A malformed hint is an error naming its source, as the old soft
	// limit's was (it exited 1), never a silent fallback to the exit mark.
	if h := strings.TrimSpace(cfg.GCHint); h != "" {
		switch {
		case strings.EqualFold(h, "exit"), strings.EqualFold(h, "budget"), strings.EqualFold(h, "off"):
		default:
			if _, err := tenancy.ParseSize(h); err != nil {
				errs = append(errs, fmt.Sprintf("[tenancy] gc_hint %q: want exit, budget, off or an IEC size (%v)", h, err))
			}
		}
	}
	if oldKey != "" {
		if _, _, err := ParseMemoryLimit(oldKey); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", oldSrc, err))
		}
	}
	if oldKey != "" {
		if cfg.GCHint == "" {
			cfg.GCHint = oldKey
			note(NoteMigration, fmt.Sprintf("%s = %s is the GC hint, superseded by [tenancy].gc_hint; the budget stays %s", oldSrc, oldKey, budgetWord(max(cfg.EnvBudget, cfg.Budget))))
		} else {
			note(NoteConflict, fmt.Sprintf("conflict: %s = %s and [tenancy].gc_hint = %s are both set — [tenancy] wins; remove memory_limit", oldSrc, oldKey, cfg.GCHint))
		}
	}

	if v := strings.TrimSpace(getenv("FATHOM_TENANCY_ADMISSION")); v != "" {
		switch v {
		case "observe", "enforce", "off":
			cfg.Admission = v
			note(NoteEnvOverride, "FATHOM_TENANCY_ADMISSION = "+v)
		default:
			errs = append(errs, fmt.Sprintf("FATHOM_TENANCY_ADMISSION %q: want observe, enforce or off", v))
		}
	}
	if v := strings.TrimSpace(getenv("FATHOM_TENANCY_EXIT")); v != "" {
		if strings.EqualFold(v, "off") {
			cfg.ExitDisabled = true
			note(NoteEnvOverride, "FATHOM_TENANCY_EXIT = off: the exit rung is disabled (shed and refuse only)")
		} else {
			errs = append(errs, fmt.Sprintf("FATHOM_TENANCY_EXIT %q: the only value is off", v))
		}
	}
	if strings.EqualFold(strings.TrimSpace(cfg.Admission), "off") {
		note(NoteAdmissionOff, "admission is off: no call is admitted by cost class (FATHOM_TENANCY_ADMISSION / [tenancy].admission is the escape hatch, §1)")
	}
	if len(errs) > 0 {
		sort.Strings(errs)
		return cfg, notes, fmt.Errorf("tenancy: %s", strings.Join(errs, "; "))
	}
	return cfg, notes, nil
}

func budgetWord(b int64) string {
	if b == 0 {
		return "auto"
	}
	return tenancy.FormatBytes(b)
}
