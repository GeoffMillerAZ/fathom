package ask

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/tools/go/packages"
)

// Fact is one computed predicate over a span. Applies says the span has a
// site the fact is about (the template's question is asked only then);
// Holds is the fact's value, meaningful only when Applies. Sites counts the
// sites examined and Breaks the sites that go against the fact's good
// case: an unchecked error, an error formatted without %w, an assertion, a
// guarded write with the lock not held, a context argument that is not the
// parameter threaded on.
type Fact struct {
	Applies bool `json:"applies"`
	Holds   bool `json:"holds"`
	Sites   int  `json:"sites"`
	Breaks  int  `json:"breaks"`
}

// FactDefinitions are the five facts' definitions, pinned word for word
// here and in docs/bench/readings/decide/exp-preregistration.md (a test
// holds the two identical). Every fact but executes_only applies when the
// span has at least one site; executes_only applies to every Test func.
var FactDefinitions = []struct{ Fact, Definition string }{
	{"unchecked_error", "A call whose result list includes the built-in error type, anywhere in the function (function literals included), is a site; it is unchecked when the call is an expression statement, or when an assignment or var declaration puts its error result into the blank identifier _. The call a defer or go statement makes is not an expression statement. The fact holds when at least one site is unchecked."},
	{"wrapped", "A call to fmt.Errorf with a constant format (no * width or precision, no explicit argument index) is examined; each argument whose type implements error is a site, wrapped when its verb is %w and not wrapped for any other verb, %v and %s among them. The fact holds when every site is wrapped."},
	{"executes_only", "It applies to a top-level func named Test (the next rune, if any, not lower-case) taking one *testing.T, in a _test.go file, whatever its body. The fact holds when its body, function literals in it included, calls no testing method named Error, Errorf, Fatal, Fatalf, Fail, FailNow, Skip, Skipf or SkipNow and passes no *testing.T or testing.TB value to any call."},
	{"locked_write", "Only a method whose receiver's struct type declares a sync.Mutex or sync.RWMutex field before other fields (Go's convention: a field declared after a mutex is guarded by the nearest mutex declared before it) has sites: each assignment (=, op=, ++, --) to such a guarded field of the receiver, through any index, field selection or dereference. A site is locked when, in source statement order within the method (function literals ignored), a receiver.<mutex>.Lock() call (the promoted Lock of an embedded mutex) precedes it with no receiver.<mutex>.Unlock() call between them. RLock is not a Lock and a deferred call is no event. The fact holds when every site is locked."},
	{"threaded_context", "Only a function with a context.Context parameter has sites: each argument it passes, function literals included, to a callee's non-variadic context.Context parameter. A site is threaded when it is the parameter, a function literal's own context.Context parameter, a variable assigned from a threaded value or from a call given one (ctx2, cancel := context.WithTimeout(ctx, d)), or such a call itself; context.Background(), context.TODO() and anything else are not. The fact holds when every site is threaded."},
}

// Facts are the five computed facts over one Go function span (EXP-2),
// defined by FactDefinitions.
type Facts struct {
	UncheckedError  Fact `json:"unchecked_error"`
	Wrapped         Fact `json:"wrapped"`
	ExecutesOnly    Fact `json:"executes_only"`
	LockedWrite     Fact `json:"locked_write"`
	ThreadedContext Fact `json:"threaded_context"`
}

// spanFacts computes the five facts over fd, a function declaration in file
// of the type-checked pkg. It is deterministic and model-free: every fact
// reads pkg.TypesInfo, so "returns an error" is a type fact, never a name.
// The caller guarantees pkg type-checked without errors.
func spanFacts(pkg *packages.Package, file *ast.File, fd *ast.FuncDecl) Facts {
	info := pkg.TypesInfo
	if fd.Body == nil {
		return Facts{}
	}
	fileName := pkg.Fset.Position(file.Pos()).Filename
	return Facts{
		UncheckedError:  uncheckedFact(info, fd.Body),
		Wrapped:         wrappedFact(info, fd.Body),
		ExecutesOnly:    executesOnlyFact(info, fd, fileName),
		LockedWrite:     lockedWriteFact(info, fd),
		ThreadedContext: threadedFact(info, fd),
	}
}

var errorType = types.Universe.Lookup("error").Type()

func unparen(e ast.Expr) ast.Expr {
	for {
		p, ok := e.(*ast.ParenExpr)
		if !ok {
			return e
		}
		e = p.X
	}
}

func isBlank(e ast.Expr) bool {
	id, ok := unparen(e).(*ast.Ident)
	return ok && id.Name == "_"
}

// isCallOfValue is false for conversions and builtins: neither is a call
// with a result list a fact reads.
func isCallOfValue(info *types.Info, call *ast.CallExpr) bool {
	fun, ok := info.Types[call.Fun]
	return ok && !fun.IsType() && !fun.IsBuiltin()
}

// errorResults returns the positions of call's results whose type is the
// built-in error type (identical, not merely implementing it).
func errorResults(info *types.Info, call *ast.CallExpr) []int {
	if !isCallOfValue(info, call) {
		return nil
	}
	var pos []int
	switch t := info.TypeOf(call).(type) {
	case nil:
	case *types.Tuple:
		for i := 0; i < t.Len(); i++ {
			if types.Identical(t.At(i).Type(), errorType) {
				pos = append(pos, i)
			}
		}
	default:
		if types.Identical(t, errorType) {
			pos = []int{0}
		}
	}
	return pos
}

func uncheckedFact(info *types.Info, body *ast.BlockStmt) Fact {
	var f Fact
	// blanks counts the error results of rhs assigned to a blank in lhs.
	blanks := func(lhs []ast.Expr, rhs []ast.Expr) int {
		n := 0
		if len(rhs) == 1 && len(lhs) > 1 {
			if call, ok := unparen(rhs[0]).(*ast.CallExpr); ok {
				for _, p := range errorResults(info, call) {
					if p < len(lhs) && isBlank(lhs[p]) {
						n++
					}
				}
			}
			return n
		}
		if len(lhs) != len(rhs) {
			return 0
		}
		for i, r := range rhs {
			if call, ok := unparen(r).(*ast.CallExpr); ok && isBlank(lhs[i]) && len(errorResults(info, call)) > 0 {
				n++
			}
		}
		return n
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.CallExpr:
			if len(errorResults(info, s)) > 0 {
				f.Sites++
			}
		case *ast.ExprStmt:
			if call, ok := unparen(s.X).(*ast.CallExpr); ok && len(errorResults(info, call)) > 0 {
				f.Breaks++
			}
		case *ast.AssignStmt:
			f.Breaks += blanks(s.Lhs, s.Rhs)
		case *ast.ValueSpec:
			lhs := make([]ast.Expr, len(s.Names))
			for i, id := range s.Names {
				lhs[i] = id
			}
			f.Breaks += blanks(lhs, s.Values)
		}
		return true
	})
	f.Applies = f.Sites > 0
	f.Holds = f.Breaks > 0
	return f
}

// funcObj is the *types.Func a call's Fun resolves to, or nil.
func funcObj(info *types.Info, fun ast.Expr) *types.Func {
	switch x := unparen(fun).(type) {
	case *ast.Ident:
		fn, _ := info.Uses[x].(*types.Func)
		return fn
	case *ast.SelectorExpr:
		fn, _ := info.Uses[x.Sel].(*types.Func)
		return fn
	}
	return nil
}

func isPkgFunc(fn *types.Func, pkgPath, name string) bool {
	if fn == nil || fn.Pkg() == nil || fn.Pkg().Path() != pkgPath || fn.Name() != name {
		return false
	}
	sig, ok := fn.Type().(*types.Signature)
	return ok && sig.Recv() == nil
}

// formatVerbs returns the verb of each argument-consuming directive in a
// fmt format, in order ("%%" consumes none). ok is false for a format the
// fact does not examine: a '*' width or precision, an explicit argument
// index, or a dangling '%'.
func formatVerbs(s string) (verbs []rune, ok bool) {
	isDigit := func(c byte) bool { return c >= '0' && c <= '9' }
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			continue
		}
		i++
		for i < len(s) && strings.IndexByte("+-# 0", s[i]) >= 0 {
			i++
		}
		if i < len(s) && (s[i] == '*' || s[i] == '[') {
			return nil, false
		}
		for i < len(s) && isDigit(s[i]) {
			i++
		}
		if i < len(s) && s[i] == '.' {
			i++
			if i < len(s) && (s[i] == '*' || s[i] == '[') {
				return nil, false
			}
			for i < len(s) && isDigit(s[i]) {
				i++
			}
		}
		if i >= len(s) || s[i] == '[' {
			return nil, false
		}
		if s[i] == '%' {
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		verbs = append(verbs, r)
		i += size - 1
	}
	return verbs, true
}

func implementsError(t types.Type) bool {
	if t == nil {
		return false
	}
	if b, ok := t.(*types.Basic); ok && b.Kind() == types.UntypedNil {
		return false
	}
	iface, _ := errorType.Underlying().(*types.Interface)
	return types.Implements(t, iface)
}

func wrappedFact(info *types.Info, body *ast.BlockStmt) Fact {
	var f Fact
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !isPkgFunc(funcObj(info, call.Fun), "fmt", "Errorf") || len(call.Args) == 0 || call.Ellipsis.IsValid() {
			return true
		}
		tv := info.Types[call.Args[0]]
		if tv.Value == nil || tv.Value.Kind() != constant.String {
			return true
		}
		verbs, ok := formatVerbs(constant.StringVal(tv.Value))
		if !ok {
			return true
		}
		for i, arg := range call.Args[1:] {
			if i >= len(verbs) {
				break
			}
			if !implementsError(info.TypeOf(arg)) {
				continue
			}
			f.Sites++
			if verbs[i] != 'w' {
				f.Breaks++
			}
		}
		return true
	})
	f.Applies = f.Sites > 0
	f.Holds = f.Breaks == 0
	return f
}

func isNamed(t types.Type, pkgPath, name string) bool {
	n, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return false
	}
	o := n.Obj()
	return o.Pkg() != nil && o.Pkg().Path() == pkgPath && o.Name() == name
}

func deref(t types.Type) types.Type {
	if p, ok := types.Unalias(t).(*types.Pointer); ok {
		return p.Elem()
	}
	return t
}

func isTestingT(t types.Type) bool {
	p, ok := types.Unalias(t).(*types.Pointer)
	return ok && isNamed(p.Elem(), "testing", "T")
}

// isTestFunc follows go test's own rule: Test, then nothing or a rune that
// is not lower-case, one *testing.T parameter, no result, in a _test.go.
func isTestFunc(info *types.Info, fd *ast.FuncDecl, fileName string) bool {
	if fd.Recv != nil || !strings.HasSuffix(fileName, "_test.go") || !strings.HasPrefix(fd.Name.Name, "Test") {
		return false
	}
	if rest := fd.Name.Name[len("Test"):]; rest != "" {
		if r, _ := utf8.DecodeRuneInString(rest); unicode.IsLower(r) {
			return false
		}
	}
	fn, ok := info.Defs[fd.Name].(*types.Func)
	if !ok {
		return false
	}
	sig := fn.Type().(*types.Signature)
	return sig.Params().Len() == 1 && sig.Results().Len() == 0 && isTestingT(sig.Params().At(0).Type())
}

var testingAsserts = map[string]bool{
	"Error": true, "Errorf": true, "Fatal": true, "Fatalf": true, "Fail": true,
	"FailNow": true, "Skip": true, "Skipf": true, "SkipNow": true,
}

func executesOnlyFact(info *types.Info, fd *ast.FuncDecl, fileName string) Fact {
	if !isTestFunc(info, fd, fileName) {
		return Fact{}
	}
	f := Fact{Applies: true}
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		f.Sites++
		if sel, ok := unparen(call.Fun).(*ast.SelectorExpr); ok {
			if s := info.Selections[sel]; s != nil && s.Kind() == types.MethodVal {
				if m := s.Obj(); m.Pkg() != nil && m.Pkg().Path() == "testing" && testingAsserts[m.Name()] {
					f.Breaks++
					return true
				}
			}
		}
		for _, a := range call.Args {
			if t := info.TypeOf(a); t != nil && (isTestingT(t) || isNamed(t, "testing", "TB")) {
				f.Breaks++
				break
			}
		}
		return true
	})
	f.Holds = f.Breaks == 0
	return f
}

func isMutex(t types.Type) bool {
	t = deref(t)
	return isNamed(t, "sync", "Mutex") || isNamed(t, "sync", "RWMutex")
}

// receiverStruct is the struct underlying a method receiver's (pointer to)
// named type.
func receiverStruct(t types.Type) (*types.Struct, bool) {
	n, ok := types.Unalias(deref(t)).(*types.Named)
	if !ok {
		return nil, false
	}
	st, ok := n.Underlying().(*types.Struct)
	return st, ok
}

func isRecv(info *types.Info, e ast.Expr, recv types.Object) bool {
	id, ok := unparen(e).(*ast.Ident)
	return ok && info.Uses[id] == recv
}

// receiverField is the top-level field index of recv a write target
// reaches (recv.f, recv.f[i], recv.f.g, *recv.f), or ok=false.
func receiverField(info *types.Info, recv types.Object, e ast.Expr) (int, bool) {
	for {
		switch x := unparen(e).(type) {
		case *ast.IndexExpr:
			e = x.X
		case *ast.IndexListExpr:
			e = x.X
		case *ast.StarExpr:
			e = x.X
		case *ast.SelectorExpr:
			if isRecv(info, x.X, recv) {
				s := info.Selections[x]
				if s == nil || s.Kind() != types.FieldVal || len(s.Index()) == 0 {
					return 0, false
				}
				return s.Index()[0], true
			}
			e = x.X
		default:
			return 0, false
		}
	}
}

// mutexOp reads call as recv.<mutex>.Lock()/Unlock()/RLock()/RUnlock(),
// or a promoted recv.Lock() of an embedded mutex: the mutex's top-level
// field index and the method name.
func mutexOp(info *types.Info, recv types.Object, call *ast.CallExpr) (int, string, bool) {
	sel, ok := unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return 0, "", false
	}
	s := info.Selections[sel]
	if s == nil || s.Kind() != types.MethodVal || s.Obj().Pkg() == nil || s.Obj().Pkg().Path() != "sync" {
		return 0, "", false
	}
	if isRecv(info, sel.X, recv) {
		if len(s.Index()) < 2 {
			return 0, "", false
		}
		return s.Index()[0], s.Obj().Name(), true
	}
	inner, ok := unparen(sel.X).(*ast.SelectorExpr)
	if !ok || !isRecv(info, inner.X, recv) {
		return 0, "", false
	}
	fs := info.Selections[inner]
	if fs == nil || fs.Kind() != types.FieldVal || len(fs.Index()) != 1 {
		return 0, "", false
	}
	return fs.Index()[0], s.Obj().Name(), true
}

func lockedWriteFact(info *types.Info, fd *ast.FuncDecl) Fact {
	if fd.Recv == nil || len(fd.Recv.List) != 1 || len(fd.Recv.List[0].Names) != 1 || fd.Recv.List[0].Names[0].Name == "_" {
		return Fact{}
	}
	recv := info.Defs[fd.Recv.List[0].Names[0]]
	if recv == nil {
		return Fact{}
	}
	st, ok := receiverStruct(recv.Type())
	if !ok {
		return Fact{}
	}
	guard := make([]int, st.NumFields())
	last, guarded := -1, false
	for i := 0; i < st.NumFields(); i++ {
		if isMutex(st.Field(i).Type()) {
			last, guard[i] = i, -1
			continue
		}
		guard[i] = last
		guarded = guarded || last >= 0
	}
	if !guarded {
		return Fact{}
	}
	var f Fact
	locked := map[int]bool{}
	write := func(e ast.Expr) {
		fi, ok := receiverField(info, recv, e)
		if !ok || fi >= len(guard) || guard[fi] < 0 {
			return
		}
		f.Sites++
		if !locked[guard[fi]] {
			f.Breaks++
		}
	}
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.FuncLit, *ast.DeferStmt:
			return false
		case *ast.ExprStmt:
			if call, ok := unparen(s.X).(*ast.CallExpr); ok {
				if fi, op, ok := mutexOp(info, recv, call); ok {
					switch op {
					case "Lock":
						locked[fi] = true
					case "Unlock":
						locked[fi] = false
					}
				}
			}
		case *ast.AssignStmt:
			if s.Tok != token.DEFINE {
				for _, l := range s.Lhs {
					write(l)
				}
			}
		case *ast.IncDecStmt:
			write(s.X)
		}
		return true
	})
	f.Applies = f.Sites > 0
	f.Holds = f.Breaks == 0
	return f
}

func isContext(t types.Type) bool { return t != nil && isNamed(t, "context", "Context") }

func threadedFact(info *types.Info, fd *ast.FuncDecl) Fact {
	fn, ok := info.Defs[fd.Name].(*types.Func)
	if !ok {
		return Fact{}
	}
	sig := fn.Type().(*types.Signature)
	threaded := map[types.Object]bool{}
	hasCtx := false
	for i := 0; i < sig.Params().Len(); i++ {
		if p := sig.Params().At(i); isContext(p.Type()) {
			hasCtx = true
			if p.Name() != "" && p.Name() != "_" {
				threaded[p] = true
			}
		}
	}
	if !hasCtx {
		return Fact{}
	}
	// A function literal's own context parameters carry the context its
	// caller passes.
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		if lit, ok := n.(*ast.FuncLit); ok {
			for _, field := range lit.Type.Params.List {
				for _, id := range field.Names {
					if o := info.Defs[id]; o != nil && isContext(o.Type()) {
						threaded[o] = true
					}
				}
			}
		}
		return true
	})
	var isThreaded func(e ast.Expr) bool
	callThreads := func(call *ast.CallExpr) bool {
		for _, a := range call.Args {
			if isThreaded(a) {
				return true
			}
		}
		return false
	}
	isThreaded = func(e ast.Expr) bool {
		switch x := unparen(e).(type) {
		case *ast.Ident:
			o := info.Uses[x]
			return o != nil && threaded[o]
		case *ast.CallExpr:
			return isContext(info.TypeOf(x)) && callThreads(x)
		}
		return false
	}
	mark := func(id ast.Expr) bool {
		ident, ok := unparen(id).(*ast.Ident)
		if !ok {
			return false
		}
		o := info.Defs[ident]
		if o == nil {
			o = info.Uses[ident]
		}
		if o == nil || threaded[o] || !isContext(o.Type()) {
			return false
		}
		threaded[o] = true
		return true
	}
	assign := func(lhs, rhs []ast.Expr) bool {
		changed := false
		if len(rhs) == 1 && len(lhs) > 1 {
			if call, ok := unparen(rhs[0]).(*ast.CallExpr); ok && callThreads(call) {
				for _, l := range lhs {
					changed = mark(l) || changed
				}
			}
			return changed
		}
		if len(lhs) != len(rhs) {
			return false
		}
		for i := range rhs {
			if isThreaded(rhs[i]) {
				changed = mark(lhs[i]) || changed
			}
		}
		return changed
	}
	for changed := true; changed; {
		changed = false
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			switch s := n.(type) {
			case *ast.AssignStmt:
				changed = assign(s.Lhs, s.Rhs) || changed
			case *ast.ValueSpec:
				lhs := make([]ast.Expr, len(s.Names))
				for i, id := range s.Names {
					lhs[i] = id
				}
				changed = assign(lhs, s.Values) || changed
			}
			return true
		})
	}
	var f Fact
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !isCallOfValue(info, call) {
			return true
		}
		csig, ok := info.TypeOf(call.Fun).Underlying().(*types.Signature)
		if !ok {
			return true
		}
		params := csig.Params()
		for i := 0; i < params.Len() && i < len(call.Args); i++ {
			if csig.Variadic() && i == params.Len()-1 {
				break
			}
			if !isContext(params.At(i).Type()) {
				continue
			}
			f.Sites++
			if !isThreaded(call.Args[i]) {
				f.Breaks++
			}
		}
		return true
	})
	f.Applies = f.Sites > 0
	f.Holds = f.Breaks == 0
	return f
}
