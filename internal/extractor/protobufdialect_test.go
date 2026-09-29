package extractor

// protobufdialect_test.go — LB-Q-protobuf (design §13.2 steps 10, 12 and 15). The per-fixture goldens run in
// the generic TestDialectGoldens (dialects/protobuf/testdata); this file runs protoextract_test.go's cases —
// the incumbent's own fixture — against both extractors (§13.2 step 15: a port keeps every SourceKind string
// the incumbent emits), pins the one contract the dialect does not keep, and names what it adds.

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// protobufContractFixture is protoextract_test.go's protoFixture, verbatim.
const protobufContractFixture = `syntax = "proto3";

package greeter;

enum Status {
  UNKNOWN = 0;
  OK = 1;
}

message HelloRequest {
  string name = 1;
}

message HelloResponse {
  string message = 1;
  Status status = 2;
}

service Greeter {
  rpc SayHello (HelloRequest) returns (HelloResponse);
  rpc SayGoodbye (HelloRequest) returns (HelloResponse);
}
`

func protobufContractExtractors(t *testing.T) map[string]Extractor {
	t.Helper()
	d, err := NewTSExtractor("protobuf")
	if err != nil {
		t.Fatalf("NewTSExtractor(protobuf): %v", err)
	}
	t.Cleanup(d.Close)
	return map[string]Extractor{"incumbent": NewProtoExtractor(), "dialect": d}
}

// bareIndex keys nodes by their last dotted segment: the dialect names an rpc by its service
// (Greeter.SayHello) and a nested message by its parents, the incumbent by the bare name.
func bareIndex(nodes []core.SymbolNode) map[string]core.SymbolNode {
	m := map[string]core.SymbolNode{}
	for _, n := range nodes {
		m[n.Name[strings.LastIndex(n.Name, ".")+1:]] = n
	}
	return m
}

// TestProtobufDialectKeepsIncumbentContracts: protoextract_test.go's cases (TestProtoExtractor_Basic, _Determinism,
// _EmptyInput) hold for both extractors — the four kinds with their byte-identical SourceKind strings, the tier,
// determinism and an empty file — except the rpc Signature, pinned per extractor below.
func TestProtobufDialectKeepsIncumbentContracts(t *testing.T) {
	for who, e := range protobufContractExtractors(t) {
		t.Run(who+"/basic", func(t *testing.T) {
			nodes, err := e.Extract("svc", "greeter.proto", []byte(protobufContractFixture))
			if err != nil {
				t.Fatalf("Extract: %v", err)
			}
			byName := bareIndex(nodes)
			for name, kind := range map[string]core.SourceKind{
				"HelloRequest": core.KindProtoMessage, "HelloResponse": core.KindProtoMessage,
				"Status": core.KindProtoEnum, "Greeter": core.KindProtoService,
				"SayHello": core.KindProtoRPC, "SayGoodbye": core.KindProtoRPC,
			} {
				n, ok := byName[name]
				if !ok {
					t.Errorf("expected symbol %s", name)
					continue
				}
				if n.Kind != kind || n.Tier != core.TierVerified {
					t.Errorf("%s: kind %q tier %q, want %q verified", name, n.Kind, n.Tier, kind)
				}
			}
			if len(nodes) != 6 {
				t.Errorf("got %d symbols, want 6", len(nodes))
			}
		})
		t.Run(who+"/determinism", func(t *testing.T) {
			n1, err1 := e.Extract("svc", "greeter.proto", []byte(protobufContractFixture))
			n2, err2 := e.Extract("svc", "greeter.proto", []byte(protobufContractFixture))
			if err1 != nil || err2 != nil || len(n1) != len(n2) {
				t.Fatalf("non-deterministic: %v %v %d vs %d", err1, err2, len(n1), len(n2))
			}
			for i := range n1 {
				if n1[i].Name != n2[i].Name || n1[i].Kind != n2[i].Kind {
					t.Errorf("non-deterministic at [%d]: %+v vs %+v", i, n1[i], n2[i])
				}
			}
		})
		t.Run(who+"/empty", func(t *testing.T) {
			nodes, err := e.Extract("svc", "empty.proto", []byte(""))
			if err != nil || len(nodes) != 0 {
				t.Errorf("empty input: %d nodes, err %v; want 0, nil", len(nodes), err)
			}
		})
	}
}

// TestProtobufDialectRPCSignature pins the contract the dialect does not keep. The incumbent writes an rpc's
// Signature as "Request -> Response", text the source never spells; a tags-engine Signature is the definition's
// own source up to its body (design §3.3), so the dialect's is the rpc as written, both types and any stream
// keyword included. Retiring protoextract.go needs this contract changed or an engine feature (LB-Q-protobuf-rest).
func TestProtobufDialectRPCSignature(t *testing.T) {
	want := map[string]string{
		"incumbent": "HelloRequest -> HelloResponse",
		"dialect":   "rpc SayHello (HelloRequest) returns (HelloResponse)",
	}
	for who, e := range protobufContractExtractors(t) {
		nodes, err := e.Extract("svc", "greeter.proto", []byte(protobufContractFixture))
		if err != nil {
			t.Fatal(err)
		}
		if got := bareIndex(nodes)["SayHello"].Signature; got != want[who] {
			t.Errorf("%s: SayHello Signature %q, want %q", who, got, want[who])
		}
	}
}

// TestProtobufDialectBeyondIncumbent: what the dialect adds — an EndLine on every symbol, a nested message and
// enum named by their parents, an rpc named by its service, and the type references a message's fields and an
// rpc's request and response make, each named by the type's last identifier.
func TestProtobufDialectBeyondIncumbent(t *testing.T) {
	d, err := NewTSExtractor("protobuf")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	src := `syntax = "proto3";
package acme.v1;
message Outer {
  message Inner { acme.v1.Leaf leaf = 1; }
  enum Kind { KIND_UNSPECIFIED = 0; }
  map<string, Inner> by_name = 2;
}
message Leaf {}
service Svc {
  rpc Get(
      Outer)
    returns (stream Outer.Inner);
}
`
	nodes, err := d.Extract("svc", "api/acme.proto", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]core.SymbolNode{}
	for _, n := range nodes {
		got[n.Name] = n
		if n.EndLine < n.Line {
			t.Errorf("%s: EndLine %d before Line %d", n.Name, n.EndLine, n.Line)
		}
	}
	for name, calls := range map[string]string{
		"Outer": "Inner", "Outer.Inner": "Leaf", "Outer.Kind": "", "Leaf": "", "Svc": "", "Svc.Get": "Inner,Outer",
	} {
		n, ok := got[name]
		if !ok {
			t.Errorf("no symbol %q (got %v)", name, nodes)
			continue
		}
		if c := strings.Join(n.Calls, ","); c != calls {
			t.Errorf("%s: references %q, want %q", name, c, calls)
		}
	}
	if n := got["Svc.Get"]; n.Line != 10 || n.EndLine != 12 || n.Signature != "rpc Get( Outer) returns (stream Outer.Inner)" {
		t.Errorf("Svc.Get: lines %d-%d signature %q", n.Line, n.EndLine, n.Signature)
	}
}
