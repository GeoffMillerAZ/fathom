package reg

// T's method is all the binary calls in this package.
type T struct{}

func (T) Run() int { return 1 }

var registered int

func init() { registered = InitOnly() }

// InitOnly runs at start-up because main links reg.
func InitOnly() int { return 2 }

// A package-level variable's initializer runs at start-up like an init(),
// and its edges are the variable's own (O1124): a func literal kept in a
// var as a test seam, and a route table built in one.
var seam = func() int { return SeamOnly() }

// SeamOnly is called only inside seam's func literal.
func SeamOnly() int { return 3 }

var table = map[string]func() int{"t": TableOnly}

// TableOnly is registered only in table.
func TableOnly() int { return 4 }
