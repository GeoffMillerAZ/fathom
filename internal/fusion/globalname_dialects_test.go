package fusion_test

// globalname_dialects_test.go — LB-Q-graphql and LB-Q-prisma (design §13.2 step 13): each dialect's imports.scm
// captures its type references, and the manifest's global-name strategy (LB-F580) binds each to the one file of
// its scope defining the type: every GraphQL file of the service, the .prisma files of the importer's folder.

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
)

func writeGlobalNameFixture(t *testing.T, files map[string]string) (string, []string) {
	t.Helper()
	dir := t.TempDir()
	var rels []string
	for rel, src := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		rels = append(rels, rel)
	}
	sort.Strings(rels)
	return dir, rels
}

// globalNameFileEdges is the resolved edges as "from file: from name -> to file: to name", cross-file only.
func globalNameFileEdges(edges []core.Edge, symbols []core.SymbolNode) map[string]bool {
	byKey := map[string]core.SymbolNode{}
	for _, s := range symbols {
		if _, dup := byKey[s.Key()]; !dup {
			byKey[s.Key()] = s
		}
	}
	out := map[string]bool{}
	for _, e := range edges {
		from, to := byKey[e.FromSymbol], byKey[e.ToSymbol]
		if to.FilePath == e.FromFile {
			continue
		}
		out[e.FromFile+": "+from.Name+" -> "+to.FilePath+": "+to.Name] = true
	}
	return out
}

func TestResolveFileEdges_GraphQLGlobalName(t *testing.T) {
	dir, rels := writeGlobalNameFixture(t, map[string]string{
		"schema/user.graphql": `type User implements Node {
  id: ID!
  posts(first: Int): [Post!]!
  role: Role
}

interface Node {
  id: ID!
}
`,
		"schema/post.graphql": `type Post implements Node {
  id: ID!
  author: User!
}

enum Role {
  ADMIN
}

union SearchResult = User | Post
`,
		"schema/query.graphql": `type Query {
  me: User
  search(term: String!): [SearchResult!]!
}
`,
		"schema/extend.graphql": `extend type Query {
  post(id: ID!): Post
}
`,
		"ops/me.graphql": `query Me($id: ID!) {
  me {
    ...UserParts
  }
}

fragment UserParts on User {
  id
  ... on Node {
    id
  }
}
`,
		"other/schema.graphql": `type Post {
  title: String
}
`,
	})
	files, symbols := loadFileResolveFixture(t, dir, "graphql", rels)
	edges, report := fusion.ResolveFileEdges(context.Background(), "graphql", dir, files, symbols)
	if report.Skipped {
		t.Fatalf("unexpectedly skipped: %s", report.SkipReason)
	}
	got := globalNameFileEdges(edges, symbols)
	// Post is defined twice (schema/post.graphql, other/schema.graphql): every reference to it binds nothing.
	// Query is defined once and extended once; the extension is no second definition. A built-in scalar binds
	// nothing; a type the file itself defines (Node in user.graphql) is no cross-file edge.
	want := []string{
		"schema/post.graphql: Post -> schema/user.graphql: Node",
		"schema/post.graphql: Post.author -> schema/user.graphql: User",
		"schema/post.graphql: SearchResult -> schema/user.graphql: User",
		"schema/user.graphql: User.role -> schema/post.graphql: Role",
		"schema/query.graphql: Query.me -> schema/user.graphql: User",
		"schema/query.graphql: Query.search -> schema/post.graphql: SearchResult",
		"ops/me.graphql: UserParts -> schema/user.graphql: User",
		"ops/me.graphql: UserParts -> schema/user.graphql: Node",
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("missing edge %s", w)
		}
	}
	if len(got) != len(want) {
		t.Errorf("got %d cross-file edges %v, want %d", len(got), got, len(want))
	}
	if report.GlobalNameAmbiguous != 2 {
		t.Errorf("GlobalNameAmbiguous = %d, want 2 (Post from user.graphql and from extend.graphql)", report.GlobalNameAmbiguous)
	}
}

func TestResolveFileEdges_PrismaGlobalName(t *testing.T) {
	dir, rels := writeGlobalNameFixture(t, map[string]string{
		"prisma/schema/base.prisma": `datasource db {
  provider = "postgresql"
  url      = env("DATABASE_URL")
}

generator client {
  provider = "prisma-client-js"
}
`,
		"prisma/schema/user.prisma": `model User {
  id    String @id
  role  Role
  posts Post[]
  address Address?
}

enum Role {
  ADMIN
  USER
}
`,
		"prisma/schema/post.prisma": `model Post {
  id       String @id
  authorId String
  author   User   @relation(fields: [authorId], references: [id])
}

type Address {
  street String
}
`,
		"apps/other/schema.prisma": `model User {
  id   Int   @id
  role Role
}
`,
	})
	files, symbols := loadFileResolveFixture(t, dir, "prisma", rels)
	edges, report := fusion.ResolveFileEdges(context.Background(), "prisma", dir, files, symbols)
	if report.Skipped {
		t.Fatalf("unexpectedly skipped: %s", report.SkipReason)
	}
	got := globalNameFileEdges(edges, symbols)
	// apps/other's schema is another folder: its User is no second definition for prisma/schema, and its Role
	// (defined only in prisma/schema) binds nothing.
	want := []string{
		"prisma/schema/user.prisma: User.posts -> prisma/schema/post.prisma: Post",
		"prisma/schema/user.prisma: User.address -> prisma/schema/post.prisma: Address",
		"prisma/schema/post.prisma: Post.author -> prisma/schema/user.prisma: User",
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("missing edge %s", w)
		}
	}
	if len(got) != len(want) {
		t.Errorf("got %d cross-file edges %v, want %d", len(got), got, len(want))
	}
	if report.GlobalNameAmbiguous != 0 || report.GlobalNameBound != 3 {
		t.Errorf("report bound=%d ambiguous=%d, want 3 and 0", report.GlobalNameBound, report.GlobalNameAmbiguous)
	}
}
