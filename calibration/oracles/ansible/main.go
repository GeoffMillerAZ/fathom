// Command ansible-oracle is the language-breadth harness's oracle for the
// ansible doc-engine dialect (design §1.3, owner decision D4): a from-scratch
// gopkg.in/yaml.v3 walk of a subject's Ansible files, never fathom's doc engine,
// written in indexbench lang-oracle's tool-oracle shape, tier C. With -ansible
// the tools themselves are run as an acceptance and name cross-check (tier A, never
// scored: neither prints a line or an include edge):
//
//		go run . -subject <checkout> [-ansible-playbook <bin>] [-ansible-inventory <bin>] > <out.json>
//
//	  - `ansible-playbook --syntax-check -i localhost,` on each playbook file (the
//	    file's top level is a sequence of plays) and `ansible-playbook --list-tasks`
//	    on those it accepts: every play and task name the tool lists that is not a
//	    template is looked up among the definitions the walk wrote (counted on
//	    stderr: found of listed, and the playbooks the tool rejects, almost always for
//	    a role or collection it cannot find: the subjects are never installed).
//	  - `ansible-inventory --list -i <file>` on each YAML inventory: the groups and
//	    hosts it lists are looked up among the walk's.
//
// No play is ever run (`--list-tasks` and `--syntax-check` evaluate nothing on a
// host) and nothing is fetched. ANSIBLE_HOME, ANSIBLE_LOCAL_TEMP and ANSIBLE_CONFIG
// are the caller's, outside the subject.
//
// A file is an Ansible file by the rules fathom's classifier applies, written here
// again from the same statement and last of all (a file another signal claims, a
// Kubernetes object, a Compose file or an OpenAPI description, is not): a path
// roles/<r>/{tasks,handlers,defaults,vars,meta}/*.yml, group_vars/ or host_vars/, an
// inventory/ file whose top-level all: has hosts or children, or a top-level
// sequence whose items carry hosts: or import_playbook:; and, beyond the roles/
// layout, a file under the tasks, handlers, defaults, vars or meta directory of a
// role root (a directory holding tasks/main.yml, as a standalone role's repository
// does), and a YAML sequence of maps under a tasks/ or handlers/ directory (the
// task files a playbook imports).
//
//   - definitions: `package` per file (named by the file); `task` per item of a
//     task list (a file's top-level items, a play's tasks, pre_tasks and
//     post_tasks, and every block, rescue and always list) named by `name`, else a
//     play's `hosts`; `rule` per play handler (named by `name`, and by `listen`);
//     `service` per role a play's roles: or an include_role/import_role names and per
//     meta dependency; `import` per include_tasks, import_tasks, include_vars and
//     import_playbook (short and ansible.builtin.* FQCN forms), named by the path;
//     `variable_decl` per top-level key of a mapping file and per key of a play's
//     vars; `section` per inventory group (a children: entry) and `element` per host
//     (a hosts: entry).
//   - references: a task's notify to a handler of the same file (declared_only).
//   - cross_references: an include, a vars_files entry or an import_playbook to the
//     Ansible file its path names (relative to the referring file's directory); a
//     role to roles/<name>/tasks/main.yml (next to the referring file, or, for a
//     file inside roles/<r>/, beside that role); a notify to a handler of that
//     name in another file of the same role (or, outside roles, the one other
//     file of the subject that declares it). A name that resolves nowhere (a
//     collection's role, a templated path, a task file no signal claims) is
//     counted on stderr and is in no truth.
//
// An INI inventory is not YAML and no dialect decodes it: its groups and hosts
// are counted on stderr as a disclosed gap.
//
// Bench-only; never a product dependency.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
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
	To     string `json:"to"`
}

type out struct {
	Tool            string `json:"tool"`
	Version         string `json:"version"`
	Tier            string `json:"tier"`
	Definitions     []def  `json:"definitions"`
	References      []ref  `json:"references"`
	CrossReferences []xref `json:"cross_references"`
}

func get(n *yaml.Node, key string) *yaml.Node {
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

func keyLine(n *yaml.Node, key string) int {
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i].Line
		}
	}
	return 0
}

func scalar(n *yaml.Node) string {
	if n == nil || n.Kind != yaml.ScalarNode || n.Tag == "!!null" {
		return ""
	}
	return n.Value
}

func items(n *yaml.Node) []*yaml.Node {
	switch {
	case n == nil:
		return nil
	case n.Kind == yaml.SequenceNode:
		return n.Content
	case n.Kind == yaml.MappingNode:
		return []*yaml.Node{n}
	case n.Kind == yaml.ScalarNode && n.Tag != "!!null":
		return []*yaml.Node{n} // a `[*]` step reads a scalar as the one item it is
	}
	return nil
}

var (
	roleFileRe  = regexp.MustCompile(`(?:^|/)roles/([^/]+)/(?:tasks|handlers|defaults|vars|meta)/(?:[^/]+/)*[^/]+\.ya?ml$`)
	varsFileRe  = regexp.MustCompile(`(?:^|/)(?:group_vars|host_vars)/(?:[^/]+/)*[^/]+\.ya?ml$`)
	includeKeys = []string{"import_tasks", "include_tasks", "include_vars", "ansible.builtin.import_tasks", "ansible.builtin.include_tasks", "ansible.builtin.include_vars"}
	playKeys    = []string{"import_playbook", "ansible.builtin.import_playbook"}
	roleKeys    = []string{"include_role", "import_role", "ansible.builtin.include_role", "ansible.builtin.import_role"}
)

func has(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

var (
	taskDirRe   = regexp.MustCompile(`(?:^|/)(?:tasks|handlers)/(?:[^/]+/)*[^/]+\.ya?ml$`)
	roleSubdirs = map[string]bool{"tasks": true, "handlers": true, "defaults": true, "vars": true, "meta": true}
)

// roleRootOf is the role root rel sits under (a directory holding tasks/main.yml,
// rel in its tasks, handlers, defaults, vars or meta directory), "" for none.
func roleRootOf(rel string, roots map[string]bool) string {
	segs := strings.Split(rel, "/")
	best := ""
	for i := 0; i < len(segs)-1; i++ {
		if !roleSubdirs[segs[i]] {
			continue
		}
		r := "."
		if i > 0 {
			r = strings.Join(segs[:i], "/")
		}
		if roots[r] {
			best = r
		}
	}
	return best
}

// isAnsible applies the signals; root is the file's first document.
func isAnsible(rel string, root *yaml.Node, roleRoots map[string]bool) bool {
	if strings.Contains(rel, ".github/workflows/") {
		return false
	}
	// Another tool's file is not Ansible's: a Kubernetes object, a Compose file, an
	// OpenAPI description, a GitLab CI pipeline.
	if root.Kind == yaml.MappingNode {
		if get(root, "apiVersion") != nil && get(root, "kind") != nil {
			return false
		}
		if s := get(root, "services"); s != nil && s.Kind != yaml.SequenceNode {
			return false
		}
		if get(root, "openapi") != nil || get(root, "swagger") != nil {
			return false
		}
	}
	if path.Base(rel) == ".gitlab-ci.yml" {
		return false
	}
	if roleFileRe.MatchString(rel) || varsFileRe.MatchString(rel) || roleRootOf(rel, roleRoots) != "" {
		return true
	}
	// A task or handler file: a YAML sequence of maps under a tasks/ or handlers/ directory.
	if taskDirRe.MatchString(rel) && root.Kind == yaml.SequenceNode {
		for _, it := range root.Content {
			if it.Kind == yaml.MappingNode && len(it.Content) > 0 {
				return true
			}
		}
	}
	segs := strings.Split(rel, "/")
	inInv := false
	for _, s := range segs[:len(segs)-1] {
		if s == "inventory" || s == "inventories" {
			inInv = true
		}
	}
	if inInv && root.Kind == yaml.MappingNode {
		if all := get(root, "all"); all != nil && all.Kind == yaml.MappingNode && (get(all, "hosts") != nil || get(all, "children") != nil) {
			return true
		}
	}
	if root.Kind == yaml.SequenceNode {
		for _, it := range root.Content {
			if it.Kind == yaml.MappingNode {
				for _, k := range append([]string{"hosts"}, playKeys...) {
					if get(it, k) != nil {
						return true
					}
				}
			}
		}
	}
	return false
}

type include struct {
	path, from, raw string
	kind            string // include | role | dependency | vars_file
}

type notify struct {
	path, from, to string
}

type fileInfo struct {
	rel      string
	base     string
	root     *yaml.Node
	role     string   // roles/<r>/ prefix when inside a role, else ""
	handlers []string // handler names declared in the file (play handlers; role handler-file tasks)
}

func main() {
	subject := flag.String("subject", "", "subject checkout")
	apb := flag.String("ansible-playbook", "", "ansible-playbook binary for the --syntax-check and --list-tasks cross-check (optional)")
	ainv := flag.String("ansible-inventory", "", "ansible-inventory binary for the --list cross-check (optional)")
	flag.Parse()
	if *subject == "" {
		fmt.Fprintln(os.Stderr, "usage: ansible-oracle -subject <dir> [-ansible-playbook <bin>] [-ansible-inventory <bin>]")
		os.Exit(2)
	}
	var files []string
	iniInventories := 0
	err := filepath.WalkDir(*subject, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(*subject, p)
		rel = filepath.ToSlash(rel)
		switch strings.ToLower(path.Ext(rel)) {
		case ".yaml", ".yml":
			files = append(files, rel)
		case "":
			if d.Name() == "inventory" || d.Name() == "hosts" {
				iniInventories++
			}
		}
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "ansible-oracle:", err)
		os.Exit(1)
	}
	sort.Strings(files)
	roleRoots := map[string]bool{}
	for _, rel := range files {
		if b := path.Base(rel); (b == "main.yml" || b == "main.yaml") && path.Base(path.Dir(rel)) == "tasks" {
			roleRoots[path.Dir(path.Dir(rel))] = true
		}
	}
	o := out{Tool: "ansible-oracle (independent gopkg.in/yaml.v3 walk)", Version: "gopkg.in/yaml.v3 v3.0.1", Tier: "C",
		Definitions: []def{}, References: []ref{}, CrossReferences: []xref{}}
	byRel := map[string]*fileInfo{}
	var order []*fileInfo
	undecodable := 0
	for _, rel := range files {
		b, err := os.ReadFile(filepath.Join(*subject, filepath.FromSlash(rel)))
		if err != nil {
			fmt.Fprintln(os.Stderr, "ansible-oracle:", err)
			os.Exit(1)
		}
		root, ok := decode(b)
		if !ok {
			undecodable++
			continue
		}
		if !isAnsible(rel, root, roleRoots) {
			continue
		}
		fi := &fileInfo{rel: rel, base: path.Base(rel), root: root}
		if m := roleFileRe.FindStringSubmatch(rel); m != nil {
			fi.role = rel[:strings.Index(rel, "roles/"+m[1]+"/")+len("roles/"+m[1])]
		}
		if r := roleRootOf(rel, roleRoots); r != "" {
			fi.role = r
		}
		byRel[rel] = fi
		order = append(order, fi)
	}
	var incs []include
	var notifies []notify
	seenRef := map[ref]bool{}
	for _, fi := range order {
		walk(&o, fi, &incs, &notifies, seenRef)
	}
	// Cross-file references.
	handlersIn := map[string]map[string]bool{} // name -> files declaring a handler of it
	for _, fi := range order {
		for _, h := range fi.handlers {
			if handlersIn[h] == nil {
				handlersIn[h] = map[string]bool{}
			}
			handlersIn[h][fi.rel] = true
		}
	}
	seenX := map[xref]bool{}
	addX := func(from, fromName, to, toName string) {
		t := byRel[to]
		if t == nil || to == from {
			return
		}
		if toName == "" {
			toName = t.base // an include or role names the file: its first entity is the file's
		}
		x := xref{from, fromName, to, toName}
		if !seenX[x] {
			seenX[x] = true
			o.CrossReferences = append(o.CrossReferences, x)
		}
	}
	templated, notFound, unclaimed, collections := 0, 0, 0, 0
	for _, in := range incs {
		fi := byRel[in.path]
		if in.raw == "" {
			continue
		}
		switch in.kind {
		case "include", "vars_file":
			if strings.Contains(in.raw, "{{") || strings.Contains(in.raw, "{%") {
				templated++
				continue
			}
			t := repoPath(in.path, in.raw)
			switch {
			case t == "":
				notFound++
			case byRel[t] != nil:
				addX(in.path, in.from, t, "")
			default:
				if _, err := os.Stat(filepath.Join(*subject, filepath.FromSlash(t))); err == nil {
					unclaimed++
				} else {
					notFound++
				}
			}
		case "role", "dependency":
			if strings.Contains(in.raw, "{{") {
				templated++
				continue
			}
			cands := []string{repoPath(in.path, "roles/"+in.raw+"/tasks/main.yml")}
			if fi.role != "" {
				cands = append(cands, path.Join(path.Dir(fi.role), in.raw, "tasks/main.yml"))
			}
			hit := false
			for _, c := range cands {
				if c != "" && byRel[c] != nil {
					addX(in.path, in.from, c, "")
					hit = true
				}
			}
			if !hit {
				if strings.Count(in.raw, ".") >= 1 {
					collections++
				} else {
					notFound++
				}
			}
		}
	}
	for _, n := range notifies {
		fi := byRel[n.path]
		var hit []string
		for f := range handlersIn[n.to] {
			if f == n.path {
				continue
			}
			if fi.role != "" && byRel[f].role != fi.role {
				continue
			}
			hit = append(hit, f)
		}
		sort.Strings(hit)
		if len(hit) == 1 {
			addX(n.path, n.from, hit[0], n.to)
		}
	}
	sort.Slice(o.CrossReferences, func(i, j int) bool {
		a, b := o.CrossReferences[i], o.CrossReferences[j]
		return a.Path+"\x00"+a.From+"\x00"+a.ToPath+"\x00"+a.To < b.Path+"\x00"+b.From+"\x00"+b.ToPath+"\x00"+b.To
	})
	fmt.Fprintf(os.Stderr, "ansible-oracle: %d ansible files, %d undecodable; %d definitions; %d cross-file edges; unresolved: %d templated, %d absent, %d to a file no signal claims, %d collection roles; %d INI inventories (not YAML, not read)\n",
		len(order), undecodable, len(o.Definitions), len(o.CrossReferences), templated, notFound, unclaimed, collections, iniInventories)
	if *apb != "" {
		crossCheckPlaybooks(*apb, *subject, order, &o)
	}
	if *ainv != "" {
		crossCheckInventories(*ainv, *subject, order, &o)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	if err := enc.Encode(o); err != nil {
		os.Exit(1)
	}
}

// walk writes fi's definitions and references.
func walk(o *out, fi *fileInfo, incs *[]include, notifies *[]notify, seenRef map[ref]bool) {
	root := fi.root
	o.Definitions = append(o.Definitions, def{fi.rel, fi.base, root.Line, "package"})
	addDef := func(name string, line int, kind string) {
		if name != "" {
			o.Definitions = append(o.Definitions, def{fi.rel, name, line, kind})
		}
	}
	// handlers declared in the file, for declared_only and the role scope.
	var playHandlers []string
	var taskNotifies []notify
	if root.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(root.Content); i += 2 {
			addDef(root.Content[i].Value, root.Content[i].Line, "variable_decl")
		}
		for _, d := range items(get(root, "dependencies")) {
			name := scalar(d)
			if name == "" {
				name = scalar(get(d, "role"))
			}
			if name == "" {
				name = scalar(get(d, "name"))
			}
			addDef(name, d.Line, "service")
			*incs = append(*incs, include{path: fi.rel, from: name, raw: name, kind: "dependency"})
		}
		descend(root, func(key string, v *yaml.Node, k *yaml.Node) {
			if v.Kind != yaml.MappingNode {
				return
			}
			switch key {
			case "children":
				for i := 0; i+1 < len(v.Content); i += 2 {
					addDef(v.Content[i].Value, v.Content[i].Line, "section")
				}
			case "hosts":
				for i := 0; i+1 < len(v.Content); i += 2 {
					addDef(v.Content[i].Value, v.Content[i].Line, "element")
				}
			}
		})
	}
	if root.Kind == yaml.SequenceNode {
		// inventory-style hosts/children inside a sequence file are read the same way
		descend(root, func(key string, v *yaml.Node, k *yaml.Node) {
			if v.Kind != yaml.MappingNode {
				return
			}
			switch key {
			case "children":
				for i := 0; i+1 < len(v.Content); i += 2 {
					addDef(v.Content[i].Value, v.Content[i].Line, "section")
				}
			case "hosts":
				for i := 0; i+1 < len(v.Content); i += 2 {
					addDef(v.Content[i].Value, v.Content[i].Line, "element")
				}
			}
		})
	}
	// taskItem reads one item of a task list.
	var taskItem func(it *yaml.Node, top bool, handler bool)
	taskItem = func(it *yaml.Node, top, handler bool) {
		if it.Kind != yaml.MappingNode {
			return
		}
		name := scalar(get(it, "name"))
		if !handler {
			label := name
			if label == "" {
				label = scalar(get(it, "hosts"))
			}
			addDef(label, it.Line, "task")
			if label != "" {
				for _, n := range items(get(it, "notify")) {
					if to := scalar(n); to != "" {
						taskNotifies = append(taskNotifies, notify{fi.rel, label, to})
					}
				}
			}
		}
		for i := 0; i+1 < len(it.Content); i += 2 {
			k, v := it.Content[i].Value, it.Content[i+1]
			switch {
			case has(includeKeys, k) || (top && has(playKeys, k)):
				n := scalar(v)
				if n == "" {
					n = scalar(get(v, "file"))
				}
				addDef(n, it.Content[i].Line, "import")
				kind := "include"
				*incs = append(*incs, include{path: fi.rel, from: n, raw: n, kind: kind})
			case has(roleKeys, k):
				n := scalar(get(v, "name"))
				if n == "" {
					n = scalar(v)
				}
				addDef(n, it.Content[i].Line, "service")
				*incs = append(*incs, include{path: fi.rel, from: n, raw: n, kind: "role"})
			}
		}
		for _, k := range []string{"block", "rescue", "always"} {
			for _, sub := range items(get(it, k)) {
				taskItem(sub, false, false)
			}
		}
	}
	if root.Kind == yaml.SequenceNode || root.Kind == yaml.MappingNode {
		for _, it := range items(root) {
			if root.Kind == yaml.MappingNode {
				break // a mapping file's keys are variables, not a play
			}
			taskItem(it, true, false)
			if it.Kind != yaml.MappingNode {
				continue
			}
			for _, k := range []string{"tasks", "pre_tasks", "post_tasks"} {
				for _, t := range items(get(it, k)) {
					taskItem(t, false, false)
				}
			}
			for _, h := range items(get(it, "handlers")) {
				if h.Kind != yaml.MappingNode {
					continue
				}
				if n := scalar(get(h, "name")); n != "" {
					addDef(n, h.Line, "rule")
					playHandlers = append(playHandlers, n)
				}
				if l := scalar(get(h, "listen")); l != "" {
					addDef(l, h.Line, "rule")
					playHandlers = append(playHandlers, l)
				}
				taskItem(h, false, true)
			}
			for _, r := range items(get(it, "roles")) {
				name := scalar(r)
				if name == "" {
					name = scalar(get(r, "role"))
				}
				if name == "" {
					name = scalar(get(r, "name"))
				}
				addDef(name, r.Line, "service")
				*incs = append(*incs, include{path: fi.rel, from: name, raw: name, kind: "role"})
			}
			if vars := get(it, "vars"); vars != nil && vars.Kind == yaml.MappingNode {
				for i := 0; i+1 < len(vars.Content); i += 2 {
					addDef(vars.Content[i].Value, vars.Content[i].Line, "variable_decl")
				}
			}
			for _, vf := range items(get(it, "vars_files")) {
				if n := scalar(vf); n != "" {
					*incs = append(*incs, include{path: fi.rel, from: fi.base, raw: n, kind: "vars_file"})
				}
			}
		}
	}
	// A role's handlers file lists tasks; each top-level name is a handler.
	if strings.Contains(fi.rel, "/handlers/") || strings.HasPrefix(fi.rel, "handlers/") {
		if root.Kind == yaml.SequenceNode {
			for _, it := range root.Content {
				if n := scalar(get(it, "name")); n != "" {
					playHandlers = append(playHandlers, n)
				}
			}
		}
	}
	fi.handlers = playHandlers
	declared := map[string]bool{}
	for _, h := range playHandlers {
		declared[h] = true
	}
	hasPlayHandlers := root.Kind == yaml.SequenceNode
	for _, n := range taskNotifies {
		r := ref{n.path, n.from, n.to}
		if hasPlayHandlers && declaredInPlay(root, n.to) && !seenRef[r] {
			seenRef[r] = true
			o.References = append(o.References, r)
		}
		*notifies = append(*notifies, n)
	}
}

// declaredInPlay: a play of the file declares a handler named (or listened to as) n.
func declaredInPlay(root *yaml.Node, n string) bool {
	for _, it := range root.Content {
		for _, h := range items(get(it, "handlers")) {
			if scalar(get(h, "name")) == n || scalar(get(h, "listen")) == n {
				return true
			}
		}
	}
	return false
}

// descend calls f for every map entry at any depth, with its key and value.
func descend(n *yaml.Node, f func(key string, v *yaml.Node, k *yaml.Node)) {
	switch n.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			f(n.Content[i].Value, n.Content[i+1], n.Content[i])
			descend(n.Content[i+1], f)
		}
	case yaml.SequenceNode:
		for _, c := range n.Content {
			descend(c, f)
		}
	}
}

// repoPath resolves raw against the directory of file, as a path inside the
// repository; "" when it climbs out.
func repoPath(file, raw string) string {
	raw = strings.ReplaceAll(raw, `\`, "/")
	var p string
	if strings.HasPrefix(raw, "/") {
		p = path.Clean(strings.TrimPrefix(raw, "/"))
	} else {
		p = path.Clean(path.Join(path.Dir(file), raw))
	}
	if p == ".." || strings.HasPrefix(p, "../") || p == "." {
		return ""
	}
	return p
}

// decode returns the first document's root.
func decode(b []byte) (*yaml.Node, bool) {
	dec := yaml.NewDecoder(bytes.NewReader(b))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		if err == io.EOF {
			return nil, false
		}
		return nil, false
	}
	if len(doc.Content) == 0 || (doc.Content[0].Kind == yaml.ScalarNode && doc.Content[0].Tag == "!!null") {
		return nil, false // no document: a comment-only file reads as null
	}
	return doc.Content[0], true
}

func run(bin string, env []string, dir string, args ...string) (string, error) {
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	var b bytes.Buffer
	cmd.Stdout = &b
	cmd.Stderr = &b
	err := cmd.Run()
	return b.String(), err
}

var nameLineRe = regexp.MustCompile(`^\s+(?:play #\d+ \([^)]*\): )?(.*?)\s+TAGS: \[.*\]\s*$`)

// crossCheckPlaybooks runs the tools on each playbook file (a sequence whose
// items have hosts:); counts on stderr.
func crossCheckPlaybooks(bin, subject string, order []*fileInfo, o *out) {
	names := map[string]bool{}
	for _, d := range o.Definitions {
		if d.Kind == "task" || d.Kind == "rule" {
			names[d.Name] = true
		}
	}
	playbooks, accepted, rejected, listed, found, templated := 0, 0, 0, 0, 0, 0
	for _, fi := range order {
		if fi.root.Kind != yaml.SequenceNode {
			continue
		}
		isPlay := false
		for _, it := range fi.root.Content {
			if get(it, "hosts") != nil {
				isPlay = true
			}
		}
		if !isPlay {
			continue
		}
		playbooks++
		dir := filepath.Join(subject, filepath.FromSlash(path.Dir(fi.rel)))
		if _, err := run(bin, nil, dir, "--syntax-check", "-i", "localhost,", path.Base(fi.rel)); err != nil {
			rejected++
			continue
		}
		accepted++
		txt, err := run(bin, nil, dir, "--list-tasks", "-i", "localhost,", path.Base(fi.rel))
		if err != nil {
			continue
		}
		for _, ln := range strings.Split(txt, "\n") {
			m := nameLineRe.FindStringSubmatch(ln)
			if m == nil || m[1] == "" {
				continue
			}
			listed++
			if strings.Contains(m[1], "{{") {
				templated++
				continue
			}
			// "role : task" is how the tool prefixes a role's task.
			n := m[1]
			if i := strings.Index(n, " : "); i >= 0 {
				n = n[i+3:]
			}
			if names[n] {
				found++
			}
		}
	}
	fmt.Fprintf(os.Stderr, "ansible-oracle: ansible-playbook (tier A cross-check, never scored): %d playbooks, --syntax-check accepts %d and rejects %d; --list-tasks names %d plays and tasks, %d templated, %d of the rest are definitions of the walk\n",
		playbooks, accepted, rejected, listed, templated, found)
}

// crossCheckInventories runs ansible-inventory --list on each YAML inventory.
func crossCheckInventories(bin, subject string, order []*fileInfo, o *out) {
	groups, hosts := map[string]bool{}, map[string]bool{}
	for _, d := range o.Definitions {
		switch d.Kind {
		case "section":
			groups[d.Name] = true
		case "element":
			hosts[d.Name] = true
		}
	}
	files, ok, listedG, listedH, foundG, foundH := 0, 0, 0, 0, 0, 0
	for _, fi := range order {
		if fi.root.Kind != yaml.MappingNode {
			continue
		}
		if all := get(fi.root, "all"); all == nil || (get(all, "hosts") == nil && get(all, "children") == nil) {
			continue
		}
		files++
		txt, err := run(bin, nil, filepath.Join(subject, filepath.FromSlash(path.Dir(fi.rel))), "--list", "-i", path.Base(fi.rel))
		if err != nil {
			continue
		}
		var inv map[string]json.RawMessage
		if i := strings.Index(txt, "{"); i >= 0 {
			if json.Unmarshal([]byte(txt[i:]), &inv) != nil {
				continue
			}
		}
		ok++
		for g, raw := range inv {
			if g == "_meta" {
				var m struct {
					Hostvars map[string]json.RawMessage `json:"hostvars"`
				}
				json.Unmarshal(raw, &m)
				for h := range m.Hostvars {
					listedH++
					if hosts[h] {
						foundH++
					}
				}
				continue
			}
			if g == "all" || g == "ungrouped" {
				continue
			}
			listedG++
			if groups[g] {
				foundG++
			}
		}
	}
	fmt.Fprintf(os.Stderr, "ansible-oracle: ansible-inventory (tier A cross-check, never scored): %d YAML inventories, %d listed; %d groups (%d are definitions of the walk), %d hosts (%d)\n",
		files, ok, listedG, foundG, listedH, foundH)
}
