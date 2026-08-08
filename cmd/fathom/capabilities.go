package main

// capabilities.go — CR-5: `fathom capabilities`, the verb behind phase T
// (docs/design/capability-registry.md).
//
// The verb answers one question that nothing could answer before: "why
// isn't X happening here?" Every off capability arrives with the
// observation that makes it off and either a command or an explicit
// nothing-to-run. Law 10: the UI and doctor render what this returns; they
// never learn anything it does not know.

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/capability"
)

func cmdCapabilities(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "explain":
			return cmdCapabilitiesExplain(args[1:], stdout, stderr)
		case "set":
			return cmdCapabilitiesSet(args[1:], stdout, stderr)
		}
	}
	return cmdCapabilitiesList(args, stdout, stderr)
}

func cmdCapabilitiesList(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("capabilities", stderr)
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL")
	repo := fs.String("repo", ".", "repository root")
	svc := fs.String("service", "", "service ID (default: resolved from the repo path)")
	asJSON := fs.Bool("json", false, "emit JSON instead of a table")
	showCost := fs.Bool("cost", false, "show what each capability costs when on")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	root, err := filepath.Abs(*repo)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	reg := capability.Default()
	probe := buildCapabilityProbe(*url, root, *svc, time.Now())
	res, cfgErrs := loadCapabilityConfig(reg, root)

	// A broken config file is reported and then IGNORED for the rest of
	// the run: the user still gets their table, and the errors are loud
	// above it. Refusing to print anything because one line is wrong helps
	// nobody diagnose the line.
	for _, e := range cfgErrs {
		fmt.Fprintf(stderr, "config: %v\n", e)
	}

	reports := reg.DetectAll(probe, res)

	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(capabilityJSON(probe, reports)); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	}

	operating := 0
	for _, r := range reports {
		if r.State.Operating {
			operating++
		}
	}
	fmt.Fprintf(stdout, "capabilities in %s (%d of %d operating)\n\n", probeTarget(probe), operating, len(reports))
	for _, r := range reports {
		mark := "✗"
		if r.State.Operating {
			mark = "✓"
		}
		fmt.Fprintf(stdout, "  %s %-15s %s\n", mark, r.Capability.ID, capabilityLine(r))
		if !r.State.Operating {
			if r.Remedy.Cmd != "" {
				fmt.Fprintf(stdout, "    %-15s → %s\n", "", r.Remedy.Cmd)
			}
			if r.Remedy.Note != "" {
				fmt.Fprintf(stdout, "    %-15s   %s\n", "", r.Remedy.Note)
			}
		}
		if *showCost {
			fmt.Fprintf(stdout, "    %-15s   cost: %s\n", "", r.Capability.Cost)
		}
	}
	fmt.Fprintln(stdout, "\n  fathom capabilities explain <id>   what it does and what it costs")
	return 0
}

// capabilityLine is the one-line status: the evidence when operating, the
// reason when not, with the scope named whenever it is not the built-in
// default — "off (your user default)" and "off (this repo)" are different
// facts and a reader debugging their setup needs both.
func capabilityLine(r capability.Report) string {
	if r.State.Operating {
		// Provenance is only meaningful for a real switch; printing
		// "[on: project]" beside a status entry implies a control that
		// does not exist.
		if r.Capability.Switchable && r.Scope != capability.ScopeBuiltIn {
			return r.State.Detail + "  [on: " + string(r.Scope) + "]"
		}
		return r.State.Detail
	}
	return r.State.Reason
}

func cmdCapabilitiesExplain(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("capabilities explain", stderr)
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL")
	repo := fs.String("repo", ".", "repository root")
	svc := fs.String("service", "", "service ID")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: fathom capabilities explain <id>")
		return 1
	}
	id := fs.Arg(0)
	reg := capability.Default()
	c, ok := reg.Get(id)
	if !ok {
		if errs := reg.ValidateSettings(capability.Settings{id: true}, capability.ScopeProject); len(errs) > 0 {
			fmt.Fprintf(stderr, "error: %v\n", errs[0])
		}
		return 1
	}

	root, _ := filepath.Abs(*repo)
	probe := buildCapabilityProbe(*url, root, *svc, time.Now())
	res, _ := loadCapabilityConfig(reg, root)
	setting := res.For(c)

	fmt.Fprintf(stdout, "%s — %s seam\n\n", c.ID, c.Seam)
	fmt.Fprintf(stdout, "  %s\n\n", wrapAt(c.Summary, 72, "  "))
	fmt.Fprintf(stdout, "  Costs when on:  %s\n", c.Cost)
	fmt.Fprintf(stdout, "  Default:        %s\n", onOff(c.Default))
	if c.Switchable {
		fmt.Fprintf(stdout, "  Settable by:    %s\n", scopeRuleText(c.Scopes))
	} else {
		fmt.Fprintf(stdout, "  Settable by:    nothing — this reports status rather than offering a switch.\n")
		fmt.Fprintf(stdout, "                  %s\n", wrapAt(c.NotSwitchableBecause, 60, "                  "))
	}
	fmt.Fprintf(stdout, "  Currently:      %s (from %s)\n", onOff(setting.On), setting.Scope)

	st := c.Detect(probe)
	if st.Operating {
		fmt.Fprintf(stdout, "  Observed:       operating — %s\n", st.Detail)
	} else {
		fmt.Fprintf(stdout, "  Observed:       NOT operating — %s\n", st.Reason)
		rem := c.Remedy(probe)
		if rem.Cmd != "" {
			fmt.Fprintf(stdout, "  To turn it on:  %s\n", rem.Cmd)
		}
		if rem.Note != "" {
			fmt.Fprintf(stdout, "                  %s\n", rem.Note)
		}
	}
	fmt.Fprintf(stdout, "\n  Evidence it produces: %s\n", capabilityEvidence(c.ID))
	return 0
}

// capabilityEvidence connects a capability to the number it moves on the
// dashboard. It is the sentence that makes configuration feel worth doing:
// turning this on is not tidiness, it changes a figure a reader can see.
func capabilityEvidence(id string) string {
	switch id {
	case "index":
		return "symbol counts and discovery actions on /observatory"
	case "obligations":
		return "\"rule injections before an edit\" in `fathom brief` and on /observatory/impact"
	case "guards":
		return "guard fires in the interventions table"
	case "modules":
		return "module-served answers in the impact model"
	case "layers":
		return "compiled claims per layer on /statusz"
	case "verify-gate":
		return "\"plan claims caught before they were believed\" in `fathom brief`"
	case "agent-surface":
		return "the standing schema tax in `fathom brief`, and per-tool traffic on /observatory/impact"
	case "session-ledger":
		return "every denominator in `fathom brief` — without it, no figure has a session count"
	case "narrate":
		return "story rows under the lens banner (a lens, never quoted as proof — law 14)"
	}
	return "not yet connected to a dashboard figure"
}

func cmdCapabilitiesSet(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("capabilities set", stderr)
	repo := fs.String("repo", ".", "repository root")
	scope := fs.String("scope", "project", "which scope to write: project|user")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() != 2 {
		fmt.Fprintln(stderr, "usage: fathom capabilities set <id> <on|off> [--scope project|user]")
		return 1
	}
	id, val := fs.Arg(0), fs.Arg(1)
	var on bool
	switch val {
	case "on", "true":
		on = true
	case "off", "false":
		on = false
	default:
		fmt.Fprintf(stderr, "error: value must be on or off, got %q\n", val)
		return 1
	}

	reg := capability.Default()
	// A status-only capability has no switch to write. Accepting the
	// setting and then reporting "switched off" while nothing changed is
	// the exact defect this field was added to remove.
	if c, ok := reg.Get(id); ok && !c.Switchable {
		fmt.Fprintf(stderr, "error: %q is not a switch — %s\n", id, c.NotSwitchableBecause)
		fmt.Fprintf(stderr, "       run `fathom capabilities explain %s` for what does control it\n", id)
		return 1
	}
	target := capability.ScopeProject
	if *scope == "user" {
		target = capability.ScopeUser
	}
	// Refuse before writing: a config file that fails to load on the next
	// read is a worse outcome than a refused command.
	if errs := reg.ValidateSettings(capability.Settings{id: on}, target); len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintf(stderr, "error: %v\n", e)
		}
		return 1
	}

	path, err := capabilityConfigPath(*repo, target)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	existing, _ := reg.LoadSettings(path, target)
	if existing == nil {
		existing = capability.Settings{}
	}
	existing[id] = on
	if err := writeCapabilitySettings(path, reg, existing); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "%s: %s (%s scope) — written to %s\n", id, onOff(on), target, path)
	return 0
}

func capabilityConfigPath(repo string, scope capability.Scope) (string, error) {
	if scope == capability.ScopeUser {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot resolve home dir: %w", err)
		}
		return filepath.Join(home, ".fathom", capability.UserFileName), nil
	}
	root, err := filepath.Abs(repo)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, ".fathom", capability.ProjectFileName), nil
}

// writeCapabilitySettings rewrites the whole settings block. The file is
// fathom's own (tier 1 ownership, docs/design/self-healing-fleet.md §3), so
// a full rewrite is legitimate here — unlike CLAUDE.md, nothing else lives
// in it.
func writeCapabilitySettings(path string, reg capability.Registry, s capability.Settings) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	ids := make([]string, 0, len(s))
	for id := range s {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var b strings.Builder
	b.WriteString("// Written by `fathom capabilities set`. Safe to hand-edit.\n")
	b.WriteString("// See `fathom capabilities` for what is on and why.\n\n")
	b.WriteString("capabilities: {\n")
	for _, id := range ids {
		summary := ""
		if c, ok := reg.Get(id); ok {
			summary = "  // " + firstClause(c.Summary)
		}
		fmt.Fprintf(&b, "\t%s: on: %-5t%s\n", id, s[id], summary)
	}
	b.WriteString("}\n")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// loadCapabilityConfig reads both scopes, returning the resolution plus any
// load errors. A broken file yields no settings for that scope and a loud
// error — never a silent partial read.
func loadCapabilityConfig(reg capability.Registry, root string) (capability.Resolution, []error) {
	var errs []error
	res := capability.Resolution{}
	if home, err := os.UserHomeDir(); err == nil {
		p := filepath.Join(home, ".fathom", capability.UserFileName)
		s, lerr := reg.LoadSettings(p, capability.ScopeUser)
		if lerr != nil {
			errs = append(errs, lerr)
		} else {
			res.User = s
		}
	}
	p := filepath.Join(root, ".fathom", capability.ProjectFileName)
	s, lerr := reg.LoadSettings(p, capability.ScopeProject)
	if lerr != nil {
		errs = append(errs, lerr)
	} else {
		res.Project = s
	}
	return res, errs
}

// capabilityJSON is the model the observatory and doctor render. Law 10:
// one shape, many surfaces.
type capabilityRow struct {
	ID          string `json:"id"`
	Seam        string `json:"seam"`
	Summary     string `json:"summary"`
	Cost        string `json:"cost"`
	Switchable  bool   `json:"switchable"`
	WhyNoSwitch string `json:"why_no_switch,omitempty"`
	Enabled     bool   `json:"enabled"`
	Scope       string `json:"scope"`
	Operating   bool   `json:"operating"`
	Detail      string `json:"detail,omitempty"`
	Reason      string `json:"reason,omitempty"`
	Cmd         string `json:"remedy_cmd,omitempty"`
	Note        string `json:"remedy_note,omitempty"`
}

type capabilityModel struct {
	Service      string          `json:"service,omitempty"`
	RepoRoot     string          `json:"repo_root"`
	Operating    int             `json:"operating"`
	Total        int             `json:"total"`
	Capabilities []capabilityRow `json:"capabilities"`
}

func capabilityJSON(p capability.Probe, reports []capability.Report) capabilityModel {
	m := capabilityModel{Service: p.Service, RepoRoot: p.RepoRoot, Total: len(reports)}
	for _, r := range reports {
		if r.State.Operating {
			m.Operating++
		}
		m.Capabilities = append(m.Capabilities, capabilityRow{
			ID:          r.Capability.ID,
			Seam:        string(r.Capability.Seam),
			Summary:     r.Capability.Summary,
			Cost:        r.Capability.Cost,
			Switchable:  r.Capability.Switchable,
			WhyNoSwitch: r.Capability.NotSwitchableBecause,
			Enabled:     r.Enabled,
			Scope:       string(r.Scope),
			Operating:   r.State.Operating,
			Detail:      r.State.Detail,
			Reason:      r.State.Reason,
			Cmd:         r.Remedy.Cmd,
			Note:        r.Remedy.Note,
		})
	}
	return m
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

func scopeRuleText(r capability.ScopeRule) string {
	if r == capability.ScopeProjectOnly {
		return "this repository only — it governs behaviour rather than expressing a preference (law 13)"
	}
	return "user scope (as a default) or this repository (governing)"
}

func firstClause(s string) string {
	for _, sep := range []string{" — ", ". ", ", so "} {
		if i := strings.Index(s, sep); i > 0 {
			return s[:i]
		}
	}
	return s
}

// wrapAt is a minimal word wrapper for the explain output.
func wrapAt(s string, width int, indent string) string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return ""
	}
	var b strings.Builder
	lineLen := 0
	for i, w := range words {
		if i > 0 {
			if lineLen+1+len(w) > width {
				b.WriteString("\n" + indent)
				lineLen = 0
			} else {
				b.WriteString(" ")
				lineLen++
			}
		}
		b.WriteString(w)
		lineLen += len(w)
	}
	return b.String()
}
