package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/bindspec"
	"github.com/geoffmilleraz/fathom/internal/capability"
	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/custodian"
	"github.com/geoffmilleraz/fathom/internal/guardspec"
	"github.com/geoffmilleraz/fathom/internal/intent"
	"github.com/geoffmilleraz/fathom/internal/wiring"

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
//	    or silently clobbering a human-authored file; and a wiring file law
//	    15 BLOCKED (a tracked region edited by hand, or a file init could
//	    not read or parse). A BLOCKED file never aborts the remaining steps
//	    — the config scaffold, --skills, the shim and the manual steps have
//	    nothing to do with it — it is recorded, every other step runs, and
//	    the blocked files are listed once at the end, one line per file.
func cmdInit(args []string, stdout, stderr io.Writer) int {
	rpcOrigin = "init"
	fs := newFlagSet("init", stderr)
	path := fs.String("path", "", "repo root to wire (default: cwd)")
	service := fs.String("service", "", "service ID (default: resolved from --path, the same identity the hooks use)")
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL")
	binOverride := fs.String("bin", "", "fathom binary path baked into hook commands (default: os.Executable, symlink-resolved)")
	var skillsFlag optionalStringFlag
	fs.Var(&skillsFlag, "skills", "install the nine shipped agent skills (fathom-activate/custodian/doctor/feedback/impact/onboard/plan/rewire/verify): claude | codex | all (bare --skills = all; omit to skip)")
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

	// c2. .fathom/ commented scaffolds for phase-O config surfaces (guard.cue,
	// modules.cue) — write-if-absent, ALWAYS commented (no active config by
	// default): binding a module or guarding a scope is an explicit,
	// reviewed opt-in, never something `fathom init` activates on a repo's
	// behalf (docs/design/platform-modules.md / guarded-edits.md deliverable 1).
	if err := writeFathomConfigScaffolds(root, stdout); err != nil {
		fmt.Fprintf(stderr, "error: writing .fathom/ scaffolds: %v\n", err)
		return 1
	}

	// c + d. Project-level Claude Code hooks and the CLAUDE.md/AGENTS.md
	// stanza (one canonical source, two files) — ONE plan → apply through
	// internal/wiring (INC-12, law 15): a region fathom is tracking that a
	// human edited is reported BLOCKED and never merged, every write lands
	// in .fathom/receipts.jsonl so `fathom fleet undo --last` reverses it,
	// and the other files still apply around a blocked one.
	spec, notes := initWiringSpec(root, svc, binPath)
	for _, n := range notes {
		fmt.Fprintln(stdout, n)
	}
	res, err := applyWiringSpec(root, spec, stdout)
	if err != nil {
		fmt.Fprintf(stderr, "error: wiring %s: %v\n", root, err)
		return 1
	}
	// A BLOCKED file is recorded here and reported at the end, never
	// returned on: aborting at this point used to skip the config scaffold,
	// --skills, the PATH shim and the manual steps — none of which touch
	// the blocked file — and leave the repo half-wired with the refusal
	// scrolled off above the abort.
	blocked := blockedByFile(res.Blocked)

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

	if len(blocked) > 0 {
		printBlockedFiles(stderr, blocked)
		return 1
	}
	return exitCode
}

// blockedFile is one file law 15 stopped on, with every distinct reason
// the plan gave for it. Eight hook entries refused because ONE settings.json
// would not parse are one blocked file with one reason — a count of items
// reported as a count of files, and a settings.json described as a
// hand-edited region, were both the reader's problem before this grouping.
type blockedFile struct {
	File    string
	Cause   string   // wiring.CauseDrift / CauseUnreadable / CauseUnproven
	Reasons []string // distinct, in plan order
}

// blockedByFile groups BLOCKED items per file, files sorted, reasons
// deduplicated. Cause is the first item's (items of one file share a cause
// in every shape the engine produces; a mixed file keeps the first, and
// every reason is still listed).
func blockedByFile(items []wiring.Item) []blockedFile {
	byFile := map[string]*blockedFile{}
	for _, it := range items {
		bf, ok := byFile[it.File]
		if !ok {
			bf = &blockedFile{File: it.File, Cause: it.Cause}
			byFile[it.File] = bf
		}
		dup := false
		for _, r := range bf.Reasons {
			if r == it.Reason {
				dup = true
				break
			}
		}
		if !dup {
			bf.Reasons = append(bf.Reasons, it.Reason)
		}
	}
	out := make([]blockedFile, 0, len(byFile))
	for _, bf := range byFile {
		out = append(out, *bf)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].File < out[j].File })
	return out
}

// blockedCauseLabel is the short human phrase for a typed cause.
func blockedCauseLabel(cause string) string {
	switch cause {
	case wiring.CauseDrift:
		return "drifted (hand-edited) region"
	case wiring.CauseUnreadable:
		return "could not be read/parsed"
	case wiring.CauseUnproven:
		return "unproven provenance"
	default:
		return "blocked"
	}
}

// printBlockedFiles is init's closing report of the files law 15 stopped
// on: one line per file naming the cause, then what to do about each kind.
func printBlockedFiles(stderr io.Writer, blocked []blockedFile) {
	fmt.Fprintf(stderr, "error: %s BLOCKED — nothing was written to %s; every other init step above ran (law 15):\n",
		plural(len(blocked), "file"), map[bool]string{true: "it", false: "them"}[len(blocked) == 1])
	drift, unreadable := false, false
	for _, bf := range blocked {
		fmt.Fprintf(stderr, "error:   %s — %s: %s\n", bf.File, blockedCauseLabel(bf.Cause), strings.Join(bf.Reasons, "; "))
		switch bf.Cause {
		case wiring.CauseDrift:
			drift = true
		case wiring.CauseUnreadable:
			unreadable = true
		}
	}
	if drift {
		fmt.Fprintln(stderr, "error: a drifted region: take fathom's version by deleting the marked block and re-running init, or keep yours and fathom will keep reporting it.")
	}
	if unreadable {
		fmt.Fprintln(stderr, "error: an unreadable file: fix it by hand (it must parse as what it is) and re-run init.")
	}
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
	if err := writeIfAbsent(filepath.Join(dir, "concerns.cue"), string(custodian.ExampleTemplate()), "concerns.cue", stdout); err != nil {
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
	fmt.Fprintf(stdout, "MANUAL: start the daemon: %s\n", daemonRestartHint(""))
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

// hookKeyFor is the wiring key for one hook spec: identity is the
// (event, matcher) group plus fathom's marker substring — the same identity
// ensureHookEntry has always used — so a repointed binary path is still
// the same key. command is the full command line to install.
func hookKeyFor(spec hookSpec, command string) wiring.Key {
	return wiring.Key{
		File:    ".claude/settings.json",
		Kind:    wiring.KindJSONHook,
		Path:    []string{spec.event, spec.matcher},
		Marker:  spec.marker,
		Body:    command,
		Timeout: spec.timeout,
	}
}

// projectHookEntries reads <root>/.claude/settings.json and returns, per
// hook spec, the entry already present in that spec's matcher group
// (found by marker, exactly as ensureHookEntry finds it). Absent, unreadable
// or unparsable settings yield an empty map — "present" is the only claim
// this makes, and it makes it only from bytes it could read.
type projectHookEntry struct {
	command string
	timeout int
}

func projectHookEntries(root string) map[hookSpecKey]projectHookEntry {
	out := map[hookSpecKey]projectHookEntry{}
	data, err := os.ReadFile(filepath.Join(root, ".claude", "settings.json"))
	if err != nil {
		return out
	}
	var parsed map[string]any
	if json.Unmarshal(data, &parsed) != nil {
		return out
	}
	hooksObj, _ := parsed["hooks"].(map[string]any)
	for _, spec := range fathomHookSpecs {
		arr, _ := hooksObj[spec.event].([]any)
		for _, g := range arr {
			gm, ok := g.(map[string]any)
			if !ok {
				continue
			}
			if gMatcher, _ := gm["matcher"].(string); gMatcher != spec.matcher {
				continue
			}
			hs, _ := gm["hooks"].([]any)
			for _, h := range hs {
				hm, ok := h.(map[string]any)
				if !ok {
					continue
				}
				cmdStr, _ := hm["command"].(string)
				if !strings.Contains(cmdStr, spec.marker) {
					continue
				}
				timeout, _ := hm["timeout"].(float64)
				out[hookSpecKeyOf(spec)] = projectHookEntry{command: cmdStr, timeout: int(timeout)}
			}
		}
	}
	return out
}

// initWiringSpec is everything `fathom init` wants a repository to carry,
// as ONE wiring spec: the CLAUDE.md and AGENTS.md regions, and the hook
// entries to ADD. Init is add-only for hooks, exactly as ensureHookEntry
// was: an entry already present in its matcher group is left alone (it may
// spell the shim, or a path a human chose), and an entry already wired at
// Claude Code's USER scope (~/.claude/settings.json — the scope doctor's
// checkHooksInstalled reads) is not duplicated at project scope, because
// Claude Code applies both scopes and the hook would fire twice. Every
// skip is returned as a note to print, never silent.
func initWiringSpec(root, service, binPath string) (wiring.Spec, []string) {
	intentOn := intentChannelWired(root)
	spec := wiring.Spec{Regions: []wiring.Region{
		stanzaRegion("CLAUDE.md", service, intentOn),
		stanzaRegion("AGENTS.md", service, intentOn),
	}}
	var notes []string
	userWired, userReadable := userScopeHookKeys()
	present := projectHookEntries(root)
	for _, hs := range fathomHookSpecs {
		if userReadable && userWired[hookSpecKeyOf(hs)] {
			notes = append(notes, "skip "+specLabel(hs)+" — already wired at user scope (~/.claude/settings.json); not duplicating at project scope")
			continue
		}
		if _, ok := present[hookSpecKeyOf(hs)]; ok {
			continue // add-only: present entries are not init's to rewrite
		}
		spec.Keys = append(spec.Keys, hookKeyFor(hs, binPath+" "+hs.sub))
	}
	if len(spec.Keys) == 0 {
		// Nothing to plan for settings.json, so the engine will not name
		// it; init still owes the reader a line per file it looked at.
		if _, err := os.Stat(filepath.Join(root, ".claude", "settings.json")); err == nil {
			notes = append(notes, "ok .claude/settings.json (already present)")
		} else {
			notes = append(notes, "ok .claude/settings.json (not created — every fathom hook already wired at user scope)")
		}
	}
	return spec, notes
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
	// fathomStanzaBegin / fathomStanzaEnd are the marker pair as `fathom
	// init` rendered it before marker format v3 (internal/wiring) — the
	// spelling every repository wired before INC-12 carries. A region
	// written or upgraded by the wiring engine keeps the same prefix and
	// the same end marker but carries the template version and the body
	// digest on the begin line ("<!-- fathom:begin v3 sha256:… -->"), so
	// anything LOOKING for a region must match fathomStanzaBeginPrefix, not
	// the v1 spelling; fathomStanza still renders the v1 form for callers
	// that want the bare template (doctor's fixtures, gen-claude-md).
	fathomStanzaBegin       = "<!-- fathom:begin -->"
	fathomStanzaBeginPrefix = "<!-- fathom:begin"
	fathomStanzaEnd         = "<!-- fathom:end -->"
	// fathomStanzaVersion is the template version recorded on the begin
	// marker. The body is compared byte-for-byte regardless, so this only
	// needs to move with the marker FORMAT.
	fathomStanzaVersion = "v3"
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
	"Opening moves by task shape (first 2-3 calls, then edit):\n" +
	"- debug/trace: find_symbol -> find_callers/references -> symbol_context\n" +
	"- feature/change: repo_map -> pack_context(concern) -> references\n" +
	"- review/verify: verify_plan -> obligations -> references on changed symbols\n" +
	"- refactor: references -> outline -> symbol_context\n" +
	"\n" +
	"Refresh the index via the `ingest` MCP tool (`{\"service\":\"{{SERVICE}}\"}`) at\n" +
	"session start and after each commit; it is hash-gated, so a clean repo\n" +
	"refreshes in well under a second.\n" +
	"\n" +
	"{{OBLIGATIONS}}\n" +
	"\n" +
	"Continuity: run `fathom prime` at session start if your harness has no\n" +
	"SessionStart hook (Codex does not) — it prints the same identity,\n" +
	"freshness, live-example, thread-brief, and custodian primer this hook\n" +
	"injects on Claude Code. File what the next session must know:\n" +
	"`fathom thread note \"…\"`.\n" +
	"\n" +
	"Done means proven: a plan task is only \"done\" when the index proves it, not\n" +
	"when it is claimed. Before checking off a `docs/plans/*.md` item, run:\n" +
	"\n" +
	"    fathom verify --service {{SERVICE}} --plan docs/plans/<plan>.md\n" +
	"\n" +
	"Exit 0 = every claimed task verifies. Exit 1 = a claimed task is unproven —\n" +
	"fix the code or uncheck the box. Exit 2 = operational (daemon down / index\n" +
	"stale) — fail-open, not a verification failure."

// The obligations paragraph, in two wordings of the SAME three physical
// lines (EP-22). The intent wording is used ONLY where the channel is
// actually wired: a stanza that promises matched invariants in a repo whose
// guard.cue carries none teaches the agent to expect something that will
// never arrive, which is worse than saying nothing.
const (
	obligationsPlain = "Obligations — governing rules for the file you are about to touch — are\n" +
		"injected automatically on every edit via the PostToolUse hook. Treat them\n" +
		"as binding, not advisory."
	obligationsWithIntent = "Obligations — governing rules for the file you are about to touch, and\n" +
		"where `docs/intent/` exists its matched invariants — are injected on every\n" +
		"edit via the PostToolUse hook. Treat them as binding, not advisory."
)

// fathomStanzaBody is the stanza WITHOUT its markers — what goes between
// them, and what the wiring engine digests.
func fathomStanzaBody(service string, intentOn bool) string {
	obligations := obligationsPlain
	if intentOn {
		obligations = obligationsWithIntent
	}
	s := strings.ReplaceAll(fathomStanzaTemplate, "{{SERVICE}}", service)
	return strings.ReplaceAll(s, "{{OBLIGATIONS}}", obligations)
}

func fathomStanza(service string, intentOn bool) string {
	return fathomStanzaBegin + "\n" + fathomStanzaBody(service, intentOn) + "\n" + fathomStanzaEnd
}

// intentChannelWired is EP-22's predicate: the intent corpus has a router
// AND at least one guard entry carries intent provenance. Router alone is
// not enough — the corpus can be adopted with the edit-time channel left
// unwired, and this repo is exactly that case today (14 sensitive surfaces,
// 0 covered), so the sentence must not render here.
func intentChannelWired(root string) bool {
	c, err := intent.Load(root)
	if err != nil || c.Tier != intent.TierRouter {
		return false
	}
	notes, scopesOf, _ := repoGuardNotes(root)
	if scopesOf == nil {
		return false
	}
	// Covered = at least one surface NOT reported as uncovered.
	uncovered := 0
	for _, cl := range intent.GuardEntryStale(c, notes, scopesOf) {
		if !strings.HasPrefix(cl.Ref, ".fathom/guard.cue") {
			uncovered++
		}
	}
	return uncovered < len(c.Surfaces)
}

// stanzaRegion is the wiring region for one stanza-carrying file.
// AdoptUntracked is set because the marker pair IS the ownership claim
// every repository wired before marker format v3 carries: a v1 region is
// fathom's, gets its digest on the first write, and is never reported as
// drift; only a tracked region that no longer matches its digest is.
func stanzaRegion(file, service string, intentOn bool) wiring.Region {
	return wiring.Region{
		File:           file,
		Syntax:         wiring.HTMLMarkers,
		Version:        fathomStanzaVersion,
		Body:           fathomStanzaBody(service, intentOn),
		AdoptUntracked: true,
	}
}

// stanzaBodyOf strips the marker pair from a rendered stanza (fathomStanza's
// output, or a v3-marked region) and returns the body between them.
func stanzaBodyOf(stanza string) string {
	s := stanza
	if i := strings.Index(s, fathomStanzaBeginPrefix); i >= 0 {
		if nl := strings.IndexByte(s[i:], '\n'); nl >= 0 {
			s = s[i+nl+1:]
		}
	}
	if i := strings.LastIndex(s, fathomStanzaEnd); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSuffix(s, "\n")
}

// applyWiringSpec runs spec through internal/wiring's plan → apply and
// narrates the outcome per file in init's own vocabulary: "created X",
// "updated X (…)", "ok X (already present)", and — the law-15 outcome —
// "BLOCKED X — <why>", which names the drift and writes nothing to that
// file while the others still apply. The receipt lands in
// .fathom/receipts.jsonl; nothing here writes a byte outside the engine.
func applyWiringSpec(root string, spec wiring.Spec, stdout io.Writer) (wiring.Result, error) {
	res, err := wiring.ApplyWiring(root, spec, wiring.ApplyOptions{Now: time.Now})
	if err != nil {
		return res, err
	}
	created := map[string]bool{}
	if res.Receipt != nil {
		for _, f := range res.Receipt.Files {
			if !f.Existed {
				created[f.Path] = true
			}
		}
	}
	type outcome struct {
		applied, skipped []wiring.Item
		blocked          []wiring.Item
	}
	byFile := map[string]*outcome{}
	get := func(file string) *outcome {
		o, ok := byFile[file]
		if !ok {
			o = &outcome{}
			byFile[file] = o
		}
		return o
	}
	for _, it := range res.Applied {
		o := get(it.File)
		o.applied = append(o.applied, it)
	}
	for _, it := range res.Skipped {
		o := get(it.File)
		o.skipped = append(o.skipped, it)
	}
	for _, it := range res.Blocked {
		o := get(it.File)
		o.blocked = append(o.blocked, it)
	}
	files := make([]string, 0, len(byFile))
	for f := range byFile {
		files = append(files, f)
	}
	sort.Strings(files)
	for _, f := range files {
		o := byFile[f]
		// One BLOCKED line per distinct reason, not per item: a
		// settings.json that will not parse blocks every hook key with the
		// same words, and eight identical lines hide the one that differs.
		for _, bf := range blockedByFile(o.blocked) {
			for _, reason := range bf.Reasons {
				fmt.Fprintf(stdout, "BLOCKED %s — %s\n", f, reason)
			}
		}
		switch {
		case len(o.applied) > 0 && created[f]:
			fmt.Fprintf(stdout, "created %s\n", f)
		case len(o.applied) > 0:
			what := "fathom hooks added"
			if o.applied[0].Kind == wiring.KindRegion {
				what = "fathom stanza"
			}
			fmt.Fprintf(stdout, "updated %s (%s)\n", f, what)
		case len(o.blocked) == 0:
			fmt.Fprintf(stdout, "ok %s (already present)\n", f)
		}
	}
	return res, nil
}

// applyStanza writes ONE stanza into path through the wiring engine — the
// one-file form of what cmdInit does for both files at once, kept for the
// callers (and tests) that address a single file. stanza is fathomStanza's
// rendered form (markers included); the body between the markers is what
// gets written. A BLOCKED verdict (a corrupted marker pair, or a tracked
// region edited by hand) is returned as an error and the file is left
// byte-identical; nothing is printed for it.
func applyStanza(path, stanza string, stdout io.Writer) error {
	root, file := filepath.Dir(path), filepath.Base(path)
	region := wiring.Region{
		File:           file,
		Syntax:         wiring.HTMLMarkers,
		Version:        fathomStanzaVersion,
		Body:           stanzaBodyOf(stanza),
		AdoptUntracked: true,
	}
	plan, err := wiring.PlanWiring(root, wiring.Spec{Regions: []wiring.Region{region}})
	if err != nil {
		return err
	}
	if blocked := plan.Blocked(); len(blocked) > 0 {
		return fmt.Errorf("%s: %s", file, blocked[0].Reason)
	}
	_, err = applyWiringSpec(root, wiring.Spec{Regions: []wiring.Region{region}}, stdout)
	return err
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
	fmt.Fprintln(stdout, "MANUAL: register fathom as an MCP server for Codex too (docs/adoption/codex.md §2):")
	fmt.Fprintln(stdout, "MANUAL:   HTTP (the shared daemon above — test with `codex mcp list`, unverified negotiation per-version):")
	fmt.Fprintf(stdout, "MANUAL:     codex mcp add fathom --url %s/rpc/agent\n", url)
	fmt.Fprintln(stdout, "MANUAL:   or stdio fallback (guaranteed-compatible, single Codex session):")
	fmt.Fprintln(stdout, "MANUAL:     codex mcp add fathom -- /abs/path/bin/fathom serve-mcp --store /abs/path/workspace/db/fleet.sqlite")
	fmt.Fprintln(stdout, "MANUAL: add the verify gate to lefthook.yml's pre-push (fathom does not edit lefthook.yml):")
	fmt.Fprintln(stdout, "MANUAL:   pre-push:")
	fmt.Fprintln(stdout, "MANUAL:     commands:")
	fmt.Fprintln(stdout, "MANUAL:       fathom-verify:")
	fmt.Fprintln(stdout, "MANUAL:         run: |")
	fmt.Fprintf(stdout, "MANUAL:           bin/fathom verify --service %s --plan docs/plans/<plan>.md --quiet\n", service)
	fmt.Fprintln(stdout, "MANUAL: keep the index fresh after each commit — through the DAEMON, never a shell ingest")
	fmt.Fprintln(stdout, "MANUAL: against the shared store (INC-3; a wedged reader once wrote a plausible empty index):")
	fmt.Fprintln(stdout, "MANUAL:   post-commit:")
	fmt.Fprintln(stdout, "MANUAL:     commands:")
	fmt.Fprintln(stdout, "MANUAL:       fathom-refresh:")
	fmt.Fprintf(stdout, "MANUAL:         run: bin/fathom post-commit-hook --dir \"$PWD\" --service %s --quiet\n", service)
	fmt.Fprintln(stdout, "MANUAL:   commit-msg:")
	fmt.Fprintln(stdout, "MANUAL:     commands:")
	fmt.Fprintln(stdout, "MANUAL:       verified: { run: grep -q '^Verified:' {1} || echo 'note: no Verified: line (changes.md L1)' }")
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
