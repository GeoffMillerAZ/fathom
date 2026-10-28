package fusion

// entityfacts_ingest_test.go — dialect facts through ingest (WAE-4): the
// facts a dialect's doc spec declares (DocResult.Facts) are stored as
// EntityFact observations of the entity's own file, the ADR-064 precedent,
// named `<entity>#<key>`, in the entity's package, at the fact's line;
// an edit or a delete purges them with the file; ReingestFile stores the
// rows ingest stores; a document a Kubernetes object embeds stores its
// facts on the manifest, qualified by the key.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func entityFactsRepo() map[string]string {
	return map[string]string{
		"monitoring/rules.yml": `groups:
  - name: node
    rules:
      - alert: NodeDown
        expr: up{job="node"} == 0
        labels:
          severity: critical
`,
		"monitoring/alertmanager.yml": `route:
  receiver: default
  routes:
    - matchers: ['alertname = NodeDown']
      receiver: pager
      continue: true
receivers:
  - name: default
  - name: pager
`,
		"deploy/alertmanager-secret.yaml": `apiVersion: v1
kind: Secret
metadata:
  name: alertmanager-main
stringData:
  alertmanager.yaml: |-
    route:
      receiver: embedded
    receivers:
      - name: embedded
type: Opaque
`,
	}
}

// entityFactRows is every stored EntityFact as "file line name value",
// sorted, each checked against its entity: the same file, the entity's
// package, tier verified, one line.
func entityFactRows(t *testing.T, st store.Store, svc string) []string {
	t.Helper()
	obs, err := st.QueryObservations(context.Background(), svc, "")
	if err != nil {
		t.Fatal(err)
	}
	pkgOf := map[string]string{}
	for _, o := range obs {
		if o.SourceKind != core.KindEntityFact {
			pkgOf[o.SourceRef+"|"+o.Symbol] = o.Package
		}
	}
	var rows []string
	for _, o := range obs {
		if o.SourceKind != core.KindEntityFact {
			continue
		}
		n := o.ToSymbolNode()
		entity, _, _ := strings.Cut(n.Name, "#")
		if pkg, ok := pkgOf[n.FilePath+"|"+entity]; !ok || pkg != n.Package {
			t.Errorf("fact %s stored off its entity %q (package %q, entity's %q, found %v)", n.Name, entity, n.Package, pkg, ok)
		}
		if n.Tier != core.TierVerified || n.EndLine != n.Line {
			t.Errorf("fact %s: tier %q lines %d-%d, want verified on one line", n.Name, n.Tier, n.Line, n.EndLine)
		}
		rows = append(rows, fmt.Sprintf("%s %d %s %s", n.FilePath, n.Line, n.Name, n.Signature))
	}
	sort.Strings(rows)
	return rows
}

// TestEntityFactsStoredAsObservationsOfOwnFile (WAE-4).
func TestEntityFactsStoredAsObservationsOfOwnFile(t *testing.T) {
	p, st, dir := obsIngest(t, entityFactsRepo(), "facts")
	want := []string{
		"deploy/alertmanager-secret.yaml 8 alertmanager.yaml/route#receiver embedded",
		"monitoring/alertmanager.yml 2 route#receiver default",
		"monitoring/alertmanager.yml 5 route.routes[0]#receiver pager",
		"monitoring/alertmanager.yml 6 route.routes[0]#continue true",
		"monitoring/rules.yml 7 NodeDown#labels.severity critical",
	}
	sort.Strings(want)
	got := entityFactRows(t, st, "facts")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stored entity facts\n got %q\nwant %q", got, want)
	}

	t.Run("reingest parity", func(t *testing.T) {
		reingestParity(t, p, st, dir, "facts", entityFactsRepo())
		if again := entityFactRows(t, st, "facts"); !reflect.DeepEqual(again, want) {
			t.Errorf("ReingestFile stored other facts than ingest\n got %q\nwant %q", again, want)
		}
	})

	t.Run("edit purges", func(t *testing.T) {
		rel := "monitoring/rules.yml"
		edited := "groups:\n  - name: node\n    rules:\n      - alert: NodeDown\n        expr: up{job=\"node\"} == 0\n"
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(edited), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := p.ReingestFile(context.Background(), st, "facts", dir, rel); err != nil {
			t.Fatal(err)
		}
		for _, r := range entityFactRows(t, st, "facts") {
			if strings.HasPrefix(r, rel+" ") {
				t.Errorf("an edit left the removed fact stored: %s", r)
			}
		}
	})

	t.Run("delete purges", func(t *testing.T) {
		rel := "monitoring/alertmanager.yml"
		if err := os.Remove(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			t.Fatal(err)
		}
		if _, err := p.Ingest(context.Background(), core.ServiceRef{ID: "facts", Path: dir}); err != nil {
			t.Fatal(err)
		}
		got := entityFactRows(t, st, "facts")
		want := []string{"deploy/alertmanager-secret.yaml 8 alertmanager.yaml/route#receiver embedded"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("after the delete\n got %q\nwant %q", got, want)
		}
	})
}
