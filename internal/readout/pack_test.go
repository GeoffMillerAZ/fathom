package readout

// pack_test.go — RO-34 (docs/plans/readouts.md): a tech pack's `readouts`
// key adds, never drops, and is served only with a passing fixture.

import (
	"path/filepath"
	"strings"
	"testing"
)

const packHeader = `pack: {name: "buildkit", version: 2, about: "buildtool kit"}
`

const packAddRule = `readouts: [{
	id:      "pack-build"
	version: 1
	about:   "buildtool: count compile lines and add the changed files"
	match: argv0: ["buildtool"]
	stage: [
		{kind: "extract", pattern: "^cc ", record: "compile", template: "{count} compiled"},
		{kind: "join", join: "changed_files"},
	]
	test: [{
		input:  "cc a\nok\n"
		expect: "cc a\nok\n[readout pack-build@1 observe: 2→2 lines; nothing dropped; 1 compiled, 0 dropped; join changed_files: not computed]\n"
	}]
}`

const packDropRule = `, {
	id:       "pack-drop"
	version:  1
	about:    "buildtool: drops lines, which a pack may not"
	match: argv0: ["buildtool"]
	recovery: "repeat"
	stage: [{kind: "drop", pattern: "^cc "}]
	test: [{
		input:  "cc a\nok\n"
		expect: "ok\n[readout pack-drop@1 observe: 2→1 lines; dropped 1 (plain 1); raw: repeat the command]\n"
	}, {input: "cc error\n", trip: true}]
}`

// packRepo writes a repository that pins one kit by a local replace and
// returns its root. body is the kit's pack.cue.
func packRepo(t *testing.T, body string, repoFile string) string {
	t.Helper()
	base := t.TempDir()
	t.Setenv("GOMODCACHE", filepath.Join(base, "gomodcache"))
	t.Setenv("GOPATH", filepath.Join(base, "gopath-unused"))
	t.Setenv("CUE_CACHE_DIR", filepath.Join(base, "cuecache"))
	t.Setenv("GOPROXY", "off")
	t.Setenv("PATH", filepath.Join(base, "empty-bin"))
	repo := filepath.Join(base, "repo")
	files := map[string]string{
		"go.mod": "module example.test/app\n\ngo 1.22\n\nrequire example.test/kit v1.0.0\n\nreplace example.test/kit => ../kit\n",
	}
	if repoFile != "" {
		files[filepath.Join(".fathom", "readouts.toml")] = repoFile
	}
	writeFiles(t, repo, files)
	writeFiles(t, filepath.Join(base, "kit"), map[string]string{
		"go.mod":               "module example.test/kit\n\ngo 1.22\n",
		"fathom-pack/pack.cue": body,
	})
	return repo
}

func packLoad(t *testing.T, body, repoFile, userFile string, trust []string) *RuleSet {
	t.Helper()
	repo := packRepo(t, body, repoFile)
	home := t.TempDir()
	if userFile != "" {
		writeFiles(t, home, map[string]string{UserFile: userFile})
	}
	rs, err := LoadRules(home)
	if err != nil {
		t.Fatal(err)
	}
	LoadRepoRules(rs, repo, trust)
	LoadPackRules(rs, repo)
	return rs
}

func TestPackReadoutRulesAreAddOnly(t *testing.T) {
	t.Run("no pack is silent", func(t *testing.T) {
		rs, _ := repoLoad(t, "", "", nil)
		LoadPackRules(rs, t.TempDir())
		if len(rs.Packs) != 0 || len(rs.Refused) != 0 {
			t.Errorf("packs %+v refused %+v, want none", rs.Packs, rs.Refused)
		}
	})

	t.Run("an add rule from a pinned pack serves under rich with no trust entry", func(t *testing.T) {
		rs := packLoad(t, packHeader+packAddRule+"]\n", "", "", nil)
		if len(rs.Refused) != 0 {
			t.Fatalf("refused = %+v", rs.Refused)
		}
		if len(rs.Packs) != 1 || rs.Packs[0].Pack != "buildkit@2" || strings.Join(rs.Packs[0].Served, ",") != "pack-build" {
			t.Fatalf("packs = %+v, want buildkit@2 serving pack-build", rs.Packs)
		}
		ro := repoRender(t, rs, ModeRich)
		if ro.Raw || ro.Rule != "pack-build@1" || ro.Mode != ModeRich || !strings.Contains(ro.Stdout, "changed: a.c") {
			t.Errorf("readout = raw %v rule %q mode %q stdout %q, want the pack rule served under rich", ro.Raw, ro.Rule, ro.Mode, ro.Stdout)
		}
	})

	t.Run("a drop rule is refused and named, the add rule still loads", func(t *testing.T) {
		rs := packLoad(t, packHeader+packAddRule+packDropRule+"]\n", "", "", nil)
		var named bool
		for _, rf := range rs.Refused {
			if rf.ID == "pack-drop" && strings.Contains(rf.Reason, "add") && strings.HasSuffix(rf.Source, "pack.cue") {
				named = true
			}
		}
		if !named {
			t.Errorf("refused = %+v, want pack-drop named", rs.Refused)
		}
		if ro := repoRender(t, rs, ModeRich); ro.Rule != "pack-build@1" {
			t.Errorf("rule %q, want the add rule", ro.Rule)
		}
		if len(rs.Packs) != 1 || len(rs.Packs[0].Withheld) != 1 || rs.Packs[0].Withheld[0].ID != "pack-drop" {
			t.Errorf("packs = %+v, want pack-drop withheld", rs.Packs)
		}
	})

	t.Run("an unfixtured readouts section is withheld and named", func(t *testing.T) {
		noTest := strings.Replace(packAddRule, "test: [{", "xtest: [{", 1)
		noTest = noTest[:strings.Index(noTest, "\txtest")] + "}"
		rs := packLoad(t, packHeader+noTest+"]\n", "", "", nil)
		if ro := repoRender(t, rs, ModeRich); !ro.Raw {
			t.Errorf("rule %q served, want raw: an unfixtured rule is never served", ro.Rule)
		}
		if len(rs.Packs) != 1 || len(rs.Packs[0].Served) != 0 || len(rs.Packs[0].Withheld) != 1 || !strings.Contains(rs.Packs[0].Withheld[0].Reason, "no fixture") {
			t.Errorf("packs = %+v, want pack-build withheld for no fixtures", rs.Packs)
		}
	})

	t.Run("a failing fixture withholds the rule", func(t *testing.T) {
		bad := strings.Replace(packAddRule, "nothing dropped", "something else", 1)
		rs := packLoad(t, packHeader+bad+"]\n", "", "", nil)
		if ro := repoRender(t, rs, ModeRich); !ro.Raw {
			t.Errorf("rule %q served with a failing fixture", ro.Rule)
		}
		if len(rs.Packs) != 1 || len(rs.Packs[0].Withheld) != 1 || !strings.Contains(rs.Packs[0].Withheld[0].Reason, "fixture 1") {
			t.Errorf("packs = %+v, want the failing fixture named", rs.Packs)
		}
	})

	t.Run("repository and user scope outrank a pack", func(t *testing.T) {
		repoRule := repoAddRule // id repo-build, same match
		rs := packLoad(t, packHeader+packAddRule+"]\n", repoRule, "", []string{repoSHA(repoRule)})
		if ro := repoRender(t, rs, ModeRich); ro.Rule != "repo-build@1" {
			t.Errorf("rule %q, want the repository's rule over the pack's", ro.Rule)
		}
		user := strings.ReplaceAll(repoAddRule, "repo-build", "user-build")
		rs = packLoad(t, packHeader+packAddRule+"]\n", repoRule, user, []string{repoSHA(repoRule)})
		if ro := repoRender(t, rs, ModeRich); ro.Rule != "user-build@1" {
			t.Errorf("rule %q, want the user's rule", ro.Rule)
		}
		same := strings.ReplaceAll(packAddRule, "pack-build", "repo-build")
		rs = packLoad(t, packHeader+same+"]\n", repoRule, "", []string{repoSHA(repoRule)})
		var named bool
		for _, rf := range rs.Refused {
			if rf.ID == "repo-build" && strings.HasSuffix(rf.Source, "pack.cue") {
				named = true
			}
		}
		if !named {
			t.Errorf("refused = %+v, want the pack's shadowed id named", rs.Refused)
		}
	})
}
