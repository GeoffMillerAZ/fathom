package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/bindspec"
	"github.com/geoffmilleraz/fathom/internal/capability"
	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/guardspec"

	"github.com/geoffmilleraz/fathom/internal/buildinfo"
)

// cmdInit implements the "init" subcommand: one-command, idempotent,
// additive-only wiring of a repo into fathom's coding-agent story —
// .fathomignore, a daemon ingest, Claude Code project hooks, the
// CLAUDE.md/AGENTS.md operating-contract stanza, and (opt-in,
// --skills[=claude|codex|all]) the shipped fathom-plan/verify/onboard/
// feedback agent skills (AK-1/AK-2, docs/plans/pack-taxonomy-adoption.md).
// See docs/plans/init-and-adoption.md IA-1/IA-2/IA-3 and
// docs/design/coding-agent-excellence.md §5.5.
//
// Every mutation is create-if-absent or marker-replace, so re-running init
// on an already-wired repo changes nothing: every step prints "ok X (already
// present)" instead of "created X", and no file's bytes change.
//
// Exit codes:
//
//	0 — fully wired AND the daemon confirms the service is indexed
//	2 — wiring is complete but the daemon is unreachable, or reachable but
//	    the service still has no symbols (operational; mirrors verify's
//	    fail-open §5.3 doctrine — init must never look "broken" for a reason
//	    that has nothing to do with the files it wrote)
//	1 — never returned for an operational reason EXCEPT a --skills marker
//	    conflict: a file already exists at a skill's target path and is not
//	    fathom-owned (no fathom:owned marker) — a real, non-operational
//	    problem the caller must resolve (rename/remove the file, or pick a
//	    different skill name), so init refuses loudly rather than fail-open
//	    or silently clobbering a human-authored file.
func cmdInit(args []string, stdout, stderr io.Writer) int {
	rpcOrigin = "init"
	fs := newFlagSet("init", stderr)
	path := fs.String("path", "", "repo root to wire (default: cwd)")
	service := fs.String("service", "", "service ID (default: resolved from --path, the same identity the hooks use)")
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL")
	binOverride := fs.String("bin", "", "fathom binary path baked into hook commands (default: os.Executable, symlink-resolved)")
	var skillsFlag optionalStringFlag
	fs.Var(&skillsFlag, "skills", "install shipped agent skills (fathom-plan/verify/onboard/feedback): claude | codex | all (bare --skills = all; omit to skip)")
	seedUserIgnoreFlag := fs.Bool("seed-user-ignore", false, "seed ~/.fathom/ignore with machine-junk defaults (docs/design/scoped-config.md); prints exactly what it writes, never silent, requires this flag")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	// Refuse positionals loudly. init takes none — and because --skills is
	// bool-shaped (bracket-optional), the natural spelling `--skills claude`
	// parses bare --skills as "all", STOPS flag parsing at the positional,
	// and would otherwise silently discard "claude" (and every later flag,
	// e.g. a following --path — scaffolding the wrong directory). Erroring
	// here, before any writes, turns that first-contact trap into an
	// immediate, self-explaining refusal.
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "error: unexpected argument %q — did you mean --skills=%s?\n", fs.Arg(0), fs.Arg(0))
		return 1
	}
	skillsMode, err := resolveSkillsMode(skillsFlag.value, skillsFlag.set)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	root := *path
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "error: resolving cwd: %v\n", err)
			return 1
		}
		root = wd
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		fmt.Fprintf(stderr, "error: resolving --path %q: %v\n", root, err)
		return 1
	}
	root = absRoot

	svc := strings.ToLower(*service)
	if svc == "" {
		// The daemon's recorded corpus paths are the authoritative
		// repo↔service identity (dogfood O15: the dir-basename guess wrote
		// a stanza naming a nonexistent service on the very repo that
		// builds fathom). Fail-open to the basename when the daemon is
		// unreachable or records no match.
		cands := candidatesWithDaemon(&http.Client{Timeout: 2 * time.Second}, *url, root)
		svc = strings.ToLower(cands[0].name)
	}

	binPath := *binOverride
	if binPath == "" {
		binPath, err = resolveBinPath()
		if err != nil {
			fmt.Fprintf(stderr, "error: resolving fathom binary path (pass --bin to override): %v\n", err)
			return 1
		}
	}

	fmt.Fprintf(stdout, "fathom init — service %q, root %s\n", svc, root)

	// a0. auto-narrate (IQ-10): an adoption's FIRST sessions are the ones
	// worth capturing, and nobody remembers to arm the dial by hand. Bounded
	// and loudly announced; an already-active window is left alone.
	armAutoNarrate(stdout)

	// a. .fathomignore
	if err := writeFathomIgnore(root, stdout); err != nil {
		fmt.Fprintf(stderr, "error: writing .fathomignore: %v\n", err)
		return 1
	}

	// b. daemon ingest (fail-soft: wiring continues either way).
	exitCode := 0
	indexed, ingestErr := initIngest(*url, svc, root, stdout)
	if ingestErr != nil {
		printDaemonDownInstructions(stdout, *url)
		exitCode = 2
	} else if !indexed {
		exitCode = 2
	}

	// c. project-level Claude Code hooks.
	if err := wireHooks(root, binPath, stdout); err != nil {
		fmt.Fprintf(stderr, "error: writing .claude/settings.json: %v\n", err)
		return 1
	}

	// c2. .fathom/ commented scaffolds for phase-O config surfaces (guard.cue,
	// modules.cue) — write-if-absent, ALWAYS commented (no active config by
	// default): binding a module or guarding a scope is an explicit,
	// reviewed opt-in, never something `fathom init` activates on a repo's
	// behalf (docs/design/platform-modules.md / guarded-edits.md deliverable 1).
	if err := writeFathomConfigScaffolds(root, stdout); err != nil {
		fmt.Fprintf(stderr, "error: writing .fathom/ scaffolds: %v\n", err)
		return 1
	}

	// d. CLAUDE.md + AGENTS.md stanza — one canonical source, two files.
	stanza := fathomStanza(svc)
	if err := applyStanza(filepath.Join(root, "CLAUDE.md"), stanza, stdout); err != nil {
		fmt.Fprintf(stderr, "error: writing CLAUDE.md: %v\n", err)
		return 1
	}
	if err := applyStanza(filepath.Join(root, "AGENTS.md"), stanza, stdout); err != nil {
		fmt.Fprintf(stderr, "error: writing AGENTS.md: %v\n", err)
		return 1
	}

	// e. ~/.fathom/config.toml scaffold (CF-8, docs/plans/config-experience.md):
	// write-if-absent, never overwrites an operator's existing file.
	if err := writeStarterConfig(config.DefaultUserPath(), stdout); err != nil {
		fmt.Fprintf(stderr, "error: writing config.toml: %v\n", err)
		return 1
	}

	// e2. ~/.fathom/ignore machine-junk seed (docs/design/scoped-config.md
	// deliverable 4) — --seed-user-ignore ONLY: init never touches
	// user-scope config on its own. Prints exactly what it writes.
	if *seedUserIgnoreFlag {
		if err := seedUserIgnoreFile(stdout); err != nil {
			fmt.Fprintf(stderr, "error: seeding ~/.fathom/ignore: %v\n", err)
			return 1
		}
	}

	// f. shipped agent skills (AK-2, docs/plans/pack-taxonomy-adoption.md):
	// --skills[=claude|codex|all] only — omitted by default, so a caller who
	// never asked for skills sees no new files.
	if err := installSkills(root, skillsMode, stdout); err != nil {
		fmt.Fprintf(stderr, "error: installing skills: %v\n", err)
		return 1
	}

	// g. manual steps init cannot do for you.
	// IO-20: the PATH shim — hooks and stanzas spell `fathom`, never a
	// checkout path; a gate that cannot be invoked must fail loudly, not
	// pass silently (the 2026-07-31 journal case).
	if msg, err := installShim(); err != nil {
		fmt.Fprintf(stdout, "shim: NOT installed — %v (hooks written by this init still work via PATH once you install one)\n", err)
	} else {
		fmt.Fprintf(stdout, "%s\n", msg)
	}

	printManualSteps(stdout, svc, *url)
	if skillsMode == "" {
		fmt.Fprintln(stdout, "MANUAL: install fathom's shipped agent skills (fathom-plan/verify/onboard/feedback):")
		fmt.Fprintln(stdout, "MANUAL:   fathom init --skills[=claude|codex|all]")
	}

	return exitCode
}

// resolveBinPath returns the current binary's path, resolving symlinks so
// the command baked into .claude/settings.json survives a `bin/fathom`
// symlink or PATH shim. Falls back to the unresolved path if symlink
// evaluation fails (e.g. the binary was already removed).
func resolveBinPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		return resolved, nil
	}
	return exe, nil
}

// ─── a. .fathomignore ──────────────────────────────────────────────────────

var defaultFathomIgnore = []string{
	"workspace/",
	"bin/",
	"tmp/",
	".claude/",
	"node_modules/",
	"vendor/",
	// C-family build trees (docs/plans/cpp-toolchain.md CT-8): a configured
	// CMake/Ninja build directory holds copied headers and generated
	// sources that would double-index every symbol.
	"build/",
	"out/",
	"cmake-build-*/",
}

// writeFathomIgnore creates <root>/.fathomignore with sensible defaults if
// absent. It never touches an existing file — a repo may have deliberately
// customized its excludes.
func writeFathomIgnore(root string, stdout io.Writer) error {
	p := filepath.Join(root, ".fathomignore")
	if _, err := os.Stat(p); err == nil {
		fmt.Fprintln(stdout, "ok .fathomignore (already present)")
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	var sb strings.Builder
	sb.WriteString("# fathom index excludes (gitignore-style; see internal/filter).\n")
	sb.WriteString("# Generated by `fathom init` — edit freely, it will not be overwritten.\n")
	for _, line := range defaultFathomIgnore {
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	if err := os.WriteFile(p, []byte(sb.String()), 0o644); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "created .fathomignore")
	return nil
}

// ─── a2. ~/.fathom/ignore seed (docs/design/scoped-config.md deliverable 4) ─

// seedUserIgnoreFile writes ~/.fathom/ignore with connector's machine-junk
// defaults, ONLY when --seed-user-ignore was passed (cmdInit's caller) —
// `fathom init` never touches user-scope config on its own (design §5:
// "No implicit creation"). Write-if-absent, exactly like writeFathomIgnore;
// prints the exact content on write so nothing here is silent.
func seedUserIgnoreFile(stdout io.Writer) error {
	path := connector.UserIgnorePath()
	if path == "" {
		fmt.Fprintln(stdout, "skip ~/.fathom/ignore — could not resolve $HOME")
		return nil
	}
	if _, err := os.Stat(path); err == nil {
		fmt.Fprintln(stdout, "ok ~/.fathom/ignore (already present)")
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	var sb strings.Builder
	sb.WriteString("# fathom user-scope ignore defaults — machine junk, unioned additively\n")
	sb.WriteString("# with every repo's .fathomignore (docs/design/scoped-config.md).\n")
	sb.WriteString("# Generated by `fathom init --seed-user-ignore` — edit freely; re-includes\n")
	sb.WriteString("# ('!pattern') are refused loudly here (user scope may only ADD exclusions).\n")
	for _, line := range connector.DefaultUserIgnoreSeed {
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	content := sb.String()

	fmt.Fprintf(stdout, "will write %s:\n", path)
	fmt.Fprint(stdout, content)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "created %s\n", path)
	return nil
}

// ─── c2. .fathom/ config scaffolds (guard.cue, modules.cue, capabilities.cue) ──

// writeFathomConfigScaffolds creates <root>/.fathom/guard.cue,
// <root>/.fathom/modules.cue and <root>/.fathom/capabilities.cue
// with commented, inert example templates if
// absent — mirroring writeFathomIgnore's write-if-absent, never-overwrite
// discipline. Both files are valid, EMPTY CUE as scaffolded (guardspec/
// bindspec's ExampleTemplate — every worked example is a comment block), so
// `fathom doctor` sees no active guard/binding until a human uncomments and
// commits one.
func writeFathomConfigScaffolds(root string, stdout io.Writer) error {
	dir := filepath.Join(root, ".fathom")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := writeIfAbsent(filepath.Join(dir, "guard.cue"), guardspec.ExampleTemplate(), "guard.cue", stdout); err != nil {
		return err
	}
	if err := writeIfAbsent(filepath.Join(dir, "modules.cue"), bindspec.ExampleTemplate(), "modules.cue", stdout); err != nil {
		return err
	}
	// Phase T: the scaffold teaches the vocabulary without changing any
	// behaviour — every line is a comment, so the file loads as an empty
	// configuration (internal/capability.LoadSettings' absent-field path,
	// the O22 contract).
	caps := string(capability.ExampleTemplate(capability.Default()))
	if err := writeIfAbsent(filepath.Join(dir, "capabilities.cue"), caps, "capabilities.cue", stdout); err != nil {
		return err
	}
	return nil
}

// writeIfAbsent writes content to p only when p does not already exist,
// printing an "ok"/"created" line matching writeFathomIgnore's convention.
func writeIfAbsent(p, content, label string, stdout io.Writer) error {
	if _, err := os.Stat(p); err == nil {
		fmt.Fprintf(stdout, "ok .fathom/%s (already present)\n", label)
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "created .fathom/%s (commented example; no active config)\n", label)
	return nil
}

// ─── b. daemon ingest ──────────────────────────────────────────────────────

// initIngest onboards svc at root via the daemon's `ingest` MCP tool, then
// confirms the result via daemonServiceIndexed — the same probe verify.go
// uses for its own fail-open check, so "indexed" means the same thing in
// both commands.
//
// err is non-nil only for a network-level daemon-down condition (so the
// caller can print reachability instructions); a daemon that answers but
// reports no symbols yet returns ok=false, err=nil with its own MANUAL line
// already printed.
func initIngest(baseURL, svc, root string, stdout io.Writer) (bool, error) {
	hc := &http.Client{Timeout: 60 * time.Second}

	reqBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "ingest",
			"arguments": map[string]any{"service": svc, "path": root},
		},
	})
	if err != nil {
		return false, err
	}
	req, err := originHeader(baseURL, bytes.NewReader(reqBody))
	if err != nil {
		return false, err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return false, err // network-level: daemon unreachable
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return false, err
	}
	if _, rpcErr := unwrapRPCResult(raw); rpcErr != nil {
		// Daemon reachable, but ingest itself failed (e.g. bad path). That's
		// not "daemon down" — report it and let the caller fail-soft to exit 2.
		fmt.Fprintf(stdout, "MANUAL: ingest failed: %v\n", rpcErr)
		return false, nil
	}

	ok, probeErr := daemonServiceIndexed(hc, baseURL, svc)
	if probeErr != nil {
		return false, probeErr
	}
	if !ok {
		fmt.Fprintf(stdout, "MANUAL: ingest ran but service %q has no symbols in the index yet — check the daemon logs\n", svc)
		return false, nil
	}

	svcs, _ := fetchServices(hc, baseURL)
	info := svcs[svc]
	langs := ""
	if info.langs != "" {
		langs = ", " + info.langs
	}
	fmt.Fprintf(stdout, "ok ingest (service %q indexed: %d symbols%s)\n", svc, info.symbols, langs)
	return true, nil
}

// printDaemonDownInstructions prints how to bring the daemon up when init
// could not reach it at all.
func printDaemonDownInstructions(stdout io.Writer, baseURL string) {
	fmt.Fprintf(stdout, "MANUAL: fathom daemon at %s is unreachable — wiring below is complete, but the index is unconfirmed.\n", baseURL)
	fmt.Fprintln(stdout, "MANUAL: start it via launchd (if installed): launchctl kickstart -k gui/$(id -u)/com.geoffmilleraz.fathom-http")
	fmt.Fprintln(stdout, "MANUAL: or run it directly: fathom serve-http --store workspace/db/fathom.sqlite")
	fmt.Fprintln(stdout, "MANUAL: then re-run `fathom init` — it is idempotent and will only confirm the ingest.")
}

// ─── c. Claude Code project hooks ─────────────────────────────────────────

// hookSpec describes one fathom Claude Code hook wiring: which event array
// it lives in, the matcher for its group ("" means no matcher key, as Stop
// groups conventionally have none), the substring used to detect an
// already-wired entry, the fathom subcommand, and its timeout in seconds.
type hookSpec struct {
	event   string
	matcher string
	marker  string
	sub     string
	timeout int
}

var fathomHookSpecs = []hookSpec{
	{event: "SessionStart", matcher: "startup|resume|clear|compact", marker: "fathom session-hook", sub: "session-hook", timeout: 10},
	// Continuity threads (vision 21 / docs/design/continuity-threads.md §3):
	// SessionEnd deposits ONE mechanical item at the moments context is
	// lost (/clear, logout); PreCompact files a pointer before compaction —
	// the exact moment in-flight state dies today. Both write to disk and
	// inject NOTHING, so their context cost is zero by construction.
	{event: "SessionEnd", matcher: "", marker: "fathom session-end-hook", sub: "session-end-hook", timeout: 10},
	{event: "PreCompact", matcher: "manual|auto", marker: "fathom pre-compact-hook", sub: "pre-compact-hook", timeout: 10},
	{event: "PostToolUse", matcher: "Edit|Write|MultiEdit", marker: "fathom edit-hook", sub: "edit-hook", timeout: 5},
	// Guarded edits (docs/design/guarded-edits.md §2.2/§2.5): the SAME
	// "edit-hook --pre" binary/code path, registered under PreToolUse.
	// Silent-by-default on any non-guarded file/command (a local glob
	// check before any daemon call — GI-8's closure, docs/plans/
	// gap1-instrumentation.md). Two matcher groups, one command each,
	// because Claude Code's PreToolUse matcher selects by tool name and a
	// guard can fire on either files (Edit|Write|MultiEdit) or commands
	// (Bash) — ensureHookEntry's marker check is scoped per matcher group
	// so both wire independently.
	{event: "PreToolUse", matcher: "Edit|Write|MultiEdit", marker: "fathom edit-hook --pre", sub: "edit-hook --pre", timeout: 5},
	{event: "PreToolUse", matcher: "Bash", marker: "fathom edit-hook --pre", sub: "edit-hook --pre", timeout: 5},
	// Production tokenomics instrument B (docs/design/production-tokenomics.md
	// §4): the SAME edit-hook --pre binary, registered under a THIRD
	// PreToolUse matcher group for Grep/Glob/Read — the counting-only
	// discovery observer (discovery.go). Zero daemon calls, zero
	// injection, ever; ensureHookEntry's marker check is scoped per
	// matcher group (O-13's fix), so this wires independently of the
	// other two PreToolUse groups above.
	{event: "PreToolUse", matcher: "Grep|Glob|Read", marker: "fathom edit-hook --pre", sub: "edit-hook --pre", timeout: 5},
	{event: "Stop", matcher: "", marker: "fathom stop-hook", sub: "stop-hook", timeout: 10},
}

// wireHooks JSON-merges the fathom hook entries into <root>/.claude/settings.json,
// creating the file if absent. Any fathom hook already wired at Claude
// Code's USER scope (~/.claude/settings.json — the same scope doctor's
// checkHooksInstalled reads) is skipped here rather than duplicated at
// project scope: Claude Code applies both scopes, so writing the same hook
// to both would make it fire twice (the bug this guards against — a
// project `fathom init` duplicating a global wiring the user already has).
// Every skip is printed explicitly, never silent.
func wireHooks(root, binPath string, stdout io.Writer) error {
	dir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	p := filepath.Join(dir, "settings.json")

	existing, err := os.ReadFile(p)
	existed := err == nil
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	merged, changed, skipped, err := mergeHookSettings(existing, binPath)
	if err != nil {
		return err
	}
	for _, label := range skipped {
		fmt.Fprintf(stdout, "skip %s — already wired at user scope (~/.claude/settings.json); not duplicating at project scope\n", label)
	}

	switch {
	case !existed && !changed:
		// Nothing left to write: project settings.json never existed and
		// every fathom hook is already covered at user scope — creating an
		// empty file would be pure noise.
		fmt.Fprintln(stdout, "ok .claude/settings.json (not created — every fathom hook already wired at user scope)")
	case !existed:
		if err := os.WriteFile(p, merged, 0o644); err != nil {
			return err
		}
		fmt.Fprintln(stdout, "created .claude/settings.json")
	case changed:
		if err := os.WriteFile(p, merged, 0o644); err != nil {
			return err
		}
		fmt.Fprintln(stdout, "updated .claude/settings.json (fathom hooks added)")
	default:
		fmt.Fprintln(stdout, "ok .claude/settings.json (already present)")
	}
	return nil
}

// userScopeHookKeys reads Claude Code's USER-scope settings
// (~/.claude/settings.json — doctor's checkHooksInstalled checks this same
// file and no other user-scope variant, so init mirrors that scope list
// exactly) and returns the (event, matcher) keys of every fathomHookSpecs
// entry already wired there. ok is false when the file is absent or
// unreadable/unparsable — normal, not a finding; the caller then wires
// everything at project scope exactly as before this function existed.
func userScopeHookKeys() (map[hookSpecKey]bool, bool) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil, false
	}
	data, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if err != nil {
		return nil, false
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, false
	}
	hooksObj, _ := parsed["hooks"].(map[string]any)
	out := map[hookSpecKey]bool{}
	for _, spec := range fathomHookSpecs {
		if hookMarkerPresent(hooksObj, spec) {
			out[hookSpecKeyOf(spec)] = true
		}
	}
	return out, true
}

// mergeHookSettings JSON-merges the fathom SessionStart/PostToolUse/Stop hook
// entries into an existing (possibly empty/absent) Claude Code project
// settings.json, preserving every unrelated top-level key, hook event, and
// hook entry. Detection is by command substring ("fathom session-hook"
// etc.), so re-running never duplicates an entry regardless of the exact
// binary path baked in. changed reports whether anything was actually
// added; skipped names (via specLabel) every spec that was NOT written
// because it is already wired at user scope (userScopeHookKeys) — a spec
// wired there under a DIFFERENT command (someone else's hook on the same
// event/matcher) does not count as wired and still installs fathom's own,
// since hookMarkerPresent only matches fathom's own marker substring.
func mergeHookSettings(existing []byte, binPath string) (out []byte, changed bool, skipped []string, err error) {
	root := map[string]any{}
	if len(bytes.TrimSpace(existing)) > 0 {
		if err := json.Unmarshal(existing, &root); err != nil {
			return nil, false, nil, fmt.Errorf("parsing existing settings.json: %w", err)
		}
	}

	hooksObj, _ := root["hooks"].(map[string]any)
	if hooksObj == nil {
		hooksObj = map[string]any{}
	}

	userWired, userReadable := userScopeHookKeys()
	for _, spec := range fathomHookSpecs {
		if userReadable && userWired[hookSpecKeyOf(spec)] {
			skipped = append(skipped, specLabel(spec))
			continue
		}
		command := binPath + " " + spec.sub
		if ensureHookEntry(hooksObj, spec, command) {
			changed = true
		}
	}
	root["hooks"] = hooksObj

	marshaled, merr := json.MarshalIndent(root, "", "  ")
	if merr != nil {
		return nil, false, nil, fmt.Errorf("marshaling settings.json: %w", merr)
	}
	marshaled = append(marshaled, '\n')
	return marshaled, changed, skipped, nil
}

// ensureHookEntry adds spec's hook command to hooksObj[spec.event] if no
// existing entry in that event already contains spec.marker anywhere (any
// matcher group). It prefers appending into an existing group whose matcher
// equals spec.matcher over creating a new group, so sibling hooks on the
// same matcher are never split apart. Reports whether it changed anything.
func ensureHookEntry(hooksObj map[string]any, spec hookSpec, command string) bool {
	arr, _ := hooksObj[spec.event].([]any)

	// "Already wired" is scoped to spec's OWN matcher group, not the whole
	// event array: guarded-edits (docs/design/guarded-edits.md) registers
	// the SAME "fathom edit-hook --pre" command under TWO PreToolUse
	// matcher groups (Edit|Write|MultiEdit and Bash) — a global,
	// event-wide substring search would see the first group's command and
	// wrongly skip wiring the second. Every pre-existing hookSpec has
	// exactly one entry per event, so this scoping is a strict
	// generalization: identical behavior when only one spec targets an
	// event, correct behavior when more than one shares it.
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
				return false // already wired
			}
		}
	}

	newHook := map[string]any{
		"type":    "command",
		"command": command,
		"timeout": spec.timeout,
	}

	for i, g := range arr {
		gm, ok := g.(map[string]any)
		if !ok {
			continue
		}
		gMatcher, _ := gm["matcher"].(string)
		if gMatcher == spec.matcher {
			hs, _ := gm["hooks"].([]any)
			gm["hooks"] = append(hs, newHook)
			arr[i] = gm
			hooksObj[spec.event] = arr
			return true
		}
	}

	newGroup := map[string]any{"hooks": []any{newHook}}
	if spec.matcher != "" {
		newGroup["matcher"] = spec.matcher
	}
	hooksObj[spec.event] = append(arr, newGroup)
	return true
}

// ─── d. CLAUDE.md / AGENTS.md stanza ──────────────────────────────────────

const (
	fathomStanzaBegin = "<!-- fathom:begin -->"
	fathomStanzaEnd   = "<!-- fathom:end -->"
)

// fathomStanza returns the canonical fathom operating-contract block for
// service, wrapped in begin/end markers. It is the ONE source emitted
// verbatim into both CLAUDE.md and AGENTS.md (Codex reads the latter
// automatically) — one stanza, two files, no drift between them.
const fathomStanzaTemplate = "## fathom code intelligence\n" +
	"\n" +
	"This repo is indexed by fathom as service \"{{SERVICE}}\". Before any Grep/Read\n" +
	"sweep, resolve where/what/who-calls via the fathom MCP tools:\n" +
	"`find_symbol`, `references`, `symbol_context`, `outline`, `repo_map`,\n" +
	"`pack_context`, `verify_plan`. Read files to *edit* them, not to discover\n" +
	"them — the index replaces exploratory grep.\n" +
	"\n" +
	"Refresh the index via the `ingest` MCP tool (`{\"service\":\"{{SERVICE}}\"}`) at\n" +
	"session start and after each commit; it is hash-gated, so a clean repo\n" +
	"refreshes in well under a second.\n" +
	"\n" +
	"Obligations — governing rules for the file you are about to touch — are\n" +
	"injected automatically on every edit via the PostToolUse hook. Treat them\n" +
	"as binding, not advisory.\n" +
	"\n" +
	"Continuity: run `fathom thread brief` at session start if your harness has\n" +
	"no SessionStart hook (Codex does not) — it prints the open threads, or\n" +
	"nothing. File what the next session must know: `fathom thread note \"…\"`.\n" +
	"\n" +
	"Done means proven: a plan task is only \"done\" when the index proves it, not\n" +
	"when it is claimed. Before checking off a `docs/plans/*.md` item, run:\n" +
	"\n" +
	"    fathom verify --service {{SERVICE}} --plan docs/plans/<plan>.md\n" +
	"\n" +
	"Exit 0 = every claimed task verifies. Exit 1 = a claimed task is unproven —\n" +
	"fix the code or uncheck the box. Exit 2 = operational (daemon down / index\n" +
	"stale) — fail-open, not a verification failure."

func fathomStanza(service string) string {
	body := strings.ReplaceAll(fathomStanzaTemplate, "{{SERVICE}}", service)
	return fathomStanzaBegin + "\n" + body + "\n" + fathomStanzaEnd
}

// applyStanza inserts stanza into path if the file is absent, replaces the
// content between the markers if present, appends the marked block if the
// file exists but has never been wired, or — AK-3,
// docs/plans/pack-taxonomy-adoption.md — refuses loudly if the markers are
// present but corrupted (only one of the pair found, or END appears before
// BEGIN, e.g. from a hand-edit that deleted or reordered a marker line).
// The three normal paths are additive/idempotent: a re-run whose stanza
// content is unchanged rewrites nothing. The corruption path changes
// nothing either — silently appending a fresh block on top of a broken
// pair would compound the corruption (two BEGINs, an orphaned END, or
// worse) instead of surfacing it for a human to fix.
func applyStanza(path, stanza string, stdout io.Writer) error {
	name := filepath.Base(path)

	existing, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := os.WriteFile(path, []byte(stanza+"\n"), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "created %s\n", name)
		return nil
	}

	content := string(existing)
	beginIdx := strings.Index(content, fathomStanzaBegin)
	endIdx := strings.Index(content, fathomStanzaEnd)

	var newContent string
	switch {
	case beginIdx >= 0 && endIdx > beginIdx:
		// Well-formed marker pair — replace the content between them.
		newContent = content[:beginIdx] + stanza + content[endIdx+len(fathomStanzaEnd):]

	case beginIdx < 0 && endIdx < 0:
		// Never wired — append a fresh block.
		sep := "\n\n"
		if strings.HasSuffix(content, "\n\n") {
			sep = ""
		} else if strings.HasSuffix(content, "\n") {
			sep = "\n"
		}
		newContent = content + sep + stanza + "\n"

	default:
		// Corrupted: exactly one marker present, or END appears at/before
		// BEGIN. Refuse rather than guess.
		return fmt.Errorf("%s has a corrupted fathom marker pair (found %s=%t, %s=%t) — fix or remove the stray marker by hand, then re-run init",
			name, fathomStanzaBegin, beginIdx >= 0, fathomStanzaEnd, endIdx >= 0)
	}

	if newContent == content {
		fmt.Fprintf(stdout, "ok %s (already present)\n", name)
		return nil
	}
	if err := os.WriteFile(path, []byte(newContent), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "updated %s (fathom stanza)\n", name)
	return nil
}

// ─── e. ~/.fathom/config.toml scaffold ────────────────────────────────────

// starterConfigTOML is written once, write-if-absent, by writeStarterConfig.
// Every profile is commented out — init never guesses a provider choice for
// an operator — but the four shapes docs/plans/config-experience.md calls
// out by name are all present as ready-to-uncomment starting points:
// subscription (claude-cli, no key at all), API-key (anthropic), AWS
// GovCloud (bedrock), and Azure Government (azure-openai). The secrets
// doctrine is stated inline, right where an operator would otherwise be
// tempted to paste a key: there is no api_key field — api_key_env names an
// environment variable, api_key_cmd is an argv-exec'd command (e.g. `op
// read …`) whose stdout is the key — see docs/configuration.md.
const starterConfigTOML = `# fathom configuration — generated by ` + "`fathom init`" + `.
# Precedence: flags > env > project .fathom/config.toml > this file > built-in
# defaults. Uncomment ONE profile below (or write your own) and set
# default_profile to its name, or select it per-command with --profile.
#
# SECRETS DOCTRINE: there is no api_key field — it does not exist in the
# schema, so pasting a literal key here fails validation loudly instead of
# silently working. Use api_key_env (an environment variable name) or
# api_key_cmd (an argv-exec'd command whose trimmed stdout is the key, e.g.
# ["op", "read", "op://vault/item/credential"]). Neither is ever logged.
# See docs/configuration.md for the full guide.

# default_profile = "personal"

# --- subscription-backed, no API key at all (rides a logged-in Claude Code CLI) ---
# [profiles.personal]
# provider = "claude-cli"

# --- direct API key via Anthropic ---
# [profiles.api]
# provider = "anthropic"
# api_key_env = "ANTHROPIC_API_KEY"

# --- AWS Bedrock, GovCloud region ---
# [profiles.gov-bedrock]
# provider = "bedrock"
# region = "us-gov-west-1"
# model = "anthropic.claude-3-5-sonnet-20241022-v2:0"

# --- Azure OpenAI, Azure Government ---
# [profiles.gov-azure]
# provider = "azure-openai"
# endpoint = "https://<resource>.openai.azure.us"
# deployment = "<deployment-name>"
# api_key_env = "AZURE_OPENAI_API_KEY"

# --- serve-http daemon defaults (restart to apply) ---
# [daemon]
# store = "workspace/db/fleet.sqlite"
# addr = "127.0.0.1:7575"
# auto_refresh = true
# default_profile = "personal"
`

// writeStarterConfig creates path (typically config.DefaultUserPath(),
// ~/.fathom/config.toml) with starterConfigTOML if absent. It never touches
// an existing file — an operator's real profiles must never be clobbered by
// a re-run of init. An empty path (config.DefaultUserPath could not resolve
// a home directory) is a silent no-op, matching how the rest of init treats
// unresolvable optional paths.
func writeStarterConfig(path string, stdout io.Writer) error {
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); err == nil {
		fmt.Fprintln(stdout, "ok ~/.fathom/config.toml (already present)")
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(starterConfigTOML), 0o644); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "created ~/.fathom/config.toml (every profile commented out — see docs/configuration.md)")
	return nil
}

// ─── f. manual steps ───────────────────────────────────────────────────────

// printManualSteps prints the steps init cannot safely do for you: MCP
// registration (a user-scope, interactive `claude mcp add`), the Codex
// hand-off (AGENTS.md is already written; Codex's MCP config is its own),
// the lefthook pre-push verify gate — printed only, never written, because
// lefthook.yml is a hand-curated file init must not touch — and a pointer
// to the config.toml scaffold e. just wrote (also never auto-edited: an
// operator picks and uncomments a profile deliberately).
func printManualSteps(stdout io.Writer, service, url string) {
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "MANUAL: register the fathom MCP server with Claude Code (one-time, user scope):")
	fmt.Fprintf(stdout, "MANUAL:   claude mcp add --transport http --scope user fathom %s/rpc/agent\n", url)
	fmt.Fprintln(stdout, "MANUAL: Codex reads AGENTS.md automatically for the operating contract (already written above);")
	fmt.Fprintln(stdout, "MANUAL: wire its MCP registration separately, per Codex's own MCP config.")
	fmt.Fprintln(stdout, "MANUAL: add the verify gate to lefthook.yml's pre-push (fathom does not edit lefthook.yml):")
	fmt.Fprintln(stdout, "MANUAL:   pre-push:")
	fmt.Fprintln(stdout, "MANUAL:     commands:")
	fmt.Fprintln(stdout, "MANUAL:       fathom-verify:")
	fmt.Fprintln(stdout, "MANUAL:         run: |")
	fmt.Fprintf(stdout, "MANUAL:           bin/fathom verify --service %s --plan docs/plans/<plan>.md\n", service)
	fmt.Fprintln(stdout, "MANUAL: pick a provider — uncomment one profile in ~/.fathom/config.toml (or your project's")
	fmt.Fprintln(stdout, "MANUAL: .fathom/config.toml) and set default_profile; `fathom config --explain` shows what")
	fmt.Fprintln(stdout, "MANUAL: resolved and from where, `fathom providers` checks auth health at zero cost.")
	fmt.Fprintln(stdout, "MANUAL: .fathom/guard.cue, .fathom/modules.cue and .fathom/capabilities.cue were scaffolded")
	fmt.Fprintln(stdout, "        commented (inert) — run `fathom capabilities` to see what is on and why; uncomment")
	fmt.Fprintln(stdout, "MANUAL: and commit to guard critical scopes (docs/design/guarded-edits.md) or bind a platform")
	fmt.Fprintln(stdout, "MANUAL: module (docs/design/platform-modules.md); neither applies until you do.")
	fmt.Fprintln(stdout, "MANUAL: Codex has no PreToolUse hook mechanism — guard.cue's pre-edit channel is Claude-Code-only;")
	fmt.Fprintln(stdout, "MANUAL: a bound module's rules still reach Codex via the obligations/pack_context MCP tools.")
}

// autoNarrateSessions bounds init's capture window: enough sessions to see
// the adoption take, small enough to stay a pioneering probe, self-expiring
// like every narrate window (vision 14 §6).
const autoNarrateSessions = 10

// armAutoNarrate arms a bounded narrate window at init time (IQ-10) unless
// one is already active. Never fails init: capture is a bonus, not a gate.
func armAutoNarrate(stdout io.Writer) {
	if _, active := loadNarrateState(time.Now()); active {
		fmt.Fprintln(stdout, "ok narrate (already armed — leaving the active window untouched)")
		return
	}
	n := autoNarrateSessions
	st := narrateState{
		SchemaVersion:     narrateSchemaVersion,
		FathomVersion:     buildinfo.Version(),
		On:                "story-rows",
		SetAt:             time.Now().UTC().Format(time.RFC3339),
		Note:              "auto-armed by fathom init (IQ-10): capture the adoption's first sessions",
		SessionsRemaining: &n,
	}
	if err := saveNarrateState(st); err != nil {
		fmt.Fprintf(stdout, "note: auto-narrate not armed (%v) — arm by hand: fathom narrate on --sessions %d\n", err, autoNarrateSessions)
		return
	}
	fmt.Fprintf(stdout, "created narrate window: story rows for the next %d sessions (self-expiring; fathom narrate off ends it early)\n", autoNarrateSessions)
}
