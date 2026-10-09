package mcp_test

// localize_identity_test.go — TL-5 (docs/plans/telemetry-links.md,
// ADR-062): a telemetry identity resolves localize's service.

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// identityBackend is a two-service store: Services and ServiceMeta answer
// per service, and FindSymbol answers from each service's own nodes.
type identityBackend struct {
	*localizeFakeBackend
	perService map[string][]core.SymbolNode
}

func (b *identityBackend) Services(context.Context) ([]string, error) {
	var ids []string
	for id := range b.perService {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

func (b *identityBackend) ServiceMeta(_ context.Context, id string) (string, string, bool, error) {
	if _, ok := b.perService[id]; !ok {
		return "", "", false, nil
	}
	return b.svcPath, b.indexCommit, true, nil
}

func (b *identityBackend) FindSymbol(_ context.Context, id, name string) ([]core.SymbolNode, error) {
	var out []core.SymbolNode
	for _, n := range b.perService[id] {
		if n.Name == name {
			out = append(out, n)
		}
	}
	return out, nil
}

func identity(service, name, scheme, file string) core.SymbolNode {
	return core.SymbolNode{Service: service, Package: "deploy", Name: name, Kind: "TelemetryIdentity", FilePath: file, Line: 4, Signature: scheme + "=" + name}
}

// TestLocalize_TelemetryIdentityResolvesToItsService is TL-5: a service
// argument that names no indexed service but exactly one service's
// TelemetryIdentity answers for that service and says so; two services
// are refused with both named; an interpolated value never matches.
func TestLocalize_TelemetryIdentityResolvesToItsService(t *testing.T) {
	base := localizeFixture(t)
	ledger := base.nodes
	be := &identityBackend{localizeFakeBackend: base, perService: map[string][]core.SymbolNode{
		"svc":   append(append([]core.SymbolNode{}, ledger...), identity("svc", "ledger-api", "service.name", "deploy/compose.yaml")),
		"other": {identity("other", "shared-name", "newrelic.app_name", "compose.yaml"), identity("other", "${SVC_NAME}", "service.name", "compose.yaml")},
		"third": {identity("third", "shared-name", "service.name", "compose.yaml")},
	}}
	srv := mcp.NewServer(be)

	got := localizeCall(t, srv, map[string]any{"service": "ledger-api", "stack_trace": "panic: at ledger.go:5 in Balance"})
	res, ok := got["service_resolved"].(map[string]any)
	if !ok || res["service"] != "svc" || res["from"] != "ledger-api" {
		t.Fatalf("service_resolved = %v, want ledger-api resolved to svc", got["service_resolved"])
	}
	if files := localizeCandidateFiles(t, got); !containsStr(files, "ledger.go") {
		t.Errorf("candidates %v, want ledger.go from the resolved service", files)
	}

	resp := roundtrip(t, srv, request{JSONRPC: "2.0", ID: 2, Method: "tools/call", Params: map[string]any{"name": "localize", "arguments": map[string]any{"service": "shared-name", "stack_trace": "panic: at ledger.go:5"}}})
	errObj, _ := resp["error"].(map[string]any)
	msg, _ := errObj["message"].(string)
	if !strings.Contains(msg, "other") || !strings.Contains(msg, "third") {
		t.Errorf("ambiguous identity: error %v, want a refusal naming other and third", resp)
	}

	got = localizeCall(t, srv, map[string]any{"service": "${SVC_NAME}", "stack_trace": "panic: at ledger.go:5"})
	if _, resolved := got["service_resolved"]; resolved {
		t.Errorf("an interpolated identity resolved: %v", got)
	}
	if note, _ := got["note"].(string); !strings.Contains(note, "not indexed") {
		t.Errorf("note = %q, want the not-indexed note", note)
	}
}
