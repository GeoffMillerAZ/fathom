package reachfix

// onlyInClosure is called ONLY inside a t.Run subtest closure — never at a
// test function's top level (VS-7).
func onlyInClosure() int {
	return 42
}
