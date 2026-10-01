package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// SV-5 (ADR-060 decision 4): `fathom plans status` marks a plan with 0 open
// and 0 claimed-unproven rows archivable and names `fathom plans archive`;
// a plan with an open or a claimed-unproven row is not.
func TestPlansStatusMarksArchivable(t *testing.T) {
	storePath := seedPlansFixtureStore(t)
	dir := t.TempDir()
	done := filepath.Join(dir, "done.md")
	if err := os.WriteFile(done, []byte("# Done\n\n"+
		"- [x] validateCharge, claimed and proven {id: D-1, symbol: validateCharge, skip_test: true, skip: \"test fixture\"}\n"+
		"- [-] Later {id: D-2, disposition: deferred, reason: \"not now\", trigger: \"someday\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	unproven := filepath.Join(dir, "unproven.md")
	if err := os.WriteFile(unproven, []byte("# Unproven\n\n"+
		"- [x] A claim nothing proves {id: U-1, symbol: NoSuchSymbolAnywhere, skip_test: true, skip: \"test fixture\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	open := filepath.Join("testdata", "plans", "fixture-b.md")
	files := []string{done, unproven, open}

	var out, errb bytes.Buffer
	if code := run(planArgs("status", storePath, files), &out, &errb); code != 0 {
		t.Fatalf("plans status --json exit %d: %s", code, errb.String())
	}
	var rows []struct {
		Plan       string `json:"plan"`
		Open       int    `json:"open"`
		Unproven   int    `json:"claimed_unproven"`
		Archivable bool   `json:"archivable"`
		Archive    string `json:"archive_command"`
	}
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		t.Fatalf("--json: %v\n%s", err, out.String())
	}
	got := map[string]bool{}
	for _, r := range rows {
		base := filepath.Base(r.Plan)
		got[base] = r.Archivable
		if r.Archivable != (r.Archive != "") {
			t.Errorf("%s: archivable %v but archive command %q", base, r.Archivable, r.Archive)
		}
		if r.Archivable && !strings.Contains(r.Archive, "fathom plans archive") {
			t.Errorf("%s: archive command %q does not name fathom plans archive", base, r.Archive)
		}
	}
	if !got["done.md"] || got["unproven.md"] || got["fixture-b.md"] {
		t.Errorf("archivable = %v, want only done.md", got)
	}

	// The human table says it too, with the command.
	args := []string{"plans", "status", "--service", "payment-service", "--store", storePath}
	for _, f := range files {
		args = append(args, "--plan", f)
	}
	var hout bytes.Buffer
	if code := run(args, &hout, &errb); code != 0 {
		t.Fatalf("plans status exit %d: %s", code, errb.String())
	}
	text := hout.String()
	if !strings.Contains(text, "archivable: done.md") || !strings.Contains(text, "fathom plans archive --service payment-service") {
		t.Errorf("human table does not mark done.md archivable with its command:\n%s", text)
	}
	if strings.Contains(text, "archivable: unproven.md") || strings.Contains(text, "archivable: fixture-b.md") {
		t.Errorf("a plan with open or unproven rows marked archivable:\n%s", text)
	}
}
