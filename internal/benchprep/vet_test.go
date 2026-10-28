package benchprep

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const mitText = "MIT License\n\nPermission is hereby granted, free of charge, to any person obtaining a copy\n"

// vetFixture commits a tiny Go subject into subjects/<name> and returns the
// subjects directory and the commit sha.
func vetFixture(t *testing.T, name, licence string, goLines int) (string, string) {
	t.Helper()
	subjects := t.TempDir()
	clone := filepath.Join(subjects, name)
	if err := os.MkdirAll(filepath.Join(clone, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	var body strings.Builder
	body.WriteString("package pkg\n")
	for i := 1; i < goLines; i++ {
		fmt.Fprintf(&body, "// line %d\n", i)
	}
	files := map[string]string{
		"LICENSE":       licence,
		"pkg/pkg.go":    body.String(),
		"pkg/a_test.go": "package pkg", // unterminated final line still counts
		"README.md":     "not counted\n",
	}
	for path, text := range files {
		if err := os.WriteFile(filepath.Join(clone, path), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-C", clone, "-c", "user.name=fixture", "-c", "user.email=fixture@example.org", "-c", "commit.gpgsign=false"}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	git("add", ".")
	git("commit", "-q", "-m", "fixture")
	// A later commit must not leak into readings at the pinned sha.
	if err := os.WriteFile(filepath.Join(clone, "pkg", "late.go"), []byte(strings.Repeat("x\n", 200000)), 0o644); err != nil {
		t.Fatal(err)
	}
	sha := git("rev-parse", "HEAD")
	git("add", ".")
	git("commit", "-q", "-m", "after the pin")
	return subjects, sha
}

func vetSubjectFixture(name, sha string) Subject {
	var s Subject
	s.Name, s.URL, s.Ref, s.SHA, s.Licence = name, "https://example.org/sample/"+name, "main", sha, "MIT"
	s.Bench.Toolchain.Versions = map[string]string{"go": "1.26.2"}
	s.Bench.Test = []string{"go test ./...", "go vet ./..."}
	s.Bench.FlakeRuns = 3
	s.Bench.L0Reading = "unmeasured: C4 awaits controller L0 runs"
	s.Bench.Contamination.FreshAfter = "2026-01-01"
	return s
}

func fakeSuite(seconds float64, exit int, network string, seen *[]SuiteRequest) SuiteRunner {
	return func(_ context.Context, req SuiteRequest) (SuiteRun, error) {
		if seen != nil {
			*seen = append(*seen, req)
		}
		return SuiteRun{Seconds: seconds, ExitCode: exit, Network: network}, nil
	}
}

func fakeMiner(total, fresh int) func(context.Context, string) ([]FixCommit, error) {
	return func(context.Context, string) ([]FixCommit, error) {
		var out []FixCommit
		for i := 0; i < total; i++ {
			created := "2025-06-01T12:00:00Z"
			if i < fresh {
				created = "2026-01-02T00:00:00+00:00"
			}
			out = append(out, FixCommit{Commit: fmt.Sprintf("%040d", i), CreatedAt: created})
		}
		return out, nil
	}
}

func passingRows() []C6Row {
	var rows []C6Row
	for _, symbol := range []string{"pkg.C", "pkg.A", "pkg.B"} {
		rows = append(rows, C6Row{Symbol: symbol, DefiningUnit: "pkg", Calls: 5, Units: []string{"a", "b", "c"}, Qualifies: true})
	}
	return append(rows, C6Row{Symbol: "pkg.Self", DefiningUnit: "pkg", Calls: 9, Units: []string{"pkg", "a", "b"}})
}

func passingDeps(subjects string) VetDeps {
	return VetDeps{
		SubjectsDir: subjects,
		Suite:       fakeSuite(30, 0, "none", nil),
		Miner:       fakeMiner(30, 15),
		C6Rows:      func(string) ([]C6Row, error) { return passingRows(), nil },
		SelfURL:     "git@example.org:Owner/Self.git",
		CorpusURLs:  []string{"https://example.org/corpus/indexed"},
	}
}

func criteria(r VetReceipt) map[string]VetCriterion {
	out := map[string]VetCriterion{}
	for _, c := range r.Criteria {
		out[c.ID] = c
	}
	return out
}

func TestVetSubjectPendingOnlyOnC4(t *testing.T) {
	subjects, sha := vetFixture(t, "sample", mitText, 5000)
	s := vetSubjectFixture("sample", sha)
	var seen []SuiteRequest
	deps := passingDeps(subjects)
	deps.Suite = fakeSuite(30, 0, "none", &seen)
	r, err := VetSubject(context.Background(), s, deps)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, c := range r.Criteria {
		ids = append(ids, c.ID)
	}
	if strings.Join(ids, ",") != "C1,C2,C3,C4,C5,C6,C7" {
		t.Fatalf("criteria order: %v", ids)
	}
	for id, c := range criteria(r) {
		want := VetPass
		if id == "C4" {
			want = VetPending
		}
		if c.Status != want {
			t.Errorf("%s = %s (%s), want %s", id, c.Status, c.Evidence, want)
		}
	}
	if r.Verdict != VerdictPending {
		t.Fatalf("verdict %s, want pending while C4 is pending", r.Verdict)
	}
	c := criteria(r)
	if c["C4"].Evidence != s.Bench.L0Reading {
		t.Errorf("C4 evidence %q", c["C4"].Evidence)
	}
	if !strings.Contains(c["C2"].Evidence, "5001 go lines in 2 tracked files") {
		t.Errorf("C2 evidence %q", c["C2"].Evidence)
	}
	if !strings.Contains(c["C1"].Evidence, "LICENSE reads MIT") {
		t.Errorf("C1 evidence %q", c["C1"].Evidence)
	}
	if !strings.Contains(c["C6"].Evidence, "3 of 4") || !strings.Contains(c["C6"].Evidence, "pkg.A, pkg.B, pkg.C") {
		t.Errorf("C6 evidence %q", c["C6"].Evidence)
	}
	if !strings.Contains(c["C7"].Evidence, "fleet check: unchecked") {
		t.Errorf("C7 evidence %q", c["C7"].Evidence)
	}
	if len(seen) != 4 || seen[0].Stage != "warmup" || seen[3].Stage != "run3" || seen[0].Command != "go test ./... && go vet ./..." || seen[0].Language != "go" {
		t.Fatalf("suite requests: %+v", seen)
	}
	if len(r.SuiteRuns) != 4 {
		t.Fatalf("suite runs: %+v", r.SuiteRuns)
	}
	again, err := VetSubject(context.Background(), s, deps)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(r)
	b, _ := json.Marshal(again)
	if string(a) != string(b) {
		t.Fatalf("receipt is not deterministic:\n%s\n%s", a, b)
	}
}

func TestVetSubjectFailuresNameEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, id, want string
		licence        string
		lines          int
		edit           func(*Subject, *VetDeps)
	}{
		{name: "copyleft", id: "C1", want: "no C1 licence recognised: LICENSE: unrecognised", licence: "GNU GENERAL PUBLIC LICENSE\nVersion 3\n"},
		{name: "licence mismatch", id: "C1", want: "LICENSE reads MIT but the manifest declares Apache-2.0", edit: func(s *Subject, _ *VetDeps) { s.Licence = "Apache-2.0" }},
		{name: "too small", id: "C2", want: "outside 5,000 to 150,000", lines: 100},
		{name: "red run", id: "C3", want: "run2 exited 1", edit: func(_ *Subject, d *VetDeps) {
			d.Suite = func(_ context.Context, req SuiteRequest) (SuiteRun, error) {
				if req.Stage == "run2" {
					return SuiteRun{Seconds: 1, ExitCode: 1, Network: "none"}, nil
				}
				return SuiteRun{Seconds: 1, Network: "none"}, nil
			}
		}},
		{name: "networked", id: "C3", want: `ran with network "bridge"`, edit: func(_ *Subject, d *VetDeps) { d.Suite = fakeSuite(1, 0, "bridge", nil) }},
		{name: "slow", id: "C3", want: "longest timed run 901.000s exceeds 900s", edit: func(_ *Subject, d *VetDeps) { d.Suite = fakeSuite(901, 0, "none", nil) }},
		{name: "low yield", id: "C5", want: "29 mined fix commits, 29 after 2026-01-01; fewer than 30", edit: func(_ *Subject, d *VetDeps) { d.Miner = fakeMiner(29, 29) }},
		{name: "stale yield", id: "C5", want: "fewer than 15 after the cutoff", edit: func(_ *Subject, d *VetDeps) { d.Miner = fakeMiner(40, 14) }},
		{name: "no cutoff, low yield", id: "C5", want: "pending Q2 (no fresh_after); fewer than 30", edit: func(s *Subject, d *VetDeps) {
			s.Bench.Contamination.FreshAfter = ""
			d.Miner = fakeMiner(10, 0)
		}},
		{name: "thin fan-in", id: "C6", want: "2 of 3 recount symbols", edit: func(_ *Subject, d *VetDeps) {
			d.C6Rows = func(string) ([]C6Row, error) { return passingRows()[1:], nil }
		}},
		{name: "self", id: "C7", want: "is the checkout's own repository", edit: func(s *Subject, d *VetDeps) { d.SelfURL = s.URL + ".git/" }},
		{name: "corpus", id: "C7", want: "is indexed by the calibration corpus", edit: func(s *Subject, d *VetDeps) { d.CorpusURLs = []string{"HTTPS://Example.org/sample/" + s.Name} }},
		{name: "org cap", id: "C7", want: "3 bench subjects share example.org/sample: a, b, sample", edit: func(s *Subject, d *VetDeps) {
			d.Peers = []Subject{*s, {Name: "b", URL: "https://example.org/sample/b"}, {Name: "a", URL: "https://example.org/Sample/a.git"}, {Name: "z", URL: "https://example.org/other/z"}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			licence, lines := tc.licence, tc.lines
			if licence == "" {
				licence = mitText
			}
			if lines == 0 {
				lines = 5000
			}
			subjects, sha := vetFixture(t, "sample", licence, lines)
			s := vetSubjectFixture("sample", sha)
			deps := passingDeps(subjects)
			if tc.edit != nil {
				tc.edit(&s, &deps)
			}
			r, err := VetSubject(context.Background(), s, deps)
			if err != nil {
				t.Fatal(err)
			}
			c := criteria(r)[tc.id]
			if c.Status != VetFail || !strings.Contains(c.Evidence, tc.want) {
				t.Fatalf("%s = %s %q, want fail naming %q", tc.id, c.Status, c.Evidence, tc.want)
			}
			if r.Verdict != VerdictRefused {
				t.Fatalf("verdict %s, want refused", r.Verdict)
			}
		})
	}
}

func TestVetSubjectPendingCriteria(t *testing.T) {
	subjects, sha := vetFixture(t, "sample", mitText, 5000)
	for _, tc := range []struct {
		name, id, want string
		edit           func(*Subject, *VetDeps)
	}{
		{"no cutoff", "C5", "30 mined fix commits; post-cutoff count pending Q2", func(s *Subject, _ *VetDeps) { s.Bench.Contamination.FreshAfter = "" }},
		{"python", "C5", "no miner for python", func(s *Subject, d *VetDeps) {
			s.Bench.Toolchain.Versions = map[string]string{"python": "3.12.0", "uv": "0.9.0"}
			d.Miner = func(context.Context, string) ([]FixCommit, error) {
				t.Fatal("miner called for python")
				return nil, nil
			}
		}},
		{"no recount", "C6", "no C6 recount for sample", func(_ *Subject, d *VetDeps) {
			d.C6Rows = func(string) ([]C6Row, error) { return nil, fs.ErrNotExist }
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := vetSubjectFixture("sample", sha)
			deps := passingDeps(subjects)
			tc.edit(&s, &deps)
			r, err := VetSubject(context.Background(), s, deps)
			if err != nil {
				t.Fatal(err)
			}
			c := criteria(r)[tc.id]
			if c.Status != VetPending || !strings.Contains(c.Evidence, tc.want) {
				t.Fatalf("%s = %s %q, want pending naming %q", tc.id, c.Status, c.Evidence, tc.want)
			}
		})
	}
}

func TestVetSubjectExcludedShare(t *testing.T) {
	subjects, sha := vetFixture(t, "sample", mitText, 5000)
	s := vetSubjectFixture("sample", sha)
	s.Bench.SuiteTests = 200
	s.Bench.Excluded = []ExcludedTest{{Test: "TestNet", Reason: "needs network", Log: "logs/net.log"}}
	r, err := VetSubject(context.Background(), s, passingDeps(subjects))
	if err != nil {
		t.Fatal(err)
	}
	if c := criteria(r)["C3"]; c.Status != VetPass || !strings.Contains(c.Evidence, "excluded 1 of 200 suite tests (0.5%)") {
		t.Fatalf("C3 = %s %q", c.Status, c.Evidence)
	}
}

func TestVetSubjectRefusesMissingCloneOrSHA(t *testing.T) {
	subjects, sha := vetFixture(t, "sample", mitText, 5000)
	if _, err := VetSubject(context.Background(), vetSubjectFixture("absent", sha), passingDeps(subjects)); err == nil || !strings.Contains(err.Error(), "no clone") {
		t.Fatalf("missing clone: %v", err)
	}
	if _, err := VetSubject(context.Background(), vetSubjectFixture("sample", strings.Repeat("0", 40)), passingDeps(subjects)); err == nil || !strings.Contains(err.Error(), "not in the clone") {
		t.Fatalf("missing sha: %v", err)
	}
}

func TestLoadCorpusURLs(t *testing.T) {
	dir := t.TempDir()
	corpus := filepath.Join(dir, "corpus.yaml")
	if err := os.WriteFile(corpus, []byte("languages:\n  - language: Go\n    homepage: {url: https://example.org/not-a-repo}\n    repos:\n      - name: one\n        url: https://Example.org/A/One.git\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "corpus.d"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "corpus.d", "x.yaml"), []byte("id: x\nrepos:\n  - name: two\n    url: https://example.org/b/two/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	urls, err := LoadCorpusURLs(corpus, filepath.Join(dir, "corpus.d"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(urls, " ") != "https://example.org/a/one https://example.org/b/two" {
		t.Fatalf("urls = %v", urls)
	}
}
