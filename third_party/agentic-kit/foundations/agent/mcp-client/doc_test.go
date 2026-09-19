package mcpclient

import (
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strings"
	"testing"
)

// pinAdmission is the sentence fragment CallTool's own comment uses to state
// that it does not verify the description pin. The package doc must carry the
// same admission, so the two can never drift apart silently.
const pinAdmission = "performs NO pin verification of its own"

// pinOverclaims are the phrasings by which the per-call pin-verification
// claim has returned, or would return, to the package doc. Each is matched
// against the whitespace-normalized doc text.
var pinOverclaims = []*regexp.Regexp{
	regexp.MustCompile(`(?i)every tools/call`),
	regexp.MustCompile(`(?i)tools/call[^.]*\bverif(y|ies|ied)\b`),
	regexp.MustCompile(`(?i)CallTool[^.]*\bverif(y|ies|ied)\b[^.]*\bpin`),
}

// TestDocDoesNotClaimPinVerification keeps the legacy CallTool guarantee
// separate from the opt-in CallToolVerified guarantee. The package doc must
// say which path verifies metadata and must not claim remote atomicity.
func TestDocDoesNotClaimPinVerification(t *testing.T) {
	fset := token.NewFileSet()

	docFile, err := parser.ParseFile(fset, "doc.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse doc.go: %v", err)
	}
	if docFile.Doc == nil {
		t.Fatal("doc.go carries no package doc comment")
	}
	pkgDoc := normalizeDoc(docFile.Doc.Text())

	clientFile, err := parser.ParseFile(fset, "client.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse client.go: %v", err)
	}
	methodDocs := make(map[string]string)
	for _, decl := range clientFile.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || fn.Doc == nil {
			continue
		}
		methodDocs[fn.Name.Name] = normalizeDoc(fn.Doc.Text())
	}
	methodDoc := methodDocs["CallTool"]
	if methodDoc == "" {
		t.Fatal("client.go: (*Client).CallTool has no doc comment")
	}
	verifiedDoc := methodDocs["CallToolVerified"]
	if verifiedDoc == "" {
		t.Fatal("client.go: (*Client).CallToolVerified has no doc comment")
	}

	if !strings.Contains(methodDoc, pinAdmission) {
		t.Errorf("CallTool's comment no longer states %q — if CallTool now verifies the pin, close TRACKING.md H8-1 and retire this test; otherwise restore the admission", pinAdmission)
	}
	if !strings.Contains(pkgDoc, pinAdmission) {
		t.Errorf("package doc does not carry CallTool's admission %q", pinAdmission)
	}
	if !strings.Contains(pkgDoc, "CallToolVerified") {
		t.Error("package doc does not name the opt-in verified call path")
	}
	if !strings.Contains(pkgDoc, "VerifyPinBeforeCall") {
		t.Error("package doc does not name the bridge opt-in")
	}
	if !strings.Contains(verifiedDoc, "not atomic") || !strings.Contains(pkgDoc, "not make a remote check and call atomic") {
		t.Error("verified-call documentation must state that remote verification and dispatch are not atomic")
	}
	for _, re := range pinOverclaims {
		if m := re.FindString(pkgDoc); m != "" {
			t.Errorf("package doc claims per-call pin verification again (%q matched %q); CallTool does not verify the pin — see its comment and TRACKING.md H8-1", re, m)
		}
	}
}

// normalizeDoc collapses a doc comment's line breaks and runs of whitespace
// so phrase matching is independent of how the comment is wrapped.
func normalizeDoc(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
