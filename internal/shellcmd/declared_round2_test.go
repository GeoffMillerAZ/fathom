package shellcmd

import "testing"

func TestUnwrapSkipsInvalidDeclaration(t *testing.T) {
	for _, bad := range []Wrapper{{Prefix: "tool > output"}, {Prefix: "tool $ARG"}, {Prefix: "tool | other"}, {Command: "command-wrapper", ScriptArg: 1, CwdArg: 1}} {
		c := ParseCommand("uv run go test ./...", bad, Wrapper{Prefix: "uv run"})
		if c.Unreadable || len(c.Units()) != 1 || c.Units()[0].Name() != "go" {
			t.Errorf("bad declaration poisoned command: %+v: %+v", bad, c)
		}
	}
}

func TestDeclaredWrapperCwdUncertaintyIsInherited(t *testing.T) {
	wrappers := []Wrapper{{Command: "command-wrapper", ScriptArg: 1}, {Command: "directory-wrapper", ScriptArg: 2, CwdArg: 1}}
	for _, source := range []string{`command-wrapper 'go test ./...'`, `command-wrapper 'cd child && go test ./...'`, `command-wrapper "directory-wrapper child 'go test ./...'"`} {
		c := ParseCommand(source, wrappers...)
		if c.Unreadable {
			t.Fatal(c.Reason)
		}
		for _, u := range c.Units() {
			if !u.CwdUnknown {
				t.Errorf("%s: lost uncertainty on %+v", source, u)
			}
		}
		for _, u := range c.Transparent().Pipelines {
			for _, unit := range u.Units {
				if !unit.CwdUnknown {
					t.Errorf("projection lost uncertainty: %+v", unit)
				}
			}
		}
	}
	c := ParseCommand(`directory-wrapper child 'go test ./...'`, wrappers...)
	for _, u := range c.Units() {
		if u.CwdUnknown || u.Cwd != "child" {
			t.Errorf("declared cwd lost: %+v", u)
		}
	}
}

func TestWrapperRefusalIsTyped(t *testing.T) {
	c := ParseCommand(`command-wrapper "$SCRIPT"`, Wrapper{Command: "command-wrapper", ScriptArg: 1})
	if !c.Unreadable || c.Refusal != RefusalWrapper {
		t.Fatalf("wrapper refusal=%+v", c)
	}
	c = ParseCommand(`echo "`)
	if !c.Unreadable || c.Refusal != RefusalNone {
		t.Fatalf("ordinary syntax refusal=%+v", c)
	}
}

func TestWrapperScriptSubstitutionDependsOnOuterQuoting(t *testing.T) {
	wrapper := Wrapper{Command: "command-wrapper", ScriptArg: 1}
	c := ParseCommand(`command-wrapper 'echo "$(printf x)"'`, wrapper)
	if c.Unreadable {
		t.Fatalf("protected substitution became unreadable: %+v", c)
	}
	c = ParseCommand(`command-wrapper "echo $(printf x)"`, wrapper)
	if !c.Unreadable || c.Refusal != RefusalWrapper {
		t.Fatalf("outer expansion was read as literal: %+v", c)
	}
}
