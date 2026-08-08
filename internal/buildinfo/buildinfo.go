// Package buildinfo exposes the producing binary's identity for telemetry
// stamping (vision 14 §3, plan IO-11..13): every persistent record fathom
// writes carries schema_version (per record type) and fathom_version (this
// package) so trend claims can filter by the semantics that produced each
// row — "this number predates the fix" becomes a query, not an
// investigation. The research bundle carried these from birth; the call
// log, hook beats, session ledger and rollups now match it.
package buildinfo

import (
	"runtime/debug"
	"sync"
)

var (
	once    sync.Once
	version string
)

// Version returns the producing binary's VCS revision (short), with a
// "-dirty" suffix when built from a modified tree, or "unknown" when the
// binary carries no build info (e.g. some test binaries). Never empty,
// stable for the process lifetime.
func Version() string {
	once.Do(func() {
		version = "unknown"
		bi, ok := debug.ReadBuildInfo()
		if !ok {
			return
		}
		var rev string
		dirty := false
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				rev = s.Value
			case "vcs.modified":
				dirty = s.Value == "true"
			}
		}
		if rev == "" {
			return
		}
		if len(rev) > 12 {
			rev = rev[:12]
		}
		if dirty {
			rev += "-dirty"
		}
		version = rev
	})
	return version
}
