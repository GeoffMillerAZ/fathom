package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func offlineModel(_ context.Context, p policy, _, _ string, _ snapshot, _ modelOptions) modelResult {
	return modelResult{Provider: p.Model.Provider, Cache: "none", Sites: []siteResult{}}
}

func invoke(t *testing.T, dir, input string, model modelEvaluator, args ...string) (int, string, string) {
	t.Helper()
	var out, errout bytes.Buffer
	args = append([]string{args[0], "--repo", dir}, args[1:]...)
	rc := run(args, strings.NewReader(input), &out, &errout, model)
	return rc, out.String(), errout.String()
}

func committedPolicy(t *testing.T) (string, string) {
	t.Helper()
	dir := fixtureRepo(t)
	if err := scaffold(filepath.Join(dir, ".quality-gate")); err != nil {
		t.Fatal(err)
	}
	fixtureWrite(t, dir, "data.json", "{}")
	return dir, fixtureCommit(t, dir)
}

func TestCLIStagedTrustedPolicyAndObservation(t *testing.T) {
	dir, _ := committedPolicy(t)
	fixtureWrite(t, dir, "data.json", "{broken")
	fixtureGit(t, dir, nil, "add", "data.json")
	fixtureWrite(t, dir, "data.json", "{}")                 // Working copy cannot conceal the index violation.
	fixtureWrite(t, dir, ".quality-gate/policy.json", "{}") // Working policy cannot weaken HEAD policy.
	rc, out, stderr := invoke(t, dir, "", offlineModel, "check", "--boundary", "pre-commit", "--staged", "--format", "json")
	if rc != 1 || stderr != "" {
		t.Fatalf("rc=%d out=%s err=%s", rc, out, stderr)
	}
	var r report
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatal(err)
	}
	if r.State != "hold" || r.PolicyCommit == "" || len(r.Findings) != 1 || r.Findings[0].RuleID != "json-valid" || strings.Contains(out, "{broken") {
		t.Fatalf("wrong report: %s", out)
	}
	rc, out, _ = invoke(t, dir, "", offlineModel, "check", "--boundary", "pre-commit", "--staged", "--observe")
	if rc != 0 || !strings.Contains(out, "observed") || !strings.Contains(out, "would be 1") {
		t.Fatalf("observation: %d %s", rc, out)
	}
	rc, _, _ = invoke(t, dir, "", offlineModel, "check", "--boundary", "pre-commit", "--staged", "--policy-file", filepath.Join(dir, ".quality-gate/policy.json"))
	if rc != 2 {
		t.Fatal("mutable policy accepted for enforcement")
	}
}

func TestCLIRefusesUnknownAndNeverCallsModelAfterHardFailure(t *testing.T) {
	dir, _ := committedPolicy(t)
	fixtureWrite(t, dir, "data.json", "bad")
	fixtureGit(t, dir, nil, "add", "data.json")
	calls := 0
	model := func(context.Context, policy, string, string, snapshot, modelOptions) modelResult {
		calls++
		return modelResult{}
	}
	rc, _, _ := invoke(t, dir, "", model, "check", "--boundary", "pre-commit", "--staged")
	if rc != 1 || calls != 0 {
		t.Fatal("hard failure dispatched model", rc, calls)
	}
	for _, args := range [][]string{
		{"check", "--boundary", "ci", "--base", "missing", "--revision", "HEAD"},
		{"check", "--boundary", "oops", "--staged"},
		{"check", "--boundary", "ci"},
		{"check", "--boundary", "ci", "--staged", "--revision", "HEAD"},
		{"check", "--boundary", "pre-commit", "--staged", "--policy-ref", "missing"},
		{"pre-push"},
		{"check", "--boundary", "ci", "--revision", "HEAD", "--format", "oops"},
		{"unknown"},
	} {
		rc, _, _ := invoke(t, dir, "", offlineModel, args...)
		if rc != 2 {
			t.Errorf("%v returned %d", args, rc)
		}
	}
}

func TestCLIDriftUnknownAndModelHold(t *testing.T) {
	for _, mode := range []string{"index", "policy", "hold", "unknown"} {
		t.Run(mode, func(t *testing.T) {
			dir, _ := committedPolicy(t)
			fixtureWrite(t, dir, "data.json", "{\"changed\":true}")
			fixtureGit(t, dir, nil, "add", "data.json")
			model := func(context.Context, policy, string, string, snapshot, modelOptions) modelResult {
				switch mode {
				case "index":
					fixtureWrite(t, dir, "data.json", "{}")
					fixtureGit(t, dir, nil, "add", "data.json")
				case "policy":
					fixtureGit(t, dir, nil, "commit", "-qm", "move HEAD")
				case "hold", "unknown":
					outcome := "concern"
					if mode == "unknown" {
						outcome = "unknown"
					}
					return modelResult{Sites: []siteResult{{ID: "test", Mode: "hold", Held: true, Outcome: outcome}}}
				}
				return modelResult{}
			}
			rc, out, errout := invoke(t, dir, "", model, "check", "--boundary", "pre-commit", "--staged")
			want := 2
			if mode == "hold" {
				want = 1
			}
			if rc != want {
				t.Fatalf("rc=%d want=%d out=%s err=%s", rc, want, out, errout)
			}
		})
	}
}

func TestCLIPushAllRefsNewRefsDeletionAndMissingBase(t *testing.T) {
	dir, base := committedPolicy(t)
	fixtureWrite(t, dir, "data.json", "not json")
	bad := fixtureCommit(t, dir)
	zero := strings.Repeat("0", 40)
	goodLine := "refs/heads/good " + base + " refs/heads/good " + base + "\n"
	badLine := "refs/heads/bad " + bad + " refs/heads/bad " + base + "\n"
	for _, tc := range []struct {
		name, input string
		want        int
	}{
		{"multi", goodLine + badLine, 1},
		{"new", "refs/heads/new " + bad + " refs/heads/new " + zero + "\n", 1},
		{"delete", "(delete) " + zero + " refs/heads/old " + bad + "\n", 0},
		{"missing", "refs/heads/new " + base + " refs/heads/new " + strings.Repeat("a", 40) + "\n", 2},
		{"empty", "", 2},
		{"malformed", "HEAD\n", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rc, out, errout := invoke(t, dir, tc.input, offlineModel, "pre-push", "--policy-ref", base, "--format", "json")
			if rc != tc.want {
				t.Fatalf("rc=%d want=%d out=%s err=%s", rc, tc.want, out, errout)
			}
			if tc.name == "multi" {
				var rs []report
				if json.Unmarshal([]byte(out), &rs) != nil || len(rs) != 2 {
					t.Fatal(out)
				}
			}
		})
	}
}

func TestCLIBootstrapValidateDigestsHelp(t *testing.T) {
	dir := fixtureRepo(t)
	path := filepath.Join(dir, "policy.json")
	if err := os.WriteFile(path, defaultPolicy, 0600); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"validate", "site-digests"} {
		rc, out, errout := invoke(t, dir, "", offlineModel, command, "--policy-file", path)
		if rc != 0 || out == "" || errout != "" {
			t.Fatalf("%s: %d %s %s", command, rc, out, errout)
		}
	}
	var out, errout bytes.Buffer
	if rc := run([]string{"help"}, strings.NewReader(""), &out, &errout, offlineModel); rc != 0 || !strings.Contains(out.String(), "Exit:") {
		t.Fatal("help")
	}
	fixtureWrite(t, dir, "data.json", "{}")
	fixtureGit(t, dir, nil, "add", "data.json")
	rc, text, errtext := invoke(t, dir, "", offlineModel, "check", "--boundary", "pre-commit", "--staged", "--observe", "--policy-file", path)
	if rc != 0 || !strings.Contains(text, "observed") {
		t.Fatalf("bootstrap: %d %s %s", rc, text, errtext)
	}
}

func TestCLICIRequiresExplicitTrustedPolicy(t *testing.T) {
	dir, base := committedPolicy(t)
	p, err := decodePolicy(defaultPolicy)
	if err != nil {
		t.Fatal(err)
	}
	p.Rules = p.Rules[:1] // Candidate weakens its own policy while breaking JSON.
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	fixtureWrite(t, dir, ".quality-gate/policy.json", string(b))
	fixtureWrite(t, dir, "data.json", "{broken")
	rev := fixtureCommit(t, dir)
	for _, boundary := range []string{"ci", "pre-publish"} {
		stagedRC, _, _ := invoke(t, dir, "", offlineModel, "check", "--boundary", boundary, "--staged", "--policy-ref", base)
		if stagedRC != 2 {
			t.Fatalf("%s accepted index instead of committed artifact", boundary)
		}
		rc, _, _ := invoke(t, dir, "", offlineModel, "check", "--boundary", boundary, "--base", base, "--revision", rev)
		if rc != 2 {
			t.Fatalf("%s trusted candidate default policy: %d", boundary, rc)
		}
		rc, _, _ = invoke(t, dir, "", offlineModel, "check", "--boundary", boundary, "--base", base, "--revision", rev, "--policy-ref", base)
		if rc != 1 {
			t.Fatalf("%s did not hold under base policy: %d", boundary, rc)
		}
	}
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("closed output") }
func TestReportOutputFailureIsUnknown(t *testing.T) {
	dir, _ := committedPolicy(t)
	for _, format := range []string{"text", "json"} {
		var stderr bytes.Buffer
		rc := run([]string{"check", "--repo", dir, "--boundary", "pre-commit", "--staged", "--format", format}, strings.NewReader(""), brokenWriter{}, &stderr, offlineModel)
		if rc != 2 {
			t.Fatalf("format %s output failure returned %d", format, rc)
		}
	}
}

func TestUnusedFlagsRejectedBeforeIO(t *testing.T) {
	for _, args := range [][]string{{"init", "--staged"}, {"lefthook", "--allow-remote"}, {"validate", "--allow-remote"}, {"pre-push", "--staged"}} {
		var out, stderr bytes.Buffer
		if rc := run(args, strings.NewReader(""), &out, &stderr, offlineModel); rc != 2 {
			t.Fatalf("unused flag accepted: %v", args)
		}
	}
}
