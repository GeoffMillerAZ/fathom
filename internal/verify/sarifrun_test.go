package verify

// sarifrun_test.go — EVX-2 (docs/plans/portable-receipts.md; vision 43
// section 6.6): the shared SARIF emitter carries the run's provenance and
// each finding's excerpt fingerprint, validates against the vendored SARIF
// 2.1.0 schema with no network, and is byte-identical across two runs.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	cuejson "cuelang.org/go/encoding/json"
	"cuelang.org/go/encoding/jsonschema"
)

// sarifSchemaValidator compiles the vendored OASIS SARIF 2.1.0 JSON schema
// (testdata/sarif, see its README) with CUE's JSON Schema decoder: no
// network, no module beyond cuelang.org/go.
func sarifSchemaValidator(t *testing.T) func(doc []byte) error {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "sarif", "sarif-schema-2.1.0.json"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := cuecontext.New()
	expr, err := cuejson.Extract("sarif-schema-2.1.0.json", raw)
	if err != nil {
		t.Fatal(err)
	}
	f, err := jsonschema.Extract(ctx.BuildExpr(expr), &jsonschema.Config{})
	if err != nil {
		t.Fatalf("decode the SARIF schema: %v", err)
	}
	schema := ctx.BuildFile(f)
	if schema.Err() != nil {
		t.Fatalf("build the SARIF schema: %v", schema.Err())
	}
	return func(doc []byte) error {
		e, err := cuejson.Extract("doc.json", doc)
		if err != nil {
			return err
		}
		return schema.Unify(ctx.BuildExpr(e)).Validate(cue.Concrete(true))
	}
}

func testProvenance() RunProvenance {
	return RunProvenance{
		Commit:         "0123456789abcdef0123456789abcdef01234567",
		Clean:          true,
		Producer:       Producer{Name: "fathom", Version: "abc123def456", Build: "2026-10-10T00:00:00Z"},
		ExtractorEpoch: "60",
	}
}

func testRunResults() []SARIFResult {
	return []SARIFResult{
		{RuleID: "test_reaches", Message: "no Test* reaches it", File: "docs/plans/a.md", Line: 7, Fingerprint: strings.Repeat("ab", 32), Help: "A claimed row's test_reaches criterion failed."},
		{RuleID: "symbol_present", Level: "warning", Message: "not found", File: "docs/plans/a.md", Line: 3, EndLine: 5},
		{RuleID: "test_reaches", Level: "note", Message: "again", File: "docs/plans/b.md"},
	}
}

type sarifRunDoc struct {
	Runs []struct {
		Tool struct {
			Driver struct {
				Name    string `json:"name"`
				Version string `json:"version"`
				Rules   []struct {
					ID   string `json:"id"`
					Help *struct {
						Text string `json:"text"`
					} `json:"help"`
				} `json:"rules"`
			} `json:"driver"`
		} `json:"tool"`
		AutomationDetails *struct {
			ID string `json:"id"`
		} `json:"automationDetails"`
		VersionControlProvenance []struct {
			RepositoryURI string `json:"repositoryUri"`
			RevisionID    string `json:"revisionId"`
		} `json:"versionControlProvenance"`
		Properties struct {
			Fathom *struct {
				Commit   string `json:"commit"`
				Clean    *bool  `json:"clean"`
				Producer struct {
					Name    string `json:"name"`
					Version string `json:"version"`
					Build   string `json:"build"`
				} `json:"producer"`
				ExtractorEpoch string `json:"extractor_epoch"`
			} `json:"fathom"`
		} `json:"properties"`
		Results []struct {
			RuleID              string            `json:"ruleId"`
			PartialFingerprints map[string]string `json:"partialFingerprints"`
			Locations           []struct {
				PhysicalLocation struct {
					Region *struct {
						StartLine int `json:"startLine"`
						EndLine   int `json:"endLine"`
					} `json:"region"`
				} `json:"physicalLocation"`
			} `json:"locations"`
		} `json:"results"`
	} `json:"runs"`
}

func TestRenderSARIFRun_ProvenanceFingerprintsHelp(t *testing.T) {
	out := RenderSARIFRun("fathom verify", testProvenance(), testRunResults())
	var doc sarifRunDoc
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if len(doc.Runs) != 1 {
		t.Fatalf("runs = %d", len(doc.Runs))
	}
	run := doc.Runs[0]
	if run.AutomationDetails == nil || run.AutomationDetails.ID != "fathom-verify/0123456789abcdef0123456789abcdef01234567" {
		t.Errorf("automationDetails.id = %+v, want the tool's category and the commit", run.AutomationDetails)
	}
	f := run.Properties.Fathom
	if f == nil {
		t.Fatalf("run.properties.fathom missing: %s", out)
	}
	if f.Commit != testProvenance().Commit || f.Clean == nil || !*f.Clean || f.ExtractorEpoch != "60" ||
		f.Producer.Name != "fathom" || f.Producer.Version != "abc123def456" || f.Producer.Build != "2026-10-10T00:00:00Z" {
		t.Errorf("run.properties.fathom = %+v", *f)
	}
	if run.Tool.Driver.Version != "abc123def456" {
		t.Errorf("driver.version = %q, want the producer's version", run.Tool.Driver.Version)
	}
	for _, r := range run.Tool.Driver.Rules {
		if r.Help == nil || r.Help.Text == "" {
			t.Errorf("rule %s has no help text", r.ID)
		}
		if r.ID == "test_reaches" && r.Help.Text != "A claimed row's test_reaches criterion failed." {
			t.Errorf("rule help = %q, want the caller's", r.Help.Text)
		}
	}
	if len(run.VersionControlProvenance) != 0 {
		t.Errorf("versionControlProvenance written with no repository URI configured: %+v", run.VersionControlProvenance)
	}
	if got := run.Results[0].PartialFingerprints[ExcerptFingerprintKey]; got != strings.Repeat("ab", 32) {
		t.Errorf("partialFingerprints[%q] = %q, want the anchor's excerpt sha256", ExcerptFingerprintKey, got)
	}
	if run.Results[1].PartialFingerprints != nil || run.Results[2].PartialFingerprints != nil {
		t.Error("a result with no anchor digest must carry no partialFingerprints")
	}
	if reg := run.Results[1].Locations[0].PhysicalLocation.Region; reg == nil || reg.StartLine != 3 || reg.EndLine != 5 {
		t.Errorf("region = %+v, want 3-5", reg)
	}

	withURI := testProvenance()
	withURI.RepositoryURI = "https://example.invalid/acme/widgets.git"
	var doc2 sarifRunDoc
	if err := json.Unmarshal(RenderSARIFRun("fathom verify", withURI, testRunResults()), &doc2); err != nil {
		t.Fatal(err)
	}
	vcp := doc2.Runs[0].VersionControlProvenance
	if len(vcp) != 1 || vcp[0].RepositoryURI != withURI.RepositoryURI || vcp[0].RevisionID != withURI.Commit {
		t.Errorf("versionControlProvenance = %+v, want the configured URI at the commit", vcp)
	}
}

func TestRenderSARIFRun_ValidatesAgainstVendoredSchema(t *testing.T) {
	validate := sarifSchemaValidator(t)
	withURI := testProvenance()
	withURI.RepositoryURI = "https://example.invalid/acme/widgets.git"
	dirty := testProvenance()
	dirty.Clean, dirty.Commit = false, ""
	for name, doc := range map[string][]byte{
		"provenance":             RenderSARIFRun("fathom verify", testProvenance(), testRunResults()),
		"repository URI":         RenderSARIFRun("fathom verify", withURI, testRunResults()),
		"no commit":              RenderSARIFRun("fathom config audit", dirty, testRunResults()),
		"no results":             RenderSARIFRun("fathom obligations", testProvenance(), nil),
		"PC-21, provenance-less": renderSARIF("fathom verify", testRunResults(), nil),
	} {
		if err := validate(doc); err != nil {
			t.Errorf("%s: SARIF refused by the 2.1.0 schema: %v\n%s", name, err, doc)
		}
	}
	// The validator bites: a level outside SARIF's vocabulary and a
	// non-string fingerprint are refused.
	bad := strings.Replace(string(RenderSARIFRun("fathom verify", testProvenance(), testRunResults())), `"level": "note"`, `"level": "fatal"`, 1)
	if err := validate([]byte(bad)); err == nil {
		t.Error("the schema accepted level \"fatal\": the validator does not bite")
	}
	bad = strings.Replace(string(RenderSARIFRun("fathom verify", testProvenance(), testRunResults())), `"fathomExcerpt/v1": "`+strings.Repeat("ab", 32)+`"`, `"fathomExcerpt/v1": 7`, 1)
	if err := validate([]byte(bad)); err == nil {
		t.Error("the schema accepted a numeric partial fingerprint: the validator does not bite")
	}
}

func TestRenderSARIFRun_ByteIdenticalAcrossRuns(t *testing.T) {
	a := RenderSARIFRun("fathom verify", testProvenance(), testRunResults())
	b := RenderSARIFRun("fathom verify", testProvenance(), testRunResults())
	if string(a) != string(b) {
		t.Fatalf("two runs differ:\n%s\n---\n%s", a, b)
	}
}
