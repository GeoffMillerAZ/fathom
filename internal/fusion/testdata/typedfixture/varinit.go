package typedfixture

// Package-level variable initializers have no enclosing function: Go runs
// them during package initialization, so the typed pass keys what they
// reference to "typedfixture.init" (TypedEdgeEpoch 3) — a route table's
// handler and a func literal kept in a var so tests can swap it.

var routeTable = map[string]func() int{"only": onlyInVarTable}

var swappable = func() int { return onlyInVarClosure() }

func onlyInVarTable() int { return 4 }

func onlyInVarClosure() int { return 5 }
