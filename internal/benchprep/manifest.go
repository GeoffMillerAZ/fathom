package benchprep

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"

	"github.com/geoffmilleraz/fathom/schema/bench"
)

// c1Licences are the SPDX ids subject vetting admits (bench-prep section 5).
var c1Licences = map[string]bool{"MIT": true, "Apache-2.0": true, "BSD-2-Clause": true, "BSD-3-Clause": true, "CC0-1.0": true}

// Subject mirrors schema/bench's #Subject: the corpus.d repo entry plus bench.
type Subject struct {
	Name    string       `json:"name"`
	URL     string       `json:"url"`
	Ref     string       `json:"ref"`
	SHA     string       `json:"sha"`
	Licence string       `json:"licence"`
	Subpath string       `json:"subpath,omitempty"`
	Idioms  []string     `json:"idioms,omitempty"`
	Sparse  []string     `json:"sparse,omitempty"`
	Notes   string       `json:"notes,omitempty"`
	Only    []string     `json:"only,omitempty"`
	Bench   SubjectBench `json:"bench"`
}

// SubjectBench is the manifest's bench block. SuiteTests and Excluded mirror
// fields the schema gains with BX-83; both stay optional here.
type SubjectBench struct {
	Toolchain struct {
		Versions       map[string]string `json:"versions"`
		LockfileSHA256 string            `json:"lockfile_sha256"`
	} `json:"toolchain"`
	Setup         []string       `json:"setup"`
	Test          []string       `json:"test"`
	SuiteSeconds  float64        `json:"suite_seconds"`
	FlakeRuns     int            `json:"flake_runs"`
	L0Reading     string         `json:"l0_reading"`
	Split         string         `json:"split,omitempty"`
	TaskSources   []TaskSource   `json:"task_sources"`
	Contamination Contamination  `json:"contamination"`
	SuiteTests    int            `json:"suite_tests,omitempty"`
	Excluded      []ExcludedTest `json:"excluded,omitempty"`
}

// TaskSource names a dataset a subject's tasks may come from.
type TaskSource struct {
	Dataset  string `json:"dataset"`
	Revision string `json:"revision"`
	Licence  string `json:"licence"`
}

// Contamination records the subject's exposure at vetting (D23, D25).
type Contamination struct {
	Age            float64  `json:"age"`
	PopularityBand string   `json:"popularity_band"`
	PriorReads     []string `json:"prior_reads,omitempty"`
	FreshAfter     string   `json:"fresh_after,omitempty"`
}

// ExcludedTest is a test the manifest's own command leaves out, with why.
type ExcludedTest struct {
	Test   string `json:"test"`
	Reason string `json:"reason"`
	Log    string `json:"log"`
}

// LoadSubjectManifests reads dir/*.yaml (JSON text with full-line # comments),
// validates each against #Subject through the CUE Go API and returns them
// sorted by name. A file stem other than the subject name, a missing sha and
// a licence off the C1 list are refused before the schema is consulted.
func LoadSubjectManifests(dir string) ([]Subject, error) {
	if _, err := os.ReadDir(dir); err != nil {
		return nil, fmt.Errorf("reading manifests: %w", err)
	}
	paths, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("%s: no *.yaml subject manifests", dir)
	}
	sort.Strings(paths)
	ctx := cuecontext.New()
	schema := ctx.CompileString(bench.Source)
	if err := schema.Err(); err != nil {
		return nil, fmt.Errorf("compiling bench schema: %w", err)
	}
	def := schema.LookupPath(cue.ParsePath("#Subject"))
	var subjects []Subject
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var source strings.Builder
		for _, line := range strings.Split(string(raw), "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "#") {
				source.WriteString(line + "\n")
			}
		}
		var s Subject
		if err := json.Unmarshal([]byte(source.String()), &s); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		stem := strings.TrimSuffix(filepath.Base(path), ".yaml")
		switch {
		case stem != s.Name:
			return nil, fmt.Errorf("%s: file stem %q is not the subject name %q", path, stem, s.Name)
		case s.SHA == "":
			return nil, fmt.Errorf("%s: missing sha", path)
		case !c1Licences[s.Licence]:
			return nil, fmt.Errorf("%s: licence %q is off the C1 list", path, s.Licence)
		}
		value := def.Unify(ctx.CompileString(source.String()))
		if err := value.Validate(cue.Concrete(true)); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		// Decode the validated value so fields the schema computes survive.
		s = Subject{}
		if err := value.Decode(&s); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		subjects = append(subjects, s)
	}
	sort.Slice(subjects, func(i, j int) bool { return subjects[i].Name < subjects[j].Name })
	return subjects, nil
}
