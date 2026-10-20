package review

import (
	"os"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
)

// RiskTableFor is root's risk table: the [risk] section of the layered
// config.toml (a repository section over the user default) over the binary's
// defaults. A legacy risk.cue is still read, whole, until the repository's
// config.toml carries a [risk] section; the section then wins and doctor
// names the file. An unreadable config or legacy file leaves the binary
// default, as LoadRiskTable does for a caller that falls back.
func RiskTableFor(root string) RiskTable {
	table := DefaultRiskTable()
	if root == "" {
		return table
	}
	loaded, cerr := config.Load(config.DefaultUserPath(), projectdir.Config.Path(root))
	legacy := projectdir.Risk.Path(root)
	if _, err := os.Stat(legacy); err == nil && !shadowed(loaded, legacy) {
		if t, terr := LoadRiskTable(legacy); terr == nil {
			table = t
		}
		return table
	}
	if cerr != nil {
		return table
	}
	if v := loaded.Config.Risk.BlastRadiusHighCallers; v > 0 {
		table.BlastRadiusHighCallers = v
	}
	if v := loaded.Config.Risk.ZoneInterestMediumRate; v > 0 {
		table.ZoneInterestMediumRate = v
	}
	return table
}

func shadowed(l config.Loaded, path string) bool {
	for _, p := range l.ShadowedLegacy {
		if p == path {
			return true
		}
	}
	return false
}
