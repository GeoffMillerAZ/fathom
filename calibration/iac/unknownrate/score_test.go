package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/lang/iac"
)

func attr(path string, c iac.Class) iac.Attr { return iac.Attr{Path: path, Class: c} }

func TestClassify(t *testing.T) {
	const p = "ingress[*].cidr_blocks"
	for _, c := range []struct {
		name  string
		attrs []iac.Attr
		want  string
	}{
		{"absent", nil, classUnset},
		{"other attribute only", []iac.Attr{attr("name", iac.ClassExpression)}, classUnset},
		{"literal", []iac.Attr{attr(p, iac.ClassLiteral)}, classLiteral},
		{"reference", []iac.Attr{attr(p, iac.ClassReference)}, classReference},
		{"defaulted", []iac.Attr{attr(p, iac.ClassDefaulted)}, classDefaulted},
		{"expression is unknown", []iac.Attr{attr(p, iac.ClassExpression)}, classUnknown},
		{"unknown is unknown", []iac.Attr{attr(p, iac.ClassUnknown)}, classUnknown},
		{"unknown beats defaulted", []iac.Attr{attr(p, iac.ClassLiteral), attr(p, iac.ClassDefaulted), attr(p, iac.ClassUnknown)}, classUnknown},
		{"defaulted beats reference", []iac.Attr{attr(p, iac.ClassReference), attr(p, iac.ClassDefaulted), attr(p, iac.ClassLiteral)}, classDefaulted},
		{"reference beats literal", []iac.Attr{attr(p, iac.ClassLiteral), attr(p, iac.ClassReference)}, classReference},
		{"below the path counts", []iac.Attr{attr(p+"[*]", iac.ClassExpression)}, classUnknown},
		{"a sibling prefix does not", []iac.Attr{attr(p+"_extra", iac.ClassExpression)}, classUnset},
		{"expression ancestor makes it unknown", []iac.Attr{attr("ingress", iac.ClassExpression)}, classUnknown},
		{"unknown ancestor makes it unknown", []iac.Attr{attr("ingress[*]", iac.ClassUnknown)}, classUnknown},
		{"literal ancestor is ignored", []iac.Attr{attr("ingress", iac.ClassLiteral)}, classUnset},
		{"reference ancestor is ignored", []iac.Attr{attr("ingress[*]", iac.ClassReference)}, classUnset},
		{"an ancestor spelled as a longer name is no ancestor", []iac.Attr{attr("ingres", iac.ClassExpression)}, classUnset},
		{"root unknown makes it unknown", []iac.Attr{attr("", iac.ClassUnknown)}, classUnknown},
		{"root unknown beats a literal", []iac.Attr{attr("", iac.ClassUnknown), attr(p, iac.ClassLiteral)}, classUnknown},
	} {
		got, _ := classify(iac.Resource{Attrs: c.attrs}, p)
		if got != c.want {
			t.Errorf("%s: %s, want %s", c.name, got, c.want)
		}
	}
	// A path equal to a non-nested path: an ancestor needs a segment boundary.
	if got, _ := classify(iac.Resource{Attrs: []iac.Attr{attr("Tags", iac.ClassExpression)}}, "Tags[*].Key"); got != classUnknown {
		t.Errorf("list ancestor: %s", got)
	}
	if got, _ := classify(iac.Resource{Attrs: []iac.Attr{attr("Vers", iac.ClassExpression)}}, "VersioningConfiguration.Status"); got != classUnset {
		t.Errorf("prefix that is not a segment: %s", got)
	}
}

func TestMatches(t *testing.T) {
	rule := Rule{ID: "R", Terraform: Binding{Types: []string{"aws_x"}, Path: "a"}, CloudFormation: Binding{Types: []string{"AWS::X"}, Path: "A"},
		Pulumi: Binding{Types: []string{"aws:x:X", "aws:x/x:X"}, Path: "a"}}
	for _, c := range []struct {
		r    iac.Resource
		want bool
	}{
		{iac.Resource{Tool: iac.ToolTerraform, Mode: iac.ModeManaged, Type: "aws_x"}, true},
		{iac.Resource{Tool: iac.ToolTerraform, Mode: iac.ModeData, Type: "aws_x"}, false},
		{iac.Resource{Tool: iac.ToolTerraform, Mode: iac.ModeModule, Type: "aws_x"}, false},
		{iac.Resource{Tool: iac.ToolTerraform, Mode: iac.ModeManaged, Type: "aws_y"}, false},
		{iac.Resource{Tool: iac.ToolCloudFormation, Type: "AWS::X"}, true},
		{iac.Resource{Tool: iac.ToolCloudFormation, Type: "aws_x"}, false},
		{iac.Resource{Tool: iac.ToolPulumi, Type: "aws:x/x:X"}, true},
		{iac.Resource{Tool: iac.ToolPulumi, Type: "aws:y:Y"}, false},
	} {
		if _, ok := rule.binding(c.r); ok != c.want {
			t.Errorf("%+v: matches %v, want %v", c.r, ok, c.want)
		}
	}
}

func TestCellRates(t *testing.T) {
	var c Cell
	for _, k := range []string{classLiteral, classLiteral, classReference, classDefaulted, classUnknown, classUnset} {
		c.add(k)
	}
	if c.Pairs != 6 || c.Unknown != 1 {
		t.Fatalf("%+v", c)
	}
	if got := c.rate(); got != 1.0/6 {
		t.Errorf("rate %v", got)
	}
	if got := c.ratePresent(); got != 1.0/5 {
		t.Errorf("rate over present %v", got)
	}
	if got := c.rateWithDefaulted(); got != 2.0/6 {
		t.Errorf("rate with defaulted %v", got)
	}
	if got := (Cell{}).rate(); got != 0 {
		t.Errorf("empty rate %v", got)
	}
}

func TestAuditKeyAndSelection(t *testing.T) {
	p := Pair{Rule: "R05", Subject: "tf-d3", Tool: iac.ToolTerraform, File: "terraform/aws/ec2.tf", Type: "aws_security_group", Name: "web", Line: 12}
	sum := sha256.Sum256([]byte("ll-m6-audit-v1|R05|tf-d3|terraform/aws/ec2.tf|aws_security_group|web|12"))
	if got := auditKey(p); got != hex.EncodeToString(sum[:]) {
		t.Fatalf("key %s", got)
	}
	var pairs []Pair
	for i := 0; i < 40; i++ {
		pairs = append(pairs, Pair{Rule: "R01", Subject: "tf-d3", Tool: iac.ToolTerraform, File: "a.tf", Type: "t", Name: fmt.Sprint("n", i), Line: i})
		pairs = append(pairs, Pair{Rule: "R01", Subject: "cfn-d1", Tool: iac.ToolCloudFormation, File: "a.yaml", Type: "T", Name: fmt.Sprint("N", i), Line: i})
	}
	for i := 0; i < 5; i++ {
		pairs = append(pairs, Pair{Rule: "R01", Subject: "pu-d1", Tool: iac.ToolPulumi, File: "Pulumi.yaml", Type: "t", Name: fmt.Sprint("n", i), Line: i})
	}
	got := auditSelect(pairs, 25)
	if len(got) != 50 {
		t.Fatalf("selected %d, want 50 (25 per tool, no Pulumi)", len(got))
	}
	per := map[string]int{}
	for _, p := range got {
		per[p.Tool]++
	}
	if per[iac.ToolTerraform] != 25 || per[iac.ToolCloudFormation] != 25 || per[iac.ToolPulumi] != 0 {
		t.Fatalf("per tool %v", per)
	}
	// Input order does not matter, and the choice is the 25 smallest keys.
	rev := append([]Pair{}, pairs...)
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	again := auditSelect(rev, 25)
	for i := range got {
		if auditKey(got[i]) != auditKey(again[i]) {
			t.Fatalf("selection depends on input order at %d", i)
		}
	}
	var tf []string
	for _, p := range pairs {
		if p.Tool == iac.ToolTerraform {
			tf = append(tf, auditKey(p))
		}
	}
	sort.Strings(tf)
	var picked []string
	for _, p := range got {
		if p.Tool == iac.ToolTerraform {
			picked = append(picked, auditKey(p))
		}
	}
	for i := range picked {
		if picked[i] != tf[i] {
			t.Fatalf("pick %d is %s, want the %dth smallest key %s", i, picked[i], i, tf[i])
		}
	}
	if small := auditSelect(pairs[:6], 25); len(small) != 6-countTool(pairs[:6], iac.ToolPulumi) {
		t.Fatalf("a tool with fewer than 25 pairs is audited whole: %d", len(small))
	}
}

func countTool(ps []Pair, tool string) int {
	n := 0
	for _, p := range ps {
		if p.Tool == tool {
			n++
		}
	}
	return n
}

func TestScoreTerraformModule(t *testing.T) {
	dest := t.TempDir()
	dir := filepath.Join(dest, "r_r")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	tf := `
variable "env" {
  default = "dev"
}
variable "nodefault" {}
resource "aws_db_instance" "a" {
  storage_encrypted       = true
  publicly_accessible     = var.env
  backup_retention_period = aws_x.y.z
}
resource "aws_db_instance" "b" {
  storage_encrypted = upper(var.nodefault)
}
resource "aws_db_instance" "c" {
}
data "aws_db_instance" "d" {
  storage_encrypted = true
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.tf"), []byte(tf), 0o644); err != nil {
		t.Fatal(err)
	}
	rules := []Rule{
		{ID: "R08", Terraform: Binding{Types: []string{"aws_db_instance"}, Path: "storage_encrypted"}},
		{ID: "R09", Terraform: Binding{Types: []string{"aws_db_instance"}, Path: "publicly_accessible"}},
		{ID: "R10", Terraform: Binding{Types: []string{"aws_db_instance"}, Path: "backup_retention_period"}},
	}
	subj := Subject{ID: "tf-x", Tool: iac.ToolTerraform, Kind: "deployed", Repo: "r/r", Path: "."}
	set, err := readSubject(context.Background(), dest, subj)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, p := range scoreSet(set, subj, rules) {
		got[p.Rule+"/"+p.Name] = p.Class
	}
	want := map[string]string{
		"R08/a": classLiteral, "R08/b": classUnknown, "R08/c": classUnset,
		"R09/a": classDefaulted, "R09/b": classUnset, "R09/c": classUnset,
		"R10/a": classReference, "R10/b": classUnset, "R10/c": classUnset,
	}
	if len(got) != len(want) {
		t.Fatalf("pairs %v", got)
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s: %s, want %s", k, got[k], w)
		}
	}
}

func TestScoreCloudFormationFile(t *testing.T) {
	dest := t.TempDir()
	dir := filepath.Join(dest, "r_r")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	tpl := `AWSTemplateFormatVersion: "2010-09-09"
Parameters:
  Enc:
    Type: String
    Default: "true"
  Key:
    Type: String
Resources:
  A:
    Type: AWS::RDS::DBInstance
    Properties:
      StorageEncrypted: true
      PubliclyAccessible: !Ref Enc
      BackupRetentionPeriod: !Ref Key
  B:
    Type: AWS::RDS::DBInstance
    Properties: !Ref Key
  C:
    Type: AWS::RDS::DBInstance
    Properties:
      StorageEncrypted: !Sub "${Key}-x"
`
	if err := os.WriteFile(filepath.Join(dir, "t.yaml"), []byte(tpl), 0o644); err != nil {
		t.Fatal(err)
	}
	rules := []Rule{
		{ID: "R08", CloudFormation: Binding{Types: []string{"AWS::RDS::DBInstance"}, Path: "StorageEncrypted"}},
		{ID: "R09", CloudFormation: Binding{Types: []string{"AWS::RDS::DBInstance"}, Path: "PubliclyAccessible"}},
	}
	subj := Subject{ID: "cfn-x", Tool: iac.ToolCloudFormation, Kind: "reusable", Repo: "r/r", Path: "t.yaml"}
	set, err := readSubject(context.Background(), dest, subj)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, p := range scoreSet(set, subj, rules) {
		got[p.Rule+"/"+p.Name] = p.Class
	}
	want := map[string]string{
		"R08/A": classLiteral, "R08/B": classUnknown, "R08/C": classUnknown,
		"R09/A": classDefaulted, "R09/B": classUnknown, "R09/C": classUnset,
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s: %s, want %s", k, got[k], w)
		}
	}
	if len(got) != len(want) {
		t.Errorf("pairs %v", got)
	}
}

func TestSubjectsAndRulesFilesLoad(t *testing.T) {
	rules, err := loadRules("rules.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 20 {
		t.Fatalf("%d rules, want 20", len(rules))
	}
	for _, r := range rules {
		for _, b := range []Binding{r.Terraform, r.CloudFormation, r.Pulumi} {
			if len(b.Types) == 0 || b.Path == "" {
				t.Errorf("%s: empty binding %+v", r.ID, b)
			}
		}
	}
	subs, err := loadSubjects("subjects.tsv")
	if err != nil {
		t.Fatal(err)
	}
	n := map[string]int{}
	for _, s := range subs {
		n[s.Tool+"/"+s.Kind]++
		if len(s.SHA) != 40 {
			t.Errorf("%s: sha %q", s.ID, s.SHA)
		}
	}
	for _, k := range []string{"terraform/reusable", "terraform/deployed", "cloudformation/reusable", "cloudformation/deployed"} {
		if n[k] != 5 {
			t.Errorf("%s: %d subjects, want 5", k, n[k])
		}
	}
	if n["pulumi/reusable"] != 3 || n["pulumi/deployed"] != 3 {
		t.Errorf("pulumi subjects %v", n)
	}
}

func TestNumbered(t *testing.T) {
	src := []string{"a", "b", "c", "d", "e"}
	if got, want := numbered(src, 2, 3, 10), "    2  b\n    3  c\n"; got != want {
		t.Errorf("got %q want %q", got, want)
	}
	if got, want := numbered(src, 0, 9, 10), "    1  a\n    2  b\n    3  c\n    4  d\n    5  e\n"; got != want {
		t.Errorf("clamped: got %q want %q", got, want)
	}
	if got := numbered(src, 1, 5, 2); got != "    1  a\n    2  b\n   ... (3 more lines)\n" {
		t.Errorf("capped: %q", got)
	}
	for in, want := range map[string]string{"a.b": "b", "ingress[*].cidr_blocks": "cidr_blocks", "acl": "acl", "x[*].y.z": "z"} {
		if got := leaf(in); got != want {
			t.Errorf("leaf(%q) = %q, want %q", in, got, want)
		}
	}
}
