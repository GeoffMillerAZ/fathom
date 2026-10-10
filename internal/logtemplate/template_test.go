package logtemplate

import (
	"reflect"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// tmpl reads src's facts and compiles them.
func tmpl(t *testing.T, path, src string) []*Template {
	t.Helper()
	var out []*Template
	for _, f := range Read(path, []byte(src)) {
		c, ok := Compile(f)
		if !ok {
			t.Fatalf("Compile(%v) failed", f)
		}
		out = append(out, c)
	}
	return out
}

func texts(ts []*Template) []string {
	var out []string
	for _, t := range ts {
		out = append(out, t.Text)
	}
	return out
}

// TestBestMatch: of the templates a line matches, the longest coverage
// wins (placeholders as wildcards), then the longest literal; an integer
// placeholder matches digits only; a literal edge is a word boundary; a
// JSON line's decoded values are matched too; a structlog event needs its
// keys.
func TestBestMatch(t *testing.T) {
	py := tmpl(t, "a.py", `import logging
log = logging.getLogger()
def f():
    log.info("order %s failed", o)
    log.info("order %s failed: %s", o, r)
    log.info("%s failed: %s", o, r)
    log.info("invoice %s total %d", i, n)
    log.info("count %d of %d items", a, b)
    log.info("count %s items", a)
    log.info("done", )
    log.info('charge "%s" declined', c)
`)
	cases := []struct {
		line string
		want []string
	}{
		{"2026-10-09 12:00:00,123 ERROR [shop.orders] order A-7 failed: timeout", []string{"order %s failed: %s"}},
		{"ERROR:shop.orders:order A-7 failed", []string{"order %s failed"}},
		{"INFO invoice INV-7 total 300", []string{"invoice %s total %d"}},
		{"INFO invoice INV-7 total many", nil},
		{"INFO count 5 of 9 items", []string{"count %d of %d items"}},
		{"the job was abandoned", nil},
		{"the job is done.", []string{"done"}},
		{`{"level":50,"time":1,"msg":"charge \"A-1\" declined","orderId":7}`, []string{`charge "%s" declined`}},
		{"nothing to see", nil},
	}
	for _, c := range cases {
		got, _ := Best(c.line, py)
		if g := texts(got); !reflect.DeepEqual(g, c.want) {
			t.Errorf("Best(%q) = %q, want %q", c.line, g, c.want)
		}
	}

	ev := tmpl(t, "b.py", `import structlog
log = structlog.get_logger()
def pay():
    log.error("payment_failed", order_id=o, reason=r)
`)
	for line, want := range map[string]bool{
		"2026-10-09 12:00:00 [error    ] payment_failed                 order_id=17 reason='card'": true,
		`{"event": "payment_failed", "order_id": 17, "reason": "card", "level": "error"}`:          true,
		"event='payment_failed' order_id=17":                                                       false,
		"payment_failed_twice order_id=1 reason=x":                                                 false,
	} {
		got, _ := Best(line, ev)
		if (len(got) == 1) != want {
			t.Errorf("event Best(%q) = %v, want match %v", line, texts(got), want)
		}
	}
}

// TestBestTiesAreAllReturnedInFileOrder: the same template written in two
// places ties; both come back, by file then line, never by map order.
func TestBestTiesAreAllReturnedInFileOrder(t *testing.T) {
	a := tmpl(t, "z/a.py", "import logging\nlog = logging.getLogger()\ndef f():\n    log.info('retry %s now', x)\n")
	b := tmpl(t, "b/b.py", "import logging\nlog = logging.getLogger()\ndef g():\n    log.info('retry %s now', x)\n")
	got, _ := Best("WARNING retry 3 now", append(a, b...))
	if len(got) != 2 || got[0].File != "b/b.py" || got[1].File != "z/a.py" {
		t.Errorf("tie = %v, want both, b/b.py first", got)
	}
}

// TestNodeRoundTrip: a fact stored as a LogTemplate observation reads back
// as the same fact, its enclosing symbol and package included.
func TestNodeRoundTrip(t *testing.T) {
	facts := Read("shop/cart.py", []byte(`import structlog
log = structlog.get_logger()
class Cart:
    def checkout(self):
        log.info("payment_failed", order_id=o, reason=r)
        log.info("coupon %s rejected for %d", c, n)
`))
	nodes := []core.SymbolNode{
		{Name: "Cart", Package: "shop.cart", Kind: "PythonClass", FilePath: "shop/cart.py", Line: 3, EndLine: 6},
		{Name: "Cart.checkout", Package: "shop.cart", Kind: "PythonFunc", FilePath: "shop/cart.py", Line: 4, EndLine: 6},
		{Name: "other", Kind: Kind, FilePath: "shop/cart.py", Line: 5, EndLine: 5},
	}
	Enclose(facts, nodes)
	if len(facts) != 2 {
		t.Fatalf("facts = %v", facts)
	}
	for _, f := range facts {
		if f.Enclosing != "Cart.checkout" || f.Package != "shop.cart" {
			t.Errorf("Enclose(%q) = %q %q, want Cart.checkout shop.cart", f.Text, f.Enclosing, f.Package)
		}
		obs := ToObservation("svc", f, "h", time.Unix(0, 0), false)
		back, ok := FromNode(obs.ToSymbolNode())
		if !ok || !reflect.DeepEqual(back, f) {
			t.Errorf("round trip:\n got  %+v\n want %+v", back, f)
		}
	}
	if _, ok := FromNode(core.SymbolNode{Kind: "PythonFunc", Name: "x", Signature: "python.logging"}); ok {
		t.Error("FromNode read a node of another kind")
	}
}
