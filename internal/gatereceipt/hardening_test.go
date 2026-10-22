package gatereceipt

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteRefusesLogFromAnotherTree(t *testing.T) {
	o := fixture(t)
	put(t, filepath.Join(o.Root, "new"), "tree B\n")
	gitTest(t, o.Root, "add", ".")
	gitTest(t, o.Root, "-c", "core.hooksPath="+os.DevNull, "commit", "-qm", "tree B")
	if _, _, err := Write(context.Background(), o); err == nil {
		t.Fatal("accepted tree A log for tree B")
	}
}

func TestWriteBindsBothLogEndpoints(t *testing.T) {
	for _, kind := range []string{"start-tree", "end-tree", "start-dirty", "end-dirty", "missing-start"} {
		t.Run(kind, func(t *testing.T) {
			o := fixture(t)
			log, err := os.ReadFile(o.Log)
			if err != nil {
				t.Fatal(err)
			}
			s := string(log)
			switch kind {
			case "start-tree":
				s = strings.Replace(s, "tree=", "tree=bad", 1)
			case "end-tree":
				i := strings.LastIndex(s, "tree=")
				if i >= 0 {
					s = s[:i] + strings.Replace(s[i:], "tree=", "tree=bad", 1)
				}
			case "start-dirty":
				s = strings.Replace(s, "clean=1", "clean=0", 1)
			case "end-dirty":
				i := strings.LastIndex(s, "clean=1")
				if i >= 0 {
					s = s[:i] + strings.Replace(s[i:], "clean=1", "clean=0", 1)
				}
			case "missing-start":
				s = strings.Join(strings.Split(s, "\n")[1:], "\n")
			}
			put(t, o.Log, s)
			if _, _, err := Write(context.Background(), o); err == nil {
				t.Fatalf("accepted %s log", kind)
			}
		})
	}
}

func TestWriteUsesLogToolchain(t *testing.T) {
	o := fixture(t)
	o.GoVersion = "writer toolchain"
	o.OS = "writer-os"
	o.Arch = "writer-arch"
	r, _, err := Write(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	if r.GoVersion != "go version fixture fixture/fixture" || r.OS != "fixture-os" || r.Arch != "fixture-arch" {
		t.Fatalf("stamped writer instead of log: %+v", r)
	}
}

func TestCheckRequiresCompatibleToolchain(t *testing.T) {
	for _, kind := range []string{"version", "os", "arch", "workspace"} {
		t.Run(kind, func(t *testing.T) {
			o := fixture(t)
			if kind == "workspace" {
				b, _ := os.ReadFile(o.Log)
				put(t, o.Log, strings.Replace(string(b), "\noff\n", "\nworkspace.work\n", 1))
			}
			if _, _, err := Write(context.Background(), o); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "version":
				o.GoVersion = "another version"
			case "os":
				o.OS = "another-os"
			case "arch":
				o.Arch = "another-arch"
			}
			if _, err := Check(context.Background(), o, "HEAD"); err == nil {
				t.Fatalf("accepted incompatible %s", kind)
			}
		})
	}
}

func TestCheckRevalidatesLogIdentity(t *testing.T) {
	for _, kind := range []string{"tree", "dirty", "toolchain"} {
		t.Run(kind, func(t *testing.T) {
			o := fixture(t)
			r, id, err := Write(context.Background(), o)
			if err != nil {
				t.Fatal(err)
			}
			b, _ := os.ReadFile(filepath.Join(o.Dir, id+".log"))
			s := string(b)
			switch kind {
			case "tree":
				s = strings.ReplaceAll(s, "tree=", "tree=bad")
			case "dirty":
				s = strings.ReplaceAll(s, "clean=1", "clean=0")
			case "toolchain":
				s = strings.Replace(s, "go version fixture", "go version another", 1)
			}
			r.LogSHA256 = digest([]byte(s))
			r.LogBytes = int64(len(s))
			b, _ = json.MarshalIndent(r, "", "  ")
			b = append(b, '\n')
			id = digest(b)
			put(t, filepath.Join(o.Dir, id+".json"), string(b))
			put(t, filepath.Join(o.Dir, id+".log"), s)
			// Keep only the mutated, internally re-digested candidate.
			entries, _ := os.ReadDir(o.Dir)
			for _, e := range entries {
				if e.Name() != id+".json" && e.Name() != id+".log" {
					if err := os.Remove(filepath.Join(o.Dir, e.Name())); err != nil {
						t.Fatal(err)
					}
				}
			}
			if _, err := Check(context.Background(), o, "HEAD"); err == nil {
				t.Fatalf("accepted changed log %s", kind)
			}
		})
	}
}

func TestCheckUsesRequestedTreeGateDefinitions(t *testing.T) {
	o := fixture(t)
	_, id, err := Write(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	tree := gitTest(t, o.Root, "rev-parse", "HEAD^{tree}")
	put(t, filepath.Join(o.Root, "Taskfile.yml"), "new gate definitions\n")
	gitTest(t, o.Root, "add", ".")
	gitTest(t, o.Root, "-c", "core.hooksPath="+os.DevNull, "commit", "-qm", "new definitions")
	if got, err := Check(context.Background(), o, tree); err != nil || got != id {
		t.Fatalf("historical tree: %s %v", got, err)
	}
}
