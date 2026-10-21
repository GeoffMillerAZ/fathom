package scip

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

func TestSourceReceiptSummaryMatchesWholeProto(t *testing.T) {
	field := func(number protowire.Number, payload []byte) []byte {
		return protowire.AppendBytes(protowire.AppendTag(nil, number, protowire.BytesType), payload)
	}
	message := func(value proto.Message) []byte {
		encoded, err := proto.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return encoded
	}
	metadataRoot := field(1, message(&scippb.Metadata{ProjectRoot: "file:///project"}))
	metadataTool := field(1, message(&scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-dotnet", Version: "1"}}))
	document := field(2, message(&scippb.Document{RelativePath: "src/Actor.cs"}))
	external := field(3, message(&scippb.SymbolInformation{Symbol: "external"}))
	unknown := field(42, []byte("unrecognized"))
	group := protowire.AppendTag(nil, 43, protowire.StartGroupType)
	group = protowire.AppendBytes(protowire.AppendTag(group, 1, protowire.BytesType), []byte("payload"))
	group = protowire.AppendTag(group, 43, protowire.EndGroupType)
	malformedText := field(2, protowire.AppendBytes(protowire.AppendTag(nil, 5, protowire.BytesType), []byte{0xff}))
	malformedExternal := field(3, protowire.AppendBytes(protowire.AppendTag(nil, 1, protowire.BytesType), []byte{0xff}))
	malformedRoot := field(1, protowire.AppendBytes(protowire.AppendTag(nil, 3, protowire.BytesType), []byte{0xff}))
	truncatedDocument := append(protowire.AppendTag(nil, 2, protowire.BytesType), 12, 0x0a, 0x01, 'x')
	truncatedMetadata := append(protowire.AppendTag(nil, 1, protowire.BytesType), 12, 0x12, 0x04, 0x0a, 0x02, 'o', 'k')
	deepGroup := []byte{}
	for i := 0; i < 110; i++ {
		deepGroup = protowire.AppendTag(deepGroup, 44, protowire.StartGroupType)
	}
	for i := 0; i < 110; i++ {
		deepGroup = protowire.AppendTag(deepGroup, 44, protowire.EndGroupType)
	}
	cases := []struct {
		name string
		raw  []byte
	}{
		{"merged metadata and reordered fields", bytes.Join([][]byte{document, metadataRoot, external, metadataTool}, nil)},
		{"unknown field and group", bytes.Join([][]byte{metadataTool, unknown, group, document}, nil)},
		{"malformed unused document UTF8", bytes.Join([][]byte{metadataTool, malformedText}, nil)},
		{"malformed unused external UTF8", bytes.Join([][]byte{metadataTool, malformedExternal}, nil)},
		{"malformed metadata UTF8", bytes.Join([][]byte{metadataTool, malformedRoot}, nil)},
		{"truncated document", bytes.Join([][]byte{metadataTool, truncatedDocument}, nil)},
		{"truncated metadata with partial tool", bytes.Join([][]byte{metadataTool, truncatedMetadata}, nil)},
		{"wrong wire type", bytes.Join([][]byte{metadataTool, protowire.AppendVarint(protowire.AppendTag(nil, 2, protowire.VarintType), 1)}, nil)},
		{"deep unknown group", bytes.Join([][]byte{metadataTool, deepGroup}, nil)},
		{"unterminated unknown group", bytes.Join([][]byte{metadataTool, protowire.AppendTag(nil, 44, protowire.StartGroupType)}, nil)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var whole scippb.Index
			wantErr := proto.Unmarshal(tc.raw, &whole)
			got, gotErr := sourceReceiptIndexSummary(tc.raw)
			if (gotErr == nil) != (wantErr == nil) {
				t.Fatalf("decode errors: whole=%v summary=%v", wantErr, gotErr)
			}
			if gotErr != nil && gotErr.Error() != wantErr.Error() {
				t.Fatalf("decode error differs: whole=%v summary=%v", wantErr, gotErr)
			}
			if got.GetMetadata().GetProjectRoot() != whole.GetMetadata().GetProjectRoot() || got.GetMetadata().GetToolInfo().GetName() != whole.GetMetadata().GetToolInfo().GetName() || got.GetMetadata().GetToolInfo().GetVersion() != whole.GetMetadata().GetToolInfo().GetVersion() {
				t.Fatalf("metadata differs: whole=%v summary=%v", whole.GetMetadata(), got.GetMetadata())
			}
			if len(got.Documents) != len(whole.Documents) {
				t.Fatalf("documents: whole=%d summary=%d", len(whole.Documents), len(got.Documents))
			}
			for i, doc := range got.Documents {
				if doc.GetRelativePath() != whole.Documents[i].GetRelativePath() {
					t.Fatalf("document %d path differs", i)
				}
			}
		})
	}
}

func TestSourceReceiptRejectsMalformedUnusedFields(t *testing.T) {
	for _, tc := range []struct {
		name   string
		field  protowire.Number
		member protowire.Number
	}{
		{"document text", 2, 5},
		{"external symbol", 3, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, path, raw, receipt := receiptFixture(t)
			nested := protowire.AppendBytes(protowire.AppendTag(nil, tc.member, protowire.BytesType), []byte{0xff})
			raw = protowire.AppendBytes(protowire.AppendTag(raw, tc.field, protowire.BytesType), nested)
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			receipt["index_sha256"] = receiptHash(raw)
			writeReceipt(t, path, receipt)
			bound, err := ValidateSourceReceipt(context.Background(), root, path, raw)
			if !bound || err == nil || !strings.Contains(err.Error(), "index decode") {
				t.Fatalf("bound=%v err=%v", bound, err)
			}
		})
	}
}

func TestSourceReceiptMalformedUnboundIndexUsesPartialMetadata(t *testing.T) {
	root, path, raw, _ := receiptFixture(t)
	if err := os.Remove(path + ".receipt.json"); err != nil {
		t.Fatal(err)
	}
	// The metadata tool name precedes a truncated top-level field. The old
	// whole-message decoder still binds this producer despite its error.
	raw = append(raw, 0x12, 0x08, 0x0a)
	bound, err := ValidateSourceReceipt(context.Background(), root, path, raw)
	if !bound || err == nil || !strings.Contains(err.Error(), "index decode") {
		t.Fatalf("bound=%v err=%v", bound, err)
	}
}

type receiptHeapProbe struct {
	baseline uint64
	retained uint64
	called   bool
}

func (p *receiptHeapProbe) Deadline() (time.Time, bool) { return time.Time{}, false }
func (p *receiptHeapProbe) Done() <-chan struct{}       { return nil }
func (p *receiptHeapProbe) Value(any) any               { return nil }
func (p *receiptHeapProbe) Err() error {
	if !p.called {
		p.called = true
		runtime.GC()
		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		if stats.HeapAlloc > p.baseline {
			p.retained = stats.HeapAlloc - p.baseline
		}
	}
	return nil
}

// The first context read occurs while hashing the index, after decoding it.
// The raw index and fixture are live at both measurements. The budget measures
// retained decoded payload, not total allocations or process peak RSS.
func TestSourceReceiptDropsUnusedDecodedPayloadBeforeHash(t *testing.T) {
	root, path, original, receipt := receiptFixture(t)
	var index scippb.Index
	if err := proto.Unmarshal(original, &index); err != nil {
		t.Fatal(err)
	}
	index.Documents = nil
	text := strings.Repeat("x", 2<<20)
	for i := 0; i < 8; i++ {
		index.Documents = append(index.Documents, &scippb.Document{RelativePath: "src/Actor.cs", Text: text})
	}
	raw, err := proto.Marshal(&index)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	receipt["index_sha256"] = receiptHash(raw)
	writeReceipt(t, path, receipt)
	runtime.GC()
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	probe := &receiptHeapProbe{baseline: stats.HeapAlloc}
	bound, err := ValidateSourceReceipt(probe, root, path, raw)
	runtime.KeepAlive(&index)
	runtime.KeepAlive(raw)
	if !bound || err != nil {
		t.Fatalf("bound=%v err=%v", bound, err)
	}
	if !probe.called {
		t.Fatal("index hash boundary was not observed")
	}
	t.Logf("retained decoded heap at index hash: %d bytes", probe.retained)
	if probe.retained > 4<<20 {
		t.Fatalf("retained decoded heap %d > 4 MiB", probe.retained)
	}
}

func receiptHash(data []byte) string { h := sha256.Sum256(data); return hex.EncodeToString(h[:]) }
func receiptFixture(t testing.TB) (string, string, []byte, map[string]any) {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "index.scip")
	if err := os.MkdirAll(filepath.Join(root, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"src/Actor.cs": "class Actor {}\n", "Game.csproj": "<Project/>\n", "sdk.dll": "reference bytes", "producer.nupkg": "producer bytes"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := proto.Marshal(&scippb.Index{Metadata: &scippb.Metadata{ProjectRoot: "file://" + filepath.ToSlash(root), ToolInfo: &scippb.ToolInfo{Name: "scip-dotnet", Version: "0.1.0-SNAPSHOT"}}, Documents: []*scippb.Document{{RelativePath: "src/Actor.cs", Language: "C#"}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatal(err)
	}
	var inputs []any
	for _, name := range []string{"src/Actor.cs", "Game.csproj", "sdk.dll", "producer.nupkg"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, map[string]any{"path": name, "sha256": receiptHash(data)})
	}
	receipt := map[string]any{"schema": "fathom.scip.source/v1", "status": "complete", "root": root, "index_sha256": receiptHash(raw), "producer": map[string]any{"name": "scip-dotnet", "version": "0.1.0-SNAPSHOT", "distribution_version": "0.2.14", "package_path": "producer.nupkg", "package_sha256": receiptHash([]byte("producer bytes"))}, "inputs": inputs, "inventories": []any{map[string]any{"root": "src", "pattern": "*.cs", "paths": []string{"src/Actor.cs"}}}}
	writeReceipt(t, path, receipt)
	return root, path, raw, receipt
}
func writeReceipt(t testing.TB, path string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".receipt.json", raw, 0644); err != nil {
		t.Fatal(err)
	}
}

// The receipt still hashes every input on each call. Its streaming scratch
// space should be bounded per validation, rather than growing per input.
func TestSourceReceiptValidationAllocationBound(t *testing.T) {
	root, path, raw, receipt := receiptFixture(t)
	const files = 32
	content := bytes.Repeat([]byte("x"), 64<<10)
	inputs := receipt["inputs"].([]any)
	for i := 0; i < files; i++ {
		name := fmt.Sprintf("extra-%02d.dat", i)
		if err := os.WriteFile(filepath.Join(root, name), content, 0600); err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, map[string]any{"path": name, "sha256": receiptHash(content)})
	}
	receipt["inputs"] = inputs
	writeReceipt(t, path, receipt)
	result := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			bound, err := ValidateSourceReceipt(context.Background(), root, path, raw)
			if !bound || err != nil {
				b.Fatalf("bound=%v err=%v", bound, err)
			}
		}
	})
	t.Logf("ValidateSourceReceipt %d files: %d B/op", files+4, result.AllocedBytesPerOp())
	if got := result.AllocedBytesPerOp(); got > 512<<10 {
		t.Fatalf("validation allocates %d B/op; want <= 512 KiB", got)
	}
}
func TestSourceReceiptFreshAndLegacyCompatibility(t *testing.T) {
	root, path, raw, _ := receiptFixture(t)
	bound, err := ValidateSourceReceipt(context.Background(), root, path, raw)
	if !bound || err != nil {
		t.Fatalf("fresh bound=%v err=%v", bound, err)
	}
	idx := BuildTSFixtureIndex()
	legacy, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path + ".receipt.json"); err != nil {
		t.Fatal(err)
	}
	bound, err = ValidateSourceReceipt(context.Background(), root, path, legacy)
	if bound || err != nil {
		t.Fatalf("legacy bound=%v err=%v", bound, err)
	}
}
func TestSourceReceiptRefusesMalformedIndexAndCancelledValidation(t *testing.T) {
	root, path, raw, _ := receiptFixture(t)
	bound, err := ValidateSourceReceipt(context.Background(), root, path, raw)
	if !bound || err != nil {
		t.Fatalf("uncancelled validation bound=%v err=%v", bound, err)
	}
	bound, err = ValidateSourceReceipt(context.Background(), root, path, []byte{0x0a, 0x80}, true)
	if !bound || err == nil || !strings.Contains(err.Error(), "index decode") {
		t.Fatalf("malformed index bound=%v err=%v", bound, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	bound, err = ValidateSourceReceipt(ctx, root, path, raw)
	if !bound || !errors.Is(err, ErrSourceReceipt) {
		t.Fatalf("cancelled validation bound=%v err=%v", bound, err)
	}
}
func TestSourceReceiptRefusesStaleMissingAndMixedInputs(t *testing.T) {
	cases := []struct {
		name, want string
		mutate     func(*testing.T, string, string, map[string]any)
	}{
		{"missing receipt", "receipt", func(t *testing.T, r, p string, v map[string]any) {
			if err := os.Remove(p + ".receipt.json"); err != nil {
				t.Fatal(err)
			}
		}},
		{"source edit", "Actor.cs", func(t *testing.T, r, p string, v map[string]any) {
			if err := os.WriteFile(filepath.Join(r, "src/Actor.cs"), []byte("class Other {}"), 0644); err != nil {
				t.Fatal(err)
			}
		}},
		{"source deletion", "Actor.cs", func(t *testing.T, r, p string, v map[string]any) {
			if err := os.Remove(filepath.Join(r, "src/Actor.cs")); err != nil {
				t.Fatal(err)
			}
		}},
		{"source addition", "inventory", func(t *testing.T, r, p string, v map[string]any) {
			if err := os.WriteFile(filepath.Join(r, "src/New.cs"), []byte("class New {}"), 0644); err != nil {
				t.Fatal(err)
			}
		}},
		{"project drift", "Game.csproj", func(t *testing.T, r, p string, v map[string]any) {
			if err := os.WriteFile(filepath.Join(r, "Game.csproj"), []byte("<Project Changed='yes'/>"), 0644); err != nil {
				t.Fatal(err)
			}
		}},
		{"reference drift", "sdk.dll", func(t *testing.T, r, p string, v map[string]any) {
			if err := os.WriteFile(filepath.Join(r, "sdk.dll"), []byte("new reference"), 0644); err != nil {
				t.Fatal(err)
			}
		}},
		{"failed attempt", "status", func(t *testing.T, r, p string, v map[string]any) { v["status"] = "started"; writeReceipt(t, p, v) }},
		{"index hash mismatch", "index hash", func(t *testing.T, r, p string, v map[string]any) {
			v["index_sha256"] = strings.Repeat("0", 64)
			writeReceipt(t, p, v)
		}},
		{"index changed after read", "index changed", func(t *testing.T, r, p string, v map[string]any) {
			if err := os.WriteFile(p, []byte("different index bytes"), 0644); err != nil {
				t.Fatal(err)
			}
		}},
		{"missing producer package", "package identity", func(t *testing.T, r, p string, v map[string]any) {
			delete(v["producer"].(map[string]any), "package_path")
			writeReceipt(t, p, v)
		}},
		{"unbound producer package", "package is not bound", func(t *testing.T, r, p string, v map[string]any) {
			v["inputs"] = v["inputs"].([]any)[:3]
			writeReceipt(t, p, v)
		}},
		{"missing project hash", "project and reference", func(t *testing.T, r, p string, v map[string]any) {
			inputs := v["inputs"].([]any)
			v["inputs"] = append(inputs[:1], inputs[2:]...)
			writeReceipt(t, p, v)
		}},
		{"missing reference hash", "project and reference", func(t *testing.T, r, p string, v map[string]any) {
			inputs := v["inputs"].([]any)
			v["inputs"] = append(inputs[:2], inputs[3:]...)
			writeReceipt(t, p, v)
		}},
		{"oversize reference", "byte budget", func(t *testing.T, r, p string, v map[string]any) {
			if err := os.Truncate(filepath.Join(r, "sdk.dll"), sourceInputBytesLimit+1); err != nil {
				t.Fatal(err)
			}
		}},
		{"producer mismatch", "producer", func(t *testing.T, r, p string, v map[string]any) {
			v["producer"] = map[string]any{"name": "other", "version": "1"}
			writeReceipt(t, p, v)
		}},
		{"root mismatch", "root", func(t *testing.T, r, p string, v map[string]any) { v["root"] = t.TempDir(); writeReceipt(t, p, v) }},
		{"relative receipt root", "absolute", func(t *testing.T, r, p string, v map[string]any) { v["root"] = "."; writeReceipt(t, p, v) }},
		{"unhashed inventory member", "no input hash", func(t *testing.T, r, p string, v map[string]any) {
			if err := os.WriteFile(filepath.Join(r, "src/Hidden.cs"), []byte("class Hidden {}"), 0644); err != nil {
				t.Fatal(err)
			}
			v["inventories"].([]any)[0].(map[string]any)["paths"] = []string{"src/Actor.cs", "src/Hidden.cs"}
			writeReceipt(t, p, v)
		}},
		{"unknown schema", "schema", func(t *testing.T, r, p string, v map[string]any) { v["schema"] = "future/v2"; writeReceipt(t, p, v) }},
		{"unbound document", "input hash", func(t *testing.T, r, p string, v map[string]any) {
			v["inputs"] = v["inputs"].([]any)[1:]
			writeReceipt(t, p, v)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, path, raw, receipt := receiptFixture(t)
			tc.mutate(t, root, path, receipt)
			bound, err := ValidateSourceReceipt(context.Background(), root, path, raw)
			if !bound || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("bound=%v err=%v want %q", bound, err, tc.want)
			}
		})
	}
}
