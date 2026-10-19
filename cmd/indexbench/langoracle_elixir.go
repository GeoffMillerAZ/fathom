package main

// langoracle_elixir.go — wave lang3, item elixir (LB-Q-elixir L3): the elixir dialect resolves its alias, import,
// require and use statements as ingest resolves them (fusion.ResolveFileEdges under the dialect's language tag),
// so with no SCIP index its file edges are scored against the import dump's (file, imported file) pairs (LB-F57):
// calibration/oracles/elixir-ast.
func init() {
	fileResolveLang["elixir"] = "elixir"
}
