package fusion

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestTypedPassSkipsConfiguredModules is EW-68: a repository names the nested
// modules the typed pass does not load in its own .fathom/config.toml,
// `[typed] skip_modules`, as repo-relative directory globs (internal/pathglob).
// A matched module is not loaded whatever the own-module rule says, keeps its
// syntax edges, and is named with reason skip_modules; the root module is
// never skipped; an absolute or escaping glob is refused (config.RepoPath)
// and skips nothing; a file that does not parse skips nothing.
func TestTypedPassSkipsConfiguredModules(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/r\n\ngo 1.22\n")
	write("r.go", "package r\n")
	write("kits/a/go.mod", "module example.com/r/kits/a\n\ngo 1.22\n")
	write("kits/a/a.go", "package a\n")
	write("calibration/oracles/p/go.mod", "module example.com/r/calibration/oracles/p\n\ngo 1.22\n")
	write("calibration/oracles/p/p.go", "package main\n")
	write("calibration/census/go.mod", "module example.com/r/calibration/census\n\ngo 1.22\n")
	write("calibration/census/c.go", "package main\n")
	write("oracles/o/go.mod", "module example.org/o\n\ngo 1.22\n")
	write("oracles/o/o.go", "package main\n")
	allowed := map[string]bool{"r.go": true, "kits/a/a.go": true, "calibration/oracles/p/p.go": true, "calibration/census/c.go": true, "oracles/o/o.go": true}
	rels := func(ms []goModule) []string {
		out := []string{}
		for _, m := range ms {
			out = append(out, m.rel+"="+m.skip)
		}
		return out
	}

	// No config: the own modules load, the foreign one is named.
	mods, foreign := typedModules(dir, allowed)
	if got, want := rels(mods), []string{".=", "calibration/census=", "calibration/oracles/p=", "kits/a="}; !reflect.DeepEqual(got, want) {
		t.Fatalf("no config: loaded %v, want %v", got, want)
	}

	write(".fathom/config.toml", "[typed]\nskip_modules = [\"calibration/oracles/**\", \"./calibration/census/\", \"oracles/**\", \"oracles/**\", \"/abs/**\", \"../out/**\"]\n")
	for _, scope := range []map[string]bool{allowed, nil} {
		mods, foreign = typedModules(dir, scope)
		if got, want := rels(mods), []string{".=", "kits/a="}; !reflect.DeepEqual(got, want) {
			t.Errorf("skip_modules (allowed nil=%v): loaded %v, want %v (the root is never skipped)", scope == nil, got, want)
		}
		want := []string{"calibration/census=" + skipConfigured, "calibration/oracles/p=" + skipConfigured, "oracles/o=" + skipConfigured}
		if got := rels(foreign); !reflect.DeepEqual(got, want) {
			t.Errorf("skip_modules (allowed nil=%v): skipped %v, want %v", scope == nil, got, want)
		}
	}
	if got, want := typedSkipModules(dir), []string{"calibration/census", "calibration/oracles/**", "oracles/**"}; !reflect.DeepEqual(got, want) {
		t.Errorf("typedSkipModules = %v, want %v (sorted, cleaned, the escaping globs refused)", got, want)
	}
	names := skippedModuleNames(foreign)
	if want := "calibration/census/go.mod (skip_modules)"; names[0] != want {
		t.Errorf("skippedModuleNames[0] = %q, want %q", names[0], want)
	}

	// Only the nested modules are skippable: "**" skips every one of them
	// and never the root; with no root go.mod there is then no typed module.
	write(".fathom/config.toml", "[typed]\nskip_modules = [\"**\"]\n")
	if mods, foreign := typedModules(dir, allowed); !reflect.DeepEqual(rels(mods), []string{".="}) || len(foreign) != 4 {
		t.Errorf("skip_modules **: loaded %v skipped %v, want the root only", rels(mods), rels(foreign))
	}
	if err := os.Remove(filepath.Join(dir, "go.mod")); err != nil {
		t.Fatal(err)
	}
	if hasTypedGoModule(dir, allowed) {
		t.Error("hasTypedGoModule: true although skip_modules skips every nested module of a repository with no root go.mod")
	}

	// A file that does not parse skips nothing.
	write(".fathom/config.toml", "[typed\nskip_modules = [\"**\"]\n")
	if got := typedSkipModules(dir); got != nil {
		t.Errorf("typedSkipModules on a malformed file = %v, want nil", got)
	}
}

// TestSkipModulesStampFoldsTheGlobSet: the pass-state stamp is "" without
// globs (the row a binary before EW-68 wrote still reads as completed), and
// otherwise names the sorted glob set, so a changed set reads as no
// completed pass and the same set in another order does not.
func TestSkipModulesStampFoldsTheGlobSet(t *testing.T) {
	if s := skipModulesStamp(nil); s != "" {
		t.Errorf("stamp of no globs = %q, want empty", s)
	}
	a := skipModulesStamp([]string{"calibration/oracles/**", "tools/gen"})
	b := skipModulesStamp([]string{"calibration/oracles/**"})
	if a == "" || b == "" || a == b || !strings.HasPrefix(a, "@skip-") {
		t.Errorf("stamps %q and %q: want two distinct @skip- stamps", a, b)
	}
	if c := skipModulesStamp([]string{"tools/gen", "calibration/oracles/**"}); c != a {
		t.Errorf("stamp depends on glob order: %q vs %q", c, a)
	}
}

// TestForeignModulesLineNamesSlowestLoadsAndKnob is EW-69: the typed pass's
// one stderr line names the modules it did not load (with why), the three
// slowest nested module loads with their seconds (slowest first, ties by
// directory), and the knob that skips a module; it is one line, and a
// repository whose nested modules all loaded still gets it.
func TestForeignModulesLineNamesSlowestLoadsAndKnob(t *testing.T) {
	loads := []moduleLoad{
		{rel: "calibration/telemetry-census", wall: 2 * time.Second},
		{rel: "calibration/oracles/prometheus", wall: 412300 * time.Millisecond},
		{rel: "calibration/oracles/grafana", wall: 2 * time.Second},
		{rel: "calibration/oracles/alertmanager", wall: 98 * time.Second},
	}
	skipped := skippedModuleNames([]goModule{{rel: "oracles/o", skip: skipNotOwn}, {rel: "tools/gen", skip: skipConfigured}})
	line := foreignModulesLine("fathom", skipped, loads)
	if strings.Count(line, "\n") != 1 || !strings.HasSuffix(line, "\n") {
		t.Fatalf("line %q: want exactly one line", line)
	}
	for _, want := range []string{
		"2 nested Go module(s) not loaded",
		"oracles/o/go.mod (module path not the repository's)",
		"tools/gen/go.mod (skip_modules)",
		"slowest nested module loads: calibration/oracles/prometheus 412.3s, calibration/oracles/alertmanager 98.0s, calibration/oracles/grafana 2.0s",
		"[typed] skip_modules in .fathom/config.toml",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("line %q: want it to contain %q", line, want)
		}
	}
	if strings.Contains(line, "telemetry-census") {
		t.Errorf("line %q names a fourth load: only the three slowest", line)
	}
	if again := foreignModulesLine("fathom", skipped, []moduleLoad{loads[3], loads[2], loads[1], loads[0]}); again != line {
		t.Errorf("the line depends on load order:\n%q\n%q", line, again)
	}
	onlyLoads := foreignModulesLine("fathom", nil, loads[:1])
	if !strings.Contains(onlyLoads, "slowest nested module loads: calibration/telemetry-census 2.0s") || !strings.Contains(onlyLoads, "[typed] skip_modules") || strings.Contains(onlyLoads, "not loaded") {
		t.Errorf("all nested modules loaded: line %q, want the slowest loads and the knob, no skipped clause", onlyLoads)
	}
	if l := foreignModulesLine("fathom", nil, nil); l != "" {
		t.Errorf("no nested module: line %q, want none", l)
	}
}
