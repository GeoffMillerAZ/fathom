package semantic

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestNoClaimEndingPathDeletes is GL-15's gate (docs/plans/governance-learning.md,
// design doc §7 "Validity windows everywhere a fact ends"): every path that
// ends a claim (status superseded or refuted, the two statuses Served()
// refuses) closes the bi-temporal validity window (ValidTo) instead of
// deleting the row. It proves this two ways for the in-package paths —
// closeClaimValidity's own direct effect and Compile()'s real
// reconciliation path (the exact code compile.go's supersede loop runs) —
// and structurally for every other path: the production-source subtests
// below name the review reconcile (CLI and MCP), the findings importer,
// and the refute verdict path, and refuse a claim-ending function that
// flips a status without closing the window, or a claim delete outside
// internal/store. The one deliberate deleter, `fathom purge`
// (internal/store/purge.go), is retention, not a claim-ending path, and is
// out of this row's scope (GL-15's text says so).
func TestNoClaimEndingPathDeletes(t *testing.T) {
	ctx := context.Background()

	t.Run("closeClaimValidity_direct", func(t *testing.T) {
		st := store.NewMemoryStore()
		claim := core.SemanticClaim{
			ServiceID: "svc", ClaimID: "cl-direct", Layer: "obligations", Kind: "obligation",
			Subject: "CLAUDE.md#rule", Rule: "Use task validate before pushing.",
			Scope: []string{"**"}, Tier: core.TierDocClaimed, TierProfile: "engineering",
			Status: core.ClaimFresh, CompiledAt: time.Unix(1, 0).UTC(),
		}
		if err := st.PutClaims(ctx, []core.SemanticClaim{claim}, nil); err != nil {
			t.Fatalf("PutClaims: %v", err)
		}
		before, err := st.ClaimsForService(ctx, "svc", "")
		if err != nil {
			t.Fatalf("ClaimsForService before: %v", err)
		}

		at := time.Unix(2, 0).UTC()
		if err := closeClaimValidity(ctx, st, "svc", "cl-direct", core.ClaimRefuted, at); err != nil {
			t.Fatalf("closeClaimValidity: %v", err)
		}

		after, err := st.ClaimsForService(ctx, "svc", "")
		if err != nil {
			t.Fatalf("ClaimsForService after: %v", err)
		}
		if len(after) != len(before) {
			t.Fatalf("claim count after close = %d, want %d (never deletes)", len(after), len(before))
		}
		if after[0].Status != core.ClaimRefuted {
			t.Errorf("status = %s, want refuted", after[0].Status)
		}
		if !after[0].ValidTo.Equal(at) {
			t.Errorf("valid_to = %v, want %v (window closed)", after[0].ValidTo, at)
		}
	})

	t.Run("compile_supersede_path", func(t *testing.T) {
		st := store.NewMemoryStore()
		c := newCompiler(t, st, llmclient.MockEntry{Response: llmclient.Response{
			Text: `{"claims":[{"rule":"Sort all outputs.","scope":["internal/mcp/**"],"cite_start":4,"cite_end":4}]}`,
		}})
		if _, err := c.Compile(ctx, compileInput(false)); err != nil {
			t.Fatalf("first Compile: %v", err)
		}
		before, err := st.ClaimsForService(ctx, "svc", LayerObligations)
		if err != nil || len(before) != 1 {
			t.Fatalf("claims after first compile = %d, %v, want 1", len(before), err)
		}

		// The doc leaves the corpus selector entirely — compile.go's
		// supersede loop is the ONLY path that retires it (the sweep
		// can't: the doc still exists on disk and its citation still
		// hash-matches).
		c2 := newCompiler(t, st)
		in := compileInput(false)
		in.Docs = nil
		if _, err := c2.Compile(ctx, in); err != nil {
			t.Fatalf("second Compile: %v", err)
		}

		after, err := st.ClaimsForService(ctx, "svc", LayerObligations)
		if err != nil {
			t.Fatalf("ClaimsForService after: %v", err)
		}
		if len(after) != len(before) {
			t.Fatalf("claim count after supersede = %d, want %d (never deletes)", len(after), len(before))
		}
		got := after[0]
		if got.Status != core.ClaimSuperseded {
			t.Fatalf("status = %s, want superseded", got.Status)
		}
		if got.ValidTo.IsZero() {
			t.Errorf("valid_to is zero after supersede, want closed at %v (the fix this test guards)", fixedNow())
		}
		if !got.ValidTo.Equal(fixedNow()) {
			t.Errorf("valid_to = %v, want %v (closed at the injected clock's instant)", got.ValidTo, fixedNow())
		}
	})

	t.Run("every_claim_ending_function_closes_the_window", func(t *testing.T) {
		// A function that flips a claim's status through SetClaimStatus
		// must, in the same function, close the window (SetClaimValidTo
		// or closeClaimValidity) at least once per flip site. The store's own SetClaimStatus
		// implementations and wrappers are exempt by name (they are the
		// primitive); Sweep is exempt because it only ever writes
		// fresh, stale or contradicted, none of which ends a claim
		// (Served() still includes all three, and a swept claim can heal).
		exemptFunc := map[string]string{
			"internal/semantic/sweep.go:Sweep": "writes fresh/stale/contradicted only; none ends a claim",
		}
		want := []string{
			// The paths the pre-push audit named: each must be seen by
			// this scan, so a rename that hides one from it fails loudly.
			"cmd/fathom/claim.go:cmdClaimVerdict",
			"cmd/fathom/review.go:runReviewBody",
			"internal/mcp/review_tool.go:applyAndPersistFindings",
			"internal/findings/external/importer.go:ApplyEvent",
			"pkg/client/semantic.go:ClaimFeedback",
		}
		seen := map[string]bool{}
		for rel, f := range productionGoFiles(t) {
			for _, decl := range f.Decls {
				fd, ok := decl.(*ast.FuncDecl)
				if !ok || fd.Body == nil || fd.Name.Name == "SetClaimStatus" {
					continue
				}
				sets, closes := 0, 0
				ast.Inspect(fd.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					switch fn := call.Fun.(type) {
					case *ast.SelectorExpr:
						switch fn.Sel.Name {
						case "SetClaimStatus":
							sets++
						case "SetClaimValidTo":
							closes++
						}
					case *ast.Ident:
						if fn.Name == "closeClaimValidity" {
							closes++
						}
					}
					return true
				})
				if sets == 0 {
					continue
				}
				key := rel + ":" + fd.Name.Name
				seen[key] = true
				if _, exempt := exemptFunc[key]; exempt {
					continue
				}
				if closes < sets {
					t.Errorf("%s flips a claim's status at %d site(s) but closes its validity window at only %d (SetClaimValidTo or closeClaimValidity): a claim-ending path must set valid_to (GL-15)", key, sets, closes)
				}
			}
		}
		for _, k := range want {
			if !seen[k] {
				t.Errorf("claim-ending path %s was not found by the scan (renamed or moved?): update GL-15's list, never drop the path", k)
			}
		}
	})

	t.Run("no_claim_delete_outside_the_store", func(t *testing.T) {
		// Outside internal/store nothing may delete a claim row: no SQL
		// DELETE naming a claims table, no delete() on a claims map.
		// Inside it, only RemoveService/RenameService (whole-service
		// lifecycle) and Purge (internal/store/purge.go, retention) do.
		var offenders []string
		for rel, f := range productionGoFiles(t) {
			inStore := strings.HasPrefix(rel, "internal/store/")
			ast.Inspect(f, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.BasicLit:
					if x.Kind != token.STRING {
						return true
					}
					lit, err := strconv.Unquote(x.Value)
					if err != nil {
						return true
					}
					up := strings.ToUpper(lit)
					for _, table := range []string{"SEMANTIC_CLAIMS", "CLAIM_EVENTS", "CLAIM_LINKS"} {
						if strings.Contains(up, "DELETE FROM "+table) {
							if !inStore || (table == "SEMANTIC_CLAIMS" && rel != "internal/store/purge.go") {
								offenders = append(offenders, rel+": DELETE FROM "+strings.ToLower(table))
							}
						}
					}
				case *ast.CallExpr:
					if id, ok := x.Fun.(*ast.Ident); ok && id.Name == "delete" && len(x.Args) > 0 {
						if sel, ok := x.Args[0].(*ast.SelectorExpr); ok && sel.Sel.Name == "claims" && rel != "internal/store/memory.go" {
							offenders = append(offenders, rel+": delete(...claims...)")
						}
					}
				}
				return true
			})
		}
		sort.Strings(offenders)
		if len(offenders) > 0 {
			t.Errorf("a claim row is deleted outside the store's whole-service lifecycle and purge (GL-15: a claim ends by closing valid_to, never by deletion):\n  %s",
				strings.Join(offenders, "\n  "))
		}
	})
}

// productionGoFiles parses every non-test Go file under cmd, internal and
// pkg (skipping testdata and vendored third_party trees), keyed by
// repo-relative slash path.
func productionGoFiles(t *testing.T) map[string]*ast.File {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed: cannot locate the repository root")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	fset := token.NewFileSet()
	out := map[string]*ast.File{}
	for _, top := range []string{"cmd", "internal", "pkg"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if n := d.Name(); n == "testdata" || n == "third_party" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			f, perr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if perr != nil {
				return perr
			}
			rel, rerr := filepath.Rel(root, path)
			if rerr != nil {
				return rerr
			}
			out[filepath.ToSlash(rel)] = f
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", top, err)
		}
	}
	if len(out) == 0 {
		t.Fatal("no production Go files found: the scan would prove nothing")
	}
	return out
}
