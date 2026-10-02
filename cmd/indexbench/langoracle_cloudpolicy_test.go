package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// TestOwnedSourceRefsByContentCloudPolicy (wave pc): the three cloud-policy
// dialects claim no name, so each is measured on exactly the files ingest's
// dispatch reads with it by content (contentDialectOf), and CloudFormation's
// own measurement is unchanged (an inline IAM PolicyDocument stays the
// template's).
func TestOwnedSourceRefsByContentCloudPolicy(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) core.SourceRef {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return core.SourceRef{Path: rel}
	}
	refs := []core.SourceRef{
		write("iam/a.json", "{\"Version\": \"2012-10-17\", \"Statement\": [{\"Effect\": \"Allow\", \"Action\": \"s3:Get*\", \"Resource\": \"*\"}]}\n"),
		write("iam/b.yaml", "Version: 2012-10-17\nStatement:\n  - Effect: Allow\n    Action: sqs:*\n    Resource: '*'\n"),
		write("azure/d.json", "{\"properties\": {\"policyRule\": {\"if\": {\"field\": \"type\", \"equals\": \"Microsoft.Web/sites\"}, \"then\": {\"effect\": \"deny\"}}}}\n"),
		write("custodian/p.yml", "policies:\n  - name: a\n    resource: ec2\n"),
		write("infra/stack.yaml", "AWSTemplateFormatVersion: '2010-09-09'\nResources:\n  P:\n    Type: AWS::IAM::ManagedPolicy\n    Properties:\n      PolicyDocument:\n        Version: 2012-10-17\n        Statement: []\n"),
		write("data.json", "{\"a\": 1}\n"),
	}
	pipe := fusion.New(nil, nil, fusion.Config{})
	defer pipe.Close()
	content := contentDialectOf(pipe, refs)
	for id, want := range map[string]string{
		"aws-iam-policy":  "iam/a.json,iam/b.yaml",
		"azure-policy":    "azure/d.json",
		"cloud-custodian": "custodian/p.yml",
		"cloudformation":  "infra/stack.yaml",
	} {
		var got []string
		for _, r := range ownedSourceRefs(langbench.Manifest{ID: id, Engine: "doc"}, root, refs, nil, content) {
			got = append(got, r.Path)
		}
		if strings.Join(got, ",") != want {
			t.Errorf("%s owns %v, want %s", id, got, want)
		}
	}
}
