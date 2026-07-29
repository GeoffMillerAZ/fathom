package bundle

// adversarial_test.go — the sanitization guarantee, proven byte-level.
// docs/design/research-flywheel.md's bundle requirements: file paths, repo/
// service names, symbol names, source code, commit SHAs of the user's
// repos, free-text notes, env vars, usernames, and hostnames must never
// appear in a bundle. Build's own type signature (Input has no free-text
// field) makes most of this true by construction; these tests seed
// secret-shaped strings into the one field that IS identifying
// (ServicePlaintextCalls) and assert, byte-for-byte, that neither the raw
// JSON nor the Markdown rendering contains them.

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

// TestBundle_TypeSystemHasNoFreeTextField is a structural/documentation
// test: it enumerates Bundle's JSON field names and asserts none of them
// suggest a free-text carrier (message, note, text, symptom, path, repo,
// error) sneaked in during a later edit. This is a regression guard for
// the allowlist discipline itself, not a functional test.
func TestBundle_TypeSystemHasNoFreeTextField(t *testing.T) {
	b := Bundle{}
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
	for key := range asMap {
		for _, word := range strings.Split(strings.ToLower(key), "_") {
			if forbidden[word] {
				t.Errorf("Bundle field %q looks like a free-text/identifying carrier (word %q) — re-check the allowlist", key, word)
			}
		}
	}
}
