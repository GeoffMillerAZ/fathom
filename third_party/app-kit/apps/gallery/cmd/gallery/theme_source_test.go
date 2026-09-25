package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestThemeSourceIsExplicitBoundedAndReadOnly(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "app.cue")
	input := []byte("// Preserve authored source exactly\ntokens: {name: \"example\"}\n")
	if err := os.WriteFile(source, input, 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := parseConfig([]string{"-design-assets=", "-theme-source", source}, "", io.Discard)
	if err != nil || cfg.themeSource != source {
		t.Fatal(cfg, err)
	}
	actual, err := readThemeSource(source)
	if err != nil || string(actual) != string(input) {
		t.Fatal("source changed", err)
	}
	actual[0] = '!'
	after, err := os.ReadFile(source)
	if err != nil || string(after) != string(input) {
		t.Fatal("read changed source file")
	}
	for _, size := range []int{0, (64 << 10) + 1} {
		bad := filepath.Join(dir, fmt.Sprintf("bad-%d.cue", size))
		if err := os.WriteFile(bad, []byte(strings.Repeat("x", size)), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readThemeSource(bad); err == nil {
			t.Fatal("invalid source size admitted", size)
		}
	}
	for _, path := range []string{dir, filepath.Join(dir, "missing")} {
		if _, err := readThemeSource(path); err == nil {
			t.Fatal("invalid source admitted", path)
		}
	}
	defaults, err := parseConfig([]string{"-design-assets="}, "", io.Discard)
	if err != nil || defaults.themeSource != "" {
		t.Fatal("source became implicit")
	}
}

func TestThemeLabIdentityBindsActualExecutable(t *testing.T) {
	identity, err := executableIdentity()
	if err != nil {
		t.Fatal(err)
	}
	name, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("local-gallery-binary:sha256:%x", sha256.Sum256(body))
	if identity != want {
		t.Fatal("identity does not bind this exact executable")
	}
}
