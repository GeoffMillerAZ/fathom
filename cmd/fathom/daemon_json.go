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
	Reachable bool   `json:"reachable"`
	Detail    string `json:"detail,omitempty"`
	Remedy    string `json:"remedy,omitempty"`
	ExitCode  int    `json:"exit_code"`
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
