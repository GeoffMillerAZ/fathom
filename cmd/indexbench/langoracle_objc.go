package main

// langoracle_objc.go — wave lang6, item objc (LB-Q-objective-c L3): the objective-c dialect resolves its #import,
// #include and @import statements as ingest resolves them (fusion.ResolveFileEdges under the dialect's registry key),
// so with no SCIP index its file edges are scored against the neutral oracle file's pairs (LB-F57): the statements
// and, marked same_package, the files the compiler binds a reference to through an import chain
// (calibration/oracles/clang-objc).
func init() {
	fileResolveLang["objective-c"] = "objective-c"
}
