package main

// pluginschema_test.go — RP-6/RP-7 (docs/plans/reach-parity.md): the
// rendered plugin tree carries the Agent Plugins 1.0 pair beside Claude
// Code's manifest, every manifest unifies with the vendored schema
// (schema/plugin/schema.cue) through the CUE Go API — always — and through
// the `cue vet` binary when it is installed (named skip otherwise), and the
// two Agent Plugins files are pinned to goldens so a rendering change is a
// reviewed diff.

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	cuejson "cuelang.org/go/encoding/json"
)

func pluginSchemaSource(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join("..", "..", "schema", "plugin", name)
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	return string(data)
}

func renderPluginTree(t *testing.T) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "fathom-plugin")
	var stdout, stderr bytes.Buffer
	if code := cmdPluginRender([]string{"--out", out}, &stdout, &stderr); code != 0 {
		t.Fatalf("plugin render exit %d: %s", code, stderr.String())
	}
	return out
}

// TestPluginManifestsVet is the anchor for RP-7.
func TestPluginManifestsVet(t *testing.T) {
	out := renderPluginTree(t)
	cctx := cuecontext.New()
	schema := cctx.CompileString(pluginSchemaSource(t, "schema.cue"))
	if err := schema.Err(); err != nil {
		t.Fatalf("schema does not compile: %v", err)
	}
	cueBin, cueErr := exec.LookPath("cue")

	cases := []struct{ file, def string }{
		{"plugin.json", "#AgentPlugin"},
		{"mcp.json", "#AgentMCP"},
		{filepath.Join(".claude-plugin", "plugin.json"), "#ClaudeManifest"},
	}
	for _, tc := range cases {
		path := filepath.Join(out, tc.file)
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s missing: %v", tc.file, err)
		}
		expr, err := cuejson.Extract(tc.file, raw)
		if err != nil {
			t.Fatalf("%s is not JSON: %v", tc.file, err)
		}
		data := cctx.BuildExpr(expr)
		def := schema.LookupPath(cue.ParsePath(tc.def))
		if err := def.Err(); err != nil {
			t.Fatalf("%s missing from the schema: %v", tc.def, err)
		}
		if err := def.Unify(data).Validate(cue.Concrete(true)); err != nil {
			t.Errorf("%s does not satisfy %s: %v\n%s", tc.file, tc.def, err, raw)
		}
		if cueErr != nil {
			continue
		}
		cmd := exec.Command(cueBin, "vet", "-d", tc.def, filepath.Join("..", "..", "schema", "plugin", "schema.cue"), path)
		if o, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("cue vet %s failed: %v\n%s", tc.file, err, o)
		}
	}
	if cueErr != nil {
		t.Logf("cue not installed — skipping the binary pass (the Go API pass above already ran)")
	}

	// One skills/ tree, spec-only frontmatter, all ten.
	entries, err := os.ReadDir(filepath.Join(out, "skills"))
	if err != nil || len(entries) != 10 {
		t.Fatalf("skills/ = %v (%v), want 10", entries, err)
	}
	// The two Agent Plugins files agree on the schema version.
	var pj, mj map[string]any
	pd, _ := os.ReadFile(filepath.Join(out, "plugin.json"))
	md, _ := os.ReadFile(filepath.Join(out, "mcp.json"))
	_ = json.Unmarshal(pd, &pj)
	_ = json.Unmarshal(md, &mj)
	if !strings.Contains(pj["$schema"].(string), "/1.0.0/") || !strings.Contains(mj["$schema"].(string), "/1.0.0/") {
		t.Errorf("schema versions must match: %v / %v", pj["$schema"], mj["$schema"])
	}
}

// TestPluginRenderGolden pins the Agent Plugins pair byte-for-byte
// (RP-6). Regenerate with:
//
//	go test ./cmd/fathom -run TestPluginRenderGolden -update
func TestPluginRenderGolden(t *testing.T) {
	out := renderPluginTree(t)
	for _, name := range []string{"plugin.json", "mcp.json"} {
		got, err := os.ReadFile(filepath.Join(out, name))
		if err != nil {
			t.Fatal(err)
		}
		golden := filepath.Join("testdata", "plugin", name)
		if *updateGolden {
			if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(golden, got, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(golden)
		if err != nil {
			t.Fatalf("golden %s missing (run with -update): %v", golden, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s differs from golden %s:\n--- got ---\n%s\n--- want ---\n%s", name, golden, got, want)
		}
	}
}

// TestPluginSchema_Negatives proves the commented cases in
// schema/plugin/testdata/invalid.cue actually fail, and valid.cue unifies.
func TestPluginSchema_Negatives(t *testing.T) {
	schema := pluginSchemaSource(t, "schema.cue")
	cctx := cuecontext.New()

	valid := pluginSchemaSource(t, filepath.Join("testdata", "valid.cue"))
	if v := cctx.CompileString(schema + "\n" + stripCUEPackageLine(valid)); v.Err() != nil || v.Validate(cue.Concrete(true)) != nil {
		t.Fatalf("valid.cue does not unify with the schema: %v / %v", v.Err(), v.Validate(cue.Concrete(true)))
	}
	cases := []struct{ name, bad string }{
		{"wrong schema version", `bad: #AgentPlugin & {"$schema": "https://agent-plugins.org/schemas/1.1.0/plugin.schema.json", name: "x"}`},
		{"uppercase name", `bad: #AgentPlugin & {"$schema": #AgentPluginSchemaURL, name: "My-Plugin"}`},
		{"double hyphen", `bad: #AgentPlugin & {"$schema": #AgentPluginSchemaURL, name: "has--double"}`},
		{"double dot", `bad: #AgentPlugin & {"$schema": #AgentPluginSchemaURL, name: "too.many..dots"}`},
		{"inline component config", `bad: #AgentPlugin & {"$schema": #AgentPluginSchemaURL, name: "x", mcpServers: {}}`},
		{"mixed server variants", `bad: #AgentMCP & {"$schema": #AgentMCPSchemaURL, mcpServers: {x: {type: "stdio", command: "x", url: "https://a"}}}`},
		{"non-loopback http", `bad: #AgentMCP & {"$schema": #AgentMCPSchemaURL, mcpServers: {x: {type: "streamable-http", url: "http://example.com/mcp"}}}`},
		{"claude path escapes root", `bad: #ClaudeManifest & {name: "x", skills: "../elsewhere"}`},
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

// updateGolden rewrites cmd/fathom/testdata/plugin/* from the current
// rendering: `go test ./cmd/fathom -run TestPluginRenderGolden -update`.
var updateGolden = flag.Bool("update", false, "rewrite golden files from the current rendering")
