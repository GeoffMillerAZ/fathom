package main

// supervised.go — is this managed daemon run by its supervisor (design
// §8.4 step 5, O1363)? A managed serve-http started by hand from a
// session holds the managed label's port; the supervisor's own instance
// then found the port "already served", exited 0 and was never relaunched
// (KeepAlive SuccessfulExit=false), so the managed daemon lived on as a
// session's child. The start records whether a supervisor runs it, the
// held-port step reads that from the holder's record, and the hand start
// is told which command restarts the supervised daemon instead.

import (
	"fmt"
	"runtime"
	"strings"
)

// detectSupervised reports whether launchd or systemd runs this process:
// on darwin XPC_SERVICE_NAME equals the managed label (what launchd sets
// for a job, the same rule detectManaged uses); on linux INVOCATION_ID is
// set (systemd sets it for every unit it starts); on either, init as the
// parent (launchd and a reparented daemon). getenv and ppid are injected.
func detectSupervised(goos, label string, getenv func(string) string, ppid int) bool {
	if ppid == 1 {
		return true
	}
	switch goos {
	case "darwin":
		return label != "" && getenv("XPC_SERVICE_NAME") == label
	case "linux":
		return getenv("INVOCATION_ID") != ""
	}
	return false
}

// supervisorRestartCommand is the command that restarts the supervised
// managed daemon: launchd's kickstart of the label on darwin (the form
// daemonRestartHint renders), the user's systemd unit elsewhere (fathom
// does not install one, so the unit is the adopter's).
func supervisorRestartCommand(goos, label string) string {
	if goos == "darwin" {
		return fmt.Sprintf("launchctl kickstart -k gui/$(id -u)/%s", label)
	}
	return "systemctl --user restart <unit>   # the systemd unit that runs fathom serve-http --managed"
}

// unsupervisedStartLine is the one stderr line a managed start with no
// supervisor logs. The start is not refused: other supervisors exist.
func unsupervisedStartLine(goos, label string, ppid int) string {
	return fmt.Sprintf("serve-http: managed, but no supervisor runs this process (parent pid %d) — it is not relaunched when it exits, and while it holds the port the supervised daemon cannot take it over; stop it and restart the supervised daemon: %s", ppid, supervisorRestartCommand(goos, label))
}

// handStartWarning closes the daemon-down text an agent reads.
const handStartWarning = "never start `fathom serve-http --managed` by hand from a session: a hand-started daemon has no supervisor and keeps the supervised one off the port"

// daemonDownHintFor is the restart hint the daemon-down text carries: the
// shared hint (daemonRestartHint), except that where launchd is absent its
// foreground serve-http command becomes the systemd restart, and the
// hand-start warning appended.
func daemonDownHintFor(goos, hint string) string {
	if goos != "darwin" && strings.HasPrefix(hint, "fathom serve-http ") {
		hint = supervisorRestartCommand(goos, "")
	}
	return hint + " — " + handStartWarning
}

// daemonDownHint is daemonDownHintFor on this platform.
func daemonDownHint() string { return daemonDownHintFor(runtime.GOOS, daemonRestartHint("")) }
