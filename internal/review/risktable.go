package review

import (
	"fmt"
	"os"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// RiskTable holds the two configurable thresholds the band computation
// compares against (schema/risk/schema.cue's #RiskTable, CX-3). Every other
// factor's weight is a fixed mapping the design doc names directly and is
// not configurable (band.go's own doc comment).
type RiskTable struct {
	// BlastRadiusHighCallers is the transitive-caller count strictly above
	// which the blast-radius factor cites "high". Zero means "unset — use
	// the binary default" (blastRadiusHighCallers()).
	BlastRadiusHighCallers int
	// ZoneInterestMediumRate is the zone-interest rate strictly above
	// which the zone-interest factor cites "medium". Zero means "unset —
	// use the binary default" (zoneInterestMediumRate()).
	ZoneInterestMediumRate float64
}

// defaultBlastRadiusHighCallers and defaultZoneInterestMediumRate are the
// binary's own built-in thresholds (design doc §2: "more than 50
// transitive callers", "above 2.0") — DefaultRiskTable() and a config file
// that omits a field both resolve to these.
const (
	defaultBlastRadiusHighCallers = 50
	defaultZoneInterestMediumRate = 2.0
)

func (t RiskTable) blastRadiusHighCallers() int {
	if t.BlastRadiusHighCallers > 0 {
		return t.BlastRadiusHighCallers
	}
	return defaultBlastRadiusHighCallers
}

func (t RiskTable) zoneInterestMediumRate() float64 {
	if t.ZoneInterestMediumRate > 0 {
		return t.ZoneInterestMediumRate
	}
	return defaultZoneInterestMediumRate
}

// DefaultRiskTable is the built-in table BuildRiskReceipt's caller passes
// when no `.fathom/risk.cue` is present — a repository with no risk table
// gets the design doc's own published numbers with zero configuration
// (same dual-mode convention as internal/testlayer.DefaultLayers).
func DefaultRiskTable() RiskTable {
	return RiskTable{
		BlastRadiusHighCallers: defaultBlastRadiusHighCallers,
		ZoneInterestMediumRate: defaultZoneInterestMediumRate,
	}
}

// LoadRiskTable reads and decodes the risk table config at path — the
// dual-mode entry point: a missing file (os.IsNotExist) returns
// DefaultRiskTable() with no error. A present file overrides only the
// fields it names; an absent field keeps the binary default (RiskTable's
// own accessor methods), so a partially-specified file is a legitimate,
// working config (mirrors internal/testlayer.LoadLayers's own dual-mode
// discipline, though unlike that loader an empty risk.cue is
// indistinguishable from "no overrides" by design — both fields are
// optional numeric thresholds with a defined default, so there is no
// meaningfully-empty-but-explicit form to preserve here).
func LoadRiskTable(path string) (RiskTable, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultRiskTable(), nil
		}
		return RiskTable{}, fmt.Errorf("review: read %s: %w", path, err)
	}

	cctx := cuecontext.New()
	v := cctx.CompileBytes(data, cue.Filename(path))
	if v.Err() != nil {
		return RiskTable{}, fmt.Errorf("review: parse %s: %w", path, v.Err())
	}

	table := RiskTable{}
	if f := v.LookupPath(cue.ParsePath("blast_radius_high_callers")); f.Exists() {
		n, err := f.Int64()
		if err != nil {
			return RiskTable{}, fmt.Errorf("review: %s: field %q: %w", path, "blast_radius_high_callers", err)
		}
		table.BlastRadiusHighCallers = int(n)
	}
	if f := v.LookupPath(cue.ParsePath("zone_interest_medium_rate")); f.Exists() {
		n, err := f.Float64()
		if err != nil {
			return RiskTable{}, fmt.Errorf("review: %s: field %q: %w", path, "zone_interest_medium_rate", err)
		}
		table.ZoneInterestMediumRate = n
	}
	return table, nil
}
