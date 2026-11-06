package iacfacts

import (
	"regexp"
)

// ReadTerraformJSONFile reads one Terraform or OpenTofu JSON-syntax file (.tf.json, .tofu.json). The JSON syntax is
// not read: the file declares blocks this read does not see, so the result holds no resource and counts one syntax
// error, which is how ReadTerraformModule counts the same file. A file past MaxFileBytes is refused with ErrLimit.
func ReadTerraformJSONFile(filePath string, src []byte, opts ...Option) (*ResourceSet, error) {
	if err := checkSize(filePath, len(src)); err != nil {
		return nil, err
	}
	return &ResourceSet{Tool: ToolTerraform, SyntaxErrors: 1}, nil
}

var (
	cfnVersionKey = regexp.MustCompile(`"?AWSTemplateFormatVersion"?[ \t]*:`)
	cfnResources  = regexp.MustCompile(`"?Resources"?[ \t]*:`)
	cfnTypeValue  = regexp.MustCompile(`"?Type"?[ \t]*:[ \t]*['"]?(AWS|Custom|Alexa)::`)
)

// LooksLikeCloudFormation reports whether src reads as a CloudFormation or SAM template: it names the template
// format version, or it has a Resources key and a resource Type in an AWS::, Custom:: or Alexa:: namespace. It is a
// routing sniff over the bytes, not a parse, so a file that passes it can still read with syntax errors.
func LooksLikeCloudFormation(src []byte) bool {
	if cfnVersionKey.Match(src) {
		return true
	}
	return cfnResources.Match(src) && cfnTypeValue.Match(src)
}
