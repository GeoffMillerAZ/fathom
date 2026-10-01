package langbench

import "strings"

// isSCIPJava reports whether sym is one scip-java wrote (its symbols open
// with the SemanticDB scheme; scip-java's Kotlin and Scala readings share it
// and share the `<init>` rule below).
func isSCIPJava(sym string) bool { return strings.HasPrefix(sym, "semanticdb ") }

// javaName is a scip-java symbol's descriptor leaf in the name fathom's
// dialect gives the same definition (any other producer's leaf is returned as
// it is): a constructor (`Foo#`<init>`().`) is its class, Foo,
// which is also how new Foo(...) is written as a call. owner is the
// descriptor text before the leaf ("…/Foo#").
func javaName(sym, leaf, owner string) string {
	if !isSCIPJava(sym) || leaf != "<init>" {
		return leaf
	}
	o := strings.TrimSuffix(owner, "#")
	return o[strings.LastIndexAny(o, "/#.:] `")+1:]
}
