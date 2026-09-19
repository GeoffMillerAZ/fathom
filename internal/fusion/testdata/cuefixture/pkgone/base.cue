// Package pkgone is the SU-10 fixture's dependency package: it declares two
// definitions consumed both same-file (#Derived) and cross-file (#Extra, in
// more.cue) so TestEnrichCUEEdges_Fixture can pin both resolution shapes.
package pkgone

#Base: {
	x: int
}

#Derived: #Base & {
	y: int
}
