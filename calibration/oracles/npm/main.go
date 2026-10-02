// Command npm-oracle is the language-breadth harness's oracle for the
// package-json doc-engine dialect (design §1.3, owner decision D4). It writes
// indexbench lang-oracle's tool-oracle JSON (-tool-oracle, design §10):
//
//	go run . -subject <checkout> -scratch <dir outside the subject> [-npm <npm>] \
//	    -xref-out <file> [-files-out <file>] > workspace/tmp/oracle/npm-<subject>.json
//
// The tool is npm itself, run on a scratch copy of the subject's package.json
// files (nothing is installed into the subject, nothing is written to it; the
// copy holds no .npmrc, lockfile or node_modules, and npm runs with an empty
// user and global config and no network):
//
//   - `npm query ':root' --prefix <dir>` on every manifest, npm's own load of
//     it (Arborist's package reader): the name, the scripts, the bin map as npm
//     normalizes it (a string bin becomes an object named after the unscoped
//     package), the exports map, and the four dependency maps.
//   - `npm query '*'` and `npm pkg get name --workspaces` at the subject's root
//     manifest, npm's own evaluation of its `workspaces` globs: which
//     directories are members, and their names.
//   - `npm ls --json --workspaces` at the root of a second scratch copy that
//     holds the node_modules links `npm install` makes for every member the
//     tool listed: for each member, each dependency edge npm resolves to a
//     sibling member, and the ones it marks invalid (the member's version does
//     not satisfy the written range).
//
// What it writes:
//
//   - definitions (stdout, tier A): a `package` per manifest that names itself,
//     at its opening brace; a `task` per script, a `route` per bin entry and per
//     exports entry, at the key's line. npm prints no position, so each line
//     comes from this program's own byte scanner (no encoding/json, no doc
//     engine), cross-checked against an encoding/json decode of the raw file
//     (the disagreement counts are on stderr, zero when the file's own words are
//     what npm read).
//   - references (stdout): (package, dependency name) for every name in
//     dependencies, devDependencies, peerDependencies and optionalDependencies.
//   - cross_references (-xref-out), entity granularity: the root manifest's
//     package to each member the tool lists (the dialect's `workspaces` edge),
//     and a member's package to the sibling member a dependency edge resolves to
//     (the dialect's module-source edge). An edge npm marks invalid is not one.
//
// Yarn's and pnpm's `workspace:` protocol is not one npm reads (EUNSUPPORTEDPROTOCOL:
// the whole ideal tree fails, and `npm ls` lists the edge as missing). The second
// scratch copy therefore rewrites each such spec to the range the protocol means
// (workspace:^, ~ and * name the workspace's own version; workspace:<range> is the
// range), so that npm, not this program, decides whether the sibling is joined.
// The cross-references file is tier A when no spec needed that, and tier C when any
// did (its join is npm's, its input is adapted); the counts are on stderr.
//
// -workspace-pm yarn|pnpm (pm.go) takes the sibling joins from the subject's own
// package manager instead, the one that defines the protocol, with no spec rewritten:
// the cross-references file is then tier A. npm's evaluation of the root's
// workspaces still names the members, and the package manager's listing is the
// cross-check.
//
// A manifest npm cannot read (invalid JSON: npm treats it as an empty package, no
// error) contributes nothing, as in the dialect, and is counted. A pnpm-workspace.yaml
// is read for disclosure only: the member directories its patterns name that npm did
// not list are counted on stderr, never added to the truth.
//
// -files-out lists the manifests npm evaluates as a workspace (the root and each
// member it lists, one per line): the dialect joins a dependency name to any
// manifest of the repository, vendored copies and test fixtures included, and npm
// joins only to a workspace member (anything else is a registry package), so the
// cross-references are scored on those manifests (`lang-oracle -only
// "$(paste -sd, <file>)"`), as cargo's and kustomize's are; the rest are attested
// for definitions and references only, and the reading says what it left out.
//
// Bench-only; never a product dependency.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
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
	To     string `json:"to,omitempty"`
}

// mainOut is the definitions-and-references file. It has no cross_references
// field at all: lang-oracle scores cross_references from the one file that
// carries the field.
type mainOut struct {
	Tool        string `json:"tool"`
	Version     string `json:"version"`
	Tier        string `json:"tier"`
	Definitions []def  `json:"definitions"`
	References  []ref  `json:"references"`
}

type xrefOut struct {
	Tool            string `json:"tool"`
	Version         string `json:"version"`
	Tier            string `json:"tier"`
	CrossReferences []xref `json:"cross_references"`
}

// ── a position scanner (no encoding/json) ─────────────────────────────────

// keyIndex is where the keys the oracle names are written: the opening brace
// of the root object, the root's own keys, and the keys of its scripts, bin and
// exports objects.
type keyIndex struct {
	rootLine int
	top      map[string]int
	sub      map[string]map[string]int
}

// line is the line of a key of a section ("" is the root object); a key written
// twice is where the last one is, as the decoder keeps the last.
func (k keyIndex) line(section, key string) (int, bool) {
	if section == "" {
		l, ok := k.top[key]
		return l, ok
	}
	l, ok := k.sub[section][key]
	return l, ok
}

var sectionsScanned = map[string]bool{"scripts": true, "bin": true, "exports": true}

// scanKeys walks the bytes of a manifest with a tokenizer of its own: strings
// (with escapes), braces, brackets, commas. It trusts nothing else, so it also
// reads a file with trailing garbage; whether the file is valid JSON is the
// decode's question.
func scanKeys(src []byte) (keyIndex, error) {
	idx := keyIndex{top: map[string]int{}, sub: map[string]map[string]int{}}
	type frame struct {
		object    bool
		expectKey bool
		curKey    string
	}
	var stack []frame
	line := 1
	i := 0
	if bytes.HasPrefix(src, []byte{0xEF, 0xBB, 0xBF}) {
		i = 3
	}
	started := false
	for ; i < len(src); i++ {
		c := src[i]
		switch c {
		case '\n':
			line++
		case '{':
			if !started {
				started = true
				idx.rootLine = line
			}
			stack = append(stack, frame{object: true, expectKey: true})
		case '[':
			if !started {
				return idx, errors.New("the root is not an object")
			}
			stack = append(stack, frame{})
		case '}', ']':
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case ',':
			if n := len(stack); n > 0 && stack[n-1].object {
				stack[n-1].expectKey = true
			}
		case '"':
			start := i
			startLine := line
			for i++; i < len(src) && src[i] != '"'; i++ {
				switch src[i] {
				case '\\':
					i++
				case '\n':
					line++
				}
			}
			if i >= len(src) {
				return idx, errors.New("an unterminated string")
			}
			n := len(stack)
			if n == 0 || !stack[n-1].object || !stack[n-1].expectKey {
				continue // a value
			}
			var key string
			if err := json.Unmarshal(src[start:i+1], &key); err != nil {
				return idx, fmt.Errorf("a key that is not a JSON string: %v", err)
			}
			stack[n-1].expectKey = false
			stack[n-1].curKey = key
			switch {
			case n == 1:
				idx.top[key] = startLine
			case n == 2 && sectionsScanned[stack[0].curKey]:
				sec := stack[0].curKey
				if idx.sub[sec] == nil {
					idx.sub[sec] = map[string]int{}
				}
				idx.sub[sec][key] = startLine
			}
		}
	}
	if !started {
		return idx, errors.New("no object")
	}
	return idx, nil
}

// ── npm's reading of a manifest ───────────────────────────────────────────

// queryNode is one node of `npm query` output: the manifest as npm loads it,
// plus the node's own fields.
type queryNode struct {
	Name                 json.RawMessage `json:"name"`
	Version              json.RawMessage `json:"version"`
	Location             string          `json:"location"`
	Scripts              json.RawMessage `json:"scripts"`
	Bin                  json.RawMessage `json:"bin"`
	Exports              json.RawMessage `json:"exports"`
	Dependencies         json.RawMessage `json:"dependencies"`
	DevDependencies      json.RawMessage `json:"devDependencies"`
	PeerDependencies     json.RawMessage `json:"peerDependencies"`
	OptionalDependencies json.RawMessage `json:"optionalDependencies"`
}

func stringOf(raw json.RawMessage) string {
	var s string
	if len(raw) == 0 || json.Unmarshal(raw, &s) != nil {
		return ""
	}
	return s
}

func objectOf(raw json.RawMessage) (map[string]json.RawMessage, bool) {
	var m map[string]json.RawMessage
	if len(bytes.TrimSpace(raw)) == 0 || json.Unmarshal(raw, &m) != nil || m == nil {
		return nil, false
	}
	return m, true
}

func isScalar(raw json.RawMessage) bool {
	switch t := bytes.TrimSpace(raw); {
	case len(t) == 0:
		return false
	case t[0] == '"' || t[0] == 't' || t[0] == 'f' || (t[0] >= '0' && t[0] <= '9') || t[0] == '-':
		return true
	}
	return false
}

func isNull(raw json.RawMessage) bool { return string(bytes.TrimSpace(raw)) == "null" }

// toolSets are the names npm's reading of one manifest holds, by family.
type toolSets struct {
	name        string
	tasks       map[string]bool
	bin         map[string]bool
	exports     map[string]bool
	refs        map[string]bool
	nullExports int
}

func toolSetsOf(n queryNode) toolSets {
	s := toolSets{name: stringOf(n.Name), tasks: map[string]bool{}, bin: map[string]bool{}, exports: map[string]bool{}, refs: map[string]bool{}}
	if m, ok := objectOf(n.Scripts); ok {
		for k, v := range m {
			if isScalar(v) {
				s.tasks[k] = true
			}
		}
	}
	if m, ok := objectOf(n.Bin); ok {
		for k, v := range m {
			if isScalar(v) {
				s.bin[k] = true
			}
		}
	}
	if m, ok := objectOf(n.Exports); ok {
		for k, v := range m {
			if isNull(v) {
				s.nullExports++
				continue
			}
			s.exports[k] = true
		}
	}
	for _, raw := range []json.RawMessage{n.Dependencies, n.DevDependencies, n.PeerDependencies, n.OptionalDependencies} {
		if m, ok := objectOf(raw); ok {
			for k := range m {
				s.refs[k] = true
			}
		}
	}
	return s
}

// definitionsOf are a manifest's definitions at the lines the scanner found; a
// name the scanner could not place is counted and left out.
func definitionsOf(rel string, s toolSets, idx keyIndex) ([]def, int) {
	var out []def
	unlocated := 0
	if s.name != "" {
		out = append(out, def{rel, s.name, idx.rootLine, "package"})
	}
	add := func(section string, names map[string]bool, kind string) {
		for _, k := range sortedKeys(names) {
			l, ok := idx.line(section, k)
			if !ok {
				// npm renamed the key (a string or array bin, a path-shaped
				// bin key): the entry is on the line of the section's own key.
				l, ok = idx.line("", section)
			}
			if !ok || l == 0 {
				unlocated++
				continue
			}
			out = append(out, def{rel, k, l, kind})
		}
	}
	add("scripts", s.tasks, "task")
	add("bin", s.bin, "route")
	add("exports", s.exports, "route")
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.Kind < b.Kind
	})
	return out, unlocated
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ── the independent decode ────────────────────────────────────────────────

// declared is what a raw encoding/json decode of the file says the same
// families hold, by the dialect's own selection rules.
type declared struct {
	name        string
	tasks       map[string]bool
	bin         map[string]bool
	binString   bool
	exports     map[string]bool
	nullExports int
	refs        map[string]bool
}

func declaredOf(src []byte) (declared, error) {
	d := declared{tasks: map[string]bool{}, bin: map[string]bool{}, exports: map[string]bool{}, refs: map[string]bool{}}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(src, &root); err != nil {
		return d, err
	}
	d.name = stringOf(root["name"])
	if m, ok := objectOf(root["scripts"]); ok {
		for k, v := range m {
			if isScalar(v) {
				d.tasks[k] = true
			}
		}
	}
	if m, ok := objectOf(root["bin"]); ok {
		for k, v := range m {
			if isScalar(v) {
				d.bin[k] = true
			}
		}
	} else if stringOf(root["bin"]) != "" {
		d.binString = true
	}
	if m, ok := objectOf(root["exports"]); ok {
		for k, v := range m {
			if isNull(v) {
				d.nullExports++
				continue
			}
			d.exports[k] = true
		}
	}
	for _, sec := range depSections {
		if m, ok := objectOf(root[sec]); ok {
			for k := range m {
				d.refs[k] = true
			}
		}
	}
	return d, nil
}

var depSections = []string{"dependencies", "devDependencies", "peerDependencies", "optionalDependencies"}

// ── workspaces ────────────────────────────────────────────────────────────

type member struct {
	name    string
	version string
	dir     string // subject-relative, slash-separated
}

func (m member) manifest() string { return path.Join(m.dir, "package.json") }

type edge struct{ from, to string } // member names

type siblingCounts struct {
	valid, invalid, missing, external int
}

type lsEntry struct {
	Version      string             `json:"version"`
	Missing      bool               `json:"missing"`
	Invalid      json.RawMessage    `json:"invalid"`
	Dependencies map[string]lsEntry `json:"dependencies"`
}

func (e lsEntry) invalid() bool {
	s := string(bytes.TrimSpace(e.Invalid))
	return s != "" && s != "null" && s != "false" && s != `""`
}

type lsRoot struct {
	Name         string             `json:"name"`
	Dependencies map[string]lsEntry `json:"dependencies"`
}

// siblingEdges reads `npm ls --json --workspaces`: under each member, each
// dependency that names another member. npm resolved it to that member's link,
// found it missing (the spec is one npm does not read), or marked it invalid
// (the member's version does not satisfy the written range). A name that is no
// member is an external dependency, never a candidate.
func siblingEdges(ls lsRoot, members map[string]member) ([]edge, siblingCounts) {
	var out []edge
	var c siblingCounts
	for _, ws := range sortedEntryKeys(ls.Dependencies) {
		if _, ok := members[ws]; !ok {
			continue
		}
		for _, dep := range sortedEntryKeys(ls.Dependencies[ws].Dependencies) {
			e := ls.Dependencies[ws].Dependencies[dep]
			if _, ok := members[dep]; !ok {
				c.external++
				continue
			}
			switch {
			case dep == ws:
			case e.Missing || e.Version == "":
				c.missing++
			case e.invalid():
				c.invalid++
			default:
				c.valid++
				out = append(out, edge{from: ws, to: dep})
			}
		}
	}
	return out, c
}

func sortedEntryKeys(m map[string]lsEntry) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// rewriteWorkspaceSpec turns a `workspace:` spec into one npm reads, and names
// the shape it had ("" when the spec is not a workspace: one). The protocol
// means the workspace package of that name: ^, ~ and * name its own version;
// workspace:<range> is the range itself. A path or an alias form is left as
// written (npm then lists the edge as missing) rather than guessed.
func rewriteWorkspaceSpec(spec, version string) (string, string) {
	rest, ok := strings.CutPrefix(spec, "workspace:")
	if !ok {
		return spec, ""
	}
	switch rest {
	case "^", "~", "*":
		if version == "" {
			return "*", rest
		}
		return version, rest
	}
	if rest == "" || strings.HasPrefix(rest, ".") || strings.HasPrefix(rest, "/") {
		return spec, "path"
	}
	if aliasSpec.MatchString(rest) {
		return spec, "alias"
	}
	return rest, "range"
}

var aliasSpec = regexp.MustCompile(`^@?[A-Za-z0-9._~-]+(/[A-Za-z0-9._~-]+)?@.+$`)

// expandWorkspaces is an independent reading of a `workspaces` array: the
// directories (of dirs, those holding a manifest) its patterns name, segment-
// wise as path.Match does, a leading "!" removing. A pattern with ** is left
// to the tool and counted.
func expandWorkspaces(dirs []string, hasManifest map[string]bool, patterns []string) ([]string, int) {
	picked := map[string]bool{}
	unexpanded := 0
	for _, p := range patterns {
		neg := strings.HasPrefix(p, "!")
		p = strings.TrimPrefix(p, "!")
		p = strings.TrimSuffix(strings.TrimPrefix(p, "./"), "/")
		if strings.Contains(p, "**") {
			unexpanded++
			continue
		}
		for _, d := range dirs {
			if !hasManifest[d] {
				continue
			}
			if ok, _ := path.Match(p, d); !ok {
				continue
			}
			if neg {
				delete(picked, d)
			} else {
				picked[d] = true
			}
		}
	}
	return sortedKeys(picked), unexpanded
}

// pnpmPatterns reads the `packages:` list of a pnpm-workspace.yaml (a block
// list of plain or quoted scalars; nothing else of the file).
func pnpmPatterns(src []byte) []string {
	var out []string
	in := false
	for _, ln := range strings.Split(string(src), "\n") {
		trim := strings.TrimRight(ln, " \t\r")
		switch {
		case strings.HasPrefix(trim, "packages:"):
			in = true
		case !in:
		case strings.HasPrefix(strings.TrimSpace(trim), "- "):
			v := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(trim), "- "))
			if len(v) > 0 && (v[0] == '\'' || v[0] == '"') {
				if end := strings.IndexByte(v[1:], v[0]); end >= 0 {
					v = v[1 : 1+end]
				}
			} else if h := strings.Index(v, " #"); h >= 0 {
				v = strings.TrimSpace(v[:h])
			}
			if v != "" {
				out = append(out, v)
			}
		case trim == "" || strings.HasPrefix(strings.TrimSpace(trim), "#"):
		default:
			in = false
		}
	}
	return out
}

// ── running npm ───────────────────────────────────────────────────────────

type runner struct {
	npm     string
	scratch string
	env     []string
}

func newRunner(npm, scratch string) (*runner, error) {
	// Two empty config files, one per level (npm refuses one file for both).
	userCfg, globalCfg := filepath.Join(scratch, "npmrc-user"), filepath.Join(scratch, "npmrc-global")
	for _, f := range []string{userCfg, globalCfg} {
		if err := os.WriteFile(f, nil, 0o644); err != nil {
			return nil, err
		}
	}
	var env []string
	for _, e := range os.Environ() {
		if strings.HasPrefix(strings.ToLower(e), "npm_config_") {
			continue
		}
		env = append(env, e)
	}
	env = append(env,
		"npm_config_userconfig="+userCfg, "npm_config_globalconfig="+globalCfg,
		"npm_config_cache="+filepath.Join(scratch, "cache"),
		"npm_config_offline=true", "npm_config_update_notifier=false",
		"npm_config_audit=false", "npm_config_fund=false", "npm_config_logs_max=0",
		"npm_config_loglevel=error", "npm_config_color=false", "NO_COLOR=1")
	return &runner{npm: npm, scratch: scratch, env: env}, nil
}

// run executes npm in dir and returns its stdout; a non-zero exit is not an
// error when stdout holds JSON (npm ls exits 1 for any unmet dependency).
func (r *runner) run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, r.npm, args...)
	cmd.Dir = dir
	cmd.Env = r.env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if err != nil && stdout.Len() == 0 {
		return nil, fmt.Errorf("npm %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(tail(stderr.String(), 800)))
	}
	return stdout.Bytes(), nil
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// ── the subject ───────────────────────────────────────────────────────────

func evalDir(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

// manifestsOf lists every package.json of the subject (subject-relative,
// slash-separated, sorted), but those under .git and node_modules.
func manifestsOf(subject string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(subject, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == "package.json" && d.Type().IsRegular() {
			rel, err := filepath.Rel(subject, p)
			if err != nil {
				return err
			}
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

func copyManifests(subject, dst string, rels []string) error {
	for _, rel := range rels {
		b, err := os.ReadFile(filepath.Join(subject, filepath.FromSlash(rel)))
		if err != nil {
			return err
		}
		p := filepath.Join(dst, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, b, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func dirOf(rel string) string {
	d := path.Dir(rel)
	if d == "." {
		return "."
	}
	return d
}

func decodeQuery(b []byte) ([]queryNode, error) {
	var nodes []queryNode
	if err := json.Unmarshal(b, &nodes); err != nil {
		return nil, err
	}
	return nodes, nil
}

type perManifest struct {
	rel     string
	src     []byte
	tool    toolSets
	idx     keyIndex
	idxErr  error
	decl    declared
	declErr error
	runErr  error
}

type crossCounts struct {
	unreadable                                      int
	nameless                                        int
	toolOnlyDefs, declOnlyDefs                      int
	toolOnlyRefs, declOnlyRefs                      int
	stringBin, nullExports, unlocated               int
	declaredDefs, declaredRefs                      int
	notes                                           []string
	spec                                            map[string]int
	declaredSibling, declaredSiblingNotListedByTool int
	memberDisagreeGlob, memberDisagreeNames         int
	pnpmPatterns, pnpmOnlyDirs                      int
}

func (c *crossCounts) note(format string, a ...any) {
	if len(c.notes) < 40 {
		c.notes = append(c.notes, fmt.Sprintf(format, a...))
	}
}

func diff(tool, decl map[string]bool) (toolOnly, declOnly []string) {
	for k := range tool {
		if !decl[k] {
			toolOnly = append(toolOnly, k)
		}
	}
	for k := range decl {
		if !tool[k] {
			declOnly = append(declOnly, k)
		}
	}
	sort.Strings(toolOnly)
	sort.Strings(declOnly)
	return
}

func main() {
	subjectFlag := flag.String("subject", "", "subject checkout (required)")
	scratchFlag := flag.String("scratch", "", "scratch directory outside the subject (required; this program empties <scratch>/npm-oracle)")
	npmFlag := flag.String("npm", "npm", "the npm binary")
	xrefOutFlag := flag.String("xref-out", "", "write the cross_references file here (required)")
	filesOutFlag := flag.String("files-out", "", "write the manifests npm evaluates as a workspace (the root and each member), one per line")
	jobs := flag.Int("j", 6, "parallel npm runs")
	wsPM := flag.String("workspace-pm", "", "yarn or pnpm: join the sibling edges with the subject's own package manager (pm.go), no spec rewritten; the cross-references are then tier A")
	pmBin := flag.String("pm", "", "with -workspace-pm: the node binary that runs the subject's yarn release (default node), or the pnpm binary (default pnpm)")
	flag.Parse()
	if *subjectFlag == "" || *scratchFlag == "" || *xrefOutFlag == "" {
		fmt.Fprintln(os.Stderr, "usage: npm-oracle -subject <checkout> -scratch <dir> -xref-out <file> [-files-out <file>] [-npm <npm>] [-j N]")
		os.Exit(2)
	}
	abs, err := filepath.Abs(*subjectFlag)
	if err != nil {
		fatal(err)
	}
	subject := evalDir(abs)
	scratchAbs, err := filepath.Abs(*scratchFlag)
	if err != nil {
		fatal(err)
	}
	if rel, err := filepath.Rel(subject, evalDir(scratchAbs)); err == nil && !strings.HasPrefix(rel, "..") {
		fatal(fmt.Errorf("-scratch %s is inside the subject; nothing may be written there", scratchAbs))
	}
	work := filepath.Join(evalDir(scratchAbs), "npm-oracle")
	if err := os.RemoveAll(work); err != nil {
		fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(work, "cache"), 0o755); err != nil {
		fatal(err)
	}
	ctx := context.Background()
	run, err := newRunner(*npmFlag, work)
	if err != nil {
		fatal(err)
	}
	vb, err := run.run(ctx, work, "--version")
	if err != nil {
		fatal(err)
	}
	version := strings.TrimSpace(string(vb))

	rels, err := manifestsOf(subject)
	if err != nil {
		fatal(err)
	}
	if len(rels) == 0 {
		fatal(fmt.Errorf("no package.json under %s", subject))
	}
	orig := filepath.Join(work, "orig")
	if err := copyManifests(subject, orig, rels); err != nil {
		fatal(err)
	}

	// Pass 1: npm's reading of every manifest.
	pm := make([]perManifest, len(rels))
	var wg sync.WaitGroup
	sem := make(chan struct{}, max(1, *jobs))
	for i, rel := range rels {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			p := &pm[i]
			p.rel = rel
			p.src, p.runErr = os.ReadFile(filepath.Join(subject, filepath.FromSlash(rel)))
			if p.runErr != nil {
				return
			}
			p.idx, p.idxErr = scanKeys(p.src)
			p.decl, p.declErr = declaredOf(p.src)
			dir := filepath.Join(orig, filepath.FromSlash(dirOf(rel)))
			b, err := run.run(ctx, dir, "query", ":root", "--prefix", dir)
			if err != nil {
				p.runErr = err
				return
			}
			nodes, err := decodeQuery(b)
			if err != nil || len(nodes) != 1 {
				p.runErr = fmt.Errorf("npm query :root at %s: %d nodes, %v", rel, len(nodes), err)
				return
			}
			p.tool = toolSetsOf(nodes[0])
		}()
	}
	wg.Wait()

	var cc crossCounts
	cc.spec = map[string]int{}
	var defs []def
	var refs []ref
	nDefKinds := map[string]int{}
	attested := 0
	for i := range pm {
		p := &pm[i]
		if p.runErr != nil {
			fatal(p.runErr)
		}
		if p.declErr != nil || p.idxErr != nil {
			// npm reads an unparseable manifest as an empty package and says
			// nothing; the decode says why, and the dialect extracts nothing.
			cc.unreadable++
			cc.note("%s: unreadable (%v): the tool lists nothing for it", p.rel, firstErr(p.declErr, p.idxErr))
			if p.tool.name != "" || len(p.tool.tasks)+len(p.tool.bin)+len(p.tool.exports)+len(p.tool.refs) > 0 {
				cc.note("%s: npm read a manifest the decode refuses; not attested", p.rel)
			}
			continue
		}
		attested++
		ds, unlocated := definitionsOf(p.rel, p.tool, p.idx)
		cc.unlocated += unlocated
		defs = append(defs, ds...)
		for _, d := range ds {
			nDefKinds[d.Kind]++
		}
		if p.tool.name == "" {
			cc.nameless++
		} else {
			for _, to := range sortedKeys(p.tool.refs) {
				refs = append(refs, ref{Path: p.rel, From: p.tool.name, To: to})
			}
		}
		cc.nullExports += p.tool.nullExports
		if p.decl.binString {
			cc.stringBin++
		}
		// The cross-check: the same families from the raw decode.
		cc.declaredDefs += len(p.decl.tasks) + len(p.decl.bin) + len(p.decl.exports)
		if p.decl.name != "" {
			cc.declaredDefs++
		}
		cc.declaredRefs += len(p.decl.refs)
		if p.tool.name != p.decl.name {
			cc.toolOnlyDefs++
			cc.note("%s: name npm %q, decode %q", p.rel, p.tool.name, p.decl.name)
		}
		for _, f := range []struct {
			fam        string
			tool, decl map[string]bool
		}{{"script", p.tool.tasks, p.decl.tasks}, {"export", p.tool.exports, p.decl.exports}} {
			to, do := diff(f.tool, f.decl)
			cc.toolOnlyDefs += len(to)
			cc.declOnlyDefs += len(do)
			for _, k := range to {
				cc.note("%s: %s %q only in the tool's reading", p.rel, f.fam, k)
			}
			for _, k := range do {
				cc.note("%s: %s %q only in the decode", p.rel, f.fam, k)
			}
		}
		if p.decl.binString {
			// npm names a string bin after the unscoped package; the decode
			// has no name for it, so the one entry is the tool's word.
			if len(p.tool.bin) != 1 {
				cc.declOnlyDefs++
				cc.note("%s: string bin, the tool lists %d bin entries", p.rel, len(p.tool.bin))
			}
		} else {
			to, do := diff(p.tool.bin, p.decl.bin)
			cc.toolOnlyDefs += len(to)
			cc.declOnlyDefs += len(do)
			for _, k := range to {
				cc.note("%s: bin %q only in the tool's reading", p.rel, k)
			}
			for _, k := range do {
				cc.note("%s: bin %q only in the decode", p.rel, k)
			}
		}
		to, do := diff(p.tool.refs, p.decl.refs)
		cc.toolOnlyRefs += len(to)
		cc.declOnlyRefs += len(do)
		for _, k := range to {
			cc.note("%s: dependency %q only in the tool's reading", p.rel, k)
		}
		for _, k := range do {
			cc.note("%s: dependency %q only in the decode", p.rel, k)
		}
	}

	// Pass 2: the workspaces npm evaluates, and the edges it draws between them.
	var xrefs []xref
	memberEdges, siblingN := 0, 0
	var sc siblingCounts
	tierX := "A"
	var pmC *pmCounts
	pmToolDesc := ""
	rootMF := "package.json"
	root := findManifest(pm, rootMF)
	members := map[string]member{}
	var memberList []member
	if root == nil {
		fmt.Fprintln(os.Stderr, "npm-oracle: the subject has no root package.json: no workspace pass")
	} else {
		linked := filepath.Join(work, "linked")
		if err := copyManifests(subject, linked, rels); err != nil {
			fatal(err)
		}
		b, err := run.run(ctx, linked, "query", "*", "--prefix", linked)
		if err != nil {
			fatal(err)
		}
		nodes, err := decodeQuery(b)
		if err != nil {
			fatal(fmt.Errorf("npm query * at the root: %v", err))
		}
		for _, n := range nodes {
			if n.Location == "" || strings.Contains(n.Location, "node_modules") {
				continue
			}
			name := stringOf(n.Name)
			if name == "" {
				cc.note("workspace %s has no name: npm names it after its directory, the dialect has no entity for it", n.Location)
				continue
			}
			m := member{name: name, version: stringOf(n.Version), dir: filepath.ToSlash(n.Location)}
			if prev, dup := members[name]; dup {
				fatal(fmt.Errorf("two workspaces named %q (%s, %s)", name, prev.dir, m.dir))
			}
			members[name] = m
			memberList = append(memberList, m)
		}
		sort.Slice(memberList, func(i, j int) bool { return memberList[i].dir < memberList[j].dir })

		// Cross-check 1: npm's other listing of the same globs.
		pb, err := run.run(ctx, linked, "pkg", "get", "name", "--workspaces", "--json", "--prefix", linked)
		if err != nil {
			fatal(err)
		}
		var listed map[string]json.RawMessage
		if err := json.Unmarshal(pb, &listed); err != nil {
			fatal(fmt.Errorf("npm pkg get --workspaces: %v", err))
		}
		for _, m := range memberList {
			if _, ok := listed[m.name]; !ok {
				cc.memberDisagreeNames++
				cc.note("npm query lists workspace %q, npm pkg get does not", m.name)
			}
		}
		for k := range listed {
			if _, ok := members[k]; !ok {
				cc.memberDisagreeNames++
				cc.note("npm pkg get lists workspace %q, npm query does not (nameless or outside the scan)", k)
			}
		}
		// Cross-check 2: an independent expansion of the root's globs.
		var rootDecl map[string]json.RawMessage
		_ = json.Unmarshal(root.src, &rootDecl)
		patterns := workspacePatterns(rootDecl["workspaces"])
		var dirs []string
		has := map[string]bool{}
		for _, rel := range rels {
			d := dirOf(rel)
			if d != "." {
				dirs = append(dirs, d)
				has[d] = true
			}
		}
		expanded, unexpanded := expandWorkspaces(dirs, has, patterns)
		toolDirs := map[string]bool{}
		for _, m := range memberList {
			toolDirs[m.dir] = true
		}
		if unexpanded == 0 {
			to, do := diff(toolDirs, toSet(expanded))
			cc.memberDisagreeGlob = len(to) + len(do)
			for _, d := range to {
				cc.note("workspace %s is the tool's, not the glob expansion's", d)
			}
			for _, d := range do {
				cc.note("workspace %s is the glob expansion's, not the tool's", d)
			}
		}
		// Disclosure: a pnpm-workspace.yaml's patterns, not evaluated by npm.
		if yb, err := os.ReadFile(filepath.Join(subject, "pnpm-workspace.yaml")); err == nil {
			pp := pnpmPatterns(yb)
			cc.pnpmPatterns = len(pp)
			pdirs, pun := expandWorkspaces(dirs, has, pp)
			for _, d := range pdirs {
				if !toolDirs[d] {
					cc.pnpmOnlyDirs++
					cc.note("pnpm-workspace.yaml names %s, a directory holding a manifest that npm did not list", d)
				}
			}
			if pun > 0 {
				cc.note("pnpm-workspace.yaml: %d patterns with ** left unexpanded", pun)
			}
		}

		var edges []edge
		if *wsPM != "" {
			// The subject's own package manager joins the siblings (pm.go).
			var listed []string
			var pc pmCounts
			edges, listed, pmToolDesc, pc, err = pmEdges(ctx, *wsPM, *pmBin, subject, work, rels, members)
			if err != nil {
				fatal(err)
			}
			to, do := diff(toSet(listed), toolDirs)
			pc.memberDisagree = len(to) + len(do)
			for _, d := range to {
				cc.note("%s lists workspace %s, npm does not", *wsPM, d)
			}
			for _, d := range do {
				cc.note("npm lists workspace %s, %s does not", d, *wsPM)
			}
			pmC = &pc
		} else {
			// The links `npm install` makes, and the protocol npm cannot read.
			for _, m := range memberList {
				link := filepath.Join(linked, "node_modules", filepath.FromSlash(m.name))
				if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
					fatal(err)
				}
				target, err := filepath.Rel(filepath.Dir(link), filepath.Join(linked, filepath.FromSlash(m.dir)))
				if err != nil {
					fatal(err)
				}
				if err := os.Symlink(target, link); err != nil {
					fatal(err)
				}
			}
			manifests := []string{rootMF}
			for _, m := range memberList {
				manifests = append(manifests, m.manifest())
			}
			for _, rel := range manifests {
				if err := rewriteManifest(filepath.Join(linked, filepath.FromSlash(rel)), members, cc.spec); err != nil {
					fatal(fmt.Errorf("%s: %v", rel, err))
				}
			}
			lb, err := run.run(ctx, linked, "ls", "--json", "--workspaces", "--prefix", linked)
			if err != nil {
				fatal(err)
			}
			var ls lsRoot
			if err := json.Unmarshal(lb, &ls); err != nil {
				fatal(fmt.Errorf("npm ls --workspaces: %v", err))
			}
			edges, sc = siblingEdges(ls, members)

			// Cross-check 3: the sibling names each member declares, from the raw decode.
			listedEdge := map[edge]bool{}
			for wsName, e := range ls.Dependencies {
				for dep := range e.Dependencies {
					listedEdge[edge{wsName, dep}] = true
				}
			}
			for _, m := range memberList {
				p := findManifest(pm, m.manifest())
				if p == nil {
					continue
				}
				for dep := range p.decl.refs {
					if _, sib := members[dep]; !sib || dep == m.name {
						continue
					}
					cc.declaredSibling++
					if !listedEdge[edge{m.name, dep}] {
						cc.declaredSiblingNotListedByTool++
						cc.note("%s declares a dependency on member %s that npm ls does not list", m.dir, dep)
					}
				}
			}
		}

		seen := map[xref]bool{}
		add := func(x xref) bool {
			if seen[x] {
				return false
			}
			seen[x] = true
			xrefs = append(xrefs, x)
			return true
		}
		if root.tool.name == "" {
			cc.note("the root package.json names no package: the dialect has no root entity, so no workspace edge can leave it")
		} else {
			for _, m := range memberList {
				if add(xref{Path: rootMF, From: root.tool.name, ToPath: m.manifest(), To: m.name}) {
					memberEdges++
				}
			}
		}
		for _, e := range edges {
			f, t := members[e.from], members[e.to]
			if add(xref{Path: f.manifest(), From: f.name, ToPath: t.manifest(), To: t.name}) {
				siblingN++
			}
		}
		for shape := range cc.spec {
			if shape != "path" && shape != "alias" {
				tierX = "C" // npm joined an edge whose spec this program rewrote
			}
		}
	}
	if *filesOutFlag != "" {
		var evaluated []string
		if root != nil {
			evaluated = append(evaluated, rootMF)
			for _, m := range memberList {
				evaluated = append(evaluated, m.manifest())
			}
		}
		if err := os.WriteFile(*filesOutFlag, []byte(strings.Join(evaluated, "\n")+"\n"), 0o644); err != nil {
			fatal(err)
		}
	}
	sort.Slice(xrefs, func(i, j int) bool {
		a, b := xrefs[i], xrefs[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.From != b.From {
			return a.From < b.From
		}
		return a.ToPath < b.ToPath
	})
	if xrefs == nil {
		xrefs = []xref{}
	}
	sort.Slice(defs, func(i, j int) bool {
		a, b := defs[i], defs[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.Kind < b.Kind
	})
	sort.Slice(refs, func(i, j int) bool {
		a, b := refs[i], refs[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.From != b.From {
			return a.From < b.From
		}
		return a.To < b.To
	})
	if defs == nil {
		defs = []def{}
	}
	if refs == nil {
		refs = []ref{}
	}

	mo := mainOut{Tool: "npm query ':root' (each manifest as npm loads it)", Version: version, Tier: "A", Definitions: defs, References: refs}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(mo); err != nil {
		fatal(err)
	}
	xo := xrefOut{Tool: "npm query + npm ls --workspaces (a scratch copy with the workspace links npm install makes)", Version: version, Tier: tierX, CrossReferences: xrefs}
	if pmC != nil {
		// The members are npm's evaluation of the root's workspaces; the sibling
		// joins are the subject's own package manager's, with no spec rewritten.
		xo.Tool = "npm query (workspace members) + " + pmToolDesc
		xo.Version = "npm " + version + "; " + strings.SplitN(pmToolDesc, " ", 3)[0] + " " + strings.SplitN(pmToolDesc, " ", 3)[1]
	}
	var xb bytes.Buffer
	xenc := json.NewEncoder(&xb)
	xenc.SetIndent("", "  ")
	if err := xenc.Encode(xo); err != nil {
		fatal(err)
	}
	if err := os.WriteFile(*xrefOutFlag, xb.Bytes(), 0o644); err != nil {
		fatal(err)
	}

	fmt.Fprintf(os.Stderr, "npm-oracle: npm %s, %d manifests (%d attested, %d unreadable), %d definitions (%d packages, %d tasks, %d routes), %d references, %d cross-references (%d workspace members, %d sibling dependency edges); tier %s\n",
		version, len(rels), attested, cc.unreadable, len(defs), nDefKinds["package"], nDefKinds["task"], nDefKinds["route"], len(refs), len(xrefs), memberEdges, siblingN, tierX)
	fmt.Fprintf(os.Stderr, "cross-check (encoding/json decode vs npm query): definitions only in the tool %d, only in the decode %d (of %d the decode declares); references only in the tool %d, only in the decode %d (of %d); string-valued bin (the one entry is the tool's) %d; null exports left out %d; nameless manifests %d; entries the scanner could not place %d\n",
		cc.toolOnlyDefs, cc.declOnlyDefs, cc.declaredDefs, cc.toolOnlyRefs, cc.declOnlyRefs, cc.declaredRefs, cc.stringBin, cc.nullExports, cc.nameless, cc.unlocated)
	if root != nil {
		fmt.Fprintf(os.Stderr, "workspaces: npm lists %d members; npm query vs npm pkg get --workspaces disagreements %d; vs an independent expansion of the root's globs %d; pnpm-workspace.yaml patterns %d, directories they name that npm did not list %d\n",
			len(memberList), cc.memberDisagreeNames, cc.memberDisagreeGlob, cc.pnpmPatterns, cc.pnpmOnlyDirs)
		var shapes []string
		for k, v := range cc.spec {
			shapes = append(shapes, fmt.Sprintf("%s:%d", k, v))
		}
		sort.Strings(shapes)
		fmt.Fprintf(os.Stderr, "workspace: protocol specs npm cannot read, by shape (path and alias forms are left as written): %s\n", strings.Join(shapes, " "))
		if pmC != nil {
			fmt.Fprintf(os.Stderr, "sibling dependencies (%s): %d edges; members it lists %d, disagreements with npm's members %d; descriptors naming a workspace whose range they do not accept %d; edges to a location no member holds %d\n",
				pmToolDesc, siblingN, pmC.listed, pmC.memberDisagree, pmC.mismatched, pmC.unknownTarget)
		} else {
			fmt.Fprintf(os.Stderr, "sibling dependencies: declared names %d; npm ls resolved %d, marked invalid %d, listed missing %d (a spec it cannot read), declared but not listed by the tool %d; external names %d\n",
				cc.declaredSibling, sc.valid, sc.invalid, sc.missing, cc.declaredSiblingNotListedByTool, sc.external)
		}
	}
	for _, n := range cc.notes {
		fmt.Fprintln(os.Stderr, "  note:", n)
	}
}

func firstErr(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}

func toSet(l []string) map[string]bool {
	m := map[string]bool{}
	for _, s := range l {
		m[s] = true
	}
	return m
}

func findManifest(pm []perManifest, rel string) *perManifest {
	for i := range pm {
		if pm[i].rel == rel {
			return &pm[i]
		}
	}
	return nil
}

// workspacePatterns reads a manifest's `workspaces`: an array, or an object
// whose `packages` is one.
func workspacePatterns(raw json.RawMessage) []string {
	var arr []string
	if json.Unmarshal(raw, &arr) == nil {
		return arr
	}
	var obj struct {
		Packages []string `json:"packages"`
	}
	if json.Unmarshal(raw, &obj) == nil {
		return obj.Packages
	}
	return nil
}

// rewriteManifest rewrites, in the scratch copy, each `workspace:` spec in the
// four dependency maps to one npm reads (rewriteWorkspaceSpec), counting by
// shape. The version is that of the member the dependency names.
func rewriteManifest(file string, members map[string]member, shapes map[string]int) error {
	b, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	var root map[string]any
	if err := json.Unmarshal(b, &root); err != nil {
		return nil // an unreadable manifest has no edges
	}
	changed := false
	for _, sec := range depSections {
		m, ok := root[sec].(map[string]any)
		if !ok {
			continue
		}
		for name, v := range m {
			spec, ok := v.(string)
			if !ok {
				continue
			}
			ver := ""
			if mem, ok := members[name]; ok {
				ver = mem.version
			}
			nspec, shape := rewriteWorkspaceSpec(spec, ver)
			if shape == "" {
				continue
			}
			shapes[shape]++
			if nspec != spec {
				m[name] = nspec
				changed = true
			}
		}
	}
	if !changed {
		return nil
	}
	nb, err := json.Marshal(root)
	if err != nil {
		return err
	}
	return os.WriteFile(file, nb, 0o644)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "npm-oracle:", err)
	os.Exit(1)
}
