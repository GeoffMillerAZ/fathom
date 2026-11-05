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
// drops comments, and drops newlines inside any bracket, and inside a brace that opens a `for` expression (but not
// inside an object constructor's or a block's brace), so a shape matched over raw adjacent tokens (`[*]`, `%{if`)
// is missed by `[/**/*]` and by a newline between its tokens. The filter applies those rules once.
//
// Two shapes cost the parser quadratic time on valid syntax, so no depth guard sees them: a run of consecutive
// literal tokens in one template (one per heredoc line or escape), which hclsyntax melds pairwise, and a run of `::`
// in a function name, which it concatenates one segment at a time. Both are bounded here, before the parser runs.
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
	// litRun counts consecutive literal tokens of one template and litWork sums their squares, which is the work
	// hclsyntax's meld of them costs. nsRun counts the `::` of one function name.
	litRun, litWork, nsRun := 0, 0, 0
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
		if t.Type == hclsyntax.TokenStringLit || t.Type == hclsyntax.TokenQuotedLit {
			litRun++
			litWork += 2*litRun - 1
			if litWork > maxLiteralWork {
				return nil, nil, limitErr(file, fmt.Sprintf("a template holds a run of %d literal pieces, past the %d-step melding budget", litRun, maxLiteralWork))
			}
		} else {
			litRun = 0
		}
		if t.Type == hclsyntax.TokenDoubleColon {
			nsRun++
			if nsRun > MaxDepth {
				return nil, nil, limitErr(file, fmt.Sprintf("a function name holds more than %d `::` segments", MaxDepth))
			}
		} else if t.Type != hclsyntax.TokenIdent {
			nsRun = 0
		}
		switch t.Type {
		case hclsyntax.TokenOBrack:
			if i+1 < len(tokens) && tokens[i+1].Type == hclsyntax.TokenStar {
				// A full splat `[*` and the token the parser reads as its close, whatever it is (`]` or, once the
				// parser is recovering from an error, any token): one more level of parser recursion for the rest of the chain.
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

// maxLiteralWork bounds, over one file, the sum of the squares of its runs of consecutive literal tokens. One run of
// 16,384 tokens, or many shorter runs adding up to as much, costs the parser about a tenth of a second.
const maxLiteralWork = 1 << 28

// boundTokens is the token stream the parser sees. Comments are dropped, except that a `#` or `//` comment absorbs
// the newline that ends it and the parser turns it back into a newline token where newlines count. A newline is
// dropped inside any bracket, and inside a brace whose first token is `for`, which the parser reads as a for
// expression and so stops looking at newlines in. Of the tokens after `[*`, the one the parser reads as the splat's
// close opens or closes nothing.
func boundTokens(lexed hclsyntax.Tokens) []hclsyntax.Token {
	out := make([]hclsyntax.Token, 0, len(lexed))
	var open []hclsyntax.TokenType
	// afterBrack is 1 right after a `[` and 2 right after a `[` `*`.
	afterBrack := 0
	for i, t := range lexed {
		if t.Type == hclsyntax.TokenComment {
			if n := len(t.Bytes); n == 0 || t.Bytes[n-1] != '\n' {
				continue
			}
			t.Type = hclsyntax.TokenNewline
		}
		if t.Type == hclsyntax.TokenNewline && len(open) > 0 && open[len(open)-1] != hclsyntax.TokenOBrace {
			continue
		}
		if afterBrack == 2 {
			afterBrack = 0
			out = append(out, t)
			continue
		}
		if afterBrack == 1 && t.Type == hclsyntax.TokenStar {
			afterBrack = 2
			open = open[:len(open)-1]
			out = append(out, t)
			continue
		}
		afterBrack = 0
		switch t.Type {
		case hclsyntax.TokenOBrack:
			open = append(open, t.Type)
			afterBrack = 1
		case hclsyntax.TokenOBrace:
			if opensForExpr(lexed[i+1:]) {
				// A for expression's brace drops newlines as a bracket does.
				open = append(open, hclsyntax.TokenOBrack)
			} else {
				open = append(open, t.Type)
			}
		case hclsyntax.TokenOParen, hclsyntax.TokenTemplateInterp, hclsyntax.TokenTemplateControl:
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

// opensForExpr reports whether the first token after a brace, skipping newlines and comments, is the identifier
// `for`, which is where the parser decides the brace opens a for expression.
func opensForExpr(rest hclsyntax.Tokens) bool {
	for _, t := range rest {
		switch t.Type {
		case hclsyntax.TokenNewline, hclsyntax.TokenComment:
			continue
		case hclsyntax.TokenIdent:
			return string(t.Bytes) == "for"
		}
		return false
	}
	return false
}
