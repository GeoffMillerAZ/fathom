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

	"github.com/geoffmilleraz/fathom/internal/dlog"
)

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
func rotateDaemonStdio(out, errW *os.File, maxBytes int64, log *dlog.Logger) {
	dir, ok := daemonLogDir()
	if !ok {
		return
	}
	if out != nil {
		truncateStdioIfNeeded(out, filepath.Join(dir, "fathom-http.log"), maxBytes, log)
	}
	if errW != nil {
		truncateStdioIfNeeded(errW, filepath.Join(dir, "fathom-http.err"), maxBytes, log)
	}
}

// truncateStdioIfNeeded is rotateDaemonStdio's per-file worker; see this
// file's header comment for why it truncates in place instead of renaming.
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

	if err := f.Truncate(0); err != nil {
		log.Warn(fmt.Sprintf("log-rotate: truncating %s: %v", path, err))
		return
	}
	// The write offset survives Truncate — without this Seek, the next
	// write would land at the file's OLD length, leaving a sparse hole of
	// NUL bytes from 0 to there instead of starting the new generation at
	// byte 0.
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		log.Warn(fmt.Sprintf("log-rotate: resetting write offset for %s: %v", path, err))
		return
	}
	log.Info(fmt.Sprintf("log-rotate: %s exceeded %d bytes, truncated in place (one prior generation kept at %s.1)", path, maxBytes, path))
}
