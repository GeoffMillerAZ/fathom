package mcp

// review_layers.go — GL-22 (docs/plans/governance-learning.md, design doc
// §8 "Layered diffs"): review_diff groups the changed files into layers by
// blast radius, ordered for review, with no model — a reviewer reads the
// leaves first, then the shared callees other code depends on, then the
// providers other services call. Every input is already computed for the
// verdict (the service's call graph, the cross-service hops into
// changed_files), so this adds no store read.

import (
	"sort"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/analysis/serviceref"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// The three layers, in review order.
const (
	diffLayerLeaf     = "leaf"
	diffLayerShared   = "shared-callee"
	diffLayerProvider = "cross-service-provider"
)

// diffLayer is one layer of a layered diff.
type diffLayer struct {
	Layer string          `json:"layer"`
	Files []diffLayerFile `json:"files"`
}

// diffLayerFile is one changed file and the evidence that placed it:
// CallerFiles counts the distinct non-test files outside it that call one
// of its symbols; Consumers names the services whose client calls join a
// route it registers.
type diffLayerFile struct {
	File        string   `json:"file"`
	CallerFiles int      `json:"caller_files"`
	Consumers   []string `json:"consumers,omitempty"`
}

// layerDiffByBlastRadius places each changed file in exactly one layer:
// cross-service-provider when another service calls a route it registers
// (hops), else shared-callee when a symbol it defines is called from
// another non-test file of the service, else leaf. A test file calling a
// symbol does not make it shared — tests are the change's own proof, not
// its dependents. Layers come in review order (leaf, shared-callee,
// cross-service-provider), empty ones omitted, files sorted by path.
func layerDiffByBlastRadius(g *callgraph.Graph, nodes []core.SymbolNode, changedFiles []string, hops []serviceref.Hop) []diffLayer {
	fileOf := make(map[string]string, len(nodes))
	for _, n := range nodes {
		fileOf[n.Key()] = n.FilePath
	}
	callers := map[string]map[string]bool{}
	if g != nil {
		for _, n := range nodes {
			for _, d := range callgraph.CallersDetail(g, n.Key()) {
				from := fileOf[d.Key]
				if from == "" || from == n.FilePath || isTestPath(from) {
					continue
				}
				if callers[n.FilePath] == nil {
					callers[n.FilePath] = map[string]bool{}
				}
				callers[n.FilePath][from] = true
			}
		}
	}
	consumers := map[string]map[string]bool{}
	for _, h := range hops {
		if consumers[h.ToFile] == nil {
			consumers[h.ToFile] = map[string]bool{}
		}
		consumers[h.ToFile][h.FromService] = true
	}

	byLayer := map[string][]diffLayerFile{}
	seen := map[string]bool{}
	for _, f := range changedFiles {
		if seen[f] {
			continue
		}
		seen[f] = true
		entry := diffLayerFile{File: f, CallerFiles: len(callers[f])}
		for svc := range consumers[f] {
			entry.Consumers = append(entry.Consumers, svc)
		}
		sort.Strings(entry.Consumers)
		layer := diffLayerLeaf
		switch {
		case len(entry.Consumers) > 0:
			layer = diffLayerProvider
		case entry.CallerFiles > 0:
			layer = diffLayerShared
		}
		byLayer[layer] = append(byLayer[layer], entry)
	}
	var out []diffLayer
	for _, name := range []string{diffLayerLeaf, diffLayerShared, diffLayerProvider} {
		files := byLayer[name]
		if len(files) == 0 {
			continue
		}
		sort.Slice(files, func(i, j int) bool { return files[i].File < files[j].File })
		out = append(out, diffLayer{Layer: name, Files: files})
	}
	return out
}
