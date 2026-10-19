package fusion

// graphql_dialect_test.go -- LB-Q-graphql: .graphql and .gql reach the graphql tags dialect through the
// registry the dialect manifests build, once graphqlextract.go is retired (lang-parity MERGE,
// docs/bench/readings/lang/parity/graphql.json).

import (
	"context"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// graphqlDialectCorpus is a schema, a file that extends it and an operation document: the shapes the
// incumbent read (types, fields) and the ones only the dialect does (an extension, an operation, a fragment,
// an enum value).
func graphqlDialectCorpus() map[string]string {
	return map[string]string{
		"api/schema.graphql":  "type Query {\n  user(id: ID!): User\n}\n\ntype User {\n  id: ID!\n  role: Role\n}\n\nenum Role {\n  ADMIN\n  MEMBER\n}\n",
		"api/extra.gql":       "extend type Query {\n  me: User\n}\n",
		"web/queries.graphql": "query Me {\n  me {\n    ...Who\n  }\n}\n\nfragment Who on User {\n  id\n}\n",
	}
}

// TestGraphQLDialectIsWhatFusionDispatches: the hand lines for .graphql and .gql in fusion.New are gone and the
// manifest loop claims both extensions.
func TestGraphQLDialectIsWhatFusionDispatches(t *testing.T) {
	p := New(connector.NewLocalConnector(), store.NewMemoryStore(), Config{})
	t.Cleanup(p.Close)
	for rel, src := range graphqlDialectCorpus() {
		e, _, ok, unclassified := p.resolveExtractor(rel, []byte(src), buildSiblingIndex(nil))
		if !ok || unclassified {
			t.Fatalf("%s: ok=%v unclassified=%v", rel, ok, unclassified)
		}
		if ts, isTS := e.(*extractor.TSExtractor); !isTS || ts.Name() != "graphql" {
			t.Errorf("%s dispatches to %T, want the graphql tags dialect", rel, e)
		}
	}
	for _, ext := range []string{".graphql", ".gql"} {
		if got := DispatchTable()[ext]; got != "tags dialect `graphql`" {
			t.Errorf("DispatchTable[%s] = %q, want tags dialect `graphql`", ext, got)
		}
	}
}

// TestIngest_GraphQLDialect is the product-reach proof: an ingest stores the schema's types and fields under
// the incumbent's SourceKinds, and what only the dialect reads (an extension's field, an enum value, an
// operation, a fragment) with the references they make.
func TestIngest_GraphQLDialect(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, graphqlDialectCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "gqlx", Path: dir}); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "gqlx", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]core.Observation{}
	for _, o := range obs {
		got[string(o.SourceKind)+"|"+o.Symbol] = o
	}
	for _, want := range []string{
		"GraphQLType|Query", "GraphQLType|User", "GraphQLType|Role", "GraphQLField|Query.user", "GraphQLField|User.role",
		"GraphQLEnumValue|Role.ADMIN", "GraphQLExtension|Query", "GraphQLField|Query.me",
		"GraphQLOperation|Me", "GraphQLFragment|Who",
	} {
		if _, ok := got[want]; !ok {
			var have []string
			for k := range got {
				have = append(have, k)
			}
			sort.Strings(have)
			t.Errorf("%s not stored; have %q", want, have)
		}
	}
	if c := got["GraphQLField|User.role"].Calls; len(c) != 1 || c[0] != "Role" {
		t.Errorf("User.role calls = %q, want [Role]", c)
	}
	if c := got["GraphQLOperation|Me"].Calls; len(c) != 1 || c[0] != "Who" {
		t.Errorf("Me calls = %q, want [Who]", c)
	}
}
