package catalogkit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

func textDigest(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func decisionFixture(t *testing.T) (Bundle, Binding, []byte, []Evidence) {
	t.Helper()
	s := fixture()
	s.Layers[0].Nodes[0].Children[0].Hidden = false
	b, err := Resolve(s, Policy{})
	if err != nil {
		t.Fatal(err)
	}
	n, _ := b.Catalog.Get("review/evidence")
	definition := []byte("A reference is supported only when the passage cites a captured run.\nPreserve this exact definition.")
	return b, Binding{NodeID: n.ID(), NodeDigest: NodeDigest(n), DefinitionDigest: textDigest(string(definition)), Version: "1", Question: decision.Question{ID: "review.evidence", Kind: decision.KindChoice, Instructions: "Does this passage cite captured evidence?", Options: []decision.Option{{Name: "supported", Criteria: "An exact evidence reference is present."}, {Name: "unknown", Criteria: "The supplied evidence cannot establish the answer."}}}, RequiredEvidence: []string{"passage"}}, definition, []Evidence{{ID: "passage", Source: "docs/review.md", Revision: "fixture-revision", Text: "The report links to the captured run.", Digest: textDigest("The report links to the captured run.")}}
}

func TestPrepareDecisionBindsEvidenceAndVersion(t *testing.T) {
	b, bind, definition, ev := decisionFixture(t)
	p, err := PrepareDecision(b, bind, definition, ev, 4096)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Request.Questions) != 1 || !strings.Contains(p.Request.State, ev[0].Text) {
		t.Fatal("packet incomplete")
	}
	var state struct {
		Definition string `json:"definition"`
	}
	if err := json.Unmarshal([]byte(p.Request.State), &state); err != nil || state.Definition != string(definition) || p.DefinitionDigest != bind.DefinitionDigest {
		t.Fatal("definition missing or altered in sealed packet")
	}
	id := DecisionIdentity{Provider: "fixture", RequestedModel: "configured", ResolvedModel: "pinned", AdapterVersion: "1", PolicyVersion: "1"}
	k, err := DecisionCacheKey(p, id)
	if err != nil {
		t.Fatal(err)
	}
	bind.Version = "2"
	other, err := PrepareDecision(b, bind, definition, ev, 4096)
	if err != nil {
		t.Fatal(err)
	}
	if next, _ := DecisionCacheKey(other, id); k == next {
		t.Fatal("definition version omitted from key")
	}
	id.ResolvedModel = "new"
	if next, _ := DecisionCacheKey(p, id); k == next {
		t.Fatal("model omitted from key")
	}
	if _, err := DecisionCacheKey(p, DecisionIdentity{}); err == nil {
		t.Fatal("unscoped cache key accepted")
	}
	// Returned question slices must not alias the definition.
	p.Request.Questions[0].Options[0].Name = "changed"
	if bind.Question.Options[0].Name != "supported" {
		t.Fatal("question alias")
	}
	if _, err := DecisionCacheKey(p, id); err == nil {
		t.Fatal("mutated prepared request accepted")
	}
}

func TestPrepareDecisionRefusesUnestablishedContext(t *testing.T) {
	for _, name := range []string{"missing", "empty", "corrupt", "duplicate", "extra", "stale", "hidden", "budget", "no-unknown"} {
		t.Run(name, func(t *testing.T) {
			b, bind, definition, ev := decisionFixture(t)
			cap := 4096
			switch name {
			case "missing":
				ev = nil
			case "empty":
				ev[0].Text = ""
				ev[0].Digest = textDigest("")
			case "corrupt":
				ev[0].Digest = strings.Repeat("0", 64)
			case "duplicate":
				ev = append(ev, ev[0])
			case "extra":
				ev = append(ev, Evidence{ID: "other", Text: "other", Digest: textDigest("other")})
			case "stale":
				bind.NodeDigest = strings.Repeat("0", 64)
			case "hidden":
				b.Catalog.Nodes[1].Hidden = true
				b.Digest = bundleDigest(b)
			case "budget":
				cap = 1
			case "no-unknown":
				bind.Question.Options[1].Name = "other"
			}
			if _, err := PrepareDecision(b, bind, definition, ev, cap); err == nil {
				t.Fatal("unestablished context accepted")
			}
		})
	}
}

func TestPrepareDecisionBindsExactDefinition(t *testing.T) {
	b, bind, definition, ev := decisionFixture(t)
	id := DecisionIdentity{Provider: "fixture", RequestedModel: "configured", ResolvedModel: "pinned", AdapterVersion: "1", PolicyVersion: "1"}
	first, err := PrepareDecision(b, bind, definition, ev, 4096)
	if err != nil {
		t.Fatal(err)
	}
	changed := []byte("A reference is supported only when the passage cites a captured run.\nThis criterion now requires a second run.")
	if _, err := PrepareDecision(b, bind, changed, ev, 4096); err == nil {
		t.Fatal("changed criterion accepted with stale binding")
	}
	bind.DefinitionDigest = textDigest(string(changed))
	second, err := PrepareDecision(b, bind, changed, ev, 4096)
	if err != nil {
		t.Fatal(err)
	}
	firstKey, err := DecisionCacheKey(first, id)
	if err != nil {
		t.Fatal(err)
	}
	secondKey, err := DecisionCacheKey(second, id)
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest == second.Digest || firstKey == secondKey || first.DefinitionDigest == second.DefinitionDigest {
		t.Fatal("changed criterion did not change sealed identity and cache key")
	}
	second.DefinitionDigest = first.DefinitionDigest
	if _, err := DecisionCacheKey(second, id); err == nil {
		t.Fatal("mutated prepared definition digest accepted")
	}
}

func TestPrepareDecisionRefusesInvalidDefinition(t *testing.T) {
	for _, tc := range []struct {
		name       string
		definition []byte
	}{
		{"missing", nil},
		{"blank", []byte(" \n\t")},
		{"invalid UTF-8", []byte{0xff}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, bind, _, ev := decisionFixture(t)
			bind.DefinitionDigest = textDigest(string(tc.definition))
			if _, err := PrepareDecision(b, bind, tc.definition, ev, 4096); err == nil {
				t.Fatal("invalid definition accepted")
			}
		})
	}
	b, bind, definition, ev := decisionFixture(t)
	bind.DefinitionDigest = ""
	if _, err := PrepareDecision(b, bind, definition, ev, 4096); err == nil {
		t.Fatal("missing definition digest accepted")
	}
}

func TestPrepareDecisionDefinitionCountsTowardByteLimit(t *testing.T) {
	b, bind, definition, ev := decisionFixture(t)
	p, err := PrepareDecision(b, bind, definition, ev, 4096)
	if err != nil {
		t.Fatal(err)
	}
	packet, err := json.Marshal(p.Request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareDecision(b, bind, definition, ev, len(packet)); err != nil {
		t.Fatalf("exact packet limit refused: %v", err)
	}
	if _, err := PrepareDecision(b, bind, definition, ev, len(packet)-1); err == nil {
		t.Fatal("packet exceeding limit accepted")
	}
	largeDefinition := []byte(strings.Repeat("d", 1<<20))
	bind.DefinitionDigest = textDigest(string(largeDefinition))
	if _, err := PrepareDecision(b, bind, largeDefinition, ev, 4096); err == nil {
		t.Fatal("oversized direct-call definition accepted")
	}
	b, bind, definition, ev = decisionFixture(t)
	ev[0].Text = strings.Repeat("e", 1<<20)
	ev[0].Digest = textDigest(ev[0].Text)
	if _, err := PrepareDecision(b, bind, definition, ev, 4096); err == nil {
		t.Fatal("oversized direct-call evidence accepted")
	}
}

func TestVerifyBundleDetectsMutation(t *testing.T) {
	b, _, _, _ := decisionFixture(t)
	b.Catalog.Nodes[0].Provenance.Overridden = true
	if VerifyBundle(b) == nil {
		t.Fatal("bundle provenance mutation went undetected")
	}
}

func TestPrepareBoundsEveryQuestionFieldAndNodeMetadata(t *testing.T) {
	for _, field := range []string{"rubric", "true", "false", "kind", "synopsis", "facet"} {
		t.Run(field, func(t *testing.T) {
			b, bind, definition, ev := decisionFixture(t)
			huge := strings.Repeat("x", 100000)
			switch field {
			case "rubric":
				bind.Question.Rubric = huge
			case "true":
				bind.Question.TrueCriteria = huge
			case "false":
				bind.Question.FalseCriteria = huge
			case "kind":
				bind.Question.Kind = decision.Kind(huge)
			case "synopsis":
				b.Catalog.Nodes[1].Synopsis = huge
			case "facet":
				b.Catalog.Nodes[1].Facets["description"] = []string{huge}
			}
			b.Digest = bundleDigest(b)
			bind.NodeDigest = NodeDigest(b.Catalog.Nodes[1])
			if _, err := PrepareDecision(b, bind, definition, ev, 4096); err == nil || !strings.Contains(err.Error(), "byte limit") {
				t.Fatalf("expected preflight byte refusal: %v", err)
			}
		})
	}
}

func TestPrepareBoundsEmptyCollectionOverhead(t *testing.T) {
	for _, field := range []string{"evidence", "options", "levels", "recipe"} {
		t.Run(field, func(t *testing.T) {
			b, bind, definition, ev := decisionFixture(t)
			switch field {
			case "evidence":
				ev = make([]Evidence, 10000)
			case "options":
				bind.Question.Options = make([]decision.Option, 10000)
			case "levels":
				bind.Question.Levels = make([]string, 10000)
			case "recipe":
				bind.RequiredEvidence = make([]string, 10000)
			}
			if _, err := PrepareDecision(b, bind, definition, ev, 1024); err == nil || !strings.Contains(err.Error(), "byte limit") {
				t.Fatalf("unbounded collection accepted: %v", err)
			}
		})
	}
}
