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
// Two shapes cost the parser quadratic time on valid syntax, so no depth guard sees them: the literal tokens of one
// template (one per heredoc line or escape), which hclsyntax melds pairwise and whose tail it moves each time, so the
// cost is melds times parts and bytes times melds, priced per template, and a run of `::`
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
	// tmpls holds one frame per open template, which prices the work hclsyntax's meld of its literal pieces costs
	// (see tmplFrame). fileMoves and fileBytes sum the settled templates. nsRun counts the `::` of one function name.
	var tmpls []tmplFrame
	var fileMoves, fileBytes int64
	nsRun := 0
	settle := func(f tmplFrame) error {
		if f.melds*f.parts > maxMeldMoves || f.concat > maxMeldBytes {
			return limitErr(file, fmt.Sprintf("a template melds %d literal pieces among %d parts and %d bytes, past the melding budget", f.melds, f.parts, f.concat))
		}
		fileMoves += f.melds * f.parts
		fileBytes += f.concat
		if fileMoves > maxMeldMovesFile || fileBytes > maxMeldBytesFile {
			return limitErr(file, "the templates of one file are past the melding budget in all")
		}
		return nil
	}
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
		if n := len(tmpls); n > 0 {
			f := &tmpls[n-1]
			switch t.Type {
			case hclsyntax.TokenStringLit, hclsyntax.TokenQuotedLit:
				f.parts++
				if size := int64(len(t.Bytes)); f.prevLit {
					// hclsyntax joins this piece onto the one before it: a string of the run's bytes so far.
					f.melds++
					f.runBytes += size
					f.concat += f.runBytes
				} else {
					f.runBytes = size
				}
				f.prevLit = true
			case hclsyntax.TokenTemplateInterp, hclsyntax.TokenTemplateControl:
				// `${` is no token of an expression, so one met while this template is the innermost open one
				// is a part of it.
				f.parts++
				f.prevLit = false
			default:
				f.prevLit = false
			}
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
			// Pop only the closer that matches the top. The parser's recover(`}`) swallows a mismatched closer
			// such as the `]}` of `{a]}[{` as one close while its frame stays, so popping on any closer lets the
			// count fall as the parser's depth grows (IAC29). Popping only on a match keeps the invariant that the
			// parser's frames are a subset of the counted ones: the count can only over-estimate.
			if len(kinds) > 0 && closes(kinds[len(kinds)-1], t.Type) {
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
			tmpls = append(tmpls, tmplFrame{})
		case hclsyntax.TokenCQuote, hclsyntax.TokenCHeredoc:
			run = 0
			if n := len(tmpls); n > 0 {
				f := tmpls[n-1]
				tmpls = tmpls[:n-1]
				if err := settle(f); err != nil {
					return nil, nil, err
				}
			}
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
	// A template the file leaves open ends with it.
	for _, f := range tmpls {
		if err := settle(f); err != nil {
			return nil, nil, err
		}
	}
	f, diags := hclsyntax.ParseConfig(src, file, hcl.InitialPos)
	return f, diags, nil
}

// tmplFrame counts one template's literal pieces. hclsyntax's meldConsecutiveStringLiterals joins each piece onto the
// one before it, which costs two things: a string concatenation of the run's bytes so far (concat sums them), and a
// memmove of every template part after it (melds times parts, an upper bound that is within a factor of two of the
// cost). Both are quadratic on valid syntax in one template, and neither shows in a depth guard.
type tmplFrame struct {
	parts, melds     int64
	runBytes, concat int64
	prevLit          bool
}

// The melding budgets. One template under them costs the parser about a tenth of a second: maxMeldMoves is melds times
// parts, which a 30,000 line heredoc exceeds and a 20,000 line one does not, and maxMeldBytes is the bytes the
// concatenations copy. The file budgets add up the templates, so many templates each just under the template budget
// cannot sum to minutes.
const (
	maxMeldMoves     = 1 << 29
	maxMeldBytes     = 1 << 31
	maxMeldMovesFile = 1 << 32
	maxMeldBytesFile = 1 << 32
)

// boundTokens is the token stream the parser sees. Comments are dropped, except that a `#` or `//` comment absorbs
// the newline that ends it and the parser turns it back into a newline token where newlines count. A newline is
// dropped inside any bracket, and inside a brace whose first token is `for`, which the parser reads as a for
// expression and so stops looking at newlines in. Of the tokens after `[*`, the one the parser reads as the splat's
// close opens or closes nothing.
func boundTokens(lexed hclsyntax.Tokens) []hclsyntax.Token {
	out := make([]hclsyntax.Token, 0, len(lexed))
	var open []hclsyntax.TokenType
	// opened holds the token that opened each level of open, which a for expression's brace hides (open records it as
	// a bracket, for the newline rule), so a closer pops only the level it matches.
	var opened []hclsyntax.TokenType
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
			opened = opened[:len(opened)-1]
			out = append(out, t)
			continue
		}
		afterBrack = 0
		switch t.Type {
		case hclsyntax.TokenOBrack:
			open = append(open, t.Type)
			opened = append(opened, t.Type)
			afterBrack = 1
		case hclsyntax.TokenOBrace:
			if opensForExpr(lexed[i+1:]) {
				// A for expression's brace drops newlines as a bracket does.
				open = append(open, hclsyntax.TokenOBrack)
			} else {
				open = append(open, t.Type)
			}
			opened = append(opened, t.Type)
		case hclsyntax.TokenOParen, hclsyntax.TokenTemplateInterp, hclsyntax.TokenTemplateControl:
			open = append(open, t.Type)
			opened = append(opened, t.Type)
		case hclsyntax.TokenCBrace, hclsyntax.TokenCBrack, hclsyntax.TokenCParen, hclsyntax.TokenTemplateSeqEnd:
			if n := len(opened); n > 0 && closes(opened[n-1], t.Type) {
				open = open[:len(open)-1]
				opened = opened[:n-1]
			}
		}
		out = append(out, t)
	}
	return out
}

// closes reports whether closer ends a level that opener began.
func closes(opener, closer hclsyntax.TokenType) bool {
	switch closer {
	case hclsyntax.TokenCBrace:
		return opener == hclsyntax.TokenOBrace
	case hclsyntax.TokenCBrack:
		return opener == hclsyntax.TokenOBrack
	case hclsyntax.TokenCParen:
		return opener == hclsyntax.TokenOParen
	case hclsyntax.TokenTemplateSeqEnd:
		return opener == hclsyntax.TokenTemplateInterp || opener == hclsyntax.TokenTemplateControl
	}
	return false
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
