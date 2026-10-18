package main

// discovery_shellreads_test.go — RO-29 (instrument B counts shell reads) and
// RO-4 (the callers of the one command lexer map `unreadable` explicitly).

import "testing"

func TestDiscoveryCountsShellReadsOfSourceFiles(t *testing.T) {
	cases := []struct {
		command string
		want    string
	}{
		{"", ""},
		{"cat internal/store/store.go", "shell-read"},
		{"head -40 cmd/fathom/main.go", "shell-read"},
		{"tail -n 20 a/b.py", "shell-read"},
		{"sed -n '10,80p' internal/x/y.go", "shell-read"},
		{"cd internal && cat store/store.go", "shell-read"},
		{"FOO=1 cat a/b.ts", "shell-read"},
		{"cat README.md", ""},        // not a source file
		{"cat $FILE.go", ""},         // path is computed
		{"cat *.go", ""},             // a glob names no file
		{"sed -n a/b.go", ""},        // no script
		{"sed -i s/a/b/ a/b.go", ""}, // not a read
		{"grep -rn foo .", "shell-search"},
		{"/usr/bin/grep -n foo bar", "shell-search"},
		{"FOO=bar grep -n foo bar", "shell-search"},
		{"env grep -n foo", "shell-search"},
		{"rg --hidden foo", "shell-search"},
		{"  find . -name '*.go'  ", "shell-search"},
		{"go test ./...", ""},
		{"git status", ""},
		{"eval \"$X\"", "unreadable"},
		{"echo 'unterminated", "unreadable"},
	}
	for _, c := range cases {
		if got := shellDiscoveryKind(c.command); got != c.want {
			t.Errorf("shellDiscoveryKind(%q) = %q, want %q", c.command, got, c.want)
		}
	}
	t.Run("a shell read emits one discovery beat", func(t *testing.T) {
		withTempBeatLog(t)
		emitDiscoveryBeatIfShellSearch("sed -n '1,40p' internal/store/store.go", "", false)
		beats := readBeatsForTest(t)
		if len(beats) != 1 || beats[0].Channel != "discovery" || beats[0].Kind != "shell-read" {
			t.Fatalf("beats = %+v", beats)
		}
	})
	t.Run("an unreadable command is counted as unreadable, not as zero reads", func(t *testing.T) {
		withTempBeatLog(t)
		emitDiscoveryBeatIfShellSearch("cat a.go; echo 'oops", "", false)
		beats := readBeatsForTest(t)
		if len(beats) != 1 || beats[0].Kind != "unreadable" {
			t.Fatalf("beats = %+v, want one unreadable beat", beats)
		}
	})
}

func TestUnreadableIsMappedByEachLexerCaller(t *testing.T) {
	const bad = "cat a/b.go; echo 'oops"
	t.Run("discovery observer records no read", func(t *testing.T) {
		if target, tool := bashObservedReadPath(bad); target != "" || tool != "" {
			t.Fatalf("bashObservedReadPath(%q) = (%q, %q), want none", bad, target, tool)
		}
	})
	t.Run("test-run matcher matches nothing", func(t *testing.T) {
		root := t.TempDir()
		if m, ok := matchDeclaredTestCommand(root, "go test ./... ; echo 'oops", []string{"go test"}); ok {
			t.Fatalf("matched %+v on an unreadable command", m)
		}
		if got := testRunScope("go test ./a ; echo 'oops", "go test"); got != "" {
			t.Fatalf("testRunScope on an unreadable command = %q, want empty", got)
		}
		if m, ok := matchDeclaredTestCommand(root, "go test ./a", []string{"go test"}); !ok {
			t.Fatalf("control: readable command did not match: %+v", m)
		}
	})
	t.Run("bash-write detector reads unreadable as doubt", func(t *testing.T) {
		paths, shaped := bashWriteTargets("cat a > out/x.go; echo 'oops")
		if !shaped {
			t.Fatalf("writeShaped = false, want doubt (paths %v)", paths)
		}
		if _, shaped := bashWriteTargets("tee 'oops"); !shaped {
			t.Fatal("an unreadable tee argument list must stay doubt")
		}
	})
}
