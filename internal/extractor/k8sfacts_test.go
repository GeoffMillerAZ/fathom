package extractor_test

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// k8sFactsFixture is LB-F451's hand-written manifest: a Pod (spec), a
// Deployment, a StatefulSet and a Job (spec.template.spec) and a CronJob
// (spec.jobTemplate.spec.template.spec); every probe type; init containers
// beside containers; a container with no resources; a templated value; env
// and args (and a probe's exec command) carrying a sentinel.
const k8sFactsFixture = "testdata/k8sfacts/workloads.yaml"

// wantK8sFacts are the fixture's facts, written by hand from the manifest:
// `<line> <workload>#<key> <value as written>`, each at its key's line.
var wantK8sFacts = []string{
	"10 Pod/shop/probe-pod#containers web",
	"15 Pod/shop/probe-pod#containers.web.resources.requests.cpu 100m",
	"16 Pod/shop/probe-pod#containers.web.resources.requests.memory 64Mi",
	"17 Pod/shop/probe-pod#containers.web.resources.requests.ephemeral-storage 1Gi",
	"19 Pod/shop/probe-pod#containers.web.resources.limits.cpu 1",
	"20 Pod/shop/probe-pod#containers.web.resources.limits.memory 128Mi",
	"21 Pod/shop/probe-pod#containers.web.resources.limits.ephemeral-storage 2Gi",
	"22 Pod/shop/probe-pod#containers.web.livenessProbe httpGet",
	"24 Pod/shop/probe-pod#containers.web.livenessProbe.path /healthz",
	"25 Pod/shop/probe-pod#containers.web.livenessProbe.port 8080",
	"26 Pod/shop/probe-pod#containers.web.livenessProbe.initialDelaySeconds 5",
	"27 Pod/shop/probe-pod#containers.web.livenessProbe.periodSeconds 10",
	"28 Pod/shop/probe-pod#containers.web.readinessProbe tcpSocket",
	"30 Pod/shop/probe-pod#containers.web.readinessProbe.port http",
	"31 Pod/shop/probe-pod#containers.web.readinessProbe.timeoutSeconds 2",
	"32 Pod/shop/probe-pod#containers.web.readinessProbe.failureThreshold 3",
	"33 Pod/shop/probe-pod#containers.web.startupProbe exec",
	"36 Pod/shop/probe-pod#containers.web.startupProbe.successThreshold 1",
	"44 Deployment/shop/api#replicas 3",
	"49 Deployment/shop/api#initContainers migrate",
	"54 Deployment/shop/api#initContainers.migrate.resources.limits.memory 32Mi",
	"59 Deployment/shop/api#containers api,bare",
	"62 Deployment/shop/api#containers.api.readinessProbe grpc",
	"64 Deployment/shop/api#containers.api.readinessProbe.port 9090",
	"77 StatefulSet/db#replicas 1",
	"80 StatefulSet/db#containers db",
	"85 StatefulSet/db#containers.db.resources.requests.memory 1Gi",
	"94 Job/seed#containers seed",
	"99 Job/seed#containers.seed.resources.requests.cpu 250m",
	"112 CronJob/ops/nightly#initContainers wait",
	"115 CronJob/ops/nightly#containers report",
	"120 CronJob/ops/nightly#containers.report.resources.limits.cpu 500m",
	"122 CronJob/ops/nightly#containers.report.livenessProbe httpGet",
	"124 CronJob/ops/nightly#containers.report.livenessProbe.port 8081",
}

// k8sSentinel is planted in the fixture's env values, arg values and a
// probe's exec command: no node may carry it (ADR-069 decision 2).
const k8sSentinel = "SENTINEL"

// TestK8sContainerFactsAllThreePodPathsInitContainersDistinct (LB-F451):
// requests, limits, the three probes, the container and init-container lists
// as facts of their own, and replicas, on the workload's entity, from all
// three pod paths; a templated value is no fact; no env value, arg value or
// exec command is read; CronJob's containers are K8sContainer images too.
func TestK8sContainerFactsAllThreePodPathsInitContainersDistinct(t *testing.T) {
	src, err := os.ReadFile(k8sFactsFixture)
	if err != nil {
		t.Fatal(err)
	}
	nodes, err := extractor.NewK8sExtractor().Extract("svc", "deploy/workloads.yaml", src)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, n := range nodes {
		if n.Kind != core.KindK8sFact {
			continue
		}
		if n.FilePath != "deploy/workloads.yaml" || n.Tier != core.TierVerified || n.ToolKind != extractor.ToolKindK8sManifest || n.EndLine != n.Line || n.Package != "workloads" {
			t.Errorf("%+v: plumbing", n)
		}
		got = append(got, fmt.Sprintf("%d %s %s", n.Line, n.Name, n.Signature))
	}
	if !reflect.DeepEqual(got, wantK8sFacts) {
		t.Errorf("facts\n got: %q\nwant: %q", got, wantK8sFacts)
	}

	// The init container and the container of one workload are distinct
	// facts, never one merged list.
	for _, n := range nodes {
		if n.Kind == core.KindK8sFact && strings.HasSuffix(n.Name, "#containers") && strings.Contains(n.Signature, "migrate") {
			t.Errorf("init container merged into the container list: %+v", n)
		}
	}

	// CronJob's job template is a pod path for the image collector too.
	images := map[string]bool{}
	for _, n := range nodes {
		if n.Kind == core.KindK8sContainer {
			images[fmt.Sprintf("%d %s %s", n.Line, n.Name, n.Signature)] = true
		}
	}
	for _, want := range []string{"101 acme/wait:1 wait", "101 acme/report:3 report", "38 acme/migrate:1 migrate", "4 acme/web:1 web"} {
		if !images[want] {
			t.Errorf("K8sContainer %q missing; got %v", want, images)
		}
	}

	for _, n := range nodes {
		if strings.Contains(fmt.Sprintf("%s %s %s %v", n.Name, n.Signature, n.Package, n.Calls), k8sSentinel) {
			t.Errorf("node carries an env value, arg value or exec command: %+v", n)
		}
	}
}

// TestK8sContainerFactsDuplicateKeyLastWins (LB-F451's parity, O1034): a
// manifest that repeats a mapping key is one kubectl and Kubernetes' own
// decode accept, the last value winning (sigs.k8s.io/yaml); yaml.v3's
// struct decode refuses it, which skipped the whole document. Its workload,
// images and facts are read from the node instead, the last key winning.
func TestK8sContainerFactsDuplicateKeyLastWins(t *testing.T) {
	src := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: etcd
spec:
  selector:
    matchLabels: {name: etcd}
  replicas: 1
  replicas: 3
  selector:
    matchLabels: {name: etcd}
  template:
    spec:
      containers:
        - name: member
          image: acme/etcd:2
          resources:
            limits:
              memory: 1Gi
              memory: 2Gi
`
	nodes, err := extractor.NewK8sExtractor().Extract("svc", "etcd.yaml", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, n := range nodes {
		got = append(got, fmt.Sprintf("%d %s %s %s", n.Line, n.Kind, n.Name, n.Signature))
	}
	want := []string{
		"1 K8sWorkload Deployment/etcd apps/v1",
		"1 K8sContainer acme/etcd:2 member",
		"9 K8sFact Deployment/etcd#replicas 3",
		"14 K8sFact Deployment/etcd#containers member",
		"20 K8sFact Deployment/etcd#containers.member.resources.limits.memory 2Gi",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("nodes\n got: %q\nwant: %q", got, want)
	}
}
