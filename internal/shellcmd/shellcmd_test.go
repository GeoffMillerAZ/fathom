package shellcmd

import (
	"reflect"
	"strings"
	"testing"
)

// TestParseCommandRefusesWhatItCannotRead is RO-3's contract: what the lexer
// cannot read is unreadable with a named cause and no pipelines, never a
// guess; the shapes it can read stay readable.
func TestParseCommandRefusesWhatItCannotRead(t *testing.T) {
	refused := []struct{ cmd, reason string }{
		{`echo 'unterminated`, "unterminated '"},
		{`echo "unterminated`, `unterminated "`},
		{"echo $(ls", "unterminated $("},
		{"echo `ls", "unterminated `"},
		{"echo ${HOME", "unterminated ${"},
		{"echo $((1+2)", "unterminated $(("},
		{"cat <<EOF\nbody\n", "unterminated heredoc"},
		{"cat <<", "heredoc without a delimiter"},
		{"if true; then echo", "missing"},
		{"while true; do echo x", "missing `done`"},
		{"case x in a) echo a ;;", "missing `esac`"},
		{"( echo a", "missing `)`"},
		{"{ echo a; ", "missing `}`"},
		{"echo a )", "unexpected `)`"},
		{"&& echo a", "unexpected `&&`"},
		{"echo a &&", "nothing after `&&`"},
		{"echo a |", "nothing after `|`"},
		{"echo a ;; echo b", "unexpected `;;`"},
		{"$CMD arg", "command name is not literal"},
		{"$(which go) test ./...", "command name is not literal"},
		{"./bin/run* x", "command name is not literal"},
		{`bash -c "$SCRIPT"`, "shell -c script is not literal"},
		{"bash -c", "shell -c without a script"},
		{`bash -c 'echo "unterminated'`, "in shell -c"},
		{`eval "ls -la"`, "eval"},
		{"env -S 'a b' cmd", "env -S"},
		{"f() { echo hi; }", "function definition"},
		{"function f { echo hi; }", "function definition"},
		{"echo $(case x in a) echo a;; esac)", "case inside a command substitution"},
		{"echo a\x00b", "NUL byte"},
		{"echo " + strings.Repeat("$(", 20) + "x" + strings.Repeat(")", 20), "nested too deep"},
		{"echo > ", "without a target"},
		{strings.Repeat("( ", 100) + "ls" + strings.Repeat(" )", 100), "nested too deep"},
		{"a=(1 2 3)", "unexpected `(`"},
		{"(( i++ ", "unterminated (("},
		{"[[ -f x ", "missing `]]`"},
	}
	for _, tc := range refused {
		c := ParseCommand(tc.cmd)
		if !c.Unreadable {
			t.Errorf("%q: read, want unreadable (%s)", tc.cmd, tc.reason)
			continue
		}
		if !strings.Contains(c.Reason, tc.reason) {
			t.Errorf("%q: reason %q, want it to name %q", tc.cmd, c.Reason, tc.reason)
		}
		if c.Pipelines != nil {
			t.Errorf("%q: unreadable but carries pipelines", tc.cmd)
		}
	}

	readable := []string{
		"",
		"# only a comment",
		"go test ./... 2>&1 | tail -n 30",
		"case $x in a|b) echo ab ;; *) echo other ;; esac",
		"case $x in\n  (a) echo a\n  ;;\n  b) ;;\nesac",
		"git status --porcelain | grep '^ M' | wc -l",
		"git log --oneline -z | xargs -0 echo",
		"cat <<'EOF' > out.txt\n$HOME stays literal\nEOF\n",
		"if [[ -f go.mod && -d .git ]]; then go build ./...; elif true; then :; else echo no; fi",
		"for f in *.go; do gofmt -l \"$f\"; done > list.txt",
		"for ((i=0; i<3; i++)); do echo $i; done",
		"while read -r l; do echo \"$l\"; done < file",
		"(( n > 2 )) && echo big",
		"sleep 5 &",
		"diff <(sort a) <(sort b)",
		"x=$(git rev-parse HEAD) && echo \"$x\"",
		"echo `date +%s`",
		"echo $'tab\\there' $\"locale\"",
		"echo a\\\nb",
		"! grep -q x file",
		"{ echo a; echo b; } 2>/dev/null | sort",
		"command -v go",
		"sudo -v",
		"env",
		"> truncated.txt",
		"FOO=1",
	}
	for _, cmd := range readable {
		if c := ParseCommand(cmd); c.Unreadable {
			t.Errorf("%q: unreadable (%s), want read", cmd, c.Reason)
		}
	}
}

func units(c Command) []string {
	var out []string
	for _, u := range c.Units() {
		s := u.Kind.String()
		if u.Keyword != "" {
			s += ":" + u.Keyword
		}
		if len(u.Wrappers) > 0 {
			s += "[" + strings.Join(u.Wrappers, ",") + "]"
		}
		if len(u.Words) > 0 {
			s += " " + strings.Join(u.Args(), " ")
		}
		out = append(out, s)
	}
	return out
}

func seps(c Command) []string {
	var out []string
	for _, p := range c.Pipelines {
		out = append(out, p.Sep)
	}
	return out
}

func mustParse(t *testing.T, cmd string) Command {
	t.Helper()
	c := ParseCommand(cmd)
	if c.Unreadable {
		t.Fatalf("%q: unreadable: %s", cmd, c.Reason)
	}
	return c
}

func TestParseCommandSegmentsOnEveryOperator(t *testing.T) {
	c := mustParse(t, "cd x && go test ./... || echo failed; ls & wait\nmake")
	if got, want := seps(c), []string{"&&", "||", ";", "&", "\n", ""}; !reflect.DeepEqual(got, want) {
		t.Fatalf("seps = %q, want %q", got, want)
	}
	if got, want := units(c), []string{"simple cd x", "simple go test ./...", "simple echo failed", "simple ls", "simple wait", "simple make"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("units = %q, want %q", got, want)
	}
}

func TestParseCommandReadsPipesAndRedirects(t *testing.T) {
	c := mustParse(t, "go test ./... 2>&1 |& tee log.txt | tail -n 30 >> out 2>/dev/null")
	if len(c.Pipelines) != 1 {
		t.Fatalf("pipelines = %d, want 1", len(c.Pipelines))
	}
	p := c.Pipelines[0]
	if got, want := p.Pipes, []string{"|&", "|"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("pipes = %q, want %q", got, want)
	}
	r := p.Units[0].Redirects
	if len(r) != 1 || r[0].Fd != 2 || r[0].Op != ">&" || r[0].Target.Text != "1" {
		t.Fatalf("first unit redirects = %+v, want 2>&1", r)
	}
	r = p.Units[2].Redirects
	if len(r) != 2 || r[0].Fd != -1 || r[0].Op != ">>" || r[0].Target.Text != "out" || r[1].Fd != 2 || r[1].Target.Text != "/dev/null" {
		t.Fatalf("tail redirects = %+v", r)
	}
}

func TestParseCommandReadsCaseArms(t *testing.T) {
	c := mustParse(t, "case \"$1\" in\n  start|up) systemctl start x ;;\n  stop) systemctl stop x ;&\n  *) echo usage ;;\nesac")
	us := c.Units()
	if len(us) != 4 || us[0].Kind != KindCompound || us[0].Keyword != "case" {
		t.Fatalf("units = %q", units(c))
	}
	if got, want := seps(*us[0].Body), []string{";;", ";&", ";;"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("arm seps = %q, want %q", got, want)
	}
	if got := units(c)[3]; got != "simple echo usage" {
		t.Fatalf("last arm = %q", got)
	}
}

func TestParseCommandReadsHeredocsAndHereStrings(t *testing.T) {
	c := mustParse(t, "cat <<-EOF > a.txt && cat <<'END' | wc -l\n\tline $HOME\n\tEOF\nraw $x\nEND\ngrep x <<< \"$v\"")
	us := c.Units()
	if len(us) != 4 {
		t.Fatalf("units = %q", units(c))
	}
	h := us[0].Redirects[0].Heredoc
	if h == nil || h.Delim != "EOF" || !h.StripTabs || h.Quoted || h.Body != "line $HOME\n" || !h.Dynamic {
		t.Fatalf("first heredoc = %+v", h)
	}
	if r := us[0].Redirects[1]; r.Op != ">" || r.Target.Text != "a.txt" {
		t.Fatalf("first redirect after heredoc = %+v", r)
	}
	h = us[1].Redirects[0].Heredoc
	if h == nil || h.Delim != "END" || !h.Quoted || h.Body != "raw $x\n" || h.Dynamic {
		t.Fatalf("second heredoc = %+v", h)
	}
	if r := us[3].Redirects[0]; r.Op != "<<<" || r.Target.Text != "$v" || !r.Target.Dynamic {
		t.Fatalf("here-string = %+v", r)
	}
}

func TestParseCommandPeelsWrappersAndReadsShellScripts(t *testing.T) {
	c := mustParse(t, `FATHOM_READOUT=raw sudo -u root GOFLAGS=-mod=mod env -i A=1 timeout -s KILL 60 nice -n 5 go test ./...`)
	u := c.Units()[0]
	if got, want := u.Wrappers, []string{"sudo", "env", "timeout", "nice"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("wrappers = %q, want %q", got, want)
	}
	if got, want := u.Args(), []string{"go", "test", "./..."}; !reflect.DeepEqual(got, want) {
		t.Fatalf("argv = %q, want %q", got, want)
	}
	var names []string
	for _, a := range u.Env {
		names = append(names, a.Name+"="+a.Value.Text)
	}
	if want := []string{"FATHOM_READOUT=raw", "GOFLAGS=-mod=mod", "A=1"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("env = %q, want %q", names, want)
	}
	if got := c.ReadoutPrefix(); got != "raw" {
		t.Fatalf("ReadoutPrefix = %q, want raw", got)
	}

	c = mustParse(t, `bash -lc 'cd x && go vet ./...' ; rtk git status`)
	if got, want := units(c), []string{"shell bash -lc cd x && go vet ./...", "simple cd x", "simple go vet ./...", "simple[rtk] git status"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("units = %q, want %q", got, want)
	}
	if !c.Rtk() {
		t.Fatal("Rtk() = false, want true")
	}
	if c := mustParse(t, "command -v go"); c.Units()[0].Name() != "command" || len(c.Units()[0].Wrappers) != 0 {
		t.Fatalf("command -v is a query, not a wrapper: %q", units(c))
	}
	if c := mustParse(t, "bash script.sh arg"); c.Units()[0].Kind != KindSimple {
		t.Fatalf("bash without -c runs a file: %q", units(c))
	}
}

func TestParseCommandWordsKeepExpansionsAsSourceText(t *testing.T) {
	c := mustParse(t, `echo "$HOME/x" '$HOME' a\ b *.go {a,b} "lit" $(git rev-parse HEAD)`)
	u := c.Units()[0]
	type w struct {
		text                   string
		quoted, dynamic, globs bool
	}
	var got []w
	for _, x := range u.Words {
		got = append(got, w{x.Text, x.Quoted, x.Dynamic, x.Glob})
	}
	want := []w{
		{"echo", false, false, false},
		{"$HOME/x", true, true, false},
		{"$HOME", true, false, false},
		{"a b", true, false, false},
		{"*.go", false, false, true},
		{"{a,b}", false, false, true},
		{"lit", true, false, false},
		{"$(git rev-parse HEAD)", false, true, false},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("words = %+v\nwant    %+v", got, want)
	}
	if len(u.Subst) != 1 || units(*u.Subst[0])[0] != "simple git rev-parse HEAD" {
		t.Fatalf("subst = %+v", u.Subst)
	}
}

func TestParseCommandIsDeterministic(t *testing.T) {
	for _, s := range fuzzSeeds {
		a, b := ParseCommand(s), ParseCommand(s)
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("%q: two parses differ", s)
		}
	}
}
