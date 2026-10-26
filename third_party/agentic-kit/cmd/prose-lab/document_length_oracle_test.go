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

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// This deliberately content-free fake proves length routing only. A fake support
// reviewer cannot establish fidelity, usefulness, audience fit or live quality.
// It does not replay, reassess or upgrade the retained real 86-word v1 failure.
type lengthOracleGenerator struct{ firstFields, calls int }

func (g *lengthOracleGenerator) Complete(_ context.Context, r llmclient.Request) (llmclient.Response, error) {
	var p struct {
		Facts []struct{ ID string } `json:"facts"`
	}
	if err := json.Unmarshal([]byte(r.Prompt), &p); err != nil {
		return llmclient.Response{}, err
	}
	ids := make([]string, 0, len(p.Facts))
	for _, f := range p.Facts {
		ids = append(ids, f.ID)
	}
	n := 1
	if g.calls == 0 {
		n = g.firstFields
	}
	g.calls++
	// Include non-ASCII whitespace so the count rule must use Unicode fields.
	text := strings.Repeat("synthetic\u3000", n-1) + "synthetic"
	body := jsonBytes(map[string]any{"segments": []map[string]any{{"kind": "assertion", "text": text, "fact_ids": ids, "contradictions": []string{}, "reason": ""}}})
	return llmclient.Response{Text: string(body), StopReason: llmclient.StopEndTurn}, nil
}
func (*lengthOracleGenerator) Stream(context.Context, llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	return nil, nil
}

func TestDocumentLengthCLIOracle(t *testing.T) {
	for _, tc := range []struct {
		name, corpus, document, lengthStatus, errorClass string
		fields                                           int
		empty                                            bool
		code                                             int
	}{
		{"v1-86-remains-held", "document-corpus", "freeform-brief", "", "coverage_or_length_held", 86, false, 2},
		{"v2-86-advisory-warning", "document-corpus-v2", "freeform-brief", "warning", "", 86, false, 0},
		{"v2-86-required-long-hold", "document-corpus-v2", "technical-design", "hold", "required_length_held", 86, false, 2},
		{"v2-advisory-pass", "document-corpus-v2", "freeform-brief", "pass", "", 150, false, 0},
		{"v2-required-pass", "document-corpus-v2", "technical-design", "pass", "", 1200, false, 0},
		{"v2-over-target-warning", "document-corpus-v2", "freeform-brief", "warning", "", 201, false, 0},
		{"v2-early-hold", "document-corpus-v2", "freeform-brief", "", "generation_or_support_held", 0, true, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			corpus := filepath.Join("testdata", tc.corpus)
			c, err := loadDocumentCorpus(corpus)
			if err != nil {
				t.Fatal(err)
			}
			cell := tc.document + "-technical-r1"
			args := documentArgs(documentTestStudy(t, c), corpus, cell)
			if c.Manifest.Version == 2 {
				args = append(args, "--goal-scope", "testdata/document-scopes/v2-length-goals.json", "--goal-scope-digest", frozenV2GoalScope)
			}
			var prepared, errOut bytes.Buffer
			if code := runDocument(context.Background(), args, &prepared, &errOut); code != 0 {
				t.Fatalf("prepare: %d %s", code, errOut.String())
			}
			var p map[string]any
			if err := json.Unmarshal(prepared.Bytes(), &p); err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			artifacts := filepath.Join(root, "artifacts")
			capture := append([]string{}, args...)
			capture[0] = "document-capture"
			capture = append(capture, "--prepared-digest", p["prepared_digest"].(string), "--live", "--journal", filepath.Join(root, "journal"), "--artifacts", artifacts)
			var gen llmclient.LLMClient = &lengthOracleGenerator{firstFields: tc.fields - (len(c.Fixtures[tc.document].Sections) - 1)}
			if tc.empty {
				gen = &documentSpy{}
			}
			now := func() time.Time { return time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC) }
			var out bytes.Buffer
			errOut.Reset()
			code := runDocumentWith(context.Background(), capture, &out, &errOut, documentDeps{generation: gen, review: &documentSupport{}, now: now})
			if code != tc.code {
				t.Fatalf("capture: %d want=%d %s", code, tc.code, errOut.String())
			}
			file := filepath.Join(artifacts, cell+".receipt.json")
			b := mustReadDocumentTest(t, file)
			var receipt documentRunReceipt
			if err := json.Unmarshal(b, &receipt); err != nil {
				t.Fatal(err)
			}
			if receipt.ErrorClass != tc.errorClass || receipt.LengthStatus != tc.lengthStatus || receipt.PublicationAuthority || receipt.IndependentAcceptance || receipt.NegativeCasesEvaluated {
				t.Fatalf("dishonest receipt: %s", b)
			}
			if !tc.empty {
				md := mustReadDocumentTest(t, filepath.Join(artifacts, cell+".md"))
				if got := len(strings.Fields(string(md))); got != tc.fields {
					t.Fatalf("oracle produced %d fields, want%d", got, tc.fields)
				}
			}
			if c.Manifest.Version == 2 {
				if receipt.SemanticQuality != "unassessed" {
					t.Fatal("length substituted for semantic quality")
				}
				if !tc.empty {
					deviation := 0
					if tc.fields < receipt.LengthMinWords {
						deviation = tc.fields - receipt.LengthMinWords
					} else if tc.fields > receipt.LengthMaxWords {
						deviation = tc.fields - receipt.LengthMaxWords
					}
					if receipt.LengthWordCount != tc.fields || receipt.LengthDeviation == nil || *receipt.LengthDeviation != deviation {
						t.Fatal("count or deviation not measured exactly")
					}
					goalsBytes := mustReadDocumentTest(t, filepath.Join(artifacts, cell+".goals.json"))
					var side documentGoalsSidecar
					if err := json.Unmarshal(goalsBytes, &side); err != nil || hashBytes(jsonBytes(side)) != receipt.GoalsDigest {
						t.Fatal("private assessment is not bound to receipt")
					}
					for _, g := range side.Goals[:len(side.Goals)-1] {
						if g.Result.RequiredStatus != "unknown" || len(g.Observations) != 0 {
							t.Fatal("fake length observation assessed semantic quality")
						}
					}
				}
				if cueBin, err := exec.LookPath("cue"); err == nil {
					cmd := exec.Command(cueBin, "vet", "schema.cue", file, "-d", "#DocumentRunReceiptV2", "-c")
					if output, err := cmd.CombinedOutput(); err != nil {
						t.Fatalf("actual v2 receipt violates CUE: %v %s", err, output)
					}
					if !tc.empty {
						for _, fault := range []string{"wrong-count-state", "wrong-deviation"} {
							var forged map[string]any
							if err := json.Unmarshal(b, &forged); err != nil {
								t.Fatal(err)
							}
							if fault == "wrong-deviation" {
								forged["length_deviation"] = *receipt.LengthDeviation + 1
							} else {
								forged["length_word_count"] = receipt.LengthMinWords
								if receipt.LengthStatus == "pass" {
									forged["length_word_count"] = 0
								}
								forged["length_deviation"] = 0
							}
							bad := filepath.Join(root, fault+".json")
							if err := os.WriteFile(bad, jsonBytes(forged), 0600); err != nil {
								t.Fatal(err)
							}
							if output, err := exec.Command(cueBin, "vet", "schema.cue", bad, "-d", "#DocumentRunReceiptV2", "-c").CombinedOutput(); err == nil {
								t.Fatalf("CUE admitted %s: %s", fault, output)
							}
						}
					}
				}
			}
			if st, err := os.Stat(file); err != nil || st.Mode().Perm()&0077 != 0 {
				t.Fatal("receipt is not private")
			}
		})
	}
}
