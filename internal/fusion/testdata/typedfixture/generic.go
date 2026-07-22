package typedfixture

// Stack is a minimal generic type; Push's edge must map to the generic
// definition ("Stack.Push"), not a fabricated per-instantiation symbol.
type Stack[T any] struct {
	items []T
}

// Push appends v — declared on the generic definition Stack[T].
func (s *Stack[T]) Push(v T) {
	s.items = append(s.items, v)
}

// CallGeneric instantiates Stack[int] and calls Push — a direct call
// through a generic instantiation.
func CallGeneric() {
	s := &Stack[int]{}
	s.Push(1)
}
