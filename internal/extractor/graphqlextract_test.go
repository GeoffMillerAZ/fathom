package extractor_test

import (
	"testing"

	"github.com/geoffmilleraz/sextant/internal/core"
	"github.com/geoffmilleraz/sextant/internal/extractor"
)

const graphqlFixture = `
type Query {
  user(id: ID!): User
  users: [User!]!
}

type User {
  id: ID!
  name: String!
  email: String
}

input CreateUserInput {
  name: String!
  email: String!
}

enum Role {
  ADMIN
  MEMBER
  GUEST
}
`

func TestGraphQLExtractor_Basic(t *testing.T) {
	x := extractor.NewGraphQLExtractor()
	nodes, err := x.Extract("svc", "schema.graphql", []byte(graphqlFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		// Use Kind+Name as composite key since fields have qualified names.
		byName[string(n.Kind)+":"+n.Name] = n
	}

	t.Run("type Query definition", func(t *testing.T) {
		n, ok := byName[string(core.KindGraphQLType)+":Query"]
		if !ok {
			t.Fatal("expected KindGraphQLType node for Query")
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
		if n.Signature != "object" {
			t.Errorf("Signature: got %q, want %q", n.Signature, "object")
		}
	})

	t.Run("type User definition", func(t *testing.T) {
		n, ok := byName[string(core.KindGraphQLType)+":User"]
		if !ok {
			t.Fatal("expected KindGraphQLType node for User")
		}
		if n.Kind != core.KindGraphQLType {
			t.Errorf("Kind: got %q, want KindGraphQLType", n.Kind)
		}
		if n.Signature != "object" {
			t.Errorf("Signature: got %q, want %q", n.Signature, "object")
		}
	})

	t.Run("input CreateUserInput definition", func(t *testing.T) {
		n, ok := byName[string(core.KindGraphQLType)+":CreateUserInput"]
		if !ok {
			t.Fatal("expected KindGraphQLType node for CreateUserInput")
		}
		if n.Signature != "input_object" {
			t.Errorf("Signature: got %q, want %q", n.Signature, "input_object")
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
	})

	t.Run("enum Role definition", func(t *testing.T) {
		n, ok := byName[string(core.KindGraphQLType)+":Role"]
		if !ok {
			t.Fatal("expected KindGraphQLType node for Role")
		}
		if n.Signature != "enum" {
			t.Errorf("Signature: got %q, want %q", n.Signature, "enum")
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
	})

	t.Run("field Query.user", func(t *testing.T) {
		n, ok := byName[string(core.KindGraphQLField)+":Query.user"]
		if !ok {
			t.Fatal("expected KindGraphQLField node for Query.user")
		}
		if n.Kind != core.KindGraphQLField {
			t.Errorf("Kind: got %q, want KindGraphQLField", n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
	})

	t.Run("field Query.users", func(t *testing.T) {
		_, ok := byName[string(core.KindGraphQLField)+":Query.users"]
		if !ok {
			t.Fatal("expected KindGraphQLField node for Query.users")
		}
	})

	t.Run("field User.name", func(t *testing.T) {
		n, ok := byName[string(core.KindGraphQLField)+":User.name"]
		if !ok {
			t.Fatal("expected KindGraphQLField node for User.name")
		}
		if n.Signature != "String!" {
			t.Errorf("Signature: got %q, want %q", n.Signature, "String!")
		}
	})

	t.Run("input field CreateUserInput.email", func(t *testing.T) {
		n, ok := byName[string(core.KindGraphQLField)+":CreateUserInput.email"]
		if !ok {
			t.Fatal("expected KindGraphQLField node for CreateUserInput.email")
		}
		if n.Kind != core.KindGraphQLField {
			t.Errorf("Kind: got %q, want KindGraphQLField", n.Kind)
		}
	})

	// Enum values should NOT produce field nodes (only OBJECT, INPUT_OBJECT, INTERFACE).
	t.Run("enum has no field nodes", func(t *testing.T) {
		_, ok := byName[string(core.KindGraphQLField)+":Role.ADMIN"]
		if ok {
			t.Error("did not expect KindGraphQLField node for Role.ADMIN (enum values are not fields)")
		}
	})
}

func TestGraphQLExtractor_Determinism(t *testing.T) {
	x := extractor.NewGraphQLExtractor()
	n1, err := x.Extract("svc", "schema.graphql", []byte(graphqlFixture))
	if err != nil {
		t.Fatalf("first Extract: %v", err)
	}
	n2, err := x.Extract("svc", "schema.graphql", []byte(graphqlFixture))
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

func TestGraphQLExtractor_EmptyInput(t *testing.T) {
	x := extractor.NewGraphQLExtractor()
	nodes, err := x.Extract("svc", "empty.graphql", []byte(""))
	if err != nil {
		t.Fatalf("Extract empty: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes for empty input, got %d", len(nodes))
	}
}

func TestGraphQLExtractor_GQLExtension(t *testing.T) {
	x := extractor.NewGraphQLExtractor()
	nodes, err := x.Extract("svc", "schema.gql", []byte(graphqlFixture))
	if err != nil {
		t.Fatalf("Extract .gql: %v", err)
	}
	if len(nodes) == 0 {
		t.Error("expected nodes for .gql file")
	}
}
