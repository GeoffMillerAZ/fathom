package main

import (
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func validTestPolicy() policy {
	return policy{
		Version: 1,
		ID:      "repo",
		Limits:  limits{MaxFiles: 10, MaxFileBytes: 1024, MaxTotalBytes: 4096, TimeoutSeconds: 30},
		Rules: []rule{{ID: "conflicts", Kind: "conflict-markers", Boundaries: []string{"pre-commit", "ci"},
			Select: selector{Prefixes: []string{""}}}},
		Model: modelPolicy{Provider: "none", MaxInputBytes: 8192, TimeoutSeconds: 30, CacheTTLSeconds: 0, Sites: []modelSite{}},
	}
}

func encodedPolicy(t *testing.T, p policy) []byte {
	t.Helper()
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestDecodePolicyStrictJSON(t *testing.T) {
	good := encodedPolicy(t, validTestPolicy())
	if _, err := decodePolicy(good); err != nil {
		t.Fatalf("valid policy rejected: %v", err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(good, &top); err != nil {
		t.Fatal(err)
	}
	modelJSON := top["model"]
	var model map[string]json.RawMessage
	if err := json.Unmarshal(modelJSON, &model); err != nil {
		t.Fatal(err)
	}
	rulesJSON := top["rules"]
	model["sites"], _ = json.Marshal([]modelSite{{ID: "review", Version: "v1", Question: "Review?", Boundaries: []string{"ci"}, Select: selector{Prefixes: []string{"src/"}}, Mode: "shadow", MinConfidence: .5}})
	top["model"], _ = json.Marshal(model)
	delete(top, "rules")
	missingRules, _ := json.Marshal(top)
	model = map[string]json.RawMessage{}
	if err := json.Unmarshal(modelJSON, &model); err != nil {
		t.Fatal(err)
	}
	delete(model, "sites")
	top["rules"] = rulesJSON
	top["model"], _ = json.Marshal(model)
	missingSites, _ := json.Marshal(top)
	for name, data := range map[string][]byte{
		"missing rules":      missingRules,
		"missing sites":      missingSites,
		"unknown top":        []byte(strings.Replace(string(good), `"version":1`, `"version":1,"extra":1`, 1)),
		"uppercase field":    []byte(strings.Replace(string(good), `"version":1`, `"Version":1`, 1)),
		"unknown nested":     []byte(strings.Replace(string(good), `"max_files":10`, `"max_files":10,"extra":1`, 1)),
		"duplicate top":      []byte(strings.Replace(string(good), `"version":1`, `"version":1,"version":1`, 1)),
		"casefold duplicate": []byte(strings.Replace(string(good), `"version":1`, `"version":1,"VERSION":1`, 1)),
		"duplicate nested":   []byte(strings.Replace(string(good), `"max_files":10`, `"max_files":10,"max_files":10`, 1)),
		"duplicate in array": []byte(strings.Replace(string(good), `"kind":"conflict-markers"`, `"kind":"conflict-markers","kind":"json-valid"`, 1)),
		"stray zero option":  []byte(strings.Replace(string(good), `"kind":"conflict-markers"`, `"kind":"conflict-markers","max_bytes":0`, 1)),
		"null model":         []byte(strings.Replace(string(good), `"model":{`, `"model":null,"ignored":{`, 1)),
		"null optional":      []byte(strings.Replace(string(good), `"select":{`, `"companion":null,"select":{`, 1)),
		"trailing":           append(append([]byte{}, good...), []byte(` {}`)...),
		"bad encoding":       append(append([]byte{}, good...), 0xff),
		"oversize":           []byte(strings.Repeat(" ", maxPolicyBytes+1)),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodePolicy(data); err == nil {
				t.Fatal("invalid policy accepted")
			}
		})
	}
	withEmptyEnv := []byte(strings.Replace(string(good), `"provider":"none"`, `"provider":"none","model_env":""`, 1))
	if _, err := decodePolicy(withEmptyEnv); err != nil {
		t.Fatalf("provider none with explicit empty env rejected: %v", err)
	}
}

func TestValidatePolicyRejectsUnsafeAndUnboundedConfig(t *testing.T) {
	tests := []struct {
		name string
		edit func(*policy)
	}{
		{"version", func(p *policy) { p.Version = 2 }},
		{"empty id", func(p *policy) { p.ID = "" }},
		{"unsafe id", func(p *policy) { p.ID = "-option" }},
		{"file count", func(p *policy) { p.Limits.MaxFiles = 10001 }},
		{"file bytes", func(p *policy) { p.Limits.MaxFileBytes = 0 }},
		{"total bytes", func(p *policy) { p.Limits.MaxTotalBytes = 16<<20 + 1 }},
		{"timeout", func(p *policy) { p.Limits.TimeoutSeconds = 301 }},
		{"no checks", func(p *policy) { p.Rules = nil }},
		{"duplicate boundary", func(p *policy) { p.Rules[0].Boundaries = []string{"ci", "ci"} }},
		{"unknown boundary", func(p *policy) { p.Rules[0].Boundaries = []string{"commit"} }},
		{"empty prefixes", func(p *policy) { p.Rules[0].Select.Prefixes = nil }},
		{"empty suffix", func(p *policy) { p.Rules[0].Select.Suffixes = []string{""} }},
		{"absolute prefix", func(p *policy) { p.Rules[0].Select.Prefixes = []string{"/etc/"} }},
		{"traversal prefix", func(p *policy) { p.Rules[0].Select.Prefixes = []string{"src/../other"} }},
		{"dot prefix", func(p *policy) { p.Rules[0].Select.Prefixes = []string{"src/./"} }},
		{"backslash", func(p *policy) { p.Rules[0].Select.Prefixes = []string{`src\x`} }},
		{"control path", func(p *policy) { p.Rules[0].Select.Prefixes = []string{"src/\n"} }},
		{"unknown kind", func(p *policy) { p.Rules[0].Kind = "exec" }},
		{"stray max bytes", func(p *policy) { p.Rules[0].MaxBytes = 1 }},
		{"missing companion", func(p *policy) { p.Rules[0].Kind = "require-companion" }},
		{"bad max bytes", func(p *policy) { p.Rules[0].Kind = "max-bytes"; p.Rules[0].MaxBytes = 0 }},
		{"duplicate ID", func(p *policy) { p.Rules = append(p.Rules, p.Rules[0]) }},
		{"too many rules", func(p *policy) { p.Rules = make([]rule, 1001) }},
		{"unknown provider", func(p *policy) { p.Model.Provider = "shell" }},
		{"literal model", func(p *policy) { p.Model.Provider = "openai"; p.Model.ModelEnv = "gpt-6" }},
		{"missing remote env", func(p *policy) { p.Model.Provider = "openai" }},
		{"input bytes", func(p *policy) { p.Model.MaxInputBytes = 1<<20 + 1 }},
		{"model timeout", func(p *policy) { p.Model.TimeoutSeconds = 121 }},
		{"cache ttl", func(p *policy) { p.Model.CacheTTLSeconds = 86401 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := validTestPolicy()
			tt.edit(&p)
			if err := validatePolicy(p); err == nil {
				t.Fatal("invalid policy accepted")
			}
		})
	}
}

func TestValidateModelSitesAndCalibration(t *testing.T) {
	p := validTestPolicy()
	p.Model.ModelEnv, p.Model.ExpectedModelEnv, p.Model.APIKeyEnv = "Q_MODEL", "Q_EXPECTED", "Q_KEY"
	if err := validatePolicy(p); err != nil {
		t.Fatalf("provider none with inert environment names rejected: %v", err)
	}
	p.Model = modelPolicy{Provider: "openai", ModelEnv: "Q_MODEL", ExpectedModelEnv: "Q_EXPECTED", APIKeyEnv: "Q_KEY", MaxInputBytes: 8192, TimeoutSeconds: 5}
	site := modelSite{ID: "review", Version: "v1", Question: "Is this change concerning?", Boundaries: []string{"ci"}, Select: selector{Prefixes: []string{"src/"}, Suffixes: []string{".go"}}, Mode: "shadow", MinConfidence: .8}
	p.Model.Sites = []modelSite{site}
	if err := validatePolicy(p); err != nil {
		t.Fatalf("valid site rejected: %v", err)
	}
	for name, edit := range map[string]func(*modelSite){
		"missing question": func(s *modelSite) { s.Question = " " },
		"mode":             func(s *modelSite) { s.Mode = "clear" },
		"confidence":       func(s *modelSite) { s.MinConfidence = math.NaN() },
		"version":          func(s *modelSite) { s.Version = "" },
		"boundary":         func(s *modelSite) { s.Boundaries = nil },
	} {
		t.Run(name, func(t *testing.T) {
			bad := p
			bad.Model.Sites = []modelSite{site}
			edit(&bad.Model.Sites[0])
			if err := validatePolicy(bad); err == nil {
				t.Fatal("invalid site accepted")
			}
		})
	}
	c := &calibration{Site: "repo/ci/review", QuestionSHA256: siteDigest(site), Provider: "openai", ResolvedModel: "pinned", ValidUntil: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), MinPerClass: 1, MinN: 3, MinAccuracy: .8, MinWilsonLo: .5, Cases: []labeledCase{{ID: "case1", Expected: "clear", Actual: "clear", Confidence: .9, Provenance: "human"}}}
	p.Model.Sites[0].Calibration = c
	if err := validatePolicy(p); err != nil {
		t.Fatalf("valid calibration rejected: %v", err)
	}
	for name, edit := range map[string]func(*calibration){
		"sha":             func(c *calibration) { c.QuestionSHA256 = "bad" },
		"provider":        func(c *calibration) { c.Provider = "typesafe" },
		"model":           func(c *calibration) { c.ResolvedModel = "" },
		"site format":     func(c *calibration) { c.Site = "review" },
		"date":            func(c *calibration) { c.ValidUntil = time.Time{} },
		"min per class":   func(c *calibration) { c.MinPerClass = 0 },
		"min n":           func(c *calibration) { c.MinN = 0 },
		"accuracy":        func(c *calibration) { c.MinAccuracy = 0 },
		"wilson":          func(c *calibration) { c.MinWilsonLo = 2 },
		"case id":         func(c *calibration) { c.Cases[0].ID = "" },
		"case answer":     func(c *calibration) { c.Cases[0].Actual = "unknown" },
		"case confidence": func(c *calibration) { c.Cases[0].Confidence = math.Inf(1) },
		"case provenance": func(c *calibration) { c.Cases[0].Provenance = "model" },
		"duplicate case":  func(c *calibration) { c.Cases = append(c.Cases, c.Cases[0]) },
	} {
		t.Run(name, func(t *testing.T) {
			bad := p
			bad.Model.Sites = []modelSite{site}
			clone := *c
			clone.Cases = append([]labeledCase{}, c.Cases...)
			edit(&clone)
			bad.Model.Sites[0].Calibration = &clone
			if err := validatePolicy(bad); err == nil {
				t.Fatal("invalid calibration accepted")
			}
		})
	}
}

func TestSelectorHelpers(t *testing.T) {
	s := selector{Prefixes: []string{"src/", "test/"}, Suffixes: []string{".go", ".cue"}}
	if !includes([]string{"ci", "pre-push"}, "ci") || includes([]string{"ci"}, "pre-commit") ||
		!matches(s, "src/main.go") || !matches(s, "test/schema.cue") || matches(s, "docs/main.go") || matches(s, "src/main.md") {
		t.Fatal("selector helper mismatch")
	}
}

func TestSchemaCalendarAndInertEnvParity(t *testing.T) {
	if _, err := exec.LookPath("cue"); err != nil {
		t.Skip("CUE CLI not installed")
	}
	vet := func(data []byte) error {
		t.Helper()
		path := filepath.Join(t.TempDir(), "policy.json")
		if err := os.WriteFile(path, data, 0600); err != nil {
			return err
		}
		return exec.Command("cue", "vet", "-d", "#Policy", "schema.cue", path).Run()
	}
	base := encodedPolicy(t, validTestPolicy())
	emptyEnv := []byte(strings.Replace(string(base), `"provider":"none"`, `"provider":"none","model_env":""`, 1))
	if _, err := decodePolicy(emptyEnv); err != nil {
		t.Fatal(err)
	}
	if err := vet(emptyEnv); err != nil {
		t.Fatalf("CUE rejected explicit inert env: %v", err)
	}
	p := validTestPolicy()
	site := modelSite{ID: "review", Version: "v1", Question: "Is there a concern?", Boundaries: []string{"ci"}, Select: selector{Prefixes: []string{"src/"}}, Mode: "shadow", MinConfidence: .5}
	site.Calibration = &calibration{Site: "repo/ci/review", QuestionSHA256: siteDigest(site), Provider: "none", ResolvedModel: "pinned", ValidUntil: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), MinPerClass: 1, MinN: 1, MinAccuracy: .5, MinWilsonLo: .5, Cases: []labeledCase{}}
	p.Model.Sites = []modelSite{site}
	good := encodedPolicy(t, p)
	if _, err := decodePolicy(good); err != nil {
		t.Fatal(err)
	}
	if err := vet(good); err != nil {
		t.Fatalf("CUE rejected valid calendar date: %v", err)
	}
	missingCases := []byte(strings.Replace(string(good), `,"cases":[]`, "", 1))
	if string(missingCases) == string(good) {
		t.Fatal("fixture did not remove cases")
	}
	if _, err := decodePolicy(missingCases); err == nil {
		t.Fatal("runtime accepted missing calibration cases")
	}
	bad := []byte(strings.Replace(string(good), "2030-01-01T00:00:00Z", "2030-99-01T00:00:00Z", 1))
	if _, err := decodePolicy(bad); err == nil {
		t.Fatal("runtime accepted invalid calendar date")
	}
	if err := vet(bad); err == nil {
		t.Fatal("CUE accepted invalid calendar date")
	}
	zero := []byte(strings.Replace(string(good), "2030-01-01T00:00:00Z", "0001-01-01T01:00:00+01:00", 1))
	if _, err := decodePolicy(zero); err == nil {
		t.Fatal("runtime accepted zero instant")
	}
	if err := vet(zero); err == nil {
		t.Fatal("CUE accepted zero instant")
	}
	longModel := []byte(strings.Replace(string(good), `"resolved_model":"pinned"`, `"resolved_model":"`+strings.Repeat("é", 200)+`"`, 1))
	if _, err := decodePolicy(longModel); err == nil {
		t.Fatal("runtime accepted overlong resolved model")
	}
	if err := vet(longModel); err == nil {
		t.Fatal("CUE accepted overlong resolved model")
	}
}
