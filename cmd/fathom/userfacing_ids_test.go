package main

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// internalIDRe matches the row-id shapes the project uses internally
// (AB-12, SR-17, F-68 style) and section references (§3.1): a stranger
// reading `fathom help` or `fathom doctor` has no way to resolve them.
var internalIDRe = regexp.MustCompile(`\b[A-Z]{1,5}-[0-9]+[a-z]?\b|§`)

// internalIDAllow are real tokens that merely look like an id.
var internalIDAllow = map[string]bool{"UTF-8": true, "UTF-16": true, "SHA-1": true, "SHA-256": true, "SHA-512": true, "ISO-8601": true, "ISO-8859": true, "TLS-1": true, "HTTP-2": true}

func internalIDs(s string) []string {
	var out []string
	for _, m := range internalIDRe.FindAllString(s, -1) {
		if !internalIDAllow[m] {
			out = append(out, m)
		}
	}
	return out
}

// TestHelpTextCarriesNoInternalIDs renders `fathom --help` and every verb's
// and subverb's help and pins that none names an internal row id.
func TestHelpTextCarriesNoInternalIDs(t *testing.T) {
	helpSandbox(t)
	var usage bytes.Buffer
	printUsage(&usage)
	if ids := internalIDs(usage.String()); len(ids) > 0 {
		t.Errorf("fathom --help: internal ids %v", ids)
	}
	for _, k := range helpKeys() {
		args := []string{k.verb}
		if k.sub != "" {
			args = append(args, k.sub)
		}
		args = append(args, "--help")
		var out, errb bytes.Buffer
		runWithStdin(args, strings.NewReader(""), &out, &errb)
		if ids := internalIDs(out.String()); len(ids) > 0 {
			t.Errorf("%v: internal ids %v\n%s", args, ids, out.String())
		}
	}
}

// TestDoctorTextCarriesNoInternalIDs scans every string literal in the
// doctor row sources (reasons, remedies, flag help): that text is rendered
// to the user.
func TestDoctorTextCarriesNoInternalIDs(t *testing.T) {
	files, _ := filepath.Glob("doctor*.go")
	fset := token.NewFileSet()
	bad := map[string]bool{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		af, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(af, func(n ast.Node) bool {
			bl, ok := n.(*ast.BasicLit)
			if !ok || bl.Kind != token.STRING {
				return true
			}
			s, err := strconv.Unquote(bl.Value)
			if err != nil {
				return true
			}
			if ids := internalIDs(s); len(ids) > 0 {
				bad[fset.Position(bl.Pos()).String()+" "+strings.Join(ids, ",")] = true
			}
			return true
		})
	}
	var list []string
	for k := range bad {
		list = append(list, k)
	}
	sort.Strings(list)
	for _, l := range list {
		t.Errorf("internal id in doctor text: %s", l)
	}
}
