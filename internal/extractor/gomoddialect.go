package extractor

// gomoddialect.go — the go-mod native dialect (LB-Q-go-mod, docs/design/
// language-breadth.md §13, engine "native": grammar_source
// native:golang.org/x/mod). It reads a go.mod and a go.work with the Go
// tool's own parser, golang.org/x/mod/modfile, and gives the document-path
// engine's cross-file pass (fusion.ResolveDocXrefs) its DocResult, so the
// module graph a repository declares lands as file-tier edges between
// entities, the L3 the tool formats reach through it (§4).
//
// Entities (dialects/go-mod/PROVENANCE.md says what is and is not modelled):
//
//   - go.mod: the module directive (GoModule, named by the module path,
//     signed "go <version>", carrying every required module as a call); a
//     ModuleDep per require (the incumbent's kind, name and version
//     signature); a GoModReplace per replace (replace/<old>[@<version>],
//     signed by its right-hand side).
//   - go.work: the workspace (GoWorkspace, named "workspace", spanning the
//     file's directives), a GoWorkUse per use (use/<directory as written>),
//     and the same GoModReplace.
//
// Cross-file references: a require resolves by module path to the one other
// go.mod declaring it (module-source, edge depends_on); a replace to a local
// directory and a go.work use resolve to that directory's go.mod (repo-path,
// edges replaced_by and workspace_member).

import (
	"bytes"
	"errors"
	"path"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"

	"github.com/geoffmilleraz/fathom/internal/core"
)

const (
	// kindGoModReplace and kindGoWorkUse have no connector-classification
	// role (the file kinds are core.KindGoModule and core.KindGoWorkspace),
	// so they are local to the dialect, as the doc-engine dialects' secondary
	// kinds are.
	kindGoModReplace core.SourceKind = "GoModReplace"
	kindGoWorkUse    core.SourceKind = "GoWorkUse"

	// goWorkspaceName is the go.work's own entity, and its root.
	goWorkspaceName = "workspace"

	// goModRecoveryRounds bounds how many times a file is re-read after its
	// rejected lines are blanked (see parseGoModFile).
	goModRecoveryRounds = 32
)

// GoModulesExtractor implements Extractor and DocResulter for go.mod and
// go.work files. Extract returns the entities; ExtractDoc also returns the
// cross-file references and the file's root entities.
type GoModulesExtractor struct{}

var (
	_ Extractor   = (*GoModulesExtractor)(nil)
	_ DocResulter = (*GoModulesExtractor)(nil)
)

// NewGoModulesExtractor returns the go-mod dialect's extractor.
func NewGoModulesExtractor() *GoModulesExtractor { return &GoModulesExtractor{} }

// NativeDialectExtractor returns the stand-alone extractor of a native-engine
// dialect that has one (the harness measures the dialect's own code through
// it, not fusion's dispatch, so a DocResulter's cross-file references reach
// the xref scoring). ok is false for an unknown id.
func NativeDialectExtractor(id string) (Extractor, bool) {
	if e, ok := ProxyDialectExtractor(id); ok {
		return e, true
	}
	if e, ok := HCLToolDialectExtractor(id); ok {
		return e, true
	}
	if e, ok := TimoniDialectExtractor(id); ok {
		return e, true
	}
	switch id {
	case "terraform":
		return NewHCLExtractor(), true
	case "go-mod":
		return NewGoModulesExtractor(), true
	case "cue":
		return NewCUEExtractor(), true // LB-Q-cue: the CUE dialect's own DocResulter (cuedoc.go)
	case "fathom-plan-rows":
		return NewPlanRowsExtractor(), true // wave fx item fmt
	case "markdown":
		return NewMarkdownExtractor(), true // LB-Q-markdown: headings, and the links between Markdown files
	}
	return nil, false
}

// Extract implements Extractor.
func (g *GoModulesExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	r, err := g.ExtractDoc(service, relPath, src)
	return r.Nodes, err
}

// ExtractDoc implements DocResulter. A file the parser rejects outright
// yields no entities and no error, as the doc engine does for malformed
// input; a file with some rejected lines yields what its other lines say.
func (g *GoModulesExtractor) ExtractDoc(service, relPath string, src []byte) (DocResult, error) {
	x := goModWalk{service: service, relPath: relPath, pkg: pkgFromPath(relPath)}
	if path.Base(relPath) == "go.work" {
		x.workFile(src)
	} else {
		x.modFile(src)
	}
	sort.SliceStable(x.res.Nodes, func(i, j int) bool {
		a, b := x.res.Nodes[i], x.res.Nodes[j]
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Name < b.Name
	})
	sort.SliceStable(x.res.CrossFile, func(i, j int) bool {
		a, b := x.res.CrossFile[i], x.res.CrossFile[j]
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Edge != b.Edge {
			return a.Edge < b.Edge
		}
		return a.From < b.From
	})
	return x.res, nil
}

// goModWalk is one file's extraction in progress.
type goModWalk struct {
	service, relPath, pkg string
	res                   DocResult
}

// node appends one entity and returns its index in the result.
func (x *goModWalk) node(name string, kind core.SourceKind, sig string, syn *modfile.Line) int {
	n := core.SymbolNode{
		Service: x.service, Package: x.pkg, Name: name, Kind: kind, FilePath: x.relPath,
		Signature: capString(sig, 200), Tier: core.TierPreScan,
	}
	if syn != nil {
		n.Line, n.EndLine = syn.Start.Line, syn.End.Line
	}
	x.res.Nodes = append(x.res.Nodes, n)
	return len(x.res.Nodes) - 1
}

func (x *goModWalk) cross(from, edge, raw, target, strategy string, syn *modfile.Line) {
	c := DocCrossFileRef{From: from, Edge: edge, Raw: raw, Target: target, Strategy: strategy}
	if syn != nil {
		c.Line = syn.Start.Line
	}
	x.res.CrossFile = append(x.res.CrossFile, c)
}

func (x *goModWalk) modFile(src []byte) {
	f := parseGoModFile(src, func(b []byte) (*modfile.File, error) { return modfile.Parse(x.relPath, b, nil) })
	if f == nil {
		return
	}
	var required []string
	seen := map[string]bool{}
	for _, r := range f.Require {
		if r.Syntax == nil || r.Mod.Path == "" {
			continue
		}
		x.node(r.Mod.Path, core.KindModuleDep, r.Mod.Version, r.Syntax)
		if !seen[r.Mod.Path] {
			seen[r.Mod.Path] = true
			required = append(required, r.Mod.Path)
		}
	}
	if f.Module != nil && f.Module.Syntax != nil && f.Module.Mod.Path != "" {
		mod := f.Module.Mod.Path
		sig := ""
		if f.Go != nil {
			sig = "go " + f.Go.Version
		}
		i := x.node(mod, core.KindGoModule, sig, f.Module.Syntax)
		sort.Strings(required)
		x.res.Nodes[i].Calls = required
		x.res.Roots = append(x.res.Roots, mod)
		for _, r := range f.Require {
			if r.Syntax != nil && r.Mod.Path != "" {
				x.cross(mod, "depends_on", r.Mod.Path, r.Mod.Path, "module-source", r.Syntax)
			}
		}
	}
	for _, r := range f.Replace {
		x.replace(r)
	}
}

func (x *goModWalk) workFile(src []byte) {
	f := parseGoModFile(src, func(b []byte) (*modfile.WorkFile, error) { return modfile.ParseWork(x.relPath, b, nil) })
	if f == nil {
		return
	}
	first, last := 0, 0
	span := func(syn *modfile.Line) {
		if syn == nil {
			return
		}
		if first == 0 || syn.Start.Line < first {
			first = syn.Start.Line
		}
		last = max(last, syn.End.Line)
	}
	sig := ""
	if f.Go != nil {
		sig = "go " + f.Go.Version
		span(f.Go.Syntax)
	}
	if f.Toolchain != nil {
		span(f.Toolchain.Syntax)
	}
	for _, g := range f.Godebug {
		span(g.Syntax)
	}
	for _, u := range f.Use {
		if u.Syntax == nil || u.Path == "" {
			continue
		}
		span(u.Syntax)
		name := "use/" + u.Path
		x.node(name, kindGoWorkUse, "", u.Syntax)
		x.cross(name, "workspace_member", u.Path, x.localTarget(u.Path), "repo-path", u.Syntax)
	}
	for _, r := range f.Replace {
		span(r.Syntax)
		x.replace(r)
	}
	if first == 0 {
		return
	}
	x.res.Nodes = append(x.res.Nodes, core.SymbolNode{
		Service: x.service, Package: x.pkg, Name: goWorkspaceName, Kind: core.KindGoWorkspace,
		FilePath: x.relPath, Line: first, EndLine: last, Signature: sig, Tier: core.TierPreScan,
	})
	x.res.Roots = append(x.res.Roots, goWorkspaceName)
}

// replace records one replace directive of a go.mod or a go.work: the entity
// and, when the right-hand side is a directory, the reference to that
// directory's go.mod.
func (x *goModWalk) replace(r *modfile.Replace) {
	if r.Syntax == nil || r.Old.Path == "" {
		return
	}
	name := "replace/" + r.Old.Path
	if r.Old.Version != "" {
		name += "@" + r.Old.Version
	}
	sig := r.New.Path
	if r.New.Version != "" {
		sig += " " + r.New.Version
	}
	x.node(name, kindGoModReplace, sig, r.Syntax)
	if modfile.IsDirectoryPath(r.New.Path) {
		x.cross(name, "replaced_by", r.New.Path, x.localTarget(r.New.Path), "repo-path", r.Syntax)
	}
}

// localTarget is the repo-relative go.mod a directory path written in this
// file names: relative to the file's own directory, "" when the path is
// absolute or climbs out of the repository (the doc engine's convention for a
// reference that cannot resolve).
func (x *goModWalk) localTarget(dir string) string {
	dir = strings.ReplaceAll(dir, `\`, "/")
	if strings.HasPrefix(dir, "/") || (len(dir) >= 2 && dir[1] == ':') {
		return ""
	}
	joined := path.Join(path.Dir(x.relPath), dir)
	if joined == ".." || strings.HasPrefix(joined, "../") {
		return ""
	}
	return path.Join(joined, "go.mod")
}

func capString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// parseGoModFile reads src with parse and, when the parser rejects lines,
// blanks each rejected line (its numbering kept) and reads again, so one bad
// directive costs that directive and not the file: a require whose path and
// version disagree, a directive from a newer Go, a merge marker. A block left
// open at the end of the file is closed. The parser stops at its first
// lexical error, so the loop is bounded; a file still rejected after
// goModRecoveryRounds, or rejected at no line, reads as nothing.
func parseGoModFile[T any](src []byte, parse func([]byte) (*T, error)) *T {
	for range goModRecoveryRounds {
		f, err := parse(src)
		if err == nil {
			return f
		}
		var list modfile.ErrorList
		if !errors.As(err, &list) {
			return nil
		}
		lines := map[int]bool{}
		unterminated := false
		for _, e := range list {
			if e.Pos.Line > 0 {
				lines[e.Pos.Line] = true
			}
			if strings.Contains(e.Err.Error(), "unterminated block") {
				unterminated = true
			}
		}
		next := blankLines(src, lines)
		if unterminated {
			next = append(bytes.TrimRight(next, "\n"), "\n)\n"...)
		}
		if bytes.Equal(next, src) {
			return nil
		}
		src = next
	}
	return nil
}

// blankLines returns src with the given 1-based lines emptied.
func blankLines(src []byte, lines map[int]bool) []byte {
	if len(lines) == 0 {
		return src
	}
	parts := bytes.Split(src, []byte("\n"))
	for i := range parts {
		if lines[i+1] {
			parts[i] = nil
		}
	}
	return bytes.Join(parts, []byte("\n"))
}
