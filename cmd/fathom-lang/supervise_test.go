package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/pkg/lang"
)

// The supervised child is this test binary, started again with childEnv set: TestMain runs the command as the child,
// with beforeRead replaced by injectFault, so a file named for a fault makes the child's read fault for real, under
// the parent's real supervision. Nothing here mocks the parent.
const childEnv = "FATHOM_LANG_TEST_CHILD"

func TestMain(m *testing.M) {
	if os.Getenv(childEnv) != "" {
		beforeRead = injectFault
		os.Exit(run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
	}
	childCommand = func(ctx context.Context, args []string) (*exec.Cmd, error) {
		cmd := exec.CommandContext(ctx, os.Args[0], args...)
		cmd.Env = append(os.Environ(), childEnv+"=1")
		return cmd, nil
	}
	os.Exit(m.Run())
}

var held [][]byte // what the allocation fault keeps alive

// injectFault makes the read of a file named for a fault fail the way a hostile file would: a real fatal stack
// overflow, a read that never returns, an allocation without end and a death by signal.
func injectFault(path string) {
	switch filepath.Base(path) {
	case "fault-stack.txt":
		debug.SetMaxStack(8 << 20)
		recurse(0)
	case "fault-hang.txt":
		for {
			time.Sleep(time.Second)
		}
	case "fault-alloc.txt":
		for {
			chunk := make([]byte, 64<<20)
			for i := 0; i < len(chunk); i += 4096 {
				chunk[i] = 1
			}
			held = append(held, chunk)
			time.Sleep(20 * time.Millisecond)
		}
	case "fault-kill.txt":
		p, _ := os.FindProcess(os.Getpid())
		p.Kill()
		time.Sleep(time.Minute)
	}
}

// recurse overflows the stack: a fatal error no recover catches.
func recurse(n int) int {
	var pad [512]byte
	pad[n%512] = byte(n)
	return recurse(n+1) + int(pad[0])
}

type batchFile struct {
	Path   string `json:"path"`
	Error  string `json:"error"`
	Result *struct {
		Dialect    string   `json:"dialect"`
		Incomplete bool     `json:"incomplete"`
		Notes      []string `json:"notes"`
		Symbols    []any    `json:"symbols"`
		Facts      []any    `json:"facts"`
	} `json:"result"`
}

func runBatch(t *testing.T, args ...string) (files []batchFile, code int, raw []byte, stderr string) {
	t.Helper()
	var out, errw bytes.Buffer
	code = run(context.Background(), append([]string{"extract"}, args...), strings.NewReader(""), &out, &errw)
	var doc struct {
		Files []batchFile `json:"files"`
	}
	if out.Len() > 0 {
		if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
			t.Fatalf("%v\n%s", err, out.String())
		}
	}
	return doc.Files, code, out.Bytes(), errw.String()
}

func faultRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "a.go", "package a\n\nfunc F() {}\n")
	write(t, root, "b.go", "package b\n\nfunc G() {}\n")
	for _, f := range []string{"fault-stack.txt", "fault-hang.txt", "fault-alloc.txt", "fault-kill.txt"} {
		write(t, root, f, "x\n")
	}
	return root
}

func shortTimings(t *testing.T) {
	t.Helper()
	prev := killMargin
	killMargin = 500 * time.Millisecond
	t.Cleanup(func() { killMargin = prev })
}

// A file whose read faults the child costs that file's result alone: it reads Incomplete with the reason, the files
// around it read as they would have, and the batch goes on in a new child.
func TestSupervisedFaultCostsOneFileAndTheBatchContinues(t *testing.T) {
	shortTimings(t)
	root := faultRoot(t)
	for _, c := range []struct {
		fault string
		args  []string
		want  string
	}{
		{"fault-stack.txt", nil, "fatal error: stack overflow"},
		{"fault-hang.txt", []string{"-deadline", "300ms"}, "passed the 300ms read deadline"},
		{"fault-alloc.txt", []string{"-max-memory", "400MiB"}, "passed its memory limit"},
		{"fault-kill.txt", nil, "signal: killed"},
	} {
		t.Run(c.fault, func(t *testing.T) {
			args := append([]string{"-root", root}, c.args...)
			files, code, _, stderr := runBatch(t, append(args, "a.go", c.fault, "b.go")...)
			if code != 0 || len(files) != 3 {
				t.Fatalf("exit %d, %d files: %s", code, len(files), stderr)
			}
			if files[0].Path != "a.go" || files[2].Path != "b.go" || files[1].Path != c.fault {
				t.Fatalf("results are not in input order: %+v", files)
			}
			for _, i := range []int{0, 2} {
				if r := files[i].Result; r == nil || r.Dialect != "go" || r.Incomplete || len(r.Symbols) != 1 {
					t.Errorf("%s did not read as it would have: %+v", files[i].Path, files[i])
				}
			}
			r := files[1].Result
			if r == nil || !r.Incomplete || files[1].Error != "" || len(r.Symbols) != 0 || len(r.Facts) != 0 || len(r.Notes) != 1 ||
				!strings.HasPrefix(r.Notes[0], killedNotePrefix) || !strings.Contains(r.Notes[0], c.want) {
				t.Fatalf("the killed file reads %+v, want Incomplete with one note naming %q", files[1], c.want)
			}
		})
	}
}

// Two files that fault in a row each cost only themselves, and the last read still comes from a running child.
func TestSupervisedRestartsAfterEveryFault(t *testing.T) {
	shortTimings(t)
	root := faultRoot(t)
	files, code, _, stderr := runBatch(t, "-root", root, "fault-kill.txt", "fault-stack.txt", "a.go")
	if code != 0 || len(files) != 3 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if !files[0].Result.Incomplete || !files[1].Result.Incomplete || files[2].Result.Incomplete || files[2].Result.Dialect != "go" {
		t.Fatalf("%+v %+v %+v", files[0], files[1], files[2])
	}
}

// A normal batch is byte-equal in both modes, whatever the order and whatever fails: a missing file and a file
// no dialect reads included.
func TestSupervisedOutputEqualsInProcess(t *testing.T) {
	root := t.TempDir()
	write(t, root, "infra/main.tf", "resource \"aws_s3_bucket\" \"b\" {\n  bucket = var.name\n}\nvariable \"name\" {}\n")
	write(t, root, "a.go", "package a\n\nfunc F() {}\n")
	write(t, root, "blob.bin", "\x00\x01\x02binary")
	write(t, root, "misc.yaml", "x: 1\n")
	write(t, root, "note <&>.go", "package n\n")
	paths := []string{"infra/main.tf", "a.go", "missing.go", "blob.bin", "misc.yaml", "note <&>.go"}
	for _, extra := range [][]string{nil, {"-walk"}, {"-deadline", "20s"}} {
		_, sc, supervised, serr := runBatch(t, append(append([]string{"-root", root}, extra...), paths...)...)
		_, ic, inProc, ierr := runBatch(t, append(append([]string{"-root", root, "-in-process"}, extra...), paths...)...)
		if sc != ic || sc != 1 {
			t.Fatalf("exit %d and %d, want 1 for the missing file in both (%s%s)", sc, ic, serr, ierr)
		}
		if !bytes.Equal(supervised, inProc) {
			t.Fatalf("the two modes differ (%v)\n--- supervised\n%s\n--- in process\n%s", extra, supervised, inProc)
		}
	}
}

// The sibling list reaches the child: a role's task file classifies by the paths beside it, so a read that lost the
// list would differ from the in-process read that has it.
func TestSupervisedPassesTheSiblingList(t *testing.T) {
	root := t.TempDir()
	write(t, root, "roles/x/tasks/main.yml", "- name: a\n  debug:\n    msg: hi\n")
	siblings := filepath.Join(root, "siblings.txt")
	if err := os.WriteFile(siblings, []byte("roles/x/meta/main.yml\nroles/x/tasks/main.yml\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	const path = "roles/x/tasks/main.yml"
	_, _, in, _ := runBatch(t, "-root", root, "-in-process", "-siblings", siblings, path)
	_, _, sup, _ := runBatch(t, "-root", root, "-siblings", siblings, path)
	_, _, none, _ := runBatch(t, "-root", root, path)
	if !bytes.Equal(in, sup) {
		t.Fatalf("the sibling list did not reach the child\n--- in process\n%s\n--- supervised\n%s", in, sup)
	}
	if bytes.Equal(sup, none) {
		t.Fatal("the sibling list changes nothing in this fixture, so the test proves nothing")
	}
}

func TestDeadlineAndMaxMemoryFlagsAreChecked(t *testing.T) {
	for _, args := range [][]string{
		{"-deadline", "31s", "a.go"}, {"-deadline", "0s", "a.go"}, {"-deadline", "-1s", "a.go"},
		{"-max-memory", "lots", "a.go"}, {"-max-memory", "0", "a.go"},
	} {
		var out, errw bytes.Buffer
		if code := run(context.Background(), append([]string{"extract"}, args...), strings.NewReader(""), &out, &errw); code != 2 {
			t.Errorf("%v: exit %d, want 2: %s", args, code, errw.String())
		}
	}
}

func TestParseSize(t *testing.T) {
	for in, want := range map[string]uint64{"4GiB": 4 << 30, "512MiB": 512 << 20, "512m": 512 << 20, "64K": 64 << 10, "1000": 1000, " 2 GB ": 2 << 30} {
		if got, err := parseSize(in); err != nil || got != want {
			t.Errorf("parseSize(%q) = %d, %v, want %d", in, got, err, want)
		}
	}
	for _, in := range []string{"", "0", "-1", "GiB", "1.5G", "99999999999999999999"} {
		if _, err := parseSize(in); err == nil {
			t.Errorf("parseSize(%q) was accepted", in)
		}
	}
}

// A child that answered by abandoning a parser still has that parser running, so the parent ends it.
func TestAbandonedAnswersEndTheChild(t *testing.T) {
	deadline := fileResult{Path: "a", Result: &lang.Result{Incomplete: true, Notes: []string{"read-deadline: the read passed its 30s deadline and was abandoned, so none of its facts are returned"}}}
	busy := fileResult{Path: "a", Result: &lang.Result{Skipped: lang.SkipBusy}}
	for name, fr := range map[string]fileResult{"deadline": deadline, "busy": busy} {
		if !abandoned(fr) {
			t.Errorf("%s: the child is not ended", name)
		}
	}
	for name, fr := range map[string]fileResult{
		"read":       {Path: "a", Result: &lang.Result{Dialect: "go"}},
		"incomplete": {Path: "a", Result: &lang.Result{Incomplete: true, Notes: []string{"incomplete: a decode failure"}}},
		"error":      {Path: "a", Error: "no such file"},
	} {
		if abandoned(fr) {
			t.Errorf("%s: a child that abandoned nothing is ended", name)
		}
	}
}
