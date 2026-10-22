package gatereceipt

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckExactTreeAndCommand(t *testing.T) {
	o := fixture(t)
	_, id, err := Write(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Check(context.Background(), o, "HEAD")
	if err != nil || got != id {
		t.Fatalf("%s %v", got, err)
	}
	tree := gitTest(t, o.Root, "rev-parse", "HEAD^{tree}")
	if got, err := Check(context.Background(), o, tree); err != nil || got != id {
		t.Fatalf("explicit tree: %s %v", got, err)
	}
	o.Command = "test"
	if _, err := Check(context.Background(), o, "HEAD"); err == nil || !strings.Contains(err.Error(), "command identity differs") {
		t.Fatalf("command: %v", err)
	}
	o.Command = "validate"
	put(t, filepath.Join(o.Root, "lefthook.yml"), "changed\n")
	gitTest(t, o.Root, "add", ".")
	gitTest(t, o.Root, "-c", "core.hooksPath="+os.DevNull, "commit", "-qm", "change config")
	if _, err := Check(context.Background(), o, tree); err == nil || !strings.Contains(err.Error(), "command identity differs") {
		t.Fatalf("config: %v", err)
	}
	put(t, filepath.Join(o.Root, "another"), "new tree\n")
	gitTest(t, o.Root, "add", ".")
	gitTest(t, o.Root, "-c", "core.hooksPath="+os.DevNull, "commit", "-qm", "change tree")
	if _, err := Check(context.Background(), o, "HEAD"); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("tree: %v", err)
	}
}

func TestCheckRefusesTamperedEvidence(t *testing.T) {
	for _, kind := range []string{"dirty", "missing-log", "altered-log", "symlink-log", "hand-edited-exit", "failure-with-edited-exit", "no-success-line", "trailing-output"} {
		t.Run(kind, func(t *testing.T) {
			o := fixture(t)
			if kind == "failure-with-edited-exit" {
				put(t, o.Log, strings.Replace(passingLog, "PASS", "FAIL", 1))
			}
			r, id, err := Write(context.Background(), o)
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "dirty":
				put(t, filepath.Join(o.Root, "new"), "new")
			case "missing-log":
				if err := os.Remove(filepath.Join(o.Dir, id+".log")); err != nil {
					t.Fatal(err)
				}
			case "altered-log":
				put(t, filepath.Join(o.Dir, id+".log"), passingLog+"changed\n")
			case "hand-edited-exit":
				r.Exit = 1
				b, _ := json.MarshalIndent(r, "", "  ")
				put(t, filepath.Join(o.Dir, id+".json"), string(b)+"\n")
			case "symlink-log":
				if err := os.Remove(filepath.Join(o.Dir, id+".log")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(o.Log, filepath.Join(o.Dir, id+".log")); err != nil {
					t.Fatal(err)
				}
			case "failure-with-edited-exit", "no-success-line", "trailing-output":
				log, _ := os.ReadFile(filepath.Join(o.Dir, id+".log"))
				if kind == "no-success-line" {
					log = []byte("all good\n")
				}
				if kind == "trailing-output" {
					log = append(log, []byte("extra output\n")...)
				}
				r.Exit = 0
				r.LogSHA256 = digest(log)
				r.LogBytes = int64(len(log))
				b, _ := json.MarshalIndent(r, "", "  ")
				b = append(b, '\n')
				if err := os.Remove(filepath.Join(o.Dir, id+".json")); err != nil {
					t.Fatal(err)
				}
				id = digest(b)
				put(t, filepath.Join(o.Dir, id+".json"), string(b))
				put(t, filepath.Join(o.Dir, id+".log"), string(log))
			}
			if _, err := Check(context.Background(), o, "HEAD"); err == nil {
				t.Fatal("accepted tampered evidence")
			}
		})
	}
}

func TestCheckSkipsInvalidCandidateAndAcceptsGoodReceipt(t *testing.T) {
	o := fixture(t)
	put(t, filepath.Join(o.Dir, "000.json"), "bad json\n")
	_, id, err := Write(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Check(context.Background(), o, "HEAD"); err != nil || got != id {
		t.Fatalf("%s %v", got, err)
	}
}
