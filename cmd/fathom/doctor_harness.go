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
	// HP-34: once a real session has verified a descriptor, its hook files
	// are covered by the same two rows Claude Code's are — this row exists
	// for the UNVERIFIED case, where a present entry may not be reported
	// as a pass.
	if d, ok := descriptorHooksVerifiable(harnessLabel); ok {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: d.Display + "'s hook files are covered by hooks_installed and hooks_firing (the descriptor is verified: " + d.VerifiedAgainst + ")"}
	}
	d, ok := harness.Lookup(doctorHarnessID(harnessLabel))
	if !ok {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("no registry descriptor for %q — nothing to read", harnessLabel)}
	}
	if !d.HooksWired() {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: hooksNotWiredReason(harnessLabel, fmt.Sprintf("%s documents %d hook binding(s) this build does not wire (data row, no codec) — nothing to read", d.Display, len(d.Hooks)))}
	}
	specs := hookSpecsFor(d.ID)
	present, fileNotes := harnessHookEntriesByScope(root, d, specs)
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

// harnessHookEntriesByScope reads every hook file a descriptor documents
// AND an editor can address, and reports which fathom specs are present
// in which scope. Shared by the unverified `harness_hooks` row above and
// the verified `hooks_installed` row (doctor_harness_firing.go) so the two
// can never disagree about what is on disk.
//
// Absence is normal and silent; an unreadable or unparsable file is NAMED
// in the returned notes and never counted as absence (I38 — an unreadable
// file once made a whole audit report zero findings).
func harnessHookEntriesByScope(root string, d harness.Descriptor, specs []hookSpec) (map[hookSpecKey][]string, []string) {
	present := map[hookSpecKey][]string{}
	var fileNotes []string
	for _, f := range d.HookFiles {
		if f.Kind == "" {
			continue // documented, not addressable by an editor — nothing to read
		}
		path := f.Path
		if strings.HasPrefix(path, "~/") {
			resolved, ok := harnessUserPath(d, f.Path)
			if !ok {
				fileNotes = append(fileNotes, fmt.Sprintf("%s: home directory unresolvable — not read", f.Path))
				continue
			}
			path = resolved
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
	return present, fileNotes
}

// harnessUserPath resolves a "~/"-prefixed descriptor path, honoring the
// descriptor's OWN home variable first (Codex: CODEX_HOME, which
// relocates the whole ~/.codex tree including hooks.json). Without this,
// doctor reads a file the running harness is not using — the class of
// wrong-repo/wrong-file reading O23 already cost this project once.
//
// The relocation replaces the harness's home DIRECTORY, so a descriptor
// path of "~/.codex/hooks.json" under CODEX_HOME=/tmp/ch resolves to
// /tmp/ch/hooks.json, not /tmp/ch/.codex/hooks.json.
func harnessUserPath(d harness.Descriptor, descriptorPath string) (string, bool) {
	rel := strings.TrimPrefix(descriptorPath, "~/")
	if d.Env.Home != "" {
		if v := os.Getenv(d.Env.Home); v != "" {
			// Drop the leading dot-directory the descriptor spells under the
			// user's home: the env var names that directory outright.
			if i := strings.IndexByte(rel, '/'); i >= 0 && strings.HasPrefix(rel, ".") {
				rel = rel[i+1:]
			}
			return filepath.Join(v, filepath.FromSlash(rel)), true
		}
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", false
	}
	return filepath.Join(home, filepath.FromSlash(rel)), true
}
