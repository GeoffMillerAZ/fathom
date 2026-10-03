package scip

import "strings"

// import_go.go — scip-go's producer-gated rules (language-breadth F.34, wave
// lf1). Every hook is a no-op for any other producer.

// noteGoSkip counts an "unmapped-endpoint" occurrence by the class of the
// symbol on the unmapped side (side is "from" or "to"), for a scip-go index.
func (r *Report) noteGoSkip(side, sym string) {
	if r.Producer.Name != "scip-go" {
		return
	}
	if r.SkippedByClass == nil {
		r.SkippedByClass = map[string]int{}
	}
	r.SkippedByClass[side+":"+goSymbolClass(sym)]++
}

// goSymbolClass is the class of a scip-go symbol by its last descriptor:
// package (a namespace, defined at every file's package clause), type, func,
// method, field (a term of a type; scip-go writes an interface's method the
// same way), package-var (a package-level term, a constant included), local,
// or other.
func goSymbolClass(sym string) string {
	switch {
	case strings.HasPrefix(sym, "local "):
		return "local"
	case strings.HasSuffix(sym, "/"):
		return "package"
	case strings.HasSuffix(sym, "#"):
		return "type"
	case strings.HasSuffix(sym, ")."):
		if strings.Contains(sym[:strings.LastIndex(sym, "(")], "#") {
			return "method"
		}
		return "func"
	case strings.HasSuffix(sym, "."):
		body := strings.TrimSuffix(sym, ".")
		if strings.Contains(body[strings.LastIndexByte(body, '/')+1:], "#") {
			return "field"
		}
		return "package-var"
	}
	return "other"
}

// GoScopedTerm reports whether sym is a term scip-go writes inside a
// function's own scope: its last descriptor is a term whose parent is a meta
// descriptor (`Options:c.`, a parameter of a function literal in a package
// variable's initializer; `PrepareCmd:cmd.`). Such a definition is a
// parameter or a local, finer than fathom's granularity, as `local N` is
// (LB-F55, LB-F239). False for any other producer.
func GoScopedTerm(sym string) bool {
	if !strings.HasPrefix(sym, "scip-go ") || !strings.HasSuffix(sym, ".") || strings.HasSuffix(sym, ").") {
		return false
	}
	rest := sym
	if i := strings.LastIndex(rest, "`/"); i >= 0 {
		rest = rest[i+2:]
	} else if i := strings.LastIndexByte(rest, '/'); i >= 0 {
		rest = rest[i+1:]
	}
	body := strings.TrimSuffix(rest, ".")
	return strings.Contains(body, ":") && !strings.Contains(body, "#")
}
