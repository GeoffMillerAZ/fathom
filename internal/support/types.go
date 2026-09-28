package support

import (
	"encoding/json"
	"fmt"
)

// types.go mirrors schema/support/schema.cue's closed vocabularies and row
// shapes (design §2/§3). CUE validates authored JSON against these same
// names; this file is the Go-side decode, kept honest by schema_test.go's
// negatives and by the embedded schema drift gate.

// Kind is the closed list of supported-thing kinds (#Kind).
type Kind string

const (
	KindLanguage  Kind = "language"
	KindTool      Kind = "tool"
	KindClient    Kind = "client"
	KindSpec      Kind = "spec"
	KindRules     Kind = "rules"
	KindCoverage  Kind = "coverage"
	KindContract  Kind = "contract"
	KindEcosystem Kind = "ecosystem"
)

// Level is a support level (#Level): L1-L4 for languages, L1-L3 for tool
// formats (L4 is n/a for a tool).
type Level string

const (
	LevelNone Level = "" // no reading has computed a level
	L1        Level = "L1"
	L2        Level = "L2"
	L3        Level = "L3"
	L4        Level = "L4"
)

// levelRank orders Level for comparison; LevelNone ranks below every real
// level so an absent reading never outranks one below-target.
func levelRank(l Level) int {
	switch l {
	case L1:
		return 1
	case L2:
		return 2
	case L3:
		return 3
	case L4:
		return 4
	default:
		return 0
	}
}

// Grade is what a reading computed for a met level or capability (#Grade).
type Grade string

const (
	GradeNone      Grade = "" // no reading
	GradeExcellent Grade = "excellent"
	GradeGood      Grade = "good"
)

// State is the closed computed-state vocabulary (#State, design §3). Every
// value is derived from registry, targets, readings, plan verdicts and (for
// Regressed) ledger history — never a status a person sets.
type State string

const (
	StateProvenExcellent State = "proven-excellent"
	StateProvenGood      State = "proven-good"
	StateBelowTarget     State = "below-target"
	StateTolerated       State = "tolerated"
	StateDeclared        State = "declared"
	StateRegressed       State = "regressed"
	StateRefused         State = "refused"
	StateDeferred        State = "deferred"
	StateNA              State = "n/a"
)

// Verdict is a cell's evidence verdict (#Cell.verdict): the owning plan
// row's status, generalized to the four values the schema allows — a
// refused/deferred disposition is recorded here as "dispositioned"; the
// finer distinction lives in State.
type Verdict string

const (
	VerdictNone            Verdict = ""
	VerdictProven          Verdict = "proven"
	VerdictClaimedUnproven Verdict = "claimed-unproven"
	VerdictOpen            Verdict = "open"
	VerdictDispositioned   Verdict = "dispositioned"
)

// CapabilityTarget is one capability's declared target (#Target.target
// values a capability maps to: a Level, or true meaning "must be proven at
// all, no specific level").
type CapabilityTarget struct {
	Level      Level // set when the target names a specific level
	ProvenOnly bool  // the target value was literal `true`
}

// Target is one row of docs/plans/support.targets.json (#Target): what is
// declared, never what is achieved. Languages and tool formats keep their
// own target list (the language queue, LB-F22) and never appear here.
type Target struct {
	Kind Kind   `json:"kind"`
	ID   string `json:"id"`
	Name string `json:"name"`
	// Targets is the decoded form of the JSON "target" object: a
	// capability name to the Level it must reach, or to ProvenOnly when
	// the source named it `true` (must be proven at all, no level).
	Targets map[string]CapabilityTarget `json:"-"`
	PlanRow string                      `json:"plan_row"`
	Wave    int                         `json:"wave"`
	Notes   string                      `json:"notes,omitempty"`
}

// targetJSON is #Target's on-disk shape (json.Unmarshal's staging type);
// Target.UnmarshalJSON decodes its loose `target` object into Targets.
type targetJSON struct {
	Kind    Kind           `json:"kind"`
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Target  map[string]any `json:"target"`
	PlanRow string         `json:"plan_row"`
	Wave    int            `json:"wave"`
	Notes   string         `json:"notes,omitempty"`
}

// UnmarshalJSON decodes one #Target row, converting its loose `target`
// object (each value a Level string or the literal `true`) into Targets.
func (t *Target) UnmarshalJSON(data []byte) error {
	var raw targetJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	t.Kind, t.ID, t.Name = raw.Kind, raw.ID, raw.Name
	t.PlanRow, t.Wave, t.Notes = raw.PlanRow, raw.Wave, raw.Notes
	t.Targets = make(map[string]CapabilityTarget, len(raw.Target))
	for name, v := range raw.Target {
		ct, err := decodeCapabilityTarget(v)
		if err != nil {
			return fmt.Errorf("support: target %s/%s: capability %q: %w", t.Kind, t.ID, name, err)
		}
		t.Targets[name] = ct
	}
	return nil
}

// decodeCapabilityTarget converts one `target` object value (a Level
// string, or the literal boolean true) into a CapabilityTarget.
func decodeCapabilityTarget(v any) (CapabilityTarget, error) {
	switch vv := v.(type) {
	case bool:
		if !vv {
			return CapabilityTarget{}, fmt.Errorf("target value false is not allowed (want true or a level)")
		}
		return CapabilityTarget{ProvenOnly: true}, nil
	case string:
		switch lvl := Level(vv); lvl {
		case L1, L2, L3, L4:
			return CapabilityTarget{Level: lvl}, nil
		}
		return CapabilityTarget{}, fmt.Errorf("invalid level %q", vv)
	default:
		return CapabilityTarget{}, fmt.Errorf("unsupported target value %v (%T)", v, v)
	}
}

// Reading is a computed support reading for a non-language kind (#Reading).
type Reading struct {
	Kind        Kind               `json:"kind"`
	ID          string             `json:"id"`
	ReadingDate string             `json:"reading_date"`
	Commit      string             `json:"commit"`
	Instrument  string             `json:"instrument"`
	Metrics     map[string]float64 `json:"metrics"`
	Met         map[string]Grade   `json:"met"`
}

// LedgerRow is one line of docs/bench/readings/support-ledger.jsonl
// (#LedgerRow).
type LedgerRow struct {
	Kind        Kind   `json:"kind"`
	ID          string `json:"id"`
	Capability  string `json:"capability"`
	LevelMet    Level  `json:"level_met,omitempty"`
	Grade       Grade  `json:"grade,omitempty"`
	ReadingDate string `json:"reading_date"`
	Commit      string `json:"commit"`
}

// Cell is one (entry, capability) of the matrix (#Cell). Target's JSON form
// is #Level | true (schema/support/schema.cue): a Level string for a
// levelled capability, or the JSON boolean `true` for a ProvenOnly one —
// never the STRING "true", which fails #Cell validation (a CUE boolean
// literal unifies only against a JSON boolean). `any` is the seam that
// lets encoding/json emit either shape; the only two values ever stored
// here are a Level string or the literal Go bool `true` (setTarget).
type Cell struct {
	Capability string  `json:"capability"`
	State      State   `json:"state"`
	Target     any     `json:"target,omitempty"`
	LevelMet   Level   `json:"level_met,omitempty"`
	Reading    string  `json:"reading,omitempty"`
	PlanRow    string  `json:"plan_row,omitempty"`
	Verdict    Verdict `json:"verdict,omitempty"`
}

// Entry is one row of the matrix (#Entry).
type Entry struct {
	Kind       Kind   `json:"kind"`
	ID         string `json:"id"`
	Name       string `json:"name"`
	Registered bool   `json:"registered"`
	Wave       int    `json:"wave,omitempty"`
	Engine     string `json:"engine,omitempty"`
	Cells      []Cell `json:"cells"`
}

// Matrix is the `fathom support --json` document (#Matrix, schema
// fathom.support/v1).
type Matrix struct {
	Schema      string  `json:"schema"`
	GeneratedAt string  `json:"generated_at"`
	Commit      string  `json:"commit"`
	Entries     []Entry `json:"entries"`
}

// RegistryEntry is one row a kind's adapter contributes: identity plus the
// capability columns the matrix projects over (design §2's "support.
// Registry exposes (kind, id, capabilities)"). languageRegistry and
// clientRegistry both produce these; a later plan's spec/rules/coverage/
// contract/ecosystem adapter does the same without this package ever
// importing their engines.
type RegistryEntry struct {
	Kind         Kind
	ID           string
	Name         string
	Wave         int
	Engine       string
	Capabilities []string
}
