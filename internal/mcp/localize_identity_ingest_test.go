package mcp_test

// localize_identity_ingest_test.go — TL-9 (docs/plans/telemetry-links.md,
// K20): a Compose file's list-form environment names the telemetry identity
// localize resolves, end to end through a real ingest: the docker-compose
// dialect reads `- OTEL_SERVICE_NAME=x` and an OTEL_RESOURCE_ATTRIBUTES
// service.name with the document engine's key-value transform and value
// predicate, and refuses an interpolated value at extraction.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

func TestLocalize_ComposeListFormIdentityResolvesAfterIngest(t *testing.T) {
	root := t.TempDir()
	for rel, body := range map[string]string{
		"go.mod":    "module example.com/ledger\n\ngo 1.22\n",
		"ledger.go": "package ledger\n\nfunc Balance(accountID string) (int64, error) {\n\tif accountID == \"\" {\n\t\treturn 0, nil\n\t}\n\treturn 1, nil\n}\n",
		"deploy/compose.yaml": "services:\n  ledger:\n    image: shop/ledger:1.0\n    environment:\n" +
			"      - OTEL_SERVICE_NAME=ledger-api\n" +
			"      - OTEL_RESOURCE_ATTRIBUTES=deployment.environment=prod,service.name=ledger-core\n" +
			"      - NEW_RELIC_APP_NAME=${NR_APP}\n",
	} {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	sq, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "identity.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sq.Close() })
	c := client.New(connector.NewLocalConnector(), sq)
	t.Cleanup(func() { _ = c.Close() })
	if _, err := c.Ingest(t.Context(), client.ServiceRef{ID: "ledgersvc", Path: root}); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	srv := mcp.NewServer(c)

	for _, name := range []string{"ledger-api", "ledger-core"} {
		got := localizeCall(t, srv, map[string]any{"service": name, "stack_trace": "panic: at ledger.go:5 in Balance"})
		res, ok := got["service_resolved"].(map[string]any)
		if !ok || res["service"] != "ledgersvc" || res["from"] != name {
			t.Errorf("%s: service_resolved = %v, want %s resolved to ledgersvc", name, got["service_resolved"], name)
		}
	}
	syms, err := c.FindSymbol(t.Context(), "ledgersvc", "${NR_APP}")
	if err != nil {
		t.Fatal(err)
	}
	if len(syms) != 0 {
		t.Errorf("an interpolated identity was stored: %v", syms)
	}
}
