package mcp_test

// secret_surface_test.go — SR-76 (wave h3): no stored secret through any
// MCP read tool. LB-F367 (obs_secrets_test.go) proved no secret-bearing
// value of an Alertmanager configuration or a Grafana provisioning file is
// stored, and that symbol_context withholds the snippet of a definition in
// such a file. This widens the answer half to every tool the widest curated
// surface serves (mcp.ExtendedSurfaceTools): each tool is asked about the
// secret-bearing files and entities, and no answer may carry the sentinel,
// in the clear or base64-encoded at any alignment. A tool that serves
// repository bytes by design (read_source; search's literal grep mode) is a
// declared raw-bytes exemption instead — named here and in
// docs/mcp-tools.md, never filtered. A tool added to the surface later
// without an entry fails TestSecretSurface_EveryToolHasAnEntry.

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/dlog"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// secretToolCase is one tool's entry: the calls aimed at the secret-bearing
// files and entities, or the reason it is a raw-bytes exemption.
type secretToolCase struct {
	calls []map[string]any
	// rawBytes, when set, declares the tool serves repository bytes by
	// design (the file as it is on disk): its answer is not asserted.
	rawBytes string
}

// secretEntities are the entities the sentinel sits in or beside: the
// Alertmanager receiver (and the one embedded in a Kubernetes Secret), the
// Grafana datasource and contact point, the two Secrets and the ConfigMap
// embedding them, and a Secret embedding no dialect's file (its values its
// own).
var secretEntities = []string{"sentinel-receiver", "alertmanager.yaml/sentinel-receiver", "prom-main", "oncall", "am", "grafana-datasources", "ConfigMap/grafana-ds", "Secret/shop/db"}

// secretFiles are the files holding the sentinel.
var secretFiles = []string{
	"monitoring/alertmanager.yml",
	"grafana/provisioning/datasources/ds.yaml",
	"grafana/provisioning/alerting/contact.yaml",
	"deploy/alertmanager-secret.yaml",
	"deploy/grafana-datasources.yaml",
	"deploy/grafana-ds-configmap.yaml",
	"deploy/db-secret.yaml",
}

func perEntity(f func(sym string) map[string]any) []map[string]any {
	var out []map[string]any
	// the sentinel itself is asked in lower case: a tool that echoes its
	// query (a summary, an empty result's reason) then never matches the
	// assertion, while a case-insensitive lookup still finds it if stored
	for _, s := range append(append([]string{}, secretEntities...), strings.ToLower(secretSentinel)) {
		out = append(out, f(s))
	}
	return out
}

func perFile(f func(file string) map[string]any) []map[string]any {
	var out []map[string]any
	for _, p := range secretFiles {
		out = append(out, f(p))
	}
	return out
}

// secretToolCases is the table: one entry per tool on the extended surface.
func secretToolCases() map[string]secretToolCase {
	svc := "sec"
	return map[string]secretToolCase{
		"blast_radius": {calls: append(append(
			[]map[string]any{{"service": svc, "kind": "change", "changed_files": secretFiles}},
			perEntity(func(s string) map[string]any { return map[string]any{"service": svc, "kind": "rename", "symbol": s} })...),
			perEntity(func(s string) map[string]any { return map[string]any{"service": svc, "kind": "delete", "symbol": s} })...)},
		"find_callers": {calls: perEntity(func(s string) map[string]any {
			return map[string]any{"service": svc, "symbol": s, "with_signatures": true, "include_heuristic": true}
		})},
		"find_symbol":   {calls: perEntity(func(s string) map[string]any { return map[string]any{"service": svc, "name": s} })},
		"findings_near": {calls: perFile(func(p string) map[string]any { return map[string]any{"service": svc, "subject": p} })},
		"ingest":        {calls: []map[string]any{{"service": svc}}},
		"list_services": {calls: []map[string]any{{}}},
		"localize": {calls: append(append(perEntity(func(s string) map[string]any {
			return map[string]any{"service": svc, "error": "receiver " + s + " failed to notify"}
		}), perEntity(func(s string) map[string]any {
			return map[string]any{"service": svc, "stack_trace": "at " + s + " (monitoring/alertmanager.yml:3)\nat " + s + " (deploy/alertmanager-secret.yaml:10)"}
		})...), map[string]any{"service": svc, "concern": "alertmanager"})},
		"obligations": {calls: append(perFile(func(p string) map[string]any {
			return map[string]any{"service": svc, "file": p, "include_snippets": true, "layer": "all"}
		}), map[string]any{"service": svc})},
		"outline": {calls: perFile(func(p string) map[string]any { return map[string]any{"service": svc, "file": p} })},
		"pack_context": {calls: append(perEntity(func(s string) map[string]any {
			return map[string]any{"service": svc, "concern": s, "budget": 20000}
		}), map[string]any{"service": svc, "concern": "alertmanager", "budget": 20000}, map[string]any{"service": svc, "concern": "grafana", "budget": 20000})},
		"query_symbols": {calls: perEntity(func(s string) map[string]any {
			return map[string]any{"service": svc, "concern": s, "symbol": s, "keywords": []string{s}, "top": 50}
		})},
		"read_source": {rawBytes: "serves a repository file's lines as they are on disk (the file's own bytes, never stored text); a secret committed to the repository is served as committed"},
		"references": {calls: perEntity(func(s string) map[string]any {
			return map[string]any{"query": s, "services": []string{svc}, "include_heuristic": true, "case_insensitive": true, "max": 200}
		})},
		"repo_map": {calls: []map[string]any{{"service": svc, "budget": 50000}}},
		"review_diff": {calls: []map[string]any{
			{"service": svc, "changed_files": secretFiles},
			{"service": svc, "changed_files": secretFiles, "concern": "alertmanager"},
		}},
		"search": {calls: append(perEntity(func(s string) map[string]any {
			return map[string]any{"service": svc, "kind": "docs", "pattern": s}
		}), map[string]any{"service": svc, "kind": "docs", "pattern": "receiver"}),
			rawBytes: "kind:docs is asserted; the literal grep mode (no kind) reads the repository's files as they are on disk, so it is a raw-bytes reader like read_source",
		},
		"staleness_report": {calls: []map[string]any{{"service": svc}}},
		"status":           {calls: []map[string]any{{"section": "sources", "service": svc}, {"section": "tools"}, {"section": "daemon"}}},
		"symbol_context":   {calls: perEntity(func(s string) map[string]any { return map[string]any{"service": svc, "symbol": s} })},
		"test_impact":      {calls: perEntity(func(s string) map[string]any { return map[string]any{"service": svc, "symbol": s} })},
		"verify_plan":      {calls: []map[string]any{{"service": svc, "plan_markdown": "# p\n\n- [x] S1 receiver {symbol: sentinel-receiver}\n- [x] S2 datasource {symbol: prom-main}\n- [x] S3 secret {symbol: am}\n"}}},
	}
}

// secretForms are the sentinel in the clear and its base64 at each of the
// three alignments it can take inside a longer encoded value (a Secret's
// data: holds a whole file base64-encoded, the sentinel somewhere in it):
// only the characters that depend on the sentinel's bytes alone.
func secretForms() []string {
	forms := []string{secretSentinel}
	for k := 0; k < 3; k++ {
		enc := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("x", k) + secretSentinel))
		start := 0
		if k > 0 {
			start = 4 // the first quantum mixes the prefix in
		}
		end := (k + len(secretSentinel)) / 3 * 4 // drop the last, partial quantum
		forms = append(forms, enc[start:end])
	}
	return forms
}

// callToolRawText is a tools/call's whole JSON-RPC response, unparsed: an
// error message and a result alike are what a client reads.
func callToolRawText(t *testing.T, srv *mcp.Server, name string, args map[string]any) (string, bool) {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": name, "arguments": args}})
	if err != nil {
		t.Fatal(err)
	}
	resp, ok := srv.HandleRequest(context.Background(), raw)
	if !ok {
		t.Fatalf("%s: no response", name)
	}
	var env struct {
		Error  any `json:"error"`
		Result struct {
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	_ = json.Unmarshal(resp, &env)
	return string(resp), env.Error == nil && !env.Result.IsError
}

// TestSecretSurface_EveryToolHasAnEntry: the table covers the extended
// surface exactly, so a tool added to it later must say how it is kept from
// serving a secret.
func TestSecretSurface_EveryToolHasAnEntry(t *testing.T) {
	cases := secretToolCases()
	surface := mcp.ExtendedSurfaceTools()
	if len(surface) < 21 {
		t.Fatalf("the extended surface lists %d tools, want at least the 21 SR-76 covers", len(surface))
	}
	for _, name := range surface {
		c, ok := cases[name]
		if !ok {
			t.Errorf("%s is on the extended surface with no secret-surface entry: aim it at the secret-bearing files and entities, or declare it a raw-bytes reader", name)
			continue
		}
		if len(c.calls) == 0 && c.rawBytes == "" {
			t.Errorf("%s: an entry with no call and no raw-bytes reason", name)
		}
	}
	on := map[string]bool{}
	for _, n := range surface {
		on[n] = true
	}
	for name := range cases {
		if !on[name] {
			t.Errorf("%s has an entry but is not on the extended surface", name)
		}
	}
}

// TestSecretSurface_NoReadToolServesASecret (SR-76): every tool, asked about
// the secret-bearing files and entities, answers without the sentinel.
func TestSecretSurface_NoReadToolServesASecret(t *testing.T) {
	fx := newSecretFixture(t)
	srv := mcp.NewServerWithFleet(fx.c, client.NewFleetAdapter(fx.c, dlog.New(io.Discard, dlog.LevelWarn, time.Now)))
	forms := secretForms()
	for _, f := range forms[1:] {
		if len(f) < 16 {
			t.Fatalf("base64 form %q too short to be the sentinel's", f)
		}
	}
	// the fixture holds every form the assertion looks for: the clear
	// sentinel and the encoded one inside a Secret's data
	var corpus bytes.Buffer
	for _, c := range fx.files {
		corpus.WriteString(c)
	}
	encodedSeen := false
	for _, f := range forms[1:] {
		encodedSeen = encodedSeen || strings.Contains(corpus.String(), f)
	}
	if !strings.Contains(corpus.String(), secretSentinel) || !encodedSeen {
		t.Fatal("the fixture does not hold the sentinel in the clear and base64-encoded: the assertion proves nothing")
	}
	cases := secretToolCases()
	names := make([]string, 0, len(cases))
	for n := range cases {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		answered := 0
		for i, args := range cases[name].calls {
			text, ok := callToolRawText(t, srv, name, args)
			if ok {
				answered++
			}
			for _, f := range forms {
				if strings.Contains(text, f) {
					t.Errorf("%s call %d %v answers with the secret (%q): %.600s", name, i, args, f, text)
					break
				}
			}
		}
		if len(cases[name].calls) > 0 && answered == 0 {
			t.Errorf("%s answered none of its %d calls without an error: its entry proves nothing", name, len(cases[name].calls))
		}
	}
	// the raw-bytes exemptions do serve the file: each exemption is real,
	// not a tool that would have passed anyway
	for _, raw := range []struct {
		tool string
		args map[string]any
	}{
		{"read_source", map[string]any{"service": "sec", "file": "grafana/provisioning/datasources/ds.yaml"}},
		{"search", map[string]any{"service": "sec", "pattern": "basicAuthUser", "context": 3}},
	} {
		if text, _ := callToolRawText(t, srv, raw.tool, raw.args); !strings.Contains(text, secretSentinel) {
			t.Errorf("%s %v did not serve the file's bytes; its raw-bytes exemption is wrong: %.300s", raw.tool, raw.args, text)
		}
	}

	// what is withheld says so, and only there: the Secrets' definitions
	// and an occurrence in a secret-valued file answer with the reason; a
	// rule file holds no secret and keeps its snippet and line text
	for _, sym := range []string{"am", "grafana-datasources", "ConfigMap/grafana-ds", "Secret/shop/db"} {
		p := callTool(t, srv, "symbol_context", map[string]any{"service": "sec", "symbol": sym})
		if p["snippet_withheld"] == nil || p["snippet"] != nil {
			t.Errorf("symbol_context %s (a Kubernetes Secret, or a manifest embedding a secret-valued file) serves no snippet_withheld reason, or a snippet: %v", sym, p)
		}
	}
	refs := callTool(t, srv, "references", map[string]any{"query": "am", "services": []string{"sec"}, "include_heuristic": true})
	if why, _ := refs["text_withheld"].(string); why == "" {
		t.Errorf("references am: no text_withheld reason for its occurrences in the Secrets: %v", refs)
	}
	rule := callTool(t, srv, "references", map[string]any{"query": "SentinelAlert", "services": []string{"sec"}, "include_heuristic": true})
	occs, _ := rule["occurrences"].([]any)
	if len(occs) == 0 || rule["text_withheld"] != nil {
		t.Fatalf("references SentinelAlert: want occurrences with their text and no withheld reason: %v", rule)
	}
	for _, o := range occs {
		if text, _ := o.(map[string]any)["text"].(string); !strings.Contains(text, "SentinelAlert") {
			t.Errorf("references SentinelAlert: an occurrence in the rule file lost its line text: %v", o)
		}
	}
}
