package extractor_test

import (
	"slices"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// callSitesFixture pins LB-F66: collectCalls records a method called on any
// receiver expression (a selector chain, a call's result), and leaves out a
// name in a position no function value can take — a type (a composite
// literal's, a declaration's, a type assertion's, new/make's argument), an
// operand of a comparison or arithmetic, a case value, the base of a further
// selector, and a selector's own field or method name. A function referenced
// as a value where it can be one (an argument) is still recorded (VS-6).
const callSitesFixture = `package p

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"strings"
)

type client struct {
	Header http.Header
}

func (c *client) helper() int { return 1 }

func helper() int { return 1 }

func handler(w http.ResponseWriter, r *http.Request) {}

func Use(c *client, resp *http.Response) {
	var buf bytes.Buffer
	opts := http.Server{Addr: ":0"}
	p := &http.Client{}
	hs := []http.Handler{}
	if resp.StatusCode == http.StatusNoContent || resp.StatusCode+1 > http.StatusOK {
	}
	switch resp.StatusCode {
	case http.StatusAccepted:
	}
	fmt.Fprintln(os.Stderr, "x")
	os.Stdout.WriteString("x")
	resp.Header.Get("a")
	strings.NewReader("x").Len()
	c.Header.Set("a", "b")
	n := c.helper()
	http.HandleFunc("/", handler)
	var v interface{} = resp
	if _, ok := v.(*http.Response); ok {
	}
	q := new(bytes.Reader)
	m := make(map[string]http.Handler)
	_, _, _, _, _, _, _ = buf, opts, p, hs, n, q, m
}
`

func TestGoExtractor_CallSites(t *testing.T) {
	x := extractor.NewGoExtractor()
	nodes, err := x.Extract("svc", "p/p.go", []byte(callSitesFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	var calls []string
	for _, n := range nodes {
		if n.Name == "Use" {
			calls = n.Calls
		}
	}
	if calls == nil {
		t.Fatal("no Use symbol or no calls")
	}
	for _, want := range []string{
		"fmt.Fprintln", "http.HandleFunc", "strings.NewReader", "new", "make",
		"os.Stdout.WriteString",   // a package variable's method
		"resp.Header.Get",         // a field's method
		"strings.NewReader().Len", // a call result's method
		"c.Header.Set",            // a receiver field's method
		"c.helper",                // as before: a receiver variable's method
		"svc::p.handler",          // a function passed as a value (VS-6)
		"os.Stderr",               // an argument may be a function value: kept
	} {
		if !slices.Contains(calls, want) {
			t.Errorf("Use.Calls lacks %q: %v", want, calls)
		}
	}
	for _, bad := range []string{
		"bytes.Buffer", "http.Server", "http.Client", "http.Handler", "http.Response", "bytes.Reader", // types
		"http.StatusNoContent", "http.StatusOK", "http.StatusAccepted", // operands and case values
		"os.Stdout",     // the base of a further selector
		"svc::p.helper", // c.helper's method name is not the package func helper
	} {
		if slices.Contains(calls, bad) {
			t.Errorf("Use.Calls records %q, which no function value can be: %v", bad, calls)
		}
	}
	if !slices.IsSorted(calls) {
		t.Errorf("Use.Calls not sorted: %v", calls)
	}
}
