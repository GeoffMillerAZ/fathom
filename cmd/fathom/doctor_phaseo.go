package main

// doctor_phaseo.go — phase-O doctor checks (docs/design/platform-modules.md
// §4 deliverable 4, docs/design/guarded-edits.md §2.4): binding_file/
// guard_file validity (loud on malformed), binding_modules_mounted (loud
// on a bound-but-unmounted module), binding_module_readiness (warn on a
// bound module below `servable`), guard_dead_scopes (warn on a guard
// whose scopes match zero indexed files), guard_include_modules_unbound
// (warn on include_modules:true with no binding to include).
//
// House honesty discipline (doctor.go's own doc comment): an uncheckable
// state is `skipped` with a reason, never a guessed pass. Absence of
// .fathom/modules.cue or .fathom/guard.cue is the overwhelmingly common
// case (phase O ships with NEITHER active by default — the fathom-init
// scaffold is commented) and is reported `pass`, not `skipped` — there is
// nothing uncertain about "no bindings/guards configured."

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/bindspec"
	"github.com/geoffmilleraz/fathom/internal/filter"
	"github.com/geoffmilleraz/fathom/internal/guardspec"
)

// callMCPTool POSTs a tools/call request for name/args to baseURL/rpc and
// returns the unwrapped result payload — the shared shape checkObligations
// Served and checkSurfaceTraffic each inline separately; factored out here
// since phase O adds several more callers of the same pattern.
func callMCPTool(hc *http.Client, baseURL, name string, args map[string]any) (json.RawMessage, error) {
	reqBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]any{"name": name, "arguments": args},
	})
	if err != nil {
		return nil, err
	}
	req, err := originHeader(baseURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	return unwrapRPCResult(raw)
}

// ─── binding_file / guard_file: malformed config is loud ──────────────────

// checkBindingFile validates <root>/.fathom/modules.cue, when present,
// against bindspec.LoadBindings — the same reader modserve.Bind uses on
// the hot serving path (which fails OPEN on a bad file); doctor is the
// loud, non-hot-path place that surfaces it instead.
func checkBindingFile(root string) doctorCheck {
	const name = "binding_file"
	path := filepath.Join(root, ".fathom", "modules.cue")
	if _, err := os.Stat(path); err != nil {
		return doctorCheck{Name: name, Status: doctorPass, Reason: "no .fathom/modules.cue (no platform-module bindings configured)"}
	}
	set, err := bindspec.LoadBindings(path)
	if err != nil {
		return doctorCheck{
			Name: name, Status: doctorFail,
			Reason: fmt.Sprintf(".fathom/modules.cue is invalid: %v", err),
			Remedy: "fix the reported field(s), or see docs/design/platform-modules.md §3.1 for the binding shape",
		}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf(".fathom/modules.cue valid — %d binding(s)", len(set.Bindings))}
}

// checkGuardFile is checkBindingFile's twin for .fathom/guard.cue.
func checkGuardFile(root string) doctorCheck {
	const name = "guard_file"
	path := filepath.Join(root, ".fathom", "guard.cue")
	if _, err := os.Stat(path); err != nil {
		return doctorCheck{Name: name, Status: doctorPass, Reason: "no .fathom/guard.cue (no guarded scopes configured)"}
	}
	set, err := guardspec.LoadGuards(path)
	if err != nil {
		return doctorCheck{
			Name: name, Status: doctorFail,
			Reason: fmt.Sprintf(".fathom/guard.cue is invalid: %v", err),
			Remedy: "fix the reported field(s), or see docs/design/guarded-edits.md §2.1 for the guard shape",
		}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf(".fathom/guard.cue valid — %d guard(s)", len(set.Guards))}
}

// ─── binding_modules_mounted: an unmounted bound module is loud ───────────

// checkBindingModulesMounted cross-checks every binding's module name
// against list_modules — a binding naming a module this daemon does not
// have mounted at all is a silent-injection trap (nothing the repo binds
// will ever reach an edit) and is reported loud (fail), matching
// platform-modules.md §4 deliverable 4 exactly.
func checkBindingModulesMounted(hc *http.Client, baseURL, root string, daemonUp bool) doctorCheck {
	const name = "binding_modules_mounted"
	set, ok, skip := loadBindingSetForDoctor(root)
	if skip != nil {
		return doctorCheck{Name: name, Status: doctorPass, Reason: *skip}
	}
	if !ok {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "binding_file is invalid — see that check"}
	}
	if !daemonUp {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "daemon unreachable — cannot list mounted modules (see daemon_reachable)"}
	}
	mounted, err := listMountedModuleNames(hc, baseURL)
	if err != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("list_modules: %v", err)}
	}
	var unmounted []string
	for _, b := range set.Bindings {
		if !mounted[b.Module] {
			unmounted = append(unmounted, b.Module)
		}
	}
	if len(unmounted) > 0 {
		sort.Strings(unmounted)
		return doctorCheck{
			Name: name, Status: doctorFail,
			Reason: fmt.Sprintf("bound module(s) not mounted on this daemon: %s — bound rules from these modules can never be served (platform-modules.md §3.3's own explicit-note doctrine covers the SERVING side; this is the config-time catch)", strings.Join(unmounted, ", ")),
			Remedy: "mount the module (serve-http --module <path>) or remove the binding from .fathom/modules.cue",
		}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("all %d bound module(s) are mounted", len(set.Bindings))}
}

func listMountedModuleNames(hc *http.Client, baseURL string) (map[string]bool, error) {
	inner, err := callMCPTool(hc, baseURL, "list_modules", map[string]any{})
	if err != nil {
		return nil, err
	}
	var payload struct {
		Modules []struct {
			Module string `json:"module"`
		} `json:"modules"`
	}
	if err := json.Unmarshal(inner, &payload); err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(payload.Modules))
	for _, m := range payload.Modules {
		out[m.Module] = true
	}
	return out, nil
}

// ─── binding_module_readiness: a bound module below `servable` warns ──────

// checkBindingModuleReadiness runs the phase-J readiness ladder
// (module_doctor.go's runDoctor) for each bound module, but ONLY when the
// module's manifest is locally resolvable — the daemon's list_modules
// reports a mounted module's STORE path (a compiled .sqlite/.db), not its
// source module.cue; the readiness ladder needs the manifest. The one
// convention this checks (fathom module compile's own typical layout,
// module.go): a module.cue sitting next to the mounted store file, or the
// mounted path itself being a directory containing module.cue. Anything
// else is honestly reported "not checkable: module not local" (per this
// row's own DoD) rather than fabricating a verdict this machine cannot
// actually compute.
func checkBindingModuleReadiness(hc *http.Client, baseURL, root string, daemonUp bool) doctorCheck {
	const name = "binding_module_readiness"
	set, ok, skip := loadBindingSetForDoctor(root)
	if skip != nil {
		return doctorCheck{Name: name, Status: doctorPass, Reason: *skip}
	}
	if !ok {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "binding_file is invalid — see that check"}
	}
	if !daemonUp {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "daemon unreachable — cannot resolve mounted module paths (see daemon_reachable)"}
	}
	paths, err := mountedModulePaths(hc, baseURL)
	if err != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("list_modules: %v", err)}
	}

	var belowServable, notLocal []string
	checked := 0
	for _, b := range set.Bindings {
		storePath, mounted := paths[b.Module]
		if !mounted {
			continue // binding_modules_mounted already reports this loudly
		}
		manifestPath := resolveLocalManifest(storePath)
		if manifestPath == "" {
			notLocal = append(notLocal, b.Module)
			continue
		}
		checked++
		report := runDoctor(manifestPath, filepath.Dir(manifestPath), storePath, b.Module, "")
		if ladderRank[report.Verdict] < ladderRank[LevelServable] {
			belowServable = append(belowServable, fmt.Sprintf("%s (%s)", b.Module, report.Verdict))
		}
	}

	switch {
	case checked == 0 && len(notLocal) > 0:
		sort.Strings(notLocal)
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("not checkable: module not local for %s (no module.cue found next to the mounted store)", strings.Join(notLocal, ", "))}
	case checked == 0:
		return doctorCheck{Name: name, Status: doctorPass, Reason: "no bound module has a locally resolvable manifest to check (or nothing bound)"}
	case len(belowServable) > 0:
		sort.Strings(belowServable)
		reason := fmt.Sprintf("bound module(s) below `servable`: %s", strings.Join(belowServable, ", "))
		if len(notLocal) > 0 {
			sort.Strings(notLocal)
			reason += fmt.Sprintf(" (not checkable, no local manifest: %s)", strings.Join(notLocal, ", "))
		}
		return doctorCheck{
			Name: name, Status: doctorWarn, Reason: reason,
			Remedy: "run `fathom module doctor` in the module's own repo to see which criteria are unmet",
		}
	default:
		reason := fmt.Sprintf("%d bound module(s) checked, all >= servable", checked)
		if len(notLocal) > 0 {
			sort.Strings(notLocal)
			reason += fmt.Sprintf(" (not checkable, no local manifest: %s)", strings.Join(notLocal, ", "))
		}
		return doctorCheck{Name: name, Status: doctorPass, Reason: reason}
	}
}

func mountedModulePaths(hc *http.Client, baseURL string) (map[string]string, error) {
	inner, err := callMCPTool(hc, baseURL, "list_modules", map[string]any{})
	if err != nil {
		return nil, err
	}
	var payload struct {
		Modules []struct {
			Module string `json:"module"`
			Path   string `json:"path"`
		} `json:"modules"`
	}
	if err := json.Unmarshal(inner, &payload); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(payload.Modules))
	for _, m := range payload.Modules {
		if _, exists := out[m.Module]; !exists {
			out[m.Module] = m.Path
		}
	}
	return out, nil
}

// resolveLocalManifest returns the local module.cue path for a mounted
// store path, or "" when none is found by either convention this check
// supports (see checkBindingModuleReadiness's doc comment).
func resolveLocalManifest(storePath string) string {
	if storePath == "" {
		return ""
	}
	if fi, err := os.Stat(storePath); err == nil && fi.IsDir() {
		if p := filepath.Join(storePath, "module.cue"); fileExists(p) {
			return p
		}
		return ""
	}
	if p := filepath.Join(filepath.Dir(storePath), "module.cue"); fileExists(p) {
		return p
	}
	return ""
}

// ─── guard_dead_scopes: a guard matching zero indexed files warns ─────────

// checkGuardDeadScopes cross-checks every FILE guard's scopes against
// list_sources — a guard whose scopes match nothing in the index is a
// prune candidate, reported (never auto-removed, design §2.4/law 8).
// Command-scoped guards are NOT checkable this way (no "indexed commands"
// exist) — reported once, honestly, rather than silently skipped per
// guard.
func checkGuardDeadScopes(hc *http.Client, baseURL, root, explicitSvc string, daemonUp bool) doctorCheck {
	const name = "guard_dead_scopes"
	set, ok, skip := loadGuardSetForDoctor(root)
	if skip != nil {
		return doctorCheck{Name: name, Status: doctorPass, Reason: *skip}
	}
	if !ok {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "guard_file is invalid — see that check"}
	}
	if !daemonUp {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "daemon unreachable — cannot list indexed sources (see daemon_reachable)"}
	}

	svc := explicitSvc
	if svc == "" {
		cands := candidatesWithDaemon(hc, baseURL, root)
		if len(cands) > 0 {
			svc = strings.ToLower(cands[0].name)
		}
	}
	sources, err := listSourcesForDoctor(hc, baseURL, svc)
	if err != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("list_sources(%s): %v", svc, err)}
	}

	var dead []string
	var commandGuardCount int
	for i, g := range set.Guards {
		if len(g.CommandScopes) > 0 {
			commandGuardCount++
		}
		if len(g.Scopes) == 0 {
			continue
		}
		m := filter.New(g.Scopes)
		matches := false
		for _, src := range sources {
			if m.Match(src) {
				matches = true
				break
			}
		}
		if !matches {
			dead = append(dead, fmt.Sprintf("guards[%d] (%s)", i, strings.Join(g.Scopes, ", ")))
		}
	}

	note := ""
	if commandGuardCount > 0 {
		note = fmt.Sprintf(" (%d command-scoped guard(s) not checkable this way — no indexed-commands concept; see research report's guarded-edit coverage instead)", commandGuardCount)
	}
	if len(dead) > 0 {
		return doctorCheck{
			Name: name, Status: doctorWarn,
			Reason: fmt.Sprintf("dead guard(s), matching zero indexed file(s): %s%s", strings.Join(dead, "; "), note),
			Remedy: "narrow or remove the scope in .fathom/guard.cue, or re-ingest if the index is stale",
		}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("every file-scoped guard matches at least one indexed file%s", note)}
}

func listSourcesForDoctor(hc *http.Client, baseURL, service string) ([]string, error) {
	if service == "" {
		return nil, fmt.Errorf("no service resolved")
	}
	inner, err := callMCPTool(hc, baseURL, "list_sources", map[string]any{"service": service})
	if err != nil {
		return nil, err
	}
	var payload struct {
		Sources []string `json:"sources"`
	}
	if err := json.Unmarshal(inner, &payload); err != nil {
		return nil, err
	}
	return payload.Sources, nil
}

// ─── guard_include_modules_unbound: include_modules with no binding warns ─

// checkGuardIncludeModulesUnbound catches a guard configured with
// include_modules:true in a repo with no (or no VALID) .fathom/modules.cue
// bindings at all — the toggle has nothing to include.
func checkGuardIncludeModulesUnbound(root string) doctorCheck {
	const name = "guard_include_modules_unbound"
	gset, gok, gskip := loadGuardSetForDoctor(root)
	if gskip != nil {
		return doctorCheck{Name: name, Status: doctorPass, Reason: *gskip}
	}
	if !gok {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "guard_file is invalid — see that check"}
	}
	var withInclude []int
	for i, g := range gset.Guards {
		if g.IncludeModules {
			withInclude = append(withInclude, i)
		}
	}
	if len(withInclude) == 0 {
		return doctorCheck{Name: name, Status: doctorPass, Reason: "no guard sets include_modules"}
	}

	bset, bok, bskip := loadBindingSetForDoctor(root)
	hasBindings := bskip == nil && bok && len(bset.Bindings) > 0
	if hasBindings {
		return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("%d guard(s) set include_modules and %d binding(s) exist to include", len(withInclude), len(bset.Bindings))}
	}
	return doctorCheck{
		Name: name, Status: doctorWarn,
		Reason: fmt.Sprintf("%d guard(s) set include_modules:true but this repo has no platform-module bindings — nothing to include", len(withInclude)),
		Remedy: "bind a module in .fathom/modules.cue, or set include_modules:false",
	}
}

// ─── shared loaders ─────────────────────────────────────────────────────────

// loadBindingSetForDoctor returns (set, ok, skipReason). skipReason is
// non-nil only for the "absent -> honest pass" case (the caller should
// report doctorPass with that exact reason, not skip); ok=false with
// skipReason=nil means the file exists but is invalid (the caller should
// point at binding_file).
func loadBindingSetForDoctor(root string) (bindspec.BindingSet, bool, *string) {
	path := filepath.Join(root, ".fathom", "modules.cue")
	if _, err := os.Stat(path); err != nil {
		reason := "no .fathom/modules.cue (no platform-module bindings configured)"
		return bindspec.BindingSet{}, false, &reason
	}
	set, err := bindspec.LoadBindings(path)
	if err != nil {
		return bindspec.BindingSet{}, false, nil
	}
	return set, true, nil
}

func loadGuardSetForDoctor(root string) (guardspec.GuardSet, bool, *string) {
	path := filepath.Join(root, ".fathom", "guard.cue")
	if _, err := os.Stat(path); err != nil {
		reason := "no .fathom/guard.cue (no guarded scopes configured)"
		return guardspec.GuardSet{}, false, &reason
	}
	set, err := guardspec.LoadGuards(path)
	if err != nil {
		return guardspec.GuardSet{}, false, nil
	}
	return set, true, nil
}
