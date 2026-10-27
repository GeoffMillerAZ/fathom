package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRetimeLoadLogPathRefusesUnsafeDestinations(t *testing.T) {
	t.Chdir(filepath.Join("..", ".."))
	workspace := filepath.Join(t.TempDir(), "workspace")
	if err := os.Mkdir(workspace, 0700); err != nil {
		t.Fatal(err)
	}
	inside := filepath.Join(workspace, "loads.json")
	outside := filepath.Join(t.TempDir(), "loads.json")
	if _, err := openRetimeLoadLog(outside, workspace); err == nil {
		t.Fatal("accepted destination outside workspace")
	}
	if err := os.WriteFile(inside, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := openRetimeLoadLog(inside, workspace); err == nil {
		t.Fatal("clobbered existing destination")
	}
	if got, err := os.ReadFile(inside); err != nil || string(got) != "keep" {
		t.Fatalf("existing data changed: %q, %v", got, err)
	}
	link := filepath.Join(workspace, "link")
	if err := os.Symlink(workspace, link); err != nil {
		t.Fatal(err)
	}
	if _, err := openRetimeLoadLog(filepath.Join(link, "loads.json"), workspace); err == nil {
		t.Fatal("accepted symlink parent")
	}
	if _, err := openRetimeLoadLog(filepath.Join(workspace, "missing", "loads.json"), workspace); err == nil {
		t.Fatal("accepted missing parent")
	}
}

func TestRetimeWorkspaceRootUsesCheckoutAndIgnoreRule(t *testing.T) {
	t.Chdir(filepath.Join("..", ".."))
	root, err := os.MkdirTemp("workspace/tmp", "retime-root-")
	if err != nil {
		t.Fatal(err)
	}
	root, err = filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	if err := os.Mkdir(filepath.Join(root, "workspace"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	if _, err := retimeWorkspaceRoot(); err == nil {
		t.Fatal("accepted a checkout whose workspace is not git-ignored")
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("workspace/\n"), 0600); err != nil {
		t.Fatal(err)
	}
	workspace, err := retimeWorkspaceRoot()
	if err != nil || workspace != filepath.Join(root, "workspace") {
		t.Fatalf("workspace = %q, %v", workspace, err)
	}
}

func TestRetimeBoundedReadingRejectsOversize(t *testing.T) {
	t.Chdir(filepath.Join("..", ".."))
	dir, err := os.MkdirTemp("workspace", "retime-reading-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	path := filepath.Join(dir, "reading.json")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(retimeReadingCap + 1); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := retimeBoundedReading(path); err == nil {
		t.Fatal("accepted reading above size bound")
	}
}

func TestRetimeLoadObserverValues(t *testing.T) {
	reads := 0
	read := func() (float64, error) {
		reads++
		if reads == 1 {
			return 7.25, nil
		}
		return 8.5, nil
	}
	observer := retimeLoadObserver{read: read}
	if err := observer.before(); err != nil {
		t.Fatal(err)
	}
	observed, err := observer.after(2, 1.23)
	if err != nil {
		t.Fatal(err)
	}
	want := retimeLoadRun{Number: 2, RateMBCPUS: 1.23, LoadBefore: 7.25, LoadAfter: 8.5}
	if observed != want {
		t.Fatalf("observed %+v, want %+v", observed, want)
	}
	if _, err := json.Marshal(observed); err != nil {
		t.Fatal(err)
	}
}

func TestCmdLangRetimeLoadLogBindsReadingAndProduct(t *testing.T) {
	root, reading := retimeFixture(t)
	stubFileClocks(t, time.Millisecond, time.Millisecond, true)
	var steps []string
	clock := fileClocks.cpu
	fileClocks.cpu = func() time.Duration {
		steps = append(steps, "clock")
		return clock()
	}
	dir, err := os.MkdirTemp("workspace", "retime-load-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	logPath := filepath.Join(dir, "load.json")
	before, err := os.ReadFile(reading)
	if err != nil {
		t.Fatal(err)
	}
	oldLoad, oldProduct := retimeLoadReader, retimeProductIdentityReader
	t.Cleanup(func() { retimeLoadReader, retimeProductIdentityReader = oldLoad, oldProduct })
	reads := 0
	retimeLoadReader = func() (float64, error) {
		reads++
		steps = append(steps, "load")
		return float64(reads), nil
	}
	retimeProductIdentityReader = func() (retimeProduct, error) {
		return retimeProduct{revision: "clean-revision", binarySHA256: "binary-hash"}, nil
	}
	var out, errs bytes.Buffer
	if code := mainRun(append(retimeArgs(root, reading), "-write", "-load-log", logPath), &out, &errs); code != 0 {
		t.Fatalf("exit %d: %s", code, errs.String())
	}
	if reads != 6 {
		t.Fatalf("load reads = %d, want 6", reads)
	}
	segments := strings.Split(strings.Join(steps, ","), "load")
	if len(segments) != 7 {
		t.Fatalf("load/clock order: %v", steps)
	}
	for run := 0; run < 3; run++ {
		if !strings.Contains(segments[2*run+1], "clock") {
			t.Fatalf("run %d has no measured clock between load reads: %v", run+1, steps)
		}
	}
	after, err := os.ReadFile(reading)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Schema   string          `json:"schema"`
		Before   string          `json:"reading_before_sha256"`
		After    string          `json:"reading_after_sha256"`
		Revision string          `json:"product_revision"`
		Binary   string          `json:"product_binary_sha256"`
		Modified bool            `json:"product_modified"`
		Runs     []retimeLoadRun `json:"runs"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Schema != "fathom-lang-retime-load-v1" || got.Before != retimeSHA(before) || got.After != retimeSHA(after) || got.Revision != "clean-revision" || got.Binary != "binary-hash" || got.Modified {
		t.Fatalf("log identity mismatch: %+v", got)
	}
	if len(got.Runs) != 3 {
		t.Fatalf("runs = %+v", got.Runs)
	}
	for i, run := range got.Runs {
		if run.Number != i+1 || run.LoadBefore != float64(2*i+1) || run.LoadAfter != float64(2*i+2) || run.RateMBCPUS <= 0 {
			t.Fatalf("run %d: %+v", i+1, run)
		}
	}
}

func TestRetimeBoundedBinarySHARejectsOversize(t *testing.T) {
	t.Chdir(filepath.Join("..", ".."))
	dir, err := os.MkdirTemp("workspace", "retime-binary-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	path := filepath.Join(dir, "large")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(128 << 20); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := retimeBoundedBinarySHA(path); err != nil {
		t.Fatalf("exact cap refused: %v", err)
	}
	f, err = os.OpenFile(path, os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate((128 << 20) + 1); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := retimeBoundedBinarySHA(path); err == nil {
		t.Fatal("accepted executable above size bound")
	}
}
