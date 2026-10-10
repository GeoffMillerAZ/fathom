package config

// scope_test.go — SEC-2 (CF-14): every key the loader accepts has a scope
// class. The accepted key set is derived from the embedded #Config schema
// (the same value readLayer validates every layer against), never from a
// hand list, so a new schema field fails here until it is classified, and
// a classified field the schema dropped fails here too.

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// schemaFieldKeys walks #Config and returns every leaf field's dotted key in
// the registry's spelling: "*" for a pattern-constrained map key, "[]" for
// an element of an array of tables. A field whose value is an error at
// every scope (decide.api_key, refused by name) is not a field anyone can
// set, and is left out.
func schemaFieldKeys(t *testing.T) []string {
	t.Helper()
	v := cuecontext.New().CompileBytes(configSchema).LookupPath(cue.ParsePath("#Config"))
	if err := v.Err(); err != nil {
		t.Fatal(err)
	}
	var keys []string
	var walk func(v cue.Value, p string)
	walk = func(v cue.Value, p string) {
		if v.IncompleteKind() == cue.StructKind {
			it, err := v.Fields(cue.Optional(true))
			if err != nil {
				t.Fatalf("%s: %v", p, err)
			}
			n := 0
			for it.Next() {
				n++
				walk(it.Value(), joinKey(p, it.Selector().Unquoted()))
			}
			if pv := v.LookupPath(cue.MakePath(cue.AnyString)); pv.Exists() {
				n++
				walk(pv, joinKey(p, "*"))
			}
			if n > 0 {
				return
			}
		}
		if v.IncompleteKind() == cue.ListKind {
			if ev := v.LookupPath(cue.MakePath(cue.AnyIndex)); ev.Exists() && ev.IncompleteKind() == cue.StructKind {
				walk(ev, p+"[]")
				return
			}
		}
		if v.Err() != nil {
			return // refused at every scope (an error() field)
		}
		keys = append(keys, p)
	}
	walk(v, "")
	sort.Strings(keys)
	return keys
}

func TestEveryConfigFieldHasAScopeClass(t *testing.T) {
	keys := schemaFieldKeys(t)
	if len(keys) < 50 {
		t.Fatalf("schema walk found %d keys, want the whole #Config: %v", len(keys), keys)
	}
	for _, k := range keys {
		fs, ok := ScopeOf(k)
		if !ok {
			t.Errorf("config field %s has no scope class: add it to fieldScopes (internal/config/scope.go) as behaviour, repo-path or user-only", k)
			continue
		}
		switch fs.Class {
		case ScopeBehaviour, ScopeRepoPath, ScopeUserOnly:
		default:
			t.Errorf("%s: class %q is not one of the three", k, fs.Class)
		}
		if strings.TrimSpace(fs.Why) == "" {
			t.Errorf("%s: no reason recorded", fs.Key)
		}
	}
	for _, fs := range FieldScopes() {
		covered := false
		for _, k := range keys {
			if k == fs.Key || (strings.HasSuffix(fs.Key, ".**") && strings.HasPrefix(k, strings.TrimSuffix(fs.Key, "**"))) {
				covered = true
				break
			}
		}
		if !covered {
			t.Errorf("fieldScopes classifies %s, which the schema no longer accepts: remove it", fs.Key)
		}
	}
}

// TestScopeRule_UserOnlyCoversEveryExecutableCredentialAndMachinePath pins
// the owner's rule on the fields it names: an executable, a credential, a
// key host, the daemon's own paths and switches, and the tenancy budget are
// user-only.
func TestScopeRule_UserOnlyCoversEveryExecutableCredentialAndMachinePath(t *testing.T) {
	for _, k := range []string{
		"profiles.*.bin", "profiles.*.endpoint", "profiles.*.api_key_cmd", "profiles.*.api_key_env",
		"decide.api_key_cmd", "decide.api_key_file", "decide.api_key_env",
		"daemon.store", "daemon.addr", "daemon.call_log", "daemon.auto_refresh", "daemon.memory_limit",
		"tenancy.budget", "tenancy.host.min_swap_volume_free",
	} {
		if RepoMaySet(k) {
			t.Errorf("%s: a repository may set it, want user-only", k)
		}
	}
	for _, k := range []string{"tests.results_dir", "tests.artifacts_dir", "findings.outbox", "connector.include", "work.ci_summaries[].path"} {
		if fs, _ := ScopeOf(k); fs.Class != ScopeRepoPath {
			t.Errorf("%s: class %q, want repo-path", k, fs.Class)
		}
	}
	for _, k := range []string{"hooks.test_run", "tests.commands", "decide.provider", "daemon.state_dir"} {
		if !RepoMaySet(k) {
			t.Errorf("%s: want a repository to be able to set it", k)
		}
	}
	if RepoMaySet("no.such.key") {
		t.Error("an unknown key must not be settable from a repository")
	}
}

func TestRepoPath_RefusesEscapesByName(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "workspace", "in"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "workspace", "out")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "workspace", "in"), filepath.Join(root, "inlink")); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		rel  string
		want error
	}{
		{"", ErrRepoPathEmpty},
		{filepath.Join(outside, "x"), ErrRepoPathAbsolute},
		{"/etc/passwd", ErrRepoPathAbsolute},
		{"..", ErrRepoPathEscapes},
		{"../sibling/x", ErrRepoPathEscapes},
		{"workspace/../../x", ErrRepoPathEscapes},
		{"workspace/out", ErrRepoPathSymlink},
		{"workspace/out/deeper/not-yet", ErrRepoPathSymlink},
	} {
		_, err := RepoPath(root, c.rel)
		if !errors.Is(err, c.want) {
			t.Errorf("RepoPath(%q) = %v, want %v", c.rel, err, c.want)
		}
		var rpe *RepoPathError
		if err != nil && !errors.As(err, &rpe) {
			t.Errorf("RepoPath(%q): error %T is not a *RepoPathError", c.rel, err)
		}
	}
	for _, rel := range []string{"workspace/in", "workspace/in/not-yet/file.json", "inlink/x", "./a/../b", "coverage"} {
		got, err := RepoPath(root, rel)
		if err != nil {
			t.Errorf("RepoPath(%q) = %v, want it inside the repository", rel, err)
			continue
		}
		if want := filepath.Join(root, filepath.Clean(rel)); got != want {
			t.Errorf("RepoPath(%q) = %q, want %q", rel, got, want)
		}
	}
}

// TestRepoPath_RelativeRoot: `fathom ingest --path .` hands a relative
// root; it is still the repository.
func TestRepoPath_RelativeRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "results"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	got, err := RepoPath(".", "results")
	if err != nil || got != "results" {
		t.Fatalf("RepoPath(\".\", \"results\") = %q, %v; want \"results\"", got, err)
	}
	if _, err := RepoPath(".", "../x"); !errors.Is(err, ErrRepoPathEscapes) {
		t.Fatalf("RepoPath(\".\", \"../x\") = %v, want ErrRepoPathEscapes", err)
	}
}

// TestRepoPath_RootBehindASymlink: a repository reached through a symlink
// (~/code -> /Volumes/...) is still itself.
func TestRepoPath_RootBehindASymlink(t *testing.T) {
	real := t.TempDir()
	if err := os.MkdirAll(filepath.Join(real, "workspace"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "repo")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if _, err := RepoPath(link, "workspace/x"); err != nil {
		t.Fatalf("RepoPath through a linked root: %v", err)
	}
}

// TestRefusedRepoPaths_CoversEveryRepoPathField: every repo-path field in
// the registry is checked, so a new one cannot be listed nowhere.
func TestRefusedRepoPaths_CoversEveryRepoPathField(t *testing.T) {
	cfg := Config{
		Connector: Connector{Include: []string{"coverage", "/abs"}},
		Findings:  Findings{Outbox: "../x.json"},
		Tests:     Tests{ResultsDir: "/r", ArtifactsDir: "/a"},
		Work:      Work{CISummaries: []CISummary{{Path: "ci/ok.xml"}, {Path: "/ci.xml"}}},
	}
	got := map[string]bool{}
	for _, r := range RefusedRepoPaths(t.TempDir(), cfg) {
		// connector.include[1] is the list field connector.include;
		// work.ci_summaries[1].path is work.ci_summaries[].path.
		k := regexp.MustCompile(`\[\d+\]$`).ReplaceAllString(r.Field, "")
		got[regexp.MustCompile(`\[\d+\]`).ReplaceAllString(k, "[]")] = true
		if r.Err == nil {
			t.Errorf("%s: no error", r.Field)
		}
	}
	for _, fs := range FieldScopes() {
		if fs.Class == ScopeRepoPath && !got[fs.Key] {
			t.Errorf("repo-path field %s is not checked by RefusedRepoPaths", fs.Key)
		}
	}
	if len(got) != 5 {
		t.Errorf("refused = %v, want the five escaping values only", got)
	}
}
