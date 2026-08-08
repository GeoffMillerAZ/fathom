package main

// configpanel.go — EX-7/FR-5: per-service configuration observability. What
// semantic layers are installed (and whether eval-gated), how many guards
// watch which scopes, whether bindings exist, how many rule-injections the
// repo received, and — loudly — the obligations-compile condition, because
// 6,985 staleness-warning serves must never hide behind a healthy-looking
// dashboard. An unconfigured repo sees the install command, not a shrug.

import (
	"os"
	"path/filepath"
	"strings"
)

// obsConfigPanel is one service's .fathom configuration state.
type obsConfigPanel struct {
	Layers      []obsLayerRow `json:"layers,omitempty"`
	LayersNote  string        `json:"layers_note,omitempty"`
	GuardCount  int           `json:"guard_count"`
	GuardNote   string        `json:"guard_note,omitempty"`
	BindingNote string        `json:"binding_note"`
	// ObligationServes: this service's rule-injection traffic from the
	// call log (the demand side of the config).
	ObligationServes int `json:"obligation_serves"`
}

type obsLayerRow struct {
	Name   string `json:"name"`
	Evaled bool   `json:"evaled"` // eval/ evidence present (D3: only evaled layers are bindable)
}

// buildConfigPanel reads a service's .fathom directory. Absences are named
// with the command that changes them.
func buildConfigPanel(path string, obligationServes int) obsConfigPanel {
	p := obsConfigPanel{ObligationServes: obligationServes}
	if path == "" {
		p.LayersNote = "no recorded corpus path — configuration not inspectable"
		p.GuardNote = "not inspectable"
		p.BindingNote = "not inspectable"
		return p
	}

	layersDir := filepath.Join(path, ".fathom", "layers")
	if entries, err := os.ReadDir(layersDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			evalDir := filepath.Join(layersDir, e.Name(), "eval")
			evaled := false
			if ev, eerr := os.ReadDir(evalDir); eerr == nil && len(ev) > 0 {
				evaled = true
			}
			p.Layers = append(p.Layers, obsLayerRow{Name: e.Name(), Evaled: evaled})
		}
	}
	if len(p.Layers) == 0 {
		p.LayersNote = "no semantic layers installed — fathom layers install <pack-dir> (app-kit and agentic-kit ship packs)"
	}

	guardFile := filepath.Join(path, ".fathom", "guard.cue")
	if raw, err := os.ReadFile(guardFile); err == nil {
		p.GuardCount = strings.Count(string(raw), "note:")
		if p.GuardCount == 0 {
			p.GuardNote = "guard.cue present but empty"
		}
	} else {
		p.GuardNote = "no guard file — pre-edit warnings are off (adapt a kit guard-pack into .fathom/guard.cue)"
	}

	if _, err := os.Stat(filepath.Join(path, ".fathom", "bindings.cue")); err == nil {
		p.BindingNote = "bindings.cue present"
	} else {
		p.BindingNote = "no bindings file — no layers are BOUND (binding requires an evaled layer, D3)"
	}
	return p
}
