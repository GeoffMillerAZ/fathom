// Package contracts defines canonical CUE-first type contracts shared across
// the agentic-kit platform. The Go types in this file mirror the CUE
// definitions in schema.cue. CUE is authoritative for the valid value space;
// Go is authoritative for runtime behaviour.
//
// No third-party dependencies are imported. Consumers get the constraint
// guarantees via Validate() without taking a dependency on a validation
// library.
//
// Boundary note: contracts.EventShape is the canonical CONTRACT/routing event
// shape — the validated, serialization-oriented envelope (string Kind,
// RFC 3339 string Timestamp, open Payload) used when events cross package or
// process boundaries under a routing contract.
// foundations/telemetry/event.Event is the canonical RUNTIME telemetry shape —
// the typed, in-process value (typed Kind enum, time.Time timestamp,
// string-only attributes) that live components emit into
// foundations/telemetry/sink Sinks. Emit event.Event at runtime; exchange
// EventShape where a contract governs the hop between systems.
package contracts

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// NodeTypeValue is a typed string that must be one of the values enumerated
// by NodeTypeEnum in schema.cue.
type NodeTypeValue string

const (
	// NodeTypeCompute identifies a general compute node.
	NodeTypeCompute NodeTypeValue = "compute"
	// NodeTypeStorage identifies a storage node.
	NodeTypeStorage NodeTypeValue = "storage"
	// NodeTypeGateway identifies a gateway/ingress node.
	NodeTypeGateway NodeTypeValue = "gateway"
	// NodeTypeInference identifies a model-inference node.
	NodeTypeInference NodeTypeValue = "inference"
	// NodeTypeOrchestrator identifies an orchestration/control-plane node.
	NodeTypeOrchestrator NodeTypeValue = "orchestrator"
)

// validNodeTypes is the set of valid NodeTypeValue members.
var validNodeTypes = map[NodeTypeValue]struct{}{
	NodeTypeCompute:      {},
	NodeTypeStorage:      {},
	NodeTypeGateway:      {},
	NodeTypeInference:    {},
	NodeTypeOrchestrator: {},
}

// FieldKind is a typed string that must be one of the values enumerated by
// FieldKindEnum in schema.cue.
type FieldKind string

const (
	// FieldKindString identifies a string-valued field.
	FieldKindString FieldKind = "string"
	// FieldKindNumber identifies a numeric-valued field.
	FieldKindNumber FieldKind = "number"
	// FieldKindBoolean identifies a boolean-valued field.
	FieldKindBoolean FieldKind = "boolean"
	// FieldKindList identifies a list-valued field.
	FieldKindList FieldKind = "list"
	// FieldKindMap identifies a map-valued field.
	FieldKindMap FieldKind = "map"
)

// validFieldKinds is the set of valid FieldKind members.
var validFieldKinds = map[FieldKind]struct{}{
	FieldKindString:  {},
	FieldKindNumber:  {},
	FieldKindBoolean: {},
	FieldKindList:    {},
	FieldKindMap:     {},
}

// EventShape is an elementary platform event. It mirrors #EventShape in
// schema.cue. Every event that crosses a package or process boundary under a
// routing contract must satisfy this type; for in-process runtime telemetry
// use foundations/telemetry/event.Event instead (see the package doc boundary
// note).
type EventShape struct {
	// ID is a unique identifier for the event. Must be non-empty.
	ID string `json:"ID"`

	// Kind classifies the event domain. Must be non-empty.
	Kind string `json:"Kind"`

	// Source identifies the emitting component or agent. Must be non-empty.
	Source string `json:"Source"`

	// Timestamp is the event time as an RFC 3339 string. Must be non-empty
	// and parseable as a valid timestamp.
	Timestamp string `json:"Timestamp"`

	// Payload carries event-specific fields. Must be non-nil.
	Payload map[string]any `json:"Payload"`
}

// Validate enforces the same constraints that #EventShape encodes in
// schema.cue. It returns a non-nil error if any constraint is violated.
func (e EventShape) Validate() error {
	var errs []string

	if strings.TrimSpace(e.ID) == "" {
		errs = append(errs, "ID must be non-empty")
	}
	if strings.TrimSpace(e.Kind) == "" {
		errs = append(errs, "Kind must be non-empty")
	}
	if strings.TrimSpace(e.Source) == "" {
		errs = append(errs, "Source must be non-empty")
	}
	if strings.TrimSpace(e.Timestamp) == "" {
		errs = append(errs, "Timestamp must be non-empty")
	} else {
		if _, err := time.Parse(time.RFC3339, e.Timestamp); err != nil {
			errs = append(errs, fmt.Sprintf("Timestamp must be a valid RFC 3339 string: %v", err))
		}
	}
	if e.Payload == nil {
		errs = append(errs, "Payload must be non-nil")
	}

	if len(errs) > 0 {
		return errors.New("EventShape: " + strings.Join(errs, "; "))
	}
	return nil
}

// MarshalJSON implements json.Marshaler. It delegates to the default encoding
// while ensuring Payload is serialised as an object (never null).
func (e EventShape) MarshalJSON() ([]byte, error) {
	type alias EventShape
	a := alias(e)
	if a.Payload == nil {
		a.Payload = map[string]any{}
	}
	return json.Marshal(a)
}

// NodeType represents a governed substrate node. It mirrors #NodeType in
// schema.cue.
type NodeType struct {
	// ID uniquely identifies the node. Must be non-empty.
	ID string `json:"ID"`

	// Type is the node's substrate classification. Must be a member of
	// NodeTypeEnum.
	Type NodeTypeValue `json:"Type"`

	// Labels are arbitrary key/value metadata attached to the node.
	Labels map[string]string `json:"Labels"`
}

// Validate enforces the same constraints that #NodeType encodes in schema.cue.
func (n NodeType) Validate() error {
	var errs []string

	if strings.TrimSpace(n.ID) == "" {
		errs = append(errs, "ID must be non-empty")
	}
	if _, ok := validNodeTypes[n.Type]; !ok {
		errs = append(errs, fmt.Sprintf(
			"Type %q is not a valid NodeTypeValue (must be one of: compute, storage, gateway, inference, orchestrator)",
			n.Type,
		))
	}
	if n.Labels == nil {
		errs = append(errs, "Labels must be non-nil")
	}

	if len(errs) > 0 {
		return errors.New("NodeType: " + strings.Join(errs, "; "))
	}
	return nil
}

// AnswerArchetype is a reusable answer-shape template. It mirrors
// #AnswerArchetype in schema.cue.
type AnswerArchetype struct {
	// Name is the human-readable identifier. Must be non-empty.
	Name string `json:"Name"`

	// Sections is the ordered list of prose section names. At least one
	// section is required, and each section name must be non-empty.
	Sections []string `json:"Sections"`

	// RequiredEvidence indicates whether the archetype mandates that at
	// least one evidence span be cited.
	RequiredEvidence bool `json:"RequiredEvidence"`
}

// Validate enforces the same constraints that #AnswerArchetype encodes in
// schema.cue.
func (a AnswerArchetype) Validate() error {
	var errs []string

	if strings.TrimSpace(a.Name) == "" {
		errs = append(errs, "Name must be non-empty")
	}
	if len(a.Sections) == 0 {
		errs = append(errs, "Sections must contain at least one element")
	}
	for i, s := range a.Sections {
		if strings.TrimSpace(s) == "" {
			errs = append(errs, fmt.Sprintf("Sections[%d] must be non-empty", i))
		}
	}

	if len(errs) > 0 {
		return errors.New("AnswerArchetype: " + strings.Join(errs, "; "))
	}
	return nil
}

// FieldSpec describes a single field in a KnowledgeCard. It mirrors
// #FieldSpec in schema.cue.
type FieldSpec struct {
	// Name is the field identifier. Must be non-empty.
	Name string `json:"Name"`

	// Kind is the primitive or composite type. Must be a member of
	// FieldKindEnum.
	Kind FieldKind `json:"Kind"`

	// Required indicates whether the field must be present on every card
	// that uses the archetype.
	Required bool `json:"Required"`
}

// Validate enforces the same constraints that #FieldSpec encodes in schema.cue.
func (f FieldSpec) Validate() error {
	var errs []string

	if strings.TrimSpace(f.Name) == "" {
		errs = append(errs, "Name must be non-empty")
	}
	if _, ok := validFieldKinds[f.Kind]; !ok {
		errs = append(errs, fmt.Sprintf(
			"Kind %q is not a valid FieldKind (must be one of: string, number, boolean, list, map)",
			f.Kind,
		))
	}

	if len(errs) > 0 {
		return errors.New("FieldSpec: " + strings.Join(errs, "; "))
	}
	return nil
}

// KnowledgeCardArchetype defines the canonical template for a knowledge card.
// It mirrors #KnowledgeCardArchetype in schema.cue.
type KnowledgeCardArchetype struct {
	// Name is the human-readable identifier. Must be non-empty.
	Name string `json:"Name"`

	// Fields is the list of field specifications. At least one field is
	// required. Each field must itself be valid.
	Fields []FieldSpec `json:"Fields"`
}

// Validate enforces the same constraints that #KnowledgeCardArchetype encodes
// in schema.cue. It also recursively validates each FieldSpec.
func (k KnowledgeCardArchetype) Validate() error {
	var errs []string

	if strings.TrimSpace(k.Name) == "" {
		errs = append(errs, "Name must be non-empty")
	}
	if len(k.Fields) == 0 {
		errs = append(errs, "Fields must contain at least one element")
	}
	for i, f := range k.Fields {
		if err := f.Validate(); err != nil {
			errs = append(errs, fmt.Sprintf("Fields[%d]: %v", i, err))
		}
	}

	if len(errs) > 0 {
		return errors.New("KnowledgeCardArchetype: " + strings.Join(errs, "; "))
	}
	return nil
}
