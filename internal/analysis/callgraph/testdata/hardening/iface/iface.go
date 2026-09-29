package iface

type Doer interface{ Do() int }

type Real struct{}

func (Real) Do() int { return 1 }

// Fake implements Doer, but only a test constructs it.
type Fake struct{}

func (Fake) Do() int { return FakeOnly() }

func FakeOnly() int { return 2 }
