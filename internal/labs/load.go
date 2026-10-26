package labs

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/encoding/yaml"
	labsschema "github.com/geoffmilleraz/fathom/schema/labs"
)

// Lab is a validated feature card. Features are scorecard or FM-1 references,
// never the inventory used to decide surface closure.
type Lab struct {
	ID       string   `json:"id"`
	Features []string `json:"features"`
	Surface  []string `json:"surface"`
	Firing   struct {
		Should    string `json:"should"`
		ShouldNot string `json:"should_not"`
		Grader    string `json:"grader"`
		Mode      string `json:"mode"`
		Reason    string `json:"reason"`
	} `json:"firing"`
	Helped struct {
		Metric                  string  `json:"metric"`
		Arm                     string  `json:"arm"`
		Direction               string  `json:"direction"`
		MinimumEffectOfInterest float64 `json:"minimum_effect_of_interest"`
	} `json:"helped"`
	Quality  []QualityReading `json:"quality"`
	Fixtures []string         `json:"fixtures"`
	Harness  []string         `json:"harness"`
	Kill     []KillCriterion  `json:"kill"`
	Knobs    []LabKnob        `json:"knobs"`
	Owners   []string         `json:"owners"`
}

type QualityReading struct {
	Instrument string `json:"instrument"`
	Reading    string `json:"reading"`
}
type KillCriterion struct {
	Criterion string `json:"criterion"`
	Reading   string `json:"reading"`
}
type LabKnob struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Location string `json:"location"`
}

func labDefinition(name string) cue.Value {
	return cuecontext.New().CompileString(labsschema.Source).LookupPath(cue.ParsePath(name))
}

func loadLabYAML(path string, definition cue.Value, dst any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	f, err := yaml.Extract(filepath.Base(path), b)
	if err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	v := definition.Unify(definition.Context().BuildFile(f))
	if err := v.Validate(cue.Concrete(true)); err != nil {
		return fmt.Errorf("%s: %s", filepath.Base(path), errors.Details(err, nil))
	}
	if err := v.Decode(dst); err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return nil
}

func surfaceSet(keys []string) map[string]bool {
	out := make(map[string]bool, len(keys))
	for _, key := range keys {
		out[key] = true
	}
	return out
}

// LoadLabs reads the *.yaml cards in dir, excluding no-lab.yaml, against #Lab.
// knownSurfaces is the caller's inventory, including any stanza or configured
// decision sites. It refuses unknown keys rather than inferring membership from
// feature IDs. Results are sorted by ID. Helped arms use the exact name
// plugin-minus-<lab slug>: each denotes removal of this card's feature only,
// with every other plugin feature retained (ADR-082). The skills and decision
// cards instead require singular templates plugin-minus-skill:{skill} and
// plugin-minus-decide:{site}, instantiated per selected member by future runners.
func LoadLabs(dir string, knownSurfaces []string) ([]Lab, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	known := surfaceSet(knownSurfaces)
	def := labDefinition("#Lab")
	cards := []Lab{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || name == "no-lab.yaml" || !strings.HasSuffix(name, ".yaml") {
			continue
		}
		var card Lab
		if err := loadLabYAML(filepath.Join(dir, name), def, &card); err != nil {
			return nil, err
		}
		if name != card.ID+".yaml" {
			return nil, fmt.Errorf("file name %q must be %s.yaml", name, card.ID)
		}
		for _, key := range card.Surface {
			if !known[key] {
				return nil, fmt.Errorf("%s: unknown surface %q", name, key)
			}
		}
		arm := "plugin-minus-" + strings.TrimPrefix(card.ID, "lab-")
		family := ""
		switch card.ID {
		case "lab-skills":
			arm, family = "plugin-minus-skill:{skill}", "skill:"
		case "lab-decision-port":
			arm, family = "plugin-minus-decide:{site}", "decide:"
		}
		if card.Helped.Arm != arm {
			return nil, fmt.Errorf("%s: helped arm must remove only this feature (%s)", name, arm)
		}
		if family != "" {
			for _, key := range card.Surface {
				if !strings.HasPrefix(key, family) {
					return nil, fmt.Errorf("%s: singular helped arm requires %s surfaces", name, family)
				}
			}
		}
		cards = append(cards, card)
	}
	sort.Slice(cards, func(i, j int) bool { return cards[i].ID < cards[j].ID })
	return cards, nil
}
