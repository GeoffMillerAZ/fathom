package main

// skillsschema_test.go — RP-2 (docs/plans/reach-parity.md): every shipped
// skill's frontmatter, as the `.agents/skills` rendering (agentsVariant)
// AND as the embedded source, satisfies the vendored Agent Skills schema
// (schema/skills/schema.cue), and `name` equals the directory the spec
// requires it to. The hard assertion runs through the CUE Go API (always);
// the `cue vet` binary pass runs when `cue` is installed and skips with a
// named reason otherwise, matching `task cue-vet`'s own
// "cue not installed — skipping".

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	cueyaml "cuelang.org/go/encoding/yaml"

	skillsdist "github.com/geoffmilleraz/fathom/dist/agents/skills"
)

func skillsSchemaSource(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join("..", "..", "schema", "skills", name)
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	return string(data)
}

// frontmatterOf returns the YAML between the leading `---` pair.
func frontmatterOf(t *testing.T, content []byte) string {
	t.Helper()
	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 || lines[0] != "---" {
		t.Fatalf("no leading frontmatter line")
	}
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			return strings.Join(lines[1:i], "\n") + "\n"
		}
	}
	t.Fatalf("frontmatter not closed")
	return ""
}

func TestShippedSkillFrontmatterVets(t *testing.T) {
	cctx := cuecontext.New()
	schema := cctx.CompileString(skillsSchemaSource(t, "schema.cue"))
	if err := schema.Err(); err != nil {
		t.Fatalf("schema does not compile: %v", err)
	}
	def := schema.LookupPath(cue.ParsePath("#Frontmatter"))
	if err := def.Err(); err != nil {
		t.Fatalf("#Frontmatter missing: %v", err)
	}
	cueBin, cueErr := exec.LookPath("cue")

	for _, name := range skillsdist.Names {
		raw, err := skillsdist.FS.ReadFile(name + "/SKILL.md")
		if err != nil {
			t.Fatal(err)
		}
		agents, err := agentsVariant(name, raw)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for label, content := range map[string][]byte{"source": raw, "agents": agents} {
			fm := frontmatterOf(t, content)
			file, err := cueyaml.Extract(name+"."+label+".yaml", []byte(fm))
			if err != nil {
				t.Fatalf("%s (%s): frontmatter is not YAML: %v", name, label, err)
			}
			data := cctx.BuildFile(file)
			if err := data.Err(); err != nil {
				t.Fatalf("%s (%s): %v", name, label, err)
			}
			if err := def.Unify(data).Validate(cue.Concrete(true)); err != nil {
				t.Errorf("%s (%s) frontmatter does not satisfy #Frontmatter: %v\n%s", name, label, err, fm)
			}
			got, _ := data.LookupPath(cue.ParsePath("name")).String()
			if got != name {
				t.Errorf("%s (%s): frontmatter name %q must equal the directory name (agentskills.io)", name, label, got)
			}
			if cueErr != nil {
				continue
			}
			dir := t.TempDir()
			fmPath := filepath.Join(dir, "frontmatter.yaml")
			if err := os.WriteFile(fmPath, []byte(fm), 0o644); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(cueBin, "vet", "-d", "#Frontmatter", filepath.Join("..", "..", "schema", "skills", "schema.cue"), fmPath)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Errorf("%s (%s): cue vet failed: %v\n%s", name, label, err, out)
			}
		}
	}
	if cueErr != nil {
		t.Logf("cue not installed — skipping the binary pass (the Go API pass above already ran)")
	}
}

// TestSkillsSchema_Negatives proves the commented cases in
// schema/skills/testdata/invalid.cue actually fail, and valid.cue unifies.
func TestSkillsSchema_Negatives(t *testing.T) {
	schema := skillsSchemaSource(t, "schema.cue")
	cctx := cuecontext.New()

	valid := skillsSchemaSource(t, filepath.Join("testdata", "valid.cue"))
	if v := cctx.CompileString(schema + "\n" + stripCUEPackageLine(valid)); v.Err() != nil || v.Validate(cue.Concrete(true)) != nil {
		t.Fatalf("valid.cue does not unify with the schema: %v / %v", v.Err(), v.Validate(cue.Concrete(true)))
	}
	cases := []struct{ name, bad string }{
		{"uppercase name", `bad: #Frontmatter & {name: "PDF-Processing", description: "d"}`},
		{"leading hyphen", `bad: #Frontmatter & {name: "-pdf", description: "d"}`},
		{"double hyphen", `bad: #Frontmatter & {name: "pdf--processing", description: "d"}`},
		{"empty description", `bad: #Frontmatter & {name: "pdf", description: ""}`},
		{"extension key", `bad: #Frontmatter & {name: "pdf", description: "d", paths: "cmd/**"}`},
		{"non-string metadata", `bad: #Frontmatter & {name: "pdf", description: "d", metadata: {version: 1}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := cctx.CompileString(schema + "\n" + tc.bad)
			if v.Err() == nil && v.Validate(cue.Concrete(true)) == nil {
				t.Fatalf("expected %s to fail, but it unified", tc.name)
			}
		})
	}
}

func stripCUEPackageLine(src string) string {
	var out []string
	for _, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "package ") {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
