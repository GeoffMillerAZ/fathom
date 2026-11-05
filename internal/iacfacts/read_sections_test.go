package iacfacts

import "testing"

// A resource section that is not a mapping, or a resource whose properties may sit behind a merge key, read as
// nothing at all with no error counted and nothing dropped, which contradicts ResourceSet.Dropped's promise that
// absence is never silent. Each now counts one syntax error and the resource, where there is one, reads unknown.
func TestReadSectionShapesAreNeverSilent(t *testing.T) {
	cfn := map[string]string{
		"alias":  "x: &all\n  A:\n    Type: AWS::S3::Bucket\nResources: *all\n",
		"if":     "Resources: !If [c, {A: {Type: 'AWS::S3::Bucket'}}, {}]\n",
		"scalar": "Resources: ${v}\n",
		"tagged": "Resources: !Transform {Name: x}\n",
	}
	for name, src := range cfn {
		set, err := ReadCloudFormationFile("t.yaml", []byte(src))
		if err != nil || len(set.Resources) != 0 || set.SyntaxErrors != 1 {
			t.Errorf("cloudformation %s: %v resources %d syntax %d", name, err, len(set.Resources), set.SyntaxErrors)
		}
	}
	pul := map[string]string{
		"alias":  "x: &all\n  a:\n    type: aws:s3/bucket:Bucket\nresources: *all\n",
		"scalar": "resources: ${v}\n",
		"tagged": "resources: !Fn {x: y}\n",
	}
	for name, src := range pul {
		set, err := ReadPulumiYAMLFile("Pulumi.yaml", []byte(src))
		if err != nil || len(set.Resources) != 0 || set.SyntaxErrors != 1 {
			t.Errorf("pulumi %s: %v resources %d syntax %d", name, err, len(set.Resources), set.SyntaxErrors)
		}
	}
	// An absent or empty section has no resources and is not an error.
	for name, src := range map[string]string{"absent": "Parameters: {}\n", "empty": "Resources:\n", "empty map": "Resources: {}\n", "plain list": "Resources: [a]\n", "plain scalar": "Resources: hello\n"} {
		if set, err := ReadCloudFormationFile("t.yaml", []byte(src)); err != nil || set.SyntaxErrors != 0 || len(set.Resources) != 0 {
			t.Errorf("cloudformation %s: %v %+v", name, err, set)
		}
	}
	if set, err := ReadPulumiYAMLFile("Pulumi.yaml", []byte("resources:\n")); err != nil || set.SyntaxErrors != 0 {
		t.Errorf("pulumi empty: %v %+v", err, set)
	}

	// A merge key may carry the properties: the resource reads with one unknown root attribute.
	set, err := ReadCloudFormationFile("t.yaml", []byte("base: &p\n  BucketName: b\nResources:\n  B:\n    Type: AWS::S3::Bucket\n    <<: *p\n"))
	if err != nil || len(set.Resources) != 1 || set.SyntaxErrors != 1 || len(set.Resources[0].Attrs) != 1 || set.Resources[0].Attrs[0].Class != ClassUnknown {
		t.Errorf("cloudformation merge: %v %+v", err, set)
	}
	set, err = ReadPulumiYAMLFile("Pulumi.yaml", []byte("base: &p\n  bucket: b\nresources:\n  r:\n    type: aws:s3/bucket:Bucket\n    <<: *p\n"))
	if err != nil || len(set.Resources) != 1 || set.SyntaxErrors != 1 || len(set.Resources[0].Attrs) != 1 || set.Resources[0].Attrs[0].Class != ClassUnknown {
		t.Errorf("pulumi merge: %v %+v", err, set)
	}
	// With properties of its own the resource reads them and counts nothing.
	set, err = ReadCloudFormationFile("t.yaml", []byte("base: &p\n  Other: x\nResources:\n  B:\n    Type: AWS::S3::Bucket\n    Properties:\n      BucketName: b\n    <<: *p\n"))
	if err != nil || set.SyntaxErrors != 0 || len(set.Resources[0].Attrs) != 1 || set.Resources[0].Attrs[0].Class != ClassLiteral {
		t.Errorf("cloudformation own properties: %v %+v", err, set)
	}
}
