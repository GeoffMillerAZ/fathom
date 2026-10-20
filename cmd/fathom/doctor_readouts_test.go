package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDoctorReadoutsRowNamesRefusedRulesAndAnotherRewriter pins RO-18: the
// readouts row reads the mode and roots, names each refused rule with its
// first failing fixture, names the harnesses without the view, and names
// another Bash output rewriter by its command's first word only. It reads
// ok or advisory, never FAIL.
func TestDoctorReadoutsRowNamesRefusedRulesAndAnotherRewriter(t *testing.T) {
	fh := readoutHookEnv(t, "[readouts]\nmode = \"lean\"\nroots = [\"/scratch/one\"]\n")
	home := os.Getenv("HOME")
	bad := strings.Replace(readoutTestRule, `id = "ls-head"`, `id = "ls-bad"`, 1)
	bad = strings.Replace(bad, "a.go\\nb.go\\n[readout ls-head@1 observe", "a.go\\n[readout ls-head@1 observe", 1)
	if err := os.WriteFile(filepath.Join(fh, "readouts.toml"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()

	clean := checkReadouts(root)
	if clean.Name != "readouts" || clean.Status == doctorFail {
		t.Fatalf("row = %+v, want name readouts and never fail", clean)
	}
	for _, want := range []string{"lean", "/scratch/one", "ls-bad", "fixture 1", "Claude Code", "Codex"} {
		if !strings.Contains(clean.Reason, want) {
			t.Errorf("reason lacks %q:\n%s", want, clean.Reason)
		}
	}
	if clean.Status != doctorWarn {
		t.Errorf("a refused rule is advisory, got %s", clean.Status)
	}
	if strings.Contains(clean.Reason, "another Bash rewriter") {
		t.Errorf("no rewriter wired yet:\n%s", clean.Reason)
	}

	settings := `{"hooks":{
	  "PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"` + home + `/bin/squeeze --level 9 TOKEN=hunter2"}]}],
	  "PostToolUse":[{"matcher":"Edit","hooks":[{"type":"command","command":"editfmt"}]},
	                 {"matcher":"","hooks":[{"type":"command","command":"FOO=bar slimmer post"}]},
	                 {"matcher":"Bash","hooks":[{"type":"command","command":"fathom readout-hook --event post-tool"}]}]}}`
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}
	got := checkReadouts(root)
	if got.Status == doctorFail {
		t.Fatalf("never fails: %+v", got)
	}
	for _, want := range []string{"another Bash rewriter", "squeeze", "slimmer"} {
		if !strings.Contains(got.Reason, want) {
			t.Errorf("reason lacks %q:\n%s", want, got.Reason)
		}
	}
	for _, leak := range []string{home, "hunter2", "--level", "editfmt", "FOO=bar", "post"} {
		if strings.Contains(got.Reason, leak) {
			t.Errorf("reason leaks %q:\n%s", leak, got.Reason)
		}
	}

	// off and no rules refused: ok.
	readoutHookEnv(t, "")
	if off := checkReadouts(t.TempDir()); off.Status != doctorPass || !strings.Contains(off.Reason, "off") {
		t.Errorf("off row = %+v, want pass naming off", off)
	}
}

const repoReadoutsRule = `schema = 1

[[rule]]
id = "repo-build"
version = 1
about = "buildtool: add the changed files"
match.argv0 = ["buildtool"]

[[rule.stage]]
kind = "join"
join = "changed_files"

[[rule.test]]
input = "ok\n"
expect = "ok\n[readout repo-build@1 observe: 1→1 lines; nothing dropped; join changed_files: not computed]\n"
`

// TestDoctorReadoutsNamesAnUntrustedRepoRuleFile pins RO-33's doctor line:
// a repository's .fathom/readouts.toml whose sha256 is not under
// `[readouts] trust` is named with its path, current hash and "not
// trusted"; listing the hash silences it; editing the file names it again.
func TestDoctorReadoutsNamesAnUntrustedRepoRuleFile(t *testing.T) {
	fh := readoutHookEnv(t, "[readouts]\nmode = \"rich\"\n")
	root := t.TempDir()
	file := filepath.Join(root, ".fathom", "readouts.toml")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(body string) string {
		t.Helper()
		if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256([]byte(body))
		return hex.EncodeToString(sum[:])
	}

	if got := checkReadouts(root); strings.Contains(got.Reason, "not trusted") {
		t.Errorf("no file is silent:\n%s", got.Reason)
	}

	sha := write(repoReadoutsRule)
	got := checkReadouts(root)
	for _, want := range []string{file, sha, "not trusted"} {
		if !strings.Contains(got.Reason, want) {
			t.Errorf("untrusted reason lacks %q:\n%s", want, got.Reason)
		}
	}
	if got.Status != doctorWarn {
		t.Errorf("an untrusted file is advisory, got %s", got.Status)
	}

	cfg := "[readouts]\nmode = \"rich\"\ntrust = [\"" + sha + "\"]\n"
	if err := os.WriteFile(filepath.Join(fh, "config.toml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := checkReadouts(root); strings.Contains(got.Reason, "not trusted") {
		t.Errorf("a trusted file is not named:\n%s", got.Reason)
	}

	sha2 := write(repoReadoutsRule + "# edited\n")
	got = checkReadouts(root)
	if !strings.Contains(got.Reason, sha2) || !strings.Contains(got.Reason, "not trusted") {
		t.Errorf("an edited file is named at its new hash %s:\n%s", sha2, got.Reason)
	}
}

// TestDoctorReadoutsNamesEachPacksServedAndWithheldRules pins RO-34's doctor
// line: a pinned tech pack's readout rules are named as served, and an
// unfixtured one as withheld.
func TestDoctorReadoutsNamesEachPacksServedAndWithheldRules(t *testing.T) {
	readoutHookEnv(t, "[readouts]\nmode = \"rich\"\n")
	base := t.TempDir()
	t.Setenv("GOMODCACHE", filepath.Join(base, "gomodcache"))
	t.Setenv("GOPROXY", "off")
	t.Setenv("PATH", filepath.Join(base, "empty-bin"))
	root := filepath.Join(base, "repo")
	rule := func(id, tests string) string {
		return "{id: \"" + id + "\", version: 1, about: \"buildtool: count\", match: argv0: [\"buildtool\"], stage: [{kind: \"extract\", pattern: \"^cc \", record: \"compile\", template: \"{count} compiled\"}]" + tests + "}"
	}
	fixture := ", test: [{input: \"cc a\\n\", expect: \"cc a\\n[readout ok-rule@1 observe: 1→1 lines; nothing dropped; 1 compiled, 0 dropped]\\n\"}]"
	files := map[string]string{
		filepath.Join(root, "go.mod"):                         "module example.test/app\n\ngo 1.22\n\nrequire example.test/kit v1.0.0\n\nreplace example.test/kit => ../kit\n",
		filepath.Join(base, "kit", "go.mod"):                  "module example.test/kit\n\ngo 1.22\n",
		filepath.Join(base, "kit", "fathom-pack", "pack.cue"): "pack: {name: \"buildkit\", version: 2, about: \"kit\"}\nreadouts: [" + rule("ok-rule", fixture) + ", " + rule("bare-rule", "") + "]\n",
	}
	for p, body := range files {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := checkReadouts(root)
	for _, want := range []string{"pack buildkit@2 readouts served ok-rule", "withheld bare-rule"} {
		if !strings.Contains(got.Reason, want) {
			t.Errorf("reason lacks %q:\n%s", want, got.Reason)
		}
	}
	if got.Status != doctorWarn {
		t.Errorf("a withheld rule is advisory, got %s", got.Status)
	}
}
