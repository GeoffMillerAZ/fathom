package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/statedir"
)

// TestNarrateExpiry pins IO-21: narrate is bounded, auto-expiring, and off
// by default — the pioneering dial, never a standing tax.
func TestNarrateExpiry(t *testing.T) {
	t.Setenv("FATHOM_NARRATE_STATE", filepath.Join(t.TempDir(), "narrate.json"))

	if narrateActive() {
		t.Fatal("narrate must default off")
	}
	var stdout, stderr bytes.Buffer
	if code := cmdNarrate([]string{"on"}, &stdout, &stderr); code == 0 {
		t.Fatal("unbounded narrate must refuse")
	}
	if code := cmdNarrate([]string{"on", "--for", "1h", "--note", "pilot"}, &stdout, &stderr); code != 0 {
		t.Fatalf("narrate on: %s", stderr.String())
	}
	if !narrateActive() {
		t.Fatal("narrate should be active")
	}
	if code := cmdNarrate([]string{"off"}, &stdout, &stderr); code != 0 {
		t.Fatal(stderr.String())
	}
	if narrateActive() {
		t.Fatal("narrate off must deactivate")
	}
}

// TestStoryRows pins IO-22: rows append per-repo while active, version-
// stamped; inactive narrate writes NOTHING.
func TestStoryRows(t *testing.T) {
	t.Setenv("FATHOM_NARRATE_STATE", filepath.Join(t.TempDir(), "narrate.json"))
	repo := t.TempDir()

	recordStoryRow(repo, "obligation-served", map[string]any{"file": "a.go"})
	if entries, _ := os.ReadDir(filepath.Join(statedir.Resolve(repo), "artifacts", "narrate")); len(entries) != 0 {
		t.Fatal("inactive narrate must write nothing")
	}

	var stdout, stderr bytes.Buffer
	if code := cmdNarrate([]string{"on", "--for", "1h"}, &stdout, &stderr); code != 0 {
		t.Fatal(stderr.String())
	}
	recordStoryRow(repo, "obligation-served", map[string]any{"file": "a.go", "rules": 3})
	dir := filepath.Join(statedir.Resolve(repo), "artifacts", "narrate")
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("want one story file, got %v (%v)", entries, err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	var row map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(data), &row); err != nil {
		t.Fatal(err)
	}
	if row["kind"] != "obligation-served" || row["file"] != "a.go" || row["rules"] != float64(3) {
		t.Errorf("row = %v", row)
	}
	if row["schema_version"] != float64(1) || row["fathom_version"] == "" {
		t.Errorf("stamps missing: %v", row)
	}
	if !strings.HasPrefix(entries[0].Name(), "story-"+time.Now().UTC().Format("2006-01-02")) {
		t.Errorf("file name = %q", entries[0].Name())
	}
}
