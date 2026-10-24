package main

import (
	"reflect"
	"testing"
)

func TestBashWriteTargetsSed(t *testing.T) {
	for _, tt := range []struct {
		name, command string
		want          []string
	}{
		{"quiet before in-place", `sed --quiet -i 's/a/b/' f`, []string{"f"}},
		{"silent before in-place", `sed --silent -i 's/a/b/' f`, []string{"f"}},
		{"extended before in-place", `sed --regexp-extended -i 's/a/b/' f`, []string{"f"}},
		{"separate before in-place", `sed --separate -i 's/a/b/' f`, []string{"f"}},
		{"null data before in-place", `sed --null-data -i 's/a/b/' f`, []string{"f"}},
		{"zero terminated before in-place", `sed --zero-terminated -i 's/a/b/' f`, []string{"f"}},
		{"unbuffered before in-place", `sed --unbuffered -i 's/a/b/' f`, []string{"f"}},
		{"follow symlinks before in-place", `sed --follow-symlinks -i 's/a/b/' f`, []string{"f"}},
		{"sandbox before in-place", `sed --sandbox -i 's/a/b/' f`, []string{"f"}},
		{"short line length", `sed -l 80 -i 's/a/b/' f`, []string{"f"}},
		{"clustered line length", `sed -nl80 -i 's/a/b/' f`, []string{"f"}},
		{"long line length", `sed --line-length=80 -i 's/a/b/' f`, []string{"f"}},
		{"separate long line length", `sed --line-length 80 -i 's/a/b/' f`, []string{"f"}},
		{"unknown before in-place", `sed --unknown value -i 's/a/b/' f`, nil},
		{"unknown before clustered in-place", `sed -Q value -ni 's/a/b/' f`, nil},
		{"BSD capital in-place", `sed -I '' 's/a/b/' f`, []string{"f"}},
		{"BSD capital in-place in cluster", `sed -EI '' 's/a/b/' f`, []string{"f"}},
		{"BSD capital in-place attached suffix", `sed -I.bak 's/a/b/' f`, []string{"f"}},
		{"abbreviated long in-place", `sed --in-pl 's/a/b/' f`, []string{"f"}},
		{"abbreviated long in-place suffix", `sed --in-place=.bak 's/a/b/' f`, []string{"f"}},
		{"shortest long in-place", `sed --i 's/a/b/' f`, []string{"f"}},
		{"not an in-place abbreviation", `sed --in-x 's/a/b/' f`, nil},
		{"find exec plus", `find . -name '*.html' -exec sed -i '' 's|a|b|g' {} +`, nil},
		{"find exec semicolon", `find . -name '*.html' -exec sed -i '' 's|a|b|g' {} \;`, nil},
		{"xargs placeholder", `xargs -I{} sed -i 's/a/b/' {}`, nil},
		{"plus placeholder", `sed -i 's/a/b/' +`, nil},
		{"quoted semicolon placeholder", `sed -i 's/a/b/' ';'`, nil},
		{"escaped semicolon placeholder", `sed -i 's/a/b/' '\;'`, nil},
		{"backslash placeholder", `sed -i 's/a/b/' '\'`, nil},
		// Non-dot suffixes remain ambiguous candidates until the post-edit disk check.
		{"bare backup candidate", `sed -i bak 's/a/b/' f`, []string{"s/a/b", "f"}},
		{"tilde backup candidate", `sed -i '~' 's/a/b/' f`, []string{"s/a/b", "f"}},
		{"only dotted operand", `sed -e 's/a/b/' -i .env`, []string{".env"}},
		{"only dotted operand before flag", `sed -e 's/a/b/' -i .env -n`, []string{".env"}},
		{"only dotted operand before expression", `sed -i .env -e 's/a/b/'`, []string{".env"}},
		{"ambiguous dotted operand", `sed -e 's/a/b/' -i .env .bashrc`, []string{".bashrc"}},
		{"BSD empty suffix", `sed -i '' 's|a/b|c/d|g' dir/f.html`, []string{"dir/f.html"}},
		{"attached empty suffix", `sed -i'' 's|a/b|c/d|g' dir/f.html`, []string{"dir/f.html"}},
		{"empty implicit script", `sed -i '' '' f1 f2`, []string{"f1", "f2"}},
		{"empty explicit script", `sed -i '' -e '' f1 f2`, []string{"f1", "f2"}},
		{"attached backup and expressions", `sed -i.bak -e 's/x/y/' -e 's/p/q/' a.go b.go`, []string{"a.go", "b.go"}},
		{"BSD separate backup", `sed -i .bak 's|a/b|c/d|g' dir/f.html`, []string{"dir/f.html"}},
		{"BSD separate backup and expression", `sed -i .bak -e 's/x/y/' f`, []string{"f"}},
		{"GNU bare in-place", `sed -i -E 's#x#y#' f`, []string{"f"}},
		{"long backup and script file", `sed --in-place=.orig -f script.sed f1 f2`, []string{"f1", "f2"}},
		{"long in-place", `sed --in-place 's/a/b/' f`, []string{"f"}},
		{"long expression", `sed -i --expression 's/a b/c d/' f`, []string{"f"}},
		{"attached long expression", `sed -i --expression='s/a/b/' f`, []string{"f"}},
		{"long script file", `sed -i --file script.sed f`, []string{"f"}},
		{"attached long script file", `sed -i --file=script.sed f`, []string{"f"}},
		{"combined flags", `sed -nE -ri 's/a/b/' f`, []string{"f"}},
		{"no-file options", `sed --posix --debug -suz -i 's/a/b/' f`, []string{"f"}},
		{"combined expression", `sed -i -nEe 's/a/b/' f`, []string{"f"}},
		{"combined attached expression", `sed -i -nEes/a/b/ f`, []string{"f"}},
		{"combined script file", `sed -i -nf script.sed f`, []string{"f"}},
		{"combined attached script file", `sed -i -nfscript.sed f`, []string{"f"}},
		{"options after operand", `sed f -i -e 's/a/b/'`, []string{"f"}},
		{"end of options", `sed -i -e 's/a/b/' -- -f`, []string{"-f"}},
		{"redirection only", `sed 's/a/b/' f > out.txt`, []string{"out.txt"}},
		{"read only", `sed -n 's/a/b/p' f`, nil},
		{"chained cd", `cd sub && sed -i '' 's|a|b|' x.md`, []string{"sub/x.md"}},
		{"unknown short option", `sed -i -Q value 's/a/b/' f`, nil},
		{"unknown long option", `sed -i --unknown value 's/a/b/' f`, nil},
		{"unknown option contains i", `sed --mystery-in-place 's/a/b/' f`, nil},
		{"unknown clustered option", `sed -i -Qe 's/a/b/' f`, nil},
		{"missing expression", `sed -i -e`, nil},
		{"missing clustered expression", `sed -i f -ne`, nil},
		{"missing script file", `sed -i f --file`, nil},
		{"empty script file", `sed -i -f '' f1 f2`, nil},
		{"empty long script file", `sed -i --file= f1 f2`, nil},
		{"unterminated quote", `sed -i 's/a/b/ f`, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := bashWriteTargets(tt.command)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("bashWriteTargets(%q) = %q, want %q", tt.command, got, tt.want)
			}
		})
	}
}

func TestBashWriteTargetsDropsUnnamable(t *testing.T) {
	for _, tt := range []struct {
		name, command string
		want          []string
		sedDoubt      bool
	}{
		{"percent", `echo x > %`, nil, false},
		{"glob redirect", `echo x > out/*.txt`, nil, false},
		{"glob tee", `echo x | tee out/*.txt`, nil, false},
		{"placeholder", `cp src {}.txt`, nil, false},
		{"sed glob keeps named", `sed -i 's/a/b/' f *.go`, []string{"f"}, true},
		{"sed glob only", `sed -i 's/a/b/' src/*.go`, nil, true},
		{"sed question mark", `sed -i 's/a/b/' f? g`, []string{"g"}, true},
		{"sed bracket", `sed -i 's/a/b/' f[12]`, nil, true},
		{"sed placeholder", `sed -i 's/a/b/' {}.txt g`, []string{"g"}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := collectBashWriteTargets(tt.command)
			if !reflect.DeepEqual(c.paths, tt.want) {
				t.Fatalf("paths = %q, want %q", c.paths, tt.want)
			}
			if !c.writeShaped {
				t.Fatalf("writeShaped = false, want true")
			}
			if c.sedDoubt != tt.sedDoubt {
				t.Fatalf("sedDoubt = %v, want %v", c.sedDoubt, tt.sedDoubt)
			}
		})
	}
}

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

// TestBashWriteTargetsUnmatchedParenNeverPanics: an unmatched `(` or `$(`
// (a malformed or truncated command) consumes to the end of the command, as
// matchParenEnd documents, and never panics. A command ending in `(` used to
// slice past its own end, so the PostToolUse edit-hook exited 2 with a Go
// panic trace for Claude Code to show; an unmatched subshell also lost the
// last byte of its text, so `(cat > a.go` named `a.g`.
func TestBashWriteTargetsUnmatchedParenNeverPanics(t *testing.T) {
	cases := []struct {
		cmd         string
		wantPaths   []string
		wantWritten bool
	}{
		{"(", nil, false},
		{"echo a; (", nil, false},
		{"( (", nil, false},
		{"x=$(", nil, false},
		{"echo a && $(", nil, false},
		{"(cat > a.go", []string{"a.go"}, true},
		{"echo $(date) > b.txt; (cd internal && cat > c.go", []string{"b.txt", "internal/c.go"}, true},
	}
	for _, tc := range cases {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("bashWriteTargets(%q) panicked: %v", tc.cmd, r)
				}
			}()
			gotPaths, gotWritten := bashWriteTargets(tc.cmd)
			if !reflect.DeepEqual(gotPaths, tc.wantPaths) || gotWritten != tc.wantWritten {
				t.Errorf("bashWriteTargets(%q) = %v, %v; want %v, %v", tc.cmd, gotPaths, gotWritten, tc.wantPaths, tc.wantWritten)
			}
		}()
	}
}

func TestBashWriteTargetsSedUncertainInPlace(t *testing.T) {
	for _, command := range []string{
		`sed --unknown value -i 's/a/b/' f`,
		`sed -Q value -ni 's/a/b/' f`,
		`sed -Qi 's/a/b/' f`,
		`sed --unknown value --in-place=.bak 's/a/b/' f`,
		`sed --unknown -I '' 's/a/b/' f`,
		`find . -exec sed -i '' 's/a/b/' {} +`,
	} {
		t.Run(command, func(t *testing.T) {
			paths, write := bashWriteTargets(command)
			if len(paths) != 0 || !write {
				t.Fatalf("got %q, %v; want no paths and an uncertain write", paths, write)
			}
		})
	}
}
