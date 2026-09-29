package fusion_test

// protobuf_ingest_test.go — LB-Q-protobuf (design §13.2 step 13): the protobuf dialect's imports.scm resolves
// across two fixture files through its manifest's file-symbol strategy — an import path is a file under the
// repository root, and a whole-file import brings the target's top-level definitions into scope, so a field or
// an rpc that names one of them (package-qualified or not) gets a file-resolved edge to it. A well-known
// google/protobuf import has no file in the repository and resolves nowhere.
//
// The dialect is built beside protoextract.go, which stays .proto's dispatched extractor until the port is
// switched (design §11 step 2: lang-parity refuses the port while its L3 target is unmet), so this proves the
// strategy on the dialect's own extraction; ingest resolves no .proto edge until the switch.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
)

func TestResolveFileEdges_ProtobufImports(t *testing.T) {
	dir := t.TempDir()
	for rel, src := range map[string]string{
		"acme/v1/common.proto": `syntax = "proto3";
package acme.v1;

message Money {
  string currency = 1;
  int64 units = 2;
}

enum Currency {
  CURRENCY_UNSPECIFIED = 0;
}
`,
		"acme/v1/order.proto": `syntax = "proto3";
package acme.v1;

import "acme/v1/common.proto";
import "google/protobuf/timestamp.proto";

message Order {
  acme.v1.Money total = 1;
  Currency currency = 2;
  google.protobuf.Timestamp placed_at = 3;
}

service Orders {
  rpc Price(Order) returns (Money);
}
`,
	} {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	files, symbols := loadFileResolveFixture(t, dir, "protobuf", []string{"acme/v1/common.proto", "acme/v1/order.proto"})
	key := map[string]string{}
	for _, s := range symbols {
		key[s.Name] = s.Key()
	}

	edges, report := fusion.ResolveFileEdges(context.Background(), "protobuf", dir, files, symbols)
	if report.Skipped {
		t.Fatalf("unexpectedly skipped: %s", report.SkipReason)
	}
	if report.FilesScanned != 2 || report.ImportsParsed != 2 {
		t.Errorf("FilesScanned %d ImportsParsed %d, want 2 and 2", report.FilesScanned, report.ImportsParsed)
	}
	for _, e := range edges {
		if e.Kind != core.EdgeFileResolved || e.FromFile != "acme/v1/order.proto" {
			t.Errorf("edge %+v: want a file-resolved edge from acme/v1/order.proto", e)
		}
	}
	got := fileResolveEdgeSetAll(edges)
	// Order's `acme.v1.Money` (a type named by its last identifier) and `Currency`, and the rpc's response,
	// resolve into common.proto, and the rpc's request to Order in its own file; Timestamp's import names no
	// file of the repository.
	want := []string{
		key["Order"] + " -> " + key["Money"],
		key["Order"] + " -> " + key["Currency"],
		key["Orders.Price"] + " -> " + key["Money"],
		key["Orders.Price"] + " -> " + key["Order"],
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("missing edge %s; got %v", w, got)
		}
	}
	if len(got) != len(want) {
		t.Errorf("got %d edges %v, want %d %v", len(got), got, len(want), want)
	}
}
