package langroute

// resolve.go — Router.Resolve, the dispatch a one-file refresh (ReingestFile)
// runs. Moved out of internal/fusion unchanged (ADR-092).

import (
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/langclass"
)

// Resolve is the extractor a one-file refresh reads relFile (repo-relative,
// forward-slash) under repoPath with: its own dispatch order, a second one
// beside ResolveExtractor's, and the sibling facts it reads from the repository
// on disk where ingest holds the whole repository's. ok is false when no
// extractor reads the file.
func (r *Router) Resolve(repoPath, relFile string, content []byte) (extractor.Extractor, bool) {
	base := filepath.Base(relFile)
	ext := filepath.Ext(relFile)
	extExtractor, ok := r.exts[base]
	if ke, _, kok := r.checkovDispatch(relFile); kok {
		extExtractor, ok = ke, true // Checkov Python checks and YAML graph checks (LB-Q-checkov-policies): resolveExtractor's decision
	}
	if ce, _, cok := r.cloudPolicyDispatch(relFile, content, nil); cok {
		// IAM policies, Azure Policy and Cloud Custodian files (LB-F138): the decision resolveExtractor makes.
		extExtractor, ok = ce, true
	}
	if ge, _, gok := GatewayDispatch(relFile, content); gok {
		// Envoy, Kong and Traefik configuration (LB-F104): the decision
		// resolveExtractor makes, a manifest name AND the content sniff.
		extExtractor, ok = ge, true
	}
	if fe, _, fok := r.formatDispatch(relFile, content); fok {
		extExtractor, ok = fe, true // .fathom/config.toml and docs/plans/*.md (LB-F219): resolveExtractor's decision
	}
	if pe, _, pok := PolicyDispatch(relFile, content); pok {
		extExtractor, ok = pe, true // Kyverno and Gatekeeper files (LB-F134): resolveExtractor's decision
	}
	var idx *SiblingIndex
	if ext == ".yml" || ext == ".yaml" {
		idx = DiskSiblingIndex(repoPath, relFile) // the chart and role roots ingest read from Discover
	}
	if ext == ".cue" {
		idx = DiskTimoniIndex(repoPath, relFile) // the module roots above the file (LB-F216)
	}
	if ce, _, cok := CRDispatch(relFile, content, idx); cok {
		// Argo, Flux and Crossplane objects (LB-F149): ingest's decision.
		extExtractor, ok = ce, true
	}
	if te, _, tok := r.TimoniDispatch(relFile, content, idx); tok {
		extExtractor, ok = te, true // Timoni modules, bundles and runtimes (LB-F216): ingest's decision
	}
	if ke, _, kok := r.k8sEmbedDispatch(relFile, content, idx); kok {
		extExtractor, ok = ke, true // a manifest embedding a dashboard or an Alertmanager configuration (LB-F366): ingest's decision
	}
	if !ok && (ext == ".yml" || ext == ".yaml") && Classify(relFile, content, idx) == ToolAnsible {
		// An Ansible file (LB-Q-ansible), read by path and content as ingest's
		// classifier reads it, not by the generic YAML extractor.
		extExtractor, ok = r.toolExts[ToolAnsible]
	}
	if !ok && (ext == ".yml" || ext == ".yaml") && langclass.ClassifyDialect(DialectRules(), relFile, content).Dialect == "gitlab-ci" {
		// A pipeline file named only by its path (.gitlab/ci/…, *.gitlab-ci.yml)
		// reads through the gitlab-ci dialect, as ingest's classifier sends it
		// there, not through the generic YAML extractor the extension names.
		extExtractor, ok = r.toolExts[ToolGitLabCI]
	}
	if !ok {
		if ae, _, aok := AzurePipelineDispatch(relFile, content, idx); aok {
			extExtractor, ok = ae, true
		}
	}
	if !ok && (ext == ".yml" || ext == ".yaml") && langclass.ClassifyDialect(DialectRules(), relFile, content).Dialect == "circleci" {
		// .circleci/config.yml, as ingest's classifier sends it (LB-Q-circleci).
		extExtractor, ok = r.toolExts[ToolCircleCI]
	}
	if !ok && (ext == ".yml" || ext == ".yaml") && strings.Contains(filepath.ToSlash(relFile), ".github/workflows/") {
		// A workflow is the github-actions dialect's, as ingest dispatches it (LB-F306).
		extExtractor, ok = r.toolExts[ToolGHA]
	}
	if !ok && (ext == ".yml" || ext == ".yaml") {
		// A late YAML dialect (lateyamldispatch.go): the classifier's last
		// signal, the decision ingest's resolveExtractor makes.
		if le := lateYAMLExtractor(Classify(relFile, content, idx)); le != nil {
			extExtractor, ok = le, true
		}
	}
	if !ok && isCloudFormation(relFile, content, nil) {
		// A CloudFormation template (LB-Q-cloudformation) is read by its
		// content, as ingest's dispatch reads it, not by the YAML or JSON
		// extractor its extension names.
		extExtractor, ok = r.toolExts[ToolCloudFormation]
	}
	if !ok && isOpenAPI(relFile, content, nil) {
		// An OpenAPI or Swagger document (LB-Q-openapi), YAML or JSON, is read
		// by its content as ingest's dispatch reads it.
		extExtractor, ok = r.toolExts[ToolOpenAPI]
	}
	if !ok && ext == ".json" && isGrafanaDashboard(relFile, content, nil) {
		// A Grafana dashboard JSON (LB-F365), by its content, as ingest reads it.
		if ge := lateYAMLExtractor(ToolGrafanaDashboard); ge != nil {
			extExtractor, ok = ge, true
		}
	}
	if !ok {
		extExtractor, _, ok = AppFrameworkDispatch(relFile, content) // ARM by content, a Pulumi stack by glob: ingest's decision (LB-F146)
	}
	if !ok {
		extExtractor, _, ok = JSONSchemaDispatch(relFile, content) // a JSON Schema document: ingest's decision (LB-Q-json-schema)
	}
	if !ok {
		extExtractor, ok = r.exts[ext]
	}
	if !ok {
		// A dialect that owns the extension only through match.sniff
		// (MATLAB's .m): ingest's dispatch, resolveExtractor, asks the
		// same classifier.
		extExtractor, ok = r.sniffedExtractor(relFile, content)
	}
	return extExtractor, ok
}
