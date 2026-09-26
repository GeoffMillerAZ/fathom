package custodian

// commands.go — the `commands_resolve` kind (CC-12's chain checks, vision
// 22 §7): every command a corpus invokes that points INTO this repo — a
// `task X` target, a repo script — still exists. One kind, four corpora,
// and the links of the chain the deterministic gate stands on:
//
//	docs (agent-config) → `task X` in a code span names a real target
//	lefthook            → a hook's `run:` calls real targets and scripts
//	Taskfile            → cmds / deps / `task:` calls and scripts resolve
//	CI                  → a pipeline's `run:` / `script:` does the same
//
// Precision first — a false violation teaches people to ignore the
// custodian — so commands are read only where the corpus puts commands:
// code spans and shell fences in markdown (never prose: "this task never
// ingests" is not `task never`; never a ```text or ```go block), and the
// command-bearing keys of the YAML
// corpora (never `desc:`, never comments). Quoted strings are blanked
// before a line is read, so `echo "no task the corpus runs"` names
// nothing. A ref this audit cannot judge (templated, behind a flag, into
// a remote include, outside the repo) is counted and named, never guessed
// either way.
//
// A command resolves from where it RUNS, not from where it is written: a
// Taskfile task's `dir:` and its include's (an include without one runs in
// its includer's directory), lefthook's `root:`, a GitHub step's
// `working-directory:` or its job's `defaults.run`. Inside an included
// Taskfile a `task:` / `deps:` call resolves in the include's namespace
// (taskfile.go). A markdown doc does not say where its reader stands, so
// a doc's command resolves from the repo root or from the doc's own
// directory — and a `task X` there against the Taskfile Task would find
// from either. A working directory this audit cannot model makes the ref
// not checkable, never dead and never resolved.

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// commandRef is one command a corpus file invokes that points into the
// repo.
type commandRef struct {
	File   string
	Line   int // 1-based
	Script bool
	Name   string // the task target, or the script path as written
	// Qualified is the target a Taskfile's own `task:` / `deps:` call
	// resolves to in the corpus's task graph — inside an include, the
	// include's namespace + Name (a leading ':' is the root's). Empty for a
	// shell `task X`, which a new task process resolves against the
	// Taskfile it finds from the directory it runs in.
	Qualified string
	// Dirs are the directories the command may run in, repo-relative ("."
	// is the root): one for a YAML corpus, the root and the doc's own
	// directory for markdown. DirUnknown says why the directory cannot be
	// modelled; a script or shell `task X` under it is not checkable.
	Dirs       []string
	DirUnknown string
}

// deadCommand is a commandRef that does not resolve, with the reason.
type deadCommand struct {
	commandRef
	Why string
}

func (d deadCommand) evidence() string {
	if d.Line == 0 {
		return fmt.Sprintf("%s → %s", d.File, d.Why) // a file-level link (a broken include)
	}
	if d.Script {
		return fmt.Sprintf("%s:%d → %s (%s)", d.File, d.Line, d.Name, d.Why)
	}
	return fmt.Sprintf("%s:%d → task %s (%s)", d.File, d.Line, d.Name, d.Why)
}

// commandSyntax says where a corpus's files carry commands: shellKeys hold
// shell text, nameKeys hold bare task names (the Taskfile's `task:` call
// and `deps:` list), and dirKeys set the working directory of the
// commands beside and beneath them.
type commandSyntax struct {
	markdown  bool
	shellKeys map[string]bool
	nameKeys  map[string]bool
	dirKeys   []dirKey
	// ghDefaults reads GitHub Actions' `defaults: {run: {working-directory}}`
	// at the workflow and job levels.
	ghDefaults bool
}

// dirKey is one key that sets a working directory, and how its value is
// read.
type dirKey struct {
	key  string
	rule dirRule
}

// dirRule is how a working-directory key's value is read.
type dirRule int

const (
	dirFromBase dirRule = iota // joined to the enclosing directory (a Taskfile task's `dir:`)
	dirFromRoot                // joined to the repo root (lefthook `root:`, GitHub `working-directory:`)
	dirOpaque                  // a directory this audit does not model (CircleCI, Azure)
)

func syntaxFor(where string) commandSyntax {
	switch where {
	case CorpusLefthook:
		return commandSyntax{shellKeys: set("run"), dirKeys: []dirKey{{"root", dirFromRoot}}}
	case CorpusTaskfile:
		return commandSyntax{shellKeys: set("cmd", "cmds"), nameKeys: set("task", "deps"), dirKeys: []dirKey{{"dir", dirFromBase}}}
	case CorpusCI:
		return commandSyntax{
			shellKeys:  set("run", "script", "before_script", "after_script", "command", "commands"),
			dirKeys:    []dirKey{{"working-directory", dirFromRoot}, {"working_directory", dirOpaque}, {"workingDirectory", dirOpaque}},
			ghDefaults: true,
		}
	default:
		return commandSyntax{markdown: true}
	}
}

func set(keys ...string) map[string]bool {
	m := make(map[string]bool, len(keys))
	for _, k := range keys {
		m[k] = true
	}
	return m
}

// extractCommands returns the in-repo command refs one corpus file makes,
// plus how many refs it saw but cannot judge. node places a Taskfile in its
// include (namespace and directory); the other corpora run from the root.
func extractCommands(where string, node taskNode, data []byte) ([]commandRef, int, error) {
	syn := syntaxFor(where)
	rel := node.file
	if syn.markdown {
		refs, unk := markdownCommands(rel, string(data))
		return refs, unk, nil
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, 0, fmt.Errorf("%s does not parse as YAML: %w", rel, err)
	}
	dir := node.dir
	if dir == "" && node.dirUnknown == "" {
		dir = "."
	}
	w := &yamlCommandWalker{rel: rel, syn: syn, lines: strings.Split(string(data), "\n"), prefix: node.prefix, dir: dir, dirUnknown: node.dirUnknown}
	w.walk(&doc)
	return w.refs, w.unknown, nil
}

type yamlCommandWalker struct {
	rel   string
	syn   commandSyntax
	lines []string // the file's source lines, to place a ref inside a folded scalar
	// prefix is a Taskfile node's namespace; dir / dirUnknown the working
	// directory of the commands being walked.
	prefix     string
	dir        string
	dirUnknown string
	refs       []commandRef
	unknown    int
}

func (w *yamlCommandWalker) walk(n *yaml.Node) {
	switch n.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, c := range n.Content {
			w.walk(c)
		}
	case yaml.MappingNode:
		defer w.enterDir(n)()
		for i := 0; i+1 < len(n.Content); i += 2 {
			key, val := n.Content[i].Value, n.Content[i+1]
			switch {
			case w.syn.shellKeys[key]:
				w.shell(val)
			case w.syn.nameKeys[key]:
				w.names(val)
			default:
				w.walk(val)
			}
		}
	}
}

// enterDir applies the working directory a mapping sets for the commands
// in and under it (its own dir key, else GitHub's `defaults.run`), and
// returns the restore.
func (w *yamlCommandWalker) enterDir(n *yaml.Node) func() {
	key, val, rule := "", "", dirRule(-1)
	for _, dk := range w.syn.dirKeys {
		if v := mappingValue(n, dk.key); v != nil && v.Kind == yaml.ScalarNode {
			key, val, rule = dk.key, v.Value, dk.rule
			break
		}
	}
	if rule < 0 && w.syn.ghDefaults {
		if v := mappingValue(mappingValue(mappingValue(n, "defaults"), "run"), "working-directory"); v != nil && v.Kind == yaml.ScalarNode {
			key, val, rule = "defaults.run.working-directory", v.Value, dirFromRoot
		}
	}
	if rule < 0 {
		return func() {}
	}
	saved, savedUnknown := w.dir, w.dirUnknown
	switch rule {
	case dirOpaque:
		w.dir, w.dirUnknown = "", fmt.Sprintf("%s %q is a working directory this audit does not model", key, val)
	case dirFromRoot:
		w.dir, w.dirUnknown = joinRunDir(".", val, key)
	default:
		// A task's `dir:` may start from Task's own absolute paths:
		// TASKFILE_DIR is the file's directory, ROOT_DIR the root
		// Taskfile's — the usual fix for an include-relative path.
		if m := taskDirVarRe.FindStringSubmatch(val); m != nil {
			base := "."
			if m[1] == "TASKFILE_DIR" {
				base = path.Dir(w.rel)
			}
			w.dir, w.dirUnknown = joinRunDir(base, "."+m[2], key)
		} else if w.dirUnknown == "" {
			w.dir, w.dirUnknown = joinRunDir(w.dir, val, key)
		}
	}
	return func() { w.dir, w.dirUnknown = saved, savedUnknown }
}

// taskDirVarRe is a task `dir:` rooted at one of Task's path variables.
var taskDirVarRe = regexp.MustCompile(`^\{\{\s*\.(TASKFILE_DIR|ROOT_DIR)\s*\}\}(/[^{}$]*)?$`)

// place stamps a ref with this walker's file, line and working directory.
func (w *yamlCommandWalker) place(r commandRef, line int) commandRef {
	r.File, r.Line = w.rel, line
	if w.dirUnknown != "" {
		r.DirUnknown = w.dirUnknown
	} else {
		r.Dirs = []string{w.dir}
	}
	return r
}

// shell reads a shell-bearing value: a scalar, or a sequence whose scalars
// are shell and whose mappings are walked (a Taskfile cmd entry like
// `{task: test}` or `{cmd: ...}`).
func (w *yamlCommandWalker) shell(n *yaml.Node) {
	switch n.Kind {
	case yaml.ScalarNode:
		switch {
		case n.Style&yaml.LiteralStyle != 0:
			// A literal block keeps its lines: line i of the value is source
			// line n.Line+1+i.
			for i, ln := range strings.Split(n.Value, "\n") {
				refs, unk := shellRefs(ln, true)
				w.unknown += unk
				for _, r := range refs {
					w.refs = append(w.refs, w.place(r, n.Line+1+i))
				}
			}
		case n.Style&yaml.FoldedStyle != 0:
			// A folded block joins its source lines, so read the joined
			// command and place each ref back on the source line carrying it.
			refs, unk := shellRefs(n.Value, true)
			w.unknown += unk
			start, block := blockLines(w.lines, n.Line)
			cursor := 0
			for _, r := range refs {
				line := start
				if at := findRefLine(block, cursor, r); at >= 0 {
					line, cursor = start+at, at
				}
				w.refs = append(w.refs, w.place(r, line))
			}
		default:
			// A plain or quoted scalar spanning lines has no reliable
			// per-line position: every ref sits on the key's line.
			for _, ln := range strings.Split(n.Value, "\n") {
				refs, unk := shellRefs(ln, true)
				w.unknown += unk
				for _, r := range refs {
					w.refs = append(w.refs, w.place(r, n.Line))
				}
			}
		}
	case yaml.SequenceNode:
		for _, c := range n.Content {
			if c.Kind == yaml.ScalarNode {
				w.shell(c)
			} else {
				w.walk(c)
			}
		}
	case yaml.MappingNode:
		w.walk(n)
	}
}

// blockLines returns the source lines of the block scalar whose indicator
// sits on line indicator (1-based): the lines under it indented at least
// as deep as its first non-blank line, and the 1-based line they start on.
func blockLines(lines []string, indicator int) (int, []string) {
	start := indicator + 1
	var out []string
	indent := -1
	for i := indicator; i < len(lines); i++ { // lines[indicator] is source line indicator+1
		ln := lines[i]
		if strings.TrimSpace(ln) == "" {
			out = append(out, ln)
			continue
		}
		ind := len(ln) - len(strings.TrimLeft(ln, " \t"))
		if indent < 0 {
			indent = ind
		}
		if ind < indent {
			break
		}
		out = append(out, ln)
	}
	return start, out
}

// findRefLine finds the first block line at or after from that carries r
// — its script path, or `task NAME` — as whole words; -1 when r spans a
// fold.
func findRefLine(block []string, from int, r commandRef) int {
	var re *regexp.Regexp
	if r.Script {
		re = scriptWordRe(r.Name)
	} else {
		re = regexp.MustCompile(`(^|[\s;&|(])task\s+` + regexp.QuoteMeta(r.Name) + `($|[\s;&|)])`)
	}
	for i := from; i < len(block); i++ {
		if re.MatchString(block[i]) {
			return i
		}
	}
	return -1
}

// scriptWordRe matches a script path as a whole word on a line — bounded
// by space, quotes, shell separators, or a YAML flow sequence's brackets
// and commas. Group 1 and 2 are the boundaries, so a substitution keeps
// them.
func scriptWordRe(p string) *regexp.Regexp {
	return regexp.MustCompile(`(^|[\s"'=\[{,(])` + regexp.QuoteMeta(p) + `($|[\s"';&|)\]},])`)
}

// names reads a task-name-bearing value: `task: build`, or a deps list
// whose scalars are names and whose mappings carry their own `task:`. The
// name resolves in this file's include namespace; a leading ':' is the
// root's.
func (w *yamlCommandWalker) names(n *yaml.Node) {
	switch n.Kind {
	case yaml.ScalarNode:
		switch {
		case n.Value == "":
		case !taskNameRe.MatchString(n.Value):
			w.unknown++
		default:
			q := w.prefix + n.Value
			if strings.HasPrefix(n.Value, ":") {
				q = n.Value[1:]
			}
			w.refs = append(w.refs, commandRef{File: w.rel, Line: n.Line, Name: n.Value, Qualified: q})
		}
	case yaml.SequenceNode:
		for _, c := range n.Content {
			if c.Kind == yaml.ScalarNode {
				w.names(c)
			} else {
				w.walk(c)
			}
		}
	case yaml.MappingNode:
		w.walk(n)
	}
}

// markdownCommands reads commands from a markdown file's code: shell
// fences line by line (an unlabelled fence, or sh / bash / shell / zsh /
// console — a ```text or ```go block is not a command line), and inline
// code spans. A bare path in a span is a POINTER (pointers_resolve's job),
// so spans contribute a script only when it is invoked (`bash
// scripts/x.sh`, `./scripts/x.sh`).
func markdownCommands(rel, text string) ([]commandRef, int) {
	var refs []commandRef
	unknown := 0
	// A doc does not say where its reader stands: its commands resolve from
	// the repo root or from the doc's own directory.
	dirs := []string{"."}
	if d := path.Dir(rel); d != "." {
		dirs = append(dirs, d)
	}
	inFence, shellFence := false, false
	for i, ln := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(ln)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			if !inFence {
				lang := ""
				if f := strings.Fields(strings.ToLower(strings.TrimLeft(trimmed, "`~"))); len(f) > 0 {
					lang = f[0]
				}
				shellFence = shellFenceInfo[lang]
			}
			inFence = !inFence
			continue
		}
		var lineRefs []commandRef
		switch {
		case inFence && shellFence:
			cmd := strings.TrimPrefix(strings.TrimPrefix(trimmed, "$ "), "% ")
			r, unk := shellRefs(cmd, true)
			lineRefs, unknown = r, unknown+unk
		case inFence:
			// another language's code: not a command line
		default:
			for _, m := range backtickTokenRe.FindAllStringSubmatch(ln, -1) {
				r, unk := shellRefs(m[1], false)
				lineRefs = append(lineRefs, r...)
				unknown += unk
			}
		}
		for _, r := range lineRefs {
			r.File, r.Line, r.Dirs = rel, i+1, dirs
			refs = append(refs, r)
		}
	}
	return refs, unknown
}

// shellFenceInfo are the fence labels read as command lines ("" is an
// unlabelled fence).
var shellFenceInfo = map[string]bool{
	"": true, "sh": true, "bash": true, "shell": true, "zsh": true, "console": true, "shell-session": true,
}

var (
	// taskNameRe is a literal Task target name (namespaces use ':'; a
	// leading ':' names the root Taskfile's).
	taskNameRe = regexp.MustCompile(`^:?[A-Za-z0-9_][A-Za-z0-9_:.-]*$`)
	// scriptPathRe is an in-repo script path worth holding to existence.
	scriptPathRe = regexp.MustCompile(`^(\./)?[A-Za-z0-9_.-]+(/[A-Za-z0-9_.-]+)*\.(sh|bash|zsh|py|rb|pl)$`)
	// quotedRe blanks quoted strings before a line is read.
	quotedRe = regexp.MustCompile(`"(?:[^"\\]|\\.)*"|'[^']*'`)
	// commandSepRe splits a shell line into simple commands.
	commandSepRe = regexp.MustCompile("[;&|()`]+|\\$\\(")
	// envAssignRe is a leading VAR=value on a simple command.
	envAssignRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)
)

// shellLead are words that precede the real command in a simple command.
var shellLead = map[string]bool{
	"if": true, "then": true, "else": true, "elif": true, "do": true, "while": true,
	"until": true, "!": true, "exec": true, "time": true, "command": true, "nohup": true,
}

// interpreters run a script named as their first non-flag argument.
var interpreters = map[string]bool{
	"bash": true, "sh": true, "zsh": true, "source": true, ".": true,
	"python": true, "python3": true, "ruby": true, "perl": true,
}

// shellRefs reads one line of shell for in-repo command refs. barePath
// says whether a command word that is itself a script path counts
// (`scripts/x.sh args`) — true for real shell, false for a markdown span,
// where a bare path is a pointer rather than an invocation.
func shellRefs(line string, barePath bool) ([]commandRef, int) {
	line = quotedRe.ReplaceAllString(line, " Q ")
	if i := commentStart(line); i >= 0 {
		line = line[:i]
	}
	var refs []commandRef
	unknown := 0
	for _, simple := range commandSepRe.Split(line, -1) {
		toks := strings.Fields(simple)
		for len(toks) > 0 && (shellLead[toks[0]] || envAssignRe.MatchString(toks[0])) {
			toks = toks[1:]
		}
		if len(toks) == 0 {
			continue
		}
		cmd := toks[0]
		switch {
		case cmd == "task":
			if len(toks) < 2 {
				continue // bare `task` runs the default target; nothing named
			}
			name := toks[1]
			switch {
			case strings.HasPrefix(name, "-"):
				if name != "--list" && name != "-l" && name != "--list-all" && name != "-a" && name != "--version" && name != "--help" && name != "-h" && name != "--init" {
					unknown++ // a flag before the target (-d, -t, …) changes what resolves
				}
			case !taskNameRe.MatchString(name):
				unknown++ // templated or otherwise not a literal name
			default:
				refs = append(refs, commandRef{Name: name})
			}
		case interpreters[cmd]:
			for _, t := range toks[1:] {
				if strings.HasPrefix(t, "-") {
					continue
				}
				if r, unk, ok := scriptToken(t); ok {
					refs = append(refs, r)
				} else {
					unknown += unk
				}
				break
			}
		case barePath || strings.HasPrefix(cmd, "./"):
			if r, unk, ok := scriptToken(cmd); ok {
				refs = append(refs, r)
			} else {
				unknown += unk
			}
		}
	}
	return refs, unknown
}

// scriptToken decides whether a token is an in-repo script path: ok when
// it is; unk=1 when it names a script this audit cannot resolve
// (templated, variable, glob).
func scriptToken(t string) (commandRef, int, bool) {
	if scriptPathRe.MatchString(t) && strings.Contains(t, "/") {
		return commandRef{Script: true, Name: t}, 0, true
	}
	for _, ext := range []string{".sh", ".bash", ".zsh", ".py", ".rb", ".pl"} {
		if strings.HasSuffix(t, ext) && strings.ContainsAny(t, "${}*~") {
			return commandRef{}, 1, false
		}
	}
	return commandRef{}, 0, false
}

// commentStart finds an unquoted shell comment (a '#' opening a word).
// Quotes are already blanked when this runs.
func commentStart(line string) int {
	for i := 0; i < len(line); i++ {
		if line[i] == '#' && (i == 0 || line[i-1] == ' ' || line[i-1] == '\t') {
			return i
		}
	}
	return -1
}

// deadCommands evaluates every command ref across a corpus. It returns
// the refs that do not resolve, how many resolved, how many were seen but
// not judgeable, and an error only for an unreadable input (I38: that is
// unable-to-check, never a smaller count — a script that exists and
// cannot be stat'd included).
func deadCommands(root, where string, files []string) (dead []deadCommand, resolved, unknown int, err error) {
	res := &commandResolver{root: root, graphs: map[string]*taskGraph{}}

	// The Taskfile corpus is read per NODE — each file under the namespace
	// and directory it is included with; every other corpus per file, from
	// the root.
	var nodes []taskNode
	var corpusGraph *taskGraph
	first := ""
	if where == CorpusTaskfile {
		var ferr error
		if first, ferr = rootTaskfile(root); ferr != nil {
			return nil, 0, 0, ferr
		}
		if first != "" {
			if corpusGraph, err = res.graph(first); err != nil {
				return nil, 0, 0, err
			}
			want := map[string]bool{}
			for _, f := range files {
				want[f] = true
			}
			for _, n := range corpusGraph.nodes {
				if want[n.file] {
					nodes = append(nodes, n)
				}
			}
			// Evidence in file order, as every other corpus reports it.
			sort.SliceStable(nodes, func(i, j int) bool { return nodes[i].file < nodes[j].file })
		}
	} else {
		for _, rel := range files {
			nodes = append(nodes, taskNode{file: rel, dir: "."})
		}
	}

	seen := map[string]bool{}
	for _, node := range nodes {
		data, rerr := os.ReadFile(filepath.Join(root, filepath.FromSlash(node.file)))
		if rerr != nil {
			return nil, 0, 0, fmt.Errorf("%s unreadable: %w", node.file, rerr)
		}
		refs, unk, xerr := extractCommands(where, node, data)
		if xerr != nil {
			return nil, 0, 0, xerr
		}
		unknown += unk
		for _, r := range refs {
			status, why, jerr := res.judge(r, corpusGraph)
			if jerr != nil {
				return nil, 0, 0, jerr
			}
			// A file included twice is read twice; a ref judged the same
			// way twice counts once.
			key := fmt.Sprintf("%s:%d:%t:%s:%d:%s", r.File, r.Line, r.Script, r.Name, status, why)
			if seen[key] {
				continue
			}
			seen[key] = true
			switch status {
			case refResolved:
				resolved++
			case refUnknown:
				unknown++
			default:
				dead = append(dead, deadCommand{commandRef: r, Why: why})
			}
		}
	}

	// A Taskfile's broken local include breaks every `task` run, whether or
	// not anything names a target inside it: it is a dead link of its own.
	if corpusGraph != nil {
		for _, why := range sortedValues(corpusGraph.broken) {
			dead = append(dead, deadCommand{commandRef: commandRef{File: first, Line: 0, Name: "include"}, Why: why})
		}
	}
	return dead, resolved, unknown, nil
}

func sortedValues(m map[string]string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range m {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}

type refStatus int

const (
	refResolved refStatus = iota
	refDead
	refUnknown
)

// commandResolver judges refs, loading each Taskfile's graph once.
type commandResolver struct {
	root   string
	graphs map[string]*taskGraph // by the Taskfile it was loaded from
	near   map[string]string     // dir → the Taskfile Task finds from it ("" for none)
}

func (res *commandResolver) graph(taskfile string) (*taskGraph, error) {
	if g, ok := res.graphs[taskfile]; ok {
		return g, nil
	}
	g, err := loadTaskGraph(res.root, taskfile)
	if err != nil {
		return nil, err
	}
	res.graphs[taskfile] = g
	return g, nil
}

// nearest is the Taskfile Task finds when started in dir: dir's own, else
// the nearest parent's, up to the repo root ("" when none).
func (res *commandResolver) nearest(dir string) (string, error) {
	if res.near == nil {
		res.near = map[string]string{}
	}
	if tf, ok := res.near[dir]; ok {
		return tf, nil
	}
	tf, err := taskfileIn(res.root, dir)
	if err != nil {
		return "", err
	}
	if tf == "" && dir != "." {
		if tf, err = res.nearest(path.Dir(dir)); err != nil {
			return "", err
		}
	}
	res.near[dir] = tf
	return tf, nil
}

// judge decides one ref. corpusGraph is the task graph a Taskfile's own
// `task:` / `deps:` names resolve in.
func (res *commandResolver) judge(r commandRef, corpusGraph *taskGraph) (refStatus, string, error) {
	if r.Qualified != "" && corpusGraph == nil {
		return refUnknown, "", nil // a Taskfile name with no graph to resolve it in
	}
	if r.Qualified != "" {
		missing, uncheckable := corpusGraph.resolveTarget(r.Qualified)
		switch {
		case uncheckable != "":
			return refUnknown, "", nil
		case missing == "":
			return refResolved, "", nil
		case missing == "no such target" && r.Qualified != strings.TrimPrefix(r.Name, ":"):
			why := fmt.Sprintf("no such target %s — a call inside an include resolves in its namespace", r.Qualified)
			if m, u := corpusGraph.resolveTarget(r.Name); m == "" && u == "" {
				why += "; the root's is :" + r.Name
			}
			return refDead, why, nil
		}
		return refDead, missing, nil
	}
	if r.DirUnknown != "" {
		return refUnknown, "", nil
	}
	if r.Script {
		return scriptStatus(res.root, r.File, r.Dirs, r.Name)
	}

	// A shell `task X`: a new task process, resolving X at the root of the
	// Taskfile it finds from each directory the command may run in.
	name := strings.TrimPrefix(r.Name, ":")
	status, why := refDead, ""
	found := false
	for _, dir := range r.Dirs {
		tf, err := res.nearest(dir)
		if err != nil {
			return 0, "", err
		}
		if tf == "" {
			continue
		}
		found = true
		g, err := res.graph(tf)
		if err != nil {
			return 0, "", err
		}
		missing, uncheckable := g.resolveTarget(name)
		switch {
		case uncheckable != "":
			status = refUnknown
		case missing == "":
			return refResolved, "", nil
		case why == "":
			why = missing
		}
	}
	if !found {
		if len(r.Dirs) == 1 && r.Dirs[0] != "." {
			return refDead, fmt.Sprintf("no Taskfile in %s/ or above it", r.Dirs[0]), nil
		}
		return refDead, "no Taskfile at the repo root", nil
	}
	if status == refUnknown {
		return refUnknown, "", nil
	}
	return refDead, why, nil
}

// scriptStatus resolves a script path from the directories its command
// may run in. A candidate that exists and cannot be stat'd is an error
// (I38), never "no such file". A path that leaves the repo from every
// directory is not this repo's to hold — not checkable — unless it
// resolves from the citing file's own directory while the command runs
// elsewhere: the include-relative path Task runs from the root.
func scriptStatus(root, rel string, dirs []string, p string) (refStatus, string, error) {
	clean := path.Clean(strings.TrimPrefix(p, "./"))
	stat := func(cand string) (bool, error) {
		ok, err := present(filepath.Join(root, filepath.FromSlash(cand)))
		if err != nil {
			return false, fmt.Errorf("%s: %s cannot be checked: %w", rel, cand, err)
		}
		return ok, nil
	}
	inside := 0
	for _, dir := range dirs {
		cand := path.Join(dir, clean)
		if escapesRepo(cand) {
			continue
		}
		inside++
		ok, err := stat(cand)
		if err != nil {
			return 0, "", err
		}
		if ok {
			return refResolved, "", nil
		}
	}

	// Name the likeliest mistake: the path is right relative to the file
	// that cites it, and wrong from where the command runs.
	hint := ""
	if own := path.Dir(rel); len(dirs) == 1 && dirs[0] != own {
		if cand := path.Join(own, clean); !escapesRepo(cand) {
			if ok, err := stat(cand); err == nil && ok {
				hint = fmt.Sprintf(" — it resolves from %s, the file's own directory, but the command runs in %s", runDirNoun(own), runDirNoun(dirs[0]))
			}
		}
	}
	switch {
	case inside > 0 && len(dirs) == 1 && dirs[0] != "." && hint == "":
		return refDead, "no such file in " + dirs[0] + "/", nil
	case inside > 0:
		return refDead, "no such file" + hint, nil
	case hint != "":
		return refDead, "leaves the repo from " + runDirNoun(dirs[0]) + hint, nil
	}
	return refUnknown, "", nil
}

func runDirNoun(dir string) string {
	if dir == "." {
		return "the repo root"
	}
	return dir + "/"
}

func checkCommandsResolve(root string, files []string, c Concern) Finding {
	f := Finding{Concern: c, Files: files}
	if len(files) == 0 {
		f.Status = StatusUnable
		f.Detail = fmt.Sprintf("the %s corpus resolved no files (%s) — there is nothing here to hold to its commands", c.Check.Where, CorpusAbout(c.Check.Where))
		return f
	}
	dead, resolved, unknown, err := deadCommands(root, c.Check.Where, files)
	if err != nil {
		f.Status = StatusUnable
		f.Detail = err.Error()
		return f
	}
	unk := ""
	if unknown > 0 {
		unk = fmt.Sprintf("; %d not checkable (templated, flagged, into a remote include, outside the repo, or run from a directory this audit cannot model)", unknown)
	}
	if len(dead) > 0 {
		for _, d := range dead {
			f.Evidence = append(f.Evidence, d.evidence())
		}
		f.Status = StatusViolated
		f.Detail = fmt.Sprintf("%d of %d command reference(s) do not resolve across %s%s", len(dead), len(dead)+resolved, countNoun(len(files), corpusNoun(c.Check.Where)), unk)
		return f
	}
	f.Status = StatusHolds
	if resolved == 0 {
		f.Detail = fmt.Sprintf("no in-repo command references found across %s%s", countNoun(len(files), corpusNoun(c.Check.Where)), unk)
		return f
	}
	f.Detail = fmt.Sprintf("%d command reference(s) resolve across %s%s", resolved, countNoun(len(files), corpusNoun(c.Check.Where)), unk)
	return f
}
