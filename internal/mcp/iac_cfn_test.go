package mcp_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/iacfacts"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// IF-C3 and IF-C4: CloudFormation facts on symbol_context and review_diff. Every schema is made up (IF-P7).

func cfnTestSchema(t *testing.T) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"typeName":                        "Example::Widget::Thing",
		"description":                     "prose a catalog must not carry",
		"properties":                      map[string]any{"Name": map[string]any{"type": "string"}, "Size": map[string]any{"type": "integer"}, "Arn": map[string]any{"type": "string"}, "OldKnob": map[string]any{"type": "boolean"}, "Tags": map[string]any{"type": "array"}},
		"required":                        []string{"Name"},
		"createOnlyProperties":            []string{"/properties/Name"},
		"conditionalCreateOnlyProperties": []string{"/properties/Size"},
		"readOnlyProperties":              []string{"/properties/Arn"},
		"deprecatedProperties":            []string{"/properties/OldKnob"},
		"additionalProperties":            false,
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// cfnHome points FATHOM_HOME at a fresh directory and, when install is set, imports a CloudFormation catalog for
// us-east-1 into it.
func cfnHome(t *testing.T, install bool) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("FATHOM_HOME", home)
	if !install {
		return
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.CreateHeader(&zip.FileHeader{Name: "example-widget-thing.json", Method: zip.Deflate, Modified: time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	w.Write(cfnTestSchema(t))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), "schemas.zip")
	if err := os.WriteFile(src, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := iacfacts.ImportCloudFormationSchema(filepath.Join(home, "iac"), src, "us-east-1", iacfacts.IaCWriterOptions{Tool: "test"}); err != nil {
		t.Fatal(err)
	}
}

const cfnTemplate = `AWSTemplateFormatVersion: "2010-09-09"
Resources:
  Widget:
    Type: Example::Widget::Thing
    Properties:
      Name: a
      Size: 3
      Arn: x
      OldKnob: true
      Colour: red
      Tags:
        - Key: k
  Opaque:
    Type: Other::Vendor::Thing
    Properties:
      X: 1
`

func cfnServer(root string) *mcp.Server {
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "stack", Name: "Widget", Kind: "CFNResource", FilePath: "stack.yaml", Line: 3, EndLine: 12, Signature: "Example::Widget::Thing"},
		{Service: "svc", Package: "stack", Name: "Opaque", Kind: "CFNResource", FilePath: "stack.yaml", Line: 13, EndLine: 16, Signature: "Other::Vendor::Thing"},
	}
	return mcp.NewServer(&reviewFakeBackend{extendedFakeBackend: extendedFakeBackend{nodes: nodes, services: []string{"svc"}, svcPath: root}})
}

func cfnFindingMap(t *testing.T, list any) map[string]map[string]any {
	t.Helper()
	out := map[string]map[string]any{}
	for _, f := range list.([]any) {
		m := f.(map[string]any)
		out[m["kind"].(string)+" "+m["path"].(string)] = m
	}
	return out
}

func TestSymbolContextCarriesTheCFNBlock(t *testing.T) {
	cfnHome(t, true)
	root := iacRepo(t, map[string]string{"stack.yaml": cfnTemplate})
	p, _ := iacCall(t, cfnServer(root), "symbol_context", map[string]any{"service": "svc", "symbol": "Widget"})
	iac := iacBlock(t, p)
	b := iac["binding"].(map[string]any)
	if b["status"] != "unversioned" || b["version"] != "unversioned" || b["region"] != "us-east-1" || b["date"] != "2026-10-30" || b["catalog"] == "" {
		t.Fatalf("binding %v", b)
	}
	got := cfnFindingMap(t, iac["findings"])
	for _, want := range []string{"unknown-property Colour", "read-only-set Arn", "deprecated OldKnob"} {
		f, ok := got[want]
		if !ok {
			t.Fatalf("missing %q in %v", want, got)
		}
		citedFact(t, f["fact"])
		if f["fact"].(map[string]any)["version"] != "unversioned" {
			t.Fatalf("fact %v", f["fact"])
		}
	}
	rep := map[string]string{}
	for _, r := range iac["replacement_if_changed"].([]any) {
		m := r.(map[string]any)
		rep[m["property"].(string)] = m["effect"].(string)
		citedFact(t, m["fact"])
	}
	if !reflect.DeepEqual(rep, map[string]string{"Name": "forces-replacement", "Size": "may-replace"}) {
		t.Fatalf("replacement_if_changed %v", rep)
	}
	// A type the registry schemas lack reads unbound and carries no finding.
	p, _ = iacCall(t, cfnServer(root), "symbol_context", map[string]any{"service": "svc", "symbol": "Opaque"})
	b = iacBlock(t, p)["binding"].(map[string]any)
	if b["status"] != "unbound" || !strings.Contains(b["note"].(string), "not in the catalog") {
		t.Fatalf("unbound binding %v", b)
	}
	if _, has := iacBlock(t, p)["findings"]; has {
		t.Fatalf("an unbound resource carries findings: %v", iacBlock(t, p))
	}
}

// With no CloudFormation catalog both surfaces answer exactly as they did: no iac key, and a Terraform catalog
// alone does not add one.
func TestCFNSurfacesUnchangedWithoutACatalog(t *testing.T) {
	root := iacRepo(t, map[string]string{"stack.yaml": cfnTemplate})
	sym := map[string]any{"service": "svc", "symbol": "Widget"}
	rev := map[string]any{"service": "svc", "changed_files": []string{"stack.yaml"}}

	cfnHome(t, false)
	symNone, _ := iacCall(t, cfnServer(root), "symbol_context", sym)
	revNone, _ := iacCall(t, cfnServer(root), "review_diff", rev)
	iacHome(t, true) // a Terraform catalog only
	symTF, _ := iacCall(t, cfnServer(root), "symbol_context", sym)
	revTF, _ := iacCall(t, cfnServer(root), "review_diff", rev)
	for name, p := range map[string]map[string]any{"symbol none": symNone, "review none": revNone, "symbol terraform": symTF, "review terraform": revTF} {
		if _, has := p["iac"]; has {
			t.Fatalf("%s: an iac key appeared with no CloudFormation catalog: %v", name, p["iac"])
		}
	}
	if !reflect.DeepEqual(symNone, symTF) || !reflect.DeepEqual(stripVolatile(revNone), stripVolatile(revTF)) {
		t.Fatal("a Terraform catalog changed a CloudFormation answer")
	}

	cfnHome(t, true)
	symWith, _ := iacCall(t, cfnServer(root), "symbol_context", sym)
	revWith, _ := iacCall(t, cfnServer(root), "review_diff", rev)
	iacBlock(t, symWith)
	iacBlock(t, revWith)
	delete(symWith, "iac")
	delete(revWith, "iac")
	if !reflect.DeepEqual(symNone, symWith) || !reflect.DeepEqual(stripVolatile(revNone), stripVolatile(revWith)) {
		t.Fatal("a CloudFormation catalog changed an answer beyond the iac key")
	}
}

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null"}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestReviewDiffFlagsAChangedCreateOnlyProperty(t *testing.T) {
	cfnHome(t, true)
	root := iacRepo(t, map[string]string{"stack.yaml": cfnTemplate})
	gitIn(t, root, "init", "-q")
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-q", "-m", "base")
	changed := strings.Replace(strings.Replace(cfnTemplate, "Name: a", "Name: b", 1), "Size: 3", "Size: 4", 1)
	changed = strings.Replace(changed, "Key: k", "Key: z", 1) // Tags is in no replacement list
	if err := os.WriteFile(filepath.Join(root, "stack.yaml"), []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	p, _ := iacCall(t, cfnServer(root), "review_diff", map[string]any{"service": "svc", "changed_files": []string{"stack.yaml"}, "since": "HEAD"})
	cfn := iacBlock(t, p)["cloudformation"].(map[string]any)
	got := cfnFindingMap(t, cfn["findings"])
	for _, want := range []string{"forces-replacement Name", "may-replace Size", "unknown-property Colour", "read-only-set Arn", "deprecated OldKnob"} {
		f, ok := got[want]
		if !ok {
			t.Fatalf("missing %q in %v", want, got)
		}
		citedFact(t, f["fact"])
		if f["line"] == nil || f["block"] != "Widget" {
			t.Fatalf("finding %v", f)
		}
	}
	for k := range got {
		if strings.Contains(k, "Tags") {
			t.Fatalf("an unflagged property was reported: %s", k)
		}
	}
	if !strings.Contains(got["forces-replacement Name"]["message"].(string), "createOnlyProperties") {
		t.Fatalf("the message does not cite the schema field: %v", got["forces-replacement Name"])
	}

	// Without since there is no base: attribute findings stay, replacement findings cannot be made, and it says so.
	p, _ = iacCall(t, cfnServer(root), "review_diff", map[string]any{"service": "svc", "changed_files": []string{"stack.yaml"}})
	cfn = iacBlock(t, p)["cloudformation"].(map[string]any)
	got = cfnFindingMap(t, cfn["findings"])
	if _, ok := got["forces-replacement Name"]; ok {
		t.Fatalf("a replacement finding without a base: %v", got)
	}
	if _, ok := got["unknown-property Colour"]; !ok {
		t.Fatalf("the attribute findings went missing: %v", got)
	}
	if !strings.Contains(fmt.Sprint(cfn["disclosures"]), "since") {
		t.Fatalf("no disclosure that replacement needs since: %v", cfn["disclosures"])
	}

	// A new file has no base to compare: no replacement finding.
	gitIn(t, root, "checkout", "-q", "--", "stack.yaml")
	if err := os.WriteFile(filepath.Join(root, "new.yaml"), []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	p, _ = iacCall(t, cfnServer(root), "review_diff", map[string]any{"service": "svc", "changed_files": []string{"new.yaml"}, "since": "HEAD"})
	got = cfnFindingMap(t, iacBlock(t, p)["cloudformation"].(map[string]any)["findings"])
	if _, ok := got["forces-replacement Name"]; ok {
		t.Fatalf("a replacement finding for a new file: %v", got)
	}
}

func TestReviewDiffCFNFindingsStayUnderTheByteCap(t *testing.T) {
	cfnHome(t, true)
	var sb strings.Builder
	sb.WriteString("AWSTemplateFormatVersion: \"2010-09-09\"\nResources:\n")
	for i := 0; i < 400; i++ {
		fmt.Fprintf(&sb, "  R%03d:\n    Type: Example::Widget::Thing\n    Properties:\n      Name: a\n      Bogus%03d: 1\n", i, i)
	}
	root := iacRepo(t, map[string]string{"big.yaml": sb.String()})
	p, size := iacCall(t, cfnServer(root), "review_diff", map[string]any{"service": "svc", "changed_files": []string{"big.yaml"}})
	if size > 16000 {
		t.Fatalf("the response is %d bytes, over the 16000 cap", size)
	}
	cfn := iacBlock(t, p)["cloudformation"].(map[string]any)
	if n := len(cfn["findings"].([]any)); n == 0 || n >= 400 {
		t.Fatalf("%d findings returned; want some, trimmed", n)
	}
	el, ok := p["elided"].(map[string]any)
	if !ok {
		t.Fatalf("the trim must be disclosed: %v", p["elided"])
	}
	if _, ok := el["fields"].(map[string]any)["/iac/cloudformation/findings"]; !ok {
		t.Fatalf("the elision names /iac/cloudformation/findings: %v", el)
	}
}

func TestCFNReadsRefuseASymlinkOutOfTheCorpus(t *testing.T) {
	cfnHome(t, true)
	outside := iacRepo(t, map[string]string{"evil.yaml": "AWSTemplateFormatVersion: x\nResources:\n  Z:\n    Type: Example::Widget::Thing\n    Properties:\n      Leaked: 1\n"})
	root := iacRepo(t, map[string]string{"stack.yaml": cfnTemplate})
	if err := os.Symlink(filepath.Join(outside, "evil.yaml"), filepath.Join(root, "link.yaml")); err != nil {
		t.Skip("symlinks unavailable")
	}
	p, _ := iacCall(t, cfnServer(root), "review_diff", map[string]any{"service": "svc", "changed_files": []string{"stack.yaml", "link.yaml"}})
	b, _ := json.Marshal(p)
	if strings.Contains(string(b), "Leaked") {
		t.Fatalf("a file outside the corpus was read: %s", b)
	}
}
