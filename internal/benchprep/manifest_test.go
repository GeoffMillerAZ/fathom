package benchprep

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"

	"github.com/geoffmilleraz/fathom/schema/bench"
)

// manifestFixture returns a concrete positive subject from schema/bench's
// valid.cue, renamed so its file stem can equal its name.
func manifestFixture(t *testing.T, name string) map[string]any {
	t.Helper()
	positive, err := os.ReadFile(filepath.Join("..", "..", "schema", "bench", "testdata", "valid.cue"))
	if err != nil {
		t.Fatal(err)
	}
	v := cuecontext.New().CompileString(bench.Source + "\n" + strings.Replace(string(positive), "package bench", "", 1))
	var data map[string]any
	if err := v.LookupPath(cue.ParsePath(name)).Decode(&data); err != nil {
		t.Fatal(err)
	}
	data["name"] = "sample-subject"
	return data
}

func writeManifest(t *testing.T, dir, stem string, data map[string]any) {
	t.Helper()
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	b = append([]byte("# draft manifest; comment lines are stripped\n"), b...)
	if err := os.WriteFile(filepath.Join(dir, stem+".yaml"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadSubjectManifestsAcceptsFixture(t *testing.T) {
	dir := t.TempDir()
	data := manifestFixture(t, "subject")
	writeManifest(t, dir, "sample-subject", data)
	// a-b.yaml sorts before a.yaml, but the name a sorts before a-b.
	for _, name := range []string{"a", "a-b"} {
		other := manifestFixture(t, "subject")
		other["name"] = name
		writeManifest(t, dir, name, other)
	}
	subjects, err := LoadSubjectManifests(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(subjects) != 3 || subjects[0].Name != "a" || subjects[1].Name != "a-b" || subjects[2].Name != "sample-subject" {
		t.Fatalf("subjects not sorted by name: %+v", subjects)
	}
	s := subjects[2]
	if s.SHA != "0123456789abcdef0123456789abcdef01234567" || s.Licence != "MIT" || s.Bench.Toolchain.Versions["go"] != "1.26.2" || s.Bench.FlakeRuns != 3 || s.Bench.Contamination.FreshAfter != "2026-01-01" {
		t.Fatalf("decoded subject: %+v", s)
	}
}

func TestLoadSubjectManifestsRefusals(t *testing.T) {
	for _, tc := range []struct {
		name, stem, want string
		mutate           func(map[string]any)
	}{
		{"stem differs from name", "other-name", "file stem", func(map[string]any) {}},
		{"missing sha", "sample-subject", "missing sha", func(d map[string]any) { delete(d, "sha") }},
		{"licence off the C1 list", "sample-subject", "licence", func(d map[string]any) { d["licence"] = "GPL-3.0-only" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			data := manifestFixture(t, "subject")
			tc.mutate(data)
			writeManifest(t, dir, tc.stem, data)
			_, err := LoadSubjectManifests(dir)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want a refusal naming %q", err, tc.want)
			}
		})
	}
}

// Every subject negative in schema/bench/testdata/invalid.cue is refused by
// the loader, which validates through the CUE Go API.
func TestLoadSubjectManifestsRefusesSchemaNegatives(t *testing.T) {
	casesFile, err := os.ReadFile(filepath.Join("..", "..", "schema", "bench", "testdata", "invalid.cue"))
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
		if strings.TrimSpace(line) != "" {
			source.WriteString(strings.TrimPrefix(line, "// ") + "\n")
		}
	}
	var cases []struct {
		Name    string `json:"name"`
		Fixture string `json:"fixture"`
		Path    string `json:"path"`
		Value   any    `json:"value"`
		Remove  bool   `json:"remove"`
	}
	if err := cuecontext.New().CompileString(source.String()).LookupPath(cue.ParsePath("cases")).Decode(&cases); err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, tc := range cases {
		if tc.Fixture == "prereg" {
			continue
		}
		seen++
		t.Run(tc.Name, func(t *testing.T) {
			data := manifestFixture(t, tc.Fixture)
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
			if tc.Remove {
				delete(obj, parts[len(parts)-1])
			} else {
				obj[parts[len(parts)-1]] = tc.Value
			}
			dir := t.TempDir()
			writeManifest(t, dir, "sample-subject", data)
			_, err := LoadSubjectManifests(dir)
			if err == nil {
				t.Fatal("invalid manifest accepted")
			}
			if key := parts[len(parts)-1]; !strings.Contains(err.Error(), key) {
				t.Fatalf("refusal does not name %s: %v", key, err)
			}
		})
	}
	if seen == 0 {
		t.Fatal("invalid.cue has no subject negatives")
	}
}

func TestLoadSubjectManifestsRefusesMissingOrEmptyDir(t *testing.T) {
	dir := t.TempDir()
	if _, err := LoadSubjectManifests(filepath.Join(dir, "does-not-exist")); err == nil || !strings.Contains(err.Error(), "reading manifests") {
		t.Fatalf("missing dir: %v", err)
	}
	if _, err := LoadSubjectManifests(dir); err == nil || !strings.Contains(err.Error(), "no *.yaml subject manifests") {
		t.Fatalf("empty dir: %v", err)
	}
}

// Values the schema computes survive: prior reads force the dev split.
func TestLoadSubjectManifestsDecodesSchemaValue(t *testing.T) {
	dir := t.TempDir()
	data := manifestFixture(t, "subject")
	bench := data["bench"].(map[string]any)
	delete(bench, "split")
	bench["contamination"].(map[string]any)["prior_reads"] = []string{"docs/bench/scenario-runs.md"}
	writeManifest(t, dir, "sample-subject", data)
	subjects, err := LoadSubjectManifests(dir)
	if err != nil {
		t.Fatal(err)
	}
	if subjects[0].Bench.Split != "dev" {
		t.Fatalf("split = %q, want dev from the schema", subjects[0].Bench.Split)
	}
}

func TestLoadSubjectManifestsRealBenchD(t *testing.T) {
	dir := filepath.Join("..", "..", "calibration", "bench.d")
	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no bench.d manifests: %v", err)
	}
	subjects, err := LoadSubjectManifests(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(subjects) != len(files) {
		t.Fatalf("loaded %d subjects from %d manifests", len(subjects), len(files))
	}
	for i := 1; i < len(subjects); i++ {
		if subjects[i-1].Name >= subjects[i].Name {
			t.Fatalf("not sorted: %s before %s", subjects[i-1].Name, subjects[i].Name)
		}
	}
}
