package core

// AssetObjectID is a persistent imported object identity, never a code symbol
// or a Unity instance ID. FileID is canonical signed64 decimal text so JSON
// clients cannot round IDs outside JavaScript's exact integer range.
type AssetObjectID struct {
	GUID   string `json:"guid"`
	FileID string `json:"file_id"`
}

type AssetObject struct {
	ID       AssetObjectID `json:"id"`
	Path     string        `json:"path"`
	Basis    string        `json:"basis"`
	TypeName string        `json:"type_name,omitempty"`
}

type AssetDeclaredLocation struct {
	Line  int    `json:"line"`
	Field string `json:"field"`
}

// AssetReference is a grouped declaration/native tuple intersection. Locations
// and native property paths are deliberately unpaired: multiplicity does not
// prove which equal-target occurrence corresponds to which native property.
// This type is intentionally unrelated to Edge and SymbolNode.Calls.
type AssetReference struct {
	Kind                            string                  `json:"kind"`
	Source                          AssetObjectID           `json:"source"`
	Target                          AssetObjectID           `json:"target"`
	SourcePath                      string                  `json:"source_path"`
	TargetPath                      string                  `json:"target_path"`
	MatchedOccurrences              int                     `json:"matched_occurrences"`
	DeclaredLocations               []AssetDeclaredLocation `json:"declared_locations"`
	NativeProperties                []string                `json:"native_properties"`
	LocationsAndPropertiesArePaired bool                    `json:"locations_and_properties_are_paired"`
}

type AssetReferenceGap struct {
	Reason                   string                  `json:"reason"`
	Source                   AssetObjectID           `json:"source"`
	Target                   AssetObjectID           `json:"target"`
	TargetPath               string                  `json:"target_path,omitempty"`
	Occurrences              int                     `json:"occurrences"`
	DeclaredLocations        []AssetDeclaredLocation `json:"declared_locations,omitempty"`
	NativeProperties         []string                `json:"native_properties,omitempty"`
	PropertySubsetIdentified bool                    `json:"property_subset_identified"`
}

type AssetReferenceCounts struct {
	RawNull               int `json:"raw_null"`
	RawNonnull            int `json:"raw_nonnull"`
	NativeAvailable       int `json:"native_available"`
	NativeUnavailable     int `json:"native_unavailable"`
	Matched               int `json:"matched"`
	AdmittedMatched       int `json:"admitted_matched"`
	NativeObservedUnbound int `json:"native_observed_unbound"`
	RawOnly               int `json:"raw_only"`
	NativeOnly            int `json:"native_only"`
}

// AssetReferenceSnapshot is a validated partial cohort. Its receipt and source
// identity, not the service's Git HEAD, govern freshness. This first source
// slice has no durable store writer; preparation is not persistent admission.
type AssetReferenceSnapshot struct {
	Schema          string               `json:"schema"`
	Family          string               `json:"family"`
	Service         string               `json:"service"`
	Project         string               `json:"project"`
	FamilyID        string               `json:"family_id"`
	ReceiptSHA256   string               `json:"receipt_sha256"`
	SourceSHA256    string               `json:"source_sha256"`
	ContextSHA256   string               `json:"context_sha256"`
	EditorVersion   string               `json:"editor_version"`
	BuildTarget     string               `json:"build_target"`
	Scopes          []string             `json:"scopes"`
	SkippedMetadata []string             `json:"skipped_metadata"`
	Limitations     []string             `json:"limitations"`
	FamilyComplete  bool                 `json:"family_complete"`
	Runtime         bool                 `json:"runtime"`
	Grade           *string              `json:"grade"`
	Entities        []AssetObject        `json:"entities"`
	References      []AssetReference     `json:"references"`
	Gaps            []AssetReferenceGap  `json:"gaps"`
	Counts          AssetReferenceCounts `json:"counts"`
}

type AssetReferenceQuery struct {
	Object    AssetObjectID
	Direction string
	Limit     int
}

type AssetReferenceResult struct {
	Status         string               `json:"status"`
	Service        string               `json:"service"`
	Project        string               `json:"project"`
	FamilyID       string               `json:"family_id"`
	Object         AssetObjectID        `json:"object"`
	Direction      string               `json:"direction"`
	Limitations    []string             `json:"limitations"`
	ReceiptSHA256  string               `json:"receipt_sha256"`
	SourceSHA256   string               `json:"source_sha256"`
	ContextSHA256  string               `json:"context_sha256"`
	EditorVersion  string               `json:"editor_version"`
	BuildTarget    string               `json:"build_target"`
	FamilyComplete bool                 `json:"family_complete"`
	References     []AssetReference     `json:"references"`
	Total          int                  `json:"total"`
	Truncated      int                  `json:"truncated"`
	Counts         AssetReferenceCounts `json:"counts"`
}
