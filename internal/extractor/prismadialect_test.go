package extractor

// prismadialect_test.go -- LB-Q-prisma (design 13.2 steps 10 and 12). The per-fixture goldens run in the generic
// TestDialectGoldens (dialects/prisma/testdata). This file pins the contracts the goldens cannot: the SourceKind
// strings, a block and its members qualifying under it, the references a field makes (a type it names and the
// fields an @relation names), the .prisma extension for any directory, determinism and an empty file.

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

const prismaContractFixture = `datasource db {
  provider = "postgresql"
  url      = env("DATABASE_URL")
}

generator client {
  provider = "prisma-client-js"
}

enum Role {
  USER
  ADMIN
}

model User {
  id    Int    @id
  role  Role   @default(USER)
  posts Post[]
}

model Post {
  id       Int    @id
  author   User   @relation(fields: [authorId], references: [id])
  authorId Int
}
`

func prismaDialect(t *testing.T) *TSExtractor {
	t.Helper()
	d, err := NewTSExtractor("prisma")
	if err != nil {
		t.Fatalf("NewTSExtractor(prisma): %v", err)
	}
	t.Cleanup(d.Close)
	return d
}

func prismaByKindName(nodes []core.SymbolNode) map[string]core.SymbolNode {
	m := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		m[string(n.Kind)+":"+n.Name] = n
	}
	return m
}

// TestPrismaDialectKindsAndReferences: every block kind, a member qualified under its block, and the references a
// field makes -- the model, enum, view or composite type its type names (a built-in scalar is none) and, for an
// @relation, the fields its fields: and references: lists name.
func TestPrismaDialectKindsAndReferences(t *testing.T) {
	for _, rel := range []string{"schema.prisma", "prisma/schema/user.prisma"} {
		nodes, err := prismaDialect(t).Extract("svc", rel, []byte(prismaContractFixture))
		if err != nil {
			t.Fatalf("Extract %s: %v", rel, err)
		}
		by := prismaByKindName(nodes)
		for _, key := range []string{
			"PrismaDatasource:db", "PrismaGenerator:client", "PrismaEnum:Role", "PrismaEnumValue:Role.ADMIN",
			"PrismaModel:User", "PrismaModel:Post", "PrismaField:User.id", "PrismaField:Post.author",
		} {
			n, ok := by[key]
			if !ok {
				t.Errorf("%s: no %s", rel, key)
				continue
			}
			if n.Tier != core.TierVerified || n.EndLine < n.Line {
				t.Errorf("%s: %s tier %q lines %d-%d", rel, key, n.Tier, n.Line, n.EndLine)
			}
		}
		for key, calls := range map[string]string{
			"PrismaField:User.role":     "Role",
			"PrismaField:User.posts":    "Post",
			"PrismaField:User.id":       "",
			"PrismaField:Post.author":   "User,authorId,id",
			"PrismaField:Post.authorId": "",
		} {
			if got := strings.Join(by[key].Calls, ","); got != calls {
				t.Errorf("%s: %s references %q, want %q", rel, key, got, calls)
			}
		}
	}
}

// TestPrismaDialectDeterministicAndEmpty: two extractions agree and an empty file extracts nothing.
func TestPrismaDialectDeterministicAndEmpty(t *testing.T) {
	d := prismaDialect(t)
	n1, err1 := d.Extract("svc", "schema.prisma", []byte(prismaContractFixture))
	n2, err2 := d.Extract("svc", "schema.prisma", []byte(prismaContractFixture))
	if err1 != nil || err2 != nil || len(n1) != len(n2) {
		t.Fatalf("non-deterministic: %v %v %d vs %d", err1, err2, len(n1), len(n2))
	}
	for i := range n1 {
		if n1[i].Name != n2[i].Name || n1[i].Kind != n2[i].Kind || n1[i].Line != n2[i].Line {
			t.Errorf("non-deterministic at [%d]: %+v vs %+v", i, n1[i], n2[i])
		}
	}
	if nodes, err := d.Extract("svc", "empty.prisma", []byte("")); err != nil || len(nodes) != 0 {
		t.Errorf("empty input: %d nodes, err %v; want 0, nil", len(nodes), err)
	}
}
