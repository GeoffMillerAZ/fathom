package pilots

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func pilotGitEnv(extra ...string) []string {
	local := map[string]bool{
		"GIT_ALTERNATE_OBJECT_DIRECTORIES": true, "GIT_CONFIG": true,
		"GIT_CONFIG_PARAMETERS": true, "GIT_CONFIG_COUNT": true,
		"GIT_OBJECT_DIRECTORY": true, "GIT_DIR": true, "GIT_WORK_TREE": true,
		"GIT_IMPLICIT_WORK_TREE": true, "GIT_GRAFT_FILE": true,
		"GIT_INDEX_FILE": true, "GIT_NO_REPLACE_OBJECTS": true,
		"GIT_REPLACE_REF_BASE": true, "GIT_PREFIX": true,
		"GIT_SHALLOW_FILE": true, "GIT_COMMON_DIR": true,
	}
	var clean []string
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if !local[name] {
			clean = append(clean, entry)
		}
	}
	return append(clean, extra...)
}

func pilotConsumerEnv(consumer string) []string {
	return []string{
		"KIT_PLATFORM_DIR=" + consumer,
		"KIT_PLATFORM_REVIEW_PATH=kits/review-consumer",
		"KIT_PLATFORM_ENV_PATH=kits/env-consumer",
		"KIT_PLATFORM_REVIEW_MODULE=example.invalid/review-consumer",
	}
}

func pilotGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = pilotGitEnv(
		"GIT_AUTHOR_NAME=Pilot", "GIT_AUTHOR_EMAIL=pilot@example.invalid",
		"GIT_COMMITTER_NAME=Pilot", "GIT_COMMITTER_EMAIL=pilot@example.invalid",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func pilotWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestRunnerUsesPinnedObjectWithoutChangingConsumer(t *testing.T) {
	if script, err := os.ReadFile("run.sh"); err != nil || len(script) == 0 {
		t.Fatalf("read runner for test-cache tracking: %v", err)
	}
	consumer := t.TempDir()
	pilotGit(t, consumer, "init", "-q")
	for _, kit := range []string{"review-consumer", "env-consumer"} {
		pilotWrite(t, filepath.Join(consumer, "kits", kit, "go.mod"), "module example.invalid/"+kit+"\n\ngo 1.26\n")
		pilotWrite(t, filepath.Join(consumer, "kits", kit, "internal", map[string]string{"review-consumer": "standard", "env-consumer": "catalog"}[kit], "placeholder.go"), "package placeholder\n")
	}
	pilotGit(t, consumer, "add", ".")
	pilotGit(t, consumer, "commit", "-qm", "pinned consumer fixture")
	pin := pilotGit(t, consumer, "rev-parse", "HEAD")
	pilotWrite(t, filepath.Join(consumer, "advanced.txt"), "advanced HEAD only\n")
	pilotGit(t, consumer, "add", "advanced.txt")
	pilotGit(t, consumer, "commit", "-qm", "advance consumer")
	advanced := pilotGit(t, consumer, "rev-parse", "HEAD")
	sentinel := filepath.Join(consumer, "dirty-sentinel")
	pilotWrite(t, sentinel, "preserve me")
	before := pilotGit(t, consumer, "status", "--porcelain")

	stubDir := t.TempDir()
	goLog := filepath.Join(t.TempDir(), "go.log")
	pilotWrite(t, filepath.Join(stubDir, "go"), "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$GO_STUB_LOG\"\n")
	if err := os.Chmod(filepath.Join(stubDir, "go"), 0700); err != nil {
		t.Fatal(err)
	}
	scratchRoot := t.TempDir()
	runnerEnv := append(append(pilotGitEnv(), pilotConsumerEnv(consumer)...),
		"CONSUMER_REVISION="+pin,
		"PILOT_SCRATCH_ROOT="+scratchRoot, "PATH="+stubDir+":"+os.Getenv("PATH"),
		"GO_STUB_LOG="+goLog,
		// These must not redirect git -C to another repository or object store.
		"GIT_DIR="+t.TempDir(), "GIT_WORK_TREE="+t.TempDir(),
		"GIT_INDEX_FILE="+filepath.Join(t.TempDir(), "index"),
		"GIT_OBJECT_DIRECTORY="+t.TempDir(),
	)
	runner := exec.Command("bash", "run.sh")
	runner.Env = runnerEnv
	out, err := runner.CombinedOutput()
	if err != nil {
		t.Fatalf("advanced dirty replay: %v: %s", err, out)
	}
	if !strings.Contains(string(out), "revision="+pin) {
		t.Fatalf("missing pin receipt: %s", out)
	}
	if got := pilotGit(t, consumer, "rev-parse", "HEAD"); got != advanced {
		t.Fatalf("HEAD changed: %s", got)
	}
	if got := pilotGit(t, consumer, "status", "--porcelain"); got != before {
		t.Fatalf("consumer changed: %q -> %q", before, got)
	}
	if data, err := os.ReadFile(sentinel); err != nil || string(data) != "preserve me" {
		t.Fatalf("sentinel changed: %q %v", data, err)
	}
	entries, err := os.ReadDir(scratchRoot)
	if err != nil || len(entries) != 1 {
		t.Fatalf("scratch entries: %d %v", len(entries), err)
	}
	snapshot := filepath.Join(scratchRoot, entries[0].Name())
	for _, path := range []string{"kits/review-consumer/internal/standard/catalog_kit_pilot_test.go", "kits/env-consumer/internal/catalog/catalog_kit_pilot_test.go"} {
		if _, err := os.Stat(filepath.Join(snapshot, path)); err != nil {
			t.Fatalf("missing injected test %s: %v", path, err)
		}
	}
	for _, path := range []string{"advanced.txt", "dirty-sentinel"} {
		if _, err := os.Stat(filepath.Join(snapshot, path)); !os.IsNotExist(err) {
			t.Fatalf("snapshot contains unpinned %s: %v", path, err)
		}
	}
	if data, err := os.ReadFile(goLog); err != nil || !strings.Contains(string(data), "work init") || strings.Count(string(data), "test -race") != 2 {
		t.Fatalf("go boundary not exercised: %q %v", data, err)
	}

	for name, revision := range map[string]string{"missing": strings.Repeat("0", 40), "symbolic": "HEAD", "short": pin[:12]} {
		t.Run(name, func(t *testing.T) {
			refusalScratch := t.TempDir()
			cmd := exec.Command("bash", "run.sh")
			cmd.Env = append(append(pilotGitEnv(), pilotConsumerEnv(consumer)...), "CONSUMER_REVISION="+revision, "PILOT_SCRATCH_ROOT="+refusalScratch)
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("invalid revision accepted: %s", out)
			}
			if entries, readErr := os.ReadDir(refusalScratch); readErr != nil || len(entries) != 0 {
				t.Fatalf("refusal wrote snapshot: %v %v", entries, readErr)
			}
		})
	}
	kitRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	kitAlias := filepath.Join(t.TempDir(), "kit-alias")
	consumerAlias := filepath.Join(t.TempDir(), "consumer-alias")
	for alias, target := range map[string]string{kitAlias: kitRoot, consumerAlias: consumer} {
		if err := os.Symlink(target, alias); err != nil {
			t.Fatal(err)
		}
	}
	for name, scratch := range map[string]string{
		"kit direct": kitRoot, "kit alias": kitAlias,
		"consumer direct": consumer, "consumer alias": consumerAlias,
		"kit missing child":      filepath.Join(kitRoot, "nonexistent-pilot-scratch-child"),
		"consumer missing child": filepath.Join(consumer, "nonexistent-pilot-scratch-child"),
	} {
		t.Run(name, func(t *testing.T) {
			physical := kitRoot
			if strings.Contains(name, "consumer") {
				physical = consumer
			}
			beforeSnapshots, err := filepath.Glob(filepath.Join(physical, "agentic-kit-catalog-pilots-*"))
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("bash", "run.sh")
			cmd.Env = append(append(pilotGitEnv(), pilotConsumerEnv(consumer)...), "CONSUMER_REVISION="+pin, "PILOT_SCRATCH_ROOT="+scratch)
			out, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(out), "scratch parent") {
				t.Fatalf("unsafe scratch accepted: %v %s", err, out)
			}
			afterSnapshots, err := filepath.Glob(filepath.Join(physical, "agentic-kit-catalog-pilots-*"))
			if err != nil || !reflect.DeepEqual(beforeSnapshots, afterSnapshots) {
				t.Fatalf("checkout gained snapshot: %v -> %v (%v)", beforeSnapshots, afterSnapshots, err)
			}
			if strings.Contains(name, "missing child") {
				if _, err := os.Stat(scratch); !os.IsNotExist(err) {
					t.Fatalf("nonexistent child was created: %v", err)
				}
			}
			if got := pilotGit(t, consumer, "status", "--porcelain"); got != before {
				t.Fatalf("consumer changed: %q -> %q", before, got)
			}
		})
	}
}

func TestRunnerSkipsWhenPlatformConfigUnset(t *testing.T) {
	for name, env := range map[string][]string{
		"no dir":      {"KIT_PLATFORM_DIR="},
		"missing dir": {"KIT_PLATFORM_DIR=" + filepath.Join(t.TempDir(), "absent")},
		"no paths":    {"KIT_PLATFORM_DIR=" + t.TempDir()},
	} {
		t.Run(name, func(t *testing.T) {
			cmd := exec.Command("bash", "run.sh")
			cmd.Env = append(pilotGitEnv(), env...)
			out, err := cmd.CombinedOutput()
			if err != nil || !strings.Contains(string(out), "skipping") {
				t.Fatalf("want clean skip, got %v: %s", err, out)
			}
		})
	}
}
