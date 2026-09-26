package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// proposeRepo is a repo with three violated concerns: a dead pointer and a
// dead hook script whose files moved (mechanically mendable), and a
// forbidden raw-cp instruction (not — fathom never writes the prose).
func proposeRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		".fathom/concerns.cue": `packs: ["lefthook"]

concerns: {
	pointers_hold: {
		want:  "every pointer resolves"
		check: {kind: "pointers_resolve"}
	}
	deploy_safety: {
		want:  "no raw-cp deploy"
		check: {kind: "forbidden_pattern", pattern: "cp bin/fathom"}
	}
	lefthook_runs_gate: {off: true}
}
`,
		"CLAUDE.md":                "# router\nDesign: `docs/custodian.md`.\nDeploy: cp bin/fathom ~/bin\n",
		"docs/design/custodian.md": "# moved here\n",
		"lefthook.yml":             "pre-push:\n  commands:\n    gate:\n      run: bin/fathom verify\n    fmt:\n      run: bash scripts/old/fmt.sh\n",
		"scripts/fmt.sh":           "#!/bin/sh\n",
	}
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// treeDigest hashes every file under root (path + bytes) — the "fathom
// never writes" oracle.
func treeDigest(t *testing.T, root string) map[string][32]byte {
	t.Helper()
	out := map[string][32]byte{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		out[rel] = sha256.Sum256(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestConfigPropose_OffersDiffNeverWrites is CC-9's contract: `fathom
// config propose` offers a unified diff for the violations it can mend
// mechanically (a moved pointer target, a moved hook script), names the
// remedy for the one it cannot (raw-cp prose), leaves every byte of the
// repo as it was across text, --json and --patch — and the --patch
// output, applied by the session with `git apply`, turns exactly the two
// mendable concerns to holds.
func TestConfigPropose_OffersDiffNeverWrites(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	t.Setenv("HOME", t.TempDir())
	root := proposeRepo(t)
	before := treeDigest(t, root)

	var out, errB bytes.Buffer
	if code := cmdConfigPropose([]string{"--repo", root}, &out, &errB); code != 0 {
		t.Fatalf("propose exits 0 when it ran (the audit is the gate), got %d: %s", code, errB.String())
	}
	text := out.String()
	for _, want := range []string{
		"3 violated concern(s): 2 with a proposed diff, 1 needing a hand",
		"docs/custodian.md moved to docs/design/custodian.md",
		"scripts/old/fmt.sh moved to scripts/fmt.sh",
		"remedy    reword or remove the matching line(s)",
		"fathom has applied nothing",
		"+Design: `docs/design/custodian.md`.",
		"+      run: bash scripts/fmt.sh",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("text output lacks %q:\n%s", want, text)
		}
	}

	out.Reset()
	if code := cmdConfigPropose([]string{"--repo", root, "--json"}, &out, &errB); code != 0 {
		t.Fatalf("--json exit %d", code)
	}
	var model configProposeModel
	if err := json.Unmarshal(out.Bytes(), &model); err != nil {
		t.Fatalf("--json does not parse: %v", err)
	}
	if model.Violated != 3 || len(model.Proposals) != 3 || model.Patch == "" {
		t.Fatalf("--json model: %+v", model)
	}

	out.Reset()
	if code := cmdConfigPropose([]string{"--repo", root, "--patch"}, &out, &errB); code != 0 {
		t.Fatalf("--patch exit %d", code)
	}
	patch := out.String()
	if !strings.HasPrefix(patch, "diff --git a/CLAUDE.md b/CLAUDE.md\n") || patch != model.Patch {
		t.Fatalf("--patch prints only the diff, the same one --json carries:\n%s", patch)
	}

	after := treeDigest(t, root)
	if len(after) != len(before) {
		t.Fatalf("propose created or removed files: %d → %d", len(before), len(after))
	}
	for rel, sum := range before {
		if after[rel] != sum {
			t.Fatalf("propose wrote %s — fathom is the instrument, never the hand", rel)
		}
	}

	// The session applies the offer: the two mendable concerns now hold,
	// the prose violation stands.
	apply := exec.Command("git", "apply", "-")
	apply.Dir = root
	apply.Stdin = strings.NewReader(patch)
	if msg, err := apply.CombinedOutput(); err != nil {
		t.Fatalf("git apply refused the offered patch: %v\n%s", err, msg)
	}
	out.Reset()
	cmdConfigAudit([]string{"--repo", root, "--json"}, &out, &errB)
	var audit configAuditModel
	if err := json.Unmarshal(out.Bytes(), &audit); err != nil {
		t.Fatalf("audit --json: %v", err)
	}
	status := map[string]string{}
	for _, f := range audit.Findings {
		status[f.Concern.ID] = string(f.Status)
	}
	if status["pointers_hold"] != "holds" || status["lefthook_commands_resolve"] != "holds" || status["deploy_safety"] != "violated" {
		t.Fatalf("after applying the offered patch: %v", status)
	}
}

// TestConfigProposeNamesAbsence: no concerns is a named absence, exit 0,
// and --patch prints nothing at all.
func TestConfigProposeNamesAbsence(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	var out, errB bytes.Buffer
	if code := cmdConfigPropose([]string{"--repo", root}, &out, &errB); code != 0 || !strings.Contains(out.String(), "no concerns registered") {
		t.Fatalf("exit %d, out %q", code, out.String())
	}
	out.Reset()
	if code := cmdConfigPropose([]string{"--repo", root, "--patch"}, &out, &errB); code != 0 || out.Len() != 0 {
		t.Fatalf("--patch with nothing to propose prints nothing: exit %d, %q", code, out.String())
	}
}
