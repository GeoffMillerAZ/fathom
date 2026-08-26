package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/capability"
)

// ─── BA-9: budget defaults are project-settable ───────────────────────────
//
// The loader half BA-5 shipped without: a `budgets` block in
// .fathom/capabilities.cue (project) and ~/.fathom/capabilities.cue (user),
// read by internal/capability, handed to budgetDefaults — project beats
// user beats the measured built-in (law 13), and a zero-value settings
// falls straight through to measuredDefaults.

func TestBudgetDefaultsLoadFromProjectSettings(t *testing.T) {
	dir := t.TempDir()
	projPath := filepath.Join(dir, "repo", ".fathom", capability.ProjectFileName)
	userPath := filepath.Join(dir, "home", ".fathom", capability.UserFileName)
	for p, body := range map[string]string{
		// The project governs `list`; a capabilities block alongside is
		// left alone (the two blocks share a file, not a loader).
		projPath: "capabilities: {\n\tnarrate: on: true\n}\n\nbudgets: {\n\tlist: 3000\n}\n",
		// The user defaults `list` (overridden) and `map` (not).
		userPath: "budgets: {\n\tlist: 900\n\tmap:  1200\n}\n",
	} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	res, errs := capability.LoadBudgetResolution(userPath, projPath)
	if len(errs) != 0 {
		t.Fatalf("loading budgets: %v", errs)
	}
	s := budgetSettings{User: res.User, Project: res.Project}

	// Values in the file are TOKENS (design §3); budgetDefaults converts to
	// the char units applyBudget/fitToBudget operate in.
	if got := budgetDefaults("query_symbols", s); got != 3000*charsPerToken {
		t.Errorf("project must win over user: query_symbols = %d, want %d", got, 3000*charsPerToken)
	}
	if got := budgetDefaults("references", s); got != 3000*charsPerToken {
		t.Errorf("project must win over user: references = %d, want %d", got, 3000*charsPerToken)
	}
	// User scope defaults a class the project has not spoken on.
	if got, ok := budgetDefaultTokens("repo_map", s); !ok || got != 1200 {
		t.Errorf("user must win over built-in: repo_map = %d/%v, want 1200/true", got, ok)
	}
	// Neither scope spoke: the measured built-in.
	if got, ok := budgetDefaultTokens("pack_context", s); !ok || got != measuredDefaults[classComposite] {
		t.Errorf("built-in must stand when no scope spoke: pack_context = %d/%v, want %d/true", got, ok, measuredDefaults[classComposite])
	}
	// A tool with no declared class is still unsized, whatever the file says.
	if got := budgetDefaults("find_symbol", s); got != 0 {
		t.Errorf("find_symbol has no class and must resolve to 0, got %d", got)
	}

	// Zero-value settings — no file at either scope — fall through to the
	// measured defaults, so every truncating tool keeps working unconfigured.
	if got := budgetDefaults("query_symbols", budgetSettings{}); got != measuredDefaults[classList]*charsPerToken {
		t.Errorf("zero-value settings = %d, want measured %d", got, measuredDefaults[classList]*charsPerToken)
	}
	absent, aerrs := capability.LoadBudgetResolution(filepath.Join(dir, "no-user.cue"), filepath.Join(dir, "no-proj.cue"))
	if len(aerrs) != 0 {
		t.Fatalf("absent files are not errors: %v", aerrs)
	}
	zero := budgetSettings{User: absent.User, Project: absent.Project}
	if got := budgetDefaults("query_symbols", zero); got != measuredDefaults[classList]*charsPerToken {
		t.Errorf("absent files must resolve to the measured default, got %d", got)
	}

	// The daemon wiring: a Server handed the resolution serves the tools
	// from it — query_symbols under a project `list` too small for the
	// fixture elides, where the measured default would not.
	srv := elisionTestServer()
	srv.SetBudgets(capability.BudgetResolution{Project: capability.Budgets{"list": 100}})
	resp := callTool(t, srv, "query_symbols", map[string]any{"service": "svc", "concern": "handler", "top": 60})
	if resp["elided"] == nil {
		t.Fatalf("a project list budget of 100 tokens must elide the 60-node fixture; got no elided block: %v", resp)
	}
	srv = elisionTestServer()
	srv.SetBudgets(capability.BudgetResolution{Project: capability.Budgets{"list": 100000}})
	resp = callTool(t, srv, "query_symbols", map[string]any{"service": "svc", "concern": "handler", "top": 60})
	if resp["elided"] != nil {
		t.Fatalf("a generous project list budget must not elide the fixture: %v", resp["elided"])
	}
}
