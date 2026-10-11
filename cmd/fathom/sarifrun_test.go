package main

// sarifrun_test.go — EVX-2, EVX-3 (docs/plans/portable-receipts.md): every
// SARIF caller renders through verify.RenderSARIFRun with its run's
// provenance. Each caller's output validates against the vendored SARIF
// 2.1.0 schema with no network, names the commit it read, and a finding at
// a line carries its excerpt fingerprint.

import (
	"bytes"
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
						EndLine   int `json:"endLine"`
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
	t.Setenv("FATHOM_HOME", t.TempDir()) // no user-layer [receipts] reaches the run
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

// fileLinesSHA256 is the excerpt sha256 of lines [start, end] of the file
// at path (end 0 is start): the anchor formula, recomputed independently.
func fileLinesSHA256(t *testing.T, path string, start, end int) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if end < start {
		end = start
	}
	lines := strings.SplitAfter(string(raw), "\n")
	return lineSHA256(strings.Join(lines[start-1:end], ""))
}

// TestObligationsSARIFCarriesProvenance: `fathom obligations --format
// sarif` names the service root's commit, and a served rule located at its
// source lines carries those lines' excerpt fingerprint.
func TestObligationsSARIFCarriesProvenance(t *testing.T) {
	t.Setenv("FATHOM_HOME", t.TempDir()) // no user-layer [receipts] reaches the run
	repo := newSelfSkipGitRepo(t)
	repo.writePlan("CLAUDE.md", "# Rules\n\n- Never call panic in handlers.\n- Always wrap errors with context.\n")
	repo.writePlan("a.go", "package a\n\nfunc A() {}\n")
	repo.commitAll("rules")
	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	var out, errBuf bytes.Buffer
	if code := run([]string{"ingest", "--service", "obl-svc", "--path", repo.root, "--store", storePath}, &out, &errBuf); code != 0 {
		t.Fatalf("ingest exit %d: %s", code, errBuf.String())
	}
	out.Reset()
	errBuf.Reset()
	if code := run([]string{"obligations", "--service", "obl-svc", "--store", storePath, "--file", "a.go", "--format", "sarif"}, &out, &errBuf); code != 0 {
		t.Fatalf("obligations --format sarif exit %d: %s", code, errBuf.String())
	}
	log := assertProvenance(t, out.Bytes(), "fathom-obligations", gitHead(t, repo.root))
	res := log.Runs[0].Results
	if len(res) == 0 || res[0].Locations[0].PhysicalLocation.Region == nil {
		t.Fatalf("want a served rule located at its source line: %s", out.String())
	}
	loc := res[0].Locations[0].PhysicalLocation
	want := fileLinesSHA256(t, filepath.Join(repo.root, loc.ArtifactLocation.URI), loc.Region.StartLine, loc.Region.EndLine)
	if got := res[0].PartialFingerprints[verify.ExcerptFingerprintKey]; got != want {
		t.Errorf("served rule fingerprint = %q, want its source lines' sha256 %s", got, want)
	}
}

// TestRulesReplaySARIFCarriesProvenance: `fathom rules replay --format
// sarif` names the replayed range's end commit. A violation sits at a
// package directory, so it carries no excerpt fingerprint.
func TestRulesReplaySARIFCarriesProvenance(t *testing.T) {
	t.Setenv("FATHOM_HOME", t.TempDir()) // no user-layer [receipts] reaches the run
	r := newReplayRepo(t)
	r.write("go.mod", "module tmpmod\n\ngo 1.24\n")
	r.write(".fathom/guard.cue", `guards: []
layer_rules: [
	{id: "a-not-b", kind: "import_direction", sentence: "a must not import b", from: ["a"], to: ["b"]},
]
`)
	r.write("a/a.go", "package a\n\nfunc F() {}\n")
	r.write("b/b.go", "package b\n\nfunc G() {}\n")
	r.commit("c1: clean")
	r.write("a/a.go", "package a\n\nimport \"tmpmod/b\"\n\nfunc F() { b.G() }\n")
	r.commit("c2: a reaches into b")

	var out, errOut bytes.Buffer
	if code := cmdRulesReplay([]string{"--root", r.dir, "--since", "2", "--format", "sarif"}, &out, &errOut); code != 0 {
		t.Fatalf("rules replay --format sarif exit %d: %s", code, errOut.String())
	}
	log := assertProvenance(t, out.Bytes(), "fathom-rules-replay", gitHead(t, r.dir))
	if len(log.Runs[0].Results) != 1 || log.Runs[0].Results[0].PartialFingerprints != nil {
		t.Errorf("want one package-located violation with no fingerprint: %s", out.String())
	}
}

// TestConfigAuditSARIFCarriesProvenance: `fathom config audit --format
// sarif` names the repository's commit, and a violated concern's rule help
// is the concern's own want.
func TestConfigAuditSARIFCarriesProvenance(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FATHOM_HOME", t.TempDir()) // no user-layer [receipts] reaches the run
	repo := newSelfSkipGitRepo(t)
	writeConcernFixture(t, repo.root)
	repo.commitAll("concerns")
	var out, errB bytes.Buffer
	if code := cmdConfigAudit([]string{"--repo", repo.root, "--format", "sarif"}, &out, &errB); code != 1 {
		t.Fatalf("a violated concern exits 1, got %d: %s %s", code, out.String(), errB.String())
	}
	log := assertProvenance(t, out.Bytes(), "fathom-config-audit", gitHead(t, repo.root))
	var help string
	for _, r := range log.Runs[0].Tool.Driver.Rules {
		if r.ID == "router_thin" {
			help = r.Help.Text
		}
	}
	if !strings.Contains(help, "the root CLAUDE.md stays a router") {
		t.Errorf("router_thin help = %q, want the concern's want", help)
	}
}

// TestIntentAuditSARIFCarriesProvenance: `fathom intent audit --format
// sarif` names the repository's commit.
func TestIntentAuditSARIFCarriesProvenance(t *testing.T) {
	t.Setenv("FATHOM_HOME", t.TempDir()) // no user-layer [receipts] reaches the run
	repo := newSelfSkipGitRepo(t)
	writeCorpusFixture(t, repo.root)
	repo.commitAll("corpus")
	t.Setenv("FATHOM_INTENT_LEDGER", filepath.Join(t.TempDir(), "intent-ledger.jsonl"))
	t.Setenv("FATHOM_VERIFY_LEDGER", filepath.Join(t.TempDir(), "verify-ledger.jsonl"))
	var out, errBuf bytes.Buffer
	code := cmdIntent([]string{"audit", "--root", repo.root, "--url", "http://127.0.0.1:1", "--service", "svc", "--read-only", "--format", "sarif"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("intent audit --format sarif exit %d: %s", code, errBuf.String())
	}
	log := assertProvenance(t, out.Bytes(), "fathom-intent-audit", gitHead(t, repo.root))
	if len(log.Runs[0].Results) == 0 {
		t.Fatalf("an unreachable index must still be named as a not-computed note: %s", out.String())
	}
}

// TestReviewReceiptSARIFCarriesProvenance: `fathom review --receipt
// --format sarif` names the indexed commit.
func TestReviewReceiptSARIFCarriesProvenance(t *testing.T) {
	t.Setenv("FATHOM_HOME", t.TempDir()) // no user-layer [receipts] reaches the run
	repo := newSelfSkipGitRepo(t)
	repo.writePlan("pkg/a.go", reviewFixtureSrc)
	repo.commitAll("a")
	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	var out, errBuf bytes.Buffer
	if code := run([]string{"ingest", "--service", "receipt-prov", "--path", repo.root, "--store", storePath}, &out, &errBuf); code != 0 {
		t.Fatalf("ingest exit %d: %s", code, errBuf.String())
	}
	out.Reset()
	errBuf.Reset()
	code := runReview(
		[]string{"--service", "receipt-prov", "--since", "HEAD~1", "--store", storePath, "--no-persist", "--receipt", "--format", "sarif"},
		fakeGitRunner([]string{"pkg/a.go"}), &out, &errBuf,
	)
	if code != 0 {
		t.Fatalf("review --receipt --format sarif exit %d: %s", code, errBuf.String())
	}
	log := assertProvenance(t, out.Bytes(), "fathom-review-receipt", gitHead(t, repo.root))
	if c := log.Runs[0].Properties.Fathom.Clean; c == nil || !*c {
		t.Errorf("an index of the committed tree reads clean: %s", out.String())
	}
}

// TestVerifySARIFNamesConfiguredRepositoryURI: with `[receipts]
// repository_uri` configured, the run writes versionControlProvenance (the
// URI at the commit), and it still validates against the schema.
func TestVerifySARIFNamesConfiguredRepositoryURI(t *testing.T) {
	t.Setenv("FATHOM_HOME", t.TempDir())
	storePath := seedPlansFixtureStore(t)
	repo := newSelfSkipGitRepo(t)
	pf := repo.writePlan("docs/plans/ss.md", "- [x] Charge handler {id: SS-1, symbol: Charge, skip_test: true}\n")
	repo.writePlan(".fathom/config.toml", "[receipts]\nrepository_uri = \"https://example.invalid/acme/payments.git\"\n")
	repo.commitAll("claim, and the repository's receipts URI")
	head := gitHead(t, repo.root)

	_, out, errBuf := runInDir(t, repo.root, []string{"verify", "--service", "payment-service", "--plan", pf, "--store", storePath, "--format", "sarif", "--ledger", filepath.Join(t.TempDir(), "ledger.jsonl")})
	validateSARIFSchema(t, []byte(out))
	var log provenanceLog
	if err := json.Unmarshal([]byte(out), &log); err != nil || len(log.Runs) != 1 {
		t.Fatalf("not a one-run SARIF log (%v): %s %s", err, out, errBuf)
	}
	vcp := log.Runs[0].VersionControlProvenance
	if len(vcp) != 1 || vcp[0]["repositoryUri"] != "https://example.invalid/acme/payments.git" || vcp[0]["revisionId"] != head {
		t.Fatalf("versionControlProvenance = %v, want the configured URI at %s", vcp, head)
	}
}
