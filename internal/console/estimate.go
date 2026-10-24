package console

import (
	"fmt"
	"math"
	"strings"
)

// RowWeightInput joins authored scope to the existing U6 difficulty profile.
// Files are explicit planned paths, never a filesystem scan. Needs is the
// number of rows this row depends on. No provider is consulted.
type RowWeightInput struct {
	ContractKind   string
	Files          []string
	Needs          int
	Profile        Profile
	AuthoredWeight int
}

// RowWeightEstimate keeps the author's value separate from advisory evidence.
// Zero AuthoredWeight means no authored value was supplied.
type RowWeightEstimate struct {
	AuthoredWeight  int    `json:"authored_weight"`
	EstimatedWeight int    `json:"estimated_weight"`
	Label           string `json:"label"`
}

// EstimateRowWeight implements docs/design/work-signals.md §2's five inputs.
// Raw weight = kind baseline * effort ratio + distinct planned files + needs
// + ceil(transitive callers / 10). Baselines: reach=3, symbol/certified/gone=2,
// other=1. The ratio is similar proven rows' median tokens / the service's
// median proven-row tokens (U6's service-scoped FleetMedianTokens). Both
// medians must be finite and positive; otherwise the ratio defaults to 1.
// Similarity must exclude the row's own weight (PreflightInput.ExcludeWeight),
// so accepting a value cannot change its estimate against the same evidence.
// Snap to the nearest of 1, 2, 3, 5, 8, 13, ties upward, capped at 13.
// Without index evidence, only the ratio and blast inputs drop out: authored
// files and needs still count. This is advisory, never measured effort, and
// AuthoredWeight never enters the arithmetic.
func EstimateRowWeight(in RowWeightInput) RowWeightEstimate {
	baseline := 1.0
	switch in.ContractKind {
	case ContractReach:
		baseline = 3
	case ContractSymbol, ContractCertified, ContractGone:
		baseline = 2
	}
	seen := make(map[string]struct{}, len(in.Files))
	for _, name := range in.Files {
		if name = strings.TrimSpace(name); name != "" {
			seen[name] = struct{}{}
		}
	}
	ratio := 1.0
	haveEffort := false
	if in.Profile.Similar.Known() {
		s := in.Profile.Similar.V
		positiveFinite := func(v float64) bool { return v > 0 && !math.IsInf(v, 0) && !math.IsNaN(v) }
		if positiveFinite(s.MedianTokens) && positiveFinite(s.FleetMedianTokens) {
			ratio, haveEffort = s.MedianTokens/s.FleetMedianTokens, true
		}
	}
	weight := baseline*ratio + float64(len(seen)) + float64(max(in.Needs, 0))
	if in.Profile.Blast.Known() {
		weight += math.Ceil(float64(max(in.Profile.Blast.V.Callers, 0)) / 10)
	}
	scale := [...]int{1, 2, 3, 5, 8, 13}
	estimated := scale[len(scale)-1]
	for i := 0; i < len(scale)-1; i++ {
		if weight < float64(scale[i]+scale[i+1])/2 {
			estimated = scale[i]
			break
		}
	}
	out := RowWeightEstimate{AuthoredWeight: in.AuthoredWeight, EstimatedWeight: estimated, Label: "estimate"}
	switch {
	case !in.Profile.Blast.Known():
		out.Label = "estimate (no index evidence)"
	case !in.Profile.Similar.Known():
		out.Label = "estimate (no similar-row effort)"
	case !haveEffort:
		out.Label = "estimate (no comparable service effort)"
	}
	if in.Profile.Blast.State == StatePartial {
		out.Label += " (blast radius is a lower bound)"
	}
	return out
}

// Summary places the advisory estimate beside the authored weight everywhere.
func (e RowWeightEstimate) Summary() string {
	authored := "absent"
	if e.AuthoredWeight > 0 {
		authored = fmt.Sprint(e.AuthoredWeight)
	}
	return fmt.Sprintf("authored weight %s; estimated weight %d; %s", authored, e.EstimatedWeight, e.Label)
}
