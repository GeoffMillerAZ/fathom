package main

import (
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// walk feeds every document of each file to a routeWalk, as main does.
func walk(t *testing.T, files map[string]string) *routeWalk {
	t.Helper()
	w := &routeWalk{}
	var paths []string
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		dec := yaml.NewDecoder(strings.NewReader(files[p]))
		for {
			var doc yaml.Node
			if dec.Decode(&doc) != nil {
				break
			}
			w.document(p, doc.Content[0])
		}
	}
	return w
}

// TestMeshPairs (LB-F315): the side-pass semantics, read from the projects'
// documentation: gateways, subsets, an egress entry, a policy's selector and
// principals, monitors in the namespaces they select, a probe's targets and a
// rule's identifying matchers.
func TestMeshPairs(t *testing.T) {
	ruleRoutes = true
	t.Cleanup(func() { ruleRoutes = false })
	w := walk(t, map[string]string{
		"a/svc.yaml": `apiVersion: v1
kind: Service
metadata: {name: reviews, namespace: shop, labels: {app: reviews, team: web}}
spec: {selector: {app: reviews}}
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: reviews-v1, namespace: shop}
spec: {template: {metadata: {labels: {app: reviews, version: v1}}}}
---
apiVersion: v1
kind: ServiceAccount
metadata: {name: reviews, namespace: shop}
`,
		"a/mesh.yaml": `apiVersion: networking.istio.io/v1
kind: Gateway
metadata: {name: gw, namespace: shop}
---
apiVersion: networking.istio.io/v1
kind: VirtualService
metadata: {name: reviews, namespace: shop}
spec:
  gateways: [gw, mesh, other/none]
  http: [{route: [{destination: {host: reviews, subset: v1}}, {destination: {host: reviews, subset: v9}}]}]
  tls: [{route: [{destination: {host: api.example.com}}]}]
---
apiVersion: networking.istio.io/v1
kind: DestinationRule
metadata: {name: reviews, namespace: shop}
spec: {host: reviews.shop.svc.cluster.local, subsets: [{name: v1}]}
---
apiVersion: networking.istio.io/v1
kind: ServiceEntry
metadata: {name: api, namespace: shop}
spec: {hosts: [api.example.com]}
---
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata: {name: reviews, namespace: shop}
spec:
  selector: {matchLabels: {app: reviews}}
  rules: [{from: [{source: {principals: ["cluster.local/ns/shop/sa/reviews", "*"]}}]}]
`,
		"b/mon.yaml": `apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata: {name: web, namespace: monitoring}
spec:
  selector: {matchExpressions: [{key: team, operator: In, values: [web]}]}
  namespaceSelector: {matchNames: [shop]}
---
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata: {name: rules, namespace: monitoring}
spec:
  groups: [{name: g, rules: [{alert: A, expr: 'up{job="reviews", namespace="shop"} == 0'}, {alert: B, expr: 'up{job=~"x|y"} == 0'}]}]
`,
	})
	refs, un := w.resolve()
	var got []string
	for _, r := range refs {
		got = append(got, r.From+" -> "+r.To)
	}
	sort.Strings(got)
	// the entry's own host is no pair (a self-join is dropped); the
	// destination's host is the existing route-host pair
	want := []string{
		"AuthorizationPolicy/shop/reviews -> Deployment/shop/reviews-v1",
		"AuthorizationPolicy/shop/reviews -> ServiceAccount/shop/reviews",
		"DestinationRule/shop/reviews -> Service/shop/reviews",
		"PrometheusRule/monitoring/rules -> Service/shop/reviews",
		"ServiceMonitor/monitoring/web -> Service/shop/reviews",
		"VirtualService/shop/reviews -> DestinationRule/shop/reviews",
		"VirtualService/shop/reviews -> Gateway/shop/gw",
		"VirtualService/shop/reviews -> Service/shop/reviews",
		"VirtualService/shop/reviews -> ServiceEntry/shop/api",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("pairs\n got: %q\nwant: %q", got, want)
	}
	reasons := map[string]int{}
	for _, u := range un {
		reasons[u.Reason]++
	}
	if reasons["undeclared"] != 2 || reasons["external"] != 1 {
		t.Errorf("unresolved %v (want the other/none gateway and subset v9 undeclared, the * principal external)", un)
	}
}
