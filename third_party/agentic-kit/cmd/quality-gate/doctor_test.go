package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDoctorConfigurationOnlyAndUnknown(t *testing.T) {
	dir, base := committedPolicy(t)
	calls := 0
	model := func(context.Context, policy, string, string, snapshot, modelOptions) modelResult {
		calls++
		return modelResult{}
	}
	rc, out, stderr := invoke(t, dir, "", model, "doctor", "--format", "json")
	if rc != 0 || stderr != "" || calls != 0 {
		t.Fatalf("doctor: rc=%d calls=%d out=%s err=%s", rc, calls, out, stderr)
	}
	var r doctorReport
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatal(err)
	}
	if r.Scope != "configuration-only" || r.Boundary != "pre-commit" || r.PolicyCommit != base || r.PolicySHA256 == "" || r.State != "pass" || r.EvaluatedAt.IsZero() || len(r.Caveats) < 2 {
		t.Fatal(out)
	}
	rc, out, _ = invoke(t, dir, "", model, "doctor", "--boundary", "ci", "--format", "json")
	if rc != 2 || !strings.Contains(out, "explicit-trusted-policy-ref-required") || calls != 0 {
		t.Fatal(rc, out)
	}
	rc, out, _ = invoke(t, dir, "", model, "doctor", "--boundary", "ci", "--policy-ref", base)
	if rc != 0 || !strings.Contains(out, "scope=configuration-only") {
		t.Fatal(rc, out)
	}
}

func TestDoctorLocalUntrustedAndNoSecrets(t *testing.T) {
	dir := fixtureRepo(t)
	path := filepath.Join(dir, "policy.json")
	if err := os.WriteFile(path, defaultPolicy, 0600); err != nil {
		t.Fatal(err)
	}
	secret := "credential-value-must-stay-private"
	t.Setenv("QUALITY_GATE_API_KEY", secret)
	rc, out, stderr := invoke(t, dir, "", offlineModel, "doctor", "--policy-file", path, "--format", "json")
	if rc != 0 || !strings.Contains(out, "local-untrusted") || strings.Contains(out+stderr, secret) {
		t.Fatalf("bootstrap: %d %s %s", rc, out, stderr)
	}
	if err := os.WriteFile(path, []byte(`{"version":1,"secret":"`+secret+`"}`), 0600); err != nil {
		t.Fatal(err)
	}
	rc, out, stderr = invoke(t, dir, "", offlineModel, "doctor", "--policy-file", path, "--format", "json")
	if rc != 2 || !strings.Contains(out, "policy-invalid") || strings.Contains(out+stderr, secret) {
		t.Fatalf("invalid policy: %d %s %s", rc, out, stderr)
	}
}

func TestDoctorRejectsUnsupportedFlagsAndOutputFailure(t *testing.T) {
	dir, _ := committedPolicy(t)
	for _, flag := range []string{"--allow-remote", "--staged", "--cache-dir", "--observe", "--revision"} {
		args := []string{"doctor", "--repo", dir, flag}
		if flag == "--cache-dir" || flag == "--revision" {
			args = append(args, "irrelevant")
		}
		var out, stderr bytes.Buffer
		if rc := run(args, strings.NewReader(""), &out, &stderr, offlineModel); rc != 2 {
			t.Fatalf("accepted %s: %d", flag, rc)
		}
	}
	for _, format := range []string{"text", "json"} {
		var stderr bytes.Buffer
		if rc := run([]string{"doctor", "--repo", dir, "--format", format}, strings.NewReader(""), brokenWriter{}, &stderr, offlineModel); rc != 2 {
			t.Fatalf("output failure %s: %d", format, rc)
		}
	}
}

func TestDoctorRequiredModelUnknownAndClock(t *testing.T) {
	p, err := decodePolicy(defaultPolicy)
	if err != nil {
		t.Fatal(err)
	}
	p.Model.Provider = "openai"
	p.Model.Sites[0].Mode = "hold"
	p.Model.Sites[0].Calibration = nil
	dir := fixtureRepo(t)
	path := filepath.Join(dir, "policy.json")
	b, _ := json.Marshal(p)
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(p.Model.ModelEnv, "pinned")
	t.Setenv(p.Model.ExpectedModelEnv, "pinned")
	t.Setenv(p.Model.APIKeyEnv, "private-credential-value")
	var out bytes.Buffer
	o := options{repo: dir, boundary: "pre-commit", policyFile: path, format: "json", now: func() time.Time { return time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC) }}
	if rc := runDoctor(&out, o, false); rc != 2 || !strings.Contains(out.String(), "required-clear-calibration-missing") || strings.Contains(out.String(), "private-credential-value") {
		t.Fatal(rc, out.String())
	}
	t.Setenv(p.Model.ExpectedModelEnv, "")
	out.Reset()
	if rc := runDoctor(&out, o, false); rc != 2 || !strings.Contains(out.String(), "required-model-pin-missing") {
		t.Fatal(rc, out.String())
	}
}

func TestDoctorGitProbeRejectsUnusableExecutable(t *testing.T) {
	dir := fixtureRepo(t)
	path := filepath.Join(dir, "policy.json")
	if err := os.WriteFile(path, defaultPolicy, 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"false", "true"} {
		t.Run(name, func(t *testing.T) {
			bin, err := exec.LookPath(name)
			if err != nil {
				t.Skip(name + " unavailable")
			}
			var out bytes.Buffer
			o := options{repo: dir, gitBin: bin, policyFile: path, boundary: "pre-commit", format: "json"}
			if rc := runDoctor(&out, o, false); rc != 2 || !strings.Contains(out.String(), "trusted-git-unavailable") || strings.Contains(out.String(), "strict-policy-valid") {
				t.Fatal(rc, out.String())
			}
		})
	}
}

func TestDoctorAliasAndEligibleConfirmBand(t *testing.T) {
	p, err := decodePolicy(defaultPolicy)
	if err != nil {
		t.Fatal(err)
	}
	p.Model.Provider = "openai"
	site := &p.Model.Sites[0]
	site.Mode, site.MinConfidence = "hold", .5
	site.Calibration = &calibration{
		Site: p.ID + "/pre-commit/" + site.ID, Provider: "openai", ResolvedModel: "resolved-identity",
		ValidUntil: time.Date(2031, 1, 1, 0, 0, 0, 0, time.UTC), MinPerClass: 1, MinN: 1,
		MinAccuracy: .1, MinWilsonLo: .01,
		Cases: []labeledCase{
			{ID: "clear", Expected: "clear", Actual: "clear", Confidence: .6, Provenance: "human"},
			{ID: "concern", Expected: "concern", Actual: "concern", Confidence: .6, Provenance: "human"},
			{ID: "insufficient", Expected: "insufficient", Actual: "insufficient", Confidence: .6, Provenance: "human"},
		},
	}
	site.Calibration.QuestionSHA256 = siteDigest(*site, p.Model.Sites...)
	if got := calibrationStatus(p.ID, "pre-commit", *site, "openai", "resolved-identity", "clear", 1, time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), p.Model.Sites...); got == "pass" {
		t.Fatal("test needs only the confirm band eligible")
	}
	dir := fixtureRepo(t)
	path := filepath.Join(dir, "policy.json")
	b, _ := json.Marshal(p)
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(p.Model.ModelEnv, "model-alias")
	t.Setenv(p.Model.ExpectedModelEnv, "resolved-identity")
	t.Setenv(p.Model.APIKeyEnv, "private-secret")
	called := 0
	at := time.Date(2030, 1, 1, 0, 0, 0, 0, time.FixedZone("offset", -7*3600))
	o := options{repo: dir, policyFile: path, boundary: "pre-commit", format: "json", now: func() time.Time { called++; return at }}
	var out bytes.Buffer
	if rc := runDoctor(&out, o, false); rc != 0 || called != 1 || !strings.Contains(out.String(), "required-clear-calibration-has-eligible-band") || strings.Contains(out.String(), "private-secret") {
		t.Fatal(rc, called, out.String())
	}
	var r doctorReport
	if err := json.Unmarshal(out.Bytes(), &r); err != nil || !r.EvaluatedAt.Equal(at.UTC()) || r.EvaluatedAt.Location() != time.UTC {
		t.Fatal(err, r.EvaluatedAt)
	}
	site.Calibration.ValidUntil = at.UTC()
	b, _ = json.Marshal(p)
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if rc := runDoctor(&out, o, false); rc != 2 || !strings.Contains(out.String(), "required-clear-calibration-expired") {
		t.Fatal(rc, out.String())
	}
}

func TestTextReportUnselectedPathsAreBoundedAndQuoted(t *testing.T) {
	r := report{State: "pass", Boundary: "pre-commit", Coverage: boundaryCoverage{UnselectedPaths: []string{"a\ncontrol"}}}
	for i := 0; i < 24; i++ {
		r.Coverage.UnselectedPaths = append(r.Coverage.UnselectedPaths, "other")
	}
	var out bytes.Buffer
	if rc := writeReport(&out, "text", r); rc != 0 {
		t.Fatal(rc)
	}
	text := out.String()
	if strings.Count(text, "    unselected:") != 20 || !strings.Contains(text, `"a\ncontrol"`) || !strings.Contains(text, "5 more unselected paths; use --format json") {
		t.Fatal(text)
	}
}
