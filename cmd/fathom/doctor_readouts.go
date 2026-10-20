package main

// doctor_readouts.go — RO-18 (ADR-071, docs/design/readouts.md): the
// `readouts` row. The configured [readouts] mode and roots, the rules
// loaded and refused (the first failing fixture of each refused rule), the
// harnesses with no view (named by the harness port's capability table),
// and another Bash output rewriter wired in the client's hook settings.
// Reads pass or warn, never FAIL.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/fathomhome"
	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/readout"
)

func checkReadouts(root string) doctorCheck {
	const name = "readouts"
	cfg, ok := readoutsUserConfig()
	if !ok {
		return doctorCheck{Name: name, Status: doctorWarn, Reason: "the user config did not load, so the readouts mode is unknown", Remedy: "fathom doctor (config_valid row)"}
	}
	mode := cfg.Mode
	if mode == "" {
		mode = config.ReadoutsOff
	}
	var parts []string
	status := doctorPass
	var remedy string

	parts = append(parts, "mode "+mode)
	if len(cfg.Roots) > 0 {
		parts = append(parts, "roots "+strings.Join(cfg.Roots, ", "))
	} else if mode != config.ReadoutsOff {
		parts = append(parts, "roots: every root")
	}

	if rs, err := readout.LoadRules(fathomhome.FathomHome()); err != nil {
		parts = append(parts, "rules did not load: "+oneLine(err.Error()))
		status = doctorWarn
	} else {
		readout.LoadRepoRules(rs, root, cfg.Trust)
		parts = append(parts, fmt.Sprintf("%d rules loaded, %d refused", len(rs.Rules), len(rs.Refused)))
		for _, rf := range rs.Refused {
			id := rf.ID
			if id == "" {
				id = filepath.Base(rf.Source)
			}
			parts = append(parts, fmt.Sprintf("refused %s: %s", id, firstFailure(rf.Reason)))
		}
		if len(rs.Refused) > 0 {
			status = doctorWarn
			remedy = "fathom readout test <rule file>"
		}
		if rf := rs.Repo; rf != nil && !rf.Trusted {
			parts = append(parts, fmt.Sprintf("%s sha256 %s not trusted", rf.Path, rf.SHA))
			status = doctorWarn
			if remedy == "" {
				remedy = "list the sha256 under [readouts] trust in ~/.fathom/config.toml to run the file's rules"
			}
		}
	}

	var has, none []string
	for _, d := range harness.All() {
		if v, ok := d.Vehicles[harness.CapReadout]; !ok || v == harness.VehicleAbsent {
			none = append(none, d.Display)
		} else {
			has = append(has, d.Display)
		}
	}
	if len(has) > 0 {
		parts = append(parts, "view on "+strings.Join(has, ", "))
	}
	if len(none) > 0 {
		parts = append(parts, "no view on "+strings.Join(none, ", "))
	}

	if names := otherBashRewriters(root); len(names) > 0 {
		parts = append(parts, "another Bash rewriter wired ("+strings.Join(names, ", ")+"): lean and rich emit nothing while it is")
		status = doctorWarn
	}
	return doctorCheck{Name: name, Status: status, Reason: strings.Join(parts, "; "), Remedy: remedy}
}

// firstFailure is a refusal's reason cut to the fixture it names, or its
// first line when no fixture is named.
func firstFailure(reason string) string {
	reason = oneLine(reason)
	if i := strings.Index(reason, ": "); i > 0 && strings.HasPrefix(reason, "fixture ") {
		return reason[:i] + " fails"
	}
	if len(reason) > 120 {
		reason = reason[:120]
	}
	return reason
}

// otherBashRewriters names, sorted, the first word of every non-fathom
// command on a PreToolUse or PostToolUse entry whose matcher covers Bash,
// in the client's project, project-local and user settings.
func otherBashRewriters(root string) []string {
	paths := []string{
		filepath.Join(root, ".claude", "settings.json"),
		filepath.Join(root, ".claude", "settings.local.json"),
	}
	if p := claudeUserSettingsFile(); p != "" {
		paths = append(paths, p)
	}
	seen := map[string]bool{}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var parsed map[string]any
		if json.Unmarshal(data, &parsed) != nil {
			continue
		}
		hooksObj, _ := parsed["hooks"].(map[string]any)
		for _, ev := range []string{"PreToolUse", "PostToolUse"} {
			groups, _ := hooksObj[ev].([]any)
			for _, g := range groups {
				gm, _ := g.(map[string]any)
				if gm == nil {
					continue
				}
				m, _ := gm["matcher"].(string)
				if !matcherCoversBash(m) {
					continue
				}
				hs, _ := gm["hooks"].([]any)
				for _, h := range hs {
					hm, _ := h.(map[string]any)
					cmd, _ := hm["command"].(string)
					if w := commandFirstWord(cmd); w != "" && w != "fathom" {
						seen[w] = true
					}
				}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for w := range seen {
		out = append(out, w)
	}
	sort.Strings(out)
	return out
}

// matcherCoversBash is the client's matcher rule: empty and "*" match every
// tool, anything else is a regular expression over the tool name.
func matcherCoversBash(m string) bool {
	if m == "" || m == "*" {
		return true
	}
	re, err := regexp.Compile("^(?:" + m + ")$")
	return err == nil && re.MatchString("Bash")
}

// commandFirstWord is a hook command's program name: the first word past any
// leading NAME=value words, reduced to its base name so no path (and no
// environment value) is ever printed.
func commandFirstWord(cmd string) string {
	for _, w := range strings.Fields(cmd) {
		if i := strings.IndexByte(w, '='); i > 0 && !strings.ContainsAny(w[:i], "/") {
			continue
		}
		return filepath.Base(strings.Trim(w, `"'`))
	}
	return ""
}
