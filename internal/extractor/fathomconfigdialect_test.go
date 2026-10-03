package extractor

// fathomconfigdialect_test.go — goldens for the fathom-config doc-engine
// dialect (LB-Q-fathom-config, wave fx item fmt): fathom's own
// .fathom/config.toml read as sections, keys, profiles and surface patterns,
// with the keys that name a profile as in-file references and the keys that
// name a repository path as cross-file references. The fixtures are the
// calibration subjects under calibration/oracles/fathom-config/fixtures,
// schema-valid against schema/config (ADR-011: reserved example names only).

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func loadFathomConfigExtractor(t *testing.T) *DocExtractor {
	t.Helper()
	x, err := NewDocDialectExtractor("fathom-config")
	if err != nil {
		t.Fatalf("NewDocDialectExtractor(fathom-config): %v", err)
	}
	return x
}

func fathomConfigFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "calibration", "oracles", "fathom-config", "fixtures", name, ".fathom", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestFathomConfigEntitiesAndReferences(t *testing.T) {
	x := loadFathomConfigExtractor(t)
	res, err := x.ExtractDoc("svc", ".fathom/config.toml", fathomConfigFixture(t, "profiles"))
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]string{}
	calls := map[string][]string{}
	for _, n := range res.Nodes {
		kinds[n.Name] = string(n.Kind)
		if len(n.Calls) > 0 {
			calls[n.Name] = n.Calls
		}
	}
	wantKinds := map[string]string{
		"default_profile": "FathomConfigSetting", "example-primary": "FathomConfigProfile", "example-alt": "FathomConfigProfile",
		"example-local": "FathomConfigProfile", "daemon": "FathomConfigSection", "daemon.auto_refresh": "FathomConfigKey",
		"decide": "FathomConfigSection", "tests.results_dir": "FathomConfigKey", "surface": "FathomConfigSection",
		"surface.pattern": "FathomConfigKey", "route": "FathomConfigSurfacePattern", "verb": "FathomConfigSurfacePattern",
		"work.ci_summaries": "FathomConfigKey", "zones": "FathomConfigSection",
	}
	for name, want := range wantKinds {
		if kinds[name] != want {
			t.Errorf("%s: kind %q, want %q", name, kinds[name], want)
		}
	}
	if _, ok := kinds["profiles"]; ok {
		t.Error("the profiles table is the container of the profile entities, not a section")
	}
	// default_profile at the root, [daemon].default_profile and [decide].profile
	// each name a declared profile; nothing else in the file is a reference.
	wantCalls := map[string][]string{
		"default_profile": {"example-primary"},
		"daemon":          {"example-alt"},
		"decide":          {"example-local"},
	}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Errorf("references = %v, want %v", calls, wantCalls)
	}
	var paths []string
	for _, c := range res.CrossFile {
		if c.Strategy != "repo-path" || c.Edge != "config_path" {
			t.Errorf("cross-file %+v: want a repo-path config_path", c)
		}
		paths = append(paths, c.From+"->"+c.Target)
	}
	want := "tests->reports/results,tests->reports/artifacts,work->ci/summary.xml,work->ci/run.json"
	if strings.Join(paths, ",") != want {
		t.Errorf("cross-file = %s, want %s", strings.Join(paths, ","), want)
	}
}

// A default_profile that names no declared profile is a dangling name, not a
// reference (the schema leaves it legal): no entity of that name exists to
// point at.
func TestFathomConfigDanglingProfileIsNoReference(t *testing.T) {
	x := loadFathomConfigExtractor(t)
	res, err := x.ExtractDoc("svc", ".fathom/config.toml", fathomConfigFixture(t, "layered"))
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range res.Nodes {
		if n.Name == "default_profile" && len(n.Calls) != 0 {
			t.Errorf("default_profile names no declared profile but carries %v", n.Calls)
		}
		if n.Name == "daemon" && !reflect.DeepEqual(n.Calls, []string{"example-main"}) {
			t.Errorf("[daemon].default_profile calls = %v, want [example-main]", n.Calls)
		}
	}
}

// This repository's own config: sections and keys, and the pattern entities
// of its two [[surface.pattern]] tables.
func TestFathomConfigOwnFile(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", ".fathom", "config.toml"))
	if err != nil {
		t.Skip("no .fathom/config.toml in this checkout")
	}
	x := loadFathomConfigExtractor(t)
	res, err := x.ExtractDoc("svc", ".fathom/config.toml", src)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, n := range res.Nodes {
		got[n.Name] = true
	}
	for _, name := range []string{"daemon", "tests", "hooks", "surface", "connector", "tool", "verb", "daemon.state_dir", "tests.results_dir"} {
		if !got[name] {
			t.Errorf("own config: no entity %q", name)
		}
	}
}
