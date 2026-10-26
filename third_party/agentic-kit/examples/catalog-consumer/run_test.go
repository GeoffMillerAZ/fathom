package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestExternalModuleReplay(t *testing.T) {
	// Explicitly read non-Go inputs so changes invalidate Go's test cache.
	for _, path := range []string{"run.sh", "fixtures/consumer.cue", "fixtures/definition.txt", "fixtures/changed-definition.txt", "fixtures/evidence.txt"} {
		if _, err := os.ReadFile(path); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("sh", "run.sh")
	cmd.Env = append(cmd.Environ(), "TMPDIR="+t.TempDir(),
		"GIT_DIR=/nonexistent/catalog-consumer-git-dir",
		"GIT_WORK_TREE=/nonexistent/catalog-consumer-work-tree",
		"GIT_COMMON_DIR=/nonexistent/catalog-consumer-common-dir",
		"GIT_INDEX_FILE=/nonexistent/catalog-consumer-index",
		"GIT_OBJECT_DIRECTORY=/nonexistent/catalog-consumer-objects",
		"GIT_ALTERNATE_OBJECT_DIRECTORIES=/nonexistent/catalog-consumer-alternates",
		"GIT_NAMESPACE=foreign",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("external replay: %v: %s", err, out)
	}
	var result struct {
		Order        []string `json:"order"`
		StableKey    string   `json:"stable_key"`
		PacketDigest string   `json:"packet_digest"`
		ReuseKey     string   `json:"reuse_key"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("decode replay: %v: %s", err, out)
	}
	if strings.Join(result.Order, ",") != "base,project" || result.StableKey != "review.evidence" || len(result.PacketDigest) != 64 || len(result.ReuseKey) != 64 {
		t.Fatalf("unexpected external replay: %+v", result)
	}
}

func TestRunnerRefusesInCheckoutScratchWithoutWriting(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "kit")
	here := filepath.Join(root, "examples", "catalog-consumer")
	if err := os.MkdirAll(here, 0o755); err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile("run.sh")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(here, "run.sh"), script, 0o755); err != nil {
		t.Fatal(err)
	}
	scratch := filepath.Join(root, "scratch")
	if err := os.Mkdir(scratch, 0o755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(base, "scratch-alias")
	if err := os.Symlink(scratch, alias); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(root, "missing")
	for _, tc := range []struct {
		name, tmpdir, diagnostic string
	}{
		{"direct", scratch, "TMPDIR must be outside"},
		{"symlink", alias, "TMPDIR must be outside"},
		{"missing", missing, "TMPDIR must name an existing directory"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command("sh", filepath.Join(here, "run.sh"))
			cmd.Env = append(cmd.Environ(), "TMPDIR="+tc.tmpdir)
			out, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(out), tc.diagnostic) {
				t.Fatalf("wanted scratch refusal, got %v: %s", err, out)
			}
			entries, err := os.ReadDir(scratch)
			if err != nil || len(entries) != 0 {
				t.Fatalf("scratch changed: %v, %v", entries, err)
			}
			if _, err := os.Stat(missing); !os.IsNotExist(err) {
				t.Fatalf("missing path was created or stat failed: %v", err)
			}
		})
	}
}
