package extractor_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

const protoFixture = `syntax = "proto3";

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

func TestProtoExtractor_Basic(t *testing.T) {
	x := extractor.NewProtoExtractor()
	nodes, err := x.Extract("svc", "greeter.proto", []byte(protoFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	t.Run("message HelloRequest", func(t *testing.T) {
		n, ok := byName["HelloRequest"]
		if !ok {
			t.Fatal("expected symbol HelloRequest")
		}
		if n.Kind != core.KindProtoMessage {
			t.Errorf("Kind: got %q, want KindProtoMessage", n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
	})

	t.Run("message HelloResponse", func(t *testing.T) {
		n, ok := byName["HelloResponse"]
		if !ok {
			t.Fatal("expected symbol HelloResponse")
		}
		if n.Kind != core.KindProtoMessage {
			t.Errorf("Kind: got %q, want KindProtoMessage", n.Kind)
		}
	})

	t.Run("enum Status", func(t *testing.T) {
		n, ok := byName["Status"]
		if !ok {
			t.Fatal("expected symbol Status")
		}
		if n.Kind != core.KindProtoEnum {
			t.Errorf("Kind: got %q, want KindProtoEnum", n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
	})

	t.Run("service Greeter", func(t *testing.T) {
		n, ok := byName["Greeter"]
		if !ok {
			t.Fatal("expected symbol Greeter")
		}
		if n.Kind != core.KindProtoService {
			t.Errorf("Kind: got %q, want KindProtoService", n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
	})

	t.Run("rpc SayHello signature", func(t *testing.T) {
		n, ok := byName["SayHello"]
		if !ok {
			t.Fatal("expected symbol SayHello")
		}
		if n.Kind != core.KindProtoRPC {
			t.Errorf("Kind: got %q, want KindProtoRPC", n.Kind)
		}
		want := "HelloRequest -> HelloResponse"
		if n.Signature != want {
			t.Errorf("Signature: got %q, want %q", n.Signature, want)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
	})

	t.Run("rpc SayGoodbye", func(t *testing.T) {
		n, ok := byName["SayGoodbye"]
		if !ok {
			t.Fatal("expected symbol SayGoodbye")
		}
		if n.Kind != core.KindProtoRPC {
			t.Errorf("Kind: got %q, want KindProtoRPC", n.Kind)
		}
		want := "HelloRequest -> HelloResponse"
		if n.Signature != want {
			t.Errorf("Signature: got %q, want %q", n.Signature, want)
		}
	})
}

func TestProtoExtractor_Determinism(t *testing.T) {
	x := extractor.NewProtoExtractor()
	n1, err := x.Extract("svc", "greeter.proto", []byte(protoFixture))
	if err != nil {
		t.Fatalf("first Extract: %v", err)
	}
	n2, err := x.Extract("svc", "greeter.proto", []byte(protoFixture))
	if err != nil {
		t.Fatalf("second Extract: %v", err)
	}
	if len(n1) != len(n2) {
		t.Fatalf("non-deterministic length: %d vs %d", len(n1), len(n2))
	}
	for i := range n1 {
		if n1[i].Name != n2[i].Name || n1[i].Kind != n2[i].Kind {
			t.Errorf("non-deterministic at [%d]: %+v vs %+v", i, n1[i], n2[i])
		}
	}
}

func TestProtoExtractor_EmptyInput(t *testing.T) {
	x := extractor.NewProtoExtractor()
	nodes, err := x.Extract("svc", "empty.proto", []byte(""))
	if err != nil {
		t.Fatalf("Extract empty: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes for empty input, got %d", len(nodes))
	}
}
