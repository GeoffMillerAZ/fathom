package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// TestActionlintResolution pins what actionlint's own caches attest for a
// local uses: the metadata file of a step's directory (action.yaml before
// action.yml when both exist, the reverse of GitHub's runner), nothing for a
// directory holding only a Dockerfile, and a job's reusable workflow only
// when its on: names workflow_call.
func TestActionlintResolution(t *testing.T) {
	meta := "name: x\ndescription: x\nruns:\n  using: composite\n  steps: []\n"
	dir := writeTree(t, map[string]string{
		".github/actions/one/action.yml":    meta,
		".github/actions/both/action.yml":   meta,
		".github/actions/both/action.yaml":  meta,
		".github/actions/docker/Dockerfile": "FROM scratch\n",
		".github/workflows/reuse.yml":       "on:\n  workflow_call:\njobs:\n  a:\n    runs-on: x\n    steps:\n      - run: true\n",
		".github/workflows/plain.yml":       "on: push\njobs:\n  a:\n    runs-on: x\n    steps:\n      - run: true\n",
	})
	r, err := newResolver(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ uses, want string }{
		{"./.github/actions/one", ".github/actions/one/action.yml"},
		{"./.github/actions/both", ".github/actions/both/action.yaml"},
		{"./.github/actions/docker", ""},
		{"./.github/actions/missing", ""},
		{"actions/checkout@v4", ""},
	} {
		if got := r.step(c.uses); got != c.want {
			t.Errorf("step(%q) = %q, want %q", c.uses, got, c.want)
		}
	}
	for _, c := range []struct{ uses, want string }{
		{"./.github/workflows/reuse.yml", ".github/workflows/reuse.yml"},
		{"./.github/workflows/plain.yml", ""},
		{"./.github/workflows/missing.yml", ""},
		{"octo/repo/.github/workflows/x.yml@v1", ""},
	} {
		if got := r.job(c.uses); got != c.want {
			t.Errorf("job(%q) = %q, want %q", c.uses, got, c.want)
		}
	}
	if r.stats.BothMetadata != 1 || r.stats.StepUses != 4 || r.stats.JobUses != 3 || r.stats.JobRejected != 2 {
		t.Errorf("stats %+v", r.stats)
	}
}
