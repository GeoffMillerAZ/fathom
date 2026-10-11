package main

// doctor_compile.go — GL-27 (G6, owner ruling on LF5): the compile path's
// three silent failures, each named by a doctor row.
//
//   - profile_bins: a configured LLM profile whose `bin` no longer exists
//     (the reproduced case: ~/.dev-cache/npm/global/bin/claude after the CLI
//     moved to ~/.local/bin/claude). Before this row the daemon logged one
//     WARN at start and compile_layer stayed off; nothing in doctor said so.
//   - compile_layer: the daemon's own compile_layer state, read from
//     status(section: "daemon"), FAIL when it started with a configured
//     provider it could not build.
//   - layers_stale: a service's obligations layer with fewer sections
//     compiled than total (INC-27 rode on a stale layer). WARN, with the
//     --dry-run estimate first: a spending compile needs owner approval.
//
// No model is called and no binary is executed: profile_bins only stats the
// path; LookPath only searches PATH.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// checkProfileBins loads the user and project config and verdicts every
// configured profile's bin (doctor_providers.go's own load shape).
func checkProfileBins(root string) doctorCheck {
	loaded, err := config.Load(config.DefaultUserPath(), filepath.Join(root, ".fathom", "config.toml"))
	if err != nil {
		return doctorCheck{Name: "profile_bins", Status: doctorSkipped, Reason: fmt.Sprintf("config did not load: %v", err)}
	}
	return profileBinsVerdict(loaded, exec.LookPath)
}

// profileBinsVerdict FAILs, naming the path, the profile and the fix, when
// a profile's bin does not exist. lookPath finds where a binary of the same
// name now lives (nil: no hint).
func profileBinsVerdict(loaded config.Loaded, lookPath func(string) (string, error)) doctorCheck {
	const name = "profile_bins"
	names := make([]string, 0, len(loaded.Config.Profiles))
	for n := range loaded.Config.Profiles {
		names = append(names, n)
	}
	sort.Strings(names)

	checked := 0
	var missing []string
	var firstName string
	for _, n := range names {
		bin := loaded.Config.Profiles[n].Bin
		if bin == "" {
			continue
		}
		checked++
		path := bin
		if strings.HasPrefix(path, "~/") {
			if home, herr := os.UserHomeDir(); herr == nil {
				path = filepath.Join(home, path[2:])
			}
		}
		if _, serr := os.Stat(path); serr == nil {
			continue
		}
		line := fmt.Sprintf("[profiles.%s] bin %s does not exist", n, bin)
		if lookPath != nil {
			if found, lerr := lookPath(filepath.Base(path)); lerr == nil && found != "" && found != path {
				line += fmt.Sprintf(" (a %s is on PATH at %s)", filepath.Base(path), found)
			}
		}
		if firstName == "" {
			firstName = n
		}
		missing = append(missing, line)
	}
	if checked == 0 {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "no configured profile names a bin"}
	}
	if len(missing) > 0 {
		return doctorCheck{
			Name: name, Status: doctorFail,
			Reason: strings.Join(missing, "; ") + " — the daemon cannot build this profile's client, so compile_layer stays disabled",
			Remedy: fmt.Sprintf("edit [profiles.%s] bin in ~/.fathom/config.toml to the CLI's current path, then restart the daemon (task deploy)", firstName),
		}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("%d profile bin(s) exist", checked)}
}

// checkCompileLayerEnabled reads the daemon's compile_layer state from
// status(section: "daemon"). A daemon with no provider configured at all is
// skipped (dry-run estimates still work); only a configured provider the
// daemon failed to build is a FAIL.
func checkCompileLayerEnabled(hc *http.Client, baseURL string, daemonUp bool) doctorCheck {
	const name = "compile_layer"
	if !daemonUp {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "daemon unreachable — compile_layer state is the daemon's own (see daemon_reachable)"}
	}
	inner, err := callMCPTool(hc, baseURL, "status", map[string]any{"section": "daemon"})
	if err != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("status(daemon) did not answer: %v", err)}
	}
	var payload struct {
		CompileLayer *struct {
			Enabled    bool   `json:"enabled"`
			Configured bool   `json:"configured"`
			Reason     string `json:"reason"`
		} `json:"compile_layer"`
	}
	if err := json.Unmarshal(inner, &payload); err != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("status(daemon) returned an unreadable answer: %v", err)}
	}
	st := payload.CompileLayer
	switch {
	case st == nil:
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "the daemon predates the compile_layer state in status(daemon)", Remedy: "task deploy"}
	case st.Enabled:
		return doctorCheck{Name: name, Status: doctorPass, Reason: "compile_layer is enabled on the daemon"}
	case !st.Configured:
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "compile_layer is off: " + st.Reason + " — dry-run estimates still work"}
	}
	return doctorCheck{
		Name: name, Status: doctorFail,
		Reason: "the daemon started with compile_layer disabled: " + st.Reason,
		Remedy: "fix the profile the daemon names: edit [profiles.<name>] bin (or the provider's credential) in ~/.fathom/config.toml, then restart the daemon (task deploy)",
	}
}

// checkStaleLayers WARNs per service when the obligations layer has fewer
// sections compiled than total (ObligationsLayerCoverage through the
// obligations tool's layer_coverage). It checks the services this repo
// resolves to, like obligations_served.
func checkStaleLayers(hc *http.Client, baseURL, root, explicitSvc string, daemonUp bool) doctorCheck {
	const name = "layers_stale"
	if !daemonUp {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "daemon unreachable — layer coverage is the daemon's (see daemon_reachable)"}
	}
	var cands []string
	if explicitSvc != "" {
		cands = []string{explicitSvc}
	} else {
		seen := map[string]bool{}
		for _, c := range candidatesWithDaemon(hc, baseURL, root) {
			if n := strings.ToLower(c.name); !seen[n] {
				seen[n] = true
				cands = append(cands, n)
			}
		}
	}
	svcs, _ := fetchServices(hc, baseURL)
	var stale, remedies []string
	checked := 0
	for _, svc := range cands {
		if _, ok := svcs[svc]; !ok {
			continue
		}
		inner, err := callMCPTool(hc, baseURL, "obligations", map[string]any{"service": svc})
		if err != nil {
			continue
		}
		var payload struct {
			LayerCoverage *doctorLayerCoverage `json:"layer_coverage"`
		}
		if json.Unmarshal(inner, &payload) != nil || payload.LayerCoverage == nil || payload.LayerCoverage.SectionsTotal == 0 {
			continue
		}
		checked++
		cov := payload.LayerCoverage
		if cov.SectionsCompiled >= cov.SectionsTotal {
			continue
		}
		stale = append(stale, fmt.Sprintf("%s: %s layer %d/%d sections compiled (budget_stopped=%v)", svc, semantic.LayerObligations, cov.SectionsCompiled, cov.SectionsTotal, cov.BudgetStopped))
		remedies = append(remedies, fmt.Sprintf("fathom compile --service %s --layer obligations --dry-run", svc))
	}
	if checked == 0 {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "no indexed service of this repo reported layer coverage"}
	}
	if len(stale) == 0 {
		return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("%d service(s): every compiled section count matches the corpus", checked)}
	}
	return doctorCheck{
		Name: name, Status: doctorWarn,
		Reason: strings.Join(stale, "; ") + " — rules for the uncompiled sections are served raw or not at all",
		Remedy: strings.Join(remedies, "; ") + "  (estimate first; a spending --dry-run=false compile needs owner approval)",
	}
}
