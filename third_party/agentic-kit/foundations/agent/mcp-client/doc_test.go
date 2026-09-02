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

// TestDocDoesNotClaimPinVerification holds the package doc to what CallTool's
// own comment says: the description pin is verified at ListTools only, and
// CallTool performs no pin verification of its own (TRACKING.md H8-1, open).
// The package doc once claimed "every tools/call" verified the pin — INC-5
// residue that outlived the H8 Stage-A docs correction — and this test fails
// the moment that claim returns in any phrasing pinOverclaims catches, or the
// moment the doc stops carrying the method comment's admission.
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
	var methodDoc string
	for _, decl := range clientFile.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || fn.Name.Name != "CallTool" {
			continue
		}
		if fn.Doc != nil {
			methodDoc = normalizeDoc(fn.Doc.Text())
		}
	}
	if methodDoc == "" {
		t.Fatal("client.go: (*Client).CallTool has no doc comment")
	}

	if !strings.Contains(methodDoc, pinAdmission) {
		t.Errorf("CallTool's comment no longer states %q — if CallTool now verifies the pin, close TRACKING.md H8-1 and retire this test; otherwise restore the admission", pinAdmission)
	}
	if !strings.Contains(pkgDoc, pinAdmission) {
		t.Errorf("package doc does not carry CallTool's admission %q", pinAdmission)
	}
	if !strings.Contains(pkgDoc, "H8-1") {
		t.Error("package doc does not point at TRACKING.md H8-1, the open pre-call pin check")
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
