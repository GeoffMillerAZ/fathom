package fusion

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// ghaRunSteps reads the run steps of one file (ghaSteps' first half).
func ghaRunSteps(relPath string, content []byte) []ghaRunStep {
	runs, _ := ghaSteps(relPath, content)
	return runs
}

func TestGHARunSteps(t *testing.T) {
	src := `on: push
defaults:
  run:
    working-directory: app
jobs:
  build:
    runs-on: x
    defaults:
      run:
        working-directory: web
    steps:
      - uses: actions/checkout@v4
      - run: make test
      - run: make lint
        working-directory: ${{ matrix.dir }}
  other:
    steps:
      - run: task build
        working-directory: ./tools/../ci
`
	got := ghaRunSteps(".github/workflows/ci.yml", []byte(src))
	var brief []string
	for _, s := range got {
		brief = append(brief, s.Name+" in "+s.Dir+" runs "+s.Run)
	}
	want := []string{
		"jobs.build.steps#1 in web runs make test",
		"jobs.build.steps#2 in ? runs make lint",
		"jobs.other.steps#0 in ci runs task build",
	}
	if !reflect.DeepEqual(brief, want) {
		t.Errorf("got %q\nwant %q", brief, want)
	}
	comp := ghaRunSteps("a/action.yml", []byte("runs:\n  using: composite\n  steps:\n    - run: echo\n      shell: bash\n"))
	if len(comp) != 1 || comp[0].Name != "runs.steps#0" {
		t.Errorf("composite steps: %+v", comp)
	}
}

func TestGHARunRefs(t *testing.T) {
	for _, tc := range []struct {
		name, run, dir string
		want           []ghaRunRef
	}{
		{"make target", "make build", ".", []ghaRunRef{{Kind: "make", Dir: ".", Target: "build"}}},
		{"gmake -C", "gmake -j4 -C sub/dir test lint", ".", []ghaRunRef{{Kind: "make", Dir: "sub/dir", Target: "test"}, {Kind: "make", Dir: "sub/dir", Target: "lint"}}},
		{"make -j N and vars", "CC=gcc make -j 8 V=1 all", ".", []ghaRunRef{{Kind: "make", Dir: ".", Target: "all"}}},
		{"make -f", "make -f build/rules.mk dist", "app", []ghaRunRef{{Kind: "make", Dir: "app", Files: []string{"build/rules.mk"}, Target: "dist"}}},
		{"make variable target", "make $TARGET", ".", nil},
		{"cd then make", "cd native && make install", ".", []ghaRunRef{{Kind: "make", Dir: "native", Target: "install"}}},
		{"task", "task --dir ci build", ".", []ghaRunRef{{Kind: "task", Dir: "ci", Target: "build"}}},
		{"task args after --", "task lint -- --fix", ".", []ghaRunRef{{Kind: "task", Dir: ".", Target: "lint"}}},
		{"npm run", "npm run build --if-present", ".", []ghaRunRef{{Kind: "npm", Dir: ".", Target: "build"}}},
		{"npm test", "npm test", "web", []ghaRunRef{{Kind: "npm", Dir: "web", Target: "test"}}},
		{"npm ci is no script", "npm ci", ".", nil},
		{"npm prefix", "npm --prefix ui run lint", ".", []ghaRunRef{{Kind: "npm", Dir: "ui", Target: "lint"}}},
		{"yarn script", "yarn lint", ".", []ghaRunRef{{Kind: "npm", Dir: ".", Target: "lint"}}},
		{"yarn install is no script", "yarn install --frozen-lockfile", ".", nil},
		{"pnpm run", "pnpm run build", ".", []ghaRunRef{{Kind: "npm", Dir: ".", Target: "build"}}},
		{"pnpm filter", "pnpm --filter web build", ".", nil},
		{"bash script", "bash -e scripts/ci.sh arg", ".", []ghaRunRef{{Kind: "script", Dir: "scripts", Target: "scripts/ci.sh"}}},
		{"bash -c is inline", "bash -c 'make build'", ".", nil},
		{"dot slash", "./scripts/deploy.sh --prod", "app", []ghaRunRef{{Kind: "script", Dir: "app/scripts", Target: "app/scripts/deploy.sh"}}},
		{"source", "source ci/env.sh", ".", []ghaRunRef{{Kind: "script", Dir: "ci", Target: "ci/env.sh"}}},
		{"absolute path", "/usr/bin/env bash", ".", nil},
		{"not first command", "echo make build", ".", nil},
		{"pipeline and sequence", "echo hi | tee x; make all && task dist", ".", []ghaRunRef{{Kind: "make", Dir: ".", Target: "all"}, {Kind: "task", Dir: ".", Target: "dist"}}},
		{"unknown directory", "make x", "?", []ghaRunRef{{Kind: "make", Dir: "?", Target: "x"}}},
	} {
		got := ghaRunRefs(tc.run, tc.dir)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: %q\n got %+v\nwant %+v", tc.name, tc.run, got, tc.want)
		}
	}
}

func TestResolveGHARun(t *testing.T) {
	mk := func(file, name, kind string, line int) core.SymbolNode {
		return core.SymbolNode{Service: "svc", Package: "p", Name: name, Kind: core.SourceKind(kind), FilePath: file, Line: line}
	}
	wf := ".github/workflows/ci.yml"
	steps := []ghaRunStep{
		{File: wf, Name: "jobs.b.steps#0", Dir: ".", Run: "make test\ntask lint\nnpm run build\nbash scripts/ci.sh\nmake missing\n./tools/none.sh"},
		{File: wf, Name: "jobs.b.steps#1", Dir: "web", Run: "npm test"},
	}
	files := []DocFile{
		docFile(wf, mk(wf, "jobs.b.steps#0", "GHARunStep", 5), mk(wf, "jobs.b.steps#1", "GHARunStep", 8)),
		docFile("Taskfile.yml", mk("Taskfile.yml", "lint", "TaskfileTask", 3)),
		docFile("package.json", mk("package.json", "build", "PackageJSONScript", 4)),
		docFile("web/package.json", mk("web/package.json", "test", "PackageJSONScript", 6)),
	}
	paths := map[string]bool{wf: true, "Makefile": true, "Taskfile.yml": true, "package.json": true, "web/package.json": true, "scripts/ci.sh": true, "tools/none.sh": true}
	extract := func(file string) []core.SymbolNode {
		switch file {
		case "Makefile":
			return []core.SymbolNode{mk("Makefile", "test", "MakeTarget", 1), mk("Makefile", "VAR", "MakeVariable", 2)}
		case "scripts/ci.sh":
			return []core.SymbolNode{mk("scripts/ci.sh", "setup", "BashFunction", 3), mk("scripts/ci.sh", "main", "BashFunction", 9)}
		}
		return nil // tools/none.sh defines no function
	}
	got := xrefBrief(resolveGHARun(steps, files, paths, extract))
	want := []string{
		wf + ":jobs.b.steps#0 -run_step-> Makefile:test",
		wf + ":jobs.b.steps#0 -run_step-> Taskfile.yml:lint",
		wf + ":jobs.b.steps#0 -run_step-> package.json:build",
		wf + ":jobs.b.steps#0 -run_step-> scripts/ci.sh:main",
		wf + ":jobs.b.steps#1 -run_step-> web/package.json:test",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func docFile(path string, nodes ...core.SymbolNode) DocFile {
	var f DocFile
	f.Path = path
	f.Result.Nodes = nodes
	return f
}
