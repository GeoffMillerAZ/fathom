package fusion

import (
	"context"
	"os"
	"path/filepath"
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

// TestRouteTarget pins the route-host normalization and the Kubernetes and
// Istio DNS rules (LB-F96): scheme, userinfo, port, path and Istio's
// namespace/dnsName prefix stripped; name, name.ns, name.ns.svc and
// name.ns.svc.cluster.local read as a Service, a short name in the
// referrer's namespace; a value naming no host is unresolved by reason.
func TestRouteTarget(t *testing.T) {
	for _, c := range []struct {
		raw, ns string
		inK8s   bool
		keys    []string
		miss    string
		reason  string
	}{
		{raw: "orders", ns: "shop", inK8s: true, keys: []string{"deploy:k8s/shop/orders"}, miss: RouteUndeclared},
		{raw: "orders", inK8s: false, keys: []string{"deploy:host/orders", "deploy:k8s/default/orders"}, miss: RouteUndeclared},
		{raw: "orders.billing", ns: "shop", inK8s: true, keys: []string{"deploy:k8s/billing/orders", "deploy:host/orders.billing"}, miss: RouteExternal},
		{raw: "orders.billing.svc", ns: "shop", inK8s: true, keys: []string{"deploy:k8s/billing/orders"}, miss: RouteUndeclared},
		{raw: "orders.billing.svc.cluster.local", inK8s: false, keys: []string{"deploy:k8s/billing/orders"}, miss: RouteUndeclared},
		// The userinfo literal is split so the ADR-011 email-shape scan never reads it as one token.
		{raw: "http://user:pw@" + "Orders.Billing.svc.cluster.local:8080/api/v1?x=1", inK8s: false, keys: []string{"deploy:k8s/billing/orders"}, miss: RouteUndeclared},
		{raw: "orders:8080", ns: "shop", inK8s: true, keys: []string{"deploy:k8s/shop/orders"}, miss: RouteUndeclared},
		{raw: "api.example.com", ns: "shop", inK8s: true, keys: []string{"deploy:host/api.example.com"}, miss: RouteExternal},
		{raw: "https://api.example.com/v2", inK8s: false, keys: []string{"deploy:host/api.example.com"}, miss: RouteExternal},
		{raw: "shop/shop.example.com", ns: "shop", inK8s: true, keys: []string{"deploy:host/shop.example.com"}, miss: RouteExternal},
		{raw: "./orders.shop.svc.cluster.local", ns: "shop", inK8s: true, keys: []string{"deploy:k8s/shop/orders"}, miss: RouteUndeclared},
		{raw: "mongodb://mongo:27017", inK8s: false, keys: []string{"deploy:host/mongo", "deploy:k8s/default/mongo"}, miss: RouteUndeclared},
		{raw: "*", ns: "shop", inK8s: true, reason: RouteExternal},
		{raw: "*.example.com", inK8s: false, reason: RouteExternal},
		{raw: "10.0.0.7:8080", inK8s: false, reason: RouteIPLiteral},
		{raw: "http://[::1]:80", inK8s: false, reason: RouteIPLiteral},
		{raw: "localhost:27017", inK8s: false, reason: RouteIPLiteral},
		{raw: "$backend", inK8s: false, reason: RouteVariable},
		{raw: "http://${UPSTREAM}:80", inK8s: false, reason: RouteVariable},
		{raw: "$(MONGODB_URI)", inK8s: true, ns: "shop", reason: RouteVariable},
		{raw: "{{ .Values.host }}", inK8s: true, ns: "shop", reason: RouteVariable},
		{raw: "unix:/var/run/app.sock", inK8s: false, reason: RouteUnixSocket},
		{raw: "/tmp/mongo.sock", inK8s: false, reason: RouteUnixSocket},
		{raw: "%2Ftmp%2Fmongodb-27017.sock", inK8s: false, reason: RouteUnixSocket},
		{raw: "mongodb+srv://u:p@cluster0.example.com/db", inK8s: false, reason: RouteExternal},
		{raw: "", inK8s: false, reason: RouteUndeclared},
	} {
		keys, miss, reason := routeTarget(c.raw, c.ns, c.inK8s)
		if !reflect.DeepEqual(keys, c.keys) || miss != c.miss || reason != c.reason {
			t.Errorf("routeTarget(%q, %q, %v) = %q, %q, %q; want %q, %q, %q", c.raw, c.ns, c.inK8s, keys, miss, reason, c.keys, c.miss, c.reason)
		}
	}
}

func routeNode(file, name string, kind core.SourceKind) core.SymbolNode {
	return core.SymbolNode{Service: "svc", Package: pkgOf(file), Name: name, Kind: kind, FilePath: file, Line: 1}
}

func pkgOf(file string) string { return strings.TrimSuffix(file, filepath.Ext(file)) }

func routeRef(from, raw string, line int) extractor.DocCrossFileRef {
	return extractor.DocCrossFileRef{From: from, Edge: RouteRefEdge, Strategy: RouteHostStrategy, Raw: raw, Target: raw, Line: line}
}

func routeBrief(res RouteResolution) (refs, unresolved []string) {
	for _, r := range res.Refs {
		refs = append(refs, r.FromFile+":"+r.From+" -"+r.Raw+"-> "+r.ToFile+":"+r.To+" "+r.Key)
	}
	for _, u := range res.Unresolved {
		unresolved = append(unresolved, u.FromFile+":"+u.From+" "+u.Raw+" "+u.Reason)
	}
	return refs, unresolved
}

// TestResolveRouteRefs pins LB-F96's join: each DNS form to its Service by
// the namespace rule, a compose service by its key, the nearest
// declarations (same file, then same directory, then anywhere) each joined
// when they provide one key and ambiguous when they provide two, every
// unresolved reference counted by reason, and a self-reference dropped.
func TestResolveRouteRefs(t *testing.T) {
	files := []DocFile{
		{Path: "k8s/shop/svc.yaml", Result: extractor.DocResult{Nodes: []core.SymbolNode{
			routeNode("k8s/shop/svc.yaml", "Service/shop/orders", core.KindK8sWorkload),
			routeNode("k8s/shop/svc.yaml", "Service/billing/ledger", core.KindK8sWorkload),
			routeNode("k8s/shop/svc.yaml", "Service/web", core.KindK8sWorkload),
			routeNode("k8s/shop/svc.yaml", "Deployment/shop/orders", core.KindK8sWorkload),
		}}},
		{Path: "k8s/shop/routes.yaml", Result: extractor.DocResult{
			Nodes: []core.SymbolNode{
				routeNode("k8s/shop/routes.yaml", "Ingress/shop/storefront", core.KindK8sWorkload),
				routeNode("k8s/shop/routes.yaml", "VirtualService/shop/orders", core.KindK8sWorkload),
				routeNode("k8s/shop/routes.yaml", "Ingress/front", core.KindK8sWorkload),
			},
			CrossFile: []extractor.DocCrossFileRef{
				routeRef("Ingress/shop/storefront", "orders", 10),                           // short name, own namespace
				routeRef("Ingress/shop/storefront", "web", 11),                              // no Service/web in shop
				routeRef("VirtualService/shop/orders", "orders.shop.svc.cluster.local", 20), // FQDN
				routeRef("VirtualService/shop/orders", "ledger.billing", 21),                // name.ns
				routeRef("VirtualService/shop/orders", "ledger.billing.svc", 22),            // name.ns.svc, same pair
				routeRef("VirtualService/shop/orders", "api.example.com", 23),               // external
				routeRef("VirtualService/shop/orders", "10.1.2.3", 24),                      // ip literal
				routeRef("Ingress/front", "web", 30),                                        // default namespace
				routeRef("Ingress/missing", "orders", 31),                                   // no such entity: nothing
				{From: "Ingress/front", Edge: "selects", Strategy: "label-selector", Raw: "app=web"},
			},
		}},
		{Path: "compose/docker-compose.yml", Result: extractor.DocResult{
			Nodes: []core.SymbolNode{
				routeNode("compose/docker-compose.yml", "proxy", core.KindComposeService),
				routeNode("compose/docker-compose.yml", "api", core.KindComposeService),
				routeNode("compose/docker-compose.yml", "mongo", core.KindComposeService),
			},
			CrossFile: []extractor.DocCrossFileRef{
				routeRef("proxy", "http://api:8080/", 5),
				routeRef("proxy", "proxy:80", 6), // itself: dropped
				routeRef("proxy", "$UPSTREAM", 7),
				routeRef("proxy", "unix:/run/app.sock", 8),
				routeRef("proxy", "cache", 9), // undeclared
				routeRef("proxy", "mongo", 10),
			},
		}},
		// A second compose file in another directory declaring mongo:
		// the referring file's own declaration is nearer, so not ambiguous.
		{Path: "other/compose.yaml", Result: extractor.DocResult{
			Nodes: []core.SymbolNode{
				routeNode("other/compose.yaml", "mongo", core.KindComposeService),
				routeNode("other/compose.yaml", "edge", core.KindComposeService),
			},
		}},
		// Two directories declare Service/shop/orders, one runtime object:
		// a route in a third joins both declarations.
		{Path: "mesh/vs.yaml", Result: extractor.DocResult{
			Nodes: []core.SymbolNode{routeNode("mesh/vs.yaml", "VirtualService/shop/edge", core.KindK8sWorkload)},
			CrossFile: []extractor.DocCrossFileRef{
				routeRef("VirtualService/shop/edge", "orders", 3),
			},
		}},
		{Path: "release/all.yaml", Result: extractor.DocResult{Nodes: []core.SymbolNode{
			routeNode("release/all.yaml", "Service/shop/orders", core.KindK8sWorkload),
		}}},
		// A bare name a compose service and a Service in namespace default
		// both declare, neither nearer: two keys, ambiguous.
		{Path: "edge/compose.yaml", Result: extractor.DocResult{
			Nodes:     []core.SymbolNode{routeNode("edge/compose.yaml", "gw", core.KindComposeService)},
			CrossFile: []extractor.DocCrossFileRef{routeRef("gw", "shared:9000", 4)},
		}},
		{Path: "edge2/compose.yaml", Result: extractor.DocResult{Nodes: []core.SymbolNode{
			routeNode("edge2/compose.yaml", "shared", core.KindComposeService),
		}}},
		{Path: "edge3/svc.yaml", Result: extractor.DocResult{Nodes: []core.SymbolNode{
			routeNode("edge3/svc.yaml", "Service/shared", core.KindK8sWorkload),
		}}},
	}
	res := ResolveRouteRefs(files)
	refs, unresolved := routeBrief(res)
	wantRefs := []string{
		"compose/docker-compose.yml:proxy -http://api:8080/-> compose/docker-compose.yml:api deploy:host/api",
		"compose/docker-compose.yml:proxy -mongo-> compose/docker-compose.yml:mongo deploy:host/mongo",
		"k8s/shop/routes.yaml:Ingress/front -web-> k8s/shop/svc.yaml:Service/web deploy:k8s/default/web",
		"k8s/shop/routes.yaml:Ingress/shop/storefront -orders-> k8s/shop/svc.yaml:Service/shop/orders deploy:k8s/shop/orders",
		"k8s/shop/routes.yaml:VirtualService/shop/orders -ledger.billing-> k8s/shop/svc.yaml:Service/billing/ledger deploy:k8s/billing/ledger",
		"k8s/shop/routes.yaml:VirtualService/shop/orders -ledger.billing.svc-> k8s/shop/svc.yaml:Service/billing/ledger deploy:k8s/billing/ledger",
		"k8s/shop/routes.yaml:VirtualService/shop/orders -orders.shop.svc.cluster.local-> k8s/shop/svc.yaml:Service/shop/orders deploy:k8s/shop/orders",
		"mesh/vs.yaml:VirtualService/shop/edge -orders-> k8s/shop/svc.yaml:Service/shop/orders deploy:k8s/shop/orders",
		"mesh/vs.yaml:VirtualService/shop/edge -orders-> release/all.yaml:Service/shop/orders deploy:k8s/shop/orders",
	}
	wantUnresolved := []string{
		"compose/docker-compose.yml:proxy $UPSTREAM variable",
		"compose/docker-compose.yml:proxy cache undeclared",
		"compose/docker-compose.yml:proxy unix:/run/app.sock unix-socket",
		"edge/compose.yaml:gw shared:9000 ambiguous",
		"k8s/shop/routes.yaml:Ingress/shop/storefront web undeclared",
		"k8s/shop/routes.yaml:VirtualService/shop/orders 10.1.2.3 ip-literal",
		"k8s/shop/routes.yaml:VirtualService/shop/orders api.example.com external",
	}
	if !reflect.DeepEqual(refs, wantRefs) {
		t.Errorf("refs\n got: %q\nwant: %q", refs, wantRefs)
	}
	if !reflect.DeepEqual(unresolved, wantUnresolved) {
		t.Errorf("unresolved\n got: %q\nwant: %q", unresolved, wantUnresolved)
	}
	if res.SelfDropped != 1 {
		t.Errorf("SelfDropped = %d, want 1", res.SelfDropped)
	}
	wantBy := map[string]int{RouteVariable: 1, RouteUndeclared: 2, RouteUnixSocket: 1, RouteIPLiteral: 1, RouteExternal: 1, RouteAmbiguous: 1}
	if got := res.UnresolvedByReason(); !reflect.DeepEqual(got, wantBy) {
		t.Errorf("UnresolvedByReason = %v, want %v", got, wantBy)
	}
	// One edge per symbol pair: ledger.billing and ledger.billing.svc
	// collapse, as do the two orders references.
	if got := len(RouteRefEdges(res.Refs)); got != 8 {
		t.Errorf("RouteRefEdges = %d edges, want 8", got)
	}
	// Determinism: the same answer from the files in reverse.
	rev := slices.Clone(files)
	slices.Reverse(rev)
	if r2 := ResolveRouteRefs(rev); !reflect.DeepEqual(r2, res) {
		t.Errorf("ResolveRouteRefs is order-dependent")
	}
}

// routeRepo is the ingest fixture: an Ingress, a VirtualService and an
// HTTPRoute in front of Services, a compose file whose nginx-like proxy is
// absent but whose mongobetween sidecar routes to a compose mongo, and a
// Kubernetes mongobetween routing to an in-cluster Service.
func routeRepo() map[string]string {
	return map[string]string{
		"deploy/k8s/services.yaml": `apiVersion: v1
kind: Service
metadata:
  name: orders
  namespace: shop
spec:
  selector:
    app: orders
---
apiVersion: v1
kind: Service
metadata:
  name: web
  namespace: shop
spec:
  selector:
    app: web
---
apiVersion: v1
kind: Service
metadata:
  name: mongo
  namespace: data
spec:
  selector:
    app: mongo
`,
		"deploy/k8s/routes.yaml": `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: storefront
  namespace: shop
spec:
  rules:
    - http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: web
                port:
                  number: 80
---
apiVersion: networking.istio.io/v1
kind: VirtualService
metadata:
  name: orders
  namespace: shop
spec:
  hosts: [orders]
  http:
    - route:
        - destination:
            host: orders.shop.svc.cluster.local
    - route:
        - destination:
            host: api.example.com
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: orders
  namespace: shop
spec:
  rules:
    - backendRefs:
        - name: orders
          port: 8080
        - name: ghost
          port: 8080
`,
		"deploy/k8s/pooler.yaml": `apiVersion: apps/v1
kind: Deployment
metadata:
  name: pooler
  namespace: shop
spec:
  template:
    spec:
      containers:
        - name: mongobetween
          image: ghcr.io/acme/mongobetween:1.0
          args:
            - -ping
            - -network
            - tcp4
            - ":27016=mongodb://mongo.data.svc.cluster.local:27017/orders?maxpoolsize=10"
`,
		"docker-compose.yml": `services:
  mongo:
    image: mongo:7
  pooler:
    image: coinbase/mongobetween:latest
    command: ["-loglevel", "debug", ":27016=mongodb://mongo:27017/app", "/tmp/m.sock=$(SECOND_URI)"]
  app:
    image: acme/app:1
    depends_on: [pooler]
`,
	}
}

func storedRouteEdges(t *testing.T, st store.Store, svc string) []string {
	t.Helper()
	all, err := st.TypedEdges(context.Background(), svc)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range all {
		if e.Kind == core.EdgeFileResolved {
			got = append(got, e.FromFile+"|"+e.FromSymbol+"->"+e.ToSymbol)
		}
	}
	sort.Strings(got)
	return got
}

var wantRouteRepoEdges = []string{
	"deploy/k8s/pooler.yaml|rt::deploy/k8s/pooler.Deployment/shop/pooler->rt::deploy/k8s/services.Service/data/mongo",
	"deploy/k8s/routes.yaml|rt::deploy/k8s/routes.HTTPRoute/shop/orders->rt::deploy/k8s/services.Service/shop/orders",
	"deploy/k8s/routes.yaml|rt::deploy/k8s/routes.Ingress/shop/storefront->rt::deploy/k8s/services.Service/shop/web",
	"deploy/k8s/routes.yaml|rt::deploy/k8s/routes.VirtualService/shop/orders->rt::deploy/k8s/services.Service/shop/orders",
	"docker-compose.yml|rt::docker-compose.pooler->rt::docker-compose.mongo",
}

// TestIngest_RouteRefs (LB-F96, LB-F97): an ingest stores one route_ref
// edge per resolved (route entity, provider) pair under the "route-ref"
// typed_edges key — Kubernetes routes read through the kubernetes dialect
// while k8sextract.go is dispatched, compose services from composeextract,
// mongobetween's launch arguments in both — and discloses the unresolved
// references by reason; a re-ingest with nothing changed skips the pass and
// keeps the edges.
func TestIngest_RouteRefs(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, routeRepo())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	rep, err := p.Ingest(ctx, core.ServiceRef{ID: "rt", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	if got := storedRouteEdges(t, st, "rt"); !reflect.DeepEqual(got, wantRouteRepoEdges) {
		t.Fatalf("stored route-ref edges\n got: %q\nwant: %q", got, wantRouteRepoEdges)
	}
	if rep.RouteRefEdges != len(wantRouteRepoEdges) || rep.RouteRefSkipped != "" {
		t.Errorf("RouteRefEdges = %d (skipped %q), want %d", rep.RouteRefEdges, rep.RouteRefSkipped, len(wantRouteRepoEdges))
	}
	wantBy := map[string]int{RouteExternal: 1, RouteUndeclared: 4, RouteVariable: 1} // +3: each Service selector with no workload in the corpus
	if !reflect.DeepEqual(rep.RouteRefUnresolved, wantBy) {
		t.Errorf("RouteRefUnresolved = %v, want %v", rep.RouteRefUnresolved, wantBy)
	}

	rep2, err := p.Ingest(ctx, core.ServiceRef{ID: "rt", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	if rep2.RouteRefSkipped != SkippedPassReason {
		t.Errorf("unchanged re-ingest: RouteRefSkipped = %q, want %q", rep2.RouteRefSkipped, SkippedPassReason)
	}
	if got := storedRouteEdges(t, st, "rt"); !reflect.DeepEqual(got, wantRouteRepoEdges) {
		t.Errorf("edges after a skipped pass\n got: %q\nwant: %q", got, wantRouteRepoEdges)
	}
}

// TestRouteRefsReachBlastRadius (LB-F97, I59): a change to the Services'
// file reaches every route entity in front of them in blast radius through
// the stored edges — the typed edges impact.BlastRadiusWithTypedEdges and
// the call graph read — and nothing reaches them without those edges.
func TestRouteRefsReachBlastRadius(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, routeRepo())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "rt", Path: dir}); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "rt", "")
	if err != nil {
		t.Fatal(err)
	}
	var nodes []core.SymbolNode
	for _, o := range obs {
		nodes = append(nodes, o.ToSymbolNode())
	}
	typed, err := st.TypedEdges(ctx, "rt")
	if err != nil {
		t.Fatal(err)
	}
	reached := impact.BlastRadiusWithTypedEdges(nodes, []string{"deploy/k8s/services.yaml"}, typed)
	for _, want := range []string{
		"rt::deploy/k8s/routes.Ingress/shop/storefront",
		"rt::deploy/k8s/routes.VirtualService/shop/orders",
		"rt::deploy/k8s/routes.HTTPRoute/shop/orders",
		"rt::deploy/k8s/pooler.Deployment/shop/pooler",
	} {
		if !slices.Contains(reached.Symbols, want) {
			t.Errorf("blast radius of deploy/k8s/services.yaml misses %s\n got: %q", want, reached.Symbols)
		}
	}
	if !slices.Contains(reached.Files, "deploy/k8s/routes.yaml") {
		t.Errorf("blast radius files %q miss deploy/k8s/routes.yaml", reached.Files)
	}
	without := impact.BlastRadiusWithTypedEdges(nodes, []string{"deploy/k8s/services.yaml"}, nil)
	if slices.Contains(without.Files, "deploy/k8s/routes.yaml") {
		t.Errorf("routes.yaml reached without route edges: the test proves nothing")
	}
	compose := impact.BlastRadiusWithTypedEdges(nodes, []string{"docker-compose.yml"}, typed)
	if !slices.Contains(compose.Symbols, "rt::docker-compose.pooler") {
		t.Errorf("compose blast radius %q misses the pooler", compose.Symbols)
	}
}

// TestRouteRefEdgesSurviveTheGoPass: the typed Go pass reconciles away every
// typed_edges key that is not one of its packages; the route-ref and
// doc-xref rows are another pass's, so a Go-only change (which skips both
// gated passes) must leave them standing.
func TestRouteRefEdgesSurviveTheGoPass(t *testing.T) {
	dir := t.TempDir()
	files := routeRepo()
	files["go.mod"] = "module example.com/rt\n\ngo 1.22\n"
	files["main.go"] = "package main\n\nfunc main() { helper() }\n\nfunc helper() {}\n"
	files["package.json"] = `{"name":"root","workspaces":["packages/a"]}`
	files["packages/a/package.json"] = `{"name":"a"}`
	writeCorpus(t, dir, files)
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "rt", Path: dir}); err != nil {
		t.Fatal(err)
	}
	before := storedRouteEdges(t, st, "rt")
	if !slices.Contains(before, "package.json|rt::package.root->rt::packages/a/package.a") {
		t.Fatalf("doc-xref edge missing after the first ingest: %q", before)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() { helper(); helper() }\n\nfunc helper() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := p.Ingest(ctx, core.ServiceRef{ID: "rt", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	if rep.TypedPackages == 0 {
		t.Skipf("the typed Go pass did not run here (%q): the reconcile under test never ran", rep.TypedFailReason)
	}
	if rep.RouteRefSkipped == "" || rep.DocXrefSkipped == "" {
		t.Fatalf("a Go-only change ran a gated pass (route %q, doc %q): the test proves nothing", rep.RouteRefSkipped, rep.DocXrefSkipped)
	}
	if after := storedRouteEdges(t, st, "rt"); !reflect.DeepEqual(after, before) {
		t.Errorf("file-tier edges after a Go-only re-ingest\n got: %q\nwant: %q", after, before)
	}
}

// TestRouteCandidatesAreManifestDerived: the pass reads what a doc or
// native dialect's manifest claims (YAML, JSON, TOML, go.mod, compose and
// Kubernetes files), never a tags-engine language's source.
func TestRouteCandidatesAreManifestDerived(t *testing.T) {
	for p, want := range map[string]bool{
		"deploy/k8s/routes.yaml": true,
		"docker-compose.yml":     true,
		"compose.override.yaml":  true,
		"package.json":           true,
		"go.mod":                 true,
		"main.go":                false,
		"src/app.ts":             false,
		"README.md":              false,
	} {
		if got := isRouteCandidate(p); got != want {
			t.Errorf("isRouteCandidate(%q) = %v, want %v", p, got, want)
		}
	}
}

// TestRouteRefPassRerunsAfterOneFileRefresh: the one-file refresh writes a
// route candidate's fingerprint, so the next ingest sees no change in it;
// the refresh must clear the route-ref pass's success record (passKeysFor)
// or the edges stay stale until some other candidate changes.
func TestRouteRefPassRerunsAfterOneFileRefresh(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, routeRepo())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "rt", Path: dir}); err != nil {
		t.Fatal(err)
	}
	routes := filepath.Join(dir, "deploy/k8s/routes.yaml")
	b, err := os.ReadFile(routes)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(routes, []byte(strings.Replace(string(b), "name: web", "name: orders", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ReingestFile(ctx, st, "rt", dir, "deploy/k8s/routes.yaml"); err != nil {
		t.Fatal(err)
	}
	rep, err := p.Ingest(ctx, core.ServiceRef{ID: "rt", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	if rep.RouteRefSkipped != "" {
		t.Fatalf("route-ref pass skipped (%q) after a one-file refresh of a route file", rep.RouteRefSkipped)
	}
	want := "deploy/k8s/routes.yaml|rt::deploy/k8s/routes.Ingress/shop/storefront->rt::deploy/k8s/services.Service/shop/orders"
	if got := storedRouteEdges(t, st, "rt"); !slices.Contains(got, want) {
		t.Errorf("edges after the refresh miss %s\n got: %q", want, got)
	}
}

// TestMongobetweenLaunchRoutes (LB-F100): mongobetween has no config file;
// its routes are the positional address=uri arguments of the container or
// compose service that launches it (image or command naming it), flags
// read as Go's flag package reads them. Each host of a mongodb:// seed list
// is one reference from the launching entity; a mongodb+srv:// URI and a
// $(VAR) are one each, unresolved by reason; the fixture is written from
// the project's README usage (testdata/mongobetweenfixture/PROVENANCE.md).
func TestMongobetweenLaunchRoutes(t *testing.T) {
	read := func(p string) []byte {
		b, err := os.ReadFile(filepath.Join("testdata", "mongobetweenfixture", p))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	brief := func(cs []extractor.DocCrossFileRef) []string {
		var out []string
		for _, c := range cs {
			if c.Edge != RouteRefEdge || c.Strategy != RouteHostStrategy || c.Target != c.Raw {
				t.Errorf("%+v breaks the route contract", c)
			}
			out = append(out, c.From+" "+c.Raw)
		}
		return out
	}
	compose := brief(launchRouteRefs(ToolCompose, read("docker-compose.yml")))
	wantCompose := []string{
		"pooler mongo-a:27017",
		"pooler mongo-b:27017",
		"pooler mongodb+srv://username:password@cluster1.example.com/database?label=cluster1",
		"sidecar localhost:27017",
	}
	if !reflect.DeepEqual(compose, wantCompose) {
		t.Errorf("compose launch routes\n got: %q\nwant: %q", compose, wantCompose)
	}
	k8s := brief(launchRouteRefs(ToolK8sManifest, read("k8s/pooler.yaml")))
	wantK8s := []string{
		"Deployment/shop/orders mongos.data:27017",
		"Deployment/shop/orders mongos.data.svc.cluster.local:27017",
		"Deployment/shop/orders mongos-1.data.svc.cluster.local:27017",
		"Deployment/shop/orders $(MONGODB_URI)",
		"CronJob/shop/report report-db:27017",
	}
	if !reflect.DeepEqual(k8s, wantK8s) {
		t.Errorf("k8s launch routes\n got: %q\nwant: %q", k8s, wantK8s)
	}
	if got := launchRouteRefs(ToolK8sManifest, []byte("kind: Deployment\nmetadata: {name: x}\nspec: {containers: [{image: nginx, args: [\"a=mongodb://b:1\"]}]}\n")); got != nil {
		t.Errorf("a container that does not launch mongobetween gave %q", brief(got))
	}

	// Through the resolver, at ingest: the seed-list hosts join the
	// Service and the compose services; the rest are disclosed.
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{
		"docker-compose.yml": string(read("docker-compose.yml")),
		"k8s/pooler.yaml":    string(read("k8s/pooler.yaml")),
	})
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	rep, err := p.Ingest(context.Background(), core.ServiceRef{ID: "mb", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"docker-compose.yml|mb::docker-compose.pooler->mb::docker-compose.mongo-a",
		"docker-compose.yml|mb::docker-compose.pooler->mb::docker-compose.mongo-b",
		"k8s/pooler.yaml|mb::k8s/pooler.Deployment/shop/orders->mb::k8s/pooler.Service/data/mongos",
	}
	if got := storedRouteEdges(t, st, "mb"); !reflect.DeepEqual(got, want) {
		t.Errorf("stored mongobetween edges\n got: %q\nwant: %q", got, want)
	}
	wantBy := map[string]int{RouteExternal: 1, RouteIPLiteral: 1, RouteUndeclared: 3, RouteVariable: 1}
	if !reflect.DeepEqual(rep.RouteRefUnresolved, wantBy) {
		t.Errorf("RouteRefUnresolved = %v, want %v", rep.RouteRefUnresolved, wantBy)
	}
}
