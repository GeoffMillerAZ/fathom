package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestIngestNamesSkippedTypedPackages (EW-83, O1506): `fathom ingest`, in
// text and with --json, carries the typed pass's skips — the typed-edges
// line counts them by reason, one line names each package, directory and
// module that loses evidence, and the JSON block lists every reason with its
// first names — also for a tree with no module at all, where the counts line
// stays silent and nothing fails.
func TestIngestNamesSkippedTypedPackages(t *testing.T) {
	write := func(t *testing.T, files map[string]string) string {
		t.Helper()
		root := t.TempDir()
		for rel, body := range files {
			name := filepath.Join(root, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(name, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return root
	}
	ingest := func(t *testing.T, svc, root string, extra ...string) (int, string, string) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		args := append([]string{"--service", svc, "--path", root, "--store", filepath.Join(t.TempDir(), "s.sqlite")}, extra...)
		code := cmdIngest(args, &stdout, &stderr)
		return code, stdout.String(), stderr.String()
	}
	skipsLine := func(out string) string {
		for _, l := range strings.Split(out, "\n") {
			if strings.HasPrefix(l, "typed-edges: syntax-only") {
				return l
			}
		}
		return ""
	}

	fixture := map[string]string{
		"go.mod":           "module example.com/sk\n\ngo 1.23\n",
		"a/a.go":           "package a\n\nfunc A() {}\n",
		"a/a_test.go":      "package a\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) { A() }\n",
		"tagged/tagged.go": "//go:build fathom_never\n\npackage tagged\n\nfunc T() {}\n",
		"us/_gen.go":       "package us\n\nfunc Gen() {}\n",
		"_wip/w.go":        "package wip\n\nfunc W() {}\n",
		"other/go.mod":     "module example.org/other\n\ngo 1.23\n",
		"other/o.go":       "package other\n\nfunc O() {}\n",
	}

	t.Run("text", func(t *testing.T) {
		code, out, errOut := ingest(t, "sk", write(t, fixture))
		if code != 0 {
			t.Fatalf("exit %d: %s %s", code, out, errOut)
		}
		if !strings.Contains(out, "typed-edges: 2/5 pkg(s) processed (0 failed, 3 skipped: 1 build constraints, 1 outside ./..., 1 test main), ") {
			t.Errorf("the typed-edges line must count the skips by reason:\n%s", out)
		}
		line := skipsLine(out)
		for _, w := range []string{
			"build constraints 1 package(s): example.com/sk/tagged",
			"outside ./... 1 package(s): ./us",
			"module path not the repository's 1 module(s): other/go.mod",
			"outside ./... 1 directory(ies): _wip",
		} {
			if !strings.Contains(line, w) {
				t.Errorf("naming line %q does not carry %q", line, w)
			}
		}
		if strings.Contains(line, "a.test") || strings.Contains(line, "test main") {
			t.Errorf("a harmless class is named only in JSON: %q", line)
		}
	})

	t.Run("json", func(t *testing.T) {
		code, out, errOut := ingest(t, "sk", write(t, fixture), "--json")
		if code != 0 {
			t.Fatalf("exit %d: %s %s", code, out, errOut)
		}
		var env ingestJSON
		if err := json.Unmarshal([]byte(out), &env); err != nil {
			t.Fatalf("not JSON: %v\n%s", err, out)
		}
		te := env.TypedEdges
		if te == nil {
			t.Fatalf("typed_edges missing: %s", out)
		}
		if te.PackagesSkipped != 3 || te.PackagesSeen != 5 || te.PackagesProcessed != 2 {
			t.Errorf("typed_edges seen %d processed %d skipped %d, want 5, 2, 3", te.PackagesSeen, te.PackagesProcessed, te.PackagesSkipped)
		}
		wantSkipped := []ingestTypedSkipJSON{
			{Reason: "build constraints", Count: 1, Packages: []string{"example.com/sk/tagged"}},
			{Reason: "outside ./...", Count: 1, Packages: []string{"./us"}},
			{Reason: "test main", Count: 1, Packages: []string{"example.com/sk/a.test"}},
		}
		if !reflect.DeepEqual(te.Skipped, wantSkipped) {
			t.Errorf("skipped:\n got %+v\nwant %+v", te.Skipped, wantSkipped)
		}
		wantNotLoaded := []ingestTypedNotLoadedJSON{
			{Reason: "module path not the repository's", Count: 1, Paths: []string{"other/go.mod"}},
			{Reason: "outside ./...", Count: 1, Paths: []string{"_wip"}},
		}
		if !reflect.DeepEqual(te.NotLoaded, wantNotLoaded) {
			t.Errorf("not_loaded:\n got %+v\nwant %+v", te.NotLoaded, wantNotLoaded)
		}
	})

	t.Run("nothing skipped leaves the line unchanged", func(t *testing.T) {
		root := write(t, map[string]string{
			"go.mod":  "module example.com/clean\n\ngo 1.23\n",
			"main.go": "package main\n\nfunc main() { helper() }\n\nfunc helper() {}\n",
		})
		code, out, errOut := ingest(t, "clean", root)
		if code != 0 {
			t.Fatalf("exit %d: %s %s", code, out, errOut)
		}
		if !strings.Contains(out, "typed-edges: 1/1 pkg(s) processed (0 failed), 1 edge(s) (1 direct / 0 iface-dispatch / 0 unresolved)\n") {
			t.Errorf("with nothing skipped the line must read as before:\n%s", out)
		}
		if l := skipsLine(out); l != "" {
			t.Errorf("nothing to name, yet %q", l)
		}
	})

	t.Run("no module", func(t *testing.T) {
		root := write(t, map[string]string{"scripts/s.go": "package main\n\nfunc main() {}\n"})
		code, out, errOut := ingest(t, "nomod", root)
		if code != 0 {
			t.Fatalf("exit %d: %s %s", code, out, errOut)
		}
		if strings.Contains(out, "pkg(s) processed") || strings.Contains(out, "(0 failed)") {
			t.Errorf("no module: the counts line keeps its print condition:\n%s", out)
		}
		if l := skipsLine(out); !strings.Contains(l, "no go.mod 1 directory(ies): scripts") {
			t.Errorf("no module: the naming line must print alone and name scripts, got %q in\n%s", l, out)
		}

		code, out, errOut = ingest(t, "nomod", root, "--json")
		if code != 0 {
			t.Fatalf("exit %d: %s %s", code, out, errOut)
		}
		var env ingestJSON
		if err := json.Unmarshal([]byte(out), &env); err != nil {
			t.Fatalf("not JSON: %v\n%s", err, out)
		}
		if env.Outcome != "ok" || env.ExitCode != 0 {
			t.Errorf("no module is not a degraded ingest: outcome %q exit %d", env.Outcome, env.ExitCode)
		}
		te := env.TypedEdges
		if te == nil {
			t.Fatalf("typed_edges must print when not_loaded alone has entries: %s", out)
		}
		want := []ingestTypedNotLoadedJSON{{Reason: "no go.mod", Count: 1, Paths: []string{"scripts"}}}
		if te.PackagesSeen != 0 || te.PackagesProcessed != 0 || te.Failed || !reflect.DeepEqual(te.NotLoaded, want) {
			t.Errorf("typed_edges = %+v, want zero counts, failed false and not_loaded %+v", te, want)
		}
	})
}
