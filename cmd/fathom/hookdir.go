package main

// hookdir.go — the directory a tool call acts in (O1230). A hook's input
// cwd is the session's working directory, and inside a workflow or Agent
// subagent that works in a scratch clone it is the parent session's
// project, never the clone: the subagent reaches the clone with a leading
// `cd <clone> && ...` or with an absolute file path. The readout hook's
// roots and joins, the gate preflight and the SubagentStop claim check read
// the directory the call itself acts in through hookEffectiveDir; the
// command guards keep the session's project (ADR-038's scoping).

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/shellcmd"
)

// hookEffectiveDir is the directory one tool call acts in. A file tool's
// call (filePath set) acts in the file's directory; a shell command acts in
// the target of its first `cd <dir>` or `pushd <dir>` when that is the
// first segment after RO-28's quiet prefixes (replayQuiet over readout.IsQuiet:
// cd, export, mkdir -p, sleep N, an assignment alone, set, source, ., tccutil
// reset);
// anything else acts in cwd. A relative target resolves against cwd
// through resolveBashDir. An unreadable command, a target the shell would
// compute (a variable, a command substitution), a cd the parser cannot
// place, or a target that is not a directory reads cwd, never a guess.
// resolved is false whenever dir is cwd's fallback.
func hookEffectiveDir(command, filePath, cwd string) (dir string, resolved bool) {
	var target string
	switch {
	case filePath != "":
		target = filepath.Dir(resolveBashDir(cwd, filePath))
	case command != "":
		t, ok := leadingCdTarget(command)
		if !ok {
			return cwd, false
		}
		target = resolveBashDir(cwd, t)
	default:
		return cwd, false
	}
	if !filepath.IsAbs(target) {
		return cwd, false // a relative path with no cwd to anchor it
	}
	if st, err := os.Stat(target); err != nil || !st.IsDir() {
		return cwd, false
	}
	return filepath.Clean(target), true
}

// hookGitDir is the directory a `git -C <at>` in command acts in: at as
// written when absolute, else against the directory the command's leading
// cd reached (hookEffectiveDir from cwd), the way the shell runs it, never
// against the work tree's root.
func hookGitDir(command, at, cwd string) string {
	base, _ := hookEffectiveDir(command, "", cwd)
	return resolveBashDir(base, at)
}

// leadingCdTarget is the literal operand of command's leading cd or pushd
// (bash-path text), ok false when the command has none the lexer can read.
func leadingCdTarget(command string) (string, bool) {
	c := shellcmd.ParseCommand(command)
	if c.Unreadable {
		return "", false
	}
	for i := range c.Pipelines {
		p := &c.Pipelines[i]
		if t, isCd, ok := cdSegmentTarget(p); isCd {
			return t, ok
		}
		if !replayQuiet(p, nil) {
			return "", false
		}
	}
	return "", false
}

// cdSegmentTarget reads one segment as `cd [-L|-P|-e|-@] [--] <dir>` or
// `pushd [-n] <dir>`: isCd reports the segment is one, ok that its operand
// is a literal directory the shell would change to.
func cdSegmentTarget(p *shellcmd.Pipeline) (target string, isCd, ok bool) {
	if len(p.Units) != 1 {
		return "", false, false
	}
	u := &p.Units[0]
	if u.Kind != shellcmd.KindSimple || len(u.Wrappers) > 0 || len(u.Words) == 0 {
		return "", false, false
	}
	name := u.Name()
	if name != "cd" && name != "pushd" {
		return "", false, false
	}
	if p.Bang || p.Sep == "&" {
		return "", true, false // a negated or backgrounded cd moves no later segment
	}
	var operands []shellcmd.Word
	opts := true
	for _, w := range u.Words[1:] {
		switch {
		case opts && w.Text == "--" && !w.Quoted:
			opts = false
		case opts && strings.HasPrefix(w.Text, "-") && !w.Quoted && len(w.Text) > 1:
			// -L, -P, -e, -@ for cd; -n for pushd: none moves the target
		default:
			operands = append(operands, w)
		}
	}
	if len(operands) != 1 {
		return "", true, false // `cd` alone, `cd -`, or a pushd stack rotation
	}
	w := operands[0]
	if !w.Literal() || w.Text == "" || w.Text == "-" || strings.HasPrefix(w.Text, "+") || (w.Quoted && strings.HasPrefix(w.Text, "~")) {
		return "", true, false
	}
	return w.Text, true, true
}

// hookCallRoot is the work tree a tool call acts in: the root of
// hookEffectiveDir's directory when the call resolves one inside another
// work tree than fallback's, else fallback itself (a cd inside the project
// keeps the project's path as the caller had it). A relative cd resolves
// against cwd, else fallback.
func hookCallRoot(command, filePath, cwd, fallback string) string {
	base := cwd
	if base == "" {
		base = fallback
	}
	dir, ok := hookEffectiveDir(command, filePath, base)
	if !ok {
		return fallback
	}
	root := gitToplevelByStat(dir)
	if root == "" || root == gitToplevelByStat(fallback) {
		return fallback
	}
	return root
}

// agentTranscriptTail bounds how much of an agent's transcript the
// SubagentStop reads back from its end for the agent's last tool call: the
// claim check runs inside a 1 s budget. A call older than the window, or
// one whose line is longer than it, is not found.
const agentTranscriptTail = 1 << 20

// transcriptToolLine is the part of one transcript line the last-call
// reader decodes: an assistant message's tool calls and the line's cwd.
type transcriptToolLine struct {
	Type    string `json:"type"`
	Cwd     string `json:"cwd"`
	Message *struct {
		Content []struct {
			Type  string `json:"type"`
			Name  string `json:"name"`
			Input struct {
				Command  string `json:"command"`
				FilePath string `json:"file_path"`
			} `json:"input"`
		} `json:"content"`
	} `json:"message"`
}

// lastToolCallDir is the directory the last Bash, Read, Edit, Write or
// MultiEdit call in the transcript at path acted in (hookEffectiveDir from
// the line's own cwd, else cwd). ok is false when the file cannot be read
// or its tail holds no such call.
func lastToolCallDir(path, cwd string) (string, bool) {
	if path == "" {
		return "", false
	}
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() {
		return "", false
	}
	off := max(st.Size()-agentTranscriptTail, 0)
	buf := make([]byte, st.Size()-off)
	if _, err := f.ReadAt(buf, off); err != nil && !errors.Is(err, io.EOF) {
		return "", false
	}
	lines := bytes.Split(buf, []byte("\n"))
	if off > 0 {
		lines = lines[1:] // the window's first line is a partial one
	}
	for i := len(lines) - 1; i >= 0; i-- {
		if !bytes.Contains(lines[i], []byte(`"tool_use"`)) {
			continue
		}
		var tl transcriptToolLine
		if json.Unmarshal(lines[i], &tl) != nil || tl.Type != "assistant" || tl.Message == nil {
			continue
		}
		base := tl.Cwd
		if base == "" {
			base = cwd
		}
		for j := len(tl.Message.Content) - 1; j >= 0; j-- {
			c := tl.Message.Content[j]
			if c.Type != "tool_use" {
				continue
			}
			switch c.Name {
			case "Bash":
				dir, _ := hookEffectiveDir(c.Input.Command, "", base)
				return dir, dir != ""
			case "Read", "Edit", "Write", "MultiEdit":
				dir, _ := hookEffectiveDir("", c.Input.FilePath, base)
				return dir, dir != ""
			}
		}
	}
	return "", false
}
