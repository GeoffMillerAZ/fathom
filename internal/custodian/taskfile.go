package custodian

// taskfile.go — the Taskfile half of the chain (CC-11/CC-12): what targets
// a repo's Taskfile actually defines, parsed as YAML rather than grepped,
// because the grep this replaces for commands_resolve (`^\s+name:`) matched
// any indented key — a `vars:` entry named like a target read as the target
// existing. Includes are followed: a local include contributes its tasks
// under its namespace (or none, with `flatten: true`) and its file joins
// the taskfile corpus; a remote or templated include makes its namespace
// uncheckable, named — never assumed present, never assumed missing.
//
// Each loaded file is also a NODE carrying what Task itself uses to run
// it, because a Taskfile's commands mean different things in different
// files (probed against task 3.52):
//
//   - namespace: inside an include, `task: build` and `deps: [build]` call
//     `<ns>:build`; a leading ':' calls the root's; a flattened include
//     shares its parent's namespace.
//   - directory: an include runs in its includer's directory — NOT its own
//     — unless it sets `dir:` (joined to the including file's directory);
//     a task's `dir:` is joined to that.

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// taskGraph is one repo's resolved Taskfile set.
type taskGraph struct {
	files   []string            // repo-relative, sorted: root Taskfile + local includes
	nodes   []taskNode          // every file under every namespace it is included as, in load order
	targets map[string]struct{} // fully-qualified target names and aliases
	// opaque names namespaces whose include this audit cannot follow
	// (remote, templated, outside the repo): a ref into one is uncheckable.
	opaque map[string]string
	// broken names namespaces whose include points at a local file that
	// does not exist: a ref into one is a broken link, and so is the
	// include itself.
	broken map[string]string
}

// taskNode is one Taskfile as Task runs it: the file, the namespace its
// calls resolve in, and the directory its tasks run in.
type taskNode struct {
	file   string // repo-relative
	prefix string // namespace with its trailing ':' ("" at the root, or flattened into it)
	// dir is where its tasks run (before a task's own `dir:`), repo-relative
	// ("." is the root); dirUnknown says why it cannot be modelled
	// (templated, absolute, outside the repo) — a script under it is not
	// checkable.
	dir        string
	dirUnknown string
}

const maxTaskIncludeDepth = 8

// loadTaskGraph parses a Taskfile (repo-relative `first`, the one `task`
// runs from its directory) and its local includes.
func loadTaskGraph(root, first string) (*taskGraph, error) {
	g := &taskGraph{
		targets: map[string]struct{}{},
		opaque:  map[string]string{},
		broken:  map[string]string{},
	}
	top := taskNode{file: first, dir: path.Dir(first)}
	if err := g.load(root, top, map[string]bool{}, map[string]bool{}, 0, true); err != nil {
		return nil, err
	}
	sort.Strings(g.files)
	return g, nil
}

// load registers one Taskfile's targets under its node's prefix. visiting
// keys on (prefix, file) — the same file included under a namespace AND
// its aliases is read once per name it answers to — and the depth bound
// ends an include cycle (whose prefixes grow on every lap) as a named
// opaque namespace rather than a hang. record is false under an alias: the
// alias names the same tasks, so its commands are not read twice.
func (g *taskGraph) load(root string, node taskNode, visiting, listed map[string]bool, depth int, record bool) error {
	rel, prefix := node.file, node.prefix
	key := prefix + "\x00" + rel
	if visiting[key] {
		return nil
	}
	visiting[key] = true
	if !listed[rel] {
		listed[rel] = true
		g.files = append(g.files, rel)
	}
	if record {
		g.nodes = append(g.nodes, node)
	}

	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return fmt.Errorf("%s unreadable: %w", rel, err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("%s does not parse as YAML: %w", rel, err)
	}
	top := documentMapping(&doc)
	if top == nil {
		return nil // an empty Taskfile defines nothing — refs into it are violations, not errors
	}

	if tasks := mappingValue(top, "tasks"); tasks != nil && tasks.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(tasks.Content); i += 2 {
			name := tasks.Content[i].Value
			g.targets[prefix+name] = struct{}{}
			if aliases := mappingValue(tasks.Content[i+1], "aliases"); aliases != nil {
				for _, a := range aliases.Content {
					if a.Kind == yaml.ScalarNode && a.Value != "" {
						g.targets[prefix+a.Value] = struct{}{}
					}
				}
			}
		}
	}

	includes := mappingValue(top, "includes")
	if includes == nil || includes.Kind != yaml.MappingNode {
		return nil
	}
	dir := filepath.ToSlash(filepath.Dir(filepath.FromSlash(rel)))
	for i := 0; i+1 < len(includes.Content); i += 2 {
		ns := includes.Content[i].Value
		spec := includes.Content[i+1]
		incPath, incDir, optional, flatten, aliases := includeSpec(spec)
		prefixes := []string{prefix + ns + ":"}
		for _, a := range aliases {
			prefixes = append(prefixes, prefix+a+":")
		}
		if flatten {
			prefixes = []string{prefix}
		}
		mark := func(m map[string]string, why string) {
			for _, p := range prefixes {
				if p != prefix { // a flattened include has no namespace to mark
					m[strings.TrimSuffix(p, ":")] = why
				}
			}
		}
		if incPath == "" || strings.Contains(incPath, "{{") || strings.Contains(incPath, "://") || strings.HasPrefix(incPath, "~") {
			mark(g.opaque, fmt.Sprintf("include %q is remote or templated (%s)", ns, incPath))
			continue
		}
		target := incPath
		if !filepath.IsAbs(target) {
			target = filepath.Join(root, filepath.FromSlash(dir), filepath.FromSlash(incPath))
		}
		relTarget, err := filepath.Rel(root, target)
		if err != nil || strings.HasPrefix(relTarget, "..") {
			mark(g.opaque, fmt.Sprintf("include %q points outside the repo (%s)", ns, incPath))
			continue
		}
		fileRel, err := taskfileAt(root, filepath.ToSlash(relTarget))
		if err != nil {
			return err
		}
		if fileRel == "" {
			if !optional {
				mark(g.broken, fmt.Sprintf("%s includes %q → %s, which does not exist", rel, ns, incPath))
			}
			continue
		}
		if depth+1 >= maxTaskIncludeDepth {
			mark(g.opaque, fmt.Sprintf("include %q nests deeper than %d levels", ns, maxTaskIncludeDepth))
			continue
		}
		child := taskNode{file: fileRel, dir: node.dir, dirUnknown: node.dirUnknown}
		if incDir != "" {
			child.dir, child.dirUnknown = joinRunDir(dir, incDir, "include "+strconv.Quote(ns)+" dir")
		}
		for j, p := range prefixes {
			child.prefix = p
			if err := g.load(root, child, visiting, listed, depth+1, record && j == 0); err != nil {
				return err
			}
		}
	}
	return nil
}

// joinRunDir joins a working-directory value (a Taskfile `dir:`, lefthook
// `root:`, a CI `working-directory:`) to base, both repo-relative. A value
// this audit cannot model — templated, absolute, home-relative, or leaving
// the repo — returns why instead, and every ref under it is not checkable.
func joinRunDir(base, val, what string) (dir, unknown string) {
	switch {
	case strings.ContainsAny(val, "{}$`~") || strings.Contains(val, "%"):
		return "", fmt.Sprintf("%s %q is templated", what, val)
	case filepath.IsAbs(val) || strings.HasPrefix(val, "/"):
		return "", fmt.Sprintf("%s %q is absolute", what, val)
	}
	j := path.Clean(path.Join(base, filepath.ToSlash(val)))
	if escapesRepo(j) {
		return "", fmt.Sprintf("%s %q leaves the repo", what, val)
	}
	return j, ""
}

// escapesRepo reports a cleaned repo-relative slash path that leaves the
// repo.
func escapesRepo(p string) bool {
	return p == ".." || strings.HasPrefix(p, "../") || path.IsAbs(p)
}

// includeSpec reads an include entry: a bare path, or a mapping with
// taskfile / dir / optional / flatten / aliases.
func includeSpec(n *yaml.Node) (file, dir string, optional, flatten bool, aliases []string) {
	switch n.Kind {
	case yaml.ScalarNode:
		return n.Value, "", false, false, nil
	case yaml.MappingNode:
		if v := mappingValue(n, "taskfile"); v != nil {
			file = v.Value
		}
		if v := mappingValue(n, "dir"); v != nil && v.Kind == yaml.ScalarNode {
			dir = v.Value
		}
		if v := mappingValue(n, "optional"); v != nil {
			optional = v.Value == "true"
		}
		if v := mappingValue(n, "flatten"); v != nil {
			flatten = v.Value == "true"
		}
		if v := mappingValue(n, "aliases"); v != nil {
			for _, a := range v.Content {
				if a.Kind == yaml.ScalarNode && a.Value != "" {
					aliases = append(aliases, a.Value)
				}
			}
		}
	}
	return file, dir, optional, flatten, aliases
}

// taskfileAt resolves an include target: a file as named, or a directory
// holding one of the root spellings. "" when neither exists.
func taskfileAt(root, rel string) (string, error) {
	abs := filepath.Join(root, filepath.FromSlash(rel))
	fi, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	if !fi.IsDir() {
		return filepath.ToSlash(filepath.Clean(rel)), nil
	}
	for _, name := range taskfileNames {
		ok, err := present(filepath.Join(abs, name))
		if err != nil {
			return "", err
		}
		if ok {
			return filepath.ToSlash(filepath.Join(filepath.Clean(rel), name)), nil
		}
	}
	return "", nil
}

// taskfileIn returns the Taskfile `task` would run in dir (repo-relative),
// in Task's own spelling order — "" when dir has none. An unstattable
// candidate is an error, never "none" (I38).
func taskfileIn(root, dir string) (string, error) {
	for _, name := range taskfileNames {
		rel := path.Join(dir, name)
		ok, err := present(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return "", err
		}
		if ok {
			return rel, nil
		}
	}
	return "", nil
}

// resolveTarget answers one `task NAME` ref: "" when the target exists,
// else why not; uncheckable reports a ref this audit cannot judge.
func (g *taskGraph) resolveTarget(name string) (missing string, uncheckable string) {
	if _, ok := g.targets[name]; ok {
		return "", ""
	}
	// Longest namespace first, so `a:b:c` under an opaque `a:b` wins over `a`.
	parts := strings.Split(name, ":")
	for i := len(parts) - 1; i >= 1; i-- {
		ns := strings.Join(parts[:i], ":")
		if why, ok := g.opaque[ns]; ok {
			return "", why
		}
		if why, ok := g.broken[ns]; ok {
			return why, ""
		}
	}
	return "no such target", ""
}

// documentMapping unwraps a parsed YAML document to its top mapping.
func documentMapping(doc *yaml.Node) *yaml.Node {
	n := doc
	if n.Kind == yaml.DocumentNode {
		if len(n.Content) == 0 {
			return nil
		}
		n = n.Content[0]
	}
	if n.Kind != yaml.MappingNode {
		return nil
	}
	return n
}

// mappingValue returns the value node for key in a mapping (nil when the
// node is not a mapping or the key is absent).
func mappingValue(n *yaml.Node, key string) *yaml.Node {
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
