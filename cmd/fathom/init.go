package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/geoffmilleraz/fathom/internal/bindspec"
	"github.com/geoffmilleraz/fathom/internal/capability"
	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/custodian"
	"github.com/geoffmilleraz/fathom/internal/guardspec"
	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/intent"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
	"github.com/geoffmilleraz/fathom/internal/wiring"
)

// cmdInit implements the "init" subcommand: one-command, idempotent,
// additive-only wiring of a repo into fathom's coding-agent story —
// .fathomignore, a daemon ingest, Claude Code project hooks, the AGENTS.md
// operating-contract stanza (plus an @AGENTS.md import region in a root
// CLAUDE file that already exists, ADR-059), and (opt-in,
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
//	    that has nothing to do with the files it wrote). The specific reason
//	    is always printed as a "MANUAL:" line on stdout AND, since the
//	    2026-09-28 stranger-check incident (a bare "fathom init exited 2"
//	    with no visible cause — the MANUAL line was there, but a caller
//	    filtering "routine" stdout noise dropped it too), as a plain
//	    "error:" line on stderr that names the reason without a "MANUAL:"
//	    or "ok " prefix a filter might treat as routine — exit 2 must never
//	    look silent.
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
	if hasJSONFlag(args) {
		return renderInitJSON(args, stdout, stderr) // RP-15: init_json.go
	}
	fs := newFlagSet("init", stderr)
	fs.Bool("json", false, "emit every write, the receipt id and the manual steps as JSON")
	path := fs.String("path", "", "repo root to wire (default: cwd)")
	service := fs.String("service", "", "service ID (default: resolved from --path, the same identity the hooks use)")
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL")
	binOverride := fs.String("bin", "", "fathom binary path baked into hook commands (default: os.Executable, symlink-resolved)")
	var skillsFlag optionalStringFlag
	fs.Var(&skillsFlag, "skills", "install the ten shipped agent skills (fathom-activate/custodian/doctor/feedback/findings/impact/onboard/plan/rewire/verify): claude | codex | agents | all | codex-prompts (every repo mode writes the one copy into .claude/skills; bare --skills = all, codex and agents also link .agents/skills to it; claude = .claude/skills only; codex-prompts = the deprecated ~/.codex/prompts twin; omit to skip)")
	seedUserIgnoreFlag := fs.Bool("seed-user-ignore", false, "seed ~/.fathom/ignore with machine-junk defaults (docs/design/scoped-config.md); prints exactly what it writes, never silent, requires this flag")
	noUserWritesFlag := fs.Bool("no-user-writes", false, "preview user configuration, ignore, prompts and PATH shim writes; operational state is still written to the state directory; --harness none requires each user-scope write to be requested explicitly")
	harnessFlag := fs.String("harness", "", initHarnessUsage)
	noSCIPFlag := fs.Bool("no-scip", false, "do not run scip-typescript / scip-python: by default init runs whichever is already on PATH for a TypeScript/JavaScript or Python repo (never installing one) and has the daemon import its index as typed edges; a producer type-checks the whole repository and can take minutes")
	dryRun := fs.Bool("dry-run", false, "list every file this init would write or change, then a Cost section with free facets, compile token/price ranges and unpriced decision sites; write none: the same steps decide each write, and a dry run stops each one before it lands (it also skips the daemon ingest and any SCIP producer, and names them)")
	demo := fs.Bool("demo", false, "try fathom on a throwaway copy of its bundled fixture fleet under a temporary HOME: wire it, ingest a service, prove a fixture plan, print the time to that first proven answer, and remove the copy; your HOME, ~/.fathom and any running daemon are never touched")
	hooksScopeFlag := fs.String("hooks-scope", "auto", "Claude Code hook scope: auto (default) skips a fathom hook already wired at ~/.claude/settings.json; project writes every fathom hook at <root>/.claude/settings.json regardless of user scope — required for a clean room or CI harness that loads only project settings (e.g. claude --restricted --settings), which never sees user-scope hooks. Also settable via FATHOM_HOOKS_SCOPE=project.")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	rpcOrigin = "init" // after Parse: help runs flag parsing alone (O671)
	if *demo {
		if *dryRun {
			fmt.Fprintln(stderr, "error: --demo and --dry-run do not combine: the demo writes only into its own throwaway copy")
			return 1
		}
		return initDemo(context.Background(), defaultInitDemoDeps(), stdout, stderr)
	}
	hooksScope, err := resolveHooksScope(*hooksScopeFlag)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
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
	// HP-13: the harness decides WHICH vehicles init writes — never more
	// than the descriptor declares, never a user-global file — and a
	// descriptor past its demotion clock is refused with the reason.
	targets, err := initTargetsFor(*harnessFlag, time.Now())
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	noUserWrites := *noUserWritesFlag || targets.ID == initHarnessNone
	userWritesCause := "--harness none"
	if *noUserWritesFlag {
		userWritesCause = "--no-user-writes"
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

	// The hooks and MCP entries init writes spell the canonical bare
	// `fathom` (the PATH shim init installs below), the form doctor and
	// fathom-rewire treat as current; only an explicit --bin pins a path.
	binPath := *binOverride
	if binPath == "" {
		binPath = "fathom"
	}

	fmt.Fprintf(stdout, "fathom init — service %q, root %s\n", svc, root)

	// RH-11: every write below goes through w. A dry run records each one
	// instead of making it, sends the steps' own narration nowhere, and
	// prints the recorded list at the end.
	w := &initWrites{dry: *dryRun}
	out := stdout
	if w.dry {
		fmt.Fprintln(stdout, "dry run: listing every file this init would write or change; nothing is written")
		out = io.Discard
	}
	userWrites, userOut := w, out
	if noUserWrites {
		userWrites = &initWrites{dry: true}
		userOut = io.Discard
	}

	// a. .fathomignore
	if err := writeFathomIgnore(w, root, out); err != nil {
		fmt.Fprintf(stderr, "error: writing .fathomignore: %v\n", err)
		return 1
	}

	// b. daemon ingest (fail-soft: wiring continues either way).
	exitCode := 0
	var indexed bool
	var ingestErr error
	if w.dry {
		// The ingest writes the daemon's store, not a file here: named,
		// skipped, and assumed to succeed so the steps that follow a
		// confirmed index (SCIP) list what they would write.
		w.note(fmt.Sprintf("ingest service %q through the daemon at %s (its store, not a file here)", svc, *url))
		indexed = true
	} else {
		indexed, ingestErr = initIngest(*url, svc, root, stdout)
	}
	if ingestErr != nil {
		printDaemonDownInstructions(stdout, *url)
		// A "MANUAL:" line above already names this, but a caller that
		// treats "MANUAL:"/"ok " stdout as routine noise (scripts/
		// stranger-check.sh's diagnostic filter did exactly this,
		// 2026-09-28) would see exit 2 with no visible cause. This line
		// carries neither prefix and goes to stderr, so it survives any
		// such filter.
		fmt.Fprintf(stderr, "error: exiting 2 — fathom daemon at %s is unreachable; wiring finished, only the index is unconfirmed (see the MANUAL line(s) above)\n", *url)
		exitCode = 2
	} else if !indexed {
		fmt.Fprintf(stderr, "error: exiting 2 — service %q did not confirm as indexed; wiring finished, only the index is unconfirmed (see the MANUAL line above for why)\n", svc)
		exitCode = 2
	}

	// c2. .fathom/ commented scaffolds for phase-O config surfaces (guard.cue,
	// modules.cue) — write-if-absent, ALWAYS commented (no active config by
	// default): binding a module or guarding a scope is an explicit,
	// reviewed opt-in, never something `fathom init` activates on a repo's
	// behalf (docs/design/platform-modules.md / guarded-edits.md deliverable 1).
	if err := writeFathomConfigScaffolds(w, root, out); err != nil {
		fmt.Fprintf(stderr, "error: writing .fathom/ scaffolds: %v\n", err)
		return 1
	}

	// c + d. Project-level Claude Code hooks, the AGENTS.md stanza and the
	// root CLAUDE file's @AGENTS.md import region (ADR-059: one stanza, one
	// file; a CLAUDE file is never created) — ONE plan → apply through
	// internal/wiring (INC-12, law 15): a region fathom is tracking that a
	// human edited is reported BLOCKED and never merged, every write lands
	// in .fathom/receipts.jsonl so `fathom fleet undo --last` reverses it,
	// and the other files still apply around a blocked one.
	spec, notes := initWiringSpecFor(root, svc, binPath, targets, hooksScope)
	for _, n := range notes {
		fmt.Fprintln(out, n)
	}
	res, err := applyWiringSpec(w, root, spec, out)
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
	if err := writeStarterConfig(userWrites, config.DefaultUserPath(), userOut); err != nil {
		fmt.Fprintf(stderr, "error: writing config.toml: %v\n", err)
		return 1
	}

	// e2. ~/.fathom/ignore machine-junk seed (docs/design/scoped-config.md
	// deliverable 4) — --seed-user-ignore ONLY: init never touches
	// user-scope config on its own. Prints exactly what it writes.
	if *seedUserIgnoreFlag {
		ignoreWrites, ignoreOut := userWrites, userOut
		if targets.ID == initHarnessNone && !*noUserWritesFlag {
			ignoreWrites, ignoreOut = w, out
		}
		if err := seedUserIgnoreFile(ignoreWrites, ignoreOut); err != nil {
			fmt.Fprintf(stderr, "error: seeding ~/.fathom/ignore: %v\n", err)
			return 1
		}
	}

	// f. shipped agent skills (AK-2, docs/plans/pack-taxonomy-adoption.md):
	// --skills[=claude|codex|agents|all] only — omitted by default, so a
	// caller who never asked for skills sees no new files. One real copy in
	// .claude/skills; .agents/skills is an owned link to it (ADR-059).
	skillsWrites, skillsOut := w, out
	if skillsMode == "codex-prompts" {
		skillsWrites, skillsOut = userWrites, userOut
		if targets.ID == initHarnessNone && !*noUserWritesFlag {
			skillsWrites, skillsOut = w, out
		}
	}
	if err := installSkills(skillsWrites, root, skillsMode, skillsOut); err != nil {
		fmt.Fprintf(stderr, "error: installing skills: %v\n", err)
		return 1
	}

	// f2. SCIP typed edges (SR-65, surface reckoning F-74): run scip-typescript
	// / scip-python when already installed and the repo is that language,
	// then have the daemon import the index. Last of the writes, so a
	// producer's minutes never delay the wiring above, and never a reason
	// for a non-zero exit — a producer that fails is a printed warning.
	if !*noSCIPFlag {
		initSCIPStep(context.Background(), w, *url, root, svc, indexed && ingestErr == nil, out, defaultSCIPInitDeps())
	}

	// g. manual steps init cannot do for you.
	// IO-20: the PATH shim — hooks and stanzas spell `fathom`, never a
	// checkout path; a gate that cannot be invoked must fail loudly, not
	// pass silently (the 2026-07-31 journal case).
	if msg, err := installShim(userWrites); err != nil {
		fmt.Fprintf(userOut, "shim: NOT installed — %v (hooks written by this init still work via PATH once you install one)\n", err)
	} else {
		fmt.Fprintf(userOut, "%s\n", msg)
	}
	if noUserWrites {
		printInitUserWritesPreview(stdout, root, userWritesCause, userWrites)
	}

	if w.dry {
		w.printDryRun(stdout, root, svc)
		if len(blocked) > 0 {
			printBlockedFiles(stderr, blocked)
			return 1
		}
		return 0
	}

	if targets.ID == harness.ClaudeCode {
		printManualSteps(stdout, svc, *url)
	} else {
		printHarnessManualSteps(stdout, root, targets, svc, *url)
	}
	if skillsMode == "" {
		fmt.Fprintln(stdout, "MANUAL: install fathom's shipped agent skills (ten, into .claude/skills; .agents/skills, the Agent Skills standard path, becomes a link to it):")
		fmt.Fprintln(stdout, "MANUAL:   fathom init --skills[=claude|codex|agents|all|codex-prompts]")
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
func writeFathomIgnore(w *initWrites, root string, stdout io.Writer) error {
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
	if err := w.writeFile(p, []byte(sb.String()), 0o644); err != nil {
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
func seedUserIgnoreFile(w *initWrites, stdout io.Writer) error {
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

	if err := w.mkdirAll(filepath.Dir(path)); err != nil {
		return err
	}
	if err := w.writeFile(path, []byte(content), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "created %s\n", path)
	return nil
}

// ─── c2. .fathom/ config scaffolds (guard.cue, modules.cue, capabilities.cue) ──

// findingsConfigTemplate is the project-scope `.fathom/config.toml`
// scaffold (FL-25, docs/plans/findings-loop.md, vision 30 §7; the
// `[decide]` block is DP-1's addition, docs/vision/32-decision-port.md):
// FULLY COMMENTED `[findings]` and `[decide]` blocks — every line an
// operator would uncomment, one at a time, with a trailing comment naming
// what it does.
// Never validated against internal/config's #Config schema (that schema
// has no `findings` field yet — it is added by the wave that builds
// FL-9's BindSubject/the importer — so this block stays pure documentation
// until then; TestWriteStarterConfig_UncommentedProfilesAreValid's fixed
// block list deliberately does NOT include this one for the same reason).
// See docs/configuration.md's "The findings loop" / "The decision port"
// sections and docs/adoption/findings-loop.md for the end-to-end guide.
const findingsConfigTemplate = `# fathom project configuration (docs/configuration.md).
# Precedence: flags > env > project .fathom/config.toml > ~/.fathom/config.toml
# > built-in defaults.

# --- the findings loop (docs/vision/30-findings-loop.md, ADR-040) ---
# Binds this repository to a review-kit subject and, optionally, a local
# outbox file the session hook imports once per session (vision 30 §3.7).
# Uncomment once a review-kit outbox exists for this repo — see
# docs/adoption/findings-loop.md.
# [findings]
# subject   = "org/repo/component"                 # the provisioned review-kit subject this repo maps to; fathom findings import refuses on any mismatch
# outbox    = ".reviewkit/outbox/projection.json"   # repo-relative; the session hook imports it once per session when its digest changes from the last import
# producers = ["reviewkit.system-review"]           # producer ids this repo trusts (schema/semantic/producers.cue's catalog) — an unlisted producer's events are refused

# --- the decision port (docs/vision/32-decision-port.md, ADR-042) ---
# Typed fast decisions (choice/score/noul) behind a provider flag. Off by
# default — provider = none makes no call and changes nothing. Uncomment to
# route the compile pre-filter, the obligations re-rank, and prompt routing
# through a real model — see docs/configuration.md's "The decision port".
# [decide]
# provider          = "none"   # none | llm | typesafe | openai — none makes no call and changes nothing
# profile           = ""       # llm: the [llm] profile lane to use (default: the compile lane)
# compile_threshold = 0.5      # a guide section whose "is this a binding rule" probability is below this is not compiled (counted, disclosed, still served raw)
# route_confidence  = 0.9      # unchanged at this default (2026-09-24, ADR-042 amendment: route's own calibration reading failed at every cutoff); the prompt hook injects a hedged route line at or above this OR in the confirm band (0.5-0.9) — same wording either way since CV-M11, never firm; nothing below 0.5
# route_timeout_ms  = 3000     # POST /route's own decider budget, in ms (100-30000); the design point is a sub-second decision model — raise for a slower (e.g. LLM) lane, the prompt hook's own shared deadline (~8s) still bounds the line
# rank_within_tier  = false    # obligations re-orders rules only inside their governing-scope tier (default false since 2026-09-23, ADR-042 amendment: negative on guards-jev-r1 once a real provider is configured — set true to opt back in)
# rerank_timeout_ms = 1500     # obligations re-rank's own decider budget, in ms (100-60000); kept under the PostToolUse edit hook's 2000ms RPC timeout with margin — an LLM lane usually exceeds this default and falls back to lexical order (rank_note names the budget); raise only for offline use
# transport         = "openrouter"                    # typesafe: "typesafe" (its own API, the default) or "openrouter" (Jev through OpenRouter); openai: "openai" only
# base_url          = "https://api.openai.com/v1"     # typesafe/openai: API origin override (default: the provider's own); this file may name only that host or a loopback one, another host belongs in ~/.fathom/config.toml
# model             = "gpt-6-luna"                    # typesafe/openai: model override (defaults: jev-latest, typesafe/jev-1.13 over openrouter, gpt-6-luna)
# (api_key_file, api_key_env and api_key_cmd are NOT read from this file: a credential and the command that fetches it are the user's, never a repository's — set them in ~/.fathom/config.toml; fathom doctor's key_source_scope names any set here)
`

// writeFathomConfigScaffolds creates <root>/.fathom/guard.cue,
// <root>/.fathom/modules.cue, <root>/.fathom/capabilities.cue and
// <root>/.fathom/config.toml with commented, inert example templates if
// absent — mirroring writeFathomIgnore's write-if-absent, never-overwrite
// discipline. Every one is inert as scaffolded (guardspec/bindspec's
// ExampleTemplate — every worked example is a comment block; config.toml's
// own scaffold is 100% `#` comments, a valid empty TOML document), so
// `fathom doctor` sees no active guard/binding/findings subject until a
// human uncomments and commits one.
func writeFathomConfigScaffolds(w *initWrites, root string, stdout io.Writer) error {
	dir := projectdir.Under(root)
	if err := w.mkdirAll(dir); err != nil {
		return err
	}
	if err := writeIfAbsent(w, filepath.Join(dir, "guard.cue"), guardspec.ExampleTemplate(), "guard.cue", stdout); err != nil {
		return err
	}
	if err := writeIfAbsent(w, filepath.Join(dir, "concerns.cue"), string(custodian.ExampleTemplate()), "concerns.cue", stdout); err != nil {
		return err
	}
	if err := writeIfAbsent(w, filepath.Join(dir, "modules.cue"), bindspec.ExampleTemplate(), "modules.cue", stdout); err != nil {
		return err
	}
	if err := writeIfAbsent(w, filepath.Join(dir, "config.toml"), findingsConfigTemplate, "config.toml", stdout); err != nil {
		return err
	}
	// Phase T: the scaffold teaches the vocabulary without changing any
	// behaviour — every line is a comment, so the file loads as an empty
	// configuration (internal/capability.LoadSettings' absent-field path,
	// the O22 contract).
	caps := string(capability.ExampleTemplate(capability.Default()))
	if err := writeIfAbsent(w, filepath.Join(dir, "capabilities.cue"), caps, "capabilities.cue", stdout); err != nil {
		return err
	}
	return nil
}

// writeIfAbsent writes content to p only when p does not already exist,
// printing an "ok"/"created" line matching writeFathomIgnore's convention.
func writeIfAbsent(w *initWrites, p, content, label string, stdout io.Writer) error {
	if _, err := os.Stat(p); err == nil {
		fmt.Fprintf(stdout, "ok .fathom/%s (already present)\n", label)
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := w.writeFile(p, []byte(content), 0o644); err != nil {
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
	return initIngestWith(baseURL, svc, root, stdout, initIngestTiming{request: initIngestRequestTimeout, pollEvery: 5 * time.Second, pollFor: defaultIngestPollFor})
}

// initIngestRequestTimeout bounds init's ingest request (EW-70). It outlasts
// the daemon's ingest wait budget (mcp.DefaultIngestWaitBudget, 120 s) and
// its RPC write deadline (daemonRPCWriteTimeout, 150 s), so a first ingest
// that outlasts the budget is answered with the run that serves it (EW-66)
// and init polls that run. At 60 s init gave up first, lost the run id, and
// could only poll the indexed probe blind for 30 minutes, also after the
// daemon had failed or refused the ingest.
const initIngestRequestTimeout = daemonRPCWriteTimeout + 30*time.Second

// initIngestTiming bounds init's ingest request and, when that request
// outlasts its timeout, how long init waits for the service to read as
// indexed.
type initIngestTiming struct {
	request, pollEvery, pollFor time.Duration
}

// defaultIngestPollFor is how long init waits for a first, from-scratch
// ingest of a large repository to read as indexed before giving up
// (fail-open, exit 2). Raised from 15m to 30m on 2026-09-28: the
// language-breadth merge (ExtractorEpoch 9->16, internal/fusion/pipeline.go —
// new tree-sitter/WASM dialects for HTML, CSS and Dockerfile, plus
// fusion.New's catch-all loop wiring every remaining declared dialect)
// roughly doubled a from-scratch stranger-check ingest's wall time
// uncontended (measured: 110s at 2bb0c6ef -> ~235s after the merge,
// docs/bench/readings/reach/). `task validate` runs stranger-check
// CONCURRENTLY with other CPU-heavy gate deps (scenariobench's own
// `go test -race`, the full race suite) — under that contention the old
// 15m budget was no longer enough headroom and a legitimately-slow-but-
// succeeding ingest read as "exit 2" (the 2026-09-28 stranger-check
// incident). 30m gives roughly the same multiple of headroom over the
// current uncontended measurement that 15m gave over the pre-merge one.
const defaultIngestPollFor = 30 * time.Minute

// initIngestWith is initIngest with its timing injected. A request that
// times out or drops mid-response is not "daemon down": a first ingest of a
// large repository can outlast any one request (the typed pass runs niced,
// RT-106) and continues detached on the daemon (RT-149), so init waits for
// the service to read as indexed. Only a refused connection is daemon-down.
//
// EW-70: an ingest the daemon answers past its wait budget names the run
// that serves it (EW-66); init then polls that run (waitForIngestRun) and
// stops the moment it reads done, so a run the daemon failed or refused
// ends init's wait at once with the daemon's own error. The indexed probe
// (waitForIndexed) stays the fallback for an answer that names no run and
// for a request that timed out anyway.
func initIngestWith(baseURL, svc, root string, stdout io.Writer, t initIngestTiming) (bool, error) {
	hc := &http.Client{Timeout: t.request}

	raw, err := callIngestTool(hc, baseURL, map[string]any{"service": svc, "path": root})
	if err != nil {
		if errors.Is(err, syscall.ECONNREFUSED) {
			return false, err // network-level: daemon unreachable
		}
		if errors.Is(err, errIngestRequestBuild) {
			return false, err
		}
		return waitForIndexed(baseURL, svc, stdout, t)
	}
	inner, rpcErr := unwrapRPCResult(raw)
	if rpcErr != nil {
		// Daemon reachable, but ingest itself failed (e.g. bad path). That's
		// not "daemon down" — report it and let the caller fail-soft to exit 2.
		fmt.Fprintf(stdout, "MANUAL: ingest failed: %v\n", rpcErr)
		return false, nil
	}
	var answer struct {
		Completed *bool  `json:"completed"`
		Run       string `json:"run"`
	}
	if json.Unmarshal(inner, &answer) == nil && answer.Completed != nil && !*answer.Completed {
		if answer.Run != "" {
			return waitForIngestRun(baseURL, svc, answer.Run, stdout, t)
		}
		return waitForIndexed(baseURL, svc, stdout, t)
	}
	return confirmIndexed(hc, baseURL, svc, stdout)
}

// errIngestRequestBuild marks a failure to build the ingest request itself
// (never a daemon condition).
var errIngestRequestBuild = errors.New("build ingest request")

// callIngestTool POSTs one `ingest` tools/call with args and returns the raw
// JSON-RPC answer.
func callIngestTool(hc *http.Client, baseURL string, args map[string]any) ([]byte, error) {
	reqBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]any{"name": "ingest", "arguments": args},
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errIngestRequestBuild, err)
	}
	req, err := originHeader(baseURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errIngestRequestBuild, err)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

// confirmIndexed is init's last word on a finished ingest: the indexed probe,
// then the ok line with the service's counts, or the MANUAL line saying the
// index still has no symbols for svc.
func confirmIndexed(hc *http.Client, baseURL, svc string, stdout io.Writer) (bool, error) {
	ok, probeErr := daemonServiceIndexed(hc, baseURL, svc)
	if probeErr != nil {
		return false, probeErr
	}
	if !ok {
		fmt.Fprintf(stdout, "MANUAL: ingest ran but service %q has no symbols in the index yet — check the daemon logs\n", svc)
		return false, nil
	}
	printIngestOK(hc, baseURL, svc, stdout)
	return true, nil
}

// printIngestOK prints init's ok line for an indexed svc.
func printIngestOK(hc *http.Client, baseURL, svc string, stdout io.Writer) {
	svcs, _ := fetchServices(hc, baseURL)
	info := svcs[svc]
	langs := ""
	if info.langs != "" {
		langs = ", " + info.langs
	}
	fmt.Fprintf(stdout, "ok ingest (service %q indexed: %s%s)\n", svc, info.countText(), langs)
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

// hookCommand is the hook command line for one fathom subcommand: a silent
// no-op (exit 0, no output) when the binary is not on PATH, so a visitor
// who opens the repo without fathom installed sees no failing hooks. bin is
// the bare `fathom` (canonical) or an explicit --bin path; repointCommand
// maps the latter back to the former.
func hookCommand(bin, sub string) string {
	return "command -v " + bin + " >/dev/null 2>&1 || exit 0; " + bin + " " + sub
}

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
	async   bool   // registered non-blocking (RT-169: an observe-only binding on a harness that runs async hooks)
	optIn   string // the user setting that must be on before the row is written or expected (hookSpecWired)
}

// fathomHookSpecs is the Claude Code hook table, derived from the harness
// registry (ADR-029: `internal/harness` is the ONE source; hookSpecsFor in
// hookharness.go is the derivation). The bindings, in order:
//
//   - SessionStart (startup|resume|clear|compact) → session-hook, 10s.
//   - SessionEnd / PreCompact (manual|auto) → the continuity deposits
//     (vision 21 / docs/design/continuity-threads.md §3): one mechanical
//     item at the moments context is lost, one pointer before compaction.
//     Both write to disk and inject NOTHING, so their context cost is zero.
//   - PostToolUse (Edit|Write|MultiEdit) → edit-hook, 5s.
//   - PreToolUse ×3 → edit-hook --pre, 5s: guarded edits
//     (docs/design/guarded-edits.md §2.2/§2.5) on Edit|Write|MultiEdit and
//     on Bash, silent-by-default on any non-guarded file/command (a local
//     glob check before any daemon call — GI-8), plus the counting-only
//     discovery observer on Grep|Glob|Read (production-tokenomics.md §4,
//     discovery.go; zero daemon calls, zero injection). Three matcher
//     groups with ONE command each, because Claude Code's PreToolUse
//     matcher selects by tool name — ensureHookEntry's marker check is
//     scoped per matcher group (O-13's fix) so each wires independently.
//   - Stop → stop-hook, 10s.
//
// TestHookSpecsForClaude pins this derived table to the pre-seam literal.
var fathomHookSpecs = hookSpecsFor(harness.ClaudeCode)

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
		Async:   spec.async, // RT-169: the observer never blocks the tool call
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
	for _, spec := range fathomHookSpecsAll() { // detection: every row, opt-in ones included (RO-38)
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
// as ONE wiring spec: the instruction regions (instructionRegions: the
// AGENTS.md stanza and a root CLAUDE file's import), and the hook
// entries to ADD. Init is add-only for hooks, exactly as ensureHookEntry
// was: an entry already present in its matcher group is left alone (it may
// spell the shim, or a path a human chose), and an entry already wired at
// Claude Code's USER scope (~/.claude/settings.json — the scope doctor's
// checkHooksInstalled reads) is not duplicated at project scope, because
// Claude Code applies both scopes and the hook would fire twice. Every
// skip is returned as a note to print, never silent.
func initWiringSpec(root, service, binPath string) (wiring.Spec, []string) {
	targets, _ := initTargetsFor(harness.ClaudeCode, time.Time{})
	return initWiringSpecFor(root, service, binPath, targets, hooksScopeAuto)
}

// initWiringSpecFor is initWiringSpec for one harness's targets (HP-13):
// the stanza goes into the descriptor's declared instruction files with
// that descriptor's hook sentences (stanzaHookSentences), and the hook
// table is planned only when the descriptor's hooks are wired — today,
// Claude Code alone. initWiringSpec (the Claude Code instance) is
// byte-for-byte what it planned before the seam.
//
// hooksScope is resolveHooksScope's result: hooksScopeAuto (default) skips
// a spec already wired at Claude Code USER scope exactly as before this
// parameter existed; hooksScopeProject writes every fathom hook spec at
// PROJECT scope regardless of user scope, for a harness that only ever
// loads project settings (clean-room/CI `claude --restricted --settings
// <path>` — user-scope hooks are invisible to it, so a repo whose operator
// already wired fathom globally would otherwise ship a hookless cell).
func initWiringSpecFor(root, service, binPath string, targets initTargets, hooksScope string) (wiring.Spec, []string) {
	intentOn := intentChannelWired(root)
	spec := wiring.Spec{}
	regions, notes := instructionRegions(root, targets.StanzaID, service, intentOn)
	spec.Regions = append(spec.Regions, regions...)
	spec.Regions = append(spec.Regions, wiring.LocalZoneIgnoreRegion())
	spec.Keys = append(spec.Keys, settingKeys(targets.Settings)...)
	if !targets.Hooks {
		return spec, notes
	}
	if targets.ID != harness.ClaudeCode { // HP-18: the descriptor's own hook file (init_harnesshooks.go)
		keys, hookNotes := harnessHookKeys(root, targets.ID, binPath)
		spec.Keys = append(spec.Keys, keys...)
		return spec, append(notes, hookNotes...)
	}
	projectOnly := hooksScope == hooksScopeProject
	if projectOnly {
		notes = append(notes, "hooks: wired at project scope (--hooks-scope project)")
	}
	userWired, userReadable := userScopeHookKeys()
	present := projectHookEntries(root)
	for _, hs := range fathomHookSpecsAt(root) { // RO-38: opt-in rows only while root's config switches them on
		if !projectOnly && userReadable && userWired[hookSpecKeyOf(hs)] {
			notes = append(notes, "skip "+specLabel(hs)+" — already wired at user scope (~/.claude/settings.json); not duplicating at project scope")
			continue
		}
		if _, ok := present[hookSpecKeyOf(hs)]; ok {
			continue // add-only: present entries are not init's to rewrite
		}
		spec.Keys = append(spec.Keys, hookKeyFor(hs, hookCommand(binPath, hs.sub)))
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

// settingKeys renders a harness's instruction settings (Gemini CLI's
// context.fileName) as json-value wiring keys: receipted, recorded in the
// ledger, and a hand edit after fathom wrote one is drift (AG-6).
func settingKeys(settings []harness.InstructionSetting) []wiring.Key {
	var keys []wiring.Key
	for _, st := range settings {
		body, err := json.Marshal(st.Value)
		if err != nil {
			continue // a []string always marshals
		}
		keys = append(keys, wiring.Key{File: st.File, Kind: wiring.KindJSONValue, Path: st.Key, Body: string(body)})
	}
	return keys
}

// Claude Code hook scopes --hooks-scope (and FATHOM_HOOKS_SCOPE) select
// between: hooksScopeAuto is today's default (skip a spec already wired at
// USER scope); hooksScopeProject forces every fathom hook spec to PROJECT
// scope regardless of user scope.
const (
	hooksScopeAuto    = "auto"
	hooksScopeProject = "project"
)

// resolveHooksScope normalizes the --hooks-scope flag value to
// hooksScopeAuto or hooksScopeProject. An empty value (the flag's own
// default) or an explicit "auto" both fall back to the FATHOM_HOOKS_SCOPE
// env var — the flag has no way to tell "left at the default" apart from
// "spelled out explicitly", both being the string "auto" — so a harness or
// CI script can set the scope once via the env var for every `fathom init`
// invocation instead of threading a flag through every call site; only an
// explicit --hooks-scope=project always wins, since it names the value
// unambiguously.
func resolveHooksScope(flagValue string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(flagValue))
	switch v {
	case "", hooksScopeAuto:
		if strings.ToLower(strings.TrimSpace(os.Getenv("FATHOM_HOOKS_SCOPE"))) == hooksScopeProject {
			return hooksScopeProject, nil
		}
		return hooksScopeAuto, nil
	case hooksScopeProject:
		return hooksScopeProject, nil
	default:
		return "", fmt.Errorf("invalid --hooks-scope value %q (want %q or %q)", flagValue, hooksScopeAuto, hooksScopeProject)
	}
}

// userScopeHookKeys reads Claude Code's USER-scope settings
// (~/.claude/settings.json — doctor's checkHooksInstalled checks this same
// file and no other user-scope variant, so init mirrors that scope list
// exactly) and returns the (event, matcher, marker) keys of every fathomHookSpecs
// entry already wired there. ok is false when the file is absent or
// unreadable/unparsable — normal, not a finding; the caller then wires
// everything at project scope exactly as before this function existed.
func userScopeHookKeys() (map[hookSpecKey]bool, bool) {
	p := claudeUserSettingsFile()
	if p == "" {
		return nil, false
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, false
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, false
	}
	hooksObj, _ := parsed["hooks"].(map[string]any)
	out := map[hookSpecKey]bool{}
	for _, spec := range fathomHookSpecsAll() { // detection: every row, opt-in ones included (RO-38)
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
	for _, spec := range fathomHookSpecsAt("") {
		if userReadable && userWired[hookSpecKeyOf(spec)] {
			skipped = append(skipped, specLabel(spec))
			continue
		}
		command := hookCommand(binPath, spec.sub)
		if ensureHookEntry(hooksObj, spec, command) {
			changed = true
		}
	}
	root["hooks"] = hooksObj

	var mbuf bytes.Buffer
	enc := json.NewEncoder(&mbuf)
	enc.SetEscapeHTML(false) // keep a hook command's `>` readable
	enc.SetIndent("", "  ")
	if merr := enc.Encode(root); merr != nil {
		return nil, false, nil, fmt.Errorf("marshaling settings.json: %w", merr)
	}
	marshaled := mbuf.Bytes()
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
	if spec.async {
		newHook["async"] = true // RT-169: the agent's tool call never waits on it
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

// ─── d. the AGENTS.md stanza (and the CLAUDE file's import, instructionregions.go) ─

const (
	// fathomStanzaBegin / fathomStanzaEnd are the marker pair as `fathom
	// init` rendered it before marker format v3 (internal/wiring) — the
	// spelling every repository wired before INC-12 carries. A region
	// written or upgraded by the wiring engine keeps the same prefix and
	// the same end marker but carries the template version and the body
	// digest on the begin line ("<!-- fathom:begin v3 sha256:… -->"), so
	// anything LOOKING for a region must match fathomStanzaBeginPrefix, not
	// the v1 spelling; fathomStanza still renders the v1 form for callers
	// that want the bare template (doctor's fixtures, gen-agents-md).
	fathomStanzaBegin       = "<!-- fathom:begin -->"
	fathomStanzaBeginPrefix = "<!-- fathom:begin"
	fathomStanzaEnd         = "<!-- fathom:end -->"
	// fathomStanzaVersion is the template version recorded on the begin
	// marker. The body is compared byte-for-byte regardless, so this only
	// needs to move with the marker FORMAT.
	fathomStanzaVersion = "v3"
)

// fathomStanza returns the canonical fathom operating-contract block for
// service, wrapped in begin/end markers. It is the ONE source, emitted
// into AGENTS.md alone (ADR-059): Codex and Claude Code (≥2.1.277) read
// it natively, and a root CLAUDE file carries only an @AGENTS.md import.
// openingMovesTable is the first-2-3-calls table, named once: the stanza
// embeds it (below) and the subagent primer re-renders it (HP-26,
// sessionhook_events.go). A subagent that never read the stanza gets the
// same four lines, not a second dialect of them.
// The four chains below name ONLY the default eight-tool floor (surface
// reckoning F-86, SR-33, SR-34: TestStanzaNamesOnlyDefaultSurfaceTools) —
// a client registered at /rpc/agent cannot call anything else, so naming
// one here would teach a call that fails. fathomStanzaTemplate discloses
// the wider verb set too, but by DESCRIPTION only (never an exact
// snake_case tool name) — the config custodian's stanza_tools_served
// check (CC-14, internal/custodian/stanza.go) flags any backticked or
// bare underscore-bearing identifier in the stanza that the /rpc/agent
// registry does not serve, and an extended-only verb never does. Naming
// one by its exact identifier here would trip that check the same way
// the old arrow-chains did — this table AND the disclosure sentence must
// both stay identifier-clean of anything off the default floor.
const openingMovesTable = "Opening moves by task shape (first 2-3 calls, then edit):\n" +
	"- bug report / stack trace / failing test: search -> symbol_context -> read_source\n" +
	"- feature/change: search -> obligations -> symbol_context\n" +
	"- review/verify: review_diff -> obligations -> verify_plan\n" +
	"- refactor/delete: blast_radius -> review_diff -> verify_plan"

const fathomStanzaTemplate = "## fathom code intelligence\n" +
	"\n" +
	"This repo is indexed by fathom as service \"{{SERVICE}}\". Route by\n" +
	"question shape, on this session's default eight-tool MCP registration:\n" +
	"where a symbol is defined is `find_symbol`; who-calls it, briefly\n" +
	"(definition, a capped caller/callee list, tests) is `symbol_context`;\n" +
	"what-breaks is `blast_radius`; a composed review verdict over a diff is\n" +
	"`review_diff`; what-governs this repository is `obligations` with no\n" +
	"path (the root conventions); is a plan's box proven is `verify_plan`;\n" +
	"the current text at a cited line is `read_source`. A literal string, a\n" +
	"config value, a log line, or a document-versus-document contradiction\n" +
	"is `search`'s job (grep with a receipt), not the other tools'.\n" +
	"Read files to *edit* them, not to discover them. The wider verb set\n" +
	"(the full resolved caller graph, staleness and what-was-known, and\n" +
	"more) lives one hop away at `/rpc/extended` (or its `/rpc/companion`\n" +
	"alias); register that surface instead of `/rpc/agent` for a session\n" +
	"that needs it.\n" +
	"\n" +
	openingMovesTable + "\n" +
	"\n" +
	"The index refreshes itself: a read of a drifted service answers from\n" +
	"what it has and queues a re-ingest in the background: nothing to call.\n" +
	"\n" +
	"{{OBLIGATIONS}}\n" +
	"\n" +
	"{{CONTINUITY}}\n" +
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
// actually wired, and it names what the channel delivers (IV-4, O1134): the
// invariant ids of the surfaces guard.cue covers, never "matched invariants
// on every edit". The obligations corpus excludes docs/intent
// (TestObligationsCorpusNeverWidensToIntent), so the generated guard entries
// are the only edit-time channel; repo-wide invariants go to the primer.
const (
	obligationsPlain = "Obligations — governing rules for the file you are about to touch — are\n" +
		"injected automatically on every edit via the PostToolUse hook. Treat them\n" +
		"as binding, not advisory."
	obligationsWithIntent = "Obligations — governing rules for the file you are about to touch, plus the\n" +
		"invariant ids of the `docs/intent/` surfaces `.fathom/guard.cue` covers — are\n" +
		"injected at the edit via the hooks. Treat them as binding, not advisory."
	// continuityClaude is the continuity paragraph as every Claude Code
	// stanza has carried it (byte-identical: the stanza golden).
	continuityClaude = "Continuity: run `fathom prime` at session start if your harness runs no\n" +
		"SessionStart hook. It prints the same identity, freshness, live-example,\n" +
		"thread-brief, and custodian primer this hook injects on Claude Code.\n" +
		"File what the next session must know: `fathom thread note \"…\"`."
)

// stanzaHookSentences renders the two hook-naming paragraphs of the stanza
// for one registry id (HP-11, vision 25 §2.2 rule 3): the text never
// promises a channel the harness lacks. Claude Code (and "", the default)
// gets the pre-seam literals; a harness whose hooks are not wired gets the
// pull-tier wording — `obligations` before an edit, `fathom prime` at
// session start — naming the harness so the reader knows why.
func stanzaHookSentences(id string, intentOn bool) (obligations, continuity string) {
	if id == "" {
		id = harness.ClaudeCode
	}
	d, ok := harness.Lookup(id)
	// The plain sentences ("injected automatically on every edit") may be
	// written only when init ITSELF wired the hooks and a real session has
	// verified them. Two different failures otherwise, and rule 3 forbids
	// both: an unverified descriptor would be promising a channel nobody
	// has seen work, and a VERIFIED descriptor whose hook file init cannot
	// write (Codex 0.153.1's user-global layer, HP-34) would be promising
	// a channel that is real but not wired here yet.
	if !ok || (d.HooksWired() && !d.Unverified() && hasInitHookFile(d)) {
		obligations = obligationsPlain
		if intentOn {
			obligations = obligationsWithIntent
		}
		return obligations, continuityClaude
	}
	if d.HooksWired() { // wired, but either unverified (HP-18) or unwritable (HP-34)
		return unverifiedHookSentences(d, intentOn)
	}
	display := d.Display
	if intentOn {
		obligations = "Obligations — governing rules for the file you are about to touch — are\n" +
			"served by the `obligations` MCP tool: call it before you edit (no hook\n" +
			"injects them on " + display + "). Treat them as binding, not advisory. The\n" +
			"invariant ids of a `docs/intent/` surface are in the router's sensitive-surface table."
	} else {
		obligations = "Obligations — governing rules for the file you are about to touch — are\n" +
			"served by the `obligations` MCP tool: call it before you edit (no hook\n" +
			"injects them on " + display + "). Treat them as binding, not advisory."
	}
	continuity = "Continuity: run `fathom prime` at session start — no fathom SessionStart\n" +
		"hook is wired on " + display + ", so nothing injects the identity, freshness,\n" +
		"live-example, thread-brief, and custodian primer for you. File what the next\n" +
		"session must know: `fathom thread note \"…\"`."
	return obligations, continuity
}

// fathomStanzaBody is the stanza WITHOUT its markers — what goes between
// them, and what the wiring engine digests — for the default (Claude
// Code) harness.
func fathomStanzaBody(service string, intentOn bool) string {
	return fathomStanzaBodyFor(harness.ClaudeCode, service, intentOn)
}

// fathomStanzaBodyFor is fathomStanzaBody with the hook sentences of one
// registry id. One template, one body per harness; the Claude Code body
// is byte-identical to what every wired repository carries.
func fathomStanzaBodyFor(id, service string, intentOn bool) string {
	obligations, continuity := stanzaHookSentences(id, intentOn)
	s := strings.ReplaceAll(fathomStanzaTemplate, "{{SERVICE}}", service)
	s = strings.ReplaceAll(s, "{{OBLIGATIONS}}", obligations)
	return strings.ReplaceAll(s, "{{CONTINUITY}}", continuity)
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
		if !strings.HasPrefix(cl.Ref, projectdir.Guard.Rel()) {
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
	return stanzaRegionFor(harness.ClaudeCode, file, service, intentOn)
}

// stanzaRegionFor is stanzaRegion with one registry id's hook sentences.
func stanzaRegionFor(id, file, service string, intentOn bool) wiring.Region {
	return wiring.Region{
		File:           file,
		Syntax:         wiring.HTMLMarkers,
		Version:        fathomStanzaVersion,
		Body:           fathomStanzaBodyFor(id, service, intentOn),
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

// ─── e. line-budget fitting (O57 item 3) ──────────────────────────────────
//
// The stanza refresh must be idempotent in line count: re-rendering the
// fathom region over an existing region must never change a file's line
// count unless the stanza's OWN content changed, and when a router_thin
// budget applies (.fathom/concerns.cue, KindMaxLines) the generator trims
// the stanza's optional blocks to fit rather than tripping it — trading
// nothing safety-relevant away. Both properties are corollaries of the
// same design: fitStanzaBody is a pure function of (repo state, template,
// budget), so two consecutive calls over the same repo state necessarily
// pick the same trim level and render the same bytes, and a call that
// WOULD trip a budget picks a smaller trim level instead of writing
// anyway.

// fathomStanzaBodyTrimmed is fathomStanzaBodyFor with 0, 1, or 2 of its
// optional blocks removed. drop 0 is byte-identical to fathomStanzaBodyFor
// — the golden every wired repository carries
// (TestStanzaHookSentencesClaudeByteIdentical) — and this function must
// never change what that returns. drop 1 additionally removes the worked
// "opening moves" example (the live, concrete tool-call sequences — useful
// but not itself a rule). drop 2 additionally removes the Continuity
// paragraph (moot on a harness whose hooks are actually wired, which is
// exactly the state a repo carrying a router_thin budget is in). Every
// other block — the routing paragraph, the refresh-index paragraph, the
// Obligations paragraph, and the done-means-proven / verify block — is
// never dropped: those carry the stanza's actual instructions, not
// decoration.
func fathomStanzaBodyTrimmed(id, service string, intentOn bool, drop int) (body string, dropped []string) {
	body = fathomStanzaBodyFor(id, service, intentOn)
	if drop <= 0 {
		return body, nil
	}
	_, continuity := stanzaHookSentences(id, intentOn)
	if drop >= 1 {
		if block := openingMovesTable + "\n\n"; strings.Contains(body, block) {
			body = strings.Replace(body, block, "", 1)
			dropped = append(dropped, "live-example (the opening-moves table)")
		}
	}
	if drop >= 2 {
		if block := continuity + "\n\n"; strings.Contains(body, block) {
			body = strings.Replace(body, block, "", 1)
			dropped = append(dropped, "continuity paragraph")
		}
	}
	return body, dropped
}

// countTextLines is custodian's countLines (unexported there), applied to
// a string: the number of lines a file of this content would report to
// `wc -l` / a max_lines concern — every "\n" plus one more if the content
// does not already end with one. Empty content is zero lines.
func countTextLines(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}

// stanzaLineBudget reads .fathom/concerns.cue (both scopes, project
// governing — law 13) for a live, non-off KindMaxLines concern targeting
// file (repo-relative, e.g. "CLAUDE.md") and returns its line budget. ok
// is false when no such concern is registered, or the concern file itself
// cannot be loaded — fail-open, matching every other custodian consumer in
// this codebase: an unreadable or absent concerns.cue imposes no budget
// rather than blocking the write.
func stanzaLineBudget(root, file string) (lines int, ok bool) {
	concerns, errs := loadConcernsBoth(root)
	if len(errs) > 0 && len(concerns) == 0 {
		return 0, false
	}
	for _, c := range concerns {
		if c.Off || c.Check.Kind != custodian.KindMaxLines {
			continue
		}
		if filepath.ToSlash(c.Check.File) != file {
			continue
		}
		if !ok || c.Check.Lines < lines {
			lines, ok = c.Check.Lines, true
		}
	}
	return lines, ok
}

// fitStanzaBody computes the ONE body every file in files gets (since
// ADR-059, files is AGENTS.md alone; the fit stays per-list so a caller
// that names several files still gets one body) — the fit decision is made
// ONCE, by previewing each successively-more-trimmed candidate against
// every file in files that carries its own max_lines budget, and applied
// uniformly, never decided independently per file. A file with no budget
// concern imposes no constraint; a file that cannot be read imposes none
// either (fail-open — the write to that file, if any, surfaces its own
// error downstream). dropped names what the winning candidate removed,
// nil when nothing was.
func fitStanzaBody(root string, files []string, id, service string, intentOn bool) (body string, dropped []string) {
	type constraint struct {
		file   string
		budget int
	}
	var budgets []constraint
	for _, f := range files {
		if n, ok := stanzaLineBudget(root, f); ok {
			budgets = append(budgets, constraint{f, n})
		}
	}
	for drop := 0; drop <= 2; drop++ {
		body, dropped = fathomStanzaBodyTrimmed(id, service, intentOn, drop)
		fits := true
		for _, b := range budgets {
			content, _, perr := wiring.PreviewRegionFile(root, wiring.Region{
				File: b.file, Syntax: wiring.HTMLMarkers, Version: fathomStanzaVersion,
				Body: body, AdoptUntracked: true,
			})
			if perr != nil {
				continue // unreadable: imposes no constraint, same convention as stanzaLineBudget
			}
			if countTextLines(content) > b.budget {
				fits = false
				break
			}
		}
		if fits {
			return body, dropped
		}
	}
	// Every optional block is gone and a budget still would not hold —
	// return the maximally trimmed candidate anyway (the caller reports
	// what was dropped; refusing to write at all would be worse than a
	// stanza that stays over budget by less than it otherwise would).
	return body, dropped
}

// stanzaRegionsFit builds one wiring.Region per file in files, all
// carrying fitStanzaBody's single shared body. notes names what was
// trimmed to make it fit, for the caller to surface to the operator; nil
// when nothing was dropped.
func stanzaRegionsFit(root string, files []string, id, service string, intentOn bool) ([]wiring.Region, []string) {
	body, dropped := fitStanzaBody(root, files, id, service, intentOn)
	regions := make([]wiring.Region, 0, len(files))
	for _, f := range files {
		regions = append(regions, wiring.Region{
			File:           f,
			Syntax:         wiring.HTMLMarkers,
			Version:        fathomStanzaVersion,
			Body:           body,
			AdoptUntracked: true,
		})
	}
	var notes []string
	if len(dropped) > 0 {
		notes = append(notes, fmt.Sprintf(
			"stanza trimmed to fit a router_thin line budget (.fathom/concerns.cue): dropped %s",
			strings.Join(dropped, ", "),
		))
	}
	return regions, notes
}

// applyWiringSpec runs spec through internal/wiring's plan → apply and
// narrates the outcome per file in init's own vocabulary: "created X",
// "updated X (…)", "ok X (already present)", and — the law-15 outcome —
// "BLOCKED X — <why>", which names the drift and writes nothing to that
// file while the others still apply. The receipt lands in
// .fathom/receipts.jsonl; nothing here writes a byte outside the engine.
func applyWiringSpec(w *initWrites, root string, spec wiring.Spec, stdout io.Writer) (wiring.Result, error) {
	res, err := w.applyWiring(root, spec)
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
			fmt.Fprintf(stdout, "updated %s (%s)\n", f, appliedLabel(o.applied))
		case len(o.blocked) == 0:
			fmt.Fprintf(stdout, "ok %s (already present)\n", f)
		}
	}
	return res, nil
}

// appliedLabel names what an apply wrote into one file, in init's words.
func appliedLabel(items []wiring.Item) string {
	it := items[0]
	switch {
	case it.Kind == wiring.KindRegion && it.After == "":
		return "fathom region removed — an @AGENTS.md import is already there"
	case it.Kind == wiring.KindRegion && strings.HasPrefix(it.After, "@") && !isStanzaBody(it.After):
		if it.Present && isStanzaBody(it.Before) {
			return "the fathom stanza moved to AGENTS.md; this file keeps an @AGENTS.md import"
		}
		return "@AGENTS.md import"
	case it.Kind == wiring.KindRegion:
		return "fathom stanza"
	case !strings.Contains(it.Target, "|"):
		return "fathom setting " + it.Target
	default:
		return "fathom hooks added"
	}
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
	_, err = applyWiringSpec(nil, root, wiring.Spec{Regions: []wiring.Region{region}}, stdout)
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
# These (and api_key_file, for [decide]) are honored ONLY in this user file,
# never in a repository's .fathom/config.toml, which could otherwise run a
# command of its own choosing on your machine.
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
func writeStarterConfig(w *initWrites, path string, stdout io.Writer) error {
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); err == nil {
		fmt.Fprintln(stdout, "ok ~/.fathom/config.toml (already present)")
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := w.mkdirAll(filepath.Dir(path)); err != nil {
		return err
	}
	if err := w.writeFile(path, []byte(starterConfigTOML), 0o644); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "created ~/.fathom/config.toml (every profile commented out — see docs/configuration.md)")
	return nil
}

// ─── f. manual steps ───────────────────────────────────────────────────────

// docsURL is where an adopter reads fathom's docs: init runs in someone
// else's repository, where a repo-relative docs/ path does not exist.
const docsURL = "https://github.com/GeoffMillerAZ/fathom/blob/main/docs"

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
	fmt.Fprintln(stdout, "MANUAL: Claude Code (2.1.277 and later) and Codex read AGENTS.md for the operating contract (written above);")
	fmt.Fprintln(stdout, "MANUAL: a root CLAUDE.md, .claude/CLAUDE.md or CLAUDE.local.md carries only an @AGENTS.md import region —")
	fmt.Fprintln(stdout, "MANUAL: fathom never creates one; `fathom doctor` stanza_present says whether Claude Code will load AGENTS.md.")
	fmt.Fprintln(stdout, "MANUAL: register fathom as an MCP server for Codex too ("+docsURL+"/adoption/codex.md):")
	fmt.Fprintln(stdout, "MANUAL:   HTTP (the shared daemon above; check with `codex mcp list`):")
	fmt.Fprintf(stdout, "MANUAL:     codex mcp add fathom --url %s/rpc/agent\n", url)
	fmt.Fprintln(stdout, "MANUAL:   or stdio, a front that shares the same daemon:")
	fmt.Fprintf(stdout, "MANUAL:     codex mcp add fathom -- fathom serve-mcp --url %s/rpc/agent\n", url)
	printGateAndConfigManualSteps(stdout, service)
	fmt.Fprintln(stdout, "MANUAL: Codex loads hooks only from the user-global ~/.codex/hooks.json, which fathom never writes:")
	fmt.Fprintln(stdout, "MANUAL:   fathom harness hooks --harness codex   # prints the entries to merge; then trust them in /hooks")
}

// printGateAndConfigManualSteps is the harness-neutral half of the manual
// steps — the lefthook gates, the config.toml pointer, the scaffolded
// .fathom/ files — shared by printManualSteps (Claude Code) and
// printHarnessManualSteps (every other descriptor), byte for byte.
func printGateAndConfigManualSteps(stdout io.Writer, service string) {
	fmt.Fprintln(stdout, "MANUAL: add the verify gate to lefthook.yml's pre-push (fathom does not edit lefthook.yml):")
	fmt.Fprintln(stdout, "MANUAL:   pre-push:")
	fmt.Fprintln(stdout, "MANUAL:     commands:")
	fmt.Fprintln(stdout, "MANUAL:       fathom-verify:")
	fmt.Fprintln(stdout, "MANUAL:         run: |")
	fmt.Fprintf(stdout, "MANUAL:           fathom verify --service %s --plan docs/plans/<plan>.md --quiet\n", service)
	initPrePushConserve(stdout, service)
	fmt.Fprintln(stdout, "MANUAL: keep the index fresh after each commit, through the daemon (never a shell ingest")
	fmt.Fprintln(stdout, "MANUAL: against the daemon's store):")
	fmt.Fprintln(stdout, "MANUAL:   post-commit:")
	fmt.Fprintln(stdout, "MANUAL:     commands:")
	fmt.Fprintln(stdout, "MANUAL:       fathom-refresh:")
	fmt.Fprintf(stdout, "MANUAL:         run: fathom post-commit-hook --dir \"$PWD\" --service %s --quiet\n", service)
	if _, err := os.Stat(config.DefaultUserPath()); err == nil {
		fmt.Fprintln(stdout, "MANUAL: pick a provider — uncomment one profile in ~/.fathom/config.toml (or your project's")
		fmt.Fprintln(stdout, "MANUAL: .fathom/config.toml) and set default_profile; `fathom config --explain` shows what")
		fmt.Fprintln(stdout, "MANUAL: resolved and from where, `fathom providers` checks auth health at zero cost.")
	}
	fmt.Fprintln(stdout, "MANUAL: .fathom/guard.cue, .fathom/modules.cue and .fathom/capabilities.cue were scaffolded")
	fmt.Fprintln(stdout, "        commented (inert) — run `fathom capabilities` to see what is on and why; uncomment")
	fmt.Fprintln(stdout, "MANUAL: and commit to guard critical scopes ("+docsURL+"/design/guarded-edits.md) or bind a")
	fmt.Fprintln(stdout, "MANUAL: platform module ("+docsURL+"/design/platform-modules.md); neither applies until you do.")
}

// initPrePushConserve prints the MANUAL step that adds a non-blocking
// `fathom conserve` step to the adopter's own pre-push gate, on the pushed
// range (design surface-reckoning.md §6, SR-63; ADR-044 §6/CV-47). Today
// only this repo's own lefthook.yml runs conserve in pre-push (the
// refactor-conserve step); every other adopter following init's manual
// steps got the verify gate only. fathom init never edits lefthook.yml
// itself (a hand-curated file, printGateAndConfigManualSteps above); this
// makes conserve part of that same printed default instead of something an
// adopter has to already know to add. Advisory only: like the verify gate's
// neighbor steps, this always exits 0 for a finding — only an unresolvable
// ref or missing binary degrades, silently, per the fail-open convention
// (docs/design/coding-agent-excellence.md §5.3).
func initPrePushConserve(stdout io.Writer, service string) {
	fmt.Fprintln(stdout, "MANUAL:       fathom-conserve:")
	fmt.Fprintln(stdout, "MANUAL:         run: |")
	fmt.Fprintln(stdout, "MANUAL:           since=$(git rev-parse --abbrev-ref --symbolic-full-name '@{u}' 2>/dev/null)")
	fmt.Fprintln(stdout, "MANUAL:           [ -n \"$since\" ] || since=\"origin/main\"")
	fmt.Fprintf(stdout, "MANUAL:           fathom conserve --service %s --since \"$since\" --quiet || true\n", service)
}

// waitForIndexed polls the daemon until svc reads as indexed or t.pollFor
// passes, saying so first. It returns ok=false with its own MANUAL line when
// the wait runs out, and an error only when the daemon stops answering.
func waitForIndexed(baseURL, svc string, stdout io.Writer, t initIngestTiming) (bool, error) {
	fmt.Fprintf(stdout, "ingest still running on the daemon (a first index of a large repository can take minutes) — waiting up to %s for %q to read as indexed\n", t.pollFor, svc)
	return waitForIndexedUntil(baseURL, svc, stdout, t, time.Now().Add(t.pollFor))
}

// waitForIndexedUntil is waitForIndexed's poll, ending at deadline.
func waitForIndexedUntil(baseURL, svc string, stdout io.Writer, t initIngestTiming, deadline time.Time) (bool, error) {
	hc := &http.Client{Timeout: 10 * time.Second}
	for {
		ok, err := daemonServiceIndexed(hc, baseURL, svc)
		if err != nil && errors.Is(err, syscall.ECONNREFUSED) {
			return false, err
		}
		if ok {
			printIngestOK(hc, baseURL, svc, stdout)
			return true, nil
		}
		if time.Now().After(deadline) {
			fmt.Fprintf(stdout, "MANUAL: service %q is still not indexed after %s — the ingest may still be running; `fathom daemon ps` and the daemon log show it, and re-running `fathom init` confirms it\n", svc, t.pollFor)
			return false, nil
		}
		time.Sleep(t.pollEvery)
	}
}

// ingestRunAnswer is the ingest tool's run form's answer (EW-66,
// internal/mcp ingestRunState): state queued, running, done or unknown;
// ok and error once done.
type ingestRunAnswer struct {
	State string `json:"state"`
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}

// waitForIngestRun polls `ingest {service, run}` (EW-70) until the run the
// daemon's past-budget answer named reads done, or t.pollFor passes: done
// and ok confirms the index (confirmIndexed); done and not ok (failed,
// refused, dropped by the gate) prints a MANUAL line naming the daemon's
// error and returns at once; unknown (the daemon restarted since; an
// interrupted ingest resumes on its next start) falls back to the indexed
// probe until the same deadline. Each poll queues nothing. A poll the
// daemon does not answer is retried; only a refused connection is
// daemon-down.
func waitForIngestRun(baseURL, svc, run string, stdout io.Writer, t initIngestTiming) (bool, error) {
	fmt.Fprintf(stdout, "ingest still running on the daemon as run %s (a first index of a large repository can take minutes) — waiting up to %s for it to finish\n", run, t.pollFor)
	hc := &http.Client{Timeout: 10 * time.Second}
	deadline := time.Now().Add(t.pollFor)
	for {
		raw, err := callIngestTool(hc, baseURL, map[string]any{"service": svc, "run": run})
		if err != nil && errors.Is(err, syscall.ECONNREFUSED) {
			return false, err
		}
		var st ingestRunAnswer
		if err == nil {
			if inner, rpcErr := unwrapRPCResult(raw); rpcErr == nil {
				_ = json.Unmarshal(inner, &st)
			}
		}
		switch st.State {
		case "done":
			if st.OK {
				return confirmIndexed(hc, baseURL, svc, stdout)
			}
			reason := st.Error
			if reason == "" {
				reason = "no reason given"
			}
			fmt.Fprintf(stdout, "MANUAL: the daemon's ingest of %q (run %s) failed: %s — fix the cause and re-run `fathom init`; the daemon log has the detail\n", svc, run, reason)
			return false, nil
		case "unknown":
			fmt.Fprintf(stdout, "the daemon has no record of run %s (it restarted since; an interrupted ingest resumes on its next start) — waiting for %q to read as indexed\n", run, svc)
			return waitForIndexedUntil(baseURL, svc, stdout, t, deadline)
		}
		if time.Now().After(deadline) {
			fmt.Fprintf(stdout, "MANUAL: run %s of %q has not finished after %s — the ingest may still be running; `fathom daemon ps` and the daemon log show it, and re-running `fathom init` confirms it\n", run, svc, t.pollFor)
			return false, nil
		}
		time.Sleep(t.pollEvery)
	}
}

// initRunner runs `fathom init` (registered below; EW-45).
type initRunner struct{}

func (initRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cmdInit(args, stdout, stderr)
}

func init() {
	declareVerb(verb{Name: "init", Group: "operate", Order: 90, Run: initRunner{},
		Summary: "One-command repo wiring: .fathomignore, ingest, Claude Code hooks, the AGENTS.md stanza (a CLAUDE file gets an @AGENTS.md import)"})
}

// initHarnessUsage is the value (a package var, so the flag definition does
// no work before Parse) of initHarnessFlagUsage.
var initHarnessUsage = initHarnessFlagUsage()

// initHarnessFlagUsage is --harness's help text. The accepted ids are read
// from the table initTargetsFor parses (the harness registry plus "none"), so
// help cannot drift from what init accepts.
func initHarnessFlagUsage() string {
	ids := make([]string, 0, len(harness.IDs())+1)
	for _, id := range harness.IDs() {
		if id == harness.ClaudeCode {
			id += " (default)"
		}
		ids = append(ids, id)
	}
	ids = append(ids, initHarnessNone)
	return "harness to wire: " + strings.Join(ids, " | ") + " — init writes only that descriptor's declared vehicles and prints its own 'mcp add' line; hooks are wired only for harnesses whose hook codec is built in"
}
