package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHarnessHooksPrintsCursorFlatEntries (GL-7): `fathom harness hooks
// --harness cursor` prints the flat .cursor/hooks.json shape — {version:1,
// hooks:{event:[{command, matcher, timeout}]}} — never the nested settings
// groups, each entry calling the given binary.
func TestHarnessHooksPrintsCursorFlatEntries(t *testing.T) {
	var out, errb bytes.Buffer
	if code := cmdHarness([]string{"hooks", "--harness", "cursor", "--bin", "/opt/fathom"}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	text := out.String()
	if !strings.Contains(text, "merge into .cursor/hooks.json") {
		t.Errorf("header does not name .cursor/hooks.json:\n%s", text)
	}
	body := text[strings.Index(text, "{"):]
	var doc struct {
		Version int                         `json:"version"`
		Hooks   map[string][]map[string]any `json:"hooks"`
	}
	if err := json.Unmarshal([]byte(body), &doc); err != nil {
		t.Fatalf("body is not JSON: %v\n%s", err, body)
	}
	if doc.Version != 1 {
		t.Errorf("version = %d, want 1", doc.Version)
	}
	e := doc.Hooks["afterFileEdit"]
	if len(e) != 1 || e[0]["command"] != "/opt/fathom edit-hook --harness cursor" || e[0]["hooks"] != nil {
		t.Errorf("afterFileEdit entries = %v, want one flat entry", e)
	}
}

// TestDoctorReadsFlatCursorHookFile (GL-7): the harness_hooks row reads a
// hand-merged .cursor/hooks.json (flat entries) and counts what it finds —
// still skipped (unverified), never a pass.
func TestDoctorReadsFlatCursorHookFile(t *testing.T) {
	dir := t.TempDir()
	var out, errb bytes.Buffer
	if code := cmdHarness([]string{"hooks", "--harness", "cursor", "--bin", "/opt/fathom"}, &out, &errb); code != 0 {
		t.Fatal(errb.String())
	}
	body := out.String()[strings.Index(out.String(), "{"):]
	if err := os.MkdirAll(filepath.Join(dir, ".cursor"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "hooks.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	c := checkHarnessHookFiles(dir, "cursor")
	if c.Status != doctorSkipped || !strings.Contains(c.Reason, "7 of 7") {
		t.Errorf("merged flat file = %+v, want 7 of 7 present and still skipped", c)
	}
}
