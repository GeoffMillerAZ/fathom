package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// TestPathFiltersFixtureFirstUse exercises the normal companion dispatch on
// a repository with independently seeded direct and transitive misses.
func TestPathFiltersFixtureFirstUse(t *testing.T) {
	root := filepath.Join("testdata", "pathfilters")
	invoke := func(root string) []byte {
		t.Helper()
		var out, errw bytes.Buffer
		args := []string{"pathfilters", "--root", root, "--map", "pathfilter-map.json"}
		if code := run(context.Background(), args, nil, &out, &errw); code != 1 || errw.Len() != 0 || out.Len() == 0 {
			t.Fatalf("exit %d, stdout %q, stderr %q", code, &out, &errw)
		}
		return out.Bytes()
	}
	wire := invoke(root)
	var doc pathFilterTestOutput
	if err := json.Unmarshal(wire, &doc); err != nil {
		t.Fatal(err)
	}
	want := []struct {
		workflow, job, event, verdict string
		missed                        []string
	}{
		{"control.yml", "", "pull_request", "covered", []string{}},
		{"control.yml", "", "push", "covered", []string{}},
		{"gap.yml", "build", "push", "missed", []string{"shared/leaf.ts", "src/skip.ts"}},
		{"gap.yml", "build", "pull_request", "missed", []string{"shared/leaf.ts"}},
	}
	if len(doc.Results) != len(want) {
		t.Fatalf("results: %+v", doc.Results)
	}
	for i, expected := range want {
		got := doc.Results[i]
		if got.Workflow != ".github/workflows/"+expected.workflow || got.Job != expected.job || got.Entry != "src/app.ts" ||
			got.Event != expected.event || got.Verdict != expected.verdict || !reflect.DeepEqual(got.Missed, expected.missed) ||
			len(got.Unknowns) != 0 || !slices.Equal(got.Paths, []string{"shared/leaf.ts", "src/app.ts", "src/middle.ts", "src/skip.ts"}) {
			t.Errorf("result %d: %+v", i, got)
		}
	}
	checkGolden(t, "pathfilters.json", wire)

	relocated := t.TempDir()
	if err := os.CopyFS(relocated, os.DirFS(root)); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(wire, invoke(root)) || !bytes.Equal(wire, invoke(relocated)) {
		t.Fatal("JSON changed on replay or outside the original fixture directory")
	}
	var out, errw bytes.Buffer
	args := []string{"pathfilters", "--root", relocated, "--map", "pathfilter-map.json", ".github/workflows/control.yml"}
	if code := run(context.Background(), args, nil, &out, &errw); code != 0 || errw.Len() != 0 || !strings.Contains(out.String(), `"covered"`) {
		t.Fatalf("covered control: exit %d, stdout %q, stderr %q", code, &out, &errw)
	}
}

func TestPathFiltersIncompleteAndConditional(t *testing.T) {
	const workflow = ".github/workflows/ci.yml"
	for _, tt := range []struct {
		name, config, source, entry, extra, wantCode, verdict string
	}{
		{"dynamic import", "on: push\njobs: {build: {}}", "import(pathFromInput);", "src/app.ts", "", "dynamic-import", "unverified"},
		{"unknown plus known miss", "on: {push: {paths: ['src/**']}}\njobs: {build: {}}", "import('../lib/base.ts'); import(pathFromInput);", "src/app.ts", "", "dynamic-import", "missed"},
		{"implicit job if", "on: push\njobs: {build: {if: github.ref == 'refs/heads/main'}}", "", "src/app.ts", "", "job-condition", "unverified"},
		{"implicit step if", "on: push\njobs: {build: {steps: [{if: github.ref == 'refs/heads/main', run: example}]}}", "", "src/app.ts", "", "job-condition", "unverified"},
		{"workflow scope step if", "on: push\njobs: {build: {steps: [{if: github.ref == 'refs/heads/main', run: example}]}}", "", "src/app.ts", "workflow scope", "job-condition", "unverified"},
		{"event condition", "on: {push: {branches: [main]}}\njobs: {build: {}}", "", "src/app.ts", "", "event-condition", "unverified"},
		{"missing mapped job", "on: push\njobs: {other: {}}", "", "src/app.ts", "", "unmapped-job", "unverified"},
		{"unmapped workflow", "on: push\njobs: {build: {}}", "", "src/app.ts", ".github/workflows/unmapped.yml", "unmapped-workflow", "unverified"},
		{"unsupported event", "on: workflow_dispatch\njobs: {build: {}}", "", "src/app.ts", "", "unsupported-events", "unverified"},
		{"invalid workflow", "on: {push: {paths: ['[bad']}}\njobs: {build: {}}", "", "src/app.ts", "", "invalid-workflow", "unverified"},
		{"missing workflow", "", "", "src/app.ts", "", "workflow-not-in-inventory", "unverified"},
		{"directory entry", "on: push\njobs: {build: {}}", "", "src", "", "file-not-in-inventory", "unverified"},
		{"missing entry", "on: push\njobs: {build: {}}", "", "src/missing.ts", "", "file-not-in-inventory", "unverified"},
		{"unsupported entry", "on: push\njobs: {build: {}}", "", "src/data.txt", "", "unsupported-construct", "unverified"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			raw := strings.Replace(pathFilterTestMap, "src/app.ts", tt.entry, 1)
			var extra []string
			switch tt.extra {
			case "":
			case "workflow scope":
				raw = strings.Replace(raw, `"job":"build",`, "", 1)
			default:
				extra = append(extra, tt.extra)
			}
			files := map[string]string{
				"coverage.json": raw,
				"src/app.ts":    tt.source,
				"src/data.txt":  "plain text",
				"lib/base.ts":   "export const ready = true;",
			}
			if tt.config != "" {
				files[workflow] = tt.config
			}
			root := setupPathFilterRepo(t, files)
			doc, code, stderr := runPathFilterTest(t, context.Background(), root, extra...)
			if code != 1 || len(doc.Results) != 1 || doc.Results[0].Verdict != tt.verdict {
				t.Fatalf("exit %d, results %+v, stderr %q", code, doc.Results, stderr)
			}
			result := doc.Results[0]
			if !hasPathFilterUnknown(result, tt.wantCode) {
				t.Fatalf("missing reason %s in %+v", tt.wantCode, result.Unknowns)
			}
			if tt.verdict == "missed" && !slices.Equal(result.Missed, []string{"lib/base.ts"}) {
				t.Fatalf("known miss lost: %+v", result)
			}
		})
	}
}

func TestPathFiltersInventoryCancellationAndRoot(t *testing.T) {
	root := setupPathFilterRepo(t, map[string]string{
		"coverage.json":            pathFilterTestMap,
		".github/workflows/ci.yml": "on: {push: {paths: ['src/**', '.git/**', 'nested/.svn/**']}}\njobs: {build: {}}",
		"src/app.ts":               "export const ready = true;",
		".git/config":              "private",
		"nested/.svn/config":       "private",
	})
	doc, code, _ := runPathFilterTest(t, context.Background(), root)
	if code != 0 || len(doc.UnusedPatterns) != 2 {
		t.Fatalf("VCS entries changed the inventory: exit %d, %+v", code, doc)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	doc, code, _ = runPathFilterTest(t, ctx, root)
	if code != 1 || len(doc.Results) != 1 || doc.Results[0].Verdict != "unverified" ||
		!hasPathFilterUnknown(doc.Results[0], "incomplete-inventory") || len(doc.UnusedPatterns) != 0 {
		t.Fatalf("canceled inventory: exit %d, %+v", code, doc)
	}

	sourceLink := filepath.Join(root, "src", "app.ts")
	outside := filepath.Join(t.TempDir(), "outside.ts")
	if err := os.WriteFile(outside, []byte("export const outside = true;"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(sourceLink); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, sourceLink); err != nil {
		t.Fatal(err)
	}
	doc, code, _ = runPathFilterTest(t, context.Background(), root)
	if code != 1 || len(doc.Results) != 1 || doc.Results[0].Verdict != "unverified" ||
		!hasPathFilterUnknown(doc.Results[0], "file-not-in-inventory") {
		t.Fatalf("out-of-root symlink: exit %d, %+v", code, doc)
	}
}

func TestPathFiltersRefusesMapThroughSymlink(t *testing.T) {
	root := setupPathFilterRepo(t, map[string]string{
		".git/pathfilter-map.json": pathFilterTestMap,
		".github/workflows/ci.yml": "on: push\njobs: {build: {}}",
		"src/app.ts":               "export const ready = true;",
	})
	if err := os.Symlink(".git", filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	var out, errw bytes.Buffer
	args := []string{"pathfilters", "--root", root, "--map", "alias/pathfilter-map.json"}
	if code := run(context.Background(), args, nil, &out, &errw); code != 1 || out.Len() != 0 {
		t.Fatalf("map through a symlink: exit %d, stdout %q, stderr %q", code, &out, &errw)
	}
}

func TestPathFiltersUnreadableInventory(t *testing.T) {
	root := setupPathFilterRepo(t, map[string]string{
		"coverage.json":            pathFilterTestMap,
		".github/workflows/ci.yml": "on: {push: {paths: ['src/**', 'blocked/**']}}\njobs: {build: {}}",
		"src/app.ts":               "export const ready = true;",
		"blocked/dep.ts":           "export const hidden = true;",
	})
	dir := filepath.Join(root, "blocked")
	if err := os.Chmod(dir, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(dir, 0o700); err != nil {
			t.Error(err)
		}
	})
	if _, err := os.ReadDir(dir); err == nil {
		t.Skip("running with privileges that bypass directory permissions")
	}
	doc, code, _ := runPathFilterTest(t, context.Background(), root)
	if code != 1 || len(doc.Results) != 1 || doc.Results[0].Verdict != "unverified" ||
		!hasPathFilterUnknown(doc.Results[0], "incomplete-inventory") || len(doc.UnusedPatterns) != 0 {
		t.Fatalf("unreadable inventory: exit %d, %+v", code, doc)
	}
}

func TestPathFiltersDependencyTargets(t *testing.T) {
	const workflow = "on: push\njobs: {build: {}}"
	t.Run("VCS metadata is not a dependency input", func(t *testing.T) {
		root := setupPathFilterRepo(t, map[string]string{
			"coverage.json":            pathFilterTestMap,
			".github/workflows/ci.yml": workflow,
			"src/app.ts":               "import '../.git/private.ts';",
			".git/private.ts":          "export const ready = true;",
		})
		doc, code, _ := runPathFilterTest(t, context.Background(), root)
		if code != 1 || len(doc.Results) != 1 || doc.Results[0].Verdict != "unverified" ||
			!slices.Equal(doc.Results[0].Paths, []string{"src/app.ts"}) {
			t.Fatalf("VCS metadata crossed into the dependency graph: exit %d, %+v", code, doc)
		}
	})
	t.Run("directory dependency", func(t *testing.T) {
		root := setupPathFilterRepo(t, map[string]string{
			"coverage.json":            strings.Replace(pathFilterTestMap, "src/app.ts", "app/main.go", 1),
			".github/workflows/ci.yml": workflow,
			"go.mod":                   "module example.test/project\n\ngo 1.26.2\n",
			"app/main.go":              "package main\nimport _ \"example.test/project/lib\"\nfunc main() {}\n",
			"lib/dep.go":               "package lib\n",
		})
		doc, code, _ := runPathFilterTest(t, context.Background(), root, "--trust-unmeasured", "go")
		if code != 1 || len(doc.Results) != 1 || doc.Results[0].Verdict != "unverified" ||
			!hasPathFilterUnknown(doc.Results[0], "directory-dependency") || slices.Contains(doc.Results[0].Paths, "lib") {
			t.Fatalf("directory dependency: exit %d, %+v", code, doc)
		}
	})
	t.Run("external dependency", func(t *testing.T) {
		root := setupPathFilterRepo(t, map[string]string{
			"coverage.json":            pathFilterTestMap,
			".github/workflows/ci.yml": workflow,
			"src/app.ts":               "import 'node:fs';",
		})
		doc, code, _ := runPathFilterTest(t, context.Background(), root)
		if code != 1 || len(doc.Results) != 1 || doc.Results[0].Verdict != "unverified" ||
			!hasPathFilterUnknown(doc.Results[0], "external-unpinned") || !slices.Equal(doc.Results[0].Paths, []string{"src/app.ts"}) {
			t.Fatalf("external dependency crossed into the checkout graph: exit %d, %+v", code, doc)
		}
	})
}

func hasPathFilterUnknown(result pathFilterTestResult, code string) bool {
	for _, unknown := range result.Unknowns {
		if unknown.Code == code {
			return true
		}
	}
	return false
}
