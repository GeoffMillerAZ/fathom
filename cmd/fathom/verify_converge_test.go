package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/specdialect"
)

// TestVerifyConvergeCLI (SI-5/SI-6 end to end): `verify --converge` on a
// Spec Kit spec in a git repository reports the three sections — the
// unclaimed code read from the spec's branch range against the index —
// and `--write-fixups` writes only inside the converge block.
func TestVerifyConvergeCLI(t *testing.T) {
	requireGit(t)
	t.Setenv("FATHOM_VERIFY_LEDGER", filepath.Join(t.TempDir(), "verify-ledger.jsonl"))
	repo := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(repo, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo, "-c", "user.email=t@t", "-c", "user.name=t"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write("go.mod", "module example.com/pay\n\ngo 1.22\n")
	write("pay/pay.go", "package pay\n\nfunc Charge() int {\n\treturn 1\n}\n\nfunc Void() int {\n\treturn 2\n}\n")
	spec := "# Tasks\n\n" +
		"- [x] T001 Charge {symbol: Charge, package: pay, skip_test: true, skip: \"fixture\"}\n" +
		"- [x] T002 Ghost {symbol: Ghost, skip_test: true, skip: \"fixture\"}\n" +
		"- [ ] T003 Write the runbook\n"
	write("specs/001-pay/tasks.md", spec)
	git("init", "-q")
	git("add", "-A")
	git("commit", "-qm", "spec and code")
	// On the branch: Charge and Void both change, Refund is new.
	write("pay/pay.go", "package pay\n\nfunc Charge() int {\n\treturn 10\n}\n\nfunc Void() int {\n\treturn 20\n}\n\nfunc Refund() int {\n\treturn 3\n}\n")

	store := filepath.Join(t.TempDir(), "s.sqlite")
	var ing bytes.Buffer
	if code := run([]string{"ingest", "--service", "pay", "--path", repo, "--store", store}, &ing, &ing); code != 0 {
		t.Fatalf("ingest: %d\n%s", code, ing.String())
	}
	pf := filepath.Join(repo, "specs", "001-pay", "tasks.md")

	var out, errb bytes.Buffer
	code := run([]string{"verify", "--service", "pay", "--plan", pf, "--store", store, "--converge", "--json"}, &out, &errb)
	if code != 1 { // T002 is claimed and fails: the gate is unchanged by --converge
		t.Fatalf("want exit 1, got %d\nstdout=%s\nstderr=%s", code, out.String(), errb.String())
	}
	var doc struct {
		Converge []specdialect.ConvergeReport `json:"converge"`
	}
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil || len(doc.Converge) != 1 {
		t.Fatalf("json: %v\n%s", err, out.String())
	}
	r := doc.Converge[0]
	if r.UnclaimedNote != "" || r.Base == "" || r.Dialect != "speckit" {
		t.Fatalf("converge header: %+v", r)
	}
	if len(r.ClaimedUnproven) != 1 || r.ClaimedUnproven[0].TaskID != "T002" || r.ClaimedUnproven[0].Line != 4 {
		t.Errorf("claimed-unproven: %+v", r.ClaimedUnproven)
	}
	if len(r.Unanchored) != 1 || r.Unanchored[0].TaskID != "T003" {
		t.Errorf("unanchored: %+v", r.Unanchored)
	}
	var names []string
	for _, s := range r.Unclaimed {
		names = append(names, s.Name)
	}
	if strings.Join(names, ",") != "Void,Refund" {
		t.Errorf("unclaimed = %v, want Void,Refund (Charge is anchored by T001)", names)
	}
	if after, _ := os.ReadFile(pf); string(after) != spec {
		t.Fatal("--converge without --write-fixups changed the spec")
	}

	// Text mode renders the sections; --write-fixups writes only the block.
	out.Reset()
	errb.Reset()
	run([]string{"verify", "--service", "pay", "--plan", pf, "--store", store, "--converge", "--write-fixups"}, &out, &errb)
	for _, want := range []string{"claimed-unproven (1)", "unanchored (1)", "unclaimed code (2)"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("text lacks %q:\n%s", want, out.String())
		}
	}
	after, _ := os.ReadFile(pf)
	if !strings.HasPrefix(string(after), spec) || !strings.Contains(string(after), specdialect.ConvergeBegin) ||
		!strings.Contains(string(after), "- [ ] T004 Make T002 pass") || !strings.Contains(string(after), "- [ ] T006 Claim or remove Refund") {
		t.Fatalf("fix-ups:\n%s\nstderr=%s", after, errb.String())
	}

	// --write-fixups without --converge is refused.
	if code := run([]string{"verify", "--service", "pay", "--plan", pf, "--store", store, "--write-fixups"}, &out, &errb); code != 2 {
		t.Fatalf("--write-fixups alone: want exit 2, got %d", code)
	}
}
