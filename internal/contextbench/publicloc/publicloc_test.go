package publicloc

import (
	"bytes"
	"encoding/json"
	"flag"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/contextbench"
)

var update = flag.Bool("update", false, "rewrite testdata/corpus.golden.json from the fixtures")

// fixtureEnv reads base_commit snapshots and callers truth from testdata/.
func fixtureEnv(t *testing.T) Env {
	t.Helper()
	return Env{
		ReadFile: func(id, rel string) ([]byte, error) {
			return os.ReadFile(filepath.Join("testdata", "checkouts", id, filepath.FromSlash(rel)))
		},
		Callers: func(id string) (CallersFile, error) {
			raw, err := os.ReadFile(filepath.Join("testdata", "callers", id+".json"))
			if err != nil {
				return CallersFile{}, err
			}
			var cf CallersFile
			return cf, json.Unmarshal(raw, &cf)
		},
	}
}

func loadFixtures(t *testing.T) ([]Instance, []Pin) {
	t.Helper()
	var all []Instance
	var pins []Pin
	for _, f := range []struct {
		source, file string
		read         func([]byte) ([]Instance, error)
	}{
		{SourceSWEExplore, "swe-explore-go.jsonl", func(b []byte) ([]Instance, error) { return ReadSWEExplore(bytes.NewReader(b)) }},
		{SourceMultiSWEBench, "multi-swe-go.jsonl", func(b []byte) ([]Instance, error) { return ReadMultiSWEBench(bytes.NewReader(b)) }},
	} {
		raw, err := os.ReadFile(filepath.Join("testdata", f.file))
		if err != nil {
			t.Fatal(err)
		}
		insts, err := f.read(raw)
		if err != nil {
			t.Fatalf("%s: %v", f.file, err)
		}
		all = append(all, insts...)
		pins = append(pins, Pin{Source: f.source, Revision: "fixture", File: f.file, SHA256: SHA256Hex(raw), Rows: len(insts)})
	}
	return all, pins
}

func TestDerivationIsDeterministic(t *testing.T) {
	insts, pins := loadFixtures(t)
	env := fixtureEnv(t)

	first, err := BuildCorpus(insts, pins, RegisteredRule, env)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildCorpus(insts, pins, RegisteredRule, env)
	if err != nil {
		t.Fatal(err)
	}
	if first.CorpusSHA256 != second.CorpusSHA256 {
		t.Fatalf("same inputs, different corpus_sha256: %s vs %s", first.CorpusSHA256, second.CorpusSHA256)
	}

	// Shuffled instance order and reversed pin order must not move the hash:
	// the corpus is a function of the pinned inputs, not of how the
	// operator listed them.
	shuffled := append([]Instance(nil), insts...)
	rand.New(rand.NewSource(7)).Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	reversedPins := []Pin{pins[1], pins[0]}
	third, err := BuildCorpus(shuffled, reversedPins, RegisteredRule, env)
	if err != nil {
		t.Fatal(err)
	}
	if third.CorpusSHA256 != first.CorpusSHA256 {
		t.Fatalf("shuffled inputs changed corpus_sha256: %s vs %s", third.CorpusSHA256, first.CorpusSHA256)
	}
	if !reflect.DeepEqual(third, first) {
		t.Fatalf("shuffled inputs changed the corpus body")
	}

	// The hash is over the canonical bytes with the field blank, and the
	// canonical bytes round-trip through LoadCorpus.
	if got := CorpusSHA256(first); got != first.CorpusSHA256 {
		t.Errorf("recomputed hash %s != stored %s", got, first.CorpusSHA256)
	}
	canon, err := MarshalCanonical(first)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "corpus.json")
	if err := os.WriteFile(path, canon, 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadCorpus(path)
	if err != nil {
		t.Fatalf("LoadCorpus: %v", err)
	}
	if !reflect.DeepEqual(loaded, first) {
		t.Errorf("corpus did not round-trip through its canonical encoding")
	}
	// A hand edit after derivation is caught at load.
	if err := os.WriteFile(path, bytes.Replace(canon, []byte(`"version": 1`), []byte(`"version": 2`), 1), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCorpus(path); err == nil || !strings.Contains(err.Error(), "corpus_sha256") {
		t.Errorf("edited corpus loaded without a hash error: %v", err)
	}
}

func TestBuildCorpus_MatchesGolden(t *testing.T) {
	insts, pins := loadFixtures(t)
	c, err := BuildCorpus(insts, pins, RegisteredRule, fixtureEnv(t))
	if err != nil {
		t.Fatal(err)
	}
	got, err := MarshalCanonical(c)
	if err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join("testdata", "corpus.golden.json")
	if *update {
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("reading golden (run with -update to create it): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("corpus differs from %s (re-run with -update if the derivation rule changed on purpose):\n%s", golden, firstDiff(got, want))
	}
}

func firstDiff(a, b []byte) string {
	al, bl := strings.Split(string(a), "\n"), strings.Split(string(b), "\n")
	for i := 0; i < len(al) && i < len(bl); i++ {
		if al[i] != bl[i] {
			return "line " + itoa(i+1) + ":\n got: " + al[i] + "\nwant: " + bl[i]
		}
	}
	return "lengths differ: got " + itoa(len(al)) + " lines, want " + itoa(len(bl))
}

func itoa(n int) string { return strconv.Itoa(n) }

func TestBuildCorpus_SelectionAndDerivationShape(t *testing.T) {
	insts, pins := loadFixtures(t)
	c, err := BuildCorpus(insts, pins, RegisteredRule, fixtureEnv(t))
	if err != nil {
		t.Fatal(err)
	}
	sel := c.Selection
	if !sel.PreRegistered {
		t.Errorf("registered rule reported as unregistered")
	}
	// 5 multi-swe rows + 3 swe-explore rows (the python row is skipped by
	// the reader): one test-only, one six-file, one duplicate.
	if sel.Candidates != 8 || sel.Selected != 5 || sel.ExcludedNoGoldFiles != 1 || sel.ExcludedTooManyFiles != 1 || sel.ExcludedDuplicate != 1 || sel.ExcludedBeyondN != 0 {
		t.Errorf("selection report = %+v", sel)
	}
	var ids []string
	for _, in := range c.Instances {
		ids = append(ids, in.InstanceID)
	}
	wantIDs := []string{"example-org__gadgets-1", "example-org__gadgets-2", "example-org__widgets-42", "swe-explore-tools-8", "swe-explore-tools-9"}
	if !reflect.DeepEqual(ids, wantIDs) {
		t.Errorf("instances = %v, want %v", ids, wantIDs)
	}

	byID := map[string]Task{}
	for _, task := range c.Tasks {
		byID[task.ID] = task
	}
	if len(c.Tasks) != 14 {
		t.Errorf("tasks = %d, want 14", len(c.Tasks))
	}
	cases := map[string][]string{
		"PG-example-org__widgets-42-file":              {"cmd/widgets/main.go", "pkg/store/store.go"},
		"PG-example-org__widgets-42-func":              {"Open", "Store.Get"},
		"PG-example-org__widgets-42-callers-Open":      {"Run", "main"},
		"PG-example-org__widgets-42-callers-Store_Get": {"Run"},
		"PG-example-org__gadgets-1-callers-Parse":      {"Format"},
		"PG-example-org__gadgets-2-func":               {"Legacy"},
		"PG-swe-explore-tools-9-file":                  {"internal/run/run.go"},
		"PG-swe-explore-tools-9-func":                  {"Runner.Exec"},
		"PG-swe-explore-tools-8-func":                  {"exec"},
		"PG-swe-explore-tools-8-callers-exec":          {"Runner.Exec"},
	}
	for id, want := range cases {
		task, ok := byID[id]
		if !ok {
			t.Errorf("task %s missing", id)
			continue
		}
		if !reflect.DeepEqual(task.GroundTruth, want) {
			t.Errorf("%s ground_truth = %v, want %v", id, task.GroundTruth, want)
		}
		if task.MaxPredicted != MaxPredicted(len(want)) {
			t.Errorf("%s max_predicted = %d", id, task.MaxPredicted)
		}
		if task.Subject.RepoURL == "" || task.Subject.BaseCommit == "" || !strings.HasPrefix(task.Subject.Service, "pub-example-org-") {
			t.Errorf("%s subject = %+v", id, task.Subject)
		}
		if task.Derivation.Rule == "" || task.Derivation.Tool == "" {
			t.Errorf("%s derivation is blank", id)
		}
		if len(task.Requires) == 0 || task.Requires[0] != RequiresSubjectSnapshot {
			t.Errorf("%s requires = %v", id, task.Requires)
		}
	}
	for _, id := range []string{"PG-example-org__gadgets-2-callers-Legacy", "PG-swe-explore-tools-9-callers-Runner_Exec"} {
		if _, ok := byID[id]; ok {
			t.Errorf("%s should have been omitted (no callers / over the cap)", id)
		}
	}
	if task := byID["PG-example-org__widgets-42-callers-Store_Get"]; task.Requires[1] != RequiresTypedCallEdges {
		t.Errorf("find_callers requires = %v", task.Requires)
	}
	if omitted := strings.Join(c.Instances[4].Omitted, "|"); !strings.Contains(omitted, "exceeds the cap") {
		t.Errorf("tools-9 omission not recorded: %q", omitted)
	}
	if notes := byID["PG-example-org__widgets-42-func"].Derivation.Notes; len(notes) != 1 || !strings.Contains(notes[0], "cmd/widgets/main.go") {
		t.Errorf("top-level insertion note missing: %v", notes)
	}
	for _, task := range c.Tasks {
		if strings.Contains(task.Prompt, "diff --git") || strings.Contains(task.Prompt, "@@") {
			t.Errorf("%s: the prompt leaks the gold patch", task.ID)
		}
	}
}

func TestBuildCorpus_LoadsThroughContextbench(t *testing.T) {
	insts, pins := loadFixtures(t)
	c, err := BuildCorpus(insts, pins, SelectionRule{MinFiles: 1, MaxFiles: 5, N: 3}, fixtureEnv(t))
	if err != nil {
		t.Fatal(err)
	}
	if c.Selection.PreRegistered || c.Selection.ExcludedBeyondN != 2 || c.Selection.Selected != 3 {
		t.Errorf("N=3 selection = %+v", c.Selection)
	}
	raw, err := MarshalCanonical(c)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "tasks-public-go.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	tasks, err := contextbench.LoadTasks(path)
	if err != nil {
		t.Fatalf("the runner's loader rejects the public corpus: %v", err)
	}
	if len(tasks) != len(c.Tasks) {
		t.Errorf("runner loaded %d tasks, corpus has %d", len(tasks), len(c.Tasks))
	}
	for _, task := range tasks {
		if task.Class != contextbench.ClassFileLocalization && task.Class != contextbench.ClassFunctionLocalization && task.Class != contextbench.ClassFindCallers {
			t.Errorf("task %s has class %q", task.ID, task.Class)
		}
	}
	crit, err := contextbench.LoadCriterion(path)
	if err != nil {
		t.Fatalf("LoadCriterion: %v", err)
	}
	if crit.ID != "H2-2" || !crit.RequireStrictlyMoreSuccesses || crit.MaxTokenRatio != 1.0 || crit.MinTrials != 3 {
		t.Errorf("criterion loaded as %+v", crit)
	}
}

func TestBuildCorpus_NoCallersTruthIsRecordedNotSilent(t *testing.T) {
	insts, pins := loadFixtures(t)
	env := fixtureEnv(t)
	env.Callers = nil
	c, err := BuildCorpus(insts, pins, RegisteredRule, env)
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range c.Tasks {
		if task.Class == "find_callers" {
			t.Fatalf("find_callers task derived without callers truth: %s", task.ID)
		}
	}
	if len(c.Tasks) != 10 {
		t.Errorf("tasks = %d, want 10 (file + function only)", len(c.Tasks))
	}
	if !strings.Contains(strings.Join(c.Instances[0].Omitted, "|"), "cmd/loctruth") {
		t.Errorf("omission not recorded: %v", c.Instances[0].Omitted)
	}
	env.ReadFile = nil
	c, err = BuildCorpus(insts, pins, RegisteredRule, env)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Tasks) != 5 {
		t.Errorf("tasks without a snapshot reader = %d, want 5 (file only)", len(c.Tasks))
	}
}

func TestBuildCorpus_MissingSnapshotIsAnError(t *testing.T) {
	insts, pins := loadFixtures(t)
	env := fixtureEnv(t)
	env.ReadFile = func(id, rel string) ([]byte, error) { return nil, os.ErrNotExist }
	if _, err := BuildCorpus(insts, pins, RegisteredRule, env); err == nil || !strings.Contains(err.Error(), "base_commit") {
		t.Errorf("missing snapshot did not fail loudly: %v", err)
	}
	env = fixtureEnv(t)
	env.Callers = func(id string) (CallersFile, error) { return CallersFile{}, os.ErrNotExist }
	if _, err := BuildCorpus(insts, pins, RegisteredRule, env); err == nil || !strings.Contains(err.Error(), "callers truth") {
		t.Errorf("missing callers truth did not fail loudly: %v", err)
	}
}

func TestParsePatch(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "multi-swe-go.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	insts, err := ReadMultiSWEBench(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	var widgets Instance
	for _, in := range insts {
		if in.InstanceID == "example-org__widgets-42" {
			widgets = in
		}
	}
	files, err := ParsePatch(widgets.Patch)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 4 {
		t.Fatalf("files = %d, want 4", len(files))
	}
	if files[0].OldPath != "pkg/store/store.go" || len(files[0].Hunks) != 2 || files[0].Hunks[1].OldStart != 18 || files[0].Hunks[1].OldLines != 7 {
		t.Errorf("store.go section = %+v", files[0])
	}
	if !files[3].Added || files[3].OldPath != "" || files[3].NewPath != "pkg/store/new.go" {
		t.Errorf("added file section = %+v", files[3])
	}
	touch := TouchedOldLines(files[0].Hunks[0])
	if !reflect.DeepEqual(touch, Touch{Inserted: []int{13}}) {
		t.Errorf("Open hunk touch = %+v", touch)
	}
	touch = TouchedOldLines(files[0].Hunks[1])
	if !reflect.DeepEqual(touch, Touch{Deleted: []int{20}, Inserted: []int{21}}) {
		t.Errorf("Get hunk touch = %+v", touch)
	}
	gold, err := GoldFiles(widgets)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gold, []string{"cmd/widgets/main.go", "pkg/store/store.go"}) {
		t.Errorf("gold files = %v", gold)
	}

	deleted, err := ParsePatch("diff --git a/lib/old.go b/lib/old.go\ndeleted file mode 100644\n--- a/lib/old.go\n+++ /dev/null\n@@ -1,2 +0,0 @@\n-package lib\n-\n")
	if err != nil {
		t.Fatal(err)
	}
	if !deleted[0].Deleted || deleted[0].OldPath != "lib/old.go" || len(deleted[0].Hunks[0].Body) != 2 {
		t.Errorf("deleted section = %+v", deleted[0])
	}
	if _, err := ParsePatch("--- a/x.go\n+++ b/x.go\n"); err == nil {
		t.Errorf("header before diff --git accepted")
	}
	if _, err := ParsePatch("diff --git a/x.go b/x.go\n--- a/x.go\n+++ b/x.go\n@@ -1,2 +1,2 @@\n-a\n+b\ndiff --git a/y.go b/y.go\n"); err == nil {
		t.Errorf("short hunk body accepted")
	}
	if _, err := ParsePatch("diff --git a/x.go b/x.go\n--- a/x.go\n+++ b/x.go\n@@ nonsense @@\n"); err == nil {
		t.Errorf("malformed hunk header accepted")
	}
}

func TestIsTestPath(t *testing.T) {
	for p, want := range map[string]bool{
		"pkg/a_test.go":           true,
		"testdata/x.go":           true,
		"pkg/testdata/x.go":       true,
		"test/x.go":               true,
		"internal/fixtures/x.go":  true,
		"pkg/a.go":                false,
		"pkg/tester/a.go":         false,
		"pkg/attestation/attx.go": false,
	} {
		if got := IsTestPath(p); got != want {
			t.Errorf("IsTestPath(%q) = %v, want %v", p, got, want)
		}
	}
	if IsGoSource("docs/readme.md") || IsGoSource("a_test.go") || !IsGoSource("a/b.go") {
		t.Errorf("IsGoSource wrong")
	}
}

func TestEnclosingFuncsForHunks(t *testing.T) {
	src := []byte(`package p

func A() {
	x := 1
	_ = x
}

func (r *R) B() {
	y := 2
	_ = y
}

func C[T any](v T) T { return v }
`)
	spans, err := FuncSpans(src, "p.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(spans) != 3 || spans[1].Name != "R.B" || spans[1].Start != 8 || spans[1].End != 11 {
		t.Errorf("spans = %+v", spans)
	}
	cases := []struct {
		name  string
		touch []Touch
		want  []string
	}{
		{"deleted line inside A", []Touch{{Deleted: []int{4}}}, []string{"A"}},
		{"insert inside B", []Touch{{Inserted: []int{10}}}, []string{"R.B"}},
		{"insert between A and B", []Touch{{Inserted: []int{7}}}, nil},
		{"insert at top of file", []Touch{{Inserted: []int{1}}}, nil},
		{"delete signature of C", []Touch{{Deleted: []int{13}}}, []string{"C"}},
		{"both", []Touch{{Deleted: []int{5}}, {Deleted: []int{9}}}, []string{"A", "R.B"}},
	}
	for _, tc := range cases {
		got := EnclosingFuncs(spans, tc.touch)
		var names []string
		for _, s := range got {
			names = append(names, s.Name)
		}
		if !reflect.DeepEqual(names, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, names, tc.want)
		}
	}
	hunks := []Hunk{{OldStart: 3, OldLines: 4, NewStart: 3, NewLines: 4, Body: []string{" func A() {", "-\tx := 1", "+\tx := 2", " \t_ = x", " }"}}}
	got, err := EnclosingFuncsForHunks(src, "p.go", hunks)
	if err != nil || len(got) != 1 || got[0].Name != "A" {
		t.Errorf("EnclosingFuncsForHunks = %+v, %v", got, err)
	}
	if _, err := EnclosingFuncsForHunks([]byte("not go"), "bad.go", hunks); err == nil {
		t.Errorf("unparseable source accepted")
	}
	regions, err := EnclosingFuncsForRegions(src, "p.go", []Region{{Path: "p.go", Start: 9, End: 10}})
	if err != nil || len(regions) != 1 || regions[0].Name != "R.B" {
		t.Errorf("EnclosingFuncsForRegions = %+v, %v", regions, err)
	}
}

func TestReaders_RejectMalformedRows(t *testing.T) {
	cases := map[string]string{
		"multi: no sha":         `{"org":"o","repo":"r","number":1,"fix_patch":"x"}`,
		"multi: no patch":       `{"org":"o","repo":"r","number":1,"base":{"sha":"abc"}}`,
		"multi: no org":         `{"repo":"r","number":1,"base":{"sha":"abc"},"fix_patch":"x"}`,
		"multi: not json":       `{`,
		"explore: no id":        `{"repo":"o/r","base_commit":"abc","problem_statement":"p","patch":"x"}`,
		"explore: bad repo":     `{"instance_id":"i","repo":"r","base_commit":"abc","problem_statement":"p","patch":"x"}`,
		"explore: no truth":     `{"instance_id":"i","repo":"o/r","base_commit":"abc","problem_statement":"p"}`,
		"explore: bad region":   `{"instance_id":"i","repo":"o/r","base_commit":"abc","problem_statement":"p","regions":{"a.go":[[5]]}}`,
		"explore: region order": `{"instance_id":"i","repo":"o/r","base_commit":"abc","problem_statement":"p","regions":[{"file":"a.go","start":9,"end":3}]}`,
	}
	for name, row := range cases {
		var err error
		if strings.HasPrefix(name, "multi") {
			_, err = ReadMultiSWEBench(strings.NewReader(row + "\n"))
		} else {
			_, err = ReadSWEExplore(strings.NewReader(row + "\n"))
		}
		if err == nil {
			t.Errorf("%s: accepted", name)
		} else if !strings.Contains(err.Error(), "line 1") {
			t.Errorf("%s: error does not name the line: %v", name, err)
		}
	}
	// Blank lines are skipped; an instance_id is derived when absent.
	insts, err := ReadMultiSWEBench(strings.NewReader("\n" + `{"org":"o","repo":"r","number":9,"title":"t","body":"b","base":{"sha":"abc"},"fix_patch":"x"}` + "\n\n"))
	if err != nil || len(insts) != 1 || insts[0].InstanceID != "o__r-9" || insts[0].Problem != "t\n\nb" {
		t.Errorf("derived id row = %+v, %v", insts, err)
	}
	if insts[0].Service() != "pub-o-r-abc" || insts[0].RepoURL() != "https://github.com/o/r" {
		t.Errorf("service/url = %q %q", insts[0].Service(), insts[0].RepoURL())
	}
}

func TestSelect_UnregisteredRuleIsLabelled(t *testing.T) {
	if RegisteredRule.PreRegistered() != true {
		t.Fatal("the registered rule must report pre-registered")
	}
	for _, r := range []SelectionRule{{1, 5, 29}, {1, 5, 41}, {1, 6, 35}, {2, 5, 35}} {
		if r.PreRegistered() {
			t.Errorf("%+v reported as pre-registered", r)
		}
	}
	if MaxPredicted(1) != 5 || MaxPredicted(2) != 5 || MaxPredicted(3) != 6 {
		t.Errorf("MaxPredicted wrong")
	}
}
