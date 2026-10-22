// Package gatereceipt records and checks local gate evidence. It performs
// no transport, signing, gate execution, or store writes.
package gatereceipt

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Options contains explicit source identity and operator-supplied metadata.
// Dir and Log may live outside Root; neither is inferred from a hostname.
type Options struct {
	Root, Dir, Log, Command                       string
	GoVersion, FathomVersion, OS, Arch, HostLabel string
}

// Receipt's declaration order is the stable JSON key order. Its id is the
// SHA-256 of those exact JSON bytes, including the final newline.
type Receipt struct {
	Version         int    `json:"version"`
	Tree            string `json:"tree"`
	Commit          string `json:"commit"`
	Dirty           bool   `json:"dirty"`
	Command         string `json:"command"`
	CommandIdentity string `json:"command_identity"`
	GoVersion       string `json:"go_version"`
	FathomVersion   string `json:"fathom_version"`
	OS              string `json:"os"`
	Arch            string `json:"arch"`
	GoWork          string `json:"go_work"`
	CGOEnabled      string `json:"cgo_enabled"`
	GoFlags         string `json:"go_flags"`
	HostLabel       string `json:"host_label"`
	Exit            int    `json:"exit"`
	LogSHA256       string `json:"log_sha256"`
	LogBytes        int64  `json:"log_bytes"`
	Start           string `json:"start"`
	End             string `json:"end"`
	TimeSource      string `json:"time_source"`
}

func digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

func git(ctx context.Context, root string, args ...string) (string, error) {
	c := exec.CommandContext(ctx, "git", args...)
	c.Dir = root
	b, err := c.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(b)), nil
}

// clean counts status records, including untracked files and submodule changes.
func clean(ctx context.Context, root string) error {
	c := exec.CommandContext(ctx, "git", "status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignore-submodules=none")
	c.Dir = root
	b, err := c.Output()
	if err != nil {
		return fmt.Errorf("git status: %w", err)
	}
	paths := bytes.Split(b, []byte{0})
	n := 0
	for i := 0; i < len(paths); i++ {
		p := paths[i]
		if len(p) < 3 {
			continue
		}
		n++
		if p[0] == 'R' || p[0] == 'C' || p[1] == 'R' || p[1] == 'C' {
			i++
		}
	}
	if n != 0 {
		return fmt.Errorf("dirty (%d path(s)); receipts require a clean working tree", n)
	}
	return nil
}

func identity(ctx context.Context, o Options, tree string) (string, error) {
	// Read exact blob bytes from the requested tree, never the writer's checkout.
	read := func(name string) ([]byte, error) {
		c := exec.CommandContext(ctx, "git", "show", tree+":"+name)
		c.Dir = o.Root
		b, err := c.Output()
		if err != nil {
			return nil, fmt.Errorf("gate definition %s: %w", name, err)
		}
		return b, nil
	}
	task, err := read("Taskfile.yml")
	if err != nil {
		return "", err
	}
	hook, err := read("lefthook.yml")
	if err != nil {
		return "", err
	}
	b, _ := json.Marshal([]string{o.Command, digest(task), digest(hook)})
	return digest(b), nil
}

var terminalLine = regexp.MustCompile(`^fathom gate: ([A-Za-z0-9_.:-]+) (PASS|FAIL)(?: exit=([0-9]+))?(?: ([^ ]+))?(?: tree=[0-9a-f]+ clean=[01] commit=[0-9a-f]+)?$`)
var taskFailureLine = regexp.MustCompile(`^task: Failed to run task "([^"]+)": exit status ([1-9][0-9]*)$`)

func logResult(b []byte, command string, mtime time.Time) (exit int, start, end, source string, err error) {
	lines := strings.Split(strings.TrimRight(string(b), "\r\n"), "\n")
	last := terminalLine.FindStringSubmatch(strings.TrimSuffix(lines[len(lines)-1], "\r"))
	// Task prints its own failure trailer after the wrapper returns. It can
	// record a failure, but can never supply the gate's success witness.
	if last == nil {
		if failed := taskFailureLine.FindStringSubmatch(strings.TrimSuffix(lines[len(lines)-1], "\r")); failed != nil {
			last = []string{failed[0], failed[1], "FAIL", failed[2], ""}
		}
	}
	if last == nil || last[1] != command {
		return 0, "", "", "", fmt.Errorf("log has no final success or failure line for %s", command)
	}
	exit = 1
	if last[2] == "PASS" {
		exit = 0
	}
	if last[3] != "" {
		code, e := strconv.Atoi(last[3])
		if e != nil || (last[2] == "PASS" && code != 0) || (last[2] == "FAIL" && code == 0) {
			err = fmt.Errorf("invalid gate exit status")
			return
		}
		exit = code
	}
	start = mtime.UTC().Format(time.RFC3339Nano)
	end = start
	source = "file-mtime"
	if last[4] != "" {
		finish, e := time.Parse(time.RFC3339Nano, last[4])
		if e != nil {
			err = fmt.Errorf("invalid gate end time: %w", e)
			return
		}
		prefix := "fathom gate: " + command + " START "
		for _, line := range lines[:len(lines)-1] {
			if strings.HasPrefix(line, prefix) {
				begin, e := time.Parse(time.RFC3339Nano, strings.Fields(strings.TrimPrefix(line, prefix))[0])
				if e != nil || begin.After(finish) {
					err = fmt.Errorf("invalid gate start time")
					return
				}
				start = begin.UTC().Format(time.RFC3339Nano)
				end = finish.UTC().Format(time.RFC3339Nano)
				source = "log"
				break
			}
		}
	}
	return
}

// Write refuses dirty trees and unfinished logs, copies the log beside the
// receipt, and publishes the receipt last. It never reads the machine hostname.
func Write(ctx context.Context, o Options) (Receipt, string, error) {
	var r Receipt
	if err := clean(ctx, o.Root); err != nil {
		return r, "", err
	}
	if o.Command == "" {
		o.Command = "validate"
	}
	tree, err := git(ctx, o.Root, "rev-parse", "--verify", "HEAD^{tree}")
	if err != nil {
		return r, "", err
	}
	commit, err := git(ctx, o.Root, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return r, "", err
	}
	ident, err := identity(ctx, o, tree)
	if err != nil {
		return r, "", err
	}
	f, err := os.Open(o.Log)
	if err != nil {
		return r, "", err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return r, "", err
	}
	if !info.Mode().IsRegular() {
		f.Close()
		return r, "", fmt.Errorf("gate log must be a regular file")
	}
	var buf bytes.Buffer
	_, err = buf.ReadFrom(f)
	closeErr := f.Close()
	if err != nil {
		return r, "", err
	}
	if closeErr != nil {
		return r, "", closeErr
	}
	b := buf.Bytes()
	stamp, err := logIdentity(b, o.Command, tree)
	if err != nil {
		return r, "", err
	}
	if stamp != commit {
		return r, "", fmt.Errorf("log commit differs from HEAD")
	}
	env, err := logEnvironment(b)
	if err != nil {
		return r, "", err
	}
	exit, start, end, source, err := logResult(b, o.Command, info.ModTime())
	if err != nil {
		return r, "", err
	}
	host := o.HostLabel
	if host == "" {
		host = "unnamed"
	}
	r = Receipt{Version: 1, Tree: tree, Commit: commit, Command: o.Command, CommandIdentity: ident, GoVersion: env.Version, FathomVersion: o.FathomVersion, OS: env.OS, Arch: env.Arch, GoWork: env.Work, CGOEnabled: env.CGO, GoFlags: env.Flags, HostLabel: host, Exit: exit, LogSHA256: digest(b), LogBytes: int64(len(b)), Start: start, End: end, TimeSource: source}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return r, "", err
	}
	data = append(data, '\n')
	id := digest(data)
	// Recheck before publishing so a change while reading the log cannot produce evidence.
	if err := clean(ctx, o.Root); err != nil {
		return r, "", err
	}
	head, err := git(ctx, o.Root, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return r, "", err
	}
	if head != commit {
		return r, "", fmt.Errorf("HEAD changed while recording gate evidence")
	}
	if o.Dir == "" {
		return r, "", fmt.Errorf("receipts directory is empty")
	}
	if err := os.MkdirAll(o.Dir, 0700); err != nil {
		return r, "", err
	}
	if err := publish(o.Dir, id+".log", b); err != nil {
		return r, "", err
	}
	if err := publish(o.Dir, id+".json", data); err != nil {
		return r, "", err
	}
	return r, id, nil
}

func publish(dir, name string, b []byte) error {
	f, err := os.CreateTemp(dir, ".receipt-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(dir, name))
}
