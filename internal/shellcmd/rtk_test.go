package shellcmd

import (
	"reflect"
	"testing"
)

func TestRtkPrefix(t *testing.T) {
	for _, tt := range []struct {
		src      string
		args     []string
		wrappers []string
	}{
		{`rtk git status`, []string{"git", "status"}, []string{"rtk"}},
		{`rtk go test ./...`, []string{"go", "test", "./..."}, []string{"rtk"}},
		{`env X=1 rtk go test ./...`, []string{"go", "test", "./..."}, []string{"env", "rtk"}},
		{`sudo rtk git status`, []string{"git", "status"}, []string{"sudo", "rtk"}},
		{`rtk --skip-env go test ./...`, []string{"go", "test", "./..."}, []string{"rtk"}},
		{`rtk -v --skip-env --verbose go test ./...`, []string{"go", "test", "./..."}, []string{"rtk"}},
		{`rtk env -f AWS`, []string{"rtk", "env", "-f", "AWS"}, nil},
		{`/usr/bin/rtk read file.txt`, []string{"/usr/bin/rtk", "read", "file.txt"}, nil},
		{`rtk --verbose go test ./...`, []string{"go", "test", "./..."}, []string{"rtk"}},
		{`rtk -vv git status`, []string{"git", "status"}, []string{"rtk"}},
		{`rtk -- git status`, []string{"git", "status"}, []string{"rtk"}},
		{`rtk`, []string{"rtk"}, nil},
		{`rtk gain`, []string{"rtk", "gain"}, nil},
		{`rtk --version`, []string{"rtk", "--version"}, nil},
		{`rtk --help git`, []string{"rtk", "--help", "git"}, nil},
		{`rtk --unknown go test`, []string{"rtk", "--unknown", "go", "test"}, nil},
		{`rtk "$TOOL" test`, []string{"rtk", "$TOOL", "test"}, nil},
	} {
		t.Run(tt.src, func(t *testing.T) {
			c := ParseCommand(tt.src)
			if c.Unreadable || len(c.Units()) != 1 {
				t.Fatalf("ParseCommand = %+v", c)
			}
			u := c.Units()[0]
			if got := u.Args(); !reflect.DeepEqual(got, tt.args) {
				t.Errorf("args = %q, want %q", got, tt.args)
			}
			if !reflect.DeepEqual(u.Wrappers, tt.wrappers) {
				t.Errorf("wrappers = %q, want %q", u.Wrappers, tt.wrappers)
			}
			wantRtk := true
			if c.Rtk() != wantRtk {
				t.Errorf("Rtk = %v, want %v", c.Rtk(), wantRtk)
			}
			if tt.src == `env X=1 rtk go test ./...` && (len(u.Env) != 1 || u.Env[0].Name != "X" || u.Env[0].Value.Text != "1") {
				t.Errorf("environment lost: %+v", u.Env)
			}
		})
	}
}

func TestRtkOwnCommandsStayUnpeeled(t *testing.T) {
	for _, sub := range []string{
		"read", "smart", "json", "deps", "env", "log", "ls", "tree", "find", "grep", "diff", "wc",
		"help", "gain", "discover", "learn", "cc-economics", "hook-audit", "init", "config", "rewrite", "verify", "recall", "telemetry", "session", "llm-proxy",
	} {
		t.Run(sub, func(t *testing.T) {
			c := ParseCommand("rtk --verbose " + sub + " git status")
			if c.Unreadable || len(c.Units()) != 1 || c.Units()[0].Name() != "rtk" || c.Units()[0].HasWrapper("rtk") || !c.Rtk() {
				t.Fatalf("RTK own command was peeled or not recognised: %+v", c)
			}
		})
	}
	for _, sub := range []string{"test", "err", "summary"} {
		t.Run(sub, func(t *testing.T) {
			c := ParseCommand("rtk --skip-env " + sub + " go test ./...")
			if c.Unreadable || len(c.Units()) != 1 || !reflect.DeepEqual(c.Units()[0].Args(), []string{"go", "test", "./..."}) || !c.Rtk() {
				t.Fatalf("RTK wrapping subcommand was not peeled: %+v", c)
			}
			for _, suffix := range []string{"", ` "$TOOL" test`, " --unknown go test"} {
				c := ParseCommand("rtk " + sub + suffix)
				if c.Unreadable || len(c.Units()) != 1 || c.Units()[0].Name() != "rtk" || c.Units()[0].HasWrapper("rtk") || !c.Rtk() {
					t.Fatalf("missing or unknown wrapped command was guessed: %+v", c)
				}
			}
		})
	}
	for _, sub := range []string{"hook", "proxy", "bypass", "run", "custom-command"} {
		t.Run(sub, func(t *testing.T) {
			c := ParseCommand("rtk " + sub + " arg")
			if c.Unreadable || len(c.Units()) != 1 || !reflect.DeepEqual(c.Units()[0].Args(), []string{sub, "arg"}) || !c.Rtk() {
				t.Fatalf("direct wrapped command was not peeled: %+v", c)
			}
		})
	}
}
