package support

// targets.go — the target-file loader (SM-3, design §2): the shared
// target list for every kind except language and tool, which keep their
// own list, the language queue (LB-F22).

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// LoadTargets reads path (docs/plans/support.targets.json's shape), vets it
// against the embedded #Targets definition, and decodes it into Target rows
// sorted by (kind, id). #Target's own schema already refuses a `language`
// or `tool` kind row (they stay in the language queue), so a bad file fails
// here before any Go-side check runs.
func LoadTargets(path string) ([]Target, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("support: read %s: %w", path, err)
	}

	cctx := cuecontext.New()
	def, err := compileSupportDef(cctx, "#Targets")
	if err != nil {
		return nil, err
	}
	val := cctx.CompileBytes(data, cue.Filename(path))
	if err := val.Err(); err != nil {
		return nil, fmt.Errorf("support: compile %s: %w", path, err)
	}
	unified := def.Unify(val)
	if err := unified.Validate(cue.Concrete(true)); err != nil {
		return nil, fmt.Errorf("support: %s does not validate against #Targets: %w", path, err)
	}

	var targets []Target
	if err := json.Unmarshal(data, &targets); err != nil {
		return nil, fmt.Errorf("support: decode %s: %w", path, err)
	}
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].Kind != targets[j].Kind {
			return targets[i].Kind < targets[j].Kind
		}
		return targets[i].ID < targets[j].ID
	})
	return targets, nil
}
