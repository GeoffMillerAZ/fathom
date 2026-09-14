package main

// configaudit_codefacts_test.go — VB-28's `fathom config audit
// --code-facts` CLI surface: default path unchanged without the flag,
// --strict gates only on anchored-drifted, and the JSON envelope shape
// (review FIX-BEFORE-MERGE item 6).

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeCodeFactsFixture(t *testing.T, root, sentence string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("# CLAUDE.md\n\n"+sentence+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestConfigAuditWithoutCodeFactsFlagUnchanged: a repo whose CLAUDE.md
// would classify anchored-drifted under --code-facts must NOT be
// affected by that at all when the flag is absent — --code-facts is
// strictly additive, opt-in behavior (default OFF, vision 22 §3c).
func TestConfigAuditWithoutCodeFactsFlagUnchanged(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	writeCodeFactsFixture(t, root, "See `MISSING.md` for details.")

	var out, errB bytes.Buffer
	code := cmdConfigAudit([]string{"--repo", root}, &out, &errB)
	if code != 0 {
		t.Fatalf("exit %d without --code-facts, stderr %s", code, errB.String())
	}
	if strings.Contains(out.String(), "anchored-drifted") || strings.Contains(out.String(), "code-fact sentence audit") {
		t.Fatalf("code-facts output leaked without the flag: %s", out.String())
	}
}

// TestConfigAuditCodeFactsAdvisoryByDefault: a drifted sentence exits 0
// unless --strict is also given (vision 22 §3c: advisory until VB-29's
// precision bar).
func TestConfigAuditCodeFactsAdvisoryByDefault(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	writeCodeFactsFixture(t, root, "See `MISSING.md` for details.")

	var out, errB bytes.Buffer
	code := cmdConfigAudit([]string{"--repo", root, "--code-facts"}, &out, &errB)
	if code != 0 {
		t.Fatalf("exit %d without --strict, want 0 (advisory), stderr %s", code, errB.String())
	}
	if !strings.Contains(out.String(), "anchored-drifted") {
		t.Fatalf("output must show the drifted sentence: %s", out.String())
	}
}

// TestConfigAuditCodeFactsStrictFailsOnDrift: --strict turns an
// anchored-drifted sentence into a gate failure.
func TestConfigAuditCodeFactsStrictFailsOnDrift(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	writeCodeFactsFixture(t, root, "See `MISSING.md` for details.")

	var out, errB bytes.Buffer
	code := cmdConfigAudit([]string{"--repo", root, "--code-facts", "--strict"}, &out, &errB)
	if code != 1 {
		t.Fatalf("exit %d with --strict over a drifted sentence, want 1, stdout %s stderr %s", code, out.String(), errB.String())
	}
}

// TestConfigAuditCodeFactsStrictPassesCleanRepo: --strict exits 0 when
// nothing classifies anchored-drifted — it gates on drift specifically,
// not on the mere presence of code-fact-shaped sentences.
func TestConfigAuditCodeFactsStrictPassesCleanRepo(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	writeCodeFactsFixture(t, root, "See `CLAUDE.md` itself.")

	var out, errB bytes.Buffer
	code := cmdConfigAudit([]string{"--repo", root, "--code-facts", "--strict"}, &out, &errB)
	if code != 0 {
		t.Fatalf("exit %d with --strict over a clean repo, want 0, stdout %s stderr %s", code, out.String(), errB.String())
	}
}

// TestConfigAuditCodeFactsJSONShape: the JSON envelope carries repo_root,
// findings (with the four-way class, file, line, sentence), and a
// coverage block (complete, note, unreadable, entries) — the shape a
// caller other than the text renderer must be able to parse.
func TestConfigAuditCodeFactsJSONShape(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	writeCodeFactsFixture(t, root, "See `MISSING.md` for details.")

	var out, errB bytes.Buffer
	code := cmdConfigAudit([]string{"--repo", root, "--code-facts", "--json"}, &out, &errB)
	if code != 0 {
		t.Fatalf("exit %d, stderr %s", code, errB.String())
	}

	var env struct {
		RepoRoot string `json:"repo_root"`
		Findings []struct {
			File     string `json:"file"`
			Line     int    `json:"line"`
			Class    string `json:"class"`
			Sentence string `json:"sentence"`
			Receipt  string `json:"receipt,omitempty"`
		} `json:"findings"`
		Coverage struct {
			Complete   bool   `json:"complete"`
			Note       string `json:"note,omitempty"`
			Unreadable []any  `json:"unreadable,omitempty"`
			Entries    int    `json:"entries"`
		} `json:"coverage"`
	}
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("decoding JSON envelope: %v\n%s", err, out.String())
	}
	if env.RepoRoot != root {
		t.Errorf("repo_root = %q, want %q", env.RepoRoot, root)
	}
	if !env.Coverage.Complete {
		t.Errorf("coverage.complete = false, want true (nothing unreadable in this fixture)")
	}
	foundDrifted := false
	for _, f := range env.Findings {
		if f.Class == "anchored-drifted" {
			foundDrifted = true
			if f.File != "CLAUDE.md" || f.Receipt == "" {
				t.Errorf("drifted finding missing file/receipt: %+v", f)
			}
		}
	}
	if !foundDrifted {
		t.Errorf("no anchored-drifted finding in %+v", env.Findings)
	}
}

// TestConfigAuditCodeFactsRejectsSarifFormat: --format sarif has no
// code-fact rendering — it must error, not silently fall through to text.
func TestConfigAuditCodeFactsRejectsSarifFormat(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	writeCodeFactsFixture(t, root, "See `MISSING.md` for details.")

	var out, errB bytes.Buffer
	code := cmdConfigAudit([]string{"--repo", root, "--code-facts", "--format", "sarif"}, &out, &errB)
	if code != 1 {
		t.Fatalf("exit %d, want 1 (sarif unsupported with --code-facts); stdout %s", code, out.String())
	}
	if !strings.Contains(errB.String(), "--code-facts") {
		t.Errorf("stderr must explain the refusal: %s", errB.String())
	}
	if out.Len() != 0 {
		t.Errorf("no text/SARIF fallback output expected, got: %s", out.String())
	}
}
