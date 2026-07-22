package fusion

// classify.go — the pass-3 tool-identity classifier (ADR-021 §2). It runs
// between the hash-gate and extractor dispatch in extractAndStore, for the
// yaml/cue-family files whose extension alone is ambiguous about what tool
// actually produced them: the ext/basename table (fusion.New's p.exts,
// connector.extClassMap) stays the candidate pre-filter ("is there a
// yaml/cue extractor at all for this file"), and classify decides which one.
//
// Grounding (wf_d0588a59): Helm templates/*.yaml fail yaml.v3 on {{ }} and
// silently emit nothing; kustomization payloads are parsed-then-discarded
// (no metadata.name); CUE modules/timoni have no SourceKind; arbitrary YAML
// funnels into the k8s extractor with silent zero output; no generated-file
// detection; no size cap.

import (
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// ToolKind identifies what a yaml/cue-family source file actually is,
// resolved by path + content + sibling-directory context. It is recorded on
// every Observation extracted from a classified file (core.Observation.
// ToolKind) — files outside the yaml/cue family are never classified and
// carry "" (honest: "not ambiguous, no classification needed"), not a lie.
type ToolKind string

const (
	// ToolUnclassified means the file's extension matched a yaml/cue
	// extractor candidate, but no content/path signal identified a known
	// tool. Per ADR-021 §2 these files are NOT handed to an extractor
	// (previously: silently funneled into the k8s extractor for a silent
	// zero-symbol result) — they are counted (report.Unclassified /
	// server_stats) and fingerprinted so the count is visible instead.
	ToolUnclassified ToolKind = "unclassified"

	ToolK8sManifest      ToolKind = "k8s"
	ToolK8sCRD           ToolKind = "k8s-crd"
	ToolCompose          ToolKind = "compose"
	ToolHelmChart        ToolKind = "helm-chart"
	ToolHelmValues       ToolKind = "helm-values"
	ToolHelmTemplateYAML ToolKind = "helm-template-yaml"
	ToolKustomize        ToolKind = "kustomize"
	ToolGHA              ToolKind = "gha"
	ToolOpenAPI          ToolKind = "openapi"
	ToolCUEModule        ToolKind = "cue-module"
	ToolTimoni           ToolKind = "timoni"
	ToolCUEPlain         ToolKind = "cue"
)

// MaxExtractSize is the size cap above which a file is skipped from
// extraction entirely (ADR-021 §2 / TI-6) — the minified/huge-file class.
// The file is still fingerprinted (so it is not re-sniffed every ingest) and
// counted (report.Oversize / server_stats) rather than silently ignored.
const MaxExtractSize = 2 * 1024 * 1024 // 2MB

// ─── content signals (cheapest-first within the content tier; path-based
// signals in classifyYAML/classifyCUE below run before any of these) ───────

var (
	// crdAPIVersionRe matches a CustomResourceDefinition's apiVersion. It is
	// checked before the generic k8s-manifest signal so a CRD (which also
	// has a top-level kind:) is tagged distinctly rather than folding into
	// the generic bucket.
	crdAPIVersionRe = regexp.MustCompile(`(?m)^apiVersion:\s*["']?apiextensions\.k8s\.io`)
	apiVersionTopRe = regexp.MustCompile(`(?m)^apiVersion:\s*\S`)
	kindTopRe       = regexp.MustCompile(`(?m)^kind:\s*\S`)
	// servicesTopRe matches docker-compose's defining shape — a bare
	// top-level `services:` mapping key — regardless of filename, so
	// docker-compose.override.yml (or any other name) is recognized too.
	servicesTopRe = regexp.MustCompile(`(?m)^services:\s*$`)
	openapiTopRe  = regexp.MustCompile(`(?m)^(openapi|swagger):\s*\S`)
	// timoniBundleRe matches a timoni bundle's defining top-level `bundle:`
	// struct field.
	timoniBundleRe = regexp.MustCompile(`(?m)^bundle:\s*\{`)
)

// Classify is the pass-3 tool-identity classifier's single entry point
// (ADR-021 §2 / TI-1): dispatches to classifyYAML or classifyCUE by
// extension. relPath's basename should NOT be one of the special keys
// fusion.New's p.exts already resolves unambiguously (Chart.yaml,
// values.yaml, kustomization.*, compose.*, docker-compose.*) or under
// .github/workflows/ — extractAndStore checks those first and never calls
// Classify for them, since no content sniff is needed. Extensions outside
// the yaml/cue family return ToolUnclassified (Classify is never called for
// them either; the constant is simply the safe default).
func Classify(relPath string, content []byte, idx *siblingIndex) ToolKind {
	switch strings.ToLower(filepath.Ext(relPath)) {
	case ".yaml", ".yml":
		return classifyYAML(relPath, content, idx)
	case ".cue":
		return classifyCUE(relPath, content, idx)
	default:
		return ToolUnclassified
	}
}

// classifyYAML resolves the ToolKind for a .yaml/.yml file whose basename is
// NOT one of the special basenames already dispatched unambiguously by
// fusion.New's p.exts (Chart.yaml, values.yaml, kustomization.{yaml,yml},
// docker-compose.*, compose.*) and which is not under .github/workflows/
// (also dispatched unambiguously, unchanged, via the existing YAMLExtractor
// wrapper — GHA detection was never the grounded defect).
//
// Path-based signals run first (cheapest, and some — helm-template —  must
// win over a content signal that would otherwise also match). Content
// signals then run cheapest-first: a plain byte-level regex over the raw
// content, no YAML parse.
func classifyYAML(relPath string, content []byte, idx *siblingIndex) ToolKind {
	slash := filepath.ToSlash(relPath)
	dir := path.Dir(slash)

	if (strings.Contains(slash, "/templates/") || strings.HasPrefix(slash, "templates/")) && idx.hasChartAncestor(dir) {
		return ToolHelmTemplateYAML
	}

	if crdAPIVersionRe.Match(content) && kindTopRe.Match(content) {
		return ToolK8sCRD
	}
	if apiVersionTopRe.Match(content) && kindTopRe.Match(content) {
		return ToolK8sManifest
	}
	if servicesTopRe.Match(content) {
		return ToolCompose
	}
	if openapiTopRe.Match(content) {
		return ToolOpenAPI
	}
	return ToolUnclassified
}

// classifyCUE resolves the ToolKind tag for a .cue file. Every ToolKind it
// returns is still extracted by the same CUEExtractor (LT-4) — classifyCUE
// only decides what to TAG the resulting observations with (Observation.
// ToolKind), giving CUE modules and timoni bundles visibility they
// previously had none of (ADR-021 §2 grounding), without duplicating CUE
// extraction logic.
func classifyCUE(relPath string, content []byte, idx *siblingIndex) ToolKind {
	slash := filepath.ToSlash(relPath)
	base := path.Base(slash)
	dir := path.Dir(slash)

	if base == "timoni.cue" || timoniBundleRe.Match(content) {
		return ToolTimoni
	}
	if idx.hasCueModAncestor(dir) {
		return ToolCUEModule
	}
	return ToolCUEPlain
}

// toolKindForBasename tags observations from a file whose extractor was
// already resolved unambiguously by basename (fusion.New's p.exts) rather
// than through classifyYAML's content signals. Returns "" for a basename
// this function doesn't recognize (defensive default; extractAndStore only
// calls it for keys already known to be in p.exts).
func toolKindForBasename(base string) ToolKind {
	switch base {
	case "Chart.yaml":
		return ToolHelmChart
	case "values.yaml":
		return ToolHelmValues
	case "kustomization.yaml", "kustomization.yml":
		return ToolKustomize
	case "docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml":
		return ToolCompose
	default:
		return ""
	}
}

// ─── sibling index (built once per Ingest from the Discover list) ─────────

// siblingIndex answers "does an ancestor directory of this file contain a
// Chart.yaml / cue.mod module root?" and "is there a Dockerfile whose parent
// directory is named X?" — built once per Ingest (option (b) from the
// grounding: O(files), not O(files²)) and threaded into extractAndStore, so
// every classify call and cross-ref lookup is an O(depth) map walk rather
// than a filesystem stat.
//
// A nil *siblingIndex is valid and behaves as empty (no ancestors, no
// Dockerfiles) — defensive default for callers (e.g. ExtractPending before
// any Ingest has run for a service) that have no index yet.
type siblingIndex struct {
	chartDirs           map[string]struct{} // dirs directly containing Chart.yaml
	cueModDirs          map[string]struct{} // dirs directly containing a cue.mod/ module root
	dockerfileByDirName map[string]string   // Dockerfile's parent-dir basename → its repo-relative path
}

// buildSiblingIndex scans refs once and records every Chart.yaml, cue.mod/
// module root, and Dockerfile it finds. refs is the Discover-pass list for
// one service — the "option (b)" plan from the grounding, computed once per
// Ingest rather than re-walking ancestors per file.
func buildSiblingIndex(refs []core.SourceRef) *siblingIndex {
	idx := &siblingIndex{
		chartDirs:           map[string]struct{}{},
		cueModDirs:          map[string]struct{}{},
		dockerfileByDirName: map[string]string{},
	}
	for _, r := range refs {
		slash := filepath.ToSlash(r.Path)
		base := path.Base(slash)
		dir := path.Dir(slash)

		switch {
		case base == "Chart.yaml":
			idx.chartDirs[dir] = struct{}{}
		case base == "Dockerfile" || strings.HasSuffix(base, ".dockerfile"):
			dn := path.Base(dir)
			if _, exists := idx.dockerfileByDirName[dn]; !exists {
				idx.dockerfileByDirName[dn] = slash
			}
		}

		// cue.mod/ is a directory, so it appears as a path SEGMENT (e.g.
		// "cue.mod/module.cue" or "svc/cue.mod/module.cue"), never as a
		// basename — record the directory that CONTAINS cue.mod/ as the
		// module root.
		if i := strings.Index(slash, "cue.mod/"); i >= 0 && (i == 0 || slash[i-1] == '/') {
			root := strings.TrimSuffix(slash[:i], "/")
			if root == "" {
				root = "."
			}
			idx.cueModDirs[root] = struct{}{}
		}
	}
	return idx
}

// hasAncestorIn reports whether dir, or any of its ancestor directories, is a
// member of set.
func hasAncestorIn(dir string, set map[string]struct{}) bool {
	if len(set) == 0 {
		return false
	}
	d := dir
	for {
		if _, ok := set[d]; ok {
			return true
		}
		if d == "." || d == "/" || d == "" {
			return false
		}
		parent := path.Dir(d)
		if parent == d {
			return false
		}
		d = parent
	}
}

func (idx *siblingIndex) hasChartAncestor(dir string) bool {
	if idx == nil {
		return false
	}
	return hasAncestorIn(dir, idx.chartDirs)
}

func (idx *siblingIndex) hasCueModAncestor(dir string) bool {
	if idx == nil {
		return false
	}
	return hasAncestorIn(dir, idx.cueModDirs)
}

// ─── cross-refs, cheap tier (ADR-021 §2 / TI-7) ────────────────────────────

// imageBaseName extracts the trailing repository-name segment of a
// container image reference, stripping any registry/path prefix and
// tag/digest suffix — deliberately naive, no registry resolution:
// "ghcr.io/org/myapp:1.2.3" → "myapp"; "myapp@sha256:…" → "myapp";
// "myapp" → "myapp".
func imageBaseName(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	if i := strings.LastIndex(ref, "/"); i >= 0 {
		ref = ref[i+1:]
	}
	if i := strings.IndexAny(ref, ":@"); i >= 0 {
		ref = ref[:i]
	}
	return ref
}

// crossRefImageToDockerfile appends a KindImageDockerfileRef reference node
// for every K8sContainer/ComposeService node in nodes whose image name
// matches a Dockerfile found anywhere in the same ingest, by parent-directory
// basename (TI-7's cheap tier: a same-repo name match, no registry
// resolution, no build-context evaluation — Tier is always TierPreScan).
// Safe to call unconditionally after any extraction: it is a no-op when idx
// has no recorded Dockerfiles or nodes carries no image-bearing kind.
func crossRefImageToDockerfile(service, relPath string, nodes []core.SymbolNode, idx *siblingIndex) []core.SymbolNode {
	if idx == nil || len(idx.dockerfileByDirName) == 0 {
		return nil
	}
	var refs []core.SymbolNode
	seen := make(map[string]bool)
	for _, n := range nodes {
		var image string
		switch n.Kind {
		case core.KindK8sContainer:
			image = n.Name
		case core.KindComposeService:
			if !strings.HasPrefix(n.Signature, "build:") {
				image = n.Signature
			}
		default:
			continue
		}
		name := imageBaseName(image)
		if name == "" || seen[name] {
			continue
		}
		dockerfile, ok := idx.dockerfileByDirName[name]
		if !ok {
			continue
		}
		seen[name] = true
		refs = append(refs, core.SymbolNode{
			Service:   service,
			Name:      name,
			Kind:      core.KindImageDockerfileRef,
			FilePath:  relPath,
			Line:      n.Line,
			Signature: "dockerfile: " + dockerfile,
			Tier:      core.TierPreScan,
		})
	}
	return refs
}
