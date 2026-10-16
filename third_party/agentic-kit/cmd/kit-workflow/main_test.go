package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	evidencegrade "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/evidence-grade"
	evidencespan "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/evidence-span"
	aiprovenance "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/provenance/ai-provenance"
	governedprosekit "github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/governed-prose-kit"
)

var fixtureNow = time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)

type unreadableHelpInput struct{}

func (unreadableHelpInput) Read([]byte) (int, error) { panic("help read stdin") }

func TestRunHelpIsDiscoverableWithoutInput(t *testing.T) {
	t.Parallel()
	for _, flag := range []string{"-h", "--help"} {
		for _, tc := range []struct {
			args []string
			want string
		}{
			{[]string{flag}, proseUsage + "\n" + lessonUsage},
			{[]string{"prose", flag}, proseUsage},
			{[]string{"prose", "check", flag}, "usage: kit-workflow prose check < request.json"},
			{[]string{"prose", "export", flag}, "usage: kit-workflow prose export < request.json"},
			{[]string{"lesson", flag}, lessonUsage},
			{[]string{"lesson", "validate", flag}, lessonValidateUsage},
			{[]string{"lesson", "adoption-check", flag}, lessonAdoptionUsage},
		} {
			t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				// Help must not depend on a context, clock, stdin, or files.
				code := run(nil, tc.args, unreadableHelpInput{}, &stdout, &stderr, nil)
				if code != 0 || stdout.String() != tc.want+"\n" || stderr.Len() != 0 {
					t.Fatalf("help: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
				}
			})
		}
		for _, operation := range []string{"validate", "adoption-check"} {
			args := lessonFixtureArgs(t, operation)
			for i := 3; i < len(args); i += 2 {
				args[i] = filepath.Join(t.TempDir(), "absent.json")
			}
			args = append(args, flag)
			var stdout, stderr bytes.Buffer
			if code := run(context.Background(), args, unreadableHelpInput{}, &stdout, &stderr, func() time.Time { panic("help read clock") }); code != 0 || stderr.Len() != 0 {
				t.Fatalf("help opened missing files: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		}
	}
}

func TestRunHelpDoesNotMaskUnknownCommandsOrFilename(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"unknown", "--help"}, {"prose", "unknown", "--help"}, {"lesson", "unknown", "--help"}, {"lesson", "validate", "--lesson", "--help"}} {
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), args, unreadableHelpInput{}, &stdout, &stderr, func() time.Time { return fixtureNow })
		var failure errorOutput
		if err := json.Unmarshal(stdout.Bytes(), &failure); err != nil || code != 2 || failure.Error.Kind != "usage" {
			t.Fatalf("args=%q code=%d output=%s", args, code, stdout.String())
		}
	}
}

func fixtureInput(t *testing.T) input {
	t.Helper()
	value := "12"
	ref := governedprosekit.SourceRef{ID: "source-a", Version: "v1", Digest: governedprosekit.ContentDigest(value)}
	span := evidencespan.Span{
		ID: "fact-a", EntityID: "entity-a", Field: "result", Value: value, Confidence: 1,
		SourceID: ref.ID, RecordID: ref.Version, RecordHash: ref.Digest,
		OccurredAt: fixtureNow.Add(-2 * time.Hour), ObservedAt: fixtureNow.Add(-time.Hour),
		Locator: evidencespan.Locator{Kind: "byte-range", Value: ref.ID, Start: 0, End: len(value) - 1},
		Author:  aiprovenance.NewHuman("fixture-reviewer"),
		Grade:   &evidencegrade.Grade{Tier: evidencegrade.TierDocumentary, Confidence: 1},
	}
	return input{
		Version: inputVersion,
		Sources: []governedprosekit.SourceSnapshot{{Ref: ref, Content: value, MediaType: "text/plain", AcquiredBy: "fixture", AcquiredAt: fixtureNow.Add(-time.Hour), Access: "public", RetainUntil: fixtureNow.Add(time.Hour)}},
		Current: []currentSource{{ID: ref.ID, Ref: ref}},
		Request: governedprosekit.SourcedRequest{Principal: "reader", Topic: "verified facts", Facts: []governedprosekit.Fact{{Span: span, Source: ref, Unit: "items"}}},
		Draft:   governedprosekit.ProseDraft{Claims: []governedprosekit.ProseClaim{{ID: "claim-a", Assertion: value, FactIDs: []string{"fact-a"}}}, Units: []governedprosekit.ProseUnit{{ID: "unit-a", Text: value, Claims: []governedprosekit.ClaimRange{{ClaimID: "claim-a", Start: 0, End: len(value)}}}}},
		Config:  governedprosekit.SourcedConfig{PolicyVersion: "policy/v1", PromptVersion: "prompt/v1", Model: "supplied-draft", Limits: governedprosekit.DefaultProseLimits()},
	}
}

func execute(t *testing.T, args []string, request input) (int, output, errorOutput, string) {
	t.Helper()
	b, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return executeJSON(t, args, b)
}

func executeJSON(t *testing.T, args []string, raw []byte) (int, output, errorOutput, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), args, bytes.NewReader(raw), &stdout, &stderr, func() time.Time { return fixtureNow })
	var success output
	var failure errorOutput
	if code == 0 {
		if err := json.Unmarshal(stdout.Bytes(), &success); err != nil {
			t.Fatalf("success JSON: %v: %s", err, stdout.String())
		}
	} else if err := json.Unmarshal(stdout.Bytes(), &failure); err != nil {
		t.Fatalf("failure JSON: %v: %s", err, stdout.String())
	}
	return code, success, failure, stderr.String()
}

func TestRunCheckValidAndAssertions(t *testing.T) {
	t.Parallel()
	code, result, _, _ := execute(t, []string{"prose", "check"}, fixtureInput(t))
	if code != 0 || result.Artifact == nil || result.Export != nil || result.Artifact.Receipt.Generation != "supplied" {
		t.Fatalf("result: code=%d %+v", code, result)
	}
	if !result.Artifact.Receipt.CheckedAt.Equal(fixtureNow) || !strings.Contains(result.Assertions.SourceBytes, "not independently authenticated") || !strings.Contains(result.Assertions.Publication, "not authenticated publication") {
		t.Fatalf("receipt/assertions: %+v", result)
	}
}

func TestRunCheckAllowsExplicitNonFactualOnly(t *testing.T) {
	t.Parallel()
	request := fixtureInput(t)
	request.Draft = governedprosekit.ProseDraft{Units: []governedprosekit.ProseUnit{{ID: "heading", Text: "Summary", NonFactualReason: "caller reviewed heading"}}}
	request.NonFactual = []string{"Summary"}
	if code, _, failure, _ := execute(t, []string{"prose", "check"}, request); code != 0 {
		t.Fatalf("allowlisted non-factual failed: %+v", failure)
	}
	request.NonFactual = nil
	if code, _, failure, _ := execute(t, []string{"prose", "check"}, request); code == 0 || failure.Error.Kind != "unsupported" {
		t.Fatalf("unreviewed non-factual passed: %+v", failure)
	}
}

func TestRunRejectsRevokedStaleAndForgedClaims(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		mutate func(*input)
		kind   string
	}{
		{"revoked", func(in *input) { in.Sources[0].Revoked = true }, "evidence"},
		{"stale", func(in *input) {
			in.Current[0].Ref = governedprosekit.SourceRef{ID: "source-a", Version: "v2", Digest: governedprosekit.ContentDigest("13")}
		}, "evidence"},
		{"forged claim 14", func(in *input) {
			in.Draft.Claims[0].Assertion, in.Draft.Units[0].Text, in.Draft.Units[0].Claims[0].End = "14", "14", 2
		}, "unsupported"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := fixtureInput(t)
			tc.mutate(&request)
			code, _, failure, _ := execute(t, []string{"prose", "check"}, request)
			if code == 0 || failure.Error.Kind != tc.kind {
				t.Fatalf("code=%d failure=%+v", code, failure)
			}
		})
	}
}

func TestRunExportRedactsRestrictedArtifactAndBindsSidecar(t *testing.T) {
	t.Parallel()
	request := fixtureInput(t)
	request.Audience = "public"
	request.Sources[0].Access = "restricted"
	code, result, failure, _ := execute(t, []string{"prose", "export"}, request)
	if code != 0 || result.Export == nil {
		t.Fatalf("export: code=%d failure=%+v", code, failure)
	}
	if result.Export.Markdown != "[redacted]" || result.Export.RedactedUnits != 1 || len(result.Export.SourceMap) != 0 || strings.Contains(result.Export.Markdown, "12") {
		t.Fatalf("restricted export leaked: %+v", result.Export)
	}
	if err := governedprosekit.VerifyProseExport(*result.Export); err != nil {
		t.Fatalf("sidecar digest invalid: %v", err)
	}
	tampered := *result.Export
	tampered.Markdown = "changed"
	if err := governedprosekit.VerifyProseExport(tampered); err == nil {
		t.Fatal("tampered export verified")
	}
}

func TestRunRejectsStrictJSONAndBounds(t *testing.T) {
	t.Parallel()
	valid := fixtureInput(t)
	raw, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		raw  []byte
	}{
		{"unknown", append(raw[:len(raw)-1], []byte(`,"unknown":true}`)...)},
		{"duplicate", []byte(`{"version":"kit-workflow/prose/v1","version":"kit-workflow/prose/v1"}`)},
		{"case-folded duplicate", bytes.Replace(raw, []byte(`"version":`), []byte(`"Version":"kit-workflow/prose/v1","version":`), 1)},
		{"nested case-folded duplicate", bytes.Replace(raw, []byte(`"principal":`), []byte(`"Principal":"reader","principal":`), 1)},
		{"invalid UTF8", bytes.Replace(raw, []byte("reader"), []byte{0xff}, 1)},
		{"trailing", append(append([]byte(nil), raw...), []byte(` {}`)...)},
		{"nesting", []byte(strings.Repeat("[", maxJSONNesting+2) + strings.Repeat("]", maxJSONNesting+2))},
		{"bound", []byte(`{"version":"` + strings.Repeat("x", maxInputBytes) + `"}`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, _, failure, _ := executeJSON(t, []string{"prose", "check"}, tc.raw)
			if code == 0 || failure.Error.Kind != "invalid" {
				t.Fatalf("code=%d failure=%+v", code, failure)
			}
		})
	}
}

func TestRunCanceledAndUsage(t *testing.T) {
	t.Parallel()
	request, err := json.Marshal(fixtureInput(t))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stdout, stderr bytes.Buffer
	if code := run(ctx, []string{"prose", "check"}, bytes.NewReader(request), &stdout, &stderr, func() time.Time { return fixtureNow }); code == 0 {
		t.Fatal("canceled input passed")
	}
	if code, _, failure, _ := executeJSON(t, []string{"prose", "generate"}, request); code != 2 || failure.Error.Kind != "usage" {
		t.Fatalf("usage code=%d failure=%+v", code, failure)
	}
}

func lessonFixtureArgs(t *testing.T, operation string) []string {
	t.Helper()
	base := filepath.Join("..", "..", "kits", "lesson-transfer", "testdata", "public-synthetic")
	args := []string{"lesson", operation, "--lesson", filepath.Join(base, "lesson.json"), "--reproducer", filepath.Join(base, "reproducer.json"), "--attestations", filepath.Join(base, "attestations.json")}
	if operation == "adoption-check" {
		args = append(args, "--receiver", filepath.Join(base, "receiver.json"), "--assertion", filepath.Join(base, "outcome.json"))
	}
	return args
}

func executeLesson(t *testing.T, args []string) (int, lessonOutput, errorOutput, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), args, strings.NewReader("unused"), &stdout, &stderr, func() time.Time { return fixtureNow })
	var success lessonOutput
	var failure errorOutput
	if code == 0 {
		if err := json.Unmarshal(stdout.Bytes(), &success); err != nil {
			t.Fatalf("lesson success JSON: %v: %s", err, stdout.String())
		}
	} else if err := json.Unmarshal(stdout.Bytes(), &failure); err != nil {
		t.Fatalf("lesson failure JSON: %v: %s", err, stdout.String())
	}
	return code, success, failure, stderr.String()
}

func TestRunLessonValidateAndPendingAssertion(t *testing.T) {
	t.Parallel()
	code, result, failure, _ := executeLesson(t, lessonFixtureArgs(t, "validate"))
	if code != 0 || result.Integrity != "bound" || result.Reproduction != "not-run" || result.Fresh || !strings.Contains(result.Boundary, "no review acceptance") {
		t.Fatalf("validate code=%d result=%+v failure=%+v", code, result, failure)
	}
	code, result, failure, _ = executeLesson(t, lessonFixtureArgs(t, "adoption-check"))
	if code != 0 || result.Reproduction != "asserted-unverified" || result.Fresh || result.Receiver == nil || !strings.Contains(result.Boundary, "remains pending") {
		t.Fatalf("adoption code=%d result=%+v failure=%+v", code, result, failure)
	}
}

func TestRunLessonRejectsAssertionsAndUnsafeFlags(t *testing.T) {
	t.Parallel()
	args := lessonFixtureArgs(t, "adoption-check")
	bad := t.TempDir() + "/outcome.json"
	if err := os.WriteFile(bad, []byte(`{"classification":"pass","evidence":"forged"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	args[len(args)-1] = bad
	if code, _, failure, _ := executeLesson(t, args); code == 0 || failure.Error.Kind != "invalid" {
		t.Fatalf("forged pass code=%d failure=%+v", code, failure)
	}
	args = lessonFixtureArgs(t, "validate")
	args = append(args, "--lesson", "duplicate")
	if code, _, failure, _ := executeLesson(t, args); code != 2 || failure.Error.Kind != "usage" {
		t.Fatalf("duplicate flag code=%d failure=%+v", code, failure)
	}
}

func TestRunLessonRejectsNilClockAndOutsideApplicability(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), lessonFixtureArgs(t, "validate"), strings.NewReader("unused"), &stdout, &stderr, nil); code == 0 {
		t.Fatal("nil lesson clock passed")
	}
	stdout.Reset()
	stderr.Reset()
	if code := run(context.Background(), lessonFixtureArgs(t, "validate"), strings.NewReader("unused"), &stdout, &stderr, func() time.Time { return time.Time{} }); code == 0 {
		t.Fatal("zero lesson clock passed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	stdout.Reset()
	stderr.Reset()
	if code := run(ctx, lessonFixtureArgs(t, "validate"), strings.NewReader("unused"), &stdout, &stderr, func() time.Time { cancel(); return fixtureNow }); code == 0 {
		t.Fatal("clock-triggered cancellation wrote a lesson result")
	}
	args := lessonFixtureArgs(t, "adoption-check")
	bad := t.TempDir() + "/receiver.json"
	if err := os.WriteFile(bad, []byte(`{"workload":"outside","revision":"1111111111111111111111111111111111111111111111111111111111111111","config_digest":"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	args[len(args)-3] = bad
	if code, _, failure, _ := executeLesson(t, args); code == 0 || failure.Error.Kind != "invalid" {
		t.Fatalf("outside applicability code=%d failure=%+v", code, failure)
	}
}
