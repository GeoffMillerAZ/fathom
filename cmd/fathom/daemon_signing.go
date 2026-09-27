package main

// daemon_signing.go — the running daemon's codesigning identity and its
// launchd posture, for `fathom daemon status` and doctor (design §8.3,
// §8.4; RT-148, RT-15).
//
// A TCC grant (Full Disk Access, Removable Volumes) given to an AD-HOC
// binary pins that build's cdhash, so every rebuild voids it and the daemon
// wedges in an unpromptable consult (O10, INC-5). `task deploy` now refuses
// to install an unsigned binary (RT-147); status names what is actually
// running, because the binary on disk and the binary serving can differ.

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// signingIdentity is what `codesign -dv` says about one binary.
type signingIdentity struct {
	Binary     string `json:"binary"`
	Identifier string `json:"identifier,omitempty"`
	Authority  string `json:"authority,omitempty"`
	AdHoc      bool   `json:"adhoc"`
	Unsigned   bool   `json:"unsigned,omitempty"`
	Error      string `json:"error,omitempty"`
	// Source says what was read: "running pid N" (codesign on the live
	// process) or the on-disk file (no running daemon found).
	Source string `json:"source,omitempty"`
}

// binarySigningIdentityOf reads target's signature (a pid or a path) and
// records where the reading came from, so status never passes a reading of
// the file on disk off as the running process's.
func binarySigningIdentityOf(ops daemonOps, target, source string) signingIdentity {
	si := binarySigningIdentity(ops, target)
	si.Source = source
	return si
}

// binarySigningIdentity runs `codesign -dv target` (which prints to stderr;
// ops.Run returns combined output) and parses the identity. target is a
// path or a pid: codesign reads a pid's signature from the running process
// itself, which after a deploy's `mv` is no longer the file at its path.
func binarySigningIdentity(ops daemonOps, target string) signingIdentity {
	si := signingIdentity{Binary: target}
	if _, err := strconv.Atoi(target); err == nil {
		si.Binary = "pid " + target
	}
	out, err := ops.Run("codesign", "-dv", target)
	text := string(out)
	if strings.Contains(text, "not signed at all") {
		si.Unsigned = true
		return si
	}
	for _, line := range strings.Split(text, "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch k {
		case "Executable":
			si.Binary = v
		case "Identifier":
			si.Identifier = v
		case "Authority":
			if si.Authority == "" {
				si.Authority = v // the leaf: the first Authority line
			}
		case "Signature":
			if v == "adhoc" {
				si.AdHoc = true
			}
		}
	}
	if err != nil && si.Identifier == "" {
		si.Error = strings.TrimSpace(firstNonEmpty(firstLine(text), err.Error()))
	}
	return si
}

// String is the status line's value, naming what was read.
func (s signingIdentity) String() string {
	line := s.identity()
	if s.Source != "" {
		line += " [" + s.Source + "]"
	}
	return line
}

func (s signingIdentity) identity() string {
	switch {
	case s.Error != "":
		return fmt.Sprintf("unknown for %s (codesign: %s)", s.Binary, s.Error)
	case s.Unsigned:
		return fmt.Sprintf("unsigned %s — no TCC grant can hold; task deploy signs with fathom-dev", s.Binary)
	case s.AdHoc:
		return fmt.Sprintf("AD-HOC %s (Codesigning ID %s) — a TCC grant pins this build's hash and every rebuild voids it (the likely cause of an EPERM wedge); task deploy signs with fathom-dev", s.Binary, firstNonEmpty(s.Identifier, "unknown"))
	case s.Authority != "":
		return fmt.Sprintf("%s (identifier %s) %s", s.Authority, firstNonEmpty(s.Identifier, "unknown"), s.Binary)
	default:
		return fmt.Sprintf("signed, no authority named (identifier %s) %s", firstNonEmpty(s.Identifier, "unknown"), s.Binary)
	}
}

// runningDaemonBinary resolves what to read the serving daemon's signature
// from, and says which: the live process (its pid, found by
// findManagedDaemon behind the pid-reuse guard) — never the file at its
// path, which a deploy's `mv` replaces while the old process keeps serving
// — else the on-disk binary the installed plist names. It returns the
// codesign target, its source line and the pid (0 for the file).
func runningDaemonBinary(ops daemonOps, addr, plistBinary string) (string, string, int) {
	if pid, _ := findManagedDaemon(ops, addr); pid > 0 {
		return strconv.Itoa(pid), fmt.Sprintf("running pid %d", pid), pid
	}
	if plistBinary != "" {
		return plistBinary, "on-disk file named by the installed plist; no running daemon found", 0
	}
	return "", "", 0
}

// checkDaemonPosture is doctor's daemon_posture row: the installed plist's
// tenancy keys against schema 2. WARN (never FAIL) on drift, naming the
// exact reconcile command for the resolved label and then the reload line —
// a remedy printed for the operator's terminal, never run by doctor.
func checkDaemonPosture(label string) doctorCheck {
	const name = "daemon_posture"
	lbl, plistPath, post, ok := installedPosture(label)
	if lbl == "" {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "no fathom launchd agent installed"}
	}
	if !ok {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("%s could not be read or parsed", plistPath)}
	}
	if len(post.Drift) == 0 {
		return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("%s is schema %d (exit 0 stays down, 60 s throttle, Background)", lbl, post.Schema)}
	}
	return doctorCheck{
		Name: name, Status: doctorWarn,
		Reason: postureReason(post),
		Remedy: postureRemedy(lbl, plistPath, post),
	}
}

// postureReason is the one-line drift summary shared by doctor and status.
func postureReason(post plistPostureReport) string {
	head := fmt.Sprintf("plist schema %d", post.Schema)
	if post.UnconditionalKeepAlive {
		head += fmt.Sprintf(" (unconditional KeepAlive: every exit relaunched within %d s)", post.ThrottleSeconds)
	}
	return head + ": " + strings.Join(post.Drift, "; ")
}

// postureRemedy is reconcile, then the reload line (run from the operator's
// own terminal: launchctl from an agent shell wedges).
func postureRemedy(label, plistPath string, post plistPostureReport) string {
	uid := os.Getuid()
	return fmt.Sprintf("%s, then from your terminal: launchctl bootout gui/%d/%s && launchctl bootstrap gui/%d %s",
		reconcileCommand(label, post), uid, label, uid, plistPath)
}
