package shellcmd

import (
	"reflect"
	"strings"
	"testing"
)

func TestDeclaredWrappers(t *testing.T) {
	ws := []Wrapper{{Prefix: "uv run"}, {Prefix: "bundle exec"}, {Command: "command-wrapper", ScriptArg: 2}, {Command: "directory-wrapper", ScriptArg: 2, CwdArg: 1}}
	for _, tc := range []struct {
		name, source string
		want         []string
		cwd          string
	}{
		{"prefix", `uv run go test ./...`, []string{"go test ./..."}, ""},
		{"second prefix", `bundle exec go test ./...`, []string{"go test ./..."}, ""},
		{"prefix boundary", `uv runner go test ./...`, []string{"uv runner go test ./..."}, ""},
		{"quoted argv", `uv run go test './a b'`, []string{"go test ./a b"}, ""},
		{"single quoted list", `command-wrapper label 'go test ./a && go build ./b'`, []string{"go test ./a", "go build ./b"}, ""},
		{"double quoted list", `command-wrapper label "go test ./a && go build ./b"`, []string{"go test ./a", "go build ./b"}, ""},
		{"nested", `command-wrapper label "command-wrapper label 'go test ./...'"`, []string{"go test ./..."}, ""},
		{"mixed prefixes", `uv run env FOO=bar timeout 30 nice -n 2 rtk go test ./...`, []string{"go test ./..."}, ""},
		{"shell after prefix", `uv run bash -c 'go test ./...'`, []string{"go test ./..."}, ""},
		{"cwd", `directory-wrapper ./child 'go test ./...'`, []string{"go test ./..."}, "child"},
		{"nested cwd", `directory-wrapper child "directory-wrapper grandchild 'go test ./...'"`, []string{"go test ./..."}, "child/grandchild"},
		{"cwd does not leak", `directory-wrapper child 'go test ./a'; go test ./b`, []string{"go test ./a", "go test ./b"}, ""},
		{"quoted data", `echo "command-wrapper label 'go test ./...'"`, []string{"echo command-wrapper label 'go test ./...'"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := ParseCommand(tc.source, ws...)
			if c.Unreadable {
				t.Fatal(c.Reason)
			}
			var got []string
			lastCwd := ""
			for _, u := range c.Units() {
				if u.Kind == KindSimple {
					got = append(got, strings.Join(u.Args(), " "))
					lastCwd = u.Cwd
				}
			}
			if !reflect.DeepEqual(got, tc.want) || lastCwd != tc.cwd {
				t.Fatalf("commands=%q cwd=%q; want %q cwd=%q", got, lastCwd, tc.want, tc.cwd)
			}
			if other := ParseCommand(tc.source, ws...); !reflect.DeepEqual(c, other) {
				t.Fatal("wrapper parsing is nondeterministic")
			}
		})
	}
}

func TestDeclaredWrappersRefuseUnreadableScripts(t *testing.T) {
	ws := []Wrapper{{Command: "command-wrapper", ScriptArg: 1}, {Command: "directory-wrapper", ScriptArg: 2, CwdArg: 1}}
	for _, tc := range []struct{ source, reason string }{
		{`command-wrapper`, "missing"},
		{`command-wrapper "$SCRIPT"`, "not literal"},
		{`command-wrapper 'echo "oops'`, "in wrapper command-wrapper: unterminated"},
		{`command-wrapper "command-wrapper \"command-wrapper 'go test ./...'\""`, "depth exceeds 2"},
		{`directory-wrapper "$DIRECTORY" 'go test ./...'`, "cwd argument"},
	} {
		c := ParseCommand(tc.source, ws...)
		if !c.Unreadable || !strings.Contains(c.Reason, tc.reason) || len(c.Pipelines) != 0 {
			t.Errorf("%q: unreadable=%v reason=%q", tc.source, c.Unreadable, c.Reason)
		}
	}
}

func TestDeclaredPrefixPreservesLexerRefusals(t *testing.T) {
	for _, src := range []string{`uv run "$COMMAND" test`, `uv run eval 'go test ./...'`} {
		c := ParseCommand(src, Wrapper{Prefix: "uv run"})
		if !c.Unreadable || c.Reason == "" {
			t.Errorf("prefix hid an unreadable command: %q: %+v", src, c)
		}
	}
}

func TestUnwrapDoesNotMutateInput(t *testing.T) {
	c := ParseCommand(`uv run go test ./...`)
	before := ParseCommand(`uv run go test ./...`)
	_ = Unwrap(c, []Wrapper{{Prefix: "uv run"}})
	if !reflect.DeepEqual(c, before) {
		t.Fatal("Unwrap mutated its caller's command")
	}
	c = ParseCommand(`command-wrapper 'go test ./...'`, Wrapper{Command: "command-wrapper", ScriptArg: 1})
	before = ParseCommand(`command-wrapper 'go test ./...'`, Wrapper{Command: "command-wrapper", ScriptArg: 1})
	_ = c.Transparent()
	if !reflect.DeepEqual(c, before) {
		t.Fatal("Transparent mutated its caller's command")
	}
}

func TestTransparentScriptKeepsBackgroundBoundary(t *testing.T) {
	c := ParseCommand(`command-wrapper 'go test ./... &' && echo done`, Wrapper{Command: "command-wrapper", ScriptArg: 1}).Transparent()
	if c.Unreadable || len(c.Pipelines) != 2 || c.Pipelines[0].Sep != "&" {
		t.Fatalf("projection lost the script's background boundary: %+v", c)
	}
}
