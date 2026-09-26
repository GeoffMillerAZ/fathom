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
// a remote include) is counted and named, never guessed either way.

import (
	"fmt"
	"os"
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
// and `deps:` list).
type commandSyntax struct {
	markdown  bool
	shellKeys map[string]bool
	nameKeys  map[string]bool
}

func syntaxFor(where string) commandSyntax {
	switch where {
	case CorpusLefthook:
		return commandSyntax{shellKeys: set("run")}
	case CorpusTaskfile:
		return commandSyntax{shellKeys: set("cmd", "cmds"), nameKeys: set("task", "deps")}
	case CorpusCI:
		return commandSyntax{shellKeys: set("run", "script", "before_script", "after_script", "command", "commands")}
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
// plus how many refs it saw but cannot judge.
func extractCommands(where, rel string, data []byte) ([]commandRef, int, error) {
	syn := syntaxFor(where)
	if syn.markdown {
		refs, unk := markdownCommands(rel, string(data))
		return refs, unk, nil
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, 0, fmt.Errorf("%s does not parse as YAML: %w", rel, err)
	}
	w := &yamlCommandWalker{rel: rel, syn: syn}
	w.walk(&doc)
	return w.refs, w.unknown, nil
}

type yamlCommandWalker struct {
	rel     string
	syn     commandSyntax
	refs    []commandRef
	unknown int
}

func (w *yamlCommandWalker) walk(n *yaml.Node) {
	switch n.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, c := range n.Content {
			w.walk(c)
		}
	case yaml.MappingNode:
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

// shell reads a shell-bearing value: a scalar, or a sequence whose scalars
// are shell and whose mappings are walked (a Taskfile cmd entry like
// `{task: test}` or `{cmd: ...}`).
func (w *yamlCommandWalker) shell(n *yaml.Node) {
	switch n.Kind {
	case yaml.ScalarNode:
		first := n.Line
		if n.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
			first = n.Line + 1 // a block scalar's text starts under its indicator
		}
		for i, ln := range strings.Split(n.Value, "\n") {
			line := first + i
			if n.Style&(yaml.LiteralStyle|yaml.FoldedStyle) == 0 {
				line = n.Line // a folded plain/quoted scalar has no reliable per-line position
			}
			refs, unk := shellRefs(ln, true)
			w.unknown += unk
			for _, r := range refs {
				r.File, r.Line = w.rel, line
				w.refs = append(w.refs, r)
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

// names reads a task-name-bearing value: `task: build`, or a deps list
// whose scalars are names and whose mappings carry their own `task:`.
func (w *yamlCommandWalker) names(n *yaml.Node) {
	switch n.Kind {
	case yaml.ScalarNode:
		switch {
		case n.Value == "":
		case !taskNameRe.MatchString(n.Value):
			w.unknown++
		default:
			w.refs = append(w.refs, commandRef{File: w.rel, Line: n.Line, Name: n.Value})
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
			r.File, r.Line = rel, i+1
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
	// taskNameRe is a literal Task target name (namespaces use ':').
	taskNameRe = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_:.-]*$`)
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
// unable-to-check, never a smaller count).
func deadCommands(root, where string, files []string) (dead []deadCommand, resolved, unknown int, err error) {
	var graph *taskGraph
	graphLoaded := false
	var graphErr error
	taskfileName := ""
	loadGraph := func() {
		if graphLoaded {
			return
		}
		graphLoaded = true
		taskfileName, graphErr = rootTaskfile(root)
		if graphErr != nil || taskfileName == "" {
			return
		}
		graph, graphErr = loadTaskGraph(root, taskfileName)
	}

	for _, rel := range files {
		data, rerr := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if rerr != nil {
			return nil, 0, 0, fmt.Errorf("%s unreadable: %w", rel, rerr)
		}
		refs, unk, xerr := extractCommands(where, rel, data)
		if xerr != nil {
			return nil, 0, 0, xerr
		}
		unknown += unk
		for _, r := range refs {
			if r.Script {
				switch ok, outside := scriptStatus(root, rel, r.Name); {
				case outside:
				case ok:
					resolved++
				default:
					dead = append(dead, deadCommand{commandRef: r, Why: "no such file"})
				}
				continue
			}
			loadGraph()
			if graphErr != nil {
				return nil, 0, 0, graphErr
			}
			if graph == nil {
				dead = append(dead, deadCommand{commandRef: r, Why: "no Taskfile at the repo root"})
				continue
			}
			missing, uncheckable := graph.resolveTarget(r.Name)
			switch {
			case uncheckable != "":
				unknown++
			case missing != "":
				dead = append(dead, deadCommand{commandRef: r, Why: missing})
			default:
				resolved++
			}
		}
	}

	// A Taskfile's broken local include breaks every `task` run, whether or
	// not anything names a target inside it: it is a dead link of its own.
	if where == CorpusTaskfile {
		loadGraph()
		if graphErr != nil {
			return nil, 0, 0, graphErr
		}
		if graph != nil {
			for _, why := range sortedValues(graph.broken) {
				dead = append(dead, deadCommand{commandRef: commandRef{File: taskfileName, Line: 0, Name: "include"}, Why: why})
			}
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

// scriptStatus resolves a script path relative to the repo root or to the
// citing file's directory (Taskfiles and hooks run from either). outside
// reports a path that escapes the repo from both bases — not this repo's
// to hold, so neither resolved nor dead.
func scriptStatus(root, rel, p string) (resolves, outside bool) {
	clean := filepath.FromSlash(strings.TrimPrefix(p, "./"))
	bases := []string{root, filepath.Join(root, filepath.Dir(filepath.FromSlash(rel)))}
	inside := 0
	for _, base := range bases {
		cand := filepath.Join(base, clean)
		if r, err := filepath.Rel(root, cand); err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
			continue
		}
		inside++
		if ok, _ := present(cand); ok {
			return true, false
		}
	}
	return false, inside == 0
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
		unk = fmt.Sprintf("; %d not checkable (templated, flagged, or into a remote include)", unknown)
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
