package assetrefs

import (
	"bytes"
	"context"
	"encoding/json"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// Prepare validates a genuinely producer-bound raw/native partial cohort. It
// returns no snapshot on refusal and makes no store/service/config mutation.
// The caller must supply trusted local root/tool identity and an active marker.
func Prepare(ctx context.Context, opts Options) (core.AssetReferenceSnapshot, error) {
	var empty core.AssetReferenceSnapshot
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	if err := tenancy.ProducerAllowsCtx(ctx, tenancy.ClassTree); err != nil {
		return empty, err
	}
	if !filepath.IsAbs(opts.Root) || filepath.Clean(opts.Root) != opts.Root || !safeText(opts.Service) || opts.Service == "" || len(opts.Service) > 512 || (opts.Project != "." && !relative(opts.Project)) || !keyPattern.MatchString(opts.FamilyID) || !artifactPath(opts.ReceiptPath) || !artifactPath(opts.ActivePath) || opts.ReceiptPath == opts.ActivePath {
		return empty, refused("missing/noncanonical caller scope")
	}
	if opts.ExpectedEditorVersion == "" || !safeText(opts.ExpectedEditorVersion) || !shaPattern.MatchString(opts.ExpectedEditorSHA256) || opts.ExpectedBuildTarget == "" || !safeText(opts.ExpectedBuildTarget) {
		return empty, refused("missing expected local Editor identity/target")
	}
	activeRaw, err := readRegular(ctx, opts.Root, opts.ActivePath, artifactLimit)
	if err != nil {
		return empty, err
	}
	var a active
	if err := strictJSON(activeRaw, &a); err != nil {
		return empty, err
	}
	if a.Schema != ActiveSchema || a.Phase != "complete" || a.Project != opts.Project || a.FamilyID != opts.FamilyID || !keyPattern.MatchString(a.RunID) || a.Receipt.Path != opts.ReceiptPath {
		return empty, refused("missing/incomplete/wrong active attempt")
	}
	receiptRaw, err := readHash(ctx, opts.Root, a.Receipt, artifactLimit)
	if err != nil {
		return empty, err
	}
	var r receipt
	if err := strictJSON(receiptRaw, &r); err != nil {
		return empty, err
	}
	if err := checkReceipt(opts, a, r); err != nil {
		return empty, err
	}
	invRaw, err := readHash(ctx, opts.Root, r.Inventory, artifactLimit)
	if err != nil {
		return empty, err
	}
	nativeRaw, err := readHash(ctx, opts.Root, r.Oracle, artifactLimit)
	if err != nil {
		return empty, err
	}
	var i inventory
	var n oracle
	if err := strictJSON(invRaw, &i); err != nil {
		return empty, err
	}
	if err := strictJSON(nativeRaw, &n); err != nil {
		return empty, err
	}
	inputs, err := checkArtifacts(opts, r, i, n)
	if err != nil {
		return empty, err
	}
	if err := validateSource(ctx, opts, r, i); err != nil {
		return empty, err
	}
	projectRoot := filepath.Join(opts.Root, filepath.FromSlash(opts.Project))
	versionBytes, err := readHash(ctx, projectRoot, fileHash{"ProjectSettings/ProjectVersion.txt", inputs["ProjectSettings/ProjectVersion.txt"]}, inputLimit)
	if err != nil {
		return empty, err
	}
	versionLine := strings.SplitN(string(versionBytes), "\n", 2)[0]
	if versionLine != "m_EditorVersion: "+opts.ExpectedEditorVersion {
		return empty, refused("Editor differs from actual project pin")
	}
	if _, err := readHash(ctx, projectRoot, r.OracleAssembly, artifactLimit); err != nil {
		return empty, err
	}
	if err := validateMetadata(ctx, projectRoot, i, inputs); err != nil {
		return empty, err
	}
	snapshot, err := normalize(ctx, opts, i, n, inputs)
	if err != nil {
		return empty, err
	}
	// Close ordinary change windows across normalization; this is not a hostile
	// filesystem transaction or a cryptographic producer signature.
	if err := validateSource(ctx, opts, r, i); err != nil {
		return empty, err
	}
	for _, file := range []fileHash{a.Receipt, r.Inventory, r.Oracle, {opts.ActivePath, digest(activeRaw)}} {
		if _, err := readHash(ctx, opts.Root, file, artifactLimit); err != nil {
			return empty, err
		}
	}
	if _, err := readHash(ctx, projectRoot, r.OracleAssembly, artifactLimit); err != nil {
		return empty, err
	}
	snapshot.ReceiptSHA256 = digest(receiptRaw)
	snapshot.SourceSHA256 = r.Source.Before
	snapshot.ContextSHA256 = r.ContextInventory.Before
	snapshot.EditorVersion = r.Editor.Version
	snapshot.BuildTarget = n.BuildTarget
	return snapshot, nil
}

func checkReceipt(opts Options, a active, r receipt) error {
	if r.Schema != ReceiptSchema || r.Status != "complete" || r.RunID != a.RunID || r.Project != opts.Project || r.FamilyID != opts.FamilyID || r.ExitCode == nil || *r.ExitCode != 0 {
		return refused("incomplete/mixed producer receipt")
	}
	if r.ContextInventory == nil {
		return refused("missing authored semantic context inventory")
	}
	if r.Producer.Name != ProducerName || r.Producer.Version != "1" || !relative(r.Producer.Path) || !shaPattern.MatchString(r.Producer.SHA256) || len(r.Producer.Inputs) == 0 || len(r.Producer.Inputs) > 32 {
		return refused("missing/wrong producer identity/closure")
	}
	last := ""
	for _, file := range r.Producer.Inputs {
		if !relative(file.Path) || !shaPattern.MatchString(file.SHA256) || file.Path <= last || file.Path == r.Producer.Path {
			return refused("duplicate/unsafe/unsorted producer inputs")
		}
		last = file.Path
	}
	if r.Editor.Version != opts.ExpectedEditorVersion || r.Editor.SHA256 != opts.ExpectedEditorSHA256 {
		return refused("wrong expected Editor identity")
	}
	start, err := time.Parse(time.RFC3339Nano, r.StartedAt)
	if err != nil {
		return refused("invalid attempt start")
	}
	finish, err := time.Parse(time.RFC3339Nano, r.FinishedAt)
	if err != nil || !finish.After(start) {
		return refused("invalid attempt interval")
	}
	if r.Coverage.FamilyComplete == nil || *r.Coverage.FamilyComplete || r.Coverage.SourceFormat != "selected-non-scene-authored-assets" || r.Coverage.Cohort != "declared-native-intersection" {
		return refused("unsupported coverage assertion")
	}
	if !artifactPath(r.Inventory.Path) || !artifactPath(r.Oracle.Path) || r.Inventory.Path == r.Oracle.Path || r.Inventory.Path == opts.ReceiptPath || r.Oracle.Path == opts.ReceiptPath || r.Inventory.Path == opts.ActivePath || r.Oracle.Path == opts.ActivePath {
		return refused("conflicting/unsafe raw artifacts")
	}
	if !relative(r.OracleAssembly.Path) || path.Dir(r.OracleAssembly.Path) != "Library/ScriptAssemblies" || path.Ext(r.OracleAssembly.Path) != ".dll" || !shaPattern.MatchString(r.OracleAssembly.SHA256) {
		return refused("invalid local oracle assembly identity")
	}
	return nil
}

func checkArtifacts(opts Options, r receipt, i inventory, n oracle) (map[string]string, error) {
	if i.Schema != "unity-declared-references/v1" || i.Parser != "stdlib-inline-map-subset/v1" || i.Project != filepath.Join(opts.Root, filepath.FromSlash(opts.Project)) || len(i.Problems) != 0 {
		return nil, refused("unsupported/incomplete inventory")
	}
	if err := checkScopes(i.Scopes); err != nil {
		return nil, err
	}
	if len(i.Entities) == 0 || len(i.Entities) > entityLimit || len(i.References) > referenceLimit || len(i.Assets) > memberLimit || len(i.Metadata) == 0 || len(i.Metadata) > memberLimit {
		return nil, refused("empty/overbound source cohort")
	}
	inputs, err := inputMap(i.Inputs)
	if err != nil {
		return nil, err
	}
	for _, skipped := range i.Skipped {
		if !relative(skipped) || !strings.HasPrefix(skipped, "Assets/") || !strings.HasSuffix(skipped, ".meta") {
			return nil, refused("unsafe skipped metadata disclosure")
		}
	}
	for _, required := range []string{"ProjectSettings/ProjectVersion.txt", "Packages/manifest.json", "Packages/packages-lock.json"} {
		if inputs[required] == "" {
			return nil, refused("missing project/package pin")
		}
	}
	if n.Schema != 1 || n.Oracle != "Unity SerializedObject/ObjectReference + AssetDatabase" || n.UnityVersion != r.Editor.Version || n.BuildTarget != opts.ExpectedBuildTarget || n.InventorySHA256 != r.Inventory.SHA256 || n.AssemblySHA256 != r.OracleAssembly.SHA256 || len(n.Unjudged) != 0 {
		return nil, refused("incomplete/mixed native oracle")
	}
	if len(n.Entities) == 0 || len(n.Entities) > entityLimit || len(n.References) > referenceLimit || len(n.Assets) > memberLimit {
		return nil, refused("empty/overbound native cohort")
	}
	nativeInputs, err := inputMap(n.Inputs)
	if err != nil {
		return nil, err
	}
	if len(inputs) != len(nativeInputs) {
		return nil, refused("mixed native input membership")
	}
	for p, hash := range inputs {
		if nativeInputs[p] != hash {
			return nil, refused("mixed native input identity")
		}
	}
	return inputs, nil
}

func validateMetadata(ctx context.Context, root string, i inventory, inputs map[string]string) error {
	if len(i.Assets) != len(i.Metadata) {
		return refused("incomplete metadata mapping")
	}
	seen := map[string]string{}
	last := ""
	metadataMembers := append([]string(nil), i.Metadata...)
	slices.Sort(metadataMembers)
	for _, meta := range metadataMembers {
		if !assetPath(meta) || !strings.HasSuffix(meta, ".meta") || meta <= last || inputs[meta] == "" {
			return refused("invalid metadata membership")
		}
		last = meta
		asset := strings.TrimSuffix(meta, ".meta")
		declared, ok := i.Assets[asset]
		if !ok || declared.Meta != meta || !guidPattern.MatchString(declared.GUID) {
			return refused("missing/invalid asset GUID mapping")
		}
		if old := seen[declared.GUID]; old != "" {
			return refused("duplicate GUID maps to multiple assets")
		}
		seen[declared.GUID] = asset
		raw, err := readHash(ctx, root, fileHash{meta, inputs[meta]}, inputLimit)
		if err != nil {
			return err
		}
		if !utf8.Valid(raw) || bytes.IndexByte(raw, 0) >= 0 {
			return refused("binary/nonUTF8 metadata")
		}
		matches := metaGUIDPattern.FindAllSubmatch(raw, -1)
		if len(matches) != 1 || string(matches[0][1]) != declared.GUID {
			return refused("missing/duplicate/mixed metadata GUID")
		}
	}
	return nil
}

// Query revalidates raw proof, active generation and current source before
// serving a bounded one-hop result. A supplied prepared DTO is not authority.
func Query(ctx context.Context, opts Options, q core.AssetReferenceQuery) (core.AssetReferenceResult, error) {
	var empty core.AssetReferenceResult
	if !validID(q.Object) || (q.Direction != "inbound" && q.Direction != "outbound") || q.Limit < 0 || q.Limit > 100 {
		return empty, refused("invalid asset query scope")
	}
	limit := q.Limit
	if limit == 0 {
		limit = 25
	}
	snapshot, err := Prepare(ctx, opts)
	if err != nil {
		return empty, err
	}
	result := core.AssetReferenceResult{Status: "current_partial", Service: opts.Service, Project: opts.Project, FamilyID: opts.FamilyID, Object: q.Object, Direction: q.Direction, ReceiptSHA256: snapshot.ReceiptSHA256, SourceSHA256: snapshot.SourceSHA256, ContextSHA256: snapshot.ContextSHA256, EditorVersion: snapshot.EditorVersion, BuildTarget: snapshot.BuildTarget, References: []core.AssetReference{}, Counts: snapshot.Counts}
	result.Limitations = append([]string(nil), snapshot.Limitations...)
	for _, reference := range snapshot.References {
		id := reference.Source
		if q.Direction == "inbound" {
			id = reference.Target
		}
		if id == q.Object {
			result.Total++
		}
	}
	result.Truncated = result.Total
	baseBytes, err := json.Marshal(result)
	if err != nil {
		return empty, refused("encode asset result: %v", err)
	}
	if len(baseBytes) > 16<<10 {
		return empty, refused("asset result metadata exceeds16KiB")
	}
	for _, reference := range snapshot.References {
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		id := reference.Source
		if q.Direction == "inbound" {
			id = reference.Target
		}
		if id != q.Object {
			continue
		}
		if len(result.References) >= limit {
			continue
		}
		result.References = append(result.References, reference)
		result.Truncated = result.Total - len(result.References)
		encoded, err := json.Marshal(result)
		if err != nil {
			return empty, refused("encode bounded asset result: %v", err)
		}
		if len(encoded) > 16<<10 {
			result.References = result.References[:len(result.References)-1]
			result.Truncated = result.Total - len(result.References)
		}
	}
	result.Truncated = result.Total - len(result.References)
	// The source may change while constructing even this bounded projection.
	// Prepare did its own post-check; repeat the active+raw/source validation
	// rather than returning apparently-current positive rows from a changed run.
	final, err := Prepare(ctx, opts)
	if err != nil {
		return empty, err
	}
	if final.ReceiptSHA256 != snapshot.ReceiptSHA256 || final.SourceSHA256 != snapshot.SourceSHA256 {
		return empty, refused("source/generation changed during query")
	}
	return result, nil
}

// Keep sorting explicit for deterministic source ordering at every wire seam.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
