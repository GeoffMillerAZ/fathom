package extractor

// phpdialect_test.go — LB-Q-php (design §13.2 steps 10, 12 and 15). The
// per-fixture goldens run in the generic TestDialectGoldens (dialects/php/
// testdata); this file holds a port's extra obligation: the incumbent's own
// contracts (phpextract_test.go) must hold for the dialect too, run here
// through both extractors so a divergence names which side moved.

import (
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func phpBothExtractors(t *testing.T) map[string]Extractor {
	t.Helper()
	d, err := NewTSExtractor("php")
	if err != nil {
		t.Fatalf("NewTSExtractor(php): %v", err)
	}
	t.Cleanup(d.Close)
	return map[string]Extractor{"incumbent": NewPhpExtractor(), "dialect": d}
}

type phpKey struct {
	name string
	kind core.SourceKind
}

func phpExtract(t *testing.T, e Extractor, path, src string) (map[phpKey]core.SymbolNode, []core.SymbolNode) {
	t.Helper()
	nodes, err := e.Extract("svc", path, []byte(src))
	if err != nil {
		t.Fatalf("Extract %s: %v", path, err)
	}
	m := map[phpKey]core.SymbolNode{}
	for _, n := range nodes {
		m[phpKey{n.Name, n.Kind}] = n
	}
	return m, nodes
}

// TestPhpDialectKeepsIncumbentContracts: names, kinds, tier, qualification
// and callee spelling the CGO walker's own tests pin (KindPhp* strings are
// byte-identical, §11 step 3).
func TestPhpDialectKeepsIncumbentContracts(t *testing.T) {
	for who, e := range phpBothExtractors(t) {
		t.Run(who+"/symbols", func(t *testing.T) {
			seen, nodes := phpExtract(t, e, "src/App.php", phpFixture)
			for _, w := range []phpKey{
				{"MyClass", core.KindPhpClass}, {"MyClass.myMethod", core.KindPhpMethod},
				{"MyInterface", core.KindPhpInterface}, {"MyInterface.doThing", core.KindPhpMethod},
				{"MyTrait", core.KindPhpTrait}, {"MyTrait.helper", core.KindPhpMethod},
				{"Status", core.KindPhpEnum}, {"freeFunction", core.KindPhpFunction},
			} {
				n, ok := seen[w]
				if !ok {
					t.Errorf("missing %+v", w)
					continue
				}
				if n.Tier != core.TierVerified || n.Service != "svc" || n.FilePath != "src/App.php" || n.EndLine <= 0 {
					t.Errorf("%+v: tier %q service %q file %q end %d", w, n.Tier, n.Service, n.FilePath, n.EndLine)
				}
			}
			for i := 1; i < len(nodes); i++ {
				if nodes[i].Line < nodes[i-1].Line {
					t.Errorf("not sorted by line at %d: %d after %d", i, nodes[i].Line, nodes[i-1].Line)
				}
			}
		})
		t.Run(who+"/method qualification", func(t *testing.T) {
			seen, _ := phpExtract(t, e, "Calculator.php", `<?php
class Calculator {
    public function add(int $a, int $b): int { return $a + $b; }
    public function sub(int $a, int $b): int { return $a - $b; }
}`)
			for _, n := range []string{"Calculator.add", "Calculator.sub"} {
				if _, ok := seen[phpKey{n, core.KindPhpMethod}]; !ok {
					t.Errorf("missing qualified %s", n)
				}
			}
			if _, ok := seen[phpKey{"add", core.KindPhpMethod}]; ok {
				t.Error("unqualified add appeared")
			}
		})
		t.Run(who+"/empty and determinism", func(t *testing.T) {
			if _, nodes := phpExtract(t, e, "empty.php", ""); len(nodes) != 0 {
				t.Errorf("empty source: %d nodes", len(nodes))
			}
			_, a := phpExtract(t, e, "App.php", phpFixture)
			_, b := phpExtract(t, e, "App.php", phpFixture)
			if len(a) != len(b) {
				t.Fatalf("non-deterministic: %d vs %d", len(a), len(b))
			}
			for i := range a {
				if a[i].Name != b[i].Name || a[i].Kind != b[i].Kind || a[i].Line != b[i].Line {
					t.Errorf("index %d differs: %+v vs %+v", i, a[i], b[i])
				}
			}
		})
		t.Run(who+"/call edges", func(t *testing.T) {
			seen, _ := phpExtract(t, e, "calls.php", phpCallFixture)
			c := seen[phpKey{"caller", core.KindPhpFunction}]
			if strings.Join(c.Calls, ",") != "callee,helper" {
				t.Errorf("caller.Calls = %v, want [callee helper]", c.Calls)
			}
			if w := seen[phpKey{"Svc.doWork", core.KindPhpMethod}]; strings.Join(w.Calls, ",") != "callee" {
				t.Errorf("Svc.doWork.Calls = %v, want [callee]", w.Calls)
			}
			if n := seen[phpKey{"callee", core.KindPhpFunction}]; len(n.Calls) != 0 {
				t.Errorf("callee.Calls = %v, want none", n.Calls)
			}
		})
	}
}

// TestPhpDialectBeyondIncumbent: what a port may add (§11 step 3) — the
// namespace as a definition and as the file's Package (design §3.5), enum
// members, a Signature on every callable and no call for a language
// construct or `new`.
func TestPhpDialectBeyondIncumbent(t *testing.T) {
	e, err := NewTSExtractor("php")
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	const src = `<?php
namespace Acme\Shop;

enum Level: int
{
    case Low = 1;

    public function label(): string
    {
        return ucfirst($this->name);
    }
}

function check(array $x, $y): bool
{
    $z = new Level();
    return isset($x[0]) && empty($y) && strlen('a') > 0;
}
`
	seen, nodes := phpExtract(t, e, "src/Level.php", src)
	if n, ok := seen[phpKey{"Acme\\Shop", "PhpNamespace"}]; !ok || n.Package != "Acme\\Shop" {
		t.Errorf("namespace symbol = %+v, ok %v", n, ok)
	}
	for _, n := range nodes {
		if n.Package != "Acme\\Shop" {
			t.Errorf("%s: Package %q, want the declared namespace", n.Name, n.Package)
		}
		if (n.Kind == core.KindPhpMethod || n.Kind == core.KindPhpFunction) && n.Signature == "" {
			t.Errorf("%s: no Signature", n.Name)
		}
	}
	if _, ok := seen[phpKey{"Level.label", core.KindPhpMethod}]; !ok {
		t.Error("an enum's method is a definition")
	}
	chk := seen[phpKey{"check", core.KindPhpFunction}]
	if strings.Join(chk.Calls, ",") != "strlen" {
		t.Errorf("check.Calls = %v, want [strlen] (isset/empty are constructs, new is not a call)", chk.Calls)
	}
}

// TestPhpDialectMembersMatchIncumbent: LB-F81 — the dialect emits the same
// property, constant and enum-case definitions as the CGO walker (name, kind,
// line), so the member rows are one fact whichever extractor ingest runs.
func TestPhpDialectMembersMatchIncumbent(t *testing.T) {
	ex := phpBothExtractors(t)
	got := map[string]map[string]int{}
	for who, e := range ex {
		_, nodes := phpExtract(t, e, "src/Box.php", phpMembersFixture)
		m := map[string]int{}
		for _, n := range nodes {
			switch n.Kind {
			case core.KindPhpProperty, core.KindPhpConst, core.KindPhpEnumCase:
				// The engine qualifies a definition by its immediate container only,
				// so a promoted constructor property (nested in its method) is the
				// bare "$name" in the dialect and "Owner.$name" in the CGO walker;
				// the member is compared by its last segment, which is what
				// lang-oracle's bare-name match reads.
				m[string(n.Kind)+" "+n.Name[strings.LastIndex(n.Name, ".")+1:]] = n.Line
			}
		}
		got[who] = m
	}
	if len(got["incumbent"]) != 12 {
		t.Errorf("incumbent members = %d, want 12: %v", len(got["incumbent"]), got["incumbent"])
	}
	for k, line := range got["incumbent"] {
		if got["dialect"][k] != line {
			t.Errorf("%s: incumbent line %d, dialect %d", k, line, got["dialect"][k])
		}
	}
	for k := range got["dialect"] {
		if _, ok := got["incumbent"][k]; !ok {
			t.Errorf("dialect-only member %s", k)
		}
	}
}

// TestPhpDialectCallSpelling (LB-F192): the php dialect's call text is what
// php-parser's own dump writes (call_recall and call_precision 1.0 against
// it on guzzle and monolog, 5,968 and 3,189 pairs): a scope or receiver
// qualifies the called name under calls.qualify (self::, static::, parent::,
// Foo::bar, $this->x(), $a?->x()), a plain function call is its name, and
// `new X` is no call, as scip-php writes it as a reference to the class
// (the 1,869 pairs scip-php lacks are method calls on receivers it cannot
// type, counted in docs/bench/readings/lang/php.json).
func TestPhpDialectCallSpelling(t *testing.T) {
	e, err := NewTSExtractor("php")
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	const src = `<?php
namespace Acme\Shop;

class Cart extends Base
{
    public function run(?Cart $other, $client): void
    {
        self::make();
        static::build();
        parent::run($other, $client);
        Helper::assist();
        $this->items();
        $this->client->send();
        $other?->items();
        $x = new Helper();
        strlen('a');
    }
}
`
	seen, _ := phpExtract(t, e, "src/Cart.php", src)
	run := seen[phpKey{"Cart.run", core.KindPhpMethod}]
	got := append([]string(nil), run.Calls...)
	sort.Strings(got)
	want := []string{"Helper::assist", "other.items", "parent::run", "self::make", "static::build", "strlen", "this.client.send", "this.items"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("Cart.run.Calls = %v, want %v", got, want)
	}
}
