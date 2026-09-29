package lib

// Wired is what the binary calls.
func Wired() int { return 1 }

// DeadViaBlank is kept alive for the linter with a blank var; nothing calls it.
func DeadViaBlank() int { return 2 }

var _ = DeadViaBlank

// Render shares its name with other.Render, which main calls.
func Render() int { return 4 }

// DeadViaUncalledMainHelper is called by a main-package func main only takes the value of.
func DeadViaUncalledMainHelper() int { return 5 }

// GenOnly is called only by a //go:build ignore generator.
func GenOnly() int { return 6 }

// TestOnly is the control: reached by a test only.
func TestOnly() int { return 7 }

// Stack's Len is the only first-party method named Len.
type Stack struct{}

func (Stack) Len() int { return StackOnly() }

func StackOnly() int { return 8 }
