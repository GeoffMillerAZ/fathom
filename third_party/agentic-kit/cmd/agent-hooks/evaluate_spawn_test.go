package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/internal/hookeval"
)

func evalPolicy(t *testing.T, mutate func(map[string]any)) []byte {
	t.Helper()
	var m map[string]any
	if json.Unmarshal(defaultPolicyJSON, &m) != nil {
		t.Fatal("policy")
	}
	if mutate != nil {
		mutate(m)
	}
	b, e := json.Marshal(m)
	if e != nil {
		t.Fatal(e)
	}
	return b
}

func evalRun(t *testing.T, policy []byte, s hookeval.Subject, advice bool) (hookeval.Result, int) {
	t.Helper()
	r, e := hookeval.NewRequest(strings.Repeat("a", 64), policy, s, advice)
	if e != nil {
		t.Fatal(e)
	}
	b, e := hookeval.MarshalRequest(r)
	if e != nil {
		t.Fatal(e)
	}
	a := &app{stdin: bytes.NewReader(b), stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}, env: func(string) string { panic("environment read") }, home: "/must-not-read"}
	code := a.run([]string{"evaluate-spawn"})
	got, e := hookeval.DecodeResult(a.stdout.(*bytes.Buffer).Bytes())
	if e != nil {
		t.Fatalf("result: %v; stderr %q", e, a.stderr.(*bytes.Buffer).String())
	}
	if hookeval.BindResult(got, r) != nil {
		t.Fatal("unbound result")
	}
	return got, code
}

func TestEvaluateSpawnStrictProfileAndRequired(t *testing.T) {
	policy := evalPolicy(t, nil)
	base := hookeval.Subject{Host: "claude", Operation: "spawn", Model: "haiku", Brief: "find file"}
	got, code := evalRun(t, policy, base, false)
	if code != 0 || got.RequiredDecision != "pass" || got.AdvisoryEvaluated {
		t.Fatalf("pass=%+v code=%d", got, code)
	}
	base.Brief = "[critic] review CSS"
	got, code = evalRun(t, policy, base, true)
	if code != 0 || got.Class != "critic" || got.ClassSource != "critic_marker" || !got.AdvisoryEvaluated {
		t.Fatalf("class/advice=%+v code=%d", got, code)
	}
	base.Brief = "find file"
	base.Model = ""
	got, code = evalRun(t, policy, base, false)
	if code != 1 || got.Codes[0] != "no_model" {
		t.Fatalf("deny=%+v code=%d", got, code)
	}
	base.Operation = "resume"
	got, code = evalRun(t, policy, base, true)
	if code != 0 || got.RequiredDecision != "not-applicable" || got.AdvisoryEvaluated {
		t.Fatalf("resume=%+v code=%d", got, code)
	}
	base.Operation = "spawn"
	base.Model = "haiku"
	base.Fork = true
	got, code = evalRun(t, policy, base, true)
	if code != 1 || got.Codes[0] != "fork_unmarked" {
		t.Fatalf("fork=%+v code=%d", got, code)
	}
	base.Brief = "[fork] find file"
	got, code = evalRun(t, policy, base, true)
	if code != 0 || got.RequiredDecision != "pass" {
		t.Fatalf("marked fork=%+v code=%d", got, code)
	}
}

func TestEvaluateSpawnCodexContextForkAndProfile(t *testing.T) {
	policy := evalPolicy(t, func(m map[string]any) { m["tiers"].([]any)[0].(map[string]any)["codex"] = []any{"cheap"} })
	s := hookeval.Subject{Host: "codex", Operation: "spawn", Model: "cheap", Effort: "low", ContextFork: true, Brief: "work"}
	got, code := evalRun(t, policy, s, true)
	if code != 1 || got.Codes[0] != "fork_unmarked" || got.AdvisoryEvaluated {
		t.Fatalf("context fork=%+v code=%d", got, code)
	}
	s.Model = ""
	s.Effort = ""
	got, code = evalRun(t, policy, s, true)
	if code != 1 || len(got.Codes) != 3 {
		t.Fatalf("aggregate=%+v code=%d", got, code)
	}
	s.Model = "cheap"
	s.Effort = "low"
	s.Brief = "[fork] work"
	got, code = evalRun(t, policy, s, true)
	if code != 0 || !got.AdvisoryEvaluated {
		t.Fatalf("marked context fork=%+v code=%d", got, code)
	}
	unmapped := evalPolicy(t, nil)
	got, code = evalRun(t, unmapped, s, false)
	if code != 2 || got.Codes[0] != "profile_invalid" {
		t.Fatalf("unmapped=%+v code=%d", got, code)
	}
	invalid := []byte(`{}`)
	got, code = evalRun(t, invalid, s, false)
	if code != 2 || got.Codes[0] != "policy_invalid" {
		t.Fatalf("invalid=%+v code=%d", got, code)
	}
}

func TestEvaluateSpawnRequestFaultIsFixed(t *testing.T) {
	a := &app{stdin: strings.NewReader(`{"policyPayload":"secret"}`), stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}}
	if code := a.run([]string{"evaluate-spawn"}); code != 2 || a.stdout.(*bytes.Buffer).Len() != 0 || strings.Contains(a.stderr.(*bytes.Buffer).String(), "secret") {
		t.Fatal("leaked malformed request")
	}
	a.stdin = strings.NewReader("secret")
	a.stderr.(*bytes.Buffer).Reset()
	if code := a.run([]string{"evaluate-spawn", "--help"}); code != 0 {
		t.Fatal("help read input")
	}
}

func TestEvaluatorRejectsDuplicatePolicyControl(t *testing.T) {
	duplicate := bytes.Replace(defaultPolicyJSON, []byte(`"require_model": true`), []byte(`"require_model": false, "require_model": true`), 1)
	if bytes.Equal(duplicate, defaultPolicyJSON) {
		t.Fatal("fixture did not mutate")
	}
	if _, err := parsePolicy(duplicate); err != nil {
		t.Fatalf("legacy behavior changed: %v", err)
	}
	s := hookeval.Subject{Host: "claude", Operation: "spawn", Model: "haiku", Brief: "work"}
	got, code := evalRun(t, duplicate, s, false)
	if code != 2 || got.RequiredDecision != "unknown" || len(got.Codes) != 1 || got.Codes[0] != "policy_invalid" {
		t.Fatalf("duplicate policy=%+v code=%d", got, code)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("private writer detail") }

func TestEvaluatorStartupDoesNotDiscoverHomeOrEnvironment(t *testing.T) {
	home := func() (string, error) { panic("home discovery") }
	getenv := func(string) string { panic("environment discovery") }
	for _, tc := range []struct {
		args  []string
		input string
		want  int
	}{
		{[]string{"evaluate-spawn", "--help"}, "unread input", 0},
		{[]string{"evaluate-spawn", "--bad"}, "unread input", 2},
		{[]string{"evaluate-spawn"}, "{}", 2},
	} {
		var out, errOut bytes.Buffer
		if got := runMain(tc.args, strings.NewReader(tc.input), &out, &errOut, getenv, home); got != tc.want {
			t.Errorf("%v exit %d want %d", tc.args, got, tc.want)
		}
	}
	r, err := hookeval.NewRequest(strings.Repeat("a", 64), defaultPolicyJSON, hookeval.Subject{Host: "claude", Operation: "spawn", Model: "haiku", Brief: "work"}, false)
	if err != nil {
		t.Fatal(err)
	}
	b, err := hookeval.MarshalRequest(r)
	if err != nil {
		t.Fatal(err)
	}
	var validOut bytes.Buffer
	if got := runMain([]string{"evaluate-spawn"}, bytes.NewReader(b), &validOut, io.Discard, getenv, home); got != 0 {
		t.Errorf("valid peer startup exit %d", got)
	}
	if _, err := hookeval.DecodeResult(validOut.Bytes()); err != nil {
		t.Errorf("valid peer result: %v", err)
	}
	var errOut bytes.Buffer
	if got := runMain([]string{"evaluate-spawn", "--help"}, strings.NewReader(""), failingWriter{}, &errOut, getenv, home); got != 2 {
		t.Errorf("help output failure exit %d", got)
	}
	if strings.Contains(errOut.String(), "private writer detail") {
		t.Error("help leaked writer error")
	}
	called := false
	if got := runMain([]string{"version"}, strings.NewReader(""), io.Discard, io.Discard, getenv, func() (string, error) { called = true; return "/legacy-home", nil }); got != 0 || !called {
		t.Errorf("legacy startup: exit %d home called %t", got, called)
	}
}
