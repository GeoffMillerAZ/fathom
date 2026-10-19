package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var localZoneNames = []string{"owned.json", "leases.jsonl", "scip/", "receipts.jsonl", "memories.jsonl"}

func runInitFor(t *testing.T, dir string) string {
	t.Helper()
	svc := strings.ToLower(filepath.Base(dir))
	srv := fakeInitDaemon(t, svc, 123)
	defer srv.Close()
	var out, errBuf bytes.Buffer
	if code := cmdInit([]string{"--path", dir, "--url", srv.URL, "--bin", testBinPath}, &out, &errBuf); code != 0 {
		t.Fatalf("init exit = %d; stdout=%s stderr=%s", code, out.String(), errBuf.String())
	}
	return out.String()
}

func readZoneIgnore(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, ".fathom", ".gitignore"))
	if err != nil {
		t.Fatalf("reading .fathom/.gitignore: %v", err)
	}
	return string(b)
}

func TestInitWritesLocalZoneIgnore(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	t.Run("fresh repo", func(t *testing.T) {
		dir := t.TempDir()
		runInitFor(t, dir)
		got := readZoneIgnore(t, dir)
		for _, n := range localZoneNames {
			if !strings.Contains("\n"+got, "\n"+n+"\n") {
				t.Errorf(".fathom/.gitignore missing line %q:\n%s", n, got)
			}
		}
	})

	t.Run("hand-written file kept, re-run idempotent", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, ".fathom"), 0o755); err != nil {
			t.Fatal(err)
		}
		hand := "# mine\nscratch/\n"
		p := filepath.Join(dir, ".fathom", ".gitignore")
		if err := os.WriteFile(p, []byte(hand), 0o644); err != nil {
			t.Fatal(err)
		}
		runInitFor(t, dir)
		first := readZoneIgnore(t, dir)
		if !strings.HasPrefix(first, hand) {
			t.Fatalf("hand-written lines not preserved at the top:\n%s", first)
		}
		if !strings.Contains(first, "owned.json") {
			t.Fatalf("region missing:\n%s", first)
		}
		runInitFor(t, dir)
		if second := readZoneIgnore(t, dir); second != first {
			t.Fatalf("re-run changed the file:\n--first--\n%s\n--second--\n%s", first, second)
		}
		if n := strings.Count(first, "fathom:begin"); n != 1 {
			t.Fatalf("want one region, got %d", n)
		}
	})

	t.Run("fleet reconcile carries the region", func(t *testing.T) {
		dir := t.TempDir()
		spec, _ := reconcileSpec(dir, wiringDiff{Repo: dir})
		found := false
		for _, r := range spec.Regions {
			if r.File != ".fathom/.gitignore" {
				continue
			}
			found = true
			for _, n := range localZoneNames {
				if !strings.Contains("\n"+r.Body+"\n", "\n"+n+"\n") {
					t.Errorf("reconcile's .fathom/.gitignore region missing line %q:\n%s", n, r.Body)
				}
			}
		}
		if !found {
			t.Errorf("reconcile spec carries no .fathom/.gitignore region: %+v", spec.Regions)
		}
	})

	t.Run("dry run lists the file and writes nothing", func(t *testing.T) {
		dir := t.TempDir()
		svc := strings.ToLower(filepath.Base(dir))
		srv := fakeInitDaemon(t, svc, 123)
		defer srv.Close()
		var out, errBuf bytes.Buffer
		if code := cmdInit([]string{"--path", dir, "--url", srv.URL, "--bin", testBinPath, "--dry-run"}, &out, &errBuf); code != 0 {
			t.Fatalf("dry-run exit = %d; %s %s", code, out.String(), errBuf.String())
		}
		if !strings.Contains(out.String(), ".fathom/.gitignore") {
			t.Errorf("dry run does not list .fathom/.gitignore:\n%s", out.String())
		}
		if _, err := os.Stat(filepath.Join(dir, ".fathom", ".gitignore")); err == nil {
			t.Error("dry run wrote .fathom/.gitignore")
		}
	})
}
