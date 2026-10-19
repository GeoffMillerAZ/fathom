package main

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/geoffmilleraz/fathom/internal/config"
)

// configcmd.go — CF-4 (docs/plans/config-experience.md): `fathom config`
// prints the effective configuration; `--explain` is the flagship UX
// feature — annotating every key with its provenance (flag/env/project/
// user/default) and, for a file source, the exact path that supplied it.
// Pure local file reads — no daemon dependency, so this command has no
// fail-open story to get right; a bad --profile is simply an error.

// cmdConfig implements "config": load the user+project layers, resolve the
// active profile (--profile, else default_profile, else the env-overlay
// alone), and print it — human text by default, --json for machine
// consumption, --explain to annotate every key with its source.
func cmdConfig(args []string, stdout, stderr io.Writer) int {
	// The config custodian's verb (vision 22, docs/plans/custodian.md CC-5)
	// hangs off the config namespace: `fathom config audit`.
	if len(args) > 0 && args[0] == "audit" {
		return cmdConfigAudit(args[1:], stdout, stderr)
	}
	// ...and its diff-offering twin (CC-9): `fathom config propose` offers
	// what the audit's violations would take to mend, and writes nothing.
	if len(args) > 0 && args[0] == "propose" {
		return cmdConfigPropose(args[1:], stdout, stderr)
	}
	fs := newFlagSet("config", stderr)
	profileFlag := fs.String("profile", "", "show this named profile instead of the resolved default")
	explain := fs.Bool("explain", false, "annotate every key with its provenance (flag/env/project/user/default) and source file/env-var")
	jsonOut := fs.Bool("json", false, "emit JSON instead of human text")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	loaded, err := config.Load(config.DefaultUserPath(), config.DefaultProjectPath())
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	profile, prov, err := config.ResolveProfile(loaded.Config, loaded.Provenance, *profileFlag)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	activeName := *profileFlag
	if activeName == "" {
		activeName = loaded.Config.DefaultProfile
	}

	if *jsonOut {
		printConfigJSON(stdout, loaded, profile, prov, activeName, *explain)
		return 0
	}
	printConfigText(stdout, loaded, profile, prov, activeName, *explain)
	return 0
}

// sourceLabel renders a SourceInfo as "source" or "source:path" for
// env/user/project sources that carry a path/env-var name.
func sourceLabel(si config.SourceInfo) string {
	if si.Path == "" {
		return string(si.Source)
	}
	return fmt.Sprintf("%s:%s", si.Source, si.Path)
}

// profileFieldRows returns the ordered (key, value) pairs of a resolved
// Profile, for both text and JSON rendering.
func profileFieldRows(p config.Profile) [][2]string {
	rows := [][2]string{
		{"provider", p.Provider},
		{"model", p.Model},
		{"endpoint", p.Endpoint},
		{"region", p.Region},
		{"deployment", p.Deployment},
		{"api_version", p.APIVersion},
		{"bin", p.Bin},
		{"api_key_env", p.APIKeyEnv},
	}
	if p.Timeout != "" {
		rows = append(rows, [2]string{"timeout", p.Timeout})
	}
	if len(p.APIKeyCmd) > 0 {
		rows = append(rows, [2]string{"api_key_cmd", fmt.Sprintf("%v", p.APIKeyCmd)})
	} else {
		rows = append(rows, [2]string{"api_key_cmd", ""})
	}
	return rows
}

func printConfigText(w io.Writer, loaded config.Loaded, profile config.Profile, prov config.Provenance, activeName string, explain bool) {
	fmt.Fprintf(w, "user config:    %s", loaded.UserPath)
	if !loaded.UserFound {
		fmt.Fprint(w, " (not found)")
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "project config: %s", loaded.ProjectPath)
	if !loaded.ProjectFound {
		fmt.Fprint(w, " (not found)")
	}
	fmt.Fprintln(w)
	// SEC-1/SEC-2: a repository's user-only fields are dropped at Load;
	// name each one (file + field + why, never a value) so the setting's
	// silence is not a mystery. They are read from the user config only.
	// A repo-path value that leaves the repository is refused where it is
	// used; name those too.
	for _, ig := range loaded.IgnoredKeySources {
		fmt.Fprintf(w, "ignored:        %s: %s [user-only, %s; set in %s]\n", ig.Path, ig.Field, ig.Reason, loaded.UserPath)
	}
	for _, r := range configRefusedRepoPaths(loaded) {
		fmt.Fprintf(w, "refused:        %s: %s %v [repo-path: name a path inside the repository]\n", loaded.ProjectPath, r.Field, r.Err)
	}

	defaultSrc := ""
	if si, ok := loaded.Provenance["default_profile"]; ok {
		defaultSrc = " [" + sourceLabel(si) + "]"
	}
	fmt.Fprintf(w, "default_profile: %s%s\n", loaded.Config.DefaultProfile, defaultSrc)

	names := make([]string, 0, len(loaded.Config.Profiles))
	for n := range loaded.Config.Profiles {
		names = append(names, n)
	}
	sort.Strings(names)
	fmt.Fprintf(w, "known profiles:  %v\n", names)

	label := activeName
	if label == "" {
		label = "(env-overlay only — no named profile selected)"
	}
	fmt.Fprintf(w, "\nactive profile: %s\n", label)
	for _, row := range profileFieldRows(profile) {
		key, val := row[0], row[1]
		if val == "" && !explain {
			continue
		}
		line := fmt.Sprintf("  %-12s %s", key+":", val)
		if explain {
			src := SourceDefaultLabel
			if si, ok := prov[key]; ok {
				src = sourceLabel(si)
			}
			line += fmt.Sprintf("  [%s]", src)
		}
		fmt.Fprintln(w, line)
	}

	fmt.Fprintln(w, "\ndaemon:")
	printDaemonRow(w, "store", loaded.Config.Daemon.Store, loaded.Provenance["daemon.store"], explain)
	printDaemonRow(w, "addr", loaded.Config.Daemon.Addr, loaded.Provenance["daemon.addr"], explain)
	printDaemonRow(w, "call_log", loaded.Config.Daemon.CallLog, loaded.Provenance["daemon.call_log"], explain)
	autoRefresh := ""
	if loaded.Config.Daemon.AutoRefresh != nil {
		autoRefresh = fmt.Sprintf("%v", *loaded.Config.Daemon.AutoRefresh)
	}
	printDaemonRow(w, "auto_refresh", autoRefresh, loaded.Provenance["daemon.auto_refresh"], explain)
	printDaemonRow(w, "default_profile", loaded.Config.Daemon.DefaultProfile, loaded.Provenance["daemon.default_profile"], explain)
	printDaemonRow(w, "state_dir", loaded.Config.Daemon.StateDir, loaded.Provenance["daemon.state_dir"], explain)

	// KS-10: the ask cap is the minimum across layers, so its row always
	// names the layer whose value is in effect; shown when a layer sets it.
	if si, ok := loaded.Provenance["decide.max_asks_per_site_per_session"]; ok {
		fmt.Fprintln(w, "\ndecide:")
		printDaemonRow(w, "max_asks_per_site_per_session", strconv.Itoa(loaded.Config.Decide.MaxAsks()), si, true)
	}
}

// SourceDefaultLabel is what --explain prints for a key no layer set.
const SourceDefaultLabel = "default"

func printDaemonRow(w io.Writer, key, val string, si config.SourceInfo, explain bool) {
	if val == "" && !explain {
		return
	}
	line := fmt.Sprintf("  %-16s %s", key+":", val)
	if explain {
		src := SourceDefaultLabel
		if si.Source != "" {
			src = sourceLabel(si)
		}
		line += fmt.Sprintf("  [%s]", src)
	}
	fmt.Fprintln(w, line)
}

func printConfigJSON(w io.Writer, loaded config.Loaded, profile config.Profile, prov config.Provenance, activeName string, explain bool) {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(configJSONValue(loaded, profile, prov, activeName, explain))
}

// configJSONValue is `fathom config --json`'s value; the console's Operate
// page renders `fathom config --explain --json` from it (P10).
func configJSONValue(loaded config.Loaded, profile config.Profile, prov config.Provenance, activeName string, explain bool) map[string]any {
	names := make([]string, 0, len(loaded.Config.Profiles))
	for n := range loaded.Config.Profiles {
		names = append(names, n)
	}
	sort.Strings(names)

	profileMap := map[string]any{}
	explainMap := map[string]string{}
	for _, row := range profileFieldRows(profile) {
		key, val := row[0], row[1]
		profileMap[key] = val
		if si, ok := prov[key]; ok {
			explainMap[key] = sourceLabel(si)
		} else {
			explainMap[key] = SourceDefaultLabel
		}
	}

	daemonMap := map[string]any{
		"store":           loaded.Config.Daemon.Store,
		"addr":            loaded.Config.Daemon.Addr,
		"call_log":        loaded.Config.Daemon.CallLog,
		"default_profile": loaded.Config.Daemon.DefaultProfile,
		"state_dir":       loaded.Config.Daemon.StateDir,
	}
	if loaded.Config.Daemon.AutoRefresh != nil {
		daemonMap["auto_refresh"] = *loaded.Config.Daemon.AutoRefresh
	} else {
		daemonMap["auto_refresh"] = nil
	}

	out := map[string]any{
		"user_config":     loaded.UserPath,
		"user_found":      loaded.UserFound,
		"project_config":  loaded.ProjectPath,
		"project_found":   loaded.ProjectFound,
		"default_profile": loaded.Config.DefaultProfile,
		"known_profiles":  names,
		"active_profile":  activeName,
		"profile":         profileMap,
		"daemon":          daemonMap,
	}
	if explain {
		out["explain"] = explainMap
	}
	if len(loaded.IgnoredKeySources) > 0 {
		ignored := make([]map[string]string, 0, len(loaded.IgnoredKeySources))
		for _, ig := range loaded.IgnoredKeySources {
			ignored = append(ignored, map[string]string{"path": ig.Path, "field": ig.Field, "scope": string(config.ScopeUserOnly), "reason": ig.Reason + "; set in the user config"})
		}
		out["ignored_key_sources"] = ignored
	}
	if refused := configRefusedRepoPaths(loaded); len(refused) > 0 {
		rows := make([]map[string]string, 0, len(refused))
		for _, r := range refused {
			rows = append(rows, map[string]string{"path": loaded.ProjectPath, "field": r.Field, "scope": string(config.ScopeRepoPath), "reason": r.Err.Error()})
		}
		out["refused_repo_paths"] = rows
	}
	return out
}

// configRefusedRepoPaths is the repo-path values in the loaded config that
// leave the repository whose .fathom/config.toml was loaded (none when no
// project layer was found).
func configRefusedRepoPaths(loaded config.Loaded) []config.RepoPathRefusal {
	if !loaded.ProjectFound || loaded.ProjectPath == "" {
		return nil
	}
	return config.RefusedRepoPaths(filepath.Dir(filepath.Dir(loaded.ProjectPath)), loaded.Config)
}

// configRunner runs `fathom config` (registered below; EW-45).
type configRunner struct{}

func (configRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cmdConfig(args, stdout, stderr)
}

func init() {
	declareVerb(verb{Name: "config", Group: "operate", Order: 100, Run: configRunner{},
		Summary: "Print the effective configuration (--explain for per-key provenance) | audit (custodian concerns) | propose (a diff for each mechanically mendable violation; writes nothing)"})
}
