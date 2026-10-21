package testevidence

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const nativeFixtureXML = `<?xml version="1.0"?><test-run id="2" testcasecount="2" total="2" passed="2" failed="0" skipped="0" inconclusive="0" result="Passed" start-time="2026-10-19 12:00:01Z" end-time="2026-10-19 12:00:02Z" duration="1.0"><test-suite type="Assembly" name="Workshop.Tests.dll"><test-suite type="ParameterizedMethod" id="p1" testcasecount="2"><test-case id="1" name="Fits(1)" fullname="Workshop.BowlTests.Fits(1)" classname="Workshop.BowlTests" methodname="Fits" result="Passed" duration="0.1"/><test-case id="2" name="Fits(2)" fullname="Workshop.BowlTests.Fits(2)" classname="Workshop.BowlTests" methodname="Fits" result="Passed" duration="0.2"/></test-suite></test-suite></test-run>`

type unityFixture struct {
	root, xml, receipt string
	r                  UnityReceipt
	envelope           map[string]any
}

func writeUnityJSON(t testing.TB, path string, v any) {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
}
func newUnityFixture(t testing.TB) *unityFixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := &unityFixture{root: root, xml: filepath.Join(root, "results.xml"), receipt: filepath.Join(root, ".artifacts/unity-tests/active/game/Workshop/EditMode.json")}
	project := "game/Workshop"
	if err := os.MkdirAll(filepath.Dir(f.receipt), 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Assets", "Packages", "ProjectSettings"} {
		if err := os.MkdirAll(filepath.Join(root, project, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{"producer.py": "# local synthetic fixture\n", "Editor": "synthetic Editor bytes\n", project + "/Assets/BowlTests.cs": "namespace Workshop { class BowlTests { void Fits(int n) {} } }\n", project + "/Assets/BowlTests.cs.meta": "guid: 0123456789abcdef0123456789abcdef\n", project + "/Packages/manifest.json": "{}\n", project + "/ProjectSettings/ProjectVersion.txt": "m_EditorVersion: 6000.3.15f1\n"}
	inputs := []UnityFile{}
	for path, content := range files {
		if err := os.WriteFile(filepath.Join(root, path), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, UnityFile{path, unityDigest([]byte(content))})
	}
	inv := []UnityInventory{}
	for _, name := range []string{"Assets", "Packages", "ProjectSettings"} {
		members := []string{}
		for path := range files {
			if strings.HasPrefix(path, project+"/"+name+"/") {
				members = append(members, path)
			}
		}
		sortStrings(members)
		inv = append(inv, UnityInventory{project + "/" + name, members})
	}
	s := UnitySource{Inputs: inputs, Inventories: inv}
	s.BeforeSHA256 = unityDigest(UnitySourceBytes(s))
	s.AfterSHA256 = s.BeforeSHA256
	zero := 0
	f.r = UnityReceipt{Schema: UnityReceiptSchema, Status: "complete", RunID: "synthetic-run-1", Root: root, Project: project, Mode: "EditMode", Target: "WebGL", StartedAt: "2026-10-19T12:00:00.123Z", FinishedAt: "2026-10-19T12:00:03Z", Producer: UnityProducer{"unity-local-runner", "1", "producer.py", unityDigest([]byte(files["producer.py"]))}, Editor: UnityEditor{filepath.Join(root, "Editor"), "6000.3.15f1", unityDigest([]byte(files["Editor"]))}, ExitCode: &zero, XML: UnityFile{"results.xml", unityDigest([]byte(nativeFixtureXML))}, Source: s, Attestation: UnityFile{Path: "test-result.json"}}
	// Absolute Editor is canonical in both its source row and explicit identity.
	for i := range f.r.Source.Inputs {
		if f.r.Source.Inputs[i].Path == "Editor" {
			f.r.Source.Inputs[i].Path = f.r.Editor.Path
		}
	}
	f.r.Source.BeforeSHA256 = unityDigest(UnitySourceBytes(f.r.Source))
	f.r.Source.AfterSHA256 = f.r.Source.BeforeSHA256
	f.r.Command = []string{f.r.Editor.Path, "-batchmode", "-projectPath", filepath.Join(root, project), "-buildTarget", "WebGL", "-runTests", "-testPlatform", "EditMode", "-testResults", f.xml}
	f.envelope = map[string]any{"suite": map[string]any{"component": project, "name": "EditMode", "class": "integration"}, "key": "sha256:" + f.r.Source.BeforeSHA256, "verdict": "pass", "cases": []Case{{ID: "Workshop.Tests::Workshop.BowlTests.Fits(1)", Name: "Workshop.BowlTests.Fits(1)", Status: "pass", Duration: "100ms", Attempts: 1}, {ID: "Workshop.Tests::Workshop.BowlTests.Fits(2)", Name: "Workshop.BowlTests.Fits(2)", Status: "pass", Duration: "200ms", Attempts: 1}}, "totals": Totals{Run: 2, Passed: 2}, "timing": map[string]any{"startedAt": "2026-10-19T12:00:01Z", "wallClock": "1s"}, "env": map[string]any{"toolchainDigest": "sha256:" + f.r.Editor.SHA256, "runnerClass": "unity", "arch": "arm64", "os": "darwin", "commit": strings.Repeat("a", 40), "dirty": true, "race": false}, "artifacts": []ArtifactRef{{"unity-nunit-xml", "sha256:" + f.r.XML.SHA256}, {"unity-source-inputs", "sha256:" + f.r.Source.BeforeSHA256}}, "flake": map[string]any{"maxRetries": 0, "quarantineAfter": 3, "budget": "0%"}, "trace": map[string]any{"traceId": f.r.RunID}}
	if err = os.WriteFile(f.xml, []byte(nativeFixtureXML), 0600); err != nil {
		t.Fatal(err)
	}
	f.saveEnvelope(t)
	f.save(t)
	return f
}
func sortStrings(v []string) {
	for i := 1; i < len(v); i++ {
		for j := i; j > 0 && v[j] < v[j-1]; j-- {
			v[j], v[j-1] = v[j-1], v[j]
		}
	}
}
func (f *unityFixture) saveEnvelope(t testing.TB) {
	t.Helper()
	p := filepath.Join(f.root, f.r.Attestation.Path)
	writeUnityJSON(t, p, f.envelope)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	f.r.Attestation.SHA256 = unityDigest(b)
}
func (f *unityFixture) save(t testing.TB) { t.Helper(); writeUnityJSON(t, f.receipt, f.r) }
func (f *unityFixture) load() (UnityEvidence, error) {
	return LoadUnityNUnitEvidence(context.Background(), f.root, f.xml, f.receipt)
}

// Source freshness still checks each file on every call. Scratch allocations
// should stay bounded as the number of hashed inputs grows.
func TestUnitySourceValidationAllocationBound(t *testing.T) {
	f := newUnityFixture(t)
	const files = 32
	content := bytes.Repeat([]byte("x"), 64<<10)
	for i := 0; i < files; i++ {
		name := fmt.Sprintf("%s/Assets/extra-%02d.cs", f.r.Project, i)
		if err := os.WriteFile(filepath.Join(f.root, name), content, 0600); err != nil {
			t.Fatal(err)
		}
		f.r.Source.Inputs = append(f.r.Source.Inputs, UnityFile{Path: name, SHA256: unityDigest(content)})
		f.r.Source.Inventories[0].Paths = append(f.r.Source.Inventories[0].Paths, name)
	}
	sortStrings(f.r.Source.Inventories[0].Paths)
	f.r.Source.BeforeSHA256 = unityDigest(UnitySourceBytes(f.r.Source))
	f.r.Source.AfterSHA256 = f.r.Source.BeforeSHA256
	result := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if err := unityValidateSource(context.Background(), f.root, f.r); err != nil {
				b.Fatal(err)
			}
		}
	})
	t.Logf("unityValidateSource %d files: %d B/op", files+6, result.AllocedBytesPerOp())
	if got := result.AllocedBytesPerOp(); got > 1<<20 {
		t.Fatalf("validation allocates %d B/op; want <= 1 MiB", got)
	}
}

func TestUnityNUnitReceiptBindsNativeAndStandardRecord(t *testing.T) {
	f := newUnityFixture(t)
	got, err := f.load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Native.Total != 2 || got.Run.Totals.Passed != 2 || got.Run.Dirty != true || got.Receipt.Symbols != nil {
		t.Fatalf("bad native record: %+v", got)
	}
	if got.Native.Cohorts[0].ObservedVerdict != "pass" {
		t.Fatalf("cohort=%+v", got.Native.Cohorts)
	}
	// Re-reading identical immutable bytes is deterministic, not another run.
	again, err := f.load()
	if err != nil || again.Run.Digest != got.Run.Digest || again.ReceiptSHA256 != got.ReceiptSHA256 {
		t.Fatalf("replay: %+v %v", again, err)
	}
}
func TestUnityNUnitReceiptRefusesUnboundStaleAndMixedEvidence(t *testing.T) {
	tests := map[string]func(*testing.T, *unityFixture){
		"started":              func(t *testing.T, f *unityFixture) { f.r.Status = "started"; f.save(t) },
		"missing envelope":     func(t *testing.T, f *unityFixture) { f.r.Attestation = UnityFile{}; f.save(t) },
		"different root":       func(t *testing.T, f *unityFixture) { f.r.Root = filepath.Dir(f.root); f.save(t) },
		"nonzero passing exit": func(t *testing.T, f *unityFixture) { n := 1; f.r.ExitCode = &n; f.save(t) },
		"missing exit":         func(t *testing.T, f *unityFixture) { f.r.ExitCode = nil; f.save(t) },
		"source changed": func(t *testing.T, f *unityFixture) {
			os.WriteFile(filepath.Join(f.root, f.r.Project, "Assets/BowlTests.cs"), []byte("changed"), 0600)
		},
		"source added": func(t *testing.T, f *unityFixture) {
			os.WriteFile(filepath.Join(f.root, f.r.Project, "Assets/Added.cs"), []byte("new"), 0600)
		},
		"source removed": func(t *testing.T, f *unityFixture) {
			os.Remove(filepath.Join(f.root, f.r.Project, "Assets/BowlTests.cs.meta"))
		},
		"mixed source digests": func(t *testing.T, f *unityFixture) { f.r.Source.AfterSHA256 = strings.Repeat("b", 64); f.save(t) },
		"duplicate input": func(t *testing.T, f *unityFixture) {
			f.r.Source.Inputs = append(f.r.Source.Inputs, f.r.Source.Inputs[0])
			f.r.Source.BeforeSHA256 = unityDigest(UnitySourceBytes(f.r.Source))
			f.r.Source.AfterSHA256 = f.r.Source.BeforeSHA256
			f.save(t)
		},
		"producer invoked through traversal": func(t *testing.T, f *unityFixture) { f.r.Producer.Path = "game/Workshop/../../producer.py"; f.save(t) },
		"producer omitted from closure": func(t *testing.T, f *unityFixture) {
			for i, v := range f.r.Source.Inputs {
				if v.Path == "producer.py" {
					f.r.Source.Inputs = append(f.r.Source.Inputs[:i], f.r.Source.Inputs[i+1:]...)
					break
				}
			}
			f.r.Source.BeforeSHA256 = unityDigest(UnitySourceBytes(f.r.Source))
			f.r.Source.AfterSHA256 = f.r.Source.BeforeSHA256
			f.save(t)
		},
		"xml changed": func(t *testing.T, f *unityFixture) {
			os.WriteFile(f.xml, []byte(strings.Replace(nativeFixtureXML, "Passed", "Failed", 1)), 0600)
		},
		"xml from earlier attempt": func(t *testing.T, f *unityFixture) {
			f.r.StartedAt = "2026-10-19T13:00:00Z"
			f.r.FinishedAt = "2026-10-19T13:00:10Z"
			f.save(t)
		},
		"mixed mode":   func(t *testing.T, f *unityFixture) { f.r.Mode = "PlayMode"; f.save(t) },
		"mixed target": func(t *testing.T, f *unityFixture) { f.r.Target = "StandaloneOSX"; f.save(t) },
		"duplicate argv": func(t *testing.T, f *unityFixture) {
			f.r.Command = append(f.r.Command, "-testPlatform", "EditMode")
			f.save(t)
		},
		"secret-like arbitrary argv rejected": func(t *testing.T, f *unityFixture) {
			f.r.Command = append(f.r.Command, "-password", "synthetic")
			f.save(t)
		},
		"traversal":      func(t *testing.T, f *unityFixture) { f.r.Project = "game/../Workshop"; f.save(t) },
		"unsafe control": func(t *testing.T, f *unityFixture) { f.r.Filter = "x\ny"; f.save(t) },
		"symlink source": func(t *testing.T, f *unityFixture) {
			p := filepath.Join(f.root, f.r.Project, "Assets/Link.cs")
			os.Symlink(filepath.Join(f.root, "producer.py"), p)
		},
		"future schema": func(t *testing.T, f *unityFixture) { f.r.Schema += "-future"; f.save(t) },
		"unknown receipt field": func(t *testing.T, f *unityFixture) {
			b, _ := os.ReadFile(f.receipt)
			b = append([]byte(`{"new":true,`), b[1:]...)
			os.WriteFile(f.receipt, b, 0600)
		},
		"duplicate JSON": func(t *testing.T, f *unityFixture) {
			b, _ := os.ReadFile(f.receipt)
			b = append([]byte(`{"status":"started",`), b[1:]...)
			os.WriteFile(f.receipt, b, 0600)
		},
		"missing explicit filter": func(t *testing.T, f *unityFixture) {
			raw, _ := os.ReadFile(f.receipt)
			var v map[string]any
			json.Unmarshal(raw, &v)
			delete(v, "filter")
			writeUnityJSON(t, f.receipt, v)
		},
		"null filter": func(t *testing.T, f *unityFixture) {
			raw, _ := os.ReadFile(f.receipt)
			var v map[string]any
			json.Unmarshal(raw, &v)
			v["filter"] = nil
			writeUnityJSON(t, f.receipt, v)
		},
		"trailing JSON": func(t *testing.T, f *unityFixture) {
			b, _ := os.ReadFile(f.receipt)
			os.WriteFile(f.receipt, append(b, []byte(` {}`)...), 0600)
		},
		"stale symbols": func(t *testing.T, f *unityFixture) {
			f.r.Symbols = &UnitySymbols{SCIPPath: "missing.scip", SCIPSHA256: strings.Repeat("a", 64), SourceReceiptSHA256: strings.Repeat("b", 64)}
			f.save(t)
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			f := newUnityFixture(t)
			mutate(t, f)
			_, err := f.load()
			if !errors.Is(err, ErrUnityEvidence) {
				t.Fatalf("accepted or wrong refusal: %v", err)
			}
		})
	}
}
func TestUnityNUnitEnvelopeCannotUpgradeNativeOutcomes(t *testing.T) {
	tests := map[string]func(map[string]any){
		"case status":    func(v map[string]any) { cases := v["cases"].([]Case); cases[0].Status = "fail" },
		"case fullname":  func(v map[string]any) { cases := v["cases"].([]Case); cases[0].Name = "Fits" },
		"duplicate case": func(v map[string]any) { cases := v["cases"].([]Case); cases[1] = cases[0] },
		"retry attempts": func(v map[string]any) { cases := v["cases"].([]Case); cases[0].Attempts = 2 },
		"case duration":  func(v map[string]any) { cases := v["cases"].([]Case); cases[0].Duration = "9s" },
		"totals":         func(v map[string]any) { v["totals"] = Totals{Run: 1, Passed: 1} },
		"verdict":        func(v map[string]any) { v["verdict"] = "error" },
		"trace missing":  func(v map[string]any) { delete(v, "trace") },
		"different run":  func(v map[string]any) { v["trace"] = map[string]string{"traceId": "other"} },
		"unbound key":    func(v map[string]any) { v["key"] = "unknown" },
		"commit missing": func(v map[string]any) { delete(v["env"].(map[string]any), "commit") },
		"dirty missing":  func(v map[string]any) { delete(v["env"].(map[string]any), "dirty") },
		"fake coverage":  func(v map[string]any) { v["coverage"] = map[string]any{"statements": 100} },
		"retry policy":   func(v map[string]any) { v["flake"].(map[string]any)["maxRetries"] = 1 },
		"suite":          func(v map[string]any) { v["suite"].(map[string]any)["name"] = "PlayMode" },
		"start":          func(v map[string]any) { v["timing"].(map[string]any)["startedAt"] = "2026-10-19T12:00:00Z" },
		"toolchain":      func(v map[string]any) { v["env"].(map[string]any)["toolchainDigest"] = "unbound" },
		"artifact":       func(v map[string]any) { v["artifacts"] = []ArtifactRef{} },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			f := newUnityFixture(t)
			mutate(f.envelope)
			f.saveEnvelope(t)
			f.save(t)
			_, err := f.load()
			if !errors.Is(err, ErrUnityEvidence) {
				t.Fatalf("accepted contradictory envelope: %v", err)
			}
		})
	}
}
func TestUnityResultMirrorMatchesSource(t *testing.T) {
	raw, err := os.ReadFile("../../schema/testkit/result.cue")
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != string(testKitResultSchema) {
		t.Fatal("embedded result schema differs from test-kit mirror")
	}
}
func TestUnitySourceCanonicalBytes(t *testing.T) {
	s := UnitySource{Inputs: []UnityFile{{"z", "b"}, {"a", "c"}}, Inventories: []UnityInventory{{Root: "z"}, {Root: "a"}}}
	want := "fathom.unity.inputs/v1\ntree\x00a\ntree\x00z\nfile\x00a\x00c\nfile\x00z\x00b\n"
	if string(UnitySourceBytes(s)) != want {
		t.Fatalf("wrong canonical bytes %q", UnitySourceBytes(s))
	}
}
func TestUnityReceiptRespectsCancelledContext(t *testing.T) {
	f := newUnityFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := LoadUnityNUnitEvidence(ctx, f.root, f.xml, f.receipt)
	if err == nil || !strings.Contains(err.Error(), "canceled") {
		t.Fatal(err)
	}
}

func TestUnityRefusalDoesNotEchoArbitraryCommandValues(t *testing.T) {
	f := newUnityFixture(t)
	sentinel := "synthetic-sensitive-argument-value"
	f.r.Command = append(f.r.Command, sentinel)
	f.save(t)
	_, err := f.load()
	if err == nil {
		t.Fatal("unknown argument admitted")
	}
	if strings.Contains(err.Error(), sentinel) {
		t.Fatal("refusal echoed an arbitrary argv value")
	}
}
