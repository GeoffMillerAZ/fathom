package main

// These are existing config.toml controls, not capabilities.cue switches.
// Keep their mutation on the same configuration the hook actually consumes.
import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
	toml "github.com/pelletier/go-toml/v2"
)

type hookSeamRow struct {
	ID      string `json:"id"`
	State   string `json:"state"`
	Reason  string `json:"reason,omitempty"`
	Cost    string `json:"cost"`
	Command string `json:"command"`
}

var hookSeamIDs = []string{"claim_check", "gate_preflight", "prompt_definitions", "read_outline_hint", "readouts", "tests_to_run"}

func hookSeamKnown(id string) bool {
	for _, v := range hookSeamIDs {
		if id == v {
			return true
		}
	}
	return false
}
func hookSeamOn(cfg config.Config, id string) bool {
	switch id {
	case "claim_check":
		return cfg.Hooks.ClaimCheck
	case "gate_preflight":
		return cfg.Hooks.GatePreflight
	case "prompt_definitions":
		return cfg.Hooks.PromptDefinitions
	case "read_outline_hint":
		return cfg.Hooks.ReadOutlineHint
	case "tests_to_run":
		return cfg.Hooks.TestsToRun
	case "readouts":
		return cfg.Readouts.Mode != "" && cfg.Readouts.Mode != config.ReadoutsOff
	}
	return false
}

func hookSeams(root string) []hookSeamRow {
	loaded, err := config.Load(config.DefaultUserPath(), projectdir.Config.Path(root))
	rows := make([]hookSeamRow, 0, len(hookSeamIDs))
	for _, id := range hookSeamIDs {
		state := "off by choice"
		next := "on"
		reason := "default off"
		if err != nil {
			state = "on but not ready"
			reason = "config unreadable: " + err.Error()
		} else if hookSeamOn(loaded.Config, id) {
			state = "on"
			next = "off"
			reason = "configured; wiring and use are shown by fathom doctor"
		}
		command := "fathom capabilities set " + id + " " + next
		cost := "local reads; no model spend"
		switch id {
		case "readouts":
			command += " --scope user"
			cost = "local output processing; opt-in daemon joins in rich mode"
		case "tests_to_run":
			cost = "one daemon reach query per eligible edit"
		case "gate_preflight":
			cost = "local evidence and daemon verify query on commit/push"
		case "prompt_definitions":
			cost = "no additional daemon call; definitions already resolved"
		}
		rows = append(rows, hookSeamRow{ID: id, State: state, Reason: reason, Cost: cost, Command: command})
	}
	return rows
}
func printHookSeams(w io.Writer, rows []hookSeamRow) {
	fmt.Fprintln(w, "\nopt-in hook seams (config.toml):")
	for _, r := range rows {
		fmt.Fprintf(w, "  %-20s %s — %s\n    %s\n    cost: %s\n", r.ID, r.State, r.Reason, r.Command, r.Cost)
	}
}

func setHookSeam(root, scope, id string, on bool) (string, error) {
	if scope != "project" && scope != "user" {
		return "", fmt.Errorf("scope must be project or user")
	}
	if id == "readouts" && scope != "user" {
		return "", fmt.Errorf("readouts is user-only; use --scope user")
	}
	path := projectdir.Config.Path(root)
	if scope == "user" {
		path = config.DefaultUserPath()
	}
	if path == "" {
		return "", fmt.Errorf("cannot resolve config path")
	}
	b, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	// Refuse malformed input rather than overwrite the user's intended settings.
	var raw map[string]any
	if err := toml.Unmarshal(b, &raw); err != nil {
		return "", err
	}
	table, key, value := "hooks", id, fmt.Sprint(on)
	if id == "readouts" {
		table, key = "readouts", "mode"
		value = `"off"`
		if on {
			value = `"rich"`
		}
	}
	lines := strings.Split(string(b), "\n")
	section := ""
	insert := -1
	found := false
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "[") {
			if section == table && insert < 0 {
				insert = i
			}
			section = strings.TrimSpace(strings.SplitN(t, "]", 2)[0][1:])
			if section == table {
				found = true
				insert = i + 1
			}
			continue
		}
		if section == table {
			k, _, ok := strings.Cut(t, "=")
			if ok && strings.TrimSpace(k) == key {
				comment := ""
				if j := strings.Index(line, "#"); j >= 0 {
					comment = " " + line[j:]
				}
				lines[i] = key + " = " + value + comment
				insert = -2
				break
			}
		}
	}
	if insert != -2 {
		entry := key + " = " + value
		if found {
			lines = append(lines[:insert], append([]string{entry}, lines[insert:]...)...)
		} else {
			lines = append(lines, "["+table+"]", entry)
		}
	}
	out := strings.Join(lines, "\n")
	// Dotted/inline forms cannot be safely rewritten by this narrow editor.
	if err := toml.Unmarshal([]byte(out), &raw); err != nil {
		return "", fmt.Errorf("cannot safely edit %s; set %s.%s manually: %w", path, table, key, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(out), 0600); err != nil {
		return "", err
	}
	return path, nil
}
