package core

import (
	"strings"
	"text/scanner"
)

// TestIdentity is the source convention that makes n a test candidate, not
// proof that a runner discovered or executed it. C# requires a recognized
// NUnit/Unity test-shaped attribute; a method's name alone is insufficient.
// Short attribute names are deliberately labelled syntax: they do not resolve
// using aliases or a custom same-named attribute. Semantic index references
// and attested runner evidence remain separate facts.
func (n SymbolNode) TestIdentity() string {
	if n.Kind == KindCSharpMethod {
		if name := csharpTestAttribute(n.Signature); name != "" {
			return "csharp-attribute-syntax:" + name
		}
		return ""
	}
	if strings.HasPrefix(string(n.Kind), "CSharp") {
		return ""
	}
	if strings.HasPrefix(n.Name, "Test") {
		return "name-prefix"
	}
	return ""
}

func (n SymbolNode) IsTest() bool { return n.TestIdentity() != "" }

type attributeToken struct {
	kind rune
	text string
}

// Read only leading attribute lists. Quoted argument strings are opaque
// scanner tokens, and commas in arguments/arrays cannot start an attribute.
// An unreadable/truncated list is not a positive identity.
func csharpTestAttribute(signature string) string {
	var s scanner.Scanner
	s.Init(strings.NewReader(signature))
	s.Mode = scanner.ScanIdents | scanner.ScanStrings | scanner.ScanChars | scanner.ScanComments | scanner.SkipComments
	bad := false
	s.Error = func(*scanner.Scanner, string) { bad = true }
	var found string
	for tok := s.Scan(); tok == '['; tok = s.Scan() {
		depth, parens := 1, 0
		var part []attributeToken
		flush := func() {
			if name := testAttributeName(part); found == "" && name != "" {
				found = name
			}
			part = nil
		}
		for depth > 0 {
			tok = s.Scan()
			if tok == scanner.EOF || bad {
				return ""
			}
			switch tok {
			case '[':
				depth++
			case ']':
				depth--
			case '(':
				parens++
			case ')':
				parens--
			}
			if depth == 0 {
				flush()
				break
			}
			if tok == ',' && depth == 1 && parens == 0 {
				flush()
				continue
			}
			part = append(part, attributeToken{tok, s.TokenText()})
		}
	}
	if bad {
		return ""
	}
	return found
}

func testAttributeName(tokens []attributeToken) string {
	// A return-value attribute annotates a different target, never this test.
	if len(tokens) > 2 && tokens[0].kind == scanner.Ident && tokens[1].kind == ':' && tokens[2].kind != ':' {
		if tokens[0].text != "method" {
			return ""
		}
		tokens = tokens[2:]
	}
	var name strings.Builder
	for i, tok := range tokens {
		if tok.kind == '(' {
			break
		}
		if tok.kind != scanner.Ident && tok.kind != '.' && tok.kind != ':' {
			return ""
		}
		if tok.kind == scanner.Ident && i > 0 && tokens[i-1].kind == scanner.Ident {
			return ""
		}
		name.WriteString(tok.text)
	}
	declared := strings.TrimPrefix(name.String(), "global::")
	bare := strings.TrimSuffix(declared, "Attribute")
	switch bare {
	case "Test", "TestCase", "TestCaseSource", "Theory", "UnityTest",
		"NUnit.Framework.Test", "NUnit.Framework.TestCase", "NUnit.Framework.TestCaseSource", "NUnit.Framework.Theory",
		"UnityEngine.TestTools.UnityTest":
		return declared
	}
	return ""
}
