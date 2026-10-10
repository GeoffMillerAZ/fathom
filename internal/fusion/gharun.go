package fusion

// gharun.go — LB-F310: what a GitHub Actions `run:` step runs. The first
// command of each line of a run step names a Makefile target (`make [-C dir]
// T`), a Taskfile task (`task T`), a package.json script (`npm run S`,
// `yarn S`, `pnpm [run] S`) or a script file (`bash path`, `./path`), and the
// step is the origin of a file-tier typed edge to that target's entity,
// stored with the doc-xref edges (the step entity is the dialect's
// GHARunStep, named `<dotted keys to the list>#<index>`, or, for a step with
// an id, the GHAStep its id names: O631, K20).
//
// The rule is a token rule over the step's shell words (shellCommands), tier
// C: nothing evaluates a shell, so a command behind a variable, a function, a
// subshell's own directory or a conditional is read as written. A reference
// resolves only when its target exists: a make target the named Makefile
// defines, a task the named Taskfile declares, a script the package.json
// carries, a script file that defines a function. Nothing else is guessed,
// which caps the grade at good (the cross-reference family stays tier C).

import (
	"path"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// ghaRunEdge and ghaRunStrategy name the edges this pass writes.
const (
	ghaRunEdge     = "run_step"
	ghaRunStrategy = "run-token"
)

// ghaRunStep is one `run:` step of a workflow or a composite action.
type ghaRunStep struct {
	File string // the workflow or action file
	Name string // the step's entity name: its id, else its GHARunStep position
	Line int
	Dir  string // the working directory, repository-relative ("." when none)
	Run  string // the run script
}

// isGHARunFile: a workflow (a .yml or .yaml directly in .github/workflows), or
// a composite or reusable action's metadata file.
func isGHARunFile(p string) bool {
	b := path.Base(p)
	if b == "action.yml" || b == "action.yaml" {
		return true
	}
	ext := strings.ToLower(path.Ext(p))
	d := path.Dir(p)
	return (ext == ".yml" || ext == ".yaml") && (d == ".github/workflows" || strings.HasSuffix(d, "/.github/workflows"))
}

// ghaBuildStep is one docker/build-push-action step: the tags it pushes, built
// from the Dockerfile its context and file inputs name (LB-F311).
type ghaBuildStep struct {
	File       string
	Uses       string // the step's uses: as written: the GHAAction entity's name
	Line       int
	Context    string
	Dockerfile string
	Tags       []string
}

// ghaSteps reads the run steps and the image build steps of one file. A run
// step's entity name is its id (the dialect's GHAStep), else the dialect's
// `fmt:{$path}#{$index}`: the keys from the root to the step list, dotted (the
// GHARunStep the key-absent selector makes of an id-less run step).
func ghaSteps(relPath string, content []byte) ([]ghaRunStep, []ghaBuildStep) {
	var doc yaml.Node
	if err := yaml.Unmarshal(content, &doc); err != nil || len(doc.Content) == 0 {
		return nil, nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, nil
	}
	var out []ghaRunStep
	var builds []ghaBuildStep
	collect := func(prefix string, seq *yaml.Node, defDir string) {
		if seq == nil || seq.Kind != yaml.SequenceNode {
			return
		}
		for i, s := range seq.Content {
			if s.Kind != yaml.MappingNode {
				continue
			}
			if uses := ghaScalar(ghaKey(s, "uses")); uses == "docker/build-push-action" || strings.HasPrefix(uses, "docker/build-push-action@") {
				with := ghaKey(s, "with")
				builds = append(builds, ghaBuildStep{File: relPath, Uses: uses, Line: s.Line,
					Context: ghaScalar(ghaKey(with, "context")), Dockerfile: ghaScalar(ghaKey(with, "file")),
					Tags: splitImageTags(ghaScalar(ghaKey(with, "tags")))})
			}
			run := ghaScalar(ghaKey(s, "run"))
			if run == "" {
				continue
			}
			dir := defDir
			if wd := ghaScalar(ghaKey(s, "working-directory")); wd != "" {
				dir = wd
			}
			name := prefix + "#" + strconv.Itoa(i)
			if id := ghaScalar(ghaKey(s, "id")); id != "" {
				name = id
			}
			out = append(out, ghaRunStep{File: relPath, Name: name, Line: s.Line, Dir: cleanRunDir(dir), Run: run})
		}
	}
	wfDir := ghaScalar(ghaKey(ghaKey(ghaKey(root, "defaults"), "run"), "working-directory"))
	if jobs := ghaKey(root, "jobs"); jobs != nil && jobs.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(jobs.Content); i += 2 {
			job := jobs.Content[i+1]
			if job.Kind != yaml.MappingNode {
				continue
			}
			jd := wfDir
			if d := ghaScalar(ghaKey(ghaKey(ghaKey(job, "defaults"), "run"), "working-directory")); d != "" {
				jd = d
			}
			collect("jobs."+jobs.Content[i].Value+".steps", ghaKey(job, "steps"), jd)
		}
	}
	if runs := ghaKey(root, "runs"); runs != nil {
		collect("runs.steps", ghaKey(runs, "steps"), "")
	}
	return out, builds
}

// splitImageTags is a build-push-action tags input: one per line or comma.
func splitImageTags(v string) []string {
	var out []string
	for _, t := range strings.FieldsFunc(v, func(r rune) bool { return r == '\n' || r == ',' || r == '\r' }) {
		if t = strings.TrimSpace(t); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// ghaBuildSites is a file's build-push-action steps as image build sites for
// the image-tag index: each tag, built from the Dockerfile the step names
// (file, else context/Dockerfile; both from the repository root, where the
// action runs). A path context only: a Git URL builds nothing here.
func ghaBuildSites(steps []ghaBuildStep) []ImageBuild {
	var out []ImageBuild
	for _, b := range steps {
		ctx := b.Context
		if ctx == "" {
			ctx = "."
		}
		if strings.Contains(ctx, "://") || strings.HasPrefix(ctx, "git@") {
			continue
		}
		bad := templated(ctx + b.Dockerfile)
		var dfs []string
		p := path.Join(ctx, "Dockerfile")
		if b.Dockerfile != "" {
			p = path.Join(".", b.Dockerfile)
		}
		if c, ok := inRepo(p); ok && !bad {
			dfs = []string{c}
		}
		for _, t := range b.Tags {
			out = append(out, ImageBuild{Tag: t, Line: b.Line, Dockerfiles: dfs, Templated: bad})
		}
	}
	return out
}

func ghaKey(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

func ghaScalar(n *yaml.Node) string {
	if n == nil || n.Kind != yaml.ScalarNode {
		return ""
	}
	return n.Value
}

// cleanRunDir is a working-directory as a repository-relative directory, "."
// for none, and "?" for one an expression computes.
func cleanRunDir(d string) string {
	d = strings.TrimSpace(d)
	if d == "" {
		return "."
	}
	if strings.Contains(d, "${{") || strings.Contains(d, "$") {
		return "?"
	}
	return joinRunDir(".", d)
}

// joinRunDir resolves p against dir; "?" when it leaves the repository.
func joinRunDir(dir, p string) string {
	if dir == "?" || p == "" {
		return dir
	}
	if strings.HasPrefix(p, "/") || strings.HasPrefix(p, "~") || strings.Contains(p, "$") {
		return "?"
	}
	c := path.Clean(path.Join(dir, p))
	if c == ".." || strings.HasPrefix(c, "../") {
		return "?"
	}
	return c
}

// ghaRunRef is one reference a run step makes to a build entry.
type ghaRunRef struct {
	Kind   string   // "make", "task", "npm" or "script"
	Dir    string   // the directory the command runs in
	Files  []string // for make and task, an explicit file (-f, -t); empty: the directory's default names
	Target string   // a target, task or script name; for a script, the file path
}

var (
	makefileNames = []string{"GNUmakefile", "makefile", "Makefile"}
	taskfileNames = []string{"Taskfile.yml", "Taskfile.yaml", "taskfile.yml", "taskfile.yaml", "Taskfile.dist.yml", "Taskfile.dist.yaml", "taskfile.dist.yml", "taskfile.dist.yaml"}
	shellNames    = map[string]bool{"bash": true, "sh": true, "zsh": true, "dash": true, "ksh": true}
	// package managers' own commands: `yarn <word>` and `pnpm <word>` run a
	// script only when the word is none of these.
	pmBuiltins = map[string]bool{
		"add": true, "audit": true, "autoclean": true, "bin": true, "cache": true, "check": true, "config": true, "create": true,
		"dedupe": true, "dlx": true, "exec": true, "explain": true, "global": true, "help": true, "import": true, "info": true,
		"init": true, "install": true, "i": true, "ci": true, "licenses": true, "link": true, "list": true, "ls": true, "login": true,
		"logout": true, "node": true, "npm": true, "outdated": true, "owner": true, "pack": true, "patch": true, "plugin": true,
		"prune": true, "publish": true, "rebuild": true, "remove": true, "rm": true, "set": true, "store": true, "tag": true,
		"team": true, "unlink": true, "unplug": true, "up": true, "update": true, "upgrade": true, "upgrade-interactive": true,
		"version": true, "versions": true, "why": true, "workspace": true, "workspaces": true, "env": true, "fetch": true, "root": true,
		"setup": true, "self-update": true, "approve-builds": true, "deploy": true, "recursive": true, "multi": true,
	}
)

// ghaRunRefs reads the references of one run script, started in dir.
func ghaRunRefs(run, dir string) []ghaRunRef {
	var out []ghaRunRef
	cwd := dir
	for _, cmd := range shellCommands([]byte(run)) {
		w := cmd.words
		// the command word: past environment assignments and prefix words
		i := 0
		for i < len(w) && (shellPrefixWords[w[i]] || (strings.Contains(w[i], "=") && !strings.HasPrefix(w[i], "-") && !strings.Contains(w[i], "/"))) {
			i++
		}
		if i >= len(w) {
			continue
		}
		w = w[i:]
		name := w[0]
		args := w[1:]
		switch base := path.Base(name); {
		case name == "cd":
			if len(args) >= 1 && !strings.HasPrefix(args[0], "-") {
				cwd = joinRunDir(cwd, args[0])
			}
		case base == "make" || base == "gmake":
			out = append(out, makeRefs(args, cwd)...)
		case base == "task" && name == "task":
			out = append(out, taskRefs(args, cwd)...)
		case name == "npm" || name == "yarn" || name == "pnpm":
			out = append(out, pmRefs(name, args, cwd)...)
		case shellNames[base]:
			if r, ok := shellScriptRef(args, cwd); ok {
				out = append(out, r)
			}
		case name == "source" || name == ".":
			if len(args) >= 1 {
				if r, ok := scriptRef(args[0], cwd); ok {
					out = append(out, r)
				}
			}
		default:
			if strings.HasPrefix(name, "./") || strings.HasPrefix(name, "../") {
				if r, ok := scriptRef(name, cwd); ok {
					out = append(out, r)
				}
			}
		}
	}
	return out
}

func plainWord(s string) bool { return s != "" && !strings.ContainsAny(s, "$`*?[{") }

// makeRefs: `make [-C dir] [-f file] [VAR=v] target...`.
func makeRefs(args []string, cwd string) []ghaRunRef {
	dir := cwd
	var files, targets []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			continue
		case a == "-C" || a == "--directory":
			if i++; i < len(args) {
				dir = joinRunDir(dir, args[i])
			}
		case strings.HasPrefix(a, "--directory="):
			dir = joinRunDir(dir, strings.TrimPrefix(a, "--directory="))
		case strings.HasPrefix(a, "-C") && len(a) > 2:
			dir = joinRunDir(dir, a[2:])
		case a == "-f" || a == "--file" || a == "--makefile":
			if i++; i < len(args) {
				files = append(files, args[i])
			}
		case strings.HasPrefix(a, "--file="):
			files = append(files, strings.TrimPrefix(a, "--file="))
		case strings.HasPrefix(a, "--makefile="):
			files = append(files, strings.TrimPrefix(a, "--makefile="))
		case a == "-I" || a == "-o" || a == "-W" || a == "-l":
			i++
		case a == "-j" || a == "--jobs":
			if i+1 < len(args) {
				if _, err := strconv.Atoi(args[i+1]); err == nil {
					i++
				}
			}
		case strings.HasPrefix(a, "-"):
		case strings.Contains(a, "="):
		default:
			if plainWord(a) {
				targets = append(targets, a)
			}
		}
	}
	var out []ghaRunRef
	for _, t := range targets {
		out = append(out, ghaRunRef{Kind: "make", Dir: dir, Files: files, Target: t})
	}
	return out
}

// taskRefs: `task [-t file] [-d dir] task...`.
func taskRefs(args []string, cwd string) []ghaRunRef {
	dir := cwd
	var files, targets []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			i = len(args)
		case a == "-d" || a == "--dir":
			if i++; i < len(args) {
				dir = joinRunDir(dir, args[i])
			}
		case strings.HasPrefix(a, "--dir="):
			dir = joinRunDir(dir, strings.TrimPrefix(a, "--dir="))
		case a == "-t" || a == "--taskfile":
			if i++; i < len(args) {
				files = append(files, args[i])
			}
		case strings.HasPrefix(a, "--taskfile="):
			files = append(files, strings.TrimPrefix(a, "--taskfile="))
		case a == "-C" || a == "--concurrency" || a == "-o" || a == "--output" || a == "-I" || a == "--interval":
			i++
		case strings.HasPrefix(a, "-"):
		case strings.Contains(a, "="):
		default:
			if plainWord(a) {
				targets = append(targets, a)
			}
		}
	}
	var out []ghaRunRef
	for _, t := range targets {
		out = append(out, ghaRunRef{Kind: "task", Dir: dir, Files: files, Target: t})
	}
	return out
}

// pmRefs: `npm run S`, `npm test`, `yarn [run] S`, `pnpm [run] S`.
func pmRefs(pm string, args []string, cwd string) []ghaRunRef {
	dir := cwd
	i := 0
	for ; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--prefix" || a == "-C" || a == "--cwd" || a == "--dir":
			if i++; i < len(args) {
				dir = joinRunDir(dir, args[i])
			}
		case strings.HasPrefix(a, "--prefix="):
			dir = joinRunDir(dir, strings.TrimPrefix(a, "--prefix="))
		case strings.HasPrefix(a, "--cwd="):
			dir = joinRunDir(dir, strings.TrimPrefix(a, "--cwd="))
		case a == "--filter" || a == "-F" || a == "-w" || a == "--workspace" || a == "--registry":
			return nil // a command for another package: not the root's script
		case strings.HasPrefix(a, "-"):
		default:
			goto sub
		}
	}
	return nil
sub:
	cmd := args[i]
	rest := args[i+1:]
	script := ""
	switch {
	case cmd == "run" || cmd == "run-script" || cmd == "rum" || cmd == "urn":
		for _, a := range rest {
			if a == "--" {
				break
			}
			if strings.HasPrefix(a, "-") {
				if pm != "npm" && (a == "--filter" || a == "-F") {
					return nil
				}
				continue
			}
			script = a
			break
		}
	case pm == "npm" && (cmd == "test" || cmd == "t" || cmd == "tst" || cmd == "start" || cmd == "stop" || cmd == "restart"):
		script = map[string]string{"t": "test", "tst": "test"}[cmd]
		if script == "" {
			script = cmd
		}
	case pm != "npm" && !pmBuiltins[cmd]:
		script = cmd
	}
	if !plainWord(script) {
		return nil
	}
	return []ghaRunRef{{Kind: "npm", Dir: dir, Target: script}}
}

// shellScriptRef: `bash [-flags] path` (not `bash -c '...'`).
func shellScriptRef(args []string, cwd string) (ghaRunRef, bool) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-c" || a == "--command":
			return ghaRunRef{}, false
		case a == "-o" || a == "+o" || a == "--rcfile" || a == "--init-file":
			i++
		case strings.HasPrefix(a, "-") || strings.HasPrefix(a, "+"):
		default:
			return scriptRef(a, cwd)
		}
	}
	return ghaRunRef{}, false
}

// scriptRef: a literal relative path (or one under the working directory).
func scriptRef(p, cwd string) (ghaRunRef, bool) {
	if !plainWord(p) || strings.HasPrefix(p, "/") || strings.HasPrefix(p, "~") || strings.Contains(p, "://") || cwd == "?" {
		return ghaRunRef{}, false
	}
	full := joinRunDir(cwd, p)
	if full == "?" {
		return ghaRunRef{}, false
	}
	return ghaRunRef{Kind: "script", Dir: path.Dir(full), Target: full}, true
}

// ghaRunResolver resolves references against what the repository holds.
type ghaRunResolver struct {
	paths   map[string]bool                       // every discovered file
	nodes   map[string]map[string]core.SymbolNode // file -> entity name -> node, doc files
	extract func(file string) []core.SymbolNode   // a non-doc file's symbols (a Makefile, a script)
	cache   map[string][]core.SymbolNode
}

func (r *ghaRunResolver) symbols(file string) []core.SymbolNode {
	if ns, ok := r.cache[file]; ok {
		return ns
	}
	var ns []core.SymbolNode
	if r.extract != nil {
		ns = r.extract(file)
	}
	r.cache[file] = ns
	return ns
}

// candidates are the files a reference may name, in order.
func (r *ghaRunResolver) candidates(ref ghaRunRef) []string {
	if ref.Dir == "?" {
		return nil
	}
	var names []string
	switch {
	case len(ref.Files) > 0:
		for _, f := range ref.Files {
			if plainWord(f) {
				names = append(names, joinRunDir(ref.Dir, f))
			}
		}
		return keepPaths(r.paths, names)
	case ref.Kind == "make":
		for _, n := range makefileNames {
			names = append(names, path.Join(ref.Dir, n))
		}
	case ref.Kind == "task":
		for _, n := range taskfileNames {
			names = append(names, path.Join(ref.Dir, n))
		}
	case ref.Kind == "npm":
		names = []string{path.Join(ref.Dir, "package.json")}
	}
	return keepPaths(r.paths, names)
}

func keepPaths(paths map[string]bool, names []string) []string {
	var out []string
	for _, n := range names {
		if n != "?" && paths[n] {
			out = append(out, n)
		}
	}
	return out
}

// resolve is the node a reference reaches, with its file.
func (r *ghaRunResolver) resolve(ref ghaRunRef) (string, core.SymbolNode, bool) {
	switch ref.Kind {
	case "task", "npm":
		for _, f := range r.candidates(ref) {
			if n, ok := r.nodes[f][ref.Target]; ok {
				return f, n, true
			}
		}
	case "make":
		for _, f := range r.candidates(ref) {
			for _, n := range r.symbols(f) {
				if n.Name == ref.Target && string(n.Kind) == "MakeTarget" {
					return f, n, true
				}
			}
		}
	case "script":
		if !r.paths[ref.Target] || r.nodes[ref.Target] != nil {
			return "", core.SymbolNode{}, false
		}
		var first *core.SymbolNode
		syms := r.symbols(ref.Target)
		for i := range syms {
			n := syms[i]
			if !callableScriptKind(n) {
				continue
			}
			if n.Name == "main" {
				return ref.Target, n, true
			}
			if first == nil || n.Line < first.Line {
				first = &syms[i]
			}
		}
		if first != nil {
			return ref.Target, *first, true
		}
	}
	return "", core.SymbolNode{}, false
}

// callableScriptKind: a function a script defines (the entry a caller runs).
func callableScriptKind(n core.SymbolNode) bool {
	switch string(n.Kind) {
	case "BashFunction", "function", "Function", "PowerShellFunction":
		return true
	}
	return false
}

// resolveGHARun resolves every run step's references to edges: sorted,
// de-duplicated per (step, target).
func resolveGHARun(steps []ghaRunStep, files []DocFile, paths map[string]bool, extract func(file string) []core.SymbolNode) []DocXref {
	res := &ghaRunResolver{paths: paths, nodes: map[string]map[string]core.SymbolNode{}, extract: extract, cache: map[string][]core.SymbolNode{}}
	for _, f := range files {
		m := map[string]core.SymbolNode{}
		for _, n := range f.Result.Nodes {
			if _, dup := m[n.Name]; !dup {
				m[n.Name] = n
			}
		}
		res.nodes[f.Path] = m
	}
	var out []DocXref
	seen := map[string]bool{}
	for _, s := range steps {
		from, ok := res.nodes[s.File][s.Name]
		if !ok || s.Dir == "?" {
			continue
		}
		for _, ref := range ghaRunRefs(s.Run, s.Dir) {
			file, to, ok := res.resolve(ref)
			if !ok || to.Key() == from.Key() {
				continue
			}
			id := s.File + "\x00" + s.Name + "\x00" + file + "\x00" + to.Name
			if seen[id] {
				continue
			}
			seen[id] = true
			out = append(out, DocXref{FromFile: s.File, From: s.Name, FromKey: from.Key(), ToFile: file, To: to.Name, ToKey: to.Key(), Edge: ghaRunEdge, Strategy: ghaRunStrategy})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		for _, p := range [][2]string{{a.FromFile, b.FromFile}, {a.From, b.From}, {a.ToFile, b.ToFile}, {a.To, b.To}} {
			if p[0] != p[1] {
				return p[0] < p[1]
			}
		}
		return false
	})
	return out
}

// Edges a build-push-action step writes (LB-F311): to the Dockerfile that
// builds the image it pushes, and to each entity of the repository that names
// that image (a Compose service's image:, a Kubernetes container's).
const (
	ghaBuildsImageEdge   = "builds_image"
	ghaImageConsumerEdge = "image_consumer"
)

// resolveGHABuilds resolves each literal tag of a build-push-action step. A
// tag holding an expression (${{ }}) names no one image and resolves to
// nothing, as everywhere else in the image-tag index.
func resolveGHABuilds(builds []ghaBuildStep, files []DocFile) []DocXref {
	type fk struct{ file, name string }
	nodeAt := map[fk]core.SymbolNode{}
	roots := map[string]string{}
	for _, f := range files {
		for _, n := range f.Result.Nodes {
			if _, dup := nodeAt[fk{f.Path, n.Name}]; !dup {
				nodeAt[fk{f.Path, n.Name}] = n
			}
		}
		if len(f.Result.Roots) > 0 {
			roots[f.Path] = f.Result.Roots[0]
		}
	}
	type consumer struct{ file, name string }
	byTag := map[string][]consumer{}
	for _, f := range files {
		for _, c := range f.Result.CrossFile {
			if c.Strategy != "image-ref" || templated(c.Raw) {
				continue
			}
			tag, ok := NormalizeImageRef(c.Raw)
			if !ok {
				continue
			}
			name := c.From
			if name == "" {
				name = roots[f.Path]
			}
			if name != "" {
				byTag[tag] = append(byTag[tag], consumer{f.Path, name})
			}
		}
	}
	var out []DocXref
	seen := map[string]bool{}
	add := func(from, to fk, edge string) {
		fn, ok1 := nodeAt[from]
		tn, ok2 := nodeAt[to]
		if !ok1 || !ok2 || fn.Key() == tn.Key() {
			return
		}
		id := from.file + "\x00" + from.name + "\x00" + to.file + "\x00" + to.name + "\x00" + edge
		if seen[id] {
			return
		}
		seen[id] = true
		out = append(out, DocXref{FromFile: from.file, From: from.name, FromKey: fn.Key(), ToFile: to.file, To: to.name, ToKey: tn.Key(), Edge: edge, Strategy: ghaRunStrategy})
	}
	for _, b := range builds {
		from := fk{b.File, b.Uses}
		for _, site := range ghaBuildSites([]ghaBuildStep{b}) {
			tag, ok := NormalizeImageRef(site.Tag)
			if site.Templated || templated(site.Tag) || !ok {
				continue
			}
			for _, d := range site.Dockerfiles {
				if r, ok := roots[d]; ok {
					add(from, fk{d, r}, ghaBuildsImageEdge)
					break
				}
			}
			for _, c := range byTag[tag] {
				if c.file != b.File {
					add(from, fk{c.file, c.name}, ghaImageConsumerEdge)
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		for _, p := range [][2]string{{a.FromFile, b.FromFile}, {a.From, b.From}, {a.ToFile, b.ToFile}, {a.To, b.To}, {a.Edge, b.Edge}} {
			if p[0] != p[1] {
				return p[0] < p[1]
			}
		}
		return false
	})
	return out
}
