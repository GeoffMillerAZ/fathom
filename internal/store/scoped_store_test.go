package store_test

// scoped_store_test.go — the two rework proofs that live at the store
// boundary:
//
//   - V5: suppression state is re-read per QUERY, so a handle opened before
//     a suppression exists filters on the very next call — the daemon case.
//   - V3: the chokepoint audit derives the constructor list from the store
//     package's own source, so a new exported constructor fails the audit
//     by default instead of quietly returning an unfiltered store.

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/scope"
	"github.com/geoffmilleraz/fathom/internal/store"
)

const (
	scopedSvc  = "svc"
	hiddenRef  = "secrets/token.go"
	visibleRef = "internal/ok.go"
)

func seedTwoRefs(t *testing.T, s store.Store) {
	t.Helper()
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	if err := s.PutObservations(context.Background(), []core.Observation{
		{ServiceID: scopedSvc, SourceRef: hiddenRef, SourceKind: core.KindGoFunc, EvidenceTier: core.TierVerified, Symbol: "Secret", Package: "secrets", Claim: "func Secret()", IngestedAt: now},
		{ServiceID: scopedSvc, SourceRef: visibleRef, SourceKind: core.KindGoFunc, EvidenceTier: core.TierVerified, Symbol: "Ok", Package: "internal", Claim: "func Ok()", IngestedAt: now},
	}); err != nil {
		t.Fatalf("PutObservations: %v", err)
	}
}

func obsRefs(t *testing.T, s store.Store) []string {
	t.Helper()
	obs, err := s.QueryObservations(context.Background(), scopedSvc, "")
	if err != nil {
		t.Fatalf("QueryObservations: %v", err)
	}
	refs := make([]string, 0, len(obs))
	for _, o := range obs {
		refs = append(refs, o.SourceRef)
	}
	sort.Strings(refs)
	return refs
}

// TestSuppressionAppliesToAlreadyOpenHandle is the V5 proof: the previous
// build installed the filter at store OPEN, but the daemon opens its store
// once and holds it for the process lifetime — the feature was inert in the
// only deployment that exists while printing that it took effect. Here the
// suppression is written AFTER the handle is open, and the very next query
// on the SAME handle filters; removing it un-filters the next query too.
func TestSuppressionAppliesToAlreadyOpenHandle(t *testing.T) {
	supPath := filepath.Join(t.TempDir(), "suppressions.jsonl")
	t.Setenv(scope.EnvVar, supPath)

	for _, backend := range []struct {
		name string
		open func(t *testing.T) store.Store
	}{
		{"sqlite", func(t *testing.T) store.Store {
			s, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "s.sqlite"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = s.Close() })
			return s
		}},
		{"memory", func(t *testing.T) store.Store { return store.NewMemoryStore() }},
	} {
		t.Run(backend.name, func(t *testing.T) {
			s := backend.open(t) // opened BEFORE any suppression exists
			seedTwoRefs(t, s)

			if got := obsRefs(t, s); len(got) != 2 {
				t.Fatalf("pre-suppression read = %v, want both refs", got)
			}

			// The daemon case: the file changes while the handle lives.
			id, err := scope.AppendSuppression(supPath, scope.Suppression{
				Service: scopedSvc, Pattern: "secrets/**", Reason: "leaked key material",
				CreatedAt: time.Now().UTC(), CreatedBy: scope.OriginCLI,
			})
			if err != nil {
				t.Fatalf("AppendSuppression: %v", err)
			}

			got := obsRefs(t, s)
			if len(got) != 1 || got[0] != visibleRef {
				t.Fatalf("the NEXT query on the already-open handle served %v — suppression state is not re-read per call (V5)", got)
			}

			// Reversal is instant on the same handle too.
			if err := scope.RemoveSuppression(supPath, id, time.Now().UTC()); err != nil {
				t.Fatalf("RemoveSuppression: %v", err)
			}
			if got := obsRefs(t, s); len(got) != 2 {
				t.Fatalf("post-unsuppress read = %v, want both refs back", got)
			}
		})
	}
}

// TestSuppressedReadDisclosesViaCollector: a filtered read records what it
// hid into the caller's context collector — count, reason, and the id one
// copy-paste away from `fathom scope unsuppress` (SP-3 at the store
// boundary).
func TestSuppressedReadDisclosesViaCollector(t *testing.T) {
	supPath := filepath.Join(t.TempDir(), "suppressions.jsonl")
	t.Setenv(scope.EnvVar, supPath)
	if _, err := scope.AppendSuppression(supPath, scope.Suppression{
		Service: scopedSvc, Pattern: "secrets/**", Reason: "leaked key material",
		CreatedAt: time.Now().UTC(), CreatedBy: scope.OriginCLI,
	}); err != nil {
		t.Fatal(err)
	}

	s := store.NewMemoryStore()
	seedTwoRefs(t, s)

	ctx, coll := scope.WithCollector(context.Background())
	if _, err := s.QueryObservations(ctx, scopedSvc, ""); err != nil {
		t.Fatal(err)
	}
	d := coll.For(scopedSvc)
	if d == nil {
		t.Fatal("a read that dropped a row recorded no disclosure — a silent filter is indistinguishable from a recall bug")
	}
	if d.Count != 1 {
		t.Errorf("disclosure count = %d, want 1", d.Count)
	}
	if !strings.Contains(d.Reason, "leaked key material") || !strings.Contains(d.Reason, "secrets/**") {
		t.Errorf("disclosure reason %q does not carry the reason and pattern", d.Reason)
	}
	if len(d.IDs) != 1 {
		t.Errorf("disclosure ids = %v, want the suppression id", d.IDs)
	}
}

// storeConstructorChecks maps EVERY exported constructor in the store
// package (discovered by parsing the package source — see
// TestEveryStoreConstructorIsScoped) to a behavioral proof that the store
// it returns filters suppressed content. A constructor with no entry here
// FAILS the audit by default — that is the V3 rework: the previous build
// hand-listed two constructors, missed MountReadOnly, and eleven shipped
// surfaces bypassed the filter while the "one chokepoint" test passed.
//
// If a constructor genuinely cannot be behaviorally checked, it goes into
// storeConstructorAllowlist with a written reason instead. That list is
// empty today, and adding to it is a reviewed decision, not a default.
var storeConstructorAllowlist = map[string]string{}

func storeConstructorChecks(t *testing.T) map[string]func(t *testing.T) {
	// seedFile creates a store file with both refs and closes it.
	seedFile := func(t *testing.T) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "audit.sqlite")
		s, err := store.NewSQLiteStore(path)
		if err != nil {
			t.Fatal(err)
		}
		seedTwoRefs(t, s)
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
		return path
	}
	assertFiltered := func(t *testing.T, s store.Store) {
		t.Helper()
		got := obsRefs(t, s)
		if len(got) != 1 || got[0] != visibleRef {
			t.Fatalf("constructor returned a store that serves suppressed content: %v", got)
		}
	}
	return map[string]func(t *testing.T){
		"NewSQLiteStore": func(t *testing.T) {
			s, err := store.NewSQLiteStore(seedFile(t))
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			assertFiltered(t, s)
		},
		"NewMemoryStore": func(t *testing.T) {
			s := store.NewMemoryStore()
			seedTwoRefs(t, s)
			assertFiltered(t, s)
		},
		"Open": func(t *testing.T) {
			s, err := store.Open(seedFile(t))
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			assertFiltered(t, s)
		},
		"BackendForPath": func(t *testing.T) {
			s, err := store.BackendForPath(seedFile(t))
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			assertFiltered(t, s)
		},
		"MountReadOnly": func(t *testing.T) {
			// The constructor the previous audit missed: a read-only mount
			// is a read surface like any other (memory verbs, module host).
			s, err := store.MountReadOnly(seedFile(t), store.LevelL0)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			assertFiltered(t, s)
		},
		"OpenPlatform": func(t *testing.T) {
			// The cross-repo fleet: the previous build's OTHER bypass — a
			// suppressed path was hidden from every single-service query
			// and served by every cross-repo one.
			dir := t.TempDir()
			s, err := store.NewSQLiteStore(filepath.Join(dir, scopedSvc+".sqlite"))
			if err != nil {
				t.Fatal(err)
			}
			seedTwoRefs(t, s)
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			ps, err := store.OpenPlatform(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer ps.Close()
			per, ok := ps.Get(scopedSvc)
			if !ok {
				t.Fatal("platform store did not load the seeded service")
			}
			assertFiltered(t, per)
			all := ps.QueryAll(context.Background(), "")
			for _, obs := range all[scopedSvc] {
				if obs.SourceRef == hiddenRef {
					t.Fatal("PlatformStore.QueryAll served a suppressed ref — the cross-repo bypass is back")
				}
			}
		},
	}
}

// TestEveryStoreConstructorIsScoped is the V3 rework: the audit DERIVES the
// constructor list from the store package's own source. Every exported
// function that can hand out a store must either prove, behaviorally, that
// what it returns filters suppressed content, or sit in the allowlist with
// a written reason. A new constructor fails here by default.
func TestEveryStoreConstructorIsScoped(t *testing.T) {
	supPath := filepath.Join(t.TempDir(), "suppressions.jsonl")
	t.Setenv(scope.EnvVar, supPath)
	if _, err := scope.AppendSuppression(supPath, scope.Suppression{
		// Fleet-wide, so every constructor's fixture is covered without
		// each check naming the service.
		Pattern: "secrets/**", Reason: "audit canary",
		CreatedAt: time.Now().UTC(), CreatedBy: scope.OriginConfig,
	}); err != nil {
		t.Fatal(err)
	}

	constructors := discoverStoreConstructors(t)
	if len(constructors) == 0 {
		t.Fatal("the source scan found no constructors — the audit is broken, not the package")
	}

	checks := storeConstructorChecks(t)
	for _, name := range constructors {
		check, hasCheck := checks[name]
		reason, allowed := storeConstructorAllowlist[name]
		switch {
		case hasCheck && allowed:
			t.Errorf("constructor %s is both checked and allowlisted — pick one", name)
		case hasCheck:
			t.Run(name, check)
		case allowed:
			if strings.TrimSpace(reason) == "" {
				t.Errorf("constructor %s is allowlisted with no reason", name)
			}
		default:
			t.Errorf("exported constructor store.%s returns a store but has neither a behavioral scope check nor an allowlist reason — "+
				"a new constructor must prove its stores filter suppressed content (V3: this exact gap is how eleven surfaces bypassed the chokepoint)", name)
		}
	}

	// Staleness guard: a check or allowlist entry naming a constructor the
	// package no longer exports is a rotted audit.
	known := map[string]bool{}
	for _, name := range constructors {
		known[name] = true
	}
	for name := range checks {
		if !known[name] {
			t.Errorf("storeConstructorChecks names %q, which the source scan no longer finds", name)
		}
	}
	for name := range storeConstructorAllowlist {
		if !known[name] {
			t.Errorf("storeConstructorAllowlist names %q, which the source scan no longer finds", name)
		}
	}
}

// discoverStoreConstructors parses every non-test .go file in this package
// directory and returns the sorted exported functions whose results include
// a store type (Store, *SQLiteStore, *MemoryStore, *PlatformStore). This is
// the list the audit runs against — derived from the source, not from
// anyone's memory of it.
func discoverStoreConstructors(t *testing.T) []string {
	t.Helper()
	storeTypes := map[string]bool{
		"Store": true, "SQLiteStore": true, "MemoryStore": true, "PlatformStore": true,
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var out []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || !fn.Name.IsExported() || fn.Type.Results == nil {
				continue
			}
			for _, res := range fn.Type.Results.List {
				typ := res.Type
				if star, ok := typ.(*ast.StarExpr); ok {
					typ = star.X
				}
				if ident, ok := typ.(*ast.Ident); ok && storeTypes[ident.Name] {
					out = append(out, fn.Name.Name)
					break
				}
			}
		}
	}
	sort.Strings(out)
	return out
}
