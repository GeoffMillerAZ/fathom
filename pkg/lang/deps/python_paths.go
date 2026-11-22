package deps

import (
	"context"
	"errors"
	"io/fs"
	"path"
	"strings"
	"unicode/utf8"
)

type pythonPaths struct {
	repo         repositoryFS
	directories  map[string]pythonDirectory
	declarations map[string]map[string]bool
	fromLists    []pythonImportSite
}

type pythonDirectory struct {
	entries  map[string]fs.DirEntry
	native   map[string]bool
	gap      bool
	complete bool
}

type pythonBinding struct {
	edges      []string
	pkg        string
	found      bool
	gap        bool
	incomplete bool
}

func (p *pythonPaths) resolve(ctx context.Context, out *result, raw, member string, line int, declarationAllowed bool) {
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
	gap, incomplete := false, false
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
			incomplete = incomplete || candidate.incomplete
		}
	}
	found := false
	var declared bool
	if declarationAllowed && len(bindings) == 1 && !gap && !incomplete {
		binding := bindings[0]
		declared = !binding.gap && !binding.incomplete && binding.found && len(binding.edges) == 1 &&
			binding.pkg != "" && binding.edges[0] != out.Path &&
			pythonIdentifier(member) && !pythonReservedDeclaration(member) &&
			p.provesFromList(ctx, binding.pkg, member) &&
			(relative == 0 || p.declarativePackage(ctx, path.Dir(out.Path)) != nil)
	}
	if declared {
		p.fromLists = append(p.fromLists, pythonImportSite{raw, line})
	}
	for _, binding := range bindings {
		emit(binding)
		found = found || binding.found
		gap = gap || binding.gap
		if member == "" || binding.pkg == "" || declared {
			continue
		}
		// A package may publish a same-named attribute or load arbitrary
		// modules via __all__ and __getattr__. Keep visible child candidates.
		pythonFromListGap(out, pythonImportSite{raw, line})
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
		b := pythonBinding{pkg: dir, found: true, incomplete: !p.directory(dir).complete}
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
		b.incomplete = b.incomplete || !entries.complete
		if entries.gap {
			b.gap = true
			return b
		}
		base := path.Join(dir, head)
		packageDir := entries.entries[head]
		source := entries.entries[head+".py"]
		b.gap = b.gap || entries.native[head] || pythonSymlink(packageDir) || pythonSymlink(source)
		if packageDir != nil && packageDir.IsDir() {
			b.incomplete = b.incomplete || !p.directory(base).complete
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
	d.complete = err == nil
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

func (p *pythonPaths) provesFromList(ctx context.Context, dir, member string) bool {
	contents := p.directory(dir)
	if !contents.complete || contents.hasImportCandidate(member, "") {
		return false
	}
	return p.declarativePackage(ctx, dir)[member]
}

func (p *pythonPaths) declarativePackage(ctx context.Context, dir string) map[string]bool {
	if dir == "." {
		return nil
	}
	if cached, ok := p.declarations[dir]; ok {
		return cached
	}
	var declarations map[string]bool
	parent, name := path.Dir(dir), path.Base(dir)
	contents := p.directory(parent)
	entry := contents.entries[name]
	regular, gap := p.initializer(dir, false)
	if pythonIdentifier(name) && (parent != "." || !pythonRuntimeModule(name)) && entry != nil && entry.IsDir() && contents.complete && regular && !gap &&
		p.directory(dir).complete &&
		!contents.hasImportCandidate(name, name) && !p.directory(dir).hasImportCandidate("__init__", "__init__.py") &&
		(parent == "." || p.declarativePackage(ctx, parent) != nil) {
		if src, code := readSource(p.repo, path.Join(dir, "__init__.py")); code == "" {
			declarations = pythonDeclarations(ctx, src)
		}
	}
	p.declarations[dir] = declarations
	return declarations
}

// A case variant or incompatible loader can make the native binding depend
// on the host. Do not inspect candidates or treat a pathname miss as proof.
func (d pythonDirectory) hasImportCandidate(name, allowed string) bool {
	for candidate := range d.entries {
		if candidate == allowed {
			continue
		}
		if strings.EqualFold(candidate, name) || !strings.Contains(candidate, ".") && !pythonASCII(candidate) {
			return true
		}
		stem, _, _ := strings.Cut(candidate, ".")
		switch strings.ToLower(path.Ext(candidate)) {
		case ".py", ".pyc", ".pyi", ".pyo", ".pyw", ".so", ".pyd", ".fwork":
			if strings.EqualFold(stem, name) || !pythonASCII(stem) {
				return true
			}
		}
	}
	return false
}

func pythonASCII(name string) bool {
	return strings.IndexFunc(name, func(r rune) bool { return r >= utf8.RuneSelf }) == -1
}

type pythonImportSite struct {
	raw  string
	line int
}

type pythonSiteEvidence struct {
	count int
	from  bool
}

func pythonFromListGap(out *result, site pythonImportSite) {
	out.addUnknown(Unknown{Code: UnknownDynamicImport, Raw: site.raw, Line: site.line,
		DecidableBy: []DecidableBy{DecideObservation, DecideDeclaration}})
}

func (p *pythonPaths) finishFromLists(ctx context.Context, out *result, src []byte, invalidSyntax bool) {
	if len(p.fromLists) == 0 || !invalidSyntax && p.attributableImports(ctx, src, out.Edges) {
		return
	}
	for _, site := range p.fromLists {
		pythonFromListGap(out, site)
	}
}

// A declaration only certifies one direct initializer edge. Keep the gap
// when the file's from-sites have multi-file footprints or when distinct
// sites can claim the same target, including an implicit relative context.
func (p *pythonPaths) attributableImports(ctx context.Context, src []byte, edges []Edge) bool {
	matches, err := pythonQueryMatches(ctx, src, "dependency-python-attribution", pythonAttributionQuery, false)
	if err != nil {
		return false
	}
	sites := make(map[pythonImportSite]pythonSiteEvidence)
	for _, match := range matches {
		statement, module := match.Captures[0], match.Captures[1]
		site := pythonImportSite{safeRaw(pythonCleanModule(module.Node.Text(src))), statement.Node.Line()}
		evidence := sites[site]
		evidence.count++
		evidence.from = statement.Name == "site.from"
		sites[site] = evidence
	}

	owners := make(map[string]pythonImportSite)
	fromTargets := make(map[pythonImportSite]string)
	claim := func(target string, site pythonImportSite) bool {
		if owner, ok := owners[target]; ok && owner != site {
			return false
		}
		owners[target] = site
		return true
	}
	for _, edge := range edges {
		if edge.Resolution != ResolvedFile {
			continue
		}
		site := pythonImportSite{edge.Raw, edge.Line}
		evidence := sites[site]
		if evidence.count != 1 || !claim(edge.To, site) {
			return false
		}
		if evidence.from {
			if prior, ok := fromTargets[site]; ok && prior != edge.To {
				return false
			}
			fromTargets[site] = edge.To
		}
		if strings.HasPrefix(site.raw, ".") {
			for dir := path.Dir(edge.To); dir != "."; dir = path.Dir(dir) {
				if regular, _ := p.initializer(dir, false); regular && !claim(path.Join(dir, "__init__.py"), site) {
					return false
				}
			}
		}
	}
	return true
}
