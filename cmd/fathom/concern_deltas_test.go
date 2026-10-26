package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/conserve"
)

const concernBaseManifest = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
  namespace: shop
spec:
  replicas: 3
  template:
    spec:
      containers:
        - name: web
          image: example.invalid/web:v1
          resources:
            requests:
              cpu: 100m
          env:
            - name: PRIVATE
              value: withheld-sentinel
        - name: helper
          image: example.invalid/helper:v1
          resources:
            limits:
              memory: 64Mi
`

func TestCmdConserveConcernDeltas(t *testing.T) {
	for _, mode := range []string{"exact", "differs", "dirty-skew", "uncommitted", "restored-base", "unrelated-dirty", "staged", "deleted", "added", "helm", "write-error", "missing-index", "in-tree-output"} {
		t.Run(mode, func(t *testing.T) {
			repo := newSelfSkipGitRepo(t)
			baseText := concernBaseManifest
			headText := strings.Replace(baseText, "replicas: 3", "replicas: 1", 1)
			headText = strings.Replace(headText, "          env:\n            - name: PRIVATE\n              value: withheld-sentinel\n", "          readinessProbe:\n            httpGet:\n              path: /ready\n              port: 8080\n", 1)
			headText = strings.Replace(headText, "name: helper", "name: assistant", 1)
			if mode == "helm" {
				baseText = "kind: Deployment\nmetadata:\n  name: {{ .Values.name }}\n"
				headText = baseText + "spec: {{ .Values.spec }}\n"
			}
			if mode != "added" {
				repo.writePlan("deploy/api.yaml", baseText)
			}
			repo.writePlan("notes.txt", "base\n")
			repo.commitAll("base")
			base := headSHAOf(t, repo.root)
			if mode == "deleted" {
				if err := os.Remove(filepath.Join(repo.root, "deploy/api.yaml")); err != nil {
					t.Fatal(err)
				}
			} else {
				repo.writePlan("deploy/api.yaml", headText)
			}
			repo.writePlan("notes.txt", "head\n")
			repo.commitAll("head")
			head := headSHAOf(t, repo.root)
			db := filepath.Join(t.TempDir(), "index.sqlite")
			if mode != "missing-index" {
				if code, _, stderr := runInDir(t, repo.root, []string{"ingest", "--path", repo.root, "--service", "svc", "--store", db}); code != 0 {
					t.Fatalf("ingest %d: %s", code, stderr)
				}
			}
			index := head
			switch mode {
			case "differs", "dirty-skew":
				// The clean HEAD differs from the indexed commit. Values still
				// come from the index, with content skew taking precedence.
				if mode == "dirty-skew" {
					repo.writePlan("deploy/api.yaml", strings.Replace(headText, "replicas: 1", "replicas: 9", 1))
				} else {
					// A metadata-only skew can still have exact file bytes.
					repo.writePlan("notes.txt", "later\n")
				}
				repo.commitAll("later")
				head = headSHAOf(t, repo.root)
				if mode == "dirty-skew" {
					repo.writePlan("deploy/api.yaml", baseText)
				}
			case "uncommitted", "staged":
				repo.writePlan("deploy/api.yaml", strings.Replace(headText, "replicas: 1", "replicas: 9", 1))
				if mode == "staged" {
					repo.run("add", ".")
				}
			case "unrelated-dirty":
				repo.writePlan("notes.txt", "dirty\n")
			case "restored-base":
				// The range still touches this file even if the worktree
				// restores the base exactly (git diff Base is then empty).
				repo.writePlan("deploy/api.yaml", baseText)
			}
			out := filepath.Join(t.TempDir(), "deltas.json")
			if mode == "in-tree-output" {
				out = filepath.Join(repo.root, "deltas.json")
			}
			if mode == "write-error" {
				out = filepath.Join(out, "missing", "deltas.json")
			}
			args := []string{"conserve", "--since", base, "--service", "svc", "--store", db, "--concern-deltas", out, "--strict", "--json"}
			code, stdout, stderr := runInDir(t, repo.root, args)
			if mode == "write-error" {
				if code != 2 || !strings.Contains(stderr, "write concern deltas") {
					t.Fatalf("write failure exit %d: %s", code, stderr)
				}
				return
			}
			if mode == "missing-index" {
				if code != 2 || !strings.Contains(stderr, "index metadata") {
					t.Fatalf("missing index exit %d: %s", code, stderr)
				}
				if _, err := os.Stat(out); !os.IsNotExist(err) {
					t.Fatalf("missing index wrote a delta file: %v", err)
				}
				return
			}
			if code != 0 {
				t.Fatalf("strict exit %d: %s", code, stderr)
			}
			data, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			var doc map[string]any
			if err := json.Unmarshal(data, &doc); err != nil {
				t.Fatal(err)
			}
			assertConcernKeys(t, doc, "base,basis,deltas,head,schema,service,skipped")
			if doc["schema"] != "fathom.concern-deltas/v1" || doc["service"] != "svc" || doc["base"] != base || doc["head"] != head {
				t.Fatalf("envelope: %s", data)
			}
			basis := doc["basis"].(map[string]any)
			assertConcernKeys(t, basis, "head_commit,index_commit,state")
			state := mode
			if mode != "differs" && mode != "uncommitted" && mode != "staged" {
				state = "exact"
			}
			if mode == "staged" || mode == "restored-base" || mode == "dirty-skew" {
				state = "uncommitted"
			}
			if basis["state"] != state || basis["index_commit"] != index || basis["head_commit"] != head {
				t.Fatalf("basis: %+v", basis)
			}
			deltas := doc["deltas"].([]any)
			if mode == "helm" {
				if len(deltas) != 0 {
					t.Fatalf("Helm deltas: %+v", deltas)
				}
				skipped := doc["skipped"].([]any)
				if len(skipped) != 4 {
					t.Fatalf("skipped: %+v", skipped)
				}
				var commits []string
				for _, raw := range skipped {
					s := raw.(map[string]any)
					if s["path"] == "notes.txt" {
						continue
					}
					assertConcernKeys(t, s, "commit,path,reason")
					if s["path"] != "deploy/api.yaml" || s["reason"] == "" {
						t.Fatalf("skipped provenance: %+v", s)
					}
					commits = append(commits, s["commit"].(string))
				}
				wantCommits := []string{base, head}
				sort.Strings(wantCommits)
				if !reflect.DeepEqual(commits, wantCommits) {
					t.Fatalf("skipped commits: %v, want %v", commits, wantCommits)
				}
				return
			}
			if len(doc["skipped"].([]any)) != 2 {
				t.Fatalf("skipped: %+v", doc["skipped"])
			}
			var shapes []string
			type fact struct {
				value string
				line  int
			}
			baseFacts := map[string]fact{
				"containers": {"web,helper", 10},
				"containers.helper.resources.limits.memory": {"64Mi", 23},
				"containers.web.resources.requests.cpu":     {"100m", 15},
				"replicas":                                  {"3", 7},
			}
			headFacts := map[string]fact{
				"containers": {"web,assistant", 10},
				"containers.assistant.resources.limits.memory": {"64Mi", 24},
				"containers.web.readinessProbe":                {"httpGet", 16},
				"containers.web.readinessProbe.path":           {"/ready", 18},
				"containers.web.readinessProbe.port":           {"8080", 19},
				"containers.web.resources.requests.cpu":        {"100m", 15},
				"replicas":                                     {"1", 7},
			}
			if mode == "added" {
				baseFacts = map[string]fact{}
			}
			if mode == "deleted" {
				headFacts = map[string]fact{}
			}
			last := ""
			for _, raw := range deltas {
				d := raw.(map[string]any)
				assertConcernKeys(t, d, "after,base_anchor,basis,before,change,container,fact_kind,fact_version,head_anchor,head_commit,key,namespace,workload")
				if d["workload"] != "Deployment/api" || d["namespace"] != "shop" || d["fact_kind"] != "K8sFact" || d["fact_version"] != "v1" || d["head_commit"] != head || !reflect.DeepEqual(d["basis"], basis) {
					t.Fatalf("delta shape: %+v", d)
				}
				key := d["key"].(string)
				if key < last {
					t.Fatalf("unsorted: %s after %s", key, last)
				}
				last = key
				if strings.HasPrefix(key, "containers.") {
					if d["container"] != strings.Split(key, ".")[1] {
						t.Fatalf("container: %+v", d)
					}
				} else if d["container"] != "" {
					t.Fatalf("workload container: %+v", d)
				}
				for _, side := range []struct {
					values map[string]fact
					value  string
					anchor string
				}{{baseFacts, "before", "base_anchor"}, {headFacts, "after", "head_anchor"}} {
					f, exists := side.values[key]
					if !exists {
						if d[side.value] != nil || d[side.anchor] != "" {
							t.Fatalf("absent side: %+v", d)
						}
					} else if d[side.value] != f.value || d[side.anchor] != fmt.Sprintf("deploy/api.yaml:%d", f.line) {
						t.Fatalf("%s: %+v, want %+v", side.value, d, f)
					}
				}
				shapes = append(shapes, key+":"+d["change"].(string))
			}
			if mode == "added" || mode == "deleted" {
				change := "added"
				if mode == "deleted" {
					change = "removed"
				}
				wantCount := len(headFacts)
				if mode == "deleted" {
					wantCount = len(baseFacts)
				}
				if len(deltas) != wantCount {
					t.Fatalf("whole manifest deltas: %+v", deltas)
				}
				for _, raw := range deltas {
					if raw.(map[string]any)["change"] != change {
						t.Fatalf("whole manifest change: %+v", raw)
					}
				}
			} else {
				want := []string{"containers:changed", "containers.assistant.resources.limits.memory:added", "containers.helper.resources.limits.memory:removed", "containers.web.readinessProbe:added", "containers.web.readinessProbe.path:added", "containers.web.readinessProbe.port:added", "replicas:changed"}
				if !reflect.DeepEqual(shapes, want) {
					t.Fatalf("shapes = %v, want %v", shapes, want)
				}
			}
			if strings.Contains(string(data), "withheld-sentinel") || (mode != "added" && mode != "deleted" && strings.Contains(string(data), "containers.web.resources.requests.cpu")) {
				t.Fatalf("unchanged/withheld/non-manifest leaked: %s", data)
			}
			var report map[string]any
			if err := json.Unmarshal([]byte(stdout), &report); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(report["concern_deltas"], doc) {
				t.Fatalf("Report.ConcernDeltas differs from file")
			}
			var names []string
			for _, p := range conserve.Planes() {
				names = append(names, p.Name())
			}
			if !reflect.DeepEqual(names, []string{"absence", "certificates", "governance", "integrity", "resurrection", "surface"}) {
				t.Fatalf("Planes changed: %v", names)
			}
			if code, _, stderr := runInDir(t, repo.root, args); code != 0 {
				t.Fatalf("repeat %d: %s", code, stderr)
			}
			again, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != string(again) {
				t.Fatal("nondeterministic file")
			}
		})
	}
}

func assertConcernKeys(t *testing.T, m map[string]any, want string) {
	t.Helper()
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if strings.Join(keys, ",") != want {
		t.Fatalf("fields %v, want %s", keys, want)
	}
}
