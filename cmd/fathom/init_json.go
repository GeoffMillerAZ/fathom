package main

// init_json.go — `fathom init --json` (RP-15, docs/plans/reach-parity.md):
// every write, the receipt id, and the manual steps as DATA. init narrates
// in a fixed vocabulary ("created X", "updated X (…)", "ok X (…)",
// "BLOCKED X — why", "MANUAL: …" — pinned by its tests), so the JSON is a
// faithful reading of that narration plus the receipt the wiring engine
// appended to .fathom/receipts.jsonl during the run. cmdInit itself is
// untouched beyond the one-line dispatch at its top.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/wiring"
)

type initWriteJSON struct {
	Path   string `json:"path"`
	Action string `json:"action"` // created | updated | ok | blocked | shim
	Detail string `json:"detail,omitempty"`
}

type initReceiptJSON struct {
	ID    string   `json:"id"`
	Time  string   `json:"time"`
	Files []string `json:"files"`
}

type initManualStepJSON struct {
	Text string `json:"text"`
	// Command is the runnable line when the step is one (an `mcp add`, a
	// `fathom …` verb, a lefthook snippet line); empty for prose.
	Command string `json:"command,omitempty"`
}

// initJSON is the `fathom init --json` envelope.
type initJSON struct {
	Service     string               `json:"service"`
	Root        string               `json:"root"`
	ExitCode    int                  `json:"exit_code"`
	Writes      []initWriteJSON      `json:"writes"`
	Blocked     []initWriteJSON      `json:"blocked"`
	Receipt     *initReceiptJSON     `json:"receipt"`
	ManualSteps []initManualStepJSON `json:"manual_steps"`
	Narration   []string             `json:"narration"`
	Stderr      []string             `json:"stderr,omitempty"`
}

var (
	initHeaderLine  = regexp.MustCompile(`^fathom init — service "([^"]*)", root (.+)$`)
	initWriteLine   = regexp.MustCompile(`^(created|updated|linked|ok) (\S+)(?: \((.*)\))?$`)
	initBlockedLine = regexp.MustCompile(`^BLOCKED (\S+) — (.*)$`)
	initManualLine  = regexp.MustCompile(`^MANUAL:\s?(.*)$`)
	initShimLine    = regexp.MustCompile(`^shim: (.*)$`)
)

// hasJSONFlag reports whether args spell --json / -json (init's flag set
// is bool-shaped for it, so no value follows).
func hasJSONFlag(args []string) bool {
	for _, a := range args {
		if a == "--json" || a == "-json" || a == "--json=true" || a == "-json=true" {
			return true
		}
	}
	return false
}

func stripJSONFlag(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if a == "--json" || a == "-json" || a == "--json=true" || a == "-json=true" {
			continue
		}
		out = append(out, a)
	}
	return out
}

// initRootFromArgs mirrors cmdInit's --path resolution far enough to find
// the receipts log before the run: --path X, --path=X, else cwd.
func initRootFromArgs(args []string) string {
	root := ""
	for i, a := range args {
		switch {
		case a == "--path" || a == "-path":
			if i+1 < len(args) {
				root = args[i+1]
			}
		case strings.HasPrefix(a, "--path="):
			root = strings.TrimPrefix(a, "--path=")
		case strings.HasPrefix(a, "-path="):
			root = strings.TrimPrefix(a, "-path=")
		}
	}
	if root == "" {
		root, _ = os.Getwd()
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return root
	}
	return abs
}

// renderInitJSON runs cmdInit with its narration captured and renders the
// envelope. Exit code is cmdInit's own.
func renderInitJSON(args []string, stdout, stderr io.Writer) int {
	root := initRootFromArgs(args)
	before, _ := wiring.LoadReceipts(root)

	var narration, errBuf bytes.Buffer
	code := cmdInit(stripJSONFlag(args), &narration, &errBuf)

	out := initJSON{
		Root:        root,
		ExitCode:    code,
		Writes:      []initWriteJSON{},
		Blocked:     []initWriteJSON{},
		ManualSteps: []initManualStepJSON{},
		Narration:   []string{},
	}
	for _, line := range strings.Split(strings.TrimRight(narration.String(), "\n"), "\n") {
		if line == "" {
			continue
		}
		out.Narration = append(out.Narration, line)
		switch {
		case initHeaderLine.MatchString(line):
			m := initHeaderLine.FindStringSubmatch(line)
			out.Service, out.Root = m[1], m[2]
		case initWriteLine.MatchString(line):
			m := initWriteLine.FindStringSubmatch(line)
			out.Writes = append(out.Writes, initWriteJSON{Path: m[2], Action: m[1], Detail: m[3]})
		case initBlockedLine.MatchString(line):
			m := initBlockedLine.FindStringSubmatch(line)
			out.Blocked = append(out.Blocked, initWriteJSON{Path: m[1], Action: "blocked", Detail: m[2]})
		case initManualLine.MatchString(line):
			m := initManualLine.FindStringSubmatch(line)
			out.ManualSteps = append(out.ManualSteps, manualStepOf(m[1]))
		case initShimLine.MatchString(line):
			m := initShimLine.FindStringSubmatch(line)
			out.Writes = append(out.Writes, initWriteJSON{Path: "shim", Action: "shim", Detail: m[1]})
		}
	}
	for _, line := range strings.Split(strings.TrimRight(errBuf.String(), "\n"), "\n") {
		if line != "" {
			out.Stderr = append(out.Stderr, line)
		}
	}
	// The receipt this run appended, if any: receipts load newest-first,
	// so a grown log's first entry is ours.
	if after, err := wiring.LoadReceipts(out.Root); err == nil && len(after) > len(before) {
		r := after[0]
		rc := initReceiptJSON{ID: r.ID, Time: r.Time, Files: []string{}}
		for _, f := range r.Files {
			rc.Files = append(rc.Files, f.Path)
		}
		out.Receipt = &rc
	}

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	// stderr still carries what cmdInit said there, so a failing run is
	// never silent on the human channel.
	_, _ = io.WriteString(stderr, errBuf.String())
	return code
}

// manualStepOf classifies one MANUAL line: an indented line that starts
// with a known runnable token is a command, everything else is prose.
func manualStepOf(text string) initManualStepJSON {
	step := initManualStepJSON{Text: text}
	trimmed := strings.TrimSpace(text)
	for _, prefix := range []string{"claude ", "codex ", "fathom ", "bin/fathom ", "launchctl ", "run: "} {
		if strings.HasPrefix(trimmed, prefix) {
			step.Command = strings.TrimPrefix(trimmed, "run: ")
			break
		}
	}
	return step
}
