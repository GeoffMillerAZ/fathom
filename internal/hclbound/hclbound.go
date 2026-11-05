// Package hclbound is the one guard in front of the HCL parser (hclsyntax). It lexes a source, counts what the parser
// recurses on and what costs it quadratic time, and refuses a source past a bound before the parser sees it, so
// that a file inside the size limit cannot end the process with a stack overflow (Go's is fatal and no recover
// catches it) or hold a core for minutes. Both callers use it: the extractors (Terraform, Packer, Nomad, Terragrunt,
// docker-bake) and the stand-alone reads (internal/iacfacts).
//
// The guard is the union of what each caller counted before, and it counts one filtered token stream, the one the
// parser's peeker sees: a comment is dropped, and a newline inside any bracket and inside a brace that opens a `for`
// expression, so a shape matched over raw adjacent tokens (`[*]`, `%{if`) is missed by `[/**/*]` and by a newline
// between its tokens. It models the parser, and each model is a belief: the Reason of a refusal says which bound
// fired, and a shape no bound counts is read. It adds no bound for a cost shape that no file has been seen to hold.
//
// The package imports the standard library and hcl only, so both callers may use it.
package hclbound

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// Bounds are the limits one caller sets. The melding budgets are fixed.
type Bounds struct {
	// MaxDepth bounds the levels the parser recurses: open brackets, a run of prefix operators, the `[*]` steps of
	// a traversal chain, the `?` of conditionals and the open template directives, counted together. It also
	// bounds the `::` segments of one function name.
	MaxDepth int
	// MaxTokens bounds the tokens of one source. Zero is no bound.
	MaxTokens int
}

// Refusal is why Check refused a source.
type Refusal struct {
	// Bound names the bound that fired: tokens, depth, namespace, template or templates.
	Bound string
	// Reason is one line that states the bound, and no source text.
	Reason string
}

// Error returns the reason.
func (r *Refusal) Error() string { return r.Reason }

// The names of the bounds.
const (
	BoundTokens    = "tokens"
	BoundDepth     = "depth"
	BoundNamespace = "namespace"
	BoundTemplate  = "template"
	BoundTemplates = "templates"
)

// Check lexes src and returns nil when the parser may read it, or the refusal. file names the source for the
// lexer's positions only. A refusal is the whole answer: the source is not parsed.
//
// The parser recurses once per bracket, once per unary operator and once per `[*]` step of a traversal chain. A run
// of prefix operators and the splats of a chain count as nesting, added to the bracket depth they sit in, so mixed
// runs are bounded together.
//
// Two shapes cost the parser quadratic time on valid syntax, so no depth guard sees them: the literal tokens of one
// template (one per heredoc line or escape), which hclsyntax melds pairwise and whose tail it moves each time, so
// the cost is melds times parts and bytes times melds, priced per template, and a run of `::` in a function name,
// which it concatenates one segment at a time. Both are bounded here.
func Check(src []byte, file string, b Bounds) *Refusal {
	lexed, _ := hclsyntax.LexConfig(src, file, hcl.InitialPos)
	if b.MaxTokens > 0 && len(lexed) > b.MaxTokens {
		return &Refusal{BoundTokens, fmt.Sprintf("%d tokens, over the bound of %d, is not read", len(lexed), b.MaxTokens)}
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
	// post counts the bracket pairs that were an operand of what follows them (an index, a call's result): the AST
	// the parser builds nests one level deeper for each, and what walks it recurses on that depth. outer is the
	// operators and postfix steps pending in the expressions the open brackets interrupt, which are on the stack too.
	// pending holds each open level's own pair, to restore at its close. A comma or a statement boundary ends both
	// counts of the expression being read.
	post, outer := 0, 0
	var pending []pendingOps
	// tmpls holds one frame per open template, which prices the work hclsyntax's meld of its literal pieces costs
	// (see tmplFrame). fileMoves and fileBytes sum the settled templates. nsRun counts the `::` of one function name.
	var tmpls []tmplFrame
	var fileMoves, fileBytes int64
	nsRun := 0
	settle := func(f tmplFrame) *Refusal {
		if f.melds*f.parts > maxMeldMoves || f.concat > maxMeldBytes {
			return &Refusal{BoundTemplate, fmt.Sprintf("a template melds %d literal pieces among %d parts and %d bytes, past the melding budget of %d moves or %d bytes, and is not read", f.melds, f.parts, f.concat, int64(maxMeldMoves), int64(maxMeldBytes))}
		}
		fileMoves += f.melds * f.parts
		fileBytes += f.concat
		if fileMoves > maxMeldMovesFile || fileBytes > maxMeldBytesFile {
			return &Refusal{BoundTemplates, fmt.Sprintf("the templates of one file meld %d moves and %d bytes in all, past the file's melding budget of %d moves or %d bytes, and is not read", fileMoves, fileBytes, int64(maxMeldMovesFile), int64(maxMeldBytesFile))}
		}
		return nil
	}
	// openTemplate pushes the frame of a template the lexer opened: its pieces are priced in it, and a directive
	// left open in it ends with it.
	openTemplate := func() {
		ctlBase = append(ctlBase, ctl)
		tmpls = append(tmpls, tmplFrame{})
	}
	endChain := func() {
		top := len(chains) - 1
		splats -= chains[top]
		chains[top] = 0
	}
	// popLevel closes the innermost open level and restores the counts it interrupted.
	popLevel := func() {
		endChain()
		p := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		outer -= p.run + p.post
		run, post = p.run, p.post
		kinds = kinds[:len(kinds)-1]
		chains = chains[:len(chains)-1]
		quests -= quest[len(quest)-1]
		quest = quest[:len(quest)-1]
	}
	tooDeep := func() *Refusal {
		if len(kinds)+outer+run+post+splats+quests+ctl > b.MaxDepth {
			return &Refusal{BoundDepth, fmt.Sprintf("nesting deeper than %d (brackets, operators, splats, conditionals and directives together) is not read", b.MaxDepth)}
		}
		return nil
	}
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		if n := len(tmpls); n > 0 {
			f := &tmpls[n-1]
			switch {
			case f.bad > 0:
				// Inside a directive the parser reads and drops: it makes no part, so the literals around it
				// still meld (IAC35). The parser skips it with recover(`}`), which counts only the `${` and `%{`
				// it meets against the `}` that ends them and no bracket, so the directive ends at the first `}`
				// no nested opener claims, whatever brackets it left open (`%{f(}`). Those brackets are
				// dropped here too, so the close below ends the directive's own level.
				switch t.Type {
				case hclsyntax.TokenTemplateInterp, hclsyntax.TokenTemplateControl:
					f.badNest++
				case hclsyntax.TokenTemplateSeqEnd:
					if f.badNest > 0 {
						f.badNest--
						break
					}
					for len(kinds) > f.bad {
						popLevel()
					}
					f.bad = 0
				}
			case t.Type == hclsyntax.TokenStringLit || t.Type == hclsyntax.TokenQuotedLit:
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
			case t.Type == hclsyntax.TokenTemplateInterp:
				// `${` is no token of an expression, so one met while this template is the innermost open one
				// is a part of it.
				f.parts++
				f.prevLit = false
			case t.Type == hclsyntax.TokenTemplateControl:
				if isDirective(tokens, i+1) {
					f.parts++
					f.prevLit = false
				} else {
					f.bad = len(kinds) + 1 // the depth the control will have once it is pushed below
				}
			default:
				f.prevLit = false
			}
		}
		if t.Type == hclsyntax.TokenDoubleColon {
			nsRun++
			if nsRun > b.MaxDepth {
				return &Refusal{BoundNamespace, fmt.Sprintf("a function name of more than %d `::` segments is not read", b.MaxDepth)}
			}
		} else if t.Type != hclsyntax.TokenIdent {
			nsRun = 0
		}
		switch t.Type {
		case hclsyntax.TokenOBrack:
			if i+1 < len(tokens) && tokens[i+1].Type == hclsyntax.TokenStar && (len(tmpls) == 0 || tmpls[len(tmpls)-1].bad == 0) {
				// Not in a dropped directive: there the parser is recovering, reads `[*` as two plain tokens and stops
				// at the first `}` no nested opener claims, which the skip below would jump over (`%{f [*}`).
				//
				// A full splat `[*` and the token the parser reads as its close, whatever it is (`]` or, once the
				// parser is recovering from an error, any token): one more level of parser recursion for the rest of the chain.
				// The close opens or closes no bracket, but a quote or a heredoc it is still opens a template in the
				// lexer, whose end then closes a frame: the frame is pushed, or that end pops the enclosing
				// template's (IAC34).
				if i+2 < len(tokens) && (tokens[i+2].Type == hclsyntax.TokenOQuote || tokens[i+2].Type == hclsyntax.TokenOHeredoc) {
					openTemplate()
				}
				i += 2
				run = 0
				chains[len(chains)-1]++
				splats++
				if r := tooDeep(); r != nil {
					return r
				}
				continue
			}
			fallthrough
		case hclsyntax.TokenOBrace, hclsyntax.TokenOParen, hclsyntax.TokenTemplateInterp, hclsyntax.TokenTemplateControl:
			pending = append(pending, pendingOps{run, post})
			outer += run + post
			run, post = 0, 0
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
			if r := tooDeep(); r != nil {
				return r
			}
		case hclsyntax.TokenCBrace, hclsyntax.TokenCBrack, hclsyntax.TokenCParen, hclsyntax.TokenTemplateSeqEnd:
			// A closer that matches no level leaves the counts as they were: the parser never nests less than the
			// brackets nothing has matched, so a stray closer cannot hide a level.
			// Pop only the closer that matches the top. The parser's recover(`}`) swallows a mismatched closer
			// such as the `]}` of `{a]}[{` as one close while its frame stays, so popping on any closer lets the
			// count fall as the parser's depth grows (IAC29). Popping only on a match keeps the invariant that the
			// parser's frames are a subset of the counted ones: the count can only over-estimate.
			if len(kinds) > 0 && closes(kinds[len(kinds)-1], t.Type) {
				endChain()
				p := pending[len(pending)-1]
				pending = pending[:len(pending)-1]
				outer -= p.run + p.post
				run, post = p.run, p.post
				if t.Type != hclsyntax.TokenTemplateSeqEnd {
					post++ // the pair is an operand of what follows it
					if r := tooDeep(); r != nil {
						return r
					}
				}
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
			if r := tooDeep(); r != nil {
				return r
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
			openTemplate()
		case hclsyntax.TokenCQuote, hclsyntax.TokenCHeredoc:
			run = 0
			if n := len(tmpls); n > 0 {
				f := tmpls[n-1]
				tmpls = tmpls[:n-1]
				if r := settle(f); r != nil {
					return r
				}
			}
			if n := len(ctlBase); n > 0 {
				ctl = ctlBase[n-1]
				ctlBase = ctlBase[:n-1]
			}
		case hclsyntax.TokenBang, hclsyntax.TokenMinus:
			endChain()
			run++
			if r := tooDeep(); r != nil {
				return r
			}
		case hclsyntax.TokenComment:
			// Neither a comment nor, inside brackets, a newline ends a run.
		case hclsyntax.TokenNewline:
			if len(kinds) == 0 || kinds[len(kinds)-1] == hclsyntax.TokenOBrace {
				endChain()
				run, post = 0, 0
				top := len(quest) - 1
				quests -= quest[top]
				quest[top] = 0
			}
		case hclsyntax.TokenDot, hclsyntax.TokenIdent, hclsyntax.TokenNumberLit, hclsyntax.TokenStar,
			hclsyntax.TokenQuotedLit:
			// Steps and index keys a traversal chain continues through.
			run = 0
		case hclsyntax.TokenComma:
			endChain()
			run, post = 0, 0
		default:
			endChain()
			run = 0
		}
	}
	// A template the file leaves open ends with it.
	for _, f := range tmpls {
		if r := settle(f); r != nil {
			return r
		}
	}
	return nil
}

// isDirective reports whether the token at tokens[i] is the keyword that makes the `%{` before it a directive the
// parser keeps as a part of its template: if, else, endif, for or endfor. Any other `%{` is an error the parser
// reports and drops.
func isDirective(tokens []hclsyntax.Token, i int) bool {
	if i >= len(tokens) || tokens[i].Type != hclsyntax.TokenIdent {
		return false
	}
	switch string(tokens[i].Bytes) {
	case "if", "else", "endif", "for", "endfor":
		return true
	}
	return false
}

// pendingOps are the operators and postfix steps an open bracket interrupted.
type pendingOps struct{ run, post int }

// tmplFrame counts one template's literal pieces. hclsyntax's meldConsecutiveStringLiterals joins each piece onto the
// one before it, which costs two things: a string concatenation of the run's bytes so far (concat sums them), and a
// memmove of every template part after it (melds times parts, an upper bound that is within a factor of two of the
// cost). Both are quadratic on valid syntax in one template, and neither shows in a depth guard. bad is, while the
// template holds a directive the parser drops, the bracket depth that directive's close will have, and 0 otherwise.
type tmplFrame struct {
	parts, melds     int64
	runBytes, concat int64
	prevLit          bool
	bad              int
	badNest          int // the `${` and `%{` met inside the dropped directive whose `}` has not come yet
}

// The melding budgets. maxMeldMoves is melds times parts, which a 30,000 line heredoc exceeds and a 20,000 line one
// does not, and maxMeldBytes is the bytes the concatenations copy. A template under them costs the parser well under
// a second. The file budgets add up the templates, so many templates each just under the template budget cannot sum
// to minutes.
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
