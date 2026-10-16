package mcp

import (
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// isPodWorkloadKind reports whether k is a Kubernetes kind whose pod template
// a Service or PodMonitor selector reaches: the object kinds observed-by
// answers for. A switch, not a package-level map (tenancy lint, design §4.7).
func isPodWorkloadKind(k string) bool {
	switch k {
	case "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet", "Job", "CronJob", "Pod":
		return true
	}
	return false
}

// kindOfObject is the Kubernetes kind of a K8sWorkload node (its name is
// `Kind/<ns>/<name>` or `Kind/<name>`).
func kindOfObject(n core.SymbolNode) string {
	if n.Kind != core.KindK8sWorkload {
		return ""
	}
	k, _, _ := strings.Cut(n.Name, "/")
	return k
}

// observedBy names the ServiceMonitor and PodMonitor objects whose selectors
// reach the workload def (TL-15), as a join over the stored route-ref edges
// (internal/fusion/routeref_mesh.go): a PodMonitor reaches a workload
// directly, a ServiceMonitor only through a Service whose own spec.selector
// reaches it (K19: Prometheus' own entities and these selectors, never a
// label join). The second result is false when def is not a pod-bearing
// Kubernetes workload, so no answer is owed. The rows are sorted and
// deterministic: {key, kind, name, via} with via the Service a ServiceMonitor
// reaches the workload through ("" for a PodMonitor).
func observedBy(def core.SymbolNode, nodes []core.SymbolNode, typed []core.Edge) ([]any, bool) {
	if !isPodWorkloadKind(kindOfObject(def)) {
		return nil, false
	}
	byKey := make(map[string]core.SymbolNode)
	for _, n := range nodes {
		if n.Kind == core.KindK8sWorkload {
			byKey[n.Key()] = n
		}
	}
	into := make(map[string][]string) // target key -> source keys (route-ref edges only)
	for _, e := range typed {
		if e.Kind == core.EdgeFileResolved {
			into[e.ToSymbol] = append(into[e.ToSymbol], e.FromSymbol)
		}
	}
	type row struct{ key, kind, name, via string }
	seen := map[[2]string]bool{}
	var rows []row
	add := func(from core.SymbolNode, via string) {
		k := [2]string{from.Key(), via}
		if seen[k] {
			return
		}
		seen[k] = true
		rows = append(rows, row{from.Key(), kindOfObject(from), from.Name, via})
	}
	for _, from := range into[def.Key()] {
		n, ok := byKey[from]
		if !ok {
			continue
		}
		switch kindOfObject(n) {
		case "PodMonitor":
			add(n, "")
		case "Service":
			for _, up := range into[from] {
				if m, ok := byKey[up]; ok && kindOfObject(m) == "ServiceMonitor" {
					add(m, n.Name)
				}
			}
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].key != rows[j].key {
			return rows[i].key < rows[j].key
		}
		return rows[i].via < rows[j].via
	})
	out := make([]any, 0, len(rows))
	for _, r := range rows {
		m := map[string]any{"key": r.key, "kind": r.kind, "name": r.name}
		if r.via != "" {
			m["via"] = r.via
		}
		out = append(out, m)
	}
	return out, true
}
