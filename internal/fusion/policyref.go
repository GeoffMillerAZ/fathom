package fusion

// policyref.go — LB-F135/LB-F136 (wave pc, docs/design/language-breadth.md §4
// "Policy references"): a policy names what it governs — a Kubernetes kind,
// a label selector, a cloud resource type, an ARN, a named entity — and the
// repository may declare those targets. ResolvePolicyRefs joins every
// policy_ref reference (Edge PolicyRefEdge) a policy dialect emits to the
// declared targets its strategy names; one resolver serves ingest
// (enrichPolicyRefEdges) and the language-breadth harness (indexbench
// lang-policies), so the bench never scores an edge ingest drops.
//
// Interim divergence from ADR-050 and F61 (Revisit when: RT-M1 reads PASS and
// SG-2 lands): F61 names policy_ref and forbids a new edge table before the
// table freeze lifts, so a policy edge is stored the route-ref way — a
// file-tier typed edge (core.EdgeFileResolved, tier pre-scan, never presented
// as typed) from the policy, rule or exception entity to the target entity,
// under the typed_edges key "policy-ref", replaced wholesale by a gated pass.
// No table, cache or background job. These edges feed impact (blast radius,
// the call graph), never a verdict.
//
// Strategies (the producer's DocCrossFileRef.Strategy):
//
//   - resource-kind: every Kubernetes object of the Target's kind (Kind: any
//     group; group/Kind: that group) in the reference's Scope (a comma list of
//     namespace patterns, "" any namespace), whose labels include Labels when
//     the reference carries a selector. Kubernetes objects are the nodes
//     ingest stores for a manifest (k8sextract's Kind/<ns>/<name>, its
//     Signature the apiVersion), their own metadata.labels read beside them.
//   - label-selector: every Kubernetes object in Scope whose labels include
//     Labels (labelsInclude, the doc-xref join).
//   - resource-type: every declared resource whose type equals Target: a
//     CloudFormation resource's Type (its Signature), a Terraform resource's
//     type (its name's first part), any other dialect's canonical resource
//     whose Signature is its type (an ARM or Bicep dialect joins with no edit).
//   - aws-arn: a literal ARN joins the declared resource of the ARN's service
//     and resource type whose physical name the repository declares (a
//     CloudFormation BucketName, a Terraform bucket = "..."); a provider
//     declares one as a DocLabelSet in scope PolicyPhysicalNameScope.
//   - service-name: the one entity of that name in another file (the doc-xref
//     unique-name join).
//
// Fan-out: a policy governs every target its strategy names, which in a
// policy library (a test fixture per policy, thousands of Pods) is far more
// than a policy's own. The join takes the nearest targets — in the policy's
// own file, else its directory and the directories below it, else the
// repository — and refuses more than policyFanOutCap of them, counted as
// fan-out. Unresolved is never guessed: a wildcard, a variable, an external
// or undeclared target, an alias no type table maps, an ambiguous name, and
// a fan-out each yield no edge and are counted by reason.

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
	"gopkg.in/yaml.v3"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// The policy contract every policy producer emits (docs/design/
// language-breadth.md §4): an extractor.DocCrossFileRef with this Edge, one
// of these Strategies, Raw the target as written, Target its normalized
// form (Raw when there is nothing to normalize), Scope and Labels for a
// selector.
const (
	PolicyRefEdge       = "policy_ref"
	PolicyResourceKind  = "resource-kind"
	PolicyLabelSelector = "label-selector"
	PolicyResourceType  = "resource-type"
	PolicyAWSARN        = "aws-arn"
	PolicyServiceName   = "service-name"
)

// Why a policy reference resolved to nothing (PolicyResolution.Unresolved).
const (
	// PolicyWildcard: the target is every kind or resource ("*", an empty
	// selector, an ARN or type with a wildcard).
	PolicyWildcard = "wildcard"
	// PolicyVariable: a value the file does not fix (${X}, {{ }},
	// [parameters(..)]).
	PolicyVariable = "variable"
	// PolicyExternal: a literal ARN whose physical name the repository
	// declares for no resource of its type.
	PolicyExternal = "external"
	// PolicyUndeclared: a kind, selector, type or name nothing in the
	// repository declares.
	PolicyUndeclared = "undeclared"
	// PolicyUnmapped: a short alias or plural no table maps to a declared
	// type (a custom resource's plural, an ARN service with no type).
	PolicyUnmapped = "unmapped"
	// PolicyAmbiguous: a service-name declared by more than one entity.
	PolicyAmbiguous = "ambiguous"
	// PolicyFanOut: the nearest targets number more than policyFanOutCap.
	PolicyFanOut = "fan-out"
)

// PolicyReasons is every unresolved reason, in the order reports print them.
var PolicyReasons = []string{PolicyWildcard, PolicyVariable, PolicyExternal, PolicyUndeclared, PolicyUnmapped, PolicyAmbiguous, PolicyFanOut}

// PolicyPhysicalNameScope is the DocLabelSet scope a provider declares a
// resource's physical name in (Labels {"name": <physical name>}); no
// namespace can be named so.
const PolicyPhysicalNameScope = "aws:physical-name"

// policyFanOutCap is the most targets one reference joins.
const policyFanOutCap = 256

// policyRefPackageKey is the typed_edges key the policy-ref pass replaces.
const policyRefPackageKey = "policy-ref"

const policyRefStateKey = typedPassStatePrefix + "policy-ref"

// PolicyRef is one resolved policy reference: policy entity to target.
type PolicyRef struct {
	FromFile, From, FromKey string
	ToFile, To, ToKey       string
	Strategy, Raw, Target   string
	Line                    int
}

// PolicyUnresolved is one policy reference that resolved to nothing.
type PolicyUnresolved struct {
	FromFile, From, Strategy, Raw, Reason string
	Line                                  int
}

// PolicyResolution is ResolvePolicyRefs' answer.
type PolicyResolution struct {
	Refs       []PolicyRef
	Unresolved []PolicyUnresolved
	// SelfDropped counts joins of a reference to its own referring entity.
	SelfDropped int
}

// UnresolvedByReason counts Unresolved by reason (only reasons that occur).
func (r PolicyResolution) UnresolvedByReason() map[string]int {
	if len(r.Unresolved) == 0 {
		return nil
	}
	out := map[string]int{}
	for _, u := range r.Unresolved {
		out[u.Reason]++
	}
	return out
}

// policyTarget is one declared target a reference may join.
type policyTarget struct {
	file string
	node core.SymbolNode
	// Kubernetes objects: kind, group ("" core; groupKnown false when the
	// node does not say), namespace and metadata.labels.
	k8s        bool
	kind, ns   string
	group      string
	groupKnown bool
	labels     map[string]string
	// declared resources: type and physical name.
	typ, physical string
}

// k8sTargetOf reads a Kubernetes object node: its kind and namespace from
// the Kind/<ns>/<name> name, its group from the incumbent's Signature (the
// apiVersion) when the Signature is one.
func k8sTargetOf(file string, n core.SymbolNode) policyTarget {
	parts := strings.Split(n.Name, "/")
	t := policyTarget{file: file, node: n, k8s: true, kind: parts[0], ns: "default"}
	if len(parts) == 3 {
		t.ns = parts[1]
	}
	switch sig := n.Signature; {
	case strings.Contains(sig, "/") && !strings.ContainsAny(sig, " \t"):
		t.group, t.groupKnown = sig[:strings.Index(sig, "/")], true
	case len(sig) > 1 && sig[0] == 'v' && sig[1] >= '0' && sig[1] <= '9':
		t.groupKnown = true // core: "v1"
	}
	return t
}

// declaredResourceType is the type a node declares, "" for none: a
// Terraform resource's type, or the Signature of another dialect's
// canonical resource (a CloudFormation resource's Type). A Kubernetes
// object is resource-kind's, never a type.
func declaredResourceType(n core.SymbolNode) string {
	switch n.Kind {
	case core.KindK8sWorkload:
		return ""
	case core.KindTerraformResource:
		t, _, _ := strings.Cut(n.Name, ".")
		return t
	}
	if c, ok := extractor.CanonicalKind(string(n.Kind)); ok && c == "resource" && n.Signature != "" && !strings.ContainsAny(n.Signature, " \t") {
		return n.Signature
	}
	return ""
}

// policyVariable reports a value the file does not fix.
func policyVariable(s string) bool {
	return strings.Contains(s, "${") || strings.Contains(s, "{{") || strings.Contains(s, "[parameters(") ||
		strings.Contains(s, "[variables(") || strings.Contains(s, "[concat(")
}

// nsInScope reports whether namespace ns is in scope (a comma list of
// namespace patterns, "" every namespace).
func nsInScope(ns, scope string) bool {
	if scope == "" {
		return true
	}
	for _, p := range strings.Split(scope, ",") {
		p = strings.TrimSpace(p)
		if p == ns || p == "*" {
			return true
		}
		if ok, _ := path.Match(p, ns); ok {
			return true
		}
	}
	return false
}

// arnTypes maps an ARN's service and resource type to the declared types
// (CloudFormation, Terraform) whose physical name it carries, and how to
// read the physical name from the ARN's resource part.
type arnRule struct {
	types []string
	name  func(resource string) string
}

func afterPrefix(prefix string) func(string) string {
	return func(r string) string {
		rest, ok := strings.CutPrefix(r, prefix)
		if !ok {
			return ""
		}
		if i := strings.IndexAny(rest, "/:"); i >= 0 {
			rest = rest[:i]
		}
		return rest
	}
}

func lastPathSeg(prefix string) func(string) string {
	return func(r string) string {
		rest, ok := strings.CutPrefix(r, prefix)
		if !ok {
			return ""
		}
		if i := strings.LastIndex(rest, "/"); i >= 0 {
			rest = rest[i+1:]
		}
		return rest
	}
}

var arnRules = map[string][]arnRule{
	"s3": {{[]string{"AWS::S3::Bucket", "aws_s3_bucket"}, func(r string) string {
		b, _, _ := strings.Cut(r, "/")
		return b
	}}},
	"dynamodb": {{[]string{"AWS::DynamoDB::Table", "aws_dynamodb_table"}, afterPrefix("table/")}},
	"lambda":   {{[]string{"AWS::Lambda::Function", "aws_lambda_function"}, afterPrefix("function:")}},
	"iam": {
		{[]string{"AWS::IAM::Role", "aws_iam_role"}, lastPathSeg("role/")},
		{[]string{"AWS::IAM::User", "aws_iam_user"}, lastPathSeg("user/")},
		{[]string{"AWS::IAM::ManagedPolicy", "aws_iam_policy"}, lastPathSeg("policy/")},
	},
	"sqs":     {{[]string{"AWS::SQS::Queue", "aws_sqs_queue"}, func(r string) string { return r }}},
	"sns":     {{[]string{"AWS::SNS::Topic", "aws_sns_topic"}, func(r string) string { return r }}},
	"kinesis": {{[]string{"AWS::Kinesis::Stream", "aws_kinesis_stream"}, afterPrefix("stream/")}},
	"ecr":     {{[]string{"AWS::ECR::Repository", "aws_ecr_repository"}, func(r string) string { return strings.TrimPrefix(r, "repository/") }}},
	"logs":    {{[]string{"AWS::Logs::LogGroup", "aws_cloudwatch_log_group"}, afterPrefix("log-group:")}},
	"states":  {{[]string{"AWS::StepFunctions::StateMachine", "aws_sfn_state_machine"}, afterPrefix("stateMachine:")}},
	"kms":     {{[]string{"AWS::KMS::Alias", "aws_kms_alias"}, func(r string) string { return r }}},
}

// arnTarget reads an ARN into the declared types it names and the physical
// name, or the reason it cannot join.
func arnTarget(arn string) (types []string, name, reason string) {
	parts := strings.SplitN(arn, ":", 6)
	if len(parts) < 6 || parts[0] != "arn" {
		if strings.Contains(arn, "*") {
			return nil, "", PolicyWildcard
		}
		return nil, "", PolicyUnmapped
	}
	service, resource := parts[2], parts[5]
	for _, r := range arnRules[service] {
		n := r.name(resource)
		if n == "" {
			continue
		}
		if strings.ContainsAny(n, "*?") {
			return nil, "", PolicyWildcard
		}
		return r.types, n, ""
	}
	if strings.ContainsAny(resource, "*?") {
		return nil, "", PolicyWildcard
	}
	return nil, "", PolicyUnmapped
}

// ResolvePolicyRefs resolves every file's policy_ref references against
// the targets the same files declare (deterministic, sorted). A
// reference's referring entity is the node of its From name in its own
// file; a reference whose entity no node carries yields nothing. Each
// strategy's candidates are narrowed to the nearest ones (the policy's
// file, else its directory subtree, else the repository); more than
// policyFanOutCap of them is a fan-out. A join to the referring entity
// itself is dropped (SelfDropped).
func ResolvePolicyRefs(files []DocFile) PolicyResolution {
	type fk struct{ file, name string }
	nodeAt := map[fk]core.SymbolNode{}
	byName := map[string][]fk{}
	labelsOf := map[fk]map[string]string{}
	physicalOf := map[fk]string{}
	for _, f := range files {
		for _, n := range f.Result.Nodes {
			k := fk{f.Path, n.Name}
			if _, dup := nodeAt[k]; !dup {
				nodeAt[k] = n
				byName[n.Name] = append(byName[n.Name], k)
			}
		}
		for _, ls := range f.Result.Labels {
			k := fk{f.Path, ls.Entity}
			if ls.Scope == PolicyPhysicalNameScope {
				if v := ls.Labels["name"]; v != "" {
					physicalOf[k] = v
				}
				continue
			}
			if _, dup := labelsOf[k]; !dup {
				labelsOf[k] = ls.Labels
			}
		}
	}
	var k8s, typed []policyTarget
	for _, f := range files {
		seen := map[string]bool{}
		for _, n := range f.Result.Nodes {
			if seen[n.Name] {
				continue
			}
			seen[n.Name] = true
			k := fk{f.Path, n.Name}
			if n.Kind == core.KindK8sWorkload && strings.Count(n.Name, "/") >= 1 {
				t := k8sTargetOf(f.Path, n)
				t.labels = labelsOf[k]
				k8s = append(k8s, t)
				continue
			}
			if typ := declaredResourceType(n); typ != "" {
				typed = append(typed, policyTarget{file: f.Path, node: n, typ: typ, physical: physicalOf[k]})
			}
		}
	}

	var res PolicyResolution
	seen := map[string]bool{}
	for _, f := range files {
		for _, c := range f.Result.CrossFile {
			if c.Edge != PolicyRefEdge {
				continue
			}
			from, ok := nodeAt[fk{f.Path, c.From}]
			if !ok {
				continue
			}
			miss := func(reason string) {
				res.Unresolved = append(res.Unresolved, PolicyUnresolved{FromFile: f.Path, From: c.From, Strategy: c.Strategy, Raw: c.Raw, Reason: reason, Line: c.Line})
			}
			target := c.Target
			if target == "" && (c.Strategy == PolicyAWSARN || c.Strategy == PolicyServiceName) {
				target = c.Raw // an ARN or a name is its own normal form
			}
			variable := policyVariable(c.Raw) || policyVariable(target) || policyVariable(c.Scope)
			for k, v := range c.Labels {
				variable = variable || policyVariable(k) || policyVariable(v)
			}
			if variable {
				miss(PolicyVariable)
				continue
			}
			var cands []policyTarget
			missReason := PolicyUndeclared
			switch c.Strategy {
			case PolicyResourceKind:
				switch {
				case target == "":
					miss(PolicyUnmapped)
					continue
				case target == "*" || strings.HasSuffix(target, "/*"):
					miss(PolicyWildcard)
					continue
				}
				group, kind, grouped := "", target, false
				if i := strings.LastIndex(target, "/"); i >= 0 {
					group, kind, grouped = target[:i], target[i+1:], true
				}
				for _, t := range k8s {
					if t.kind != kind || !nsInScope(t.ns, c.Scope) {
						continue
					}
					if grouped && t.groupKnown && t.group != group {
						continue
					}
					if len(c.Labels) > 0 && !labelsInclude(t.labels, c.Labels) {
						continue
					}
					cands = append(cands, t)
				}
			case PolicyLabelSelector:
				if len(c.Labels) == 0 {
					miss(PolicyWildcard)
					continue
				}
				for _, t := range k8s {
					if nsInScope(t.ns, c.Scope) && labelsInclude(t.labels, c.Labels) {
						cands = append(cands, t)
					}
				}
			case PolicyResourceType:
				switch {
				case target == "":
					miss(PolicyUnmapped) // an alias the producer maps to no declared type
					continue
				case strings.ContainsAny(target, "*?"):
					miss(PolicyWildcard)
					continue
				}
				for _, t := range typed {
					if t.typ == target {
						cands = append(cands, t)
					}
				}
			case PolicyAWSARN:
				types, name, reason := arnTarget(target)
				if reason != "" {
					miss(reason)
					continue
				}
				missReason = PolicyExternal
				for _, t := range typed {
					if t.physical == name && containsString(types, t.typ) {
						cands = append(cands, t)
					}
				}
			case PolicyServiceName:
				var hit []fk
				for _, k := range byName[target] {
					if k.file != f.Path {
						hit = append(hit, k)
					}
				}
				switch len(hit) {
				case 0:
					miss(PolicyUndeclared)
				case 1:
					cands = []policyTarget{{file: hit[0].file, node: nodeAt[hit[0]]}}
				default:
					miss(PolicyAmbiguous)
				}
				if len(hit) != 1 {
					continue
				}
			default:
				continue
			}
			if len(cands) == 0 {
				miss(missReason)
				continue
			}
			near := nearestPolicyTargets(f.Path, cands)
			if len(near) > policyFanOutCap {
				miss(PolicyFanOut)
				continue
			}
			for _, to := range near {
				if to.node.Key() == from.Key() {
					res.SelfDropped++
					continue
				}
				r := PolicyRef{FromFile: f.Path, From: c.From, FromKey: from.Key(), ToFile: to.file, To: to.node.Name, ToKey: to.node.Key(),
					Strategy: c.Strategy, Raw: c.Raw, Target: target, Line: c.Line}
				id := r.FromFile + "\x00" + r.From + "\x00" + r.ToFile + "\x00" + r.To + "\x00" + r.Strategy + "\x00" + r.Raw
				if !seen[id] {
					seen[id] = true
					res.Refs = append(res.Refs, r)
				}
			}
		}
	}
	sort.Slice(res.Refs, func(i, j int) bool {
		a, b := res.Refs[i], res.Refs[j]
		for _, p := range [][2]string{{a.FromFile, b.FromFile}, {a.From, b.From}, {a.ToFile, b.ToFile}, {a.To, b.To}, {a.Strategy, b.Strategy}, {a.Raw, b.Raw}} {
			if p[0] != p[1] {
				return p[0] < p[1]
			}
		}
		return a.Line < b.Line
	})
	sort.Slice(res.Unresolved, func(i, j int) bool {
		a, b := res.Unresolved[i], res.Unresolved[j]
		for _, p := range [][2]string{{a.FromFile, b.FromFile}, {a.From, b.From}, {a.Strategy, b.Strategy}, {a.Raw, b.Raw}, {a.Reason, b.Reason}} {
			if p[0] != p[1] {
				return p[0] < p[1]
			}
		}
		return a.Line < b.Line
	})
	return res
}

// nearestPolicyTargets keeps the candidates nearest the policy file: in the
// file itself, else in its directory and the directories below it, else
// all of them.
func nearestPolicyTargets(file string, cands []policyTarget) []policyTarget {
	dir := path.Dir(file)
	for _, in := range []func(string) bool{
		func(p string) bool { return p == file },
		func(p string) bool { return dir == "." || p == dir || strings.HasPrefix(p, dir+"/") },
	} {
		var near []policyTarget
		for _, t := range cands {
			if in(t.file) {
				near = append(near, t)
			}
		}
		if len(near) > 0 {
			return near
		}
	}
	return cands
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// PolicyRefEdges is the store form of resolved policy references: one
// file-tier typed edge per (policy entity, target entity) symbol pair, sorted.
func PolicyRefEdges(refs []PolicyRef) []core.Edge {
	seen := map[[2]string]bool{}
	var edges []core.Edge
	for _, r := range refs {
		k := [2]string{r.FromKey, r.ToKey}
		if seen[k] {
			continue
		}
		seen[k] = true
		edges = append(edges, core.Edge{FromFile: r.FromFile, FromSymbol: r.FromKey, ToSymbol: r.ToKey, Kind: core.EdgeFileResolved})
	}
	sortEdges(edges)
	return edges
}

// isPolicyCandidate: a path the policy pass reads — every path a doc or
// native dialect's manifest claims (isRouteCandidate: Kubernetes manifests,
// Terraform, a policy dialect added later) and every .json (a CloudFormation
// template, an IAM or Azure Policy document, each claimed by its content).
func isPolicyCandidate(p string) bool {
	return isRouteCandidate(p) || strings.EqualFold(path.Ext(p), ".json")
}

// IsPolicyCandidate reports whether the policy pass reads relPath (the
// harness's file set, the same predicate ingest gates on).
func IsPolicyCandidate(relPath string) bool { return isPolicyCandidate(relPath) }

// policyFile reads one candidate file for the policy pass: the nodes
// ingest's dispatch stores for it, its policy_ref references (a
// DocResulter's own), and the facts its targets carry beside the nodes — a
// Kubernetes object's metadata.labels (read from the manifest: the
// incumbent stores none, and the kubernetes dialect's labels are a
// workload's pod-template labels, a Service selector's join), and a
// CloudFormation or Terraform resource's physical name. ok is false for a
// file with neither a reference nor a target; its nodes are trimmed to the
// referring entities and the targets.
func (p *Pipeline) policyFile(service string, idx *siblingIndex, relPath string, content []byte) (DocFile, bool) {
	e, tk, ok, _ := p.resolveExtractor(relPath, content, idx)
	if !ok || e == nil {
		return DocFile{}, false
	}
	var nodes []core.SymbolNode
	var refs []extractor.DocCrossFileRef
	if doc, isDoc := e.(extractor.DocResulter); isDoc {
		res, err := doc.ExtractDoc(service, relPath, content)
		if err != nil {
			return DocFile{}, false
		}
		nodes = res.Nodes
		for _, c := range res.CrossFile {
			if c.Edge == PolicyRefEdge {
				refs = append(refs, c)
			}
		}
	} else {
		if tk != ToolK8sManifest && tk != ToolK8sCRD {
			return DocFile{}, false
		}
		ns, err := e.Extract(service, relPath, content)
		if err != nil {
			return DocFile{}, false
		}
		nodes = ns
	}
	var labels []extractor.DocLabelSet
	for _, n := range nodes {
		if n.Kind == core.KindK8sWorkload {
			labels = k8sObjectLabels(content)
			break
		}
	}
	switch {
	case tk == ToolCloudFormation:
		labels = append(labels, cfnPhysicalNames(content)...)
	case strings.EqualFold(path.Ext(relPath), ".tf"):
		labels = append(labels, tfPhysicalNames(relPath, content)...)
	}
	keep := policyEntityNodes(nodes, refs)
	for _, n := range nodes {
		if (n.Kind == core.KindK8sWorkload && strings.Contains(n.Name, "/")) || declaredResourceType(n) != "" {
			keep = append(keep, n)
		}
	}
	if len(keep) == 0 {
		return DocFile{}, false
	}
	return DocFile{Path: relPath, Result: extractor.DocResult{Nodes: keep, CrossFile: refs, Labels: labels}}, true
}

// k8sObjectLabels reads each object's own metadata.labels, as a label set
// on its Kind/<ns>/<name> entity in its namespace.
func k8sObjectLabels(content []byte) []extractor.DocLabelSet {
	var out []extractor.DocLabelSet
	for _, root := range yamlDocs(content) {
		kind, name := yscalar(ymap(root, "kind")), yscalar(ymap(root, "metadata", "name"))
		if kind == "" || name == "" {
			continue
		}
		ns := yscalar(ymap(root, "metadata", "namespace"))
		lab := ymap(root, "metadata", "labels")
		m, _, ok := ylabels(lab)
		if !ok {
			continue
		}
		scope := ns
		if scope == "" {
			scope = "default"
		}
		out = append(out, extractor.DocLabelSet{Entity: k8sEntityName(kind, ns, name), Scope: scope, Labels: m, Line: lab.Line})
	}
	return out
}

// cfnNameProperty is the property a CloudFormation resource type declares
// its physical name in.
var cfnNameProperty = map[string]string{
	"AWS::S3::Bucket": "BucketName", "AWS::DynamoDB::Table": "TableName", "AWS::Lambda::Function": "FunctionName",
	"AWS::IAM::Role": "RoleName", "AWS::IAM::User": "UserName", "AWS::IAM::ManagedPolicy": "ManagedPolicyName",
	"AWS::SQS::Queue": "QueueName", "AWS::SNS::Topic": "TopicName", "AWS::Kinesis::Stream": "Name",
	"AWS::ECR::Repository": "RepositoryName", "AWS::Logs::LogGroup": "LogGroupName",
	"AWS::StepFunctions::StateMachine": "StateMachineName", "AWS::KMS::Alias": "AliasName",
}

// cfnPhysicalNames reads each CloudFormation resource's literal physical
// name (an intrinsic, a tagged or non-scalar value, is none).
func cfnPhysicalNames(content []byte) []extractor.DocLabelSet {
	var out []extractor.DocLabelSet
	for _, root := range yamlDocs(content) {
		rs := ymap(root, "Resources")
		if rs == nil || rs.Kind != yaml.MappingNode {
			continue
		}
		for i := 0; i+1 < len(rs.Content); i += 2 {
			id, r := rs.Content[i].Value, rs.Content[i+1]
			prop, ok := cfnNameProperty[yscalar(ymap(r, "Type"))]
			if !ok {
				continue
			}
			v := ymap(r, "Properties", prop)
			if name := yscalar(v); name != "" && (v.Tag == "" || v.Tag == "!!str") {
				out = append(out, extractor.DocLabelSet{Entity: id, Scope: PolicyPhysicalNameScope, Labels: map[string]string{"name": name}, Line: v.Line})
			}
		}
	}
	return out
}

// tfNameAttribute is the attribute a Terraform resource type declares its
// physical name in.
var tfNameAttribute = map[string]string{
	"aws_s3_bucket": "bucket", "aws_dynamodb_table": "name", "aws_lambda_function": "function_name",
	"aws_iam_role": "name", "aws_iam_user": "name", "aws_iam_policy": "name", "aws_sqs_queue": "name",
	"aws_sns_topic": "name", "aws_kinesis_stream": "name", "aws_ecr_repository": "name",
	"aws_cloudwatch_log_group": "name", "aws_sfn_state_machine": "name", "aws_kms_alias": "name",
}

// tfPhysicalNames reads each Terraform resource's literal physical name (an
// interpolation or a reference is none).
func tfPhysicalNames(relPath string, content []byte) []extractor.DocLabelSet {
	f, diags := hclsyntax.ParseConfig(content, relPath, hcl.InitialPos)
	if diags.HasErrors() || f == nil {
		return nil
	}
	body, ok := f.Body.(*hclsyntax.Body)
	if !ok {
		return nil
	}
	var out []extractor.DocLabelSet
	for _, b := range body.Blocks {
		if b.Type != "resource" || len(b.Labels) < 2 {
			continue
		}
		attr, ok := tfNameAttribute[b.Labels[0]]
		if !ok {
			continue
		}
		a := b.Body.Attributes[attr]
		if a == nil {
			continue
		}
		v, d := a.Expr.Value(nil)
		if d.HasErrors() || !v.IsKnown() || v.IsNull() || v.Type() != cty.String {
			continue
		}
		out = append(out, extractor.DocLabelSet{Entity: b.Labels[0] + "." + b.Labels[1], Scope: PolicyPhysicalNameScope,
			Labels: map[string]string{"name": v.AsString()}, Line: a.SrcRange.Start.Line})
	}
	return out
}

// PolicyFileForBench is policyFile for the language-breadth harness
// (indexbench lang-policies): ingest's gates, dispatch and policy reading
// on one file of a subject, so the bench scores exactly what ingest stores.
func (p *Pipeline) PolicyFileForBench(serviceID string, refs []core.SourceRef, relPath string, content []byte) (DocFile, bool) {
	if len(content) > MaxExtractSize || connector.IsBinary(content) || gateDialectFile(relPath, content).excluded != "" {
		return DocFile{}, false
	}
	p.mu.Lock()
	idx, ok := p.siblingIdx[serviceID]
	if !ok {
		idx = buildSiblingIndex(refs)
		p.siblingIdx[serviceID] = idx
	}
	p.mu.Unlock()
	return p.policyFile(serviceID, idx, relPath, content)
}

// policyRefReport is the policy-ref pass's disclosure on the ingest Report.
type policyRefReport struct {
	edges      int
	unresolved map[string]int
	skipped    string
}

// enrichPolicyRefEdges is the Ingest hook for LB-F136: when a policy
// candidate changed (or the last pass did not complete), every candidate is
// read through ingest's gates and dispatch (policyFile), the policy_ref
// references are resolved (ResolvePolicyRefs) and the edges replace the
// "policy-ref" typed edges wholesale. Best-effort per file; a halt is
// returned. No table, cache or background job (F61, the table freeze):
// typed_edges and the pass-state row are existing rows, and the target
// index lives for one pass.
func (p *Pipeline) enrichPolicyRefEdges(ctx context.Context, svc core.ServiceRef, refs []core.SourceRef, changed []string) (policyRefReport, error) {
	if p.cfg.DisableTypedEdges {
		return policyRefReport{skipped: "typed edges disabled"}, nil
	}
	if !passNeeded(changed, p.passStateOK(ctx, svc.ID, policyRefStateKey), isPolicyCandidate) {
		return policyRefReport{skipped: SkippedPassReason}, nil
	}
	p.beginPassState(ctx, svc.ID, policyRefStateKey)
	idx := buildSiblingIndex(refs)
	var files []DocFile
	for _, r := range refs {
		if !isPolicyCandidate(r.Path) {
			continue
		}
		content, err := p.conn.Fetch(ctx, r)
		if err != nil {
			if h := haltError(ctx, err); h != nil {
				return policyRefReport{}, h
			}
			continue
		}
		if len(content) > MaxExtractSize || connector.IsBinary(content) || gateDialectFile(r.Path, content).excluded != "" {
			continue
		}
		if f, ok := p.policyFile(svc.ID, idx, r.Path, content); ok {
			files = append(files, f)
		}
	}
	res := ResolvePolicyRefs(files)
	edges := PolicyRefEdges(res.Refs)
	if err := p.st.PutTypedEdges(ctx, svc.ID, policyRefPackageKey, edges); err != nil {
		return policyRefReport{}, fmt.Errorf("policy-ref edges: store: %w", err)
	}
	p.commitPassState(ctx, svc.ID, policyRefStateKey)
	return policyRefReport{edges: len(edges), unresolved: res.UnresolvedByReason()}, nil
}
