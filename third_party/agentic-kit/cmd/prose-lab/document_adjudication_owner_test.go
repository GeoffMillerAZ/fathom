package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	fitness "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/fitness-function-registry"
)

// These deliberately meaningless generated words prove review plumbing only.
// Declared fixture reviewers are not real human review or semantic acceptance.
type ownerDAGenerator struct{ inner lengthOracleGenerator }

func (g *ownerDAGenerator) Complete(ctx context.Context, r llmclient.Request) (llmclient.Response, error) {
	v, err := g.inner.Complete(ctx, r)
	v.Text = strings.ReplaceAll(v.Text, "synthetic", "OWNER_ARTIFACT_SENTINEL")
	return v, err
}
func (*ownerDAGenerator) Stream(context.Context, llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	return nil, errors.New("unused offline stream")
}

type ownerDAFixture struct {
	args            []string
	artifacts, cell string
	receipt         documentRunReceipt
	side            documentGoalsSidecar
}

func ownerDASHA(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func ownerDABytes(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func ownerDAFixtureAt(t *testing.T, document string, fields int) ownerDAFixture {
	t.Helper()
	corpus := "testdata/document-corpus-v2"
	c, err := loadDocumentCorpus(corpus)
	if err != nil {
		t.Fatal(err)
	}
	study := documentTestStudy(t, c)
	cell := document + "-technical-r1"
	base := append(documentArgs(study, corpus, cell)[1:], "--goal-scope", "testdata/document-scopes/v2-length-goals.json", "--goal-scope-digest", frozenV2GoalScope)
	var prepared, errOut bytes.Buffer
	if code := runDocument(context.Background(), append([]string{"document-prepare"}, base...), &prepared, &errOut); code != 0 {
		t.Fatalf("capture preparation %d %s", code, errOut.String())
	}
	var pin map[string]any
	if err := json.Unmarshal(prepared.Bytes(), &pin); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	artifacts := filepath.Join(root, "artifacts")
	capture := append(append([]string{"document-capture"}, base...), "--prepared-digest", pin["prepared_digest"].(string), "--live", "--journal", filepath.Join(root, "journal"), "--artifacts", artifacts)
	now := func() time.Time { return time.Date(2026, 10, 25, 18, 0, 0, 0, time.UTC) }
	var out bytes.Buffer
	errOut.Reset()
	g := &ownerDAGenerator{inner: lengthOracleGenerator{firstFields: fields - len(c.Fixtures[document].Sections) + 1}}
	code := runDocumentWith(context.Background(), capture, &out, &errOut, documentDeps{generation: g, review: &documentSupport{}, now: now})
	if code != 0 && !(document == "technical-design" && code == 2) {
		t.Fatalf("offline capture %d %s", code, errOut.String())
	}
	raw := mustReadDocumentTest(t, filepath.Join(artifacts, cell+".receipt.json"))
	f := ownerDAFixture{args: append(append([]string{}, base...), "--artifacts", artifacts, "--receipt-digest", ownerDASHA(raw), "--at", "2026-10-25T18:30:00Z"), artifacts: artifacts, cell: cell}
	if err := json.Unmarshal(raw, &f.receipt); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(mustReadDocumentTest(t, filepath.Join(artifacts, cell+".goals.json")), &f.side); err != nil {
		t.Fatal(err)
	}
	if len(f.side.Goals) != 6 {
		t.Fatal("owner expected five semantic goals plus length")
	}
	return f
}

func ownerDAInvoke(t *testing.T, command string, base []string, extra ...string) (int, map[string]any, []byte, string) {
	t.Helper()
	args := append(append(append([]string{}, command), base...), extra...)
	var out, errOut bytes.Buffer
	code := runLab(context.Background(), args, &out, &errOut)
	var m map[string]any
	if out.Len() != 0 {
		if err := json.Unmarshal(out.Bytes(), &m); err != nil {
			t.Fatalf("non-JSON review output: %v", err)
		}
	}
	return code, m, append([]byte{}, out.Bytes()...), errOut.String()
}
func ownerDAReplace(base []string, flag, value string) []string {
	out := append([]string{}, base...)
	for i := range out {
		if out[i] == flag {
			out[i+1] = value
			return out
		}
	}
	panic("owner test flag missing")
}
func ownerDAInput(t *testing.T, f ownerDAFixture, packet, kind, relationship, method string) map[string]any {
	t.Helper()
	obs := []fitness.Observation{}
	for _, goal := range f.side.Goals[:5] {
		criterion := goal.Profile.Criteria[0]
		obs = append(obs, fitness.Observation{CriterionID: criterion.ID, DefinitionDigest: criterion.DefinitionDigest, SubjectDigest: goal.Result.SubjectDigest, Outcome: fitness.FindingPass, Method: fitness.EvidenceMethod(method), Reason: "OWNER_REASON_SENTINEL", ArtifactDigests: []string{f.receipt.DocumentDigest, f.receipt.MarkdownDigest}, ObservedAt: time.Date(2026, 10, 25, 18, 15, 0, 0, time.UTC), ValidUntil: time.Date(2026, 10, 25, 19, 0, 0, 0, time.UTC)})
	}
	return map[string]any{"version": "prose-lab.document-adjudication/1", "packet_digest": packet, "reviewer": map[string]any{"id": "owner-private-reviewer-sentinel", "kind": kind, "relationship": relationship}, "observations": obs}
}
func ownerDAReport(t *testing.T, f ownerDAFixture, packet string, input any, extra ...string) (int, map[string]any, []byte, string) {
	t.Helper()
	args := []string{"--packet-digest", packet}
	if input != nil {
		b := ownerDABytes(t, input)
		path := filepath.Join(t.TempDir(), "review.json")
		if err := os.WriteFile(path, b, 0600); err != nil {
			t.Fatal(err)
		}
		args = append(args, "--adjudication", path, "--adjudication-digest", ownerDASHA(b))
	}
	return ownerDAInvoke(t, "document-review-report", f.args, append(args, extra...)...)
}
func ownerDAVet(t *testing.T, definition string, b []byte) {
	t.Helper()
	bin, err := exec.LookPath("cue")
	if err != nil {
		t.Skip("CUE unavailable; actual wire conformance untested")
	}
	p := filepath.Join(t.TempDir(), "actual.json")
	if err := os.WriteFile(p, b, 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(bin, "vet", "-c", "schema.cue", p, "-d", definition).CombinedOutput(); err != nil {
		t.Fatalf("actual %s violates CUE: %v %s", definition, err, out)
	}
}
func ownerDASnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]string{}
	for _, e := range entries {
		if e.IsDir() {
			m[e.Name()] = "directory"
			continue
		}
		m[e.Name()] = ownerDASHA(mustReadDocumentTest(t, filepath.Join(root, e.Name())))
	}
	return m
}
func ownerDAAuthority(t *testing.T, m map[string]any, b []byte) {
	t.Helper()
	if m["trust"] != "caller-declared" || m["identity_verification"] != "not_performed" || m["independent_acceptance"] != false || m["publication_authority"] != false {
		t.Fatalf("dishonest review authority: %s", b)
	}
	for _, hidden := range []string{"OWNER_ARTIFACT_SENTINEL", "OWNER_REASON_SENTINEL", "owner-private-reviewer-sentinel"} {
		if bytes.Contains(b, []byte(hidden)) {
			t.Fatalf("default output leaked %s", hidden)
		}
	}
}

func TestOwnerDocumentAdjudicationDeclaredEvidence(t *testing.T) {
	f := ownerDAFixtureAt(t, "freeform-brief", 150)
	before := ownerDASnapshot(t, f.artifacts)
	code, p, b, errOut := ownerDAInvoke(t, "document-review-prepare", f.args)
	if code != 0 {
		t.Fatalf("review prepare %d %s", code, errOut)
	}
	ownerDAAuthority(t, p, b)
	ownerDAVet(t, "#DocumentReviewPacket", b)
	packet := p["packet_digest"].(string)
	if len(p["semantic_goals"].([]any)) != 5 {
		t.Fatal("semantic roster shortened")
	}
	code, pLater, _, errOut := ownerDAInvoke(t, "document-review-prepare", ownerDAReplace(f.args, "--at", "2026-10-25T18:31:00Z"))
	if code != 0 || pLater["packet_digest"] != packet {
		t.Fatalf("evaluation time changed immutable packet: %d %s", code, errOut)
	}
	code, content, contentBytes, errOut := ownerDAInvoke(t, "document-review-prepare", f.args, "--include-content")
	if code != 0 || content["packet_digest"] != packet || !bytes.Contains(contentBytes, []byte("OWNER_ARTIFACT_SENTINEL")) {
		t.Fatalf("explicit content projection %d %s", code, errOut)
	}
	ownerDAVet(t, "#DocumentReviewPacket", contentBytes)

	for _, tc := range []struct {
		name, kind, relationship, method, status string
		missing                                  bool
		code                                     int
	}{
		{"missing", "", "", "", "unknown", true, 2},
		{"declared-human", "human", "independent", "human", "pass", false, 0},
		{"agent-heuristic", "agent", "independent", "heuristic", "unknown", false, 2},
		{"human-producer", "human", "producer", "human", "unknown", false, 2},
		{"human-unknown-independence", "human", "unknown", "human", "unknown", false, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var input any
			if !tc.missing {
				input = ownerDAInput(t, f, packet, tc.kind, tc.relationship, tc.method)
			}
			code, report, out, errOut := ownerDAReport(t, f, packet, input)
			if code != tc.code || report["declared_goals_status"] != tc.status || report["declared_criteria_status"] != tc.status {
				t.Fatalf("declared status %d %s %s", code, errOut, out)
			}
			ownerDAAuthority(t, report, out)
			ownerDAVet(t, "#DocumentReviewReport", out)
		})
	}
	for _, fault := range []string{"agent-as-human", "null-observations", "foreign-goal", "duplicate-goal", "wrong-packet"} {
		t.Run(fault, func(t *testing.T) {
			input := ownerDAInput(t, f, packet, "human", "independent", "human")
			rows := input["observations"].([]fitness.Observation)
			switch fault {
			case "agent-as-human":
				input["reviewer"].(map[string]any)["kind"] = "agent"
			case "null-observations":
				input["observations"] = nil
			case "foreign-goal":
				rows[0].CriterionID = "unselected-goal"
			case "duplicate-goal":
				rows[1] = rows[0]
			case "wrong-packet":
				input["packet_digest"] = strings.Repeat("0", 64)
			}
			code, _, out, _ := ownerDAReport(t, f, packet, input)
			if code != 2 || len(out) != 0 {
				t.Fatalf("invalid declaration emitted data: %d %s", code, out)
			}
		})
	}
	for _, fault := range []string{"stale", "future", "wrong-subject", "wrong-definition", "not-applicable", "known-failure"} {
		t.Run(fault, func(t *testing.T) {
			input := ownerDAInput(t, f, packet, "human", "independent", "human")
			rows := input["observations"].([]fitness.Observation)
			want, codeWant := "unknown", 2
			switch fault {
			case "stale":
				rows[0].ValidUntil = time.Date(2026, 10, 25, 18, 20, 0, 0, time.UTC)
			case "future":
				rows[0].ObservedAt = time.Date(2026, 10, 25, 18, 45, 0, 0, time.UTC)
			case "wrong-subject":
				rows[0].SubjectDigest = strings.Repeat("0", 64)
			case "wrong-definition":
				rows[0].DefinitionDigest = strings.Repeat("0", 64)
			case "not-applicable":
				rows[0].Outcome = fitness.FindingNotApplicable
			case "known-failure":
				rows[0].Outcome = fitness.FindingFail
				want, codeWant = "fail", 1
			}
			code, report, out, errOut := ownerDAReport(t, f, packet, input)
			if code != codeWant || report["declared_criteria_status"] != want {
				t.Fatalf("invalidated observation %d %s %s", code, errOut, out)
			}
			ownerDAAuthority(t, report, out)
			ownerDAVet(t, "#DocumentReviewReport", out)
		})
	}
	if after := ownerDASnapshot(t, f.artifacts); !reflect.DeepEqual(before, after) {
		t.Fatal("read-only review changed original artifacts")
	}
}

func TestOwnerDocumentAdjudicationHardLengthCannotBeDeclaredAway(t *testing.T) {
	f := ownerDAFixtureAt(t, "technical-design", 86)
	before := ownerDASnapshot(t, f.artifacts)
	code, packet, out, errOut := ownerDAInvoke(t, "document-review-prepare", f.args)
	if code != 0 || packet["candidate_status"] != "held" {
		t.Fatalf("complete held candidate not inspectable: %d %s %s", code, errOut, out)
	}
	input := ownerDAInput(t, f, packet["packet_digest"].(string), "human", "independent", "human")
	code, report, out, errOut := ownerDAReport(t, f, packet["packet_digest"].(string), input)
	if code != 1 || report["declared_goals_status"] != "pass" || report["declared_criteria_status"] != "fail" {
		t.Fatalf("declared semantic pass cleared required length: %d %s %s", code, errOut, out)
	}
	ownerDAAuthority(t, report, out)
	ownerDAVet(t, "#DocumentReviewReport", out)
	if !reflect.DeepEqual(before, ownerDASnapshot(t, f.artifacts)) {
		t.Fatal("held artifact or receipt was rewritten")
	}
}

type ownerDAWriteFailure struct{}

func (ownerDAWriteFailure) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestOwnerDocumentAdjudicationReadOnlyAndOutputFailures(t *testing.T) {
	f := ownerDAFixtureAt(t, "freeform-brief", 150)
	before := ownerDASnapshot(t, f.artifacts)
	entries, err := os.ReadDir(f.artifacts)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if err := os.Chmod(filepath.Join(f.artifacts, entry.Name()), 0400); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(f.artifacts, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(f.artifacts, 0700) })
	if code, _, _, errOut := ownerDAInvoke(t, "document-review-prepare", f.args); code != 0 {
		t.Fatalf("private immutable evidence refused: %d %s", code, errOut)
	}
	args := append([]string{"document-review-prepare"}, f.args...)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out, errOut bytes.Buffer
	if code := runLab(ctx, args, &out, &errOut); code != 2 || out.Len() != 0 {
		t.Fatal("canceled review emitted a packet")
	}
	errOut.Reset()
	if code := runLab(context.Background(), args, ownerDAWriteFailure{}, &errOut); code != 2 {
		t.Fatal("output failure reported success")
	}
	if code, _, out, _ := ownerDAInvoke(t, "document-review-prepare", f.args, "--adjudication="); code != 2 || len(out) != 0 {
		t.Fatal("prepare accepted an explicitly empty report-only flag")
	}
	if !reflect.DeepEqual(before, ownerDASnapshot(t, f.artifacts)) {
		t.Fatal("review mutated read-only artifacts")
	}
}

func TestOwnerDocumentAdjudicationHelpIsDiscoverable(t *testing.T) {
	for _, command := range []string{"document-review-prepare", "document-review-report"} {
		for _, flag := range []string{"--help", "-h", "help"} {
			var out, errOut bytes.Buffer
			if code := runLab(context.Background(), []string{command, flag}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "--receipt-digest") || !strings.Contains(out.String(), "--at") || !strings.Contains(out.String(), "caller-declared") {
				t.Fatalf("help unavailable without private inputs: %d %s", code, errOut.String())
			}
		}
	}
	var out, errOut bytes.Buffer
	if code := runLab(context.Background(), []string{"--help"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "document-review-prepare|document-review-report") {
		t.Fatal("root help hides document review")
	}
	out.Reset()
	if code := runLab(context.Background(), []string{"document-review-prepare", "--unknown-owner-secret", "--help"}, &out, &errOut); code != 2 || out.Len() != 0 || strings.Contains(errOut.String(), "unknown-owner-secret") {
		t.Fatal("help masked unknown input or echoed a private flag")
	}
}

func TestOwnerDocumentAdjudicationCUECannotOverstate(t *testing.T) {
	bin, err := exec.LookPath("cue")
	if err != nil {
		t.Skip("CUE unavailable; refusal parity untested")
	}
	reject := func(t *testing.T, definition string, value any) {
		t.Helper()
		p := filepath.Join(t.TempDir(), "forged.json")
		if err := os.WriteFile(p, ownerDABytes(t, value), 0600); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command(bin, "vet", "-c", "schema.cue", p, "-d", definition).CombinedOutput(); err == nil {
			t.Fatalf("CUE accepted contradictory %s: %s", definition, out)
		}
	}
	f := ownerDAFixtureAt(t, "technical-design", 86)
	code, packet, _, errOut := ownerDAInvoke(t, "document-review-prepare", f.args)
	if code != 0 {
		t.Fatalf("prepare %d %s", code, errOut)
	}
	pin := packet["packet_digest"].(string)
	t.Run("agent-as-human", func(t *testing.T) {
		reject(t, "#DocumentAdjudication", ownerDAInput(t, f, pin, "agent", "independent", "human"))
	})
	t.Run("required-length-held-but-criteria-pass", func(t *testing.T) {
		code, report, _, errOut := ownerDAReport(t, f, pin, ownerDAInput(t, f, pin, "human", "independent", "human"))
		if code != 1 {
			t.Fatalf("report %d %s", code, errOut)
		}
		report["declared_criteria_status"] = "pass"
		reject(t, "#DocumentReviewReport", report)
	})
}

func TestOwnerDocumentAdjudicationSidecarCannotChoosePolicy(t *testing.T) {
	f := ownerDAFixtureAt(t, "freeform-brief", 150)
	// A coordinated, freshly pinned sidecar and receipt forgery remains structurally
	// self-consistent. Only reconstruction from the independent fixture catches it.
	f.side.Goals[0].Profile.Criteria[0].AllowedMethods = []fitness.EvidenceMethod{fitness.EvidenceMethodHeuristic}
	var err error
	f.side.Goals[0].Result, err = fitness.AssessProfile(f.side.Goals[0].Profile, f.side.Goals[0].Result.SubjectDigest, nil, f.side.Goals[0].Result.EvaluatedAt)
	if err != nil {
		t.Fatal(err)
	}
	side := ownerDABytes(t, f.side)
	f.receipt.GoalsDigest = ownerDASHA(side)
	receipt := ownerDABytes(t, f.receipt)
	if err := os.WriteFile(filepath.Join(f.artifacts, f.cell+".goals.json"), side, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.artifacts, f.cell+".receipt.json"), receipt, 0600); err != nil {
		t.Fatal(err)
	}
	args := ownerDAReplace(f.args, "--receipt-digest", ownerDASHA(receipt))
	code, _, out, _ := ownerDAInvoke(t, "document-review-prepare", args)
	if code != 2 || len(out) != 0 {
		t.Fatalf("candidate sidecar became authoritative policy: %d %s", code, out)
	}
}
