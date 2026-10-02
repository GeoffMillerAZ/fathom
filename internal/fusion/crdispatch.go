package fusion

// crdispatch.go — ingest's claim of Argo, Flux and Crossplane custom
// resources (LB-F149, language-breadth iac wave, item k8s). Each is a
// doc-engine dialect (internal/extractor/dialects/{argo,flux,crossplane}) whose
// doc spec names, in detect.content_sniff, the apiVersion API groups (and, for
// Argo, the kinds) that make a document one. Every such file also opens as a
// Kubernetes object (a top-level apiVersion and kind), so Classify says
// ToolK8sManifest; the dispatch preempts that verdict, and only that verdict.
//
// A file is claimed only when ALL of these hold:
//   - it is a .yaml or .yml file Classify gives ToolK8sManifest (a Helm
//     template of a chart, a CRD, a Kustomize file, a Compose file and every
//     file a basename or glob claims keep their verdicts);
//   - every non-empty document of the file passes ONE dialect's sniff (a
//     multi-document file is claimed only when it is wholly that dialect's).
//
// A file with documents of one of these dialects AND others (a Flux
// Kustomization beside a Namespace, an Argo Application beside a Deployment)
// stays Kubernetes: it is mixed, counted by CRMixed, never split. A claimed
// document keeps the object entity the kubernetes dialect gives (the
// `Kind/<namespace>/<name>` name, the K8sWorkload source kind), so the Service
// selector, route-host and image joins that key off it are unchanged.
//
// crDialects is a table so another wave's CR dialects (gatekeeper, kyverno)
// append an id here and need no further edit in resolveExtractor or
// ReingestFile: both call crDispatch once, beside gatewayDispatch.

import (
	"bytes"
	"path/filepath"
	"strings"
	"sync"

	"github.com/geoffmilleraz/fathom/internal/extractor"
)

const (
	// ToolArgo: an Argo CD (Application, ApplicationSet, AppProject) or Argo
	// Workflows (Workflow, WorkflowTemplate, ClusterWorkflowTemplate,
	// CronWorkflow) object file (LB-Q-argo).
	ToolArgo ToolKind = "argo"
	// ToolFlux: a Flux (toolkit.fluxcd.io) object file (LB-Q-flux).
	ToolFlux ToolKind = "flux"
	// ToolCrossplane: a Crossplane definition, composition or package file
	// (apiextensions.crossplane.io, pkg.crossplane.io) (LB-Q-crossplane).
	ToolCrossplane ToolKind = "crossplane"
)

// crDialect is one custom-resource dialect the dispatch may claim a file for.
type crDialect struct {
	id   string
	kind ToolKind
}

// crDialects is the dispatch table, in claim order (the groups are disjoint,
// so the order decides nothing). Append a row for another CR dialect; its
// doc.json carries the sniff.
var crDialects = []crDialect{
	{"argo", ToolArgo},
	{"flux", ToolFlux},
	{"crossplane", ToolCrossplane},
}

var (
	crOnce sync.Once
	crExts map[string]*extractor.DocExtractor
)

// crExtractors is the table's dialects' document engines, built once; a
// dialect that does not load is absent (TestDialectManifests fails the
// build's tests on that, and dispatch then claims nothing for it).
func crExtractors() map[string]*extractor.DocExtractor {
	crOnce.Do(func() {
		crExts = map[string]*extractor.DocExtractor{}
		for _, d := range crDialects {
			if x, err := extractor.NewDocDialectExtractor(d.id); err == nil {
				crExts[d.id] = x
			}
		}
	})
	return crExts
}

// crVerdict is crRead's reading of one file.
type crVerdict struct {
	Ext   extractor.Extractor
	Kind  ToolKind
	ID    string // the dialect id, "" when nothing claims the file
	Mixed bool   // some document is a dialect's, and the file is not wholly one
}

// crDispatch returns the dialect's extractor and ToolKind when relPath is a
// file of Argo, Flux or Crossplane objects and nothing else. idx is the
// sibling index ingest holds (nil in a one-file refresh).
func crDispatch(relPath string, content []byte, idx *siblingIndex) (extractor.Extractor, ToolKind, bool) {
	v := crRead(relPath, content, idx)
	return v.Ext, v.Kind, v.ID != ""
}

// CRMixed reports whether relPath is a multi-document file with documents of
// one CR dialect (Argo, Flux, Crossplane) and documents that are not: such a
// file stays Kubernetes. The language-breadth oracles count these.
func CRMixed(relPath string, content []byte) bool {
	return crRead(relPath, content, nil).Mixed
}

func crRead(relPath string, content []byte, idx *siblingIndex) crVerdict {
	if ext := strings.ToLower(filepath.Ext(relPath)); ext != ".yaml" && ext != ".yml" {
		return crVerdict{}
	}
	if !apiVersionTopRe.Match(content) || !kindTopRe.Match(content) {
		return crVerdict{}
	}
	exts := crExtractors()
	// Cheap first: no table dialect's sniff matches any part of the file.
	hit := false
	for _, d := range crDialects {
		if x := exts[d.id]; x != nil && x.DetectsContent(content) {
			hit = true
			break
		}
	}
	if !hit {
		return crVerdict{}
	}
	docs := crDocuments(content)
	for _, d := range crDialects {
		x := exts[d.id]
		if x == nil {
			continue
		}
		n := 0
		for _, doc := range docs {
			if x.DetectsContent(doc) {
				n++
			}
		}
		if n == 0 {
			continue
		}
		if n != len(docs) {
			return crVerdict{Mixed: true}
		}
		// Only a file that would otherwise be a plain Kubernetes manifest: a
		// Helm template of a chart, a CRD or a basename claim keep theirs.
		if Classify(relPath, content, idx) != ToolK8sManifest {
			return crVerdict{}
		}
		return crVerdict{Ext: x, Kind: d.kind, ID: d.id}
	}
	return crVerdict{}
}

// crDocuments splits a YAML stream on its `---` separators and returns each
// document that holds anything but blank lines and comments.
func crDocuments(content []byte) [][]byte {
	var docs [][]byte
	var cur []byte
	flush := func() {
		for _, line := range bytes.Split(cur, []byte("\n")) {
			if t := bytes.TrimSpace(line); len(t) > 0 && t[0] != '#' {
				docs = append(docs, cur)
				break
			}
		}
		cur = nil
	}
	for _, line := range bytes.SplitAfter(content, []byte("\n")) {
		if bytes.HasPrefix(line, []byte("---")) && (len(bytes.TrimSpace(line)) == 3 || line[3] == ' ' || line[3] == '\t') {
			flush()
			// `--- # comment` or `--- key: v`: the rest of the line may be part of
			// the next document (a directive end marker may carry content).
			if rest := bytes.TrimSpace(line[3:]); len(rest) > 0 && rest[0] != '#' {
				cur = append(cur, rest...)
				cur = append(cur, '\n')
			}
			continue
		}
		cur = append(cur, line...)
	}
	flush()
	return docs
}
