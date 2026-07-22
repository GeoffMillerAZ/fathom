package typedfixture

// CallFuncValue calls a func-typed parameter — a func value, never
// statically resolvable to one declared symbol (EdgeUnresolved).
func CallFuncValue(f func() int) int {
	return f()
}

// UseFuncValue stores helper as a func value, then calls it indirectly —
// the call site (fn()) is still EdgeUnresolved: fn's static type is a bare
// func type, not the *types.Func for helper.
func UseFuncValue() int {
	fn := helper
	return fn()
}

// UseMethodValue stores a method value, then calls it indirectly — same
// EdgeUnresolved outcome as a func value.
func UseMethodValue(a AdapterA) string {
	fn := a.Complete
	return fn("y")
}
