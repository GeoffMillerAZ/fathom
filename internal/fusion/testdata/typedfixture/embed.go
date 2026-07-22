package typedfixture

// Base declares Hello; Derived embeds Base so Hello is promoted.
type Base struct{}

// Hello is declared on Base.
func (Base) Hello() string { return "hello" }

// Derived embeds Base, promoting Hello.
type Derived struct {
	Base
}

// CallPromoted calls a promoted method through the embedding type — the
// edge must resolve to Base.Hello (the declaring type), matching how
// goextract itself would have named the symbol (GoExtractor.Extract records
// a method's Package as its literal receiver type, "Base", never the
// embedding type).
func CallPromoted(d Derived) string {
	return d.Hello()
}
