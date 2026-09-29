package metrics

// armstate.go — IT-2: one shared, arm-aware absence note. Several surfaces
// (fathom flywheel's call-log staleness check, AggregateChannels' guard/
// module "not measurable" notes) can read as a defect — a missing hook, a
// dead channel — when the REAL cause is a measurement OFF arm (vision 14
// §2) deliberately silencing hook channels for a counterfactual comparison.
// Blaming a missing hook for an engineered silence is itself a false-defect
// report; this file gives every such surface one honest way to check.
//
// cmd/fathom (package main) already owns a richer arm reader (arm.go's
// armState/loadArmState — schema_version, until, note) that this package
// cannot import (internal/metrics cannot depend on package main). This is
// a deliberately narrower, independent reader over the same on-disk file,
// covering only the fields absence-reporting needs — cmd/fathom/doctor.go
// keeps its own read via arm.go, exactly as intended.

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/geoffmilleraz/fathom/internal/fathomhome"
)

// ArmState is the subset of the on-disk arm ledger (~/.fathom/arm.json)
// this package reads: {"arm":"off","set_at":RFC3339,"sessions_remaining":N}.
type ArmState struct {
	Arm               string
	SetAt             time.Time
	SessionsRemaining *int
}

// DefaultArmStatePath returns the arm ledger path: FATHOM_ARM_STATE when
// set (the same override cmd/fathom's own armStatePath honors, and the
// injection seam tests use), else ~/.fathom/arm.json. "" when $HOME can't
// be resolved.
func DefaultArmStatePath() string {
	return fathomhome.PerFile("FATHOM_ARM_STATE", "arm.json")
}

// ArmStateForWindow reads the arm ledger at path (DefaultArmStatePath()
// when path == "") and reports whether an OFF arm covers any part of
// [from, to]. This reader's schema carries no expiry timestamp (only a
// session-count budget), so an arm set at or before `to` is treated as
// covering the window unless its session budget already reads exhausted
// (<=0) — a conservative, read-only check. Absent, unreadable, malformed,
// non-"off", or not-yet-set-at-window-time state is "no arm", silently:
// this must never be the reason a caller fails or blocks.
func ArmStateForWindow(path string, from, to time.Time) (ArmState, bool) {
	if path == "" {
		path = DefaultArmStatePath()
	}
	if path == "" {
		return ArmState{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ArmState{}, false
	}
	var raw struct {
		Arm               string `json:"arm"`
		SetAt             string `json:"set_at"`
		SessionsRemaining *int   `json:"sessions_remaining"`
	}
	if json.Unmarshal(data, &raw) != nil || raw.Arm != "off" {
		return ArmState{}, false
	}
	setAt, err := time.Parse(time.RFC3339, raw.SetAt)
	if err != nil {
		return ArmState{}, false
	}
	st := ArmState{Arm: raw.Arm, SetAt: setAt, SessionsRemaining: raw.SessionsRemaining}
	if setAt.After(to) {
		return st, false // the arm did not exist yet during this window
	}
	if raw.SessionsRemaining != nil && *raw.SessionsRemaining <= 0 {
		return st, false // budget already exhausted
	}
	return st, true
}

// Note is the standard absence line every consumer renders when the OFF
// arm — not a missing/broken hook — explains a silent channel: "absence is
// the experiment," not a defect to chase.
func (s ArmState) Note() string {
	n := "N"
	if s.SessionsRemaining != nil {
		n = strconv.Itoa(*s.SessionsRemaining)
	}
	return fmt.Sprintf("OFF measurement arm active (%s session(s) remaining) — hook channels silent by design; absence is the experiment", n)
}
