// Package qualityplan exposes the CUE-authored converted-component work queue.
// Its records are authored planning claims, not executed checks or certification.
package qualityplan

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/geoffmilleraz/app-kit/appkit/catalog"
)

const (
	claimBoundary      = "authored-planning-not-certification"
	maxListJSONBytes   = 64 << 10
	maxDetailJSONBytes = 128 << 10
)

var idPattern = regexp.MustCompile(`^[a-z][a-z0-9._-]{0,95}$`)

//go:embed schema.cue
var embeddedSchema []byte

//go:embed inventory.cue
var embeddedInventory []byte

// Ref identifies authored evidence or source material without reading it.
type Ref struct {
	Path string `json:"path"`
	Note string `json:"note"`
}

// PinnedRef records an authored byte identity. Loading a Snapshot does not
// re-read or authenticate the referenced file.
type PinnedRef struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// Review records the authored reviewer and review reference.
type Review struct {
	By  string `json:"by"`
	Ref string `json:"ref"`
}

// StandardDefinition is one requirement in the enhancement standard catalog.
type StandardDefinition struct {
	Name        string   `json:"name"`
	Requirement string   `json:"requirement"`
	GateRefs    []string `json:"gate_refs"`
}

// StandardStatus is an authored implementation status for one standard.
type StandardStatus struct {
	Status   string  `json:"status"`
	Reason   string  `json:"reason,omitempty"`
	NextStep string  `json:"next_step,omitempty"`
	Evidence []Ref   `json:"evidence"`
	Review   *Review `json:"review,omitempty"`
}

// GateStatus is an authored verification-gate claim.
type GateStatus struct {
	Status           string     `json:"status"`
	Reason           string     `json:"reason,omitempty"`
	NextStep         string     `json:"next_step,omitempty"`
	Evidence         []Ref      `json:"evidence"`
	Receipt          *PinnedRef `json:"receipt,omitempty"`
	SourceRevision   string     `json:"source_revision,omitempty"`
	ArtifactRevision string     `json:"artifact_revision,omitempty"`
	Review           *Review    `json:"review,omitempty"`
}

// ConsumerRevision identifies the authored revision selected by a consumer.
type ConsumerRevision struct {
	Kind   string `json:"kind"`
	SHA256 string `json:"sha256"`
}

// Consumer records an authored consumer integration claim.
type Consumer struct {
	App               string           `json:"app"`
	Revision          ConsumerRevision `json:"revision"`
	IntegrationStatus string           `json:"integration_status"`
	Evidence          []Ref            `json:"evidence"`
	Receipt           *PinnedRef       `json:"receipt,omitempty"`
	Review            *Review          `json:"review,omitempty"`
}

// Budget is one named limit in a selected conversion contract.
type Budget struct {
	Name  string `json:"name"`
	Limit string `json:"limit"`
}

// OriginalSelection pins the original selected for conversion.
type OriginalSelection struct {
	Kind           string `json:"kind"`
	ID             string `json:"id"`
	RevisionSHA256 string `json:"revision_sha256"`
}

// TargetSelection pins the target selected for conversion.
type TargetSelection struct {
	ID             string `json:"id"`
	RevisionSHA256 string `json:"revision_sha256"`
}

// ConversionSelection carries the authored workload and acceptance boundary.
type ConversionSelection struct {
	Original           OriginalSelection `json:"original"`
	Target             TargetSelection   `json:"target"`
	AcceptanceContract PinnedRef         `json:"acceptance_contract"`
	Workload           string            `json:"workload"`
	Budgets            []Budget          `json:"budgets"`
}

// Item is the complete authored readiness work item.
type Item struct {
	ID                          string                    `json:"id"`
	Kind                        string                    `json:"kind"`
	Owner                       string                    `json:"owner"`
	Task                        string                    `json:"task"`
	Profile                     string                    `json:"profile"`
	Priority                    string                    `json:"priority"`
	Core                        bool                      `json:"core"`
	State                       string                    `json:"state"`
	EnhancementStandardRevision string                    `json:"enhancement_standard_revision"`
	CapabilityIDs               []string                  `json:"capability_ids"`
	SourceRefs                  []Ref                     `json:"source_refs"`
	Standards                   map[string]StandardStatus `json:"standards"`
	Gates                       map[string]GateStatus     `json:"gates"`
	Selection                   *ConversionSelection      `json:"selection,omitempty"`
	Consumers                   []Consumer                `json:"consumers"`
}

// BacklogPolicy describes how unselected work enters the authored queue.
type BacklogPolicy struct {
	Mode               string   `json:"mode"`
	DefaultState       string   `json:"default_state"`
	SelectionRequires  []string `json:"selection_requires"`
	OwnerBatchRequires []string `json:"owner_batch_requires"`
	UnselectedClaim    string   `json:"unselected_claim"`
}

// Summary is the bounded list projection of a work item.
type Summary struct {
	ID            string   `json:"id"`
	Kind          string   `json:"kind"`
	Task          string   `json:"task"`
	Profile       string   `json:"profile"`
	Core          bool     `json:"core"`
	State         string   `json:"state"`
	CapabilityIDs []string `json:"capability_ids"`
}

// ListSelection explains discovery coverage, never quality or completion.
// AvailableScopes describes matching authored items before applying Scope.
// Status is matched, filtered-out, or untracked; unknown capabilities are errors.
type ListSelection struct {
	CapabilityID    string   `json:"capability_id,omitempty"`
	Scope           string   `json:"scope"`
	Status          string   `json:"status"`
	AvailableScopes []string `json:"available_scopes"`
	NextAction      string   `json:"next_action"`
}

// ListResult is a compact, revision-bound planning projection. Revision binds
// the authored inventory, not the wire format or executing checker build.
// CatalogRevision separately identifies the validated capability metadata.
type ListResult struct {
	Revision                    string        `json:"revision"`
	CatalogRevision             string        `json:"catalog_revision"`
	EnhancementStandardRevision string        `json:"enhancement_standard_revision"`
	ClaimBoundary               string        `json:"claim_boundary"`
	Items                       []Summary     `json:"items"`
	Selection                   ListSelection `json:"selection"`
}

// DetailResult is one full authored item and its governing standards/policy.
type DetailResult struct {
	Revision                    string                        `json:"revision"`
	CatalogRevision             string                        `json:"catalog_revision"`
	EnhancementStandardRevision string                        `json:"enhancement_standard_revision"`
	ClaimBoundary               string                        `json:"claim_boundary"`
	Item                        Item                          `json:"item"`
	Standards                   map[string]StandardDefinition `json:"standards"`
	BacklogPolicy               BacklogPolicy                 `json:"backlog_policy"`
}

type inventoryDocument struct {
	SchemaVersion               int                           `json:"schema_version"`
	EnhancementStandardRevision string                        `json:"enhancement_standard_revision"`
	Standards                   map[string]StandardDefinition `json:"standards"`
	BacklogPolicy               BacklogPolicy                 `json:"backlog_policy"`
	Items                       []Item                        `json:"items"`
}

// Snapshot is an immutable, validated view of the embedded authored inventory.
// List and Get always return detached values.
type Snapshot struct {
	revision                    string
	catalogRevision             string
	enhancementStandardRevision string
	standards                   map[string]StandardDefinition
	backlogPolicy               BacklogPolicy
	items                       []Item
	catalogIDs                  map[string]struct{}
}

// Default loads the embedded CUE inventory and validates it against the embedded
// schema and the current first-party catalog. It performs no file or network I/O.
func Default() (*Snapshot, error) {
	capabilities, err := catalog.Default()
	if err != nil {
		return nil, fmt.Errorf("qualityplan: load catalog: %w", err)
	}
	return ForCatalog(capabilities)
}

// ForCatalog validates the embedded authored inventory against one exact catalog.
// It uses the immutable search snapshot, not its exported inspection copies.
// Like Default, it performs no file, network or evidence verification I/O.
func ForCatalog(capabilities *catalog.Catalog) (*Snapshot, error) {
	return loadSnapshot(embeddedInventory, capabilities)
}

func loadSnapshot(source []byte, capabilities *catalog.Catalog) (*Snapshot, error) {
	ctx := cuecontext.New()
	schema := ctx.CompileBytes(embeddedSchema, cue.Filename("schema.cue"))
	if err := schema.Err(); err != nil {
		return nil, fmt.Errorf("qualityplan: compile embedded schema: %w", err)
	}
	value := ctx.CompileBytes(source, cue.Filename("inventory.cue"))
	if err := value.Err(); err != nil {
		return nil, fmt.Errorf("qualityplan: compile inventory: %w", err)
	}
	value = schema.LookupPath(cue.ParsePath("#Inventory")).Unify(value)
	if err := value.Validate(cue.Concrete(true)); err != nil {
		return nil, fmt.Errorf("qualityplan: inventory does not satisfy #Inventory: %w", err)
	}
	var document inventoryDocument
	if err := value.Decode(&document); err != nil {
		return nil, fmt.Errorf("qualityplan: decode inventory: %w", err)
	}
	if capabilities == nil {
		return nil, fmt.Errorf("qualityplan: nil catalog")
	}
	profiles := make(map[string]map[string]struct{})
	var catalogRevision, cursor string
	for {
		page, err := capabilities.Search(catalog.Query{Limit: 50, Cursor: cursor})
		if err != nil {
			return nil, fmt.Errorf("qualityplan: read validated catalog: %w", err)
		}
		catalogRevision = page.Revision
		for _, entry := range page.Items {
			allowed := make(map[string]struct{}, len(entry.Profiles))
			for _, profile := range entry.Profiles {
				allowed[profile] = struct{}{}
			}
			profiles[entry.ID] = allowed
		}
		cursor = page.NextCursor
		if cursor == "" {
			break
		}
	}
	sort.Slice(document.Items, func(i, j int) bool { return document.Items[i].ID < document.Items[j].ID })
	for i, item := range document.Items {
		if i > 0 && document.Items[i-1].ID == item.ID {
			return nil, fmt.Errorf("qualityplan: duplicate item ID %q", item.ID)
		}
		consumerIDs := make(map[[3]string]struct{}, len(item.Consumers))
		for _, consumer := range item.Consumers {
			identity := [3]string{consumer.App, consumer.Revision.Kind, consumer.Revision.SHA256}
			if _, duplicate := consumerIDs[identity]; duplicate {
				return nil, fmt.Errorf("qualityplan: item %q has duplicate consumer identity for app %q and revision %s:%s", item.ID, consumer.App, consumer.Revision.Kind, consumer.Revision.SHA256)
			}
			consumerIDs[identity] = struct{}{}
		}
		for _, capabilityID := range item.CapabilityIDs {
			allowed, ok := profiles[capabilityID]
			if !ok {
				return nil, fmt.Errorf("qualityplan: item %q references unknown capability %q", item.ID, capabilityID)
			}
			if _, ok := allowed[item.Profile]; !ok {
				return nil, fmt.Errorf("qualityplan: item %q capability %q does not support profile %q", item.ID, capabilityID, item.Profile)
			}
		}
	}
	canonical, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("qualityplan: encode canonical inventory: %w", err)
	}
	revision := sha256.Sum256(canonical)
	return &Snapshot{
		revision:                    hex.EncodeToString(revision[:]),
		catalogRevision:             catalogRevision,
		enhancementStandardRevision: document.EnhancementStandardRevision,
		standards:                   document.Standards,
		backlogPolicy:               document.BacklogPolicy,
		items:                       document.Items,
		catalogIDs:                  catalogIDSet(profiles),
	}, nil
}

func catalogIDSet[V any](values map[string]V) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for key := range values {
		out[key] = struct{}{}
	}
	return out
}

// List returns a detached compact projection. Scope is core (also the empty
// default), on-demand (all non-core items), or all. A capability filter is exact.
func (s *Snapshot) List(capabilityID, scope string) (ListResult, error) {
	if s == nil || s.revision == "" {
		return ListResult{}, fmt.Errorf("qualityplan: uninitialized snapshot")
	}
	if scope == "" {
		scope = "core"
	}
	if scope != "core" && scope != "on-demand" && scope != "all" {
		return ListResult{}, fmt.Errorf("qualityplan: invalid scope")
	}
	if capabilityID != "" {
		if err := validateID(capabilityID); err != nil {
			return ListResult{}, fmt.Errorf("qualityplan: invalid capability ID: %w", err)
		}
		if _, ok := s.catalogIDs[capabilityID]; !ok {
			return ListResult{}, fmt.Errorf("qualityplan: unknown capability %q", capabilityID)
		}
	}
	result := ListResult{
		Revision:                    s.revision,
		CatalogRevision:             s.catalogRevision,
		EnhancementStandardRevision: s.enhancementStandardRevision,
		ClaimBoundary:               claimBoundary,
		Items:                       make([]Summary, 0),
		Selection: ListSelection{
			CapabilityID: capabilityID, Scope: scope, AvailableScopes: make([]string, 0, 2),
		},
	}
	hasCore, hasOnDemand := false, false
	for _, item := range s.items {
		if capabilityID != "" && !slices.Contains(item.CapabilityIDs, capabilityID) {
			continue
		}
		if item.Core {
			hasCore = true
		} else {
			hasOnDemand = true
		}
		if scope == "core" && !item.Core || scope == "on-demand" && item.Core {
			continue
		}
		result.Items = append(result.Items, Summary{
			ID: item.ID, Kind: item.Kind, Task: item.Task, Profile: item.Profile,
			Core: item.Core, State: item.State, CapabilityIDs: slices.Clone(item.CapabilityIDs),
		})
	}
	if hasCore {
		result.Selection.AvailableScopes = append(result.Selection.AvailableScopes, "core")
	}
	if hasOnDemand {
		result.Selection.AvailableScopes = append(result.Selection.AvailableScopes, "on-demand")
	}
	switch {
	case len(result.Items) > 0:
		result.Selection.Status = "matched"
		result.Selection.NextAction = "Inspect matching items with get_quality_work using expected_revision=revision and expected_catalog_revision=catalog_revision from this result. Review their evidence and applicable gates before adoption; these records are authored planning."
	case len(result.Selection.AvailableScopes) > 0:
		result.Selection.Status = "filtered-out"
		result.Selection.NextAction = fmt.Sprintf("Matching authored work exists outside this scope. Repeat list_quality_work with scope=%q (or all) and expected_catalog_revision=catalog_revision from this result; inspect its evidence before adoption.", result.Selection.AvailableScopes[0])
	default:
		result.Selection.Status = "untracked"
		result.Selection.NextAction = "Read just-in-time-conversion/contribution-loop; prepare docs/app-kit.md or an existing issue for consumer review with original exact query/profile, task, capability_id, revision and catalog_revision. Lookup writes nothing."
	}
	if err := checkJSONLimit("list", result, maxListJSONBytes); err != nil {
		return ListResult{}, err
	}
	return result, nil
}

// Get returns a detached complete authored item. expectedRevision may be empty;
// when supplied it must be the exact current lowercase SHA-256 revision.
func (s *Snapshot) Get(id, expectedRevision string) (DetailResult, error) {
	if s == nil || s.revision == "" {
		return DetailResult{}, fmt.Errorf("qualityplan: uninitialized snapshot")
	}
	if err := validateID(id); err != nil {
		return DetailResult{}, fmt.Errorf("qualityplan: invalid item ID: %w", err)
	}
	if expectedRevision != "" {
		if !isLowerHash(expectedRevision) {
			return DetailResult{}, fmt.Errorf("qualityplan: invalid expected revision")
		}
		if expectedRevision != s.revision {
			return DetailResult{}, fmt.Errorf("qualityplan: revision mismatch: expected %s, current %s", expectedRevision, s.revision)
		}
	}
	i, ok := slices.BinarySearchFunc(s.items, id, func(item Item, id string) int { return strings.Compare(item.ID, id) })
	if !ok {
		return DetailResult{}, fmt.Errorf("qualityplan: unknown item %q", id)
	}
	item, err := cloneJSON(s.items[i])
	if err != nil {
		return DetailResult{}, fmt.Errorf("qualityplan: detach item: %w", err)
	}
	standards, err := cloneJSON(s.standards)
	if err != nil {
		return DetailResult{}, fmt.Errorf("qualityplan: detach standards: %w", err)
	}
	policy, err := cloneJSON(s.backlogPolicy)
	if err != nil {
		return DetailResult{}, fmt.Errorf("qualityplan: detach backlog policy: %w", err)
	}
	result := DetailResult{
		Revision: s.revision, EnhancementStandardRevision: s.enhancementStandardRevision,
		CatalogRevision: s.catalogRevision,
		ClaimBoundary:   claimBoundary, Item: item, Standards: standards, BacklogPolicy: policy,
	}
	if err := checkJSONLimit("detail", result, maxDetailJSONBytes); err != nil {
		return DetailResult{}, err
	}
	return result, nil
}

// ListForCatalog refuses a changed catalog before selecting authored work.
// Empty expectedCatalogRevision retains List's current-snapshot behavior.
func (s *Snapshot) ListForCatalog(capabilityID, scope, expectedCatalogRevision string) (ListResult, error) {
	if err := s.checkCatalogRevision(expectedCatalogRevision); err != nil {
		return ListResult{}, err
	}
	return s.List(capabilityID, scope)
}

// GetForCatalog independently checks capability and inventory identities before
// returning an item. Empty guards retain the corresponding current lookup.
func (s *Snapshot) GetForCatalog(id, expectedRevision, expectedCatalogRevision string) (DetailResult, error) {
	if err := s.checkCatalogRevision(expectedCatalogRevision); err != nil {
		return DetailResult{}, err
	}
	return s.Get(id, expectedRevision)
}

func (s *Snapshot) checkCatalogRevision(expected string) error {
	if s == nil || s.revision == "" || s.catalogRevision == "" {
		return fmt.Errorf("qualityplan: uninitialized snapshot")
	}
	if expected == "" {
		return nil
	}
	if !isLowerHash(expected) {
		return fmt.Errorf("qualityplan: invalid expected_catalog_revision; use the exact catalog revision from capability discovery")
	}
	if expected != s.catalogRevision {
		return fmt.Errorf("qualityplan: catalog revision mismatch: expected %s, current %s; restart capability discovery and review before quality lookup", expected, s.catalogRevision)
	}
	return nil
}

func validateID(id string) error {
	if len(id) > 96 || !idPattern.MatchString(id) {
		return fmt.Errorf("must match [a-z][a-z0-9._-]{0,95}")
	}
	return nil
}

func isLowerHash(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && value == strings.ToLower(value)
}

func checkJSONLimit(label string, value any, limit int) error {
	body, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("qualityplan: encode %s: %w", label, err)
	}
	if len(body) > limit {
		return fmt.Errorf("qualityplan: %s JSON exceeds %d bytes", label, limit)
	}
	return nil
}

func cloneJSON[T any](value T) (T, error) {
	var clone T
	body, err := json.Marshal(value)
	if err != nil {
		return clone, err
	}
	if err := json.Unmarshal(body, &clone); err != nil {
		return clone, err
	}
	return clone, nil
}
