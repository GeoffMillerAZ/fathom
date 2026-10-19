package scip

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
	"google.golang.org/protobuf/proto"
)

func receiptHash(data []byte) string { h := sha256.Sum256(data); return hex.EncodeToString(h[:]) }
func receiptFixture(t *testing.T) (string, string, []byte, map[string]any) {
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
func writeReceipt(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".receipt.json", raw, 0644); err != nil {
		t.Fatal(err)
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
