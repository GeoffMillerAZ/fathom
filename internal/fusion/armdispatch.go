package fusion

// armdispatch.go — ingest's claim of the Azure and AWS app-framework
// configuration files no basename names (wave iac, LB-F146): an Azure
// Resource Manager template or parameters file (LB-Q-arm-templates), a
// .json claimed by its content alone, and a Pulumi stack configuration file
// (LB-Q-pulumi-yaml), a Pulumi.<stack>.yaml claimed by the pulumi-yaml
// manifest's glob. The files a basename names — serverless.yml (LB-Q-serverless),
// Pulumi.yaml (LB-Q-pulumi-yaml) and cdk.json (LB-Q-aws-cdk) — are fusion
// fast-path keys (register.go), read before any content signal.
//
// An ARM file is a .json whose content the arm-templates doc spec's
// detect.content_sniff claims (a `"$schema"` naming deploymentTemplate.json,
// its subscription-, managementGroup- and tenant-level variants, or
// deploymentParameters.json) AND that decodes to one: a root template whose
// own `$schema` names a deployment template, or a parameters file whose
// root `$schema` names deploymentParameters.json and that sets a parameter.
// A document that only embeds a template (an Azure Policy definition's
// deployIfNotExists deployment) has no root `$schema` and is never one.
//
// resolveExtractor (ingest and ExtractForBench) asks after the CloudFormation
// and OpenAPI sniffs, and ReingestFile at the same point, so a basename claim
// (package.json) and every earlier verdict stand.

import (
	"path"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

const (
	// ToolARMTemplate: an Azure Resource Manager template or parameters file.
	ToolARMTemplate ToolKind = "arm-templates"
	// ToolPulumiYAML: a Pulumi YAML project or stack configuration file.
	ToolPulumiYAML ToolKind = "pulumi-yaml"
)

var (
	armOnce sync.Once
	armExt  *extractor.DocExtractor
	stkOnce sync.Once
	stkExt  *extractor.DocExtractor

	// armTemplateSchemaRe and armParamsSchemaRe read a root `$schema`.
	armTemplateSchemaRe = regexp.MustCompile(`(?i)deploymenttemplate\.json`)
	armParamsSchemaRe   = regexp.MustCompile(`(?i)deploymentparameters\.json`)
	// armFirstSchemaRe is a file's first `$schema` value: a parameters
	// file's root one (a parameters file embeds no template).
	armFirstSchemaRe = regexp.MustCompile(`"\$schema"\s*:\s*"([^"]*)"`)
)

// armExtractor is the arm-templates dialect's document engine, built once;
// nil when the manifest does not load (TestDialectManifests fails the
// build's tests on that, and dispatch then claims nothing).
func armExtractor() *extractor.DocExtractor {
	armOnce.Do(func() {
		if x, err := extractor.NewDocDialectExtractor("arm-templates"); err == nil {
			armExt = x
		}
	})
	return armExt
}

// pulumiStackExtractor is the pulumi-yaml dialect's document engine for a
// stack configuration file, built once (nil when it does not load).
func pulumiStackExtractor() *extractor.DocExtractor {
	stkOnce.Do(func() {
		if x, err := extractor.NewDocDialectExtractor("pulumi-yaml"); err == nil {
			stkExt = x
		}
	})
	return stkExt
}

// armTemplateJSON reports whether content is an ARM template or parameters
// file (see the file comment). Only a file the sniff claims is decoded.
func armTemplateJSON(content []byte) bool {
	x := armExtractor()
	if x == nil || !x.DetectsContent(content) {
		return false
	}
	res, err := x.ExtractDoc("", "template.json", content)
	if err != nil {
		return false
	}
	for _, n := range res.Nodes {
		switch n.Kind {
		case "ARMTemplate":
			if armTemplateSchemaRe.MatchString(n.Signature) {
				return true
			}
		case "ARMParameterValue":
			m := armFirstSchemaRe.FindSubmatch(content)
			return m != nil && armParamsSchemaRe.Match(m[1])
		}
	}
	return false
}

// pulumiStackFile reports whether relPath is a Pulumi stack configuration
// file: the pulumi-yaml manifest claims it by glob (Pulumi.<stack>.yaml,
// never the project file, which its basename claims) and it decodes to a
// mapping whose `config` sets a key (the dialect's entities).
func pulumiStackFile(relPath string, content []byte) bool {
	v := connector.ClassifyDialect(dialectRules(), relPath, nil)
	if v.Dialect != string(ToolPulumiYAML) || v.By != connector.ClassifiedByGlob {
		return false
	}
	x := pulumiStackExtractor()
	if x == nil {
		return false
	}
	res, err := x.ExtractDoc("", path.Base(filepath.ToSlash(relPath)), content)
	return err == nil && len(res.Nodes) > 0
}

// appFrameworkDispatch returns the dialect's extractor and ToolKind when
// relPath is an ARM template or parameters file (.json, by content) or a
// Pulumi stack configuration file (.yaml/.yml, by the manifest's glob).
func appFrameworkDispatch(relPath string, content []byte) (extractor.Extractor, ToolKind, bool) {
	switch filepath.Ext(relPath) {
	case ".json":
		if armTemplateJSON(content) {
			return armExtractor(), ToolARMTemplate, true
		}
	case ".yaml", ".yml":
		if pulumiStackFile(relPath, content) {
			return pulumiStackExtractor(), ToolPulumiYAML, true
		}
	}
	return nil, "", false
}

// AppFrameworkContentDialect names the dialect ingest's dispatch reads
// relPath with by its content alone among this file's claims:
// "arm-templates" for an ARM template or parameters file, "" otherwise. The
// language-breadth harness measures the content-classified arm-templates
// dialect on exactly these files (indexbench lang-oracle), beside
// Pipeline.ContentDialect's CloudFormation and OpenAPI.
func AppFrameworkContentDialect(relPath string, content []byte) string {
	if filepath.Ext(relPath) == ".json" && armTemplateJSON(content) {
		return string(ToolARMTemplate)
	}
	return ""
}
