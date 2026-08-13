package main

// lifecyclehooks.go — SessionEnd and PreCompact adapters
// (docs/design/continuity-threads.md §3), plus the session stamp that lets
// the Stop hook label ledger rows with how the session began.
//
// Every deposit here is a MECHANICAL fact — git state, counts, ids. The
// transcript-mining refusal is structural: these commands receive the
// transcript PATH and never open it.

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// sessionStamp records how a session began and what the brief cost — the
// labeling that makes the vision-21 bet measurable (fresh-with-brief vs
// resumed cohorts). Written by the SessionStart hook, joined and removed
// by the Stop hook's ledger write.
type sessionStamp struct {
	SchemaVersion int    `json:"schema_version"`
	SessionID     string `json:"session_id"`
	Source        string `json:"source"` // startup|resume|clear|compact|fork
	BriefInjected bool   `json:"brief_injected"`
	BriefBytes    int    `json:"brief_bytes"`
	At            string `json:"at"`
}

func sessionStampDir() string {
	if p := os.Getenv("FATHOM_SESSION_STAMPS"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".fathom", "session-stamps")
}

func writeSessionStamp(st sessionStamp) {
	dir := sessionStampDir()
	if dir == "" || st.SessionID == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	raw, err := json.Marshal(st)
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(dir, st.SessionID+".json"), raw, 0o600)
}

// readSessionStamp returns the stamp for a session, if one exists. A
// missing stamp yields the zero value: the ledger then records
// session_source "" — unknown, never guessed (law 3).
func readSessionStamp(sessionID string) sessionStamp {
	if sessionID == "" {
		return sessionStamp{}
	}
	raw, err := os.ReadFile(filepath.Join(sessionStampDir(), sessionID+".json"))
	if err != nil {
		return sessionStamp{}
	}
	var st sessionStamp
	if json.Unmarshal(raw, &st) != nil {
		return sessionStamp{}
	}
	return st
}

// lifecycleHookInput is the stdin envelope both lifecycle hooks receive.
type lifecycleHookInput struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	CWD            string `json:"cwd"`
	Reason         string `json:"reason"`  // SessionEnd
	Trigger        string `json:"trigger"` // PreCompact: manual|auto
}

func readLifecycleInput() (lifecycleHookInput, bool) {
	raw, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
	if err != nil {
		return lifecycleHookInput{}, false
	}
	var in lifecycleHookInput
	if json.Unmarshal(raw, &in) != nil {
		return lifecycleHookInput{}, false
	}
	return in, true
}

// cmdSessionEndHook deposits ONE mechanical item when a session ends with
// something worth pointing at — uncommitted work, stashes, or
// claimed-unproven plan rows. A clean, empty session deposits NOTHING:
// silence is fine, and a store full of "session ended" rows is noise
// pretending to be continuity.
func cmdSessionEndHook(args []string, stdout, stderr io.Writer) int {
	if fathomChannelsDisabled() {
		return 0
	}
	fs := newFlagSet("session-end-hook", stderr)
	if err := fs.Parse(args); err != nil {
		return 0 // hook contract: silent
	}
	in, ok := readLifecycleInput()
	if !ok || in.SessionID == "" {
		return 0
	}
	// Only the reasons where context is actually LOST. A resume-shaped
	// end keeps its context; depositing there duplicates what survives.
	if in.Reason != "clear" && in.Reason != "logout" && in.Reason != "exit" && in.Reason != "prompt_input_exit" {
		return 0
	}
	root := in.CWD
	if root == "" {
		root, _ = os.Getwd()
	}
	facts := mechanicalSessionFacts(root)
	if facts == "" {
		return 0 // nothing in flight: nothing to say
	}
	repo := strings.ToLower(filepath.Base(root))
	now := time.Now()
	_ = appendThreadItem(threadItem{
		SchemaVersion: threadSchemaVersion,
		ID:            newThreadID(now, in.SessionID+facts),
		Thread:        repo,
		Kind:          "session-end",
		Text:          fmt.Sprintf("session ended (%s) with %s", in.Reason, facts),
		Pointers:      map[string]string{"session_id": in.SessionID, "repo": repo},
		FiledAt:       now.UTC().Format(time.RFC3339),
		FiledBy:       "hook",
	})
	return 0
}

// cmdPreCompactHook files one pointer BEFORE compaction — the exact moment
// in-flight context dies today. The pointer makes the pre-summary span
// findable (the transcript stays on disk, keyed by session id); the
// transcript itself is never read.
func cmdPreCompactHook(args []string, stdout, stderr io.Writer) int {
	if fathomChannelsDisabled() {
		return 0
	}
	fs := newFlagSet("pre-compact-hook", stderr)
	if err := fs.Parse(args); err != nil {
		return 0
	}
	in, ok := readLifecycleInput()
	if !ok || in.SessionID == "" {
		return 0
	}
	root := in.CWD
	if root == "" {
		root, _ = os.Getwd()
	}
	repo := strings.ToLower(filepath.Base(root))
	now := time.Now()
	_ = appendThreadItem(threadItem{
		SchemaVersion: threadSchemaVersion,
		ID:            newThreadID(now, "precompact"+in.SessionID),
		Thread:        repo,
		Kind:          "pre-compact",
		Text:          fmt.Sprintf("compaction (%s) at %s — the full pre-summary detail stays in this session's transcript", orUnknown(in.Trigger), now.UTC().Format("15:04Z")),
		Pointers:      map[string]string{"session_id": in.SessionID, "repo": repo},
		FiledAt:       now.UTC().Format(time.RFC3339),
		FiledBy:       "hook",
	})
	return 0
}

// mechanicalSessionFacts summarizes what a departing session leaves in
// flight — git facts plus claimed-unproven plan rows. Empty when nothing
// is in flight.
func mechanicalSessionFacts(root string) string {
	run := func(args ...string) string {
		out, err := exec.Command("git", append([]string{"-C", root}, args...)...).Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	}
	var parts []string
	if s := run("status", "--porcelain"); s != "" {
		parts = append(parts, fmt.Sprintf("%d uncommitted file(s)", len(strings.Split(s, "\n"))))
	}
	if st := run("stash", "list"); st != "" {
		parts = append(parts, fmt.Sprintf("%d stash(es)", len(strings.Split(st, "\n"))))
	}
	if b := run("rev-parse", "--abbrev-ref", "HEAD"); b != "" && b != "main" && b != "master" {
		parts = append(parts, "branch "+b)
	}
	if n := countClaimedUnproven(root); n > 0 {
		parts = append(parts, fmt.Sprintf("%d claimed-unproven plan row(s)", n))
	}
	return strings.Join(parts, ", ")
}

// countClaimedUnproven counts [x] rows carrying an {id: …} whose proof the
// gate has not blessed — approximated mechanically as checked rows in
// plans modified since HEAD (uncommitted plan edits are exactly the
// claims a departing session leaves hanging).
func countClaimedUnproven(root string) int {
	out, err := exec.Command("git", "-C", root, "diff", "--name-only", "--", "docs/plans").Output()
	if err != nil {
		return 0
	}
	n := 0
	for _, f := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if f == "" {
			continue
		}
		data, rerr := os.ReadFile(filepath.Join(root, f))
		if rerr != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			t := strings.TrimSpace(line)
			if strings.HasPrefix(t, "- [x]") && strings.Contains(t, "{id: ") {
				n++
			}
		}
	}
	return n
}

func orUnknown(s string) string {
	if s == "" {
		return "unknown trigger"
	}
	return s
}
