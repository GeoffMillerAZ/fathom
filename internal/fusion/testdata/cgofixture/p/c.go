package p

/*
static int two(void) { return 2; }
*/
import "C"

// Doer is satisfied by *H, which only this cgo file constructs.
type Doer interface{ Do() int }

type H struct{}

func New() Doer { return &H{} }

func (h *H) Do() int { return int(C.two()) + helper() }
