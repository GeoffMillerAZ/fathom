package main

// langoracle_shell.go — wave lf6, item shell (LB-Q-bash, LB-Q-powershell
// L3): the bash and powershell dialects resolve their `source` and `.`,
// dot-sourcing, Import-Module, `using module` and manifest module statements
// as ingest resolves them (fusion.ResolveFileEdges under the dialect's
// language tag), so with no SCIP index their file edges are scored against
// the import dump's (file, imported file) pairs (LB-F57):
// calibration/oracles/bash-source and calibration/oracles/powershell-ast's
// PowerShellImportDump.ps1.
func init() {
	fileResolveLang["bash"] = "bash"
	fileResolveLang["powershell"] = "powershell"
}
