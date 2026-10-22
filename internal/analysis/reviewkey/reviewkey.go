// Package reviewkey computes ONE token over every input a review verdict
// depends on (vision 30 §3.2, ADR-040 decision 6, docs/plans/
// findings-loop.md rows FL-17/FL-18): a caller (the review kit, or fathom's
// own `review-key` CLI/pkg/client.ReviewKey) uses it to decide whether a
// prior review result may be reused, without a second bespoke hashing API.
//
// Compute is pure over its Inputs: every file read, git walk, and store
// lookup happens in the caller (pkg/client.ReviewKey, shared by cmd/fathom/
// reviewkey.go — internal/AGENTS.md's injected-I/O convention). The one
// exception is the dependency closure, which Compute itself walks — over
// data the caller already resolved (Nodes/TypedEdges) — by reusing
// internal/analysis/callgraph's typed-edge-only graph (BuildResolved/
// CalleesDetail), the SAME machinery internal/analysis/impact's blast-radius
// analysis (and the blast_radius MCP tool) walks. Never a new import parser.
package reviewkey

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// SchemaVersion is Key's wire schema version (vision 30 §3.2: "All five are
// versioned with an explicit schema_version; a consumer that sees a version
// it does not know refuses").
const SchemaVersion = 1

// Component names — the fixed set Key.Components always carries (present
// even when its own Unknown entries mean it was computed over a partial
// view — see Key's doc comment).
const (
	ComponentManifest = "manifest"
	ComponentClosure  = "closure"
	ComponentRules    = "rules"
	ComponentPlans    = "plans"
	ComponentTooling  = "tooling"
	ComponentIndex    = "index"
)

// ManifestEntry is one changed path's current content digest — the same
// shape `fathom review --manifest FILE` accepts. An entry the caller could
// not read (missing, permission-denied, …) carries Unreadable=true and an
// empty SHA256; it is excluded from the manifest digest and recorded as an
// Unknown entry, never hashed as if it were an empty file.
type ManifestEntry struct {
	Path       string `json:"path"`
	SHA256     string `json:"sha256,omitempty"`
	Unreadable bool   `json:"unreadable,omitempty"`
}

// Key is the computed review key: one sha256 over every component's own
// digest, plus the honesty envelope (Complete/Unknown) that says whether any
// input the key SHOULD reflect could not actually be resolved.
type Key struct {
	SchemaVersion int `json:"schema_version"`
	// Key is the sha256 hex digest over Components (sorted "name=digest"
	// lines) — the one token a caller compares across runs.
	Key string `json:"key"`
	// Complete is false iff Unknown is non-empty: the key still computes
	// over whatever WAS resolved (a caller may reuse narrower results, or
	// refuse — reviewkey only reports), but at least one input could not be
	// verified.
	Complete bool `json:"complete"`
	// Unknown names every input this key could NOT verify, one entry each
	// (e.g. "manifest: docs/x.md is unreadable", "dependency closure:
	// pkg/foo has no typed edges"). Sorted, never nil-vs-empty significant
	// beyond len()==0.
	Unknown []string `json:"unknown,omitempty"`
	// Components is every named component's own digest — always the fixed
	// six names (ComponentManifest..ComponentIndex), even when Unknown
	// names a gap in that component's own input.
	Components map[string]string `json:"components"`
	// ChangedSincePrior is Diff(*Prior, this Key) — set only when a Prior
	// key was supplied to Compute.
	ChangedSincePrior []string `json:"changed_since_prior,omitempty"`
}

// Request is the caller-facing shape both `fathom review-key` and
// pkg/client.ReviewKey accept — the request half of the P1 contract (vision
// 30 §3.2); pkg/client.ReviewKey resolves it into Inputs (reading rules/
// plans files, corpus root, watermarks, …) before calling Compute.
type Request struct {
	Service  string
	Manifest []ManifestEntry
	// Prior is an earlier Key to diff against (ChangedSincePrior).
	Prior *Key
}

// Inputs bundles every already-resolved dependency Compute needs — the
// caller (pkg/client.ReviewKey) does every read: manifest entries, the
// corpus's current symbol/typed-edge set (for the dependency closure walk),
// per-file store fingerprints, the rule pack + guard.cue + layer manifests,
// docs/plans/** content, tooling identity, and index watermarks.
type Inputs struct {
	// Manifest is the changed-path digest set — see ManifestEntry's doc
	// comment for the unreadable-path handling.
	Manifest []ManifestEntry

	// Nodes/TypedEdges are the corpus's current symbol/typed-edge set.
	// Closure walks ONLY the typed (compiler-resolved) call graph forward
	// from every symbol defined in a readable manifest path — see the
	// package doc comment.
	Nodes      []core.SymbolNode
	TypedEdges []core.Edge
	// ResolvedGraph is the caller's immutable compiler-resolved graph.
	ResolvedGraph *callgraph.Graph
	// Fingerprint returns the store's current content fingerprint for a
	// repo-relative file (store.Fingerprint — the same view review's
	// PersistInput.ScopeFingerprint reads), digested per file the closure
	// walk reaches. nil, or a file it can't return, digests that file with
	// an empty fingerprint — honest best-effort, not a refusal: the closure
	// walk's own "no typed edges" Unknown entries are what name a REAL gap.
	Fingerprint func(path string) (hash string, ok bool)

	// RulesFiles/PlansFiles are pre-read (repo-relative path → content) sets
	// the caller resolved: the project rule pack
	// (.fathom/rules/archconformance.cue) + .fathom/guard.cue + any
	// project-local layer manifests (.fathom/layers/*/layer.cue) for
	// RulesFiles; docs/plans/** for PlansFiles. *Unknown names a path the
	// caller could not read (never hashed as empty).
	RulesFiles   map[string][]byte
	RulesUnknown []string
	PlansFiles   map[string][]byte
	PlansUnknown []string

	// Tooling is fathom's own identity — version/build plus extractor
	// grammar identities, as component=value pairs the caller resolved.
	Tooling map[string]string

	// ServiceWatermark/StoreWatermark are the index identity
	// (pkg/client.ServiceWatermark/StoreWatermark). IndexUnknown names why
	// index identity could not be resolved (e.g. the service was never
	// ingested) — the index component still computes over whatever these
	// two strings hold ("" is a valid, if uninformative, input).
	ServiceWatermark string
	StoreWatermark   string
	IndexUnknown     string

	// Prior is an earlier Key to diff against — see Key.ChangedSincePrior.
	Prior *Key
}

// Compute builds one Key over in. Deterministic: identical Inputs (however
// their maps/slices were constructed — Go map iteration order never leaks
// into the digest) always produce an identical Key.
func Compute(_ context.Context, in Inputs) (Key, error) {
	components := make(map[string]string, 6)
	var unknown []string

	manifestDigest, manifestUnknown := manifestComponent(in.Manifest)
	components[ComponentManifest] = manifestDigest
	unknown = append(unknown, manifestUnknown...)

	closureDigest, closureUnknown := closureComponent(in)
	components[ComponentClosure] = closureDigest
	unknown = append(unknown, closureUnknown...)

	components[ComponentRules] = digestFileSet(in.RulesFiles)
	for _, u := range in.RulesUnknown {
		unknown = append(unknown, "rules: "+u)
	}

	components[ComponentPlans] = digestFileSet(in.PlansFiles)
	for _, u := range in.PlansUnknown {
		unknown = append(unknown, "plans: "+u)
	}

	components[ComponentTooling] = digestKV(in.Tooling)

	components[ComponentIndex] = digestKV(map[string]string{
		"service_watermark": in.ServiceWatermark,
		"store_watermark":   in.StoreWatermark,
	})
	if in.IndexUnknown != "" {
		unknown = append(unknown, "index: "+in.IndexUnknown)
	}

	sort.Strings(unknown)

	k := Key{
		SchemaVersion: SchemaVersion,
		Complete:      len(unknown) == 0,
		Unknown:       unknown,
		Components:    components,
	}
	k.Key = digestKV(components)
	if in.Prior != nil {
		k.ChangedSincePrior = Diff(*in.Prior, k)
	}
	return k, nil
}

// Diff reports the component names whose digest differs between prior and
// current, sorted — FL-18's changed_since_prior.
func Diff(prior, current Key) []string {
	names := make(map[string]bool, len(prior.Components)+len(current.Components))
	for k := range prior.Components {
		names[k] = true
	}
	for k := range current.Components {
		names[k] = true
	}
	var out []string
	for name := range names {
		if prior.Components[name] != current.Components[name] {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// manifestComponent digests the sorted "path=sha256" lines of every
// READABLE manifest entry; an unreadable/empty-digest entry is excluded and
// named in the returned unknown list instead.
func manifestComponent(entries []ManifestEntry) (string, []string) {
	lines := make([]string, 0, len(entries))
	var unknown []string
	for _, e := range entries {
		if e.Unreadable || e.SHA256 == "" {
			unknown = append(unknown, fmt.Sprintf("manifest: %s is unreadable", e.Path))
			continue
		}
		lines = append(lines, e.Path+"="+e.SHA256)
	}
	sort.Strings(lines)
	return sha256Hex(strings.Join(lines, "\n")), unknown
}

// closureComponent walks the RESOLVED (typed) call graph forward from every
// symbol defined in a readable manifest path, collecting the set of OTHER
// files reached (never the changed files themselves — that is the
// manifest's own job), and digests their sorted store fingerprints. A
// changed file's package that carries no typed edge anywhere in
// in.TypedEdges is named as an unknown entry: the typed pass never ran over
// it (or this is a non-Go corpus), so the closure cannot vouch for its
// dependencies.
func closureComponent(in Inputs) (string, []string) {
	nodeByFile := make(map[string][]core.SymbolNode, len(in.Nodes))
	nodeByKey := make(map[string]core.SymbolNode, len(in.Nodes))
	for _, n := range in.Nodes {
		nodeByFile[n.FilePath] = append(nodeByFile[n.FilePath], n)
		nodeByKey[n.Key()] = n
	}

	changed := make(map[string]bool, len(in.Manifest))
	changedPkgs := map[string]bool{}
	for _, e := range in.Manifest {
		if e.Unreadable {
			continue
		}
		changed[e.Path] = true
		for _, n := range nodeByFile[e.Path] {
			changedPkgs[n.PackagePath()] = true
		}
	}

	pkgHasTypedEdges := map[string]bool{}
	for _, e := range in.TypedEdges {
		if n, ok := nodeByKey[e.FromSymbol]; ok {
			pkgHasTypedEdges[n.PackagePath()] = true
		}
	}
	var unknown []string
	for pkg := range changedPkgs {
		if !pkgHasTypedEdges[pkg] {
			unknown = append(unknown, fmt.Sprintf("dependency closure: %s has no typed edges", pkg))
		}
	}
	sort.Strings(unknown)

	g := in.ResolvedGraph
	if g == nil {
		g = callgraph.BuildResolved(in.Nodes, in.TypedEdges)
	}
	visited := map[string]bool{}
	queue := make([]string, 0, len(changed))
	for _, n := range in.Nodes {
		if changed[n.FilePath] {
			key := n.Key()
			if !visited[key] {
				visited[key] = true
				queue = append(queue, key)
			}
		}
	}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, callee := range callgraph.CalleesDetail(g, cur) {
			if !visited[callee.Key] {
				visited[callee.Key] = true
				queue = append(queue, callee.Key)
			}
		}
	}

	reachedFiles := map[string]bool{}
	for key := range visited {
		n, ok := nodeByKey[key]
		if !ok || changed[n.FilePath] {
			continue
		}
		reachedFiles[n.FilePath] = true
	}
	files := make([]string, 0, len(reachedFiles))
	for f := range reachedFiles {
		files = append(files, f)
	}
	sort.Strings(files)

	lines := make([]string, 0, len(files))
	for _, f := range files {
		hash := ""
		if in.Fingerprint != nil {
			if h, ok := in.Fingerprint(f); ok {
				hash = h
			}
		}
		lines = append(lines, f+"="+hash)
	}
	return sha256Hex(strings.Join(lines, "\n")), unknown
}

// digestFileSet hashes a set of (repo-relative path → content) pairs,
// sorted by path so the digest never depends on map iteration order.
func digestFileSet(files map[string][]byte) string {
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, p := range paths {
		h.Write([]byte(p))
		h.Write([]byte{0})
		h.Write(files[p])
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// digestKV hashes a plain string map as sorted "key=value" lines.
func digestKV(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines, k+"="+m[k])
	}
	return sha256Hex(strings.Join(lines, "\n"))
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
