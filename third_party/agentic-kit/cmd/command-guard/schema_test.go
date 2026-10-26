package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCUESchemaByteLimits(t *testing.T) {
	cue, err := exec.LookPath("cue")
	if err != nil {
		t.Skip("cue executable unavailable")
	}

	// Exercise the same package vet invocation used by the CI cue-vet task.
	cmd := exec.Command(cue, "vet", "./testdata/")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("cue vet ./testdata/: %v\n%s", err, out)
	}

	schema, err := os.ReadFile("schema.cue")
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name, definition string
		limit            int
		fields           func(string) string
	}{
		{"rule", "#Rule", 2048, func(s string) string {
			return fmt.Sprintf(`{id: "rule", pattern: %s}`, cueString(s))
		}},
		{"scanner", "#ScannerPolicy", 4096, func(s string) string {
			return fmt.Sprintf(`{kind: "gitleaks", profile: "gitleaks-stdin-8.30.1", executable: %s, sha256: "%s", timeoutMillis: 1, maxReportBytes: 1}`, cueString(s), strings.Repeat("0", 64))
		}},
		{"search", "#SearchRequest", 1024, func(s string) string {
			return fmt.Sprintf(`{version: "command-guard.request/1", files: [{path: "a", sha256: "%s"}], kind: "search-literal", literal: %s}`, strings.Repeat("0", 64), cueString(s))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, v := range []struct {
				name, value string
				valid       bool
			}{
				{"ascii-at-limit", strings.Repeat("a", tc.limit), true},
				{"utf8-at-byte-limit", strings.Repeat("a", tc.limit-2) + "é", true},
				{"ascii-over-limit", strings.Repeat("a", tc.limit+1), false},
				{"utf8-one-byte-over", strings.Repeat("a", tc.limit-1) + "é", false},
				{"utf8-two-bytes-over", strings.Repeat("a", tc.limit-1) + "€", false},
			} {
				t.Run(v.name, func(t *testing.T) {
					dir := t.TempDir()
					if err := os.WriteFile(filepath.Join(dir, "schema.cue"), schema, 0600); err != nil {
						t.Fatal(err)
					}
					fixture := fmt.Sprintf("package commandguard\nv: %s & %s\n", tc.definition, tc.fields(v.value))
					if err := os.WriteFile(filepath.Join(dir, "value.cue"), []byte(fixture), 0600); err != nil {
						t.Fatal(err)
					}
					cmd := exec.Command(cue, "vet", "-c", ".")
					cmd.Dir = dir
					out, err := cmd.CombinedOutput()
					if (err == nil) != v.valid {
						t.Fatalf("cue vet -c ., valid=%t, bytes=%d, runes=%d: %v\n%s", v.valid, len(v.value), len([]rune(v.value)), err, out)
					}
				})
			}
		})
	}
}

func cueString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
