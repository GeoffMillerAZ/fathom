package readout

// repo_test.go — RO-33 (docs/plans/readouts.md, ADR-071 decision 11): a
// repository's .fathom/readouts.toml adds, never drops, and stays in observe
// until the user lists its sha256.

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"
)

const repoAddRule = `schema = 1

[[rule]]
id = "repo-build"
version = 1
about = "buildtool: count compile lines and add the changed files"
match.argv0 = ["buildtool"]

[[rule.stage]]
kind = "extract"
pattern = '^cc '
record = "compile"
template = "{count} compiled"

[[rule.stage]]
kind = "join"
join = "changed_files"

[[rule.test]]
input = "cc a\nok\n"
expect = "cc a\nok\n[readout repo-build@1 observe: 2→2 lines; nothing dropped; 1 compiled, 0 dropped; join changed_files: not computed]\n"
`

const repoDropRule = `
[[rule]]
id = "repo-drop"
version = 1
about = "buildtool: drops lines, which a repository may not"
match.argv0 = ["buildtool"]
recovery = "repeat"

[[rule.stage]]
kind = "drop"
pattern = '^cc '

[[rule.test]]
input = "cc a\nok\n"
expect = "ok\n[readout repo-drop@1 observe: 2→1 lines; dropped 1 (plain 1); raw: repeat the command]\n"

[[rule.test]]
input = "cc error\n"
trip = true
`

func repoSHA(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func repoRender(t *testing.T, rs *RuleSet, mode string) Readout {
	t.Helper()
	return Render(rs, Input{
		Command: lexed("buildtool all"),
		Stdout:  "cc a\nok\n",
		Mode:    mode,
		Joins:   map[string][]string{"changed_files": {"changed: a.c"}},
	})
}

func repoLoad(t *testing.T, repoFile, userFile string, trust []string) (*RuleSet, string) {
	t.Helper()
	repo, home := t.TempDir(), t.TempDir()
	files := map[string]string{}
	if repoFile != "" {
		files[filepath.Join(".fathom", "readouts.toml")] = repoFile
	}
	writeFiles(t, repo, files)
	if userFile != "" {
		writeFiles(t, home, map[string]string{UserFile: userFile})
	}
	rs, err := LoadRules(home)
	if err != nil {
		t.Fatal(err)
	}
	LoadRepoRules(rs, repo, trust)
	return rs, repo
}

func TestRepoRuleCannotDropAndStaysObserveUntilTrusted(t *testing.T) {
	t.Run("no file is silent", func(t *testing.T) {
		rs, _ := repoLoad(t, "", "", nil)
		if rs.Repo != nil {
			t.Errorf("Repo = %+v, want nil", rs.Repo)
		}
		for _, rf := range rs.Refused {
			if strings.Contains(rf.Source, "readouts.toml") {
				t.Errorf("refusal for an absent file: %+v", rf)
			}
		}
	})

	t.Run("untrusted adds a line only in observe", func(t *testing.T) {
		rs, _ := repoLoad(t, repoAddRule, "", nil)
		if rs.Repo == nil || rs.Repo.Trusted || rs.Repo.SHA != repoSHA(repoAddRule) {
			t.Fatalf("Repo = %+v, want untrusted with the file's sha256", rs.Repo)
		}
		if len(rs.Refused) != 0 {
			t.Fatalf("refused = %+v", rs.Refused)
		}
		ro := repoRender(t, rs, ModeRich)
		if ro.Rule != "repo-build@1" || ro.Mode != ModeObserve {
			t.Errorf("rule %q mode %q, want repo-build@1 in observe", ro.Rule, ro.Mode)
		}
	})

	t.Run("trusted emits under rich", func(t *testing.T) {
		rs, _ := repoLoad(t, repoAddRule, "", []string{repoSHA(repoAddRule)})
		if rs.Repo == nil || !rs.Repo.Trusted {
			t.Fatalf("Repo = %+v, want trusted", rs.Repo)
		}
		ro := repoRender(t, rs, ModeRich)
		if ro.Raw || ro.Mode != ModeRich || !strings.Contains(ro.Stdout, "changed: a.c") {
			t.Errorf("readout = raw %v mode %q stdout %q, want the added line under rich", ro.Raw, ro.Mode, ro.Stdout)
		}
	})

	t.Run("a drop rule is refused and named, the rest loads", func(t *testing.T) {
		file := repoAddRule + repoDropRule
		rs, _ := repoLoad(t, file, "", []string{repoSHA(file)})
		var named bool
		for _, rf := range rs.Refused {
			if rf.ID == "repo-drop" && strings.HasSuffix(rf.Source, filepath.Join(".fathom", "readouts.toml")) && strings.Contains(rf.Reason, "add") {
				named = true
			}
		}
		if !named {
			t.Errorf("refused = %+v, want repo-drop named with its file", rs.Refused)
		}
		if ro := repoRender(t, rs, ModeRich); ro.Rule != "repo-build@1" || ro.Mode != ModeRich {
			t.Errorf("rule %q mode %q, want the add rule served", ro.Rule, ro.Mode)
		}
	})

	t.Run("an edited file reverts to observe and is named", func(t *testing.T) {
		rs, _ := repoLoad(t, repoAddRule+"# edited\n", "", []string{repoSHA(repoAddRule)})
		if rs.Repo == nil || rs.Repo.Trusted || rs.Repo.SHA != repoSHA(repoAddRule+"# edited\n") {
			t.Fatalf("Repo = %+v, want untrusted at the new sha256", rs.Repo)
		}
		if ro := repoRender(t, rs, ModeRich); ro.Mode != ModeObserve {
			t.Errorf("mode %q, want observe", ro.Mode)
		}
	})

	t.Run("user scope wins every conflict", func(t *testing.T) {
		user := strings.ReplaceAll(repoAddRule, "repo-build", "user-build")
		rs, _ := repoLoad(t, repoAddRule, user, []string{repoSHA(repoAddRule)})
		if ro := repoRender(t, rs, ModeRich); ro.Rule != "user-build@1" {
			t.Errorf("rule %q, want the user's rule over the repository's", ro.Rule)
		}
		// Same id: the user's rule stays, the repository's is refused by name.
		same := strings.ReplaceAll(repoAddRule, "repo-build", "user-build")
		rs, _ = repoLoad(t, same, user, []string{repoSHA(same)})
		var named bool
		for _, rf := range rs.Refused {
			if rf.ID == "user-build" && strings.Contains(rf.Reason, "user") {
				named = true
			}
		}
		if !named {
			t.Errorf("refused = %+v, want the shadowed id named", rs.Refused)
		}
	})
}
