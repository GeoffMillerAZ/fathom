package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/langbench"
)

const earlierTiming = "throughput (design 1.9 T1, T2): VM runs 0.11, 0.13, 0.12 MB/CPU-s"

func retimeFixture(t *testing.T) (string, string) {
	t.Helper()
	t.Chdir(filepath.Join("..", ".."))
	root := t.TempDir()
	for _, f := range []string{"a.lua", "b.lua", "outside.lua"} {
		if err := os.WriteFile(filepath.Join(root, f), []byte(strings.Repeat("-- a comment keeping the fixture readable and above the throughput floor\n", 100)+"function hello() return 1 end\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	git := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = root
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git: %v: %s", err, out)
		}
	}
	git("init", "-q")
	git("add", ".")
	// Write a fixture commit without running git commit or hooks.
	tree := gitOut(t.Context(), root, "write-tree")
	c := exec.Command("git", "commit-tree", tree, "-m", "fixture")
	c.Dir = root
	c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid")
	sha, err := c.Output()
	if err != nil {
		t.Fatal(err)
	}
	git("update-ref", "HEAD", strings.TrimSpace(string(sha)))
	path := filepath.Join(t.TempDir(), "lua.json")
	tags := ""
	for _, f := range []string{"a.lua", "b.lua"} {
		tags += `{"_type":"tag","name":"hello","path":"` + f + `","line":101,"end":101,"kind":"function","signature":"()"}` + "\n"
	}
	ctags := fakeCtags(t, t.TempDir(), tags)
	stubFileClocks(t, time.Second, time.Second, true)
	for _, name := range []string{"first", "second"} {
		var out, errs bytes.Buffer
		code := cmdLangOracle([]string{"-dialect", "lua", "-manifest", "internal/extractor/dialects/lua/dialect.json", "-subject", root, "-subject-name", name, "-only", "a.lua,b.lua", "-ctags", ctags, "-out", path, "-commit", "original"}, &out, &errs)
		if code != 0 {
			t.Fatalf("oracle exit %d: %s", code, errs.String())
		}
	}
	r := readReading(t, path)
	for i := range r.Subjects {
		r.Subjects[i].Tenancy.CompiledBytes = int64(100 + i)
		r.Subjects[i].Tenancy.PeakModulePages = 128 + i
	}
	langbench.Regrade(&r)
	r.Notes = append(r.Notes, earlierTiming)
	saveRetimeReading(t, path, r)
	return root, path
}

func saveRetimeReading(t *testing.T, path string, r langbench.Reading) {
	t.Helper()
	b, err := langbench.MarshalReading(r)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
}

func retimeArgs(root, path string) []string {
	return []string{"lang-retime", "-reading", path, "-subject", "second=" + root, "-subject", "first=" + root}
}

func TestCmdLangRetime(t *testing.T) {
	root, path := retimeFixture(t)
	before := readReading(t, path)
	if before.LevelMet != "none" {
		t.Fatalf("fixture must fail throughput: %s", before.LevelMet)
	}
	stubFileClocks(t, 11*time.Millisecond, time.Millisecond, true)
	var calls int
	var cpu time.Duration = time.Second
	fileClocks.cpu = func() time.Duration {
		steps := []time.Duration{time.Millisecond, 8 * time.Millisecond, 6 * time.Millisecond, 6 * time.Millisecond, 7 * time.Millisecond, time.Millisecond}
		subjectRun := calls / 4 // two clock reads per file, two files per subject
		if subjectRun >= len(steps) {
			t.Fatal("unexpected extra timed files or runs")
		}
		cpu += steps[subjectRun]
		calls++
		return cpu
	}
	var out, errs bytes.Buffer
	args := append(retimeArgs(root, path), "-write")
	if code := mainRun(args, &out, &errs); code != 0 {
		t.Fatalf("exit %d: %s", code, errs.String())
	}
	after := readReading(t, path)
	if after.LevelMet != "L1" {
		t.Fatalf("level = %s, grades %+v", after.LevelMet, after.Grade)
	}
	for i, s := range after.Subjects {
		// Pooled totals select run 1 (18ms), between run 2 (24ms) and
		// run 3 (16ms). Taking subject medians would mix runs instead.
		wantCPU, wantMax := float64(2), int64(1)
		if s.Name == "second" {
			wantCPU, wantMax = 16, 8
		}
		if s.Tenancy.ExtractCPUMS != wantCPU || s.Tenancy.ExtractMS != 22 || s.Tenancy.MaxFileMS != wantMax {
			t.Fatalf("not median run: %+v", s.Tenancy)
		}
		s.Tenancy.ExtractCPUMS = before.Subjects[i].Tenancy.ExtractCPUMS
		s.Tenancy.ExtractMS = before.Subjects[i].Tenancy.ExtractMS
		s.Tenancy.MaxFileMS = before.Subjects[i].Tenancy.MaxFileMS
		if !reflect.DeepEqual(s, before.Subjects[i]) {
			t.Fatal("subject changed outside timing fields")
		}
	}
	if after.FathomCommit != before.FathomCommit || after.MeasuredAt != before.MeasuredAt || !reflect.DeepEqual(after.Oracles, before.Oracles) || !reflect.DeepEqual(after.Metrics, before.Metrics) {
		t.Fatal("original measurement changed")
	}
	var notes []string
	for _, n := range after.Notes {
		if strings.HasPrefix(n, "throughput (design 1.9 T1, T2)") {
			notes = append(notes, n)
		}
	}
	if len(notes) != 1 || !strings.Contains(notes[0], "earlier: "+earlierTiming) || !strings.Contains(notes[0], "original") || !strings.Contains(notes[0], "median run 1") {
		t.Fatalf("disclosure: %v", notes)
	}
	for _, want := range []string{"run 1: 1.63 MB/CPU-s", "run 2: 1.22 MB/CPU-s", "run 3: 1.83 MB/CPU-s", "median run 1: 1.63 MB/CPU-s"} {
		if !slices.Contains(strings.Split(notes[0], "; "), want) {
			t.Errorf("disclosure lacks %q: %s", want, notes[0])
		}
	}
	if !strings.Contains(notes[0], ": "+runtime.GOOS+"/"+runtime.GOARCH+" ") {
		t.Errorf("disclosure lacks machine class: %s", notes[0])
	}
	if hostname, err := os.Hostname(); err != nil {
		t.Fatal(err)
	} else if hostname != "" && strings.Contains(notes[0], hostname) {
		t.Fatal("disclosure contains hostname")
	}
	for _, want := range []string{"run 1", "run 2", "run 3", "median", "none -> L1", "grade"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("summary lacks %q: %s", want, &out)
		}
	}
}

func TestCmdLangRetimeRefuses(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*testing.T, string, string)
		want   string
		code   int
	}{
		{"changed file", func(t *testing.T, root, path string) {
			if err := os.WriteFile(filepath.Join(root, "a.lua"), []byte("function changed() end\n"), 0600); err != nil {
				t.Fatal(err)
			}
		}, "dirty checkout", 1},
		{"all measured files removed", func(t *testing.T, root, path string) {
			for _, name := range []string{"a.lua", "b.lua"} {
				if err := os.Remove(filepath.Join(root, name)); err != nil {
					t.Fatal(err)
				}
			}
		}, "dirty checkout", 1},
		{"same-size edit", func(t *testing.T, root, path string) {
			p := filepath.Join(root, "a.lua")
			b, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			line := strings.SplitN(string(b), "\n", 2)[0]
			replacement := "function zz() return 2 end"
			b = []byte(strings.Replace(string(b), line, replacement+strings.Repeat(" ", len(line)-len(replacement)), 1))
			if err := os.WriteFile(p, b, 0600); err != nil {
				t.Fatal(err)
			}
		}, "dirty checkout", 1},
		{"untracked file outside scope", func(t *testing.T, root, path string) {
			if err := os.WriteFile(filepath.Join(root, "untracked.txt"), []byte("new"), 0600); err != nil {
				t.Fatal(err)
			}
		}, "dirty checkout", 1},
		{"extractor mismatch outside repo", func(t *testing.T, root, path string) {
			t.Chdir(t.TempDir())
		}, "extractor", 1},
		{"fathom definitions mismatch", func(t *testing.T, root, path string) {
			r := readReading(t, path)
			r.Subjects[0].Counts[langbench.CntFathomDefs]++
			saveRetimeReading(t, path, r)
		}, "counts.fathom_defs", 1},
		{"callables mismatch", func(t *testing.T, root, path string) {
			r := readReading(t, path)
			r.Subjects[0].Counts[langbench.CntCallables]++
			saveRetimeReading(t, path, r)
		}, "counts.callables", 1},
		{"signed callables mismatch", func(t *testing.T, root, path string) {
			r := readReading(t, path)
			r.Subjects[0].Counts[langbench.CntCallablesSigned]--
			saveRetimeReading(t, path, r)
		}, "counts.callables_signed", 1},
		{"excluded kind disappears", func(t *testing.T, root, path string) {
			r := readReading(t, path)
			r.Subjects[0].ExcludedByKind["fathom:OldKind"] = 1
			saveRetimeReading(t, path, r)
		}, "excluded_by_kind.fathom:OldKind", 1},
		{"imports mismatch", func(t *testing.T, root, path string) {
			r := readReading(t, path)
			r.Subjects[0].Families[langbench.FamilyImports] = "A"
			r.Subjects[0].Counts[langbench.CntImports] = 1
			saveRetimeReading(t, path, r)
		}, "counts.imports", 1},
		{"sha mismatch", func(t *testing.T, root, path string) {
			r := readReading(t, path)
			r.Subjects[0].SHA = strings.Repeat("0", 40)
			saveRetimeReading(t, path, r)
		}, "sha", 1},
		{"count mismatch", func(t *testing.T, root, path string) {
			r := readReading(t, path)
			r.Subjects[0].Counts["type_refs"] = 99
			saveRetimeReading(t, path, r)
		}, "counts.type_refs", 1},
		{"parse errors mismatch", func(t *testing.T, root, path string) {
			r := readReading(t, path)
			n := 1
			r.Subjects[0].Tenancy.FilesWithError = &n
			saveRetimeReading(t, path, r)
		}, "files_with_error", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, path := retimeFixture(t)
			tc.change(t, root, path)
			original, _ := os.ReadFile(path)
			var out, errs bytes.Buffer
			if code := mainRun(append(retimeArgs(root, path), "-write"), &out, &errs); code != tc.code {
				t.Fatalf("exit %d: %s", code, &errs)
			}
			if !strings.Contains(errs.String(), tc.want) {
				t.Fatalf("missing diff %q: %s", tc.want, &errs)
			}
			if tc.want != "sha" && !strings.Contains(errs.String(), "full re-measure") {
				t.Fatalf("missing advice: %s", &errs)
			}
			got, _ := os.ReadFile(path)
			if !bytes.Equal(got, original) {
				t.Fatal("refusal wrote reading")
			}
		})
	}
}

func TestCmdLangRetimeUsageAndDryRun(t *testing.T) {
	root, path := retimeFixture(t)
	for _, args := range [][]string{{"lang-retime"}, {"lang-retime", "-reading", path, "-subject", "first=" + root}, append(retimeArgs(root, path), "-runs", "2")} {
		var out, errs bytes.Buffer
		if code := mainRun(args, &out, &errs); code != 2 {
			t.Fatalf("exit %d, want usage: %s", code, &errs)
		}
	}
	stubFileClocks(t, time.Millisecond, time.Millisecond, true)
	original, _ := os.ReadFile(path)
	var out, errs bytes.Buffer
	if code := mainRun(retimeArgs(root, path), &out, &errs); code != 0 {
		t.Fatalf("exit %d: %s", code, &errs)
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, original) {
		t.Fatal("dry run wrote reading")
	}
}

func TestCmdLangRetimeChecksEveryRun(t *testing.T) {
	root, path := retimeFixture(t)
	stubFileClocks(t, time.Millisecond, time.Millisecond, true)
	var calls int
	var cpu time.Duration = time.Second
	fileClocks.cpu = func() time.Duration {
		if calls == 8 { // after the first complete run, change the next file to be read
			if err := os.WriteFile(filepath.Join(root, "b.lua"), []byte("function changed() end\n"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		calls++
		cpu += time.Millisecond
		return cpu
	}
	original, _ := os.ReadFile(path)
	var out, errs bytes.Buffer
	if code := mainRun(append(retimeArgs(root, path), "-write"), &out, &errs); code != 1 {
		t.Fatalf("exit %d: %s", code, &errs)
	}
	for _, want := range []string{"run 2 first", "dirty checkout", "full re-measure"} {
		if !strings.Contains(errs.String(), want) {
			t.Fatalf("missing %q: %s", want, &errs)
		}
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, original) {
		t.Fatal("late refusal wrote reading")
	}
}

type retimeMovedExtractor struct{ extractor.Extractor }

func (e retimeMovedExtractor) Extract(service, path string, src []byte) ([]core.SymbolNode, error) {
	nodes, err := e.Extractor.Extract(service, path, src)
	for i := range nodes {
		if nodes[i].Kind == "LuaFunction" {
			nodes[i].Kind = "MovedDefinition"
		}
	}
	return nodes, err
}

func TestRetimeMovedDefinition(t *testing.T) {
	root, path := retimeFixture(t)
	r := readReading(t, path)
	spec, source, err := resolveBenchSpec(r.Dialect, "")
	if err != nil {
		t.Fatal(err)
	}
	with, close, err := langOracleExtractor(spec.Manifest, source)
	if err != nil {
		t.Fatal(err)
	}
	defer close()
	// The subject stays clean and identical. Only extractor output changes,
	// as when tags.scm moves a definition outside the scored kinds.
	opts := oracleOpts{noOracles: true, retimeKinds: r.Subjects[0].KindIntersection, only: []string{"a.lua", "b.lua"}, ctags: "missing", scip: "missing", tools: []string{"missing"}}
	unchanged, err := measureSubject(t.Context(), spec, root, r.Subjects[0].Name, opts, with)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{langbench.CntFathomDefs, langbench.CntCallables, langbench.CntCallablesSigned} {
		if unchanged.subject.Counts[key] != r.Subjects[0].Counts[key] {
			t.Errorf("no-oracle %s = %d, want %d", key, unchanged.subject.Counts[key], r.Subjects[0].Counts[key])
		}
	}
	measured, err := measureSubject(t.Context(), spec, root, r.Subjects[0].Name, opts, retimeMovedExtractor{with})
	if err != nil {
		t.Fatal(err)
	}
	diff := strings.Join(retimeIdentityDiff(r.Subjects[0], measured.subject, false), "\n")
	for _, want := range []string{"counts.fathom_defs", "counts.callables", "counts.callables_signed", "excluded_by_kind.fathom:MovedDefinition"} {
		if !strings.Contains(diff, want) {
			t.Errorf("missing %q in diff: %s", want, diff)
		}
	}
}

func TestRetimeParseErrorFailureNote(t *testing.T) {
	root, path := retimeFixture(t)
	stubFileClocks(t, time.Millisecond, time.Millisecond, true)
	var calls int
	fileClocks.wall = func() time.Time {
		calls++
		if calls == 4 { // extraction has read both files; the error counter reads next
			if err := os.Remove(filepath.Join(root, "b.lua")); err != nil {
				t.Fatal(err)
			}
		}
		return time.Unix(0, int64(calls)*int64(time.Millisecond))
	}
	var out, errs bytes.Buffer
	if code := mainRun(append(retimeArgs(root, path), "-write"), &out, &errs); code != 1 {
		t.Fatalf("exit %d: %s", code, &errs)
	}
	if !strings.Contains(errs.String(), "files_with_error not measured:") || !strings.Contains(errs.String(), "no such file or directory") {
		t.Fatalf("measurement failure note lost: %s", &errs)
	}
}

func TestRetimeImportCountsWithoutOracle(t *testing.T) {
	root := copyFixture(t, "rustfixture")
	// One external import is also a fathom-only count, excluded from imports.
	f, err := os.OpenFile(filepath.Join(root, "src", "main.rs"), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := f.WriteString("\nuse std::fmt;\n")
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		t.Fatalf("append import: %v, %v", writeErr, closeErr)
	}
	t.Chdir(filepath.Join("..", ".."))
	spec, source, err := resolveBenchSpec("rust", "")
	if err != nil {
		t.Fatal(err)
	}
	with, closeExtractor, err := langOracleExtractor(spec.Manifest, source)
	if err != nil {
		t.Fatal(err)
	}
	defer closeExtractor()
	measured, err := measureSubject(t.Context(), spec, root, "fixture", oracleOpts{noOracles: true, retimeImports: true, ctags: "missing", scip: "missing", tools: []string{"missing"}}, with)
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]int{langbench.CntImports: 4, langbench.CntImportsExternal: 1, langbench.CntImportEdgesFathom: 4} {
		if got := measured.subject.Counts[key]; got != want {
			t.Errorf("no-oracle %s = %d, want %d", key, got, want)
		}
	}
}

// TestCmdLangRetimeTenancyOnly: a reading whose counts were scored on another
// scope than its tenancy block refuses a plain re-time, and -tenancy-only
// re-times it on the tenancy run's own facts and says so in the note.
func TestCmdLangRetimeTenancyOnly(t *testing.T) {
	root, path := retimeFixture(t)
	r := readReading(t, path)
	for i := range r.Subjects {
		r.Subjects[i].Counts[langbench.CntFathomDefs] += 5
	}
	saveRetimeReading(t, path, r)
	var out, errs bytes.Buffer
	if code := mainRun(retimeArgs(root, path), &out, &errs); code != 1 || !strings.Contains(errs.String(), "counts.fathom_defs") {
		t.Fatalf("plain re-time: exit %d, want 1 naming counts.fathom_defs: %s", code, errs.String())
	}
	out.Reset()
	errs.Reset()
	args := append(retimeArgs(root, path), "-tenancy-only", "-write")
	if code := mainRun(args, &out, &errs); code != 0 {
		t.Fatalf("-tenancy-only: exit %d: %s", code, errs.String())
	}
	after := readReading(t, path)
	var note string
	for _, n := range after.Notes {
		if strings.HasPrefix(n, retimeNotePrefix) {
			note = n
		}
	}
	if !strings.Contains(note, "-tenancy-only") || !strings.Contains(note, "files, bytes, faults and parse errors only") {
		t.Fatalf("note does not disclose the tenancy-only identity: %q", note)
	}
	if after.Subjects[0].Counts[langbench.CntFathomDefs] != r.Subjects[0].Counts[langbench.CntFathomDefs] {
		t.Fatalf("-tenancy-only changed a count: %d", after.Subjects[0].Counts[langbench.CntFathomDefs])
	}
}
