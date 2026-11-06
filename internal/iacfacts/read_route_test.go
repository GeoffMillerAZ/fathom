package iacfacts

import (
	"errors"
	"strings"
	"testing"
)

func TestLooksLikeCloudFormation(t *testing.T) {
	for src, want := range map[string]bool{
		"AWSTemplateFormatVersion: \"2010-09-09\"\nResources: {}\n":                                      true,
		`{"AWSTemplateFormatVersion": "2010-09-09", "Resources": {}}`:                                    true,
		"Resources:\n  B:\n    Type: AWS::S3::Bucket\n":                                                  true,
		`{"Resources":{"B":{"Type":"AWS::S3::Bucket"}}}`:                                                 true,
		"Resources:\n  B:\n    Type: Custom::Thing\n":                                                    true,
		"Transform: AWS::Serverless-2016-10-31\nResources:\n  F:\n    Type: AWS::Serverless::Function\n": true,
		"apiVersion: v1\nkind: ConfigMap\n":                                                              false,
		`{"Resources": {"a": "b"}}`:                                                                      false,
		"Resources:\n  a: b\n":                                                                           false,
		"":                                                                                               false,
		"name: demo\nruntime: yaml\nresources:\n  r:\n    type: aws:s3/bucket:Bucket\n":                  false,
	} {
		if got := LooksLikeCloudFormation([]byte(src)); got != want {
			t.Errorf("LooksLikeCloudFormation(%q) = %v, want %v", src, got, want)
		}
	}
}

func TestReadTerraformJSONFile(t *testing.T) {
	set, err := ReadTerraformJSONFile("m.tf.json", []byte(`{"resource":{"a":{"b":{}}}}`))
	if err != nil || set.Tool != ToolTerraform || len(set.Resources) != 0 || set.SyntaxErrors != 1 {
		t.Fatalf("got %+v, %v", set, err)
	}
	_, err = ReadTerraformJSONFile("m.tf.json", []byte(strings.Repeat(" ", MaxFileBytes+1)))
	if !errors.Is(err, ErrLimit) {
		t.Fatalf("a file over the limit: %v", err)
	}
}
