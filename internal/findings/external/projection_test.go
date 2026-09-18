package external

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
)

// hex64 returns a deterministic 64-hex digest for seed — a stand-in for a
// real sha256 payload/finding digest in test fixtures.
func hex64(seed string) string {
	h := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(h[:])
}

// validEventJSON returns a complete, valid observation event JSON object
// (no surrounding braces of a projection) — the base every test mutates.
func validEventJSON(findingSeed string) string {
	return fmt.Sprintf(`{
  "schemaVersion": 1,
  "kind": "observation",
  "findingId": %q,
  "producer": {"name": "reviewkit.system-review", "version": "1.0.0"},
  "subject": {"organization": "acme", "repository": "widget", "component": "api"},
  "sourceId": "rule-nil-deref",
  "sourceVersion": 1,
  "sourceRevision": "abc123",
  "payloadDigest": %q,
  "observedAt": "2026-09-01T12:00:00Z",
  "outcome": "fail",
  "coverage": "complete",
  "anchor": {"path": "internal/foo/bar.go", "digest": %q, "start": 10, "end": 12, "symbol": "Bar"},
  "sourceStatus": "open",
  "rule": {"id": "R1", "version": "1", "text": "nil deref risk", "remedy": "add a nil check", "closure": "manual review", "class": "correctness", "tier": "pre-scan"},
  "visibility": {"organization": "acme", "repository": "widget", "component": "api"}
}`, hex64(findingSeed+"-finding"), hex64(findingSeed+"-payload"), hex64(findingSeed+"-anchor"))
}

func wrapProjection(events ...string) []byte {
	return []byte(`{"schemaVersion":1,"delivery":"pending","reason":"outbox flush","events":[` + strings.Join(events, ",") + `]}`)
}

func TestParseProjection_Happy(t *testing.T) {
	raw := wrapProjection(validEventJSON("e1"))
	p, err := ParseProjection(raw)
	if err != nil {
		t.Fatalf("ParseProjection: %v", err)
	}
	if p.SchemaVersion != 1 || len(p.Events) != 1 {
		t.Fatalf("p = %+v", p)
	}
	ev := p.Events[0]
	if ev.Kind != "observation" || ev.Outcome != "fail" || ev.Rule.Tier != "pre-scan" {
		t.Errorf("ev = %+v", ev)
	}
	if ev.Anchor == nil || ev.Anchor.Path != "internal/foo/bar.go" || ev.Anchor.Start != 10 || ev.Anchor.End != 12 {
		t.Errorf("ev.Anchor = %+v", ev.Anchor)
	}
	if ev.Visibility != ev.Subject {
		t.Errorf("visibility %+v != subject %+v", ev.Visibility, ev.Subject)
	}
}

func TestParseProjection_TierAbsent_DefaultsAssumed(t *testing.T) {
	e := strings.Replace(validEventJSON("e2"), `"tier": "pre-scan"`, `"tier": ""`, 1)
	e = strings.Replace(e, `"class": "correctness", `, "", 1)
	raw := wrapProjection(e)
	p, err := ParseProjection(raw)
	if err != nil {
		t.Fatalf("ParseProjection: %v", err)
	}
	if p.Events[0].Rule.Tier != "assumed" {
		t.Errorf("Rule.Tier = %q, want \"assumed\"", p.Events[0].Rule.Tier)
	}
	if p.Events[0].Rule.Class != "" {
		t.Errorf("Rule.Class = %q, want \"\" (unclassified)", p.Events[0].Rule.Class)
	}
}

func TestParseProjection_UnanchoredProcessSubject(t *testing.T) {
	e := validEventJSON("e3")
	// Drop the anchor object entirely — remove the whole "anchor": {...} block.
	i := strings.Index(e, `"anchor":`)
	j := strings.Index(e[i:], "},") + i + 2
	e = e[:i] + e[j:]
	raw := wrapProjection(e)
	p, err := ParseProjection(raw)
	if err != nil {
		t.Fatalf("ParseProjection: %v\nevent:\n%s", err, e)
	}
	if p.Events[0].Anchor != nil {
		t.Errorf("Anchor = %+v, want nil", p.Events[0].Anchor)
	}
}

func TestParseProjection_Refusals(t *testing.T) {
	cases := []struct {
		name string
		raw  []byte
	}{
		{"unknown top-level field", []byte(`{"schemaVersion":1,"delivery":"pending","bogus":true,"events":[]}`)},
		{"unsupported schema version", []byte(`{"schemaVersion":2,"delivery":"pending","events":[]}`)},
		{"unsupported target", []byte(`{"schemaVersion":1,"delivery":"pending","target":"other-system","events":[]}`)},
		{"trailing data", []byte(`{"schemaVersion":1,"delivery":"pending","events":[]}{}`)},
		{"event unknown field", wrapProjection(strings.Replace(validEventJSON("bad1"), `"sourceStatus": "open",`, `"sourceStatus": "open", "bogus": 1,`, 1))},
		{"event bad kind", wrapProjection(strings.Replace(validEventJSON("bad2"), `"kind": "observation"`, `"kind": "sighting"`, 1))},
		{"event bad findingId", wrapProjection(strings.Replace(validEventJSON("bad3"), fmt.Sprintf(`"findingId": %q`, hex64("bad3-finding")), `"findingId": "not-hex"`, 1))},
		{"event source version zero", wrapProjection(strings.Replace(validEventJSON("bad4"), `"sourceVersion": 1`, `"sourceVersion": 0`, 1))},
		{"event bad outcome", wrapProjection(strings.Replace(validEventJSON("bad5"), `"outcome": "fail"`, `"outcome": "maybe"`, 1))},
		{"event bad coverage", wrapProjection(strings.Replace(validEventJSON("bad6"), `"coverage": "complete"`, `"coverage": "partial"`, 1))},
		{"event bad observedAt", wrapProjection(strings.Replace(validEventJSON("bad7"), `"observedAt": "2026-09-01T12:00:00Z"`, `"observedAt": "not-a-date"`, 1))},
		{"event anchor path absolute", wrapProjection(strings.Replace(validEventJSON("bad8"), `"path": "internal/foo/bar.go"`, `"path": "/etc/passwd"`, 1))},
		{"event anchor path traversal", wrapProjection(strings.Replace(validEventJSON("bad9"), `"path": "internal/foo/bar.go"`, `"path": "../../etc/passwd"`, 1))},
		{"event bad class", wrapProjection(strings.Replace(validEventJSON("bad10"), `"class": "correctness"`, `"class": "vibes"`, 1))},
		{"event bad tier", wrapProjection(strings.Replace(validEventJSON("bad11"), `"tier": "pre-scan"`, `"tier": "omniscient"`, 1))},
		{"visibility mismatch", wrapProjection(strings.Replace(validEventJSON("bad12"), `"visibility": {"organization": "acme", "repository": "widget", "component": "api"}`, `"visibility": {"organization": "other", "repository": "widget", "component": "api"}`, 1))},
		{"missing rule.closure", wrapProjection(strings.Replace(validEventJSON("bad13"), `"closure": "manual review",`, `"closure": "",`, 1))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseProjection(tc.raw); err == nil {
				t.Errorf("ParseProjection(%s): expected error, got nil", tc.name)
			}
		})
	}
}

func TestParseProjection_AnnotationEvent(t *testing.T) {
	e := strings.Replace(validEventJSON("ann1"), `"kind": "observation"`, `"kind": "annotation"`, 1)
	e = strings.Replace(e, `"sourceStatus": "open",`, `"sourceStatus": "open", "annotation": "looks like a false positive to me",`, 1)
	raw := wrapProjection(e)
	p, err := ParseProjection(raw)
	if err != nil {
		t.Fatalf("ParseProjection: %v", err)
	}
	if p.Events[0].Annotation == "" {
		t.Error("Annotation should be set for an annotation-kind event")
	}
}

func TestParseProjection_AnnotationRequiredOnAnnotationKind(t *testing.T) {
	e := strings.Replace(validEventJSON("ann2"), `"kind": "observation"`, `"kind": "annotation"`, 1)
	raw := wrapProjection(e)
	if _, err := ParseProjection(raw); err == nil {
		t.Error("expected refusal: annotation kind with no annotation text")
	}
}

func TestParseProjection_AnnotationForbiddenOnNonAnnotationKind(t *testing.T) {
	e := strings.Replace(validEventJSON("ann3"), `"sourceStatus": "open",`, `"sourceStatus": "open", "annotation": "stray text",`, 1)
	raw := wrapProjection(e)
	if _, err := ParseProjection(raw); err == nil {
		t.Error("expected refusal: annotation text set on a non-annotation-kind event")
	}
}
