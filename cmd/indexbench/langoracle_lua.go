package main

// langoracle_lua.go — wave lang1, item lua (LB-Q-lua L3): the lua dialect resolves its literal `require`
// statements as ingest resolves them (fusion.ResolveFileEdges under the dialect's language tag), so with no
// SCIP index its file edges are scored against the import dump's (file, imported file) pairs (LB-F57):
// calibration/oracles/lua-require.
func init() {
	fileResolveLang["lua"] = "lua"
}
