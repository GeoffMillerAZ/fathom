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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

// checkHarnessHookFiles is the harness_hooks row.
func checkHarnessHookFiles(root, harnessLabel string, parents ...context.Context) doctorCheck {
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
	present, fileNotes, _, _ := harnessHookEntriesByScope(root, d, specs, parents...)
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
	_, flat := harness.FlatHookShape(d)
	switch {
	case flat && len(missing) > 0:
		// GL-7/GL-8: init cannot write a flat hook file; the verb prints it.
		remedy = "fathom harness hooks --harness " + d.ID + " — merge the printed entries into the named file"
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
func harnessHookEntriesByScope(root string, d harness.Descriptor, specs []hookSpec, parents ...context.Context) (map[hookSpecKey][]string, []string, map[hookSpecKey]bool, bool) {
	return harnessHookEntriesByScopeMode(root, d, specs, false, parents...)
}

// includeCommands retains exact command identity for the duplicate-injection check.
func harnessHookEntriesByScopeMode(root string, d harness.Descriptor, specs []hookSpec, includeCommands bool, parents ...context.Context) (map[hookSpecKey][]string, []string, map[hookSpecKey]bool, bool) {
	ctx, cancel := doctorContextArg(parents)
	defer cancel()
	uncertain := map[hookSpecKey]bool{}
	indirect := false
	present := map[hookSpecKey][]string{}
	var fileNotes []string
	for _, f := range d.HookFiles {
		if f.Kind == "" && f.Scope == "project" && (f.Shape == "cursor-hooks" || f.Shape == "copilot-hooks") {
			// GL-7/GL-8: a flat hook file the user merged by hand is read
			// (never written): one entry per binding, no nested group.
			p, n := flatHookEntries(root, f, specs, includeCommands)
			for k, v := range p {
				present[k] = append(present[k], v...)
			}
			fileNotes = append(fileNotes, n...)
			continue
		}
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
			ev := doctorHookEvidence(ctx, root, hooksObj, spec)
			uncertain[hookSpecKeyOf(spec)] = uncertain[hookSpecKeyOf(spec)] || (ev.unknown && ev.missingScript == "")
			if !ev.found && ev.missingScript != "" {
				fileNotes = append(fileNotes, "hook script not found: "+ev.missingScript)
			}
			indirect = indirect || ev.indirect
			if ev.found {
				k := hookSpecKeyOf(spec)
				if !includeCommands {
					present[k] = append(present[k], f.Scope+" "+f.Path)
				} else {
					for _, command := range doctorHookCommands(ctx, root, hooksObj, spec) {
						present[k] = append(present[k], f.Scope+" "+f.Path+"\x00"+command)
					}
				}
			}
		}
	}
	return present, fileNotes, uncertain, indirect
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
	if d.ID == harness.ClaudeCode && strings.HasPrefix(descriptorPath, "~/.claude/") {
		base := harness.ClaudeConfigRoot(wiringDriftHome())
		if base == "" {
			return "", false
		}
		return filepath.Join(base, strings.TrimPrefix(descriptorPath, "~/.claude/")), true
	}

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

// flatHookEntries reads a project flat hook file (Cursor's
// .cursor/hooks.json, Copilot's .github/hooks/*.json — a glob) and
// reports which fathom specs it carries: an entry under the spec's event
// whose matcher equals the spec's and whose command (Cursor `command`,
// Copilot `bash` or `command`) contains the spec's marker.
func flatHookEntries(root string, f harness.HookFile, specs []hookSpec, includeCommands bool) (map[hookSpecKey][]string, []string) {
	present := map[hookSpecKey][]string{}
	var notes []string
	paths, _ := filepath.Glob(filepath.Join(root, filepath.FromSlash(f.Path)))
	sort.Strings(paths)
	for _, p := range paths {
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		data, err := os.ReadFile(p)
		if err != nil {
			notes = append(notes, fmt.Sprintf("%s exists but could not be read (%v) — entries there are invisible to this check", rel, err))
			continue
		}
		var parsed struct {
			Hooks map[string][]map[string]any `json:"hooks"`
		}
		if err := json.Unmarshal(data, &parsed); err != nil {
			notes = append(notes, fmt.Sprintf("%s did not parse as JSON (%v) — entries there are invisible to this check", rel, err))
			continue
		}
		for _, spec := range specs {
			for _, e := range parsed.Hooks[spec.event] {
				matcher, _ := e["matcher"].(string)
				if matcher != spec.matcher {
					continue
				}
				cmd, _ := e["command"].(string)
				if bash, _ := e["bash"].(string); bash != "" {
					cmd = bash
				}
				if strings.Contains(cmd, spec.marker) {
					k := hookSpecKeyOf(spec)
					scope := f.Scope + " " + rel
					if includeCommands {
						scope += "\x00" + cmd
					}
					present[k] = append(present[k], scope)
					if !includeCommands {
						break
					}
				}
			}
		}
	}
	return present, notes
}

// Isolate one command at a time so later rows sharing an event and matcher
// are compared too. Static wrapper inspection is the same as the wiring check.
func doctorHookCommands(ctx context.Context, root string, hooksObj map[string]any, spec hookSpec) []string {
	var commands []string
	groups, _ := hooksObj[spec.event].([]any)
	for _, entry := range groups {
		group, _ := entry.(map[string]any)
		matcher, _ := group["matcher"].(string)
		if matcher != spec.matcher {
			continue
		}
		hooks, _ := group["hooks"].([]any)
		for _, h := range hooks {
			hook, _ := h.(map[string]any)
			command, _ := hook["command"].(string)
			one := map[string]any{spec.event: []any{map[string]any{"matcher": matcher, "hooks": []any{h}}}}
			if doctorHookEvidence(ctx, root, one, spec).found {
				commands = append(commands, command)
			}
		}
	}
	return commands
}
