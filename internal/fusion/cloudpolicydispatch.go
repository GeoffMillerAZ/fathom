package fusion

// cloudpolicydispatch.go — ingest's claim of AWS IAM policy documents, Azure
// Policy definitions and initiatives, and Cloud Custodian policy files
// (LB-F138, language-breadth wave pc). Each is a doc-engine dialect
// (internal/extractor/dialects/{aws-iam-policy,azure-policy,cloud-custodian})
// that claims no name: a policy is a .json, .yaml or .yml whose content says
// so. A file is claimed only when BOTH hold: the dialect's detect.content_sniff
// matches and the file decodes to one of the dialect's document entities (an
// IAM policy root with Version 2012-10-17 or 2008-10-17 and a Statement; a
// definition with a policyRule or an initiative with policyDefinitions; a
// policies list whose entries carry a name and a resource), copying
// cloudFormationJSON's sniff-plus-decode check.
//
// It never claims a file something earlier claims: a basename the pipeline
// names (package.json) or a dialect manifest names by basename or glob, a
// GitHub Actions workflow, a YAML file the classifier gives a tool kind (a
// Kubernetes manifest, a CRD, Compose, GitLab CI, OpenAPI, a Helm template,
// CloudFormation), a CloudFormation or OpenAPI JSON document, and an Envoy,
// Kong or Traefik file (gatewayDispatch). So an inline IAM PolicyDocument in
// a CloudFormation template stays the template's (LB-X-iam-inline). The hook
// sits in resolveExtractor and ReingestFile before gatewayDispatch.

import (
	"path/filepath"
	"strings"
	"sync"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

const (
	// ToolAWSIAMPolicy: an AWS IAM policy document (LB-Q-aws-iam-policy).
	ToolAWSIAMPolicy ToolKind = "aws-iam-policy"
	// ToolAzurePolicy: an Azure Policy definition or initiative (LB-Q-azure-policy).
	ToolAzurePolicy ToolKind = "azure-policy"
	// ToolCloudCustodian: a Cloud Custodian policies file (LB-Q-cloud-custodian).
	ToolCloudCustodian ToolKind = "cloud-custodian"
)

// cloudPolicyDialects lists the claims in dispatch order. A file decodes to
// one of a dialect's documents when, for some set in docs, every SourceKind of
// the set is among the entities the dialect reads from it (the decode check:
// an IAM root and a statement that is a map; a definition and the effect of
// its rule, or an initiative and a member; a policy with a name and a
// resource), so a JSON file that only names a key stays generic JSON.
var cloudPolicyDialects = []struct {
	id   string
	tool ToolKind
	docs [][]core.SourceKind
}{
	{"aws-iam-policy", ToolAWSIAMPolicy, [][]core.SourceKind{{"IAMPolicy", "IAMStatement"}}},
	{"azure-policy", ToolAzurePolicy, [][]core.SourceKind{{"AzurePolicyDefinition", "AzurePolicyEffect"}, {"AzurePolicyInitiative", "AzurePolicyMember"}}},
	{"cloud-custodian", ToolCloudCustodian, [][]core.SourceKind{{"CustodianPolicy"}}},
}

var (
	cloudPolicyOnce sync.Once
	cloudPolicyExts map[string]*extractor.DocExtractor
)

// cloudPolicyExtractors is the three dialects' document engines, built once;
// a dialect that does not load is absent (TestDialectManifests fails the
// build's tests on that, and dispatch then claims nothing for it).
func cloudPolicyExtractors() map[string]*extractor.DocExtractor {
	cloudPolicyOnce.Do(func() {
		cloudPolicyExts = map[string]*extractor.DocExtractor{}
		for _, d := range cloudPolicyDialects {
			if x, err := extractor.NewDocDialectExtractor(d.id); err == nil {
				cloudPolicyExts[d.id] = x
			}
		}
	})
	return cloudPolicyExts
}

// cloudPolicyDispatch returns the dialect's extractor and tool kind when
// relPath is an IAM policy, an Azure Policy definition or initiative, or a
// Cloud Custodian policies file; ok is false for every other file. idx is the
// sibling index the YAML classifier reads (nil in ReingestFile).
func (p *Pipeline) cloudPolicyDispatch(relPath string, content []byte, idx *siblingIndex) (extractor.Extractor, ToolKind, bool) {
	id, tk := cloudPolicyClaim(p, relPath, content, idx)
	if id == "" {
		return nil, "", false
	}
	return cloudPolicyExtractors()[id], tk, true
}

// CloudPolicyDialect names the doc-engine dialect ingest's dispatch reads
// relPath with by its content alone ("aws-iam-policy", "azure-policy" or
// "cloud-custodian"), "" for any other file, with ContentDialect's contract:
// the language-breadth harness measures a content-classified dialect on
// exactly these files (indexbench lang-oracle). refs is the service's
// Discover list.
func (p *Pipeline) CloudPolicyDialect(relPath string, content []byte, refs []core.SourceRef) string {
	id, _ := cloudPolicyClaim(p, relPath, content, buildSiblingIndex(refs))
	return id
}

// cloudPolicyClaim is the one decision both entry points share: the claiming
// dialect's id and tool kind, "" when none claims relPath.
func cloudPolicyClaim(p *Pipeline, relPath string, content []byte, idx *siblingIndex) (string, ToolKind) {
	ext := strings.ToLower(filepath.Ext(relPath))
	if ext != ".json" && ext != ".yaml" && ext != ".yml" {
		return "", ""
	}
	if _, named := p.exts[filepath.Base(relPath)]; named {
		return "", ""
	}
	if by := connector.ClassifyDialect(dialectRules(), relPath, content).By; by == connector.ClassifiedByBasename || by == connector.ClassifiedByGlob {
		return "", ""
	}
	exts := cloudPolicyExtractors()
	var sniffed []int
	for i, d := range cloudPolicyDialects {
		if x := exts[d.id]; x != nil && x.DetectsContent(content) {
			sniffed = append(sniffed, i)
		}
	}
	if len(sniffed) == 0 {
		return "", ""
	}
	if ext == ".json" {
		if cloudFormationJSON(content) || openAPIJSON(content) {
			return "", ""
		}
	} else if strings.Contains(filepath.ToSlash(relPath), ".github/workflows/") || Classify(relPath, content, idx) != ToolUnclassified {
		return "", ""
	}
	if _, _, claimed := gatewayDispatch(relPath, content); claimed {
		return "", ""
	}
	for _, i := range sniffed {
		d := cloudPolicyDialects[i]
		res, err := exts[d.id].ExtractDoc("", filepath.Base(relPath), content)
		if err != nil {
			continue
		}
		have := map[core.SourceKind]bool{}
		for _, n := range res.Nodes {
			have[n.Kind] = true
		}
		for _, set := range d.docs {
			all := true
			for _, k := range set {
				all = all && have[k]
			}
			if all {
				return d.id, d.tool
			}
		}
	}
	return "", ""
}
