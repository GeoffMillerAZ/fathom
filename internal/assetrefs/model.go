// Package assetrefs prepares source-bound Unity asset-reference cohorts. It
// never writes observations, typed edges, code calls or execution evidence.
package assetrefs

import (
	"errors"
)

const ReceiptSchema = "fathom.unity.asset-references/v1"
const ActiveSchema = "fathom.unity.asset-references.active/v1"
const SnapshotSchema = "fathom.asset-reference-snapshot/v1"
const ProducerName = "unity-asset-references-local"

var ErrRefused = errors.New("Unity asset references refused")

// Options is caller-selected scope. Root is supplied by ServiceMeta, not by an
// artifact. ExpectedEditorSHA256 is an explicit local tool identity; this code
// never selects or reads an external executable path supplied by a receipt.
type Options struct {
	Root                  string
	Service               string
	Project               string
	FamilyID              string
	ReceiptPath           string
	ActivePath            string
	ExpectedEditorVersion string
	ExpectedEditorSHA256  string
	ExpectedBuildTarget   string
}

type fileHash struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type producer struct {
	Name    string     `json:"name"`
	Version string     `json:"version"`
	Path    string     `json:"path"`
	SHA256  string     `json:"sha256"`
	Inputs  []fileHash `json:"inputs"`
}
type editor struct {
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
}
type sourceHash struct {
	Before string `json:"before_sha256"`
	After  string `json:"after_sha256"`
}
type contextInventory struct {
	Schema          string     `json:"schema"`
	Before          string     `json:"before_sha256"`
	After           string     `json:"after_sha256"`
	AssetsCode      []fileHash `json:"assets_code"`
	ProjectSettings []fileHash `json:"project_settings"`
}
type coverage struct {
	FamilyComplete *bool  `json:"family_complete"`
	SourceFormat   string `json:"source_format"`
	Cohort         string `json:"cohort"`
}

type receipt struct {
	Schema           string            `json:"schema"`
	Status           string            `json:"status"`
	RunID            string            `json:"run_id"`
	Project          string            `json:"project"`
	FamilyID         string            `json:"family_id"`
	Producer         producer          `json:"producer"`
	Editor           editor            `json:"editor"`
	StartedAt        string            `json:"started_at"`
	FinishedAt       string            `json:"finished_at"`
	ExitCode         *int              `json:"exit_code"`
	Inventory        fileHash          `json:"inventory"`
	Oracle           fileHash          `json:"oracle"`
	OracleAssembly   fileHash          `json:"oracle_assembly"`
	Source           sourceHash        `json:"source"`
	ContextInventory *contextInventory `json:"context_inventory"`
	Coverage         coverage          `json:"coverage"`
}
type active struct {
	Schema   string   `json:"schema"`
	Project  string   `json:"project"`
	FamilyID string   `json:"family_id"`
	Phase    string   `json:"phase"`
	RunID    string   `json:"run_id"`
	Receipt  fileHash `json:"receipt"`
}

type rawAsset struct {
	GUID     string `json:"guid"`
	Meta     string `json:"meta"`
	Importer string `json:"importer"`
}
type rawEntity struct {
	Path     string `json:"path"`
	FileID   *int64 `json:"file_id"`
	ClassID  int    `json:"class_id"`
	Stripped bool   `json:"stripped"`
}
type rawReference struct {
	Source   string `json:"source"`
	SourceID *int64 `json:"source_file_id"`
	Line     int    `json:"line"`
	Field    string `json:"field_text"`
	FileID   *int64 `json:"file_id"`
	GUID     string `json:"guid"`
	Role     string `json:"role"`
	Status   string `json:"status"`
	Target   string `json:"target"`
}
type inventory struct {
	Schema      string              `json:"schema"`
	Parser      string              `json:"parser"`
	Project     string              `json:"project"`
	Scopes      []string            `json:"scopes"`
	Inputs      []fileHash          `json:"inputs"`
	Metadata    []string            `json:"metadata_members"`
	Skipped     []string            `json:"skipped"`
	Assets      map[string]rawAsset `json:"assets"`
	Entities    []rawEntity         `json:"entities"`
	References  []rawReference      `json:"references"`
	Problems    []any               `json:"problems"`
	Summary     map[string]int      `json:"summary"`
	Evidence    string              `json:"evidence"`
	Limitations []string            `json:"limitations"`
}
type nativeEntity struct {
	Path     string `json:"path"`
	GUID     string `json:"guid"`
	FileID   *int64 `json:"fileID"`
	TypeName string `json:"typeName"`
}
type nativeReference struct {
	Source     string `json:"source"`
	Property   string `json:"propertyPath"`
	SourceGUID string `json:"sourceGuid"`
	TargetGUID string `json:"targetGuid"`
	TargetPath string `json:"targetPath"`
	SourceID   *int64 `json:"sourceFileID"`
	TargetID   *int64 `json:"targetFileID"`
	Available  *bool  `json:"targetAvailable"`
}
type nativeAsset struct {
	Path           string   `json:"path"`
	GUID           string   `json:"guid"`
	DependencyHash string   `json:"dependencyHash"`
	Dependencies   []string `json:"dependencies"`
}
type oracle struct {
	Schema          int               `json:"schemaVersion"`
	Oracle          string            `json:"oracle"`
	UnityVersion    string            `json:"unityVersion"`
	BuildTarget     string            `json:"buildTarget"`
	InventorySHA256 string            `json:"inventorySha256"`
	AssemblySHA256  string            `json:"producerAssemblySha256"`
	Inputs          []fileHash        `json:"inputs"`
	Entities        []nativeEntity    `json:"entities"`
	References      []nativeReference `json:"references"`
	Assets          []nativeAsset     `json:"assets"`
	Unjudged        []string          `json:"unjudged"`
}
