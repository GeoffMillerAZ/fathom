package fusion_test

// matlab_ingest_test.go — LB-Q-matlab: a `.m` file reaches the matlab dialect through discovery, classification
// and dispatch when its content is MATLAB's, and only then. `.m` is ambiguous (Objective-C, MATLAB), so no
// manifest claims it by extension: the dialect's match.sniff is the only claim, and a file no sniff settles
// (an Objective-C file, a script that opens with code) is fingerprinted and counted as ambiguous_unclassified,
// never guessed. What only the dialect does — a class, its properties and methods, a nested function, a signature
// — proves the dispatch is the dialect's.

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// matlabFiles is a small repository: two MATLAB files the sniff claims (a function file whose first line is a
// function, a classdef file whose first line is a comment), a MATLAB script the sniff also claims by its comment,
// an Objective-C file the sniff must not claim, and a `.m` that opens with code (ambiguous).
var matlabFiles = map[string]string{
	"lab/solve.m": `function [y, n] = solve(x, ...
                        opts)
% SOLVE  Solve it.
    y = helper(x);
    n = numel(y);
    function bump()
        y = y + 1;
    end
    bump();
end

function r = helper(v)
    r = v * 2;
end
`,
	"lab/@Account/Account.m": `% ACCOUNT  A bank account.
classdef Account < handle
    properties
        Balance = 0;
    end
    methods
        function obj = Account()
        end
        function deposit(obj, amount)
            obj.Balance = obj.Balance + amount;
        end
    end
end
`,
	"lab/demo.m": "% a script\nx = 1;\ndisp(x);\n",
	"ios/AppDelegate.m": `#import <Foundation/Foundation.h>
#import "AppDelegate.h"

@implementation AppDelegate
- (void)applicationDidFinishLaunching:(NSNotification *)note {
    NSLog(@"up");
}
@end
`,
	"lab/bare.m": "x = 1;\ny = x + 1;\n",
}

func ingestMatlab(t *testing.T) (map[string]core.Observation, fusion.Report, store.Store, string, core.ServiceRef) {
	t.Helper()
	dir := t.TempDir()
	for rel, src := range matlabFiles {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "matlab-svc", Path: dir}
	pl := fusion.New(connector.NewLocalConnector(), st, fusion.Config{Concurrency: 1})
	t.Cleanup(pl.Close)
	rep, err := pl.Ingest(context.Background(), svc)
	if err != nil {
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
	return got, rep, st, dir, svc
}

func TestIngest_MatlabReachesTheMatlabDialect(t *testing.T) {
	got, rep, _, _, _ := ingestMatlab(t)

	want := map[string]struct {
		kind core.SourceKind
		sig  string
	}{
		"lab/solve.m#solve":                      {"MatlabFunction", "function [y, n] = solve(x, ... opts)"},
		"lab/solve.m#bump":                       {"MatlabFunction", "function bump()"},
		"lab/solve.m#helper":                     {"MatlabFunction", "function r = helper(v)"},
		"lab/@Account/Account.m#Account":         {"MatlabClass", ""},
		"lab/@Account/Account.m#Balance":         {"MatlabProperty", ""},
		"lab/@Account/Account.m#Account.Account": {"MatlabMethod", "function obj = Account()"},
		"lab/@Account/Account.m#Account.deposit": {"MatlabMethod", "function deposit(obj, amount)"},
	}
	for key, w := range want {
		o, ok := got[key]
		if !ok {
			t.Errorf("no observation %q (got %v)", key, sortedKeys(got))
			continue
		}
		if o.SourceKind != w.kind || o.Claim != w.sig || o.EndLine < o.Line {
			t.Errorf("%s: kind %q claim %q lines %d-%d, want %s %q", key, o.SourceKind, o.Claim, o.Line, o.EndLine, w.kind, w.sig)
		}
	}
	for key := range got {
		if strings.HasPrefix(key, "lab/bare.m") || strings.HasPrefix(key, "lab/demo.m") {
			t.Errorf("unexpected observation %q: a file no MATLAB definition lives in, or one the sniff must not claim", key)
		}
		if o := got[key]; strings.HasPrefix(key, "ios/") && strings.HasPrefix(string(o.SourceKind), "Matlab") {
			t.Errorf("%s: the Objective-C file yielded the MATLAB kind %q", key, o.SourceKind)
		}
	}
	// Three files carry a MATLAB claim (function first, comment first twice) and the Objective-C file the
	// Objective-C dialect's (LB-Q-objective-c): four sniff verdicts. The script that opens with code is the one
	// file the sniffs leave unclassified.
	if rep.ClassifiedBySniff != 4 {
		t.Errorf("ClassifiedBySniff = %d, want 4", rep.ClassifiedBySniff)
	}
	if rep.AmbiguousUnclassified != 1 {
		t.Errorf("AmbiguousUnclassified = %d, want 1 (lab/bare.m)", rep.AmbiguousUnclassified)
	}
}

// TestIngest_MatlabSniffLeavesObjectiveCUnclaimed is the classification test the sniff owes Objective-C: an
// Objective-C `.m` (#import, @interface, @implementation) is not claimed by the matlab dialect, so no MATLAB
// symbol is ever read out of it; the Objective-C dialect reads it (LB-Q-objective-c).
func TestIngest_MatlabSniffLeavesObjectiveCUnclaimed(t *testing.T) {
	got, rep, st, _, svc := ingestMatlab(t)
	objc := 0
	for key, o := range got {
		if !strings.HasPrefix(key, "ios/AppDelegate.m") {
			continue
		}
		if strings.HasPrefix(string(o.SourceKind), "Matlab") {
			t.Fatalf("Objective-C file yielded the MATLAB kind: %q %q", key, o.SourceKind)
		}
		if strings.HasPrefix(string(o.SourceKind), "ObjC") {
			objc++
		}
	}
	if objc == 0 {
		t.Fatalf("the Objective-C dialect read nothing out of ios/AppDelegate.m: %v", sortedKeys(got))
	}
	if rep.AmbiguousUnclassified < 1 {
		t.Fatalf("lab/bare.m was not counted as ambiguous_unclassified: %+v", rep)
	}
	// Fingerprinted, so it is not re-sniffed on every ingest.
	if _, found, err := st.Fingerprint(context.Background(), svc.ID, "ios/AppDelegate.m"); err != nil || !found {
		t.Fatalf("Objective-C file not fingerprinted: found=%v err=%v", found, err)
	}
}

// TestReingestFile_MatlabSniffedFile: the one-file refresh dispatches a sniffed `.m` as ingest does.
func TestReingestFile_MatlabSniffedFile(t *testing.T) {
	_, _, st, dir, svc := ingestMatlab(t)
	pl := fusion.New(connector.NewLocalConnector(), st, fusion.Config{Concurrency: 1})
	t.Cleanup(pl.Close)
	edit := "function y = renamed(x)\n    y = x;\nend\n"
	if err := os.WriteFile(filepath.Join(dir, "lab", "solve.m"), []byte(edit), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := pl.ReingestFile(context.Background(), st, svc.ID, dir, "lab/solve.m"); err != nil {
		t.Fatalf("ReingestFile: %v", err)
	}
	obs, err := st.QueryObservations(context.Background(), svc.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, o := range obs {
		if o.SourceRef == "lab/solve.m" {
			names = append(names, o.Symbol)
		}
	}
	if len(names) != 1 || names[0] != "renamed" {
		t.Fatalf("lab/solve.m symbols after the refresh = %v, want [renamed]", names)
	}
}

func sortedKeys(m map[string]core.Observation) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
