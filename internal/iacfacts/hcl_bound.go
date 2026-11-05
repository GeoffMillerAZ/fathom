package iacfacts

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// parseHCLBounded is the one HCL entry of this package: the Terraform reader and the binding reader both come
// through it. It lexes src, refuses past the token and nesting bounds with ErrLimit, then parses. The HCL parser
// recurses once per bracket, once per unary operator and once per `[*]` step of a traversal chain, and Go's stack
// overflow is a fatal error no recover can catch, so a 300 KB run of `!` or `-` or of `[*]` must be refused before
// the parser sees it. A run of prefix operators and the splats of a chain count as nesting, added to the bracket
// depth they sit in, so mixed runs are bounded together.
func parseHCLBounded(file string, src []byte) (*hcl.File, hcl.Diagnostics, error) {
	tokens, _ := hclsyntax.LexConfig(src, file, hcl.InitialPos)
	if len(tokens) > MaxNodes {
		return nil, nil, limitErr(file, fmt.Sprintf("%d tokens, over %d", len(tokens), MaxNodes))
	}
	// chains holds, per open bracket level (index 0 is the file), the `[*]` steps of the traversal chain being
	// read at that level. A closed `]` does not shrink the parser's stack, since the recursion keeps going for every
	// later step of the chain, so the count lives until a token that cannot continue the chain. A newline ends a
	// chain at the top of a file or of a block body, and is whitespace inside any other bracket.
	chains := []int{0}
	var kinds []hclsyntax.TokenType
	splats, run := 0, 0
	endChain := func() {
		top := len(chains) - 1
		splats -= chains[top]
		chains[top] = 0
	}
	tooDeep := func() error {
		if len(kinds)+run+splats > MaxDepth {
			return limitErr(file, fmt.Sprintf("operators, splats and brackets nest past %d deep", MaxDepth))
		}
		return nil
	}
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		switch t.Type {
		case hclsyntax.TokenOBrack:
			if i+2 < len(tokens) && tokens[i+1].Type == hclsyntax.TokenStar && tokens[i+2].Type == hclsyntax.TokenCBrack {
				// A full splat `[*]`: one more level of parser recursion for the rest of the chain.
				i += 2
				run = 0
				chains[len(chains)-1]++
				splats++
				if err := tooDeep(); err != nil {
					return nil, nil, err
				}
				continue
			}
			fallthrough
		case hclsyntax.TokenOBrace, hclsyntax.TokenOParen, hclsyntax.TokenTemplateInterp, hclsyntax.TokenTemplateControl:
			run = 0
			kinds = append(kinds, t.Type)
			chains = append(chains, 0)
			if err := tooDeep(); err != nil {
				return nil, nil, err
			}
		case hclsyntax.TokenCBrace, hclsyntax.TokenCBrack, hclsyntax.TokenCParen, hclsyntax.TokenTemplateSeqEnd:
			run = 0
			if len(kinds) > 0 {
				endChain()
				kinds = kinds[:len(kinds)-1]
				chains = chains[:len(chains)-1]
			}
		case hclsyntax.TokenBang, hclsyntax.TokenMinus:
			endChain()
			run++
			if err := tooDeep(); err != nil {
				return nil, nil, err
			}
		case hclsyntax.TokenComment:
			// Neither a comment nor, inside brackets, a newline ends a run.
		case hclsyntax.TokenNewline:
			if len(kinds) == 0 || kinds[len(kinds)-1] == hclsyntax.TokenOBrace {
				endChain()
			}
		case hclsyntax.TokenDot, hclsyntax.TokenIdent, hclsyntax.TokenNumberLit, hclsyntax.TokenStar,
			hclsyntax.TokenOQuote, hclsyntax.TokenCQuote, hclsyntax.TokenQuotedLit:
			// Steps and index keys a traversal chain continues through.
			run = 0
		default:
			endChain()
			run = 0
		}
	}
	f, diags := hclsyntax.ParseConfig(src, file, hcl.InitialPos)
	return f, diags, nil
}
