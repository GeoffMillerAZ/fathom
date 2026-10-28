package benchprep

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/errors"
	"github.com/geoffmilleraz/fathom/schema/bench"
)

// Prereg is the machine-evaluable half of a signed preregistration, decoded
// from docs/bench/prereg/<id>.json after the file unifies with #Prereg
// (schema/bench/schema.cue). One JSON per prereg id, which every run under
// it names with `scenariobench run -prereg`; no per-run files (D1, BX-62).
type Prereg struct {
	ID                     string             `json:"id"`
	Strata                 []string           `json:"strata"`
	Arms                   []string           `json:"arms"`
	BaselineArm            string             `json:"baseline_arm"`
	Hypotheses             []PreregHypothesis `json:"hypotheses"`
	Alpha                  float64            `json:"alpha"`
	MultiplicityCorrection PreregCorrection   `json:"multiplicity_correction"`
	ThreeOutcomeRule       PreregOutcomeRule  `json:"three_outcome_rule"`
	TurnCap                int                `json:"turn_cap"`
	CostCapUSD             float64            `json:"cost_cap_usd"`
	Seed                   int                `json:"seed"`
	Exclusions             []string           `json:"exclusions"`
	DeviationLog           []PreregDeviation  `json:"deviation_log"`
}

// PreregHypothesis is one #Hypothesis.
type PreregHypothesis struct {
	ID                      string             `json:"id"`
	Statement               string             `json:"statement"`
	Predictions             []PreregPrediction `json:"predictions"`
	PrimaryMetric           string             `json:"primary_metric"`
	MinimumEffectOfInterest float64            `json:"minimum_effect_of_interest"`
	N                       int                `json:"n"`
	Test                    string             `json:"test"`
}

// PreregPrediction is one #Prediction; EquivalenceMargin is set for a
// predicted null.
type PreregPrediction struct {
	Stratum           string  `json:"stratum"`
	Arm               string  `json:"arm"`
	Direction         string  `json:"direction"`
	EquivalenceMargin float64 `json:"equivalence_margin,omitempty"`
}

// PreregCorrection is the prereg's multiplicity correction.
type PreregCorrection struct {
	Method string   `json:"method"`
	Family []string `json:"family"`
}

// PreregOutcomeRule is the prereg's three-outcome rule.
type PreregOutcomeRule struct {
	Confirmed     string         `json:"confirmed"`
	Refuted       string         `json:"refuted"`
	Inconclusive  string         `json:"inconclusive"`
	Tie           string         `json:"tie"`
	PredictedNull PreregNullRule `json:"predicted_null"`
}

// PreregNullRule is the TOST reading for a predicted null.
type PreregNullRule struct {
	Equivalent   string `json:"equivalent"`
	Harm         string `json:"harm"`
	Inconclusive string `json:"inconclusive"`
	AboveDelta   string `json:"above_delta"`
}

// PreregDeviation is one deviation_log entry.
type PreregDeviation struct {
	Date        string `json:"date"`
	Description string `json:"description"`
	Reason      string `json:"reason"`
}

// LoadPrereg reads the prereg JSON at path, unifies it with #Prereg through
// the CUE Go API, decodes it into a Prereg and returns it with the sha256 of
// the file's bytes, the provenance every cell and ledger row of a run under
// it carries (BX-62). It refuses, naming the cause, a file that is not JSON,
// one #Prereg rejects (a closed definition, so an unknown field too), and one
// not named <id>.json for its own id (D1). The schema check runs before the
// name check, so a schema negative is always refused by the schema.
func LoadPrereg(path string) (Prereg, string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Prereg{}, "", fmt.Errorf("reading prereg: %w", err)
	}
	sum := sha256.Sum256(b)
	sha := hex.EncodeToString(sum[:])
	if !json.Valid(b) {
		return Prereg{}, "", fmt.Errorf("prereg %s is not JSON", path)
	}
	ctx := cuecontext.New()
	def := ctx.CompileString(bench.Source).LookupPath(cue.ParsePath("#Prereg"))
	if err := def.Err(); err != nil {
		return Prereg{}, "", fmt.Errorf("compiling #Prereg: %w", err)
	}
	data := ctx.CompileBytes(b, cue.Filename(filepath.Base(path)))
	if err := def.Unify(data).Validate(cue.Concrete(true)); err != nil {
		return Prereg{}, "", fmt.Errorf("prereg %s does not satisfy #Prereg: %s", path, errors.Details(err, nil))
	}
	var p Prereg
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return Prereg{}, "", fmt.Errorf("decoding prereg %s: %w", path, err)
	}
	if want := p.ID + ".json"; filepath.Base(path) != want {
		return Prereg{}, "", fmt.Errorf("prereg %s is not named for its id %q: want %s (D1: one JSON per prereg id)", path, p.ID, want)
	}
	return p, sha, nil
}
