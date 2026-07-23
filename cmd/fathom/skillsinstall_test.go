package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	skillsdist "github.com/geoffmilleraz/fathom/dist/agents/skills"
)

func TestResolveSkillsMode(t *testing.T) {
	cases := []struct {
		raw     string
		want    string
		wantErr bool
	}{
		{raw: "", want: ""},
		{raw: "true", want: "all"}, // bare --skills (optionalStringFlag.IsBoolFlag)
		{raw: "claude", want: "claude"},
		{raw: "codex", want: "codex"},
		{raw: "all", want: "all"},
		{raw: "bogus", wantErr: true},
	}
	for _, tc := range cases {
		got, err := resolveSkillsMode(tc.raw)
		if tc.wantErr {
			if err == nil {
				t.Errorf("resolveSkillsMode(%q): expected error, got nil", tc.raw)
			}
			continue
		}
		if err != nil {
			t.Errorf("resolveSkillsMode(%q): unexpected error: %v", tc.raw, err)
		}
		if got != tc.want {
			t.Errorf("resolveSkillsMode(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestInstallSkills_Claude_Fresh(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	if err := installSkills(root, "claude", &out); err != nil {
		t.Fatalf("installSkills: %v", err)
	}
	for _, name := range skillsdist.Names {
		p := filepath.Join(root, ".claude", "skills", name, "SKILL.md")
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("reading %s: %v", p, err)
		}
		s := string(raw)
		if !strings.Contains(s, "name: "+name) {
			t.Errorf("%s missing frontmatter name: %s\n%s", p, name, s)
		}
		if !strings.Contains(s, skillMarker) {
			t.Errorf("%s missing fathom-owned marker:\n%s", p, s)
		}
	}
	// Codex prompts must NOT have been written in "claude" mode.
	if _, err := os.Stat(filepath.Join(root, ".codex")); err == nil {
		t.Error(".codex directory created in claude-only mode")
	}
	if !strings.Contains(out.String(), "created") {
		t.Errorf("expected stdout to report created files, got: %s", out.String())
	}
}

func TestInstallSkills_Codex_Fresh(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := t.TempDir()

	var out bytes.Buffer
	if err := installSkills(root, "codex", &out); err != nil {
		t.Fatalf("installSkills: %v", err)
	}
	for _, name := range skillsdist.Names {
		p := filepath.Join(home, ".codex", "prompts", name+".md")
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("reading %s: %v", p, err)
		}
		s := string(raw)
		if !strings.Contains(s, "description:") {
			t.Errorf("%s missing Codex frontmatter description:\n%s", p, s)
		}
		if !strings.Contains(s, skillMarker) {
			t.Errorf("%s missing fathom-owned marker:\n%s", p, s)
		}
	}
	// Claude Code skills must NOT have been written in "codex" mode.
	if _, err := os.Stat(filepath.Join(root, ".claude", "skills")); err == nil {
		t.Error(".claude/skills directory created in codex-only mode")
	}
}

func TestInstallSkills_All_WritesBothHarnesses(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := t.TempDir()

	var out bytes.Buffer
	if err := installSkills(root, "all", &out); err != nil {
		t.Fatalf("installSkills: %v", err)
	}
	for _, name := range skillsdist.Names {
		if _, err := os.Stat(filepath.Join(root, ".claude", "skills", name, "SKILL.md")); err != nil {
			t.Errorf("claude skill %s not installed: %v", name, err)
		}
		if _, err := os.Stat(filepath.Join(home, ".codex", "prompts", name+".md")); err != nil {
			t.Errorf("codex prompt %s not installed: %v", name, err)
		}
	}
}

func TestInstallSkills_Empty_NoOp(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	if err := installSkills(root, "", &out); err != nil {
		t.Fatalf("installSkills: %v", err)
	}
	if out.String() != "" {
		t.Errorf("expected no output for mode \"\", got %q", out.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".claude")); err == nil {
		t.Error(".claude directory created for mode \"\"")
	}
}

// TestInstallSkills_Idempotent proves a re-run only touches fathom-owned
// files, and reports "already current" (no byte change) when content hasn't
// changed — the AK-2 idempotency requirement.
func TestInstallSkills_Idempotent(t *testing.T) {
	root := t.TempDir()

	var out1 bytes.Buffer
	if err := installSkills(root, "claude", &out1); err != nil {
		t.Fatalf("first installSkills: %v", err)
	}

	before := map[string][]byte{}
	for _, name := range skillsdist.Names {
		p := filepath.Join(root, ".claude", "skills", name, "SKILL.md")
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("reading %s after first run: %v", p, err)
		}
		before[p] = raw
	}

	var out2 bytes.Buffer
	if err := installSkills(root, "claude", &out2); err != nil {
		t.Fatalf("second installSkills: %v", err)
	}
	for p, raw := range before {
		got, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("reading %s after second run: %v", p, err)
		}
		if !bytes.Equal(raw, got) {
			t.Errorf("%s changed on second (idempotent) run", p)
		}
	}
	if !strings.Contains(out2.String(), "already current") {
		t.Errorf("second run stdout missing 'already current': %s", out2.String())
	}
}

// TestInstallSkills_PreexistingNonOwnedFile_RefusesLoudly is AK-2's required
// scenario: a target repo that already has its own skill/prompt with the
// same name (no fathom:owned marker) must not be silently clobbered.
func TestInstallSkills_PreexistingNonOwnedFile_RefusesLoudly(t *testing.T) {
	root := t.TempDir()
	conflictDir := filepath.Join(root, ".claude", "skills", "fathom-onboard")
	if err := os.MkdirAll(conflictDir, 0o755); err != nil {
		t.Fatal(err)
	}
	userContent := []byte("---\nname: fathom-onboard\ndescription: my own skill, not fathom's\n---\nmine.\n")
	conflictPath := filepath.Join(conflictDir, "SKILL.md")
	if err := os.WriteFile(conflictPath, userContent, 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err := installSkills(root, "claude", &out)
	if err == nil {
		t.Fatal("expected installSkills to refuse, got nil error")
	}
	if !strings.Contains(err.Error(), conflictPath) || !strings.Contains(err.Error(), "not fathom-owned") {
		t.Errorf("error doesn't name the conflict clearly: %v", err)
	}

	// The user's file must be untouched.
	got, rerr := os.ReadFile(conflictPath)
	if rerr != nil {
		t.Fatal(rerr)
	}
	if !bytes.Equal(got, userContent) {
		t.Error("pre-existing non-fathom-owned file was modified")
	}

	// Skills alphabetically/positionally before the conflict (fathom-plan,
	// fathom-verify — see skillsdist.Names) were still installed: the
	// refusal stops the loop at the conflict, it doesn't roll back what
	// already succeeded.
	for _, name := range []string{"fathom-plan", "fathom-verify"} {
		if _, statErr := os.Stat(filepath.Join(root, ".claude", "skills", name, "SKILL.md")); statErr != nil {
			t.Errorf("expected %s to be installed before the conflict was hit: %v", name, statErr)
		}
	}
	// fathom-feedback comes after the conflict in skillsdist.Names, so it
	// must NOT have been installed.
	if _, statErr := os.Stat(filepath.Join(root, ".claude", "skills", "fathom-feedback", "SKILL.md")); statErr == nil {
		t.Error("fathom-feedback was installed despite the loop stopping at the earlier conflict")
	}
}

// TestCmdInit_SkillsFlag_Claude proves --skills=claude installs the skill
// set as part of a full `fathom init` run, and that omitting --skills
// (the default) installs nothing, keeping cmdInit's existing behavior
// unchanged for callers that don't ask for skills.
func TestCmdInit_SkillsFlag_Claude(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	srv := fakeInitDaemon(t, strings.ToLower(filepath.Base(dir)), 5)
	defer srv.Close()

	var out, errBuf bytes.Buffer
	code := cmdInit([]string{"--path", dir, "--url", srv.URL, "--bin", testBinPath, "--skills=claude"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d; stdout=%s stderr=%s", code, out.String(), errBuf.String())
	}
	for _, name := range skillsdist.Names {
		if _, err := os.Stat(filepath.Join(dir, ".claude", "skills", name, "SKILL.md")); err != nil {
			t.Errorf("skill %s not installed: %v", name, err)
		}
	}
}

func TestCmdInit_SkillsFlagOmitted_InstallsNothing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	srv := fakeInitDaemon(t, strings.ToLower(filepath.Base(dir)), 5)
	defer srv.Close()

	var out, errBuf bytes.Buffer
	code := cmdInit([]string{"--path", dir, "--url", srv.URL, "--bin", testBinPath}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d; stdout=%s stderr=%s", code, out.String(), errBuf.String())
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude", "skills")); err == nil {
		t.Error(".claude/skills was created despite --skills being omitted")
	}
	if !strings.Contains(out.String(), "fathom init --skills") {
		t.Errorf("expected a MANUAL hint about --skills, stdout=%s", out.String())
	}
}

// TestCmdInit_SkillsFlagBare_MeansAll proves the bracket-optional
// `--skills[=claude|codex|all]` syntax: a bare --skills (no `=value`)
// installs both harnesses, not zero.
func TestCmdInit_SkillsFlagBare_MeansAll(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := t.TempDir()
	srv := fakeInitDaemon(t, strings.ToLower(filepath.Base(dir)), 5)
	defer srv.Close()

	var out, errBuf bytes.Buffer
	code := cmdInit([]string{"--path", dir, "--url", srv.URL, "--bin", testBinPath, "--skills"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d; stdout=%s stderr=%s", code, out.String(), errBuf.String())
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude", "skills", "fathom-plan", "SKILL.md")); err != nil {
		t.Errorf("claude skill not installed by bare --skills: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".codex", "prompts", "fathom-plan.md")); err != nil {
		t.Errorf("codex prompt not installed by bare --skills: %v", err)
	}
}

// TestCmdInit_SkillsFlag_ConflictReturnsExit1 proves the whole cmdInit
// command surfaces a skills marker conflict as a hard failure (exit 1),
// not folded into init's usual operational fail-open (exit 2).
func TestCmdInit_SkillsFlag_ConflictReturnsExit1(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	srv := fakeInitDaemon(t, strings.ToLower(filepath.Base(dir)), 5)
	defer srv.Close()

	conflictDir := filepath.Join(dir, ".claude", "skills", "fathom-plan")
	if err := os.MkdirAll(conflictDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(conflictDir, "SKILL.md"), []byte("mine, not fathom's\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errBuf bytes.Buffer
	code := cmdInit([]string{"--path", dir, "--url", srv.URL, "--bin", testBinPath, "--skills=claude"}, &out, &errBuf)
	if code != 1 {
		t.Fatalf("exit = %d, want 1; stdout=%s stderr=%s", code, out.String(), errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "installing skills") {
		t.Errorf("stderr missing skills-install context: %s", errBuf.String())
	}
}
