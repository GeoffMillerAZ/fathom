package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/buildinfo"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestVersionJSONCarriesCacheKeyComponents (WAE-2): `fathom version --json`
// prints the fathom-side components of an index snapshot's cache key
// (docs/design/index-snapshots.md section 4.1) beside the keys it already
// printed: extractor_epoch, typed_edge_epoch, format_version (the store's
// PRAGMA user_version) and fact_kinds. Each is compared with the constant
// that defines it, never a literal, so an epoch bump never edits this test.
// The existing keys stay as they were (`daemon install --reconcile` reads
// tenancy_contract from the installed binary).
func TestVersionJSONCarriesCacheKeyComponents(t *testing.T) {
	var out bytes.Buffer
	if code := cmdVersion([]string{"--json"}, &out, &bytes.Buffer{}); code != 0 {
		t.Fatalf("version --json exited %d", code)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode %q: %v", out.String(), err)
	}

	str := func(key string) string {
		t.Helper()
		var s string
		if err := json.Unmarshal(got[key], &s); err != nil {
			t.Fatalf("%s = %s, want a string: %v", key, got[key], err)
		}
		return s
	}
	if v := str("extractor_epoch"); v != fusion.ExtractorEpoch {
		t.Errorf("extractor_epoch = %q, want fusion.ExtractorEpoch %q", v, fusion.ExtractorEpoch)
	}
	if v := str("typed_edge_epoch"); v != fusion.TypedEdgeEpoch {
		t.Errorf("typed_edge_epoch = %q, want fusion.TypedEdgeEpoch %q", v, fusion.TypedEdgeEpoch)
	}
	var format int
	if err := json.Unmarshal(got["format_version"], &format); err != nil || format != store.FormatVersion() {
		t.Errorf("format_version = %s (%v), want store.FormatVersion() %d", got["format_version"], err, store.FormatVersion())
	}
	var kinds map[string]any
	if err := json.Unmarshal(got["fact_kinds"], &kinds); err != nil || kinds == nil {
		t.Errorf("fact_kinds = %s (%v), want a JSON object", got["fact_kinds"], err)
	}

	if v := str("version"); v != Version {
		t.Errorf("version = %q, want %q", v, Version)
	}
	var contract int
	if err := json.Unmarshal(got["tenancy_contract"], &contract); err != nil || contract != tenancyContract {
		t.Errorf("tenancy_contract = %s, want %d", got["tenancy_contract"], tenancyContract)
	}
	for _, key := range []string{"revision", "revision_time", "dirty"} {
		if _, ok := got[key]; !ok {
			t.Errorf("version --json lost %q", key)
		}
	}
}

// TestVersionIsTheBuildinfoRelease pins the CLI version to the release the MCP
// initialize reply reports, so the two never drift.
func TestVersionIsTheBuildinfoRelease(t *testing.T) {
	if want := "fathom " + buildinfo.Release; Version != want {
		t.Fatalf("Version = %q, want %q", Version, want)
	}
}

func TestFormatBuildRevisionModuleFallback(t *testing.T) {
	cases := []struct {
		name string
		st   buildinfo.Stamp
		want string
	}{
		{"nothing", buildinfo.Stamp{}, Version + " (revision unknown)"},
		{"module and sum", buildinfo.Stamp{ModuleVersion: "v9.9.9", ModuleSum: "h1:abc="}, Version + " (module v9.9.9, sum h1:abc=)"},
		{"module only", buildinfo.Stamp{ModuleVersion: "v9.9.9"}, Version + " (module v9.9.9)"},
		{"vcs wins", buildinfo.Stamp{Revision: "0123456789abcdef", ModuleVersion: "v9.9.9"}, Version + " @ 0123456789ab"},
	}
	for _, c := range cases {
		if got := formatBuildRevision(c.st); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestVersionJSONCarriesModuleIdentity(t *testing.T) {
	old := versionStamp
	versionStamp = func() buildinfo.Stamp {
		return buildinfo.Stamp{ModuleVersion: "v9.9.9", ModuleSum: "h1:abc="}
	}
	defer func() { versionStamp = old }()
	var out bytes.Buffer
	if code := cmdVersion([]string{"--json"}, &out, &bytes.Buffer{}); code != 0 {
		t.Fatalf("exit %d", code)
	}
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["module_version"] != "v9.9.9" || got["module_sum"] != "h1:abc=" {
		t.Errorf("module_version/module_sum = %v / %v", got["module_version"], got["module_sum"])
	}
	out.Reset()
	cmdVersion([]string{}, &out, &bytes.Buffer{})
	if want := Version + " (module v9.9.9, sum h1:abc=)"; strings.TrimSpace(out.String()) != want {
		t.Errorf("text = %q, want %q", out.String(), want)
	}
}
