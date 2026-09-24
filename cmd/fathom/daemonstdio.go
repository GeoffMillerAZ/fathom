package main

// daemonstdio.go — DG-6 (diagnostics sweep): bound the launchd-piped
// daemon log files.
//
// daemon.go's plistBody points launchd's StandardOutPath/StandardErrorPath
// at ~/.fathom/logs/fathom-http.log and .err with no size cap; live
// evidence showed these growing unbounded (one measured at 4,466 lines /
// 950KB and climbing). beats.go already proved a one-prior-generation
// rotation for the hooks beat log (rotateBeatLogIfNeeded, now
// internal/dlog.RotateIfNeeded) — but that primitive renames the path,
// which only works for a writer that REOPENS by path on every write.
//
// launchd owns stdout/stderr's file descriptor for the daemon's ENTIRE
// lifetime — it opens the file once, before exec, and hands the daemon
// that already-open fd. Renaming the path out from under it does nothing
// to the fd itself: the daemon would keep writing into the renamed file
// forever, and a fresh "fathom-http.log" would never receive another byte.
// So these two are truncated IN PLACE on the daemon's own still-open
// os.Stdout/os.Stderr handles instead — os.SameFile confirms path names
// the SAME file the handle is connected to before doing anything (a manual
// invocation with stdout redirected elsewhere is left alone), the current
// bytes are copied aside as ONE prior generation (path+".1"), and the
// handle's own write offset is reset to 0 via Truncate+Seek — the offset
// reset is what keeps the next write from landing at the OLD end-of-file
// as a sparse-file hole full of NUL bytes instead of at the true start of
// the new generation.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/geoffmilleraz/fathom/internal/dlog"
)

// scavengeTick is the periodic housekeeping runMemoryScavenger's ticker
// fires each interval (O108 memory return, DG-6 log rotation, and
// finding [2]/[7]'s dedupe-flush) — factored out of the ticker's closure
// (servehttp.go) so the wiring is unit-testable instead of only provable
// by waiting on a real ticker. now is injected (never time.Now directly)
// so a test can assert FlushDedupe ran without depending on wall time.
//
// outLog and flog each own an independent dedupeState (dlog.New
// constructs one per Logger), so both need their own flush: before this,
// Logger.FlushDedupe (DG-2) was fully implemented and unit-tested but
// never called from any production code path, so a deduped repeating
// condition (e.g. fleetAdapter's "fleet: sources for %s" warning) logged
// its first occurrence and then silently suppressed every further one for
// the rest of the process's life — worse than the bare fmt.Fprintf it
// replaced, since an operator had no way to tell "happened once" from
// "still happening."
func scavengeTick(outFile, errFile *os.File, maxBytes int64, outLog, flog *dlog.Logger, now func() time.Time) {
	debug.FreeOSMemory()
	rotateDaemonStdio(outFile, errFile, maxBytes, outLog, flog)
	outLog.FlushDedupe(now())
	flog.FlushDedupe(now())
}

// daemonStdioRotateBytes bounds fathom-http.log/.err, mirroring
// beats.go's beatLogRotateBytes. A package var (not a const) so tests can
// shrink it rather than writing a real 64MB fixture.
var daemonStdioRotateBytes int64 = 64 << 20

// daemonLogDir is daemon.go's own ~/.fathom/logs convention (daemon.go's
// install command computes the identical path when it writes the launchd
// plist's StandardOutPath/StandardErrorPath) — factored out so the two
// call sites can never drift apart.
func daemonLogDir() (string, bool) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", false
	}
	return filepath.Join(home, ".fathom", "logs"), true
}

// rotateDaemonStdio bounds this process's own os.Stdout/os.Stderr when
// launchd has piped them to daemon.go's known fathom-http.log/.err paths.
// Safe to call from any process (a manual, non-launchd invocation simply
// never matches os.SameFile and is left untouched). out/errW are injected
// so tests can substitute real temp files for os.Stdout/os.Stderr — the
// production caller (servehttp.go, piggybacking the scavenge ticker) passes
// those two directly.
//
// outLog/errLog are the loggers that ACTUALLY write to out/errW
// respectively (servehttp.go's outLog writes to stdout, flog to stderr) —
// two, not one, because truncateStdioIfNeeded must serialize its
// Truncate+Seek against the SAME lock the file's own writer uses
// (Logger.WithWriteLock, finding [1]/DG-1+DG-6): locking the wrong
// logger's mutex would leave the exact race it exists to close wide open
// for whichever file that mismatched logger doesn't actually write to.
func rotateDaemonStdio(out, errW *os.File, maxBytes int64, outLog, errLog *dlog.Logger) {
	dir, ok := daemonLogDir()
	if !ok {
		return
	}
	if out != nil {
		truncateStdioIfNeeded(out, filepath.Join(dir, "fathom-http.log"), maxBytes, outLog)
	}
	if errW != nil {
		truncateStdioIfNeeded(errW, filepath.Join(dir, "fathom-http.err"), maxBytes, errLog)
	}
}

// truncateStdioIfNeeded is rotateDaemonStdio's per-file worker; see this
// file's header comment for why it truncates in place instead of renaming.
//
// log MUST be the Logger that writes to f (never some other stream's
// logger): the backup-read/Truncate/Seek sequence runs inside
// log.WithWriteLock so it can never interleave with a concurrent
// Debug/Info/Warn/Error call's Write landing between the Truncate and the
// Seek — which would otherwise leave a sparse hole of NUL bytes in f
// (finding [1]: the daemon runs 24/7 with many goroutines logging through
// the very same *os.File this function truncates).
func truncateStdioIfNeeded(f *os.File, path string, maxBytes int64, log *dlog.Logger) {
	fInfo, err := f.Stat()
	if err != nil || fInfo.Size() < maxBytes {
		return
	}
	pathInfo, statErr := os.Stat(path)
	if statErr != nil || !os.SameFile(fInfo, pathInfo) {
		// f is not actually connected to path (e.g. a manual run with
		// stdout redirected elsewhere, or launchd not in the picture at
		// all) — nothing named path is safe to back up or truncate here.
		return
	}

	if content, readErr := os.ReadFile(path); readErr == nil {
		if writeErr := os.WriteFile(path+".1", content, 0o644); writeErr != nil {
			log.Warn(fmt.Sprintf("log-rotate: writing prior generation for %s: %v", path, writeErr))
		}
	} else {
		log.Warn(fmt.Sprintf("log-rotate: reading %s to back it up before truncating: %v", path, readErr))
	}

	// Only the Truncate+Seek pair itself needs log.WithWriteLock — NOT the
	// log.Warn/log.Info calls around it, which would DEADLOCK: they write
	// through this SAME Logger's lockedWriter, which takes the SAME
	// (non-reentrant) mutex WithWriteLock is already holding. Scoping the
	// lock to just these two calls is also exactly what closes the race:
	// any concurrent Debug/Info/Warn/Error call's Write blocks on the same
	// mutex until BOTH the Truncate and the Seek have completed, so it can
	// never land at the pre-truncate offset.
	var truncErr, seekErr error
	log.WithWriteLock(func() {
		if truncErr = f.Truncate(0); truncErr != nil {
			return
		}
		// The write offset survives Truncate — without this Seek, the next
		// write would land at the file's OLD length, leaving a sparse hole
		// of NUL bytes from 0 to there instead of starting the new
		// generation at byte 0.
		_, seekErr = f.Seek(0, io.SeekStart)
	})
	if truncErr != nil {
		log.Warn(fmt.Sprintf("log-rotate: truncating %s: %v", path, truncErr))
		return
	}
	if seekErr != nil {
		log.Warn(fmt.Sprintf("log-rotate: resetting write offset for %s: %v", path, seekErr))
		return
	}
	log.Info(fmt.Sprintf("log-rotate: %s exceeded %d bytes, truncated in place (one prior generation kept at %s.1)", path, maxBytes, path))
}
