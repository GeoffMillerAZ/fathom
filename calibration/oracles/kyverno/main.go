// Command kyverno-oracle is the language-breadth harness's oracle for the
// kyverno doc-engine dialect (LB-Q-kyverno): an independent gopkg.in/yaml.v3
// walk of a subject's Kyverno files — never fathom's doc engine — written in
// indexbench lang-policies' tool-oracle shape: definitions (policies of every
// kind as Kind/[ns/]name, legacy rules by name, PolicyExceptions, the Kyverno
// CLI's Test manifests), references (an exception's policyName, ruleNames and
// policyRefs; a Test's results[].policy and results[].rule), cross_references
// (a Test's policies[] and exceptions[] files to the file's first entity, an
// exception to the one ClusterPolicy/<name> or Policy/<ns>/<name> another file
// declares) and policy_references/policy_unresolved (targets.go: the match and
// exclude kinds, CEL resourceRules and selectors joined to the subject's
// Kubernetes objects).
//
// Tier C. The kyverno CLI lists no policy's rules offline (`kyverno apply`
// reports only rules a resource matched, `kyverno test` only the results a
// Test declares), so the walk is the oracle and, with -kyverno, `kyverno test`
// is the cross-check: every result row a Test's run evaluated (expected pass,
// fail or warn: the rule applied) names a (rule or policy, resource) pair the
// walk's policy_references must hold; autogen rules (Pod rules Kyverno
// generates for controllers) are counted apart. Bench-only; never a product
// dependency.
//
//	go run . -subject <checkout> [-kyverno <bin>] > <out.json>
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type def struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Line int    `json:"line"`
	Kind string `json:"kind"`
}

type ref struct {
	Path string `json:"path"`
	From string `json:"from"`
	To   string `json:"to"`
}

type xref struct {
	Path   string `json:"path"`
	From   string `json:"from"`
	ToPath string `json:"to_path"`
	To     string `json:"to"`
}

type out struct {
	Tool             string             `json:"tool"`
	Version          string             `json:"version"`
	Tier             string             `json:"tier"`
	Definitions      []def              `json:"definitions"`
	References       []ref              `json:"references"`
	CrossReferences  []xref             `json:"cross_references"`
	PolicyReferences []policyRef        `json:"policy_references"`
	PolicyUnresolved []policyUnresolved `json:"policy_unresolved"`
	PolicyCrossCheck *crossCheck        `json:"policy_crosscheck,omitempty"`
}

var policyKinds = map[string]bool{
	"ClusterPolicy": true, "Policy": true, "ClusterCleanupPolicy": true, "CleanupPolicy": true,
	"ValidatingPolicy": true, "NamespacedValidatingPolicy": true, "MutatingPolicy": true, "NamespacedMutatingPolicy": true,
	"GeneratingPolicy": true, "NamespacedGeneratingPolicy": true, "DeletingPolicy": true, "NamespacedDeletingPolicy": true,
	"ImageValidatingPolicy": true, "NamespacedImageValidatingPolicy": true,
}

var namespacedKinds = map[string]bool{
	"Policy": true, "CleanupPolicy": true, "NamespacedValidatingPolicy": true, "NamespacedMutatingPolicy": true,
	"NamespacedGeneratingPolicy": true, "NamespacedDeletingPolicy": true, "NamespacedImageValidatingPolicy": true,
}

// kyvernoDoc reports whether a document belongs to the dialect: an API group
// of Kyverno's policies, or the Kyverno CLI's Test.
func kyvernoDoc(root *yaml.Node) bool {
	av, kind := scalar(get(root, "apiVersion")), scalar(get(root, "kind"))
	g, _, ok := strings.Cut(av, "/")
	if !ok {
		return false
	}
	switch g {
	case "kyverno.io", "policies.kyverno.io":
		return true
	case "cli.kyverno.io":
		return kind == "Test"
	}
	return false
}

var apiVersionLine = regexp.MustCompile(`^apiVersion\s*:(.*)$`)

// claimed: every document that is an object belongs to the dialect, and at
// least one does.
func claimed(f yamlFile) bool {
	ext := strings.ToLower(path.Ext(f.rel))
	if ext != ".yaml" && ext != ".yml" || len(f.docs) == 0 {
		return false
	}
	for _, d := range f.docs {
		if d.Kind != yaml.MappingNode || !kyvernoDoc(d) {
			return false
		}
	}
	// a document that failed to decode leaves the rest unknown: the
	// raw text's apiVersion lines must all be Kyverno's
	for _, line := range strings.Split(string(f.raw), "\n") {
		if m := apiVersionLine.FindStringSubmatch(line); m != nil {
			g, _, _ := strings.Cut(strings.Trim(strings.TrimSpace(m[1]), `"'`), "/")
			if g != "kyverno.io" && g != "policies.kyverno.io" && g != "cli.kyverno.io" {
				return false
			}
		}
	}
	return true
}

// kindTarget reads a Kyverno kind as written (Kind, Kind/subresource,
// Version/Kind, Group/Version/Kind) into its target: Kind, or group/Kind.
func kindTarget(raw string) (string, bool) {
	segs := strings.Split(raw, "/")
	for i, s := range segs {
		if s != "" && s[0] >= 'A' && s[0] <= 'Z' {
			if i == 2 && segs[0] != "*" && segs[0] != "" {
				return segs[0] + "/" + s, true
			}
			return s, true
		}
	}
	if segs[len(segs)-1] == "*" {
		return "*", true
	}
	return "", false
}

// plurals: the built-in API's plural resources by group.
var plurals = map[string]map[string]string{
	"": {"pods": "Pod", "services": "Service", "configmaps": "ConfigMap", "secrets": "Secret", "namespaces": "Namespace",
		"serviceaccounts": "ServiceAccount", "persistentvolumeclaims": "PersistentVolumeClaim", "persistentvolumes": "PersistentVolume",
		"nodes": "Node", "endpoints": "Endpoints", "events": "Event", "limitranges": "LimitRange", "resourcequotas": "ResourceQuota",
		"replicationcontrollers": "ReplicationController", "podtemplates": "PodTemplate", "bindings": "Binding"},
	"apps":                         {"deployments": "Deployment", "statefulsets": "StatefulSet", "daemonsets": "DaemonSet", "replicasets": "ReplicaSet", "controllerrevisions": "ControllerRevision"},
	"batch":                        {"jobs": "Job", "cronjobs": "CronJob"},
	"autoscaling":                  {"horizontalpodautoscalers": "HorizontalPodAutoscaler"},
	"policy":                       {"poddisruptionbudgets": "PodDisruptionBudget", "podsecuritypolicies": "PodSecurityPolicy"},
	"networking.k8s.io":            {"ingresses": "Ingress", "networkpolicies": "NetworkPolicy", "ingressclasses": "IngressClass"},
	"rbac.authorization.k8s.io":    {"roles": "Role", "rolebindings": "RoleBinding", "clusterroles": "ClusterRole", "clusterrolebindings": "ClusterRoleBinding"},
	"storage.k8s.io":               {"storageclasses": "StorageClass", "csidrivers": "CSIDriver", "csinodes": "CSINode", "csistoragecapacities": "CSIStorageCapacity", "volumeattachments": "VolumeAttachment"},
	"scheduling.k8s.io":            {"priorityclasses": "PriorityClass"},
	"node.k8s.io":                  {"runtimeclasses": "RuntimeClass"},
	"coordination.k8s.io":          {"leases": "Lease"},
	"discovery.k8s.io":             {"endpointslices": "EndpointSlice"},
	"certificates.k8s.io":          {"certificatesigningrequests": "CertificateSigningRequest"},
	"apiextensions.k8s.io":         {"customresourcedefinitions": "CustomResourceDefinition"},
	"flowcontrol.apiserver.k8s.io": {"flowschemas": "FlowSchema", "prioritylevelconfigurations": "PriorityLevelConfiguration"},
	"apiregistration.k8s.io":       {"apiservices": "APIService"},
	"authentication.k8s.io":        {"tokenreviews": "TokenReview"},
	"authorization.k8s.io":         {"subjectaccessreviews": "SubjectAccessReview", "selfsubjectaccessreviews": "SelfSubjectAccessReview", "localsubjectaccessreviews": "LocalSubjectAccessReview", "selfsubjectrulesreviews": "SelfSubjectRulesReview"},
	"events.k8s.io":                {"events": "Event"},
	"extensions":                   {"deployments": "Deployment", "daemonsets": "DaemonSet", "replicasets": "ReplicaSet", "ingresses": "Ingress", "networkpolicies": "NetworkPolicy", "podsecuritypolicies": "PodSecurityPolicy"},
	"admissionregistration.k8s.io": {"validatingwebhookconfigurations": "ValidatingWebhookConfiguration", "mutatingwebhookconfigurations": "MutatingWebhookConfiguration", "validatingadmissionpolicies": "ValidatingAdmissionPolicy", "validatingadmissionpolicybindings": "ValidatingAdmissionPolicyBinding"},
}

func pluralTarget(group, res string) (string, bool) {
	res, _, _ = strings.Cut(res, "/")
	if res == "*" {
		return "*", true
	}
	if group == "*" {
		var gs []string
		for g := range plurals {
			gs = append(gs, g)
		}
		sort.Strings(gs)
		for _, g := range gs {
			if k, ok := plurals[g][res]; ok {
				return k, true
			}
		}
		return "", false
	}
	k, ok := plurals[group][res]
	if !ok {
		return "", false
	}
	if group == "" {
		return k, true
	}
	return group + "/" + k, true
}

// blockTargets reads one resources block: a target per kind (narrowed by
// the block's selector), or the selector alone (also for the "*" kind).
func blockTargets(file, from string, r *yaml.Node, scope []string) []target {
	if len(scope) == 0 {
		for _, n := range scalars(get(r, "namespaces")) {
			scope = append(scope, n.Value)
		}
	}
	sel := get(r, "selector", "matchLabels")
	labels := labelMap(sel)
	var out []target
	star := false
	for _, k := range scalars(get(r, "kinds")) {
		kt, ok := kindTarget(k.Value)
		if kt == "*" && labels != nil {
			star = true
			continue
		}
		out = append(out, target{path: file, from: from, raw: k.Value, kind: kt, unmapped: !ok, scope: scope, labels: labels, line: k.Line})
	}
	if labels != nil && (star || len(scalars(get(r, "kinds"))) == 0) {
		out = append(out, target{path: file, from: from, raw: canon(labels), labels: labels, scope: scope, line: sel.Line})
	}
	return out
}

func matchTargets(file, from string, m *yaml.Node, scope []string) []target {
	var out []target
	for _, key := range []string{"any", "all"} {
		for _, f := range items(get(m, key)) {
			if r := get(f, "resources"); r != nil {
				out = append(out, blockTargets(file, from, r, scope)...)
			}
		}
	}
	if r := get(m, "resources"); r != nil {
		out = append(out, blockTargets(file, from, r, scope)...)
	}
	return out
}

func celTargets(file, from string, mc *yaml.Node, scope []string) []target {
	sel := get(mc, "objectSelector", "matchLabels")
	labels := labelMap(sel)
	var out []target
	for _, key := range []string{"resourceRules", "excludeResourceRules"} {
		for _, rr := range items(get(mc, key)) {
			groups := scalars(get(rr, "apiGroups"))
			gs := []string{}
			for _, g := range groups {
				gs = append(gs, g.Value)
			}
			if len(gs) == 0 {
				gs = []string{"*"}
			}
			for _, res := range scalars(get(rr, "resources")) {
				for _, g := range gs {
					kt, ok := pluralTarget(g, res.Value)
					if kt == "*" && labels != nil {
						out = append(out, target{path: file, from: from, raw: canon(labels), labels: labels, scope: scope, line: sel.Line})
						continue
					}
					out = append(out, target{path: file, from: from, raw: res.Value, kind: kt, unmapped: !ok, scope: scope, labels: labels, line: res.Line})
				}
			}
		}
	}
	return out
}

// entity is one claimed file's first entity, and every policy entity.
type policyDecl struct{ path, name string }

func main() {
	subject := flag.String("subject", "", "subject checkout")
	kyverno := flag.String("kyverno", "", "kyverno CLI for the `kyverno test` cross-check (optional)")
	flag.Parse()
	if *subject == "" {
		fmt.Fprintln(os.Stderr, "usage: kyverno-oracle -subject <dir> [-kyverno <bin>]")
		os.Exit(2)
	}
	files, err := walkYAML(*subject)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kyverno-oracle:", err)
		os.Exit(1)
	}
	o := out{Tool: "kyverno-oracle (independent gopkg.in/yaml.v3 walk)", Version: "gopkg.in/yaml.v3 v3.0.1", Tier: "C",
		Definitions: []def{}, References: []ref{}, CrossReferences: []xref{}, PolicyReferences: []policyRef{}, PolicyUnresolved: []policyUnresolved{}}
	charts := chartDirsOf(files)
	var objs []object
	var targets []target
	root := map[string]string{}         // claimed file -> its first entity
	byName := map[string][]policyDecl{} // policy entity name -> declarations
	type pending struct{ path, from, pn string }
	var exceptions []pending
	type testFile struct {
		path, entity string
		node         *yaml.Node
	}
	var tests []testFile
	refSeen := map[ref]bool{}
	addRef := func(r ref) {
		if r.To != "" && !refSeen[r] {
			refSeen[r] = true
			o.References = append(o.References, r)
		}
	}
	for _, f := range files {
		if !claimed(f) {
			objs = append(objs, objectsOf(f, charts)...)
			continue
		}
		for _, d := range f.docs {
			kind, name := scalar(get(d, "kind")), scalar(get(d, "metadata", "name"))
			if kind == "" || name == "" {
				continue
			}
			ns := scalar(get(d, "metadata", "namespace"))
			ent := qualify(kind, ns, name)
			var scope []string
			if namespacedKinds[kind] && ns != "" {
				scope = []string{ns}
			}
			spec := get(d, "spec")
			switch {
			case policyKinds[kind]:
				o.Definitions = append(o.Definitions, def{f.rel, ent, d.Line, "policy"})
				byName[ent] = append(byName[ent], policyDecl{f.rel, ent})
				if root[f.rel] == "" {
					root[f.rel] = ent
				}
				switch {
				case kind == "ClusterPolicy" || kind == "Policy":
					for _, r := range items(get(spec, "rules")) {
						rn := scalar(get(r, "name"))
						if rn == "" || r.Kind != yaml.MappingNode {
							continue
						}
						o.Definitions = append(o.Definitions, def{f.rel, rn, r.Line, "rule"})
						for _, key := range []string{"match", "exclude"} {
							targets = append(targets, matchTargets(f.rel, rn, get(r, key), scope)...)
						}
					}
				case kind == "ClusterCleanupPolicy" || kind == "CleanupPolicy":
					for _, key := range []string{"match", "exclude"} {
						targets = append(targets, matchTargets(f.rel, ent, get(spec, key), scope)...)
					}
				default:
					targets = append(targets, celTargets(f.rel, ent, get(spec, "matchConstraints"), scope)...)
				}
			case kind == "PolicyException":
				o.Definitions = append(o.Definitions, def{f.rel, ent, d.Line, "policy"})
				if root[f.rel] == "" {
					root[f.rel] = ent
				}
				for _, ex := range items(get(spec, "exceptions")) {
					pn := scalar(get(ex, "policyName"))
					addRef(ref{f.rel, ent, pn})
					for _, rn := range scalars(get(ex, "ruleNames")) {
						addRef(ref{f.rel, ent, rn.Value})
					}
					if pn != "" {
						exceptions = append(exceptions, pending{f.rel, ent, pn})
					}
				}
				for _, pr := range items(get(spec, "policyRefs")) {
					addRef(ref{f.rel, ent, scalar(get(pr, "name"))})
				}
				for _, key := range []string{"match", "exclude"} {
					targets = append(targets, matchTargets(f.rel, ent, get(spec, key), scope)...)
				}
			case kind == "Test":
				ent = qualify(kind, "", name)
				o.Definitions = append(o.Definitions, def{f.rel, ent, d.Line, "test"})
				if root[f.rel] == "" {
					root[f.rel] = ent
				}
				for _, r := range items(get(d, "results")) {
					addRef(ref{f.rel, ent, scalar(get(r, "policy"))})
					addRef(ref{f.rel, ent, scalar(get(r, "rule"))})
				}
				tests = append(tests, testFile{f.rel, ent, d})
			}
		}
	}
	xseen := map[xref]bool{}
	addX := func(x xref) {
		if x.To != "" && !xseen[x] && !(x.Path == x.ToPath && x.From == x.To) {
			xseen[x] = true
			o.CrossReferences = append(o.CrossReferences, x)
		}
	}
	for _, t := range tests {
		for _, key := range []string{"policies", "exceptions"} {
			for _, p := range scalars(get(t.node, key)) {
				target := path.Clean(path.Join(path.Dir(t.path), p.Value))
				if e := root[target]; e != "" {
					addX(xref{t.path, t.entity, target, e})
					continue
				}
				// a directory: the one claimed file directly in it
				var in []string
				for f := range root {
					if path.Dir(f) == target {
						in = append(in, f)
					}
				}
				if len(in) == 1 {
					addX(xref{t.path, t.entity, in[0], root[in[0]]})
				}
			}
		}
	}
	for _, e := range exceptions {
		for _, n := range []string{"ClusterPolicy/" + e.pn, "Policy/" + e.pn} {
			var other []policyDecl
			for _, d := range byName[n] {
				if d.path != e.path {
					other = append(other, d)
				}
			}
			if len(other) == 1 {
				addX(xref{e.path, e.from, other[0].path, other[0].name})
			}
		}
	}
	o.PolicyReferences, o.PolicyUnresolved = resolveTargets(targets, objs)
	if *kyverno != "" {
		var testPaths []string
		for _, t := range tests {
			testPaths = append(testPaths, t.path)
		}
		podTarget := map[[2]string]bool{}
		for _, t := range targets {
			if t.kind == "Pod" {
				podTarget[[2]string{t.path, t.from}] = true
			}
		}
		o.PolicyCrossCheck = crossCheckTests(*kyverno, *subject, testPaths, files, o.PolicyReferences, podTarget)
		fmt.Fprintf(os.Stderr, "kyverno-oracle: %s: %d of %d evaluated pairs agree (%s)\n", o.PolicyCrossCheck.Tool, o.PolicyCrossCheck.Agree, o.PolicyCrossCheck.Checked, o.PolicyCrossCheck.Note)
	}
	sort.Slice(o.CrossReferences, func(i, j int) bool {
		a, b := o.CrossReferences[i], o.CrossReferences[j]
		return a.Path+"\x00"+a.From+"\x00"+a.ToPath+"\x00"+a.To < b.Path+"\x00"+b.From+"\x00"+b.ToPath+"\x00"+b.To
	})
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	if err := enc.Encode(o); err != nil {
		os.Exit(1)
	}
}

// podControllers are the kinds Kyverno's autogen applies a Pod policy to.
var podControllers = map[string]bool{"Deployment": true, "StatefulSet": true, "DaemonSet": true, "Job": true, "CronJob": true, "ReplicaSet": true, "ReplicationController": true}

// testRow is one result row of `kyverno test -o json`.
type testRow struct {
	Policy   string `json:"POLICY"`
	Rule     string `json:"RULE"`
	Resource string `json:"RESOURCE"`
	Result   string `json:"RESULT"`
}

// crossCheckTests runs `kyverno test` on each Test file and checks, for
// every row the run evaluated with an expected pass, fail or warn (the rule
// applied to the resource) and passed, that the walk joins the rule (or the
// CEL policy) to that resource.
func crossCheckTests(bin, subject string, tests []string, files []yamlFile, refs []policyRef, podTarget map[[2]string]bool) *crossCheck {
	ver, _ := exec.Command(bin, "version").Output()
	version := strings.TrimSpace(strings.TrimPrefix(strings.SplitN(string(ver), "\n", 2)[0], "Version:"))
	cc := &crossCheck{Tool: "kyverno test " + version, Tier: "A", Disagree: []string{}}
	have := map[[4]string]bool{}
	for _, r := range refs {
		have[[4]string{r.Path, r.From, r.ToPath, r.To}] = true
	}
	byRel := map[string]yamlFile{}
	for _, f := range files {
		byRel[f.rel] = f
	}
	autogen, failedRuns, unplaced := 0, 0, 0
	for _, tp := range tests {
		tf := byRel[tp]
		var test *yaml.Node
		for _, d := range tf.docs {
			if scalar(get(d, "kind")) == "Test" {
				test = d
			}
		}
		if test == nil {
			continue
		}
		dir := path.Dir(tp)
		expected := map[[3]string]string{} // policy, rule, resource name -> result
		for _, r := range items(get(test, "results")) {
			names := scalars(get(r, "resources"))
			if n := scalar(get(r, "resource")); n != "" {
				names = append(names, &yaml.Node{Value: n})
			}
			for _, n := range names {
				expected[[3]string{scalar(get(r, "policy")), scalar(get(r, "rule")), n.Value}] = strings.ToLower(scalar(get(r, "result")))
			}
		}
		var policyFiles, resourceFiles []string
		for _, p := range scalars(get(test, "policies")) {
			policyFiles = append(policyFiles, path.Clean(path.Join(dir, p.Value)))
		}
		for _, p := range scalars(get(test, "resources")) {
			resourceFiles = append(resourceFiles, path.Clean(path.Join(dir, p.Value)))
		}
		// A run that prints no result table is retried: on the external
		// volume a read can fail transiently ("operation not permitted").
		var b []byte
		start, end := -1, -1
		for attempt := 0; attempt < 5 && (start < 0 || end < start); attempt++ {
			cmd := exec.Command(bin, "test", filepath.Join(subject, filepath.FromSlash(dir)), "-f", path.Base(tp), "-o", "json", "--remove-color")
			b, _ = cmd.CombinedOutput()
			start, end = bytes.Index(b, []byte("\n[")), bytes.LastIndex(b, []byte("]"))
		}
		if start < 0 || end < start {
			failedRuns++
			fmt.Fprintf(os.Stderr, "kyverno-oracle: %s: no result table: %.300s\n", tp, b)
			continue
		}
		var rows []testRow
		if err := json.Unmarshal(b[start+1:end+1], &rows); err != nil {
			failedRuns++
			continue
		}
		for _, row := range rows {
			if strings.HasPrefix(row.Rule, "autogen-") {
				autogen++
				continue
			}
			segs := strings.Split(row.Resource, "/")
			if len(segs) < 3 || row.Result != "Pass" {
				continue
			}
			name, ns, kind := segs[len(segs)-1], segs[len(segs)-2], segs[len(segs)-3]
			if len(segs) == 3 { // a cluster-scoped object: apiVersion/Kind/name
				ns, kind = "", segs[1]
			}
			want := expected[[3]string{row.Policy, row.Rule, name}]
			if want == "" {
				want = expected[[3]string{row.Policy, row.Rule, ns + "/" + name}]
			}
			if want != "pass" && want != "fail" && want != "warn" {
				continue
			}
			from := row.Rule
			polFile := ""
			for _, pf := range policyFiles {
				for _, d := range byRel[pf].docs {
					n := scalar(get(d, "metadata", "name"))
					if n == row.Policy || scalar(get(d, "metadata", "namespace"))+"/"+n == row.Policy {
						polFile = pf
						if from == "" {
							from = qualify(scalar(get(d, "kind")), scalar(get(d, "metadata", "namespace")), n)
						}
					}
				}
			}
			objFile, objName := "", ""
			for _, rf := range resourceFiles {
				for _, d := range byRel[rf].docs {
					if scalar(get(d, "kind")) != kind || scalar(get(d, "metadata", "name")) != name {
						continue
					}
					dns := scalar(get(d, "metadata", "namespace"))
					if dns != "" && dns != ns {
						continue
					}
					objFile, objName = rf, qualify(kind, dns, name)
				}
			}
			if polFile == "" || objFile == "" {
				unplaced++
				continue
			}
			if !have[[4]string{polFile, from, objFile, objName}] && podControllers[kind] && podTarget[[2]string{polFile, from}] {
				autogen++ // a Pod policy Kyverno applied to a Pod controller (autogen)
				continue
			}
			cc.Checked++
			if have[[4]string{polFile, from, objFile, objName}] {
				cc.Agree++
			} else {
				cc.Disagree = append(cc.Disagree, fmt.Sprintf("%s#%s -> %s#%s", polFile, from, objFile, objName))
			}
		}
	}
	sort.Strings(cc.Disagree)
	cc.Note = fmt.Sprintf("%d Test files run; %d autogen rows (a Pod policy or rule Kyverno applies to Pod controllers: an autogen- rule, or a controller row of a policy whose targets are Pods) not checked; %d rows whose policy or resource no listed file declares; %d runs gave no result table", len(tests), autogen, unplaced, failedRuns)
	return cc
}
