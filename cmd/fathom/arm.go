package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/geoffmilleraz/fathom/internal/buildinfo"
)

// The measurement arm (vision 14 §2, plan IO-14): `fathom arm off` is the
// counterfactual program's OFF switch — FATHOM_CHANNELS=off with a memory.
// Unlike the bare env var (the O15 lesson: a silent kill switch is a
// measurement bug), an arm is STAMPED (who set it, when, with what bound),
// AUTO-EXPIRING (a wall-clock deadline and/or a session budget — never
// indefinite unless explicitly so), ANNOUNCED (doctor and the session hook
// both surface an active arm loudly), and RECORDED (the session ledger
// stamps every row with the arm it ran under, which is what makes paired
// comparison possible at all).

// armState is the JSON at ~/.fathom/arm.json (0600, like the ledger).
type armState struct {
	SchemaVersion int    `json:"schema_version"`
	FathomVersion string `json:"fathom_version"`

	Arm   string `json:"arm"` // "off" — channels disabled for measurement
	SetAt string `json:"set_at"`
	Note  string `json:"note,omitempty"`

	// Until, when non-empty, is the RFC3339 expiry — the arm deactivates
	// itself at that instant with no further action.
	Until string `json:"until,omitempty"`
	// SessionsRemaining, when non-nil, is a countdown the session hook
	// decrements; the arm deactivates at zero.
	SessionsRemaining *int `json:"sessions_remaining,omitempty"`
}

const armSchemaVersion = 1

func armStatePath() string {
	if p := os.Getenv("FATHOM_ARM_STATE"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".fathom", "arm.json")
}

// loadArmState returns the current arm state and whether an OFF arm is
// ACTIVE right now (expiry evaluated against now). Absent/unreadable state
// is "no arm", silently — the arm machinery must never break a hook.
func loadArmState(now time.Time) (armState, bool) {
	path := armStatePath()
	if path == "" {
		return armState{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return armState{}, false
	}
	var st armState
	if err := json.Unmarshal(data, &st); err != nil {
		return armState{}, false
	}
	if st.Arm != "off" {
		return st, false
	}
	if st.Until != "" {
		if t, err := time.Parse(time.RFC3339, st.Until); err == nil && now.After(t) {
			return st, false
		}
	}
	if st.SessionsRemaining != nil && *st.SessionsRemaining <= 0 {
		return st, false
	}
	return st, true
}

// armOffActive is the hook-facing check composed into
// fathomChannelsDisabled — the stateful sibling of FATHOM_CHANNELS=off.
func armOffActive() bool {
	_, active := loadArmState(time.Now())
	return active
}

// decrementArmSessions is called by the session hook once per session
// start while an OFF arm with a session budget is active. Best-effort.
func decrementArmSessions() {
	st, active := loadArmState(time.Now())
	if !active || st.SessionsRemaining == nil {
		return
	}
	n := *st.SessionsRemaining - 1
	st.SessionsRemaining = &n
	writeArmState(st)
}

func writeArmState(st armState) error {
	path := armStatePath()
	if path == "" {
		return fmt.Errorf("no home directory")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

// currentArmLabel returns the label the session ledger stamps: "off" while
// an OFF arm is active, "on" otherwise (the default arm — channels live).
func currentArmLabel(now time.Time) string {
	if _, active := loadArmState(now); active {
		return "off"
	}
	return "on"
}

func cmdArm(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		st, active := loadArmState(time.Now())
		switch {
		case active:
			left := ""
			if st.Until != "" {
				left = " until " + st.Until
			}
			if st.SessionsRemaining != nil {
				left += fmt.Sprintf(" (%d session(s) remaining)", *st.SessionsRemaining)
			}
			fmt.Fprintf(stdout, "arm: OFF — channels disabled for measurement%s; set %s by fathom %s\n", left, st.SetAt, st.FathomVersion)
		case st.Arm == "off":
			fmt.Fprintln(stdout, "arm: on (a previous OFF arm has expired; ledger rows stamped while it ran keep their arm)")
		default:
			fmt.Fprintln(stdout, "arm: on (channels live — the default)")
		}
		return 0
	}

	switch args[0] {
	case "off":
		fs := flag.NewFlagSet("arm off", flag.ContinueOnError)
		fs.SetOutput(stderr)
		forStr := fs.String("for", "", "wall-clock expiry (e.g. 14d or 48h) — the arm turns itself off")
		sessions := fs.Int("sessions", 0, "session budget — the session hook counts it down")
		note := fs.String("note", "", "why this arm exists (lands in the state file)")
		if err := fs.Parse(args[1:]); err != nil {
			return 1
		}
		var forDur time.Duration
		if *forStr != "" {
			var derr error
			forDur, derr = parseHumaneDuration(*forStr)
			if derr != nil {
				fmt.Fprintf(stderr, "error: %v\n", derr)
				return 1
			}
		}
		if forDur <= 0 && *sessions <= 0 {
			fmt.Fprintln(stderr, "error: an OFF arm needs a bound — pass --for and/or --sessions (an indefinite kill switch is FATHOM_CHANNELS=off, deliberately less comfortable)")
			return 1
		}
		st := armState{SchemaVersion: armSchemaVersion, FathomVersion: buildinfo.Version(), Arm: "off", SetAt: time.Now().UTC().Format(time.RFC3339), Note: *note}
		if forDur > 0 {
			st.Until = time.Now().UTC().Add(forDur).Format(time.RFC3339)
		}
		if *sessions > 0 {
			st.SessionsRemaining = sessions
		}
		if err := writeArmState(st); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "arm: OFF recorded — channels disabled for measurement (doctor and the session hook will announce it). Undo: fathom arm on\n")
		return 0
	case "on":
		path := armStatePath()
		if path != "" {
			_ = os.Remove(path)
		}
		fmt.Fprintln(stdout, "arm: on — channels live")
		return 0
	default:
		fmt.Fprintf(stderr, "usage: fathom arm [off --for D --sessions N [--note ...] | on]\n")
		return 1
	}
}
