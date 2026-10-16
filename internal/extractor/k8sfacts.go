package extractor

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// k8sFact is one fact a workload's manifest writes (LB-F451): its key, the
// value as written and the key's line, relative to the document.
type k8sFact struct {
	key, value string
	line       int
}

// k8sPodPaths are the three places a pod spec lives (ADR-067 decision 12):
// a Pod's `spec`; the template of a Deployment, StatefulSet, DaemonSet,
// ReplicaSet, ReplicationController or Job; and a CronJob's job template.
var k8sPodPaths = [][]string{
	{"spec"},
	{"spec", "template", "spec"},
	{"spec", "jobTemplate", "spec", "template", "spec"},
}

// k8sReplicaKinds are the kinds whose `spec.replicas` sizes a pod template
// (apps/v1 Deployment, StatefulSet and ReplicaSet, core/v1
// ReplicationController).
var k8sReplicaKinds = map[string]bool{"Deployment": true, "StatefulSet": true, "ReplicaSet": true, "ReplicationController": true}

// k8sContainerLists are the pod spec's container lists, each a fact of its
// own: an init container is never merged into the container list.
var k8sContainerLists = []string{"initContainers", "containers"}

// k8sResourceNames are the compute resources a container requests and is
// limited to.
var k8sResourceNames = []string{"cpu", "memory", "ephemeral-storage"}

// k8sProbes are the three probes, k8sProbeHandlers the handler a probe names
// (the first present, in this order), k8sProbeTimings its numeric fields.
var (
	k8sProbes        = []string{"livenessProbe", "readinessProbe", "startupProbe"}
	k8sProbeHandlers = []string{"exec", "httpGet", "tcpSocket", "grpc"}
	k8sProbeTimings  = []string{"initialDelaySeconds", "periodSeconds", "timeoutSeconds", "successThreshold", "failureThreshold"}
)

// k8sContainerFacts (LB-F451, ADR-067 decision 12) reads the facts of one
// manifest document root: `replicas` for a kind that sizes a pod template;
// then, from the first of the three pod paths that holds a container list,
// each list as a fact of its own (`containers`, `initContainers`: the names
// in written order), and per named container `<list>.<name>.resources.
// {requests,limits}.{cpu,memory,ephemeral-storage}` and, per probe,
// `<list>.<name>.<probe>` (its handler: exec, httpGet, tcpSocket, grpc, or
// none) with its port, an httpGet path and its timings. A value is kept as
// written, and only when literal: a `$(VAR)`, `${VAR}` or `{{ }}` value is
// decided at deploy time and is no fact (TL-5's rule). It never reads env,
// envFrom, args, command or a probe's exec command or headers (ADR-069
// decision 2): their values are withheld.
func k8sContainerFacts(root *yaml.Node, kind string) []k8sFact {
	var out []k8sFact
	// scalar keeps val, a literal scalar, as fact k at its key's line.
	scalar := func(val, at *yaml.Node, k string) {
		if v := k8sYScalar(val); k8sLiteral(v) {
			out = append(out, k8sFact{key: k, value: v, line: at.Line})
		}
	}
	if k8sReplicaKinds[kind] {
		if spec := k8sYGet(root, "spec"); spec != nil {
			if k, v := k8sYEntry(spec, "replicas"); v != nil {
				scalar(v, k, "replicas")
			}
		}
	}
	pod := k8sPodSpec(root)
	for _, list := range k8sContainerLists {
		key, seq := k8sYEntry(pod, list)
		cs := k8sYSeq(seq)
		if len(cs) == 0 {
			continue
		}
		names := make([]string, 0, len(cs))
		for _, c := range cs {
			names = append(names, k8sYScalar(k8sYGet(c, "name")))
		}
		out = append(out, k8sFact{key: list, value: strings.Join(names, ","), line: key.Line})
		for i, c := range cs {
			if names[i] == "" {
				continue // an unnamed container has no stable key
			}
			prefix := list + "." + names[i] + "."
			for _, bound := range []string{"requests", "limits"} {
				m := k8sYGet(c, "resources", bound)
				for _, r := range k8sResourceNames {
					if k, v := k8sYEntry(m, r); v != nil {
						scalar(v, k, prefix+"resources."+bound+"."+r)
					}
				}
			}
			for _, probe := range k8sProbes {
				pk, p := k8sYEntry(c, probe)
				if p == nil || p.Kind != yaml.MappingNode {
					continue
				}
				handler, h := "none", (*yaml.Node)(nil)
				for _, name := range k8sProbeHandlers {
					if n := k8sYGet(p, name); n != nil && n.Kind == yaml.MappingNode {
						handler, h = name, n
						break
					}
				}
				out = append(out, k8sFact{key: prefix + probe, value: handler, line: pk.Line})
				if handler != "exec" && h != nil {
					if handler == "httpGet" {
						if k, v := k8sYEntry(h, "path"); v != nil {
							scalar(v, k, prefix+probe+".path")
						}
					}
					if k, v := k8sYEntry(h, "port"); v != nil {
						scalar(v, k, prefix+probe+".port")
					}
				}
				for _, t := range k8sProbeTimings {
					if k, v := k8sYEntry(p, t); v != nil {
						scalar(v, k, prefix+probe+"."+t)
					}
				}
			}
		}
	}
	return out
}

// k8sPodSpec is the first of the three pod paths whose spec holds a
// container list, nil for none.
func k8sPodSpec(root *yaml.Node) *yaml.Node {
	for _, p := range k8sPodPaths {
		spec := k8sYGet(root, p...)
		for _, list := range k8sContainerLists {
			if len(k8sYSeq(k8sYGet(spec, list))) > 0 {
				return spec
			}
		}
	}
	return nil
}

// k8sYEntry is a mapping's key node and value node for key, nil for none.
// A key the mapping repeats takes its last value, as Kubernetes' own decode
// reads it (sigs.k8s.io/yaml, kubectl).
func k8sYEntry(n *yaml.Node, key string) (k, v *yaml.Node) {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil, nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			k, v = n.Content[i], n.Content[i+1]
		}
	}
	return k, v
}

// k8sYGet walks keys through nested mappings, each key's last value
// winning (k8sYEntry), nil when a step is missing.
func k8sYGet(n *yaml.Node, keys ...string) *yaml.Node {
	for _, key := range keys {
		_, n = k8sYEntry(n, key)
		if n == nil {
			return nil
		}
	}
	return n
}

// k8sDocFromNode reads the fields k8sDoc holds from a document's root
// mapping, each key's last value winning. Extract falls back to it when
// yaml.v3's struct decode refuses the document because a mapping repeats a
// key, which kubectl and Kubernetes' own decode accept (O1034); ok is false
// for a root that is no mapping.
func k8sDocFromNode(root *yaml.Node) (doc k8sDoc, ok bool) {
	if root == nil || root.Kind != yaml.MappingNode {
		return doc, false
	}
	doc.APIVersion = k8sYScalar(k8sYGet(root, "apiVersion"))
	doc.Kind = k8sYScalar(k8sYGet(root, "kind"))
	doc.Metadata.Name = k8sYScalar(k8sYGet(root, "metadata", "name"))
	doc.Metadata.Namespace = k8sYScalar(k8sYGet(root, "metadata", "namespace"))
	read := func(pod *yaml.Node) k8sPodContainers {
		var pc k8sPodContainers
		for _, list := range []struct {
			key string
			out *[]k8sContainer
		}{{"containers", &pc.Containers}, {"initContainers", &pc.InitContainers}} {
			for _, c := range k8sYSeq(k8sYGet(pod, list.key)) {
				*list.out = append(*list.out, k8sContainer{Name: k8sYScalar(k8sYGet(c, "name")), Image: k8sYScalar(k8sYGet(c, "image"))})
			}
		}
		return pc
	}
	spec := read(k8sYGet(root, "spec"))
	doc.Spec.Containers, doc.Spec.InitContainers = spec.Containers, spec.InitContainers
	doc.Spec.Template.Spec = read(k8sYGet(root, "spec", "template", "spec"))
	doc.Spec.JobTemplate.Spec.Template.Spec = read(k8sYGet(root, "spec", "jobTemplate", "spec", "template", "spec"))
	return doc, true
}
