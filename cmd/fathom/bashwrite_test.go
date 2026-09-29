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
		{
			name:        "cd into a subdirectory before a redirect resolves relative to it, not the root",
			cmd:         `cd internal/store && cat > y.go`,
			wantPaths:   []string{"internal/store/y.go"},
			wantWritten: true,
		},
		{
			name:        "cd chains: a second relative cd is relative to the first",
			cmd:         `cd internal && cd store && echo x > y.go`,
			wantPaths:   []string{"internal/store/y.go"},
			wantWritten: true,
		},
		{
			name:        "pushd into a subdirectory before a redirect resolves relative to it",
			cmd:         `pushd internal/mcp && cat > z.go`,
			wantPaths:   []string{"internal/mcp/z.go"},
			wantWritten: true,
		},
		{
			name:        "popd restores the cwd pushd saved",
			cmd:         `pushd internal/mcp && echo x > z.go && popd && echo y > w.go`,
			wantPaths:   []string{"internal/mcp/z.go", "w.go"},
			wantWritten: true,
		},
		{
			name:        "an absolute cd makes the resolved target absolute too",
			cmd:         `cd /tmp/out && cat > w.go`,
			wantPaths:   []string{"/tmp/out/w.go"},
			wantWritten: true,
		},
		{
			name:        "cd to a variable target is unparseable: doubt, not a wrong-directory guess",
			cmd:         `cd $DYNAMIC_DIR && cat > y.go`,
			wantPaths:   nil,
			wantWritten: true,
		},
		{
			name:        "bare cd - (previous dir) is unparseable: doubt",
			cmd:         `cd - && cat > y.go`,
			wantPaths:   nil,
			wantWritten: true,
		},
		{
			name:        "a subshell's cd does not leak into what follows it",
			cmd:         `(cd internal/store && cat > y.go) && cat > z.go`,
			wantPaths:   []string{"internal/store/y.go", "z.go"},
			wantWritten: true,
		},

		// EW-1 / O122: a quoted span is text, never a redirect or a writer.
		{
			name:        "a double-quoted arrow is text, not a redirect (the O122 repro)",
			cmd:         `for id in a b; do echo "$id -> x"; done`,
			wantPaths:   nil,
			wantWritten: false,
		},
		{
			name:        "a single-quoted > is text",
			cmd:         `echo 'a > b'`,
			wantPaths:   nil,
			wantWritten: false,
		},
		{
			name:        "a writer name inside a double-quoted argument is text (sed -i)",
			cmd:         `echo "run sed -i on the file, then tee the result"`,
			wantPaths:   nil,
			wantWritten: false,
		},
		{
			name:        "a writer name inside a single-quoted argument is text (cp, mv)",
			cmd:         `git commit -m 'cp the draft over, mv the rest'`,
			wantPaths:   nil,
			wantWritten: false,
		},
		{
			name:        "a quoted arrow does not hide a real redirect that follows it",
			cmd:         `echo "a -> b" > internal/store/backend.go`,
			wantPaths:   []string{"internal/store/backend.go"},
			wantWritten: true,
		},
		{
			name:        "a double-quoted redirect target is still a target",
			cmd:         `cat > "internal/store/y.go"`,
			wantPaths:   []string{"internal/store/y.go"},
			wantWritten: true,
		},
		{
			name:        "a single-quoted target with a space is still one target",
			cmd:         `echo x > 'docs/a b.md'`,
			wantPaths:   []string{"docs/a b.md"},
			wantWritten: true,
		},
		{
			name:        "sed -i with quoted script and a quoted > inside it",
			cmd:         `sed -i 's/a > b/c/' internal/mcp/tools.go`,
			wantPaths:   []string{"internal/mcp/tools.go"},
			wantWritten: true,
		},
		{
			name:        "an escaped quote does not open a quoted span",
			cmd:         `echo \" > internal/store/y.go`,
			wantPaths:   []string{"internal/store/y.go"},
			wantWritten: true,
		},
		// EW-1 / O122 addendum: a heredoc body is data.
		{
			name: "a heredoc body naming a writer and a redirect is data; only the redirect before the body counts",
			cmd: "cat > /tmp/x.sh <<'EOF'\n" +
				"cp bin/fathom /usr/local/bin/fathom\n" +
				"echo ${1:?usage} > out.txt\n" +
				"tee -a log.txt\n" +
				"EOF",
			wantPaths:   []string{"/tmp/x.sh"},
			wantWritten: true,
		},
		{
			name: "a heredoc with no write outside its body is not write-shaped",
			cmd: "cat <<'EOF'\n" +
				"echo hi > out.txt\n" +
				"cp a b\n" +
				"EOF",
			wantPaths:   nil,
			wantWritten: false,
		},
		{
			name: "an apostrophe in a heredoc body does not open a quote that swallows the next statement",
			cmd: "cat <<EOF > a.go\n" +
				"don't stop\n" +
				"EOF\n" +
				"echo y > b.go",
			wantPaths:   []string{"a.go", "b.go"},
			wantWritten: true,
		},
		{
			name: "a <<- heredoc's terminator may be tab-indented",
			cmd: "cat <<-END > a.go\n" +
				"\tcp x y\n" +
				"\tEND\n" +
				"echo y > b.go",
			wantPaths:   []string{"a.go", "b.go"},
			wantWritten: true,
		},
		{
			name:        "a here-string is not a heredoc",
			cmd:         `cat <<< "x" > a.go`,
			wantPaths:   []string{"a.go"},
			wantWritten: true,
		},
		{
			name:        "shell arithmetic shift is not a heredoc",
			cmd:         "echo $((1<<3)) > a.go\necho y > b.go",
			wantPaths:   []string{"a.go", "b.go"},
			wantWritten: true,
		},
		{
			name:        "a tilde target stays a tilde path, never a repo-relative one",
			cmd:         `printf '%s\n' x >> ~/.claude/projects/p/memory/MEMORY.md`,
			wantPaths:   []string{"~/.claude/projects/p/memory/MEMORY.md"},
			wantWritten: true,
		},
		{
			name:        "cd into a tilde directory keeps later targets tilde-rooted",
			cmd:         `cd ~/notes && echo x > today.md`,
			wantPaths:   []string{"~/notes/today.md"},
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
