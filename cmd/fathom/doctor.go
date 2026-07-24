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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	skillsdist "github.com/geoffmilleraz/fathom/dist/agents/skills"
	"github.com/geoffmilleraz/fathom/internal/config"
	toml "github.com/pelletier/go-toml/v2"
)

// doctorStatus is one check's verdict.
type doctorStatus string

const (
	doctorPass    doctorStatus = "pass"
	doctorFail    doctorStatus = "fail"
	doctorSkipped doctorStatus = "skipped"
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
		checkHooksFiring(root, harness),
		checkStanzaPresent(root),
		checkSkillsInstalled(root, harness),
		checkConfigValid(root),
		checkObligationsServed(hc, baseURL, root, explicitSvc, daemonUp),
		checkSurfaceTraffic(hc, baseURL, daemonUp),
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
func checkBinaryFresh(root string) doctorCheck {
	const name = "binary_fresh"
	remedy := "cp bin/fathom ~/.local/libexec/fathom/fathom && launchctl kickstart -k gui/$(id -u)/com.geoffmilleraz.fathom-http (deploy the rebuilt binary — docs/adoption/claude-code.md §5)"

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

// checkHooksInstalled looks for the three fathom hooks across EVERY
// settings scope Claude Code merges — project .claude/settings.json,
// project .claude/settings.local.json, and user ~/.claude/settings.json —
// because hooks wired at user scope (a perfectly good wiring) must never
// read as a project-scope FAIL whose remedy would install duplicates
// (every PostToolUse would then fire twice). When no readable scope wires
// them but the call log proves they fire anyway, the verdict names that
// contradiction instead of prescribing the duplicate-installing remedy.
func checkHooksInstalled(root, harness string) doctorCheck {
	const name = "hooks_installed"
	if harness != "claude" {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "Codex has no SessionStart/PostToolUse/Stop hook mechanism (docs/adoption/codex.md §6) — nothing to check"}
	}
	remedy := "fathom init --path " + root

	type scopeFile struct{ label, path string }
	scopes := []scopeFile{
		{"project", filepath.Join(root, ".claude", "settings.json")},
		{"project-local", filepath.Join(root, ".claude", "settings.local.json")},
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		scopes = append(scopes, scopeFile{"user", filepath.Join(home, ".claude", "settings.json")})
	}

	wiredScopes := map[string][]string{} // spec.sub -> scope labels that wire it
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
				wiredScopes[spec.sub] = append(wiredScopes[spec.sub], sc.label)
			}
		}
	}

	var missing []string
	labelSet := map[string]bool{}
	var labels []string
	for _, spec := range fathomHookSpecs {
		found := wiredScopes[spec.sub]
		if len(found) == 0 {
			missing = append(missing, spec.sub)
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

	// Contradiction probe: the settings scopes doctor can read wire nothing
	// (or not everything), yet the call log proves edit-hook traffic — the
	// hooks ARE firing from a scope this check cannot see (managed
	// settings, an unreadable file). In that state the normal remedy is
	// actively harmful: `fathom init` would add a project-scope SECOND
	// wiring and every hook would fire twice.
	if n := editHookCallCount(root); n > 0 {
		reason := fmt.Sprintf("no readable settings scope wires hook(s) %s (checked project .claude/settings.json, .claude/settings.local.json, user ~/.claude/settings.json), yet the call log records %d edit-hook call(s) — the hooks are evidently wired at a scope doctor cannot read. Running `fathom init` now would install project-scope DUPLICATES: every hook would fire twice.", strings.Join(missing, ", "), n)
		if len(scopeNotes) > 0 {
			reason += " " + strings.Join(scopeNotes, "; ")
		}
		return doctorCheck{Name: name, Status: doctorFail, Reason: reason, Remedy: "find the scope that wires them (e.g. read ~/.claude/settings.json, or the harness's hook settings UI) before changing anything — do not run `fathom init` for this"}
	}

	reason := fmt.Sprintf("no readable settings scope wires hook(s): %s (checked project .claude/settings.json, .claude/settings.local.json, user ~/.claude/settings.json)", strings.Join(missing, ", "))
	if len(scopeNotes) > 0 {
		reason += "; " + strings.Join(scopeNotes, "; ")
	}
	return doctorCheck{Name: name, Status: doctorFail, Reason: reason, Remedy: remedy}
}

// editHookCallCount reports how many call-log rows carry origin
// "edit-hook" — the firing evidence checkHooksInstalled's contradiction
// probe and checkHooksFiring both read (same resolveCallLogPath +
// readCallLog pair, instrument_adoption.go). Zero on an absent/unreadable
// log.
func editHookCallCount(root string) int {
	logPath := resolveCallLogPath()
	if !filepath.IsAbs(logPath) {
		logPath = filepath.Join(root, logPath)
	}
	entries, _, read := readCallLog(logPath)
	if !read {
		return 0
	}
	n := 0
	for _, e := range entries {
		if e.Origin == "edit-hook" {
			n++
		}
	}
	return n
}

// hookMarkerPresent is the read-only twin of init.go's ensureHookEntry: the
// same marker-substring detection, without the insert-if-absent half.
func hookMarkerPresent(hooksObj map[string]any, spec hookSpec) bool {
	arr, _ := hooksObj[spec.event].([]any)
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
			if strings.Contains(cmdStr, spec.marker) {
				return true
			}
		}
	}
	return false
}

// ─── 5. lefthook gates (harness-agnostic — git-native) ─────────────────────

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
	hasVerify := strings.Contains(content, "fathom verify")
	hasReview := strings.Contains(content, "fathom review")
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

// checkHooksFiring answers "has the hook stack ever actually run", not just
// "is it configured" — an EXISTENCE check over the call log (mirrors
// scripts/check-surface-traffic.sh's own existence-vs-liveness-window
// distinction), via resolveCallLogPath/readCallLog (instrument_adoption.go,
// same package — no import needed, no logic duplicated).
func checkHooksFiring(root, harness string) doctorCheck {
	const name = "hooks_firing"
	if harness != "claude" {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "Codex has no PostToolUse hook to fire; the lefthook_gates check covers its git-native equivalent"}
	}

	logPath := resolveCallLogPath()
	if !filepath.IsAbs(logPath) {
		logPath = filepath.Join(root, logPath)
	}
	entries, _, read := readCallLog(logPath)
	if !read {
		return doctorCheck{Name: name, Status: doctorFail, Reason: fmt.Sprintf("call log %s not found — no hook has ever written to it", logPath), Remedy: "edit a file in this repo under Claude Code (fires edit-hook), then re-run doctor"}
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
		beginIdx := strings.Index(content, fathomStanzaBegin)
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

// doctorObligationsProbeFile is the "governed file" doctor queries
// obligations for. CLAUDE.md is chosen because `fathom init` writes it into
// every wired repo, so the check is meaningful without repo-specific
// configuration — see the reason text below for the honest limit this
// implies (a zero result can mean "never compiled" OR "this file isn't in
// any compiled rule's scope"; doctor cannot tell those apart from one file).
const doctorObligationsProbeFile = "CLAUDE.md"

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
func checkObligationsServed(hc *http.Client, baseURL, root, explicitSvc string, daemonUp bool) doctorCheck {
	const name = "obligations_served"

	if !daemonUp {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "daemon unreachable — cannot query obligations (see daemon_reachable)"}
	}

	// Hook-candidate identities, deduped, in hook preference order.
	var candNames []string
	seen := map[string]bool{}
	for _, c := range serviceCandidates(root) {
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

	svcs := fetchServices(hc, baseURL) // best-effort; empty map on any error
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

	reqBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": svc, "file": doctorObligationsProbeFile},
		},
	})
	if err != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: err.Error()}
	}
	req, err := originHeader(baseURL, bytes.NewReader(reqBody))
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
		return doctorCheck{Name: name, Status: doctorFail, Reason: fmt.Sprintf("obligations(%s, %s): %v", svc, doctorObligationsProbeFile, err), Remedy: remedy}
	}
	var payload struct {
		Rules []struct {
			Status string `json:"status"`
		} `json:"rules"`
		Watermark  string `json:"compiled_at_watermark"`
		IndexStale bool   `json:"index_stale"`
	}
	if err := json.Unmarshal(inner, &payload); err != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("parsing obligations response: %v", err)}
	}
	if len(payload.Rules) == 0 && payload.Watermark == "" {
		return doctorCheck{
			Name: name, Status: doctorFail,
			Reason: fmt.Sprintf("obligations(%s, %s) served zero rules and no compile watermark — either the obligations layer has never been compiled for %q, or %s specifically is not in any compiled rule's scope", svc, doctorObligationsProbeFile, svc, doctorObligationsProbeFile),
			Remedy: remedy,
		}
	}

	// Drift honesty (rules are served FLAGGED, never dropped — serve.go's
	// Obligation.Status contract): a payload full of stale/contradicted
	// rules must not read as an unqualified clean pass.
	drifted := 0
	for _, r := range payload.Rules {
		if r.Status != "" {
			drifted++
		}
	}
	reason := fmt.Sprintf("obligations served for service %s, file %s (%d rule(s), watermark %s)", svc, doctorObligationsProbeFile, len(payload.Rules), payload.Watermark)
	if drifted > 0 {
		reason += fmt.Sprintf(" — note: %d of %d rule(s) carry a drift status (stale/contradicted); served flagged, not dropped — recompile to refresh", drifted, len(payload.Rules))
	}
	if payload.IndexStale {
		reason += " — note: index_stale is true, the compiled index disagrees with the file on disk"
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: reason + hookMismatchNote}
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
func checkSurfaceTraffic(hc *http.Client, baseURL string, daemonUp bool) doctorCheck {
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
	if !ok || entry.Calls == 0 {
		return doctorCheck{Name: name, Status: doctorFail, Reason: "surface_traffic is present but the \"agent\" surface (/rpc/agent) shows zero calls since this daemon process started (the counters are process-lifetime — they reset on restart)", Remedy: "make one real tool call against /rpc/agent (a live session, or the fathom-doctor skill's tie-in step), then re-run doctor"}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("agent surface: %d call(s) since this daemon process started, last at %s", entry.Calls, entry.LastTS)}
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

	pass, fail, skipped := tallyDoctorChecks(checks)
	fmt.Fprintf(w, "\n%d checks: %d pass, %d fail, %d skipped\n", len(checks), pass, fail, skipped)
}

func statusLabel(s doctorStatus) string {
	switch s {
	case doctorPass:
		return "PASS"
	case doctorFail:
		return "FAIL"
	default:
		return "SKIP"
	}
}

func tallyDoctorChecks(checks []doctorCheck) (pass, fail, skipped int) {
	for _, c := range checks {
		switch c.Status {
		case doctorPass:
			pass++
		case doctorFail:
			fail++
		case doctorSkipped:
			skipped++
		}
	}
	return pass, fail, skipped
}

func printDoctorJSON(w io.Writer, harness, svc, root string, checks []doctorCheck) {
	pass, fail, skipped := tallyDoctorChecks(checks)
	out := map[string]any{
		"service": svc,
		"harness": harness,
		"root":    root,
		"checks":  checks,
		"summary": map[string]any{"total": len(checks), "pass": pass, "fail": fail, "skipped": skipped},
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}
