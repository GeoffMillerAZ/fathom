package fusion

// policydispatch.go — ingest's claim of Kyverno and OPA Gatekeeper files
// (LB-F134, language-breadth wave pc). Both are Kubernetes objects, so before
// this claim every one of them was a K8sWorkload of k8sextract.go
// (`ClusterPolicy/<name>`); now a .yaml or .yml whose every document is a
// Kyverno or a Gatekeeper object reads through its doc-engine dialect
// (internal/extractor/dialects/{kyverno,gatekeeper}), and the dialect's
// policy targets reach the policy-ref pass as policy_ref references
// (policyref.go).
//
// A file is claimed by its documents' top-level apiVersion group, decided
// before the Kubernetes verdict (one hook in resolveExtractor, one in
// ReingestFile, each after gatewayDispatch):
//
//   - kyverno: kyverno.io (ClusterPolicy, Policy, PolicyException, the
//     cleanup policies), policies.kyverno.io (ValidatingPolicy, MutatingPolicy,
//     GeneratingPolicy, DeletingPolicy, ImageValidatingPolicy, their
//     Namespaced* forms, PolicyException) and cli.kyverno.io kind Test (the
//     Kyverno CLI's test manifest, whose results name policies and rules);
//   - gatekeeper: templates.gatekeeper.sh (ConstraintTemplate),
//     constraints.gatekeeper.sh (every Constraint kind) and test.gatekeeper.sh
//     kind Suite (gator verify's suite, which names template and constraint
//     files).
//
// Multi-document rule: a file is claimed only when every document that is an
// object (a top-level apiVersion or kind) belongs to the same dialect. A file
// that mixes a policy with the Deployments it governs, or with another tool's
// objects (a chainsaw test, a kyverno CLI Values file), stays Kubernetes, as
// it was: claiming it would drop the other documents' K8sWorkload symbols,
// which no policy dialect extracts. A document with neither key (a comment, an
// empty document) is ignored.
//
// The dialects are content-claimed (their manifests name no extension,
// basename or glob: a .yaml is Kubernetes's), so the language-breadth harness
// owns their files through PolicyDialect, the same predicate.

import (
	"bytes"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"gopkg.in/yaml.v3"
)

const (
	// ToolKyverno: a Kyverno policy, exception or CLI test file (LB-Q-kyverno).
	ToolKyverno ToolKind = "kyverno"
	// ToolGatekeeper: a Gatekeeper ConstraintTemplate, constraint or gator
	// suite file (LB-Q-gatekeeper).
	ToolGatekeeper ToolKind = "gatekeeper"
)

// policyKinds maps a dialect id to its ToolKind.
var policyKinds = map[string]ToolKind{
	"kyverno":    ToolKyverno,
	"gatekeeper": ToolGatekeeper,
}

var (
	// policyAPIRe is the cheap pre-check: some top-level apiVersion names a
	// Kyverno or Gatekeeper group. Only a file it matches is split into
	// documents.
	policyAPIRe = regexp.MustCompile(`(?m)^apiVersion[ \t]*:[ \t]*["']?(?:(?:(?:policies|cli)\.)?kyverno\.io/|(?:templates|constraints|test)\.gatekeeper\.sh/)`)
	// docSepRe splits a YAML stream at its document markers.
	docSepRe = regexp.MustCompile(`(?m)^---[ \t]*(?:#[^\n]*)?\r?$`)
	// topAPIVersionRe and topKindRe read a document's top-level apiVersion
	// and kind (classify.go's byte-level reading, with YAML's optional space
	// before the colon: `apiVersion : kyverno.io/v1` is valid and written).
	topAPIVersionRe = regexp.MustCompile(`(?m)^apiVersion[ \t]*:[ \t]*["']?([^\s"'#]+)`)
	topKindRe       = regexp.MustCompile(`(?m)^kind[ \t]*:[ \t]*["']?([^\s"'#]+)`)
)

var (
	policyOnce sync.Once
	policyExts map[string]*extractor.DocExtractor
)

// policyExtractors is the two dialects' document engines, built once; a
// dialect that does not load is absent (TestDialectManifests fails the build's
// tests on that, and dispatch then claims nothing for it).
func policyExtractors() map[string]*extractor.DocExtractor {
	policyOnce.Do(func() {
		policyExts = map[string]*extractor.DocExtractor{}
		for id := range policyKinds {
			if x, err := extractor.NewDocDialectExtractor(id); err == nil {
				policyExts[id] = x
			}
		}
	})
	return policyExts
}

// policyGroupDialect is the dialect one document's apiVersion and kind
// belong to, "" for none.
func policyGroupDialect(apiVersion, kind string) string {
	group, _, ok := strings.Cut(apiVersion, "/")
	if !ok {
		return ""
	}
	switch group {
	case "kyverno.io", "policies.kyverno.io":
		return "kyverno"
	case "cli.kyverno.io":
		if kind == "Test" {
			return "kyverno"
		}
	case "templates.gatekeeper.sh", "constraints.gatekeeper.sh":
		return "gatekeeper"
	case "test.gatekeeper.sh":
		if kind == "Suite" {
			return "gatekeeper"
		}
	}
	return ""
}

// PolicyDialect is the policy dialect ("kyverno", "gatekeeper") that claims
// relPath, "" for none: a .yaml or .yml whose every object document belongs
// to that dialect (the multi-document rule above). The harness owns a
// content-claimed dialect's files through it (indexbench lang-policies).
func PolicyDialect(relPath string, content []byte) string {
	switch strings.ToLower(filepath.Ext(relPath)) {
	case ".yaml", ".yml":
	default:
		return ""
	}
	if !policyAPIRe.Match(content) {
		return ""
	}
	dialect := ""
	for _, doc := range docSepRe.Split(string(content), -1) {
		av, kd := "", ""
		if m := topAPIVersionRe.FindStringSubmatch(doc); m != nil {
			av = m[1]
		}
		if m := topKindRe.FindStringSubmatch(doc); m != nil {
			kd = m[1]
		}
		if av == "" && kd == "" {
			if strings.TrimSpace(stripYAMLComments(doc)) != "" {
				return "" // a document that is not an object: not a policy file
			}
			continue
		}
		d := policyGroupDialect(av, kd)
		if d == "" || (dialect != "" && d != dialect) {
			return ""
		}
		dialect = d
	}
	return dialect
}

// stripYAMLComments drops whole-line comments (what an otherwise empty
// document holds).
func stripYAMLComments(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(line); t != "" && !strings.HasPrefix(t, "#") {
			b.WriteString(t)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// policyDispatch returns the policy dialect's extractor and ToolKind when
// PolicyDialect claims relPath and the dialect's content sniff agrees.
func policyDispatch(relPath string, content []byte) (extractor.Extractor, ToolKind, bool) {
	id := PolicyDialect(relPath, content)
	if id == "" {
		return nil, "", false
	}
	x := policyExtractors()[id]
	if x == nil || !x.DetectsContent(content) {
		return nil, "", false
	}
	return policyDocExtractor{DocExtractor: x, refs: policyRefReaders[id]}, policyKinds[id], true
}

// policyDocExtractor is a policy dialect's document engine plus its policy
// targets: the doc spec draws the entities, references and cross-file
// references, and refs reads the match blocks into policy_ref references
// (strategies resource-kind and label-selector), which a doc spec cannot
// express (a kind's group and version normalized, a plural resource mapped
// to its kind, a Gatekeeper kinds × apiGroups product, a selector narrowing
// the kinds of its own block).
type policyDocExtractor struct {
	*extractor.DocExtractor
	refs func(content []byte) []extractor.DocCrossFileRef
}

// ExtractDoc implements extractor.DocResulter: the dialect's result, with
// its policy_ref references appended (sorted as the engine sorts).
func (x policyDocExtractor) ExtractDoc(service, relPath string, src []byte) (extractor.DocResult, error) {
	res, err := x.DocExtractor.ExtractDoc(service, relPath, src)
	if err != nil || len(res.Nodes) == 0 || x.refs == nil {
		return res, err
	}
	res.CrossFile = append(res.CrossFile, x.refs(src)...)
	sort.SliceStable(res.CrossFile, func(i, j int) bool {
		a, b := res.CrossFile[i], res.CrossFile[j]
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Raw < b.Raw
	})
	return res, nil
}

var policyRefReaders = map[string]func([]byte) []extractor.DocCrossFileRef{
	"kyverno":    kyvernoPolicyRefs,
	"gatekeeper": gatekeeperPolicyRefs,
}

// ── policy targets ──────────────────────────────────────────────────────────

// yamlDocs decodes every document of a YAML stream; a document that fails
// to decode ends the stream (the doc engine's rule: what decoded stands).
func yamlDocs(content []byte) []*yaml.Node {
	dec := yaml.NewDecoder(bytes.NewReader(content))
	var out []*yaml.Node
	for {
		var n yaml.Node
		if err := dec.Decode(&n); err != nil {
			return out
		}
		if len(n.Content) == 1 && n.Content[0].Kind == yaml.MappingNode {
			out = append(out, n.Content[0])
		}
	}
}

// ymap returns a mapping node's value for key.
func ymap(n *yaml.Node, keys ...string) *yaml.Node {
	for _, key := range keys {
		if n == nil || n.Kind != yaml.MappingNode {
			return nil
		}
		var next *yaml.Node
		for i := 0; i+1 < len(n.Content); i += 2 {
			if n.Content[i].Value == key {
				next = n.Content[i+1]
				break
			}
		}
		n = next
	}
	return n
}

func yscalar(n *yaml.Node) string {
	if n == nil || n.Kind != yaml.ScalarNode {
		return ""
	}
	return n.Value
}

// yitems is a sequence node's items (nil for anything else).
func yitems(n *yaml.Node) []*yaml.Node {
	if n == nil || n.Kind != yaml.SequenceNode {
		return nil
	}
	return n.Content
}

// ystrings is a sequence's scalar items.
func ystrings(n *yaml.Node) []*yaml.Node {
	var out []*yaml.Node
	for _, it := range yitems(n) {
		if it.Kind == yaml.ScalarNode && it.Value != "" {
			out = append(out, it)
		}
	}
	return out
}

// ylabels is a selector's matchLabels as a map and its canonical form
// ("k=v,k2=v2", keys sorted); ok is false when n is no map of scalars.
func ylabels(n *yaml.Node) (map[string]string, string, bool) {
	if n == nil || n.Kind != yaml.MappingNode || len(n.Content) == 0 {
		return nil, "", false
	}
	m := map[string]string{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i+1].Kind != yaml.ScalarNode {
			return nil, "", false
		}
		m[n.Content[i].Value] = n.Content[i+1].Value
	}
	return m, canonicalLabels(m), true
}

func canonicalLabels(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + "=" + m[k]
	}
	return strings.Join(parts, ",")
}

// k8sEntityName is the Kubernetes object naming every dialect and the
// incumbent share: Kind/<ns>/<name>, Kind/<name> with no namespace.
func k8sEntityName(kind, ns, name string) string {
	if ns != "" {
		return kind + "/" + ns + "/" + name
	}
	return kind + "/" + name
}

// policyTargetBlock is one match block's targets: the kinds (each with its
// Target), the namespaces scope and the label selector.
type policyTargetBlock struct {
	from   string
	kinds  []policyKindRef
	scope  string
	labels map[string]string
	canon  string
	line   int // the selector's line (a label-selector reference)
}

type policyKindRef struct {
	raw, target string
	line        int
}

// refs turns a block into policy_ref references: one resource-kind
// reference per kind (its selector narrowing it, its namespaces scoping
// it); a selector with no kind, or with the wildcard kind, is a
// label-selector reference instead; the wildcard kind alone stays a
// resource-kind reference whose Target "*" the resolver counts as a
// wildcard.
func (b policyTargetBlock) refs() []extractor.DocCrossFileRef {
	var out []extractor.DocCrossFileRef
	selector := false
	for _, k := range b.kinds {
		if k.target == "*" && b.labels != nil {
			selector = true
			continue
		}
		out = append(out, extractor.DocCrossFileRef{From: b.from, Edge: PolicyRefEdge, Strategy: PolicyResourceKind,
			Raw: k.raw, Target: k.target, Line: k.line, Scope: b.scope, Labels: b.labels})
	}
	if b.labels != nil && (selector || len(b.kinds) == 0) {
		out = append(out, extractor.DocCrossFileRef{From: b.from, Edge: PolicyRefEdge, Strategy: PolicyLabelSelector,
			Raw: b.canon, Target: b.canon, Line: b.line, Scope: b.scope, Labels: b.labels})
	}
	return out
}

// kyvernoNamespaced are the Kyverno kinds whose objects live in a namespace
// and govern only it.
var kyvernoNamespaced = map[string]bool{
	"Policy": true, "CleanupPolicy": true, "NamespacedValidatingPolicy": true, "NamespacedMutatingPolicy": true,
	"NamespacedGeneratingPolicy": true, "NamespacedDeletingPolicy": true, "NamespacedImageValidatingPolicy": true,
}

// kyvernoCEL are the CEL policy kinds (policies.kyverno.io): one match
// (spec.matchConstraints), no named rules, so the policy is the referrer.
var kyvernoCEL = map[string]bool{
	"ValidatingPolicy": true, "NamespacedValidatingPolicy": true, "MutatingPolicy": true, "NamespacedMutatingPolicy": true,
	"GeneratingPolicy": true, "NamespacedGeneratingPolicy": true, "DeletingPolicy": true, "NamespacedDeletingPolicy": true,
	"ImageValidatingPolicy": true, "NamespacedImageValidatingPolicy": true,
}

// kyvernoPolicyRefs reads a Kyverno file's policy targets: a ClusterPolicy's
// or Policy's rules' match and exclude blocks (any[], all[] and the legacy
// direct resources form) from the rule; a cleanup policy's and a
// PolicyException's own match from the policy entity; a CEL policy's
// matchConstraints resourceRules and excludeResourceRules (plural resources
// mapped to kinds) and its objectSelector from the policy entity. A
// namespaced kind's scope is its own namespace; otherwise a block's
// resources.namespaces.
func kyvernoPolicyRefs(content []byte) []extractor.DocCrossFileRef {
	var out []extractor.DocCrossFileRef
	for _, root := range yamlDocs(content) {
		kind := yscalar(ymap(root, "kind"))
		name := yscalar(ymap(root, "metadata", "name"))
		if kind == "" || name == "" {
			continue
		}
		ns := yscalar(ymap(root, "metadata", "namespace"))
		entity := k8sEntityName(kind, ns, name)
		scope := ""
		if kyvernoNamespaced[kind] {
			scope = ns
		}
		spec := ymap(root, "spec")
		switch {
		case kind == "ClusterPolicy" || kind == "Policy":
			for _, rule := range yitems(ymap(spec, "rules")) {
				rn := yscalar(ymap(rule, "name"))
				if rn == "" {
					continue
				}
				for _, key := range []string{"match", "exclude"} {
					out = append(out, kyvernoMatchRefs(ymap(rule, key), rn, scope)...)
				}
			}
		case kind == "ClusterCleanupPolicy" || kind == "CleanupPolicy" || kind == "PolicyException":
			for _, key := range []string{"match", "exclude"} {
				out = append(out, kyvernoMatchRefs(ymap(spec, key), entity, scope)...)
			}
		case kyvernoCEL[kind]:
			out = append(out, celMatchRefs(ymap(spec, "matchConstraints"), entity, scope)...)
		}
	}
	return out
}

// kyvernoMatchRefs reads one match or exclude block: its any[] and all[]
// filters' resources, and the legacy `resources` written directly under it.
func kyvernoMatchRefs(m *yaml.Node, from, scope string) []extractor.DocCrossFileRef {
	if m == nil {
		return nil
	}
	var blocks []*yaml.Node
	for _, key := range []string{"any", "all"} {
		for _, f := range yitems(ymap(m, key)) {
			if r := ymap(f, "resources"); r != nil {
				blocks = append(blocks, r)
			}
		}
	}
	if r := ymap(m, "resources"); r != nil {
		blocks = append(blocks, r)
	}
	var out []extractor.DocCrossFileRef
	for _, r := range blocks {
		b := policyTargetBlock{from: from, scope: scope}
		if b.scope == "" {
			var nss []string
			for _, n := range ystrings(ymap(r, "namespaces")) {
				nss = append(nss, n.Value)
			}
			b.scope = strings.Join(nss, ",")
		}
		if sel := ymap(r, "selector", "matchLabels"); sel != nil {
			if m, canon, ok := ylabels(sel); ok {
				b.labels, b.canon, b.line = m, canon, sel.Line
			}
		}
		for _, k := range ystrings(ymap(r, "kinds")) {
			b.kinds = append(b.kinds, policyKindRef{raw: k.Value, target: kyvernoKindTarget(k.Value), line: k.Line})
		}
		out = append(out, b.refs()...)
	}
	return out
}

// kyvernoKindTarget normalizes a Kyverno kind as written — Kind,
// Kind/subresource, Version/Kind, Group/Version/Kind[/subresource], a "*"
// group or version — to the resource-kind Target: Kind, or group/Kind when a
// group is named. "*" (any kind) stays "*"; a string with no Kind segment
// is "" (unmapped).
func kyvernoKindTarget(raw string) string {
	segs := strings.Split(strings.TrimSpace(raw), "/")
	at := -1
	for i, s := range segs {
		if s != "" && s[0] >= 'A' && s[0] <= 'Z' {
			at = i
			break
		}
	}
	if at < 0 {
		if segs[len(segs)-1] == "*" {
			return "*"
		}
		return ""
	}
	kind := segs[at]
	if at == 2 && segs[0] != "" && segs[0] != "*" {
		return segs[0] + "/" + kind
	}
	return kind
}

// celMatchRefs reads a CEL policy's matchConstraints: each resourceRules
// and excludeResourceRules entry's apiGroups × resources (a plural resource
// mapped to its kind through the built-in Kubernetes resources; a custom
// resource's plural is unmapped), narrowed by objectSelector.matchLabels.
func celMatchRefs(mc *yaml.Node, from, scope string) []extractor.DocCrossFileRef {
	if mc == nil {
		return nil
	}
	var labels map[string]string
	canon, line := "", 0
	if sel := ymap(mc, "objectSelector", "matchLabels"); sel != nil {
		if m, c, ok := ylabels(sel); ok {
			labels, canon, line = m, c, sel.Line
		}
	}
	var out []extractor.DocCrossFileRef
	for _, key := range []string{"resourceRules", "excludeResourceRules"} {
		for _, rr := range yitems(ymap(mc, key)) {
			b := policyTargetBlock{from: from, scope: scope, labels: labels, canon: canon, line: line}
			groups := ystrings(ymap(rr, "apiGroups"))
			for _, res := range ystrings(ymap(rr, "resources")) {
				if len(groups) == 0 {
					b.kinds = append(b.kinds, policyKindRef{raw: res.Value, target: celResourceTarget("*", res.Value), line: res.Line})
				}
				for _, g := range groups {
					b.kinds = append(b.kinds, policyKindRef{raw: res.Value, target: celResourceTarget(g.Value, res.Value), line: res.Line})
				}
			}
			out = append(out, b.refs()...)
		}
	}
	return dedupePolicyRefs(out)
}

// celResourceTarget maps a CEL resource rule's (apiGroup, plural resource)
// to the resource-kind Target: "*" for the wildcard resource, Kind for the
// core group or any group, group/Kind otherwise, "" when the plural is no
// built-in resource of that group.
func celResourceTarget(group, resource string) string {
	res, _, _ := strings.Cut(resource, "/") // pods/exec is a Pod's subresource
	if res == "*" {
		return "*"
	}
	if group == "*" {
		groups := make([]string, 0, len(builtinResources))
		for g := range builtinResources {
			groups = append(groups, g)
		}
		sort.Strings(groups)
		for _, g := range groups {
			if k, ok := builtinResources[g][res]; ok {
				return k
			}
		}
		return ""
	}
	k, ok := builtinResources[group][res]
	switch {
	case !ok:
		return ""
	case group == "":
		return k
	}
	return group + "/" + k
}

// builtinResources is the built-in Kubernetes API's plural resources by
// group (what discovery answers a cluster with no custom resources); a
// custom resource's plural is not in it.
var builtinResources = map[string]map[string]string{
	"": {
		"pods": "Pod", "services": "Service", "configmaps": "ConfigMap", "secrets": "Secret", "namespaces": "Namespace",
		"serviceaccounts": "ServiceAccount", "persistentvolumeclaims": "PersistentVolumeClaim",
		"persistentvolumes": "PersistentVolume", "nodes": "Node", "endpoints": "Endpoints", "events": "Event",
		"limitranges": "LimitRange", "resourcequotas": "ResourceQuota", "replicationcontrollers": "ReplicationController",
		"podtemplates": "PodTemplate", "bindings": "Binding",
	},
	"apps": {"deployments": "Deployment", "statefulsets": "StatefulSet", "daemonsets": "DaemonSet", "replicasets": "ReplicaSet",
		"controllerrevisions": "ControllerRevision"},
	"batch":                     {"jobs": "Job", "cronjobs": "CronJob"},
	"autoscaling":               {"horizontalpodautoscalers": "HorizontalPodAutoscaler"},
	"policy":                    {"poddisruptionbudgets": "PodDisruptionBudget", "podsecuritypolicies": "PodSecurityPolicy"},
	"networking.k8s.io":         {"ingresses": "Ingress", "networkpolicies": "NetworkPolicy", "ingressclasses": "IngressClass"},
	"rbac.authorization.k8s.io": {"roles": "Role", "rolebindings": "RoleBinding", "clusterroles": "ClusterRole", "clusterrolebindings": "ClusterRoleBinding"},
	"storage.k8s.io": {"storageclasses": "StorageClass", "csidrivers": "CSIDriver", "csinodes": "CSINode",
		"csistoragecapacities": "CSIStorageCapacity", "volumeattachments": "VolumeAttachment"},
	"scheduling.k8s.io":            {"priorityclasses": "PriorityClass"},
	"node.k8s.io":                  {"runtimeclasses": "RuntimeClass"},
	"coordination.k8s.io":          {"leases": "Lease"},
	"discovery.k8s.io":             {"endpointslices": "EndpointSlice"},
	"certificates.k8s.io":          {"certificatesigningrequests": "CertificateSigningRequest"},
	"apiextensions.k8s.io":         {"customresourcedefinitions": "CustomResourceDefinition"},
	"flowcontrol.apiserver.k8s.io": {"flowschemas": "FlowSchema", "prioritylevelconfigurations": "PriorityLevelConfiguration"},
	"apiregistration.k8s.io":       {"apiservices": "APIService"},
	"authentication.k8s.io":        {"tokenreviews": "TokenReview"},
	"authorization.k8s.io": {"subjectaccessreviews": "SubjectAccessReview", "selfsubjectaccessreviews": "SelfSubjectAccessReview",
		"localsubjectaccessreviews": "LocalSubjectAccessReview", "selfsubjectrulesreviews": "SelfSubjectRulesReview"},
	"events.k8s.io": {"events": "Event"},
	"extensions": {"deployments": "Deployment", "daemonsets": "DaemonSet", "replicasets": "ReplicaSet", "ingresses": "Ingress",
		"networkpolicies": "NetworkPolicy", "podsecuritypolicies": "PodSecurityPolicy"},
	"admissionregistration.k8s.io": {"validatingwebhookconfigurations": "ValidatingWebhookConfiguration",
		"mutatingwebhookconfigurations": "MutatingWebhookConfiguration", "validatingadmissionpolicies": "ValidatingAdmissionPolicy",
		"validatingadmissionpolicybindings": "ValidatingAdmissionPolicyBinding"},
}

// gatekeeperPolicyRefs reads a Gatekeeper file's policy targets: each
// constraint's spec.match — kinds[] as apiGroups × kinds (a "*" or absent
// group is any group, "" the core group), labelSelector.matchLabels
// narrowing them, namespaces scoping them — from the constraint entity
// (Kind/name). A constraint with no kinds and no selector matches every
// object: one resource-kind reference "*" (counted as a wildcard).
func gatekeeperPolicyRefs(content []byte) []extractor.DocCrossFileRef {
	var out []extractor.DocCrossFileRef
	for _, root := range yamlDocs(content) {
		av := yscalar(ymap(root, "apiVersion"))
		if !strings.HasPrefix(av, "constraints.gatekeeper.sh/") {
			continue
		}
		kind, name := yscalar(ymap(root, "kind")), yscalar(ymap(root, "metadata", "name"))
		if kind == "" || name == "" {
			continue
		}
		b := policyTargetBlock{from: kind + "/" + name}
		match := ymap(root, "spec", "match")
		var nss []string
		for _, n := range ystrings(ymap(match, "namespaces")) {
			nss = append(nss, n.Value)
		}
		b.scope = strings.Join(nss, ",")
		if sel := ymap(match, "labelSelector", "matchLabels"); sel != nil {
			if m, canon, ok := ylabels(sel); ok {
				b.labels, b.canon, b.line = m, canon, sel.Line
			}
		}
		for _, ks := range yitems(ymap(match, "kinds")) {
			groups := ystrings(ymap(ks, "apiGroups"))
			for _, k := range ystrings(ymap(ks, "kinds")) {
				if len(groups) == 0 {
					b.kinds = append(b.kinds, policyKindRef{raw: k.Value, target: gatekeeperKindTarget("*", k.Value), line: k.Line})
				}
				for _, g := range groups {
					b.kinds = append(b.kinds, policyKindRef{raw: k.Value, target: gatekeeperKindTarget(g.Value, k.Value), line: k.Line})
				}
			}
		}
		if len(b.kinds) == 0 && b.labels == nil {
			line := root.Line
			if match != nil {
				line = match.Line
			}
			b.kinds = []policyKindRef{{raw: "*", target: "*", line: line}}
		}
		out = append(out, dedupePolicyRefs(b.refs())...)
	}
	return out
}

func gatekeeperKindTarget(group, kind string) string {
	switch {
	case kind == "*":
		return "*"
	case group == "" || group == "*":
		return kind
	}
	return group + "/" + kind
}

// dedupePolicyRefs drops a repeated reference (a kind written under two
// groups that both normalize to it, a selector shared by several blocks).
func dedupePolicyRefs(refs []extractor.DocCrossFileRef) []extractor.DocCrossFileRef {
	seen := map[string]bool{}
	out := refs[:0]
	for _, r := range refs {
		id := r.From + "\x00" + r.Strategy + "\x00" + r.Raw + "\x00" + r.Target + "\x00" + r.Scope + "\x00" + canonicalLabels(r.Labels)
		if !seen[id] {
			seen[id] = true
			out = append(out, r)
		}
	}
	return out
}

// policyEntityNodes is the nodes a policy file's references name (the
// referring rule, policy, exception or constraint entities).
func policyEntityNodes(nodes []core.SymbolNode, refs []extractor.DocCrossFileRef) []core.SymbolNode {
	from := map[string]bool{}
	for _, c := range refs {
		from[c.From] = true
	}
	var out []core.SymbolNode
	for _, n := range nodes {
		if from[n.Name] {
			out = append(out, n)
		}
	}
	return out
}
