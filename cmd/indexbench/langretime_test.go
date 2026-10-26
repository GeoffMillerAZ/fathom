package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

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
		}, "bytes", 1},
		{"all measured files removed", func(t *testing.T, root, path string) {
			for _, name := range []string{"a.lua", "b.lua"} {
				if err := os.Remove(filepath.Join(root, name)); err != nil {
					t.Fatal(err)
				}
			}
		}, "files", 1},
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
	for _, want := range []string{"run 2 first", "bytes", "full re-measure"} {
		if !strings.Contains(errs.String(), want) {
			t.Fatalf("missing %q: %s", want, &errs)
		}
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, original) {
		t.Fatal("late refusal wrote reading")
	}
}
