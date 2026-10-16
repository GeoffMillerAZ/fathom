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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/gitro"
	"github.com/geoffmilleraz/fathom/internal/harness"
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

// cmdTestRunHook is the hook body for one harness event: decode, gate,
// parse, explain, inject, log. It always returns 0 (I9).
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
		root = eventProjectDir(codec, in)
	}
	if root == "" {
		return 0
	}
	// The gate, cheapest first, all local: a layer that never names
	// test_run cannot switch the hook on; a command that runs no declared
	// test is not the hook's business; only then the validated config.
	// The raw pre-gate honors the scope registry (SEC-2): the repository's
	// file can open it only for a field a repository may set, so a
	// user-only field config.Load would drop never opens the gate.
	userRaw, projRaw := readConfigLayer(config.DefaultUserPath()), readConfigLayer(filepath.Join(root, ".fathom", "config.toml"))
	names := func(key, word string) bool {
		return bytes.Contains(userRaw, []byte(word)) || (config.RepoMaySet(key) && bytes.Contains(projRaw, []byte(word)))
	}
	if !names("hooks.test_run", "test_run") {
		return 0
	}
	m, matched := matchDeclaredTestCommand(root, in.Command, nil)
	var loaded *config.Loaded
	if !matched && names("tests.commands", "commands") {
		if l, err := config.Load(config.DefaultUserPath(), filepath.Join(root, ".fathom", "config.toml")); err == nil {
			loaded = &l
			m, matched = matchDeclaredTestCommand(root, in.Command, l.Config.Tests.Commands)
		}
	}
	if !matched {
		return 0
	}
	if loaded == nil {
		l, err := config.Load(config.DefaultUserPath(), filepath.Join(root, ".fathom", "config.toml"))
		if err != nil {
			return 0
		}
		loaded = &l
	}
	if !loaded.Config.Hooks.TestRun {
		return 0
	}
	rpcOrigin = testRunChannel

	res := testrun.Analyze(in.ToolOutput, testrun.Evidence{ExitCode: in.ExitCode, Filtered: m.Piped, Truncated: in.OutputTruncated})
	relativizeFrames(&res.Run, root)
	rec := testRunRecord{Event: event, Invocation: m.Invocation, Piped: m.Piped, Result: res, ExitCode: in.ExitCode, Session: in.SessionID}
	defer func() { logTestRunRow(rec) }()
	if res.Verdict != testrun.Fail {
		return 0
	}
	full := in.ToolOutput
	if ev == harness.EventToolFailure && in.ToolError != "" {
		full = in.ToolError // keep the "Exit code N" line in the saved copy
	}
	saved := saveTestRunOutput(root, full)

	ctx, cancel := testRunContext()
	defer cancel()
	line, missed := runTestRunWithDeadline(ctx, func(ctx context.Context) string {
		changed := testrun.ChangedGoSymbols(root, workingTreeDiff(ctx, root))
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
		return renderTestRunLine(res, cands, saved)
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
	if ctx.Err() != nil {
		return "", true
	}
	line := work(ctx)
	if ctx.Err() != nil {
		return "", true
	}
	return line, false
}

// workingTreeDiff is `git diff HEAD --unified=0` over Go files, read
// without writing the repository (gitro), or "" on any error.
func workingTreeDiff(ctx context.Context, root string) string {
	out, err := gitro.Output(ctx, root, "diff", "HEAD", "--unified=0", "--no-color", "--no-ext-diff", "--", "*.go")
	if err != nil {
		return ""
	}
	return string(out)
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
	return rel
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
// they reach, or localize's candidate when none, then the saved output's
// path — which survives any cut. Deterministic, one line, at most
// testRunLineBudget bytes.
func renderTestRunLine(res testrun.Result, c testrun.Candidates, saved string) string {
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
	if len(order) == 0 && len(leaves) > 0 {
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
	if r.Emitted {
		meta["bytes"] = strconv.Itoa(r.Bytes)
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
