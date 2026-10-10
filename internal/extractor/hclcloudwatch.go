package extractor

// hclcloudwatch.go — LB-F371 (wave c3, item cw): CloudWatch as IaC in
// Terraform, the terraform dialect's half of LB-F368/LB-F369. A
// aws_cloudwatch_log_metric_filter's metric_transformation written with a
// literal namespace and name is a metric identity (a TerraformMetric entity,
// canonical key, named `<namespace>/<name>` and signed
// `aws.cloudwatch.metric=<namespace>/<name>`, the cloudformation dialect's
// CFNMetric form, so the identity join crosses dialects); a literal
// function_name on an aws_lambda_function is TL-5's Lambda function name (a
// TelemetryIdentity, as the cloudformation dialect's); an
// aws_cloudwatch_metric_alarm's literal namespace and metric_name, and each
// metric_query's metric, are identity references to the metric, and a
// literal log_group_name /aws/lambda/<name> an identity reference to the
// function (fusion's identityxref.go resolves both). The resource an alarm's
// dimensions name and the topics its actions notify are references already
// (collectRefs). A value from a variable, a local or an interpolation is
// decided at plan time and names nothing (a module's variable-driven alarm,
// terraform-aws-modules/terraform-aws-cloudwatch's every one: a ceiling).

import (
	"fmt"
	"regexp"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// KindTerraformMetric is a metric filter's metric identity.
const KindTerraformMetric core.SourceKind = "TerraformMetric"

// tfLambdaLogGroupRe is a Lambda function's log group name.
var tfLambdaLogGroupRe = regexp.MustCompile(`^/aws/lambda/([^/${}\s]+)$`)

// tfIdentityRef is one identity reference a resource makes.
type tfIdentityRef struct {
	edge, value string
	line        int
}

// tfStringAttr is an attribute written as a literal string.
func tfStringAttr(body *hclsyntax.Body, name string) (string, int, bool) {
	if body == nil {
		return "", 0, false
	}
	a, ok := body.Attributes[name]
	if !ok {
		return "", 0, false
	}
	switch e := a.Expr.(type) {
	case *hclsyntax.TemplateExpr:
		if !e.IsStringLiteral() {
			return "", 0, false
		}
	case *hclsyntax.LiteralValueExpr:
	default:
		return "", 0, false
	}
	v, diags := a.Expr.Value(nil)
	if diags.HasErrors() || v.Type() != cty.String || v.IsNull() {
		return "", 0, false
	}
	return v.AsString(), a.SrcRange.Start.Line, v.AsString() != ""
}

// tfIdentityNodes are the identity entities a resource declares: a metric
// filter's metrics (once per name in the block), a function's literal name.
func tfIdentityNodes(service, pkg, relPath, rType string, b *hclsyntax.Block) []core.SymbolNode {
	node := func(name, sig string, kind core.SourceKind, line, end int) core.SymbolNode {
		return core.SymbolNode{Service: service, Package: pkg, Name: name, Kind: kind, FilePath: relPath, Line: line, EndLine: end, Signature: sig, Tier: core.TierVerified}
	}
	var out []core.SymbolNode
	switch rType {
	case "aws_cloudwatch_log_metric_filter":
		seen := map[string]bool{}
		for _, mt := range b.Body.Blocks {
			if mt.Type != "metric_transformation" {
				continue
			}
			ns, _, ok1 := tfStringAttr(mt.Body, "namespace")
			name, _, ok2 := tfStringAttr(mt.Body, "name")
			if !ok1 || !ok2 || seen[ns+"/"+name] {
				continue
			}
			seen[ns+"/"+name] = true
			id := ns + "/" + name
			out = append(out, node(id, "aws.cloudwatch.metric="+id, KindTerraformMetric, mt.TypeRange.Start.Line, mt.Range().End.Line))
		}
	case "aws_lambda_function":
		if fn, _, ok := tfStringAttr(b.Body, "function_name"); ok {
			out = append(out, node(fn, "aws.lambda.function_name="+fn, KindTelemetryIdentity, b.TypeRange.Start.Line, b.Range().End.Line))
		}
	}
	return out
}

// tfIdentityRefs are the identity references a resource makes, by line.
func tfIdentityRefs(rType string, body *hclsyntax.Body) []tfIdentityRef {
	var out []tfIdentityRef
	metric := func(b *hclsyntax.Body, nsAttr, nameAttr string) {
		ns, line, ok1 := tfStringAttr(b, nsAttr)
		name, _, ok2 := tfStringAttr(b, nameAttr)
		if ok1 && ok2 {
			out = append(out, tfIdentityRef{edge: "metric", value: fmt.Sprintf("aws.cloudwatch.metric=%s/%s", ns, name), line: line})
		}
	}
	if rType == "aws_cloudwatch_metric_alarm" {
		metric(body, "namespace", "metric_name")
		for _, q := range body.Blocks {
			if q.Type != "metric_query" {
				continue
			}
			for _, m := range q.Body.Blocks {
				if m.Type == "metric" {
					metric(m.Body, "namespace", "metric_name")
				}
			}
		}
	}
	if lg, line, ok := tfStringAttr(body, "log_group_name"); ok {
		if m := tfLambdaLogGroupRe.FindStringSubmatch(lg); m != nil {
			out = append(out, tfIdentityRef{edge: "log_group", value: "aws.lambda.function_name=" + m[1], line: line})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].line < out[j].line })
	return out
}
