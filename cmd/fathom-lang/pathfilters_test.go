package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const pathFilterTestMap = `{"schema_version":1,"coverage":[{"workflow":".github/workflows/ci.yml","job":"build","entries":["src/app.ts"]}]}`

func TestPathFiltersCommand(t *testing.T) {
	root := setupPathFilterRepo(t, map[string]string{
		"coverage.json": pathFilterTestMap,
		".github/workflows/ci.yml": `on:
  push:
    paths: ['**']
  pull_request:
    paths: ['src/**', 'missing/**']
jobs:
  build: {runs-on: ubuntu-latest}
  unrelated: {if: github.repository_owner == 'some-owner'}
`,
		"src/app.ts":  "import '../lib/base';",
		"lib/base.ts": "export const ready = true;",
	})
	doc, code, stderr := runPathFilterTest(t, context.Background(), root)
	if code != 1 || len(doc.Results) != 2 {
		t.Fatalf("exit %d, output %+v, stderr %q", code, doc, stderr)
	}
	for i, want := range []struct {
		event, verdict string
		missed         []string
	}{{"push", "covered", []string{}}, {"pull_request", "missed", []string{"lib/base.ts"}}} {
		got := doc.Results[i]
		if got.Event != want.event || got.Verdict != want.verdict || !reflect.DeepEqual(got.Missed, want.missed) ||
			!reflect.DeepEqual(got.Paths, []string{"lib/base.ts", "src/app.ts"}) || len(got.Unknowns) != 0 {
			t.Errorf("result %d: %+v", i, got)
		}
	}
	if len(doc.UnusedPatterns) != 1 || doc.UnusedPatterns[0].Pattern != "missing/**" {
		t.Fatalf("unused patterns: %+v", doc.UnusedPatterns)
	}
}

func TestPathFiltersHelpAndInvalidMap(t *testing.T) {
	for _, command := range [][]string{{"-h"}, {"pathfilters", "--help"}} {
		var out, errw bytes.Buffer
		if code := run(context.Background(), command, nil, &out, &errw); code != 0 ||
			!strings.Contains(out.String(), "pathfilters --map") || errw.Len() != 0 {
			t.Fatalf("help %v: exit %d, stdout %q, stderr %q", command, code, &out, &errw)
		}
	}
	for _, raw := range []string{
		strings.Replace(pathFilterTestMap, `"entries"`, `"Entries"`, 1),
		strings.Replace(pathFilterTestMap, `"job":"build"`, `"job":null`, 1),
		strings.Replace(pathFilterTestMap, `"src/app.ts"`, `"../escape.ts"`, 1),
		strings.Replace(pathFilterTestMap, `"job":"build"`, `"job":"a","job":"build"`, 1),
		pathFilterTestMap + "{}",
	} {
		root := setupPathFilterRepo(t, map[string]string{"coverage.json": raw})
		var out, errw bytes.Buffer
		if code := run(context.Background(), []string{"pathfilters", "--root", root, "--map", "coverage.json"}, nil, &out, &errw); code != 2 || out.Len() != 0 {
			t.Fatalf("map %s: exit %d, stdout %q, stderr %q", raw, code, &out, &errw)
		}
	}
}

type pathFilterTestResult struct {
	Workflow, Job, Event, Entry, Verdict string
	Paths, Missed                        []string
	Unknowns                             []struct {
		Path, Code string
	}
}

type pathFilterTestOutput struct {
	Results        []pathFilterTestResult
	UnusedPatterns []struct {
		Workflow, Event, Pattern string
	} `json:"unused_patterns"`
}

func runPathFilterTest(t *testing.T, ctx context.Context, root string, extra ...string) (pathFilterTestOutput, int, string) {
	t.Helper()
	var out, errw bytes.Buffer
	args := append([]string{"pathfilters", "--root", root, "--map", "coverage.json"}, extra...)
	code := run(ctx, args, nil, &out, &errw)
	var doc pathFilterTestOutput
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("exit %d, stdout %q, stderr %q: %v", code, &out, &errw, err)
	}
	return doc, code, errw.String()
}

func setupPathFilterRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, src := range files {
		name = filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(name), fs.ModePerm); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}
