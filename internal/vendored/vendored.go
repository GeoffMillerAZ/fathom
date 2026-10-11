// Package vendored is fathom's classifier of a vendored tree by its
// repo-relative path, kept as the one-line delegate the Go typed pass
// (internal/fusion/typed_nestedmodules.go) and custodian call.
//
// It moved here from internal/custodian (vendoredPath) in wave h5 (EW-62),
// and RF-2 (docs/plans/repository-facts.md) unified it with search's
// searchVendorSegments into internal/repofacts' one convention table: Path
// now delegates, so there is one vendor rule, never two.
package vendored

import "github.com/geoffmilleraz/fathom/internal/repofacts"

// Path reports whether rel, a repo-relative slash path, lies in a
// tracked-but-vendored tree: a directory segment third_party, vendor or
// node_modules at any depth, or rel is one such segment itself
// (repofacts.IsVendored). A third-party snapshot's files are that
// project's, not this repository's: custodian does not audit its
// CLAUDE.md, and the typed pass does not load its go.mod as the
// repository's own module.
func Path(rel string) bool {
	return repofacts.IsVendored(rel)
}
