package cognitive_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/cognitive"
)

const simpleSrc = `package p

func Add(a, b int) int {
	return a + b
}
`

const ifSrc = `package p

func Classify(n int) string {
	if n < 0 {          // +1 (depth 0) => score 1
		return "neg"
	} else if n == 0 {  // +1 else-if, no nesting => score 2
		return "zero"
	} else {            // +1 else => score 3
		return "pos"
	}
}
`

const loopSrc = `package p

func Sum(ns []int) int {
	sum := 0
	for _, n := range ns { // +1 (depth 0)
		if n > 0 {           // +1+1 (depth 1)
			sum += n
		}
	}
	return sum
}
`

const boolOpSrc = `package p

func Check(a, b, c bool) bool {
	return a && b || c  // +1 (&&) +1 (||) = 2
}
`

const nestedSwitchSrc = `package p

func F(x int) {
	switch x {            // +1 (depth 0)
	case 1:
		for i := 0; i < x; i++ { // +1+1 (depth 1)
			_ = i
		}
	}
}
`

const methodSrc = `package p

type T struct{}

func (t *T) Hello() string {
	if true {  // +1
		return "hi"
	}
	return ""
}
`

func TestScore_Simple(t *testing.T) {
	if got := cognitive.Score(simpleSrc); got != 0 {
		t.Fatalf("simple func: want 0, got %d", got)
	}
}

func TestScore_If(t *testing.T) {
	// if (+1) + else-if (+1) + else (+1) = 3
	if got := cognitive.Score(ifSrc); got != 3 {
		t.Fatalf("if/else-if/else: want 3, got %d", got)
	}
}

func TestScore_Loop(t *testing.T) {
	// range(+1) + if inside(+1+1) = 3
	if got := cognitive.Score(loopSrc); got != 3 {
		t.Fatalf("loop: want 3, got %d", got)
	}
}

func TestScore_BoolOps(t *testing.T) {
	// && (+1) + || (+1) = 2
	if got := cognitive.Score(boolOpSrc); got != 2 {
		t.Fatalf("bool ops: want 2, got %d", got)
	}
}

func TestScore_NestedSwitch(t *testing.T) {
	// switch(+1) + for inside(+1+1) = 3
	if got := cognitive.Score(nestedSwitchSrc); got != 3 {
		t.Fatalf("nested switch: want 3, got %d", got)
	}
}

func TestAnalyze_MethodReceiver(t *testing.T) {
	results, err := cognitive.Analyze("method.go", methodSrc)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("want 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Func != "T.Hello" {
		t.Errorf("want func=T.Hello, got %q", r.Func)
	}
	if r.Score != 1 {
		t.Errorf("want score=1, got %d", r.Score)
	}
	if r.File != "method.go" {
		t.Errorf("want file=method.go, got %q", r.File)
	}
}

func TestAnalyze_SortedDescending(t *testing.T) {
	src := `package p

func A() {
	if true { if true { if true {} } } // nesting heavy
}

func B() {} // score 0
`
	results, err := cognitive.Analyze("f.go", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) < 2 {
		t.Fatal("expected at least 2 results")
	}
	if results[0].Score < results[1].Score {
		t.Errorf("results should be descending by score; got %d then %d", results[0].Score, results[1].Score)
	}
}

func TestAnalyze_ParseError(t *testing.T) {
	_, err := cognitive.Analyze("bad.go", "not valid go !!!!")
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestScore_Determinism(t *testing.T) {
	for i := 0; i < 5; i++ {
		if got := cognitive.Score(loopSrc); got != 3 {
			t.Fatalf("iteration %d: want 3, got %d", i, got)
		}
	}
}

func TestScore_Empty(t *testing.T) {
	if got := cognitive.Score("package p"); got != 0 {
		t.Fatalf("empty: want 0, got %d", got)
	}
}
