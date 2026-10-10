package main

import (
	"bytes"
	"flag"
	"regexp"
	"strings"
	"testing"
)

// defaultsOf is the flag.PrintDefaults output of a FlagSet defining flags.
func defaultsOf(flags []helpFlag) string {
	var b bytes.Buffer
	fs := flag.NewFlagSet("x", flag.ContinueOnError)
	fs.SetOutput(&b)
	for _, f := range flags {
		defineHelpFlag(fs, f)
	}
	fs.PrintDefaults()
	return b.String()
}

// TestStaticFlagsMatchPrintDefaults (CL-10) holds printStaticFlags to
// flag.PrintDefaults of the equivalent FlagSet, byte for byte, for every
// static key, and for a table covering every type.
func TestStaticFlagsMatchPrintDefaults(t *testing.T) {
	for key, flags := range verbHelpStatic() {
		var got bytes.Buffer
		printStaticFlags(&got, key, flags)
		want := "Usage of " + key + ":\n" + defaultsOf(flags)
		if got.String() != want {
			t.Errorf("%s:\ngot:\n%s\nwant:\n%s", key, got.String(), want)
		}
	}
	all := []helpFlag{
		{"s", "string", "", "one-letter string"},
		{"b", "bool", "", "one-letter bool"},
		{"word", "string", "x", "a string"},
		{"flag", "bool", "true", "a bool"},
		{"n", "int", "3", "an int"},
		{"big", "int64", "", "an int64"},
		{"u", "uint", "", "a uint"},
		{"u64", "uint64", "7", "a uint64"},
		{"f", "float64", "1.5", "a float"},
		{"wait", "duration", "2s", "a duration"},
		{"v", "var", "", "a Var"},
		{"t", "text", "", "a TextVar"},
		{"fn", "func", "", "a Func"},
		{"bf", "boolfunc", "", "a BoolFunc"},
		{"q", "string", "", "names a `thing` in back quotes"},
		{"multi", "string", "", "two\nlines"},
	}
	var got bytes.Buffer
	printStaticFlags(&got, "all", all)
	if want := "Usage of all:\n" + defaultsOf(all); got.String() != want {
		t.Errorf("all types:\ngot:\n%s\nwant:\n%s", got.String(), want)
	}
	for _, w := range []string{"  -wait duration\n", "  -u64 uint\n", "  -big int\n", "  -f float\n", "  -v value\n", "  -bf\n", "  -q thing\n"} {
		if !strings.Contains(got.String(), w) {
			t.Errorf("all types lack %q:\n%s", w, got.String())
		}
	}
}

var (
	flagLineShape = regexp.MustCompile(`^  -\S+( \S+)?$`)
	oneLetterTab  = regexp.MustCompile(`^  -\S \S*\t`)
)

// TestHelpShapesAgree (CL-12) runs runVerbHelp for a delegated verb and for
// static verbs and holds every flag line of both to one shape: a flag line
// of at most a name and a type, then a tab-indented usage line.
func TestHelpShapesAgree(t *testing.T) {
	for _, target := range []struct{ verb, sub, typed string }{{"observe", "", ""}, {"map", "new", ""}, {"edit-hook", "", "  -url string\n"}, {"serve-http", "", "  -ttl duration\n"}, {"plans", "lease", "  -holder string\n"}} {
		v, ok := lookupVerb(target.verb)
		if !ok {
			t.Fatalf("no verb %q", target.verb)
		}
		var b bytes.Buffer
		if runVerbHelp(&b, v, target.sub) != 0 {
			t.Fatalf("%s %s: help exit != 0", target.verb, target.sub)
		}
		if !strings.Contains(b.String(), target.typed) {
			t.Errorf("%s %s: help lacks the typed line %q:\n%s", target.verb, target.sub, target.typed, b.String())
		}
		lines := strings.Split(b.String(), "\n")
		flagLines := 0
		for i, l := range lines {
			if !strings.HasPrefix(l, "  -") {
				continue
			}
			flagLines++
			if oneLetterTab.MatchString(l) {
				continue
			}
			if !flagLineShape.MatchString(l) {
				t.Errorf("%s %s: flag line %q is not '  -name [type]'", target.verb, target.sub, l)
			}
			if i+1 >= len(lines) || !strings.HasPrefix(lines[i+1], "    \t") {
				t.Errorf("%s %s: flag line %q is not followed by a '    \\t' usage line", target.verb, target.sub, l)
			}
		}
		if flagLines == 0 {
			t.Errorf("%s %s: help printed no flag lines:\n%s", target.verb, target.sub, b.String())
		}
	}
}
