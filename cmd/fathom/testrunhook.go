package main

// testrunhook.go — the test-run hook (ADR-056 decision 6; design
// docs/design/test-run-hook.md; plan docs/plans/test-run-hook.md TR-2,
// TR-6 to TR-8, TR-10): fathom's level 10, tiers 0 and 1, provider none.
//
// `fathom session-hook --event test-run` rides PostToolUse(Bash) and
// `--event failed-test-run` rides PostToolUseFailure(Bash): TR-V1 read that
// a failing test command arrives on the first when piped and on the second
// when not. On a Bash command that runs one of the repository's declared
// test commands, with `[hooks] test_run` on, the hook parses the output
// (internal/testrun tier 0), and on a failure injects ONE line naming the
// failing tests and the changed symbols they reach (tier 1), with the
// full output saved under workspace/tmp. Nothing on a pass or an unknown
// result. Any other Bash call exits before a daemon call.
//
// Hard limits (design section 3; TR-X2): inject only — no updatedToolOutput,
// no block, no permission decision — and no command execution: the harness
// ran the test under its own permissions; fathom only reads what came back
// (plus `git diff HEAD`, a read that never writes the repository).

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/gitro"
	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
	"github.com/geoffmilleraz/fathom/internal/testrun"
)

// testRunChannel is the hook log channel every test-run row is written to
// (ADR-049 decision 2: rows in the existing hook log, no new table).
const testRunChannel = "test-run-hook"

// testRunStdinCap bounds the envelope read: Claude Code caps Bash output
// at 30,000 characters by default (TR-V1), so 1 MiB leaves room for a
// raised BASH_MAX_OUTPUT_LENGTH without reading an unbounded stream.
const testRunStdinCap = 1 << 20

// testRunLineBudget caps the injected line (I35's per-payload discipline:
// one line, well inside the 320-token ceiling). The saved-output path is
// never cut.
const testRunLineBudget = 700

// testRunLineCut discloses a cut (law 3: never a silent truncation).
const testRunLineCut = " …[cut at the test-run line's byte cap]"

// testRunSavedDir is where the full output of a failing run is saved,
// relative to the repository root (I31: artifacts live under workspace/).
const testRunSavedDir = "workspace/tmp/test-run"

// testRunEntryHook is `session-hook --event test-run` and `--event
// failed-test-run`, the test-run hook's own entries, which stay accepted
// (RO-21): while [readouts] mode is on the readout hook runs the tier in its
// process instead, so this entry exits 0 silent and a settings file wired
// before the fold, carrying both entries, still runs the tier once.
func testRunEntryHook(codec harness.Codec, ev harness.Event, event, baseURL, dirOverride string, stdin io.Reader, stdout io.Writer) int {
	if readoutsWiringOn() {
		return 0
	}
	return cmdTestRunHook(codec, ev, event, baseURL, dirOverride, stdin, stdout)
}

// cmdTestRunHook is the hook body for one harness event: decode, gate,
// parse, explain, inject, log. It always returns 0 (I9). The readout hook
// calls it in process (readoutTestRunTier); testRunEntryHook is its own
// entry.
func cmdTestRunHook(codec harness.Codec, ev harness.Event, event, baseURL, dirOverride string, stdin io.Reader, stdout io.Writer) int {
	if stdin == nil {
		return 0
	}
	raw, err := io.ReadAll(io.LimitReader(stdin, testRunStdinCap))
	if err != nil {
		return 0
	}
	in, err := codec.DecodeInput(ev, raw)
	if err != nil || in.Tool != harness.ToolShell || strings.TrimSpace(in.Command) == "" {
		return 0
	}
	root := dirOverride
	if root == "" {
		// The run's keys are the work tree the command ran in (a leading
		// cd, O1230), which the claim check and the gate preflight read.
		root = hookCallRoot(in.Command, "", in.Cwd, eventProjectDir(codec, in))
	}
	if root == "" {
		return 0
	}
	syntaxDir := in.Cwd
	if syntaxDir == "" {
		syntaxDir = root
	}
	// The gate, cheapest first, all local: a layer that never names
	// test_run cannot switch the hook on; a command that runs no declared
	// test is not the hook's business; only then the validated config.
	// The raw pre-gate honors the scope registry (SEC-2): the repository's
	// file can open it only for a field a repository may set, so a
	// user-only field config.Load would drop never opens the gate.
	userRaw, projRaw := readConfigLayer(config.DefaultUserPath()), readConfigLayer(projectdir.Config.Path(root))
	names := func(key, word string) bool {
		return bytes.Contains(userRaw, []byte(word)) || (config.RepoMaySet(key) && bytes.Contains(projRaw, []byte(word)))
	}
	if !names("hooks.test_run", "test_run") && !capabilityCueNames(root, "test-run-hook") {
		return 0
	}
	m, matched := matchDeclaredTestCommand(root, in.Command, nil, syntaxDir)
	var loaded *config.Loaded
	if !matched && names("tests.commands", "commands") {
		if l, err := config.Load(config.DefaultUserPath(), projectdir.Config.Path(root)); err == nil {
			loaded = &l
			m, matched = matchDeclaredTestCommand(root, in.Command, l.Config.Tests.Commands, syntaxDir)
		}
	}
	if !matched {
		return 0
	}
	if loaded == nil {
		l, err := config.Load(config.DefaultUserPath(), projectdir.Config.Path(root))
		if err != nil {
			return 0
		}
		loaded = &l
	}
	if !capabilityOnAt(root, "test-run-hook", loaded.Config.Hooks.TestRun) {
		return 0
	}
	rpcOrigin = testRunChannel
	rpcSession = in.SessionID // O1151: the daemon sees this hook session

	res := testrun.Analyze(in.ToolOutput, testrun.Evidence{ExitCode: in.ExitCode, Filtered: m.Piped, Truncated: in.OutputTruncated})
	relativizeFrames(&res.Run, root)
	rec := testRunRecord{Event: event, Invocation: m.Invocation, Piped: m.Piped, Result: res, ExitCode: in.ExitCode, Session: in.SessionID, AgentID: in.AgentID, ToolUseID: in.ToolUseID}
	defer func() { logTestRunRow(rec) }()
	// RO-35: the keys a delta needs, on every matched run, inside the hook's
	// deadline. One context serves them and the failing path's line.
	ctx, cancel := testRunHookContext()
	defer cancel()
	diff, diffOK := fillTestRunRowKeys(ctx, &rec, root, in.SessionID, in.Command, m.Invocation, syntaxDir)
	if res.Verdict != testrun.Fail {
		return 0
	}
	full := in.ToolOutput
	if ev == harness.EventToolFailure && in.ToolError != "" {
		full = in.ToolError // keep the "Exit code N" line in the saved copy
	}
	saved := saveTestRunOutput(root, full)

	line, missed := runTestRunWithDeadline(ctx, func(ctx context.Context) string {
		if !diffOK {
			diff = workingTreeDiff(ctx, root)
		}
		changed := testrun.ChangedGoSymbols(root, diff)
		var idx testrun.Index
		if svc, ok := testRunService(ctx, baseURL, root); ok {
			idx = daemonTestRunIndex{ctx: ctx, baseURL: baseURL, service: svc}
		}
		cands := testrun.FindCandidates(res.Run, changed, idx)
		for _, r := range cands.Reach {
			if len(r.Changed) > 0 {
				rec.Reached++
			}
		}
		var extra []string
		if loaded.Config.Readouts.Mode == config.ReadoutsRich {
			// RO-23: rich mode's test delta, a local read of the hook log
			if d := testRunDeltaClause(&rec, res.Run, in.Command, !m.Piped && !in.OutputTruncated, syntaxDir); d != "" {
				extra = append(extra, d)
			}
		}
		return renderTestRunLine(res, cands, saved, extra...)
	})
	rec.DeadlineMissed = missed
	if line == "" {
		return 0
	}
	out, err := codec.EncodeOutput(ev, harness.HookOutput{Context: line})
	if err != nil || len(out) == 0 {
		return 0
	}
	fmt.Fprintln(stdout, string(out))
	rec.Emitted, rec.Bytes = true, len(line)
	return 0
}

// relativizeFrames rewrites absolute frame paths under root (a panic's
// stack) as repo-relative, so the line names files the way the agent
// reads them.
func relativizeFrames(r *testrun.Run, root string) {
	rel := func(fs []testrun.Frame) {
		for i := range fs {
			if filepath.IsAbs(fs[i].File) {
				if p, err := filepath.Rel(root, fs[i].File); err == nil && !strings.HasPrefix(p, "..") {
					fs[i].File = filepath.ToSlash(p)
				}
			}
		}
	}
	for i := range r.Failures {
		rel(r.Failures[i].Frames)
	}
	for i := range r.Panics {
		rel(r.Panics[i].Frames)
	}
}

// readConfigLayer returns a config layer's bytes (capped), or nil.
func readConfigLayer(p string) []byte {
	if p == "" {
		return nil
	}
	f, err := os.Open(p)
	if err != nil {
		return nil
	}
	defer f.Close()
	b, _ := io.ReadAll(io.LimitReader(f, 1<<20))
	return b
}

// testRunContext is the hook's deadline as a context: the running hook
// guard's (the descriptor's TimeoutSec minus max(1 s, 20%), I53), or
// testRunNoGuardBudget outside a hook. Tier 1's calls are bound by it alone
// (EW-76): no call carries a shorter fixed timeout of its own.
func testRunContext() (context.Context, context.CancelFunc) {
	if g := activeHookGuard(); g != nil && !g.Deadline.IsZero() {
		return g.Deadline.Context(context.Background())
	}
	return context.WithTimeout(context.Background(), testRunNoGuardBudget)
}

// testRunHookContext is the seam through which the test-run hook takes its
// deadline context: testRunContext in the binary, a context the test
// cancels itself so no wall-clock budget decides the outcome.
var testRunHookContext = testRunContext

// testRunNoGuardBudget bounds tier 1 when no hook guard runs (a direct
// call, never the wired hook).
const testRunNoGuardBudget = 8 * time.Second

// testRunHTTPClient is the client tier 1's daemon calls use (EW-76, G11):
// no Timeout of its own, so a call runs to the hook's deadline (its ctx)
// instead of a fixed 2 s cap under it, which cut every test-run call it
// abandoned at 99aa205a with half of the hook's budget unspent. A variable
// so a test can observe the client a call is given.
var testRunHTTPClient = func() *http.Client { return &http.Client{} }

// runTestRunWithDeadline runs tier 1's work inside ctx's deadline (TR-10,
// I53). The work's RPCs and git read are bound to ctx; a deadline that has
// passed before the work starts, or while it runs, is silence — a line
// that arrives late is dropped, never injected — and missed reports it.
func runTestRunWithDeadline(ctx context.Context, work func(context.Context) string) (string, bool) {
	if deadlinePassed(ctx) {
		return "", true
	}
	line := work(ctx)
	if deadlinePassed(ctx) {
		return "", true
	}
	return line, false
}

// deadlinePassed reports a ctx that is done or whose deadline instant has
// been reached. Err() alone races a second timer set to the same instant
// (the hook guard's transport): the call it cancels can return, and the
// work finish, before ctx's own timer fires.
func deadlinePassed(ctx context.Context) bool {
	if ctx.Err() != nil {
		return true
	}
	dl, ok := ctx.Deadline()
	return ok && !time.Now().Before(dl)
}

// workingTreeDiff is `git diff HEAD --unified=0` over Go files, read
// without writing the repository (gitro), or "" on any error.
func workingTreeDiff(ctx context.Context, root string) string {
	d, _ := workingTreeDiffErr(ctx, root)
	return d
}

// workingTreeDiffErr is workingTreeDiff that tells an empty diff from a
// failed read (no HEAD, a dead ctx), which RO-35's tree key must not hash.
func workingTreeDiffErr(ctx context.Context, root string) (string, bool) {
	out, err := gitro.Output(ctx, root, "diff", "HEAD", "--unified=0", "--no-color", "--no-ext-diff", "--", "*.go")
	if err != nil {
		return "", false
	}
	return string(out), true
}

// saveTestRunOutput writes the full output under root/workspace/tmp/test-run,
// named by its content hash (the same output always lands at the same
// path), and returns the repo-relative path, or "" when it cannot.
func saveTestRunOutput(root, output string) string {
	sum := sha256.Sum256([]byte(output))
	rel := path.Join(testRunSavedDir, hex.EncodeToString(sum[:8])+".log")
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return ""
	}
	if err := os.WriteFile(abs, []byte(output), 0o644); err != nil {
		return ""
	}
	pruneTestRunSaved(filepath.Dir(abs), filepath.Base(abs))
	return rel
}

// The bound on workspace/tmp/test-run (RO-36): the oldest saved outputs go
// first once either is exceeded.
const (
	testRunSavedMaxFiles = 50
	testRunSavedMaxBytes = 8 << 20
)

// testRunSavedName is the hook's own file name: 16 lowercase hex and .log.
var testRunSavedName = regexp.MustCompile(`^[0-9a-f]{16}\.log$`)

// pruneTestRunSaved keeps dir at testRunSavedMaxFiles files and
// testRunSavedMaxBytes bytes, removing the oldest (by modification time,
// then name) first. It touches only regular files named like the hook's
// own (I63: never a symlink, a directory or a stranger's file) and never
// the file named keep, the one just written, which the injected line cites.
// Every error is swallowed (I9): a prune that cannot finish leaves the
// directory over its bound and the hook unharmed.
func pruneTestRunSaved(dir, keep string) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type saved struct {
		name string
		mod  time.Time
		size int64
	}
	var files []saved
	var total int64
	for _, e := range ents {
		if !testRunSavedName.MatchString(e.Name()) || !e.Type().IsRegular() {
			continue
		}
		fi, err := e.Info()
		if err != nil || !fi.Mode().IsRegular() {
			continue
		}
		files = append(files, saved{e.Name(), fi.ModTime(), fi.Size()})
		total += fi.Size()
	}
	sort.Slice(files, func(i, j int) bool {
		if !files[i].mod.Equal(files[j].mod) {
			return files[i].mod.Before(files[j].mod)
		}
		return files[i].name < files[j].name
	})
	count := len(files)
	for _, f := range files {
		if count <= testRunSavedMaxFiles && total <= testRunSavedMaxBytes {
			return
		}
		if f.name == keep {
			continue
		}
		if os.Remove(filepath.Join(dir, f.name)) == nil {
			count--
			total -= f.size
		}
	}
}

// testRunService resolves root to its indexed service (the cached
// identity lookup the other advisory events use), inside ctx.
func testRunService(ctx context.Context, baseURL, root string) (string, bool) {
	if ctx.Err() != nil {
		return "", false
	}
	name, _, ok := indexedServiceFor(&http.Client{Timeout: 2 * time.Second}, baseURL, root)
	return name, ok
}

// daemonTestRunIndex is the testrun.Index port over the daemon's /rpc:
// test_impact bound to the hook's deadline, and no localize (EW-76).
type daemonTestRunIndex struct {
	ctx     context.Context
	baseURL string
	service string
}

func (d daemonTestRunIndex) TestImpact(ref string) ([]string, error) {
	var p struct {
		Tests []string `json:"tests"`
	}
	// The call graph resolves a directory-qualified key only in full
	// ("svc::dir/pkg.Name"); the bare "dir/pkg.Name" names nothing.
	err := callTestRunTool(d.ctx, d.baseURL, "test_impact", map[string]any{"service": d.service, "symbol": d.service + "::" + ref}, &p)
	return p.Tests, err
}

// Localize asks the daemon nothing on the hook path (EW-76, G11; the
// reading is docs/bench/readings/efficiency-wave/hook-path-deadlines.md).
// The line names localize's first candidate only when no changed symbol
// reaches the failing tests, and for a failing_test seed that candidate is
// the failing test itself (find_symbol on its name outscores every graph
// hop), which the run's own output already names; for a stack trace it is
// the trace's own frame. The call also needs the service's call graph: on
// this repository's index a build took 10 to 14 s, which no hook budget
// holds, and a caller that leaves early cancels the build, so the hook's
// attempts never even warmed it. At 99aa205a the fleet's call log read 385
// of 437 test-run localize calls abandoned. The agent's own localize (no
// hook deadline) is unchanged.
func (d daemonTestRunIndex) Localize(testrun.LocalizeSeed) ([]testrun.Located, error) {
	return nil, nil
}

// callTestRunTool makes one tools/call to the daemon inside ctx and
// decodes the tool's payload into out.
func callTestRunTool(ctx context.Context, baseURL, tool string, args map[string]any, out any) error {
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": tool, "arguments": args},
	})
	if err != nil {
		return err
	}
	req, err := originHeader(baseURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	resp, err := testRunHTTPClient().Do(req.WithContext(ctx))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: HTTP %d", tool, resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	inner, err := unwrapRPCResult(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(inner, out)
}

// ─── the line (TR-6) ────────────────────────────────────────────────────

// renderTestRunLine is the one injected line: "" unless the run failed;
// otherwise the failing tests (first frame each), the changed symbols
// they reach, or localize's candidate when none, then extra (rich mode's
// test delta, RO-23), then the saved output's path — which survives any
// cut. Deterministic, one line, at most testRunLineBudget bytes.
func renderTestRunLine(res testrun.Result, c testrun.Candidates, saved string, extra ...string) string {
	if res.Verdict != testrun.Fail {
		return ""
	}
	r := res.Run
	leaves := r.LeafFailures()
	var parts []string
	switch {
	case len(leaves) > 0:
		names := testRunNames(leaves)
		shown := names
		if len(shown) > 3 {
			shown = append(append([]string{}, names[:3]...), fmt.Sprintf("+%d more", len(names)-3))
		}
		parts = append(parts, fmt.Sprintf("%d failed: %s", len(leaves), strings.Join(shown, ", ")))
	case len(r.BuildErrors) > 0:
		b := r.BuildErrors[0]
		lead := fmt.Sprintf("build failed: %s:%d: %s", b.File, b.Line, testRunClip(b.Message, 90))
		if n := len(r.BuildErrors) - 1; n > 0 {
			lead += fmt.Sprintf(" (+%d more)", n)
		}
		parts = append(parts, lead)
	case len(r.Panics) > 0:
		parts = append(parts, "panic: "+testRunClip(r.Panics[0].Message, 90))
	default:
		lead := "failed"
		for _, p := range r.Packages {
			if p.Status == testrun.StatusFail || p.Status == testrun.StatusBuildFailed {
				lead = "failed: " + p.Package
				break
			}
		}
		if ex := res.Evidence.ExitCode; ex != nil {
			lead += fmt.Sprintf(" (exit %d)", *ex)
		}
		parts = append(parts, lead+"; no failing test parsed")
	}
	if len(leaves) > 0 && len(r.BuildErrors) > 0 {
		b := r.BuildErrors[0]
		parts = append(parts, fmt.Sprintf("build failed: %s:%d", b.File, b.Line))
	}
	if len(r.Panics) > 0 && len(leaves) > 0 {
		// The panicking test's frame is already named beside it above.
		p := r.Panics[0]
		parts = append(parts, fmt.Sprintf("panic in %s: %s", p.Test, testRunClip(p.Message, 60)))
	}
	parts = append(parts, reachClauses(leaves, c)...)
	parts = append(parts, extra...)
	body := "🧪 fathom test run: " + strings.Join(parts, "; ")
	tail := ""
	if saved != "" {
		tail = " — full output: " + saved
	}
	if len(body)+len(tail) > testRunLineBudget {
		keep := testRunLineBudget - len(tail) - len(testRunLineCut)
		body = clipBytes(body, keep) + testRunLineCut
	}
	return body + tail
}

// testRunNames renders each leaf failure as its test name (with its first
// frame), qualified by its package's last element only when two failing
// tests share a name.
func testRunNames(leaves []testrun.Failure) []string {
	count := map[string]int{}
	for _, f := range leaves {
		count[f.Test]++
	}
	out := make([]string, 0, len(leaves))
	for _, f := range leaves {
		name := f.Test
		if count[f.Test] > 1 && f.Package != "" {
			name = path.Base(f.Package) + "." + f.Test
		}
		if len(f.Frames) > 0 {
			name += fmt.Sprintf(" (%s:%d)", f.Frames[0].File, f.Frames[0].Line)
		}
		out = append(out, name)
	}
	return out
}

// reachClauses groups the failing tests by the changed symbol they reach:
// "TestRound, TestTotal reach money.Round (changed in working tree,
// money/money.go:41)", most-reaching symbol first, at most three; when no
// failing test reaches a changed symbol, it says so and names localize's
// first candidate.
func reachClauses(leaves []testrun.Failure, c testrun.Candidates) []string {
	short := map[string]string{}
	for _, f := range leaves {
		short[f.ID()] = f.Test
	}
	type group struct {
		sym   testrun.ChangedSymbol
		tests []string
	}
	groups := map[string]*group{}
	var order []string
	for _, r := range c.Reach {
		for _, s := range r.Changed {
			g, ok := groups[s.Ref]
			if !ok {
				g = &group{sym: s}
				groups[s.Ref] = g
				order = append(order, s.Ref)
			}
			name := short[r.Test]
			if name == "" {
				name = r.Test
			}
			g.tests = append(g.tests, name)
		}
	}
	sort.SliceStable(order, func(i, j int) bool {
		a, b := groups[order[i]], groups[order[j]]
		if len(a.tests) != len(b.tests) {
			return len(a.tests) > len(b.tests)
		}
		if a.sym.File != b.sym.File {
			return a.sym.File < b.sym.File
		}
		return a.sym.Line < b.sym.Line
	})
	var out []string
	for i, ref := range order {
		if i == 3 {
			out = append(out, fmt.Sprintf("+%d more changed symbols reached", len(order)-3))
			break
		}
		g := groups[ref]
		sort.Strings(g.tests)
		verb := "reaches"
		who := strings.Join(g.tests, ", ")
		if len(g.tests) > 1 {
			verb = "reach"
			if len(g.tests) == len(leaves) {
				who = "all " + strconv.Itoa(len(g.tests))
				if len(g.tests) == 2 {
					who = "both"
				}
			}
		}
		out = append(out, fmt.Sprintf("%s %s %s (changed in working tree, %s:%d)", who, verb, g.sym.Name, g.sym.File, g.sym.Line))
	}
	if len(order) == 0 && len(leaves) > 0 && !c.ReachUnknown {
		clause := "no changed symbol reaches them"
		if len(leaves) == 1 {
			clause = "no changed symbol reaches it"
		}
		if len(c.Localized) > 0 {
			l := c.Localized[0]
			clause += fmt.Sprintf("; localize: %s (%s)", symbolDisplay(l.Symbol), l.File)
		}
		out = append(out, clause)
	}
	if c.Skipped > 0 {
		out = append(out, fmt.Sprintf("%d changed symbols not checked", c.Skipped))
	}
	return out
}

// symbolDisplay shortens an index key ("svc::dir/pkg.Name") to "pkg.Name".
func symbolDisplay(key string) string {
	if i := strings.Index(key, "::"); i >= 0 {
		key = key[i+2:]
	}
	if i := strings.LastIndex(key, "/"); i >= 0 {
		key = key[i+1:]
	}
	return key
}

// testRunClip shortens s to n bytes on a rune boundary, marking the cut.
func testRunClip(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return clipBytes(s, n-1) + "…"
}

// clipBytes returns s's longest prefix of at most n bytes ending on a rune
// boundary.
func clipBytes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if len(s) <= n {
		return s
	}
	for n > 0 && n < len(s) && (s[n]&0xC0) == 0x80 {
		n--
	}
	return s[:n]
}

// ─── the log row (TR-7) ─────────────────────────────────────────────────

// testRunRecord is one matched run, as logged.
type testRunRecord struct {
	Event          string
	Invocation     string
	Piped          bool
	Result         testrun.Result
	ExitCode       *int
	Session        string
	Reached        int
	Emitted        bool
	Bytes          int
	DeadlineMissed bool
	// RO-35: HEAD's sha, the working-tree digest and the lexed scope. Empty
	// means a miss and the row carries no such key.
	Head, Tree, Scope string
	CwdUnknown        bool
	// AgentID is the subagent that ran the command (RO-39: the input's
	// agent_id, RO-V1); "" for the main agent, and the row carries no key.
	AgentID string
	// ToolUseID is the harness's id for the call (the input's tool_use_id,
	// on PostToolUse and PostToolUseFailure alike); "" when the input
	// carries none, and the row carries no key (O1210).
	ToolUseID string
	// Delta is the rich-mode test delta's outcome (RO-23: readout.Delta*,
	// or unread); "" outside rich mode, and the row carries no key.
	Delta string
}

// testRunMaxLoggedIDs caps the failing ids one row carries.
const testRunMaxLoggedIDs = 10

// logTestRunRow writes the run's one row to the hook log: which declared
// command, which event, the runner, the three-way result, the failing
// test ids (at most 10) and counts, the exit code when carried, and what
// the hook did. Never the output text, and marked attested=false: a piped
// output is not an attested run and is never certificate evidence (I43).
func logTestRunRow(r testRunRecord) {
	run := r.Result.Run
	leaves := run.LeafFailures()
	ids := make([]string, 0, len(leaves))
	for i, f := range leaves {
		if i == testRunMaxLoggedIDs {
			break
		}
		ids = append(ids, f.ID())
	}
	meta := map[string]string{
		"event":        r.Event,
		"invocation":   r.Invocation,
		"piped":        boolStr(r.Piped),
		"runner":       string(run.Runner),
		"result":       string(r.Result.Verdict),
		"failed":       strconv.Itoa(len(leaves)),
		"panics":       strconv.Itoa(len(run.Panics)),
		"build_errors": strconv.Itoa(len(run.BuildErrors)),
		"packages":     strconv.Itoa(len(run.Packages)),
		"reached":      strconv.Itoa(r.Reached),
		"emitted":      boolStr(r.Emitted),
		"attested":     "false",
	}
	if len(ids) > 0 {
		meta["failing"] = strings.Join(ids, ",")
	}
	if r.ExitCode != nil {
		meta["exit_code"] = strconv.Itoa(*r.ExitCode)
	}
	if r.Result.Evidence.Truncated {
		meta["truncated"] = "true"
	}
	if r.Emitted {
		meta["bytes"] = strconv.Itoa(r.Bytes)
	}
	if r.AgentID != "" {
		meta["agent_id"] = r.AgentID // RO-39: a SubagentStop claim reads the subagent's own rows
	}
	if r.ToolUseID != "" {
		meta["tool_use_id"] = r.ToolUseID // O1210: the baseline locates the run in the transcript by it
	}
	if r.CwdUnknown {
		meta["cwd"] = "undeclared"
	}
	if r.Delta != "" {
		meta["delta"] = r.Delta
	}
	for k, v := range testRunRowKeys(r) {
		meta[k] = v
	}
	if r.DeadlineMissed {
		meta["deadline"] = "missed"
		// GL-24: a run the hook could not answer in time names why it said
		// nothing; the bash-write path already counts a fetch that ran out
		// of time as daemon-error.
		if !r.Emitted {
			meta["reason"] = string(silenceDaemonError)
		}
	}
	emitBeat(testRunChannel, string(r.Result.Verdict), firingMeta(meta, r.Session, ""))
}

// testRunRowKeys is RO-35's additive row meta: `head`, `tree` and `scope`,
// each present only when computed. A miss leaves the key absent, never
// empty, so a reader of the log tells "not computed" from a value.
func testRunRowKeys(r testRunRecord) map[string]string {
	keys := map[string]string{}
	for k, v := range map[string]string{"head": r.Head, "tree": r.Tree, "scope": r.Scope} {
		if v != "" {
			keys[k] = v
		}
	}
	return keys
}

// fillTestRunRowKeys computes the row's head, tree and scope inside ctx.
// head is HEAD's sha. tree is the sha256 of the Go working-tree diff
// (workingTreeDiff's text) plus the session's edit set: each untracked Go
// file the session edited (editset.go), as its path and content hash, since
// `git diff HEAD` does not see a new file. scope is the lexed command
// (testRunScope). A key whose read failed or ran past the deadline stays
// empty. It returns the diff text for the failing path's reach, and whether
// that text is a real read.
func fillTestRunRowKeys(ctx context.Context, rec *testRunRecord, root, session, command, invocation string, syntaxRoots ...string) (string, bool) {
	if len(syntaxRoots) == 0 {
		syntaxRoots = []string{root}
	}
	rec.Scope = testRunScope(command, invocation, syntaxRoots...)
	parsed := parseShellCommand(command, syntaxRoots...)
	for _, u := range parsed.Units() {
		if u.CwdUnknown {
			rec.Head, rec.Tree, rec.CwdUnknown = "", "", true
			return "", false
		}
	}
	if deadlinePassed(ctx) {
		return "", false
	}
	head, herr := gitro.Head(ctx, root)
	diff, ok := workingTreeDiffErr(ctx, root)
	if herr == nil && !deadlinePassed(ctx) {
		rec.Head = head
	}
	if !ok || deadlinePassed(ctx) {
		return diff, ok && !deadlinePassed(ctx)
	}
	h := sha256.New()
	h.Write([]byte(diff))
	if untracked, err := gitro.Output(ctx, root, "ls-files", "--others", "--exclude-standard", "-z", "--", "*.go"); err == nil {
		files := strings.Split(strings.TrimRight(string(untracked), "\x00"), "\x00")
		sort.Strings(files)
		now := time.Now()
		for _, f := range files {
			if f == "" || !fileRecentlyEdited(session, f, now) {
				continue
			}
			body, err := readCapped(filepath.Join(root, filepath.FromSlash(f)), testRunStdinCap)
			if err != nil {
				continue
			}
			sum := sha256.Sum256(body)
			fmt.Fprintf(h, "\x00untracked\x00%s\x00%x", f, sum)
		}
	}
	if deadlinePassed(ctx) {
		return diff, false
	}
	rec.Tree = hex.EncodeToString(h.Sum(nil))
	return diff, true
}

// readCapped reads at most limit bytes of the file at p.
func readCapped(p string, limit int64) ([]byte, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, limit))
}

// testRunValueFlag reports whether a `go test` flag takes its value as the
// next word (a `-flag=value` spelling carries its own), so the lexer does
// not read a value as a package pattern.
func testRunValueFlag(flag string) bool {
	switch flag {
	case "-run", "-skip", "-count", "-timeout", "-tags",
		"-p", "-parallel", "-cpu", "-bench", "-benchtime",
		"-coverprofile", "-covermode", "-coverpkg", "-vet",
		"-o", "-exec", "-ldflags", "-gcflags", "-mod",
		"-shuffle", "-fuzz", "-fuzztime", "-outputdir",
		"-blockprofile", "-cpuprofile", "-memprofile", "-mutexprofile",
		"-trace", "-asmflags", "-C", "-modfile", "-overlay",
		"-pkgdir", "-toolexec", "-installsuffix", "-buildvcs":
		return true
	}
	return false
}

// testRunScope lexes the command's matched segment into the scope it ran:
// the package patterns of a `go test` (comma-joined, first seen order) then
// `run=` and `skip=` for the -run and -skip filters, space-joined. "" when
// nothing is lexed (a task or script invocation names its scope inside the
// script). It reads the one command lexer, so a runner named inside a quoted
// argument never reads as the invocation;.
func testRunScope(command, invocation string, roots ...string) string {
	if command == "" || invocation == "" {
		return ""
	}
	want := strings.Fields(invocation)
	segs, unreadable := lexShellSegments(command, roots...)
	if unreadable {
		return "" // RO-4: an unreadable command has no lexed scope
	}
	for _, seg := range segs {
		words := stripCommandWrappers(seg.words)
		if len(want) == 0 || !invocationMatches(words, want) {
			continue
		}
		args := words[len(want):]
		goTest := len(want) >= 2 && path.Base(want[0]) == "go" && want[1] == "test"
		var pkgs []string
		seen := map[string]bool{}
		var run, skip string
	scan:
		for i := 0; i < len(args); i++ {
			a := args[i]
			switch {
			case a == "-args" || a == "--":
				break scan
			case strings.HasPrefix(a, "-"):
				name, val, hasVal := strings.Cut(strings.TrimLeft(a, "-"), "=")
				if !hasVal && testRunValueFlag("-"+name) && i+1 < len(args) {
					i++
					val, hasVal = args[i], true
				}
				switch name {
				case "run":
					run = val
				case "skip":
					skip = val
				}
			case goTest && !seen[a]:
				seen[a] = true
				pkgs = append(pkgs, a)
			}
		}
		var parts []string
		if len(pkgs) > 0 {
			parts = append(parts, strings.Join(pkgs, ","))
		}
		if run != "" {
			parts = append(parts, "run="+run)
		}
		if skip != "" {
			parts = append(parts, "skip="+skip)
		}
		return strings.Join(parts, " ")
	}
	return ""
}
