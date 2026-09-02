package a

// Foo calls Bar.
func Foo() int { return Bar() }

// Bar is a leaf.
func Bar() int { return 1 }

// T has a method.
type T struct{}

// M calls Foo.
func (t *T) M() int { return Foo() }

// I is dispatched dynamically; calls through it are not static callers.
type I interface{ M() int }

// Via calls I.M, never T.M statically.
func Via(i I) int { return i.M() }

// Gen is generic.
func Gen[V any](v V) V { return v }

// UseGen instantiates Gen.
func UseGen() int { return Gen(1) }

var _ = Foo()
