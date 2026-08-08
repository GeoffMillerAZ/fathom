package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// fathom fleet reconcile (vision 14 §7, plan IO-17/18): wiring is
// reconciled, not scaffolded — a drifted hook stack is a DIFF to apply,
// never a refusal to touch. Dry-run by default; --apply applies the SAFE
// subset only: repointing stale absolute binary paths to the shim-stable
// `fathom`, and adding missing template entries via init's own idempotent
// merge (which defers to user scope — one scope, never both). Structural
// surgery (scope moves, removals) stays a human decision.

// wiringDiff is one repo's drift against the current template.
type wiringDiff struct {
	Repo         string   `json:"repo"`
	Missing      []string `json:"missing,omitempty"`       // template entries absent at every readable scope
	StaleCmds    []string `json:"stale_cmds,omitempty"`    // commands spelling an absolute fathom path
	StanzaNote   string   `json:"stanza_note,omitempty"`   // pre-rename branding etc.
	SettingsNote string   `json:"settings_note,omitempty"` // unreadable / absent
}

// staleFathomPath matches an absolute path ending in /fathom (optionally
// bin/fathom) inside a hook command — the class every audit found: hooks
// pinned to a checkout's bin/ that drifts from the deployed binary.
var staleFathomPath = regexp.MustCompile(`(^|[\s"'=])(/[^\s"']*/fathom)([\s"']|$)`)

// analyzeWiring diffs one repo's installed hooks against fathomHookSpecs.
// User-scope entries count as present (they serve every repo), but stale
// paths are reported wherever they appear.
func analyzeWiring(root string) wiringDiff {
	d := wiringDiff{Repo: root}
	scopes := [][2]string{{"project", filepath.Join(root, ".claude", "settings.json")}}
	if home, err := os.UserHomeDir(); err == nil {
		scopes = append(scopes, [2]string{"user", filepath.Join(home, ".claude", "settings.json")})
	}

	type installed struct{ present bool }
	have := map[string]*installed{}
	for i := range fathomHookSpecs {
		s := fathomHookSpecs[i]
		have[s.event+"|"+s.matcher] = &installed{}
	}
	readable := false
	for _, sc := range scopes {
		data, err := os.ReadFile(sc[1])
		if err != nil {
			continue
		}
		readable = true
		var rootObj map[string]any
		if json.Unmarshal(data, &rootObj) != nil {
			d.SettingsNote = sc[0] + " settings.json unparsable"
			continue
		}
		hooksObj, _ := rootObj["hooks"].(map[string]any)
		for _, spec := range fathomHookSpecs {
			key := spec.event + "|" + spec.matcher
			arr, _ := hooksObj[spec.event].([]any)
			for _, g := range arr {
				gm, ok := g.(map[string]any)
				if !ok {
					continue
				}
				gMatcher, _ := gm["matcher"].(string)
				if gMatcher != spec.matcher {
					continue
				}
				hs, _ := gm["hooks"].([]any)
				for _, h := range hs {
					hm, ok := h.(map[string]any)
					if !ok {
						continue
					}
					cmdStr, _ := hm["command"].(string)
					if strings.Contains(cmdStr, spec.marker) {
						have[key].present = true
						if m := staleFathomPath.FindStringSubmatch(cmdStr); m != nil {
							d.StaleCmds = append(d.StaleCmds, fmt.Sprintf("%s scope %s[%s]: %s", sc[0], spec.event, spec.matcher, cmdStr))
						}
					}
				}
			}
		}
	}
	if !readable {
		d.SettingsNote = "no readable settings.json at project or user scope"
	}
	for _, spec := range fathomHookSpecs {
		if !have[spec.event+"|"+spec.matcher].present {
			d.Missing = append(d.Missing, spec.event+"["+spec.matcher+"] → fathom "+spec.sub)
		}
	}
	sort.Strings(d.Missing)
	sort.Strings(d.StaleCmds)

	if cm, err := os.ReadFile(filepath.Join(root, "CLAUDE.md")); err == nil {
		low := strings.ToLower(string(cm))
		if strings.Contains(low, ".sextantignore") || strings.Contains(low, "sextant mcp") {
			d.StanzaNote = "CLAUDE.md carries pre-rename branding (sextant) — regenerate with fathom gen-claude-md"
		}
	}
	return d
}

// repointStaleCommands rewrites absolute fathom paths to the bare `fathom`
// (the shim) inside a settings file's fathom-marked hook commands. Returns
// the rewritten bytes and how many commands changed. Only command strings
// containing "fathom" are touched — nothing else in the file is rewritten.
func repointStaleCommands(data []byte) ([]byte, int, error) {
	var rootObj map[string]any
	if err := json.Unmarshal(data, &rootObj); err != nil {
		return nil, 0, err
	}
	changed := 0
	hooksObj, _ := rootObj["hooks"].(map[string]any)
	for _, arrAny := range hooksObj {
		arr, _ := arrAny.([]any)
		for _, g := range arr {
			gm, ok := g.(map[string]any)
			if !ok {
				continue
			}
			hs, _ := gm["hooks"].([]any)
			for _, h := range hs {
				hm, ok := h.(map[string]any)
				if !ok {
					continue
				}
				cmdStr, _ := hm["command"].(string)
				if !strings.Contains(cmdStr, "fathom") {
					continue
				}
				next := staleFathomPath.ReplaceAllString(cmdStr, "${1}fathom${3}")
				if next != cmdStr {
					hm["command"] = next
					changed++
				}
			}
		}
	}
	if changed == 0 {
		return data, 0, nil
	}
	out, err := json.MarshalIndent(rootObj, "", "  ")
	if err != nil {
		return nil, 0, err
	}
	return append(out, '\n'), changed, nil
}

func cmdFleetReconcile(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("fleet reconcile", stderr)
	apply := fs.Bool("apply", false, "apply the safe subset (shim repoint + additive template merge); dry-run otherwise")
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL (repo discovery)")
	var repos repeatedFlag
	fs.Var(&repos, "repo", "repo root to reconcile (repeatable; default: every daemon service with a local path)")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	roots := []string(repos)
	if len(roots) == 0 {
		hc := &http.Client{Timeout: 4 * time.Second}
		svcs, err := fetchServices(hc, *url)
		if err != nil {
			fmt.Fprintf(stderr, "error: no --repo given and service discovery failed: %v\n", err)
			return 1
		}
		for _, info := range svcs {
			if info.path != "" {
				if _, serr := os.Stat(info.path); serr == nil {
					roots = append(roots, info.path)
				}
			}
		}
		sort.Strings(roots)
	}
	if len(roots) == 0 {
		fmt.Fprintln(stderr, "error: nothing to reconcile — no local repo roots found")
		return 1
	}

	if msg, err := installShim(); err != nil {
		fmt.Fprintf(stdout, "shim: %v\n", err)
	} else {
		fmt.Fprintf(stdout, "%s\n", msg)
	}

	drifted := 0
	for _, root := range roots {
		d := analyzeWiring(root)
		if len(d.Missing) == 0 && len(d.StaleCmds) == 0 && d.StanzaNote == "" && d.SettingsNote == "" {
			fmt.Fprintf(stdout, "%s: wiring current\n", root)
			continue
		}
		drifted++
		fmt.Fprintf(stdout, "%s:\n", root)
		for _, m := range d.Missing {
			fmt.Fprintf(stdout, "  missing: %s\n", m)
		}
		for _, s := range d.StaleCmds {
			fmt.Fprintf(stdout, "  stale:   %s\n", s)
		}
		if d.StanzaNote != "" {
			fmt.Fprintf(stdout, "  stanza:  %s\n", d.StanzaNote)
		}
		if d.SettingsNote != "" {
			fmt.Fprintf(stdout, "  note:    %s\n", d.SettingsNote)
		}

		if !*apply {
			continue
		}
		// Safe subset, in order: repoint stale commands in the PROJECT
		// file (user scope belongs to the user — reported, never edited),
		// then the additive template merge via init's own idempotent
		// machinery (which defers to user scope; one scope, never both).
		settingsPath := filepath.Join(root, ".claude", "settings.json")
		if data, err := os.ReadFile(settingsPath); err == nil {
			if next, n, rerr := repointStaleCommands(data); rerr == nil && n > 0 {
				if werr := os.WriteFile(settingsPath, next, 0o644); werr == nil {
					fmt.Fprintf(stdout, "  applied: %d command(s) repointed to the shim\n", n)
				}
			}
		}
		if len(d.Missing) > 0 {
			if err := wireHooks(root, "fathom", stdout); err != nil {
				fmt.Fprintf(stdout, "  apply failed (hooks): %v\n", err)
			} else {
				fmt.Fprintf(stdout, "  applied: missing template entries merged (project scope, deferring to user scope)\n")
			}
		}
	}
	mode := "dry-run — pass --apply for the safe subset"
	if *apply {
		mode = "applied the safe subset"
	}
	fmt.Fprintf(stdout, "reconcile: %d repo(s), %d drifted (%s)\n", len(roots), drifted, mode)
	return 0
}

// repeatedFlag collects a repeatable string flag.
type repeatedFlag []string

func (r *repeatedFlag) String() string     { return strings.Join(*r, ",") }
func (r *repeatedFlag) Set(v string) error { *r = append(*r, v); return nil }
