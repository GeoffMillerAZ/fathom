package iacfacts

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// parseHCLBounded is the one HCL entry of this package: the Terraform reader and the binding reader both come
// through it. It lexes src, refuses past the token and nesting bounds with ErrLimit, then parses. The HCL parser
// recurses once per bracket and once per unary operator, and Go's stack overflow is a fatal error no recover can
// catch, so a 300 KB run of `!` or `-` must be refused before the parser sees it. A run of prefix operators counts
// as nesting, added to the bracket depth it sits in, so mixed runs of operators and brackets are bounded together.
func parseHCLBounded(file string, src []byte) (*hcl.File, hcl.Diagnostics, error) {
	tokens, _ := hclsyntax.LexConfig(src, file, hcl.InitialPos)
	if len(tokens) > MaxNodes {
		return nil, nil, limitErr(file, fmt.Sprintf("%d tokens, over %d", len(tokens), MaxNodes))
	}
	depth, run := 0, 0
	for _, t := range tokens {
		switch t.Type {
		case hclsyntax.TokenOBrace, hclsyntax.TokenOBrack, hclsyntax.TokenOParen,
			hclsyntax.TokenTemplateInterp, hclsyntax.TokenTemplateControl:
			run = 0
			if depth++; depth > MaxDepth {
				return nil, nil, limitErr(file, fmt.Sprintf("brackets nest past %d deep", MaxDepth))
			}
		case hclsyntax.TokenCBrace, hclsyntax.TokenCBrack, hclsyntax.TokenCParen,
			hclsyntax.TokenTemplateSeqEnd:
			run = 0
			if depth > 0 {
				depth--
			}
		case hclsyntax.TokenBang, hclsyntax.TokenMinus:
			if run++; depth+run > MaxDepth {
				return nil, nil, limitErr(file, fmt.Sprintf("operators and brackets nest past %d deep", MaxDepth))
			}
		case hclsyntax.TokenComment, hclsyntax.TokenNewline:
			// Neither ends a run: inside brackets a newline is whitespace.
		default:
			run = 0
		}
	}
	f, diags := hclsyntax.ParseConfig(src, file, hcl.InitialPos)
	return f, diags, nil
}
