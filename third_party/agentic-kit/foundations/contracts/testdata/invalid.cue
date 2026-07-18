// invalid.cue — NEGATIVE fixtures, documented (not live-vetted).
//
// These shapes intentionally violate schema.cue. They are kept commented so the
// repo gate (`cue vet ./...`) stays green; their REJECTION is asserted by the Go
// tests in contracts_test.go (Validate() must reject each). To verify a case at
// the CUE layer manually, uncomment it and run:
//   cue vet -c schema.cue <(this case)
//
// 1. EventShape missing required ID:
//      badEvent: #EventShape & {Kind: "signal", Source: "x", Timestamp: "..."}
//      -> badEvent.ID: field is required but not present
//
// 2. NodeType with a Type outside the enum:
//      badNode: #NodeType & {ID: "n1", Type: "database"}
//      -> Type: conflicting values, not in {compute,storage,gateway,inference,orchestrator}
//
// 3. AnswerArchetype with empty Sections:
//      badAnswer: #AnswerArchetype & {Name: "x", Sections: []}
//      -> Sections: must be non-empty
//
// 4. KnowledgeCardArchetype with a field Kind outside the enum:
//      badCard: #KnowledgeCardArchetype & {Name: "x", Fields: [{Name: "f", Kind: "object"}]}
//      -> Fields.0.Kind: conflicting values, not in the FieldKind enum
