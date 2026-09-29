package fusion

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
)

var gitHooksBasenames = []string{
	"lefthook.yml", "lefthook.yaml", ".lefthook.yml", ".lefthook.yaml",
	"lefthook-local.yml", "lefthook-local.yaml", ".lefthook-local.yml", ".lefthook-local.yaml",
	".pre-commit-config.yaml", ".pre-commit-hooks.yaml",
}

// TestGitHooksClassification (LB-Q-git-hooks-config, design §5): the manifest
// claims lefthook's eight YAML spellings, pre-commit's config and a hook
// repository's manifest by basename, at any depth. lefthook's JSON and TOML
// configs, a .yml pre-commit config (pre-commit reads the .yaml name only), a
// backup and other YAML beside them are not claimed.
func TestGitHooksClassification(t *testing.T) {
	rules := dialectRules()
	for _, name := range gitHooksBasenames {
		for _, p := range []string{name, "tools/" + name, "a/b/" + name} {
			v := connector.ClassifyDialect(rules, p, nil)
			if v.Dialect != "git-hooks-config" || v.By != connector.ClassifiedByBasename {
				t.Errorf("ClassifyDialect(%q) = {%q %q}, want git-hooks-config by basename", p, v.Dialect, v.By)
			}
			if lang := connector.LanguageForPath(p); lang != "git-hooks-config" {
				t.Errorf("LanguageForPath(%q) = %q, want git-hooks-config", p, lang)
			}
		}
	}
	for _, p := range []string{"lefthook.json", "lefthook.toml", ".pre-commit-config.yml", "lefthook.yml.bak", "hooks/lefthook-extra.yml", "pre-commit-config.yaml"} {
		if v := connector.ClassifyDialect(rules, p, nil); v.Dialect == "git-hooks-config" {
			t.Errorf("ClassifyDialect(%q) = git-hooks-config; only the ten basenames are claimed", p)
		}
	}
	if got := connector.TypedEdgeProducer("git-hooks-config"); got != "syntax" {
		t.Errorf("TypedEdgeProducer(git-hooks-config) = %q, want syntax", got)
	}
}

// TestGitHooksDialectIsWhatFusionDispatches: every one of the ten basenames
// reaches the doc-engine dialect (it carries a DocResult) through fusion.New's
// dispatch, in any directory; a lefthook config under another name stays
// unclassified YAML.
func TestGitHooksDialectIsWhatFusionDispatches(t *testing.T) {
	p := New(connector.NewLocalConnector(), store.NewMemoryStore(), Config{})
	t.Cleanup(p.Close)
	for _, name := range gitHooksBasenames {
		for _, rel := range []string{name, "tools/" + name} {
			e, _, ok, unclassified := p.resolveExtractor(rel, []byte("pre-commit:\n  commands:\n    x:\n      run: y\n"), buildSiblingIndex(nil))
			if !ok || unclassified {
				t.Fatalf("%s: ok=%v unclassified=%v; want an extractor", rel, ok, unclassified)
			}
			if _, isDoc := e.(extractor.DocResulter); !isDoc {
				t.Errorf("%s dispatches to %T, want the doc-engine dialect", rel, e)
			}
		}
	}
	if _, _, ok, unclassified := p.resolveExtractor("hooks/lefthook-extra.yml", []byte("pre-commit:\n  commands:\n    x:\n      run: y\n"), buildSiblingIndex(nil)); ok || !unclassified {
		t.Errorf("hooks/lefthook-extra.yml: ok=%v unclassified=%v; a config under another name stays unclassified", ok, unclassified)
	}
}

func gitHooksRepoCorpus() map[string]string {
	return map[string]string{
		"lefthook.yml": "min_version: 1.5.0\ncolors:\n  cyan: 14\npre-commit:\n  parallel: true\n  commands:\n    lint:\n      run: golangci-lint run\n" +
			"  scripts:\n    \"check.sh\":\n      runner: bash\npre-push:\n  jobs:\n    - name: test\n      run: go test ./...\n",
		"lefthook-local.yml": "pre-commit:\n  commands:\n    lint:\n      skip: true\n",
		"web/.lefthook.yaml": "commit-msg:\n  commands:\n    commitlint:\n      run: npx commitlint --edit\n",
		".pre-commit-config.yaml": "repos:\n  - repo: https://github.com/pre-commit/pre-commit-hooks\n    rev: v6.0.0\n    hooks:\n      - id: trailing-whitespace\n" +
			"  - repo: local\n    hooks:\n      - id: mypy\n        name: mypy\n        entry: mypy\n        language: system\n",
		".pre-commit-hooks.yaml":   "- id: check-json\n  name: check json\n  entry: check-json\n  language: python\n",
		"hooks/lefthook-extra.yml": "pre-commit:\n  commands:\n    not-read:\n      run: x\n",
		"lefthook.json":            "{\"pre-commit\": {\"commands\": {\"not-read\": {\"run\": \"x\"}}}}\n",
	}
}

// TestIngest_GitHooksRepo (the product-reach proof): an ingest extracts every
// lefthook config, pre-commit config and hook manifest through the
// git-hooks-config dialect — hooks, their commands, scripts and named jobs,
// and pre-commit's hooks and hook definitions by id — and leaves a config
// under another name, and lefthook's JSON form, alone.
func TestIngest_GitHooksRepo(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, gitHooksRepoCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "gh", Path: dir}); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "gh", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, o := range obs {
		switch o.SourceKind {
		case core.KindLefthookHook, "LefthookCommand", "LefthookScript", "LefthookJob", core.KindPreCommitHook, core.KindPreCommitHookDef:
			got[o.SourceRef+"|"+string(o.SourceKind)+"|"+o.Symbol] = true
		}
		if o.Symbol == "not-read" {
			t.Errorf("%s|%s: a file that is not one of the ten basenames was extracted as %s", o.SourceRef, o.Symbol, o.SourceKind)
		}
	}
	want := []string{
		"lefthook.yml|LefthookHook|pre-commit",
		"lefthook.yml|LefthookCommand|lint",
		"lefthook.yml|LefthookScript|check.sh",
		"lefthook.yml|LefthookHook|pre-push",
		"lefthook.yml|LefthookJob|test",
		"lefthook-local.yml|LefthookHook|pre-commit",
		"lefthook-local.yml|LefthookCommand|lint",
		"web/.lefthook.yaml|LefthookHook|commit-msg",
		"web/.lefthook.yaml|LefthookCommand|commitlint",
		".pre-commit-config.yaml|PreCommitHook|trailing-whitespace",
		".pre-commit-config.yaml|PreCommitHook|mypy",
		".pre-commit-hooks.yaml|PreCommitHookDef|check-json",
	}
	var have []string
	for k := range got {
		have = append(have, k)
	}
	sort.Strings(have)
	sort.Strings(want)
	if !reflect.DeepEqual(have, want) {
		t.Fatalf("observations\n got: %q\nwant: %q", have, want)
	}
}

// TestReingestFile_GitHooksRepo: the one-file refresh reads a lefthook config
// through the same dialect an ingest does (the basename decides), so an edit
// that adds a command shows as a LefthookCommand.
func TestReingestFile_GitHooksRepo(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, gitHooksRepoCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "gh", Path: dir}); err != nil {
		t.Fatal(err)
	}
	rel := "web/.lefthook.yaml"
	edited := "commit-msg:\n  commands:\n    commitlint:\n      run: npx commitlint --edit\n    spell:\n      run: cspell\n"
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ReingestFile(ctx, st, "gh", dir, rel); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "gh", "")
	if err != nil {
		t.Fatal(err)
	}
	var cmds []string
	for _, o := range obs {
		if o.SourceRef == rel && o.SourceKind == "LefthookCommand" {
			cmds = append(cmds, o.Symbol)
		}
	}
	sort.Strings(cmds)
	if want := []string{"commitlint", "spell"}; !reflect.DeepEqual(cmds, want) {
		t.Fatalf("commands of %s after ReingestFile = %v, want %v", rel, cmds, want)
	}
}
