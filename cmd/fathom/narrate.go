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
	"github.com/geoffmilleraz/fathom/internal/statedir"
)

// Narrate mode (vision 14 §6, plan IO-21/22): the enhanced-understanding
// dial — explicitly costly, auto-expiring, off by default. While active,
// the hooks append STORY ROWS: per-intervention records (the rule served
// and where; the verify catch and what it blocked; the guard note that
// fired) to a per-repo JSONL under artifacts/narrate/ in the repo's state
// directory (statedir.Resolve — outside the repo tree by default, O63
// item 5). Story rows are a LENS over mechanism-labeled events, never a
// source of truth
// (law 14 candidate): nothing downstream — gates, metrics, bundles — may
// consume them as evidence, and they never enter the research bundle.
// The LLM weekly narrative is a later wave; v0 is capture only.

type narrateState struct {
	SchemaVersion int    `json:"schema_version"`
	FathomVersion string `json:"fathom_version"`

	On    string `json:"on"` // "story-rows" — the v0 capture tier
	SetAt string `json:"set_at"`
	Note  string `json:"note,omitempty"`

	Until             string `json:"until,omitempty"`
	SessionsRemaining *int   `json:"sessions_remaining,omitempty"`
}

const narrateSchemaVersion = 1

func narrateStatePath() string {
	if p := os.Getenv("FATHOM_NARRATE_STATE"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".fathom", "narrate.json")
}

func loadNarrateState(now time.Time) (narrateState, bool) {
	path := narrateStatePath()
	if path == "" {
		return narrateState{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return narrateState{}, false
	}
	var st narrateState
	if err := json.Unmarshal(data, &st); err != nil {
		return narrateState{}, false
	}
	if st.On == "" {
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

func narrateActive() bool {
	_, active := loadNarrateState(time.Now())
	return active
}

// recordStoryRow appends one story row while narrate is active. Best-effort
// and silent, exactly like emitBeat — narration must never break a hook.
// Rows land per-repo (artifacts/narrate/story-<date>.jsonl under projDir's
// state directory — statedir.Resolve, outside the repo tree by default,
// O63 item 5) so they stay with the code they narrate and never leave the
// machine.
func recordStoryRow(projDir, kind string, detail map[string]any) {
	if !narrateActive() || projDir == "" {
		return
	}
	dir := filepath.Join(statedir.Resolve(projDir), "artifacts", "narrate")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	row := map[string]any{
		"schema_version": narrateSchemaVersion,
		"fathom_version": buildinfo.Version(),
		"ts":             time.Now().UTC().Format(time.RFC3339Nano),
		"kind":           kind,
	}
	for k, v := range detail {
		row[k] = v
	}
	data, err := json.Marshal(row)
	if err != nil {
		return
	}
	path := filepath.Join(dir, "story-"+time.Now().UTC().Format("2006-01-02")+".jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(data, '\n'))
}

func cmdNarrate(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		st, active := loadNarrateState(time.Now())
		if active {
			bound := st.Until
			if st.SessionsRemaining != nil {
				bound += fmt.Sprintf(" / %d session(s) remaining", *st.SessionsRemaining)
			}
			fmt.Fprintf(stdout, "narrate: ON (story rows) until %s; set %s by fathom %s\n", bound, st.SetAt, st.FathomVersion)
		} else {
			fmt.Fprintln(stdout, "narrate: off (the default — story capture costs attention and disk; arm it deliberately, let it expire)")
		}
		return 0
	}
	switch args[0] {
	case "on":
		fs := flag.NewFlagSet("narrate on", flag.ContinueOnError)
		fs.SetOutput(stderr)
		forStr := fs.String("for", "", "wall-clock expiry (e.g. 14d, 2w, 336h)")
		sessions := fs.Int("sessions", 0, "session budget")
		note := fs.String("note", "", "why (lands in the state file)")
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
			fmt.Fprintln(stderr, "error: narrate needs a bound — pass --for and/or --sessions (auto-expiry is the design, vision 14 §6)")
			return 1
		}
		st := narrateState{SchemaVersion: narrateSchemaVersion, FathomVersion: buildinfo.Version(), On: "story-rows", SetAt: time.Now().UTC().Format(time.RFC3339), Note: *note}
		if forDur > 0 {
			st.Until = time.Now().UTC().Add(forDur).Format(time.RFC3339)
		}
		if *sessions > 0 {
			st.SessionsRemaining = sessions
		}
		if err := saveNarrateState(st); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, "narrate: ON (story rows) — hooks now append per-intervention rows under artifacts/narrate/ in each active repo's state directory (outside the repo tree by default); expires by its bound, or `fathom narrate off` ends it early")
		return 0
	case "off":
		if path := narrateStatePath(); path != "" {
			_ = os.Remove(path)
		}
		fmt.Fprintln(stdout, "narrate: off — captured story rows stay where they were written")
		return 0
	default:
		fmt.Fprintf(stderr, "usage: fathom narrate [on --for D --sessions N [--note ...] | off]\n")
		return 1
	}
}

// saveNarrateState persists st to narrateStatePath (0600, dir 0700).
func saveNarrateState(st narrateState) error {
	path := narrateStatePath()
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
