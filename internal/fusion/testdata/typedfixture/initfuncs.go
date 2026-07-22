package typedfixture

// Two init() funcs in the same package — goextract records both under the
// bare name "init" (an ambiguous SymbolKey the store can hold multiple rows
// under), so the typed pass documented-skips init() as a caller entirely
// rather than attaching edges to a name that might resolve to the wrong row.

var initCountA int

func init() {
	initCountA = 1
	_ = helper()
}

var initCountB int

func init() {
	initCountB = 2
}
