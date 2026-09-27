package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/corpus"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// rootedReferencesBackend gives each fake service its own root on disk.
type rootedReferencesBackend struct {
	*referencesFakeBackend
	roots map[string]string
}

func (r *rootedReferencesBackend) ServiceMeta(_ context.Context, svc string) (string, string, bool, error) {
	root, ok := r.roots[svc]
	return root, "", ok, nil
}

func newRootedReferencesSrv(t *testing.T) (*mcp.Server, map[string]string) {
	t.Helper()
	base := t.TempDir()
	roots := map[string]string{"svcA": filepath.Join(base, "a"), "svcB": filepath.Join(base, "b")}
	for _, r := range roots {
		if err := os.MkdirAll(filepath.Join(r, "sub"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	be := referencesBackendForTest()
	return mcp.NewServer(&rootedReferencesBackend{referencesFakeBackend: be, roots: roots}), roots
}

// callRefs calls references under ctx and returns (payload, error object).
func callRefs(t *testing.T, ctx context.Context, srv *mcp.Server, args map[string]any) (map[string]any, map[string]any) {
	t.Helper()
	in, out := &bytes.Buffer{}, &bytes.Buffer{}
	sendLine(t, in, request{JSONRPC: "2.0", ID: 1, Method: "tools/call", Params: map[string]any{"name": "references", "arguments": args}})
	_ = srv.Serve(ctx, in, out)
	var resp map[string]any
	if err := json.NewDecoder(bytes.NewReader(out.Bytes())).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if e, ok := resp["error"].(map[string]any); ok {
		return nil, e
	}
	return extractPayload(t, resp), nil
}

func occServices(p map[string]any) map[string]bool {
	out := map[string]bool{}
	if s, ok := p["occurrence_service"].(string); ok {
		out[s] = true
	}
	occs, _ := p["occurrences"].([]any)
	for _, o := range occs {
		if s, ok := o.(map[string]any)["svc"].(string); ok {
			out[s] = true
		}
	}
	return out
}

// RT-86 (§4.5, Breaking): with no `services`, references searches the
// caller's own service — the one whose root holds the caller's cwd — and
// `services: ["*"]` is the explicit fleet opt-in.
func TestReferences_DefaultsToOwnServiceFleetOptIn(t *testing.T) {
	srv, roots := newRootedReferencesSrv(t)
	ctx := mcp.WithCaller(context.Background(), mcp.CallerInfo{PID: 1, Cwd: filepath.Join(roots["svcB"], "sub")})

	p, errObj := callRefs(t, ctx, srv, map[string]any{"query": "FindMe"})
	if errObj != nil {
		t.Fatalf("references with a resolvable caller refused: %v", errObj)
	}
	if p["scope"] != "caller-service" {
		t.Fatalf("scope = %v, want caller-service", p["scope"])
	}
	if got := occServices(p); len(got) != 1 || !got["svcB"] {
		t.Fatalf("occurrence services = %v, want only the caller's svcB", got)
	}

	p, errObj = callRefs(t, ctx, srv, map[string]any{"query": "FindMe", "services": []any{"*"}})
	if errObj != nil {
		t.Fatalf("fleet references refused: %v", errObj)
	}
	if p["scope"] != "fleet" {
		t.Fatalf("scope = %v, want fleet", p["scope"])
	}
	if got := occServices(p); !got["svcA"] || !got["svcB"] {
		t.Fatalf("fleet occurrence services = %v, want svcA and svcB", got)
	}
	if got := srv.CostClassOfForTest("references", `{"query":"x","services":["*"]}`); got != "fleet" {
		t.Fatalf(`services ["*"] class = %s, want fleet`, got)
	}
}

// RT-159 (§4.5, Breaking): no `services` and no resolvable caller identity
// (an HTTP agent sends no cwd) is a named -32602 listing the indexed
// services and the ["*"] opt-in — never a fleet scan, never an empty result.
func TestReferences_NoIdentityNamesChoices(t *testing.T) {
	srv, _ := newRootedReferencesSrv(t)
	for name, ctx := range map[string]context.Context{
		"no caller":          context.Background(),
		"caller without cwd": mcp.WithCaller(context.Background(), mcp.CallerInfo{PID: 1}),
		"cwd outside roots":  mcp.WithCaller(context.Background(), mcp.CallerInfo{PID: 1, Cwd: t.TempDir()}),
	} {
		p, errObj := callRefs(t, ctx, srv, map[string]any{"query": "FindMe"})
		if errObj == nil {
			t.Fatalf("%s: references answered %v, want a -32602 naming the choices", name, p)
		}
		if code, _ := errObj["code"].(float64); code != -32602 {
			t.Fatalf("%s: code = %v, want -32602", name, errObj["code"])
		}
		msg, _ := errObj["message"].(string)
		for _, want := range []string{"svcA", "svcB", `services: ["*"]`} {
			if !strings.Contains(msg, want) {
				t.Fatalf("%s: message %q does not name %q", name, msg, want)
			}
		}
	}
}

// budgetStoppedBackend reports every scan as stopped at its byte budget.
type budgetStoppedBackend struct {
	*rootedReferencesBackend
	budgets []corpus.Budget
}

func (b *budgetStoppedBackend) ScanSources(ctx context.Context, svc string, bud corpus.Budget, fn func(core.SourceFile) bool) (corpus.Coverage, error) {
	b.budgets = append(b.budgets, bud)
	cov, err := b.rootedReferencesBackend.ScanSources(ctx, svc, bud, fn)
	cov.Total++
	cov.Stopped = corpus.StopBytes
	return cov, err
}

// RT-86: the text scan runs under a byte and wall budget, and a scan the
// budget stopped is a named lower bound; the services it never reached are
// counted, not silently dropped.
func TestReferences_BudgetedScanNamesPartial(t *testing.T) {
	_, roots := newRootedReferencesSrv(t)
	be := &budgetStoppedBackend{rootedReferencesBackend: &rootedReferencesBackend{referencesFakeBackend: referencesBackendForTest(), roots: roots}}
	srv := mcp.NewServer(be)
	p, errObj := callRefs(t, context.Background(), srv, map[string]any{"query": "FindMe", "services": []any{"*"}})
	if errObj != nil {
		t.Fatal(errObj)
	}
	if len(be.budgets) != 1 || be.budgets[0].Bytes <= 0 || be.budgets[0].Wall <= 0 {
		t.Fatalf("scan budgets = %+v, want one bounded scan (the second service is never reached)", be.budgets)
	}
	partial, _ := p["partial"].(string)
	for _, want := range []string{"≥ 2 occurrences", "budget bytes", "1 service(s) not scanned"} {
		if !strings.Contains(partial, want) {
			t.Fatalf("partial = %q, want it to contain %q", partial, want)
		}
	}
}
