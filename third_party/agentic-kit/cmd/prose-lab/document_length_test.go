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

func TestDocumentV2ReasonGoCUEParity(t *testing.T) {
	bin, err := exec.LookPath("cue")
	if err != nil {
		t.Skip("cue unavailable in focused environment")
	}
	base, err := os.ReadFile("testdata/document-corpus-v2/architecture-decision.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, class, reason string
		valid               bool
	}{
		{"required-ascii-4096", "required", strings.Repeat("a", 4096), true},
		{"required-multibyte-4096", "required", strings.Repeat("é", 2048), true},
		{"required-ascii-4097", "required", strings.Repeat("a", 4097), false},
		{"required-multibyte-4097", "required", strings.Repeat("é", 2048) + "a", false},
		{"required-ascii-whitespace", "required", " \t\n", false},
		{"required-nbsp", "required", "\u00a0", false},
		{"required-nel", "required", "\u0085", false},
		{"required-ideographic", "required", "\u3000", false},
		{"advisory-empty", "advisory", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var raw map[string]any
			if err := json.Unmarshal(base, &raw); err != nil {
				t.Fatal(err)
			}
			goal := raw["length_goal"].(map[string]any)
			goal["class"], goal["reason"] = tc.class, tc.reason
			b, err := json.Marshal(raw)
			if err != nil {
				t.Fatal(err)
			}
			checkReasonWireParity(t, bin, b, tc.valid)
		})
	}
	var raw map[string]any
	if err := json.Unmarshal(base, &raw); err != nil {
		t.Fatal(err)
	}
	goal := raw["length_goal"].(map[string]any)
	goal["class"], goal["reason"] = "required", "x"
	b, _ := json.Marshal(raw)
	for _, tc := range []struct {
		name  string
		bytes []byte
	}{
		{"missing", bytes.Replace(b, []byte(`,"reason":"x"`), nil, 1)},
		{"null", bytes.Replace(b, []byte(`"reason":"x"`), []byte(`"reason":null`), 1)},
	} {
		t.Run(tc.name, func(t *testing.T) { checkReasonWireParity(t, bin, tc.bytes, false) })
	}
	t.Run("duplicate", func(t *testing.T) {
		dup := bytes.Replace(b, []byte(`"reason":"x"`), []byte(`"reason":"x","reason":"x"`), 1)
		if _, err := decodeDocumentFixture(dup); err == nil {
			t.Fatal("duplicate reason accepted by actual Go intake")
		}
	})
}

func checkReasonWireParity(t *testing.T, cueBin string, fixture []byte, expected bool) {
	t.Helper()
	_, goErr := decodeDocumentFixture(fixture)
	data := filepath.Join(t.TempDir(), "fixture.json")
	if err := os.WriteFile(data, fixture, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(cueBin, "vet", "schema.cue", data, "-d", "#DocumentFixtureV2", "-c")
	output, cueErr := cmd.CombinedOutput()
	if (goErr == nil) != expected || (cueErr == nil) != expected {
		t.Fatalf("expected=%t Go=%v CUE=%v: %s", expected, goErr, cueErr, output)
	}
}

func TestDocumentV2PinnedScopePreflight(t *testing.T) {
	c, err := loadDocumentCorpus("testdata/document-corpus-v2")
	if err != nil {
		t.Fatal(err)
	}
	if c.Manifest.Version != 2 || len(c.Fixtures) != 6 {
		t.Fatal("v2 corpus")
	}
	scopePath := "testdata/document-scopes/v2-length-goals.json"
	scopeBytes, err := os.ReadFile(scopePath)
	if err != nil {
		t.Fatal(err)
	}
	scopeDigest := hashBytes(scopeBytes)
	if _, err = loadDocumentGoalScope(scopePath, scopeDigest, c); err != nil {
		t.Fatal(err)
	}
	study := documentTestStudy(t, c)
	args := append(documentArgs(study, "testdata/document-corpus-v2", c.Manifest.Cells[0].ID), "--goal-scope", scopePath, "--goal-scope-digest", scopeDigest)
	var out, errOut bytes.Buffer
	if code := runDocument(context.Background(), args, &out, &errOut); code != 0 {
		t.Fatalf("prepare %d: %s", code, errOut.String())
	}
	var prepared map[string]any
	if err := json.Unmarshal(out.Bytes(), &prepared); err != nil || prepared["version"] != "prose-lab.document-prepared/2" || prepared["goal_scope_digest"] != scopeDigest || len(prepared["length_goals"].([]any)) != 6 {
		t.Fatal("v2 preparation metadata", err)
	}
	spy := &documentSpy{}
	for _, pair := range [][2]string{{"--goal-scope", scopePath}, {"--goal-scope-digest", scopeDigest}} {
		bad := append(documentArgs(study, "testdata/document-corpus-v2", c.Manifest.Cells[0].ID), pair[0], pair[1])
		if runDocumentWith(context.Background(), bad, &bytes.Buffer{}, &bytes.Buffer{}, documentDeps{generation: spy, review: spy}) == 0 {
			t.Fatal("unpaired scope flag accepted")
		}
	}
	bad := append(documentArgs(study, "testdata/document-corpus", c.Manifest.Cells[0].ID), "--goal-scope=", "--goal-scope-digest=")
	if runDocumentWith(context.Background(), bad, &bytes.Buffer{}, &bytes.Buffer{}, documentDeps{generation: spy, review: spy}) == 0 {
		t.Fatal("v1 accepted explicit empty flags")
	}
	if spy.calls != 0 {
		t.Fatal("preflight made provider call")
	}
	mutated := filepath.Join(t.TempDir(), "scope.json")
	if err := os.WriteFile(mutated, append(append([]byte{}, scopeBytes...), byte(' ')), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadDocumentGoalScope(mutated, scopeDigest, c); err == nil {
		t.Fatal("changed raw scope bytes accepted")
	}
}

func TestDocumentV2LengthAssessmentOffline(t *testing.T) {
	c, err := loadDocumentCorpus("testdata/document-corpus-v2")
	if err != nil {
		t.Fatal(err)
	}
	now := func() time.Time { return time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC) }
	for _, tc := range []struct{ documentID, want string }{{"architecture-decision", "warning"}, {"technical-design", "hold"}} {
		t.Run(tc.documentID, func(t *testing.T) {
			cell, _ := c.cell(tc.documentID + "-technical-r1")
			gen, rev := &documentGenerator{}, &documentSupport{}
			engine, target, err := documentEngine(gen, rev, "gen-model", "rev-model", 256, c.Fixtures[cell.DocumentID], cell, now)
			if err != nil {
				t.Fatal(err)
			}
			v, err := engine.Generate(context.Background(), target)
			if err != nil {
				t.Fatal(err)
			}
			r, err := engine.Render(context.Background(), v)
			if err != nil {
				t.Fatal(err)
			}
			if err = documentStructuralCoverage(c.Fixtures[cell.DocumentID], v); err != nil {
				t.Fatal(err)
			}
			side, status, deviation, err := documentLengthAssess(c.Fixtures[cell.DocumentID], v, r, "41988f2f526415763d64faa7193e477f1370c56bfd2a0e269427dbbf0c004403", now())
			if err != nil || status != tc.want || deviation == 0 || side.WordCount != len(bytes.Fields([]byte(r.Markdown))) {
				t.Fatalf("assessment %s %d: %v", status, deviation, err)
			}
			for _, g := range side.Goals[:len(side.Goals)-1] {
				if g.Result.RequiredStatus != "unknown" || len(g.Observations) != 0 {
					t.Fatal("semantic goal was assessed")
				}
			}
			last := side.Goals[len(side.Goals)-1]
			if last.Result.RequiredStatus != "fail" || len(last.Observations) != 1 {
				t.Fatal("length observation missing")
			}
			r.Markdown += " substituted"
			if _, _, _, err := documentLengthAssess(c.Fixtures[cell.DocumentID], v, r, "41988f2f526415763d64faa7193e477f1370c56bfd2a0e269427dbbf0c004403", now()); err == nil {
				t.Fatal("substituted artifact accepted")
			}
		})
	}
}
