package b

import "example.com/loctruthfixture/a"

// Run calls into a.
func Run() int {
	t := &a.T{}
	return a.Foo() + t.M()
}

// Twice calls Foo on two lines.
func Twice() int {
	x := a.Foo()
	y := a.Foo()
	return x + y
}
