package bundle

// adversarial_test.go — the sanitization guarantee, proven byte-level.
// docs/design/research-flywheel.md's bundle requirements: file paths, repo/
// service names, symbol names, source code, commit SHAs of the user's
// repos, free-text notes, env vars, usernames, and hostnames must never
// appear in a bundle. Build's own type signature (Input has no free-text
// field) makes most of this true by construction; these tests seed
// secret-shaped strings into the fields that ARE identifying or
// client-controlled (ServicePlaintextCalls, and — per critic Finding 1,
// 2026-07-28 — Tool/Surface/Arm) and assert, byte-for-byte, that neither the
// raw JSON nor the Markdown rendering contains them.

import (
	"encoding/json"
	"strings"
	"testing"
)

// secretShapedStrings are fake-but-realistic-looking sensitive values: an
// API-key shape, an absolute path, an email, a hostname, and a username —
// exactly the categories the design doc's "never included" list names.
var secretShapedStrings = []string{
	"sk-ant-api03-FAKESECRETVALUEnotreal1234567890",
	"/Users/jdoe/code/acme-internal-payments/internal/billing.go",
	"jdoe@example.com",
	"jdoes-macbook-pro.local",
	"jdoe",
}

func TestBuild_NeverLeaksSecretShapedServiceNames(t *testing.T) {
	svcCalls := map[string]int{}
	for i, s := range secretShapedStrings {
		// Simulate a service/repo whose name itself happens to embed one of
		// these shapes (the realistic worst case: a repo path used as the
		// service identity, as CLAUDE.md's own ADR-011 org-agnostic
		// discipline warns against).
		svcCalls["repo-named-"+s+"-suffix"] = i + 1
	}

	in := Input{
		GeneratedAt:           "2026-07-28T00:00:00Z",
		CallLogRead:           true,
		TotalCalls:            100,
		ServicePlaintextCalls: svcCalls,
		Salt:                  "fixed-test-salt",
	}
	b := Build(in)

	raw, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	md := RenderMarkdown(b)

	for _, secret := range secretShapedStrings {
		if strings.Contains(string(raw), secret) {
			t.Errorf("bundle JSON leaks secret-shaped string %q:\n%s", secret, raw)
		}
		if strings.Contains(md, secret) {
			t.Errorf("bundle Markdown leaks secret-shaped string %q:\n%s", secret, md)
		}
	}
	if strings.Contains(string(raw), "fixed-test-salt") || strings.Contains(md, "fixed-test-salt") {
		t.Error("bundle leaks the salt")
	}

	// Positive control: the hashes themselves DO appear (that's the point —
	// counts stay distinguishable).
	if len(b.ServiceActivity) != len(secretShapedStrings) {
		t.Fatalf("ServiceActivity = %d rows, want %d", len(b.ServiceActivity), len(secretShapedStrings))
	}
}

// TestBuild_HostileToolSurfaceArm_RejectedByValidate proves the CUE-side
// allowlist (schema/research-bundle/schema.cue's #Label pattern) is a real,
// independent second layer for Tool/Surface/Arm — critic Finding 1's
// requirement. cmd/fathom/research_bundle.go's sanitizeLabel is the primary,
// Go-side defense (it never lets a hostile value reach Input in the first
// place); this test proves that even if that defense had a bug and let a
// hostile string straight into Input, Validate still refuses to certify the
// resulting Bundle — "the trust never rests on a comment again."
func TestBuild_HostileToolSurfaceArm_RejectedByValidate(t *testing.T) {
	hostile := []string{
		"/Users/jdoe/code/acme-internal-payments/internal/billing.go",
		"agent | injected",
		"fathom\nEVIL",
		"sk-ant-api03-FAKESECRETVALUEnotreal1234567890",
		"UPPERCASE_NOT_ALLOWED",
	}
	for _, s := range hostile {
		t.Run(s, func(t *testing.T) {
			toolIn := Input{
				GeneratedAt: "2026-07-28T00:00:00Z", CallLogRead: true,
				ToolCalls: []ToolCount{{Tool: s, Calls: 1}},
			}
			if err := Validate(Build(toolIn)); err == nil {
				t.Errorf("Validate accepted a hostile tool value %q — the CUE #Label guard did not fire", s)
			}

			surfIn := Input{
				GeneratedAt: "2026-07-28T00:00:00Z", CallLogRead: true,
				SurfaceTraffic: []SurfaceCount{{Surface: s, Calls: 1}},
			}
			if err := Validate(Build(surfIn)); err == nil {
				t.Errorf("Validate accepted a hostile surface value %q — the CUE #Label guard did not fire", s)
			}

			armIn := Input{
				GeneratedAt: "2026-07-28T00:00:00Z",
				BenchArms:   []BenchArmCount{{Arm: s, TaskCount: 1, SuccessCount: 1, SuccessRate: 1}},
			}
			if err := Validate(Build(armIn)); err == nil {
				t.Errorf("Validate accepted a hostile bench arm value %q — the CUE #Label guard did not fire", s)
			}
		})
	}
}

// TestBundle_TypeSystemHasNoFreeTextField is a structural/documentation
// test: it walks Bundle's JSON field names — including inside the nested
// CallTraffic/Injection/VerifyGate groups (populated here so their fields
// actually appear in the marshaled output, unlike a zero Bundle where
// omitempty would hide them) — and asserts none of them suggest a free-text
// carrier (message, note, text, symptom, path, repo, error) sneaked in
// during a later edit. This is a regression guard for the allowlist
// discipline itself, not a functional test.
func TestBundle_TypeSystemHasNoFreeTextField(t *testing.T) {
	p := 1.0
	obs := 1
	b := Bundle{
		CallTraffic: &CallTrafficStats{
			SurfaceTraffic: []SurfaceCount{{Surface: "agent", Calls: 1}},
			ToolCalls:      []ToolCount{{Tool: "find_symbol", Calls: 1}},
			LatencyP50MS:   &p,
			LatencyP95MS:   &p,
		},
		Injection:         &InjectionStats{CoverageRate: &p},
		VerifyGate:        &VerifyGateStats{},
		TotalObservations: &obs,
		OpenObservations:  &obs,
		ServiceActivity:   []ServiceHashCount{{ServiceHash: strings.Repeat("a", 64), Calls: 1}},
		BenchArms:         []BenchArmCount{{Arm: "fathom", TaskCount: 1}},
		Retros:            &RetroStats{Outcomes: []RetroOutcomeCount{{Channel: "guard", Outcome: "committed", Firings: 1}}},
	}
	raw, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	var asMap map[string]any
	if err := json.Unmarshal(raw, &asMap); err != nil {
		t.Fatal(err)
	}
	forbidden := map[string]bool{
		"message": true, "note": true, "notes": true, "text": true, "symptom": true,
		"path": true, "repo": true, "error": true, "hostname": true, "username": true,
		"env": true, "name": true,
	}
	walkKeys(t, asMap, forbidden)
}

// walkKeys recursively checks every object key (split on "_") against
// forbidden, descending into nested objects and arrays-of-objects so a
// free-text field added inside a new nested group (CallTraffic, Injection,
// VerifyGate, or any future one) is caught the same as a top-level one.
func walkKeys(t *testing.T, v any, forbidden map[string]bool) {
	t.Helper()
	switch val := v.(type) {
	case map[string]any:
		for key, sub := range val {
			for _, word := range strings.Split(strings.ToLower(key), "_") {
				if forbidden[word] {
					t.Errorf("Bundle field %q looks like a free-text/identifying carrier (word %q) — re-check the allowlist", key, word)
				}
			}
			walkKeys(t, sub, forbidden)
		}
	case []any:
		for _, item := range val {
			walkKeys(t, item, forbidden)
		}
	}
}
