package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestKeyAt(t *testing.T) {
	lines := strings.Split("tasks:\n  build:\n  generate:mocks:\n  release:*:\n  \"quoted: key\":\n  'it''s':\n  odd:  # c\n    desc: x\n  - task\n", "\n")
	for _, c := range []struct {
		line, col int
		want      string
		ok        bool
	}{
		{2, 3, "build", true},
		{3, 3, "generate:mocks", true},
		{4, 3, "release:*", true},
		{5, 3, "quoted: key", true},
		{6, 3, "it's", true},
		{7, 3, "odd", true},
		{1, 1, "tasks", true},
		{9, 5, "", false}, // a sequence item, not a key
		{99, 1, "", false},
	} {
		got, ok := keyAt(lines, c.line, c.col)
		if got != c.want || ok != c.ok {
			t.Errorf("keyAt(%d,%d) = %q, %v; want %q, %v", c.line, c.col, got, ok, c.want, c.ok)
		}
	}
}

func TestTaskRefs(t *testing.T) {
	for _, c := range []struct {
		name, src string
		want      []string
	}{
		{"full form", `{"deps":["a",{"task":"b","vars":{"X":1}},{"for":["1"],"task":"c"}],"cmds":["echo",{"task":"d"},{"cmd":"x"},{"defer":{"task":"e"}},{"defer":"rm x"}]}`, []string{"a", "b", "c", "d", "e"}},
		{"list shorthand", `["echo a",{"task":"f"},{"defer":{"task":"g"}}]`, []string{"f", "g"}},
		{"string shorthand", `"echo hi"`, nil},
		{"null", `null`, nil},
		{"no refs", `{"desc":"x","cmds":["a","b"]}`, nil},
	} {
		got := taskRefs(json.RawMessage(c.src))
		sort.Strings(got)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: taskRefs = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestIncludeTarget(t *testing.T) {
	for src, want := range map[string]string{
		`"./docs"`: "./docs",
		`{"taskfile":"./api/Taskfile.yml","dir":"./api"}`: "./api/Taskfile.yml",
		`{"dir":"./x"}`: "",
		`null`:          "",
	} {
		if got := includeTarget(json.RawMessage(src)); got != want {
			t.Errorf("includeTarget(%s) = %q, want %q", src, got, want)
		}
	}
	for v, want := range map[string]bool{"https://example.com/Taskfile.yml": true, "git@github.com:o/r.git": true, "./local": false, "../up/Taskfile.yml": false} {
		if got := remoteRE.MatchString(v); got != want {
			t.Errorf("remote(%q) = %v, want %v", v, got, want)
		}
	}
}

func TestIncludedFile(t *testing.T) {
	work := t.TempDir()
	for _, f := range []string{"a/Taskfile.yml", "b/taskfile.yaml", "b/Taskfile.dist.yml", "c/Taskfile.dist.yml", "d/notes.txt", "e/tasks.yml", "Taskfile.yml"} {
		p := filepath.Join(work, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("version: '3'\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct {
		dir, target, want string
		ok                bool
	}{
		{"", "./a", "a/Taskfile.yml", true},
		{"", "./a/Taskfile.yml", "a/Taskfile.yml", true},
		{"", "./b", "b/taskfile.yaml", true}, // go-task's own order: taskfile.yaml before Taskfile.dist.yml
		{"", "./c", "c/Taskfile.dist.yml", true},
		{"", "./d", "", false},                     // a directory with no Taskfile
		{"", "./e/tasks.yml", "e/tasks.yml", true}, // any file
		{"a", "../b", "b/taskfile.yaml", true},
		{"a", "../../outside", "", false},
		{"", "./missing", "", false},
	} {
		got, ok := includedFile(work, c.dir, c.target)
		if got != c.want || ok != c.ok {
			t.Errorf("includedFile(%q, %q) = %q, %v; want %q, %v", c.dir, c.target, got, ok, c.want, c.ok)
		}
	}
}

func TestWithoutInternal(t *testing.T) {
	src := "version: '3'\ntasks:\n  a:\n    internal: true\n    cmds: [x]\n  b:\n    desc: y # internal: true\n    internal: yes  # hidden\n  c:\n    internal: false\n"
	got, n := withoutInternal([]byte(src))
	if n != 2 {
		t.Fatalf("blanked %d lines, want 2", n)
	}
	want := "version: '3'\ntasks:\n  a:\n\n    cmds: [x]\n  b:\n    desc: y # internal: true\n\n  c:\n    internal: false\n"
	if string(got) != want {
		t.Errorf("withoutInternal\n got: %q\nwant: %q", got, want)
	}
	if strings.Count(string(got), "\n") != strings.Count(src, "\n") {
		t.Error("a line moved: the tool's line numbers would no longer be the file's")
	}
}

func TestLeadingLines(t *testing.T) {
	for src, want := range map[string]int{
		"version: '3'\n":                 0,
		"# c\n\nversion: '3'\n":          2,
		"---\n# c\nversion: '3'\n":       2,
		"%YAML 1.2\n---\nversion: '3'\n": 2,
		"":                               1,
	} {
		if got := leadingLines([]byte(src)); got != want {
			t.Errorf("leadingLines(%q) = %d, want %d", src, got, want)
		}
	}
}
