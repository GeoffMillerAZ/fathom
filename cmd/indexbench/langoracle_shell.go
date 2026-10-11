package main

// langoracle_shell.go — wave lf6, item shell (LB-Q-bash L3): the bash
// dialect resolves its `source` and `.` statements as ingest resolves them
// (fusion.ResolveFileEdges under the dialect's language tag), so with no SCIP
// index its file edges are scored against the import dump's (file, imported
// file) pairs (LB-F57): calibration/oracles/bash-source.
func init() {
	fileResolveLang["bash"] = "bash"
}
