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
	"bytes"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// ToolKind identifies what a yaml/cue-family source file actually is,
// resolved by path + content + sibling-directory context. It is recorded on
// every Observation extracted from a classified file (core.Observation.
// ToolKind) — files outside the yaml/cue family are never classified and
// carry "" (honest: "not ambiguous, no classification needed"), not a lie,
// except a .json CloudFormation template, which its content classifies
// (ToolCloudFormation) exactly as it does the YAML ones.
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
	ToolGitLabCI         ToolKind = "gitlab-ci"
	ToolOpenAPI          ToolKind = "openapi"
	ToolCUEModule        ToolKind = "cue-module"
	ToolTimoni           ToolKind = "timoni"
	ToolCUEPlain         ToolKind = "cue"
	// ToolCloudFormation (LB-Q-cloudformation): an AWS CloudFormation or SAM
	// template, a .yaml, .yml or .json no name claims, read by its content
	// (the cloudformation dialect's detect.content_sniff).
	ToolCloudFormation ToolKind = "cloudformation"
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
	// Trailing whitespace and comments are valid YAML on that line
	// (`services: # our services`) and must not defeat the signal (critic
	// F5).
	servicesTopRe = regexp.MustCompile(`(?m)^services:[ \t]*(#.*)?\r?$`)
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
	// GitLab CI (LB-Q-gitlab-ci): by path, through the dialect manifest's own
	// basenames and globs (the root .gitlab-ci.yml, .gitlab/ci/**, *.gitlab-ci.yml).
	// Before the Compose signal: a pipeline's deprecated global `services:` list
	// is a bare top-level `services:` key too.
	if connector.ClassifyDialect(dialectRules(), relPath, content).Dialect == "gitlab-ci" {
		return ToolGitLabCI
	}
	if composeServices(content) {
		return ToolCompose
	}
	if openapiTopRe.Match(content) {
		return ToolOpenAPI
	}
	// AWS CloudFormation / SAM (LB-Q-cloudformation): a template is YAML no
	// name claims, so it is read by its content, and last: only a file no
	// signal above claimed can be one, so every verdict above is unchanged.
	// The signal is the dialect's own detect.content_sniff
	// (AWSTemplateFormatVersion, an AWS:: Transform, or a top-level Resources
	// map whose Types are Namespace::Service names).
	if cloudFormationDetects(content) {
		return ToolCloudFormation
	}
	return ToolUnclassified
}

var (
	cfnOnce sync.Once
	cfnExt  *extractor.DocExtractor
)

// cloudFormationExtractor is the cloudformation dialect's document engine,
// built once; nil when the manifest does not load (TestDialectManifests
// fails the build's tests on that, and routing then claims nothing).
func cloudFormationExtractor() *extractor.DocExtractor {
	cfnOnce.Do(func() {
		if x, err := extractor.NewDocDialectExtractor("cloudformation"); err == nil {
			cfnExt = x
		}
	})
	return cfnExt
}

// cloudFormationDetects reports whether content is a CloudFormation or SAM
// template by the dialect's detect.content_sniff.
func cloudFormationDetects(content []byte) bool {
	x := cloudFormationExtractor()
	return x != nil && x.DetectsContent(content)
}

// cloudFormationJSON reports whether a .json file is a CloudFormation
// template: the sniff claims it and it decodes to one (a root mapping with a
// Resources key, the dialect's require_keys), so a JSON document that only
// mentions a template's key (a schema, a resource specification) keeps the
// generic JSON extractor. Only a file the sniff claims is decoded.
func cloudFormationJSON(content []byte) bool {
	x := cloudFormationExtractor()
	if x == nil || !x.DetectsContent(content) {
		return false
	}
	res, err := x.ExtractDoc("", "template.json", content)
	return err == nil && len(res.Roots) > 0
}

// isCloudFormation reports whether ingest's dispatch reads relPath as a
// CloudFormation template: a .yaml or .yml the classifier gives
// ToolCloudFormation, or a .json that is one (cloudFormationJSON; a
// basename claim, package.json, wins first: resolveExtractor, ReingestFile).
func isCloudFormation(relPath string, content []byte, idx *siblingIndex) bool {
	switch filepath.Ext(relPath) {
	case ".yaml", ".yml":
		return Classify(relPath, content, idx) == ToolCloudFormation
	case ".json":
		return cloudFormationJSON(content)
	}
	return false
}

var (
	openAPIOnce sync.Once
	openAPIExt  *extractor.DocExtractor
)

// openAPIExtractor is the openapi dialect's document engine, built once; nil
// when the manifest does not load (TestDialectManifests fails on that, and
// routing then claims no JSON document).
func openAPIExtractor() *extractor.DocExtractor {
	openAPIOnce.Do(func() {
		if x, err := extractor.NewDocDialectExtractor("openapi"); err == nil {
			openAPIExt = x
		}
	})
	return openAPIExt
}

// openAPIJSON reports whether a .json file is an OpenAPI or Swagger document
// (LB-Q-openapi): the dialect's sniff claims it and it decodes to one (a root
// mapping with a top-level "openapi" or "swagger" key, the document entity),
// so a JSON document that only mentions the word (a schema, an example)
// keeps the generic JSON extractor. A YAML one is the classifier's
// openapiTopRe. Only a file the sniff claims is decoded.
func openAPIJSON(content []byte) bool {
	x := openAPIExtractor()
	if x == nil || !x.DetectsContent(content) {
		return false
	}
	res, err := x.ExtractDoc("", "api.json", content)
	return err == nil && len(res.Roots) > 0
}

// isOpenAPI reports whether ingest's dispatch reads relPath as an OpenAPI or
// Swagger document: a .yaml or .yml the classifier gives ToolOpenAPI, or a
// .json that is one (openAPIJSON). ReingestFile asks it, so a one-file
// refresh reads a document as an ingest does, not as the generic YAML or
// JSON extractor its extension names.
func isOpenAPI(relPath string, content []byte, idx *siblingIndex) bool {
	switch filepath.Ext(relPath) {
	case ".yaml", ".yml":
		return Classify(relPath, content, idx) == ToolOpenAPI
	case ".json":
		return openAPIJSON(content)
	}
	return false
}

// ContentDialect names the doc-engine dialect ingest's dispatch reads
// relPath with when no basename, glob or extension claims it and its content
// does: "cloudformation" for a CloudFormation or SAM template, "openapi" for
// an OpenAPI or Swagger document, "" otherwise.
// refs is the service's Discover list (the sibling index the YAML classifier
// reads). The language-breadth harness measures a content-classified dialect
// on exactly these files (indexbench lang-oracle).
func (p *Pipeline) ContentDialect(relPath string, content []byte, refs []core.SourceRef) string {
	_, tk, ok, _ := p.resolveExtractor(relPath, content, buildSiblingIndex(refs))
	switch {
	case ok && tk == ToolCloudFormation:
		return string(ToolCloudFormation)
	case ok && tk == ToolOpenAPI:
		return string(ToolOpenAPI)
	}
	return ""
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

// composeServices reports whether content holds Compose's defining shape: a
// bare top-level `services:` key (servicesTopRe) whose value is not a block
// sequence. Compose's services is a mapping; a GitLab CI pipeline's
// deprecated global `services:` is a list of images, and a file that only
// carries that list is not a Compose file (O324: the docker-compose dialect
// would give it a ComposeFile root entity). The first line after the key
// that is neither blank nor a comment decides: an item (`- `) is a list. Any
// match in the file counts (a multi-document file). Byte-level, no parse.
func composeServices(content []byte) bool {
	for _, loc := range servicesTopRe.FindAllIndex(content, -1) {
		rest := content[loc[1]:]
		list := false
		for len(rest) > 0 {
			line := rest
			if i := bytes.IndexByte(rest, '\n'); i >= 0 {
				line, rest = rest[:i], rest[i+1:]
			} else {
				rest = nil
			}
			t := bytes.TrimSpace(line)
			if len(t) == 0 || t[0] == '#' {
				continue
			}
			list = t[0] == '-' && (len(t) == 1 || t[1] == ' ' || t[1] == '\t')
			break
		}
		if !list {
			return true
		}
	}
	return false
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
// Chart.yaml / cue.mod module root?" and "is there a Dockerfile matching
// image name X?" — built once per Ingest (option (b) from the grounding:
// O(files), not O(files²)) and threaded into extractAndStore, so every
// classify call and cross-ref lookup is an O(depth) map walk rather than a
// filesystem stat.
//
// Dockerfile keying (critic F3): a `name.dockerfile` file names the image
// it builds in its own STEM, so it is keyed by stem — keying it by
// directory name made `build/myapp.dockerfile` match an image literally
// named "build" (a lie) while missing "myapp". A bare `Dockerfile` carries
// no name of its own, so it is keyed by its parent DIRECTORY name; the
// repo-root Dockerfile (dir ".") has no usable name either way and is
// reachable only through the lone-Dockerfile fallback (see
// crossRefImageToDockerfile).
//
// A nil *siblingIndex is valid and behaves as empty (no ancestors, no
// Dockerfiles) — defensive default for callers (e.g. ExtractPending before
// any Ingest has run for a service) that have no index yet.
type siblingIndex struct {
	chartDirs           map[string]struct{} // dirs directly containing Chart.yaml
	cueModDirs          map[string]struct{} // dirs directly containing a cue.mod/ module root
	dockerfileByStem    map[string]string   // *.dockerfile stem → repo-relative path
	dockerfileByDirName map[string]string   // bare Dockerfile's parent-dir basename → repo-relative path
	rootDockerfile      string              // path of a bare Dockerfile at the repo root, "" when absent
	dockerfileCount     int                 // total Dockerfile-class files seen in the ingest
}

// buildSiblingIndex scans refs once and records every Chart.yaml, cue.mod/
// module root, and Dockerfile it finds. refs is the Discover-pass list for
// one service — the "option (b)" plan from the grounding, computed once per
// Ingest rather than re-walking ancestors per file.
func buildSiblingIndex(refs []core.SourceRef) *siblingIndex {
	idx := &siblingIndex{
		chartDirs:           map[string]struct{}{},
		cueModDirs:          map[string]struct{}{},
		dockerfileByStem:    map[string]string{},
		dockerfileByDirName: map[string]string{},
	}
	for _, r := range refs {
		slash := filepath.ToSlash(r.Path)
		base := path.Base(slash)
		dir := path.Dir(slash)

		switch {
		case base == "Chart.yaml":
			idx.chartDirs[dir] = struct{}{}
		case strings.HasSuffix(base, ".dockerfile"):
			idx.dockerfileCount++
			stem := strings.TrimSuffix(base, ".dockerfile")
			if _, exists := idx.dockerfileByStem[stem]; stem != "" && !exists {
				idx.dockerfileByStem[stem] = slash
			}
		case base == "Dockerfile":
			idx.dockerfileCount++
			if dir == "." {
				idx.rootDockerfile = slash
			} else {
				dn := path.Base(dir)
				if _, exists := idx.dockerfileByDirName[dn]; !exists {
					idx.dockerfileByDirName[dn] = slash
				}
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
// matches a Dockerfile found anywhere in the same ingest (TI-7's cheap
// tier: a same-repo name match, no registry resolution, no build-context
// evaluation — Tier is always TierPreScan). Resolution order (critic F3):
//
//  1. *.dockerfile STEM match — the file names the image it builds.
//  2. bare Dockerfile parent-DIRECTORY-name match (dir-per-image layout).
//  3. Lone-root fallback: when the repo contains exactly ONE
//     Dockerfile-class file and it is the bare root Dockerfile, any image
//     ref resolves to it — the single-service-repo layout, where the root
//     Dockerfile is the only build there is. This fallback trades a little
//     precision for the most common layout of all (a public image like
//     postgres referenced in the same repo also matches); TierPreScan and
//     the "dockerfile:" signature keep the heuristic honest, and repos
//     with 2+ Dockerfiles never guess.
//
// Safe to call unconditionally after any extraction: it is a no-op when idx
// has no recorded Dockerfiles or nodes carries no image-bearing kind.
func crossRefImageToDockerfile(service, relPath string, nodes []core.SymbolNode, idx *siblingIndex) []core.SymbolNode {
	if idx == nil || idx.dockerfileCount == 0 {
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
		dockerfile, ok := idx.dockerfileByStem[name]
		if !ok {
			dockerfile, ok = idx.dockerfileByDirName[name]
		}
		if !ok && idx.dockerfileCount == 1 && idx.rootDockerfile != "" {
			dockerfile, ok = idx.rootDockerfile, true
		}
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
