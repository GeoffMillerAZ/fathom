package deps

import (
	"errors"
	"io/fs"
	"path"
	"strings"
)

type pythonPaths struct {
	repo        repositoryFS
	directories map[string]pythonDirectory
}

type pythonDirectory struct {
	entries map[string]fs.DirEntry
	native  map[string]bool
	gap     bool
}

type pythonBinding struct {
	edges []string
	pkg   string
	found bool
	gap   bool
}

func (p *pythonPaths) resolve(out *result, raw, member string, line int) {
	emit := func(b pythonBinding) {
		for _, target := range b.edges {
			if target != out.Path {
				out.addEdge(Edge{Kind: KindImport, Raw: raw, Line: line, Resolution: ResolvedFile, To: target, Provenance: ProvenanceBuiltin})
			}
		}
	}
	addGap := func() {
		out.addEdge(Edge{Kind: KindImport, Raw: raw, Line: line, Resolution: ResolvedUnknown, Provenance: ProvenanceBuiltin})
		out.addUnknown(Unknown{Code: UnknownMissingConfig, Raw: raw, Line: line,
			DecidableBy: []DecidableBy{DecideConfigFile, DecideDeclaration, DecideToolchain}})
	}
	var bindings []pythonBinding
	gap := false
	dir := path.Dir(out.Path)
	module := strings.TrimLeft(raw, ".")
	relative := len(raw) - len(module)
	if relative > 0 {
		for depth := 0; depth < relative; depth++ {
			if dir == "." {
				addGap()
				return
			}
			ok, uncertain := p.initializer(dir, true)
			gap = gap || !ok || uncertain
			if depth+1 < relative {
				dir = path.Dir(dir)
			}
		}
		// Even a regular leaf package can sit inside a namespace. Its
		// relative binding needs the surrounding package identity as well.
		for ancestor := path.Dir(dir); ancestor != "."; ancestor = path.Dir(ancestor) {
			ok, uncertain := p.initializer(ancestor, true)
			gap = gap || !ok || uncertain
		}
		bindings = append(bindings, p.lookup(dir, module))
	} else {
		root := p.lookup(".", module)
		bindings = append(bindings, root)
		head, _, _ := strings.Cut(module, ".")
		gap = root.found && pythonRuntimeModule(head)
		// A src layout or a direct script launch needs configuration. Report
		// visible candidates, and withhold completeness when a second root
		// can change a binding. Arbitrary PYTHONPATH roots are not guessed.
		var alternateRoots []string
		if src := p.directory(".").entries["src"]; dir != "src" && src != nil && (src.IsDir() || pythonSymlink(src)) {
			alternateRoots = append(alternateRoots, "src")
		}
		if dir != "." {
			alternateRoots = append(alternateRoots, dir)
		}
		for _, alternate := range alternateRoots {
			candidate := p.lookup(alternate, module)
			if candidate.found || len(candidate.edges) != 0 {
				candidate.gap = true
				bindings = append(bindings, candidate)
			}
			gap = gap || candidate.gap
		}
	}
	found := false
	for _, binding := range bindings {
		emit(binding)
		found = found || binding.found
		gap = gap || binding.gap
		if member == "" || binding.pkg == "" {
			continue
		}
		// A package may publish a same-named attribute or load arbitrary
		// modules via __all__ and __getattr__. Keep visible child candidates.
		out.addUnknown(Unknown{Code: UnknownDynamicImport, Raw: raw, Line: line,
			DecidableBy: []DecidableBy{DecideObservation, DecideDeclaration}})
		if member != "*" {
			child := p.lookup(binding.pkg, member)
			emit(child)
			gap = gap || child.gap
		}
	}
	if gap || !found {
		addGap()
	}
}

// Interpreter and platform modules can bypass the filesystem search. An
// underscore also covers platform-private built-ins that vary by CPython build.
func pythonRuntimeModule(head string) bool {
	switch head {
	case "abc", "atexit", "builtins", "codecs", "collections", "errno", "faulthandler", "gc",
		"importlib", "io", "itertools", "marshal", "math", "nt", "ntpath", "os", "posix",
		"posixpath", "pwd", "runpy", "site", "stat", "sys", "time", "winreg", "zipimport":
		return true
	}
	return strings.HasPrefix(head, "_")
}

func (p *pythonPaths) lookup(dir, module string) pythonBinding {
	if module == "" {
		b := pythonBinding{pkg: dir, found: true}
		if ok, gap := p.initializer(dir, false); ok {
			b.edges = []string{path.Join(dir, "__init__.py")}
		} else {
			b.gap = true
			b.found = !gap
		}
		return b
	}
	var b pythonBinding
	for head, tail, more := strings.Cut(module, "."); ; head, tail, more = strings.Cut(tail, ".") {
		if head == "" || !validPath(head) || strings.Contains(head, "/") {
			b.gap = true
			return b
		}
		entries := p.directory(dir)
		if entries.gap {
			b.gap = true
			return b
		}
		base := path.Join(dir, head)
		packageDir := entries.entries[head]
		source := entries.entries[head+".py"]
		b.gap = b.gap || entries.native[head] || pythonSymlink(packageDir) || pythonSymlink(source)
		if packageDir != nil && packageDir.IsDir() {
			if ok, gap := p.initializer(base, more); ok {
				b.gap = b.gap || gap
				b.edges = append(b.edges, path.Join(base, "__init__.py"))
				if !more {
					b.pkg, b.found = base, true
					return b
				}
				dir = base
				continue
			} else {
				b.gap = b.gap || gap
			}
		}
		if source != nil && source.Type().IsRegular() {
			b.edges = append(b.edges, base+".py")
			b.found = !more
			return b
		}
		if packageDir == nil || !packageDir.IsDir() {
			return b
		}
		b.gap = true
		if !more {
			b.pkg, b.found = base, true
			return b
		}
		dir = base
	}
}

func (p *pythonPaths) initializer(dir string, inspect bool) (regular, gap bool) {
	contents := p.directory(dir)
	entry := contents.entries["__init__.py"]
	gap = contents.gap || contents.native["__init__"] || pythonSymlink(entry)
	if entry == nil || !entry.Type().IsRegular() {
		return false, gap
	}
	if inspect {
		source, code := readSource(p.repo, path.Join(dir, "__init__.py"))
		gap = gap || code != "" || pythonPackagePathMayChange(string(source))
	}
	return true, gap
}

func pythonPackagePathMayChange(src string) bool {
	for _, token := range []string{"__path__", "__spec__", "__package__", "declare_namespace", "extend_path",
		"meta_path", "path_hooks", "__import__", "importlib", "exec", "eval", "getattr", "setattr", "globals", "sys"} {
		if strings.Contains(src, token) {
			return true
		}
	}
	return false
}

func pythonSymlink(entry fs.DirEntry) bool {
	return entry != nil && entry.Type()&fs.ModeSymlink != 0
}

func (p *pythonPaths) directory(dir string) pythonDirectory {
	if d, ok := p.directories[dir]; ok {
		return d
	}
	d := pythonDirectory{entries: make(map[string]fs.DirEntry), native: make(map[string]bool)}
	entries, err := fs.ReadDir(p.repo, dir)
	d.gap = err != nil && !errors.Is(err, fs.ErrNotExist)
	for _, entry := range entries {
		name := entry.Name()
		d.entries[name] = entry
		if !entry.IsDir() {
			switch path.Ext(name) {
			case ".so", ".pyd", ".pyc":
				head, _, _ := strings.Cut(name, ".")
				d.native[head] = true
			}
		}
	}
	p.directories[dir] = d
	return d
}
