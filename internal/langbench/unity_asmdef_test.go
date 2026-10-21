package langbench

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

func unityFixture(t *testing.T) (fstest.MapFS, []byte) {
	t.Helper()
	files := fstest.MapFS{
		"Assets/Root/Root.asmdef":        {Data: []byte(`{"name":"Fixture.Root","references":["Fixture.Core"]}`)},
		"Assets/Root/Root.cs":            {Data: []byte("namespace Fixture { class Root {} }\n")},
		"Assets/Root/Nested/Core.asmdef": {Data: []byte(`{"name":"Fixture.Core","noEngineReferences":true,"overrideReferences":true,"precompiledReferences":[]}`)},
		"Assets/Root/Nested/Core.cs":     {Data: []byte("namespace Fixture { class Core {} }\n")},
	}
	hash := func(p string) string { s := sha256.Sum256(files[p].Data); return hex.EncodeToString(s[:]) }
	oracle := map[string]any{"schemaVersion": 1, "oracle": "UnityEditor.Compilation.CompilationPipeline.GetAssemblies", "unityVersion": "6000.3.15f1", "buildTarget": "StandaloneOSX", "assembliesType": "Editor", "scope": "generic synthetic fixture; not an executed Unity receipt", "availableAssemblyNames": []string{"Fixture.Core", "Fixture.Root"}, "assemblies": []map[string]any{
		{"name": "Fixture.Root", "asmdefPath": "Assets/Root/Root.asmdef", "definitionSha256": hash("Assets/Root/Root.asmdef"), "declaredReferences": []string{"Fixture.Core"}, "assemblyReferences": []string{"Fixture.Core"}, "sourceFiles": []string{"Assets/Root/Root.cs"}, "sourceFacts": []map[string]string{{"path": "Assets/Root/Root.cs", "owner": "Fixture.Root.dll", "sha256": hash("Assets/Root/Root.cs")}}},
		{"name": "Fixture.Core", "asmdefPath": "Assets/Root/Nested/Core.asmdef", "definitionSha256": hash("Assets/Root/Nested/Core.asmdef"), "noEngineReferencesDeclared": true, "overrideReferencesDeclared": true, "precompiledReferencesDeclared": []string{}, "declaredReferences": []string{}, "assemblyReferences": []string{}, "compiledAssemblyReferences": []string{"System.Runtime"}, "emittedReferencesAvailable": true, "emittedAssemblyReferences": []string{"System.Runtime"}, "sourceFiles": []string{"Assets/Root/Nested/Core.cs"}, "sourceFacts": []map[string]string{{"path": "Assets/Root/Nested/Core.cs", "owner": "Fixture.Core.dll", "sha256": hash("Assets/Root/Nested/Core.cs")}}},
	}}
	raw, err := json.Marshal(oracle)
	if err != nil {
		t.Fatal(err)
	}
	return files, raw
}

func TestUnityAsmdefCalibrationBindsCompilerOracle(t *testing.T) {
	files, raw := unityFixture(t)
	r, err := CalibrateUnityAsmdefs(files, raw)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Passed || !r.Complete || r.Assemblies != 2 || r.OwnerMatches != 2 || r.ReferenceMatches != 1 || r.NoEngineMatches != 1 {
		t.Fatalf("reading=%+v", r)
	}
	if r.Evidence != "declared-assembly-facts; compiler-calibrated; no typed or runtime call edges" {
		t.Fatalf("tier confusion: %q", r.Evidence)
	}
}

func TestUnityAsmdefCalibrationRefusesFalseOrMissingFacts(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		mutate     func(fstest.MapFS, []byte) []byte
	}{
		{"wrong owner", "owner-mismatch", func(f fstest.MapFS, b []byte) []byte {
			return []byte(strings.Replace(string(b), "Fixture.Core.dll", "Fixture.Root.dll", 1))
		}},
		{"missing source", "source-unreadable", func(f fstest.MapFS, b []byte) []byte { delete(f, "Assets/Root/Nested/Core.cs"); return b }},
		{"changed source", "source-digest-mismatch", func(f fstest.MapFS, b []byte) []byte {
			f["Assets/Root/Nested/Core.cs"].Data = []byte("changed")
			return b
		}},
		{"missing definition", "definition-unreadable", func(f fstest.MapFS, b []byte) []byte { delete(f, "Assets/Root/Nested/Core.asmdef"); return b }},
		{"false compiler reference", "reference-not-compiled", func(f fstest.MapFS, b []byte) []byte {
			return []byte(strings.Replace(string(b), `"assemblyReferences":["Fixture.Core"]`, `"assemblyReferences":[]`, 1))
		}},
		{"missing target assembly", "reference-not-available", func(f fstest.MapFS, b []byte) []byte {
			return []byte(strings.Replace(string(b), `"availableAssemblyNames":["Fixture.Core","Fixture.Root"]`, `"availableAssemblyNames":["Fixture.Root"]`, 1))
		}},
		{"engine dependency in engine-free assembly", "engine-reference-present", func(f fstest.MapFS, b []byte) []byte {
			return []byte(strings.Replace(string(b), `"System.Runtime"`, `"UnityEngine.CoreModule"`, 1))
		}},
		{"missing compiled reference coverage", "compiled-references-missing", func(f fstest.MapFS, b []byte) []byte {
			return []byte(strings.Replace(string(b), `"compiledAssemblyReferences":["System.Runtime"]`, `"compiledAssemblyReferences":null`, 1))
		}},
		{"false override declaration", "override-declaration-mismatch", func(f fstest.MapFS, b []byte) []byte {
			return []byte(strings.Replace(string(b), `"overrideReferencesDeclared":true`, `"overrideReferencesDeclared":false`, 1))
		}},
		{"source fact outside project", "invalid-source-path", func(f fstest.MapFS, b []byte) []byte {
			return []byte(strings.ReplaceAll(string(b), "Assets/Root/Nested/Core.cs", "../Core.cs"))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, b := unityFixture(t)
			b = tc.mutate(f, b)
			r, err := CalibrateUnityAsmdefs(f, b)
			if err != nil {
				t.Fatal(err)
			}
			if r.Passed {
				t.Fatalf("false pass: %+v", r)
			}
			found := false
			for _, p := range r.Problems {
				found = found || p.Kind == tc.want
			}
			if !found {
				t.Fatalf("want %s: %+v", tc.want, r)
			}
		})
	}
}

func TestUnityAsmdefCalibrationDoesNotGuessUnsupportedReferences(t *testing.T) {
	f, b := unityFixture(t)
	f["Assets/Root/Link.asmref"] = &fstest.MapFile{Data: []byte(`{"reference":"Fixture.Core"}`)}
	r, err := CalibrateUnityAsmdefs(f, b)
	if err != nil {
		t.Fatal(err)
	}
	if r.Complete {
		t.Fatalf("asmref was silently treated as supported: %+v", r)
	}
	if _, err = CalibrateUnityAsmdefs(f, []byte(`{"schemaVersion":2}`)); err == nil {
		t.Fatal("unrecognized receipt accepted")
	}
	// fs paths must be validated before any reader can see traversal.
	var _ fs.FS = f
}

func TestUnityAsmdefGUIDAndReceiptCoverageStayUnjudged(t *testing.T) {
	f, b := unityFixture(t)
	var receipt UnityAssemblyOracle
	if err := json.Unmarshal(b, &receipt); err != nil {
		t.Fatal(err)
	}
	ref := "GUID:0123456789abcdef0123456789abcdef"
	def := []byte(`{"name":"Fixture.Root","references":["` + ref + `"]}`)
	f["Assets/Root/Root.asmdef"].Data = def
	receipt.Assemblies[0].DefinitionSHA256 = unityHash(def)
	receipt.Assemblies[0].DeclaredReferences = []string{ref}
	raw, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	r, err := CalibrateUnityAsmdefs(f, raw)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Passed || r.Complete || len(r.Unjudged) != 1 || r.Unjudged[0].Kind != "guid-reference" {
		t.Fatalf("GUID guessed: %+v", r)
	}
	if _, err := CalibrateUnityAsmdefs(f, append(raw, []byte(` {"extra":true}`)...)); err == nil {
		t.Fatal("trailing receipt data accepted")
	}
}

func TestUnityAsmdefSeparatesAvailableAndEmittedReferences(t *testing.T) {
	for _, tc := range []struct {
		name                      string
		available, emitted        []string
		observed                  bool
		wantProblem, wantUnjudged string
	}{
		{name: "Editor SDK availability is not emitted use", available: []string{"System.Runtime", "UnityEditor.iOS.Extensions.Xcode", "Unity.Android.Gradle"}, emitted: []string{"System.Runtime"}, observed: true},
		{name: "emitted Unity dependency refuses", available: []string{"System.Runtime"}, emitted: []string{"System.Runtime", "UnityEngine.CoreModule"}, observed: true, wantProblem: "emitted-unity-reference"},
		{name: "unavailable emitted metadata stays unjudged", available: []string{"System.Runtime"}, observed: false, wantUnjudged: "emitted-references-unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, b := unityFixture(t)
			var receipt map[string]any
			if err := json.Unmarshal(b, &receipt); err != nil {
				t.Fatal(err)
			}
			core := receipt["assemblies"].([]any)[1].(map[string]any)
			core["compiledAssemblyReferences"] = tc.available
			core["emittedReferencesAvailable"] = tc.observed
			core["emittedAssemblyReferences"] = tc.emitted
			raw, err := json.Marshal(receipt)
			if err != nil {
				t.Fatal(err)
			}
			r, err := CalibrateUnityAsmdefs(f, raw)
			if err != nil {
				t.Fatal(err)
			}
			if tc.wantProblem != "" {
				if r.Passed || len(r.Problems) != 1 || r.Problems[0].Kind != tc.wantProblem {
					t.Fatalf("emitted dependency not refused: %+v", r)
				}
			} else if tc.wantUnjudged != "" {
				if r.Complete || len(r.Unjudged) != 1 || r.Unjudged[0].Kind != tc.wantUnjudged {
					t.Fatalf("unobserved metadata claimed: %+v", r)
				}
			} else {
				if !r.Passed || !r.Complete || r.NoEngineMatches != 1 {
					t.Fatalf("available SDK misclassified as engine use: %+v", r)
				}
				encoded, _ := json.Marshal(r)
				if !strings.Contains(string(encoded), "other-unity-prefixed-available-reference") {
					t.Fatal("available SDK caveat disappeared")
				}
			}
		})
	}
}
