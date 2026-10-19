package fusion

// typedcalls_conversions.go — rapid type analysis at the point of conversion
// (O1237, TypedEdgeEpoch 7). A method that only the standard library calls,
// through an interface the standard library declares, has no call site the
// index sees: cmd/fathom's init sets http.DefaultTransport to a
// *hookguard.Transport, and net/http (not indexed) calls its RoundTrip. What
// the index does see is the conversion: a value of a first-party type meets
// an interface declared outside the module. resolveFileEdges gives the
// converting function one core.EdgeIfaceDispatch edge to each of the type's
// methods in that interface's method set, so the product walk reaches the
// method exactly where reached code hands the value over.
//
// Covered, each from the go/types fact named: an assignment (the left
// side's type, info.TypeOf), a declaration with a type (`var x I = v`, the
// declared type), a call argument and a variadic one (the callee's
// *types.Signature recorded for the call's Fun; a builtin's is specialized
// to the call), a return (the enclosing function's or literal's result
// types), a composite-literal element (the literal's struct field, slice or
// array element, or map key and value type), a send (the channel's element
// type) and an explicit conversion `I(v)` (Fun recorded as a type). A call
// returning several values feeds an assignment or return element-wise.
//
// Not covered: a value whose static type is an interface, a first-party one
// included (its dynamic type is unknown here: a T converted to a
// first-party interface that is later passed to http.Handle stays
// unreached); a type parameter converted inside a generic function; a value
// the standard library type-asserts out of an `any` (fmt calling String on
// a Println argument); reflection. A conversion to the blank identifier
// discards the value and is none.

import (
	"go/ast"
	"go/token"
	"go/types"
)

// conversion is one place a value of type src is converted to type dst.
type conversion struct {
	dst, src types.Type
}

// conversionsUnder lists the conversions under root, which runs with
// signature sig (nil for a package variable's initializer: only the
// function literals inside it return). Each function literal's returns are
// read against its own signature.
func conversionsUnder(info *types.Info, root ast.Node, sig *types.Signature) []conversion {
	var out []conversion
	add := func(dst types.Type, val ast.Expr) {
		if dst == nil || val == nil {
			return
		}
		if tv, ok := info.Types[val]; ok && tv.IsValue() {
			out = append(out, conversion{dst: dst, src: tv.Type})
		}
	}
	// addAll pairs destinations with values: element-wise, or each result
	// of the one call a multi-value assignment or return spreads.
	addAll := func(dsts []types.Type, vals []ast.Expr) {
		if len(dsts) == len(vals) {
			for i := range vals {
				add(dsts[i], vals[i])
			}
			return
		}
		if len(vals) != 1 {
			return
		}
		tuple, ok := info.TypeOf(vals[0]).(*types.Tuple)
		if !ok || tuple.Len() != len(dsts) {
			return
		}
		for i := range dsts {
			if dsts[i] != nil {
				out = append(out, conversion{dst: dsts[i], src: tuple.At(i).Type()})
			}
		}
	}
	var visit func(root ast.Node, sig *types.Signature)
	visit = func(root ast.Node, sig *types.Signature) {
		ast.Inspect(root, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.FuncLit:
				if s, ok := info.TypeOf(node).(*types.Signature); ok {
					visit(node.Body, s)
				}
				return false

			case *ast.AssignStmt:
				if node.Tok != token.ASSIGN {
					return true // := takes the value's type; op= converts nothing
				}
				dsts := make([]types.Type, len(node.Lhs))
				for i, lhs := range node.Lhs {
					if id, ok := lhs.(*ast.Ident); !ok || id.Name != "_" {
						dsts[i] = info.TypeOf(lhs)
					}
				}
				addAll(dsts, node.Rhs)

			case *ast.ValueSpec:
				if node.Type == nil {
					return true
				}
				typ := info.TypeOf(node.Type)
				dsts := make([]types.Type, len(node.Names))
				for i, name := range node.Names {
					if name.Name != "_" {
						dsts[i] = typ
					}
				}
				addAll(dsts, node.Values)

			case *ast.ReturnStmt:
				if sig == nil || sig.Results().Len() == 0 || len(node.Results) == 0 {
					return true
				}
				res := sig.Results()
				dsts := make([]types.Type, res.Len())
				for i := range dsts {
					dsts[i] = res.At(i).Type()
				}
				addAll(dsts, node.Results)

			case *ast.CallExpr:
				tv, ok := info.Types[node.Fun]
				if !ok || tv.Type == nil {
					return true
				}
				if tv.IsType() {
					if len(node.Args) == 1 {
						add(tv.Type, node.Args[0])
					}
					return true
				}
				s, ok := tv.Type.Underlying().(*types.Signature)
				if !ok {
					return true
				}
				params := s.Params()
				for i, arg := range node.Args {
					switch {
					case s.Variadic() && i >= params.Len()-1:
						if node.Ellipsis.IsValid() {
							continue // f(xs...) passes the slice itself
						}
						if sl, ok := params.At(params.Len() - 1).Type().Underlying().(*types.Slice); ok {
							add(sl.Elem(), arg)
						}
					case i < params.Len():
						add(params.At(i).Type(), arg)
					}
				}

			case *ast.CompositeLit:
				t := info.TypeOf(node)
				if t == nil {
					return true
				}
				if p, ok := t.Underlying().(*types.Pointer); ok {
					t = p.Elem()
				}
				switch u := t.Underlying().(type) {
				case *types.Struct:
					for i, elt := range node.Elts {
						if kv, ok := elt.(*ast.KeyValueExpr); ok {
							if key, ok := kv.Key.(*ast.Ident); ok {
								for j := 0; j < u.NumFields(); j++ {
									if f := u.Field(j); f.Name() == key.Name {
										add(f.Type(), kv.Value)
									}
								}
							}
						} else if i < u.NumFields() {
							add(u.Field(i).Type(), elt)
						}
					}
				case *types.Slice:
					addElems(add, u.Elem(), nil, node.Elts)
				case *types.Array:
					addElems(add, u.Elem(), nil, node.Elts)
				case *types.Map:
					addElems(add, u.Elem(), u.Key(), node.Elts)
				}

			case *ast.SendStmt:
				if t := info.TypeOf(node.Chan); t != nil {
					if ch, ok := t.Underlying().(*types.Chan); ok {
						add(ch.Elem(), node.Value)
					}
				}
			}
			return true
		})
	}
	visit(root, sig)
	return out
}

// addElems feeds a slice, array or map literal's elements to add: each
// value converts to elem, and a map entry's key to key.
func addElems(add func(types.Type, ast.Expr), elem, key types.Type, elts []ast.Expr) {
	for _, elt := range elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if key != nil {
				add(key, kv.Key)
			}
			add(elem, kv.Value)
			continue
		}
		add(elem, elt)
	}
}

// externalInterface is dst's interface when dst is a named interface type
// with methods that the module does not declare: the standard library's
// (http.RoundTripper, io.Reader), a dependency's, or the universe's error.
// A call site of a first-party interface's method already dispatches to
// its implementers, so a conversion to one adds nothing.
func externalInterface(dst types.Type, firstParty *typedFirstParty) (*types.Interface, bool) {
	named, ok := types.Unalias(dst).(*types.Named)
	if !ok {
		return nil, false
	}
	iface, ok := named.Underlying().(*types.Interface)
	if !ok || iface.NumMethods() == 0 {
		return nil, false
	}
	if pkg := named.Obj().Pkg(); pkg != nil && firstParty.has(pkg.Path()) {
		return nil, false
	}
	return iface, true
}

// firstPartyConcrete reports whether src is a first-party named type, or a
// pointer to one, that is no interface.
func firstPartyConcrete(src types.Type, firstParty *typedFirstParty) bool {
	t := types.Unalias(src)
	if p, ok := t.(*types.Pointer); ok {
		t = types.Unalias(p.Elem())
	}
	named, ok := t.(*types.Named)
	if !ok || types.IsInterface(named) {
		return false
	}
	pkg := named.Obj().Pkg()
	return pkg != nil && firstParty.has(pkg.Path())
}
