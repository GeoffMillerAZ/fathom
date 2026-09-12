package harness

// schema_test.go — HP-9: schema/harness/schema.cue validates the exported
// registry JSON. The hard assertion runs through the CUE Go API (always);
// the `cue vet` binary pass runs when `cue` is installed and skips cleanly
// otherwise, matching `task cue-vet`'s own "cue not installed — skipping".

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	cuejson "cuelang.org/go/encoding/json"
)

func harnessSchemaPath(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "schema", "harness", name)
}

func harnessSchemaSource(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(harnessSchemaPath(t, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}

// TestHarnessSchemaVetsExport is the anchor for HP-9: the registry, as
// exported, unifies with #Registry.
func TestHarnessSchemaVetsExport(t *testing.T) {
	export, err := ExportJSON()
	if err != nil {
		t.Fatalf("ExportJSON: %v", err)
	}

	cctx := cuecontext.New()
	schema := cctx.CompileString(harnessSchemaSource(t, "schema.cue"))
	if err := schema.Err(); err != nil {
		t.Fatalf("schema does not compile: %v", err)
	}
	if err := schema.Validate(); err != nil {
		t.Fatalf("schema does not validate on its own: %v", err)
	}
	expr, err := cuejson.Extract("export.json", export)
	if err != nil {
		t.Fatalf("export is not JSON: %v", err)
	}
	data := cctx.BuildExpr(expr)
	if err := data.Err(); err != nil {
		t.Fatalf("export does not build: %v", err)
	}
	def := schema.LookupPath(cue.ParsePath("#Registry"))
	if err := def.Err(); err != nil {
		t.Fatalf("#Registry missing: %v", err)
	}
	if err := def.Unify(data).Validate(cue.Concrete(true)); err != nil {
		t.Fatalf("exported registry does not satisfy #Registry:\n%v\n--- export ---\n%s", err, export)
	}

	t.Run("cue vet binary", func(t *testing.T) {
		cueBin, err := exec.LookPath("cue")
		if err != nil {
			t.Skip("cue not installed — skipping the binary pass (the Go API pass above already ran)")
		}
		dir := t.TempDir()
		exportPath := filepath.Join(dir, "export.json")
		if err := os.WriteFile(exportPath, export, 0o644); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(cueBin, "vet", "-d", "#Registry", harnessSchemaPath(t, "schema.cue"), exportPath)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("cue vet failed: %v\n%s", err, out)
		}
	})
}

// TestHarnessSchema_Negatives proves the commented cases in
// testdata/invalid.cue actually fail, and that valid.cue unifies.
func TestHarnessSchema_Negatives(t *testing.T) {
	schema := harnessSchemaSource(t, "schema.cue")
	cctx := cuecontext.New()

	valid := harnessSchemaSource(t, filepath.Join("testdata", "valid.cue"))
	if v := cctx.CompileString(schema + "\n" + stripPackageLine(valid)); v.Err() != nil || v.Validate(cue.Concrete(true)) != nil {
		t.Fatalf("valid.cue does not unify with the schema: %v / %v", v.Err(), v.Validate(cue.Concrete(true)))
	}

	cases := []struct{ name, bad string }{
		{"tier outside the ladder", `bad: #Descriptor & {tier: "E"}`},
		{"vehicle outside the vocabulary", `bad: #Descriptor & {vehicles: {prime: "telepathy"}}`},
		{"a capability missing from vehicles", `bad: #Descriptor & {vehicles: close({prime: "pull"})}`},
		{"non-positive binding timeout", `bad: #EventBinding & {capability: "prime", event: "session-start", harness_event: "SessionStart", verb: "session-hook", marker: "fathom session-hook", timeout_sec: 0, vehicle: "hook-inject"}`},
		{"non-ISO codec landing date", `bad: #Descriptor & {codec_landed: "yesterday"}`},
		{"event catalogue row with no wired verdict", `bad: #EventCatalogEntry & close({event: "SessionStart", reason: "maybe?"})`},
		{"canonical event outside the vocabulary", `bad: #EventBinding & {capability: "prime", event: "telepathy", harness_event: "X", verb: "v", marker: "m", timeout_sec: 1, vehicle: "hook-inject"}`},
		{"capability outside the vocabulary", `bad: #EventBinding & {capability: "subagent-primer", event: "subagent-start", harness_event: "SubagentStart", verb: "v", marker: "m", timeout_sec: 1, vehicle: "hook-inject"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := cctx.CompileString(schema + "\n" + tc.bad)
			if v.Err() == nil && v.Validate() == nil {
				t.Fatalf("expected %s to fail, but it unified", tc.name)
			}
		})
	}
}

func stripPackageLine(src string) string {
	var out []string
	for _, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "package ") {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
