package extractor

// graphqldialect_test.go -- LB-Q-graphql (design 13.2 steps 10, 12 and 15). The per-fixture goldens run in the
// generic TestDialectGoldens (dialects/graphql/testdata). This file holds the contracts graphqlextract.go's own
// tests pinned (the SourceKind strings, the tier, determinism, an empty file, the .gql extension), pins the one
// contract the dialect does not keep (a type's Signature) and names what the dialect adds.

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// graphqlContractFixture is graphqlextract_test.go's graphqlFixture, verbatim.
const graphqlContractFixture = `
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

func graphqlDialect(t *testing.T) *TSExtractor {
	t.Helper()
	d, err := NewTSExtractor("graphql")
	if err != nil {
		t.Fatalf("NewTSExtractor(graphql): %v", err)
	}
	t.Cleanup(d.Close)
	return d
}

func graphqlByKindName(nodes []core.SymbolNode) map[string]core.SymbolNode {
	m := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		m[string(n.Kind)+":"+n.Name] = n
	}
	return m
}

// TestGraphQLDialectKeepsIncumbentContracts: TestGraphQLExtractor_Basic, _Determinism, _EmptyInput and
// _GQLExtension hold for the dialect: GraphQLType for every type definition, GraphQLField named Type.field for the
// fields of an object, interface or input type, no field node for an enum value, tier verified.
func TestGraphQLDialectKeepsIncumbentContracts(t *testing.T) {
	d := graphqlDialect(t)
	for _, rel := range []string{"schema.graphql", "schema.gql"} {
		nodes, err := d.Extract("svc", rel, []byte(graphqlContractFixture))
		if err != nil {
			t.Fatalf("Extract %s: %v", rel, err)
		}
		by := graphqlByKindName(nodes)
		for _, name := range []string{"Query", "User", "CreateUserInput", "Role"} {
			n, ok := by[string(core.KindGraphQLType)+":"+name]
			if !ok {
				t.Errorf("%s: no GraphQLType %s", rel, name)
				continue
			}
			if n.Tier != core.TierVerified {
				t.Errorf("%s: %s tier %q, want verified", rel, name, n.Tier)
			}
		}
		for _, name := range []string{"Query.user", "Query.users", "User.name", "CreateUserInput.email"} {
			if _, ok := by[string(core.KindGraphQLField)+":"+name]; !ok {
				t.Errorf("%s: no GraphQLField %s", rel, name)
			}
		}
		if _, ok := by[string(core.KindGraphQLField)+":Role.ADMIN"]; ok {
			t.Errorf("%s: an enum value is no GraphQLField", rel)
		}
		if _, ok := by["GraphQLEnumValue"+":Role.ADMIN"]; !ok {
			t.Errorf("%s: no GraphQLEnumValue Role.ADMIN", rel)
		}
	}
	n1, err1 := d.Extract("svc", "schema.graphql", []byte(graphqlContractFixture))
	n2, err2 := d.Extract("svc", "schema.graphql", []byte(graphqlContractFixture))
	if err1 != nil || err2 != nil || len(n1) != len(n2) {
		t.Fatalf("non-deterministic: %v %v %d vs %d", err1, err2, len(n1), len(n2))
	}
	for i := range n1 {
		if n1[i].Name != n2[i].Name || n1[i].Kind != n2[i].Kind {
			t.Errorf("non-deterministic at [%d]: %+v vs %+v", i, n1[i], n2[i])
		}
	}
	if nodes, err := d.Extract("svc", "empty.graphql", []byte("")); err != nil || len(nodes) != 0 {
		t.Errorf("empty input: %d nodes, err %v; want 0, nil", len(nodes), err)
	}
}

// TestGraphQLDialectSignatureContract pins the contract the dialect does not keep. The incumbent wrote a type's
// Signature as the lowercased kind of the definition ("object", "input_object", "enum") and a field's as its type
// string ("String!"); a tags-engine Signature is the definition's own source up to its body (design 3.3), so a
// type's is its header ("type Query") and a field's the field as written ("name: String!"). A described
// definition's Signature starts at its description, which the grammar makes part of the definition node.
func TestGraphQLDialectSignatureContract(t *testing.T) {
	nodes, err := graphqlDialect(t).Extract("svc", "schema.graphql", []byte(graphqlContractFixture))
	if err != nil {
		t.Fatal(err)
	}
	by := graphqlByKindName(nodes)
	for key, want := range map[string]string{
		"GraphQLType:Query":                 "type Query",
		"GraphQLType:CreateUserInput":       "input CreateUserInput",
		"GraphQLType:Role":                  "enum Role",
		"GraphQLField:User.name":            "name: String!",
		"GraphQLField:Query.user":           "user(id: ID!): User",
		"GraphQLEnumValue:Role.ADMIN":       "ADMIN",
		"GraphQLField:CreateUserInput.name": "name: String!",
	} {
		if got := by[key].Signature; got != want {
			t.Errorf("%s: Signature %q, want %q", key, got, want)
		}
	}
}

// TestGraphQLDialectBeyondIncumbent: what the dialect adds -- an EndLine on every symbol, an enum value, a
// directive definition, a type extension named by its target (its fields qualify under it), a named operation and
// a fragment, and the type references each makes (a built-in scalar is none), where the incumbent read a schema
// only and returned nothing for a document holding an operation.
func TestGraphQLDialectBeyondIncumbent(t *testing.T) {
	src := `directive @tag(of: Label) on FIELD

extend type Query {
  me: User
}

query Me($by: Filter, $n: Int) {
  me { ...Who ... on Admin { level } }
}

fragment Who on User { id }
`
	nodes, err := graphqlDialect(t).Extract("svc", "api/q.graphql", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]core.SymbolNode{}
	for _, n := range nodes {
		got[string(n.Kind)+":"+n.Name] = n
		if n.EndLine < n.Line {
			t.Errorf("%s: EndLine %d before Line %d", n.Name, n.EndLine, n.Line)
		}
	}
	for key, calls := range map[string]string{
		"GraphQLDirective:tag":   "Label",
		"GraphQLExtension:Query": "",
		"GraphQLField:Query.me":  "User",
		"GraphQLOperation:Me":    "Admin,Filter,Who",
		"GraphQLFragment:Who":    "User",
	} {
		n, ok := got[key]
		if !ok {
			t.Errorf("no symbol %q (got %v)", key, nodes)
			continue
		}
		if c := strings.Join(n.Calls, ","); c != calls {
			t.Errorf("%s: references %q, want %q", key, c, calls)
		}
	}
	if n := got["GraphQLOperation:Me"]; n.Line != 7 || n.EndLine != 9 || n.Signature != "query Me($by: Filter, $n: Int)" {
		t.Errorf("Me: lines %d-%d signature %q", n.Line, n.EndLine, n.Signature)
	}
}
