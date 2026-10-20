package extractor

// cuefilespecs.go — the CUE dialect's reader as the file resolver's
// FileSpecExtractor (LB-F583, fusion's cue-module strategy): one
// whole-package import per import declaration, bound to the package's
// identifier (the name the extractor's references through the import are
// written with: "seg.#A"), and the file's own package as its one re-export,
// so a reference through an import of a package split across files reaches
// the file defining the name.

import (
	cuepath "path"
	"strings"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/literal"
	"cuelang.org/go/cue/parser"

	"github.com/geoffmilleraz/fathom/internal/core"
)

var _ FileSpecExtractor = (*CUEExtractor)(nil)

// Name is the dialect id, the file resolver's language key.
func (c *CUEExtractor) Name() string { return "cue" }

// Close releases nothing: the reader holds no module.
func (c *CUEExtractor) Close() {}

// ExtractFileSpecs implements FileSpecExtractor: the file's imports, and its
// own package (".:name", the package name instantiated at the file's own
// directory, the cue binary's spelling of it) as a re-export.
func (c *CUEExtractor) ExtractFileSpecs(relPath string, src []byte) (FileSpecs, error) {
	f, _ := parser.ParseFile(relPath, src, parser.ImportsOnly)
	if f == nil {
		return FileSpecs{}, nil
	}
	var fs FileSpecs
	for _, s := range cueImportSpans(f) {
		fs.Imports = append(fs.Imports, s.ImportSpec)
	}
	if name := f.PackageName(); name != "" && name != "_" {
		for _, d := range f.Decls {
			if p, ok := d.(*ast.Package); ok {
				fs.Exports = []ExportSpec{{Module: ".:" + name, Line: p.Pos().Line()}}
				break
			}
		}
	}
	return fs, nil
}

// ExtractImportSpans implements FileSpecExtractor: each import declaration's
// lines.
func (c *CUEExtractor) ExtractImportSpans(relPath string, src []byte) ([]ImportSpan, error) {
	f, _ := parser.ParseFile(relPath, src, parser.ImportsOnly)
	if f == nil {
		return nil, nil
	}
	return cueImportSpans(f), nil
}

// cueImportSpans is f's import declarations, as importDecls reads them (a
// malformed path is no import).
func cueImportSpans(f *ast.File) []ImportSpan {
	var out []ImportSpan
	for imp := range f.ImportSpecs() {
		p, ok := cueImportPathOf(imp)
		if !ok {
			continue
		}
		line := imp.Pos().Line()
		out = append(out, ImportSpan{
			ImportSpec: ImportSpec{LocalName: cueImportIdent(p), Module: p, Kind: ImportWhole, Line: line},
			StartLine:  line, EndLine: max(line, imp.End().Line()),
		})
	}
	return out
}

// cueImportPathOf is an import declaration's path, unquoted; ok is false for
// a malformed one (see importDecls: a partial tree from a mid-edit save).
func cueImportPathOf(imp *ast.ImportSpec) (string, bool) {
	if imp.Path == nil || imp.Path.Value == "" {
		return "", false
	}
	p := imp.Path.Value
	trimmed := strings.TrimLeft(p, "#")
	if len(trimmed) == 0 || (trimmed[0] != '"' && trimmed[0] != '\'') {
		return "", false
	}
	p, err := literal.Unquote(p)
	if err != nil || p == "" {
		return "", false
	}
	return p, true
}

// cueImportIdent is the identifier an import path binds (an alias aside):
// its `:name` qualifier, else its last element less a major version
// (`cue.dev/x/githubactions@v0` binds githubactions).
func cueImportIdent(p string) string {
	bare, qual, _ := strings.Cut(p, ":")
	if qual != "" {
		return qual
	}
	seg := cuepath.Base(bare)
	if at := strings.LastIndexByte(seg, '@'); at > 0 {
		seg = seg[:at]
	}
	return seg
}

// CUEPackageOf is the package name s, a symbol of the CUE dialect, is the
// package clause of; ok is false for any other symbol. The cue-module
// strategy indexes a universe's packages by it, reading no file.
func CUEPackageOf(s core.SymbolNode) (string, bool) {
	if s.Kind != kindCUEPackage {
		return "", false
	}
	name, ok := strings.CutPrefix(s.Name, cuePackagePrefix)
	return name, ok && name != ""
}

// IsCUEPackageEntity reports whether s is a CUE file's package clause: what
// an import of the file's package targets.
func IsCUEPackageEntity(s core.SymbolNode) bool {
	_, ok := CUEPackageOf(s)
	return ok
}

// CUEModulePath is the module path a cue.mod/module.cue declares (its
// `module` field, a major version suffix dropped), "" when it declares none
// or does not parse.
func CUEModulePath(src []byte) string {
	f, _ := parser.ParseFile("module.cue", src)
	if f == nil {
		return ""
	}
	for _, d := range f.Decls {
		field, ok := d.(*ast.Field)
		if !ok {
			continue
		}
		if name, _, err := ast.LabelName(field.Label); err != nil || name != "module" {
			continue
		}
		lit, ok := field.Value.(*ast.BasicLit)
		if !ok {
			return ""
		}
		mp, err := literal.Unquote(lit.Value)
		if err != nil {
			return ""
		}
		if at := strings.LastIndexByte(mp, '@'); at > 0 {
			mp = mp[:at]
		}
		return mp
	}
	return ""
}
