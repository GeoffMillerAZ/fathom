package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// recordingReader fails the test's "never read stdin" assertion when read.
type recordingReader struct{ reads int }

func (r *recordingReader) Read([]byte) (int, error) { r.reads++; return 0, os.ErrClosed }

// helpSandbox points HOME, FATHOM_HOME and cwd at an empty temp dir and the
// daemon URL at a closed port, returning the dir.
func helpSandbox(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("FATHOM_HOME", dir)
	t.Setenv("FATHOM_HTTP", "http://127.0.0.1:1")
	old, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	return dir
}

func dirEntries(t *testing.T, dir string) []string {
	t.Helper()
	var got []string
	_ = filepath.Walk(dir, func(p string, _ os.FileInfo, _ error) error {
		if p != dir {
			got = append(got, p)
		}
		return nil
	})
	return got
}

func TestEveryVerbHelpExitsZeroWithoutRunning(t *testing.T) {
	dir := helpSandbox(t)
	type tc struct {
		args []string
		name string
	}
	var cases []tc
	for _, v := range verbs() {
		for _, h := range []string{"-h", "--help", "-help", "help"} {
			cases = append(cases, tc{[]string{v.Name, h}, v.Name})
		}
		cases = append(cases, tc{[]string{"help", v.Name}, v.Name})
	}
	for _, c := range cases {
		in := &recordingReader{}
		var out, errb bytes.Buffer
		rc := runWithStdin(c.args, in, &out, &errb)
		if rc != 0 {
			t.Errorf("%v: rc %d, want 0", c.args, rc)
		}
		if !strings.Contains(out.String(), c.name) || out.Len() == 0 {
			t.Errorf("%v: stdout %q does not name the verb", c.args, out.String())
		}
		if errb.Len() != 0 {
			t.Errorf("%v: stderr %q, want empty", c.args, errb.String())
		}
		if in.reads != 0 {
			t.Errorf("%v: stdin read %d times", c.args, in.reads)
		}
	}
	if got := dirEntries(t, dir); len(got) != 0 {
		t.Errorf("help created files: %v", got)
	}
	for name := range verbSubverbs() {
		if _, ok := lookupVerb(name); !ok {
			t.Errorf("verbSubverbs names unregistered verb %q", name)
		}
	}
}

func TestHelpUnknownVerbFails(t *testing.T) {
	var out, errb bytes.Buffer
	if rc := runWithStdin([]string{"help", "nope"}, &recordingReader{}, &out, &errb); rc != 1 {
		t.Errorf("rc %d, want 1", rc)
	}
	if out.Len() != 0 || !strings.Contains(errb.String(), "Usage: fathom") {
		t.Errorf("stdout %q stderr %q", out.String(), errb.String())
	}
}

func TestRootHelpAndVersionExitZero(t *testing.T) {
	helpSandbox(t)
	var want bytes.Buffer
	cmdVersion(nil, &want, nil)
	for _, a := range []string{"--help", "-h", "-help"} {
		var out, errb bytes.Buffer
		if rc := runWithStdin([]string{a}, &recordingReader{}, &out, &errb); rc != 0 || errb.Len() != 0 || !strings.Contains(out.String(), "Subcommands:") {
			t.Errorf("%s: rc %d stdout %q stderr %q", a, rc, out.String(), errb.String())
		}
	}
	for _, a := range []string{"--version", "-version"} {
		var out, errb bytes.Buffer
		if rc := runWithStdin([]string{a}, &recordingReader{}, &out, &errb); rc != 0 || out.String() != want.String() || errb.Len() != 0 {
			t.Errorf("%s: rc %d stdout %q want %q", a, rc, out.String(), want.String())
		}
	}
	for _, args := range [][]string{nil, {"nope"}} {
		var out, errb bytes.Buffer
		if rc := runWithStdin(args, &recordingReader{}, &out, &errb); rc != 1 || out.Len() != 0 || !strings.Contains(errb.String(), "Usage: fathom") {
			t.Errorf("%v: rc %d stdout %q stderr %q", args, rc, out.String(), errb.String())
		}
	}
}

func TestSingleDashLongFlagsStillDispatch(t *testing.T) {
	helpSandbox(t)
	// Only a SOLE help token is intercepted: help words elsewhere, and
	// single-dash long flags, reach the verb, so its own parser answers.
	for _, args := range [][]string{
		{"version", "-json"},
		{"version", "-json", "-h"},
		{"verify", "-plan", "no-such-plan.md", "-help"},
	} {
		var out, errb bytes.Buffer
		runWithStdin(args, &recordingReader{}, &out, &errb)
		if strings.Contains(out.String(), "Run `fathom") {
			t.Errorf("%v was intercepted as help: %q", args, out.String())
		}
	}
	var out bytes.Buffer
	if rc := runWithStdin([]string{"version", "-json"}, &recordingReader{}, &out, &bytes.Buffer{}); rc != 0 || !strings.Contains(out.String(), `"version"`) {
		t.Errorf("version -json: rc %d out %q", rc, out.String())
	}
	if !isHelpToken("-help") || isHelpToken("-dry-run") || isHelpToken("-plan") {
		t.Error("isHelpToken misclassifies")
	}
}
