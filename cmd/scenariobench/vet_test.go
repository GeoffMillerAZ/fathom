package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/benchprep"
)

func TestDockerSuiteArgv(t *testing.T) {
	root := t.TempDir()
	d := dockerSuite{WorkRoot: filepath.Join(root, "work"), BenchInputs: filepath.Join(root, "bench.d"), UID: 1000, GID: 1001}
	req := benchprep.SuiteRequest{Subject: "sample", Clone: filepath.Join(root, "subjects", "sample"), Language: "python", Stage: "run1", Command: "cd /work/source && pytest -q"}
	argv, err := d.argv(req)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"docker", "run", "--rm", "--network", "none", "--cpus", "4", "--user", "1000:1001",
		"-v", filepath.Join(root, "subjects", "sample") + ":/subject:ro", "-v", filepath.Join(root, "work", "sample") + ":/work",
		"-v", filepath.Join(root, "bench.d") + ":/bench-inputs:ro", "-w", "/subject", "python:3.12-bookworm", "sh", "-c", "cd /work/source && pytest -q"}
	if strings.Join(argv, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("argv:\n%q\nwant\n%q", argv, want)
	}
	d.UID = 0
	if _, err := d.argv(req); err == nil || !strings.Contains(err.Error(), "uid 0") {
		t.Fatalf("uid 0 accepted: %v", err)
	}
}

func TestCmdVetWritesReceipts(t *testing.T) {
	root := t.TempDir()
	clone := filepath.Join(root, "subjects", "sample-subject")
	if err := os.MkdirAll(clone, 0o755); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		out, err := exec.Command("git", append([]string{"-C", clone, "-c", "user.name=fixture", "-c", "user.email=fixture@example.org", "-c", "commit.gpgsign=false"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	if err := os.WriteFile(filepath.Join(clone, "LICENSE"), []byte("Permission is hereby granted, free of charge, to any person\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(clone, "mod.py"), []byte(strings.Repeat("x = 1\n", 6000)), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-q", "-m", "fixture")
	sha := git("rev-parse", "HEAD")

	manifests := filepath.Join(root, "bench.d")
	corpus := filepath.Join(root, "calibration")
	c6 := filepath.Join(root, "c6")
	for _, dir := range []string{manifests, filepath.Join(corpus, "corpus.d"), c6} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	manifest := `# fixture
{"name": "sample-subject", "url": "https://example.org/sample/subject", "ref": "main", "sha": "` + sha + `", "licence": "MIT",
 "bench": {"toolchain": {"versions": {"python": "3.12.0"}, "lockfile_sha256": "` + strings.Repeat("a", 64) + `"},
  "setup": [], "test": ["cd /work/source && pytest -q"], "suite_seconds": 1, "flake_runs": 3, "l0_reading": "unmeasured",
  "task_sources": [{"dataset": "fixture", "revision": "v1", "licence": "MIT"}],
  "contamination": {"age": 10, "popularity_band": "low"}}}
`
	files := map[string]string{
		filepath.Join(manifests, "sample-subject.yaml"): manifest,
		filepath.Join(corpus, "corpus.yaml"):            "languages: []\n",
		filepath.Join(c6, "sample-subject.json"):        `[{"symbol": "a", "defining_unit": "u", "calls": 5, "units": ["x", "y", "z"], "qualifies": true}]`,
	}
	for path, text := range files {
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var seen []benchprep.SuiteRequest
	restore := newVetSuite
	t.Cleanup(func() { newVetSuite = restore })
	newVetSuite = func(dockerSuite) benchprep.SuiteRunner {
		return func(_ context.Context, req benchprep.SuiteRequest) (benchprep.SuiteRun, error) {
			seen = append(seen, req)
			return benchprep.SuiteRun{Seconds: 2, Network: "none"}, nil
		}
	}
	out := filepath.Join(root, "vet")
	args := []string{"vet", "-manifests", manifests, "-corpus-dir", corpus, "-subjects-dir", filepath.Join(root, "subjects"), "-c6", c6, "-self-url", "https://example.org/self/repo", "-out", out}
	stdout := captureStdout(t, func() {
		if rc := run(args); rc != 0 {
			t.Errorf("exit=%d", rc)
		}
	})
	if stdout != "sample-subject: refused\n" {
		t.Fatalf("stdout %q", stdout)
	}
	if len(seen) != 4 {
		t.Fatalf("suite requests %d, want warm-up plus 3", len(seen))
	}
	raw, err := os.ReadFile(filepath.Join(out, "sample-subject.json"))
	if err != nil {
		t.Fatal(err)
	}
	var receipt benchprep.VetReceipt
	if err := json.Unmarshal(raw, &receipt); err != nil {
		t.Fatal(err)
	}
	status := map[string]string{}
	for _, c := range receipt.Criteria {
		status[c.ID] = c.Status
	}
	want := map[string]string{"C1": "pass", "C2": "pass", "C3": "pass", "C4": "pending", "C5": "pending", "C6": "fail", "C7": "pass"}
	for id, s := range want {
		if status[id] != s {
			t.Errorf("%s = %s, want %s (%s)", id, status[id], s, raw)
		}
	}
	if rc := run([]string{"vet", "-manifests", manifests, "-corpus-dir", corpus, "-subjects-dir", filepath.Join(root, "subjects"), "-self-url", "x", "-out", out, "absent"}); rc != 2 {
		t.Fatalf("unknown subject exit=%d, want 2", rc)
	}
}
