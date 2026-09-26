package main

// daemon_json.go — `fathom daemon status --json` (RP-17,
// docs/plans/reach-parity.md): the installed supervisor (or its absence,
// with the platform), reachability, and the remedy line as data.

import (
	"encoding/json"
	"io"
)

// daemonStatusJSON is the `fathom daemon status --json` envelope.
type daemonStatusJSON struct {
	Platform string `json:"platform"`
	// Label is the launchd label discovered on this machine; "" when none
	// is installed (or the platform has no launchd).
	Label string `json:"label,omitempty"`
	Plist string `json:"plist,omitempty"`
	URL   string `json:"url"`
	// Reachable is the fact that matters: /healthz answered 200.
	// PlistSchema, PostureDrift and PostureRemedy are the installed plist's
	// launchd posture against schema 2 (RT-15); the remedy is reconcile
	// then the reload line, for the operator's terminal.
	PlistSchema   int      `json:"plist_schema,omitempty"`
	PostureDrift  []string `json:"posture_drift,omitempty"`
	PostureRemedy string   `json:"posture_remedy,omitempty"`
	// Switch is the kill switch's state line when disabled.json or
	// paused.json is set (RT-3/RT-6).
	Switch string `json:"switch,omitempty"`
	// Signing is the running daemon binary's codesigning identity (RT-148).
	Signing   *signingIdentity `json:"signing,omitempty"`
	Reachable bool             `json:"reachable"`
	Detail    string           `json:"detail,omitempty"`
	Remedy    string           `json:"remedy,omitempty"`
	ExitCode  int              `json:"exit_code"`
}

// renderDaemonStatusJSON writes the envelope and returns its exit code
// (0 reachable, 1 not — the same contract as the text path).
func renderDaemonStatusJSON(w io.Writer, st daemonStatusJSON) int {
	if !st.Reachable {
		st.ExitCode = 1
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(st)
	return st.ExitCode
}
