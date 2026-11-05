package iacfacts

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"

	"github.com/geoffmilleraz/fathom/internal/hclbound"
)

// parseHCLBounded is the one HCL entry of this package: the Terraform reader and the binding reader both come
// through it. It refuses past the token, nesting and melding bounds with ErrLimit, then parses. The guard itself,
// shared with the extractors, is internal/hclbound, and so are its tests.
func parseHCLBounded(file string, src []byte) (*hcl.File, hcl.Diagnostics, error) {
	if r := hclbound.Check(src, file, hclbound.Bounds{MaxDepth: MaxDepth, MaxTokens: MaxNodes}); r != nil {
		return nil, nil, limitErr(file, r.Reason)
	}
	f, diags := hclsyntax.ParseConfig(src, file, hcl.InitialPos)
	return f, diags, nil
}
