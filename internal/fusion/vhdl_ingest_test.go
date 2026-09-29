package fusion_test

// vhdl_ingest_test.go — LB-Q-vhdl: a .vhd or .vhdl file reaches the vhdl dialect through discovery,
// classification and dispatch (both extensions were disclosed as unsupported and fingerprinted by the
// no-extractor path), and what the dialect extracts reaches the store — a package, an entity, its
// architecture and the labelled process inside it, a function and a procedure with a signature, and the
// calls a subprogram or process makes named by callee.

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

var vhdlIngestFiles = map[string]string{
	"rtl/counter_pkg.vhd": `package counter_pkg is
  function next_value(v : natural) return natural;
end package counter_pkg;
`,
	"rtl/counter_pkg_body.vhd": `package body counter_pkg is
  function next_value(v : natural) return natural is
  begin
    return v + 1;
  end function next_value;
end package body counter_pkg;
`,
	"rtl/counter.vhdl": `library ieee;
use ieee.std_logic_1164.all;
use work.counter_pkg.all;

entity counter is
  port (clk : in std_logic; q : out natural);
end entity counter;

architecture rtl of counter is
  signal value : natural := 0;
begin
  tick : process (clk)
  begin
    if rising_edge(clk) then
      value <= next_value(value);
    end if;
  end process tick;
  q <= value;
end architecture rtl;
`,
	"rtl/readme.txt": "not VHDL\n",
}

func TestIngest_VhdlReachesTheVhdlDialect(t *testing.T) {
	dir := t.TempDir()
	for rel, src := range vhdlIngestFiles {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{"rtl/counter_pkg.vhd", "rtl/counter_pkg_body.vhd", "rtl/counter.vhdl"} {
		if lang, ok := connector.UnsupportedLanguageForPath(path); ok {
			t.Fatalf("%s is still disclosed as unsupported (%s)", path, lang)
		}
	}
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "vhdl-svc", Path: dir}
	pl := fusion.New(connector.NewLocalConnector(), st, fusion.Config{Concurrency: 1})
	t.Cleanup(pl.Close)
	ctx := context.Background()
	if _, err := pl.Ingest(ctx, svc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	obs, err := st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]core.Observation{}
	for _, o := range obs {
		got[o.SourceRef+"#"+o.Symbol] = o
	}
	want := map[string]struct {
		kind  core.SourceKind
		sig   string
		calls string
	}{
		"rtl/counter_pkg.vhd#counter_pkg":                 {"VhdlPackage", "", ""},
		"rtl/counter_pkg.vhd#counter_pkg.next_value":      {"VhdlFunctionSpec", "function next_value(v : natural) return natural", ""},
		"rtl/counter_pkg_body.vhd#counter_pkg":            {"VhdlPackageBody", "", ""},
		"rtl/counter_pkg_body.vhd#counter_pkg.next_value": {"VhdlFunction", "function next_value(v : natural) return natural", ""},
		"rtl/counter.vhdl#counter":                        {"VhdlEntity", "", ""},
		"rtl/counter.vhdl#rtl":                            {"VhdlArchitecture", "", ""},
		"rtl/counter.vhdl#rtl.tick":                       {"VhdlProcess", "tick : process (clk)", "next_value,rising_edge"},
	}
	for key, w := range want {
		o, ok := got[key]
		if !ok {
			t.Errorf("no observation %q (got %v)", key, sortedKeys(got))
			continue
		}
		if o.SourceKind != w.kind || o.Claim != w.sig || o.EndLine < o.Line || strings.Join(o.Calls, ",") != w.calls {
			t.Errorf("%s: kind %q claim %q calls %q lines %d-%d, want %s %q %q", key, o.SourceKind, o.Claim, strings.Join(o.Calls, ","), o.Line, o.EndLine, w.kind, w.sig, w.calls)
		}
	}
	for key := range got {
		if strings.HasPrefix(key, "rtl/readme.txt") {
			t.Errorf("unexpected observation %q: a file that is not VHDL", key)
		}
	}
}
