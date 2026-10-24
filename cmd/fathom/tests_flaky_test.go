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

	"github.com/geoffmilleraz/fathom/internal/gatereceipt"
)

func TestTestsFlakyTextAndJSON(t *testing.T) {
	source, err := filepath.Abs("../../internal/gatereceipt/testdata/flaky")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("FATHOM_HOME", home)
	t.Chdir(root)
	// Generate a repository; receipt configuration uses the existing resolver.
	gitFixture := func(args ...string) { t.Helper(); runGitForFlaky(t, root, args...) }
	gitFixture("init", "-q")
	dest := filepath.Join(root, "workspace/logs")
	if err := os.MkdirAll(dest, 0700); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(source, "workspace/logs"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(source, "workspace/logs", e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dest, e.Name()), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	receipts, err := os.ReadDir(filepath.Join(source, "receipts"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, "receipts"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, e := range receipts {
		b, err := os.ReadFile(filepath.Join(source, "receipts", e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, "receipts", e.Name()), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	var out, stderr bytes.Buffer
	if code := runTests([]string{"flaky"}, &out, &stderr); code != 0 {
		t.Fatalf("code %d: %s", code, &stderr)
	}
	text := out.String()
	for _, s := range []string{"fixture/alpha", "TestSlow", "TestChanged", "30s", "unknown identity: 2"} {
		if !strings.Contains(text, s) {
			t.Fatalf("missing %q in %s", s, text)
		}
	}
	if strings.Index(text, "TestTieA") > strings.Index(text, "TestTieZ") {
		t.Fatal("unstable tie ordering")
	}
	out.Reset()
	stderr.Reset()
	if code := runTests([]string{"flaky", "--json"}, &out, &stderr); code != 0 {
		t.Fatalf("code %d: %s", code, &stderr)
	}
	var report gatereceipt.FlakyReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Tests) != 5 || report.Tests[0].Lost != 30*time.Second || report.UnknownIdentity != 2 {
		t.Fatalf("JSON: %s", &out)
	}
	for _, args := range [][]string{{"--since", "bad"}, {"--since", "-1s"}, {"unexpected"}, {"--unknown"}} {
		out.Reset()
		stderr.Reset()
		if code := runTests(append([]string{"flaky"}, args...), &out, &stderr); code == 0 || stderr.Len() == 0 {
			t.Fatalf("accepted %v", args)
		}
	}
	out.Reset()
	stderr.Reset()
	if code := runTestsFlaky([]string{"--since", "1h", "--json"}, func() time.Time { return time.Date(2026, 10, 22, 6, 0, 0, 0, time.UTC) }, &out, &stderr); code != 0 {
		t.Fatalf("since: %d %s", code, &stderr)
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Tests) != 0 || report.UnknownIdentity != 0 {
		t.Fatalf("since: %s", &out)
	}
}

func runGitForFlaky(t *testing.T, root string, args ...string) {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = root
	c.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	if b, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git: %s %v", b, err)
	}
}

func TestTestsFlakyHelpAndUsage(t *testing.T) {
	var out, stderr bytes.Buffer
	if code := runVerbHelp(&out, findFlakyTestsVerb(t), "flaky"); code != 0 {
		t.Fatalf("help code %d", code)
	}
	if !strings.Contains(out.String(), "undated") {
		t.Fatalf("help omits undated cutoff: %s", &out)
	}
	if code := runTests(nil, &out, &stderr); code != 1 || !strings.Contains(stderr.String(), "tests flaky") {
		t.Fatalf("usage: %d %s", code, &stderr)
	}
}

func findFlakyTestsVerb(t *testing.T) verb {
	t.Helper()
	for _, v := range verbs() {
		if v.Name == "tests" {
			return v
		}
	}
	t.Fatal("tests verb missing")
	return verb{}
}
