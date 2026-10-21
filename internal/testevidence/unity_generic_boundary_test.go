package testevidence

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func nativeBoundaryEnvelope(t *testing.T, runner, artifact string, wrapped bool) Run {
	t.Helper()
	value := map[string]any{
		"suite": SuiteRef{Component: "component", Name: "unit", Class: "unit"},
		"key":   "fixture", "verdict": "pass", "env": map[string]any{"runnerClass": runner},
		"cases":  []Case{{ID: "tests/ActorTests.cs::Game.ActorTests.Adds", Name: "Game.ActorTests.Adds", Status: "pass", Duration: "1ms", Attempts: 1}},
		"totals": Totals{Run: 1, Passed: 1},
	}
	if artifact != "" {
		value["artifacts"] = []ArtifactRef{{Name: artifact, Digest: "sha256:declared"}}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if wrapped {
		payload, err := json.Marshal(map[string]any{"predicate": value})
		if err != nil {
			t.Fatal(err)
		}
		raw, err = json.Marshal(map[string]any{"payloadType": "application/vnd.in-toto+json", "payload": base64.StdEncoding.EncodeToString(payload), "tier": "local"})
		if err != nil {
			t.Fatal(err)
		}
	}
	run, err := decodeOneEnvelope(raw)
	if err != nil {
		t.Fatal(err)
	}
	return run
}

func nativeBoundaryStore(t *testing.T) (store.Store, core.SymbolNode) {
	t.Helper()
	st := store.NewMemoryStore()
	observation := core.Observation{ServiceID: "boundary", SourceRef: "tests/ActorTests.cs", SourceKind: core.KindCSharpMethod, Symbol: "ActorTests.Adds", Package: "Game", Line: 1, EndLine: 2, Claim: "[Test] public void Adds()"}
	if err := st.PutObservations(context.Background(), []core.Observation{observation}); err != nil {
		t.Fatal(err)
	}
	return st, observation.ToSymbolNode()
}

func TestUnityNUnitDeclaredEnvelopeCannotUseGenericJoin(t *testing.T) {
	for _, wrapped := range []bool{false, true} {
		for _, declared := range []struct{ runner, artifact string }{
			{"unity", ""}, {"Unity", ""}, {"local", "unity-nunit-xml"}, {"", "unity-source-inputs"},
		} {
			st, symbol := nativeBoundaryStore(t)
			native := nativeBoundaryEnvelope(t, declared.runner, declared.artifact, wrapped)
			// Discriminating control: this exact case WOULD join through
			// upstream TQ28's file/name path. Its native declaration forbids it.
			_, generic := JoinCasesToSymbols(native.Cases, native.Suite.Name, []core.SymbolNode{symbol})
			if generic.Joined != 1 {
				t.Fatal("fixture no longer discriminates the generic fallback")
			}
			ordinary := nativeBoundaryEnvelope(t, "local", "", false)
			if _, err := IngestRuns(context.Background(), st, "boundary", "w", []Run{ordinary, native}); !errors.Is(err, ErrUnityEvidence) {
				t.Fatalf("declared native envelope fell through: wrapped=%v declaration=%+v err=%v", wrapped, declared, err)
			}
			rows, err := st.NewestTestCaseRuns(context.Background(), "boundary")
			if err != nil || len(rows) != 0 {
				t.Fatalf("refused mixed batch wrote evidence: rows=%+v err=%v", rows, err)
			}
		}
	}
}

func TestUnityNUnitBoundaryPreservesOrdinaryGenericEnvelope(t *testing.T) {
	for _, wrapped := range []bool{false, true} {
		st, symbol := nativeBoundaryStore(t)
		run := nativeBoundaryEnvelope(t, "local", "", wrapped)
		report, err := IngestRuns(context.Background(), st, "boundary", "w", []Run{run})
		if err != nil || report.CasesJoined != 1 || len(report.CasesUnmatched) != 0 {
			t.Fatalf("ordinary TQ28 envelope changed: report=%+v err=%v", report, err)
		}
		rows, err := st.NewestTestCaseRuns(context.Background(), "boundary")
		if err != nil || len(rows) != 1 || rows[0].TestSymbolKey != symbol.Key() || rows[0].NativeJSON != "" {
			t.Fatalf("ordinary envelope promoted or lost: rows=%+v err=%v", rows, err)
		}
	}
}
