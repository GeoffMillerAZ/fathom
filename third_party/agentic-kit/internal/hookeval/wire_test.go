package hookeval

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func sample(t *testing.T) Request {
	t.Helper()
	r, e := NewRequest(strings.Repeat("a", 64), []byte(`{"version":1}`), Subject{Host: "codex", Operation: "spawn", Model: "m", Effort: "low", Brief: "work"}, true)
	if e != nil {
		t.Fatal(e)
	}
	return r
}

func TestRequestBindingAndStrictDecode(t *testing.T) {
	r := sample(t)
	b, e := MarshalRequest(r)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := DecodeRequest(b); e != nil {
		t.Fatal(e)
	}
	for _, bad := range [][]byte{
		bytes.Replace(b, []byte(`"advisorySelected":true`), []byte(`"advisorySelected":null`), 1),
		bytes.Replace(b, []byte(`"advisorySelected":true`), []byte(`"advisorySelected":true,"advisorySelected":false`), 1),
		bytes.Replace(b, []byte(`"advisorySelected":true`), []byte(`"AdvisorySelected":true`), 1),
		append(append([]byte{}, b...), []byte(` {}`)...),
		bytes.Replace(b, []byte(`"brief":"work"`), []byte(`"brief":"changed"`), 1),
	} {
		if _, e := DecodeRequest(bad); e == nil {
			t.Errorf("accepted malformed request: %s", bad)
		}
	}
	r.Subject.ContextFork = true
	r.Subject.Fork = true
	if ValidateRequest(r) == nil {
		t.Error("accepted contradictory fork")
	}
}

func TestResultLawsAndBinding(t *testing.T) {
	r := sample(t)
	v := Result{Version: ResultVersion, RequestID: r.RequestID, ContextSHA256: r.ContextSHA256, PolicySHA256: r.PolicySHA256, NoActionAuthority: true, RequiredDecision: "pass", Codes: []string{"allowed"}, Class: "worker", ClassSource: "default", AdvisoryEvaluated: true, AdviceMatches: 1}
	b, e := MarshalResult(v)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = DecodeResult(b); e != nil {
		t.Fatal(e)
	}
	if e = BindResult(v, r); e != nil {
		t.Fatal(e)
	}
	v.PolicySHA256 = strings.Repeat("b", 64)
	if BindResult(v, r) == nil {
		t.Error("accepted binding tamper")
	}
	v.PolicySHA256 = r.PolicySHA256
	v.NoActionAuthority = false
	if ValidateResult(v) == nil {
		t.Error("accepted action authority")
	}
	v.NoActionAuthority = true
	v.RequiredDecision = "deny"
	if ValidateResult(v) == nil {
		t.Error("accepted allowed deny")
	}
	v.RequiredDecision = "pass"
	v.Codes = []string{"allowed", "allowed"}
	if ValidateResult(v) == nil {
		t.Error("accepted duplicate codes")
	}
	var m map[string]any
	if json.Unmarshal(b, &m) != nil {
		t.Fatal("decode")
	}
	delete(m, "noActionAuthority")
	bad, _ := json.Marshal(m)
	if _, e := DecodeResult(bad); e == nil {
		t.Error("accepted missing false/true field")
	}
}

func TestRequestBudgetAndUTF8(t *testing.T) {
	s := Subject{Host: "claude", Operation: "spawn", Brief: strings.Repeat("x", 65536)}
	if _, err := NewRequest(strings.Repeat("a", 64), bytes.Repeat([]byte{'x'}, MaxPolicyBytes), s, false); err != nil {
		t.Fatalf("within budget: %v", err)
	}
	s.Brief += "x"
	if _, err := NewRequest(strings.Repeat("a", 64), []byte("{}"), s, false); err == nil {
		t.Error("accepted oversized brief")
	}
	s.Brief = string([]byte{0xff})
	if _, err := NewRequest(strings.Repeat("a", 64), []byte("{}"), s, false); err == nil {
		t.Error("accepted invalid UTF-8")
	}
	s.Brief = ""
	if _, err := NewRequest(strings.Repeat("a", 64), bytes.Repeat([]byte{'x'}, MaxPolicyBytes+1), s, false); err == nil {
		t.Error("accepted oversized policy")
	}
}
