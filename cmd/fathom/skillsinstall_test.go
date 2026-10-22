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
		value   string
		set     bool
		want    string
		wantErr bool
	}{
		{value: "", set: false, want: ""},       // flag omitted entirely
		{value: "true", set: true, want: "all"}, // bare --skills (optionalStringFlag.IsBoolFlag)
		{value: "", set: true, wantErr: true},   // explicit --skills= — refused, never "all"
		{value: "claude", set: true, want: "claude"},
		{value: "codex", set: true, want: "codex"},
		{value: "all", set: true, want: "all"},
		{value: "agents", set: true, want: "agents"},
		{value: "codex-prompts", set: true, want: "codex-prompts"},
		{value: "bogus", set: true, wantErr: true},
	}
	for _, tc := range cases {
		got, err := resolveSkillsMode(tc.value, tc.set)
		if tc.wantErr {
			if err == nil {
				t.Errorf("resolveSkillsMode(%q, %t): expected error, got nil", tc.value, tc.set)
			}
			continue
		}
		if err != nil {
			t.Errorf("resolveSkillsMode(%q, %t): unexpected error: %v", tc.value, tc.set, err)
		}
		if got != tc.want {
			t.Errorf("resolveSkillsMode(%q, %t) = %q, want %q", tc.value, tc.set, got, tc.want)
		}
	}
}

func TestInstallSkills_Claude_Fresh(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	if err := installSkills(nil, root, "claude", &out); err != nil {
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

// TestInstallSkills_Codex_Fresh pins RP-1 (docs/plans/reach-parity.md):
// `--skills=codex` writes the path Codex DOCUMENTS for skills —
// <repo>/.agents/skills/<name>/SKILL.md — and no longer the deprecated
// ~/.codex/prompts twin (0.8.0 Breaking; that twin is --skills=codex-prompts).
func TestInstallSkills_Codex_Fresh(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := t.TempDir()

	var out bytes.Buffer
	if err := installSkills(nil, root, "codex", &out); err != nil {
		t.Fatalf("installSkills: %v", err)
	}
	for _, name := range skillsdist.Names {
		p := filepath.Join(root, ".agents", "skills", name, "SKILL.md")
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("reading %s: %v", p, err)
		}
		s := string(raw)
		if !strings.Contains(s, "name: "+name) {
			t.Errorf("%s missing spec frontmatter name:\n%s", p, s)
		}
		if !strings.Contains(s, skillMarker) {
			t.Errorf("%s missing fathom-owned marker:\n%s", p, s)
		}
	}
	if _, err := os.Stat(filepath.Join(home, ".codex")); err == nil {
		t.Error("~/.codex was written by --skills=codex — the prompts twin is --skills=codex-prompts only")
	}
	// ADR-059: the path Codex reads is a link to the one real copy.
	if fi, err := os.Lstat(filepath.Join(root, ".agents", "skills")); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf(".agents/skills is not a link in codex mode (err=%v)", err)
	}
	if fi, err := os.Lstat(filepath.Join(root, ".claude", "skills", "fathom-plan", "SKILL.md")); err != nil || !fi.Mode().IsRegular() {
		t.Errorf(".claude/skills does not hold the real copy in codex mode (err=%v)", err)
	}
}

// TestInstallSkills_CodexPrompts_Fresh: the deprecated ~/.codex/prompts
// twin still renders, but only behind the explicit mode — the one
// user-global write --skills can make.
func TestInstallSkills_CodexPrompts_Fresh(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := t.TempDir()

	var out bytes.Buffer
	if err := installSkills(nil, root, "codex-prompts", &out); err != nil {
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
	for _, d := range []string{".claude", ".agents"} {
		if _, err := os.Stat(filepath.Join(root, d)); err == nil {
			t.Errorf("%s directory created in codex-prompts mode", d)
		}
	}
}

// TestSkillsTargetsFor pins the mode → target table (RP-1, ADR-059): every
// repo-scoped mode writes the real copy into .claude/skills, all/codex/agents
// add the .agents/skills link, and the user-global prompts twin needs its
// own explicit mode.
func TestSkillsTargetsFor(t *testing.T) {
	cases := map[string][]string{
		"":              nil,
		"all":           {skillsTargetClaude, skillsTargetAgents},
		"claude":        {skillsTargetClaude},
		"codex":         {skillsTargetClaude, skillsTargetAgents},
		"agents":        {skillsTargetClaude, skillsTargetAgents},
		"codex-prompts": {skillsTargetCodexPrompts},
	}
	for mode, want := range cases {
		got := skillsTargetsFor(mode)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("skillsTargetsFor(%q) = %v, want %v", mode, got, want)
		}
	}
}

// TestAgentsVariant pins RP-2: the .agents/skills rendering carries only
// the Agent Skills spec fields, `name` equals the directory, the body (and
// the marker line) is untouched, and a file without frontmatter is refused.
func TestAgentsVariant(t *testing.T) {
	in := "---\nname: wrong-name\ndescription: >\n  two\n  lines\npaths: \"cmd/**\"\nhooks:\n  PostToolUse:\n    - matcher: x\nlicense: MIT\n---\n\n" + skillMarker + " -->\n\nbody `paths:` stays\n"
	got, err := agentsVariant("fathom-x", []byte(in))
	if err != nil {
		t.Fatal(err)
	}
	want := "---\nname: fathom-x\ndescription: >\n  two\n  lines\nlicense: MIT\n---\n\n" + skillMarker + " -->\n\nbody `paths:` stays\n"
	if string(got) != want {
		t.Errorf("agentsVariant:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	if _, err := agentsVariant("fathom-x", []byte("no frontmatter\n")); err == nil {
		t.Error("a file without frontmatter must be refused")
	}
	// No name at all: the directory name is supplied.
	got, err = agentsVariant("fathom-y", []byte("---\ndescription: d\n---\nb\n"))
	if err != nil || !strings.HasPrefix(string(got), "---\nname: fathom-y\ndescription: d\n---\n") {
		t.Errorf("missing name must be supplied from the dir: %q (%v)", got, err)
	}
}

func TestInstallSkills_All_WritesBothHarnesses(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := t.TempDir()

	var out bytes.Buffer
	if err := installSkills(nil, root, "all", &out); err != nil {
		t.Fatalf("installSkills: %v", err)
	}
	for _, name := range skillsdist.Names {
		if _, err := os.Stat(filepath.Join(root, ".claude", "skills", name, "SKILL.md")); err != nil {
			t.Errorf("claude skill %s not installed: %v", name, err)
		}
		if _, err := os.Stat(filepath.Join(root, ".agents", "skills", name, "SKILL.md")); err != nil {
			t.Errorf("agents skill %s not installed: %v", name, err)
		}
	}
	// "all" is the repo-scoped pair; the user-global prompts twin needs
	// its own explicit mode (RP-1).
	if _, err := os.Stat(filepath.Join(home, ".codex")); err == nil {
		t.Error("~/.codex written by --skills=all")
	}
}

func TestInstallSkills_Empty_NoOp(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	if err := installSkills(nil, root, "", &out); err != nil {
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
	if err := installSkills(nil, root, "claude", &out1); err != nil {
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
	if err := installSkills(nil, root, "claude", &out2); err != nil {
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
	err := installSkills(nil, root, "claude", &out)
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
// `--skills[=…]` syntax: a bare --skills (no `=value`) installs the
// repo-scoped pair (.claude/skills and .agents/skills), not zero — and
// never the user-global prompts twin.
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
	if _, err := os.Stat(filepath.Join(dir, ".agents", "skills", "fathom-plan", "SKILL.md")); err != nil {
		t.Errorf("agents skill not installed by bare --skills: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".codex", "prompts", "fathom-plan.md")); err == nil {
		t.Error("bare --skills wrote the user-global ~/.codex/prompts twin")
	}
}

// dirEntryNames returns the names of dir's entries (empty slice for an
// empty or absent dir) — for asserting a refused cmdInit wrote nothing.
func dirEntryNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("reading %s: %v", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// TestCmdInit_PositionalSkillsValue_Refused is the critic's first proven
// trap (D1): the natural spelling `fathom init --skills claude` parses bare
// --skills as "all" and leaves "claude" as a positional — before this
// guard, init silently discarded it and installed BOTH harnesses. Now the
// positional is refused loudly, with the fix spelled out, before any file
// is written.
func TestCmdInit_PositionalSkillsValue_Refused(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cwd := t.TempDir()
	t.Chdir(cwd)

	var out, errBuf bytes.Buffer
	code := cmdInit([]string{"--skills", "claude"}, &out, &errBuf)
	if code != 1 {
		t.Fatalf("exit = %d, want 1; stdout=%s stderr=%s", code, out.String(), errBuf.String())
	}
	if !strings.Contains(errBuf.String(), `unexpected argument "claude"`) ||
		!strings.Contains(errBuf.String(), "--skills=claude") {
		t.Errorf("stderr missing the refusal + suggestion: %s", errBuf.String())
	}
	// Refused BEFORE any writes: the CWD (init's default --path) is untouched.
	if names := dirEntryNames(t, cwd); len(names) != 0 {
		t.Errorf("cwd was written to despite refusal: %v", names)
	}
}

// TestCmdInit_PositionalStopsFlagParsing_Refused is the critic's worst-case
// trap (D1): `--skills claude --path X` — flag parsing stops at the
// positional "claude", so --path would be silently DROPPED and init would
// scaffold .fathomignore/settings/CLAUDE.md/AGENTS.md into the CWD instead
// of X. The positional guard refuses first: exit 1, X untouched, CWD
// untouched.
func TestCmdInit_PositionalStopsFlagParsing_Refused(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cwd := t.TempDir()
	t.Chdir(cwd)
	target := t.TempDir()

	var out, errBuf bytes.Buffer
	code := cmdInit([]string{"--skills", "claude", "--path", target}, &out, &errBuf)
	if code != 1 {
		t.Fatalf("exit = %d, want 1; stdout=%s stderr=%s", code, out.String(), errBuf.String())
	}
	if !strings.Contains(errBuf.String(), `unexpected argument "claude"`) {
		t.Errorf("stderr missing the refusal: %s", errBuf.String())
	}
	if names := dirEntryNames(t, target); len(names) != 0 {
		t.Errorf("target dir was written to despite refusal: %v", names)
	}
	if names := dirEntryNames(t, cwd); len(names) != 0 {
		t.Errorf("cwd was written to despite refusal (the exact pre-fix failure): %v", names)
	}
}

// TestCmdInit_SkillsEmptyValue_Refused: `--skills=` (explicit empty value)
// is refused, never silently treated as "all" — the D1 minor.
func TestCmdInit_SkillsEmptyValue_Refused(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()

	var out, errBuf bytes.Buffer
	code := cmdInit([]string{"--path", dir, "--skills="}, &out, &errBuf)
	if code != 1 {
		t.Fatalf("exit = %d, want 1; stdout=%s stderr=%s", code, out.String(), errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "empty --skills=") {
		t.Errorf("stderr missing the empty-value refusal: %s", errBuf.String())
	}
	if names := dirEntryNames(t, dir); len(names) != 0 {
		t.Errorf("target dir was written to despite refusal: %v", names)
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

// TestSkillsAgentsRendering pins IO-24 (vision 16 R2): --skills=agents
// writes the spec-conformant SKILL.md set to the shared .agents/skills
// discovery path Codex and Gemini both walk — and the pack now counts
// eleven (including the remote-job contract, ADR-078).
func TestSkillsAgentsRendering(t *testing.T) {
	root := t.TempDir()
	var out strings.Builder
	if err := installSkills(nil, root, "agents", &out); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(root, ".agents", "skills"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 11 {
		t.Fatalf("got %d skills, want 11 (the pack including fathom-offload)", len(entries))
	}
	data, err := os.ReadFile(filepath.Join(root, ".agents", "skills", "fathom-rewire", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "name: fathom-rewire") {
		t.Error("frontmatter name must match the dir (agentskills.io conformance)")
	}
	if !strings.Contains(string(data), "fathom:owned") {
		t.Error("marker missing — re-render safety depends on it")
	}
}

// TestResolveSkillsModeAgents pins IO-24's mode plumbing.
func TestResolveSkillsModeAgents(t *testing.T) {
	got, err := resolveSkillsMode("agents", true)
	if err != nil || got != "agents" {
		t.Fatalf("resolveSkillsMode(agents) = %q, %v", got, err)
	}
	if _, err := resolveSkillsMode("bogus", true); err == nil {
		t.Fatal("invalid mode must refuse")
	}
}
