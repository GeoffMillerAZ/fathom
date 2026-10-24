package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
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

// helpFlagLine matches a flag's entry the way flag.PrintDefaults (and the
// static lists) print it: "  -name" then a space, a tab or the line end.
func helpFlagLine(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^  -` + regexp.QuoteMeta(name) + `(?:[ \t]|$)`)
}

// TestEveryVerbHelpExitsZeroWithoutRunning (CL-2, CL-5, CL-8): every verb
// and every subverb answers each help spelling, and `fathom help <verb>
// [<sub>]`, on stdout with exit 0: the header, every flag its handler
// defines (read from source by the parse-first proof), and for a parent its
// subverbs — without reading stdin, writing a file, or reaching a daemon.
func TestEveryVerbHelpExitsZeroWithoutRunning(t *testing.T) {
	ix := loadHelpIndex(t)
	var hits atomic.Int64
	daemon := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusTeapot)
	}))
	defer daemon.Close()
	dir := helpSandbox(t)
	t.Setenv("FATHOM_HTTP", daemon.URL)
	subs := verbSubverbs()
	runs := 0
	for _, k := range helpKeys() {
		p := ix.proveHelp(k.verb, k.sub)
		base := []string{k.verb}
		if k.sub != "" {
			base = append(base, strings.Fields(k.sub)...)
		}
		var forms [][]string
		for _, h := range []string{"-h", "--help", "-help", "help"} {
			forms = append(forms, append(append([]string(nil), base...), h))
		}
		forms = append(forms, append([]string{"help"}, base...))
		for _, args := range forms {
			runs++
			in := &recordingReader{}
			var out, errb bytes.Buffer
			rc := runWithStdin(args, in, &out, &errb)
			if rc != 0 {
				t.Errorf("%v: rc %d, want 0", args, rc)
			}
			if !strings.Contains(out.String(), strings.Join(base, " ")) {
				t.Errorf("%v: stdout %q does not name %q", args, out.String(), strings.Join(base, " "))
			}
			if errb.Len() != 0 {
				t.Errorf("%v: stderr %q, want empty", args, errb.String())
			}
			if in.reads != 0 {
				t.Errorf("%v: stdin read %d times", args, in.reads)
			}
			if k.verb == "help" {
				continue // help's own help is the verb list
			}
			for _, f := range p.Flags {
				if !helpFlagLine(f.Name).MatchString(out.String()) {
					t.Errorf("%v: stdout does not list -%s (%s help):\n%s", args, f.Name, p.Kind, out.String())
				}
			}
			children := subs[k.verb]
			if k.sub != "" {
				children = verbNestedSubverbs()[k.verb+" "+k.sub]
			}
			for _, s := range children {
				if !strings.Contains(out.String(), s) {
					t.Errorf("%v: stdout does not list subverb %q", args, s)
				}
			}
		}
	}
	if got := dirEntries(t, dir); len(got) != 0 {
		t.Errorf("help created files: %v", got)
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("help reached the daemon %d times; no verb body may run", n)
	}
	if runs < 800 {
		t.Errorf("only %d help invocations; the table lost its verbs", runs)
	}
	for name := range verbSubverbs() {
		if _, ok := lookupVerb(name); !ok {
			t.Errorf("verbSubverbs names unregistered verb %q", name)
		}
	}
}

// TestHelpWordAfterUnknownSubverbDispatches: only a listed subverb makes a
// trailing help token a help request; anything else reaches the verb.
func TestHelpWordAfterUnknownSubverbDispatches(t *testing.T) {
	helpSandbox(t)
	var out, errb bytes.Buffer
	if rc := runWithStdin([]string{"map", "nope", "--help"}, &recordingReader{}, &out, &errb); rc == 0 || !strings.Contains(errb.String(), "unknown map verb") {
		t.Errorf("map nope --help: rc %d stdout %q stderr %q", rc, out.String(), errb.String())
	}
	out.Reset()
	errb.Reset()
	if rc := runWithStdin([]string{"help", "map", "nope"}, &recordingReader{}, &out, &errb); rc != 1 || out.Len() != 0 {
		t.Errorf("help map nope: rc %d stdout %q stderr %q", rc, out.String(), errb.String())
	}
	// `fathom help <verb> --help` is the verb's help, as before.
	out.Reset()
	errb.Reset()
	if rc := runWithStdin([]string{"help", "map", "--help"}, &recordingReader{}, &out, &errb); rc != 0 || !strings.Contains(out.String(), "Subcommands: new") || errb.Len() != 0 {
		t.Errorf("help map --help: rc %d stdout %q stderr %q", rc, out.String(), errb.String())
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

func TestVerbHelpNestedCommands(t *testing.T) {
	dir := helpSandbox(t)
	for _, base := range [][]string{{"plans", "weight"}, {"plans", "weight", "accept"}} {
		forms := [][]string{append([]string{"help"}, base...)}
		for _, h := range []string{"-h", "--help", "-help", "help"} {
			forms = append(forms, append(append([]string(nil), base...), h))
		}
		for _, args := range forms {
			var out, errs bytes.Buffer
			in := &recordingReader{}
			if code := runWithStdin(args, in, &out, &errs); code != 0 || errs.Len() != 0 || in.reads != 0 || !strings.Contains(out.String(), strings.Join(base, " ")) {
				t.Errorf("%v: exit %d: %s %s; stdin reads %d", args, code, &out, &errs, in.reads)
			}
			if len(base) == 3 {
				for _, flag := range []string{"plan", "store", "service", "dry-run"} {
					if !helpFlagLine(flag).MatchString(out.String()) {
						t.Errorf("%v omits -%s: %s", args, flag, &out)
					}
				}
			} else if !strings.Contains(out.String(), "Subcommands: accept") {
				t.Errorf("%v omits nested subcommands: %s", args, &out)
			}
		}
	}
	for _, args := range [][]string{{"help", "plans", "weight", "bogus"}, {"plans", "weight", "bogus", "--help"}} {
		var out, errs bytes.Buffer
		if code := runWithStdin(args, &recordingReader{}, &out, &errs); code == 0 || out.Len() != 0 {
			t.Errorf("unknown nested command %v: exit %d: %s %s", args, code, &out, &errs)
		}
	}
	if got := dirEntries(t, dir); len(got) != 0 {
		t.Errorf("nested help created files: %v", got)
	}
}
