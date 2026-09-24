package conserve

// surface.go — the "surface" plane (ADR-044 §2.1-§2.3, I44/I45): the
// capability inventory's base→head delta, excused by a plan's prior
// retires:/surface_change: declaration, plus ConsumerLosses (CV-20, §2.3):
// a removed or changed `route:` key that another service's client code
// still calls names the caller.
//
// Registers itself from init() (conserve.go's own doc comment: "two
// planes landing in the same wave from different workers never conflict
// on this file").

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/inventory"
	"github.com/geoffmilleraz/fathom/internal/analysis/serviceref"
	"github.com/geoffmilleraz/fathom/internal/config"
)

func init() {
	Register(surfacePlane{})
}

type surfacePlane struct{}

func (surfacePlane) Name() string { return "surface" }

// Check computes the capability inventory at Base and Head over
// in.ChangedFiles (design doc §0: "base facts come from the changed
// files only"), diffs them (inventory.Diff), and reports a Finding for
// every removed/changed key — Declared from in.Declarations, exactly as
// I45 requires. A removed/changed `route:` key additionally gets
// ConsumerLosses' cross-repo consumer findings appended. Missing is
// non-empty only when Input itself cannot read file content at all (a
// caller-built Input with no BaseFile/HeadFile) — the plane's core
// mechanism needs nothing else (no store, no typed pass), so it is
// UNPROVEN far less often than integrity/absence.
func (surfacePlane) Check(ctx context.Context, in Input) ([]Finding, []string) {
	if in.BaseFile == nil || in.HeadFile == nil {
		return nil, []string{"no base/head file reader"}
	}

	var baseFiles, headFiles []inventory.File
	for _, path := range in.ChangedFiles {
		if b, ok, err := in.BaseFile(path); err == nil && ok {
			baseFiles = append(baseFiles, inventory.File{Path: path, Content: b})
		}
		if h, ok, err := in.HeadFile(path); err == nil && ok {
			headFiles = append(headFiles, inventory.File{Path: path, Content: h})
		}
	}

	patterns := repoSurfacePatterns(in.Root)
	baseItems := inventory.Extract(baseFiles, patterns)
	headItems := inventory.Extract(headFiles, patterns)
	delta := inventory.Diff(baseItems, headItems)

	baseByKey := make(map[string]inventory.Item, len(baseItems))
	for _, it := range baseItems {
		baseByKey[it.Key] = it
	}
	headByKey := make(map[string]inventory.Item, len(headItems))
	for _, it := range headItems {
		headByKey[it.Key] = it
	}

	var findings []Finding
	for _, key := range delta.Removed {
		it := baseByKey[key]
		findings = append(findings, Finding{
			Plane: "surface", Kind: "removed", Key: key,
			Detail:   it.Detail,
			Base:     citeOf(it.File, it.Line),
			Declared: in.Declarations.Declared(key),
			Remedy:   fmt.Sprintf("add `retires: %s` to the row that removed it, or restore it", key),
		})
		findings = append(findings, ConsumerLosses(ctx, in, key)...)
	}
	for _, key := range delta.Changed {
		bi, hi := baseByKey[key], headByKey[key]
		findings = append(findings, Finding{
			Plane: "surface", Kind: "changed", Key: key,
			Detail:   fmt.Sprintf("%s -> %s", bi.Detail, hi.Detail),
			Base:     citeOf(bi.File, bi.Line),
			Head:     citeOf(hi.File, hi.Line),
			Declared: in.Declarations.Declared(key),
			Remedy:   fmt.Sprintf("add `surface_change: %s` to the row that changed it, or revert the change", key),
		})
		findings = append(findings, ConsumerLosses(ctx, in, key)...)
	}

	return findings, nil
}

// repoSurfacePatterns best-effort reads root's `.fathom/config.toml` for
// `[[surface.pattern]]` entries (CV-17). A missing/unreadable config, or
// an empty root, yields no patterns — the built-in kinds keep working
// regardless.
func repoSurfacePatterns(root string) []inventory.Pattern {
	if root == "" {
		return nil
	}
	loaded, err := config.Load("", filepath.Join(root, ".fathom", "config.toml"))
	if err != nil {
		return nil
	}
	src := config.SurfacePatterns(loaded.Config)
	if len(src) == 0 {
		return nil
	}
	out := make([]inventory.Pattern, len(src))
	for i, p := range src {
		out[i] = inventory.Pattern{Kind: p.Kind, Glob: p.Glob, Regex: p.Regex}
	}
	return out
}

// ConsumerLosses reports which OTHER services' client code still calls a
// removed or changed `route:` key (CV-20, ADR-044 §2.3): key's own
// "METHOD path" names the exact join target — internal/store has no
// content-serving method (it persists symbols, never raw file bytes), so
// the client side is read fresh off disk under each other known
// service's own recorded corpus root, the same "known files, re-read
// from disk" pattern internal/certify.resolveBehaviour uses. Only
// `route:` keys are checked; every other kind returns nil immediately. A
// nil Store, or a service whose corpus root cannot be resolved, or a
// malformed key, degrades to no findings rather than an error — this is
// an ADDITIONAL, best-effort fact ConsumerLosses layers onto the surface
// plane's own base/head diff, never a reason for the plane itself to go
// unproven.
func ConsumerLosses(ctx context.Context, in Input, key string) []Finding {
	rest, ok := strings.CutPrefix(key, "route:")
	if !ok || in.Store == nil {
		return nil
	}
	method, template, ok := parseRouteKey(rest)
	if !ok {
		return nil
	}

	ids, err := in.Store.ListServiceIDs(ctx)
	if err != nil {
		return nil
	}

	var findings []Finding
	for _, id := range ids {
		if id == in.Service {
			continue
		}
		meta, found, merr := in.Store.GetServiceMeta(ctx, id)
		if merr != nil || !found || meta.Path == "" {
			continue
		}
		for _, call := range walkClientCalls(meta.Path) {
			if call.Method != method || call.Template != template {
				continue
			}
			findings = append(findings, Finding{
				Plane: "surface", Kind: "consumer_loss", Key: key,
				Detail:   fmt.Sprintf("%s still calls %s %s", id, method, template),
				Head:     citeOf(call.File, call.Line),
				Declared: in.Declarations.Declared(key),
				Remedy:   fmt.Sprintf("coordinate the removal of %s with %s before merging, or restore it", key, id),
			})
		}
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Head != findings[j].Head {
			return findings[i].Head < findings[j].Head
		}
		return findings[i].Detail < findings[j].Detail
	})
	return findings
}

// parseRouteKey splits a `route:` inventory key's remainder ("METHOD
// path") into (method, normalized template) — normalized through
// internal/analysis/serviceref.NormalizeTemplate, the SAME normalization
// ExtractClientCalls already applies to every client call's own Template,
// so a server route written ":id"-style and a client call written
// "%s"-style compare equal.
func parseRouteKey(rest string) (method, template string, ok bool) {
	m, p, found := strings.Cut(rest, " ")
	if !found || m == "" || p == "" {
		return "", "", false
	}
	return m, serviceref.NormalizeTemplate(p), true
}

// walkClientCalls re-reads every non-test .go file under root (vendor/
// testdata/hidden directories skipped, mirroring internal/platformlink.
// BuildPlatformGraph's own walk) and extracts outbound HTTP client calls
// via internal/analysis/serviceref.ExtractClientCalls (Go-only, that
// package's own documented scope today). An unreadable file, or a root
// that does not exist at all, yields no calls rather than an error —
// root may legitimately be stale (a service recorded at ingest time,
// since moved or removed).
func walkClientCalls(root string) []serviceref.ClientCall {
	var out []serviceref.ClientCall
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // best-effort: skip unreadable entries, never abort the walk
		}
		if d.IsDir() {
			name := d.Name()
			if name == "vendor" || name == "testdata" || (name != "." && strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}
		out = append(out, serviceref.ExtractClientCalls("go", filepath.ToSlash(rel), src)...)
		return nil
	})
	return out
}
