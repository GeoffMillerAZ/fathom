package adoption

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func selectionTestFixture() Selection {
	return Selection{
		SchemaVersion: 1, Consumer: "synthetic-app", Profile: "go-web",
		Layers: []string{"L3b", "L2"}, Capabilities: []string{"shell.layout", "shell.appearance"},
		CatalogRevision: strings.Repeat("c", 64), Contract: "docs/appkit.md",
		Module:    &ModuleRequirement{File: "go.mod", Version: "v0.0.0-00010101000000-000000000000", AllowLocalReplace: true},
		Artifacts: []Artifact{{Path: "web/z.css", SHA256: strings.Repeat("a", 64)}, {Path: "web/a.js", SHA256: strings.Repeat("b", 64)}},
		Agents:    true,
	}
}

func selectionTestJSON(t *testing.T, value any) []byte {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func selectionTestObject(t *testing.T) map[string]any {
	t.Helper()
	var object map[string]any
	if err := json.Unmarshal(selectionTestJSON(t, selectionTestFixture()), &object); err != nil {
		t.Fatal(err)
	}
	return object
}

func TestSelectionDecodeNormalizesWithoutMutatingInput(t *testing.T) {
	body, err := json.MarshalIndent(selectionTestFixture(), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	original := bytes.Clone(body)
	got, err := Decode(body)
	if err != nil {
		t.Fatal(err)
	}
	want := selectionTestFixture()
	want.Layers = []string{"L2", "L3b"}
	want.Capabilities = []string{"shell.appearance", "shell.layout"}
	want.Artifacts = []Artifact{want.Artifacts[1], want.Artifacts[0]}
	if !reflect.DeepEqual(got, want) || !bytes.Equal(body, original) {
		t.Fatalf("normalization lost declarations or changed input: %+v", got)
	}
	again, err := Decode(selectionTestJSON(t, got))
	if err != nil || !reflect.DeepEqual(again, got) {
		t.Fatalf("normalized selection does not round-trip: %v", err)
	}
}

func TestSelectionDecodeClosedSchemaAndRequiredValues(t *testing.T) {
	for name, mutate := range map[string]func(map[string]any){
		"unknown top-level":             func(m map[string]any) { m["surprise"] = true },
		"unknown module":                func(m map[string]any) { m["module"].(map[string]any)["resolved"] = true },
		"unknown artifact":              func(m map[string]any) { m["artifacts"].([]any)[0].(map[string]any)["executable"] = true },
		"null artifact entry":           func(m map[string]any) { m["artifacts"] = []any{nil} },
		"null module version":           func(m map[string]any) { m["module"].(map[string]any)["version"] = nil },
		"case alias":                    func(m map[string]any) { m["Consumer"] = m["consumer"]; delete(m, "consumer") },
		"case alias alongside original": func(m map[string]any) { m["Consumer"] = "another-app" },
		"module case alias":             func(m map[string]any) { m["module"].(map[string]any)["Version"] = "v1.2.3" },
		"wrong schema version":          func(m map[string]any) { m["schema_version"] = 2 },
		"string schema version":         func(m map[string]any) { m["schema_version"] = "1" },
		"missing schema version":        func(m map[string]any) { delete(m, "schema_version") },
		"missing contract":              func(m map[string]any) { delete(m, "contract") },
		"empty contract":                func(m map[string]any) { m["contract"] = "" },
		"blank contract":                func(m map[string]any) { m["contract"] = "   " },
		"invalid consumer":              func(m map[string]any) { m["consumer"] = "Synthetic App" },
		"blank profile":                 func(m map[string]any) { m["profile"] = "\t " },
		"blank layer":                   func(m map[string]any) { m["layers"] = []string{" "} },
		"blank capability":              func(m map[string]any) { m["capabilities"] = []string{""} },
		"bad revision":                  func(m map[string]any) { m["catalog_revision"] = strings.Repeat("C", 64) },
		"short artifact digest":         func(m map[string]any) { m["artifacts"].([]any)[0].(map[string]any)["sha256"] = "abc" },
		"missing module permission":     func(m map[string]any) { delete(m["module"].(map[string]any), "allow_local_replace") },
		"string agent requirement":      func(m map[string]any) { m["agents"] = "true" },
		"duplicate capabilities":        func(m map[string]any) { m["capabilities"] = []string{"shell.layout", "shell.layout"} },
		"duplicate layers":              func(m map[string]any) { m["layers"] = []string{"L2", "L2"} },
		"duplicate artifact paths":      func(m map[string]any) { m["artifacts"].([]any)[1].(map[string]any)["path"] = "web/z.css" },
		"control in capability":         func(m map[string]any) { m["capabilities"] = []string{"shell.\nlayout"} },
		"control in layer":              func(m map[string]any) { m["layers"] = []string{"L2\u0085"} },
	} {
		t.Run(name, func(t *testing.T) {
			object := selectionTestObject(t)
			mutate(object)
			if _, err := Decode(selectionTestJSON(t, object)); err == nil {
				t.Fatal("invalid or ambiguous declaration accepted")
			}
		})
	}
	for _, field := range []string{"schema_version", "consumer", "profile", "layers", "capabilities", "catalog_revision", "contract", "module", "artifacts", "agents"} {
		t.Run("null "+field, func(t *testing.T) {
			object := selectionTestObject(t)
			object[field] = nil
			if _, err := Decode(selectionTestJSON(t, object)); err == nil {
				t.Fatal("null field accepted")
			}
		})
	}
}

func TestSelectionDecodeRejectsAmbiguousAndNonJSONInput(t *testing.T) {
	valid := selectionTestJSON(t, selectionTestFixture())
	for name, body := range map[string][]byte{
		"duplicate top-level":   bytes.Replace(valid, []byte(`"schema_version":1`), []byte(`"schema_version":1,"schema_version":1`), 1),
		"duplicate module":      bytes.Replace(valid, []byte(`"file":"go.mod"`), []byte(`"file":"go.mod","file":"go.mod"`), 1),
		"duplicate artifact":    bytes.Replace(valid, []byte(`"path":"web/z.css"`), []byte(`"path":"web/z.css","path":"web/z.css"`), 1),
		"escaped duplicate key": bytes.Replace(valid, []byte(`"schema_version":1`), []byte(`"schema_version":1,"schema_\u0076ersion":1`), 1),
		"trailing object":       append(bytes.Clone(valid), []byte(` {}`)...),
		"truncated":             valid[:len(valid)-1],
		"empty":                 nil,
		"null":                  []byte(`null`),
		"array":                 []byte(`[]`),
		"executable CUE":        []byte(`import "os"` + "\n" + `consumer: os.Getenv("USER")`),
		"CUE expression":        []byte(`{schema_version: 1, consumer: "synthetic-app"}`),
		"invalid UTF8":          append(bytes.Clone(valid), 0xff),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Decode(body); err == nil {
				t.Fatal("non-JSON or ambiguous declaration accepted")
			}
		})
	}
}

func TestSelectionDecodeBounds(t *testing.T) {
	valid := selectionTestJSON(t, selectionTestFixture())
	atLimit := append(bytes.Clone(valid), bytes.Repeat([]byte(" "), MaxManifestBytes-len(valid))...)
	if _, err := Decode(atLimit); err != nil {
		t.Fatalf("valid JSON at the byte limit rejected: %v", err)
	}
	if _, err := Decode(append(atLimit, ' ')); err == nil {
		t.Fatal("oversized manifest accepted")
	}
	for name, mutate := range map[string]func(*Selection){
		"consumer":          func(s *Selection) { s.Consumer = strings.Repeat("a", 81) },
		"profile":           func(s *Selection) { s.Profile = strings.Repeat("p", 41) },
		"layer length":      func(s *Selection) { s.Layers = []string{strings.Repeat("L", 121)} },
		"capability length": func(s *Selection) { s.Capabilities = []string{strings.Repeat("c", 121)} },
		"layer count":       func(s *Selection) { s.Layers = []string{"L0", "L1", "L2", "L3a", "L3b", "L4", "L5"} },
		"capability count": func(s *Selection) {
			s.Capabilities = nil
			for n := 0; n < 65; n++ {
				s.Capabilities = append(s.Capabilities, fmt.Sprintf("fixture.%02d", n))
			}
		},
		"artifact count": func(s *Selection) {
			s.Artifacts = nil
			for n := 0; n < 65; n++ {
				s.Artifacts = append(s.Artifacts, Artifact{Path: fmt.Sprintf("web/%02d.css", n), SHA256: strings.Repeat("a", 64)})
			}
		},
		"contract path": func(s *Selection) { s.Contract = strings.Repeat("a", 513) },
		"module path":   func(s *Selection) { s.Module.File = strings.Repeat("a", 513) },
		"artifact path": func(s *Selection) { s.Artifacts[0].Path = strings.Repeat("a", 513) },
	} {
		t.Run(name, func(t *testing.T) {
			fixture := selectionTestFixture()
			mutate(&fixture)
			if _, err := Decode(selectionTestJSON(t, fixture)); err == nil {
				t.Fatal("selection field/count bound was ignored")
			}
		})
	}
	maximum := selectionTestFixture()
	maximum.Consumer, maximum.Profile, maximum.Contract = strings.Repeat("a", 80), strings.Repeat("p", 40), strings.Repeat("a", 512)
	maximum.Layers = []string{"L4", "L3b", "L3a", "L2", "L1", "L0"}
	maximum.Capabilities, maximum.Artifacts = nil, nil
	for n := 0; n < 64; n++ {
		maximum.Capabilities = append(maximum.Capabilities, fmt.Sprintf("fixture.%02d", n))
		maximum.Artifacts = append(maximum.Artifacts, Artifact{Path: fmt.Sprintf("web/%02d.css", n), SHA256: strings.Repeat("a", 64)})
	}
	if _, err := Decode(selectionTestJSON(t, maximum)); err != nil {
		t.Fatalf("inclusive path/string/array limits rejected: %v", err)
	}
}

func TestSelectionVocabularyAndRequirementOwnership(t *testing.T) {
	fixture := selectionTestFixture()
	fixture.Module, fixture.Agents = nil, false
	fixture.Capabilities = []string{}
	fixture.Profile, fixture.Layers = "future-profile", []string{"future-layer"}
	if _, err := Decode(selectionTestJSON(t, fixture)); err != nil {
		t.Fatalf("artifact-only declaration or deferred vocabulary rejected: %v", err)
	}
	fixture.Artifacts = []Artifact{}
	if _, err := Decode(selectionTestJSON(t, fixture)); err == nil {
		t.Fatal("empty requirements accepted")
	}
	fixture.Capabilities = []string{"not-yet-in-the-catalog"}
	if _, err := Decode(selectionTestJSON(t, fixture)); err != nil {
		t.Fatalf("Decode must leave capability membership to Check: %v", err)
	}
	fixture.Capabilities, fixture.Agents = []string{}, true
	if _, err := Decode(selectionTestJSON(t, fixture)); err != nil {
		t.Fatalf("agent-routing-only requirement rejected: %v", err)
	}
}

func TestSelectionCanonicalModuleVersions(t *testing.T) {
	for _, version := range []string{"v1.2.3", "v0.0.0", "v1.2.3-rc.1", "v0.0.0-00010101000000-000000000000"} {
		fixture := selectionTestFixture()
		fixture.Module.Version = version
		if _, err := Decode(selectionTestJSON(t, fixture)); err != nil {
			t.Errorf("canonical declaration %q rejected: %v", version, err)
		}
	}
	for _, version := range []string{"", "latest", "1.2.3", "v1", "v1.2", "v01.2.3", "v1.2.3+metadata", "../app-kit", "v1.2.3\n"} {
		fixture := selectionTestFixture()
		fixture.Module.Version = version
		if _, err := Decode(selectionTestJSON(t, fixture)); err == nil {
			t.Errorf("noncanonical declaration %q accepted", version)
		}
	}
}

func TestSelectionDecodeChecksEveryDeclaredPath(t *testing.T) {
	for _, field := range []string{"contract", "module", "artifact"} {
		for _, path := range []string{"../outside", "/absolute", "docs/../contract.md", `docs\contract.md`, "docs/\x00.md"} {
			t.Run(field+"/"+path, func(t *testing.T) {
				fixture := selectionTestFixture()
				switch field {
				case "contract":
					fixture.Contract = path
				case "module":
					fixture.Module.File = path
				case "artifact":
					fixture.Artifacts[0].Path = path
				}
				if _, err := Decode(selectionTestJSON(t, fixture)); err == nil {
					t.Fatal("unsafe declared path accepted")
				}
			})
		}
	}
}

func TestSelectionSafePathAndRouter(t *testing.T) {
	for _, path := range []string{"docs/appkit.md", "web/css/theme.css", ".config/adoption.json", "docs/日本語.md", "docs/with spaces.md", strings.Repeat("a", 512)} {
		if !SafePath(path) {
			t.Errorf("safe relative path rejected: %q", path)
		}
	}
	for _, path := range []string{"", ".", "..", "../outside", "a/../../outside", "/absolute", "//host/file", "a//b", "a/./b", "a/../b", "a/", `a\b`, "C:/file", "docs/name:part", "docs/\x00.md", "docs/\t.md", "docs/\n.md", "docs/\r.md", "docs/\u0085.md", "docs/\x7f.md", "docs/\xff.md", strings.Repeat("a", 513)} {
		if SafePath(path) {
			t.Errorf("unsafe/nonportable path accepted: %q", path)
		}
		if got, err := Router(path); err == nil || got != "" {
			t.Errorf("router accepted unsafe path %q: %q, %v", path, got, err)
		}
	}
	for _, path := range []string{"docs/`code`.md", "docs/<script>.md", "docs/[label].md", "docs/route>.md"} {
		if got, err := Router(path); err == nil || got != "" {
			t.Errorf("router admitted markup-bearing path %q", path)
		}
	}
	const want = "<!-- appkit:begin -->\n## App-kit\n\nApp-kit adoption lives at docs/appkit.md: selected layers/profile, design context,\nrevision, proof commands and divergences. Before material UI/CLI work, consult it and\nthe kit's intent router through appkit MCP or the pinned local docs. Select a fitting\ncapability/recipe; verify the actual task and states in the target UI. Record reusable\nimprovements or a reasoned divergence and return applicable fixes upstream.\n<!-- appkit:end -->\n"
	for n := 0; n < 2; n++ {
		got, err := Router("docs/appkit.md")
		if err != nil || got != want || strings.Count(got, "\n") > 12 || len(got) > 1024 {
			t.Fatalf("paired guidance blocks must be short and byte-identical: %q, %v", got, err)
		}
	}
}
