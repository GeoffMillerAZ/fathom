package hookeval

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCUEWireParity generates disposable JSON cases outside the module. The
// committed CUE fixtures stay positive; negative cases live in this test.
func TestCUEWireParity(t *testing.T) {
	bin := os.Getenv("HOOKEVAL_CUE_BIN")
	if bin == "" {
		var err error
		bin, err = exec.LookPath("cue")
		if err != nil {
			t.Skip("cue unavailable")
		}
	}
	schema, err := os.ReadFile("schema.cue")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.cue")
	if err := os.WriteFile(schemaPath, schema, 0o600); err != nil {
		t.Fatal(err)
	}
	check := func(name, kind string, data []byte, wantValid bool) {
		t.Helper()
		path := filepath.Join(dir, name+".json")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(bin, "vet", "-c", "-d", kind, schemaPath, path)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if wantValid && err != nil {
			t.Errorf("%s rejected by CUE: %v: %s", name, err, out)
		}
		if !wantValid && err == nil {
			t.Errorf("%s accepted by CUE", name)
		}
	}
	request, err := NewRequest(strings.Repeat("a", 64), []byte("{}"), Subject{Host: "codex", Operation: "spawn", Model: "m", Effort: "low", Brief: "work"}, false)
	if err != nil {
		t.Fatal(err)
	}
	reqBytes, err := MarshalRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	check("valid-request", "#Request", reqBytes, true)
	var req map[string]any
	if err := json.Unmarshal(reqBytes, &req); err != nil {
		t.Fatal(err)
	}
	mutateRequest := func(name string, mutate func(map[string]any)) {
		t.Helper()
		clone := cloneObject(t, req)
		mutate(clone)
		b, err := json.Marshal(clone)
		if err != nil {
			t.Fatal(err)
		}
		check(name, "#Request", b, false)
	}
	mutateRequest("missing-false", func(m map[string]any) { delete(m, "advisorySelected") })
	mutateRequest("missing-subject-false", func(m map[string]any) { delete(m["subject"].(map[string]any), "fork") })
	mutateRequest("missing-context-false", func(m map[string]any) { delete(m["subject"].(map[string]any), "contextFork") })
	mutateRequest("byte-bound", func(m map[string]any) { m["subject"].(map[string]any)["model"] = strings.Repeat("é", 129) })
	mutateRequest("base64-syntax", func(m map[string]any) { m["policyPayload"] = "!" })
	mutateRequest("request-null", func(m map[string]any) { m["advisorySelected"] = nil })
	result := Result{Version: ResultVersion, RequestID: request.RequestID, ContextSHA256: request.ContextSHA256, PolicySHA256: request.PolicySHA256, NoActionAuthority: true, RequiredDecision: "pass", Codes: []string{"allowed"}, Class: "worker", ClassSource: "default", AdviceMatches: 0}
	resBytes, err := MarshalResult(result)
	if err != nil {
		t.Fatal(err)
	}
	check("valid-result", "#Result", resBytes, true)
	var res map[string]any
	if err := json.Unmarshal(resBytes, &res); err != nil {
		t.Fatal(err)
	}
	mutateResult := func(name string, mutate func(map[string]any)) {
		t.Helper()
		clone := cloneObject(t, res)
		mutate(clone)
		b, err := json.Marshal(clone)
		if err != nil {
			t.Fatal(err)
		}
		check(name, "#Result", b, false)
	}
	mutateResult("missing-zero", func(m map[string]any) { delete(m, "adviceMatches") })
	mutateResult("missing-advisory-false", func(m map[string]any) { delete(m, "advisoryEvaluated") })
	mutateResult("missing-authority-true", func(m map[string]any) { delete(m, "noActionAuthority") })
	mutateResult("pass-denial-code", func(m map[string]any) { m["codes"] = []string{"no_model"} })
	mutateResult("duplicate-code", func(m map[string]any) { m["codes"] = []string{"allowed", "allowed"} })
	mutateResult("class-source", func(m map[string]any) { m["classSource"] = "critic_pattern" })
	mutateResult("unknown-class", func(m map[string]any) { m["requiredDecision"] = "unknown"; m["codes"] = []string{"policy_invalid"} })
	mutateResult("resume-code", func(m map[string]any) {
		m["requiredDecision"] = "not-applicable"
		m["class"] = "not-applicable"
		m["classSource"] = "not-applicable"
	})
	mutateResult("advice-overflow", func(m map[string]any) { m["adviceMatches"] = 33 })
	mutateResult("authority-null", func(m map[string]any) { m["noActionAuthority"] = nil })
	check("integer-overflow", "#Result", bytes.Replace(resBytes, []byte(`"adviceMatches":0`), []byte(`"adviceMatches":9223372036854775808`), 1), false)
}

func cloneObject(t *testing.T, m map[string]any) map[string]any {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}
