package main

// Rich mode's injected lines (docs/plans/readouts.md phase 2, ADR-071):
// built now and reachable only under the user's [readouts] mode = rich
// (default off); turning rich on waits for RO-M1's PASS. A line is added
// beside the raw output, never in place of it, so the output and any saved
// path stand whole. A miss is silent and named in the row (rich_missed),
// never a wrong line.

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/fathomhome"
	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/readout"
	"github.com/geoffmilleraz/fathom/internal/shellcmd"
	"github.com/geoffmilleraz/fathom/internal/testrun"
)

// readoutRichLineBudget caps each rich line: one line, well inside I35's
// 320-token whole-payload ceiling, as the test-run line's cap is.
const readoutRichLineBudget = testRunLineBudget

// readoutRich is one run's rich lines and what its row records. The lines
// the compile records feed (the compile line and its tests line) lead, the
// first enrich of them, and the policy table decides whether they are
// served; the guard line after them is the command's own advisory and
// stands apart from the table.
type readoutRich struct {
	lines  []string
	kinds  []string
	missed []string
	enrich int
	// records counts the enrichment records by kind; overlap counts those
	// naming the session's edits, failing those naming a test in the last
	// run's failing set (the table's EditedOverlap and FailingOverlap).
	records          map[string]int
	overlap, failing int
	// choice is the table's answer (chose once asked), fitted whether the
	// user's fitted cut points answered, withheld the kinds it held back.
	choice   readout.Choice
	chose    bool
	fitted   bool
	withheld []string
}

// readoutRichCodec is the hook's codec that also keeps the context the
// test-run tier encodes, so a rich line joins the tier's line in one hook
// output instead of a second JSON document on stdout.
type readoutRichCodec struct {
	harness.Codec
	contexts []string
}

func (c *readoutRichCodec) EncodeOutput(ev harness.Event, out harness.HookOutput) ([]byte, error) {
	b, err := c.Codec.EncodeOutput(ev, out)
	if err == nil && len(b) > 0 && out.Context != "" {
		c.contexts = append(c.contexts, out.Context)
	}
	return b, err
}

// readoutRichOutput holds the test-run tier's output while rich mode may
// add a line: flush writes the tier's bytes unchanged when no rich line was
// added, else one output carrying the tier's context and the rich lines.
type readoutRichOutput struct {
	codec   harness.Codec
	ev      harness.Event
	tier    bytes.Buffer
	capture *readoutRichCodec
	done    bool
}

func newReadoutRichOutput(codec harness.Codec, ev harness.Event) *readoutRichOutput {
	return &readoutRichOutput{codec: codec, ev: ev, capture: &readoutRichCodec{Codec: codec}}
}

// flush writes once; it reports whether the rich lines were emitted.
func (o *readoutRichOutput) flush(stdout io.Writer, lines []string) bool {
	if o == nil || o.done {
		return false
	}
	o.done = true
	if len(lines) > 0 {
		ctx := strings.Join(append(append([]string(nil), o.capture.contexts...), lines...), "\n")
		if out, err := o.codec.EncodeOutput(o.ev, harness.HookOutput{Context: ctx}); err == nil && len(out) > 0 {
			fmt.Fprintln(stdout, string(out))
			return true
		}
	}
	_, _ = stdout.Write(o.tier.Bytes())
	return false
}

// readoutRichLines computes rich mode's lines for one call inside ctx; the
// daemon joins take their own deadline inside the hook's ceiling, counted
// from start (RO-30).
func readoutRichLines(ctx context.Context, in harness.HookInput, dir string, c *shellcmd.Command, start time.Time) readoutRich {
	cl := readoutCompileLine(ctx, in, dir, c, start)
	r := readoutRich{missed: cl.missed, overlap: cl.overlap}
	if cl.line != "" {
		r.lines = append(r.lines, cl.line)
		r.kinds = append(r.kinds, readout.RecordCompileError)
		r.records = map[string]int{readout.RecordCompileError: cl.records}
	}
	if cl.tests != "" {
		r.lines = append(r.lines, cl.tests)
		r.kinds = append(r.kinds, readoutRichTestsKind)
		if r.records == nil {
			r.records = map[string]int{}
		}
		r.records[readoutRichTestsKind] = len(cl.shown)
		r.failing = readoutFailingOverlap(in, cl.shown)
	}
	r.enrich = len(r.lines)
	if ctx.Err() != nil {
		return r
	}
	if line := readoutGuardLine(dir, c); line != "" {
		r.lines = append(r.lines, line)
		r.kinds = append(r.kinds, readoutRichGuardKind)
	}
	return r
}

// goCompileTool names the `go build` or `go vet` a command runs, "" when it
// runs neither.
func goCompileTool(c *shellcmd.Command) string {
	if c == nil || c.Unreadable {
		return ""
	}
	for _, u := range c.Units() {
		if u.Kind != shellcmd.KindSimple || u.Name() != "go" || len(u.Words) < 2 {
			continue
		}
		if sub := u.Words[1].Text; sub == "build" || sub == "vet" {
			return "go " + sub
		}
	}
	return ""
}

// readoutRichTestsKind is the rich kind of the tests line RO-30's join adds.
const readoutRichTestsKind = "tests_to_run"

// readoutCompile is readoutCompileLine's answer: the line and the tests
// line, the joins that missed, and what the policy table reads of them.
type readoutCompile struct {
	line, tests string
	missed      []string
	// records is the compile_error records' count; overlap the groups that
	// name the session's edits (readout.CompileOverlap); shown the tests
	// the tests line names.
	records, overlap int
	shown            []testRef
}

// readoutCompileLine is RO-24's line for a go build or go vet run: its
// errors as compile_error records grouped by root, the root a symbol the
// working tree changed placed at its definition, and the group touching a
// file this session edited first. RO-30's daemon joins place an identifier
// root the working tree did not change at the index's definition and name
// the tests that reach the changed files (tests, its own line); a missed
// join reads "index: not consulted (<reason>)" at the line's end. missed
// names a join that did not answer.
func readoutCompileLine(ctx context.Context, in harness.HookInput, dir string, c *shellcmd.Command, start time.Time) (cl readoutCompile) {
	tool := goCompileTool(c)
	if tool == "" {
		return cl
	}
	text := in.Shell.Stdout
	if in.Shell.Stderr != "" {
		text += "\n" + in.Shell.Stderr
	}
	root := gitToplevelByStat(dir)
	norm := func(p string) string {
		abs := p
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(dir, p)
		}
		if root != "" {
			if rel, ok := relativeToRoot(root, abs); ok {
				return filepath.ToSlash(rel)
			}
		}
		return strings.TrimPrefix(filepath.ToSlash(p), "./")
	}
	recs := readout.CompileErrors(compileReadoutText(c, text), norm)
	if len(recs) == 0 {
		return cl
	}
	cl.records = len(recs)
	var defs []readout.CompileDef
	var files []string
	if root != "" {
		if diff, ok := workingTreeDiffErr(ctx, root); ok {
			for _, s := range testrun.ChangedGoSymbols(root, diff) {
				defs = append(defs, readout.CompileDef{Name: s.Name, File: s.File, Line: s.Line})
				if !slices.Contains(files, s.File) {
					files = append(files, s.File)
				}
			}
		} else {
			cl.missed = append(cl.missed, "changed_symbols")
		}
	}
	changed := slices.Clone(defs)
	suffix := ""
	if root != "" {
		idx, reached, miss := readoutDaemonJoins(readoutHookClock().Sub(start), root, readout.UnplacedIdents(recs, defs), files)
		defs = append(defs, idx...)
		if miss != "" {
			suffix = "; " + daemonJoinMissLine(miss)
			cl.missed = append(cl.missed, "index")
		} else if shown, more := pickTestsToRun(reached, ""); len(shown) > 0 {
			cl.tests, cl.shown = testsToRunText(shown, more, ""), shown
		}
	}
	now := readoutHookClock()
	edited := func(file string) bool {
		if root == "" {
			return false
		}
		abs := filepath.Join(root, filepath.FromSlash(file))
		for _, cand := range localCandidateRoots(dir) {
			if rel, ok := relativeToRoot(cand, abs); ok && fileRecentlyEdited(in.SessionID, rel, now) {
				return true
			}
		}
		return fileInEditSet(abs, in.Cwd, in.SessionID)
	}
	cl.overlap = readout.CompileOverlap(recs, changed, edited)
	cl.line = readout.CompileErrorLine(tool, recs, defs, edited, readoutRichLineBudget-len(suffix)) + suffix
	return cl
}

var compileReadoutHeader = regexp.MustCompile(`^# \S+(?: \[.*\])?$`)
var compileReadoutDiagnostic = regexp.MustCompile(`^(?:vet: )?([^\s:][^:]*\.go):\d+:(?:(\d+):)? (.+)$`)
var compileReadoutLinterSuffix = regexp.MustCompile(` \([A-Za-z][A-Za-z0-9_-]*\)$`)

// compileReadoutText keeps diagnostics attributable to build or vet. A
// mixed command needs a package header; a head/tail-cut compiler output
// can instead establish its origin with the full file:line:column shape.
func compileReadoutText(c *shellcmd.Command, text string) string {
	mixed := false
	for _, u := range c.Units() {
		switch u.Name() {
		case "gofmt", "deadcode", "staticcheck", "golangci-lint":
			mixed = true
		}
	}
	lines := strings.Split(text, "\n")
	header := false
	for i, line := range lines {
		line = strings.TrimRight(line, "\r")
		if compileReadoutHeader.MatchString(line) {
			header = true
			continue
		}
		m := compileReadoutDiagnostic.FindStringSubmatch(line)
		if strings.TrimSpace(line) == "" || (mixed && m != nil && compileReadoutLinterSuffix.MatchString(m[3])) {
			header = false
			lines[i] = ""
			continue
		}
		keep := m != nil
		if keep {
			for _, part := range strings.Split(strings.TrimPrefix(filepath.ToSlash(m[1]), "./"), "/") {
				if part == "testdata" || strings.HasPrefix(part, "_") || (part != ".." && strings.HasPrefix(part, ".")) {
					keep = false
				}
			}
			msg := m[3]
			if strings.HasPrefix(msg, "unreachable func:") {
				keep = false
			}
			moduleOrEmbed := strings.HasPrefix(msg, "no required module provides package ") ||
				strings.HasPrefix(msg, "missing go.sum entry for module providing package ") ||
				(strings.HasPrefix(msg, "module ") && strings.Contains(msg, "does not contain package ")) ||
				(strings.HasPrefix(msg, "package ") && strings.Contains(msg, " is not in std")) ||
				strings.HasPrefix(msg, "cannot find package ") || strings.Contains(msg, "import lookup disabled by -mod=") ||
				strings.Contains(msg, "go:embed") || (strings.HasPrefix(msg, "pattern ") && strings.Contains(msg, ": no matching files found"))
			if !header && (mixed || m[2] == "" || moduleOrEmbed) {
				keep = false
			}
		}
		if !keep {
			lines[i] = "" // preserve the raw diagnostic's line number
			if line != "" && !strings.HasPrefix(line, "\t") && !strings.HasPrefix(line, " ") {
				header = false
			}
		}
	}
	return strings.Join(lines, "\n")
}

// readoutFailingOverlap counts the tests the tests line names that the
// session's latest test-run row logged failing (the agent's own run, never
// a sub-agent's when the call is the main agent's): the table's
// FailingOverlap. An unreadable log counts none.
func readoutFailingOverlap(in harness.HookInput, shown []testRef) int {
	rows, ok := sessionTestRunRows(beatLogPath(), in.SessionID, in.AgentID)
	if !ok || len(rows) == 0 {
		return 0
	}
	failing := rows[len(rows)-1].Failing
	n := 0
	for _, t := range shown {
		for _, f := range failing {
			if f == t.name || strings.HasSuffix(f, "."+t.name) || strings.HasPrefix(path.Base(f), t.name+"/") || strings.Contains(f, "."+t.name+"/") {
				n++
				break
			}
		}
	}
	return n
}

// readoutRichChoose asks the policy table whether this call's enrichment
// lines are served, from the call's counts and flags only (never its text,
// I61), and holds them back unless it answers enriched. The table answers
// with the user's fitted cut points when that file loads, else the
// built-ins; lean is never served here (no rule is promoted). The guard
// line is not the table's to withhold.
func readoutRichChoose(ctx context.Context, r *readoutRich, ev harness.Event, in harness.HookInput, dir string, c *shellcmd.Command, rin readout.Input, ro readout.Readout, rule *readout.Rule, joins readoutJoinRun, quiet []string) {
	f := readout.Features{
		Event: readout.EventSuccess, Bytes: ro.BytesIn, Lines: ro.LinesIn, Records: r.records,
		EditedOverlap: r.overlap, FailingOverlap: r.failing, Recovery: -1, SubAgent: in.AgentID != "",
	}
	if ev == harness.EventToolFailure || in.Shell.Interrupted {
		f.Event = readout.EventFailure
	}
	f.Class = ro.Stream
	if f.Class == "" {
		f.Class = readout.StreamOf(c, quiet)
	}
	if f.Class == readout.StreamDiagnostic {
		f.Protected = readout.ProtectedLines(in.Shell.Stdout)
	}
	f.Protected += readout.ProtectedLines(in.Shell.Stderr)
	if rule != nil {
		f.RuleFloor = rule.When.BytesGT
	}
	if p, ok := readout.WholeFileRead(c); ok {
		f.WholeRead = true
		if slices.Contains(joins.ran, readoutJoinEdited) {
			f.ReadEdited = rin.Edited
		} else if ctx.Err() == nil {
			f.ReadEdited, _ = readoutFileEdited(ctx, in, dir, p)
		}
	}
	p, fitted, _ := readout.LoadPolicyFile(fathomhome.FathomHome())
	r.fitted = fitted
	r.choice, r.chose = readout.Choose(f, p), true
	if r.choice.Mode != readout.ChoiceEnriched && r.enrich > 0 {
		r.withheld = append(r.withheld, r.kinds[:r.enrich]...)
		r.lines, r.kinds = slices.Clone(r.lines[r.enrich:]), slices.Clone(r.kinds[r.enrich:])
		r.enrich = 0
	}
}

// readoutPolicyMeta records the table's answer on the row, the receipt a
// raw result carries since it carries no fathom text: the choice, the rule
// that answered, decided_by table, the contested band, whether fitted cut
// points answered, the kinds held back, and the features the fit reads back
// (the overlaps and the records by kind; counts only, never text).
func readoutPolicyMeta(meta map[string]string, r readoutRich) {
	meta["policy"] = r.choice.Mode
	meta["policy_rule"] = strconv.Itoa(r.choice.Rule)
	meta["decided_by"] = r.choice.DecidedBy
	if r.choice.Contested {
		meta["contested"] = "true"
	}
	if r.fitted {
		meta["policy_cuts"] = "fitted"
	}
	meta["edited_overlap"] = strconv.Itoa(r.overlap)
	meta["failing_overlap"] = strconv.Itoa(r.failing)
	if len(r.records) > 0 {
		meta["records"] = readoutKindCounts(r.records)
	}
	if len(r.withheld) > 0 {
		meta["rich_withheld"] = strings.Join(r.withheld, ",")
	}
}

// readoutKindCounts is counts by kind as sorted "kind:n" pairs.
func readoutKindCounts(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + ":" + strconv.Itoa(m[k])
	}
	return strings.Join(parts, ",")
}
