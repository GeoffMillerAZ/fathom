package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"

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

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}
