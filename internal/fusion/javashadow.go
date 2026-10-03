package fusion

import (
	"strings"

	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// javashadow.go is O513 (JLS 6.4.1, 6.4.2): in Java a simple type name a file
// writes is looked up in this order, and the package scope is last. A
// single-type import (`import java.util.Objects;`) shadows a same-package
// class of that name, even when the imported type is outside the repo and
// resolves to no file; so does a type the file itself declares, nested or
// top-level. The package-scope match (matchPackage) never binds a shadowed
// name: an edge to the same-package class would be one the file does not mean.
//
// The rule is Java's alone: C#, Kotlin and PHP resolve names through other
// orders (a using alias, a namespace chain, a function fallback), so their
// scope sets stay empty and their resolver output unchanged.

// javaShadowNames is the simple names that shadow the package scope in the
// file at path: every single-type import's bound name and every type the
// file declares (the last segment of its qualified name, and each enclosing
// type's).
func (idx *resolveIndex) javaShadowNames(path string, imports []extractor.ImportSpec) map[string]bool {
	out := map[string]bool{}
	for _, spec := range imports {
		if spec.Kind == extractor.ImportNamed && spec.LocalName != "" {
			out[spec.LocalName] = true
		}
	}
	for _, s := range idx.byFile[path] {
		if !idx.isType[s.Key()] {
			continue
		}
		for _, seg := range strings.Split(s.Name, ".") {
			out[seg] = true
		}
	}
	return out
}

// shadows reports whether the head segment of a call text or reference name
// is a name this file shadows out of its package scope. A qualified name
// whose head is not shadowed (`com.acme.Foo`) is untouched.
func (sc fileScope) shadows(name string) bool {
	if len(sc.shadow) == 0 {
		return false
	}
	segs := refSegments(name)
	return len(segs) > 0 && sc.shadow[segs[0]]
}
