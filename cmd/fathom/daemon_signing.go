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
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/console"
)

// signingIdentity is what `codesign -dv` says about one binary.
type signingIdentity struct {
	Binary     string `json:"binary"`
	Identifier string `json:"identifier,omitempty"`
	Authority  string `json:"authority,omitempty"`
	AdHoc      bool   `json:"adhoc"`
	Unsigned   bool   `json:"unsigned,omitempty"`
	Error      string `json:"error,omitempty"`
}

// binarySigningIdentity runs `codesign -dv path` (which prints to stderr;
// ops.Run returns combined output) and parses the identity.
func binarySigningIdentity(ops daemonOps, path string) signingIdentity {
	si := signingIdentity{Binary: path}
	out, err := ops.Run("codesign", "-dv", path)
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

// String is the status line's value.
func (s signingIdentity) String() string {
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

// runningDaemonBinary resolves the executable of the daemon serving addr:
// the record's pid's command (when it is still a serve-http), else the
// installed plist's ProgramArguments[0]. "" when neither is known.
func runningDaemonBinary(ops daemonOps, addr, plistBinary string) (string, string) {
	home, _ := os.UserHomeDir()
	if p, err := console.DaemonRecordPath(home, addr); err == nil {
		if rec, err := console.ReadDaemonRecord(p); err == nil && rec.PID > 0 {
			if pi, alive := ops.Proc(rec.PID); alive && strings.Contains(pi.Command, "serve-http") {
				if exe := strings.Fields(pi.Command)[0]; filepath.IsAbs(exe) {
					return exe, fmt.Sprintf("running pid %d", rec.PID)
				}
			}
		}
	}
	if plistBinary != "" {
		return plistBinary, "the installed plist's binary (no running daemon record)"
	}
	return "", ""
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
