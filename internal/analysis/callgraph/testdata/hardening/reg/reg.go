package reg

// T's method is all the binary calls in this package.
type T struct{}

func (T) Run() int { return 1 }

var registered int

func init() { registered = InitOnly() }

// InitOnly runs at start-up because main links reg.
func InitOnly() int { return 2 }
