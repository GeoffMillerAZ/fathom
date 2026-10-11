package main

// sarifrun_test.go — EVX-2, EVX-3 (docs/plans/portable-receipts.md): every
// SARIF caller renders through verify.RenderSARIFRun with its run's
// provenance. Each caller's output validates against the vendored SARIF
// 2.1.0 schema with no network, names the commit it read, and a finding at
// a line carries its excerpt fingerprint.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	cuejson "cuelang.org/go/encoding/json"
	"cuelang.org/go/encoding/jsonschema"

	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// validateSARIFSchema validates doc against the SARIF 2.1.0 JSON schema
// vendored under internal/verify/testdata/sarif (no network).
func validateSARIFSchema(t *testing.T, doc []byte) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "internal", "verify", "testdata", "sarif", "sarif-schema-2.1.0.json"))
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
		t.Fatal(err)
	}
	schema := ctx.BuildFile(f)
	e, err := cuejson.Extract("doc.json", doc)
	if err != nil {
		t.Fatalf("not JSON: %v\n%s", err, doc)
	}
	if err := schema.Unify(ctx.BuildExpr(e)).Validate(cue.Concrete(true)); err != nil {
		t.Fatalf("SARIF refused by the 2.1.0 schema: %v\n%s", err, doc)
	}
}

// provenanceLog is the slice of a SARIF log the provenance tests read.
type provenanceLog struct {
	Runs []struct {
		AutomationDetails struct {
			ID string `json:"id"`
		} `json:"automationDetails"`
		Properties struct {
			Fathom struct {
				Commit         string `json:"commit"`
				Clean          *bool  `json:"clean"`
				ExtractorEpoch string `json:"extractor_epoch"`
				Producer       struct {
					Name string `json:"name"`
				} `json:"producer"`
			} `json:"fathom"`
		} `json:"properties"`
		VersionControlProvenance []map[string]any `json:"versionControlProvenance"`
		Tool                     struct {
			Driver struct {
				Rules []struct {
					ID   string `json:"id"`
					Help struct {
						Text string `json:"text"`
					} `json:"help"`
				} `json:"rules"`
			} `json:"driver"`
		} `json:"tool"`
		Results []struct {
			RuleID              string            `json:"ruleId"`
			Level               string            `json:"level"`
			PartialFingerprints map[string]string `json:"partialFingerprints"`
			Locations           []struct {
				PhysicalLocation struct {
					ArtifactLocation struct {
						URI string `json:"uri"`
					} `json:"artifactLocation"`
					Region *struct {
						StartLine int `json:"startLine"`
					} `json:"region"`
				} `json:"physicalLocation"`
			} `json:"locations"`
		} `json:"results"`
	} `json:"runs"`
}

// assertProvenance validates doc and checks its run names commit (""
// when the run read none), the extractor epoch and the producer, under
// category; it returns the parsed log.
func assertProvenance(t *testing.T, doc []byte, category, commit string) provenanceLog {
	t.Helper()
	validateSARIFSchema(t, doc)
	var log provenanceLog
	if err := json.Unmarshal(doc, &log); err != nil || len(log.Runs) != 1 {
		t.Fatalf("not a one-run SARIF log (%v): %s", err, doc)
	}
	run := log.Runs[0]
	if run.AutomationDetails.ID != category+"/"+commit {
		t.Errorf("automationDetails.id = %q, want %q", run.AutomationDetails.ID, category+"/"+commit)
	}
	f := run.Properties.Fathom
	if f.Commit != commit || f.Clean == nil || f.ExtractorEpoch != fusion.ExtractorEpoch || f.Producer.Name != "fathom" {
		t.Errorf("run.properties.fathom = %+v, want commit %q, a clean bit, epoch %s, producer fathom", f, commit, fusion.ExtractorEpoch)
	}
	if len(run.VersionControlProvenance) != 0 {
		t.Errorf("versionControlProvenance with no repository URI configured: %v", run.VersionControlProvenance)
	}
	for _, r := range run.Tool.Driver.Rules {
		if r.Help.Text == "" {
			t.Errorf("rule %s has no help text", r.ID)
		}
	}
	return log
}

func lineSHA256(line string) string {
	sum := sha256.Sum256([]byte(line))
	return hex.EncodeToString(sum[:])
}

func gitHead(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// TestVerifySARIFCarriesProvenance is EVX-2's product path: `fathom verify
// --format sarif` in a repository names the commit, and the refused row's
// result carries the plan line's excerpt fingerprint, byte-identical
// across two runs.
func TestVerifySARIFCarriesProvenance(t *testing.T) {
	storePath := seedPlansFixtureStore(t)
	repo := newSelfSkipGitRepo(t)
	row := "- [x] Charge handler {id: SS-1, symbol: Charge, skip_test: true}"
	pf := repo.writePlan("docs/plans/ss.md", "# plan\n\n"+row+"\n")
	repo.commitAll("claim with a bare skip_test")
	head := gitHead(t, repo.root)

	args := []string{"verify", "--service", "payment-service", "--plan", pf, "--store", storePath, "--format", "sarif", "--ledger", filepath.Join(t.TempDir(), "ledger.jsonl")}
	code, out, errBuf := runInDir(t, repo.root, args)
	if code != 1 {
		t.Fatalf("want exit 1 (a refused row), got %d\nstdout=%s\nstderr=%s", code, out, errBuf)
	}
	log := assertProvenance(t, []byte(out), "fathom-verify", head)
	if c := log.Runs[0].Properties.Fathom.Clean; c == nil || !*c {
		t.Error("a committed plan read in a clean work tree must read clean")
	}
	res := log.Runs[0].Results
	if len(res) == 0 {
		t.Fatalf("no results: %s", out)
	}
	if uri := res[0].Locations[0].PhysicalLocation.ArtifactLocation.URI; uri != "docs/plans/ss.md" {
		t.Errorf("uri = %q, want the repo-relative plan path", uri)
	}
	if got := res[0].PartialFingerprints[verify.ExcerptFingerprintKey]; got != lineSHA256(row+"\n") {
		t.Errorf("partialFingerprints[%q] = %q, want the row line's sha256", verify.ExcerptFingerprintKey, got)
	}
	_, out2, _ := runInDir(t, repo.root, args)
	if out2 != out {
		t.Errorf("two runs differ:\n%s\n---\n%s", out, out2)
	}
}
