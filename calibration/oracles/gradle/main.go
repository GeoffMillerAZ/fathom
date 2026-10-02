// Command gradle-oracle is the language-breadth harness's tier-A oracle for the gradle dialect
// (Gradle build scripts in the Groovy DSL; LB-F95, design §1.3). It writes indexbench
// lang-oracle's tool-oracle JSON (-tool-oracle, design §10): definitions, references and
// cross_references.
//
//	go run . -subject <checkout> -scratch <dir> -model <file> [-offline] [-files-out <file>] > gradle-<subject>.json
//	go run . -subject <checkout> -model <file> -reuse [-files-out <file>]          # re-read a model
//
// Gradle evaluates the build. The subject is copied to -scratch (so no build output lands in
// the checkout) and its own wrapper runs `help` with the init script oracle.gradle, which writes,
// after every project is configured, the settings' project descriptors and settings plugins and,
// per project, its build file, applied plugins, task names, each candidate task's dependsOn and
// finalizedBy, and each configuration's declared dependencies (oracle.gradle's header). The
// candidates it checks (plugin ids, task names) come from a first scan of every *.gradle file.
//
// Gradle writes no source line, so each entity's line comes from a separate line scan of the
// files Gradle evaluated (the settings script and each project's build script) — a brace-depth
// and regular-expression reader independent of tree-sitter — and an entity is in the truth only
// when Gradle attests it:
//   - include (module): each project path a settings `include` names, when the settings hold a
//     project of that path;
//   - plugin (resource): each `id '...'` of a top-level plugins block and each `apply plugin:
//     '...'`, when the project the line configures has applied that id (the settings, for the
//     settings script; any project, for an `apply false`);
//   - dependency (package): each string notation ('group:name[:version]', compared by group and
//     name, a `$` segment matching any) and each project(':x') in a dependencies block, when the
//     configuration the line names declares it in the project the line configures; a
//     buildscript block's are its buildscript configurations';
//   - task (task): each `task name`, `task('name')` and `tasks.register/create('name')`, when the
//     project the line configures holds a task of that name.
//
// The project a line configures is the file's own (the project whose build script it is),
// the one an enclosing project(':x') { } block names, or any project of the build inside a
// subprojects { }, allprojects { } or configure(...) { } block.
//
// References are a declared task's dependsOn and finalizedBy as Gradle evaluated them (the task
// realized), and a project dependency's or include's path to the project it names. Cross-file
// references are a project dependency and an include to the build script of the project Gradle
// resolved them to, when that script exists in the subject (file granularity).
//
// Bench-only; never a product dependency.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type def struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Line int    `json:"line"`
	Kind string `json:"kind"`
}

type ref struct {
	Path string `json:"path"`
	From string `json:"from"`
	To   string `json:"to"`
}

type xref struct {
	Path   string `json:"path"`
	From   string `json:"from"`
	ToPath string `json:"to_path"`
}

type out struct {
	Tool            string `json:"tool"`
	Version         string `json:"version"`
	Tier            string `json:"tier"`
	Definitions     []def  `json:"definitions"`
	References      []ref  `json:"references"`
	CrossReferences []xref `json:"cross_references"`
}

// ── the Gradle model (oracle.gradle's output) ───────────────────────────

type descriptor struct {
	Path      string `json:"path"`
	Dir       string `json:"dir"`
	BuildFile string `json:"buildFile"`
}

type taskDetail struct {
	Name        string   `json:"name"`
	DependsOn   []string `json:"dependsOn"`
	FinalizedBy []string `json:"finalizedBy"`
}

type dependency struct {
	Configuration string `json:"configuration"`
	Project       string `json:"project"`
	Group         string `json:"group"`
	Name          string `json:"name"`
	Version       string `json:"version"`
}

type project struct {
	Path         string       `json:"path"`
	Dir          string       `json:"dir"`
	BuildFile    string       `json:"buildFile"`
	Plugins      []string     `json:"plugins"`
	TaskNames    []string     `json:"taskNames"`
	Tasks        []taskDetail `json:"tasks"`
	Dependencies []dependency `json:"dependencies"`
}

type model struct {
	Gradle   string `json:"gradle"`
	Settings struct {
		Dir      string       `json:"dir"`
		Plugins  []string     `json:"plugins"`
		Projects []descriptor `json:"projects"`
	} `json:"settings"`
	Projects []project `json:"projects"`
	Errors   []string  `json:"errors"`
}

// ── the line scan ───────────────────────────────────────────────────────

type entity struct {
	name, config string // config: a dependency's configuration ("buildscript:" prefixed)
	line         int
	project      bool // a project(':x') dependency
	applyFalse   bool // a plugins-block id with `apply false`
	// scope is the project the line configures: "" the file's own, a path inside a
	// project(':x') { } block, "*" inside subprojects/allprojects/configure blocks.
	scope string
}

type scan struct {
	includes, plugins, deps, tasks []entity
}

var (
	reQuoted      = regexp.MustCompile(`(['"])((?:\\.|[^\\])*?)['"]`)
	rePluginsOpen = regexp.MustCompile(`^\s*plugins\s*\{`)
	rePluginID    = regexp.MustCompile(`^\s*id\s*\(?\s*(['"])([^'"]+)['"]`)
	reApplyFalse  = regexp.MustCompile(`\bapply\s*\(?\s*false\b`)
	reApply       = regexp.MustCompile(`\bapply\s*\(?\s*plugin\s*:\s*(['"])([^'"]+)['"]`)
	reDepsOpen    = regexp.MustCompile(`(^|[^.\w])dependencies\s*\{`)
	reBuildscript = regexp.MustCompile(`^\s*buildscript\s*\{`)
	reConfigWord  = regexp.MustCompile(`^\s*([A-Za-z_]\w*)`)
	reProject     = regexp.MustCompile(`\bproject\s*\(\s*(?:path\s*:\s*)?(['"])([^'"]+)['"][^)]*\)(\s*\.)?`)
	reProjectOpen = regexp.MustCompile(`^\s*project\s*\(\s*(['"])([^'"]+)['"]\s*\)\s*\{`)
	reAnyOpen     = regexp.MustCompile(`^\s*(subprojects|allprojects|configure\s*\()`)
	reInclude     = regexp.MustCompile(`^\s*include\s*[(\s'"]`)
	reIfOpen      = regexp.MustCompile(`^\s*if\s*\(.*\{\s*$`)
	reTaskWord    = regexp.MustCompile(`^\s*task\s+([A-Za-z_]\w*)`)
	reTaskCall    = regexp.MustCompile(`\btask\s*\(\s*(['"])([^'"]+)['"]`)
	reTaskReg     = regexp.MustCompile(`\btasks\.(?:register|create)\s*\(\s*(['"])([^'"]+)['"]`)
)

// codeLines strips comments from src and returns each line's code (strings kept) and the brace
// depth at the start of each line (braces inside strings and comments not counted).
func codeLines(src string) (code []string, depth []int) {
	lines := strings.Split(src, "\n")
	inBlock := false
	d := 0
	for _, l := range lines {
		var b strings.Builder
		var quote byte
		for i := 0; i < len(l); i++ {
			c := l[i]
			if inBlock {
				if c == '*' && i+1 < len(l) && l[i+1] == '/' {
					inBlock = false
					i++
				}
				continue
			}
			if quote != 0 {
				b.WriteByte(c)
				if c == '\\' && i+1 < len(l) {
					b.WriteByte(l[i+1])
					i++
				} else if c == quote {
					quote = 0
				}
				continue
			}
			if c == '/' && i+1 < len(l) && l[i+1] == '/' {
				break
			}
			if c == '/' && i+1 < len(l) && l[i+1] == '*' {
				inBlock = true
				i++
				continue
			}
			if c == '\'' || c == '"' {
				quote = c
			}
			b.WriteByte(c)
		}
		code = append(code, b.String())
		depth = append(depth, d)
		d += braceDelta(b.String())
		if d < 0 {
			d = 0
		}
	}
	return code, depth
}

// braceDelta is the change in brace depth a line of code makes, outside its strings.
func braceDelta(code string) int {
	n := 0
	var quote byte
	for i := 0; i < len(code); i++ {
		c := code[i]
		if quote != 0 {
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
		case '{':
			n++
		case '}':
			n--
		}
	}
	return n
}

// quotedStrings is the content of each quoted literal of a line of code.
func quotedStrings(code string) []string {
	var out []string
	for _, m := range reQuoted.FindAllStringSubmatch(code, -1) {
		out = append(out, m[2])
	}
	return out
}

// scanScript reads one build or settings script.
func scanScript(src string, settings bool) scan {
	code, depth := codeLines(src)
	var s scan
	type block struct {
		kind  string // plugins, deps, if, buildscript, project, any
		depth int    // depth at the line that opened it
		pfx   string // deps: configuration prefix; project: the path
	}
	var stack []block
	lastConfig := ""
	includeOpen := false
	for i, c := range code {
		line := i + 1
		d := depth[i]
		// Pop every block whose body ended before this line.
		for len(stack) > 0 && d <= stack[len(stack)-1].depth {
			stack = stack[:len(stack)-1]
		}
		top := func(kind string) (block, bool) {
			if len(stack) > 0 && stack[len(stack)-1].kind == kind && d == stack[len(stack)-1].depth+1 {
				return stack[len(stack)-1], true
			}
			return block{}, false
		}
		inBuildscript := false
		scope := ""
		for _, b := range stack {
			switch b.kind {
			case "buildscript":
				inBuildscript = true
			case "project":
				scope = b.pfx
			case "any":
				scope = "*"
			}
		}
		// includes: the settings' top level, or a top-level if body; an include's strings may
		// continue on the next lines while its argument list is open.
		if settings {
			_, inIf := top("if")
			if (d == 0 || inIf) && reInclude.MatchString(c) {
				for _, q := range quotedStrings(c) {
					s.includes = append(s.includes, entity{name: q, line: line})
				}
				t := strings.TrimSpace(c)
				includeOpen = strings.HasSuffix(t, ",") || (strings.Count(t, "(") > strings.Count(t, ")"))
				continue
			}
			if includeOpen {
				for _, q := range quotedStrings(c) {
					s.includes = append(s.includes, entity{name: q, line: line})
				}
				t := strings.TrimSpace(c)
				includeOpen = strings.HasSuffix(t, ",") || (strings.Count(t, "(") > strings.Count(t, ")") && !strings.Contains(t, ")"))
				continue
			}
		}
		if _, ok := top("plugins"); ok {
			if m := rePluginID.FindStringSubmatch(c); m != nil {
				s.plugins = append(s.plugins, entity{name: m[2], line: line, applyFalse: reApplyFalse.MatchString(c), scope: scope})
			}
		}
		for _, m := range reApply.FindAllStringSubmatch(c, -1) {
			s.plugins = append(s.plugins, entity{name: m[2], line: line, scope: scope})
		}
		if b, ok := top("deps"); ok {
			word := ""
			if m := reConfigWord.FindStringSubmatch(c); m != nil {
				word = m[1]
			}
			strs := quotedStrings(c)
			cfg := word
			switch word {
			case "", "project", "platform", "enforcedPlatform", "testFixtures":
				cfg = lastConfig // a notation continued from its configuration's open parenthesis
			case "add":
				cfg = ""
				if len(strs) >= 2 {
					cfg, strs = strs[0], strs[1:]
				}
			}
			if cfg == word {
				lastConfig = ""
				if t := strings.TrimSpace(c); strings.HasSuffix(t, "(") || strings.HasSuffix(t, ",") {
					lastConfig = word
				}
			}
			// A line that opens a nested block before any string (constraints { }) declares nothing
			// here; a notation followed by its own closure (cfg('g:a:v') { }) does.
			q := strings.IndexAny(c, `'"`)
			nested := strings.Contains(c, "{") && (q < 0 || strings.Index(c, "{") < q)
			if cfg != "" && !nested {
				if ps := reProject.FindAllStringSubmatch(c, -1); len(ps) > 0 {
					// project(':x').sourceSets.test.output is a file collection of that
					// project's output, not a project dependency.
					for _, p := range ps {
						if p[3] == "" {
							s.deps = append(s.deps, entity{name: p[2], config: b.pfx + cfg, line: line, project: true, scope: scope})
						}
					}
				} else {
					for _, q := range strs {
						if strings.Contains(q, ":") {
							s.deps = append(s.deps, entity{name: q, config: b.pfx + cfg, line: line, scope: scope})
						}
					}
				}
			}
		}
		if m := reTaskWord.FindStringSubmatch(c); m != nil {
			s.tasks = append(s.tasks, entity{name: m[1], line: line, scope: scope})
		}
		for _, m := range reTaskCall.FindAllStringSubmatch(c, -1) {
			s.tasks = append(s.tasks, entity{name: m[2], line: line, scope: scope})
		}
		for _, m := range reTaskReg.FindAllStringSubmatch(c, -1) {
			s.tasks = append(s.tasks, entity{name: m[2], line: line, scope: scope})
		}
		// Blocks this line opens (its body starts on the next line).
		if braceDelta(c) > 0 {
			switch {
			case d == 0 && rePluginsOpen.MatchString(c):
				stack = append(stack, block{kind: "plugins", depth: d})
			case reBuildscript.MatchString(c):
				stack = append(stack, block{kind: "buildscript", depth: d})
			case reProjectOpen.MatchString(c):
				stack = append(stack, block{kind: "project", depth: d, pfx: reProjectOpen.FindStringSubmatch(c)[2]})
			case reAnyOpen.MatchString(c):
				stack = append(stack, block{kind: "any", depth: d})
			case reDepsOpen.MatchString(c):
				pfx := ""
				if inBuildscript {
					pfx = "buildscript:"
				}
				stack = append(stack, block{kind: "deps", depth: d, pfx: pfx})
			case d == 0 && settings && reIfOpen.MatchString(c):
				stack = append(stack, block{kind: "if", depth: d})
			default:
				stack = append(stack, block{kind: "other", depth: d})
			}
		}
	}
	return s
}

// ── attestation ─────────────────────────────────────────────────────────

type depKey struct{ config, group, name string }

type index struct {
	m        model
	root     string // the directory Gradle ran in (paths in the model are under it)
	byFile   map[string]*project
	byPath   map[string]*project
	settings map[string]bool   // project paths the settings hold
	dirs     map[string]string // a descriptor's project directory -> its path
}

func newIndex(m model, root string) *index {
	ix := &index{m: m, root: root, byFile: map[string]*project{}, byPath: map[string]*project{}, settings: map[string]bool{}, dirs: map[string]string{}}
	for i := range m.Projects {
		p := &m.Projects[i]
		ix.byPath[p.Path] = p
		if rel, ok := ix.rel(p.BuildFile); ok {
			ix.byFile[rel] = p
		}
	}
	for _, d := range m.Settings.Projects {
		ix.settings[d.Path] = true
		if rel, ok := ix.rel(d.Dir); ok {
			ix.dirs[rel] = d.Path
		}
	}
	return ix
}

// rel is an absolute path of the model as a subject-relative slash path.
func (ix *index) rel(abs string) (string, bool) {
	r, err := filepath.Rel(ix.root, abs)
	if err != nil || strings.HasPrefix(r, "..") {
		return "", false
	}
	return filepath.ToSlash(r), true
}

// absPath is a written project path resolved from the declaring project (Gradle's rule: a path
// without a leading ':' is relative to the project it is resolved from).
func absPath(written, from string) string {
	if strings.HasPrefix(written, ":") {
		return written
	}
	if from == ":" || from == "" {
		return ":" + written
	}
	return from + ":" + written
}

func segMatch(written, actual string) bool {
	return strings.Contains(written, "$") || written == actual
}

func (p *project) declares(e entity) bool {
	for _, d := range p.Dependencies {
		if d.Configuration != e.config {
			continue
		}
		if e.project {
			if d.Project != "" && d.Project == absPath(e.name, p.Path) {
				return true
			}
			if d.Project != "" && d.Project == absPath(e.name, ":") {
				return true
			}
			continue
		}
		parts := strings.Split(e.name, ":")
		if d.Project == "" && len(parts) >= 2 && segMatch(parts[0], d.Group) && segMatch(parts[1], d.Name) {
			return true
		}
	}
	return false
}

// candidatesFor is the projects an entity scanned in a file can belong to: the project its
// enclosing project(':x') { } block names, every project inside subprojects/allprojects/
// configure blocks and for a script no project owns, else the file's own project.
func (ix *index) candidatesFor(e entity, own *project) []*project {
	switch {
	case e.scope != "" && e.scope != "*":
		if p := ix.byPath[absPath(e.scope, ":")]; p != nil {
			return []*project{p}
		}
		return nil
	case e.scope == "*" || own == nil:
		out := make([]*project, len(ix.m.Projects))
		for i := range ix.m.Projects {
			out[i] = &ix.m.Projects[i]
		}
		return out
	default:
		return []*project{own}
	}
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

// counts is the cross-check written to stderr (and -summary-out).
type counts map[string]int

// unattested lists each scanned entity Gradle did not attest (-unattested-out), for diagnosis.
var unattested []string

func noteUnattested(c counts, key, f string, e entity) {
	c[key]++
	unattested = append(unattested, fmt.Sprintf("%s %s:%d %s %s", key, f, e.line, e.config, e.name))
}

func (ix *index) assemble(subject string, files []string, c counts) out {
	o := out{Tool: "gradle (help, init-script model)", Version: ix.m.Gradle, Tier: "A"}
	seenRef := map[ref]bool{}
	addRef := func(r ref) {
		if !seenRef[r] {
			seenRef[r] = true
			o.References = append(o.References, r)
		}
	}
	targetFile := func(path string) (string, bool) {
		p := ix.byPath[path]
		if p == nil {
			return "", false
		}
		rel, ok := ix.rel(p.BuildFile)
		if !ok {
			return "", false
		}
		if _, err := os.Stat(filepath.Join(subject, filepath.FromSlash(rel))); err != nil {
			return "", false
		}
		return rel, true
	}
	for _, f := range files {
		raw, err := os.ReadFile(filepath.Join(subject, filepath.FromSlash(f)))
		if err != nil {
			c["unreadable"]++
			continue
		}
		own := ix.byFile[f]
		isSettings := own == nil && (filepath.Base(f) == "settings.gradle")
		s := scanScript(string(raw), isSettings)
		for _, e := range s.includes {
			abs := absPath(e.name, ":")
			// A project renamed after its include (project(':a:b').name = 'c') keeps the
			// directory the include gave it: the entry is attested by that directory.
			if p, ok := ix.dirs[strings.ReplaceAll(strings.Trim(e.name, ":"), ":", "/")]; ok && !ix.settings[abs] {
				abs = p
				c["include_attested_by_directory"]++
			}
			if !ix.settings[abs] {
				noteUnattested(c, "include_not_attested", f, e)
				continue
			}
			c["include"]++
			o.Definitions = append(o.Definitions, def{Path: f, Name: e.name, Line: e.line, Kind: "module"})
			addRef(ref{Path: f, From: e.name, To: e.name})
			if to, ok := targetFile(abs); ok {
				o.CrossReferences = append(o.CrossReferences, xref{Path: f, From: e.name, ToPath: to})
			}
		}
		for _, e := range s.plugins {
			ok := false
			if isSettings {
				ok = contains(ix.m.Settings.Plugins, e.name)
			} else {
				ps := ix.candidatesFor(e, own)
				if e.applyFalse {
					// apply false puts the plugin on the build's classpath for the projects
					// that apply it themselves.
					ps = ix.candidatesFor(entity{scope: "*"}, own)
				}
				for _, p := range ps {
					if contains(p.Plugins, e.name) {
						ok = true
						break
					}
				}
			}
			if !ok {
				noteUnattested(c, "plugin_not_attested", f, e)
				continue
			}
			c["plugin"]++
			o.Definitions = append(o.Definitions, def{Path: f, Name: e.name, Line: e.line, Kind: "resource"})
		}
		for _, e := range s.deps {
			var by *project
			for _, p := range ix.candidatesFor(e, own) {
				if p.declares(e) {
					by = p
					break
				}
			}
			if by == nil {
				noteUnattested(c, "dependency_not_attested", f, e)
				continue
			}
			c["dependency"]++
			o.Definitions = append(o.Definitions, def{Path: f, Name: e.name, Line: e.line, Kind: "package"})
			if e.project {
				addRef(ref{Path: f, From: e.name, To: e.name})
				target := absPath(e.name, by.Path)
				if ix.byPath[target] == nil {
					target = absPath(e.name, ":")
				}
				if to, ok := targetFile(target); ok {
					o.CrossReferences = append(o.CrossReferences, xref{Path: f, From: e.name, ToPath: to})
				}
			}
		}
		for _, e := range s.tasks {
			var by *project
			for _, p := range ix.candidatesFor(e, own) {
				if contains(p.TaskNames, e.name) {
					by = p
					break
				}
			}
			if by == nil {
				noteUnattested(c, "task_not_attested", f, e)
				continue
			}
			c["task"]++
			o.Definitions = append(o.Definitions, def{Path: f, Name: e.name, Line: e.line, Kind: "task"})
			for _, t := range by.Tasks {
				if t.Name != e.name {
					continue
				}
				for _, to := range append(append([]string{}, t.DependsOn...), t.FinalizedBy...) {
					addRef(ref{Path: f, From: e.name, To: to})
					c["task_reference"]++
				}
			}
		}
	}
	sort.Slice(o.Definitions, func(i, j int) bool {
		a, b := o.Definitions[i], o.Definitions[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Name < b.Name
	})
	sort.Slice(o.References, func(i, j int) bool {
		a, b := o.References[i], o.References[j]
		return a.Path+"\x00"+a.From+"\x00"+a.To < b.Path+"\x00"+b.From+"\x00"+b.To
	})
	sort.Slice(o.CrossReferences, func(i, j int) bool {
		a, b := o.CrossReferences[i], o.CrossReferences[j]
		return a.Path+"\x00"+a.From+"\x00"+a.ToPath < b.Path+"\x00"+b.From+"\x00"+b.ToPath
	})
	return o
}

// measuredFiles is the settings script and every project build script Gradle evaluated that
// exists in the subject, sorted.
func (ix *index) measuredFiles(subject string) []string {
	set := map[string]bool{}
	add := func(abs string) {
		rel, ok := ix.rel(abs)
		if !ok {
			return
		}
		if _, err := os.Stat(filepath.Join(subject, filepath.FromSlash(rel))); err == nil {
			set[rel] = true
		}
	}
	if ix.m.Settings.Dir != "" {
		add(filepath.Join(ix.m.Settings.Dir, "settings.gradle"))
	}
	for _, p := range ix.m.Projects {
		if strings.HasSuffix(p.BuildFile, ".gradle") {
			add(p.BuildFile)
		}
	}
	var out []string
	for f := range set {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

// ── candidates and the Gradle run ───────────────────────────────────────

func skipDir(name string) bool {
	switch name {
	case ".git", ".gradle", "build", "node_modules", "out":
		return true
	}
	return false
}

// candidates is every plugin id and task name any *.gradle file of the subject spells.
func candidates(subject string) (plugins, tasks []string, err error) {
	ps, ts := map[string]bool{}, map[string]bool{}
	err = filepath.WalkDir(subject, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != subject && skipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".gradle") {
			return nil
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		s := scanScript(string(raw), d.Name() == "settings.gradle")
		for _, e := range s.plugins {
			ps[e.name] = true
		}
		for _, e := range s.tasks {
			ts[e.name] = true
		}
		return nil
	})
	for k := range ps {
		plugins = append(plugins, k)
	}
	for k := range ts {
		tasks = append(tasks, k)
	}
	sort.Strings(plugins)
	sort.Strings(tasks)
	return plugins, tasks, err
}

func copyTree(src, dst string) error {
	cmd := exec.Command("rsync", "-a", "--delete", src+"/", dst+"/")
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runGradle(ctx context.Context, dir, initScript, modelPath, candPath string, offline bool, log io.Writer) error {
	args := []string{"-n", "10", "./gradlew", "-q", "--no-configuration-cache", "--no-daemon",
		"-Dorg.gradle.jvmargs=-Xmx2g", "-I", initScript,
		"-DfathomOracle.out=" + modelPath, "-DfathomOracle.candidates=" + candPath}
	if offline {
		args = append(args, "--offline")
	}
	args = append(args, "help")
	cmd := exec.CommandContext(ctx, "nice", args...)
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = log, log
	return cmd.Run()
}

func main() {
	subject := flag.String("subject", "", "the subject checkout (required)")
	scratch := flag.String("scratch", "", "a directory outside the subject to copy it to and run Gradle in")
	initScript := flag.String("init", "oracle.gradle", "the init script")
	modelPath := flag.String("model", "", "where the Gradle model is written (or, with -reuse, read)")
	reuse := flag.Bool("reuse", false, "read -model instead of running Gradle")
	root := flag.String("root", "", "with -reuse: the directory Gradle ran in (default: the model's settings dir)")
	offline := flag.Bool("offline", false, "run Gradle --offline (a warm GRADLE_USER_HOME)")
	timeout := flag.Duration("timeout", 20*time.Minute, "the Gradle run's limit")
	filesOut := flag.String("files-out", "", "write the measured files (subject-relative, one per line)")
	summaryOut := flag.String("summary-out", "", "write the cross-check counts as JSON")
	unattestedOut := flag.String("unattested-out", "", "write each scanned entity Gradle did not attest, one per line")
	flag.Parse()
	if *subject == "" || *modelPath == "" || (!*reuse && *scratch == "") {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*subject, *scratch, *initScript, *modelPath, *root, *reuse, *offline, *timeout, *filesOut, *summaryOut, *unattestedOut); err != nil {
		fmt.Fprintln(os.Stderr, "gradle-oracle:", err)
		os.Exit(1)
	}
}

func run(subject, scratch, initScript, modelPath, root string, reuse, offline bool, timeout time.Duration, filesOut, summaryOut, unattestedOut string) error {
	subject, _ = filepath.Abs(subject)
	if !reuse {
		plugins, tasks, err := candidates(subject)
		if err != nil {
			return err
		}
		scratch, _ = filepath.Abs(scratch)
		if strings.HasPrefix(scratch+"/", subject+"/") {
			return errors.New("-scratch must be outside the subject")
		}
		if err := os.MkdirAll(scratch, 0o755); err != nil {
			return err
		}
		if err := copyTree(subject, scratch); err != nil {
			return err
		}
		candPath := modelPath + ".candidates.json"
		cb, _ := json.Marshal(map[string][]string{"plugins": plugins, "tasks": tasks})
		if err := os.WriteFile(candPath, cb, 0o644); err != nil {
			return err
		}
		absInit, _ := filepath.Abs(initScript)
		absModel, _ := filepath.Abs(modelPath)
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		logf, err := os.Create(modelPath + ".log")
		if err != nil {
			return err
		}
		defer logf.Close()
		start := time.Now()
		if err := runGradle(ctx, scratch, absInit, absModel, candPath, offline, logf); err != nil {
			return fmt.Errorf("gradle run (%s, log %s): %w", time.Since(start).Round(time.Second), modelPath+".log", err)
		}
		fmt.Fprintf(os.Stderr, "gradle-oracle: gradle ran in %s (%d candidate plugin ids, %d candidate task names)\n", time.Since(start).Round(time.Second), len(plugins), len(tasks))
		root = scratch
	}
	raw, err := os.ReadFile(modelPath)
	if err != nil {
		return err
	}
	var m model
	if err := json.Unmarshal(raw, &m); err != nil {
		return err
	}
	if root == "" {
		root = m.Settings.Dir
	}
	root, _ = filepath.Abs(root)
	if real, err := filepath.EvalSymlinks(root); err == nil {
		root = real
	}
	ix := newIndex(m, root)
	files := ix.measuredFiles(subject)
	c := counts{"files": len(files), "projects": len(m.Projects), "gradle_errors": len(m.Errors)}
	o := ix.assemble(subject, files, c)
	if filesOut != "" {
		if err := os.WriteFile(filesOut, []byte(strings.Join(files, "\n")+"\n"), 0o644); err != nil {
			return err
		}
	}
	if unattestedOut != "" {
		if err := os.WriteFile(unattestedOut, []byte(strings.Join(unattested, "\n")+"\n"), 0o644); err != nil {
			return err
		}
	}
	if summaryOut != "" {
		sb, _ := json.MarshalIndent(c, "", "  ")
		if err := os.WriteFile(summaryOut, sb, 0o644); err != nil {
			return err
		}
	}
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "gradle-oracle: %s %d\n", k, c[k])
	}
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	enc := json.NewEncoder(w)
	enc.SetIndent("", " ")
	return enc.Encode(o)
}
