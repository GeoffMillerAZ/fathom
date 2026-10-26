package proseheadless_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	ph "github.com/geoffmilleraz/fathom/third_party/agentic-kit/scenarios/prose-headless"
)

func TestWireShapesMatchCUE(t *testing.T) {
	cue, err := exec.LookPath("cue")
	if err != nil {
		t.Skip("CUE tool unavailable; task cue-vet has the same optional prerequisite")
	}
	h := newHarness(t)
	steady := func(context.Context) (string, error) { return "snapshot-1", nil }
	batch := must(ph.GenerateVariants(context.Background(), h.engine, h.target, []ph.VariantSpec{{ID: "exec", Audience: "executive"}}, ph.Independent, steady, 2))
	q := ph.Question{ID: "q", Kind: "conflict", Subject: "fact-date", SubjectVersion: "v", DocumentDigest: h.doc.Receipt.DocumentDigest, Evidence: []string{"fact-date"}, Prompt: "x", Required: true}
	a := ph.Answer{QuestionID: "q", Outcome: ph.OutcomeDispute, Text: "no", SeenVersion: "v"}
	for name, v := range map[string]any{
		"#Question": q, "#Answer": a, "#Record": ph.Record{Question: q, Answer: a, Actor: "bob", At: epoch},
		"#CriterionObservation": passAll(h.doc)[0], "#Policy": h.policy, "#PublishOp": h.op("internal-brief", "wiki", "acct-1"),
		"#VariantSpec": ph.VariantSpec{ID: "exec"}, "#QueueReport": h.queue.Report(), "#ProjectionDelta": ph.ProjectionDelta{Numerator: 1, Denominator: 2}, "#BatchResult": batch, "#ApplyReport": ph.ApplyReport{},
	} {
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "actual.json")
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		out, err := exec.CommandContext(ctx, cue, "vet", "schema.cue", path, "-d", name).CombinedOutput()
		cancel()
		if err != nil {
			t.Errorf("actual Go JSON violates %s: %v\n%s", name, err, out)
		}
	}
}
