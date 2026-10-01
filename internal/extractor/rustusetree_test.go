package extractor

// rustusetree_test.go — LB-F83: the rust dialect's `use` trees compose across
// nested groups. A group's prefix is a path the query cannot hand to a member
// two braces deep, so the engine composes the enclosing prefixes outermost
// first (composeImportPrefix).

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

func rustImportsOf(t *testing.T, src string) []string {
	t.Helper()
	e, err := NewTSExtractor("rust")
	if err != nil {
		t.Fatalf("NewTSExtractor(rust): %v", err)
	}
	defer e.Close()
	specs, err := e.ExtractImportSpans("src/lib.rs", []byte(src))
	if err != nil {
		t.Fatalf("ExtractImportSpans: %v", err)
	}
	var out []string
	for _, s := range specs {
		kind := map[ImportKind]string{ImportNamed: "named", ImportWhole: "whole", ImportWildcard: "wildcard"}[s.Kind]
		out = append(out, fmt.Sprintf("%s %s<-%s as %s @%d-%d", kind, s.Module, s.ExportedName, s.LocalName, s.StartLine, s.EndLine))
	}
	sort.Strings(out)
	return out
}

// TestRustDialectUseTreeImports: nested use trees, every depth, with the
// self, alias, wildcard and path members the flat forms already had.
func TestRustDialectUseTreeImports(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		want      []string
	}{
		{"flat group unchanged", "use a::b::{c, d as e};", []string{
			"named a::b<-c as c @1-1",
			"named a::b<-d as e @1-1",
		}},
		{"nested group", "use a::{b::{c, d}, e};", []string{
			"named a::b<-c as c @1-1",
			"named a::b<-d as d @1-1",
			"named a<-e as e @1-1",
		}},
		{"three deep with alias, self and wildcard", "use a::{b::{c::{d, self}, e as f, g::*}, h};", []string{
			"named a::b::c<-d as d @1-1",
			"whole a::b::c<- as c @1-1",
			"named a::b<-e as f @1-1",
			"wildcard a::b::g<- as * @1-1",
			"named a<-h as h @1-1",
		}},
		{"path member inside a group", "use crate::flags::{defs::FLAGS, CompletionType};", []string{
			"named crate::flags::defs<-FLAGS as FLAGS @1-1",
			"whole crate::flags::defs::FLAGS<- as FLAGS @1-1",
			"named crate::flags<-CompletionType as CompletionType @1-1",
		}},
		{"top-level use list", "use { bstr::BString, grep::printer::{ColorSpecs, SummaryKind}, };", []string{
			"named bstr<-BString as BString @1-1",
			"whole bstr::BString<- as BString @1-1",
			"named grep::printer<-ColorSpecs as ColorSpecs @1-1",
			"named grep::printer<-SummaryKind as SummaryKind @1-1",
		}},
		{"self and super roots, multi-line span", "use super::{\n    x::{y, z},\n    w,\n};", []string{
			"named super::x<-y as y @1-4",
			"named super::x<-z as z @1-4",
			"named super<-w as w @1-4",
		}},
		{"two statements stay apart", "use a::{b::{c}};\nuse d::{e::{f}};", []string{
			"named a::b<-c as c @1-1",
			"named d::e<-f as f @2-2",
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := rustImportsOf(t, tc.src)
			want := append([]string(nil), tc.want...)
			sort.Strings(want)
			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Errorf("specs\n got:\n  %s\nwant:\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
			}
		})
	}
}
