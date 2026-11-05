package fusion

// routing_alias.go — the names internal/fusion keeps for what moved to
// internal/langroute (ADR-092, LL-4): the tool kind and its constants, the
// sibling index, the gate constants. Callers and tests keep their spelling.

import (
	"gopkg.in/yaml.v3"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/fathomformat"
	"github.com/geoffmilleraz/fathom/internal/langroute"
)

// ToolKind is langroute.ToolKind.
type ToolKind = langroute.ToolKind

// siblingIndex is langroute.SiblingIndex; its Attached slot holds this
// package's file-spec memo.
type siblingIndex = langroute.SiblingIndex

// The tool kinds a classifier verdict names.
const (
	ToolAnsible          = langroute.ToolAnsible
	ToolARMTemplate      = langroute.ToolARMTemplate
	ToolPulumiYAML       = langroute.ToolPulumiYAML
	ToolAzurePipelines   = langroute.ToolAzurePipelines
	ToolCheckovPolicies  = langroute.ToolCheckovPolicies
	ToolUnclassified     = langroute.ToolUnclassified
	ToolK8sManifest      = langroute.ToolK8sManifest
	ToolK8sCRD           = langroute.ToolK8sCRD
	ToolCompose          = langroute.ToolCompose
	ToolHelmChart        = langroute.ToolHelmChart
	ToolHelmValues       = langroute.ToolHelmValues
	ToolHelmTemplateYAML = langroute.ToolHelmTemplateYAML
	ToolKustomize        = langroute.ToolKustomize
	ToolGHA              = langroute.ToolGHA
	ToolGitLabCI         = langroute.ToolGitLabCI
	ToolCircleCI         = langroute.ToolCircleCI
	ToolOpenAPI          = langroute.ToolOpenAPI
	ToolCUEModule        = langroute.ToolCUEModule
	ToolTimoni           = langroute.ToolTimoni
	ToolCUEPlain         = langroute.ToolCUEPlain
	ToolCloudFormation   = langroute.ToolCloudFormation
	ToolAWSIAMPolicy     = langroute.ToolAWSIAMPolicy
	ToolAzurePolicy      = langroute.ToolAzurePolicy
	ToolCloudCustodian   = langroute.ToolCloudCustodian
	ToolArgo             = langroute.ToolArgo
	ToolFlux             = langroute.ToolFlux
	ToolCrossplane       = langroute.ToolCrossplane
	ToolEnvoy            = langroute.ToolEnvoy
	ToolKong             = langroute.ToolKong
	ToolTraefik          = langroute.ToolTraefik
	ToolPgBouncer        = langroute.ToolPgBouncer
	ToolProxySQL         = langroute.ToolProxySQL
	ToolJSONSchema       = langroute.ToolJSONSchema
	ToolPrometheus       = langroute.ToolPrometheus
	ToolOTelCollector    = langroute.ToolOTelCollector
	ToolAlertmanager     = langroute.ToolAlertmanager
	ToolGrafanaDashboard = langroute.ToolGrafanaDashboard
	ToolTaskfile         = langroute.ToolTaskfile
	ToolKyverno          = langroute.ToolKyverno
	ToolGatekeeper       = langroute.ToolGatekeeper
)

// The tool kinds of fathom'"'"'s own formats (internal/fathomformat).
const (
	ToolFathomConfig     = fathomformat.ToolFathomConfig
	ToolFathomPlanRows   = fathomformat.ToolFathomPlanRows
	ToolFathomConcerns   = fathomformat.ToolFathomConcerns
	ToolFathomLayerPacks = fathomformat.ToolFathomLayerPacks
)

// MaxExtractSize is langroute.MaxExtractSize.
const MaxExtractSize = langroute.MaxExtractSize

// The dialect gate's excluded reasons and the sniff count.
const (
	ExcludedSkippedGenerated = langroute.ExcludedSkippedGenerated
	ExcludedAmbiguous        = langroute.ExcludedAmbiguous
	CountClassifiedBySniff   = langroute.CountClassifiedBySniff
)

// Classify is langroute.Classify.
func Classify(relPath string, content []byte, idx *siblingIndex) ToolKind {
	return langroute.Classify(relPath, content, idx)
}

// PolicyDialect is langroute.PolicyDialect.
func PolicyDialect(relPath string, content []byte) string {
	return langroute.PolicyDialect(relPath, content)
}

// CRMixed is langroute.CRMixed.
func CRMixed(relPath string, content []byte) bool { return langroute.CRMixed(relPath, content) }

// JSONSchemaContentDialect is langroute.JSONSchemaContentDialect.
func JSONSchemaContentDialect(relPath string, content []byte) string {
	return langroute.JSONSchemaContentDialect(relPath, content)
}

// AppFrameworkContentDialect is langroute.AppFrameworkContentDialect.
func AppFrameworkContentDialect(relPath string, content []byte) string {
	return langroute.AppFrameworkContentDialect(relPath, content)
}

// DispatchTable is the read-only view of the per-key extractor dispatch that
// New wires (LB-F356): see langroute.Router.DispatchTable. It is computed from a
// freshly constructed Pipeline's router, so docs/languages.md's "Engine /
// dispatch" column is generated from what ingest does.
func DispatchTable() map[string]string {
	p := New(nil, nil, Config{})
	defer p.Close()
	return p.Router.DispatchTable()
}

// dialectRules is langroute.DialectRules.
func dialectRules() []connector.DialectRule { return langroute.DialectRules() }

// binaryExcluded is langroute.BinaryExcluded.
func binaryExcluded(relPath string, content []byte) bool {
	return langroute.BinaryExcluded(relPath, content)
}

// binaryExcludedWith is langroute.BinaryExcludedWith.
func binaryExcludedWith(rules []connector.DialectRule, relPath string, content []byte) bool {
	return langroute.BinaryExcludedWith(rules, relPath, content)
}

// dialectVerdict is the dialect gate's decision on one read file.
type dialectVerdict struct {
	excluded string
	sniffed  bool
}

// gateDialectFile is langroute.GateDialectFile.
func gateDialectFile(relPath string, content []byte) dialectVerdict {
	v := langroute.GateDialectFile(relPath, content)
	return dialectVerdict{excluded: v.Excluded, sniffed: v.Sniffed}
}

// gateDialectFileWith is langroute.GateDialectFileWith.
func gateDialectFileWith(rules []connector.DialectRule, relPath string, content []byte) dialectVerdict {
	v := langroute.GateDialectFileWith(rules, relPath, content)
	return dialectVerdict{excluded: v.Excluded, sniffed: v.Sniffed}
}

// buildSiblingIndex is langroute.BuildSiblingIndex, with this package's
// per-ingest file-spec memo attached (EW-52).
func buildSiblingIndex(refs []core.SourceRef) *siblingIndex {
	idx := langroute.BuildSiblingIndex(refs)
	idx.Attached = newSpecMemo()
	return idx
}

// extractWithFacts is langroute.ExtractWithFacts.
func extractWithFacts(x extractor.Extractor, service, relPath string, src []byte) ([]core.SymbolNode, error) {
	return langroute.ExtractWithFacts(x, service, relPath, src)
}

// The YAML and Kubernetes helpers the cross-file passes share with the dispatch.

func yamlDocs(content []byte) []*yaml.Node { return langroute.YAMLDocs(content) }

func ymap(n *yaml.Node, keys ...string) *yaml.Node { return langroute.YMap(n, keys...) }

func yscalar(n *yaml.Node) string { return langroute.YScalar(n) }

func ylabels(n *yaml.Node) (map[string]string, string, bool) { return langroute.YLabels(n) }

func k8sEntityName(kind, ns, name string) string { return langroute.K8sEntityName(kind, ns, name) }

func k8sEmbedCandidate(content []byte) bool { return langroute.K8sEmbedCandidate(content) }

func k8sEmbeddedResult(service, relPath string, content []byte) (extractor.DocResult, bool) {
	return langroute.K8sEmbeddedResult(service, relPath, content)
}

func policyEntityNodes(nodes []core.SymbolNode, refs []extractor.DocCrossFileRef) []core.SymbolNode {
	return langroute.PolicyEntityNodes(nodes, refs)
}
