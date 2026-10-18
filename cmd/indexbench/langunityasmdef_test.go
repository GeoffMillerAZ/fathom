package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLangUnityAsmdefCommand(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	def := []byte(`{"name":"Fixture.Core","noEngineReferences":true,"overrideReferences":true,"precompiledReferences":[]}`)
	src := []byte("namespace Fixture { class Core {} }\n")
	for name, b := range map[string][]byte{"Core.asmdef": def, "Core.cs": src} {
		if err := os.WriteFile(filepath.Join(root, "Assets", name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	hash := func(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
	oracle := map[string]any{"schemaVersion": 1, "oracle": "UnityEditor.Compilation.CompilationPipeline.GetAssemblies", "unityVersion": "6000.3.15f1", "buildTarget": "StandaloneOSX", "assembliesType": "Editor", "scope": "synthetic CLI fixture", "availableAssemblyNames": []string{"Fixture.Core"}, "assemblies": []map[string]any{{"name": "Fixture.Core", "asmdefPath": "Assets/Core.asmdef", "definitionSha256": hash(def), "noEngineReferencesDeclared": true, "overrideReferencesDeclared": true, "precompiledReferencesDeclared": []string{}, "declaredReferences": []string{}, "assemblyReferences": []string{}, "compiledAssemblyReferences": []string{"System.Runtime"}, "emittedReferencesAvailable": true, "emittedAssemblyReferences": []string{"System.Runtime"}, "sourceFiles": []string{"Assets/Core.cs"}, "sourceFacts": []map[string]string{{"path": "Assets/Core.cs", "owner": "Fixture.Core.dll", "sha256": hash(src)}}}}}
	raw, err := json.Marshal(oracle)
	if err != nil {
		t.Fatal(err)
	}
	receipt := filepath.Join(t.TempDir(), "oracle.json")
	if err := os.WriteFile(receipt, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := mainRun([]string{"lang-unity-asmdef", "-project", root, "-oracle", receipt}, &stdout, &stderr); code != 0 {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, &stdout, &stderr)
	}
	if !strings.Contains(stdout.String(), `"owner_matches": 1`) || !strings.Contains(stdout.String(), `"passed": true`) {
		t.Fatal(stdout.String())
	}
	if err := os.WriteFile(filepath.Join(root, "Assets", "Core.cs"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := mainRun([]string{"lang-unity-asmdef", "-project", root, "-oracle", receipt}, &stdout, &stderr); code != 1 || !strings.Contains(stdout.String(), "source-digest-mismatch") {
		t.Fatalf("false pass code=%d stdout=%s stderr=%s", code, &stdout, &stderr)
	}
	stdout.Reset()
	stderr.Reset()
	if code := mainRun([]string{"lang-unity-asmdef"}, &stdout, &stderr); code != 2 {
		t.Fatalf("missing flags exit=%d", code)
	}
}
