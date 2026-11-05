package iacfacts

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"testing/fstest"
)

// IAC4: sensitive and ephemeral are two flags. An override replaces the flag it names and leaves the other, so a
// variable is secret when either flag is on after the override. The reader held one boolean and wrote it last, so an
// override with `sensitive = true` and `ephemeral = false` left the default readable, and so did a base `sensitive =
// true` under an override `ephemeral = false`. Real OpenTofu 1.13 keeps sensitive in both.
var overrideFlagCases = []struct {
	name string
	base string // the flag lines of the base declaration
	over map[string]string
	// secret is whether the default is withheld, and what OpenTofu reports (sensitive or ephemeral).
	secret bool
}{
	{"hcl sensitive true, ephemeral false", "", map[string]string{"x_override.tf": "variable \"v\" {\n  sensitive = true\n  ephemeral = false\n}\n"}, true},
	{"json sensitive true, ephemeral false", "", map[string]string{"x_override.tf.json": `{"variable": {"v": {"sensitive": true, "ephemeral": false}}}`}, true},
	{"base sensitive, override ephemeral false", "  sensitive = true\n", map[string]string{"x_override.tf": "variable \"v\" {\n  ephemeral = false\n}\n"}, true},
	{"base ephemeral, override sensitive false", "  ephemeral = true\n", map[string]string{"x_override.tf": "variable \"v\" {\n  sensitive = false\n}\n"}, true},
	{"json base sensitive, override ephemeral false", "  sensitive = true\n", map[string]string{"x_override.tf.json": `{"variable": {"v": {"ephemeral": false}}}`}, true},
	{"json base ephemeral, override sensitive false", "  ephemeral = true\n", map[string]string{"x_override.tf.json": `{"variable": {"v": {"sensitive": false}}}`}, true},
	{"base both, override sensitive false", "  sensitive = true\n  ephemeral = true\n", map[string]string{"x_override.tf": "variable \"v\" {\n  sensitive = false\n}\n"}, true},
	// Controls: every flag that is on is cleared by the override.
	{"base sensitive, override sensitive false", "  sensitive = true\n", map[string]string{"x_override.tf": "variable \"v\" {\n  sensitive = false\n}\n"}, false},
	{"base both, override both false", "  sensitive = true\n  ephemeral = true\n", map[string]string{"x_override.tf": "variable \"v\" {\n  sensitive = false\n  ephemeral = false\n}\n"}, false},
	{"json base both, override both false", "  sensitive = true\n  ephemeral = true\n", map[string]string{"x_override.tf.json": `{"variable": {"v": {"sensitive": false, "ephemeral": false}}}`}, false},
}

func TestReadTerraformOverrideFlagsAreHeldApart(t *testing.T) {
	for _, c := range overrideFlagCases {
		files := map[string]string{"main.tf": sprintfFlags(c.base)}
		for k, v := range c.over {
			files[k] = v
		}
		m := fstest.MapFS{}
		for k, v := range files {
			m[k] = file(v)
		}
		set, err := ReadTerraformModule(m, ".", WithValues())
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		a := attrOf(t, set.Resources[0], "x")
		if withheld := a.Value == nil; withheld != c.secret {
			t.Errorf("%s: default withheld = %v, want %v (%+v)", c.name, withheld, c.secret, a)
		}
	}
}

func sprintfFlags(flags string) string {
	return "variable \"v\" {\n  default = \"hunter2\"\n" + flags + "}\nresource \"t\" \"r\" {\n  x = var.v\n}\n"
}

// The table is what OpenTofu says. It runs `tofu show -json -config` on each case when tofu is installed.
func TestOverrideFlagsMatchOpenTofu(t *testing.T) {
	tofu, err := exec.LookPath("tofu")
	if err != nil {
		t.Skip("tofu is not installed")
	}
	for _, c := range overrideFlagCases {
		dir := t.TempDir()
		// No resource: a resource of an unknown type would make init fetch a provider.
		files := map[string]string{"main.tf": "variable \"v\" {\n  default = \"hunter2\"\n" + c.base + "}\n"}
		for k, v := range c.over {
			files[k] = v
		}
		for k, v := range files {
			if err := os.WriteFile(filepath.Join(dir, k), []byte(v), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		run := func(args ...string) []byte {
			cmd := exec.Command(tofu, args...)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "CHECKPOINT_DISABLE=1", "TF_IN_AUTOMATION=1")
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("%s: tofu %v: %v\n%s%s", c.name, args, err, out, stderr.String())
			}
			return out
		}
		run("init", "-backend=false", "-input=false", "-no-color")
		var show struct {
			RootModule struct {
				Variables map[string]struct {
					Sensitive bool `json:"sensitive"`
					Ephemeral bool `json:"ephemeral"`
				} `json:"variables"`
			} `json:"root_module"`
			Configuration struct {
				RootModule struct {
					Variables map[string]struct {
						Sensitive bool `json:"sensitive"`
						Ephemeral bool `json:"ephemeral"`
					} `json:"variables"`
				} `json:"root_module"`
			} `json:"configuration"`
		}
		if err := json.Unmarshal(run("show", "-json", "-config", "-no-color"), &show); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		v, ok := show.RootModule.Variables["v"]
		if !ok {
			cv := show.Configuration.RootModule.Variables["v"]
			v.Sensitive, v.Ephemeral = cv.Sensitive, cv.Ephemeral
		}
		if got := v.Sensitive || v.Ephemeral; got != c.secret {
			t.Errorf("%s: tofu says secret = %v (sensitive %v, ephemeral %v), the table says %v", c.name, got, v.Sensitive, v.Ephemeral, c.secret)
		}
	}
}
