package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/scip"
	"github.com/geoffmilleraz/fathom/pkg/client"
	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
	"google.golang.org/protobuf/proto"
)

func writeCmdSourceReceipt(t *testing.T, root, path string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var index scippb.Index
	if err := proto.Unmarshal(raw, &index); err != nil {
		t.Fatal(err)
	}
	index.Metadata.ProjectRoot = "file://" + filepath.ToSlash(root)
	raw, err = proto.Marshal(&index)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatal(err)
	}
	digest := func(data []byte) string { h := sha256.Sum256(data); return hex.EncodeToString(h[:]) }
	var inputs []scip.SourceInput
	var names []string
	for _, doc := range index.Documents {
		data, err := os.ReadFile(filepath.Join(root, doc.RelativePath))
		if err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, scip.SourceInput{Path: doc.RelativePath, SHA256: digest(data)})
		names = append(names, doc.RelativePath)
	}
	receipt := scip.SourceReceipt{Schema: scip.SourceReceiptSchema, Status: "complete", Root: root, IndexSHA256: digest(raw), Producer: scip.SourceProducer{Name: index.Metadata.ToolInfo.Name, Version: index.Metadata.ToolInfo.Version}, Inputs: inputs, Inventories: []scip.SourceInventory{{Root: ".", Pattern: "*.ts", Paths: names}}}
	data, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".receipt.json", data, 0644); err != nil {
		t.Fatal(err)
	}
}
func TestCmdIngest_SourceReceiptRefusalInvalidatesPriorTypedRows(t *testing.T) {
	for _, mutation := range []string{"source edit", "source addition", "missing receipt", "index corruption"} {
		t.Run(mutation, func(t *testing.T) {
			root := t.TempDir()
			writeTSFixture(t, root, true)
			path := filepath.Join(root, "index.scip")
			writeCmdSourceReceipt(t, root, path)
			store := filepath.Join(t.TempDir(), "s.sqlite")
			args := []string{"--service", "source-receipt", "--path", root, "--store", store, "--json"}
			var out, diagnostics bytes.Buffer
			if code := cmdIngest(args, &out, &diagnostics); code != 0 {
				t.Fatalf("fresh exit=%d %s", code, diagnostics.String())
			}
			switch mutation {
			case "source edit":
				if err := os.WriteFile(filepath.Join(root, "math.ts"), []byte("export const changed=1;"), 0644); err != nil {
					t.Fatal(err)
				}
			case "source addition":
				if err := os.WriteFile(filepath.Join(root, "new.ts"), []byte("export const added=1;"), 0644); err != nil {
					t.Fatal(err)
				}
			case "missing receipt":
				if err := os.Remove(path + ".receipt.json"); err != nil {
					t.Fatal(err)
				}
			case "index corruption":
				if err := os.WriteFile(path, []byte("broken"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			out.Reset()
			diagnostics.Reset()
			if code := cmdIngest(args, &out, &diagnostics); code == 0 || !strings.Contains(diagnostics.String(), "receipt") {
				t.Fatalf("stale exit=%d out=%s diagnostics=%s", code, out.String(), diagnostics.String())
			}
			c, err := client.NewConstantClient(client.ModeLocal, store)
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			edges, err := c.TypedEdges(context.Background(), "source-receipt")
			if err != nil {
				t.Fatal(err)
			}
			for _, edge := range edges {
				if edge.Kind == "direct" {
					t.Fatalf("stale direct edge survived refusal: %+v", edge)
				}
			}
		})
	}
}

func TestCmdIngest_SourceReceiptExplicitFirstImportFailsClosed(t *testing.T) {
	root := t.TempDir()
	writeTSFixture(t, root, true)
	original := filepath.Join(root, "index.scip")
	writeCmdSourceReceipt(t, root, original)
	directory := filepath.Join(root, ".cache")
	if err := os.MkdirAll(directory, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "typed.scip")
	for _, suffix := range []string{"", ".receipt.json"} {
		if err := os.Rename(original+suffix, path+suffix); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "math.ts"), []byte("export const changed=1;"), 0644); err != nil {
		t.Fatal(err)
	}
	args := []string{"--service", "source-explicit", "--path", root, "--store", filepath.Join(t.TempDir(), "s.sqlite"), "--scip", path, "--json"}
	var out, diagnostics bytes.Buffer
	if code := cmdIngest(args, &out, &diagnostics); code == 0 || !strings.Contains(diagnostics.String(), "receipt") {
		t.Fatalf("explicit stale exit=%d out=%s diagnostics=%s", code, out.String(), diagnostics.String())
	}
}
