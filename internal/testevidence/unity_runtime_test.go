package testevidence

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

func unityTestStore(t *testing.T, backend string) store.Store {
	t.Helper()
	if backend == "memory" {
		return store.NewMemoryStore()
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "native.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}
func seedUnitySymbol(t *testing.T, st store.Store, f *unityFixture, s core.SymbolNode) {
	t.Helper()
	source, readErr := os.ReadFile(filepath.Join(f.root, s.FilePath))
	if readErr != nil {
		t.Fatal(readErr)
	}
	err := st.PutObservations(context.Background(), []core.Observation{{ServiceID: s.Service, SourceRef: s.FilePath, SourceKind: s.Kind, Symbol: s.Name, Package: s.Package, Line: s.Line, EndLine: s.EndLine, Claim: s.Signature, EvidenceTier: core.TierVerified, ContentHash: fusion.FingerprintHash(source)}})
	if err != nil {
		t.Fatal(err)
	}
}
func ingestUnityFixture(t *testing.T, st store.Store, f *unityFixture, s core.SymbolNode) Run {
	t.Helper()
	e, err := f.load()
	if err != nil {
		t.Fatal(err)
	}
	r, err := UnityEvidenceRun(e, []core.SymbolNode{s})
	if err != nil {
		t.Fatal(err)
	}
	report, err := IngestRuns(context.Background(), st, "svc", "test-watermark", []Run{r})
	if err != nil {
		t.Fatal(err)
	}
	if report.CasesJoined != 2 {
		t.Fatalf("joined=%d", report.CasesJoined)
	}
	return r
}
func nativeStatus(t *testing.T, st store.Store, key string) verify.AttestedRun {
	t.Helper()
	r, err := LoadRuntimeRuns(context.Background(), st, "svc", nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.NewestAttestedRuns([]string{key})
	if err != nil {
		t.Fatal(err)
	}
	return got[key]
}
func changeUnityNativeXML(t *testing.T, f *unityFixture, raw string, status, verdict string, totals Totals) {
	t.Helper()
	if err := os.WriteFile(f.xml, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	f.r.XML.SHA256 = unityDigest([]byte(raw))
	cases := f.envelope["cases"].([]Case)
	cases[1].Status = status
	f.envelope["totals"] = totals
	f.envelope["verdict"] = verdict
	f.envelope["artifacts"] = []ArtifactRef{{"unity-nunit-xml", "sha256:" + f.r.XML.SHA256}, {"unity-source-inputs", "sha256:" + f.r.Source.BeforeSHA256}}
	f.saveEnvelope(t)
	f.save(t)
}
func TestUnityNUnitRuntimeRoundtripAggregatesVariantsAndRevalidates(t *testing.T) {
	for _, backend := range []string{"memory", "sqlite"} {
		t.Run(backend, func(t *testing.T) {
			f := newUnityFixture(t)
			s := addUnityTypedFixture(t, f)
			st := unityTestStore(t, backend)
			seedUnitySymbol(t, st, f, s)
			run := ingestUnityFixture(t, st, f, s)
			if got := nativeStatus(t, st, s.Key()); got.Status != "pass" || !got.Dirty || got.Run != run.Digest {
				t.Fatalf("current runtime %+v", got)
			}
			if got := nativeStatus(t, st, "Workshop.BowlTests.Fits"); got.Status != "pass" {
				t.Fatalf("qualified identity %+v", got)
			}
			ingestUnityFixture(t, st, f, s) // exact replay is idempotent
			history, err := st.TestCaseRunsForSymbol(context.Background(), "svc", s.Key(), 0)
			if err != nil || len(history) != 2 || history[0].NativeJSON == "" {
				t.Fatalf("native roundtrip %+v %v", history, err)
			}
			// The root-level XML still says Passed, but no old result survives a newer
			// started receipt. No ingestion of a fake incomplete run is required.
			f.r.Status = "started"
			f.r.RunID = "next-attempt"
			f.save(t)
			if got := nativeStatus(t, st, s.Key()); got.Status != "unknown" || !strings.Contains(got.UnknownReason, "active receipt changed") {
				t.Fatalf("old pass survived started: %+v", got)
			}
			f.r.Status = "complete"
			f.r.RunID = "synthetic-run-1"
			f.save(t)
			if got := nativeStatus(t, st, s.Key()); got.Status != "pass" {
				t.Fatalf("exact active replay %+v", got)
			}
			if err := os.WriteFile(filepath.Join(f.root, f.r.Project, "Assets/BowlTests.cs"), []byte("changed source"), 0600); err != nil {
				t.Fatal(err)
			}
			if got := nativeStatus(t, st, s.Key()); got.Status != "unknown" || !strings.Contains(got.UnknownReason, "source input changed") {
				t.Fatalf("old pass survived source change: %+v", got)
			}
			if _, err := IngestRuns(context.Background(), st, "svc", "other", []Run{run}); err == nil {
				t.Fatal("stale replay ingested")
			}
		})
	}
}
func TestUnityNUnitRuntimeNeverChoosesPassingParameterizedLeaf(t *testing.T) {
	for _, outcome := range []string{"Failed", "Skipped", "Inconclusive", "Missing"} {
		for _, backend := range []string{"memory", "sqlite"} {
			t.Run(backend+"/"+outcome, func(t *testing.T) {
				f := newUnityFixture(t)
				s := addUnityTypedFixture(t, f)
				raw := nativeFixtureXML
				status := "skip"
				verdict := "error"
				want := "unknown"
				totals := Totals{Run: 2, Passed: 1, Skipped: 1}
				switch outcome {
				case "Failed":
					raw = strings.Replace(raw, `result="Passed"`, `result="Failed"`, 1)
					raw = strings.Replace(raw, `passed="2" failed="0"`, `passed="1" failed="1"`, 1)
					status = "fail"
					verdict = "fail"
					want = "fail"
					totals = Totals{Run: 2, Passed: 1, Failed: 1}
				case "Skipped":
					raw = strings.Replace(raw, `passed="2"`, `passed="1"`, 1)
					raw = strings.Replace(raw, `skipped="0"`, `skipped="1"`, 1)
					raw = strings.Replace(raw, `result="Passed"`, `result="Skipped"`, 1)
				case "Inconclusive":
					raw = strings.Replace(raw, `passed="2"`, `passed="1"`, 1)
					raw = strings.Replace(raw, `inconclusive="0"`, `inconclusive="1"`, 1)
					raw = strings.Replace(raw, `result="Passed"`, `result="Inconclusive"`, 1)
				case "Missing":
					raw = strings.Replace(raw, `testcasecount="2" total="2" passed="2"`, `testcasecount="2" total="1" passed="1"`, 1)
					start := strings.Index(raw, `<test-case id="2"`)
					end := strings.Index(raw[start:], "/>") + start + 2
					raw = raw[:start] + raw[end:]
					cases := f.envelope["cases"].([]Case)
					f.envelope["cases"] = cases[:1]
					status = "pass"
					verdict = "pass"
					totals = Totals{Run: 1, Passed: 1}
				}
				if outcome != "Missing" {
					old := `methodname="Fits" result="Passed" duration="0.2"`
					raw = strings.Replace(raw, old, `methodname="Fits" result="`+outcome+`" duration="0.2"`, 1)
					changeUnityNativeXML(t, f, raw, status, verdict, totals)
				} else {
					if err := os.WriteFile(f.xml, []byte(raw), 0600); err != nil {
						t.Fatal(err)
					}
					f.r.XML.SHA256 = unityDigest([]byte(raw))
					f.envelope["totals"] = totals
					f.envelope["verdict"] = verdict
					f.envelope["artifacts"] = []ArtifactRef{{"unity-nunit-xml", "sha256:" + f.r.XML.SHA256}, {"unity-source-inputs", "sha256:" + f.r.Source.BeforeSHA256}}
					f.saveEnvelope(t)
					f.save(t)
				}
				st := unityTestStore(t, backend)
				seedUnitySymbol(t, st, f, s)
				e, err := f.load()
				if err != nil {
					t.Fatal(err)
				}
				r, err := UnityEvidenceRun(e, []core.SymbolNode{s})
				if err != nil {
					t.Fatal(err)
				}
				if _, err = IngestRuns(context.Background(), st, "svc", "test", []Run{r}); err != nil {
					t.Fatal(err)
				}
				if got := nativeStatus(t, st, s.Key()); got.Status != want {
					t.Fatalf("outcome=%s runtime %+v", outcome, got)
				}
			})
		}
	}
}
func TestUnityNUnitUnjoinedWithoutCurrentTypedIdentity(t *testing.T) {
	f := newUnityFixture(t)
	e, err := f.load()
	if err != nil {
		t.Fatal(err)
	}
	r, err := UnityEvidenceRun(e, nil)
	if err != nil {
		t.Fatal(err)
	}
	st := store.NewMemoryStore()
	report, err := IngestRuns(context.Background(), st, "svc", "w", []Run{r})
	if err != nil || report.CasesWritten != 2 || report.CasesJoined != 0 {
		t.Fatalf("%+v %v", report, err)
	}
	if got := nativeStatus(t, st, "Fits"); got.Status != "" {
		t.Fatalf("unjoined became runtime %+v", got)
	}
}
func TestUnityNUnitStoreRefusesConflictingRunIDReplayAtomically(t *testing.T) {
	for _, backend := range []string{"memory", "sqlite"} {
		t.Run(backend, func(t *testing.T) {
			st := unityTestStore(t, backend)
			ctx := context.Background()
			r := store.TestRun{RunDigest: "first", Suite: "EditMode", NativeRunID: "native-1", NativeJSON: `{"a":1}`, Cases: []store.TestCase{{CaseID: "1", TestSymbolKey: "svc::X", Status: "pass"}}}
			if err := st.PutTestRuns(ctx, "svc", []store.TestRun{r}); err != nil {
				t.Fatal(err)
			}
			if err := st.PutTestRuns(ctx, "svc", []store.TestRun{r}); err != nil {
				t.Fatal(err)
			}
			different := r
			different.RunDigest = "different"
			if err := st.PutTestRuns(ctx, "svc", []store.TestRun{different}); err == nil {
				t.Fatal("different payload accepted same run id")
			}
			history, err := st.TestCaseRunsForSymbol(ctx, "svc", "svc::X", 0)
			if err != nil || len(history) != 1 {
				t.Fatalf("replay mutated store %+v %v", history, err)
			}
			if err := st.PutTestRuns(ctx, "other", []store.TestRun{different}); err != nil {
				t.Fatalf("cross-service run id incorrectly refused %v", err)
			}
		})
	}
}
func TestUnityNUnitRuntimeRefusesMalformedStoredBinding(t *testing.T) {
	st := store.NewMemoryStore()
	st.PutTestRuns(context.Background(), "svc", []store.TestRun{{RunDigest: "d", NativeJSON: `{"unknown":"not evidence"}`, Cases: []store.TestCase{{CaseID: "1", TestSymbolKey: "svc::X", Status: "pass"}}}})
	got := nativeStatus(t, st, "svc::X")
	if got.Status != "unknown" || got.UnknownReason == "" {
		t.Fatal(got)
	}
}
func TestUnityNUnitNativeBindingJSONIsClosed(t *testing.T) {
	b := UnityBinding{Root: "/tmp", CaseKeys: map[string]string{}, Cohorts: map[string]string{}}
	raw, _ := json.Marshal(b)
	var copy UnityBinding
	if err := unityDecode(raw, &copy); err != nil {
		t.Fatal(err)
	}
}

func TestUnityNUnitRunIDConflictCannotHideBehindMatchingDigest(t *testing.T) {
	for _, backend := range []string{"memory", "sqlite"} {
		t.Run(backend, func(t *testing.T) {
			st := unityTestStore(t, backend)
			ctx := context.Background()
			first := store.TestRun{RunDigest: "A", Suite: "EditMode", NativeRunID: "X", NativeJSON: "binding-A"}
			second := store.TestRun{RunDigest: "B", Suite: "EditMode", NativeRunID: "Y", NativeJSON: "binding-B"}
			if err := st.PutTestRuns(ctx, "svc", []store.TestRun{first, second}); err != nil {
				t.Fatal(err)
			}
			second.NativeRunID = "X"
			if err := st.PutTestRuns(ctx, "svc", []store.TestRun{second}); err == nil {
				t.Fatal("matching digest hid conflicting native run id")
			}
		})
	}
}
