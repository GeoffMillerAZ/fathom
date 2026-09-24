package typedfixture

// Greeter exercises the two receiver/call-syntax combinations DG-21 (O39)
// asks be reproduced directly: a VALUE-receiver method called both through
// a value variable and through a pointer variable (auto-deref), and a
// POINTER-receiver method called both through a pointer variable and
// through an addressable value variable (auto address-of). All four call
// sites must resolve to the SAME two symbol keys regardless of the call
// syntax used — namedTypeName (typedcalls.go) strips the pointer, but this
// fixture exists to prove that end to end through the real typed pass,
// not just at the unit level.
type Greeter struct {
	name string
}

// Greet is a VALUE receiver.
func (g Greeter) Greet() string { return "hello " + g.name }

// SetName is a POINTER receiver.
func (g *Greeter) SetName(n string) { g.name = n }

// GreetViaValue calls the value-receiver method through a value variable.
func GreetViaValue(g Greeter) string {
	return g.Greet()
}

// GreetViaPointer calls the SAME value-receiver method through a pointer
// variable — Go auto-dereferences; the resolved target is identical to
// GreetViaValue's.
func GreetViaPointer(g *Greeter) string {
	return g.Greet()
}

// SetNameViaPointer calls the pointer-receiver method through a pointer
// variable — the direct, unsurprising case.
func SetNameViaPointer(g *Greeter) {
	g.SetName("via-pointer")
}

// SetNameViaAddressableValue calls the SAME pointer-receiver method through
// an addressable value parameter — Go auto-takes the address; the resolved
// target is identical to SetNameViaPointer's.
func SetNameViaAddressableValue(g Greeter) {
	g.SetName("via-value")
}
