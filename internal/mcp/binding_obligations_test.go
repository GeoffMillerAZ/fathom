package mcp_test

// binding_obligations_test.go — end-to-end proof that the obligations tool
// itself (not just internal/modserve's own unit tests) unions in bound
// platform-module rules when a service's ServiceMeta path has a
// .fathom/modules.cue and the daemon has a moduleBackend wired in
// (docs/design/platform-modules.md §3.3). Exercises the exact wiring
// NewServerWithModules -> registerTools -> registerSemanticTools ->
// modserve.Bind.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/modulehost"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// pathedSemanticBackend layers a real, resolvable ServiceMeta path on top
// of semanticFakeBackend — the obligations handler needs an actual
// filesystem path to look for .fathom/modules.cue under.
type pathedSemanticBackend struct {
	semanticFakeBackend
	path string
}

func (p *pathedSemanticBackend) ServiceMeta(_ context.Context, _ string) (string, string, bool, error) {
	return p.path, "deadbeef", true, nil
}

func writeTestBindings(t *testing.T, root, content string) {
	t.Helper()
	dir := filepath.Join(root, ".fathom")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "modules.cue"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestObligationsTool_BoundModule_UnionsAndTags(t *testing.T) {
	root := t.TempDir()
	writeTestBindings(t, root, `bindings: [{module: "platform-standards"}]`)

	backend := &pathedSemanticBackend{path: root}
	backend.obligations = semantic.Obligations{
		Service: "svc", File: "a.go",
		Rules: []semantic.Obligation{{Rule: "repo-rule", Tier: "verified"}},
	}
	modules := &fakeModuleBackend{
		obligations: []modulehost.FederatedObligation{{
			Module:  "platform-standards",
			Service: "standards-corpus",
			Obligations: semantic.Obligations{
				Rules: []semantic.Obligation{{Rule: "mod-rule", Tier: "doc-claimed", Src: "STANDARDS.md#L1-L2"}},
			},
		}},
	}

	srv := mcp.NewServerWithModules(backend, nil, nil, modules)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "file": "a.go"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	payload := extractPayload(t, resp)
	rules, ok := payload["rules"].([]any)
	if !ok || len(rules) != 2 {
		t.Fatalf("rules = %v, want repo rule + module rule", payload["rules"])
	}
	first := rules[0].(map[string]any)
	if first["rule"] != "repo-rule" {
		t.Errorf("rules[0] = %v, want repo rule to keep priority position", first)
	}
	if _, has := first["module"]; has {
		t.Errorf("repo rule must carry no module tag: %v", first)
	}
	second := rules[1].(map[string]any)
	if second["rule"] != "mod-rule" || second["module"] != "platform-standards" {
		t.Errorf("rules[1] = %v, want the module rule tagged \"platform-standards\"", second)
	}
}

// TestObligationsTool_NoModulesWired_Unchanged proves the composition is a
// complete no-op on a plain NewServer() (no moduleBackend at all) — every
// pre-phase-O deployment and every existing test in this package must keep
// seeing byte-identical behavior.
func TestObligationsTool_NoModulesWired_Unchanged(t *testing.T) {
	root := t.TempDir()
	writeTestBindings(t, root, `bindings: [{module: "platform-standards"}]`)
	backend := &pathedSemanticBackend{path: root}
	backend.obligations = semantic.Obligations{
		Service: "svc", File: "a.go",
		Rules: []semantic.Obligation{{Rule: "repo-rule"}},
	}
	srv := mcp.NewServer(backend)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "file": "a.go"},
		},
	})
	payload := extractPayload(t, resp)
	rules, ok := payload["rules"].([]any)
	if !ok || len(rules) != 1 {
		t.Fatalf("rules = %v, want exactly the repo rule (no module host wired)", payload["rules"])
	}
}

// TestObligationsTool_UnmountedBinding_ExplicitNote proves module_notes
// surfaces on the wire, never silence, when a binding names a module this
// daemon's host doesn't have mounted.
func TestObligationsTool_UnmountedBinding_ExplicitNote(t *testing.T) {
	root := t.TempDir()
	writeTestBindings(t, root, `bindings: [{module: "platform-standards"}]`)
	backend := &pathedSemanticBackend{path: root}
	// No modules mounted at all under this name -- FederatedObligations
	// returns empty results for an unrecognized module in fakeModuleBackend's
	// filter-by-name logic, which is NOT the same as modulehost.Host's real
	// "not mounted" error path. Exercise the real error path directly via a
	// small erroring fake instead.
	modules := &erroringModuleBackend{err: `modulehost: module "platform-standards" is not mounted`}
	srv := mcp.NewServerWithModules(backend, nil, nil, modules)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "file": "a.go"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	payload := extractPayload(t, resp)
	notes, ok := payload["module_notes"].([]any)
	if !ok || len(notes) != 1 {
		t.Fatalf("module_notes = %v, want exactly one note", payload["module_notes"])
	}
	note := notes[0].(map[string]any)
	if note["module"] != "platform-standards" || note["note"] == "" {
		t.Errorf("module_notes[0] = %v", note)
	}
}

// erroringModuleBackend implements moduleBackend with FederatedObligations
// always failing (simulating "not mounted") — the other three methods are
// unused by the obligations path but required by the interface.
type erroringModuleBackend struct {
	err string
}

func (e *erroringModuleBackend) ListModules(context.Context) ([]modulehost.ModuleInfo, error) {
	return nil, nil
}

func (e *erroringModuleBackend) FederatedObligations(context.Context, string, string, int) ([]modulehost.FederatedObligation, error) {
	return nil, errStr(e.err)
}

func (e *erroringModuleBackend) FederatedQuery(context.Context, string, string, int) ([]modulehost.FederatedQueryEntry, error) {
	return nil, nil
}

func (e *erroringModuleBackend) ModuleStaleness(context.Context, string) ([]modulehost.ModuleStaleness, bool, error) {
	return nil, false, nil
}

type errStr string

func (e errStr) Error() string { return string(e) }
