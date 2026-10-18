package shellcmd

import "strings"

// ParseCommand lexes one Bash command line. It never fails: what it cannot
// read comes back with Unreadable set and Reason naming the first cause.
func ParseCommand(src string) Command {
	return *parseAt(src, 0)
}

func parseAt(src string, depth int) *Command {
	if depth > maxDepth {
		return unreadableCommand("nested too deep")
	}
	l := &lexer{src: src, depth: depth}
	if err := l.run(false); err != nil {
		return unreadableCommand(reasonOf(err))
	}
	c, err := parseTokens(l.toks, depth)
	if err != nil {
		return unreadableCommand(reasonOf(err))
	}
	return c
}

func unreadableCommand(reason string) *Command {
	return &Command{Unreadable: true, Reason: reason}
}

type parser struct {
	toks  []token
	i     int
	depth int
	nest  int // open subshells, groups and compounds
}

// maxNest bounds compound nesting in one command line.
const maxNest = 64

type stopFn func(t *token) bool

func stopEOF(*token) bool { return false }

func stopOp(op string) stopFn {
	return func(t *token) bool { return t.kind == tOp && t.text == op }
}

func stopWords(words ...string) stopFn {
	return func(t *token) bool {
		if t.kind != tWord || t.word.Quoted {
			return false
		}
		for _, w := range words {
			if t.text == w {
				return true
			}
		}
		return false
	}
}

func stopCaseArm(t *token) bool {
	if t.kind == tOp {
		return t.text == ";;" || t.text == ";&" || t.text == ";;&"
	}
	return t.kind == tWord && !t.word.Quoted && t.text == "esac"
}

func parseTokens(toks []token, depth int) (*Command, error) {
	p := &parser{toks: toks, depth: depth}
	c, err := p.list(stopEOF)
	if err != nil {
		return nil, err
	}
	if !p.eof() {
		return nil, p.unexpected()
	}
	return c, nil
}

func (p *parser) eof() bool    { return p.i >= len(p.toks) }
func (p *parser) peek() *token { return &p.toks[p.i] }
func (p *parser) isOp(op string) bool {
	return !p.eof() && p.peek().kind == tOp && p.peek().text == op
}

func (p *parser) isWord(w string) bool {
	return !p.eof() && p.peek().kind == tWord && !p.peek().word.Quoted && p.peek().text == w
}

func (p *parser) skipNewlines() {
	for p.isOp("\n") {
		p.i++
	}
}

func (p *parser) unexpected() error {
	if p.eof() {
		return fail("unexpected end of command")
	}
	t := p.peek()
	text := t.text
	switch {
	case t.kind == tArith:
		text = "(("
	case text == "\n":
		text = "newline"
	}
	return fail("unexpected `" + text + "`")
}

func (p *parser) expectWord(w string) error {
	if !p.isWord(w) {
		if p.eof() {
			return fail("missing `" + w + "`")
		}
		return p.unexpected()
	}
	p.i++
	return nil
}

func (p *parser) expectOp(op string) error {
	if !p.isOp(op) {
		if p.eof() {
			return fail("missing `" + op + "`")
		}
		return p.unexpected()
	}
	p.i++
	return nil
}

// list parses pipelines until stop matches at a command position, or the end.
func (p *parser) list(stop stopFn) (*Command, error) {
	c := &Command{}
	p.skipNewlines()
	for !p.eof() && !stop(p.peek()) {
		pl, err := p.pipeline()
		if err != nil {
			return nil, err
		}
		if p.eof() {
			c.Pipelines = append(c.Pipelines, pl)
			break
		}
		t := p.peek()
		if t.kind == tOp {
			switch t.text {
			case "&&", "||":
				pl.Sep = t.text
				p.i++
				p.skipNewlines()
				c.Pipelines = append(c.Pipelines, pl)
				if p.eof() || stop(p.peek()) {
					return nil, fail("nothing after `" + pl.Sep + "`")
				}
				continue
			case ";", "&", "\n":
				pl.Sep = t.text
				p.i++
				p.skipNewlines()
				c.Pipelines = append(c.Pipelines, pl)
				continue
			}
		}
		if stop(t) {
			c.Pipelines = append(c.Pipelines, pl)
			break
		}
		return nil, p.unexpected()
	}
	return c, nil
}

func (p *parser) pipeline() (Pipeline, error) {
	var pl Pipeline
	if p.isWord("!") {
		pl.Bang = true
		p.i++
		if p.eof() {
			return pl, fail("nothing after `!`")
		}
	}
	for {
		u, err := p.command()
		if err != nil {
			return pl, err
		}
		pl.Units = append(pl.Units, u)
		if p.isOp("|") || p.isOp("|&") {
			pl.Pipes = append(pl.Pipes, p.peek().text)
			p.i++
			p.skipNewlines()
			if p.eof() {
				return pl, fail("nothing after `|`")
			}
			continue
		}
		return pl, nil
	}
}

func (p *parser) command() (Unit, error) {
	if p.eof() {
		return Unit{}, p.unexpected()
	}
	p.nest++
	defer func() { p.nest-- }()
	if p.nest > maxNest {
		return Unit{}, fail("nested too deep")
	}
	t := p.peek()
	switch t.kind {
	case tArith:
		p.i++
		u := Unit{Kind: KindCompound, Keyword: "(("}
		return u, p.redirects(&u)
	case tOp:
		if t.text != "(" {
			return Unit{}, p.unexpected()
		}
		p.i++
		body, err := p.list(stopOp(")"))
		if err != nil {
			return Unit{}, err
		}
		if err := p.expectOp(")"); err != nil {
			return Unit{}, err
		}
		if len(body.Pipelines) == 0 {
			return Unit{}, fail("empty subshell")
		}
		u := Unit{Kind: KindSubshell, Body: body}
		return u, p.redirects(&u)
	case tWord:
		if !t.word.Quoted {
			switch t.text {
			case "{":
				p.i++
				body, err := p.list(stopWords("}"))
				if err != nil {
					return Unit{}, err
				}
				if err := p.expectWord("}"); err != nil {
					return Unit{}, err
				}
				if len(body.Pipelines) == 0 {
					return Unit{}, fail("empty group")
				}
				u := Unit{Kind: KindGroup, Body: body}
				return u, p.redirects(&u)
			case "if":
				return p.ifClause()
			case "while", "until":
				return p.loop(t.text)
			case "for", "select":
				return p.forClause(t.text)
			case "case":
				return p.caseClause()
			case "[[":
				return p.testClause()
			case "function":
				return Unit{}, fail("function definition")
			case "then", "elif", "else", "fi", "do", "done", "esac", "}":
				return Unit{}, p.unexpected()
			}
		}
		return p.simple()
	case tRedir:
		return p.simple()
	}
	return Unit{}, p.unexpected()
}

// redirects parses the redirections that follow a compound's closer.
func (p *parser) redirects(u *Unit) error {
	for !p.eof() && p.peek().kind == tRedir {
		if err := p.redir(u); err != nil {
			return err
		}
	}
	return nil
}

func (p *parser) redir(u *Unit) error {
	op := p.peek()
	p.i++
	if p.eof() || p.peek().kind != tWord {
		return fail("redirect `" + op.text + "` without a target")
	}
	tgt := p.peek()
	p.i++
	u.Redirects = append(u.Redirects, Redirect{Fd: op.fd, Op: op.text, Target: tgt.word, Heredoc: tgt.heredoc})
	u.Subst = append(u.Subst, tgt.subst...)
	return nil
}

// appendBody appends a parsed list's pipelines to body.
func appendBody(body, part *Command) {
	body.Pipelines = append(body.Pipelines, part.Pipelines...)
}

// part parses a non-empty list ended by one of the words, and consumes the
// word that ended it.
func (p *parser) part(what string, body *Command, ends ...string) (string, error) {
	c, err := p.list(stopWords(ends...))
	if err != nil {
		return "", err
	}
	if len(c.Pipelines) == 0 {
		return "", fail("empty " + what)
	}
	if p.eof() {
		return "", fail("missing `" + ends[len(ends)-1] + "`")
	}
	end := p.peek().text
	if err := p.expectWord(end); err != nil {
		return "", err
	}
	appendBody(body, c)
	return end, nil
}

func (p *parser) ifClause() (Unit, error) {
	p.i++
	body := &Command{}
	if _, err := p.part("if condition", body, "then"); err != nil {
		return Unit{}, err
	}
	for {
		end, err := p.part("then branch", body, "elif", "else", "fi")
		if err != nil {
			return Unit{}, err
		}
		if end == "elif" {
			if _, err := p.part("elif condition", body, "then"); err != nil {
				return Unit{}, err
			}
			continue
		}
		if end == "else" {
			if _, err := p.part("else branch", body, "fi"); err != nil {
				return Unit{}, err
			}
		}
		break
	}
	u := Unit{Kind: KindCompound, Keyword: "if", Body: body}
	return u, p.redirects(&u)
}

func (p *parser) loop(kw string) (Unit, error) {
	p.i++
	body := &Command{}
	if _, err := p.part(kw+" condition", body, "do"); err != nil {
		return Unit{}, err
	}
	if _, err := p.part(kw+" body", body, "done"); err != nil {
		return Unit{}, err
	}
	u := Unit{Kind: KindCompound, Keyword: kw, Body: body}
	return u, p.redirects(&u)
}

func (p *parser) forClause(kw string) (Unit, error) {
	p.i++
	u := Unit{Kind: KindCompound, Keyword: kw}
	switch {
	case !p.eof() && p.peek().kind == tArith && kw == "for":
		p.i++
	case !p.eof() && p.peek().kind == tWord:
		name := p.peek()
		if !name.word.Literal() || name.word.Quoted {
			return Unit{}, fail("`" + kw + "` variable is not a name")
		}
		u.Words = append(u.Words, name.word)
		p.i++
		p.skipNewlines()
		if p.isWord("in") {
			p.i++
			for !p.eof() && p.peek().kind == tWord {
				u.Words = append(u.Words, p.peek().word)
				u.Subst = append(u.Subst, p.peek().subst...)
				p.i++
			}
		}
	default:
		return Unit{}, p.unexpected()
	}
	if p.isOp(";") || p.isOp("\n") {
		p.i++
	}
	p.skipNewlines()
	if err := p.expectWord("do"); err != nil {
		return Unit{}, err
	}
	body := &Command{}
	if _, err := p.part(kw+" body", body, "done"); err != nil {
		return Unit{}, err
	}
	u.Body = body
	return u, p.redirects(&u)
}

func (p *parser) caseClause() (Unit, error) {
	p.i++
	if p.eof() || p.peek().kind != tWord {
		return Unit{}, fail("`case` without a subject")
	}
	u := Unit{Kind: KindCompound, Keyword: "case"}
	u.Words = append(u.Words, p.peek().word)
	u.Subst = append(u.Subst, p.peek().subst...)
	p.i++
	p.skipNewlines()
	if err := p.expectWord("in"); err != nil {
		return Unit{}, err
	}
	body := &Command{}
	for {
		p.skipNewlines()
		if p.isWord("esac") {
			p.i++
			break
		}
		if p.eof() {
			return Unit{}, fail("missing `esac`")
		}
		if p.isOp("(") {
			p.i++
		}
		for {
			if p.eof() || p.peek().kind != tWord {
				return Unit{}, fail("`case` pattern expected")
			}
			u.Subst = append(u.Subst, p.peek().subst...)
			p.i++
			if p.isOp("|") {
				p.i++
				continue
			}
			break
		}
		if err := p.expectOp(")"); err != nil {
			return Unit{}, err
		}
		arm, err := p.list(stopCaseArm)
		if err != nil {
			return Unit{}, err
		}
		if !p.eof() && p.peek().kind == tOp && stopCaseArm(p.peek()) {
			if n := len(arm.Pipelines); n > 0 {
				arm.Pipelines[n-1].Sep = p.peek().text
			}
			p.i++
		} else if !p.isWord("esac") {
			if p.eof() {
				return Unit{}, fail("missing `esac`")
			}
			return Unit{}, p.unexpected()
		}
		appendBody(body, arm)
	}
	u.Body = body
	return u, p.redirects(&u)
}

// testClause reads `[[ … ]]`: its operators are words, not control operators.
func (p *parser) testClause() (Unit, error) {
	p.i++
	u := Unit{Kind: KindCompound, Keyword: "[["}
	for {
		if p.eof() {
			return Unit{}, fail("missing `]]`")
		}
		t := p.peek()
		p.i++
		switch t.kind {
		case tWord:
			if !t.word.Quoted && t.text == "]]" {
				return u, p.redirects(&u)
			}
			u.Words = append(u.Words, t.word)
			u.Subst = append(u.Subst, t.subst...)
		case tOp:
			if t.text == "\n" {
				return Unit{}, fail("newline inside `[[`")
			}
			u.Words = append(u.Words, Word{Text: t.text})
		case tRedir:
			u.Words = append(u.Words, Word{Text: t.text})
		case tArith:
			u.Words = append(u.Words, Word{Text: "((" + t.text + "))"})
		}
	}
}

func (p *parser) simple() (Unit, error) {
	u := Unit{Kind: KindSimple}
	var words []token
	for !p.eof() {
		t := p.peek()
		if t.kind == tRedir {
			if err := p.redir(&u); err != nil {
				return Unit{}, err
			}
			continue
		}
		if t.kind == tWord {
			if len(words) == 0 {
				if a, ok := assignment(t); ok {
					u.Env = append(u.Env, a)
					u.Subst = append(u.Subst, t.subst...)
					p.i++
					continue
				}
			}
			words = append(words, *t)
			u.Subst = append(u.Subst, t.subst...)
			p.i++
			continue
		}
		if t.kind == tOp && t.text == "(" && len(words) == 1 && len(u.Env) == 0 {
			return Unit{}, fail("function definition")
		}
		break
	}
	if len(words) == 0 && len(u.Env) == 0 && len(u.Redirects) == 0 {
		return Unit{}, p.unexpected()
	}
	wrappers, env, rest, err := peel(words)
	if err != nil {
		return Unit{}, err
	}
	u.Wrappers = wrappers
	u.Env = append(u.Env, env...)
	for _, t := range rest {
		u.Words = append(u.Words, t.word)
	}
	if len(u.Words) == 0 {
		return u, nil
	}
	if !u.Words[0].Literal() {
		return Unit{}, fail("command name is not literal")
	}
	switch u.Name() {
	case "eval":
		return Unit{}, fail("eval runs text the lexer cannot read")
	}
	body, isShell, err := shellScript(u.Words, p.depth)
	if err != nil {
		return Unit{}, err
	}
	if isShell {
		u.Kind = KindShell
		u.Body = body
	}
	return u, nil
}

// assignment reads NAME=value or NAME+=value from an unquoted name.
func assignment(t *token) (Assign, bool) {
	raw := t.raw
	j := 0
	if j >= len(raw) || !isNameStart(raw[j]) {
		return Assign{}, false
	}
	for j < len(raw) && isNameChar(raw[j]) {
		j++
	}
	app := false
	if strings.HasPrefix(raw[j:], "+=") {
		app = true
	} else if !strings.HasPrefix(raw[j:], "=") {
		return Assign{}, false
	}
	name := raw[:j]
	skip := j + 1
	if app {
		skip++
	}
	v := t.word
	v.Text = v.Text[skip:]
	v.Glob = false
	return Assign{Name: name, Value: v, Append: app}, true
}
