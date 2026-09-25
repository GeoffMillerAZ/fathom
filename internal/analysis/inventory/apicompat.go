// apicompat.go — CV-M4's fix (ADR-044 amendment 2026-09-24, docs/bench/
// readings/cv-m4-apidiff-calibration-2026-09-24.md): make the `api:` kind's
// removed/changed classification agree with golang.org/x/exp/apidiff's own
// compatible/incompatible split for the three measured disagreement
// classes, without a go/types pass.
//
// Why AST identity, not go/types, is the right tier here: all three classes
// apidiff and fathom disagreed on — `any`/`interface{}` spelling, additive
// struct fields, and a type-alias introduction — are facts about how ONE
// package's OWN declaration is written, never about resolving another
// package's exported API. `any` is Go's predeclared alias for the empty
// interface (a lexical fact); a struct's or interface's own field/method
// set is syntax the declaring file already carries; `type X = T` vs
// `type X T` is the parser's own Assign token. None of that needs an
// imported package's types resolved, so a go/types pass over the changed
// package's files (let alone golang.org/x/tools/go/packages' transitive
// module load — the very cost apidiff itself pays, ~7-75s/pair on this
// calibration corpus) would buy nothing here and would break P19's "cost
// scales with the change" rule the moment a changed package's import graph
// is large. Where a future disagreement class genuinely needs cross-
// package resolution (e.g. Go's numeric-widening compatibility table,
// generic instantiation identity), that is real, un-covered scope for a
// later reading — not something this file claims to handle.
//
// TypeDetail is populated ONLY for a Go "api:" TYPE declaration item
// (extractGoAPI's *ast.TypeSpec case); it is nil for every func/method/
// const/var item and for every non-Go kind, and Diff falls back to plain
// normalized-Detail-text comparison whenever either side lacks one — the
// design doc's own "fall back to a normalized textual form" clause.
package inventory

import (
	"go/ast"
	"go/token"
)

// TypeDetail is one Go "api:" type declaration's structure, sufficient to
// apply apidiff's own compatibility rules (compatibility.go's
// checkCompatibleStruct/checkCompatibleInterface) between a base and head
// Item sharing the same Key, entirely from two independent AST parses (no
// cross-file, no cross-package resolution).
type TypeDetail struct {
	// Alias is true for `type X = T` (the parser's Assign token present),
	// false for `type X T`. apidiff's own correspondence check
	// (establishCorrespondence) only lets a *types.Named correspond to
	// another *types.Named or a *types.Basic; an alias whose RHS is a
	// composite type (a func/struct/interface/slice/... literal, never
	// itself Named) makes the two SIDES different kinds of type outright,
	// which apidiff always reports incompatible ("changed from X to
	// <RHS>") — so a flip of this bit, either direction, is always a loss
	// here too, whatever the RHS text does (CV-M4 class C).
	Alias bool
	// Struct is non-nil when the declaration's underlying (or, for an
	// alias, its immediate RHS) is a struct literal: exported field name
	// -> normalized type text. Anonymous (embedded) fields are keyed by
	// their own base identifier (embeddedFieldName), matching
	// extractGoJSON's convention. Field TAGS are never part of this map —
	// apidiff's own comment on checkCompatibleStruct: "Field tags are
	// ignored: they have no compile-time implications" (the `json:`
	// inventory kind already tracks tag changes separately).
	Struct map[string]string
	// Interface is non-nil when the declaration's underlying (or alias
	// RHS) is an interface literal: method name -> normalized signature
	// text, PLUS one entry per embedded interface field (a Field with no
	// Name whose Type is a plain, possibly package-qualified, possibly
	// generic-instantiated named type — never a type-set element, for
	// which embeddedFieldName returns "" and the field is skipped),
	// keyed "embed <base-identifier>" (a space can never appear in a Go
	// identifier, so this key space never collides with a real method
	// name) and valued by the embed's own rendered type text
	// (interfaceMethodShapes). This makes an embed being added, removed,
	// or retargeted register as a change to THIS interface's own shape,
	// via the exact same comparison interfaceCompatible already applies
	// to a named method. It does NOT flatten the embedded interface's
	// OWN method set into this map — this package never resolves another
	// declaration's shape — so a change reachable ONLY by looking inside
	// what the embed itself requires (rather than at the embed's own
	// identity) is a documented gap, same tier as the struct
	// "comparable" regression apidiff also checks and this file does not
	// (both need resolving a THIRD declaration's shape, not just the two
	// sides of the one being compared).
	Interface map[string]string
	// InterfaceHasUnexportedMethod is true when Interface's own BASE side
	// already declares a method whose name is unexported. apidiff's own
	// asymmetric rule (compatibility.go checkCompatibleInterface): when
	// true, no outside package could implement the interface anyway, so
	// ANY method addition (exported or not) is compatible; when false,
	// adding ANY method — even an exported one — breaks an external
	// implementer's method set and is incompatible.
	InterfaceHasUnexportedMethod bool
	// Other is the normalized rendered text of the underlying type (or
	// alias RHS) for every declaration whose Struct and Interface are
	// both nil — a named basic/slice/array/map/chan/func/pointer type.
	// Compared as plain text, same tier as every non-type api: item.
	Other string
}

// typeDetailFor builds s's TypeDetail from its own AST alone.
func typeDetailFor(fset *token.FileSet, s *ast.TypeSpec) *TypeDetail {
	td := &TypeDetail{Alias: s.Assign != token.NoPos}
	switch t := s.Type.(type) {
	case *ast.StructType:
		td.Struct = structFieldShapes(fset, t)
	case *ast.InterfaceType:
		td.Interface, td.InterfaceHasUnexportedMethod = interfaceMethodShapes(fset, t)
	default:
		td.Other = renderNode(fset, s.Type)
	}
	return td
}

// structFieldShapes returns st's own immediate exported fields (never
// fields promoted through embedding — apidiff's "exported *selectable*
// fields" pass is a documented gap here, see TypeDetail.Interface's own
// comment for why) keyed by Go field name, valued by normalized type text.
func structFieldShapes(fset *token.FileSet, st *ast.StructType) map[string]string {
	out := map[string]string{}
	if st.Fields == nil {
		return out
	}
	for _, field := range st.Fields.List {
		typeText := renderNode(fset, field.Type)
		if len(field.Names) == 0 {
			if fname := embeddedFieldName(field.Type); fname != "" && isExportedName(fname) {
				out[fname] = typeText
			}
			continue
		}
		for _, name := range field.Names {
			if name.IsExported() {
				out[name.Name] = typeText
			}
		}
	}
	return out
}

// interfaceMethodShapes returns its own immediate named methods keyed by
// method name, valued by normalized stripped-parameter-name signature
// text, PLUS one "embed <base-identifier>" entry per embedded interface
// field (TypeDetail.Interface's own doc comment) — a type-set element
// (union `A | B`, tilde `~T`) is still skipped: it is also a Field with no
// Name, but embeddedFieldName returns "" for it, so no key is emitted —
// plus whether any NAMED method is unexported.
func interfaceMethodShapes(fset *token.FileSet, it *ast.InterfaceType) (map[string]string, bool) {
	out := map[string]string{}
	hasUnexported := false
	if it.Methods == nil {
		return out, false
	}
	for _, m := range it.Methods.List {
		if len(m.Names) != 1 {
			// An embedded interface or a type-set element — neither has
			// a Name. embeddedFieldName resolves the former to its base
			// identifier (mirroring structFieldShapes' own treatment of
			// an embedded struct field) and returns "" for the latter
			// (a *ast.BinaryExpr union or *ast.UnaryExpr tilde matches
			// none of its cases), so a type-set element is correctly
			// left untracked here.
			if fname := embeddedFieldName(m.Type); fname != "" {
				out["embed "+fname] = renderNode(fset, m.Type)
			}
			continue
		}
		sig, ok := m.Type.(*ast.FuncType)
		if !ok {
			continue
		}
		name := m.Names[0]
		out[name.Name] = strippedSignature(fset, sig)
		if !name.IsExported() {
			hasUnexported = true
		}
	}
	return out, hasUnexported
}

// itemChanged reports whether head differs from base in a way apidiff
// would call incompatible. Both items already share the same Key (Diff's
// own caller-side invariant); Kind and Type decide which comparison tier
// applies:
//   - both sides carry a Go "api:" TypeDetail: typesCompatible's
//     struct/interface/alias rules (this file's whole point).
//   - anything else (a func/method/const/var api: item, or any other
//     kind entirely): the design doc's own textual fallback — plain
//     Detail equality, now over normalized text (renderNode's own
//     any/interface{} collapse already ran when Detail was built, so this
//     also fixes CV-M4 class A for every non-type item, e.g. a func
//     signature, with no TypeDetail involved at all).
func itemChanged(base, head Item) bool {
	if base.Kind == "api" && head.Kind == "api" && base.Type != nil && head.Type != nil {
		return !typesCompatible(base.Type, head.Type)
	}
	return base.Detail != head.Detail
}

// typesCompatible reports whether apidiff would call base->head compatible
// for a Go type declaration — never incompatible-classifying more than
// apidiff's own compatibility.go rules do.
func typesCompatible(base, head *TypeDetail) bool {
	if base.Alias != head.Alias {
		return false // a defined type and an alias are different type kinds (CV-M4 class C)
	}
	switch {
	case base.Struct != nil && head.Struct != nil:
		return structCompatible(base.Struct, head.Struct)
	case base.Interface != nil && head.Interface != nil:
		return interfaceCompatible(base, head)
	case base.Struct == nil && base.Interface == nil && head.Struct == nil && head.Interface == nil:
		return base.Other == head.Other
	default:
		return false // the underlying kind itself changed (e.g. struct -> non-struct)
	}
}

// structCompatible applies apidiff's checkCompatibleObjectSets rule for a
// struct's own exported fields: removing an existing field, or changing
// its type, is incompatible; adding a new one is not (CV-M4 class B).
func structCompatible(base, head map[string]string) bool {
	for name, bt := range base {
		ht, ok := head[name]
		if !ok || ht != bt {
			return false
		}
	}
	return true
}

// interfaceCompatible applies apidiff's checkCompatibleInterface rule:
// removing or changing an existing method is always incompatible; adding
// one is compatible only when base already had an unexported method
// (nobody outside the package could implement it anyway), otherwise an
// addition — exported or not — is incompatible (it would break an
// external implementer's method set).
func interfaceCompatible(base, head *TypeDetail) bool {
	for name, bt := range base.Interface {
		ht, ok := head.Interface[name]
		if !ok || ht != bt {
			return false
		}
	}
	if base.InterfaceHasUnexportedMethod {
		return true
	}
	for name := range head.Interface {
		if _, ok := base.Interface[name]; !ok {
			return false
		}
	}
	return true
}

// normalizeAnyType returns e with every predeclared `any` identifier
// rewritten to the canonical empty-interface literal `interface{}` — the
// reverse spelling (`interface{}` written out) already renders that way,
// so this is enough to make renderNode/strippedSignature collapse both
// spellings to one text (CV-M4 class A: cli/cli's `any` modernization
// sweep). Only ever recurses into positions that are themselves type
// expressions; a *ast.SelectorExpr's own X (package qualifier) and Sel
// (the other package's exported name — which can never be "any": Go
// identifiers reachable through a selector are always exported, and "any"
// is lowercase) are deliberately left untouched, so an import literally
// aliased "any" is never mistaken for the predeclared identifier.
func normalizeAnyType(e ast.Expr) ast.Expr {
	switch t := e.(type) {
	case nil:
		return nil
	case *ast.Ident:
		if t.Name == "any" {
			return &ast.InterfaceType{Methods: &ast.FieldList{}}
		}
		return t
	case *ast.StarExpr:
		return &ast.StarExpr{X: normalizeAnyType(t.X)}
	case *ast.ParenExpr:
		return &ast.ParenExpr{X: normalizeAnyType(t.X)}
	case *ast.ArrayType:
		return &ast.ArrayType{Len: t.Len, Elt: normalizeAnyType(t.Elt)}
	case *ast.Ellipsis:
		return &ast.Ellipsis{Elt: normalizeAnyType(t.Elt)}
	case *ast.MapType:
		return &ast.MapType{Key: normalizeAnyType(t.Key), Value: normalizeAnyType(t.Value)}
	case *ast.ChanType:
		return &ast.ChanType{Dir: t.Dir, Value: normalizeAnyType(t.Value)}
	case *ast.FuncType:
		return &ast.FuncType{
			TypeParams: t.TypeParams,
			Params:     normalizeFieldListTypes(t.Params),
			Results:    normalizeFieldListTypes(t.Results),
		}
	case *ast.FuncLit:
		// A `var Foo = func(w interface{}) ... { ... }` (no explicit
		// `s.Type`, cli/cli's own utils.TerminalSize) renders its VALUE,
		// the whole func literal, as Detail — normalize only its Type
		// (the part any/interface{} identity is about); Body is kept
		// exactly as parsed, never walked here.
		ft, _ := normalizeAnyType(t.Type).(*ast.FuncType)
		return &ast.FuncLit{Type: ft, Body: t.Body}
	case *ast.StructType:
		return &ast.StructType{Fields: normalizeFieldListTypes(t.Fields)}
	case *ast.InterfaceType:
		return &ast.InterfaceType{Methods: normalizeFieldListTypes(t.Methods)}
	case *ast.IndexExpr:
		return &ast.IndexExpr{X: normalizeAnyType(t.X), Index: normalizeAnyType(t.Index)}
	case *ast.IndexListExpr:
		idxs := make([]ast.Expr, len(t.Indices))
		for i, idx := range t.Indices {
			idxs[i] = normalizeAnyType(idx)
		}
		return &ast.IndexListExpr{X: normalizeAnyType(t.X), Indices: idxs}
	default:
		return e
	}
}

// valueDetail returns a *ast.ValueSpec value expression's Detail text.
// apidiff's own `checkObjects` (apidiff.go) only ever compares a
// `*types.Var`'s TYPE (`checkCorrespondence(..., old.Type(), new.Type())`)
// — never its initializer expression — so a `var` with no explicit
// `Type:` whose initializer is a boolean literal, another basic literal,
// or a same-file identifier resolving to one (litKinds, built once per
// file by fileLiteralValueKinds) renders as that inferred KIND rather
// than the literal source text: `var X = true` and a later `var X =
// someBoolConst` are the identical `bool`-typed var to apidiff, however
// differently they are spelled (a real gap this reading's full 50-pair
// run found: cobra v1.5.0->v1.6.0 promoted `true`/`false` literals to
// named constants for its own `EnableCommandSorting`/
// `EnablePrefixMatching` vars — apidiff: compatible; a textual diff:
// "changed"). A `const`, unlike a `var`, keeps its literal text (CV-M4
// scoped this fix to what the discovered false positive needed — a
// const's actual VALUE, not just its type, is part of apidiff's own
// compatibility check, `constChanges`'s `constant.Compare`, so collapsing
// two literally-different const values here would trade one gap for a
// worse one). Anything valueDetail cannot confidently classify (a call, a
// composite literal, an unresolved identifier) falls back to the
// initializer's own rendered text, same as before this fix.
func valueDetail(fset *token.FileSet, tok token.Token, val ast.Expr, litKinds map[string]string) string {
	if tok == token.VAR {
		if kind, ok := resolveBasicKind(val, litKinds); ok {
			return "var:" + kind
		}
	}
	return renderNode(fset, val)
}

// fileLiteralValueKinds returns, for every top-level const/var name in
// file whose own value is directly a boolean identifier or a basic
// literal, that literal's kind — a ONE-HOP table valueDetail consults so
// a var initialized from such a same-file identifier (rather than the
// literal itself) still resolves; an identifier initialized from
// something else (another identifier, a call, ...) is simply absent from
// the table, and valueDetail's caller falls back to rendered text for it,
// same as an unresolvable reference always has.
func fileLiteralValueKinds(file *ast.File) map[string]string {
	out := map[string]string{}
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || (gd.Tok != token.CONST && gd.Tok != token.VAR) {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range vs.Names {
				if name == nil || i >= len(vs.Values) {
					continue
				}
				if kind, ok := resolveBasicKind(vs.Values[i], nil); ok {
					out[name.Name] = kind
				}
			}
		}
	}
	return out
}

// resolveBasicKind reports val's basic kind — "bool" for the predeclared
// true/false identifiers or, when litKinds is given, a same-file
// identifier already resolved to one; "int"/"float"/"string"/"rune"/
// "complex" for the matching literal token kind. ok is false for anything
// else (litKinds may be nil, disabling the identifier lookup — used by
// fileLiteralValueKinds itself, one hop only, never chasing a chain).
func resolveBasicKind(val ast.Expr, litKinds map[string]string) (string, bool) {
	switch v := val.(type) {
	case *ast.Ident:
		if v.Name == "true" || v.Name == "false" {
			return "bool", true
		}
		if litKinds != nil {
			if kind, ok := litKinds[v.Name]; ok {
				return kind, true
			}
		}
	case *ast.BasicLit:
		switch v.Kind {
		case token.INT:
			return "int", true
		case token.FLOAT:
			return "float", true
		case token.STRING:
			return "string", true
		case token.CHAR:
			return "rune", true
		case token.IMAG:
			return "complex", true
		}
	}
	return "", false
}

// normalizeFieldListTypes rewrites every field's Type in fl via
// normalizeAnyType, keeping Names and Tag as-is (strippedFieldList already
// discards names for a signature; structFieldShapes needs them kept for a
// struct's own field-name keys).
func normalizeFieldListTypes(fl *ast.FieldList) *ast.FieldList {
	if fl == nil {
		return nil
	}
	out := &ast.FieldList{}
	for _, f := range fl.List {
		out.List = append(out.List, &ast.Field{Names: f.Names, Type: normalizeAnyType(f.Type), Tag: f.Tag})
	}
	return out
}
