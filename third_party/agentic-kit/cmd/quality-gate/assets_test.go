package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScaffoldPrivateAndExact(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".quality-gate")
	if err := scaffold(dir); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o700 {
		t.Fatalf("directory mode = %o, want 700", got)
	}
	for name, want := range map[string][]byte{
		"policy.json":  defaultPolicy,
		"schema.cue":   policySchema,
		"lefthook.yml": []byte(lefthookFragment),
	} {
		path := filepath.Join(dir, name)
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("%s differs from embedded asset", name)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("%s mode = %o, want 600", name, got)
		}
	}
}

func TestScaffoldRefusesExistingDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".quality-gate")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "policy.json")
	if err := os.WriteFile(marker, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := scaffold(dir); err == nil {
		t.Fatal("existing directory was overwritten")
	}
	got, err := os.ReadFile(marker)
	if err != nil || string(got) != "original" {
		t.Fatalf("existing file changed: %q, %v", got, err)
	}
}

func TestEmbeddedPolicyAndHookShape(t *testing.T) {
	p, err := decodePolicy(defaultPolicy)
	if err != nil {
		t.Fatalf("default policy rejected by runtime: %v", err)
	}
	if p.Version != 1 || p.ID != "repository-quality" || p.Model.Provider != "none" {
		t.Fatalf("unexpected default identity/provider: %+v", p)
	}
	if len(p.Rules) != 2 || len(p.Model.Sites) != 1 || p.Model.Sites[0].Mode != "shadow" {
		t.Fatalf("unexpected default rules/sites: %+v", p)
	}
	if !strings.Contains(lefthookFragment, "commit-msg:") || strings.Contains(lefthookFragment, "{staged_files}") || strings.Contains(lefthookFragment, "{push_files}") || !strings.Contains(lefthookFragment, "use_stdin: true") {
		t.Fatal("hook fragment filters files or loses pre-push stdin")
	}
}
