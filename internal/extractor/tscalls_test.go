package extractor

import (
	"reflect"
	"slices"
	"testing"
)

// TestCompactCallee (LB-F65): with calls.qualify a whole-expression callee is
// kept as its receiver chain plus the called name, whitespace and argument
// or index contents dropped, PHP's sigils and arrows and TypeScript's
// optional chaining read as '.'; a callee that ends in no name (a subscript
// or a call's result called) has none and is dropped.
func TestCompactCallee(t *testing.T) {
	for in, want := range map[string]string{
		"f":                             "f",
		"self.foo":                      "self.foo",
		"a.b(x, y).c":                   "a.b().c",
		"text[:-1].rsplit":              "text[].rsplit",
		"this.metadataScanner\n  .scan": "this.metadataScanner.scan",
		"a?.b":                          "a.b",
		"f(\")\", g(1)).h":              "f().h",
		"x.filter((k) => {\n // it's a key\n return k }).map": "x.filter().map",
		"this.connect$": "this.connect$",
		"$":             "$",
		"obj[k]":        "",
		"f(x)":          "",
		"(lambda: 1)":   "",
		"m[\"x\"]":      "",
		"a(\"b).c":      "a(\"b).c",
	} {
		if got := compactCallee(in); got != want {
			t.Errorf("compactCallee(%q) = %q, want %q", in, got, want)
		}
	}
	// A receiver or scope (PHP's) also drops its sigils.
	for in, want := range map[string]string{
		"$this":               "this",
		"$this->client":       "this.client",
		"$x?->y":              "x.y",
		"\\GuzzleHttp\\Utils": "\\GuzzleHttp\\Utils",
		"(new Foo)":           "()",
	} {
		if got, ok := compactExpr(in, true); !ok || got != want {
			t.Errorf("compactExpr(%q, sigils) = %q, %v, want %q", in, got, ok, want)
		}
	}
}

func extractCalls(t *testing.T, dialect, rel, src string) map[string][]string {
	t.Helper()
	e, err := NewTSExtractor(dialect)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	nodes, err := e.Extract("svc", rel, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]string{}
	for _, n := range nodes {
		out[n.Name] = n.Calls
	}
	return out
}

// TestPythonCallQualification (LB-F65): the python dialect qualifies its call
// text (calls.qualify) and gives a call outside every function to the
// definition it belongs to: a decorator's call to the definition it
// decorates, a module-level assignment's call to the variable it defines.
func TestPythonCallQualification(t *testing.T) {
	calls := extractCalls(t, "python", "m.py", `import fastapi

app = fastapi.FastAPI(title="x")


@app.get("/items/{item_id}")
def read_item(item_id: int):
    rows = db.query(Item).filter(
        Item.id == item_id
    ).all()
    handlers["x"](1)
    return helper(rows)


class Service:
    @staticmethod
    @cache(maxsize=2)
    def build(self):
        return self.make()
`)
	for name, want := range map[string][]string{
		"app":           {"fastapi.FastAPI"},
		"read_item":     {"app.get", "db.query", "db.query().filter", "db.query().filter().all", "helper"},
		"Service.build": {"cache", "self.make"},
	} {
		if !reflect.DeepEqual(calls[name], want) {
			t.Errorf("%s.Calls = %q, want %q", name, calls[name], want)
		}
	}
	if calls["Service"] != nil {
		t.Errorf("Service.Calls = %q, want none: a decorated method's decorators are its own", calls["Service"])
	}
}

// TestPHPCallQualification (LB-F65): the php dialect composes a scoped call
// from its scope (Foo::bar, self::bar) and a member call from its receiver
// ($this->foo() is this.foo), so fusion can bind the qualifier.
func TestPHPCallQualification(t *testing.T) {
	calls := extractCalls(t, "php", "src/A.php", `<?php
namespace App;

use App\Util\Helper;

class A
{
    public function run(): void
    {
        Helper::assist();
        self::local();
        \App\Util\Other::make();
        $this->client->send();
        $this->helper();
        $x?->go();
        $this->make($a, $b)->send();
        (new Helper())->fluent();
        strlen("x");
        new Helper();
    }
}
`)
	want := []string{"().fluent", "Helper::assist", "\\App\\Util\\Other::make", "self::local", "strlen", "this.client.send", "this.helper", "this.make", "this.make().send", "x.go"}
	got := slices.Clone(calls["A.run"])
	if !reflect.DeepEqual(got, want) {
		t.Errorf("A.run Calls = %q, want %q", got, want)
	}
}
