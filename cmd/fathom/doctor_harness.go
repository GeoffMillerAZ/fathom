package main

// doctor_harness.go — the `harness_hooks` row (HP-18/HP-19): for a
// non-Claude descriptor whose codec is wired, doctor READS the descriptor's
// own hook files and reports which fathom entries are present — as
// `skipped: unverified`, never PASS, because a present entry is not a
// firing hook on a harness no real session has verified here (Codex runs a
// non-managed hook only after /hooks trust; Gemini re-prompts on
// fingerprint change), and only hooks_firing beats from a real install can
// turn `unverified` into a dated verified_against (ADR-029 §5). A missing
// entry is a real finding (evidence, not a guess); an unreadable file is
// named, never counted as absent (I38).

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

// checkHarnessHookFiles is the harness_hooks row.
func checkHarnessHookFiles(root, harnessLabel string) doctorCheck {
	const name = "harness_hooks"
	if harnessLabel == "claude" {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "Claude Code's hook files are covered by hooks_installed and hooks_firing"}
	}
	d, ok := harness.Lookup(doctorHarnessID(harnessLabel))
	if !ok {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("no registry descriptor for %q — nothing to read", harnessLabel)}
	}
	if !d.HooksWired() {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: hooksNotWiredReason(harnessLabel, fmt.Sprintf("%s documents %d hook binding(s) this build does not wire (data row, no codec) — nothing to read", d.Display, len(d.Hooks)))}
	}
	specs := hookSpecsFor(d.ID)
	home, _ := os.UserHomeDir()
	present := map[hookSpecKey][]string{}
	var fileNotes []string
	for _, f := range d.HookFiles {
		if f.Kind == "" {
			continue // documented, not addressable by an editor — nothing to read
		}
		path := f.Path
		if strings.HasPrefix(path, "~/") {
			if home == "" {
				fileNotes = append(fileNotes, fmt.Sprintf("%s: home directory unresolvable — not read", f.Path))
				continue
			}
			path = filepath.Join(home, path[2:])
		} else {
			path = filepath.Join(root, filepath.FromSlash(path))
		}
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue // absent is normal
			}
			fileNotes = append(fileNotes, fmt.Sprintf("%s exists but could not be read (%v) — entries there are invisible to this check", f.Path, err))
			continue
		}
		var parsed map[string]any
		if err := json.Unmarshal(data, &parsed); err != nil {
			fileNotes = append(fileNotes, fmt.Sprintf("%s did not parse as JSON (%v) — entries there are invisible to this check", f.Path, err))
			continue
		}
		hooksObj, _ := parsed["hooks"].(map[string]any)
		for _, spec := range specs {
			if hookMarkerPresent(hooksObj, spec) {
				k := hookSpecKeyOf(spec)
				present[k] = append(present[k], f.Scope+" "+f.Path)
			}
		}
	}
	var missing []string
	for _, spec := range specs {
		if len(present[hookSpecKeyOf(spec)]) == 0 {
			missing = append(missing, specLabel(spec))
		}
	}
	found := len(specs) - len(missing)
	initFile, _ := d.InitHookFile()
	reason := fmt.Sprintf("skipped: unverified — %d of %d fathom hook entries present for %s", found, len(specs), d.Display)
	if found > 0 && found == len(specs) {
		reason += fmt.Sprintf(" (all in %s)", initFile.Path)
	}
	if len(missing) > 0 {
		reason += "; missing: " + strings.Join(missing, ", ")
	}
	if len(fileNotes) > 0 {
		reason += "; " + strings.Join(fileNotes, "; ")
	}
	reason += "; a present entry is not a firing hook — the descriptor is unverified on this machine (only hooks_firing beats from a real " + d.Display + " session can verify it)"
	remedy := ""
	switch {
	case len(fileNotes) > 0 && found < len(specs):
		remedy = "make the named file readable/parsable, then re-run doctor; `fathom init --harness=" + d.ID + " --path " + root + "` adds the missing entries"
	case len(missing) > 0:
		remedy = "fathom init --harness=" + d.ID + " --path " + root
	case initFile.Trust != "":
		remedy = initFile.Trust
	}
	return doctorCheck{Name: name, Status: doctorSkipped, Reason: reason, Remedy: remedy}
}
