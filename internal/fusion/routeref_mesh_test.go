package fusion

import (
	"context"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/impact"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestLabelSelector pins the side pass's label selector: matchLabels and the
// four matchExpressions operators, its kubectl string form and back, and
// the selector of everything.
func TestLabelSelector(t *testing.T) {
	s := labelSelectorOf(map[string]string{"app": "reviews", "team": "web"}, []labelReq{
		{key: "tier", op: "in", vals: []string{"a", "b"}},
		{key: "canary", op: "!"},
		{key: "zone", op: "notin", vals: []string{"x"}},
		{key: "owner", op: "exists"},
	})
	want := "app=reviews,!canary,owner,team=web,tier in (a,b),zone notin (x)"
	if got := s.String(); got != want {
		t.Fatalf("String = %q, want %q", got, want)
	}
	back, ok := parseLabelSelector(want)
	if !ok || back.String() != want {
		t.Fatalf("parseLabelSelector(%q) = %q, %v", want, back.String(), ok)
	}
	for _, c := range []struct {
		labels map[string]string
		want   bool
	}{
		{map[string]string{"app": "reviews", "team": "web", "tier": "a", "owner": "x"}, true},
		{map[string]string{"app": "reviews", "team": "web", "tier": "c", "owner": "x"}, false},
		{map[string]string{"app": "reviews", "team": "web", "tier": "a", "owner": "x", "canary": "1"}, false},
		{map[string]string{"app": "reviews", "team": "web", "tier": "a", "owner": "x", "zone": "x"}, false},
		{map[string]string{"app": "reviews", "team": "web", "tier": "a"}, false},
	} {
		if got := back.matches(c.labels); got != c.want {
			t.Errorf("matches(%v) = %v, want %v", c.labels, got, c.want)
		}
	}
	all, ok := parseLabelSelector("{}")
	if !ok || !all.matches(map[string]string{"any": "thing"}) || all.String() != "{}" {
		t.Errorf("the empty selector must select everything and print {}")
	}
	if _, ok := parseLabelSelector(""); ok {
		t.Errorf("an empty string is no selector")
	}
}

// TestGatewayAndPrincipalObjects pins Istio's naming: a VirtualService
// gateway `<ns>/<name>`, the legacy `<name>.<ns>.svc.cluster.local`, or a
// short name in the rule's namespace; a principal
// `<trust domain>/ns/<ns>/sa/<name>`; wildcards and other forms name none.
func TestGatewayAndPrincipalObjects(t *testing.T) {
	for v, want := range map[string]string{
		"shop-gw":                          "Gateway/shop/shop-gw",
		"edge/public-gw":                   "Gateway/edge/public-gw",
		"public-gw.edge.svc.cluster.local": "Gateway/edge/public-gw",
		"edge/*":                           "",
		"/x":                               "",
	} {
		if got := gatewayObject(v, "shop"); got != want {
			t.Errorf("gatewayObject(%q) = %q, want %q", v, got, want)
		}
	}
	for p, want := range map[string]string{
		"cluster.local/ns/shop/sa/reviews":         "ServiceAccount/shop/reviews",
		"spiffe://example.org/ns/default/sa/sleep": "ServiceAccount/default/sleep",
		"cluster.local/ns/shop/sa/*":               "",
		"*":                                        "",
		"testing@example.org/testing@example.org":  "",
	} {
		if got := principalObject(p); got != want {
			t.Errorf("principalObject(%q) = %q, want %q", p, got, want)
		}
	}
}

// meshRepo is the side pass's fixture: Istio routing, security and egress
// objects in front of two workloads and their Service, a Gateway in another
// directory, and prometheus-operator monitors selecting them.
func meshRepo() map[string]string {
	return map[string]string{
		"deploy/mesh/workloads.yaml": `apiVersion: v1
kind: ServiceAccount
metadata:
  name: reviews
  namespace: shop
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: reviews-v1
  namespace: shop
spec:
  selector:
    matchLabels: {app: reviews, version: v1}
  template:
    metadata:
      labels: {app: reviews, version: v1}
    spec:
      serviceAccountName: reviews
      containers:
        - name: reviews
          image: acme/reviews:1
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: reviews-v2
  namespace: shop
spec:
  template:
    metadata:
      labels: {app: reviews, version: v2}
    spec:
      containers:
        - name: reviews
          image: acme/reviews:2
---
apiVersion: v1
kind: Service
metadata:
  name: reviews
  namespace: shop
  labels: {app: reviews, team: storefront}
spec:
  selector: {app: reviews}
---
apiVersion: v1
kind: Service
metadata:
  name: ratings
  namespace: shop
  labels: {app: ratings}
spec:
  selector: {app: ratings}
`,
		"deploy/mesh/istio.yaml": `apiVersion: networking.istio.io/v1
kind: Gateway
metadata:
  name: shop-gw
  namespace: shop
spec:
  selector: {istio: ingressgateway}
  servers:
    - port: {number: 80, name: http, protocol: HTTP}
      hosts: ["*"]
---
apiVersion: networking.istio.io/v1
kind: VirtualService
metadata:
  name: reviews
  namespace: shop
spec:
  hosts: [reviews]
  gateways: [shop-gw, mesh, edge/public-gw, missing-gw]
  http:
    - route:
        - destination: {host: reviews, subset: v1}
          weight: 80
        - destination: {host: reviews.shop.svc.cluster.local, subset: v2}
          weight: 20
        - destination: {host: reviews, subset: v9}
---
apiVersion: networking.istio.io/v1
kind: DestinationRule
metadata:
  name: reviews
  namespace: shop
spec:
  host: reviews
  subsets:
    - name: v1
      labels: {version: v1}
    - name: v2
      labels: {version: v2}
---
apiVersion: networking.istio.io/v1
kind: ServiceEntry
metadata:
  name: payments-api
  namespace: shop
spec:
  hosts: [api.payments.example.com]
  location: MESH_EXTERNAL
  resolution: DNS
  ports: [{number: 443, name: https, protocol: TLS}]
---
apiVersion: networking.istio.io/v1
kind: VirtualService
metadata:
  name: payments
  namespace: shop
spec:
  hosts: [api.payments.example.com]
  tls:
    - match: [{port: 443, sniHosts: [api.payments.example.com]}]
      route:
        - destination: {host: api.payments.example.com}
---
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: reviews
  namespace: shop
spec:
  selector:
    matchLabels: {app: reviews}
  rules:
    - from:
        - source:
            principals: ["cluster.local/ns/shop/sa/reviews", "cluster.local/ns/shop/sa/ghost", "*"]
`,
		"deploy/edge/gateway.yaml": `apiVersion: networking.istio.io/v1
kind: Gateway
metadata:
  name: public-gw
  namespace: edge
spec:
  selector: {istio: ingressgateway}
  servers:
    - port: {number: 443, name: https, protocol: HTTPS}
      hosts: ["shop.example.com"]
`,
		"deploy/monitoring/monitors.yaml": `apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: reviews
  namespace: monitoring
spec:
  selector:
    matchLabels: {team: storefront}
    matchExpressions:
      - {key: app, operator: In, values: [reviews, ratings]}
  namespaceSelector:
    matchNames: [shop]
  endpoints: [{port: http}]
---
apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
metadata:
  name: reviews-pods
  namespace: monitoring
spec:
  selector:
    matchLabels: {app: reviews}
  namespaceSelector:
    any: true
  podMetricsEndpoints: [{port: http}]
---
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: nothing
  namespace: monitoring
spec:
  selector:
    matchLabels: {app: nobody}
---
apiVersion: monitoring.coreos.com/v1
kind: Probe
metadata:
  name: storefront
  namespace: monitoring
spec:
  prober:
    url: blackbox.monitoring.svc:19115
  targets:
    staticConfig:
      static: ["http://reviews.shop.svc:9080/health", "https://shop.example.com"]
`,
		"deploy/monitoring/blackbox.yaml": `apiVersion: v1
kind: Service
metadata:
  name: blackbox
  namespace: monitoring
spec:
  selector: {app: blackbox}
`,
	}
}

// TestMeshScan (LB-F307, LB-F312): the side pass reads each mesh and
// monitoring object's references in Kubernetes' own terms — a gateway
// (never `mesh`), a destination's subset, a policy's workload selector and
// principals, a monitor's selector in each namespace it selects — and the
// providers and label sets its peers declare.
func TestMeshScan(t *testing.T) {
	refs, labels := meshScan([]byte(meshRepo()["deploy/mesh/istio.yaml"] + "---\n" + meshRepo()["deploy/monitoring/monitors.yaml"] + "---\n" + meshRepo()["deploy/mesh/workloads.yaml"]))
	var got []string
	for _, c := range refs {
		got = append(got, c.From+" "+c.Edge+" "+c.Strategy+" "+c.Raw+" -> "+c.Target+" "+c.Scope)
	}
	want := []string{
		"VirtualService/shop/reviews mesh_gateway route-object edge/public-gw -> Gateway/edge/public-gw ",
		"VirtualService/shop/reviews mesh_gateway route-object missing-gw -> Gateway/shop/missing-gw ",
		"VirtualService/shop/reviews mesh_gateway route-object shop-gw -> Gateway/shop/shop-gw ",
		"VirtualService/shop/reviews mesh_subset route-subset reviews -> v1 ",
		"VirtualService/shop/reviews mesh_subset route-subset reviews.shop.svc.cluster.local -> v2 ",
		"VirtualService/shop/reviews mesh_subset route-subset reviews -> v9 ",
		"DestinationRule/shop/reviews provides_subset route-provides reviews -> v1 ",
		"DestinationRule/shop/reviews provides_subset route-provides reviews -> v2 ",
		"ServiceEntry/shop/payments-api provides_host route-provides api.payments.example.com ->  ",
		"AuthorizationPolicy/shop/reviews authz_selector route-selector app=reviews -> app=reviews pods/shop",
		"AuthorizationPolicy/shop/reviews authz_principal route-object * ->  ",
		"AuthorizationPolicy/shop/reviews authz_principal route-object cluster.local/ns/shop/sa/ghost -> ServiceAccount/shop/ghost ",
		"AuthorizationPolicy/shop/reviews authz_principal route-object cluster.local/ns/shop/sa/reviews -> ServiceAccount/shop/reviews ",
		"ServiceMonitor/monitoring/reviews monitor_selector route-selector app in (ratings,reviews),team=storefront -> app in (ratings,reviews),team=storefront services/shop",
		"PodMonitor/monitoring/reviews-pods monitor_selector route-selector app=reviews -> app=reviews pods/*",
		"ServiceMonitor/monitoring/nothing monitor_selector route-selector app=nobody -> app=nobody services/monitoring",
		"Service/shop/reviews service_selector route-selector app=reviews -> app=reviews pods/shop",
		"Service/shop/ratings service_selector route-selector app=ratings -> app=ratings pods/shop",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("meshScan refs\n got: %q\nwant: %q", got, want)
	}
	var gotLabels []string
	for _, ls := range labels {
		keys := make([]string, 0, len(ls.Labels))
		for k, v := range ls.Labels {
			keys = append(keys, k+"="+v)
		}
		sort.Strings(keys)
		gotLabels = append(gotLabels, ls.Entity+" "+ls.Scope+" "+strings.Join(keys, ","))
	}
	wantLabels := []string{
		"Deployment/shop/reviews-v1 pods/shop app=reviews,version=v1",
		"Deployment/shop/reviews-v2 pods/shop app=reviews,version=v2",
		"Service/shop/reviews services/shop app=reviews,team=storefront",
		"Service/shop/ratings services/shop app=ratings",
	}
	if !reflect.DeepEqual(gotLabels, wantLabels) {
		t.Errorf("meshScan labels\n got: %q\nwant: %q", gotLabels, wantLabels)
	}
	if r, l := meshScan([]byte("kind: [unterminated\n")); r != nil || l != nil {
		t.Errorf("malformed YAML yields nothing, got %v %v", r, l)
	}
}

var wantMeshEdges = []string{
	// AuthorizationPolicy: the workload selector and a principal's ServiceAccount
	"deploy/mesh/istio.yaml|mesh::deploy/mesh/istio.AuthorizationPolicy/shop/reviews->mesh::deploy/mesh/workloads.Deployment/shop/reviews-v1",
	"deploy/mesh/istio.yaml|mesh::deploy/mesh/istio.AuthorizationPolicy/shop/reviews->mesh::deploy/mesh/workloads.Deployment/shop/reviews-v2",
	"deploy/mesh/istio.yaml|mesh::deploy/mesh/istio.AuthorizationPolicy/shop/reviews->mesh::deploy/mesh/workloads.ServiceAccount/shop/reviews",
	// the DestinationRule's host (route-host, LB-F96)
	"deploy/mesh/istio.yaml|mesh::deploy/mesh/istio.DestinationRule/shop/reviews->mesh::deploy/mesh/workloads.Service/shop/reviews",
	// an egress route joins the ServiceEntry declaring its host
	"deploy/mesh/istio.yaml|mesh::deploy/mesh/istio.VirtualService/shop/payments->mesh::deploy/mesh/istio.ServiceEntry/shop/payments-api",
	// the VirtualService: its DestinationRule's subsets, its gateways, its Service
	"deploy/mesh/istio.yaml|mesh::deploy/mesh/istio.VirtualService/shop/reviews->mesh::deploy/edge/gateway.Gateway/edge/public-gw",
	"deploy/mesh/istio.yaml|mesh::deploy/mesh/istio.VirtualService/shop/reviews->mesh::deploy/mesh/istio.DestinationRule/shop/reviews",
	"deploy/mesh/istio.yaml|mesh::deploy/mesh/istio.VirtualService/shop/reviews->mesh::deploy/mesh/istio.Gateway/shop/shop-gw",
	"deploy/mesh/istio.yaml|mesh::deploy/mesh/istio.VirtualService/shop/reviews->mesh::deploy/mesh/workloads.Service/shop/reviews",
	// a Service's spec.selector: the workloads it fronts (the ServiceMonitor hop of observed-by)
	"deploy/mesh/workloads.yaml|mesh::deploy/mesh/workloads.Service/shop/reviews->mesh::deploy/mesh/workloads.Deployment/shop/reviews-v1",
	"deploy/mesh/workloads.yaml|mesh::deploy/mesh/workloads.Service/shop/reviews->mesh::deploy/mesh/workloads.Deployment/shop/reviews-v2",
	// monitors: a PodMonitor's pods, a ServiceMonitor's Services, a Probe's targets and prober
	"deploy/monitoring/monitors.yaml|mesh::deploy/monitoring/monitors.PodMonitor/monitoring/reviews-pods->mesh::deploy/mesh/workloads.Deployment/shop/reviews-v1",
	"deploy/monitoring/monitors.yaml|mesh::deploy/monitoring/monitors.PodMonitor/monitoring/reviews-pods->mesh::deploy/mesh/workloads.Deployment/shop/reviews-v2",
	"deploy/monitoring/monitors.yaml|mesh::deploy/monitoring/monitors.Probe/monitoring/storefront->mesh::deploy/mesh/workloads.Service/shop/reviews",
	"deploy/monitoring/monitors.yaml|mesh::deploy/monitoring/monitors.Probe/monitoring/storefront->mesh::deploy/monitoring/blackbox.Service/monitoring/blackbox",
	"deploy/monitoring/monitors.yaml|mesh::deploy/monitoring/monitors.ServiceMonitor/monitoring/reviews->mesh::deploy/mesh/workloads.Service/shop/reviews",
}

// TestIngest_MeshRouteRefs (LB-F307, LB-F312): an ingest stores the side
// pass's joins under the route-ref typed_edges key beside the route hosts —
// a VirtualService to its Gateways (`mesh` names none, a missing one is
// undeclared) and to the DestinationRule declaring each subset it names, an
// egress route to the ServiceEntry declaring its host, an
// AuthorizationPolicy to the workloads it selects and the ServiceAccount a
// principal names (a wildcard is external), a ServiceMonitor to the
// Services and a PodMonitor to the workloads their selectors match in the
// namespaces they select, a Probe to its static targets and prober — and a
// change to the workloads' file reaches the policies and monitors in blast
// radius.
func TestIngest_MeshRouteRefs(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, meshRepo())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	rep, err := p.Ingest(ctx, core.ServiceRef{ID: "mesh", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	if got := storedRouteEdges(t, st, "mesh"); !reflect.DeepEqual(got, wantMeshEdges) {
		t.Fatalf("stored route-ref edges\n got: %q\nwant: %q", got, wantMeshEdges)
	}
	// unresolved: the missing gateway, subset v9, the ghost ServiceAccount
	// the nothing monitor and the ratings and blackbox Services' selectors
	// (undeclared); the wildcard principal, the
	// Gateway servers' hosts and the Probe's public target (external)
	wantBy := map[string]int{RouteUndeclared: 6, RouteExternal: 4}
	if !reflect.DeepEqual(rep.RouteRefUnresolved, wantBy) {
		t.Errorf("RouteRefUnresolved = %v, want %v", rep.RouteRefUnresolved, wantBy)
	}
	obs, err := st.QueryObservations(ctx, "mesh", "")
	if err != nil {
		t.Fatal(err)
	}
	var nodes []core.SymbolNode
	for _, o := range obs {
		nodes = append(nodes, o.ToSymbolNode())
	}
	typed, err := st.TypedEdges(ctx, "mesh")
	if err != nil {
		t.Fatal(err)
	}
	reached := impact.BlastRadiusWithTypedEdges(nodes, []string{"deploy/mesh/workloads.yaml"}, typed)
	for _, want := range []string{
		"mesh::deploy/mesh/istio.AuthorizationPolicy/shop/reviews",
		"mesh::deploy/monitoring/monitors.ServiceMonitor/monitoring/reviews",
		"mesh::deploy/monitoring/monitors.PodMonitor/monitoring/reviews-pods",
	} {
		if !slices.Contains(reached.Symbols, want) {
			t.Errorf("blast radius of deploy/mesh/workloads.yaml misses %s\n got: %q", want, reached.Symbols)
		}
	}
	gw := impact.BlastRadiusWithTypedEdges(nodes, []string{"deploy/edge/gateway.yaml"}, typed)
	if !slices.Contains(gw.Symbols, "mesh::deploy/mesh/istio.VirtualService/shop/reviews") {
		t.Errorf("a change to the Gateway's file does not reach the VirtualService bound to it: %q", gw.Symbols)
	}
}

// TestResolveRouteRefs_MeshNearestAndDeterministic: a selector joins every
// match in the nearest scope holding one (a workload in the policy's own
// file wins over one elsewhere), an object reference joins every nearest
// declaration of the object, and the answer does not depend on file order.
func TestResolveRouteRefs_MeshNearestAndDeterministic(t *testing.T) {
	mk := func(p, body string) DocFile {
		nodes, err := extractor.NewK8sExtractor().Extract("svc", p, []byte(body))
		if err != nil {
			t.Fatal(err)
		}
		refs, labels := meshScan([]byte(body))
		return DocFile{Path: p, Result: extractor.DocResult{Nodes: nodes, CrossFile: refs, Labels: labels}}
	}
	policy := `apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata: {name: web, namespace: shop}
spec:
  selector: {matchLabels: {app: web}}
  rules: [{from: [{source: {principals: [cluster.local/ns/shop/sa/web]}}]}]
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: web, namespace: shop}
spec:
  template: {metadata: {labels: {app: web}}}
`
	elsewhere := `apiVersion: apps/v1
kind: Deployment
metadata: {name: web-old, namespace: shop}
spec:
  template: {metadata: {labels: {app: web}}}
---
apiVersion: v1
kind: ServiceAccount
metadata: {name: web, namespace: shop}
`
	again := "apiVersion: v1\nkind: ServiceAccount\nmetadata: {name: web, namespace: shop}\n"
	files := []DocFile{mk("a/policy.yaml", policy), mk("b/all.yaml", elsewhere), mk("b/sa.yaml", again)}
	res := ResolveRouteRefs(files)
	refs, _ := routeBrief(res)
	want := []string{
		"a/policy.yaml:AuthorizationPolicy/shop/web -app=web-> a/policy.yaml:Deployment/shop/web ",
		"a/policy.yaml:AuthorizationPolicy/shop/web -cluster.local/ns/shop/sa/web-> b/all.yaml:ServiceAccount/shop/web ",
		"a/policy.yaml:AuthorizationPolicy/shop/web -cluster.local/ns/shop/sa/web-> b/sa.yaml:ServiceAccount/shop/web ",
	}
	if !reflect.DeepEqual(refs, want) {
		t.Errorf("refs\n got: %q\nwant: %q", refs, want)
	}
	rev := slices.Clone(files)
	slices.Reverse(rev)
	if r2 := ResolveRouteRefs(rev); !reflect.DeepEqual(r2, res) {
		t.Errorf("ResolveRouteRefs is order-dependent on side-pass references")
	}
}

// TestIngest_PrometheusRuleJoinWithheld (LB-F312, deferred): the Stage A
// static reading's kill fired (docs/bench/readings/telemetry/
// stage-a-static.md: label_join_recall .583 under .60 on kube-prometheus,
// where every join made was right), so a PrometheusRule's expressions join
// nothing: the rule object is stored, its expressions give the route pass no
// reference and no unresolved count.
func TestIngest_PrometheusRuleJoinWithheld(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{
		"k8s/services.yaml": `apiVersion: v1
kind: Service
metadata: {name: orders, namespace: shop}
spec: {ports: [{port: 80}]}
---
apiVersion: v1
kind: Service
metadata: {name: checkout, namespace: monitoring}
spec: {ports: [{port: 80}]}
`,
		"k8s/rules.yaml": `apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: shop-rules
  namespace: monitoring
spec:
  groups:
    - name: shop.rules
      rules:
        - alert: OrdersDown
          expr: up{job="orders", namespace="shop"} == 0
        - record: job:http_requests:rate5m
          expr: sum by (job) (rate(http_requests_total{job="checkout"}[5m]))
        - alert: AnyDown
          expr: up{job=~"orders|checkout"} == 0
`,
	})
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	rep, err := p.Ingest(context.Background(), core.ServiceRef{ID: "pr", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	if got := storedRouteEdges(t, st, "pr"); len(got) != 0 {
		t.Fatalf("a PrometheusRule joined %q; the Stage A kill withholds the join", got)
	}
	if len(rep.RouteRefUnresolved) != 0 {
		t.Errorf("RouteRefUnresolved = %v, want none", rep.RouteRefUnresolved)
	}
	obs, err := st.QueryObservations(context.Background(), "pr", "")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, o := range obs {
		found = found || o.ToSymbolNode().Name == "PrometheusRule/monitoring/shop-rules"
	}
	if !found {
		t.Errorf("the PrometheusRule object is not stored")
	}
}
