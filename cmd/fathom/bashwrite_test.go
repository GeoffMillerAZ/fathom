package main

import (
	"reflect"
	"testing"
)

func TestBashWriteTargets(t *testing.T) {
	cases := []struct {
		name        string
		cmd         string
		wantPaths   []string
		wantWritten bool
	}{
		{
			name:        "plain redirection",
			cmd:         `echo "package store" > internal/store/backend.go`,
			wantPaths:   []string{"internal/store/backend.go"},
			wantWritten: true,
		},
		{
			name:        "append redirection",
			cmd:         `echo "extra line" >> docs/CLAUDE.md`,
			wantPaths:   []string{"docs/CLAUDE.md"},
			wantWritten: true,
		},
		{
			name: "heredoc with redirection before the body",
			cmd: "cat <<'EOF' > internal/store/backend.go\n" +
				"package store\n" +
				"EOF",
			wantPaths:   []string{"internal/store/backend.go"},
			wantWritten: true,
		},
		{
			name:        "stderr redirection is not a file target",
			cmd:         `go build ./... 2>&1`,
			wantPaths:   nil,
			wantWritten: false,
		},
		{
			name:        "redirect to /dev/null is not a file target",
			cmd:         `go test ./... > /dev/null`,
			wantPaths:   nil,
			wantWritten: false,
		},
		{
			name:        "sed -i with a trailing target file",
			cmd:         `sed -i '' 's/foo/bar/' internal/mcp/tools.go`,
			wantPaths:   []string{"internal/mcp/tools.go"},
			wantWritten: true,
		},
		{
			name:        "tee writes to its argument",
			cmd:         `echo hi | tee internal/store/backend.go`,
			wantPaths:   []string{"internal/store/backend.go"},
			wantWritten: true,
		},
		{
			name:        "cp into the tree",
			cmd:         `cp /tmp/draft.go internal/store/backend.go`,
			wantPaths:   []string{"internal/store/backend.go"},
			wantWritten: true,
		},
		{
			name:        "mv into the tree",
			cmd:         `mv internal/store/draft.go internal/store/backend.go`,
			wantPaths:   []string{"internal/store/backend.go"},
			wantWritten: true,
		},
		{
			name:        "git apply is doubt: no extractable target",
			cmd:         `git apply /tmp/fix.patch`,
			wantPaths:   nil,
			wantWritten: true,
		},
		{
			name:        "git am is doubt too",
			cmd:         `git am /tmp/0001-fix.patch`,
			wantPaths:   nil,
			wantWritten: true,
		},
		{
			name:        "read-only command is not write-shaped",
			cmd:         `grep -rn "TODO" internal/`,
			wantPaths:   nil,
			wantWritten: false,
		},
		{
			name:        "go test is not write-shaped",
			cmd:         `go test ./...`,
			wantPaths:   nil,
			wantWritten: false,
		},
		{
			name:        "empty command",
			cmd:         "",
			wantPaths:   nil,
			wantWritten: false,
		},
		{
			name:        "cp with only one argument (no destination) is not write-shaped enough to name a target, but the verb still counts as doubt",
			cmd:         `cp -v internal/store/backend.go`,
			wantPaths:   nil,
			wantWritten: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotPaths, gotWritten := bashWriteTargets(tc.cmd)
			if !reflect.DeepEqual(gotPaths, tc.wantPaths) {
				t.Errorf("bashWriteTargets(%q) paths = %v, want %v", tc.cmd, gotPaths, tc.wantPaths)
			}
			if gotWritten != tc.wantWritten {
				t.Errorf("bashWriteTargets(%q) writeShaped = %v, want %v", tc.cmd, gotWritten, tc.wantWritten)
			}
		})
	}
}
