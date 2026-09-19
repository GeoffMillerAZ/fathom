// more.cue exercises a CROSS-FILE, same-package reference: #Extra lives in
// this file, #Base in base.cue — resolvable only because EnrichCUEEdges
// builds its def table from every file in the cue/load instance, not just
// the current one (cueextract.go's own per-file localDefs cannot see this).
package pkgone

#Extra: #Base & {
	w: int
}
