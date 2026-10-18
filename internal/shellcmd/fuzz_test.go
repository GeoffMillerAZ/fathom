package shellcmd

import (
	"reflect"
	"testing"
)

// fuzzSeeds is the in-code half of the fuzz corpus (the other half is
// testdata/fuzz/FuzzParseCommand): the shapes a sloppy lexer gets wrong
// (rtk #3197's `;;`), porcelain and NUL-separated output, and pipes.
var fuzzSeeds = []string{
	"case $x in a) echo a ;; b|c) echo bc ;& *) ;; esac",
	"case \"$1\" in\n  (start) run ;;\n  *) usage ;;\nesac",
	"git status --porcelain=v1 -z | xargs -0 -n1 echo",
	"git diff --name-only HEAD~1 | grep '\\.go$' | xargs gofmt -l",
	"go test -json ./... | jq -r 'select(.Action==\"fail\")'",
	"go test ./... 2>&1 | tail -n 30",
	"ls -la | head -20",
	"cd internal && rg -n 'func ' --type go | sort | uniq -c",
	"cat <<'EOF' | bash\necho $HOME\nEOF\n",
	"bash -c 'for f in *.go; do echo \"$f\"; done'",
	"sudo -u root FOO=1 make install >/dev/null 2>&1 &",
	"FATHOM_READOUT=raw git log --oneline -n 50",
	"rtk git diff --stat",
	"x=$( (cd a; ls) | wc -l ) && echo $x",
	"echo `echo \\`nested\\``",
	"if [[ $a == b* && -n \"$c\" ]]; then :; fi",
	"(( i = 1 << 2 )); echo $((i * 3))",
	"diff <(git show HEAD:a.go) a.go",
	"echo ${var:-$(date)} ${#arr[@]} $'\\x41\\n'",
	"a=1 b=2 env -u c -- printenv a",
	"while :; do sleep 1; done & wait $!",
	"echo 'it''s' \"q\\\"q\" \\# not-a-comment # comment",
	"f() { :; }",
	"cat <<EOF",
	"echo $(",
}

// FuzzParseCommand holds the lexer's safety properties over any input: no
// panic, a deterministic result, a reason exactly when unreadable, no
// pipelines when unreadable, and a known Kind on every unit.
func FuzzParseCommand(f *testing.F) {
	for _, s := range fuzzSeeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		a := ParseCommand(s)
		if b := ParseCommand(s); !reflect.DeepEqual(a, b) {
			t.Fatalf("%q: two parses differ", s)
		}
		if a.Unreadable != (a.Reason != "") {
			t.Fatalf("%q: unreadable=%v reason=%q", s, a.Unreadable, a.Reason)
		}
		if a.Unreadable && a.Pipelines != nil {
			t.Fatalf("%q: unreadable with pipelines", s)
		}
		for _, u := range a.Units() {
			if u.Kind.String() == "unknown" {
				t.Fatalf("%q: unit of unknown kind %d", s, u.Kind)
			}
			if u.Kind == KindShell && u.Body == nil {
				t.Fatalf("%q: shell unit without a body", s)
			}
			if (u.Kind == KindSimple || u.Kind == KindShell) && len(u.Words) > 0 && !u.Words[0].Literal() {
				t.Fatalf("%q: readable unit with a non-literal command name", s)
			}
		}
	})
}
