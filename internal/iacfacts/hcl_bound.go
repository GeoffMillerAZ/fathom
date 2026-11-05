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
//
// Every check below reads one filtered stream, boundTokens, and none reads the raw lexer output. The parser's peeker
// drops comments, and drops newlines inside any bracket but a brace, so a shape matched over raw adjacent tokens
// (`[*]`, `%{if`) is missed by `[/**/*]` and by a newline between its tokens. The filter applies those rules once.
func parseHCLBounded(file string, src []byte) (*hcl.File, hcl.Diagnostics, error) {
	lexed, _ := hclsyntax.LexConfig(src, file, hcl.InitialPos)
	if len(lexed) > MaxNodes {
		return nil, nil, limitErr(file, fmt.Sprintf("%d tokens, over %d", len(lexed), MaxNodes))
	}
	tokens := boundTokens(lexed)
	// chains holds, per open bracket level (index 0 is the file), the `[*]` steps of the traversal chain being
	// read at that level. A closed `]` does not shrink the parser's stack, since the recursion keeps going for every
	// later step of the chain, so the count lives until a token that cannot continue the chain. A newline ends a
	// chain at the top of a file or of a block body.
	chains := []int{0}
	var kinds []hclsyntax.TokenType
	// quest counts, per bracket level, the `?` whose `:` has not come yet, and ctl the template `%{if}` and `%{for}`
	// directives whose end has not come yet. The parser recurses once per pending `?` and copies the diagnostics of
	// every level below it, and does the same for an open directive, so both nest like brackets. quests is their sum.
	quest := []int{0}
	var ctlBase []int
	splats, run, quests, ctl := 0, 0, 0, 0
	endChain := func() {
		top := len(chains) - 1
		splats -= chains[top]
		chains[top] = 0
	}
	tooDeep := func() error {
		if len(kinds)+run+splats+quests+ctl > MaxDepth {
			return limitErr(file, fmt.Sprintf("operators, splats and brackets nest past %d deep", MaxDepth))
		}
		return nil
	}
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		switch t.Type {
		case hclsyntax.TokenOBrack:
			if i+2 < len(tokens) && tokens[i+1].Type == hclsyntax.TokenStar && tokens[i+2].Type == hclsyntax.TokenCBrack {
				// A full splat `[*]`, adjacent in the filtered stream: one more level of parser recursion for the rest of the chain.
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
			quest = append(quest, 0)
			if t.Type == hclsyntax.TokenTemplateControl && i+1 < len(tokens) && tokens[i+1].Type == hclsyntax.TokenIdent {
				switch string(tokens[i+1].Bytes) {
				case "if", "for":
					ctl++
				case "endif", "endfor":
					if ctl > 0 {
						ctl--
					}
				}
			}
			if err := tooDeep(); err != nil {
				return nil, nil, err
			}
		case hclsyntax.TokenCBrace, hclsyntax.TokenCBrack, hclsyntax.TokenCParen, hclsyntax.TokenTemplateSeqEnd:
			run = 0
			if len(kinds) > 0 {
				endChain()
				kinds = kinds[:len(kinds)-1]
				chains = chains[:len(chains)-1]
				quests -= quest[len(quest)-1]
				quest = quest[:len(quest)-1]
			}
		case hclsyntax.TokenQuestion:
			endChain()
			run = 0
			quest[len(quest)-1]++
			quests++
			if err := tooDeep(); err != nil {
				return nil, nil, err
			}
		case hclsyntax.TokenColon:
			endChain()
			run = 0
			if top := len(quest) - 1; quest[top] > 0 {
				quest[top]--
				quests--
			}
		case hclsyntax.TokenOQuote, hclsyntax.TokenOHeredoc:
			// A directive left open in a template ends with it.
			run = 0
			ctlBase = append(ctlBase, ctl)
		case hclsyntax.TokenCQuote, hclsyntax.TokenCHeredoc:
			run = 0
			if n := len(ctlBase); n > 0 {
				ctl = ctlBase[n-1]
				ctlBase = ctlBase[:n-1]
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
				top := len(quest) - 1
				quests -= quest[top]
				quest[top] = 0
			}
		case hclsyntax.TokenDot, hclsyntax.TokenIdent, hclsyntax.TokenNumberLit, hclsyntax.TokenStar,
			hclsyntax.TokenQuotedLit:
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

// boundTokens is the token stream the parser sees. Comments are dropped, except that a `#` or `//` comment absorbs
// the newline that ends it and the parser turns it back into a newline token where newlines count. A newline is
// dropped inside any bracket but a brace, as the parser's peeker does for parentheses, brackets and template
// sequences.
func boundTokens(lexed hclsyntax.Tokens) []hclsyntax.Token {
	out := make([]hclsyntax.Token, 0, len(lexed))
	var open []hclsyntax.TokenType
	for _, t := range lexed {
		if t.Type == hclsyntax.TokenComment {
			if n := len(t.Bytes); n == 0 || t.Bytes[n-1] != '\n' {
				continue
			}
			t.Type = hclsyntax.TokenNewline
		}
		switch t.Type {
		case hclsyntax.TokenNewline:
			if len(open) > 0 && open[len(open)-1] != hclsyntax.TokenOBrace {
				continue
			}
		case hclsyntax.TokenOBrack, hclsyntax.TokenOBrace, hclsyntax.TokenOParen, hclsyntax.TokenTemplateInterp,
			hclsyntax.TokenTemplateControl:
			open = append(open, t.Type)
		case hclsyntax.TokenCBrace, hclsyntax.TokenCBrack, hclsyntax.TokenCParen, hclsyntax.TokenTemplateSeqEnd:
			if len(open) > 0 {
				open = open[:len(open)-1]
			}
		}
		out = append(out, t)
	}
	return out
}
