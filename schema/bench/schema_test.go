package bench_test

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	if !v.LookupPath(cue.ParsePath(name)).Exists() {
		t.Fatalf("positive fixture %q does not exist", name)
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
	for _, name := range []string{"subject", "prereg", "exclusionSubject"} {
		t.Run(name, func(t *testing.T) {
			def, data := fixture(t, name)
			if name != "prereg" {
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

func TestBenchPriorReadFixture(t *testing.T) {
	_, data := fixture(t, "priorReadSubject")
	bench := data["bench"].(map[string]any)
	if bench["split"] != "dev" {
		t.Fatalf("prior-read fixture split = %v, want dev", bench["split"])
	}
	reads := bench["contamination"].(map[string]any)["prior_reads"].([]any)
	if len(reads) == 0 {
		t.Fatal("prior-read fixture must contain concrete prior reads")
	}
}

func TestBenchManifestPriorReadsExist(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	manifests, err := filepath.Glob(filepath.Join(repoRoot, "calibration", "bench.d", "*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(manifests) == 0 {
		t.Fatal("no bench manifests found")
	}
	for _, manifest := range manifests {
		t.Run(filepath.Base(manifest), func(t *testing.T) {
			b, err := os.ReadFile(manifest)
			if err != nil {
				t.Fatal(err)
			}
			// These YAML manifests contain JSON with full-line comments.
			var source strings.Builder
			for _, line := range strings.Split(string(b), "\n") {
				if !strings.HasPrefix(strings.TrimSpace(line), "#") {
					source.WriteString(line + "\n")
				}
			}
			var data struct {
				Bench struct {
					Contamination struct {
						PriorReads []string `json:"prior_reads"`
					} `json:"contamination"`
				} `json:"bench"`
			}
			if err := json.Unmarshal([]byte(source.String()), &data); err != nil {
				t.Fatal(err)
			}
			for _, path := range data.Bench.Contamination.PriorReads {
				if path == "" || filepath.IsAbs(path) || !filepath.IsLocal(path) {
					t.Errorf("prior read must be a nonempty repository-relative path: %q", path)
					continue
				}
				info, err := os.Stat(filepath.Join(repoRoot, path))
				if err != nil {
					t.Errorf("prior read %q: %v", path, err)
				} else if !info.Mode().IsRegular() {
					t.Errorf("prior read %q is not a regular file", path)
				}
			}
		})
	}
}

func TestBenchSubjectSplitAndPriorReads(t *testing.T) {
	for _, mode := range []string{"unassigned", "prior-read", "empty-prior-reads"} {
		t.Run(mode, func(t *testing.T) {
			def, data := fixture(t, "subject")
			bench := data["bench"].(map[string]any)
			delete(bench, "split")
			contamination := bench["contamination"].(map[string]any)
			if mode == "prior-read" {
				contamination["prior_reads"] = []string{"docs/bench/scenario-runs.md"}
			} else if mode == "empty-prior-reads" {
				contamination["prior_reads"] = []string{}
			}
			b, err := json.Marshal(data)
			if err != nil {
				t.Fatal(err)
			}
			value := def.Unify(def.Context().CompileBytes(b))
			if err := value.Validate(cue.Concrete(true)); err != nil {
				t.Fatal(err)
			}
			if mode == "prior-read" {
				split, err := value.LookupPath(cue.ParsePath("bench.split")).String()
				if err != nil || split != "dev" {
					t.Fatalf("prior read must force dev: %q, %v", split, err)
				}
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
			if tc.Name == "held-out prior read" && (tc.Fixture != "priorReadSubject" || tc.Path != "bench.split" || tc.Value != "held-out" || tc.Remove) {
				t.Fatal("held-out prior-read case must change only priorReadSubject.bench.split to held-out")
			}
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
