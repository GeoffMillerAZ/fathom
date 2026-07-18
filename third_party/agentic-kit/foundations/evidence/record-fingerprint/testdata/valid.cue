// valid.cue — concrete values that satisfy the #Fingerprint and #ChangeKind
// constraints defined in schema.cue. Running `cue vet ./...` from the package
// root validates these against the schema.
package recordfingerprint

// A fingerprint with all fields populated.
_fp1: #Fingerprint & {
	RecordID:    "doc-001"
	ContentHash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	SeenAt:      "2024-01-01T00:00:00Z"
}

// A second fingerprint with a different record and timestamp.
_fp2: #Fingerprint & {
	RecordID:    "doc-002"
	ContentHash: "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	SeenAt:      "2024-06-01T12:34:56Z"
}

// ChangeKind values.
_kindNew:       #ChangeKind & "new"
_kindUnchanged: #ChangeKind & "unchanged"
_kindChanged:   #ChangeKind & "changed"
