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

// Release is the fathom release `fathom version` prints and the MCP
// initialize serverInfo.version reports: one constant, so the two agree.
const Release = "v0.13.1"

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

// Stamp is the producing binary's full VCS identity, for provenance
// surfaces that must compare two binaries (vision 26 §1.1: the daemon's
// /statusz against the CLI that reads it). Revision is the full
// vcs.revision ("" when the binary carries none), Dirty is vcs.modified,
// BuiltAt is vcs.time as stamped ("" when absent). Version() above is the
// short form of the same facts.
type Stamp struct {
	Revision string
	Dirty    bool
	BuiltAt  string
}

var (
	stampOnce sync.Once
	stamp     Stamp
)

// Read returns the binary's Stamp; stable for the process lifetime and
// never an error — a binary with no build info yields the zero Stamp.
func Read() Stamp {
	stampOnce.Do(func() {
		bi, ok := debug.ReadBuildInfo()
		if !ok {
			return
		}
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				stamp.Revision = s.Value
			case "vcs.modified":
				stamp.Dirty = s.Value == "true"
			case "vcs.time":
				stamp.BuiltAt = s.Value
			}
		}
	})
	return stamp
}

// SameRevision reports whether two revision strings name the same commit:
// either may be a short prefix of the other (a 12-char ledger stamp against
// a 40-char /statusz field), a "-dirty" suffix is ignored, and an empty
// side is never "same" — an unknown revision matches nothing.
func SameRevision(a, b string) bool {
	a = trimDirty(a)
	b = trimDirty(b)
	if a == "" || b == "" {
		return false
	}
	if len(a) > len(b) {
		a, b = b, a
	}
	return b[:len(a)] == a
}

func trimDirty(s string) string {
	if i := len(s) - len("-dirty"); i > 0 && s[i:] == "-dirty" {
		return s[:i]
	}
	return s
}
