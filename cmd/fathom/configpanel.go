package main

// configpanel.go — EX-7/FR-5: per-service configuration observability. What
// semantic layers are installed (and whether eval-gated), how many guards
// watch which scopes, whether bindings exist, how many rule-injections the
// repo received, and — loudly — the obligations-compile condition, because
// 6,985 staleness-warning serves must never hide behind a healthy-looking
// dashboard. An unconfigured repo sees the install command, not a shrug.

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/geoffmilleraz/fathom/internal/bindspec"
	"github.com/geoffmilleraz/fathom/internal/guardspec"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
	"github.com/geoffmilleraz/fathom/internal/readcoverage"
	readmanifest "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/read-manifest"
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

	layersDir := projectdir.Layers.Path(path)
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

	guards := inspectGuardRules(path)
	p.GuardCount = guards.Count
	p.GuardNote = guards.Note

	p.BindingNote = bindingNote(path)
	return p
}

// bindingNote reads the platform-module bindings from modules.cue, the file
// `fathom init` writes and modserve.Bind serves, through the same loader.
func bindingNote(root string) string {
	modules := projectdir.Modules.Path(root)
	if _, err := os.Stat(modules); err != nil {
		return "no bindings file — no layers are BOUND (binding requires an evaled layer, D3)"
	}
	set, err := bindspec.LoadBindings(modules)
	switch {
	case err != nil:
		return "modules.cue is invalid: " + err.Error()
	case len(set.Bindings) == 0:
		return "modules.cue present, no bindings declared yet"
	}
	return fmt.Sprintf("%d binding(s) in modules.cue", len(set.Bindings))
}

type guardRuleDiagnostic struct {
	Count  int
	Note   string
	Broken bool
}

// inspectGuardRules is the single filesystem/semantic path for every
// reporting surface that counts repository guards. The manifest owns the
// read; ParseGuards applies exactly LoadGuards' CUE semantics to those bytes.
func inspectGuardRules(root string) guardRuleDiagnostic {
	guardFile := projectdir.Guard.Path(root)
	reads := readmanifest.NewSet()
	reads.RelativeTo(root)
	data, ok := reads.ReadFile(guardFile)
	manifest := reads.Manifest()
	if !ok {
		if !manifest.Complete() {
			return guardRuleDiagnostic{
				Note:   readcoverage.RenderCount(manifest, 0, "guard rule(s)"),
				Broken: true,
			}
		}
		return guardRuleDiagnostic{Note: "no guard file — pre-edit warnings are off (adapt a kit guard-pack into .fathom/guard.cue)"}
	}

	set, err := guardspec.ParseGuards(guardFile, data)
	if err != nil {
		return guardRuleDiagnostic{Note: "guard.cue is invalid: " + err.Error(), Broken: true}
	}
	if len(set.Guards) == 0 {
		return guardRuleDiagnostic{Note: "guard.cue present but declares no guard rules"}
	}
	return guardRuleDiagnostic{Count: len(set.Guards)}
}
