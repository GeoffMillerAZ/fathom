package assetrefs

import (
	"context"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

func objectID(guid string, id *int64) core.AssetObjectID {
	value := ""
	if id != nil {
		value = strconv.FormatInt(*id, 10)
	}
	return core.AssetObjectID{GUID: guid, FileID: value}
}
func validID(id core.AssetObjectID) bool {
	if !guidPattern.MatchString(id.GUID) || len(id.FileID) > 20 {
		return false
	}
	n, err := strconv.ParseInt(id.FileID, 10, 64)
	return err == nil && n != 0 && strconv.FormatInt(n, 10) == id.FileID
}
func objectKey(id core.AssetObjectID) string { return id.GUID + "#" + id.FileID }
func sourceKey(p string, id *int64) string {
	if id == nil {
		return p + "\x00<missing>"
	}
	return p + "\x00" + strconv.FormatInt(*id, 10)
}

type tuple struct {
	Source core.AssetObjectID
	Target core.AssetObjectID
}

func tupleKey(t tuple) string { return objectKey(t.Source) + "\x00" + objectKey(t.Target) }

type referenceGroup struct {
	Tuple  tuple
	Path   string
	Raw    []core.AssetDeclaredLocation
	Native []string
}

func normalize(ctx context.Context, opts Options, i inventory, n oracle, inputs map[string]string) (core.AssetReferenceSnapshot, error) {
	var empty core.AssetReferenceSnapshot
	governor := tenancy.NewRowGovernor(ctx, tenancy.ClassTree)
	snapshot := core.AssetReferenceSnapshot{Schema: SnapshotSchema, Family: "unity-declared", Service: opts.Service, Project: opts.Project, FamilyID: opts.FamilyID, Scopes: append([]string(nil), i.Scopes...), Entities: []core.AssetObject{}, References: []core.AssetReference{}, Gaps: []core.AssetReferenceGap{}}
	snapshot.SkippedMetadata = append([]string(nil), i.Skipped...)
	slices.Sort(snapshot.SkippedMetadata)
	snapshot.Limitations = []string{"Selected non-scene authored cohort; no complete asset or language grade.", "Declared/native occurrences are grouped; raw locations and native properties are unpaired.", "Native-only and unbound package targets are disclosed, not admitted declared edges.", "No code calls, callback execution, test certification or art approval."}
	snapshot.Limitations = append(snapshot.Limitations, "Input-current under bound authored code/settings and recorded trusted Editor/target; complete external package/tool-bundle semantics and current interactive Editor state are not established.")
	rawSources := map[string]bool{}
	sourcePaths := map[string]bool{}
	for _, row := range i.Entities {
		if err := governor.Row(); err != nil {
			return empty, err
		}
		if !entitySourcePath(row.Path) || !inScope(row.Path, i.Scopes) || inputs[row.Path] == "" || row.FileID == nil || *row.FileID == 0 || row.ClassID <= 0 {
			return empty, refused("unsupported/unbound raw source entity")
		}
		key := sourceKey(row.Path, row.FileID)
		if rawSources[key] {
			return empty, refused("duplicate raw source fileID")
		}
		rawSources[key] = true
		sourcePaths[row.Path] = true
	}
	nativeSources := map[string]core.AssetObjectID{}
	objects := map[string]core.AssetObject{}
	for _, row := range n.Entities {
		if err := governor.Row(); err != nil {
			return empty, err
		}
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		key := sourceKey(row.Path, row.FileID)
		asset, ok := i.Assets[row.Path]
		id := objectID(row.GUID, row.FileID)
		if !rawSources[key] || !ok || asset.GUID != row.GUID || !validID(id) || !safeText(row.TypeName) || row.TypeName == "" || len(row.TypeName) > 1024 {
			return empty, refused("mixed/unbound native source identity")
		}
		if _, duplicate := nativeSources[key]; duplicate {
			return empty, refused("duplicate native source entity")
		}
		if _, duplicate := objects[objectKey(id)]; duplicate {
			return empty, refused("duplicate native persistent identity")
		}
		nativeSources[key] = id
		objects[objectKey(id)] = core.AssetObject{ID: id, Path: path.Join(opts.Project, row.Path), Basis: "native-source", TypeName: row.TypeName}
	}
	if len(rawSources) != len(nativeSources) {
		return empty, refused("incomplete native source enumeration")
	}
	assetSeen := map[string]bool{}
	for _, row := range n.Assets {
		if !sourcePaths[row.Path] || assetSeen[row.Path] || i.Assets[row.Path].GUID != row.GUID || !guidPattern.MatchString(row.DependencyHash) {
			return empty, refused("mixed/duplicate native source asset")
		}
		assetSeen[row.Path] = true
		if len(row.Dependencies) == 0 || len(row.Dependencies) > memberLimit {
			return empty, refused("missing/overbound native dependencies")
		}
		lastDependency := ""
		includesSelf := false
		for _, dependency := range row.Dependencies {
			if !relative(dependency) || dependency <= lastDependency {
				return empty, refused("unsafe/duplicate/unsorted native dependency")
			}
			lastDependency = dependency
			if dependency == row.Path {
				includesSelf = true
			}
			if strings.HasPrefix(dependency, "Assets/") && inputs[dependency] == "" {
				return empty, refused("unbound authored native dependency")
			}
		}
		if !includesSelf {
			return empty, refused("native dependency inventory omits source asset")
		}
	}
	if len(assetSeen) != len(sourcePaths) {
		return empty, refused("incomplete native dependency enumeration")
	}
	groups := map[string]*referenceGroup{}
	getGroup := func(t tuple) *referenceGroup {
		key := tupleKey(t)
		group := groups[key]
		if group == nil {
			group = &referenceGroup{Tuple: t}
			groups[key] = group
		}
		return group
	}
	locations := map[string]bool{}
	for _, row := range i.References {
		if err := governor.Row(); err != nil {
			return empty, err
		}
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		source, ok := nativeSources[sourceKey(row.Source, row.SourceID)]
		if !ok || row.Line <= 0 || row.Line > 10000000 || !safeText(row.Field) || row.Field == "" || len(row.Field) > 4096 {
			return empty, refused("invalid raw reference origin")
		}
		location := row.Source + "\x00" + strconv.Itoa(row.Line)
		if locations[location] {
			return empty, refused("duplicate raw reference location")
		}
		locations[location] = true
		if row.FileID == nil {
			return empty, refused("missing raw fileID")
		}
		if row.Target != "" && !assetPath(row.Target) {
			return empty, refused("unsafe raw target path")
		}
		if *row.FileID == 0 {
			if row.Status != "null" {
				return empty, refused("mixed raw null status")
			}
			snapshot.Counts.RawNull++
			continue
		}
		switch row.Status {
		case "resolved_local", "resolved_serialized", "unknown_imported_or_outside_parse_scope_id", "external_or_missing_guid":
		default:
			return empty, refused("unknown/invalid raw target identity")
		}
		guid := row.GUID
		if guid == "" {
			guid = source.GUID
		}
		target := objectID(guid, row.FileID)
		if !validID(target) {
			return empty, refused("invalid raw target GUID/fileID")
		}
		group := getGroup(tuple{source, target})
		group.Raw = append(group.Raw, core.AssetDeclaredLocation{Line: row.Line, Field: row.Field})
		snapshot.Counts.RawNonnull++
	}
	properties := map[string]bool{}
	// A persistent GUID owns one asset path across the entire cohort, not
	// merely inside one source/target tuple or the admitted-edge branch.
	// Seed authored ownership before looking at package/native-only targets.
	targetOwners := map[string]string{}
	for _, assetPath := range sortedKeys(i.Assets) {
		targetOwners[i.Assets[assetPath].GUID] = assetPath
	}
	for _, row := range n.References {
		if err := governor.Row(); err != nil {
			return empty, err
		}
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		source, ok := nativeSources[sourceKey(row.Source, row.SourceID)]
		if !ok || source.GUID != row.SourceGUID || row.Property == "" || !safeText(row.Property) || len(row.Property) > 4096 {
			return empty, refused("invalid native property origin")
		}
		propertyKey := objectKey(source) + "\x00" + row.Property
		if properties[propertyKey] {
			return empty, refused("duplicate native property identity")
		}
		properties[propertyKey] = true
		if row.Available == nil || row.TargetID == nil {
			return empty, refused("missing native availability/fileID")
		}
		if !*row.Available {
			if row.TargetGUID != "" || *row.TargetID != 0 || row.TargetPath != "" {
				return empty, refused("conflicting unavailable native target")
			}
			snapshot.Counts.NativeUnavailable++
			continue
		}
		target := objectID(row.TargetGUID, row.TargetID)
		if !validID(target) || !relative(row.TargetPath) {
			return empty, refused("missing available native target identity")
		}
		if owner, known := targetOwners[row.TargetGUID]; known && owner != row.TargetPath {
			return empty, refused("conflicting global native GUID owner path")
		}
		targetOwners[row.TargetGUID] = row.TargetPath
		group := getGroup(tuple{source, target})
		if group.Path != "" && group.Path != row.TargetPath {
			return empty, refused("conflicting native target path")
		}
		group.Path = row.TargetPath
		group.Native = append(group.Native, row.Property)
		snapshot.Counts.NativeAvailable++
	}
	for _, key := range sortedKeys(groups) {
		if err := governor.Row(); err != nil {
			return empty, err
		}
		group := groups[key]
		slices.Sort(group.Native)
		slices.SortFunc(group.Raw, func(a, b core.AssetDeclaredLocation) int {
			if a.Line < b.Line {
				return -1
			}
			if a.Line > b.Line {
				return 1
			}
			return strings.Compare(a.Field, b.Field)
		})
		if len(group.Raw) > len(group.Native) {
			return empty, refused("raw-only or missing native target occurrence")
		}
		matched := len(group.Raw)
		nativeOnly := len(group.Native) - matched
		snapshot.Counts.Matched += matched
		snapshot.Counts.NativeOnly += nativeOnly
		if nativeOnly > 0 {
			snapshot.Gaps = append(snapshot.Gaps, core.AssetReferenceGap{Reason: "native-only-not-declared", Source: group.Tuple.Source, Target: group.Tuple.Target, TargetPath: group.Path, Occurrences: nativeOnly, NativeProperties: append([]string(nil), group.Native...)})
		}
		if matched == 0 {
			continue
		}
		targetAsset, mapped := i.Assets[group.Path]
		bound := mapped && targetAsset.GUID == group.Tuple.Target.GUID && inputs[group.Path] != "" && inputs[targetAsset.Meta] != ""
		if !bound {
			if strings.HasPrefix(group.Path, "Assets/") {
				return empty, refused("unbound/missing authored target GUID/fileID")
			}
			snapshot.Counts.NativeObservedUnbound += matched
			snapshot.Gaps = append(snapshot.Gaps, core.AssetReferenceGap{Reason: "native-observed-unbound", Source: group.Tuple.Source, Target: group.Tuple.Target, TargetPath: group.Path, Occurrences: matched, DeclaredLocations: group.Raw, NativeProperties: group.Native})
			continue
		}
		if sourcePaths[group.Path] {
			id, err := strconv.ParseInt(group.Tuple.Target.FileID, 10, 64)
			if err != nil {
				return empty, refused("invalid target fileID")
			}
			if _, ok := nativeSources[sourceKey(group.Path, &id)]; !ok {
				return empty, refused("unknown local target fileID")
			}
		}
		object := core.AssetObject{ID: group.Tuple.Target, Path: path.Join(opts.Project, group.Path), Basis: "observed-target"}
		if old, exists := objects[objectKey(object.ID)]; exists {
			if old.Path != object.Path {
				return empty, refused("conflicting object owner path")
			}
		} else {
			objects[objectKey(object.ID)] = object
		}
		sourceObject := objects[objectKey(group.Tuple.Source)]
		snapshot.References = append(snapshot.References, core.AssetReference{Kind: "declared-object-reference", Source: group.Tuple.Source, Target: group.Tuple.Target, SourcePath: sourceObject.Path, TargetPath: object.Path, MatchedOccurrences: matched, DeclaredLocations: group.Raw, NativeProperties: group.Native})
		snapshot.Counts.AdmittedMatched += matched
	}
	if snapshot.Counts.Matched == 0 {
		return empty, refused("empty independently matched cohort")
	}
	for _, key := range sortedKeys(objects) {
		snapshot.Entities = append(snapshot.Entities, objects[key])
	}
	return snapshot, nil
}
