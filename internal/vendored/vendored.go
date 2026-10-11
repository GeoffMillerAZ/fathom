// Package vendored is fathom's classifier of a vendored tree by its
// repo-relative path, a leaf with no imports so any package can share it.
//
// It moved here from internal/custodian (vendoredPath) in wave h5 (EW-62):
// the Go typed pass's own-module rule skips a nested module under a
// vendored directory with this same classifier rather than grow its own,
// and importing custodian from fusion would have pulled custodian's
// state-directory resolver into every package that imports fusion
// (testisolate's RT-19 walk). RF-2 unifies it with search's
// searchVendorSegments; until then there are two, never three.
package vendored

import "strings"

// Path reports whether rel, a repo-relative slash path, lies in a
// tracked-but-vendored tree: its first segment is third_party, vendor or
// node_modules. A third-party snapshot's files are that project's, not
// this repository's: custodian does not audit its CLAUDE.md, and the typed
// pass does not load its go.mod as the repository's own module.
func Path(rel string) bool {
	first := rel
	if i := strings.IndexByte(rel, '/'); i >= 0 {
		first = rel[:i]
	}
	switch first {
	case "third_party", "vendor", "node_modules":
		return true
	}
	return false
}
