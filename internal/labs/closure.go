package labs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"cuelang.org/go/cue/cuecontext"
)

// NoLabSurface names an instrument that already owns a surface, or explains
// why that surface has no firing question. Instrument "none" needs a reason.
type NoLabSurface struct {
	Surface    string `json:"surface"`
	Instrument string `json:"instrument"`
	Reason     string `json:"reason"`
}

// CheckLabClosure refuses unknown surface keys and reports sorted gaps in the
// caller's shipped inventory. Only surface keys decide coverage; scorecard and
// FM-1 IDs are carried unchanged by LoadLabs for cross-reference.
// no-lab.yaml is optional when the cards alone cover the inventory.
func CheckLabClosure(dir string, shipped []string) error {
	cards, err := LoadLabs(dir, shipped)
	if err != nil {
		return err
	}
	known := surfaceSet(shipped)
	covered := map[string]string{}
	for _, card := range cards {
		for _, key := range card.Surface {
			if owner := covered[key]; owner != "" {
				return fmt.Errorf("%s: surface %q already claimed by %s", card.ID, key, owner)
			}
			covered[key] = card.ID
		}
	}
	var exemptions struct {
		Surfaces []NoLabSurface `json:"surfaces"`
	}
	def := cuecontext.New().CompileString(`close({surfaces!: [...close({surface!: string, instrument?: string, reason?: string})]})`)
	err = loadLabYAML(filepath.Join(dir, "no-lab.yaml"), def, &exemptions)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	seen := map[string]bool{}
	for _, entry := range exemptions.Surfaces {
		if !known[entry.Surface] {
			return fmt.Errorf("no-lab.yaml: unknown surface %q", entry.Surface)
		}
		if seen[entry.Surface] {
			return fmt.Errorf("no-lab.yaml: duplicate surface %q", entry.Surface)
		}
		seen[entry.Surface] = true
		instrument, reason := strings.TrimSpace(entry.Instrument), strings.TrimSpace(entry.Reason)
		if (instrument == "" || instrument == "none") && reason == "" {
			return fmt.Errorf("no-lab.yaml: %s needs an instrument or a reason", entry.Surface)
		}
		if owner := covered[entry.Surface]; owner != "" {
			return fmt.Errorf("no-lab.yaml: surface %q already claimed by %s", entry.Surface, owner)
		}
		covered[entry.Surface] = "no-lab.yaml"
	}
	missing := []string{}
	for key := range known {
		if covered[key] == "" {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		return fmt.Errorf("lab closure missing: %s", strings.Join(missing, ", "))
	}
	return nil
}
