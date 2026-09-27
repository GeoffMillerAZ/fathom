// Package tenancy holds fathom's responsible-tenant machinery (vision 36,
// ADR-047, docs/design/responsible-tenant.md): what the daemon may cost the
// machine it shares, and how the operator turns it off.
package tenancy

// killswitch.go — the kill switch (design §9, RT-6). The operator could not
// stop the daemon on 2026-09-26: launchd relaunched it and bootout failed
// from a broken session (INC-23). `~/.fathom/daemon/disabled.json` is
// fathom's own off switch, read before config, store or bind; this file is
// the pure verdict over the two switch files, so the startup check stays a
// stat-and-read with nothing else to go wrong.

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Exit codes a switched-off start uses. 0 means "stay down" to a schema-2
// plist (KeepAlive {SuccessfulExit: false}); 75 (EX_TEMPFAIL) means "not
// yet" — launchd retries it after the throttle, which is what `--for` wants.
const (
	ExitStayDown   = 0
	ExitRetryLater = 75
)

// Switch file names under the daemon state directory (~/.fathom/daemon).
const (
	DisabledFile = "disabled.json"
	PausedFile   = "paused.json"
)

// SwitchRecord is the body of disabled.json / paused.json. Every field is
// optional: an empty file (the "no binary available" `touch`) is a valid
// switch that reads "reason unknown".
type SwitchRecord struct {
	By     string    `json:"by,omitempty"`
	Reason string    `json:"reason,omitempty"`
	Detail string    `json:"detail,omitempty"`
	Since  time.Time `json:"since,omitzero"`
	Until  time.Time `json:"until,omitzero"`
	// Addr is the managed daemon's listen address (host:port). Every writer
	// records it: the hooks' kill-switch stat (RT-10) silences only hooks
	// addressing this daemon, never a scratch daemon elsewhere (§2.3, §9).
	Addr string `json:"addr,omitempty"`
}

// SwitchFile is one switch file as the caller found it: Present when it
// exists (or may exist: a stat or read that failed for any reason but "no
// such file" — I38), Raw its bytes (possibly empty or malformed), ReadErr
// why it could not be read, when it could not.
type SwitchFile struct {
	Present bool
	Raw     []byte
	ReadErr string
}

// unreadableError marks a switch file that is present but unreadable.
type unreadableError struct{ msg string }

func (e unreadableError) Error() string { return e.msg }

// parseSwitchFile decodes f, turning a read failure into an unreadableError.
func parseSwitchFile(f SwitchFile) (SwitchRecord, error) {
	if f.ReadErr != "" {
		return SwitchRecord{}, unreadableError{f.ReadErr}
	}
	return ParseSwitch(f.Raw)
}

// KillSwitchInput is everything EvaluateKillSwitch needs; the caller does
// the I/O (a stat and a small read per file) and injects the clock.
type KillSwitchInput struct {
	Disabled SwitchFile
	Paused   SwitchFile
	Now      time.Time
}

// KillSwitchAction is what a starting (or ticking) daemon does.
type KillSwitchAction int

const (
	// ActionRun: no switch applies; start normally.
	ActionRun KillSwitchAction = iota
	// ActionExitStay: disabled; exit ExitStayDown before config, store or bind.
	ActionExitStay
	// ActionExitRetry: disabled until a future time; exit ExitRetryLater.
	ActionExitRetry
	// ActionSafeMode: paused; stay up in safe mode.
	ActionSafeMode
)

func (a KillSwitchAction) String() string {
	switch a {
	case ActionExitStay:
		return "disabled"
	case ActionExitRetry:
		return "disabled-until"
	case ActionSafeMode:
		return "paused"
	default:
		return "run"
	}
}

// KillSwitchVerdict is the decision plus the one line the daemon logs.
type KillSwitchVerdict struct {
	Action   KillSwitchAction
	ExitCode int
	Record   SwitchRecord
	// Expired is true when a timed stop was found but its until has passed.
	Expired bool
	// Malformed is true when a present switch file did not parse; the
	// switch still applies.
	Malformed bool
	Message   string
}

// ParseSwitch decodes a switch file body. Empty or whitespace-only input is
// the zero record without error.
func ParseSwitch(raw []byte) (SwitchRecord, error) {
	var rec SwitchRecord
	trimmed := true
	for _, b := range raw {
		if b != ' ' && b != '\n' && b != '\r' && b != '\t' {
			trimmed = false
			break
		}
	}
	if trimmed {
		return rec, nil
	}
	if err := json.Unmarshal(raw, &rec); err != nil {
		return SwitchRecord{}, err
	}
	return rec, nil
}

// MarshalSwitch renders a switch record as indented JSON with a newline.
func MarshalSwitch(rec SwitchRecord) []byte {
	raw, _ := json.MarshalIndent(rec, "", "  ")
	return append(raw, '\n')
}

// EvaluateKillSwitch is the pure verdict over the two switch files.
// disabled.json outranks paused.json. A present-but-malformed file still
// applies: a switch the operator wrote is never ignored for its syntax.
func EvaluateKillSwitch(in KillSwitchInput) KillSwitchVerdict {
	if in.Disabled.Present {
		rec, err := parseSwitchFile(in.Disabled)
		v := KillSwitchVerdict{Record: rec, Malformed: err != nil}
		if !rec.Until.IsZero() && !in.Now.Before(rec.Until) {
			// The timed stop has passed: start normally, and say so once.
			v.Action = ActionRun
			v.Expired = true
			v.Message = "fathom: the timed stop " + describe(rec, err) + " expired at " + rec.Until.UTC().Format(time.RFC3339) + " — starting normally"
			return v
		}
		if !rec.Until.IsZero() {
			v.Action = ActionExitRetry
			v.ExitCode = ExitRetryLater
			v.Message = fmt.Sprintf("fathom: disabled %s until %s — exiting %d (retried later); `fathom daemon start` re-enables now",
				describe(rec, err), rec.Until.UTC().Format(time.RFC3339), ExitRetryLater)
			return v
		}
		v.Action = ActionExitStay
		v.ExitCode = ExitStayDown
		v.Message = "fathom: disabled " + describe(rec, err) + " — exiting 0 before config, store or bind; `fathom daemon start` re-enables"
		return v
	}
	if in.Paused.Present {
		rec, err := parseSwitchFile(in.Paused)
		return KillSwitchVerdict{
			Action: ActionSafeMode, Record: rec, Malformed: err != nil,
			Message: "fathom: paused " + describe(rec, err) + " — serving with the safe-mode flag set (its effects, auto-refresh off and heavy requests refused, arrive with RT-39/RT-150; nothing is curtailed yet); `fathom daemon resume` clears it",
		}
	}
	return KillSwitchVerdict{Action: ActionRun}
}

// describe renders "by X since T: reason" for the verdict line.
func describe(rec SwitchRecord, parseErr error) string {
	var unreadable unreadableError
	if errors.As(parseErr, &unreadable) {
		return "(the switch file is unreadable: " + unreadable.msg + "; reason unknown)"
	}
	if parseErr != nil {
		return "(the switch file is not valid JSON: " + parseErr.Error() + "; reason unknown)"
	}
	s := ""
	if rec.By != "" {
		s += "by " + rec.By
	}
	if !rec.Since.IsZero() {
		if s != "" {
			s += " "
		}
		s += "since " + rec.Since.UTC().Format(time.RFC3339)
	}
	reason := rec.Reason
	if reason == "" {
		reason = "reason unknown"
	}
	if rec.Detail != "" {
		reason += " (" + rec.Detail + ")"
	}
	if s == "" {
		return "(" + reason + ")"
	}
	return "(" + s + ": " + reason + ")"
}
