package extractor

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestHeaderDialect pins the one header verdict (LB-Q-objective-c): a header
// goes to Objective-C when its code carries an @interface, @protocol or
// @implementation, or an #import and no C++ signal; else the C++ sniff decides
// between C++ and C. Comments and strings never count.
func TestHeaderDialect(t *testing.T) {
	for name, tc := range map[string]struct{ src, want string }{
		"interface":           {"@interface Foo : NSObject\n@end\n", "objective-c"},
		"protocol":            {"@protocol Bar <NSObject>\n@end\n", "objective-c"},
		"implementation":      {"@implementation Foo\n@end\n", "objective-c"},
		"import only":         {"#import <Foundation/Foundation.h>\nint f(void);\n", "objective-c"},
		"spaced import":       {"  #  import \"x.h\"\nint f(void);\n", "objective-c"},
		"msvc import in c++":  {"#import <comdef.h>\nclass Widget {};\n", "cpp"},
		"objc++ interface":    {"#import \"x.h\"\n@interface W : NSObject\n- (std::string)name;\n@end\nnamespace n {}\n", "objective-c"},
		"keyword in comment":  {"// @interface Foo\nint f(void);\n", "c"},
		"keyword in string":   {"const char *s = \"@interface\";\n", "c"},
		"longer word":         {"int @interfaces;\n", "c"},
		"plain c":             {"#include <stdio.h>\nstruct s { int a; };\n", "c"},
		"cpp class":           {"#pragma once\nclass Widget {};\n", "cpp"},
		"import word in name": {"#include \"important.h\"\nint f(void);\n", "c"},
	} {
		t.Run(name, func(t *testing.T) {
			if got := HeaderDialect([]byte(tc.src)); got != tc.want {
				t.Errorf("HeaderDialect = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestCHeaderRouterObjC is the router with an Objective-C leg: an Objective-C
// header is read by the Objective-C dialect, a C one still by C, and a router
// built without the leg (NewCHeaderRouter alone) reads everything as before.
func TestCHeaderRouterObjC(t *testing.T) {
	c, err := NewTSExtractor("c")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	cpp, err := NewTSExtractor("cpp")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cpp.Close)
	objc, err := NewTSExtractor("objective-c")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(objc.Close)
	src := []byte("#import <Foundation/Foundation.h>\n@interface Foo : NSObject\n- (void)run:(int)a with:(int)b;\n@end\n")

	without := NewCHeaderRouter(c, cpp)
	if got := without.Route(src); got != Extractor(c) {
		t.Errorf("a router with no Objective-C leg routed an Objective-C header to %T, want C", got)
	}
	r := NewCHeaderRouter(c, cpp).WithObjC(objc)
	if got := r.Route(src); got != Extractor(objc) {
		t.Fatalf("Route = %T, want the Objective-C extractor", got)
	}
	nodes, err := r.Extract("svc", "Foo.h", src)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]core.SourceKind{}
	for _, n := range nodes {
		got[n.Name] = n.Kind
	}
	if got["Foo"] != "ObjCInterface" || got["Foo.run:with:"] != "ObjCMethod" {
		t.Errorf("symbols = %v, want ObjCInterface Foo and ObjCMethod Foo.run:with:", got)
	}
	if cr := r.Route([]byte("struct s { int a; };\n")); cr != Extractor(c) {
		t.Errorf("a C header routed to %T, want C", cr)
	}
}

// TestObjCSelectorNames pins the selector as the name (the row's contract):
// a method, a class method and a message send are named by their full
// selector, a keyword selector joined from its parts and a zero-argument one
// bare, whatever the number of keywords and however the signature is split
// over lines.
func TestObjCSelectorNames(t *testing.T) {
	e, err := NewTSExtractor("objective-c")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.Close)
	src := `@implementation A
- (void)a { [self b:1 c:2 d:3]; [B zero]; [x e:1]; }
+ (instancetype)make:(int)one
               and:(int)two
              then:(int)three
               for:(int)four
              with:(int)five
               six:(int)six
             seven:(int)seven
             eight:(int)eight {
    return nil;
}
@end
`
	nodes, err := e.Extract("svc", "A.m", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"A":   true,
		"A.a": true,
		"A.make:and:then:for:with:six:seven:eight:": true,
	}
	var calls []string
	for _, n := range nodes {
		if !want[n.Name] {
			t.Errorf("unexpected symbol %q", n.Name)
		}
		delete(want, n.Name)
		if n.Name == "A.a" {
			calls = n.Calls
		}
	}
	if len(want) != 0 {
		t.Errorf("missing symbols %v", want)
	}
	if strings.Join(calls, ",") != "b:c:d:,e:,zero" {
		t.Errorf("calls = %v, want the selectors b:c:d:, e: and zero", calls)
	}
}

// TestObjCMacroTexts pins the pre-parse step: enumeration macros read as an
// enum of the same name at equal byte length, a bare availability macro line
// and a trailing decoration are blanked, and a file with no macro yields
// nothing to try.
func TestObjCMacroTexts(t *testing.T) {
	src := "NS_ASSUME_NONNULL_BEGIN\ntypedef NS_ENUM(NSInteger, Mode) {\n    ModeA,\n    ModeB API_DEPRECATED(\"x\", ios(8.0, 9.0)) = 2,\n};\nFOUNDATION_EXPORT NSString *Fn(void);\ntypedef NSString *Key NS_STRING_ENUM;\n- (void)f NS_UNAVAILABLE;\nNS_ASSUME_NONNULL_END\n"
	texts := objcMacroTexts([]byte(src))
	if len(texts) == 0 {
		t.Fatal("no rewrite")
	}
	for _, text := range texts {
		if len(text) != len(src) {
			t.Fatalf("length %d, want %d (equal byte length keeps every offset)", len(text), len(src))
		}
		if strings.Count(string(text), "\n") != strings.Count(src, "\n") {
			t.Fatal("a line break moved")
		}
	}
	got := string(texts[0])
	for _, gone := range []string{"NS_ENUM", "NS_ASSUME_NONNULL_BEGIN", "NS_ASSUME_NONNULL_END", "API_DEPRECATED", "FOUNDATION_EXPORT", "NS_STRING_ENUM", "NS_UNAVAILABLE", "typedef NS_"} {
		if strings.Contains(got, gone) {
			t.Errorf("%q survived the rewrite:\n%s", gone, got)
		}
	}
	if !strings.Contains(got, "enum Mode") || !strings.Contains(got, "NSString *Fn(void);") {
		t.Errorf("the enum or the prototype did not survive:\n%s", got)
	}
	if objcMacroTexts([]byte("@interface A\n@end\n")) != nil {
		t.Error("a file with no macro has a rewrite")
	}
	// return FOO_BAR; is an operand, never a decoration.
	if out := objcMacroTexts([]byte("int f(void) { return FOO_BAR; }\n")); out != nil {
		t.Errorf("return FOO_BAR; was rewritten: %q", out)
	}
}

// TestTSNamePartsKeepsTheLongest pins dropShorterParts: of the name.part
// matches over one node, only the longest selector stands.
func TestTSNamePartsKeepsTheLongest(t *testing.T) {
	defs := []*tsDef{
		{name: "a:", start: 1, end: 9, parts: 1},
		{name: "a:b:", start: 1, end: 9, parts: 2},
		{name: "c:", start: 20, end: 29, parts: 1},
		{name: "plain", start: 1, end: 9},
	}
	calls := []tsCallSite{
		{text: "x:", start: 3, end: 7, parts: 1},
		{text: "x:y:", start: 3, end: 7, parts: 2},
		{text: "z", start: 3, end: 7},
	}
	d, c := dropShorterParts(defs, calls)
	var names []string
	for _, x := range d {
		names = append(names, x.name)
	}
	if strings.Join(names, ",") != "a:b:,c:,plain" {
		t.Errorf("defs = %v", names)
	}
	var texts []string
	for _, x := range c {
		texts = append(texts, x.text)
	}
	if strings.Join(texts, ",") != "x:y:,z" {
		t.Errorf("calls = %v", texts)
	}
}
