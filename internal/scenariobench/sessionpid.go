// sessionpid.go — O57 finding 1's pidfile half, applying O52's own pattern
// (subject.go's scratchDaemonPIDFile/reapStaleScratchDaemon, for the
// scratch daemon) to a SESSION's own `claude` process: a driver killed
// mid-run used to leave its agent children running, free to keep editing a
// shared, reused worktree after the driver itself had given up on it —
// exactly the O52 shape, one layer up.
//
// Two pidfiles are written per live session, for two different readers:
//
//   - sessionAttemptPIDFile, beside the session's own raw attempt dir, is
//     the durable, per-session artifact a LATER driver invocation can find
//     and reap (ReapOrphanSessionsUnder) — general forensic/cleanup sweep,
//     scoped to one attempt.
//   - sessionWorktreePIDFile, beside the SHARED worktree (mirroring
//     scratchDaemonPIDFile exactly), is what PrepareSubject's own
//     shared-worktree-reset step reads: unlike a stray scratch daemon
//     (fathom's own, safe to kill unconditionally), a still-live AGENT
//     process is not something this driver auto-kills on someone else's
//     behalf — RefuseIfLiveSessionInWorktree reports a reason instead, so
//     the cell is refused (LOST) rather than racing a live edit.
package scenariobench

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// sessionCommandMarker is the substring a reaped/checked process's command
// line must carry to be treated as "this is a session's own agent
// process" — "claude" in production; tests point it at their own stand-in
// (e.g. "sleep"), exactly like subject.go's scratchDaemonCommandMarker.
var sessionCommandMarker = "claude"

// sessionAttemptPIDFile returns the pidfile a live session writes BESIDE
// its own raw attempt dir (a sibling path, never inside it, matching
// scratchDaemonPIDFile's own convention of sitting beside what it names).
func sessionAttemptPIDFile(rawDir string) string {
	return strings.TrimSuffix(rawDir, "/") + ".pid"
}

// sessionWorktreePIDFile returns the pidfile a live session writes BESIDE
// the shared worktree it is running in — the exact naming convention
// scratchDaemonPIDFile already uses for the scratch daemon, one event
// class over.
func sessionWorktreePIDFile(wtPath string) string {
	return strings.TrimSuffix(wtPath, "/") + ".session.pid"
}

// writeSessionPIDFiles records pid as the live session running in wtPath,
// for rawDir's own attempt — best-effort: an error writing either file is
// returned, but a caller that cannot write a pidfile still has a running
// session on its hands and should not treat this as fatal to the session
// itself (RunSession logs and continues rather than aborting the process
// it already started).
func writeSessionPIDFiles(rawDir, wtPath string, pid int) error {
	body := []byte(strconv.Itoa(pid) + "\n")
	var errs []string
	if rawDir != "" {
		if err := os.WriteFile(sessionAttemptPIDFile(rawDir), body, 0o644); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if wtPath != "" {
		if err := os.WriteFile(sessionWorktreePIDFile(wtPath), body, 0o644); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("writing session pidfile(s): %s", strings.Join(errs, "; "))
	}
	return nil
}

// removeSessionPIDFiles removes both of a finished session's pidfiles.
// Best-effort: a missing file is not an error.
func removeSessionPIDFiles(rawDir, wtPath string) {
	if rawDir != "" {
		_ = os.Remove(sessionAttemptPIDFile(rawDir))
	}
	if wtPath != "" {
		_ = os.Remove(sessionWorktreePIDFile(wtPath))
	}
}

// readLivePID reads pidfile and reports the pid it names and whether that
// pid is currently alive AND still carries sessionCommandMarker in its own
// command line (so a recycled pid belonging to an unrelated process is
// never mistaken for a live session) — mirrors reapStaleScratchDaemon's own
// liveness check (subject.go). ok is false for a missing/unparsable
// pidfile, a dead pid, or a recycled one; pidfile is left untouched either
// way (the caller decides whether to remove it).
func readLivePID(pidfile string) (pid int, ok bool) {
	data, err := os.ReadFile(pidfile)
	if err != nil {
		return 0, false
	}
	pid, err = strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 1 {
		return 0, false
	}
	//fathom:tenancy-allow exec-noctx benchmark harness run by an operator, never in the daemon; the child's lifetime is the bench run's
	out, err := exec.Command("ps", "-o", "command=", "-p", strconv.Itoa(pid)).Output()
	if err != nil || !strings.Contains(string(out), sessionCommandMarker) {
		return pid, false
	}
	return pid, true
}

// ReapOrphanSession kills (process group, then the pid itself) whatever
// live, marker-matching process pidfile names, and removes pidfile either
// way — reapStaleScratchDaemon's own shape (subject.go), applied to a
// session's own pidfile. A missing pidfile, a dead pid, or one recycled by
// an unrelated process are all no-ops beyond removing the file.
func ReapOrphanSession(pidfile string) {
	pid, alive := readLivePID(pidfile)
	defer os.Remove(pidfile)
	if pid == 0 || !alive {
		return
	}
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	_ = syscall.Kill(pid, syscall.SIGKILL)
}

// ReapOrphanSessionsUnder walks rawRoot for every "*.pid" sibling a live
// session ever wrote (sessionAttemptPIDFile) and reaps each — the general
// "a later driver can reap an orphan" sweep O57 finding 1 asks for,
// scoped to one -raw-dir root. Returns the pids it found live (before
// reaping), for logging; a rawRoot that does not exist is not an error.
func ReapOrphanSessionsUnder(rawRoot string) []int {
	var reaped []int
	entries, err := os.ReadDir(rawRoot)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".pid") {
			continue
		}
		path := filepath.Join(rawRoot, e.Name())
		if pid, alive := readLivePID(path); alive {
			reaped = append(reaped, pid)
		}
		ReapOrphanSession(path)
	}
	return reaped
}

// RefuseIfLiveSessionInWorktree reports a non-empty refusal reason when
// wtPath's own session pidfile names a process that is still alive and
// still carries sessionCommandMarker — O57 finding 1: unlike a stray
// scratch daemon (fathom's own process, safely killed on sight), a live
// AGENT process found here means some driver invocation is (or very
// recently was) actually mid-session in this shared worktree, and resetting
// it out from under that session would either race a real edit or destroy
// one; PrepareSubject refuses the cell (LOST) instead of guessing. A dead
// or recycled pid is cleared (pidfile removed) and reports "".
func RefuseIfLiveSessionInWorktree(wtPath string) string {
	pidfile := sessionWorktreePIDFile(wtPath)
	pid, alive := readLivePID(pidfile)
	if pid == 0 {
		return "" // no pidfile, or unparsable — nothing to refuse over
	}
	if !alive {
		_ = os.Remove(pidfile)
		return ""
	}
	return fmt.Sprintf("a session (pid %d) is still running in this shared worktree — refusing to reset it out from under a live agent", pid)
}
