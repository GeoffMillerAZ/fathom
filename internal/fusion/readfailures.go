package fusion

// Ingest honesty under a wedged reader (docs/plans/instrument-truth.md
// IT-14 / IT-15; .fathom/observations.md O16).
//
// Before this file, Pipeline.Ingest aborted on the FIRST Fetch error, and
// the Report carried no per-file read-failure counter — so an ingest could
// only ever be loud-and-empty (abort) or quiet, and a reader wedged for the
// whole process tree (the macOS TCC verdict for an external volume, which
// fails every open() with EPERM while stat() keeps working) had no honest
// column to land in. The rules now:
//
//   - A per-file READ failure (EPERM / EACCES / EIO, or ENOENT for a file
//     deleted between Discover and Fetch) is recorded — path plus errno
//     text — and the file is skipped. Its stored evidence is left exactly
//     as it was: the file stays in the Discover set, so the orphan purge
//     never treats "unreadable" as "gone".
//   - Any other Fetch error (context cancellation, connector transport)
//     still aborts the ingest before a single write, as before.
//   - An ingest that could read NOTHING refuses to touch an index that
//     already holds observations for the service: it returns a Refused
//     *ReadFailuresError before the freshness watermark, the per-file
//     purges, PutObservations, the typed-edge pass and the orphan purge —
//     a wedge can never overwrite a good index with a plausible empty one.
//   - Any read failure at all makes Ingest return a *ReadFailuresError
//     alongside the (complete, honest) Report: exit codes and JSON-RPC
//     error contracts that already treated "fetch error" as failure keep
//     doing so; the difference is that the readable files were indexed and
//     the failure names every file it could not read.

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"strings"
	"syscall"
)

// ReadFailure is one file the connector could not read during an Ingest:
// its repo-relative path and the connector's error text (which, for the
// os.ReadFile-backed connectors, carries the errno — "operation not
// permitted", "permission denied", "no such file or directory").
type ReadFailure struct {
	Path string
	Err  string
}

// ReadFailuresError is returned by Pipeline.Ingest when at least one file
// could not be read. Failures is sorted by Path. Refused is true when the
// ingest could read nothing and the store already held observations for
// Service — then NOTHING was written (Existing is the observation count the
// refusal protected); otherwise every readable file was processed normally
// and only the listed files kept their previous evidence.
//
// Incomplete (F2, docs/bench/readings/rt-waves-c-f-2026-09-27.md §6) is
// true when the ingest could read nothing and there was no evidence to
// protect (a first ingest, or an empty index): nothing was written, no
// freshness watermark was recorded and ingest_state stays in_progress, so
// the service is never served as indexed at HEAD and the next ingest reads
// every file again (RT-109).
type ReadFailuresError struct {
	Service    string
	Failures   []ReadFailure
	Refused    bool
	Existing   int
	Incomplete bool
}

// WroteNothing reports whether the ingest stopped before its first write —
// refused (IT-15) or held back incomplete (F2) — so callers skip the
// post-ingest steps that assume an index was written.
func (e *ReadFailuresError) WroteNothing() bool { return e.Refused || e.Incomplete }

// Error names the cause: the count, the first failed path with its errno
// text, and — when refused — the evidence the refusal protected.
func (e *ReadFailuresError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "fusion: ingest %q: %d file(s) could not be read", e.Service, len(e.Failures))
	if len(e.Failures) > 0 {
		fmt.Fprintf(&b, " (first: %s: %s)", e.Failures[0].Path, e.Failures[0].Err)
	}
	switch {
	case e.Refused:
		fmt.Fprintf(&b, "; refused to replace the %d observation(s) already stored for %q with a zero-symbol result — nothing was written", e.Existing, e.Service)
	case e.Incomplete:
		fmt.Fprintf(&b, "; every file failed, so the ingest was not recorded complete — nothing was written, no watermark was recorded, and %q stays in_progress for a retry", e.Service)
	default:
		b.WriteString("; the stored evidence for those files was left as it was")
	}
	return b.String()
}

// externalVolumeHint is the operator hint for EPERM ("operation not
// permitted") on a read: on macOS that is the signature of a wedged TCC
// verdict for an external volume — not a Unix permission problem — and the
// fix is a TCC reset for the terminal's bundle id, not a chmod.
const externalVolumeHint = "external-volume TCC wedge? tccutil reset SystemPolicyRemovableVolumes <terminal bundle id>"

// Hint returns the operator hint for the failure class, or "" when the
// errno text carries no known signature (EACCES is a real permission
// problem and gets no hint).
func (e *ReadFailuresError) Hint() string {
	for _, f := range e.Failures {
		if strings.Contains(f.Err, "operation not permitted") {
			return externalVolumeHint
		}
	}
	return ""
}

// readOps are the *fs.PathError.Op / *os.SyscallError.Syscall values a
// file READ produces (os.ReadFile fails as "open" or "read"; the stat /
// readdir family covers a connector that stats before it reads). An error
// from any other operation — "fork/exec" for a connector that shells out,
// "connect" / "dial" for a transport — is not a read failure even when it
// bottoms out in one of the read errnos.
var readOps = map[string]bool{
	"open": true, "openat": true, "read": true, "pread": true, "readat": true,
	"stat": true, "lstat": true, "fstat": true, "readlink": true,
	"readdir": true, "readdirent": true, "fdopendir": true, "getdirentries": true,
}

// isReadFailure reports whether a Fetch error is a per-file read failure —
// the file is in the Discover set but its bytes could not be read — as
// opposed to a cancellation or transport error that must abort the ingest.
//
// The classification is deliberately narrow, because anything it admits
// is skipped instead of aborting the ingest. The chain must carry a
// file-read *fs.PathError or *os.SyscallError (readOps), and its errno
// must be one a reader produces: EPERM / EACCES / EIO / ENOENT / ENOTDIR /
// EISDIR (or the platform's spelling of not-found / permission-denied via
// the fs sentinels). A bare errno, a *net.OpError (ECONNREFUSED /
// ECONNRESET from a connector's transport also bottom out in an errno) and
// a fork/exec *fs.PathError stay fatal, as every non-read Fetch error does.
func isReadFailure(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return false
	}
	var pathErr *fs.PathError
	var sysErr *os.SyscallError
	switch {
	case errors.As(err, &pathErr):
		if !readOps[pathErr.Op] {
			return false
		}
	case errors.As(err, &sysErr):
		if !readOps[sysErr.Syscall] {
			return false
		}
	default:
		return false
	}
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return false
	}
	switch errno {
	case syscall.EPERM, syscall.EACCES, syscall.EIO, syscall.ENOENT, syscall.ENOTDIR, syscall.EISDIR:
		return true
	}
	// Windows spells not-found / access-denied with its own error codes;
	// Errno.Is maps them (and nothing else) onto the two fs sentinels.
	return errors.Is(errno, fs.ErrNotExist) || errors.Is(errno, fs.ErrPermission)
}
