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

	skillsdist "github.com/geoffmilleraz/fathom/dist/agents/skills"
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
	Repo          string   `json:"repo"`
	Service       string   `json:"service,omitempty"`        // the registry's id, when known
	Missing       []string `json:"missing,omitempty"`        // template entries absent at every readable scope
	StaleCmds     []string `json:"stale_cmds,omitempty"`     // commands spelling an absolute fathom path
	MissingSkills []string `json:"missing_skills,omitempty"` // shipped skills absent from .claude/skills (FR-3)
	StanzaNote    string   `json:"stanza_note,omitempty"`    // pre-rename branding etc.
	StaleStanzas  []string `json:"stale_stanzas,omitempty"`  // marker-owned regions whose content != the current template
	SettingsNote  string   `json:"settings_note,omitempty"`  // unreadable / absent
}

// staleFathomPath matches an absolute path ending in /fathom (optionally
// bin/fathom) inside a hook command — the class every audit found: hooks
// pinned to a checkout's bin/ that drifts from the deployed binary.
var staleFathomPath = regexp.MustCompile(`(^|[\s"'=])(/[^\s"']*/fathom)([\s"']|$)`)

// analyzeWiring diffs one repo's installed hooks against fathomHookSpecs.
// User-scope entries count as present (they serve every repo), but stale
// paths are reported wherever they appear.
func analyzeWiring(root string) wiringDiff { return analyzeWiringFor(root, "") }

// analyzeWiringFor is analyzeWiring with the registry's service id, which
// the stanza comparison must use.
func analyzeWiringFor(root, service string) wiringDiff {
	d := wiringDiff{Repo: root, Service: service}
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

	// Shipped-skills check (FR-3, docs/plans/exploration-ledger.md): a repo
	// whose hooks are current can still fail sessions on Unknown-skill —
	// repos wired before a skill shipped never receive it. Presence is the
	// Claude shape (<root>/.claude/skills/<name>/SKILL.md); the Codex shape
	// is user-global and therefore not a per-repo drift signal.
	for _, name := range skillsdist.Names {
		if _, err := os.Stat(filepath.Join(root, ".claude", "skills", name, "SKILL.md")); err != nil {
			d.MissingSkills = append(d.MissingSkills, name)
		}
	}

	if cm, err := os.ReadFile(filepath.Join(root, "CLAUDE.md")); err == nil {
		low := strings.ToLower(string(cm))
		if strings.Contains(low, ".sextantignore") || strings.Contains(low, "sextant mcp") {
			d.StanzaNote = "CLAUDE.md carries pre-rename branding (sextant) — regenerate with fathom gen-claude-md"
		}
	}

	// Stanza CONTENT drift (found 2026-08-13): the template gained the
	// continuity/Codex line and reconcile reported "wiring current" for
	// every repo — it checked stanza PRESENCE, never content against the
	// current template. A template change that no reconcile can deliver is
	// a template change that never ships. Compared per-file (CLAUDE.md and
	// AGENTS.md independently), rendered with the REGISTRY's service id
	// (dirname only as a daemon-unreachable fallback) so a repo that chose
	// a different id is neither falsely flagged nor rewritten wrongly.
	d.StaleStanzas = staleStanzaFiles(root, d.Service)
	return d
}

// staleStanzaFiles reports which of the repo's stanza-carrying files hold a
// fathom-owned region whose content differs from the current template.
// Files without markers are NOT reported — wiring a never-wired repo is
// init's job; reconcile only refreshes what fathom already owns (law 15's
// boundary: presence of the markers IS the ownership claim).
func staleStanzaFiles(root, service string) []string {
	if service == "" {
		service = strings.ToLower(filepath.Base(root))
	}
	want := strings.TrimSpace(fathomStanza(service))
	var out []string
	for _, name := range []string{"CLAUDE.md", "AGENTS.md"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			continue
		}
		content := string(data)
		b := strings.Index(content, fathomStanzaBegin)
		e := strings.Index(content, fathomStanzaEnd)
		if b < 0 || e < 0 || e < b {
			continue // unwired or corrupted — not reconcile's to touch
		}
		got := strings.TrimSpace(content[b : e+len(fathomStanzaEnd)])
		if got != want {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
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
	// svcByRoot carries the REGISTRY's service id per root. Rendering the
	// stanza with an inferred dirname would rewrite a repo that chose a
	// different id with the WRONG name — the registry knows, so ask it.
	svcByRoot := map[string]string{}
	if len(roots) == 0 {
		hc := &http.Client{Timeout: 4 * time.Second}
		svcs, err := fetchServices(hc, *url)
		if err != nil {
			fmt.Fprintf(stderr, "error: no --repo given and service discovery failed: %v\n", err)
			return 1
		}
		for name, info := range svcs {
			if info.path != "" {
				if _, serr := os.Stat(info.path); serr == nil {
					roots = append(roots, info.path)
					svcByRoot[info.path] = strings.ToLower(name)
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
		d := analyzeWiringFor(root, svcByRoot[root])
		if len(d.Missing) == 0 && len(d.StaleCmds) == 0 && len(d.MissingSkills) == 0 && d.StanzaNote == "" && d.SettingsNote == "" && len(d.StaleStanzas) == 0 {
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
		if len(d.MissingSkills) > 0 {
			fmt.Fprintf(stdout, "  missing: %d shipped skill(s) for claude (Unknown-skill failures in sessions)\n", len(d.MissingSkills))
		}
		if d.StanzaNote != "" {
			fmt.Fprintf(stdout, "  stanza:  %s\n", d.StanzaNote)
		}
		if len(d.StaleStanzas) > 0 {
			fmt.Fprintf(stdout, "  stanza:  %s behind the current template (the owned region refreshes on --apply)\n", strings.Join(d.StaleStanzas, ", "))
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
		// Stanza refresh: applyStanza replaces ONLY the content between the
		// markers fathom owns, is idempotent, and refuses a corrupted
		// marker pair rather than compounding it — the law-15 boundary in
		// the mechanism itself.
		if len(d.StaleStanzas) > 0 {
			svc := d.Service
			if svc == "" {
				svc = strings.ToLower(filepath.Base(root))
			}
			stanza := fathomStanza(svc)
			for _, name := range d.StaleStanzas {
				if err := applyStanza(filepath.Join(root, name), stanza, stdout); err != nil {
					fmt.Fprintf(stdout, "  apply failed (stanza %s): %v\n", name, err)
				} else {
					fmt.Fprintf(stdout, "  applied: %s stanza refreshed to the current template\n", name)
				}
			}
		}
		// FR-3: missing shipped skills install through init's own
		// installSkills machinery (Claude shape only — the per-repo one),
		// which refuses to touch a same-named file fathom does not own.
		if len(d.MissingSkills) > 0 {
			if err := installSkills(root, "claude", stdout); err != nil {
				fmt.Fprintf(stdout, "  apply failed (skills): %v\n", err)
			} else {
				fmt.Fprintf(stdout, "  applied: %d missing shipped skill(s) installed (claude shape)\n", len(d.MissingSkills))
			}
		}
	}
	mode := "dry-run — pass --apply for the safe subset"
	if *apply {
		mode = "applied the safe subset"
	}
	// CC-10 (docs/plans/custodian.md): concern coverage is INFORMATIONAL,
	// never drift — a repo with no config contracts is normal; reconcile
	// just refuses to let the absence be invisible.
	uncovered := 0
	for _, root := range roots {
		if _, err := os.Stat(filepath.Join(root, ".fathom", "concerns.cue")); err != nil {
			uncovered++
		}
	}
	if uncovered > 0 {
		fmt.Fprintf(stdout, "concern coverage: %d repo(s) carry no .fathom/concerns.cue (fathom init scaffolds a commented template; not drift)\n", uncovered)
	}
	fmt.Fprintf(stdout, "reconcile: %d repo(s), %d drifted (%s)\n", len(roots), drifted, mode)
	return 0
}

// repeatedFlag collects a repeatable string flag.
type repeatedFlag []string

func (r *repeatedFlag) String() string     { return strings.Join(*r, ",") }
func (r *repeatedFlag) Set(v string) error { *r = append(*r, v); return nil }
