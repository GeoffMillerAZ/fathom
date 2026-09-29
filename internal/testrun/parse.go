// Package testrun is the test-run hook's deterministic core (ADR-056
// decision 6; docs/design/test-run-hook.md sections 3 and 4; plan
// docs/plans/test-run-hook.md, TR-3 to TR-5): tier 0 parses a test run's
// output into failing test ids, panics, file:line frames and build errors
// and reads the three-way result (pass, fail, unknown); tier 1 names the
// changed symbols each failing test reaches.
//
// Everything here is a pure function of its inputs (the output text, the
// hook's evidence, the working-tree diff and the index's answers behind the
// Index port): no clock, no network, no command execution, sorted outputs.
// A piped test output is never an attested run (I43): nothing here writes
// or reads certificate evidence.
package testrun

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Runner names the output format a parse recognised.
type Runner string

// The runners tier 0 parses (other runners only on demand, design section 3).
const (
	RunnerNone   Runner = "none"
	RunnerGo     Runner = "go"
	RunnerGoJSON Runner = "go-json"
	RunnerJUnit  Runner = "junit"
)

// Frame is one file:line location named in a failure's output.
type Frame struct {
	File string `json:"file"`
	Line int    `json:"line"`
}

// Failure is one failing test. Package is the import path (go) or the test
// case's classname (JUnit); "" when the output never said.
type Failure struct {
	Package string  `json:"package,omitempty"`
	Test    string  `json:"test"`
	Frames  []Frame `json:"frames,omitempty"`
}

// ID is the failing test's id: "<package>.<test>", or the test alone when
// the package is unknown.
func (f Failure) ID() string {
	if f.Package == "" {
		return f.Test
	}
	return f.Package + "." + f.Test
}

// Panic is one panic in the run, with its project frames (runtime and
// testing frames dropped).
type Panic struct {
	Package string  `json:"package,omitempty"`
	Test    string  `json:"test,omitempty"`
	Message string  `json:"message"`
	Frames  []Frame `json:"frames,omitempty"`
}

// BuildError is one compile error that stopped a package's tests.
type BuildError struct {
	Package string `json:"package,omitempty"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Col     int    `json:"col,omitempty"`
	Message string `json:"message"`
}

// Status is a package's final result line.
type Status string

// Package statuses.
const (
	StatusOK          Status = "ok"
	StatusFail        Status = "fail"
	StatusBuildFailed Status = "build-failed"
	StatusNoTests     Status = "no-tests"
	StatusSkip        Status = "skip"
)

// PackageResult is one package's (or JUnit suite's) final result.
type PackageResult struct {
	Package string `json:"package"`
	Status  Status `json:"status"`
}

// Summary is a JUnit document's root counters.
type Summary struct {
	Tests    int `json:"tests"`
	Failures int `json:"failures"`
	Errors   int `json:"errors"`
	Skipped  int `json:"skipped"`
}

// Run is a parsed test run. Complete is the parse's own coverage check:
// the output carries a final result for every package it shows (go: no
// failure, panic or started test is left without its package's result
// line; go -json: every package that started reached a final event;
// JUnit: the document's root closed). It says nothing about packages the
// output never showed — a filtered or truncated output is the caller's
// evidence, not the parser's.
type Run struct {
	Runner      Runner          `json:"runner"`
	Failures    []Failure       `json:"failures,omitempty"`
	Panics      []Panic         `json:"panics,omitempty"`
	BuildErrors []BuildError    `json:"build_errors,omitempty"`
	Packages    []PackageResult `json:"packages,omitempty"`
	Summary     *Summary        `json:"summary,omitempty"`
	Complete    bool            `json:"complete"`
}

// LeafFailures drops a failing test whose failure is implied by a failing
// subtest (TestTable when TestTable/half failed), keeping the order.
func (r Run) LeafFailures() []Failure {
	out := make([]Failure, 0, len(r.Failures))
	for _, f := range r.Failures {
		implied := false
		for _, g := range r.Failures {
			if g.Package == f.Package && strings.HasPrefix(g.Test, f.Test+"/") {
				implied = true
				break
			}
		}
		if !implied {
			out = append(out, f)
		}
	}
	return out
}

// Failed reports any failure evidence in the parse: a failing test, a
// panic, a build error, a failed package line or a JUnit failure count.
func (r Run) Failed() bool {
	if len(r.Failures)+len(r.Panics)+len(r.BuildErrors) > 0 {
		return true
	}
	for _, p := range r.Packages {
		if p.Status == StatusFail || p.Status == StatusBuildFailed {
			return true
		}
	}
	return r.Summary != nil && r.Summary.Failures+r.Summary.Errors > 0
}

// maxFrames caps the frames kept per failure or panic.
const maxFrames = 3

// ParseTestRun is tier 0: it recognises plain `go test`, `go test -json`
// and JUnit XML in output (any other text is RunnerNone) and returns the
// run it describes, sorted and deduplicated.
func ParseTestRun(output string) Run {
	output = strings.ReplaceAll(output, "\r\n", "\n")
	var r Run
	switch {
	case junitStart(output) >= 0:
		r = parseJUnit(output[junitStart(output):])
	case looksGoJSON(output):
		r = parseGoJSON(output)
	case looksGoPlain(output):
		r = parseGoPlain(output)
	default:
		return Run{Runner: RunnerNone}
	}
	r.normalize()
	return r
}

// normalize sorts every slice and drops duplicates, so equal runs compare
// equal whatever order the runner printed them in.
func (r *Run) normalize() {
	sort.SliceStable(r.Failures, func(i, j int) bool { return r.Failures[i].ID() < r.Failures[j].ID() })
	fails := r.Failures[:0]
	for i, f := range r.Failures {
		if i > 0 && f.ID() == fails[len(fails)-1].ID() {
			last := &fails[len(fails)-1]
			last.Frames = appendFrames(last.Frames, f.Frames...)
			continue
		}
		fails = append(fails, f)
	}
	r.Failures = fails
	sort.SliceStable(r.Panics, func(i, j int) bool {
		a, b := r.Panics[i], r.Panics[j]
		if a.Package != b.Package {
			return a.Package < b.Package
		}
		if a.Test != b.Test {
			return a.Test < b.Test
		}
		return a.Message < b.Message
	})
	sort.SliceStable(r.BuildErrors, func(i, j int) bool {
		a, b := r.BuildErrors[i], r.BuildErrors[j]
		if a.Package != b.Package {
			return a.Package < b.Package
		}
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Line < b.Line
	})
	be := r.BuildErrors[:0]
	for i, b := range r.BuildErrors {
		if i > 0 && b == be[len(be)-1] {
			continue
		}
		be = append(be, b)
	}
	r.BuildErrors = be
	sort.SliceStable(r.Packages, func(i, j int) bool { return r.Packages[i].Package < r.Packages[j].Package })
	if len(r.Failures) == 0 {
		r.Failures = nil
	}
	if len(r.BuildErrors) == 0 {
		r.BuildErrors = nil
	}
}

func appendFrames(dst []Frame, fs ...Frame) []Frame {
	for _, f := range fs {
		if len(dst) >= maxFrames {
			return dst
		}
		dup := false
		for _, d := range dst {
			if d == f {
				dup = true
				break
			}
		}
		if !dup {
			dst = append(dst, f)
		}
	}
	return dst
}

// ─── plain go test ──────────────────────────────────────────────────────

var (
	goPkgLineRe   = regexp.MustCompile(`^(ok|FAIL|\?)\s+(\S+)(?:\s+(.*))?$`)
	goFailRe      = regexp.MustCompile(`^\s*--- FAIL: (\S+) \(`)
	goPassSkipRe  = regexp.MustCompile(`^\s*--- (?:PASS|SKIP): (\S+) \(`)
	goEventRe     = regexp.MustCompile(`^=== (?:RUN|CONT|NAME|PAUSE)\s+(\S+)`)
	goLogFrameRe  = regexp.MustCompile(`^\s+([A-Za-z0-9_.\-/]+\.go):(\d+):`)
	goStackRe     = regexp.MustCompile(`^\t(\S+\.go):(\d+)(?: \+0x[0-9a-f]+)?$`)
	goBuildHdrRe  = regexp.MustCompile(`^# (\S+)`)
	goBuildErrRe  = regexp.MustCompile(`^(?:vet: )?([A-Za-z0-9_.\-/]+\.go):(\d+)(?::(\d+))?: (.+)$`)
	goRunningRe   = regexp.MustCompile(`^\s+(Test\S*) \(`)
	goPanicRe     = regexp.MustCompile(`^panic: (.+)$`)
	goPlainSignRe = regexp.MustCompile(`(?m)^(?:(?:ok|FAIL|\?)\s+\S+|\s*--- (?:FAIL|PASS|SKIP): |=== RUN\s|PASS$|FAIL$|panic: )`)
)

// goStackTestRe is a stack frame's function line inside a test function (its
// body or a closure in it): "github.com/o/r/x.TestX(0x1400…)" or
// "….TestX.func1()". The receiver-less top-level Test name is the test.
var goStackTestRe = regexp.MustCompile(`^[^\s(]+\.(Test[^.(\s]*)(?:\.func\d+(?:\.\d+)*)?\(`)

func looksGoPlain(s string) bool { return goPlainSignRe.MatchString(s) }

// goLineScanner holds the plain parser's state; the -json parser reuses it
// for each test's and package's own output.
type goLineScanner struct {
	run *Run
	// pending holds failures and panics printed before their package's
	// result line names the package (plain output only).
	pendFail  []Failure
	pendPanic []Panic
	// logs buffers a running test's log frames until its --- line (verbose
	// output logs before the verdict, the default after it).
	logs       map[string][]Frame
	current    string // the test the next log lines belong to
	lastFailed string
	inPanic    *Panic
	lastFunc   string
	running    bool // inside a "running tests:" block of a timeout panic
	buildPkg   string
	dangling   bool // something was printed that no package result closed
}

func newGoLineScanner(r *Run) *goLineScanner {
	return &goLineScanner{run: r, logs: map[string][]Frame{}}
}

func parseGoPlain(s string) Run {
	r := Run{Runner: RunnerGo}
	sc := newGoLineScanner(&r)
	for _, line := range strings.Split(s, "\n") {
		sc.line(line, true)
	}
	r.Complete = len(r.Packages) > 0 && !sc.dangling && len(sc.pendFail) == 0 && len(sc.pendPanic) == 0
	// Output cut before a package's result line (`| head`, an interrupted
	// run) still names what failed; the package stays unknown.
	sc.flush("")
	return r
}

// line consumes one line. pkgLines says whether package result lines are
// honoured (plain output); the -json parser takes results from events.
func (sc *goLineScanner) line(line string, pkgLines bool) {
	if sc.inPanic != nil {
		if sc.panicLine(line) {
			return
		}
	}
	switch {
	case goBuildHdrRe.MatchString(line):
		sc.buildPkg = strings.Fields(goBuildHdrRe.FindStringSubmatch(line)[1])[0]
		return
	case sc.buildPkg != "" && goBuildErrRe.MatchString(line):
		m := goBuildErrRe.FindStringSubmatch(line)
		ln, _ := strconv.Atoi(m[2])
		col, _ := strconv.Atoi(m[3])
		sc.run.BuildErrors = append(sc.run.BuildErrors, BuildError{Package: sc.buildPkg, File: m[1], Line: ln, Col: col, Message: m[4]})
		return
	}
	sc.buildPkg = ""
	if pkgLines {
		if m := goPkgLineRe.FindStringSubmatch(line); m != nil && !strings.HasSuffix(m[2], ":") {
			sc.packageLine(m[1], m[2], m[3])
			return
		}
	}
	if m := goFailRe.FindStringSubmatch(line); m != nil {
		test := m[1]
		sc.pendFail = append(sc.pendFail, Failure{Test: test, Frames: appendFrames(nil, sc.logs[test]...)})
		delete(sc.logs, test)
		sc.current, sc.lastFailed, sc.dangling = test, test, true
		return
	}
	if m := goPassSkipRe.FindStringSubmatch(line); m != nil {
		delete(sc.logs, m[1])
		sc.current = ""
		return
	}
	if m := goEventRe.FindStringSubmatch(line); m != nil {
		sc.current, sc.dangling = m[1], true
		return
	}
	if m := goPanicRe.FindStringSubmatch(line); m != nil {
		sc.pendPanic = append(sc.pendPanic, Panic{Test: sc.lastFailed, Message: m[1]})
		sc.inPanic = &sc.pendPanic[len(sc.pendPanic)-1]
		sc.lastFunc, sc.running, sc.dangling = "", false, true
		return
	}
	if m := goLogFrameRe.FindStringSubmatch(line); m != nil && sc.current != "" {
		ln, _ := strconv.Atoi(m[2])
		f := Frame{File: m[1], Line: ln}
		if n := len(sc.pendFail); n > 0 && sc.pendFail[n-1].Test == sc.current {
			sc.pendFail[n-1].Frames = appendFrames(sc.pendFail[n-1].Frames, f)
		} else {
			sc.logs[sc.current] = appendFrames(sc.logs[sc.current], f)
		}
	}
}

// panicLine consumes a line inside a panic's trace; false ends the trace.
func (sc *goLineScanner) panicLine(line string) bool {
	p := sc.inPanic
	switch {
	case strings.TrimSpace(line) == "" || strings.HasPrefix(line, "goroutine ") || strings.HasPrefix(line, "\tpanic: "):
		return true
	case strings.HasPrefix(line, "running tests:"):
		sc.running = true
		return true
	case sc.running && goRunningRe.MatchString(line):
		if p.Test == "" {
			p.Test = goRunningRe.FindStringSubmatch(line)[1]
		}
		return true
	case goStackRe.MatchString(line):
		if projectFrame(sc.lastFunc) {
			m := goStackRe.FindStringSubmatch(line)
			ln, _ := strconv.Atoi(m[2])
			p.Frames = appendFrames(p.Frames, Frame{File: m[1], Line: ln})
		}
		return true
	case strings.HasPrefix(line, "created by ") || (!strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && strings.HasSuffix(line, ")")):
		sc.lastFunc = line
		// A piped tail (`| tail -30`) can lose the "--- FAIL:" line above a
		// long stack (O175): the stack's own test frame still names the test.
		if m := goStackTestRe.FindStringSubmatch(line); m != nil && p.Test == "" && projectFrame(line) {
			p.Test = m[1]
		}
		return true
	}
	sc.inPanic, sc.running = nil, false
	return false
}

// projectFrame reports whether a stack frame's function line belongs to the
// code under test rather than the Go runtime or the testing package.
func projectFrame(funcLine string) bool {
	f := strings.TrimPrefix(funcLine, "created by ")
	for _, p := range []string{"runtime.", "testing.", "panic(", "reflect.", "sync.", "internal/"} {
		if strings.HasPrefix(f, p) {
			return false
		}
	}
	return f != ""
}

// packageLine closes a package: it names the pending failures and panics.
func (sc *goLineScanner) packageLine(verb, pkg, rest string) {
	st := StatusOK
	switch verb {
	case "FAIL":
		st = StatusFail
		if strings.Contains(rest, "[build failed]") || strings.Contains(rest, "[setup failed]") || strings.HasPrefix(pkg, "[") {
			st = StatusBuildFailed
		}
	case "?":
		st = StatusNoTests
	}
	sc.run.Packages = append(sc.run.Packages, PackageResult{Package: pkg, Status: st})
	sc.flush(pkg)
}

// flush assigns the pending failures and panics to pkg. A panic that names
// a test no "--- FAIL:" line named (the line fell outside a piped tail, or a
// timeout's "running tests:" block named it) is that test's failure too: a
// test that panicked failed.
func (sc *goLineScanner) flush(pkg string) {
	for _, p := range sc.pendPanic {
		if p.Test == "" {
			continue
		}
		named := false
		for _, f := range sc.pendFail {
			if f.Test == p.Test {
				named = true
				break
			}
		}
		if !named {
			sc.pendFail = append(sc.pendFail, Failure{Test: p.Test})
		}
	}
	for _, f := range sc.pendFail {
		f.Package = pkg
		for _, p := range sc.pendPanic {
			if p.Test == f.Test {
				f.Frames = appendFrames(f.Frames, p.Frames...)
			}
		}
		sc.run.Failures = append(sc.run.Failures, f)
	}
	for _, p := range sc.pendPanic {
		p.Package = pkg
		sc.run.Panics = append(sc.run.Panics, p)
	}
	sc.pendFail, sc.pendPanic, sc.inPanic = nil, nil, nil
	sc.logs = map[string][]Frame{}
	sc.current, sc.lastFailed, sc.dangling = "", "", false
}

// ─── go test -json ──────────────────────────────────────────────────────

type goEvent struct {
	Action      string `json:"Action"`
	Package     string `json:"Package"`
	Test        string `json:"Test"`
	Output      string `json:"Output"`
	ImportPath  string `json:"ImportPath"`
	FailedBuild string `json:"FailedBuild"`
}

func looksGoJSON(s string) bool {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var ev goEvent
		if json.Unmarshal([]byte(line), &ev) == nil && ev.Action != "" {
			return true
		}
	}
	return false
}

func parseGoJSON(s string) Run {
	r := Run{Runner: RunnerGoJSON}
	type key struct{ pkg, test string }
	outputs := map[key][]string{}
	var order []key
	started := map[string]bool{}
	terminal := map[string]bool{}
	failed := map[key]bool{}
	buildOut := map[string][]string{}
	var buildOrder []string
	var stray []string // non-JSON lines (a build's stderr in -json mode)
	for _, line := range strings.Split(s, "\n") {
		t := strings.TrimSpace(line)
		var ev goEvent
		if !strings.HasPrefix(t, "{") || json.Unmarshal([]byte(t), &ev) != nil || ev.Action == "" {
			if t != "" {
				stray = append(stray, line)
			}
			continue
		}
		switch ev.Action {
		case "build-output":
			pkg := strings.Fields(ev.ImportPath + " ")[0]
			if _, ok := buildOut[pkg]; !ok {
				buildOrder = append(buildOrder, pkg)
			}
			buildOut[pkg] = append(buildOut[pkg], strings.TrimSuffix(ev.Output, "\n"))
		case "start":
			started[ev.Package] = true
		case "output":
			k := key{ev.Package, ev.Test}
			if _, ok := outputs[k]; !ok {
				order = append(order, k)
			}
			outputs[k] = append(outputs[k], strings.TrimSuffix(ev.Output, "\n"))
			if ev.Package != "" && !terminal[ev.Package] {
				started[ev.Package] = true
			}
		case "fail":
			if ev.Test != "" {
				failed[key{ev.Package, ev.Test}] = true
				continue
			}
			terminal[ev.Package] = true
			st := StatusFail
			if ev.FailedBuild != "" {
				st = StatusBuildFailed
			}
			r.Packages = append(r.Packages, PackageResult{Package: ev.Package, Status: st})
		case "pass", "skip":
			if ev.Test != "" {
				continue
			}
			terminal[ev.Package] = true
			st := StatusOK
			if ev.Action == "skip" {
				st = StatusSkip
				for _, o := range outputs[key{ev.Package, ""}] {
					if strings.Contains(o, "[no test files]") {
						st = StatusNoTests
					}
				}
			}
			r.Packages = append(r.Packages, PackageResult{Package: ev.Package, Status: st})
		}
	}
	for _, pkg := range buildOrder {
		sc := newGoLineScanner(&r)
		sc.buildPkg = pkg
		for _, l := range buildOut[pkg] {
			if goBuildHdrRe.MatchString(l) {
				continue // the header names the test binary; the event names the package
			}
			sc.line(l, false)
			sc.buildPkg = pkg
		}
	}
	if len(stray) > 0 {
		sc := newGoLineScanner(&r)
		for _, l := range stray {
			sc.line(l, false)
		}
	}
	// Each test's own output: its log frames and any panic it printed.
	for _, k := range order {
		if k.test == "" {
			sc := newGoLineScanner(&r)
			for _, l := range outputs[k] {
				sc.line(l, false)
			}
			for _, p := range sc.pendPanic {
				p.Package = k.pkg
				r.Panics = append(r.Panics, p)
			}
			continue
		}
		if !failed[k] {
			continue
		}
		sc := newGoLineScanner(&r)
		sc.current = k.test
		for _, l := range outputs[k] {
			if goFailRe.MatchString(l) || goEventRe.MatchString(l) {
				continue // the event, not the text, says which test
			}
			sc.line(l, false)
			if sc.inPanic == nil && len(sc.pendPanic) == 0 {
				sc.current = k.test
			}
		}
		f := Failure{Package: k.pkg, Test: k.test, Frames: appendFrames(nil, sc.logs[k.test]...)}
		for _, p := range sc.pendPanic {
			p.Package, p.Test = k.pkg, k.test
			r.Panics = append(r.Panics, p)
			f.Frames = appendFrames(f.Frames, p.Frames...)
		}
		r.Failures = append(r.Failures, f)
	}
	for k := range failed {
		if _, ok := outputs[k]; !ok {
			r.Failures = append(r.Failures, Failure{Package: k.pkg, Test: k.test})
		}
	}
	r.Complete = len(r.Packages) > 0
	for pkg := range started {
		if !terminal[pkg] {
			r.Complete = false
		}
	}
	return r
}

// ─── JUnit XML ──────────────────────────────────────────────────────────

// junitStart is the offset of a JUnit document in s, or -1.
func junitStart(s string) int {
	i := strings.Index(s, "<testsuite")
	if i < 0 {
		return -1
	}
	if j := strings.LastIndex(s[:i], "<?xml"); j >= 0 {
		return j
	}
	return i
}

type junitFail struct {
	Message string `xml:"message,attr"`
	Text    string `xml:",chardata"`
}

type junitCase struct {
	Classname string      `xml:"classname,attr"`
	Name      string      `xml:"name,attr"`
	File      string      `xml:"file,attr"`
	Line      int         `xml:"line,attr"`
	Failures  []junitFail `xml:"failure"`
	Errors    []junitFail `xml:"error"`
	Skipped   *struct{}   `xml:"skipped"`
}

type junitSuite struct {
	Name   string       `xml:"name,attr"`
	Cases  []junitCase  `xml:"testcase"`
	Suites []junitSuite `xml:"testsuite"`
}

type junitRoot struct {
	XMLName  xml.Name
	Tests    int          `xml:"tests,attr"`
	Failures int          `xml:"failures,attr"`
	Errors   int          `xml:"errors,attr"`
	Skipped  int          `xml:"skipped,attr"`
	Name     string       `xml:"name,attr"`
	Cases    []junitCase  `xml:"testcase"`
	Suites   []junitSuite `xml:"testsuite"`
}

var junitFrameRe = regexp.MustCompile(`([A-Za-z0-9_.\-/]+\.[A-Za-z0-9]+):(\d+)`)

func parseJUnit(s string) Run {
	r := Run{Runner: RunnerJUnit}
	var root junitRoot
	dec := xml.NewDecoder(bytes.NewReader([]byte(s)))
	dec.Strict = false
	if err := dec.Decode(&root); err != nil {
		return r
	}
	sum := Summary{}
	var walk func(suite string, cases []junitCase, suites []junitSuite)
	walk = func(suite string, cases []junitCase, suites []junitSuite) {
		if suite != "" && len(cases) > 0 {
			st := StatusOK
			for _, c := range cases {
				if len(c.Failures)+len(c.Errors) > 0 {
					st = StatusFail
				}
			}
			r.Packages = append(r.Packages, PackageResult{Package: suite, Status: st})
		}
		for _, c := range cases {
			sum.Tests++
			if c.Skipped != nil {
				sum.Skipped++
			}
			bad := append(append([]junitFail{}, c.Failures...), c.Errors...)
			sum.Failures += len(c.Failures)
			sum.Errors += len(c.Errors)
			if len(bad) == 0 {
				continue
			}
			pkg := c.Classname
			if pkg == "" {
				pkg = suite
			}
			f := Failure{Package: pkg, Test: c.Name}
			if c.File != "" && c.Line > 0 {
				f.Frames = appendFrames(f.Frames, Frame{File: c.File, Line: c.Line})
			}
			for _, b := range bad {
				for _, m := range junitFrameRe.FindAllStringSubmatch(b.Message+"\n"+b.Text, -1) {
					ln, _ := strconv.Atoi(m[2])
					f.Frames = appendFrames(f.Frames, Frame{File: m[1], Line: ln})
				}
			}
			r.Failures = append(r.Failures, f)
		}
		for _, sub := range suites {
			walk(sub.Name, sub.Cases, sub.Suites)
		}
	}
	walk(root.Name, root.Cases, root.Suites)
	// The root's own counters are the runner's summary when it wrote them.
	if root.Tests > 0 || root.Failures > 0 || root.Errors > 0 {
		sum.Tests, sum.Failures, sum.Errors = root.Tests, root.Failures, root.Errors
		if root.Skipped > 0 {
			sum.Skipped = root.Skipped
		}
	}
	r.Summary = &sum
	r.Complete = true
	return r
}
