package main

import (
	"context"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// One matcher: argv words a wrapper resolves without a shell (a Python
// spawn's list) reach the same per-unit judgment as the shell line that
// spells them, walk for walk.
func TestDoctorShellLineAndArgvAgree(t *testing.T) {
	strict := func(w []string) bool {
		return len(w) > 2 && filepath.Base(w[0]) == "fathom" && w[1] == "verify" && slices.Contains(w[2:], "--strict")
	}
	root := t.TempDir()
	doctorWriteFixture(t, root, "scripts/gate.sh", "fathom verify --strict\n")
	doctorWriteFixture(t, root, "scripts/deep.sh", "sh scripts/gate.sh\n")
	doctorWriteFixture(t, root, "scripts/lint.py", "print('lint')\n")
	doctorWriteFixture(t, root, "Taskfile.yml", "version: '3'\ntasks:\n  validate:\n    cmds: [sh scripts/gate.sh]\n  ci:\n    deps: [x]\n    cmds: [echo]\n")
	doctorWriteFixture(t, root, "lefthook.yml", "pre-push:\n  commands:\n    gate:\n      run: fathom verify --strict\n")
	for _, tc := range []struct {
		name, line string
		depth      int
		sub        string
		match      func([]string) bool
	}{
		{"fathom subcommand", "fathom prompt-hook", 0, "prompt-hook", nil},
		{"env prefix", "env FATHOM_X=1 fathom serve-mcp", 1, "serve-mcp", nil},
		{"fathom variable", "$FATHOM prompt-hook", 1, "prompt-hook", nil},
		{"shell script", "sh scripts/gate.sh", 0, "", strict},
		{"script path", "./scripts/gate.sh", 1, "", strict},
		{"script at the depth limit", "sh scripts/deep.sh", 2, "", strict},
		{"task", "task validate", 0, "", strict},
		{"task with deps", "task ci", 0, "", strict},
		{"missing task", "task absent", 0, "", strict},
		{"lefthook run", "lefthook run pre-push", 0, "", strict},
		{"missing script", "./scripts/absent.sh", 0, "prompt-hook", nil},
		{"python script", "python3 scripts/lint.py verify", 0, "", strict},
		{"dynamic shell", "eval thing", 1, "prompt-hook", nil},
		{"sh -c", "sh -c thing", 1, "prompt-hook", nil},
		{"dynamic path", "$SOMEWHERE/gate.sh", 1, "prompt-hook", nil},
		{"foreign program", "/usr/local/bin/task validate", 0, "", strict},
		{"unrelated", "go test ./...", 0, "", strict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			shell := doctorWrapperReader{ctx: context.Background(), root: root, remaining: doctorWrapperBytes, matches: tc.match}
			argv := shell
			want := shell.command(tc.line, tc.sub, tc.depth)
			got := argv.argv(strings.Fields(tc.line), tc.sub, tc.depth)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("argv evidence %+v, shell line %+v", got, want)
			}
			if argv.remaining != shell.remaining || !reflect.DeepEqual(argv.walked, shell.walked) || !reflect.DeepEqual(argv.depthCuts, shell.depthCuts) {
				t.Fatalf("walks differ: argv %d %v %v, shell %d %v %v", argv.remaining, argv.walked, argv.depthCuts, shell.remaining, shell.walked, shell.depthCuts)
			}
		})
	}
}
