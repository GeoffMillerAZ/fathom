//go:build fathom_grammar_audit

package tsengine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func auditCPPFixture(t *testing.T) ([]byte, GrammarAuditSpec) {
	t.Helper()
	raw, err := openGrammarBlob("cpp")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	return raw, GrammarAuditSpec{Blob: "cpp", EntryPoint: "tree_sitter_cpp", ABI: 14, Bytes: len(raw), SHA256: hex.EncodeToString(sum[:]), Licence: "MIT"}
}

func TestGrammarAuditParserEmbeddedCPPBaseline(t *testing.T) {
	raw, spec := auditCPPFixture(t)
	base := LiveModules()
	ownedBase := ownedModules.Load()
	p, err := NewGrammarAuditParser(context.Background(), raw, spec)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := p.Parse([]byte("int main() { return 0; }\n"))
	if err != nil {
		t.Fatal(err)
	}
	tree.Close()
	q, err := p.CompileQuery("(ERROR) @error")
	if err != nil {
		t.Fatal(err)
	}
	q.Close()
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if got := LiveModules(); got != base {
		t.Fatalf("live module count %d, want %d", got, base)
	}
	if got := ownedModules.Load(); got != ownedBase {
		t.Fatalf("owned module count %d, want %d", got, ownedBase)
	}
}

func TestGrammarAuditRefusesInvalidIdentityBeforeInstantiation(t *testing.T) {
	raw, spec := auditCPPFixture(t)
	base := LiveModules()
	ownedBase := ownedModules.Load()
	for _, change := range []func(*GrammarAuditSpec){
		func(s *GrammarAuditSpec) { s.SHA256 = "0" + s.SHA256[1:] },
		func(s *GrammarAuditSpec) { s.Bytes++ },
		func(s *GrammarAuditSpec) { s.Licence = "GPL-3.0" },
		func(s *GrammarAuditSpec) { s.ABI = 0x57420006 },
		func(s *GrammarAuditSpec) { s.EntryPoint = "tree_sitter_c" },
	} {
		bad := spec
		change(&bad)
		if p, err := NewGrammarAuditParser(context.Background(), raw, bad); err == nil {
			p.Close()
			t.Fatal("accepted invalid audit identity")
		}
		if got := LiveModules(); got != base {
			t.Fatalf("live module count %d, want %d", got, base)
		}
		if got := ownedModules.Load(); got != ownedBase {
			t.Fatalf("owned module count %d, want %d", got, ownedBase)
		}
	}
}

func TestGrammarAuditWholeModuleCap(t *testing.T) {
	for _, size := range []int{maxGrammarAuditBytes, maxGrammarAuditBytes + 1} {
		raw := make([]byte, size)
		sum := sha256.Sum256(raw)
		spec := GrammarAuditSpec{Blob: "cpp", EntryPoint: "tree_sitter_cpp", ABI: 14, Bytes: len(raw), SHA256: hex.EncodeToString(sum[:]), Licence: "MIT"}
		p, err := NewGrammarAuditParser(context.Background(), raw, spec)
		if p != nil {
			p.Close()
		}
		if err == nil {
			t.Fatalf("invalid WASM accepted at size %d", size)
		}
		if size == maxGrammarAuditBytes && !strings.Contains(err.Error(), "audit compile:") {
			t.Fatalf("equality must pass identity and reach compilation: %v", err)
		}
		if size > maxGrammarAuditBytes && !strings.Contains(err.Error(), "invalid C++ audit module identity") {
			t.Fatalf("cap+1 must fail before compilation: %v", err)
		}
	}
}
