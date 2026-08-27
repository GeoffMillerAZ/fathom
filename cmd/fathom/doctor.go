package main

// doctor.go — DI-1 (docs/plans/doctor-and-impact.md): `fathom doctor` is the
// deterministic wiring-diagnosis checklist for a fathom-adopting repo. It
// answers "is fathom actually wired here, end to end?" with one command
// instead of the manual, multi-doc "you know it works when…" checklists in
// docs/adoption/claude-code.md §14 and docs/adoption/codex.md §7.
//
// Every check follows the C4-5 honesty discipline (docs/plans/review-verb.md):
// an uncheckable state (unreadable config, unreachable daemon, unresolvable
// home directory) is reported `skipped` with a reason and, where possible, a
// manual instruction — NEVER a guessed `pass`. A real, observable problem is
// `fail` with a one-command `remedy`. doctor is strictly read-only: it never
// writes a file, calls a mutating MCP tool, or restarts anything.
//
// Where a check can reuse an existing surface instead of duplicating logic,
// it does: config.Load for CUE validation, the `obligations`/`server_stats`
// MCP tools over the same HTTP plumbing verify.go/init.go already use,
// resolveCallLogPath/readCallLog from instrument_adoption.go (same package,
// no import needed), and fathomHookSpecs/fathomStanzaBegin/fathomStanzaEnd/
// skillMarker from init.go/skillsinstall.go (read-only reuse of their
// detection conventions, never their write paths).
import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	heartbeat "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/heartbeat"
	skillsdist "github.com/geoffmilleraz/fathom/dist/agents/skills"
	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	toml "github.com/pelletier/go-toml/v2"
)

// doctorStatus is one check's verdict.
type doctorStatus string

const (
	doctorPass    doctorStatus = "pass"
	doctorFail    doctorStatus = "fail"
	doctorSkipped doctorStatus = "skipped"
	// doctorWarn is phase O's addition (docs/design/platform-modules.md
	// §4 deliverable 4 / guarded-edits.md §2.4): a real, observable
	// condition worth a human's attention (a dead guard, a sub-servable
	// bound module) that must NEVER fail the gate — a config-quality
	// signal, not a wiring-broken verdict. doctorExitCode treats it
	// exactly like doctorPass for exit-code purposes; it is visually and
	// machine-distinguishable (statusLabel/JSON) so it is never silently
	// indistinguishable from a real pass either.
	doctorWarn doctorStatus = "warn"
)

// doctorCheck is one row of the checklist: {name, status, reason, remedy}
// per the plan's design section. Remedy is always a single command (or
// short instruction), never a paragraph — it is omitted entirely on pass.
type doctorCheck struct {
	Name   string       `json:"name"`
	Status doctorStatus `json:"status"`
	Reason string       `json:"reason"`
	Remedy string       `json:"remedy,omitempty"`
}

// cmdDoctor implements the "doctor" subcommand.
//
// Exit codes:
//
//	0 — every check passed (skipped checks do not fail the gate; they are
//	    honestly-uncheckable states, not failures)
//	1 — at least one check failed
//	2 — doctor itself could not run (bad flags, unresolvable --path/cwd) —
//	    an operational error, not a verdict on the repo's wiring
func cmdDoctor(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("doctor", stderr)
	path := fs.String("path", "", "repo root to diagnose (default: cwd)")
	harnessFlag := fs.String("harness", "auto", "claude | codex | auto — which harness's wiring to check")
	service := fs.String("service", "", "service ID (default: resolved from --path, the same identity the hooks use)")
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL")
	jsonOut := fs.Bool("json", false, "emit JSON instead of a human table")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "error: unexpected argument %q\n", fs.Arg(0))
		return 2
	}
	switch *harnessFlag {
	case "claude", "codex", "auto":
	default:
		fmt.Fprintf(stderr, "error: --harness must be claude, codex, or auto (got %q)\n", *harnessFlag)
		return 2
	}

	root := *path
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "error: resolving cwd: %v\n", err)
			return 2
		}
		root = wd
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		fmt.Fprintf(stderr, "error: resolving --path %q: %v\n", root, err)
		return 2
	}
	root = absRoot

	// K5's second half (EP-24): doctor is one of the two instruments a
	// session can reach for after seeing the attention line. Law 7 — an
	// instrument that leaves no trace cannot be shown to have been used.
	rpcOrigin = "doctor"
	emitBeat("doctor", "run", nil)

	// svc here is display identity only (the table header). The
	// obligations_served check does its own candidate resolution against
	// the daemon's actual service list — the same all-candidates probing
	// the hooks perform (edithook.go's candidate loop) — so the header's
	// first-candidate guess can never silently misdirect a remedy.
	explicitSvc := strings.ToLower(*service)
	svc := explicitSvc
	if svc == "" {
		cands := serviceCandidates(root)
		svc = strings.ToLower(cands[0].name)
	}

	harness := resolveDoctorHarness(*harnessFlag, root)
	checks := runDoctorChecks(root, explicitSvc, harness, *url)

	if *jsonOut {
		printDoctorJSON(stdout, harness, svc, root, checks)
	} else {
		printDoctorTable(stdout, harness, svc, root, checks)
	}

	return doctorExitCode(checks)
}

// resolveDoctorHarness resolves "auto" to a concrete harness by looking for
// each one's wiring evidence; an explicit claude/codex always wins outright.
// Detection never errors — an unresolvable auto default falls back to
// "claude" (the primary, dogfooded path), same spirit as init's default
// service-name resolution never refusing to run.
func resolveDoctorHarness(flagVal, root string) string {
	if flagVal == "claude" || flagVal == "codex" {
		return flagVal
	}
	if fileExists(filepath.Join(root, ".claude", "settings.json")) {
		return "claude"
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" && fileExists(filepath.Join(home, ".claude.json")) {
		return "claude"
	}
	if fileExists(filepath.Join(root, ".codex", "config.toml")) {
		return "codex"
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" && fileExists(filepath.Join(home, ".codex", "config.toml")) {
		return "codex"
	}
	return "claude"
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// runDoctorChecks runs every check in a fixed, deterministic order (rough
// dependency order: registration/reachability first, then the
// injection/gate stack, then serving state) and returns every row —
// doctor never short-circuits on an early failure, so one bad check never
// hides the rest of the diagnosis.
func runDoctorChecks(root, explicitSvc, harness, baseURL string) []doctorCheck {
	hc := &http.Client{Timeout: 5 * time.Second}

	checks := []doctorCheck{
		checkMCPRegistration(root, harness, baseURL),
	}
	daemonUp, daemonCheck := checkDaemonReachable(hc, baseURL)
	checks = append(
		checks,
		daemonCheck,
		checkBinaryFresh(root),
		checkHooksInstalled(root, harness),
		checkLefthookGates(root),
		checkHooksFiring(hc, baseURL, daemonUp, root, harness),
		checkStanzaPresent(root),
		checkSkillsInstalled(root, harness),
		checkConfigValid(root),
		checkObligationsServed(hc, baseURL, root, explicitSvc, daemonUp),
		checkSurfaceTraffic(hc, baseURL, daemonUp, root),
		// Phase O (docs/design/platform-modules.md §4 / guarded-edits.md
		// §2.4): config-validity first, then the checks that need it.
		checkBindingFile(root),
		checkGuardFile(root),
		checkBindingModulesMounted(hc, baseURL, root, daemonUp),
		checkBindingModuleReadiness(hc, baseURL, root, daemonUp),
		checkGuardDeadScopes(hc, baseURL, root, explicitSvc, daemonUp),
		checkGuardIncludeModulesUnbound(root),
		// Phase P (docs/design/production-tokenomics.md / scoped-config.md):
		// env-driven honesty checks first, then scoped-config disclosure.
		checkChannelsEnv(),
		checkBinaryOnPath(),
		checkIdentCacheEnv(),
		checkUserScopeIgnore(root),
		checkUserScopeGuard(),
		checkSessionLedgerTokens(),
		// Phase Q (docs/design/observability-surfaces.md §8 item 3).
		checkMetricsTier(),
		// Phase R (docs/plans/cpp-toolchain.md CT-7).
		checkCppToolchain(root),
		// Phase T (docs/design/capability-registry.md): the one row that
		// answers "why isn't X happening here?" — last, because it reads
		// every fact the rows above diagnose individually.
		checkCapabilities(baseURL, root, explicitSvc),
		// CC-6 (docs/plans/custodian.md): the config custodian — are this
		// repo's registered config contracts still true?
		checkConcerns(root),
		checkIntent(root),
		checkUnprovenClaims(root, baseURL, explicitSvc),
	)
	return checks
}

// doctorExitCode implements the 0/1 half of the exit contract (2 is
// returned earlier, before any check runs, for a doctor-itself failure).
func doctorExitCode(checks []doctorCheck) int {
	for _, c := range checks {
		if c.Status == doctorFail {
			return 1
		}
	}
	return 0
}

// ─── 1. MCP registration ───────────────────────────────────────────────────

// checkMCPRegistration confirms the harness's own MCP client config points
// fathom at /rpc/agent (or its /rpc/companion alias) — never guessed: an
// unreadable or unparsable config is `skipped` with a manual fallback
// instruction, not a clean pass.
func checkMCPRegistration(root, harness, baseURL string) doctorCheck {
	switch harness {
	case "codex":
		return checkCodexMCPRegistration(root)
	default:
		return checkClaudeMCPRegistration(baseURL)
	}
}

func registeredAtAgentSurface(url string) bool {
	trimmed := strings.TrimRight(url, "/")
	return strings.HasSuffix(trimmed, "/rpc/agent") || strings.HasSuffix(trimmed, "/rpc/companion")
}

func checkClaudeMCPRegistration(baseURL string) doctorCheck {
	const name = "mcp_registration"
	remedy := fmt.Sprintf("claude mcp add --transport http --scope user fathom %s/rpc/agent", baseURL)

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "cannot resolve home directory to read ~/.claude.json — check manually: claude mcp get fathom", Remedy: remedy}
	}
	path := filepath.Join(home, ".claude.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("%s not readable (%v) — check manually: claude mcp get fathom", path, err), Remedy: remedy}
	}
	var doc struct {
		McpServers map[string]struct {
			Type string `json:"type"`
			URL  string `json:"url"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("%s did not parse as JSON (%v)", path, err), Remedy: remedy}
	}
	entry, ok := doc.McpServers["fathom"]
	if !ok {
		return doctorCheck{Name: name, Status: doctorFail, Reason: fmt.Sprintf("no \"fathom\" entry in %s's mcpServers", path), Remedy: remedy}
	}
	if entry.Type != "http" || !registeredAtAgentSurface(entry.URL) {
		return doctorCheck{Name: name, Status: doctorFail, Reason: fmt.Sprintf("fathom is registered in %s but not at /rpc/agent (type=%q url=%q)", path, entry.Type, entry.URL), Remedy: remedy}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("registered in %s at %s", path, entry.URL)}
}

// codexMCPEntry mirrors the subset of Codex's [mcp_servers.<name>] TOML
// shape doctor needs (docs/adoption/codex.md §2): `url` selects the HTTP
// transport, `command` the stdio transport.
type codexMCPEntry struct {
	Command string `toml:"command"`
	URL     string `toml:"url"`
}

type codexConfigDoc struct {
	McpServers map[string]codexMCPEntry `toml:"mcp_servers"`
}

func checkCodexMCPRegistration(root string) doctorCheck {
	const name = "mcp_registration"
	remedy := `add [mcp_servers.fathom] with url = "http://127.0.0.1:7575/rpc/agent" to ~/.codex/config.toml (docs/adoption/codex.md §2)`

	var candidates []string
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidates = append(candidates, filepath.Join(home, ".codex", "config.toml"))
	}
	candidates = append(candidates, filepath.Join(root, ".codex", "config.toml"))

	var readAny bool
	var parseErr error
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		readAny = true
		var doc codexConfigDoc
		if err := toml.Unmarshal(data, &doc); err != nil {
			parseErr = fmt.Errorf("%s: %w", path, err)
			continue
		}
		entry, ok := doc.McpServers["fathom"]
		if !ok {
			continue
		}
		switch {
		case entry.URL != "" && registeredAtAgentSurface(entry.URL):
			return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("registered in %s at %s", path, entry.URL)}
		case entry.URL != "":
			return doctorCheck{Name: name, Status: doctorFail, Reason: fmt.Sprintf("fathom is registered in %s but not at /rpc/agent (url=%q)", path, entry.URL), Remedy: remedy}
		case entry.Command != "":
			return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("registered in %s via stdio (command=%q) — the /rpc/agent check does not apply to a stdio transport", path, entry.Command)}
		}
	}
	if !readAny {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "no readable ~/.codex/config.toml or .codex/config.toml — verify manually: codex mcp list (docs/adoption/codex.md §2)", Remedy: remedy}
	}
	if parseErr != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("could not parse Codex config: %v", parseErr), Remedy: remedy}
	}
	return doctorCheck{Name: name, Status: doctorFail, Reason: "no [mcp_servers.fathom] entry found in any readable Codex config", Remedy: remedy}
}

// ─── 2. daemon reachable ────────────────────────────────────────────────────

func checkDaemonReachable(hc *http.Client, baseURL string) (bool, doctorCheck) {
	const name = "daemon_reachable"
	remedy := "launchctl kickstart -k gui/$(id -u)/com.geoffmilleraz.fathom-http (or: fathom serve-http --store workspace/db/fathom.sqlite)"

	resp, err := hc.Get(baseURL + "/healthz")
	if err != nil {
		return false, doctorCheck{Name: name, Status: doctorFail, Reason: fmt.Sprintf("GET %s/healthz: %v", baseURL, err), Remedy: remedy}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, doctorCheck{Name: name, Status: doctorFail, Reason: fmt.Sprintf("GET %s/healthz returned %d", baseURL, resp.StatusCode), Remedy: remedy}
	}
	return true, doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("%s/healthz -> ok", baseURL)}
}

// ─── 3. binary freshness ────────────────────────────────────────────────────

// fathomModulePath is fathom's own go.mod module path — the one signal
// isFathomSourceTree needs to tell "this IS the fathom checkout" apart from
// every other repo doctor might be pointed at.
const fathomModulePath = "github.com/geoffmilleraz/fathom"

// isFathomSourceTree reports whether root is the fathom checkout itself
// (its go.mod declares fathomModulePath), never a guess: it is the ONLY
// repo where comparing a fathom binary's embedded revision, or resolving a
// daemon-relative call-log path, against root is meaningful. O23
// (docs/dogfood-observations.md): doctor was written and exercised only
// inside fathom's own repo, where "the repo being doctored" and "the repo
// the binary/daemon came from" happen to be the same directory — checked
// here so binary_fresh (this section) and the call-log resolution
// (checkHooksFiring, below) never again conflate the two in an adopter
// repo. A missing or unreadable go.mod is treated as "not fathom" (the
// overwhelmingly common case for anything that is not this repo), never a
// guessed match.
func isFathomSourceTree(root string) bool {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if mod, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.TrimSpace(mod) == fathomModulePath
		}
	}
	return false
}

// doctorBinaryRevision returns the running `fathom doctor` binary's own
// embedded VCS revision and dirty flag (version.go's vcsInfo, no ldflags
// needed). A package var so tests can simulate a known build identity
// instead of depending on `go test`'s own build info.
var doctorBinaryRevision = func() (rev string, dirty, ok bool) {
	r, _, modified := vcsInfo()
	return r, modified, r != ""
}

// checkBinaryFresh compares the currently-running binary's own build
// revision against `git rev-parse HEAD` of the checked repo — the same
// one-line doctrine CLAUDE.md's "Dogfood" section and
// docs/adoption/claude-code.md §13 both document by hand
// (`fathom version` vs `git rev-parse HEAD`). This is a LOCAL check: it
// says whether the binary you just ran doctor with is current — the best
// available proxy since there is no remote "ask the daemon its version"
// RPC today — and every PASS says so explicitly (the daemon's installed
// copy stays unverified). A matching stamp is additionally NOT conclusive
// in two verified-detectable cases, both annotated when present:
// vcs.modified (a dirty build's stamp names the commit, not the code) and
// enclosing-checkout stamp poisoning (a `go build` run inside a linked
// worktree NESTED under this checkout embeds THIS checkout's HEAD, not the
// worktree's — traced live: `go build -x` from such a worktree runs every
// `git` command against the enclosing checkout's root).
//
// O23 (docs/dogfood-observations.md): the binary is fathom's own build
// artifact, so "is it fresh" is a question about the FATHOM source tree's
// HEAD — meaningless anywhere else. Live failure this fixes: "binary @
// a0b74df3 does not match HEAD 061afdba", where 061afdba was an ADOPTER
// repo's HEAD, not fathom's — comparing fathom's build revision to a
// different project's commit history can never be meaningful, so this
// check now only runs when root IS the fathom checkout; every other repo
// gets an honest SKIP instead of a guaranteed-wrong FAIL.
func checkBinaryFresh(root string) doctorCheck {
	const name = "binary_fresh"
	remedy := "task deploy (signs the rebuilt binary, installs it via temp+rename, restarts the daemon, and asserts it serves HEAD) — never a raw cp over the running executable: an unsigned copy loses the TCC grant and cp truncates a live binary (O25; docs/adoption/claude-code.md §5)"

	if !isFathomSourceTree(root) {
		return doctorCheck{
			Name: name, Status: doctorSkipped,
			Reason: fmt.Sprintf("binary freshness is only checkable from the fathom source tree (%s's go.mod does not declare %s) — compare `fathom version` to the daemon's deployed commit by hand, or run doctor --path against a fathom checkout", root, fathomModulePath),
		}
	}

	rev, dirty, ok := doctorBinaryRevision()
	if !ok {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "this binary carries no embedded VCS revision (built without -buildvcs, or outside a git checkout) — compare `fathom version` to `git rev-parse HEAD` by hand", Remedy: remedy}
	}

	head, err := doctorGitHead(root)
	if err != nil || head == "" {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("could not resolve git HEAD at %s: %v", root, err), Remedy: remedy}
	}

	if strings.HasPrefix(head, rev) || strings.HasPrefix(rev, head) {
		reason := fmt.Sprintf("binary @ %s matches HEAD %s — local binary only, the daemon's installed copy is unverified", shortSHA(rev), shortSHA(head))
		if dirty {
			reason += "; vcs.modified=true (built from a locally modified tree — the stamp names the commit, not the exact code)"
		}
		if n := nestedLinkedWorktrees(root); n > 0 {
			reason += fmt.Sprintf("; note: %d linked worktree(s) are nested under this checkout — a `go build` run inside one stamps THIS checkout's HEAD (enclosing-checkout resolution), so a matching stamp does not prove the binary was built from this tree's code", n)
		}
		return doctorCheck{Name: name, Status: doctorPass, Reason: reason}
	}

	reason := fmt.Sprintf("binary @ %s does not match HEAD %s", shortSHA(rev), shortSHA(head))
	// Worktree-stamp caveat (this repo's own history — CLAUDE.md's
	// "Dogfood" section, docs/dogfood-observations.md O3): only asserted
	// when it can actually be verified — a linked git worktree's own .git
	// is a FILE (pointing at the main repo's gitdir), never a directory.
	// Comparing THIS binary's embedded revision against a DIFFERENT
	// worktree's HEAD is apples-to-oranges, not genuine staleness, so the
	// note is appended only when that condition is independently true —
	// never asserted speculatively.
	if isGitWorktree(root) {
		reason += " (note: this path is a linked git worktree — if the binary was built from a different worktree of the same repo, a mismatch may reflect worktree divergence rather than a stale deploy)"
	}
	return doctorCheck{Name: name, Status: doctorFail, Reason: reason, Remedy: remedy}
}

func doctorGitHead(root string) (string, error) {
	out, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// isGitWorktree reports whether root is a linked git worktree (its .git is
// a file, "gitdir: <path>", rather than the main repo's .git directory).
func isGitWorktree(root string) bool {
	info, err := os.Stat(filepath.Join(root, ".git"))
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// nestedLinkedWorktrees counts linked worktrees whose paths sit UNDER root
// (e.g. .claude/worktrees/agent-*). These are the stamp-poisoning hazard
// checkBinaryFresh's doc comment describes: `go build -buildvcs` from
// inside such a worktree resolves the VCS root to the ENCLOSING checkout
// and embeds its HEAD, so a binary carrying root's HEAD may actually have
// been compiled from a nested worktree's different code. Best-effort: any
// git failure reports zero (the note is additive, never load-bearing).
func nestedLinkedWorktrees(root string) int {
	out, err := exec.Command("git", "-C", root, "worktree", "list", "--porcelain").Output()
	if err != nil {
		return 0
	}
	self := root
	if r, err := filepath.EvalSymlinks(root); err == nil {
		self = r
	}
	sep := string(filepath.Separator)
	n := 0
	for _, line := range strings.Split(string(out), "\n") {
		p, ok := strings.CutPrefix(line, "worktree ")
		if !ok {
			continue
		}
		rp := p
		if r, err := filepath.EvalSymlinks(p); err == nil {
			rp = r
		}
		if rp != self && strings.HasPrefix(rp, self+sep) {
			n++
		}
	}
	return n
}

func shortSHA(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

// ─── 4. hooks installed (Claude Code only) ─────────────────────────────────

// checkHooksInstalled looks for the fathom hooks across EVERY settings
// scope Claude Code merges — project .claude/settings.json, project
// .claude/settings.local.json, and user ~/.claude/settings.json — because
// hooks wired at user scope (a perfectly good wiring) must never read as a
// project-scope FAIL whose remedy would install duplicates (every
// PostToolUse would then fire twice).
//
// When a hook is missing from every readable scope, the contradiction
// probe asks whether that SPECIFIC hook variant has firing evidence
// somewhere doctor cannot see — using whichever log can actually
// distinguish it from its siblings. edit-hook's PostToolUse invocation and
// its two PreToolUse ("--pre") matcher groups all share one CLI and one
// call-log origin ("edit-hook"), so call-log traffic from one can never be
// treated as evidence for another — only the hooks beat log's phase/target
// dimension (edithook.go/edithook_pre.go) tells them apart. session-hook
// and stop-hook are each their own CLI with no ambiguous sibling, so their
// call-log origin IS variant-specific evidence on its own.
//
// A missing hook with no evidence anywhere is genuinely missing —
// `fathom init` (idempotent, and — since it now also reads user scope
// before writing — safe even when some OTHER hook is wired there) is the
// correct remedy. A missing hook WITH evidence is wired at a scope doctor
// cannot read (managed settings, e.g.); prescribing `fathom init` for that
// one would install a project-scope duplicate.
func checkHooksInstalled(root, harness string) doctorCheck {
	const name = "hooks_installed"
	if harness != "claude" {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "Codex has no SessionStart/PostToolUse/Stop hook mechanism (docs/adoption/codex.md §6) — nothing to check"}
	}

	type scopeFile struct{ label, path string }
	scopes := []scopeFile{
		{"project", filepath.Join(root, ".claude", "settings.json")},
		{"project-local", filepath.Join(root, ".claude", "settings.local.json")},
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		scopes = append(scopes, scopeFile{"user", filepath.Join(home, ".claude", "settings.json")})
	}

	wiredScopes := map[hookSpecKey][]string{} // spec key (event+matcher) -> scope labels that wire it
	var scopeNotes []string
	for _, sc := range scopes {
		data, err := os.ReadFile(sc.path)
		if err != nil {
			continue // an absent scope file is normal, not a finding
		}
		var parsed map[string]any
		if err := json.Unmarshal(data, &parsed); err != nil {
			scopeNotes = append(scopeNotes, fmt.Sprintf("%s scope %s did not parse as JSON (%v) — hooks wired there are invisible to this check", sc.label, sc.path, err))
			continue
		}
		hooksObj, _ := parsed["hooks"].(map[string]any)
		for _, spec := range fathomHookSpecs {
			if hookMarkerPresent(hooksObj, spec) {
				k := hookSpecKeyOf(spec)
				wiredScopes[k] = append(wiredScopes[k], sc.label)
			}
		}
	}

	var missing []hookSpec
	labelSet := map[string]bool{}
	var labels []string
	for _, spec := range fathomHookSpecs {
		found := wiredScopes[hookSpecKeyOf(spec)]
		if len(found) == 0 {
			missing = append(missing, spec)
			continue
		}
		for _, l := range found {
			if !labelSet[l] {
				labelSet[l] = true
				labels = append(labels, l)
			}
		}
	}

	if len(missing) == 0 {
		reason := fmt.Sprintf("all %d fathom hooks wired (%s scope)", len(fathomHookSpecs), strings.Join(labels, " + "))
		if len(scopeNotes) > 0 {
			reason += "; " + strings.Join(scopeNotes, "; ")
		}
		return doctorCheck{Name: name, Status: doctorPass, Reason: reason}
	}

	checked := "checked project .claude/settings.json, .claude/settings.local.json, user ~/.claude/settings.json"

	var missingLabels []string
	var evidenced []string // "<label> (evidenced by <source>: N call(s))"
	var trulyMissing []string
	for _, spec := range missing {
		label := specLabel(spec)
		missingLabels = append(missingLabels, label)
		if n, source := hookVariantSeenEvidence(root, spec); n > 0 {
			evidenced = append(evidenced, fmt.Sprintf("%s (evidenced by %s: %d call(s))", label, source, n))
		} else {
			trulyMissing = append(trulyMissing, label)
		}
	}

	switch {
	case len(evidenced) > 0 && len(trulyMissing) == 0:
		// Every missing hook has firing evidence elsewhere — wired at a
		// scope doctor cannot read. Never prescribe init: it would install
		// project-scope duplicates for hooks that already fire.
		reason := fmt.Sprintf("no readable settings scope wires hook(s) %s (%s), yet: %s — evidently wired at a scope doctor cannot read. Running `fathom init` now would install project-scope DUPLICATES for these: every hook would fire twice.",
			strings.Join(missingLabels, ", "), checked, strings.Join(evidenced, "; "))
		if len(scopeNotes) > 0 {
			reason += " " + strings.Join(scopeNotes, "; ")
		}
		return doctorCheck{Name: name, Status: doctorFail, Reason: reason, Remedy: "find the scope that wires them (e.g. read ~/.claude/settings.json, or the harness's hook settings UI) before changing anything — do not run `fathom init` for this"}

	case len(evidenced) > 0 && len(trulyMissing) > 0:
		// Mixed: some missing hooks have contradiction evidence, others
		// have none. `fathom init` is now safe to run for the genuinely
		// missing ones — it reads user scope first and skips anything
		// already wired there — but the evidenced ones need a human to
		// find the scope that wires them (possibly one even init cannot
		// read, e.g. managed settings).
		reason := fmt.Sprintf("no readable settings scope wires hook(s) %s (%s). %s show no firing evidence anywhere doctor can see — genuinely missing. %s — evidently wired at a scope doctor cannot read.",
			strings.Join(missingLabels, ", "), checked, strings.Join(trulyMissing, ", "), strings.Join(evidenced, "; "))
		if len(scopeNotes) > 0 {
			reason += " " + strings.Join(scopeNotes, "; ")
		}
		remedy := fmt.Sprintf("fathom init --path %s installs the genuinely-missing hook(s) (%s) safely — it now skips anything already wired at user scope; if the evidenced hook(s) still aren't wired afterward, find the scope that wires them by hand (do not assume `fathom init` covers those)", root, strings.Join(trulyMissing, ", "))
		return doctorCheck{Name: name, Status: doctorFail, Reason: reason, Remedy: remedy}

	default:
		// Nothing missing has any firing evidence anywhere — genuinely
		// missing, install is the correct and safe remedy.
		reason := fmt.Sprintf("no readable settings scope wires hook(s): %s (%s)", strings.Join(missingLabels, ", "), checked)
		if len(scopeNotes) > 0 {
			reason += "; " + strings.Join(scopeNotes, "; ")
		}
		return doctorCheck{Name: name, Status: doctorFail, Reason: reason, Remedy: "fathom init --path " + root}
	}
}

// hookSpecKey identifies one hookSpec's (event, matcher) group — the same
// granularity ensureHookEntry (init.go) and hookMarkerPresent scope their
// own marker checks to. fathomHookSpecs has TWO entries sharing the same
// sub/marker ("edit-hook --pre", one per PreToolUse matcher group), so
// grouping by spec.sub alone would conflate them; grouping by (event,
// matcher) keeps every matcher group independently trackable.
type hookSpecKey struct{ event, matcher string }

func hookSpecKeyOf(spec hookSpec) hookSpecKey {
	return hookSpecKey{event: spec.event, matcher: spec.matcher}
}

// specLabel names one hookSpec including its matcher group, so two specs
// that share a sub (the two PreToolUse "edit-hook --pre" groups) render as
// two distinguishable entries instead of one duplicated string.
func specLabel(spec hookSpec) string {
	if spec.matcher == "" {
		return fmt.Sprintf("%s [%s]", spec.sub, spec.event)
	}
	return fmt.Sprintf("%s [%s, matcher %s]", spec.sub, spec.event, spec.matcher)
}

// hookVariantSeenEvidence reports how many times spec's SPECIFIC hook
// variant has fired, and which log the evidence came from — see
// checkHooksInstalled's doc comment for why the evidence source differs
// per hook (edit-hook's pre/post/matcher variants need the beat log's
// phase+target dimension; session-hook/stop-hook are unambiguous in the
// call log already). n is 0 with source "" for a spec this function has no
// distinguishing signal for (never a guessed pass).
func hookVariantSeenEvidence(root string, spec hookSpec) (n int, source string) {
	switch spec.sub {
	case "edit-hook":
		return editHookBeatCount("post", ""), "hooks beat log (edit-hook, phase=post)"
	case "edit-hook --pre":
		target := "file"
		if spec.matcher == "Bash" {
			target = "command"
		}
		return editHookBeatCount("pre", target), fmt.Sprintf("hooks beat log (edit-hook, phase=pre, target=%s)", target)
	case "session-hook", "stop-hook":
		return originCallCount(root, spec.sub), fmt.Sprintf("call log (origin=%s)", spec.sub)
	default:
		return 0, ""
	}
}

// editHookBeatCount counts "seen" beats on the "edit-hook" channel
// (beats.go's per-user hooks.jsonl) matching phase ("pre" or "post" — a
// beat with no phase meta predates phase-O and is treated as "post", the
// only variant that existed then) and, when target is non-empty, matching
// target too ("file" for the Edit|Write|MultiEdit PreToolUse group,
// "command" for the Bash one — edithook_pre.go). This is the ONE signal
// that can tell edit-hook's PostToolUse invocation apart from its two
// PreToolUse matcher groups; the call log cannot (all three share rpcOrigin
// "edit-hook", set unconditionally in cmdEditHook before the --pre branch).
func editHookBeatCount(phase, target string) int {
	path := beatLogPath()
	if path == "" {
		return 0
	}
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	beats, err := heartbeat.Load(f, true) // acceptPartial: a torn tail write must not blank the whole history
	if err != nil {
		return 0
	}
	n := 0
	for _, b := range beats {
		if b.Channel != "edit-hook" || b.Kind != "seen" {
			continue
		}
		p := b.Meta["phase"]
		if p == "" {
			p = "post"
		}
		if p != phase {
			continue
		}
		if target != "" && b.Meta["target"] != target {
			continue
		}
		n++
	}
	return n
}

// originCallCount reports how many call-log rows carry the given origin —
// checkHooksFiring's own pattern (readCallLog/resolveCallLogPath,
// instrument_adoption.go), generalized from "edit-hook" to any origin so
// session-hook/stop-hook (each unambiguous in the call log, unlike
// edit-hook's pre/post variants) can reuse it directly. This is only ever
// used for a SECONDARY signal (checkHooksInstalled's contradiction-evidence
// probe, never the primary pass/fail verdict), so — unlike
// resolveCallLogCandidates above — it deliberately does not query the
// daemon for its own "fathom" service path; it only trusts a relative
// call-log path joined against root when root actually IS the fathom
// source tree (O23: the same repo-relative assumption checkHooksFiring
// had, scoped down here rather than plumbing hc/baseURL through
// checkHooksInstalled for a check this minor). Outside the fathom tree
// this honestly returns 0 (no evidence) rather than checking the wrong
// repo and reporting a false negative as if it were conclusive.
func originCallCount(root, origin string) int {
	logPath := resolveCallLogPath()
	if !filepath.IsAbs(logPath) {
		if !isFathomSourceTree(root) {
			return 0
		}
		logPath = filepath.Join(root, logPath)
	}
	entries, _, read := readCallLog(logPath)
	if !read {
		return 0
	}
	n := 0
	for _, e := range entries {
		if e.Origin == origin {
			n++
		}
	}
	return n
}

// hookMarkerPresent is the read-only twin of init.go's ensureHookEntry: the
// same marker-substring detection, scoped to spec's OWN (event, matcher)
// group exactly like ensureHookEntry's own write path — otherwise the two
// PreToolUse groups (Edit|Write|MultiEdit and Bash), which share the
// identical "fathom edit-hook --pre" marker, would be indistinguishable: a
// marker found in EITHER group would wrongly read as both being wired.
func hookMarkerPresent(hooksObj map[string]any, spec hookSpec) bool {
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
				return true
			}
		}
	}
	return false
}

// ─── 5. lefthook gates (harness-agnostic — git-native) ─────────────────────

// lefthookInvokes reports whether any line of a lefthook config invokes the
// given fathom subcommand — through ANY spelling of the binary: a literal
// "fathom", an absolute/relative path ending in it, or a shell variable
// named for it ($FATHOM, "${FATHOM}"). The audit's live false negative: a
// pre-push spelled FATHOM=/path/bin/fathom; "$FATHOM" verify … was reported
// as "no fathom verify step" by the literal-substring check, telling a
// correctly-gated repo to add what it had (O5-adjacent, plan IO-5).
func lefthookInvokes(content, subcmd string) bool {
	re := regexp.MustCompile(`(?i)fathom\}?"?'?\s+` + subcmd + `\b`)
	for _, line := range strings.Split(content, "\n") {
		if re.MatchString(line) {
			return true
		}
	}
	return false
}

func checkLefthookGates(root string) doctorCheck {
	const name = "lefthook_gates"
	remedy := "add a `fathom verify` (and optionally `fathom review`) step to lefthook.yml's pre-push — docs/adoption/claude-code.md §7.4"

	p := filepath.Join(root, "lefthook.yml")
	data, err := os.ReadFile(p)
	if err != nil {
		alt := filepath.Join(root, "lefthook.yaml")
		data, err = os.ReadFile(alt)
		if err != nil {
			return doctorCheck{Name: name, Status: doctorFail, Reason: fmt.Sprintf("no lefthook.yml/lefthook.yaml at %s", root), Remedy: remedy}
		}
		p = alt
	}
	content := string(data)
	hasVerify := lefthookInvokes(content, "verify")
	hasReview := lefthookInvokes(content, "review")
	switch {
	case hasVerify && hasReview:
		return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("%s has both a fathom verify and a fathom review step", p)}
	case hasVerify:
		return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("%s has a fathom verify step (no fathom review step present — optional, C3-1)", p)}
	default:
		return doctorCheck{Name: name, Status: doctorFail, Reason: fmt.Sprintf("%s has no \"fathom verify\" step in its pre-push commands", p), Remedy: remedy}
	}
}

// ─── 6. hooks firing (Claude Code only) ─────────────────────────────────────

// fathomServicePath asks the daemon for the "fathom" service's recorded
// corpus root — the fleet-dogfood convention (CLAUDE.md's "Dogfood: the
// fleet index") is that a fathom daemon indexes its OWN source tree under
// exactly that service name. Combined with the launchd WorkingDirectory
// convention (docs/adoption/claude-code.md §5: "anchors the relative
// --call-log default … without it launchd starts the daemon at / and call
// logging is disabled"), that recorded path IS the daemon's own working
// directory in the overwhelmingly common deployment. Best-effort: ("",
// false) on any error, an absent/unnamed "fathom" service, or a relative
// (pre-O10) recorded path — never a guessed location.
func fathomServicePath(hc *http.Client, baseURL string) (string, bool) {
	svcs, _ := fetchServices(hc, baseURL)
	info, ok := svcs["fathom"]
	if !ok || info.path == "" || !filepath.IsAbs(info.path) {
		return "", false
	}
	return info.path, true
}

// resolveCallLogCandidates returns the plausible absolute filesystem
// locations for the daemon's call log, in priority order, deduplicated.
// O23 (docs/dogfood-observations.md): the call log belongs to the DAEMON,
// never the repo being doctored — resolveCallLogPath (instrument_adoption.go)
// mirrors the daemon's own [daemon].call_log / literal-default resolution
// (reused here verbatim, never re-derived), but its relative fallback is
// only safe to join against a root actually known to BE the daemon's
// working directory:
//
//  1. An explicit ABSOLUTE [daemon].call_log always wins outright — it
//     names its own location unambiguously, independent of any repo.
//  2. A relative path is joined against root ONLY when root is itself the
//     fathom checkout (isFathomSourceTree) — the one case where "the repo
//     being doctored" and "the daemon's working directory" are actually
//     the same directory, by the launchd WorkingDirectory convention.
//  3. A relative path is also joined against the daemon's own recorded
//     "fathom" service path (fathomServicePath) when reachable — the
//     candidate that actually matches production in every adopter repo
//     that is not fathom itself, since the daemon indexes its own source
//     tree under that name (CLAUDE.md's fleet-dogfood convention).
//
// Zero candidates means neither is determinable — the caller reports that
// honestly (skipped), never guesses a third location.
func resolveCallLogCandidates(hc *http.Client, baseURL string, daemonUp bool, root string) []string {
	logPath := resolveCallLogPath()
	if filepath.IsAbs(logPath) {
		return []string{logPath}
	}

	var out []string
	seen := map[string]bool{}
	add := func(base string) {
		if base == "" {
			return
		}
		p := filepath.Join(base, logPath)
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	if isFathomSourceTree(root) {
		add(root)
	}
	if daemonUp {
		if p, ok := fathomServicePath(hc, baseURL); ok {
			add(p)
		}
	}
	return out
}

// checkHooksFiring answers "has the hook stack ever actually run", not just
// "is it configured" — an EXISTENCE check over the call log (mirrors
// scripts/check-surface-traffic.sh's own existence-vs-liveness-window
// distinction), via resolveCallLogPath/readCallLog (instrument_adoption.go,
// same package — no import needed, no logic duplicated).
//
// O23 (docs/dogfood-observations.md): the call log lives with the DAEMON,
// not the doctored repo — live failure this fixes: "call log
// <adopter>/workspace/logs/mcp-calls.jsonl not found — no hook has ever
// written to it" reported FAIL in a repo where hooks were in fact firing,
// because the log was joined against the wrong root. checkHooksFiring now
// tries every candidate resolveCallLogCandidates offers (self-hosting root,
// then the daemon's own recorded "fathom" service path) before concluding
// anything is missing, and reports SKIPPED — never a guessed FAIL — when no
// candidate is determinable at all.
func checkHooksFiring(hc *http.Client, baseURL string, daemonUp bool, root, harness string) doctorCheck {
	const name = "hooks_firing"
	if harness != "claude" {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "Codex has no PostToolUse hook to fire; the lefthook_gates check covers its git-native equivalent"}
	}

	candidates := resolveCallLogCandidates(hc, baseURL, daemonUp, root)
	if len(candidates) == 0 {
		return doctorCheck{
			Name: name, Status: doctorSkipped,
			Reason: fmt.Sprintf("call log path %q is relative, this repo is not the fathom source tree, and the daemon does not index a service named \"fathom\" to locate its own workspace — cannot determine where the daemon's call log lives (the log belongs to the daemon, not this repo)", resolveCallLogPath()),
			Remedy: "set [daemon].call_log to an absolute path in ~/.fathom/config.toml, or check the daemon's own workspace by hand (docs/adoption/claude-code.md §5)",
		}
	}

	for _, logPath := range candidates {
		entries, _, read := readCallLog(logPath)
		if !read {
			continue
		}
		editHookCalls, verifyCalls := 0, 0
		for _, e := range entries {
			switch e.Origin {
			case "edit-hook":
				editHookCalls++
			case "verify-cli":
				verifyCalls++
			}
		}
		if editHookCalls == 0 {
			return doctorCheck{
				Name: name, Status: doctorFail,
				Reason: fmt.Sprintf("%s has %d entries but none with origin edit-hook — the PostToolUse hook has never fired", logPath, len(entries)),
				Remedy: "edit a file in this repo under Claude Code, then re-run doctor; if still silent, confirm obligations is compiled: fathom compile --service <svc> --dry-run=false",
			}
		}
		return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("%s: %d edit-hook call(s), %d verify-cli call(s)", logPath, editHookCalls, verifyCalls)}
	}

	return doctorCheck{
		Name: name, Status: doctorFail,
		Reason: fmt.Sprintf("checked %d candidate call-log location(s) (%s) — none exist; no hook has ever written to any of them", len(candidates), strings.Join(candidates, ", ")),
		Remedy: "edit a file in this repo under Claude Code (fires edit-hook), then re-run doctor",
	}
}

// ─── 7. CLAUDE.md / AGENTS.md stanza ────────────────────────────────────────

// checkStanzaPresent is harness-independent: `fathom init` always writes the
// identical stanza to both files regardless of --skills, so both are always
// expected. Read-only twin of init.go's applyStanza three-case detection
// (absent / well-formed / corrupted), never the write path.
func checkStanzaPresent(root string) doctorCheck {
	const name = "stanza_present"
	remedy := "fathom init --path " + root

	var problems []string
	for _, fname := range []string{"CLAUDE.md", "AGENTS.md"} {
		p := filepath.Join(root, fname)
		data, err := os.ReadFile(p)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", fname, err))
			continue
		}
		content := string(data)
		beginIdx := strings.Index(content, fathomStanzaBeginPrefix)
		endIdx := strings.Index(content, fathomStanzaEnd)
		switch {
		case beginIdx >= 0 && endIdx > beginIdx:
			// well-formed
		case beginIdx < 0 && endIdx < 0:
			problems = append(problems, fmt.Sprintf("%s: never wired (no fathom markers)", fname))
		default:
			problems = append(problems, fmt.Sprintf("%s: corrupted marker pair (begin=%t end=%t)", fname, beginIdx >= 0, endIdx >= 0))
		}
	}
	if len(problems) > 0 {
		return doctorCheck{Name: name, Status: doctorFail, Reason: strings.Join(problems, "; "), Remedy: remedy}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: "fathom stanza well-formed in both CLAUDE.md and AGENTS.md"}
}

// ─── 8. skills installed ────────────────────────────────────────────────────

// checkSkillsInstalled byte-compares every shipped skill's on-disk file
// against the embedded source (skillsdist.FS) — the same content the
// running binary would render on a fresh `fathom init --skills`. Missing,
// stale (present but byte-different), and not-fathom-owned (present but
// missing the marker, so init itself refuses to touch it) are reported as
// distinct problems, never lumped into one undifferentiated "fail".
func checkSkillsInstalled(root, harness string) doctorCheck {
	const name = "skills_installed"

	var missing, stale, notOwned []string
	current := 0

	check := func(dest, embeddedPath string) {
		want, err := skillsdist.FS.ReadFile(embeddedPath)
		if err != nil {
			return // the embedded source is fixed at compile time; should not happen
		}
		got, err := os.ReadFile(dest)
		if err != nil {
			missing = append(missing, dest)
			return
		}
		if !strings.Contains(string(got), skillMarker) {
			notOwned = append(notOwned, dest)
			return
		}
		if string(got) != string(want) {
			stale = append(stale, dest)
			return
		}
		current++
	}

	switch harness {
	case "codex":
		remedy := "fathom init --skills=codex"
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return doctorCheck{Name: name, Status: doctorSkipped, Reason: "cannot resolve home directory to check ~/.codex/prompts/", Remedy: remedy}
		}
		for _, skillName := range skillsdist.Names {
			check(filepath.Join(home, ".codex", "prompts", skillName+".md"), skillName+"/codex-prompt.md")
		}
		return summarizeSkillsCheck(name, remedy, harness, missing, stale, notOwned, current)
	default:
		remedy := "fathom init --skills=claude --path " + root
		for _, skillName := range skillsdist.Names {
			check(filepath.Join(root, ".claude", "skills", skillName, "SKILL.md"), skillName+"/SKILL.md")
		}
		return summarizeSkillsCheck(name, remedy, "claude", missing, stale, notOwned, current)
	}
}

func summarizeSkillsCheck(name, remedy, harness string, missing, stale, notOwned []string, current int) doctorCheck {
	switch {
	case len(missing) == len(skillsdist.Names):
		return doctorCheck{Name: name, Status: doctorFail, Reason: fmt.Sprintf("none of the %d shipped skills are installed for %s", len(skillsdist.Names), harness), Remedy: remedy}
	case len(missing) > 0 || len(stale) > 0 || len(notOwned) > 0:
		var parts []string
		if len(missing) > 0 {
			parts = append(parts, "missing: "+strings.Join(missing, ", "))
		}
		if len(stale) > 0 {
			parts = append(parts, "stale (differs from the shipped source): "+strings.Join(stale, ", "))
		}
		if len(notOwned) > 0 {
			parts = append(parts, "not fathom-owned (no marker; init refuses to touch): "+strings.Join(notOwned, ", "))
		}
		return doctorCheck{Name: name, Status: doctorFail, Reason: strings.Join(parts, "; "), Remedy: remedy}
	default:
		return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("all %d shipped skills current for %s", current, harness)}
	}
}

// ─── 9. .fathom/config.toml CUE-valid ───────────────────────────────────────

// checkConfigValid invokes the same config.Load the daemon/CLI already use
// for every config read — CUE validation happens inside it, so this check
// never re-implements the schema.
func checkConfigValid(root string) doctorCheck {
	const name = "config_valid"
	userPath := config.DefaultUserPath()
	projPath := filepath.Join(root, ".fathom", "config.toml")

	if _, err := config.Load(userPath, projPath); err != nil {
		return doctorCheck{Name: name, Status: doctorFail, Reason: err.Error(), Remedy: "fix the reported key/file, or remove the offending config.toml and re-run `fathom init` to scaffold a fresh one"}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: "user + project .fathom/config.toml (where present) validate against the CUE schema"}
}

// ─── 10. obligations served ─────────────────────────────────────────────────

// doctorObligationsProbeFile is the CLAUDE.md fallback probe: `fathom init`
// writes it into every wired repo, so it is meaningful without
// repo-specific configuration. O23 (docs/dogfood-observations.md): a
// service whose compiled obligations are code-scoped (e.g. "**/*.go") can
// serve real rules while CLAUDE.md alone answers zero — live evidence: the
// same daemon served 6 rules on cmd/conjure/main.go while probing CLAUDE.md
// alone found none. checkObligationsServed now probes a real INDEXED
// source file (obligationsProbeCandidates) ahead of this fallback, so a
// code-scoped-only install reports the real PASS it deserves instead of a
// false FAIL.
const doctorObligationsProbeFile = "CLAUDE.md"

// obligationsProbeCandidates returns the ordered, deduplicated list of
// repo-relative files checkObligationsServed probes obligations with: a
// real indexed source file in the service's most common extension first
// (dominantLanguageProbeFile — "" when undeterminable, e.g. list_sources
// unavailable or the service is empty), then the CLAUDE.md/AGENTS.md
// fallbacks every `fathom init`-wired repo carries. Each candidate is tried
// in turn (checkObligationsServed) and the first that actually serves rules
// answers the check — never just the first one, which is what let a
// code-scoped-only install FAIL under CLAUDE.md alone (O23).
func obligationsProbeCandidates(hc *http.Client, baseURL, svc string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(f string) {
		if f != "" && !seen[f] {
			seen[f] = true
			out = append(out, f)
		}
	}
	add(dominantLanguageProbeFile(hc, baseURL, svc))
	add(doctorObligationsProbeFile)
	add("AGENTS.md")
	return out
}

// dominantLanguageProbeFile picks a real, indexed, repo-relative file to
// probe obligations with, biased toward the service's most common source
// extension — best-effort via list_sources (listSourcesForDoctor,
// doctor_phaseo.go; reused, not re-derived). Returns "" when list_sources
// has nothing to offer (RPC failure, empty service, no extensioned files),
// never a guessed path.
func dominantLanguageProbeFile(hc *http.Client, baseURL, svc string) string {
	sources, err := listSourcesForDoctor(hc, baseURL, svc)
	if err != nil || len(sources) == 0 {
		return ""
	}
	counts := map[string]int{}
	for _, s := range sources {
		if ext := strings.ToLower(filepath.Ext(s)); ext != "" {
			counts[ext]++
		}
	}
	if len(counts) == 0 {
		return ""
	}
	exts := make([]string, 0, len(counts))
	for e := range counts {
		exts = append(exts, e)
	}
	sort.Strings(exts) // deterministic tie-break
	bestExt, bestN := "", -1
	for _, e := range exts {
		if counts[e] > bestN {
			bestExt, bestN = e, counts[e]
		}
	}
	for _, s := range sources {
		if strings.ToLower(filepath.Ext(s)) == bestExt {
			return s
		}
	}
	return ""
}

// obligationsProbeResult is one probe file's obligations() answer.
type obligationsProbeResult struct {
	file  string
	rules []struct {
		Status string `json:"status"`
	}
	watermark  string
	indexStale bool
}

// fetchObligationsProbe calls the obligations MCP tool for one
// (service, file) pair via callMCPTool (doctor_phaseo.go; reused, not
// re-derived).
func fetchObligationsProbe(hc *http.Client, baseURL, svc, file string) (obligationsProbeResult, error) {
	inner, err := callMCPTool(hc, baseURL, "obligations", map[string]any{"service": svc, "file": file})
	if err != nil {
		return obligationsProbeResult{}, err
	}
	var payload struct {
		Rules []struct {
			Status string `json:"status"`
		} `json:"rules"`
		Watermark  string `json:"compiled_at_watermark"`
		IndexStale bool   `json:"index_stale"`
	}
	if err := json.Unmarshal(inner, &payload); err != nil {
		return obligationsProbeResult{}, err
	}
	return obligationsProbeResult{file: file, rules: payload.Rules, watermark: payload.Watermark, indexStale: payload.IndexStale}, nil
}

// obligationsLayerCompiled reports whether the "obligations" semantic layer
// has EVER been compiled for svc, via staleness_report's optional `layers`
// block (pkg/client.ClaimsSummary's per-layer claim-status counts) — a
// signal independent of any single probe FILE's scope match. This is what
// lets checkObligationsServed tell "never compiled" (this returns false)
// apart from "compiled, but none of the probed files fall in any rule's
// scope" (this returns true even though every probe served zero rules) —
// the third, honest outcome O23 asks for, distinct from both.
func obligationsLayerCompiled(hc *http.Client, baseURL, svc string) (bool, error) {
	inner, err := callMCPTool(hc, baseURL, "staleness_report", map[string]any{"service": svc})
	if err != nil {
		return false, err
	}
	var payload struct {
		Layers map[string]json.RawMessage `json:"layers"`
	}
	if err := json.Unmarshal(inner, &payload); err != nil {
		return false, err
	}
	raw, ok := payload.Layers[semantic.LayerObligations]
	if !ok {
		return false, nil
	}
	var statusCounts map[string]int
	if err := json.Unmarshal(raw, &statusCounts); err != nil {
		return false, nil // unexpected shape — no evidence either way, not an error
	}
	for _, n := range statusCounts {
		if n > 0 {
			return true, nil
		}
	}
	return false, nil
}

// checkObligationsServed resolves the service the same way the hooks do —
// EVERY candidate identity (edithook.go's serviceCandidates loop), not a
// blind first guess — intersected with the daemon's actual service list,
// so its remedy can never name a service that is not in the index (a
// `fathom compile --service <ghost>` would be a zero-section bogus run).
// A repo whose candidates match NO indexed service gets the distinct
// not-indexed verdict (that state also means the hooks cannot inject here
// — dogfood-observations.md O15 is exactly this, live). An explicit
// --service is honored but cross-checked against the hook candidates: a
// service the hooks would never resolve gets a pass-with-note, because
// obligations serving over MCP does not imply injection works.
//
// O23 (docs/dogfood-observations.md): probing CLAUDE.md alone reports FAIL
// for a genuinely healthy, code-scoped-only install (live: 6 rules served
// on cmd/conjure/main.go, zero on CLAUDE.md). This now probes SEVERAL
// candidates (obligationsProbeCandidates: a real indexed source file first,
// then CLAUDE.md/AGENTS.md) and, only when every probe comes back empty,
// distinguishes "never compiled" (FAIL, remedy = compile) from "compiled,
// but none of the probed files matched any rule's scope" (obligationsLayer
// Compiled — a distinct WARN, never conflated with the FAIL case).
func checkObligationsServed(hc *http.Client, baseURL, root, explicitSvc string, daemonUp bool) doctorCheck {
	const name = "obligations_served"

	if !daemonUp {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "daemon unreachable — cannot query obligations (see daemon_reachable)"}
	}

	// Hook-candidate identities, deduped, in hook preference order — via
	// candidatesWithDaemon, the EXACT resolution the hooks use post-O15:
	// the daemon's recorded corpus paths are authoritative, so a repo whose
	// dir basename names no service still resolves to the indexed service
	// that records this root.
	var candNames []string
	seen := map[string]bool{}
	for _, c := range candidatesWithDaemon(hc, baseURL, root) {
		n := strings.ToLower(c.name)
		if !seen[n] {
			seen[n] = true
			candNames = append(candNames, n)
		}
	}

	probeNames := candNames
	if explicitSvc != "" {
		probeNames = []string{explicitSvc}
	}

	svcs, _ := fetchServices(hc, baseURL) // best-effort; empty map on any error
	indexed := make([]string, 0, len(svcs))
	for s := range svcs {
		indexed = append(indexed, s)
	}
	sort.Strings(indexed)

	svc := ""
	for _, n := range probeNames {
		if _, ok := svcs[n]; ok {
			svc = n
			break
		}
	}
	if svc == "" {
		return doctorCheck{
			Name: name, Status: doctorFail,
			Reason: fmt.Sprintf("no candidate service ID (%s) is in the daemon's index (indexed: %s) — the hooks resolve the same candidates, so obligations injection cannot fire for this repo either", strings.Join(probeNames, ", "), strings.Join(indexed, ", ")),
			Remedy: fmt.Sprintf("ingest this repo under a matching name (fathom init --path %s), or re-run doctor with --service <id> if it is indexed under a different one", root),
		}
	}
	remedy := fmt.Sprintf("fathom compile --service %s --dry-run=false", svc)

	// Explicit-service honesty note: serving obligations for a service the
	// hooks would never resolve proves the layer compiles, NOT that the
	// edit-hook can inject it (O15's exact trap).
	hookMismatchNote := ""
	if explicitSvc != "" {
		isCandidate := false
		for _, n := range candNames {
			if n == explicitSvc {
				isCandidate = true
				break
			}
		}
		if !isCandidate {
			hookMismatchNote = fmt.Sprintf(" — note: --service %s is NOT among the hook candidate IDs (%s), so the hooks will not resolve this service and obligations injection will not fire in sessions here even though serving works", explicitSvc, strings.Join(candNames, ", "))
		}
	}

	probes := obligationsProbeCandidates(hc, baseURL, svc)

	var tried []string
	var lastRPCErr error
	for _, file := range probes {
		res, err := fetchObligationsProbe(hc, baseURL, svc, file)
		if err != nil {
			lastRPCErr = err
			continue
		}
		tried = append(tried, file)
		if len(res.rules) == 0 && res.watermark == "" {
			continue // this probe answered zero — try the next candidate before giving up
		}

		// Drift honesty (rules are served FLAGGED, never dropped — serve.go's
		// Obligation.Status contract): a payload full of stale/contradicted
		// rules must not read as an unqualified clean pass.
		drifted := 0
		for _, r := range res.rules {
			if r.Status != "" {
				drifted++
			}
		}
		reason := fmt.Sprintf("obligations served for service %s, file %s (%d rule(s), watermark %s)", svc, res.file, len(res.rules), res.watermark)
		if drifted > 0 {
			reason += fmt.Sprintf(" — note: %d of %d rule(s) carry a drift status (stale/contradicted); served flagged, not dropped — recompile to refresh", drifted, len(res.rules))
		}
		if res.indexStale {
			reason += " — note: index_stale is true, the compiled index disagrees with the file on disk"
		}
		return doctorCheck{Name: name, Status: doctorPass, Reason: reason + hookMismatchNote}
	}

	if len(tried) == 0 && lastRPCErr != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: lastRPCErr.Error()}
	}

	// Every probe served zero rules and no watermark. O23's third outcome:
	// distinguish "never compiled" from "compiled, but none of the probed
	// files fall in any rule's scope" via obligationsLayerCompiled — a
	// signal independent of any single probe file.
	compiled, compErr := obligationsLayerCompiled(hc, baseURL, svc)
	switch {
	case compErr != nil:
		return doctorCheck{
			Name: name, Status: doctorFail,
			Reason: fmt.Sprintf("obligations(%s, ...) served zero rules and no compile watermark for every probed file (%s) — either the obligations layer has never been compiled for %q, or none of these specific files is in any compiled rule's scope (could not confirm which via staleness_report: %v)", svc, strings.Join(tried, ", "), svc, compErr),
			Remedy: remedy,
		}
	case !compiled:
		return doctorCheck{
			Name: name, Status: doctorFail,
			Reason: fmt.Sprintf("the obligations layer has never been compiled for service %q (probed %s, all served zero rules and no watermark)", svc, strings.Join(tried, ", ")),
			Remedy: remedy,
		}
	default:
		return doctorCheck{
			Name: name, Status: doctorWarn,
			Reason: fmt.Sprintf("the obligations layer IS compiled for service %q, but none of the probed files (%s) fall within any compiled rule's scope — this may be correct (rules scoped narrower than these probes) rather than broken; verify with obligations(service:%q, file:<a file you actually edit>)", svc, strings.Join(tried, ", "), svc),
		}
	}
}

// ─── 11. surface_traffic flowing ────────────────────────────────────────────

// checkSurfaceTraffic is the CLI-native twin of
// scripts/check-surface-traffic.sh's existence-check semantics ("has this
// surface fired", not "did it fire recently"), scoped to the "agent"
// surface (the recommended registration target per §1). Scope caveat,
// from server_stats' own tool description (internal/mcp/meta.go): the
// surface_traffic counters are PROCESS-LIFETIME — "since this process
// started", reset on every daemon restart — so a zero here means "no
// calls since this daemon process started", never "no calls ever".
func checkSurfaceTraffic(hc *http.Client, baseURL string, daemonUp bool, root string) doctorCheck {
	const name = "surface_traffic"
	remedy := "deploy a binary built after A1-1 (surface_traffic instrumentation) — docs/adoption/claude-code.md §5's deploy step"

	if !daemonUp {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "daemon unreachable — cannot query server_stats (see daemon_reachable)"}
	}

	req, err := originHeader(baseURL, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"server_stats","arguments":{}}}`))
	if err != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: err.Error()}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("POST %s/rpc: %v", baseURL, err)}
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: err.Error()}
	}
	inner, err := unwrapRPCResult(raw)
	if err != nil {
		return doctorCheck{Name: name, Status: doctorFail, Reason: fmt.Sprintf("server_stats: %v", err), Remedy: remedy}
	}
	var payload struct {
		SurfaceTraffic map[string]struct {
			Calls  int    `json:"calls"`
			LastTS string `json:"last_ts"`
		} `json:"surface_traffic"`
	}
	if err := json.Unmarshal(inner, &payload); err != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("parsing server_stats response: %v", err)}
	}
	if payload.SurfaceTraffic == nil {
		return doctorCheck{Name: name, Status: doctorFail, Reason: "server_stats has no surface_traffic block — the daemon predates A1-1 call-log instrumentation, or no call logger is wired (--call-log)", Remedy: remedy}
	}
	entry, ok := payload.SurfaceTraffic["agent"]
	if ok && entry.Calls > 0 {
		return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("agent surface: %d call(s) since this daemon process started, last at %s", entry.Calls, entry.LastTS)}
	}

	// IT-4: the live counter is process-lifetime and reads zero after every
	// daemon restart (comment above) — that alone is not proof nothing ever
	// happened. Fall back to the persistent call log, resolved the same way
	// hooks_firing resolves it, before reporting a fail.
	if n, lastTS, found := recentAgentCallsInPersistentLog(hc, baseURL, daemonUp, root, 7*24*time.Hour); found {
		return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("live counter reset by restart; persistent log shows %d agent call(s), last %s", n, lastTS)}
	}
	return doctorCheck{Name: name, Status: doctorFail, Reason: "surface_traffic is present but the \"agent\" surface (/rpc/agent) shows zero calls since this daemon process started (the counters are process-lifetime — they reset on restart), and the persistent call log shows no agent-surface call in the last 7 days either", Remedy: "make one real tool call against /rpc/agent (a live session, or the fathom-doctor skill's tie-in step), then re-run doctor"}
}

// recentAgentCallsInPersistentLog scans the persistent call log — every
// candidate resolveCallLogCandidates offers, the same resolution
// checkHooksFiring uses — for rows whose surface names "agent" and whose
// timestamp falls within `within` of now. found is false when no candidate
// log carries such a row, including when no candidate is even resolvable.
func recentAgentCallsInPersistentLog(hc *http.Client, baseURL string, daemonUp bool, root string, within time.Duration) (count int, lastTS string, found bool) {
	cutoff := time.Now().Add(-within)
	for _, logPath := range resolveCallLogCandidates(hc, baseURL, daemonUp, root) {
		entries, _, read := readCallLog(logPath)
		if !read {
			continue
		}
		for _, e := range entries {
			if !strings.Contains(e.Surface, "agent") {
				continue
			}
			ts, err := time.Parse(time.RFC3339, e.TS)
			if err != nil || ts.Before(cutoff) {
				continue
			}
			count++
			if e.TS > lastTS {
				lastTS = e.TS
			}
		}
	}
	return count, lastTS, count > 0
}

// ─── output ─────────────────────────────────────────────────────────────────

func printDoctorTable(w io.Writer, harness, svc, root string, checks []doctorCheck) {
	fmt.Fprintf(w, "fathom doctor — service %q, harness %q, root %s\n\n", svc, harness, root)
	fmt.Fprintf(w, "%-20s %-7s %s\n", "CHECK", "STATUS", "REASON")
	for _, c := range checks {
		fmt.Fprintf(w, "%-20s %-7s %s\n", c.Name, statusLabel(c.Status), c.Reason)
		if c.Remedy != "" {
			fmt.Fprintf(w, "%-20s %-7s remedy: %s\n", "", "", c.Remedy)
		}
	}

	pass, warn, fail, skipped := tallyDoctorChecks(checks)
	if warn > 0 {
		fmt.Fprintf(w, "\n%d checks: %d pass, %d warn, %d fail, %d skipped\n", len(checks), pass, warn, fail, skipped)
	} else {
		fmt.Fprintf(w, "\n%d checks: %d pass, %d fail, %d skipped\n", len(checks), pass, fail, skipped)
	}
}

func statusLabel(s doctorStatus) string {
	switch s {
	case doctorPass:
		return "PASS"
	case doctorFail:
		return "FAIL"
	case doctorWarn:
		return "WARN"
	default:
		return "SKIP"
	}
}

func tallyDoctorChecks(checks []doctorCheck) (pass, warn, fail, skipped int) {
	for _, c := range checks {
		switch c.Status {
		case doctorPass:
			pass++
		case doctorWarn:
			warn++
		case doctorFail:
			fail++
		case doctorSkipped:
			skipped++
		}
	}
	return pass, warn, fail, skipped
}

func printDoctorJSON(w io.Writer, harness, svc, root string, checks []doctorCheck) {
	pass, warn, fail, skipped := tallyDoctorChecks(checks)
	out := map[string]any{
		"service": svc,
		"harness": harness,
		"root":    root,
		"checks":  checks,
		// warn is additive to the pre-existing summary shape (never
		// renamed/removed) — an older consumer reading only pass/fail/
		// skipped still sees the same numbers those fields always meant.
		"summary": map[string]any{"total": len(checks), "pass": pass, "warn": warn, "fail": fail, "skipped": skipped},
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}
