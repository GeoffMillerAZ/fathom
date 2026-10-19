package callgraph_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestProductReachFollowsAConversionToAnExternalInterface is O1237, read off
// a real ingest of testdata/stdiface. A method that only the standard
// library calls, through an interface it declares (net/http calling an
// http.RoundTripper's RoundTrip), has no caller the index sees. The typed
// pass gives the function that converts a first-party value to such an
// interface a dispatch edge to each of the type's methods in the
// interface's method set, so the product walk reaches the method when
// reached code makes that conversion: an assignment, a call argument (a
// variadic one included), a return, a composite-literal field, a send, an
// explicit conversion and a package variable's initializer. It does not
// reach a method outside the method set, a type reached code holds but
// never converts, a conversion only unreached code, a test or a blank
// declaration makes, or a conversion to an interface the module declares.
func TestProductReachFollowsAConversionToAnExternalInterface(t *testing.T) {
	src, err := filepath.Abs(filepath.Join("testdata", "stdiface"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	st := store.NewMemoryStore()
	p := fusion.New(connector.NewLocalConnector(), st, fusion.Config{})
	t.Cleanup(p.Close)
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "stdmod", Path: src}); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	obs, err := st.QueryObservations(ctx, "stdmod", "")
	if err != nil {
		t.Fatal(err)
	}
	nodes := make([]core.SymbolNode, 0, len(obs))
	for _, o := range obs {
		nodes = append(nodes, o.ToSymbolNode())
	}
	typed, err := st.TypedEdges(ctx, "stdmod")
	if err != nil {
		t.Fatal(err)
	}
	if len(typed) == 0 {
		t.Fatal("the typed pass wrote no edges: the corpus did not type-check")
	}

	// The edge starts where the conversion is: main's init for the
	// transport, the package variable for its initializer (O1124).
	for _, want := range [][2]string{
		{"stdmod::cmd/app/main.init", "stdmod::rt/Transport.Transport.RoundTrip"},
		{"stdmod::rt/rt.Default", "stdmod::rt/VarHandler.VarHandler.ServeHTTP"},
		{"stdmod::rt/rt.NewReader", "stdmod::rt/Reader.Reader.Read"},
	} {
		found := false
		for _, e := range typed {
			if e.FromSymbol == want[0] && e.ToSymbol == want[1] {
				found = true
				if e.Kind != core.EdgeIfaceDispatch {
					t.Errorf("%s -> %s: kind %q, want %q", want[0], want[1], e.Kind, core.EdgeIfaceDispatch)
				}
			}
		}
		if !found {
			t.Errorf("no typed edge %s -> %s", want[0], want[1])
		}
	}

	r := callgraph.ProductEntryReach(callgraph.BuildBoth(nodes, typed))
	for _, c := range []struct {
		symbol string
		wired  bool
		why    string
	}{
		{"stdmod::rt/Transport.Transport.RoundTrip", true, "main's init assigns a *Transport to http.DefaultTransport"},
		{"stdmod::rt/rt.roundTripHelper", true, "the wired RoundTrip calls it"},
		{"stdmod::rt/Handler.Handler.ServeHTTP", true, "main passes a Handler to http.Handle"},
		{"stdmod::rt/Reader.Reader.Read", true, "NewReader, which main calls, returns a *Reader as an io.Reader"},
		{"stdmod::rt/FieldRT.FieldRT.RoundTrip", true, "main sets it as an http.Client's Transport field"},
		{"stdmod::rt/SentErr.SentErr.Error", true, "main sends it on a chan error"},
		{"stdmod::rt/Named.Named.String", true, "main converts it to fmt.Stringer"},
		{"stdmod::rt/JoinErr.JoinErr.Error", true, "main passes it to errors.Join's variadic error"},
		{"stdmod::rt/VarHandler.VarHandler.ServeHTTP", true, "rt's var Default holds it as an http.Handler, and main links rt"},
		{"stdmod::rt/Unconverted.Unconverted.Name", true, "control: main calls it"},
		{"stdmod::rt/Transport.Transport.Close", false, "Close is no method of http.RoundTripper"},
		{"stdmod::rt/rt.closeHelper", false, "only Close calls it"},
		{"stdmod::rt/Unconverted.Unconverted.RoundTrip", false, "main holds an Unconverted and never converts it"},
		{"stdmod::rt/Orphan.Orphan.RoundTrip", false, "only unusedInstall, which nothing calls, converts it"},
		{"stdmod::rt/Asserted.Asserted.RoundTrip", false, "only `var _ http.RoundTripper = (*Asserted)(nil)` converts it"},
		{"stdmod::rt/Blank.Blank.Error", false, "only `var _ error = rt.Blank{}` converts it"},
		{"stdmod::rt/LocalOnly.LocalOnly.RoundTrip", false, "converted only to Local, which the module declares and nothing calls"},
		{"stdmod::rt/TestOnlyRT.TestOnlyRT.RoundTrip", false, "only rt_test.go converts it"},
	} {
		entry, ok := r.ReachedBy(c.symbol)
		if ok != c.wired {
			t.Errorf("%s: product-reached = %v (entry %q), want %v: %s", c.symbol, ok, entry, c.wired, c.why)
		}
	}
}
