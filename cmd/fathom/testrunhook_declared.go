package main

// testrunhook_declared.go — TR-1 (docs/plans/test-run-hook.md, design
// docs/design/test-run-hook.md section 3): the repository's DECLARED test
// commands, and whether a Bash command invokes one. The test-run hook acts
// only on a match, and a Bash call that does not match exits before any
// daemon call, so this file is local work only: a few small file reads and
// a string scan, no subprocess, no network.
//
// Declared means written down by the repository, never guessed from its
// languages: a Taskfile task, a Makefile target or a package.json script
// that is test-named or runs a test runner (directly, through a nested
// task, or through a dependency), the runner such a task invokes (so
// `go test ./one/pkg -run X` matches a Taskfile whose test task runs
// `go test ./...`), and every `[tests] commands` entry in
// .fathom/config.toml.

import (
	"encoding/json"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/geoffmilleraz/fathom/internal/shellcmd"
)

// declaredTestCommand is one invocation the repository declares as a test
// command: the leading words a Bash command segment must start with, and
// where the declaration was read.
type declaredTestCommand struct {
	Invocation string
	Source     string
}

// testCommandMatch is a Bash command's match against the declared test
// commands. Piped reports that the matched segment's output went into a
// pipe (`| tail -30`), so the hook sees a filtered view of the run and the
// shell's exit status is the filter's, never the runner's (TR-V1).
type testCommandMatch struct {
	Invocation string
	Source     string
	Piped      bool
}

// declaredFileCap bounds each declaration file read (a hook reads no file
// whole without a cap).
const declaredFileCap = 1 << 20

// matchDeclaredTestCommand reports whether command runs one of root's
// declared test commands (configured is `[tests] commands`). The first
// matching segment of the command wins, and within a segment the first
// declaration in declaration order ([tests] commands, then the Taskfile,
// the Makefile and package.json, each in name order), so the answer is
// deterministic.
func matchDeclaredTestCommand(root, command string, configured []string) (testCommandMatch, bool) {
	if strings.TrimSpace(command) == "" {
		return testCommandMatch{}, false
	}
	return matchDeclaredIn(declaredTestCommands(root, configured), command)
}

// matchDeclaredIn is matchDeclaredTestCommand over a declaration list
// already read (readout replay reads each root's once).
func matchDeclaredIn(declared []declaredTestCommand, command string) (testCommandMatch, bool) {
	if len(declared) == 0 || strings.TrimSpace(command) == "" {
		return testCommandMatch{}, false
	}
	segs, unreadable := lexShellSegments(command)
	if unreadable {
		return testCommandMatch{}, false // RO-4: a command the lexer cannot read matches nothing
	}
	for _, seg := range segs {
		words := stripCommandWrappers(seg.words)
		if len(words) == 0 {
			continue
		}
		for _, d := range declared {
			if invocationMatches(words, strings.Fields(d.Invocation)) {
				return testCommandMatch{Invocation: d.Invocation, Source: d.Source, Piped: seg.pipedOut}, true
			}
		}
	}
	return testCommandMatch{}, false
}

// declaredTestCommands reads every declaration under root, deduplicated by
// invocation (the first source wins), in declaration order.
func declaredTestCommands(root string, configured []string) []declaredTestCommand {
	var out []declaredTestCommand
	seen := map[string]bool{}
	add := func(inv, src string) {
		inv = strings.Join(strings.Fields(inv), " ")
		if inv == "" || seen[inv] {
			return
		}
		seen[inv] = true
		out = append(out, declaredTestCommand{Invocation: inv, Source: src})
	}
	for _, c := range configured {
		add(strings.TrimPrefix(strings.TrimSpace(c), "./"), ".fathom/config.toml [tests] commands")
	}
	for _, d := range taskfileTestCommands(root) {
		add(d.Invocation, d.Source)
	}
	for _, d := range makefileTestCommands(root) {
		add(d.Invocation, d.Source)
	}
	for _, d := range packageJSONTestCommands(root) {
		add(d.Invocation, d.Source)
	}
	return out
}

// readDeclFile returns the first existing file among names under root,
// read up to declaredFileCap, and its name.
func readDeclFile(root string, names ...string) (string, []byte, bool) {
	for _, n := range names {
		f, err := os.Open(filepath.Join(root, n))
		if err != nil {
			continue
		}
		b, err := io.ReadAll(io.LimitReader(f, declaredFileCap))
		f.Close()
		if err != nil {
			continue
		}
		return n, b, true
	}
	return "", nil, false
}

// ─── the three declaration files ────────────────────────────────────────

// taskfileTestCommands reads a go-task Taskfile: "task <name>" for every
// test task, then the runners those tasks invoke.
func taskfileTestCommands(root string) []declaredTestCommand {
	name, raw, ok := readDeclFile(root, "Taskfile.yml", "Taskfile.yaml", "taskfile.yml", "taskfile.yaml", "Taskfile.dist.yml")
	if !ok {
		return nil
	}
	var doc struct {
		Vars  map[string]any `yaml:"vars"`
		Tasks map[string]any `yaml:"tasks"`
	}
	if yaml.Unmarshal(raw, &doc) != nil || len(doc.Tasks) == 0 {
		return nil
	}
	vars := map[string]string{}
	for k, v := range doc.Vars {
		if s, ok := v.(string); ok {
			vars[k] = s
		}
	}
	bodies := map[string][]string{} // task -> its command lines
	refs := map[string][]string{}   // task -> tasks it runs (cmds task:, deps)
	for tname, tv := range doc.Tasks {
		cmds, sub := taskfileTaskShape(tv)
		for i, c := range cmds {
			cmds[i] = expandTaskVars(c, vars)
		}
		bodies[tname] = cmds
		refs[tname] = sub
	}
	// A `task <name>` nested inside a command line is a reference too.
	for tname, cmds := range bodies {
		for _, c := range cmds {
			segs, unreadable := lexShellSegments(c)
			if unreadable {
				continue // RO-4: an unreadable line names no task reference
			}
			for _, seg := range segs {
				for i := 0; i+1 < len(seg.words); i++ {
					if path.Base(seg.words[i]) == "task" {
						if _, ok := bodies[seg.words[i+1]]; ok {
							refs[tname] = append(refs[tname], seg.words[i+1])
						}
					}
				}
			}
		}
	}
	return declaredFromGraph(name, "task ", bodies, refs)
}

// taskfileTaskShape returns a Taskfile task's command lines and the tasks
// it references, across the forms Taskfile v3 accepts: a string, a list of
// commands, or a map with cmds and deps (each item a string or a map with
// cmd or task).
func taskfileTaskShape(v any) (cmds, refs []string) {
	item := func(x any, isDep bool) {
		switch t := x.(type) {
		case string:
			if isDep {
				refs = append(refs, t)
			} else {
				cmds = append(cmds, t)
			}
		case map[string]any:
			if s, ok := t["cmd"].(string); ok {
				cmds = append(cmds, s)
			}
			if s, ok := t["task"].(string); ok {
				refs = append(refs, s)
			}
		}
	}
	switch t := v.(type) {
	case string:
		cmds = append(cmds, t)
	case []any:
		for _, x := range t {
			item(x, false)
		}
	case map[string]any:
		switch c := t["cmds"].(type) {
		case []any:
			for _, x := range c {
				item(x, false)
			}
		case string:
			cmds = append(cmds, c)
		}
		if s, ok := t["cmd"].(string); ok {
			cmds = append(cmds, s)
		}
		if d, ok := t["deps"].([]any); ok {
			for _, x := range d {
				item(x, true)
			}
		}
	}
	return cmds, refs
}

var taskVarRe = regexp.MustCompile(`\{\{\s*\.([A-Za-z_][A-Za-z0-9_]*)\s*\}\}`)

// expandTaskVars returns line with each {{.NAME}} replaced by the Taskfile
// var's value, then (as a second line joined by a newline, so runner
// detection sees both) by the lowercased name: a variable named after a
// runner (GO, CARGO, PYTEST) stands for that runner even when its value is
// a wrapper script.
func expandTaskVars(line string, vars map[string]string) string {
	if !taskVarRe.MatchString(line) {
		return line
	}
	byValue := taskVarRe.ReplaceAllStringFunc(line, func(m string) string {
		name := taskVarRe.FindStringSubmatch(m)[1]
		if v, ok := vars[name]; ok {
			return v
		}
		return strings.ToLower(name)
	})
	byName := taskVarRe.ReplaceAllStringFunc(line, func(m string) string {
		return strings.ToLower(taskVarRe.FindStringSubmatch(m)[1])
	})
	return byValue + "\n" + byName
}

var (
	makeVarRe    = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\s*(?:\?=|:=|::=|\+=|=)\s*(.*)$`)
	makeTargetRe = regexp.MustCompile(`^([A-Za-z0-9_./-]+(?:\s+[A-Za-z0-9_./-]+)*)\s*:([^=].*|)$`)
	makeRefRe    = regexp.MustCompile(`\$[({]([A-Za-z_][A-Za-z0-9_]*)[)}]`)
)

// makefileTestCommands reads a Makefile: "make <target>" for every test
// target (test-named, runs a runner, depends on or recursively makes a test
// target), then the runners those targets invoke.
func makefileTestCommands(root string) []declaredTestCommand {
	name, raw, ok := readDeclFile(root, "GNUmakefile", "makefile", "Makefile")
	if !ok {
		return nil
	}
	vars := map[string]string{}
	bodies := map[string][]string{}
	refs := map[string][]string{}
	var current []string
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "\t") {
			for _, t := range current {
				bodies[t] = append(bodies[t], strings.TrimSpace(line))
			}
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if m := makeVarRe.FindStringSubmatch(trimmed); m != nil {
			vars[m[1]] = strings.TrimSpace(m[2])
			current = nil
			continue
		}
		if m := makeTargetRe.FindStringSubmatch(trimmed); m != nil {
			current = nil
			prereqs := strings.Fields(strings.SplitN(m[2], ";", 2)[0])
			for _, t := range strings.Fields(m[1]) {
				if strings.HasPrefix(t, ".") {
					continue // .PHONY and other special targets
				}
				current = append(current, t)
				if _, ok := bodies[t]; !ok {
					bodies[t] = nil
				}
				refs[t] = append(refs[t], prereqs...)
			}
			continue
		}
		current = nil
	}
	for t, lines := range bodies {
		for i, l := range lines {
			l = makeRefRe.ReplaceAllStringFunc(l, func(m string) string {
				v := makeRefRe.FindStringSubmatch(m)[1]
				if v == "MAKE" {
					return "make"
				}
				if val, ok := vars[v]; ok {
					return val
				}
				return strings.ToLower(v)
			})
			lines[i] = l
			segs, unreadable := lexShellSegments(l)
			if unreadable {
				continue // RO-4: an unreadable recipe line names no make reference
			}
			for _, seg := range segs {
				for j := 0; j+1 < len(seg.words); j++ {
					if path.Base(seg.words[j]) == "make" {
						refs[t] = append(refs[t], seg.words[j+1])
					}
				}
			}
		}
	}
	return declaredFromGraph(name, "make ", bodies, refs)
}

// packageJSONTestCommands reads package.json's scripts: the package-manager
// invocations of every test script, then the runners those scripts call.
func packageJSONTestCommands(root string) []declaredTestCommand {
	name, raw, ok := readDeclFile(root, "package.json")
	if !ok {
		return nil
	}
	var doc struct {
		Scripts map[string]string `json:"scripts"`
	}
	if json.Unmarshal(raw, &doc) != nil || len(doc.Scripts) == 0 {
		return nil
	}
	bodies := map[string][]string{}
	refs := map[string][]string{}
	for s, body := range doc.Scripts {
		bodies[s] = []string{body}
		segs, unreadable := lexShellSegments(body)
		if unreadable {
			continue // RO-4: an unreadable script body names no script reference
		}
		for _, seg := range segs {
			w := stripCommandWrappers(seg.words)
			if len(w) >= 3 && isPackageManager(w[0]) && w[1] == "run" {
				refs[s] = append(refs[s], w[2])
			}
		}
	}
	tests := testNodes(bodies, refs)
	var out []declaredTestCommand
	for _, s := range tests {
		src := name + " scripts." + s
		if s == "test" {
			out = append(out, declaredTestCommand{"npm test", src}, declaredTestCommand{"yarn test", src}, declaredTestCommand{"pnpm test", src})
		}
		for _, pm := range []string{"npm run ", "yarn run ", "pnpm run ", "bun run "} {
			out = append(out, declaredTestCommand{pm + s, src})
		}
		out = append(out, declaredTestCommand{"yarn " + s, src}, declaredTestCommand{"pnpm " + s, src})
	}
	return append(out, runnerCommands(name+" scripts.", tests, bodies)...)
}

func isPackageManager(w string) bool {
	switch w {
	case "npm", "yarn", "pnpm", "bun":
		return true
	}
	return false
}

// declaredFromGraph turns one file's named bodies and references into
// declarations: prefix+name for every test node, then the runners.
func declaredFromGraph(file, prefix string, bodies, refs map[string][]string) []declaredTestCommand {
	tests := testNodes(bodies, refs)
	out := make([]declaredTestCommand, 0, len(tests))
	for _, t := range tests {
		out = append(out, declaredTestCommand{Invocation: prefix + t, Source: file + ": " + prefix + t})
	}
	return append(out, runnerCommands(file+": "+prefix, tests, bodies)...)
}

// testNodes returns, sorted, every name that is test-named, whose body
// invokes a test runner, or that references a test node (to a fixpoint).
func testNodes(bodies, refs map[string][]string) []string {
	is := map[string]bool{}
	for n, lines := range bodies {
		if testNamed(n) || len(runnersIn(lines)) > 0 {
			is[n] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for n := range bodies {
			if is[n] {
				continue
			}
			for _, r := range refs[n] {
				if is[r] {
					is[n], changed = true, true
					break
				}
			}
		}
	}
	out := make([]string, 0, len(is))
	for n := range is {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// runnerCommands declares every test runner the test nodes' own bodies
// invoke, sourced to the first node (in name order) that invokes it.
func runnerCommands(srcPrefix string, tests []string, bodies map[string][]string) []declaredTestCommand {
	var out []declaredTestCommand
	seen := map[string]bool{}
	for _, t := range tests {
		for _, r := range runnersIn(bodies[t]) {
			if !seen[r] {
				seen[r] = true
				out = append(out, declaredTestCommand{Invocation: r, Source: srcPrefix + t + " runs " + r})
			}
		}
	}
	return out
}

// testNamed reports a test-named task, target or script: a name part
// (split on anything not a letter or digit) that is or starts with "test".
func testNamed(name string) bool {
	for _, p := range strings.FieldsFunc(strings.ToLower(name), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	}) {
		if strings.HasPrefix(p, "test") {
			return true
		}
	}
	return false
}

// testRunners are the runner invocations a declaration body is scanned
// for, by their leading words (the first word compared by base name).
var testRunners = [][]string{
	{"go", "test"}, {"gotestsum"}, {"cargo", "test"}, {"cargo", "nextest"},
	{"pytest"}, {"python", "-m", "pytest"}, {"python3", "-m", "pytest"},
	{"jest"}, {"vitest"}, {"mocha"}, {"playwright", "test"},
	{"bun", "test"}, {"deno", "test"}, {"dotnet", "test"}, {"mvn", "test"},
	{"gradle", "test"}, {"gradlew", "test"}, {"swift", "test"}, {"mix", "test"},
	{"rspec"}, {"phpunit"}, {"ctest"},
}

// runnersIn returns the runner invocations lines contain, in first-seen
// order.
func runnersIn(lines []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, l := range lines {
		segs, unreadable := lexShellSegments(l)
		if unreadable {
			continue // RO-4: an unreadable line names no runner
		}
		for _, seg := range segs {
			w := stripCommandWrappers(seg.words)
			for _, r := range testRunners {
				if invocationMatches(w, r) {
					inv := strings.Join(r, " ")
					if !seen[inv] {
						seen[inv] = true
						out = append(out, inv)
					}
				}
			}
		}
	}
	return out
}

// invocationMatches reports whether words start with want, comparing the
// first word by base name (and without a leading "./") so
// /usr/local/go/bin/go and ./bin/itest match "go" and "bin/itest".
func invocationMatches(words, want []string) bool {
	if len(want) == 0 || len(words) < len(want) {
		return false
	}
	first := strings.TrimPrefix(words[0], "./")
	if first != want[0] && !(strings.Contains(first, "/") && !strings.Contains(want[0], "/") && path.Base(first) == want[0]) {
		return false
	}
	for i := 1; i < len(want); i++ {
		if words[i] != want[i] {
			return false
		}
	}
	return true
}

// ─── a small shell splitter ─────────────────────────────────────────────

// shellSeg is one simple command of a shell line: its words, and whether
// its output went into a pipe.
type shellSeg struct {
	words    []string
	pipedOut bool
}

// lexShellSegments reads a shell line with the one command lexer
// (shellcmd.ParseCommand) and returns its simple commands in source order:
// each unit's argv (env prefixes and wrappers peeled, redirects kept out)
// and whether its output went into a pipe. Units nested in a subshell,
// group, compound or `bash -c` body are flattened in. Quoted text stays one
// word, so a runner named inside an argument (an echo, a commit message)
// never reads as an invocation. unreadable is true when the lexer could not
// read the line (RO-4); segs is then nil and each caller decides what that
// means for itself (the test-run matcher matches nothing).
func lexShellSegments(line string) (segs []shellSeg, unreadable bool) {
	cmd := shellcmd.ParseCommand(line)
	if cmd.Unreadable {
		return nil, true
	}
	collectShellSegs(&cmd, &segs)
	return segs, false
}

func collectShellSegs(c *shellcmd.Command, out *[]shellSeg) {
	if c == nil {
		return
	}
	for _, p := range c.Pipelines {
		for i := range p.Units {
			u := &p.Units[i]
			if u.Kind == shellcmd.KindSimple && len(u.Words) > 0 {
				*out = append(*out, shellSeg{words: u.Args(), pipedOut: i < len(p.Units)-1})
			}
			collectShellSegs(u.Body, out)
		}
	}
}

var envAssignRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

// stripCommandWrappers drops the leading words that only set up a command
// — environment assignments and env, time, command, exec, nohup, nice,
// timeout, npx/pnpx/bunx and `pnpm exec` — so the runner itself leads.
func stripCommandWrappers(words []string) []string {
	for len(words) > 0 {
		w := words[0]
		switch {
		case envAssignRe.MatchString(w):
			words = words[1:]
		case w == "env":
			words = words[1:]
			for len(words) > 0 && strings.HasPrefix(words[0], "-") {
				words = words[1:]
			}
		case w == "time" || w == "command" || w == "exec" || w == "nohup" || w == "npx" || w == "pnpx" || w == "bunx":
			words = words[1:]
			for len(words) > 0 && strings.HasPrefix(words[0], "-") && !strings.Contains(words[0], "=") {
				words = words[1:]
			}
		case w == "nice":
			words = words[1:]
			if len(words) > 1 && words[0] == "-n" {
				words = words[2:]
			}
		case w == "timeout":
			words = words[1:]
			for len(words) > 0 && strings.HasPrefix(words[0], "-") {
				words = words[1:]
			}
			if len(words) > 0 {
				words = words[1:] // the duration
			}
		case (w == "pnpm" || w == "yarn") && len(words) > 1 && (words[1] == "exec" || words[1] == "dlx"):
			words = words[2:]
		default:
			return words
		}
	}
	return words
}
