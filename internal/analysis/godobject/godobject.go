// Package godobject identifies "god object" types — types that accumulate too
// many methods or fields, violating the Single-Responsibility Principle.
//
// Detection heuristic over []core.SymbolNode:
//   - A type T is collected from nodes whose Kind is KindGoType, KindJSClass
//     (or KindJSClassComponent, a React class component, LB-F377),
//     KindPyClass, KindJavaClass, or KindRubyClass.
//   - Methods are counted by scanning all nodes in the same Package whose Name
//     contains "." and whose prefix matches the type name (e.g. "T.Foo").
//   - Fields are approximated from the node Signature field: the number of
//     comma-separated tokens inside the outermost braces `{ ... }`, or zero if
//     the signature has no braces.
//
// All exported functions are pure, deterministic, and safe for concurrent use.
package godobject

import (
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// GodObject is a type that exceeds the configured method or field threshold.
type GodObject struct {
	Type    string // fully-qualified type name (Package + "." + Name)
	Methods int    // number of methods found
	Fields  int    // estimated field count from signature
}

// Config controls detection thresholds.
type Config struct {
	// MethodThreshold is the minimum number of methods for a type to be
	// flagged. Default (zero value) is treated as 10.
	MethodThreshold int
	// FieldThreshold is the minimum number of fields for a type to be
	// flagged. Default (zero value) is treated as 10.
	FieldThreshold int
}

func (c Config) methodMin() int {
	if c.MethodThreshold <= 0 {
		return 10
	}
	return c.MethodThreshold
}

func (c Config) fieldMin() int {
	if c.FieldThreshold <= 0 {
		return 10
	}
	return c.FieldThreshold
}

// isTypeKind returns true for node kinds that represent a class/type.
func isTypeKind(k core.SourceKind) bool {
	switch k {
	case core.KindGoType, core.KindGoInterface,
		core.KindJSClass, core.KindJSClassComponent, core.KindPyClass,
		core.KindJavaClass, core.KindJavaInterface,
		core.KindRubyClass, core.KindRubyModule:
		return true
	}
	return false
}

// isMethodKind returns true for node kinds that represent a method.
func isMethodKind(k core.SourceKind) bool {
	switch k {
	case core.KindGoMethod, core.KindJSMethod,
		core.KindJavaMethod, core.KindRubyMethod:
		return true
	}
	return false
}

// Analyze scans nodes for god objects according to cfg thresholds.  The result
// is sorted descending by Methods+Fields, then ascending by Type name.
func Analyze(nodes []core.SymbolNode, cfg Config) []GodObject {
	mMin := cfg.methodMin()
	fMin := cfg.fieldMin()

	// Build a set of types per (service, package).
	type typeKey struct{ service, pkg, name string }

	typeSigs := make(map[typeKey]string) // type -> signature
	methodCount := make(map[typeKey]int) // type -> method count
	typeExists := make(map[typeKey]bool) // which types we saw

	for _, n := range nodes {
		if isTypeKind(n.Kind) {
			k := typeKey{n.Service, n.Package, n.Name}
			typeSigs[k] = n.Signature
			typeExists[k] = true
			if _, ok := methodCount[k]; !ok {
				methodCount[k] = 0
			}
		}
	}

	// Count methods — either by isMethodKind or by Go-convention "Type.Method" name.
	for _, n := range nodes {
		pkg := n.Package
		svc := n.Service

		if isMethodKind(n.Kind) {
			// Name may be "Type.Method"
			if dot := strings.Index(n.Name, "."); dot > 0 {
				typeName := n.Name[:dot]
				k := typeKey{svc, pkg, typeName}
				if typeExists[k] {
					methodCount[k]++
					continue
				}
			}
			// fallback: attribute to package-level type with same prefix
		}

		// Go convention: KindGoFunc named "TypeName.Method"
		if n.Kind == core.KindGoFunc || n.Kind == core.KindGoMethod {
			if dot := strings.Index(n.Name, "."); dot > 0 {
				typeName := n.Name[:dot]
				k := typeKey{svc, pkg, typeName}
				if typeExists[k] {
					methodCount[k]++
				}
			}
		}
	}

	var results []GodObject
	for k := range typeExists {
		methods := methodCount[k]
		fields := estimateFields(typeSigs[k])
		if methods >= mMin || fields >= fMin {
			qualName := k.name
			if k.pkg != "" {
				qualName = k.pkg + "." + k.name
			}
			results = append(results, GodObject{
				Type:    qualName,
				Methods: methods,
				Fields:  fields,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		si := results[i].Methods + results[i].Fields
		sj := results[j].Methods + results[j].Fields
		if si != sj {
			return si > sj
		}
		return results[i].Type < results[j].Type
	})
	return results
}

// estimateFields counts the approximate number of fields in a struct signature.
// It looks for content inside the first `{...}` pair.
func estimateFields(sig string) int {
	start := strings.Index(sig, "{")
	end := strings.LastIndex(sig, "}")
	if start < 0 || end <= start {
		return 0
	}
	body := strings.TrimSpace(sig[start+1 : end])
	if body == "" {
		return 0
	}
	// Each field is on its own line (typical Go struct formatting) or separated
	// by semicolons. Fall back to newline / semicolon splitting.
	lines := strings.FieldsFunc(body, func(r rune) bool {
		return r == '\n' || r == ';'
	})
	count := 0
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" && !strings.HasPrefix(l, "//") {
			count++
		}
	}
	return count
}
