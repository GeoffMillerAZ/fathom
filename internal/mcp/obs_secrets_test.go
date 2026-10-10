package mcp_test

// obs_secrets_test.go — LB-F367 (wave c3, item obs): no secret-bearing
// field of an Alertmanager configuration or a Grafana provisioning file is
// selected or stored. One sentinel is planted in every field Alertmanager's
// own configuration types declare a secret (listed by reflection over its
// config package: calibration/oracles/alertmanager/secret_fields.json,
// `alertmanager-oracle -list-secret-fields`), in Grafana's secureJsonData,
// basicAuthPassword and password and a contact point's settings, in a file
// of its own and embedded in Kubernetes Secrets; after an ingest into a
// SQLite store, the sentinel occurs in no row of any table and in no MCP
// answer about the receivers, datasources and contact points it sits in.

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

const secretSentinel = "SECRETSENTINEL7c2e91"

// secretTree builds a YAML document holding val at every secret path: keys
// `.`-joined, `[]` a list's one item, `{}` a map's one entry ("k").
type secretTree struct {
	kind  byte // 'm' map, 'l' list, 'v' value
	kids  map[string]*secretTree
	item  *secretTree
	value string
}

func (n *secretTree) child(key string) *secretTree {
	if n.kids == nil {
		n.kind, n.kids = 'm', map[string]*secretTree{}
	}
	c := n.kids[key]
	if c == nil {
		c = &secretTree{}
		n.kids[key] = c
	}
	return c
}

func (n *secretTree) plant(path, val string) {
	segs := strings.Split(path, ".")
	cur := n
	for i, seg := range segs {
		name := strings.TrimRight(seg, "[]{}")
		cur = cur.child(name)
		for _, mark := range seg[len(name):] {
			switch mark {
			case '[':
				cur.kind = 'l'
				if cur.item == nil {
					cur.item = &secretTree{}
				}
				cur = cur.item
			case '{':
				cur = cur.child("k")
			}
		}
		if i == len(segs)-1 {
			cur.kind, cur.value = 'v', val
		}
	}
}

func (n *secretTree) render() any {
	switch n.kind {
	case 'm':
		m := map[string]any{}
		for k, c := range n.kids {
			m[k] = c.render()
		}
		return m
	case 'l':
		return []any{n.item.render()}
	}
	return n.value
}

func indent(s, pad string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i := range lines {
		lines[i] = pad + lines[i]
	}
	return strings.Join(lines, "\n")
}

// TestIngest_AlertmanagerSecretsNeverStored (LB-F367).
func TestIngest_AlertmanagerSecretsNeverStored(t *testing.T) {
	b, err := os.ReadFile("../../calibration/oracles/alertmanager/secret_fields.json")
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	if err := json.Unmarshal(b, &paths); err != nil {
		t.Fatal(err)
	}
	if len(paths) < 30 {
		t.Fatalf("%d secret fields listed, want Alertmanager's (at least 30)", len(paths))
	}
	tree := &secretTree{}
	for _, p := range paths {
		tree.plant(p, secretSentinel)
	}
	tree.child("route").child("receiver").kind, tree.child("route").child("receiver").value = 'v', "sentinel-receiver"
	recv := tree.child("receivers")
	if recv.item == nil {
		t.Fatal("no receivers[] secret field: the reflection listed none")
	}
	recv.item.child("name").kind, recv.item.child("name").value = 'v', "sentinel-receiver"
	am, err := yaml.Marshal(tree.render())
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(am), secretSentinel); n < len(paths) {
		t.Fatalf("the configuration holds %d sentinels, want one per secret field (%d)", n, len(paths))
	}
	grafanaDS := `apiVersion: 1
datasources:
  - name: Prometheus
    uid: prom-main
    type: prometheus
    url: http://prometheus:9090
    basicAuth: true
    basicAuthUser: grafana
    basicAuthPassword: ` + secretSentinel + `
    password: ` + secretSentinel + `
    secureJsonData:
      basicAuthPassword: ` + secretSentinel + `
      httpHeaderValue1: ` + secretSentinel + `
`
	grafanaCP := `apiVersion: 1
contactPoints:
  - orgId: 1
    name: oncall
    receivers:
      - uid: oncall-1
        type: slack
        settings:
          url: https://hooks.example.org/` + secretSentinel + `
          token: ` + secretSentinel + `
      - uid: oncall-2
        type: webhook
        settings:
          url: https://example.org/hook
          password: ` + secretSentinel + `
          authorization_credentials: ` + secretSentinel + `
policies:
  - orgId: 1
    receiver: oncall
`
	files := map[string]string{
		"monitoring/alertmanager.yml": string(am),
		"monitoring/rules.yml": `groups:
  - name: g
    rules:
      - alert: SentinelAlert
        expr: up == 0
`,
		"grafana/provisioning/datasources/ds.yaml":   grafanaDS,
		"grafana/provisioning/alerting/contact.yaml": grafanaCP,
		"deploy/alertmanager-secret.yaml": "apiVersion: v1\nkind: Secret\nmetadata:\n  name: am\nstringData:\n  alertmanager.yaml: |-\n" + indent(string(am), "    ") +
			"\n  slack-token: " + secretSentinel + "\n",
		"deploy/grafana-datasources.yaml": "apiVersion: v1\nkind: Secret\nmetadata:\n  name: grafana-datasources\ndata:\n  datasources.yaml: " +
			base64.StdEncoding.EncodeToString([]byte(grafanaDS)) + "\n  admin-password: " + base64.StdEncoding.EncodeToString([]byte(secretSentinel)) + "\n",
	}
	root := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	dbPath := filepath.Join(t.TempDir(), "secrets.sqlite")
	sq, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sq.Close() })
	c := client.New(connector.NewLocalConnector(), sq)
	t.Cleanup(func() { _ = c.Close() })
	ctx := context.Background()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "sec", Path: root}); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	// the ingest read them: the receiver, the datasource, the contact point
	obs, err := sq.QueryObservations(ctx, "sec", "")
	if err != nil {
		t.Fatal(err)
	}
	read := map[string]bool{}
	for _, o := range obs {
		read[string(o.SourceKind)+" "+o.Symbol] = true
	}
	for _, want := range []string{"AlertmanagerReceiver sentinel-receiver", "AlertmanagerReceiver alertmanager.yaml/sentinel-receiver", "GrafanaDatasource prom-main", "GrafanaContactPoint oncall"} {
		if !read[want] {
			t.Errorf("the ingest did not read %s; read %d observations", want, len(obs))
		}
	}

	// every table, every row, every column
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tables, err := db.Query(`SELECT name FROM sqlite_master WHERE type IN ('table','view') ORDER BY name`)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for tables.Next() {
		var n string
		if err := tables.Scan(&n); err != nil {
			t.Fatal(err)
		}
		names = append(names, n)
	}
	tables.Close()
	if len(names) < 10 {
		t.Fatalf("the store has %d tables: not the SQLite store", len(names))
	}
	scanned := 0
	for _, table := range names {
		rows, err := db.Query(fmt.Sprintf("SELECT * FROM %q", table))
		if err != nil {
			continue // an FTS shadow or a virtual table without a plain read
		}
		cols, _ := rows.Columns()
		for rows.Next() {
			vals := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				t.Fatal(err)
			}
			scanned++
			for i, v := range vals {
				var s string
				switch x := v.(type) {
				case []byte:
					s = string(x)
				default:
					s = fmt.Sprint(x)
				}
				if strings.Contains(s, secretSentinel) || strings.Contains(s, base64.StdEncoding.EncodeToString([]byte(secretSentinel))) {
					t.Errorf("table %s column %s stores a secret: %.200q", table, cols[i], s)
				}
			}
		}
		rows.Close()
	}
	if scanned == 0 {
		t.Fatal("no row read: the dump proves nothing")
	}

	// and no MCP answer about the entities the secrets sit in
	srv := mcp.NewServer(c)
	asked := []string{"sentinel-receiver", "alertmanager.yaml/sentinel-receiver", "prom-main", "oncall", "SentinelAlert"}
	sort.Strings(asked)
	for _, sym := range asked {
		payload := callTool(t, srv, "symbol_context", map[string]any{"service": "sec", "symbol": sym})
		raw, _ := json.Marshal(payload)
		if strings.Contains(string(raw), secretSentinel) {
			t.Errorf("symbol_context %s answers with a secret: %.300s", sym, raw)
		}
		if !strings.Contains(string(raw), sym) {
			t.Errorf("symbol_context %s does not name the symbol: %.300s", sym, raw)
		}
		if sym != "SentinelAlert" && payload["snippet_withheld"] == nil {
			t.Errorf("symbol_context %s serves no snippet_withheld reason: %.300s", sym, raw)
		}
		if sym == "SentinelAlert" && payload["snippet"] == nil {
			t.Errorf("symbol_context %s (a rule file holds no secret) lost its snippet: %.300s", sym, raw)
		}
	}
}
