package capability

import "sort"

// Seam names which of fathom's four extension points delivers a
// capability. The set is CLOSED: a proposed capability that fits none of
// these is rejected until its author writes down, per seam, why that seam
// does not fit (docs/design/capability-registry.md §3). Widening a seam is
// nearly always cheaper than adding one.
type Seam string

const (
	SeamCore    Seam = "core"    // the index and its surfaces
	SeamModules Seam = "modules" // expert modules bound into a repo
	SeamLayers  Seam = "layers"  // semantic layer packs
	SeamGuards  Seam = "guards"  // pre-edit / pre-command guidance
	SeamProfile Seam = "profile" // which tools a surface exposes
)

// ScopeRule says which configuration scopes may set a capability. Law 13:
// user scope may set a DEFAULT for repositories that have not spoken; only
// project scope may GOVERN. A capability whose whole purpose is governance
// (a gate, a guard) is project-only, so a user-central preference can
// never quietly switch off a control a repository relies on.
type ScopeRule string

const (
	ScopeAny         ScopeRule = "any"          // user may default, project governs
	ScopeProjectOnly ScopeRule = "project-only" // only the repository may decide
)

// State is a capability's observed condition. It is deliberately not a
// bool: an off capability that cannot say why is the defect this package
// exists to remove.
type State struct {
	// Operating is true only when the capability is observably working —
	// not when configuration says it should be.
	Operating bool
	// Reason is REQUIRED when Operating is false: one clause, lowercase,
	// naming the observation that makes this off.
	Reason string
	// Detail is optional evidence when Operating is true ("6 rules
	// compiled"). It is what turns a green tick into something a reader
	// can check.
	Detail string
	// Broken separates the two ways a capability can be off, which look
	// identical from a config file and could not be more different to act
	// on:
	//
	//   ABSENT  — its inputs do not exist here. No module is bound, no
	//             layer pack installed, no rules authored. Normal, common,
	//             and usually a deliberate choice. Never a failure.
	//   BROKEN  — its inputs DO exist and something is failing to deliver
	//             them. Rules compiled but no hook fires. An index that is
	//             stale. This is a defect and doctor may fail on it.
	//
	// Collapsing the two is how a diagnostic tool teaches people to ignore
	// it: nine "you could also enable this" failures and the one real
	// break is invisible among them.
	Broken bool
}

// Remedy is what to do about an off capability: either an exact command,
// or an explicit note saying why there is nothing to run. One of the two
// is always present — "off, and no explanation" is the failure mode.
type Remedy struct {
	Cmd  string
	Note string
}

// Probe is the set of already-gathered facts a Detect function may read.
// It is a plain value so detection is pure: the caller owns every
// filesystem read, HTTP call and store query, and the registry stays
// testable in a table.
//
// A zero Probe means "a repository fathom has never touched", and every
// capability must degrade to off-with-a-reason against it — that is what
// TestRegistryEveryEntryExplainsItsAbsence asserts, and it is the O23
// regression (doctor's repo-relative assumptions) made structurally
// impossible.
type Probe struct {
	RepoRoot string
	Service  string

	// Index freshness is THREE states, not two, and the difference
	// decides whether doctor may fail: known-fresh, known-behind (a
	// defect), and unknown (nobody could tell — an absence of
	// information, never evidence of a problem).
	IndexedSymbols int
	IndexFresh     bool
	IndexStale     bool
	IndexNote      string // the freshness verdict verbatim, when not fresh

	// Intent corpus (ADR-026). Three states, like index freshness: a router
	// present, decision records only, or neither — never "off" for a repo
	// that simply has not adopted the practice.
	IntentRouterPresent bool
	IntentDecisionsHome string
	IntentConditions    int

	// Wiring
	HooksReferenced  bool
	HooksNote        string
	StanzaPresent    bool
	VerifyInLefthook bool

	// Seams
	GuardRules     int
	GuardProblem   string // invalid or unreadable input; never an absence
	ModuleBindings int
	LayerPacks     []string
	// ObligationsCompiled is whether the obligations layer has a recorded
	// compile for this service. Authored-but-uncompiled and
	// compiled-but-undelivered are DIFFERENT states with different
	// remedies, so both facts travel rather than one merged boolean.
	ObligationRules     int
	ObligationsCompiled bool

	// Instruments
	NarrateActive     bool
	LedgerSessions    int
	AgentSurfaceTools int
	AgentSchemaBytes  int
}

// Capability is one registered, toggleable thing.
type Capability struct {
	// ID is stable and never renamed — it appears in user config files.
	ID string
	// Seam is which extension point delivers it.
	Seam Seam
	// Summary is one line written for someone who has never used fathom.
	Summary string
	// Default is what happens with no configuration at all.
	Default bool
	// Scopes says who may set it (law 13).
	Scopes ScopeRule
	// Cost states what it costs when on, in the units a reader cares
	// about — "nothing", "one hook round-trip per edit (~15 ms local)",
	// "11,839 bytes of tool schema per session".
	Cost string
	// Switchable is whether a configuration setting actually changes
	// behaviour — that is, whether some enforcement site reads the
	// resolution and acts on it.
	//
	// This field exists because the first cut of this package shipped
	// without it, and the omission produced precisely the defect the
	// package was built to prevent: `capabilities set guards off` wrote a
	// file, the table then reported "switched off by this repository", and
	// guards kept firing on every edit. A config read was being rendered
	// as a behavioural fact (law 14).
	//
	// A capability with Switchable false is STATUS, not a switch: it
	// reports what is happening and `set` refuses it, naming what to run
	// instead. Setting Switchable true obliges you to add an enforcement
	// site AND a case to TestSwitchableCapabilitiesAreEnforced, which
	// proves that flipping the setting changes observed behaviour.
	Switchable bool
	// NotSwitchableBecause is required when Switchable is false: what this
	// entry reports, and what the real control is.
	NotSwitchableBecause string
	// Detect is pure over the probe.
	Detect func(Probe) State
	// Remedy is consulted only when Detect reports off.
	Remedy func(Probe) Remedy
}

// Registry is the set of registered capabilities, keyed by ID.
type Registry struct {
	byID map[string]Capability
}

// All returns every capability sorted by ID — deterministic, per the house
// contract that outputs are sorted.
func (r Registry) All() []Capability {
	out := make([]Capability, 0, len(r.byID))
	for _, c := range r.byID {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Get returns the capability with the given ID.
func (r Registry) Get(id string) (Capability, bool) {
	c, ok := r.byID[id]
	return c, ok
}

// IDs returns every registered ID, sorted.
func (r Registry) IDs() []string {
	out := make([]string, 0, len(r.byID))
	for id := range r.byID {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// Report is one capability resolved against a probe and a configuration:
// what it is, whether it is switched on, whether it is actually operating,
// and what to do about the gap.
type Report struct {
	Capability Capability
	// Enabled is the resolved configuration value, and Scope is which
	// scope supplied it. "Guards: off (your user default)" and "Guards:
	// off (this repo's .fathom/capabilities.cue)" are different facts and
	// a reader debugging their setup must be able to tell them apart.
	Enabled bool
	Scope   Scope
	// State is the observation. A capability can be Enabled and not
	// Operating — that combination is the interesting one and it is
	// exactly what a config-only check would miss.
	State  State
	Remedy Remedy
}

// DetectAll resolves configuration and runs detection for every registered
// capability, returning one Report each, sorted by ID.
//
// A capability that is switched OFF by configuration is never probed: its
// State reports off with the scope that disabled it as the reason, so a
// reader is never told "no guard rules compiled" about a capability they
// deliberately turned off.
func (r Registry) DetectAll(p Probe, cfg Resolution) []Report {
	caps := r.All()
	out := make([]Report, 0, len(caps))
	for _, c := range caps {
		setting := cfg.For(c)
		rep := Report{Capability: c, Enabled: setting.On, Scope: setting.Scope}
		switch {
		case !setting.On && c.Switchable:
			rep.State = State{Reason: "switched off by " + setting.Scope.describe()}
			rep.Remedy = Remedy{Cmd: "fathom capabilities set " + c.ID + " on"}
		default:
			rep.State = c.Detect(p)
			if !rep.State.Operating && c.Remedy != nil {
				rep.Remedy = c.Remedy(p)
			}
		}
		out = append(out, rep)
	}
	return out
}
