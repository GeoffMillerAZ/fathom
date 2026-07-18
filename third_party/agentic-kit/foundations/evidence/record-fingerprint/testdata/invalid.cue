// invalid.cue — executable negative contracts for #Fingerprint and #ChangeKind.
// Each binding below is intentionally invalid — it demonstrates what the schema
// MUST reject. Per repo convention (see Taskfile cue-vet), negative cases live
// as commented documentation; the executable rejection contracts are the Go
// validation tests. testdata/ IS vetted by the gate, so these bindings stay
// commented — uncomment one locally to watch `cue vet ./testdata/` reject it.
package recordfingerprint

// _badEmptyRecordID: empty RecordID violates the _nonEmpty constraint.
// _badEmptyRecordID: #Fingerprint & {
// 	RecordID:    ""
// 	ContentHash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
// 	SeenAt:      "2024-01-01T00:00:00Z"
// }

// _badEmptyContentHash: empty ContentHash violates the _nonEmpty constraint.
// _badEmptyContentHash: #Fingerprint & {
// 	RecordID:    "doc-001"
// 	ContentHash: ""
// 	SeenAt:      "2024-01-01T00:00:00Z"
// }

// _badShortContentHash: a 6-char hex string violates the 64-char SHA-256
// regex constraint on ContentHash.
// _badShortContentHash: #Fingerprint & {
// 	RecordID:    "doc-001"
// 	ContentHash: "abc123"
// 	SeenAt:      "2024-01-01T00:00:00Z"
// }

// _badUppercaseContentHash: uppercase hex violates the lowercase-only
// regex constraint on ContentHash.
// _badUppercaseContentHash: #Fingerprint & {
// 	RecordID:    "doc-001"
// 	ContentHash: "E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855"
// 	SeenAt:      "2024-01-01T00:00:00Z"
// }

// _badSeenAtNoTimezone: SeenAt without a timezone offset violates the strict
// _rfc3339 constraint — bare local timestamps are not RFC3339-compliant.
// _badSeenAtNoTimezone: #Fingerprint & {
// 	RecordID:    "doc-001"
// 	ContentHash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
// 	SeenAt:      "2024-06-01T14:02:00"
// }

// _badChangeKind: value not in the allowed set violates the #ChangeKind constraint.
// _badChangeKind: #ChangeKind & "modified"
