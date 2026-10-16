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
	// archive's absorption guard: the ids are named, so archive would accept.
	if err := os.WriteFile(filepath.Join(dir, "CHANGELOG.md"), []byte("D-1 and D-2 shipped\n"), 0o644); err != nil {
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

// A plan with nothing open whose row ids archive's absorption guard would
// refuse is "archive blocked", never "archivable": status runs the same
// check as archive and names the missing ids (human line and --json).
func TestPlansStatusBlockedWhenIDsUnabsorbed(t *testing.T) {
	storePath := seedPlansFixtureStore(t)
	dir := t.TempDir()
	plan := filepath.Join(dir, "blocked.md")
	if err := os.WriteFile(plan, []byte("# Blocked\n\n"+
		"- [x] one {id: B-1, symbol: validateCharge, skip_test: true, skip: \"test fixture\"}\n"+
		"- [x] two {id: B-2, symbol: validateCharge, skip_test: true, skip: \"test fixture\"}\n"+
		"- [x] three {id: B-3, symbol: validateCharge, skip_test: true, skip: \"test fixture\"}\n"+
		"- [x] four {id: B-4, symbol: validateCharge, skip_test: true, skip: \"test fixture\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "CHANGELOG.md"), []byte("B-1 shipped\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errb bytes.Buffer
	args := []string{"plans", "status", "--service", "payment-service", "--store", storePath, "--plan", plan}
	if code := run(args, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	text := out.String()
	if strings.Contains(text, "archivable:") {
		t.Errorf("plan archive would refuse is marked archivable:\n%s", text)
	}
	if !strings.Contains(text, "archive blocked: blocked.md — 3 row ids not absorbed (first: B-2, B-3, B-4)") {
		t.Errorf("missing blocked line:\n%s", text)
	}

	out.Reset()
	if code := run(append(args, "--json"), &out, &errb); code != 0 {
		t.Fatalf("json exit %d: %s", code, errb.String())
	}
	var rows []struct {
		Archivable bool     `json:"archivable"`
		Blocked    bool     `json:"archive_blocked"`
		Missing    []string `json:"unabsorbed_ids"`
	}
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil || len(rows) != 1 {
		t.Fatalf("--json: %v\n%s", err, out.String())
	}
	if rows[0].Archivable || !rows[0].Blocked || strings.Join(rows[0].Missing, ",") != "B-2,B-3,B-4" {
		t.Errorf("json split wrong: %+v", rows[0])
	}
}

// When archive's absorption check cannot run (here CHANGELOG.md is a
// directory, so reading it fails), status must not advertise the plan
// archivable: a failed check is "archive blocked" with its error, never a pass.
func TestPlansStatusBlockedWhenAbsorptionCheckFails(t *testing.T) {
	storePath := seedPlansFixtureStore(t)
	dir := t.TempDir()
	plan := filepath.Join(dir, "unchecked.md")
	if err := os.WriteFile(plan, []byte("# Unchecked\n\n"+
		"- [x] one {id: C-1, symbol: validateCharge, skip_test: true, skip: \"test fixture\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "CHANGELOG.md"), 0o755); err != nil {
		t.Fatal(err)
	}

	var out, errb bytes.Buffer
	args := []string{"plans", "status", "--service", "payment-service", "--store", storePath, "--plan", plan}
	if code := run(args, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	text := out.String()
	if strings.Contains(text, "archivable:") {
		t.Errorf("a plan whose absorption check failed is marked archivable:\n%s", text)
	}
	if !strings.Contains(text, "archive blocked: unchecked.md — absorption check failed") {
		t.Errorf("missing failed-check line:\n%s", text)
	}

	out.Reset()
	if code := run(append(args, "--json"), &out, &errb); code != 0 {
		t.Fatalf("json exit %d: %s", code, errb.String())
	}
	var rows []struct {
		Archivable bool   `json:"archivable"`
		Blocked    bool   `json:"archive_blocked"`
		CheckError string `json:"archive_check_error"`
	}
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil || len(rows) != 1 {
		t.Fatalf("--json: %v\n%s", err, out.String())
	}
	if rows[0].Archivable || !rows[0].Blocked || rows[0].CheckError == "" {
		t.Errorf("json: %+v, want blocked with the check's error", rows[0])
	}
}
