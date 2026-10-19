package testevidence

import (
	_ "embed"
	"fmt"
	"strings"
	"time"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/geoffmilleraz/fathom/internal/testrun"
)

//go:embed testkit_result.cue
var testKitResultSchema []byte

type unityEnvelope struct {
	Suite   SuiteRef `json:"suite"`
	Key     string   `json:"key"`
	Verdict string   `json:"verdict"`
	Cases   []Case   `json:"cases"`
	Totals  Totals   `json:"totals"`
	Timing  struct {
		StartedAt  string `json:"startedAt"`
		WallClock  string `json:"wallClock"`
		PerCaseP95 string `json:"perCaseP95,omitempty"`
	} `json:"timing"`
	Env struct {
		ToolchainDigest string `json:"toolchainDigest"`
		RunnerClass     string `json:"runnerClass"`
		Arch            string `json:"arch"`
		OS              string `json:"os"`
		Commit          string `json:"commit"`
		Dirty           *bool  `json:"dirty"`
		Race            *bool  `json:"race"`
	} `json:"env"`
	Artifacts []ArtifactRef `json:"artifacts"`
	Flake     struct {
		MaxRetries      int    `json:"maxRetries"`
		QuarantineAfter int    `json:"quarantineAfter"`
		Budget          string `json:"budget"`
	} `json:"flake"`
	Trace struct {
		TraceID string `json:"traceId"`
	} `json:"trace"`
	Coverage *CoverageReport `json:"coverage,omitempty"`
}

func unityValidateEnvelope(raw []byte, r UnityReceipt, native testrun.NUnitRun) (Run, error) {
	var w unityEnvelope
	if err := unityDecode(raw, &w); err != nil {
		return Run{}, fmt.Errorf("native test-kit envelope JSON: %w", err)
	}
	cctx := cuecontext.New()
	schemaBytes := string(testKitManifestSchema) + "\n" + strings.Replace(string(testKitResultSchema), "package testkit", "", 1)
	schema := cctx.CompileString(schemaBytes)
	if err := schema.Err(); err != nil {
		return Run{}, fmt.Errorf("compile test-kit mirror: %w", err)
	}
	value := schema.LookupPath(cue.ParsePath("#TestResult")).Unify(cctx.CompileBytes(raw))
	if err := value.Validate(cue.Concrete(true)); err != nil {
		return Run{}, fmt.Errorf("native envelope violates #TestResult")
	}
	if w.Suite.Component != r.Project || w.Suite.Name != r.Mode || w.Suite.Class != "integration" || w.Key != "sha256:"+r.Source.BeforeSHA256 || w.Trace.TraceID != r.RunID {
		return Run{}, fmt.Errorf("native suite/key/run identity differs from envelope")
	}
	if w.Env.ToolchainDigest != "sha256:"+r.Editor.SHA256 || w.Env.RunnerClass != "unity" || w.Env.Commit == "" || w.Env.Dirty == nil || w.Env.Race == nil || *w.Env.Race {
		return Run{}, fmt.Errorf("native toolchain/commit/dirty/race facts missing or inconsistent")
	}
	if w.Coverage != nil {
		return Run{}, fmt.Errorf("native XML cannot attest coverage")
	}
	if w.Flake.MaxRetries != 0 || w.Flake.QuarantineAfter != 3 || w.Flake.Budget != "0%" {
		return Run{}, fmt.Errorf("native no-retry policy differs")
	}
	nativeAt, _ := testrun.NUnitTime(native.StartedAt)
	envelopeAt, err := time.Parse(time.RFC3339Nano, w.Timing.StartedAt)
	if err != nil || !nativeAt.Equal(envelopeAt) {
		return Run{}, fmt.Errorf("native envelope start differs")
	}
	nativeWall, _ := testrun.NUnitDuration(native.Duration)
	wall, err := time.ParseDuration(w.Timing.WallClock)
	if err != nil || nativeWall != wall {
		return Run{}, fmt.Errorf("native envelope wall duration differs")
	}
	expectedTotals := Totals{Run: native.Total, Passed: native.Passed, Failed: native.Failed, Skipped: native.Skipped + native.Inconclusive}
	if w.Totals != expectedTotals || len(w.Cases) != len(native.Cases) {
		return Run{}, fmt.Errorf("native envelope totals differ")
	}
	verdict := "error"
	if native.Failed > 0 {
		verdict = "fail"
	} else if native.Total > 0 && native.Passed == native.Total {
		verdict = "pass"
	}
	if w.Verdict != verdict {
		return Run{}, fmt.Errorf("native envelope verdict differs")
	}
	cases := map[string]Case{}
	for _, c := range w.Cases {
		if _, ok := cases[c.ID]; ok {
			return Run{}, fmt.Errorf("native envelope duplicate case")
		}
		cases[c.ID] = c
	}
	for _, c := range native.Cases {
		id := c.Assembly + "::" + c.FullName
		actual, ok := cases[id]
		if !ok {
			return Run{}, fmt.Errorf("native envelope missing case identity")
		}
		status := map[string]string{"Passed": "pass", "Failed": "fail", "Skipped": "skip", "Inconclusive": "skip"}[c.Result]
		duration, err := time.ParseDuration(actual.Duration)
		nativeDuration, _ := testrun.NUnitDuration(c.Duration)
		if !ok || actual.Name != c.FullName || actual.Status != status || actual.Attempts != 1 || err != nil || duration != nativeDuration {
			return Run{}, fmt.Errorf("native envelope case outcome/duration/attempts differs")
		}
		if strings.EqualFold(c.Label, "Retry") || strings.EqualFold(c.Label, "Flaky") {
			return Run{}, fmt.Errorf("native retry lineage unsupported")
		}
	}
	artifacts := map[string]string{}
	for _, a := range w.Artifacts {
		if _, ok := artifacts[a.Name]; ok {
			return Run{}, fmt.Errorf("duplicate native artifact name")
		}
		artifacts[a.Name] = a.Digest
	}
	if len(artifacts) != 2 || artifacts["unity-nunit-xml"] != "sha256:"+r.XML.SHA256 || artifacts["unity-source-inputs"] != "sha256:"+r.Source.BeforeSHA256 {
		return Run{}, fmt.Errorf("native artifact digests differ")
	}
	run, err := decodeOneEnvelope(raw)
	if err != nil {
		return Run{}, err
	}
	run.SourceFile = r.Attestation.Path
	return run, nil
}
