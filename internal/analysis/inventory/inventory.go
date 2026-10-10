// Package inventory computes ADR-044 §2.1's capability inventory: every
// item a corpus's file CONTENT offers — a Go exported declaration, a JSON
// wire tag, an HTTP route, an environment-variable read, a proto RPC, a
// cobra CLI verb, or whatever a repo's own `[[surface.pattern]]` entries
// name — as one flat, language-neutral list of {key, kind, file, line,
// detail} items.
//
// Extract runs over file CONTENT alone (File.Path + File.Content), never
// over an index or a git ref: the same function must produce base's
// inventory from `git show <base>:<path>` bytes and head's from worktree
// bytes, with nothing re-ingested at base (docs/design/
// refactor-conservation.md §0, §2.1 — P19). It is pure and deterministic:
// no I/O, no clock, sorted output (internal/AGENTS.md's determinism
// contract).
//
// Where an existing analyzer already does the job over raw source text,
// Extract reuses it rather than re-implementing: internal/analysis/routes
// for `route:`, internal/analysis/envkeys for `env:`. The Go `api:`/`json:`
// kinds and the proto `rpc:`/cobra `cli:` kinds have no existing
// content-only extractor (internal/analysis/apisurface operates on
// already-indexed []core.SymbolNode, which base content never has — P19 —
// so this package cannot reuse it directly; its export-visibility rule is
// mirrored here instead, over a fresh go/ast parse of the same file).
package inventory

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"path"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/emicklei/proto"
	"github.com/geoffmilleraz/fathom/internal/analysis/envkeys"
	"github.com/geoffmilleraz/fathom/internal/analysis/routes"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/filter"
)

// File is one source file's content, exactly as a base or head read
// produces it (git show / worktree read) — Extract never does I/O of its
// own, so the identical function runs on base and head content.
type File struct {
	Path    string
	Content []byte
}

// Pattern is one repo-configured `[[surface.pattern]]` entry (design doc
// §2.1): Glob scopes which files it applies to (internal/filter's
// gitignore-style glob syntax, "" meaning every file), Regex's first
// capturing group names the matched item, and Kind is the "<kind>:" prefix
// every match is emitted under. A caller with internal/config.SurfacePattern
// values converts them 1:1 — this package stays free of any config/TOML
// dependency so it can be unit tested, and run over base content, without
// one.
type Pattern struct {
	Kind  string
	Glob  string
	Regex string
}

// Item is one capability-inventory entry (design doc §2.1's table). Key is
// the wire identity ("<kind>:<name>") Diff compares across base and head;
// Detail is whatever else about the item changing should count as
// "changed" rather than merely "present at both" — a stripped function
// signature, a JSON field's Go type, a route handler name.
type Item struct {
	Key    string
	Kind   string
	File   string
	Line   int
	Detail string
	// Type carries structural detail for a Go "api:" TYPE declaration item
	// (apicompat.go, CV-M4, ADR-044 amendment 2026-09-24) — nil for every
	// func/method/const/var item and for every non-Go kind. Diff uses it,
	// when present on both sides of a key, to apply apidiff's own
	// compatible/incompatible rules instead of a whole-text comparison;
	// with either side nil it falls back to plain Detail equality, the
	// design doc's own "normalized textual form" clause.
	Type *TypeDetail
}

// Extract returns every capability-inventory item found across files, plus
// whatever patterns' configured-pattern matches — sorted by Key and
// deduplicated (a key produced twice, e.g. the same route registered in
// two files, keeps its first occurrence in (File, Line) order). Files
// whose name ends "_test.go" are skipped entirely: test code is never
// part of the capability surface a refactor must conserve, for every kind,
// not merely the Go-specific ones (design doc §2.1's table implicitly
// assumes production code; this is the one rule Extract adds beyond it,
// applied uniformly rather than kind by kind).
func Extract(files []File, patterns []Pattern) []Item {
	// Filter _test.go out ONCE, up front: every kind below — including
	// the configured-pattern kind — must see the identical file set, or a
	// pattern like CV-26's own `tool:` glob (internal/mcp/*.go) silently
	// picks up every *_test.go fixture's own toolDescriptor literal too
	// (a real bug this exact fix closed: a test-only fake tool name
	// otherwise showed up as a surface item indistinguishable from a real
	// registration).
	var prod []File
	for _, f := range files {
		if strings.HasSuffix(f.Path, "_test.go") {
			continue
		}
		prod = append(prod, f)
	}

	var out []Item
	var routeFiles []routes.Result
	for _, f := range prod {
		switch {
		case strings.HasSuffix(f.Path, ".go"):
			out = append(out, extractGoAPI(f)...)
			out = append(out, extractGoJSON(f)...)
			out = append(out, extractGoCLI(f)...)
		case strings.HasSuffix(f.Path, ".proto"):
			out = append(out, extractProtoRPC(f)...)
		}
		if lang := routeLangFor(f.Path); lang != "" {
			routeFiles = append(routeFiles, routes.ExtractFile(lang, f.Path, f.Content))
		}
		out = append(out, extractEnv(f)...)
	}
	out = append(out, routeItems(routeFiles)...)
	for _, p := range patterns {
		out = append(out, extractConfigured(prod, p)...)
	}
	return sortDedupe(out)
}

func sortDedupe(items []Item) []Item {
	sort.Slice(items, func(i, j int) bool {
		if items[i].File != items[j].File {
			return items[i].File < items[j].File
		}
		if items[i].Line != items[j].Line {
			return items[i].Line < items[j].Line
		}
		return items[i].Key < items[j].Key
	})
	seen := make(map[string]bool, len(items))
	out := make([]Item, 0, len(items))
	for _, it := range items {
		if it.Key == "" || seen[it.Key] {
			continue
		}
		seen[it.Key] = true
		out = append(out, it)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// Delta is Extract's base→head comparison (design doc §2.2): Removed and
// Changed are both losses (a caller excuses them via a plan's retires:/
// surface_change: declaration, I45); Added is never a loss. All three are
// sorted.
type Delta struct {
	Removed []string
	Changed []string
	Added   []string
}

// Diff computes base→head's Delta by Item.Key, comparing each key present
// at both sides via itemChanged to decide removed/changed/added —
// apisurface.Diff's own three-way split, generalised from indexed Go
// symbols to every inventory kind. For a Go "api:" TYPE declaration
// present on both sides, itemChanged applies apidiff's own compatibility
// rules (apicompat.go, CV-M4) instead of a whole-text comparison, so an
// apidiff-compatible edit (an additive struct field, an any/interface{}
// respelling) is never reported changed; every other item kind keeps the
// original plain-Detail-equality rule.
func Diff(base, head []Item) Delta {
	baseByKey := make(map[string]Item, len(base))
	for _, it := range base {
		baseByKey[it.Key] = it
	}
	headByKey := make(map[string]Item, len(head))
	for _, it := range head {
		headByKey[it.Key] = it
	}

	var d Delta
	for k := range baseByKey {
		if _, ok := headByKey[k]; !ok {
			d.Removed = append(d.Removed, k)
		}
	}
	for k, hi := range headByKey {
		bi, ok := baseByKey[k]
		if !ok {
			d.Added = append(d.Added, k)
		} else if itemChanged(bi, hi) {
			d.Changed = append(d.Changed, k)
		}
	}
	sort.Strings(d.Removed)
	sort.Strings(d.Changed)
	sort.Strings(d.Added)
	return d
}

// ─── api: / json: / cli: (Go, go/ast) ──────────────────────────────────────

// extractGoAPI returns "api:<importpath>.<Name>" / "api:<importpath>.
// <Type>.<Method>" items for every exported top-level func, method, type,
// const and var in f — skipping package "main" and any package under an
// "internal/" directory component (design doc §2.1), the same visibility
// rule internal/analysis/apisurface applies to already-indexed symbols,
// re-derived here over content go/ast never had a chance to index (base
// content, P19). importpath is f's own repo-relative directory (path.Dir):
// this package has no go.mod/module-path knowledge (it is handed file
// content only, sometimes from a commit where go.mod itself may have
// looked different), so the directory is the closest stable,
// content-derivable analogue — sufficient to tell "this package's Foo" from
// "that package's Foo" across a diff, which is Extract's whole job.
func extractGoAPI(f File) []Item {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, f.Path, f.Content, 0)
	if err != nil {
		return nil
	}
	if file.Name == nil || file.Name.Name == "main" {
		return nil
	}
	dir := path.Dir(f.Path)
	if isInternalDir(dir) {
		return nil
	}

	litKinds := fileLiteralValueKinds(file)

	var out []Item
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			out = append(out, goFuncItem(fset, f.Path, dir, d)...)
		case *ast.GenDecl:
			out = append(out, goGenDeclItems(fset, f.Path, dir, d, litKinds)...)
		}
	}
	return out
}

func goFuncItem(fset *token.FileSet, filePath, dir string, d *ast.FuncDecl) []Item {
	if d.Name == nil || !d.Name.IsExported() {
		return nil
	}
	line := fset.Position(d.Pos()).Line
	sig := strippedSignature(fset, d.Type)
	if d.Recv == nil || len(d.Recv.List) == 0 {
		return []Item{{
			Key: fmt.Sprintf("api:%s.%s", dir, d.Name.Name), Kind: "api",
			File: filePath, Line: line, Detail: sig,
		}}
	}
	recv := receiverTypeName(d.Recv.List[0].Type)
	if recv == "" || !isExportedName(recv) {
		return nil
	}
	return []Item{{
		Key: fmt.Sprintf("api:%s.%s.%s", dir, recv, d.Name.Name), Kind: "api",
		File: filePath, Line: line, Detail: sig,
	}}
}

func goGenDeclItems(fset *token.FileSet, filePath, dir string, d *ast.GenDecl, litKinds map[string]string) []Item {
	var out []Item
	for _, spec := range d.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			if s.Name == nil || !s.Name.IsExported() {
				continue
			}
			out = append(out, Item{
				Key: fmt.Sprintf("api:%s.%s", dir, s.Name.Name), Kind: "api",
				File: filePath, Line: fset.Position(s.Pos()).Line,
				Detail: renderNode(fset, s.Type),
				Type:   typeDetailFor(fset, s),
			})
		case *ast.ValueSpec:
			for i, name := range s.Names {
				if name == nil || !name.IsExported() {
					continue
				}
				detail := ""
				switch {
				case s.Type != nil:
					detail = renderNode(fset, s.Type)
				case i < len(s.Values):
					detail = valueDetail(fset, d.Tok, s.Values[i], litKinds)
				}
				out = append(out, Item{
					Key: fmt.Sprintf("api:%s.%s", dir, name.Name), Kind: "api",
					File: filePath, Line: fset.Position(name.Pos()).Line, Detail: detail,
				})
			}
		}
	}
	return out
}

// extractGoJSON returns "json:<importpath>.<Type>.<tag>" items for every
// JSON-tagged exported field of an exported struct type in f (design doc
// §2.1) — same package-visibility exclusion as extractGoAPI. A field with
// no `json:"..."` tag, or tagged `json:"-"`, is never emitted: encoding/
// json's implicit field-name default is not a declared wire surface.
func extractGoJSON(f File) []Item {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, f.Path, f.Content, 0)
	if err != nil {
		return nil
	}
	if file.Name == nil || file.Name.Name == "main" {
		return nil
	}
	dir := path.Dir(f.Path)
	if isInternalDir(dir) {
		return nil
	}

	var out []Item
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name == nil || !ts.Name.IsExported() {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok || st.Fields == nil {
				continue
			}
			for _, field := range st.Fields.List {
				if field.Tag == nil {
					continue
				}
				tagVal, err := strconv.Unquote(field.Tag.Value)
				if err != nil {
					continue
				}
				jsonTag := reflect.StructTag(tagVal).Get("json")
				name, _, _ := strings.Cut(jsonTag, ",")
				if name == "" || name == "-" {
					continue
				}
				detail := renderNode(fset, field.Type)
				line := fset.Position(field.Pos()).Line

				if len(field.Names) == 0 {
					// An embedded (anonymous) field carrying an explicit
					// json tag is NOT flattened/promoted — encoding/json's
					// own documented rule treats it as a normal named
					// field under that tag, so it is part of the wire
					// surface exactly like any other tagged field. Its Go
					// field name (and thus its exportedness) is the
					// embedded type's own base identifier.
					if fname := embeddedFieldName(field.Type); fname != "" && isExportedName(fname) {
						out = append(out, Item{
							Key:  fmt.Sprintf("json:%s.%s.%s", dir, ts.Name.Name, name),
							Kind: "json", File: f.Path, Line: line, Detail: detail,
						})
					}
					continue
				}
				for _, fname := range field.Names {
					if !fname.IsExported() {
						continue
					}
					out = append(out, Item{
						Key:  fmt.Sprintf("json:%s.%s.%s", dir, ts.Name.Name, name),
						Kind: "json", File: f.Path, Line: line, Detail: detail,
					})
				}
			}
		}
	}
	return out
}

// extractGoCLI returns "cli:<verb>" items for every cobra `Command{Use:
// "verb ..."}` composite literal in f (design doc §2.1) — the first
// whitespace-delimited token of the Use string is the verb, cobra's own
// convention ("list [flags]" names verb "list"). Recognizes both
// `cobra.Command{...}` (a package-qualified selector whose Sel.Name is
// "Command") and a bare `Command{...}` (a dot-import) — never anything
// else named "Command", but a false positive there would need an unrelated
// type named exactly "Command" with a field also named exactly "Use" set
// to a string literal, which this repo's own convention (declareVerb,
// cmd/fathom/verbs.go) avoids entirely by not using cobra at all.
func extractGoCLI(f File) []Item {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, f.Path, f.Content, 0)
	if err != nil {
		return nil
	}
	var out []Item
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok || !isCobraCommandType(lit.Type) {
			return true
		}
		for _, elt := range lit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, ok := kv.Key.(*ast.Ident)
			if !ok || key.Name != "Use" {
				continue
			}
			bl, ok := kv.Value.(*ast.BasicLit)
			if !ok || bl.Kind != token.STRING {
				continue
			}
			raw, uerr := strconv.Unquote(bl.Value)
			if uerr != nil {
				continue
			}
			fields := strings.Fields(raw)
			if len(fields) == 0 {
				continue
			}
			out = append(out, Item{
				Key: "cli:" + fields[0], Kind: "cli",
				File: f.Path, Line: fset.Position(kv.Pos()).Line,
			})
		}
		return true
	})
	return out
}

func isCobraCommandType(e ast.Expr) bool {
	switch t := e.(type) {
	case *ast.SelectorExpr:
		return t.Sel != nil && t.Sel.Name == "Command"
	case *ast.Ident:
		return t.Name == "Command"
	}
	return false
}

// ─── rpc: (.proto, github.com/emicklei/proto) ──────────────────────────────

// extractProtoRPC returns "rpc:<package>.<Service>/<Method>" items for
// every RPC method of every service declared in a .proto file (design doc
// §2.1). package is the file's own `package foo.bar;` statement, when
// present; omitted (bare "<Service>/<Method>") when the file declares
// none. A file that fails to parse contributes nothing, the same
// "skip unparseable" discipline internal/analysis/routes documents.
func extractProtoRPC(f File) []Item {
	def, err := proto.NewParser(bytes.NewReader(f.Content)).Parse()
	if err != nil || def == nil {
		return nil
	}
	pkgName := ""
	for _, e := range def.Elements {
		if p, ok := e.(*proto.Package); ok {
			pkgName = p.Name
			break
		}
	}
	var out []Item
	for _, e := range def.Elements {
		svc, ok := e.(*proto.Service)
		if !ok {
			continue
		}
		svcName := svc.Name
		if pkgName != "" {
			svcName = pkgName + "." + svc.Name
		}
		for _, se := range svc.Elements {
			rpc, ok := se.(*proto.RPC)
			if !ok {
				continue
			}
			out = append(out, Item{
				Key:  fmt.Sprintf("rpc:%s/%s", svcName, rpc.Name),
				Kind: "rpc", File: f.Path, Line: rpc.Position.Line,
				Detail: rpc.RequestType + " -> " + rpc.ReturnsType,
			})
		}
	}
	return out
}

// ─── route: / env: (reused analyzers) ──────────────────────────────────────

// routeItems turns every file's route reading into route: items, after
// routes.Compose has applied the mounts across files (the route seam,
// LB-F321): a route is keyed at the path it is served on, and a mount is
// never an item. internal/analysis/routes is reused verbatim — design doc
// §2.1's explicit "reusing internal/analysis/routes, envkeys ... where they
// fit"; it needs no store or index, so base content runs through it exactly
// like head content.
func routeItems(files []routes.Result) []Item {
	var out []Item
	for _, r := range routes.Compose(files) {
		out = append(out, Item{
			Key:  fmt.Sprintf("route:%s %s", r.Method, r.Path),
			Kind: "route", File: r.File, Line: r.Line, Detail: r.Handler,
		})
	}
	return out
}

// extractEnv reuses internal/analysis/envkeys.Scan verbatim over f's
// content, for the languages it has always read (Go, JS/TS, Python).
func extractEnv(f File) []Item {
	if !envScanned(f.Path) {
		return nil
	}
	var out []Item
	for _, k := range envkeys.Scan(f.Path, string(f.Content)) {
		out = append(out, Item{
			Key: "env:" + k.Name, Kind: "env", File: k.File, Line: k.Line,
		})
	}
	return out
}

// routeLangFor is the route-reader language of p (connector's token), or
// "" when no route reader reads it (routes.HasReader).
func routeLangFor(p string) string {
	if lang := connector.LanguageForPath(p); routes.HasReader(lang) {
		return lang
	}
	return ""
}

// envScanned reports whether envkeys.Scan reads p: the file kinds the
// inventory has always scanned for env: items.
func envScanned(p string) bool {
	for _, ext := range []string{".go", ".ts", ".tsx", ".js", ".jsx", ".mjs", ".py"} {
		if strings.HasSuffix(p, ext) {
			return true
		}
	}
	return false
}

// ─── configured `[[surface.pattern]]` kinds ────────────────────────────────

// extractConfigured returns "<p.Kind>:<name>" items for every match of
// p.Regex's first capturing group across files whose path matches p.Glob
// (every file when p.Glob is "") — design doc §2.1's "configured patterns"
// row, this repo's own mechanism for a `tool:`/`verb:` surface no built-in
// kind covers. A malformed regex, or a pattern naming no capture group,
// yields no items rather than erroring — Extract itself never fails.
func extractConfigured(files []File, p Pattern) []Item {
	if p.Kind == "" || p.Regex == "" {
		return nil
	}
	re, err := regexp.Compile(p.Regex)
	if err != nil || re.NumSubexp() < 1 {
		return nil
	}
	var m *filter.Matcher
	if p.Glob != "" {
		m = filter.New([]string{p.Glob})
	}
	var out []Item
	for _, f := range files {
		if m != nil && !m.Match(f.Path) {
			continue
		}
		for _, loc := range re.FindAllSubmatchIndex(f.Content, -1) {
			if len(loc) < 4 || loc[2] < 0 {
				continue
			}
			name := string(f.Content[loc[2]:loc[3]])
			if name == "" {
				continue
			}
			out = append(out, Item{
				Key: p.Kind + ":" + name, Kind: p.Kind,
				File: f.Path, Line: lineForOffset(f.Content, loc[0]),
			})
		}
	}
	return out
}

func lineForOffset(content []byte, offset int) int {
	if offset > len(content) {
		offset = len(content)
	}
	return 1 + bytes.Count(content[:offset], []byte("\n"))
}

// ─── small shared helpers ───────────────────────────────────────────────────

func isInternalDir(dir string) bool {
	for _, seg := range strings.Split(dir, "/") {
		if seg == "internal" {
			return true
		}
	}
	return false
}

func isExportedName(name string) bool {
	if name == "" {
		return false
	}
	r := []rune(name)
	return unicode.IsUpper(r[0])
}

// receiverTypeName extracts a method receiver's base type name, stripping
// pointer indirection and generic type parameters — mirrors internal/
// extractor's own unexported receiverTypeName (that package's copy is not
// importable; the logic is small enough to duplicate rather than export
// it purely for this one caller).
func receiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return receiverTypeName(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		return receiverTypeName(t.X)
	case *ast.IndexListExpr:
		return receiverTypeName(t.X)
	}
	return ""
}

// embeddedFieldName returns an embedded (anonymous) struct field's Go
// field name — the identifier encoding/json (and every other consumer of
// that field's declared name) uses for it — stripping pointer indirection,
// a package qualifier (an embedded type from another package, `pkg.Foo`,
// names its field `Foo`, never `pkg.Foo`), and generic type arguments.
// Unlike receiverTypeName (a method receiver can never be package-
// qualified — Go disallows it), an embedded field can be, so this handles
// *ast.SelectorExpr too. "" when expr is not a shape an embedded field's
// type can take.
func embeddedFieldName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return embeddedFieldName(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return t.Sel.Name
	case *ast.IndexExpr:
		return embeddedFieldName(t.X)
	case *ast.IndexListExpr:
		return embeddedFieldName(t.X)
	}
	return ""
}

// renderNode renders e in canonical Go syntax via go/printer — used for
// type expressions (a struct field's type, a type declaration's
// underlying type) where, unlike a function signature, parameter names are
// not in play and the expression alone is already exactly what should
// change detail when a refactor changes it. e is normalized first
// (normalizeAnyType, apicompat.go CV-M4 class A) so `any` and `interface{}`
// always render identically — the same collapse a go/types identity check
// would give for free, done here at the AST level instead.
func renderNode(fset *token.FileSet, e ast.Expr) string {
	e = normalizeAnyType(e)
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, e); err != nil {
		return ""
	}
	return buf.String()
}

// strippedSignature renders ft as "func(<types>) (<types>)" with every
// parameter and result NAME removed — design doc §2.1: "detail is the
// signature with parameter names removed", so a pure parameter rename
// never shows as api: "changed". Cardinality is preserved: a grouped field
// `a, b int` (two names, one type) still renders as two comma-separated
// occurrences of "int", exactly like the named signature would parse as
// two parameters.
func strippedSignature(fset *token.FileSet, ft *ast.FuncType) string {
	params := strippedFieldList(fset, ft.Params)
	if ft.Results == nil || len(ft.Results.List) == 0 {
		return "func(" + params + ")"
	}
	results := strippedFieldList(fset, ft.Results)
	if len(ft.Results.List) == 1 && len(ft.Results.List[0].Names) == 0 {
		return "func(" + params + ") " + results
	}
	return "func(" + params + ") (" + results + ")"
}

func strippedFieldList(fset *token.FileSet, fl *ast.FieldList) string {
	if fl == nil {
		return ""
	}
	var parts []string
	for _, f := range fl.List {
		t := renderNode(fset, f.Type)
		n := len(f.Names)
		if n == 0 {
			n = 1
		}
		for i := 0; i < n; i++ {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, ", ")
}
