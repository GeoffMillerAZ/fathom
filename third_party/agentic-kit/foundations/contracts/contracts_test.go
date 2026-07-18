package contracts_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/contracts"
)

// ── EventShape ────────────────────────────────────────────────────────────────

func TestEventShape_Validate_valid(t *testing.T) {
	e := contracts.EventShape{
		ID:        "evt-001",
		Kind:      "tool.call",
		Source:    "agent/recon-01",
		Timestamp: "2024-01-15T10:30:00Z",
		Payload:   map[string]any{"tool": "web_search"},
	}
	if err := e.Validate(); err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestEventShape_Validate_invalid(t *testing.T) {
	base := contracts.EventShape{
		ID:        "evt-001",
		Kind:      "tool.call",
		Source:    "agent/recon-01",
		Timestamp: "2024-01-15T10:30:00Z",
		Payload:   map[string]any{"tool": "web_search"},
	}

	cases := []struct {
		name    string
		mutate  func(*contracts.EventShape)
		wantErr string
	}{
		{
			name:    "empty ID",
			mutate:  func(e *contracts.EventShape) { e.ID = "" },
			wantErr: "ID must be non-empty",
		},
		{
			name:    "whitespace ID",
			mutate:  func(e *contracts.EventShape) { e.ID = "   " },
			wantErr: "ID must be non-empty",
		},
		{
			name:    "empty Kind",
			mutate:  func(e *contracts.EventShape) { e.Kind = "" },
			wantErr: "Kind must be non-empty",
		},
		{
			name:    "empty Source",
			mutate:  func(e *contracts.EventShape) { e.Source = "" },
			wantErr: "Source must be non-empty",
		},
		{
			name:    "empty Timestamp",
			mutate:  func(e *contracts.EventShape) { e.Timestamp = "" },
			wantErr: "Timestamp must be non-empty",
		},
		{
			name:    "invalid Timestamp format",
			mutate:  func(e *contracts.EventShape) { e.Timestamp = "not-a-date" },
			wantErr: "Timestamp must be a valid RFC 3339",
		},
		{
			name:    "nil Payload",
			mutate:  func(e *contracts.EventShape) { e.Payload = nil },
			wantErr: "Payload must be non-nil",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := base
			tc.mutate(&e)
			err := e.Validate()
			if err == nil {
				t.Fatal("expected a validation error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestEventShape_JSONRoundTrip(t *testing.T) {
	original := contracts.EventShape{
		ID:        "evt-roundtrip",
		Kind:      "heartbeat",
		Source:    "worker/01",
		Timestamp: "2024-06-01T00:00:00Z",
		Payload:   map[string]any{"ping": "pong", "count": float64(3)},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var restored contracts.EventShape
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if restored.ID != original.ID {
		t.Errorf("ID: got %q, want %q", restored.ID, original.ID)
	}
	if restored.Kind != original.Kind {
		t.Errorf("Kind: got %q, want %q", restored.Kind, original.Kind)
	}
	if restored.Source != original.Source {
		t.Errorf("Source: got %q, want %q", restored.Source, original.Source)
	}
	if restored.Timestamp != original.Timestamp {
		t.Errorf("Timestamp: got %q, want %q", restored.Timestamp, original.Timestamp)
	}
	if len(restored.Payload) != len(original.Payload) {
		t.Errorf("Payload len: got %d, want %d", len(restored.Payload), len(original.Payload))
	}
}

// ── NodeType ──────────────────────────────────────────────────────────────────

func TestNodeType_Validate_valid(t *testing.T) {
	n := contracts.NodeType{
		ID:     "node-01",
		Type:   contracts.NodeTypeCompute,
		Labels: map[string]string{"region": "us-east-1"},
	}
	if err := n.Validate(); err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestNodeType_Validate_allEnumValues(t *testing.T) {
	validTypes := []contracts.NodeTypeValue{
		contracts.NodeTypeCompute,
		contracts.NodeTypeStorage,
		contracts.NodeTypeGateway,
		contracts.NodeTypeInference,
		contracts.NodeTypeOrchestrator,
	}
	for _, nt := range validTypes {
		t.Run(string(nt), func(t *testing.T) {
			n := contracts.NodeType{
				ID:     "node-x",
				Type:   nt,
				Labels: map[string]string{},
			}
			if err := n.Validate(); err != nil {
				t.Fatalf("unexpected error for type %q: %v", nt, err)
			}
		})
	}
}

func TestNodeType_Validate_invalid(t *testing.T) {
	base := contracts.NodeType{
		ID:     "node-01",
		Type:   contracts.NodeTypeCompute,
		Labels: map[string]string{},
	}

	cases := []struct {
		name    string
		mutate  func(*contracts.NodeType)
		wantErr string
	}{
		{
			name:    "empty ID",
			mutate:  func(n *contracts.NodeType) { n.ID = "" },
			wantErr: "ID must be non-empty",
		},
		{
			name:    "bad enum Type",
			mutate:  func(n *contracts.NodeType) { n.Type = "database" },
			wantErr: "not a valid NodeTypeValue",
		},
		{
			name:    "nil Labels",
			mutate:  func(n *contracts.NodeType) { n.Labels = nil },
			wantErr: "Labels must be non-nil",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n := base
			tc.mutate(&n)
			err := n.Validate()
			if err == nil {
				t.Fatal("expected a validation error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestNodeType_JSONRoundTrip(t *testing.T) {
	original := contracts.NodeType{
		ID:     "node-storage-02",
		Type:   contracts.NodeTypeStorage,
		Labels: map[string]string{"tier": "hot", "region": "eu-west-1"},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var restored contracts.NodeType
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if restored.ID != original.ID || restored.Type != original.Type {
		t.Errorf("round-trip mismatch: got %+v, want %+v", restored, original)
	}
}

// ── AnswerArchetype ───────────────────────────────────────────────────────────

func TestAnswerArchetype_Validate_valid(t *testing.T) {
	a := contracts.AnswerArchetype{
		Name:             "investigation-summary",
		Sections:         []string{"executive-summary", "findings", "recommendations"},
		RequiredEvidence: true,
	}
	if err := a.Validate(); err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestAnswerArchetype_Validate_invalid(t *testing.T) {
	base := contracts.AnswerArchetype{
		Name:             "investigation-summary",
		Sections:         []string{"findings"},
		RequiredEvidence: false,
	}

	cases := []struct {
		name    string
		mutate  func(*contracts.AnswerArchetype)
		wantErr string
	}{
		{
			name:    "empty Name",
			mutate:  func(a *contracts.AnswerArchetype) { a.Name = "" },
			wantErr: "Name must be non-empty",
		},
		{
			name:    "empty Sections slice",
			mutate:  func(a *contracts.AnswerArchetype) { a.Sections = []string{} },
			wantErr: "Sections must contain at least one element",
		},
		{
			name:    "nil Sections slice",
			mutate:  func(a *contracts.AnswerArchetype) { a.Sections = nil },
			wantErr: "Sections must contain at least one element",
		},
		{
			name:    "empty section name in list",
			mutate:  func(a *contracts.AnswerArchetype) { a.Sections = []string{"findings", ""} },
			wantErr: "Sections[1] must be non-empty",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := base
			tc.mutate(&a)
			err := a.Validate()
			if err == nil {
				t.Fatal("expected a validation error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestAnswerArchetype_JSONRoundTrip(t *testing.T) {
	original := contracts.AnswerArchetype{
		Name:             "brief-answer",
		Sections:         []string{"summary", "evidence"},
		RequiredEvidence: true,
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var restored contracts.AnswerArchetype
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if restored.Name != original.Name || restored.RequiredEvidence != original.RequiredEvidence {
		t.Errorf("round-trip mismatch: got %+v, want %+v", restored, original)
	}
	if len(restored.Sections) != len(original.Sections) {
		t.Errorf("Sections len: got %d, want %d", len(restored.Sections), len(original.Sections))
	}
}

// ── KnowledgeCardArchetype ────────────────────────────────────────────────────

func TestKnowledgeCardArchetype_Validate_valid(t *testing.T) {
	k := contracts.KnowledgeCardArchetype{
		Name: "vulnerability-record",
		Fields: []contracts.FieldSpec{
			{Name: "cve-id", Kind: contracts.FieldKindString, Required: true},
			{Name: "severity", Kind: contracts.FieldKindNumber, Required: true},
			{Name: "mitigated", Kind: contracts.FieldKindBoolean, Required: false},
		},
	}
	if err := k.Validate(); err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestKnowledgeCardArchetype_Validate_allFieldKinds(t *testing.T) {
	validKinds := []contracts.FieldKind{
		contracts.FieldKindString,
		contracts.FieldKindNumber,
		contracts.FieldKindBoolean,
		contracts.FieldKindList,
		contracts.FieldKindMap,
	}
	for _, fk := range validKinds {
		t.Run(string(fk), func(t *testing.T) {
			f := contracts.FieldSpec{Name: "test-field", Kind: fk, Required: false}
			if err := f.Validate(); err != nil {
				t.Fatalf("unexpected error for kind %q: %v", fk, err)
			}
		})
	}
}

func TestKnowledgeCardArchetype_Validate_invalid(t *testing.T) {
	base := contracts.KnowledgeCardArchetype{
		Name: "vulnerability-record",
		Fields: []contracts.FieldSpec{
			{Name: "cve-id", Kind: contracts.FieldKindString, Required: true},
		},
	}

	cases := []struct {
		name    string
		mutate  func(*contracts.KnowledgeCardArchetype)
		wantErr string
	}{
		{
			name:    "empty Name",
			mutate:  func(k *contracts.KnowledgeCardArchetype) { k.Name = "" },
			wantErr: "Name must be non-empty",
		},
		{
			name:    "empty Fields slice",
			mutate:  func(k *contracts.KnowledgeCardArchetype) { k.Fields = []contracts.FieldSpec{} },
			wantErr: "Fields must contain at least one element",
		},
		{
			name:    "nil Fields slice",
			mutate:  func(k *contracts.KnowledgeCardArchetype) { k.Fields = nil },
			wantErr: "Fields must contain at least one element",
		},
		{
			name: "field with bad Kind",
			mutate: func(k *contracts.KnowledgeCardArchetype) {
				k.Fields = []contracts.FieldSpec{
					{Name: "meta", Kind: "object", Required: false},
				}
			},
			wantErr: "not a valid FieldKind",
		},
		{
			name: "field with empty Name",
			mutate: func(k *contracts.KnowledgeCardArchetype) {
				k.Fields = []contracts.FieldSpec{
					{Name: "", Kind: contracts.FieldKindString, Required: false},
				}
			},
			wantErr: "Name must be non-empty",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k := base
			tc.mutate(&k)
			err := k.Validate()
			if err == nil {
				t.Fatal("expected a validation error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestKnowledgeCardArchetype_JSONRoundTrip(t *testing.T) {
	original := contracts.KnowledgeCardArchetype{
		Name: "asset-record",
		Fields: []contracts.FieldSpec{
			{Name: "asset-id", Kind: contracts.FieldKindString, Required: true},
			{Name: "tags", Kind: contracts.FieldKindList, Required: false},
		},
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var restored contracts.KnowledgeCardArchetype
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if restored.Name != original.Name {
		t.Errorf("Name: got %q, want %q", restored.Name, original.Name)
	}
	if len(restored.Fields) != len(original.Fields) {
		t.Errorf("Fields len: got %d, want %d", len(restored.Fields), len(original.Fields))
	}
	for i := range original.Fields {
		if restored.Fields[i] != original.Fields[i] {
			t.Errorf("Fields[%d]: got %+v, want %+v", i, restored.Fields[i], original.Fields[i])
		}
	}
}
