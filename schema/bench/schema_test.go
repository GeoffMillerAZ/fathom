package bench_test

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

func fixture(t *testing.T, name string) (cue.Value, map[string]any) {
	t.Helper()
	schema, err := os.ReadFile("schema.cue")
	if err != nil {
		t.Fatal(err)
	}
	positive, err := os.ReadFile("testdata/valid.cue")
	if err != nil {
		t.Fatal(err)
	}
	ctx := cuecontext.New()
	v := ctx.CompileString(string(schema) + "\n" + strings.Replace(string(positive), "package bench", "", 1))
	if err := v.Validate(cue.Concrete(true)); err != nil {
		t.Fatalf("positive fixtures: %v", err)
	}
	var data map[string]any
	if err := v.LookupPath(cue.ParsePath(name)).Decode(&data); err != nil {
		t.Fatal(err)
	}
	definition := "#Subject"
	if name == "prereg" {
		definition = "#Prereg"
	}
	return v.LookupPath(cue.ParsePath(definition)), data
}

// Validate the same concrete values cue-vet discovers, with both split values
// and with the optional corpus subpath omitted.
func TestBenchSchemaValid(t *testing.T) {
	for _, name := range []string{"subject", "prereg"} {
		t.Run(name, func(t *testing.T) {
			def, data := fixture(t, name)
			if name == "subject" {
				delete(data, "subpath")
				data["bench"].(map[string]any)["split"] = "held-out"
				data["bench"].(map[string]any)["setup"] = []any{}
			} else {
				data["deviation_log"] = []any{map[string]any{"date": "2026-02-01", "description": "Raised n before data", "reason": "Power table update"}}
			}
			b, err := json.Marshal(data)
			if err != nil {
				t.Fatal(err)
			}
			if err := def.Unify(def.Context().CompileBytes(b)).Validate(cue.Concrete(true)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestBenchSchemaNegatives(t *testing.T) {
	casesFile, err := os.ReadFile("testdata/invalid.cue")
	if err != nil {
		t.Fatal(err)
	}
	_, commented, ok := strings.Cut(string(casesFile), "// cases: [")
	if !ok {
		t.Fatal("invalid.cue has no cases list")
	}
	var source strings.Builder
	source.WriteString("cases: [\n")
	for _, line := range strings.Split(commented, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, "// ") {
			t.Fatal("invalid.cue contains an uncommented negative")
		}
		source.WriteString(strings.TrimPrefix(line, "// ") + "\n")
	}
	var cases []struct {
		Name    string `json:"name"`
		Fixture string `json:"fixture"`
		Path    string `json:"path"`
		Value   any    `json:"value"`
		Remove  bool   `json:"remove"`
	}
	v := cuecontext.New().CompileString(source.String())
	if err := v.LookupPath(cue.ParsePath("cases")).Decode(&cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("invalid.cue has no negative examples")
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			def, data := fixture(t, tc.Fixture)
			var node any = data
			parts := strings.Split(tc.Path, ".")
			for _, key := range parts[:len(parts)-1] {
				if list, ok := node.([]any); ok {
					index, err := strconv.Atoi(key)
					if err != nil || index < 0 || index >= len(list) {
						t.Fatalf("invalid fixture index: %s", key)
					}
					node = list[index]
				} else {
					node = node.(map[string]any)[key]
				}
			}
			obj := node.(map[string]any)
			key := parts[len(parts)-1]
			if tc.Remove {
				delete(obj, key)
			} else {
				obj[key] = tc.Value
			}
			b, err := json.Marshal(data)
			if err != nil {
				t.Fatal(err)
			}
			err = def.Unify(def.Context().CompileBytes(b)).Validate(cue.Concrete(true))
			if err == nil {
				t.Fatal("invalid value accepted")
			}
			// The diagnostic must identify the field under test, rather than
			// rejecting an unrelated incomplete field in the positive fixture.
			if !strings.Contains(err.Error(), key) {
				t.Fatalf("diagnostic does not name %s: %v", key, err)
			}
		})
	}
}
