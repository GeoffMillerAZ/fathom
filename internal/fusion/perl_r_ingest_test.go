package fusion_test

// perl_r_ingest_test.go — LB-Q-perl, LB-Q-r: Perl and R files reach their tree-sitter dialects through
// discovery, classification and dispatch. A `.pm` and an R file are claimed by extension; a `.pl` and a `.t`
// only by the Perl dialect's content sniff (Prolog shares `.pl`), and an extensionless script by its perl
// shebang. What only the dialects do (a package, a sub's signature, an R function assignment) proves the
// dispatch is theirs.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

var perlRFiles = map[string]string{
	"lib/Shop/Cart.pm": "package Shop::Cart;\nuse strict;\n\nsub total($self, $tax = 0) {\n  return $self->sum + $tax;\n}\n\n1;\n",
	"bin/report.pl":    "#!/usr/bin/perl\nuse strict;\n\nsub report {\n  print \"ok\\n\";\n}\n",
	"t/cart.t":         "use strict;\nuse Test::More;\n\nsub fixture { return 1 }\n\nok(fixture());\n",
	"scripts/tool":     "#!/usr/bin/env perl\nuse warnings;\nsub run_tool { return 1 }\n",
	"logic/rules.pl":   ":- dynamic fact/1.\nfact(a).\nrule(X) :- fact(X).\n",
	"R/model.R":        "fit <- function(data, k = 3) {\n  helper <- function(x) x + 1\n  helper(k)\n}\n",
	"R/util.r":         "`%+%` <- function(a, b) paste0(a, b)\n",
}

func TestIngest_PerlAndRReachTheirDialects(t *testing.T) {
	dir := t.TempDir()
	for rel, src := range perlRFiles {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "perl-r-svc", Path: dir}
	pl := fusion.New(connector.NewLocalConnector(), st, fusion.Config{Concurrency: 1})
	t.Cleanup(pl.Close)
	if _, err := pl.Ingest(context.Background(), svc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	obs, err := st.QueryObservations(context.Background(), svc.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]core.Observation{}
	for _, o := range obs {
		got[o.SourceRef+"#"+o.Symbol] = o
	}
	want := map[string]struct {
		kind core.SourceKind
		sig  string
	}{
		"lib/Shop/Cart.pm#Shop::Cart": {"PerlPackage", ""},
		"lib/Shop/Cart.pm#total":      {"PerlSub", "sub total($self, $tax = 0)"},
		"bin/report.pl#report":        {"PerlSub", "sub report"},
		"t/cart.t#fixture":            {"PerlSub", "sub fixture"},
		"scripts/tool#run_tool":       {"PerlSub", "sub run_tool"},
		"R/model.R#fit":               {"RFunction", "fit <- function(data, k = 3)"},
		"R/model.R#helper":            {"RFunction", "helper <- function(x)"},
		"R/util.r#`%+%`":              {"RFunction", "`%+%` <- function(a, b)"},
	}
	for key, w := range want {
		o, ok := got[key]
		if !ok {
			t.Errorf("no observation %q (got %v)", key, sortedKeys(got))
			continue
		}
		if o.SourceKind != w.kind || o.Claim != w.sig {
			t.Errorf("%s: kind %q claim %q, want %s %q", key, o.SourceKind, o.Claim, w.kind, w.sig)
		}
	}
	for key := range got {
		if strings.HasPrefix(key, "logic/") {
			t.Errorf("unexpected observation %q: a Prolog `.pl` the Perl sniff must not claim", key)
		}
	}
}
