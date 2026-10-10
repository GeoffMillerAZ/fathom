package fusion

// lambdahandler.go — LB-F370, LB-F371 (wave c3, item cw): what code a Lambda
// function runs. A CloudFormation or SAM template's function (an
// AWS::Serverless::Function, an AWS::Lambda::Function) and a Terraform
// aws_lambda_function name their handler as a string and their code as a
// local path; the function's entity is the origin of a file-tier typed edge
// (stored with the doc-xref edges, never a gate) to the handler's symbol in
// the indexed code. Only the named file is extracted, as the GitHub Actions
// run-step binding does (gharun.go).
//
// The rules, per runtime family, are the runtimes' own (tier C: nothing is
// built or deployed):
//
//   - python: `pkg.module.func` (a `/` reads as a `.`) is def func in
//     <code>/pkg/module.py, else <code>/pkg/module/__init__.py;
//   - nodejs: `dir/file.export` is the function export in <code>/dir/file.js,
//     .mjs or .cjs; a SAM esbuild function (Metadata BuildMethod esbuild) is
//     its BuildProperties.EntryPoints entry of that path (a TypeScript
//     source), and a .ts, .mts or .cts at the path is read last;
//   - java: `pkg.Class::method` is method in <code>/src/main/java/pkg/Class.java
//     (else the one pkg/Class.java under <code>); a bare `pkg.Class` is its
//     handleRequest;
//   - ruby: `file.method` is def method in <code>/file.rb.
//
// <code> is the function's code directory: SAM's CodeUri (a Globals.Function
// default applies, and SAM's own default, the template's directory, when
// neither names one), an AWS::Lambda::Function's package-able Code path, or a
// Terraform function's archive_file source_dir (or its one source_file,
// whose base name is the module, or a source block's filename whose content
// is a file() of the repository). Handler and Runtime default from
// Globals.Function too; a Runtime that is no literal (a parameter, a
// variable) is read from the code: the first of python, nodejs and ruby whose
// rule names a file the package holds. Nothing binds, and the reason is counted
// (LambdaHandlerStats), for an image (PackageType Image, ImageUri), inline
// code (InlineCode, Code.ZipFile, an archive_file source block), code in S3
// or a built artifact (a .zip or .jar), a value computed at deploy time
// (!Sub, !Ref, a variable), the go, provided, dotnet and custom runtimes, a
// Terraform module call, and a handler whose file or symbol the repository
// does not hold.

import (
	"context"
	"path"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"gopkg.in/yaml.v3"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// Edge and strategy of a handler binding.
const (
	LambdaHandlerEdge     = "handler"
	LambdaHandlerStrategy = "lambda-handler"
)

// Reasons a function binds no handler (LambdaHandlerStats.Unbound's keys).
const (
	lambdaImage     = "image"
	lambdaInline    = "inline"
	lambdaRemote    = "remote"
	lambdaArtifact  = "artifact"
	lambdaTemplated = "templated"
	lambdaRuntime   = "runtime"
	lambdaNoHandler = "no-handler"
	lambdaNoFile    = "no-file"
	lambdaNoSymbol  = "no-symbol"
	lambdaModule    = "module"
)

// LambdaHandlerStats is what a resolution read: the functions declared, the
// ones bound, and the rest by reason.
type LambdaHandlerStats struct {
	Functions, Bound int
	Unbound          map[string]int
}

// lambdaFn is one Lambda function a template or a Terraform module declares.
type lambdaFn struct {
	File, Name       string // the declaring file and the function's entity name
	Runtime, Handler string
	CodeDir          string // repository directory the handler's module path is read from
	// Files, when set, is the package's every file by its name in the
	// package (a Terraform source_file by its base name, a source block's
	// filename by the file() its content reads): the handler's module is one
	// of them, never a path under CodeDir.
	Files       map[string]string
	EntryPoints []string // SAM esbuild entry points, repository paths
	Unbound     string   // why nothing binds, "" when the rules apply
}

// LambdaCode is what the handler binding reads beyond the doc files: the
// repository's paths, a file's content, and a code file's symbols.
type LambdaCode struct {
	Paths   map[string]bool
	Read    func(path string) ([]byte, bool)
	Symbols func(path string) []core.SymbolNode
}

// ResolveLambdaHandlers binds every Lambda function the set's CloudFormation
// templates and Terraform files declare to its handler symbol: sorted, one
// edge per function.
func ResolveLambdaHandlers(files []DocFile, code LambdaCode) ([]DocXref, LambdaHandlerStats) {
	st := LambdaHandlerStats{Unbound: map[string]int{}}
	var fns []lambdaFn
	byFile := map[string]map[string]core.SymbolNode{}
	tfByDir := map[string][]string{} // a module directory's .tf files
	tfFns := map[string]bool{}       // the directories declaring a function or a function module
	for _, f := range files {
		m := map[string]core.SymbolNode{}
		cfn, cfnFns := false, false
		for _, n := range f.Result.Nodes {
			switch n.Kind {
			case "CFNTemplate":
				cfn = true
			case "CFNResource", core.KindTerraformResource, core.KindTerraformModule:
				if _, dup := m[n.Name]; !dup {
					m[n.Name] = n
				}
				switch {
				case !isLambdaDeclaration(n):
				case n.Kind == "CFNResource":
					cfnFns = true // a function resource: the template is read for its functions
				default:
					tfFns[path.Dir(f.Path)] = true
				}
			}
		}
		byFile[f.Path] = m
		switch {
		case cfn:
			// A function cfnLambdaFunctions reads binds only through its
			// CFNResource entity, signed by its Type: a template with none
			// declares no function this resolution can bind.
			if !cfnFns {
				break
			}
			if src, ok := code.Read(f.Path); ok {
				fns = append(fns, cfnLambdaFunctions(f.Path, src)...)
			}
		case strings.HasSuffix(f.Path, ".tf"):
			tfByDir[path.Dir(f.Path)] = append(tfByDir[path.Dir(f.Path)], f.Path)
		}
	}
	tfDirs := make([]string, 0, len(tfFns))
	for d := range tfFns {
		tfDirs = append(tfDirs, d)
	}
	sort.Strings(tfDirs)
	for _, d := range tfDirs {
		fns = append(fns, tfLambdaFunctions(d, tfByDir[d], code.Read)...)
	}
	cache := map[string][]core.SymbolNode{}
	symbols := func(file string) []core.SymbolNode {
		if s, ok := cache[file]; ok {
			return s
		}
		s := code.Symbols(file)
		cache[file] = s
		return s
	}
	var out []DocXref
	for _, fn := range fns {
		from, ok := byFile[fn.File][fn.Name]
		if !ok {
			continue
		}
		st.Functions++
		why := fn.Unbound
		var file string
		var to core.SymbolNode
		if why == "" {
			file, to, why = bindHandler(fn, code.Paths, symbols)
		}
		if why != "" {
			st.Unbound[why]++
			continue
		}
		st.Bound++
		out = append(out, DocXref{FromFile: fn.File, From: fn.Name, FromKey: from.Key(), ToFile: file, To: to.Name, ToKey: to.Key(), Edge: LambdaHandlerEdge, Strategy: LambdaHandlerStrategy})
	}
	sortDocXrefs(out)
	return out, st
}

// bindHandler applies the runtime's rule: the handler's file and symbol, or
// why there is none.
func bindHandler(fn lambdaFn, paths map[string]bool, symbols func(string) []core.SymbolNode) (string, core.SymbolNode, string) {
	h := fn.Handler
	pick := func(files []string, match func(core.SymbolNode) bool) (string, core.SymbolNode, string) {
		for _, f := range files {
			if f == "" || !paths[f] {
				continue
			}
			var best *core.SymbolNode
			syms := symbols(f)
			for i := range syms {
				if match(syms[i]) && (best == nil || syms[i].Line < best.Line) {
					best = &syms[i]
				}
			}
			if best == nil {
				return "", core.SymbolNode{}, lambdaNoSymbol
			}
			return f, *best, ""
		}
		return "", core.SymbolNode{}, lambdaNoFile
	}
	moduleFiles := func(module string, exts ...string) []string {
		var out []string
		for _, e := range exts {
			if fn.Files != nil {
				out = append(out, fn.Files[path.Clean(module+e)])
			} else {
				out = append(out, joinRepo(fn.CodeDir, module+e))
			}
		}
		return out
	}
	rt := fn.Runtime
	if rt == "" {
		rt = inferRuntime(fn, paths)
	}
	switch {
	case strings.HasPrefix(rt, "python"):
		i := strings.LastIndex(h, ".")
		if i <= 0 || i == len(h)-1 {
			return "", core.SymbolNode{}, lambdaNoHandler
		}
		module, name := strings.ReplaceAll(strings.ReplaceAll(h[:i], ".", "/"), "//", "/"), h[i+1:]
		files := moduleFiles(module, ".py", "/__init__.py")
		return pick(files, func(n core.SymbolNode) bool {
			return n.Name == name && (n.Kind == "PythonFunc" || n.Kind == "PythonVar")
		})
	case strings.HasPrefix(rt, "nodejs"):
		base := path.Base(h)
		root := strings.TrimSuffix(h, base)
		j := strings.Index(base, ".")
		if j <= 0 || j == len(base)-1 {
			return "", core.SymbolNode{}, lambdaNoHandler
		}
		module, export := root+base[:j], base[j+1:]
		if k := strings.LastIndex(export, "."); k >= 0 {
			export = export[k+1:]
		}
		var files []string
		for _, e := range fn.EntryPoints {
			stem := strings.TrimSuffix(e, path.Ext(e))
			if stem == joinRepo(fn.CodeDir, module) || strings.HasSuffix(stem, "/"+module) {
				files = append(files, e)
			}
		}
		files = append(files, moduleFiles(module, ".js", ".mjs", ".cjs", ".ts", ".mts", ".cts")...)
		return pick(files, func(n core.SymbolNode) bool { return n.Name == export && n.Kind == "JSFunc" })
	case strings.HasPrefix(rt, "java"):
		class, method, _ := strings.Cut(h, "::")
		if method == "" {
			method = "handleRequest"
		}
		if class == "" {
			return "", core.SymbolNode{}, lambdaNoHandler
		}
		simple := class[strings.LastIndex(class, ".")+1:]
		if k := strings.LastIndex(simple, "$"); k >= 0 {
			simple = simple[k+1:]
		}
		rel := strings.ReplaceAll(strings.Split(class, "$")[0], ".", "/") + ".java"
		files := []string{joinRepo(fn.CodeDir, "src/main/java/"+rel)}
		if !paths[files[0]] {
			files = append(files, underDir(paths, fn.CodeDir, "/"+rel)...)
		}
		return pick(files, func(n core.SymbolNode) bool { return n.Name == simple+"."+method && n.Kind == "JavaMethod" })
	case strings.HasPrefix(rt, "ruby"):
		base := path.Base(h)
		root := strings.TrimSuffix(h, base)
		j := strings.Index(base, ".")
		if j <= 0 || j == len(base)-1 {
			return "", core.SymbolNode{}, lambdaNoHandler
		}
		name := base[j+1:]
		if k := strings.LastIndex(name, "::"); k >= 0 {
			name = name[k+2:] // Module::Class.method: Class.method
		}
		return pick(moduleFiles(root+base[:j], ".rb"), func(n core.SymbolNode) bool { return n.Name == name && n.Kind == "RubyMethod" })
	}
	return "", core.SymbolNode{}, lambdaRuntime
}

// joinRepo joins a repository directory and a relative path ("" when the
// result leaves the repository).
func joinRepo(dir, rel string) string {
	p := path.Clean(path.Join(dir, rel))
	if p == ".." || strings.HasPrefix(p, "../") {
		return ""
	}
	return p
}

// underDir is every repository path under dir that ends with suffix, sorted.
func underDir(paths map[string]bool, dir, suffix string) []string {
	prefix := ""
	if dir != "." && dir != "" {
		prefix = dir + "/"
	}
	var out []string
	for p := range paths {
		if strings.HasPrefix(p, prefix) && strings.HasSuffix(p, suffix) {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// ── CloudFormation and SAM ────────────────────────────────────────────────

// cfnLambdaFunctions reads a template's Lambda functions, in document order.
func cfnLambdaFunctions(relPath string, content []byte) []lambdaFn {
	var doc yaml.Node
	if err := yaml.Unmarshal(content, &doc); err != nil || len(doc.Content) == 0 {
		return nil
	}
	root := doc.Content[0]
	globals := yamlGet(yamlGet(root, "Globals"), "Function")
	res := yamlGet(root, "Resources")
	if res == nil || res.Kind != yaml.MappingNode {
		return nil
	}
	dir := path.Dir(relPath)
	var out []lambdaFn
	for i := 0; i+1 < len(res.Content); i += 2 {
		name, body := res.Content[i].Value, res.Content[i+1]
		typ, _ := yamlLiteral(yamlGet(body, "Type"))
		props := yamlGet(body, "Properties")
		fn := lambdaFn{File: relPath, Name: name}
		switch typ {
		case "AWS::Serverless::Function":
			fn.Runtime = cfnString(props, globals, "Runtime")
			fn.Handler = cfnString(props, globals, "Handler")
			switch {
			case yamlIs(props, globals, "PackageType", "Image") || yamlGet(props, "ImageUri") != nil:
				fn.Unbound = lambdaImage
			case yamlGet(props, "InlineCode") != nil:
				fn.Unbound = lambdaInline
			default:
				uri := yamlGet(props, "CodeUri")
				if uri == nil {
					uri = yamlGet(globals, "CodeUri")
				}
				fn.CodeDir, fn.Unbound = cfnCodeDir(dir, uri, ".")
			}
			if meta := yamlGet(body, "Metadata"); fn.Unbound == "" && yamlIs(meta, nil, "BuildMethod", "esbuild") {
				if eps := yamlGet(yamlGet(meta, "BuildProperties"), "EntryPoints"); eps != nil && eps.Kind == yaml.SequenceNode {
					for _, e := range eps.Content {
						if v, ok := yamlLiteral(e); ok {
							if p := joinRepo(fn.CodeDir, v); p != "" {
								fn.EntryPoints = append(fn.EntryPoints, p)
							}
						}
					}
				}
			}
		case "AWS::Lambda::Function":
			fn.Runtime, _ = yamlLiteral(yamlGet(props, "Runtime"))
			fn.Handler, _ = yamlLiteral(yamlGet(props, "Handler"))
			code := yamlGet(props, "Code")
			switch {
			case yamlIs(props, nil, "PackageType", "Image") || yamlGet(code, "ImageUri") != nil:
				fn.Unbound = lambdaImage
			case yamlGet(code, "ZipFile") != nil:
				fn.Unbound = lambdaInline
			case code == nil:
				fn.Unbound = lambdaNoFile
			default:
				fn.CodeDir, fn.Unbound = cfnCodeDir(dir, code, "")
			}
		default:
			continue
		}
		if fn.Unbound == "" {
			switch h := yamlGet(props, "Handler"); {
			case fn.Handler != "":
			case h != nil || (globals != nil && yamlGet(globals, "Handler") != nil):
				fn.Unbound = lambdaTemplated
			default:
				fn.Unbound = lambdaNoHandler
			}
		}
		out = append(out, fn)
	}
	return out
}

// cfnCodeDir is a CodeUri's or Code's local directory, relative to the
// template's; def is SAM's default when the property is absent ("" for
// none). A map (an S3 location), an S3 URI, a built artifact and a value
// computed at deploy time name no directory, and say why.
func cfnCodeDir(dir string, n *yaml.Node, def string) (string, string) {
	v := def
	if n != nil {
		lit, ok := yamlLiteral(n)
		switch {
		case n.Kind == yaml.MappingNode && (yamlGet(n, "Bucket") != nil || yamlGet(n, "S3Bucket") != nil):
			return "", lambdaRemote
		case !ok:
			return "", lambdaTemplated
		}
		v = lit
	}
	switch low := strings.ToLower(v); {
	case v == "":
		return "", lambdaNoFile
	case strings.Contains(v, "://"):
		return "", lambdaRemote
	case strings.Contains(v, "${") || strings.Contains(v, "{{"):
		return "", lambdaTemplated
	case strings.HasSuffix(low, ".zip") || strings.HasSuffix(low, ".jar"):
		return "", lambdaArtifact
	}
	p := joinRepo(dir, strings.ReplaceAll(v, `\`, "/"))
	if p == "" {
		return "", lambdaNoFile
	}
	return p, ""
}

// cfnString is a literal property, else its Globals.Function default ("" when
// neither is a literal).
func cfnString(props, globals *yaml.Node, key string) string {
	if n := yamlGet(props, key); n != nil {
		v, _ := yamlLiteral(n)
		return v
	}
	v, _ := yamlLiteral(yamlGet(globals, key))
	return v
}

// yamlIs reports whether the property (else its Globals default) is the
// literal want.
func yamlIs(props, globals *yaml.Node, key, want string) bool {
	n := yamlGet(props, key)
	if n == nil {
		n = yamlGet(globals, key)
	}
	v, ok := yamlLiteral(n)
	return ok && v == want
}

func yamlGet(n *yaml.Node, key string) *yaml.Node {
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

// yamlLiteral is a scalar written as itself: no local tag (`!Sub`, `!Ref`: an
// intrinsic computed at deploy time).
func yamlLiteral(n *yaml.Node) (string, bool) {
	if n == nil || n.Kind != yaml.ScalarNode || (n.Tag != "" && !strings.HasPrefix(n.Tag, "!!")) {
		return "", false
	}
	return n.Value, true
}

// ── Terraform ─────────────────────────────────────────────────────────────

// tfArchive is a data "archive_file": its output and what it packages.
type tfArchive struct {
	output, sourceDir, sourceFile string
	// files is the source blocks' package: a block's filename to the
	// repository file its content reads (`content = file("${path.module}/x.py")`);
	// inline is set when some block's content is written in place.
	files  map[string]string
	inline bool
}

// tfLambdaFunctions reads one module directory's aws_lambda_function
// resources, their code through the archive_file they name.
func tfLambdaFunctions(dir string, files []string, read func(string) ([]byte, bool)) []lambdaFn {
	archives := map[string]tfArchive{}
	type fnBlock struct {
		file string
		b    *hclsyntax.Block
	}
	var blocks []fnBlock
	var modules []lambdaFn
	sort.Strings(files)
	for _, f := range files {
		src, ok := read(f)
		if !ok {
			continue
		}
		cf, _ := hclsyntax.ParseConfig(src, f, hcl.Pos{Line: 1, Column: 1})
		if cf == nil {
			continue
		}
		body, ok := cf.Body.(*hclsyntax.Body)
		if !ok {
			continue
		}
		for _, b := range body.Blocks {
			switch {
			case b.Type == "data" && len(b.Labels) == 2 && b.Labels[0] == "archive_file":
				a := tfArchive{}
				a.output, _ = tfPath(b.Body, "output_path", dir)
				a.sourceDir, _ = tfPath(b.Body, "source_dir", dir)
				a.sourceFile, _ = tfPath(b.Body, "source_file", dir)
				for _, sb := range b.Body.Blocks {
					if sb.Type != "source" {
						continue
					}
					name, ok1 := tfLiteral(sb.Body, "filename")
					from, ok2 := tfFileCall(sb.Body, "content", dir)
					if !ok1 || !ok2 {
						a.inline = true
						continue
					}
					if a.files == nil {
						a.files = map[string]string{}
					}
					a.files[path.Clean(name)] = from
				}
				archives[b.Labels[1]] = a
			case b.Type == "resource" && len(b.Labels) == 2 && b.Labels[0] == "aws_lambda_function":
				blocks = append(blocks, fnBlock{f, b})
			case b.Type == "module" && len(b.Labels) == 1 && b.Body.Attributes["handler"] != nil:
				modules = append(modules, lambdaFn{File: f, Name: "module." + b.Labels[0], Unbound: lambdaModule})
			}
		}
	}
	var out []lambdaFn
	for _, fb := range blocks {
		b := fb.b
		fn := lambdaFn{File: fb.file, Name: "aws_lambda_function." + b.Labels[1]}
		fn.Runtime, _ = tfLiteral(b.Body, "runtime")
		h, ok := tfLiteral(b.Body, "handler")
		fn.Handler = h
		attrs := b.Body.Attributes
		switch {
		case attrs["image_uri"] != nil:
			fn.Unbound = lambdaImage
		case attrs["s3_bucket"] != nil:
			fn.Unbound = lambdaRemote
		case !ok:
			fn.Unbound = lambdaTemplated
		case attrs["filename"] == nil:
			fn.Unbound = lambdaNoFile
		default:
			fn.Unbound = lambdaTemplated
			var a tfArchive
			found := false
			expr := attrs["filename"].Expr
			if w, ok := expr.(*hclsyntax.TemplateWrapExpr); ok {
				expr = w.Wrapped
			}
			if t, diags := hcl.AbsTraversalForExpr(expr); !diags.HasErrors() && len(t) >= 3 && t.RootName() == "data" {
				if s1, ok1 := t[1].(hcl.TraverseAttr); ok1 && s1.Name == "archive_file" {
					if s2, ok2 := t[2].(hcl.TraverseAttr); ok2 {
						a, found = archives[s2.Name]
					}
				}
			} else if p, ok := tfPath(b.Body, "filename", dir); ok {
				fn.Unbound = lambdaArtifact
				for _, name := range sortedArchiveNames(archives) {
					if archives[name].output == p {
						a, found = archives[name], true
						break
					}
				}
			}
			if found {
				switch {
				case a.inline:
					fn.Unbound = lambdaInline
				case a.sourceDir != "":
					fn.CodeDir, fn.Unbound = a.sourceDir, ""
				case a.sourceFile != "":
					fn.Files, fn.Unbound = map[string]string{path.Base(a.sourceFile): a.sourceFile}, ""
				case a.files != nil:
					fn.Files, fn.Unbound = a.files, ""
				default:
					fn.Unbound = lambdaTemplated
				}
			}
		}
		out = append(out, fn)
	}
	return append(out, modules...)
}

func sortedArchiveNames(m map[string]tfArchive) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// tfLiteral is an attribute written as a literal string.
func tfLiteral(body *hclsyntax.Body, name string) (string, bool) {
	a := body.Attributes[name]
	if a == nil {
		return "", false
	}
	if t, ok := a.Expr.(*hclsyntax.TemplateExpr); ok && t.IsStringLiteral() {
		v, diags := t.Value(nil)
		if !diags.HasErrors() && v.Type().FriendlyName() == "string" {
			return v.AsString(), true
		}
	}
	if l, ok := a.Expr.(*hclsyntax.LiteralValueExpr); ok && l.Val.Type().FriendlyName() == "string" {
		return l.Val.AsString(), true
	}
	return "", false
}

// tfPath is a path attribute as a repository path: a literal string or a
// template of literals and path.module, path.root or path.cwd (each the
// module's directory), relative to the module's directory.
func tfPath(body *hclsyntax.Body, name, dir string) (string, bool) {
	a := body.Attributes[name]
	if a == nil {
		return "", false
	}
	var parts []hclsyntax.Expression
	switch e := a.Expr.(type) {
	case *hclsyntax.TemplateExpr:
		parts = e.Parts
	case *hclsyntax.TemplateWrapExpr:
		parts = []hclsyntax.Expression{e.Wrapped}
	default:
		parts = []hclsyntax.Expression{e}
	}
	var b strings.Builder
	for i, p := range parts {
		switch e := p.(type) {
		case *hclsyntax.LiteralValueExpr:
			if e.Val.Type().FriendlyName() != "string" {
				return "", false
			}
			b.WriteString(e.Val.AsString())
		case *hclsyntax.ScopeTraversalExpr:
			t := e.Traversal
			if i != 0 || len(t) != 2 || t.RootName() != "path" {
				return "", false
			}
			b.WriteString("./")
		default:
			return "", false
		}
	}
	v := b.String()
	if v == "" || strings.Contains(v, "://") {
		return "", false
	}
	p := joinRepo(dir, v)
	return p, p != ""
}

// inferRuntime is the runtime family a function whose Runtime is no literal
// (a parameter, a variable) runs, read from its code: the first of python,
// nodejs and ruby whose rule names a file the package holds ("" for none).
func inferRuntime(fn lambdaFn, paths map[string]bool) string {
	h := fn.Handler
	i := strings.LastIndex(h, ".")
	if i <= 0 {
		return ""
	}
	base := path.Base(h)
	j := strings.Index(base, ".")
	if j <= 0 {
		return ""
	}
	has := func(rel string) bool {
		if fn.Files != nil {
			return fn.Files[path.Clean(rel)] != ""
		}
		p := joinRepo(fn.CodeDir, rel)
		return p != "" && paths[p]
	}
	py := strings.ReplaceAll(h[:i], ".", "/")
	if has(py+".py") || has(py+"/__init__.py") {
		return "python"
	}
	mod := strings.TrimSuffix(h, base) + base[:j]
	for _, e := range []string{".js", ".mjs", ".cjs", ".ts", ".mts", ".cts"} {
		if has(mod + e) {
			return "nodejs"
		}
	}
	if has(mod + ".rb") {
		return "ruby"
	}
	return ""
}

// tfFileCall is an attribute written `file(<path>)`, its path as tfPath reads
// one.
func tfFileCall(body *hclsyntax.Body, name, dir string) (string, bool) {
	a := body.Attributes[name]
	if a == nil {
		return "", false
	}
	call, ok := a.Expr.(*hclsyntax.FunctionCallExpr)
	if !ok || call.Name != "file" || len(call.Args) != 1 {
		return "", false
	}
	return tfPath(&hclsyntax.Body{Attributes: hclsyntax.Attributes{"p": &hclsyntax.Attribute{Name: "p", Expr: call.Args[0]}}}, "p", dir)
}

// isLambdaDeclaration reports whether n declares a Lambda function as
// ResolveLambdaHandlers reads one: a CloudFormation resource signed
// AWS::Serverless::Function or AWS::Lambda::Function (its Type), a Terraform
// aws_lambda_function resource, or a Terraform module call whose signature
// names lambda (a function module). It is the doc-xref pass's Lambda gate
// too (EW-57): a code file is that pass's input only where some stored
// symbol is one.
func isLambdaDeclaration(n core.SymbolNode) bool {
	switch n.Kind {
	case "CFNResource":
		return n.Signature == "AWS::Serverless::Function" || n.Signature == "AWS::Lambda::Function"
	case core.KindTerraformResource:
		return strings.HasPrefix(n.Name, "aws_lambda_function.")
	case core.KindTerraformModule:
		return strings.Contains(n.Signature, "lambda")
	}
	return false
}

// lambdaDeclarationKinds are the kinds isLambdaDeclaration can hold for.
var lambdaDeclarationKinds = []core.SourceKind{"CFNResource", core.KindTerraformResource, core.KindTerraformModule}

// serviceDeclaresLambda reports whether service's stored symbols declare a
// Lambda function (isLambdaDeclaration): one read of the declaring kinds'
// rows (store.KindObservationsStore), never the whole symbol set, and never
// a template re-read. A store without that read, or a read that fails,
// answers true — the pass runs, as it did before the gate.
func serviceDeclaresLambda(ctx context.Context, st store.Store, service string) bool {
	ks, ok := st.(store.KindObservationsStore)
	if !ok {
		return true
	}
	obs, err := ks.ObservationsOfKinds(ctx, service, lambdaDeclarationKinds)
	if err != nil {
		return true
	}
	for _, o := range obs {
		if isLambdaDeclaration(core.SymbolNode{Kind: o.SourceKind, Name: o.Symbol, Signature: o.Claim}) {
			return true
		}
	}
	return false
}

// isLambdaHandlerSource: a code file a handler binding may name, whose change
// re-runs the doc-xref pass where the service declares a Lambda function
// (ReingestFile parity: a handler's file renamed or removed is an edge gone;
// EW-57: docXrefPassNeeded, passInvalidator.keysFor).
func isLambdaHandlerSource(p string) bool {
	switch strings.ToLower(path.Ext(p)) {
	case ".py", ".js", ".mjs", ".cjs", ".ts", ".mts", ".cts", ".java", ".rb":
		return true
	}
	return false
}

// lambdaXrefs is the doc-xref pass's CloudWatch and Lambda half (LB-F369,
// LB-F370, LB-F371): the identity joins (identityxref.go) and each function's
// handler symbol, a template, a .tf file or a code file read only when a
// binding names it.
func (p *Pipeline) lambdaXrefs(ctx context.Context, svc core.ServiceRef, files []DocFile, refs []core.SourceRef, idx *siblingIndex) []DocXref {
	xs := ResolveIdentityXrefs(files)
	byPath := make(map[string]core.SourceRef, len(refs))
	paths := make(map[string]bool, len(refs))
	for _, r := range refs {
		byPath[r.Path] = r
		paths[r.Path] = true
	}
	read := func(file string) ([]byte, bool) {
		r, ok := byPath[file]
		if !ok {
			return nil, false
		}
		content, err := p.conn.Fetch(ctx, r)
		if err != nil || len(content) > MaxExtractSize || connector.IsBinary(content) || gateDialectFile(file, content).excluded != "" {
			return nil, false
		}
		return content, true
	}
	symbols := func(file string) []core.SymbolNode {
		content, ok := read(file)
		if !ok {
			return nil
		}
		e, _, ok, _ := p.resolveExtractor(file, content, idx)
		if !ok {
			return nil
		}
		nodes, err := e.Extract(svc.ID, file, content)
		if err != nil {
			return nil
		}
		return nodes
	}
	hs, st := ResolveLambdaHandlers(files, LambdaCode{Paths: paths, Read: read, Symbols: symbols})
	if st.Functions > st.Bound {
		p.diagInfo("fusion: doc-xref pass %q: Lambda handlers: %d functions, %d bound, unbound by reason %v\n", svc.ID, st.Functions, st.Bound, st.Unbound)
	}
	return append(xs, hs...)
}
