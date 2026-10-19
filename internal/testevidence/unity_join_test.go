package testevidence

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/scip"
	"github.com/geoffmilleraz/fathom/internal/store"
	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
	"google.golang.org/protobuf/proto"
)

func addUnityTypedFixture(t *testing.T, f *unityFixture) core.SymbolNode {
	t.Helper()
	project := f.r.Project
	source := project + "/Assets/BowlTests.cs"
	files := map[string]string{project + "/Workshop.Tests.csproj": `<Project><PropertyGroup><AssemblyName>Workshop.Tests</AssemblyName></PropertyGroup><ItemGroup><Compile Include="Assets/BowlTests.cs"/></ItemGroup></Project>`, "sdk.dll": "synthetic reference fixture", "scip-dotnet.nupkg": "synthetic producer fixture"}
	for path, body := range files {
		if err := os.WriteFile(filepath.Join(f.root, path), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		f.r.Source.Inputs = append(f.r.Source.Inputs, UnityFile{path, unityDigest([]byte(body))})
	}
	index := &scippb.Index{Metadata: &scippb.Metadata{ProjectRoot: "file://" + filepath.ToSlash(f.root), ToolInfo: &scippb.ToolInfo{Name: "scip-dotnet", Version: "0.1.0-SNAPSHOT"}}, Documents: []*scippb.Document{{RelativePath: source, Language: "C#", Occurrences: []*scippb.Occurrence{{Symbol: "scip-dotnet nuget . . Workshop/BowlTests#Fits().", SymbolRoles: 1, Range: []int32{0, 43, 47}}}}}}
	raw, err := proto.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(f.root, "index.scip")
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	inputs := []scip.SourceInput{}
	for _, v := range f.r.Source.Inputs {
		inputs = append(inputs, scip.SourceInput{Path: v.Path, SHA256: v.SHA256})
	}
	receipt := scip.SourceReceipt{Schema: scip.SourceReceiptSchema, Status: "complete", Root: f.root, IndexSHA256: unityDigest(raw), Producer: scip.SourceProducer{Name: "scip-dotnet", Version: "0.1.0-SNAPSHOT", DistributionVersion: "0.2.14", PackagePath: "scip-dotnet.nupkg", PackageSHA256: unityDigest([]byte(files["scip-dotnet.nupkg"]))}, Inputs: inputs, Inventories: []scip.SourceInventory{{Root: project + "/Assets", Pattern: "*.cs", Paths: []string{source}}}}
	writeUnityJSON(t, path+".receipt.json", receipt)
	receiptBytes, err := os.ReadFile(path + ".receipt.json")
	if err != nil {
		t.Fatal(err)
	}
	f.r.Symbols = &UnitySymbols{SCIPPath: "index.scip", SCIPSHA256: unityDigest(raw), SourceReceiptSHA256: unityDigest(receiptBytes)}
	f.r.Source.BeforeSHA256 = unityDigest(UnitySourceBytes(f.r.Source))
	f.r.Source.AfterSHA256 = f.r.Source.BeforeSHA256
	f.envelope["key"] = "sha256:" + f.r.Source.BeforeSHA256
	f.envelope["artifacts"] = []ArtifactRef{{"unity-nunit-xml", "sha256:" + f.r.XML.SHA256}, {"unity-source-inputs", "sha256:" + f.r.Source.BeforeSHA256}}
	f.saveEnvelope(t)
	f.save(t)
	return core.SymbolNode{Service: "svc", Kind: core.KindCSharpMethod, Name: "BowlTests.Fits", Package: "Workshop", FilePath: source, Line: 1, EndLine: 1, Signature: "[TestCase(1)] [TestCase(2)] public void Fits(int value)"}
}
func TestUnityNUnitTypedJoinRequiresExactAssemblyDefinitionAndUniqueMethod(t *testing.T) {
	f := newUnityFixture(t)
	symbol := addUnityTypedFixture(t, f)
	e, err := f.load()
	if err != nil {
		t.Fatal(err)
	}
	joined, binding, err := UnityJoinCases(e, []core.SymbolNode{symbol})
	if err != nil {
		t.Fatal(err)
	}
	if len(joined) != 2 || joined[0].TestSymbolKey != symbol.Key() || joined[1].TestSymbolKey != symbol.Key() || binding.Cohorts[symbol.Key()] != "pass" {
		t.Fatalf("join %+v binding %+v", joined, binding)
	}
	tests := map[string]func(UnityEvidence, core.SymbolNode) (UnityEvidence, []core.SymbolNode){
		"missing definitions": func(e UnityEvidence, s core.SymbolNode) (UnityEvidence, []core.SymbolNode) {
			s.Line++
			return e, []core.SymbolNode{s}
		},
		"same bare name other namespace": func(e UnityEvidence, s core.SymbolNode) (UnityEvidence, []core.SymbolNode) {
			s.Package = "Other"
			return e, []core.SymbolNode{s}
		},
		"overloaded same source identity": func(e UnityEvidence, s core.SymbolNode) (UnityEvidence, []core.SymbolNode) {
			copy := s
			copy.Line++
			return e, []core.SymbolNode{s, copy}
		},
		"duplicate indexed declaration": func(e UnityEvidence, s core.SymbolNode) (UnityEvidence, []core.SymbolNode) {
			return e, []core.SymbolNode{s, s}
		},
		"non test method": func(e UnityEvidence, s core.SymbolNode) (UnityEvidence, []core.SymbolNode) {
			s.Signature = "public void Fits(int value)"
			return e, []core.SymbolNode{s}
		},
		"retry unsupported": func(e UnityEvidence, s core.SymbolNode) (UnityEvidence, []core.SymbolNode) {
			s.Signature = "[TestCase(1)] [Retry(2)] public void Fits(int value)"
			return e, []core.SymbolNode{s}
		},
		"wrong native assembly": func(e UnityEvidence, s core.SymbolNode) (UnityEvidence, []core.SymbolNode) {
			e.Native.Cohorts[0].Assembly = "Other.Tests"
			return e, []core.SymbolNode{s}
		},
		"nested native identity": func(e UnityEvidence, s core.SymbolNode) (UnityEvidence, []core.SymbolNode) {
			e.Native.Cohorts[0].ClassName = "Workshop.Outer+BowlTests"
			return e, []core.SymbolNode{s}
		},
		"no typed artifact": func(e UnityEvidence, s core.SymbolNode) (UnityEvidence, []core.SymbolNode) {
			e.Receipt.Symbols = nil
			return e, []core.SymbolNode{s}
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			f := newUnityFixture(t)
			s := addUnityTypedFixture(t, f)
			e, err := f.load()
			if err != nil {
				t.Fatal(err)
			}
			e, syms := mutate(e, s)
			joined, _, err := UnityJoinCases(e, syms)
			if err != nil {
				t.Fatal(err)
			}
			for _, c := range joined {
				if c.TestSymbolKey != "" {
					t.Fatalf("guessed join %+v", c)
				}
			}
		})
	}
}
func TestUnityNUnitUnattestedGenericEnvelopeCannotJoinCSharp(t *testing.T) {
	f := newUnityFixture(t)
	s := addUnityTypedFixture(t, f)
	joined, stats := JoinCasesToSymbols(f.envelope["cases"].([]Case), "EditMode", []core.SymbolNode{s})
	if stats.Joined != 0 || joined[0].TestSymbolKey != "" {
		t.Fatal("generic envelope bypassed native receipt")
	}
}

func TestUnityRetryPolicyReadsOnlyAttributeHeads(t *testing.T) {
	for _, signature := range []string{`[Test] [Retry(2)] void Fits()`, `[NUnit.Framework.Repeat(2)] [Test] void Fits()`, `[Test, Retry] void Fits()`, `[Test, global::Other.RetryAttribute(3)] void Fits()`} {
		if !unityRetryAttribute(signature) {
			t.Errorf("missed policy %s", signature)
		}
	}
	for _, signature := range []string{`[TestCase("Retry")] void Fits()`, `[TestCase("Repeat")] void RetryIsRejected()`, `[TestCase(new[]{"Retry"})] void Fits()`, `[Test] void RepeatableName()`} {
		if unityRetryAttribute(signature) {
			t.Errorf("value/name mistaken for policy %s", signature)
		}
	}
}

func rewriteUnityTypedDefinition(t *testing.T, f *unityFixture, mutate func(*scippb.Occurrence)) {
	t.Helper()
	path := filepath.Join(f.root, "index.scip")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var index scippb.Index
	if err = proto.Unmarshal(raw, &index); err != nil {
		t.Fatal(err)
	}
	mutate(index.Documents[0].Occurrences[0])
	raw, err = proto.Marshal(&index)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	receiptBytes, err := os.ReadFile(path + ".receipt.json")
	if err != nil {
		t.Fatal(err)
	}
	var receipt scip.SourceReceipt
	if err = json.Unmarshal(receiptBytes, &receipt); err != nil {
		t.Fatal(err)
	}
	receipt.IndexSHA256 = unityDigest(raw)
	writeUnityJSON(t, path+".receipt.json", receipt)
	receiptBytes, _ = os.ReadFile(path + ".receipt.json")
	f.r.Symbols.SCIPSHA256 = unityDigest(raw)
	f.r.Symbols.SourceReceiptSHA256 = unityDigest(receiptBytes)
	f.save(t)
}
func TestUnityNUnitTypedDefinitionMustMatchOwnerAndValidRange(t *testing.T) {
	for _, which := range []string{"other class", "negative range", "reversed range", "extra coordinates"} {
		t.Run(which, func(t *testing.T) {
			f := newUnityFixture(t)
			s := addUnityTypedFixture(t, f)
			rewriteUnityTypedDefinition(t, f, func(o *scippb.Occurrence) {
				switch which {
				case "other class":
					o.Symbol = "scip-dotnet nuget . . Workshop/Other#Fits()."
				case "negative range":
					o.Range = []int32{0, 43, -1}
				case "reversed range":
					o.Range = []int32{0, 47, 43}
				case "extra coordinates":
					o.Range = []int32{0, 43, 47, 50, 90}
				}
			})
			e, err := f.load()
			if err != nil {
				t.Fatal(err)
			}
			joined, _, err := UnityJoinCases(e, []core.SymbolNode{s})
			if err != nil {
				t.Fatal(err)
			}
			for _, c := range joined {
				if c.TestSymbolKey != "" {
					t.Fatal("malformed/wrong-owner definition joined")
				}
			}
		})
	}
}
func TestUnityNUnitCurrentSymbolsRequiresIndexedSourceHash(t *testing.T) {
	f := newUnityFixture(t)
	sourcePath := f.r.Project + "/Assets/BowlTests.cs"
	source := []byte("namespace Workshop { public class BowlTests { [TestCase(1)] [TestCase(2)] public void Fits(int n) {} } }\n")
	if err := os.WriteFile(filepath.Join(f.root, sourcePath), source, 0600); err != nil {
		t.Fatal(err)
	}
	for i := range f.r.Source.Inputs {
		if f.r.Source.Inputs[i].Path == sourcePath {
			f.r.Source.Inputs[i].SHA256 = unityDigest(source)
		}
	}
	symbol := addUnityTypedFixture(t, f)
	e, err := f.load()
	if err != nil {
		t.Fatal(err)
	}
	st := store.NewMemoryStore()
	pipeline := fusion.New(connector.NewLocalConnector(), st, fusion.Config{DisableTypedEdges: true})
	if _, err = pipeline.Ingest(context.Background(), core.ServiceRef{ID: "svc", Path: f.root}); err != nil {
		t.Fatal(err)
	}
	observations, err := st.QueryObservations(context.Background(), "svc", "")
	if err != nil {
		t.Fatal(err)
	}
	methodRows := []core.Observation{}
	for _, o := range observations {
		if o.SourceKind == core.KindCSharpMethod {
			methodRows = append(methodRows, o)
		}
	}
	if len(methodRows) != 1 {
		t.Fatalf("actual method observations %+v", methodRows)
	}
	o := methodRows[0]
	if o.ContentHash != fusion.FingerprintHash(append([]byte(nil), source...)) || o.ContentHash == unityDigest(source) {
		t.Fatal("fixture did not exercise actual epoch-salted fingerprint")
	}
	current := UnityCurrentSymbols(e, methodRows)
	if len(current) != 1 {
		t.Fatalf("fresh fusion observation refused: %+v", o)
	}
	joined, _, err := UnityJoinCases(e, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(joined) != 2 || joined[0].TestSymbolKey != symbol.Key() || joined[1].TestSymbolKey != symbol.Key() {
		t.Fatalf("actual extracted typed join %+v", joined)
	}
	old := sha256.Sum256(append(append([]byte(nil), source...), []byte("\x00fathom-extractor-epoch:not-current")...))
	for _, hash := range []string{"", "stale", unityDigest(source), fmt.Sprintf("%x", old)} {
		o.ContentHash = hash
		if got := UnityCurrentSymbols(e, []core.Observation{o}); len(got) != 0 {
			t.Fatalf("wrong fingerprint %q admitted", hash)
		}
	}
	// A stale receipt cannot borrow current syntax observations after bytes change.
	if err := os.WriteFile(filepath.Join(f.root, sourcePath), append(source, []byte("// changed\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	if got := UnityCurrentSymbols(e, methodRows); len(got) != 0 {
		t.Fatal("changed raw source admitted")
	}
}

// A runner filter may select just one passing parameterized case. Unity's
// testcasecount then describes that selection, not the whole declared family.
func TestUnityNUnitFilteredRunsStayUnjoined(t *testing.T) {
	f := newUnityFixture(t)
	symbol := addUnityTypedFixture(t, f)
	raw := strings.ReplaceAll(nativeFixtureXML, `testcasecount="2"`, `testcasecount="1"`)
	raw = strings.ReplaceAll(raw, `total="2" passed="2"`, `total="1" passed="1"`)
	raw = strings.ReplaceAll(raw, `<test-case id="2" name="Fits(2)" fullname="Workshop.BowlTests.Fits(2)" classname="Workshop.BowlTests" methodname="Fits" result="Passed" duration="0.2"/>`, "")
	if err := os.WriteFile(f.xml, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	f.r.XML.SHA256 = unityDigest([]byte(raw))
	f.r.Filter = "Workshop.BowlTests.Fits(1)"
	f.r.Command = append(f.r.Command, "-testFilter", f.r.Filter)
	f.envelope["cases"] = f.envelope["cases"].([]Case)[:1]
	f.envelope["totals"] = Totals{Run: 1, Passed: 1}
	f.envelope["artifacts"] = []ArtifactRef{{"unity-nunit-xml", "sha256:" + f.r.XML.SHA256}, {"unity-source-inputs", "sha256:" + f.r.Source.BeforeSHA256}}
	f.saveEnvelope(t)
	f.save(t)
	e, err := f.load()
	if err != nil {
		t.Fatal(err)
	}
	if e.Native.Total != 1 || len(e.Native.Cohorts) != 1 || e.Native.Cohorts[0].Expected != 1 {
		t.Fatal("fixture is not an actual one-case selected cohort")
	}
	joined, binding, err := UnityJoinCases(e, []core.SymbolNode{symbol})
	if err != nil {
		t.Fatal(err)
	}
	if len(joined) != 1 || joined[0].TestSymbolKey != "" || len(binding.Cohorts) != 0 {
		t.Fatalf("filtered passing leaf promoted to whole-method result: %+v %+v", joined, binding)
	}
	st := store.NewMemoryStore()
	seedUnitySymbol(t, st, f, symbol)
	run, err := UnityEvidenceRun(e, []core.SymbolNode{symbol})
	if err != nil {
		t.Fatal(err)
	}
	report, err := IngestRuns(context.Background(), st, "svc", "filtered", []Run{run})
	if err != nil {
		t.Fatal(err)
	}
	if report.CasesJoined != 0 {
		t.Fatal("filtered case joined on ingestion")
	}
	if got := nativeStatus(t, st, symbol.Key()); got.Status != "" {
		t.Fatalf("filtered method runtime evidence %+v", got)
	}
}
