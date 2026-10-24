package gatereceipt

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// FlakyTest attributes each failing gate's recorded duration to its named test.
// Costs can overlap when several tests failed in the same gate.
type FlakyTest struct {
	Package    string        `json:"package"`
	Test       string        `json:"test"`
	FailedRuns int           `json:"failed_runs"`
	Lost       time.Duration `json:"lost_ns"`
	Trees      []string      `json:"trees"`
}

// FlakyReport keeps unbound runs separate from same-tree evidence.
type FlakyReport struct {
	Tests           []FlakyTest `json:"tests"`
	UnknownIdentity int         `json:"unknown_identity"`
}

type flakyCase struct{ pkg, test string }

// The tree pins Taskfile.yml and lefthook.yml; Task's stderr echoes are not
// present in saved gate logs and may stop early when a step fails.
type flakyScope struct {
	tree, gate string
	environment
}
type flakyRun struct {
	flakyScope
	header     bool
	exit       int
	start, end time.Time
	cases      map[flakyCase]bool // true is a failure.
	passed     map[string]bool    // packages whose results are all uncached passes.
}

var flakyTree = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
var flakyVerdict = regexp.MustCompile(`^\s*--- (PASS|FAIL): (\S+) \(`)
var flakyPackage = regexp.MustCompile(`^(?:ok|FAIL)\s+(\S+)(?:\s|$)`)
var flakyReceiptName = regexp.MustCompile(`^[0-9a-f]{64}\.json$`)
var flakySavedName = regexp.MustCompile(`^[0-9a-f]{16}\.log$`)

// FlakyDirRole selects the evidence filenames accepted from a directory.
type FlakyDirRole string

const (
	FlakyReceipts  FlakyDirRole = "receipts"
	FlakyGateLogs  FlakyDirRole = "gate-logs"
	FlakySavedRuns FlakyDirRole = "saved-runs"
)

// FlakyDir identifies an evidence directory independently of its path spelling.
type FlakyDir struct {
	Path string
	Role FlakyDirRole
}

func flakyEvidenceName(role FlakyDirRole, name string, entries map[string]bool) bool {
	switch role {
	case FlakyGateLogs:
		return strings.HasPrefix(name, "gate.")
	case FlakySavedRuns:
		return flakySavedName.MatchString(name)
	case FlakyReceipts:
		return flakyReceiptName.MatchString(name) || (strings.HasSuffix(name, ".log") && entries[strings.TrimSuffix(name, ".log")+".json"] && flakyReceiptName.MatchString(strings.TrimSuffix(name, ".log")+".json"))
	default:
		return false
	}
}

// FlakyTests reads existing gate logs, saved hook output and adjacent receipts.
// dirs are explicit evidence directories; absent directories contain no evidence.
// since is an injected end-time cutoff (zero means all history). No git process,
// store, clock, daemon or network is needed to inspect imported evidence.
func FlakyTests(dirs []FlakyDir, since time.Time) (FlakyReport, error) {
	report := FlakyReport{Tests: []FlakyTest{}}
	files := map[string]bool{}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir.Path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return report, fmt.Errorf("read evidence: %w", err)
		}
		names := map[string]bool{}
		for _, entry := range entries {
			if !entry.IsDir() {
				names[entry.Name()] = true
			}
		}
		for _, entry := range entries {
			name := entry.Name()
			if !entry.IsDir() && flakyEvidenceName(dir.Role, name, names) {
				path, err := filepath.Abs(filepath.Join(dir.Path, name))
				if err != nil {
					return report, err
				}
				files[path] = true
			}
		}
	}
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	runs := map[string]flakyRun{}
	for _, path := range paths {
		// A companion receipt owns its log even when the receipt is unreadable:
		// do not bypass corrupt imported evidence by reinterpreting its log alone.
		if !strings.HasSuffix(path, ".json") && files[strings.TrimSuffix(path, ".log")+".json"] {
			continue
		}
		run, key := readFlakyRun(path)
		if !run.header {
			continue
		}
		if !since.IsZero() && (run.end.IsZero() || run.end.Before(since)) {
			continue
		}
		old, exists := runs[key]
		if !exists || (old.tree == "" && run.tree != "") {
			runs[key] = run
		}
	}
	ordered := make([]flakyRun, 0, len(runs))
	for _, run := range runs {
		if run.tree == "" {
			report.UnknownIdentity++
			continue
		}
		ordered = append(ordered, run)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if !ordered[i].end.Equal(ordered[j].end) {
			return ordered[i].end.Before(ordered[j].end)
		}
		if !ordered[i].start.Equal(ordered[j].start) {
			return ordered[i].start.Before(ordered[j].start)
		}
		return ordered[i].tree < ordered[j].tree
	})
	type treeCase struct {
		flakyScope
		flakyCase
	}
	pending := map[treeCase][]flakyRun{}
	rows := map[flakyCase]*FlakyTest{}
	trees := map[flakyCase]map[string]bool{}
	for _, run := range ordered {
		// Plain go test does not print individual PASS lines. An ok package
		// discharges only earlier named failures with the same clean tree,
		// gate task and complete toolchain/environment header.
		// It never invents a test name or bridges incomplete scope metadata.
		for key := range pending {
			if key.flakyScope == run.flakyScope && run.exit == 0 && run.passed[key.pkg] && !run.cases[key.flakyCase] {
				run.cases[key.flakyCase] = false
			}
		}
		for c, failed := range run.cases {
			key := treeCase{run.flakyScope, c}
			if failed {
				pending[key] = append(pending[key], run)
				continue
			}
			remaining := pending[key][:0]
			for _, fail := range pending[key] {
				// Overlapping gates cannot establish fail-then-pass ordering.
				if !fail.end.Before(run.start) {
					remaining = append(remaining, fail)
					continue
				}
				row := rows[c]
				if row == nil {
					row = &FlakyTest{Package: c.pkg, Test: c.test}
					rows[c] = row
					trees[c] = map[string]bool{}
				}
				row.FailedRuns++
				row.Lost += fail.end.Sub(fail.start)
				trees[c][run.tree] = true
			}
			pending[key] = remaining
		}
	}
	for c, row := range rows {
		for tree := range trees[c] {
			row.Trees = append(row.Trees, tree)
		}
		sort.Strings(row.Trees)
		report.Tests = append(report.Tests, *row)
	}
	sort.Slice(report.Tests, func(i, j int) bool {
		a, b := report.Tests[i], report.Tests[j]
		if a.Lost != b.Lost {
			return a.Lost > b.Lost
		}
		if a.Package != b.Package {
			return a.Package < b.Package
		}
		return a.Test < b.Test
	})
	return report, nil
}

func readFlakyRun(path string) (flakyRun, string) {
	run := flakyRun{}
	key := path
	read := func(p string) ([]byte, error) {
		info, err := os.Lstat(p)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("not a regular evidence file")
		}
		return os.ReadFile(p)
	}
	var receipt *Receipt
	valid := true
	if strings.HasSuffix(path, ".json") {
		data, err := read(path)
		var r Receipt
		if err != nil || json.Unmarshal(data, &r) != nil {
			valid = false
		} else {
			receipt = &r
			run.end, _ = time.Parse(time.RFC3339Nano, r.End)
			valid = r.Version == 1 && !r.Dirty && flakyTree.MatchString(r.Tree) && digest(data) == strings.TrimSuffix(filepath.Base(path), ".json")
		}
		path = strings.TrimSuffix(path, ".json") + ".log"
	}
	b, err := read(path)
	if err != nil {
		return run, key
	}
	key = digest(b)
	run.cases, run.passed = flakyOutcomes(string(b))
	lines := strings.Split(strings.TrimRight(string(b), "\r\n"), "\n")
	fields := strings.Fields(lines[0])
	if len(fields) < 4 || fields[0] != "fathom" || fields[1] != "gate:" || fields[3] != "START" {
		return run, key
	}
	run.header = true
	// logResult expects a nonempty timestamp after START. Validate before
	// delegating so malformed imported logs remain unknown rather than panic.
	if len(fields) < 5 {
		return run, key
	}
	if _, err := time.Parse(time.RFC3339Nano, fields[4]); err != nil {
		return run, key
	}
	command := fields[2]
	exit, start, end, source, err := logResult(b, command, time.Time{})
	if err != nil || source != "log" {
		return run, key
	}
	run.start, err = time.Parse(time.RFC3339Nano, start)
	if err != nil {
		return run, key
	}
	run.end, err = time.Parse(time.RFC3339Nano, end)
	if err != nil {
		return run, key
	}
	header := endpointLine.FindStringSubmatch(strings.TrimSuffix(lines[0], "\r"))
	if header == nil || !flakyTree.MatchString(header[3]) {
		return run, key
	}
	commit, err := logIdentity(b, command, header[3])
	if err != nil {
		return run, key
	}
	if receipt != nil {
		r := receipt
		valid = valid && r.Tree == header[3] && r.Commit == commit && r.Command == command && r.Exit == exit && r.LogBytes == int64(len(b)) && r.LogSHA256 == digest(b) && r.TimeSource == source && r.Start == start && r.End == end
	}
	if valid {
		env, err := logEnvironment(b)
		if err == nil {
			run.tree, run.gate, run.environment, run.exit = header[3], command, env, exit
		}
	}
	return run, key
}

// Go's verbose output names a test at --- PASS/FAIL and its package at the
// following package summary. JSON events name both directly. Package ok
// results only resolve matching earlier failures when every fresh result passed.
// A cached result contributes no cases or package pass: Go replays old output.
// Other steps' fresh results for that package remain evidence in the same run.
func flakyOutcomes(output string) (map[flakyCase]bool, map[string]bool) {
	cases := map[flakyCase]bool{}
	passed := map[string]bool{}
	pending := map[string]bool{}
	type jsonPackage struct {
		cases  map[string]bool
		cached bool
	}
	jsonPending := map[string]*jsonPackage{}
	add := func(c flakyCase, failed bool) { cases[c] = cases[c] || failed }
	result := func(pkg string, ok bool) {
		previous, seen := passed[pkg]
		passed[pkg] = ok && (!seen || previous)
	}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSuffix(line, "\r")
		var ev struct{ Action, Package, Test, Output string }
		if strings.HasPrefix(line, "{") && json.Unmarshal([]byte(line), &ev) == nil && ev.Package != "" {
			pkg := jsonPending[ev.Package]
			if pkg == nil {
				pkg = &jsonPackage{cases: map[string]bool{}}
				jsonPending[ev.Package] = pkg
			}
			for _, outputLine := range strings.Split(ev.Output, "\n") {
				m := flakyPackage.FindStringSubmatch(strings.TrimSuffix(outputLine, "\r"))
				if m != nil && m[1] == ev.Package && strings.Contains(outputLine, "(cached)") {
					pkg.cached = true
				}
			}
			if ev.Action == "pass" || ev.Action == "fail" {
				if ev.Test != "" {
					pkg.cases[ev.Test] = pkg.cases[ev.Test] || ev.Action == "fail"
				} else {
					if !pkg.cached {
						for name, failed := range pkg.cases {
							add(flakyCase{ev.Package, name}, failed)
						}
						result(ev.Package, ev.Action == "pass")
					}
					delete(jsonPending, ev.Package)
				}
			}
			continue
		}
		if m := flakyVerdict.FindStringSubmatch(line); m != nil {
			pending[m[2]] = pending[m[2]] || m[1] == "FAIL"
			continue
		}
		if m := flakyPackage.FindStringSubmatch(line); m != nil {
			if !strings.Contains(line, "(cached)") {
				result(m[1], strings.HasPrefix(line, "ok"))
				for name, failed := range pending {
					add(flakyCase{m[1], name}, failed)
				}
			}
			clear(pending)
		}
	}
	return cases, passed
}
