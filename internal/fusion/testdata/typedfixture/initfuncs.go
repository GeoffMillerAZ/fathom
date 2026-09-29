package typedfixture

// Two init() funcs in the same package — goextract records both under the
// bare name "init", and the typed pass keys both as one caller,
// "typedfixture.init": the runtime runs every init of a linked package, so
// what either calls is reached from that key.

var initCountA int

func init() {
	initCountA = 1
	_ = helper()
}

var initCountB int

func init() {
	initCountB = 2
}
