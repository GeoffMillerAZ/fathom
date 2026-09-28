package extractor

import "testing"

// TestNormalizeScopedName pins the manifest's qualify.scope_sep rule
// (LB-Q-cpp, the CT-1 contract of docs/plans/cpp-toolchain.md carried onto
// the tags engine): a definition name or callee that spells a scope path with
// the language's scope operator is rewritten to the graph's one separator, so
// an out-of-class definition and an inline method land on the same key.
func TestNormalizeScopedName(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"render", "render"},
		{"Widget::render", "Widget.render"},
		{"ui::Widget::render", "ui.Widget.render"},
		{"a :: b", "a.b"},
		{"::global", "global"},
		{"std::move", "std.move"},
		// Template argument lists belong to the type, not the symbol's key.
		{"Box<T>::make", "Box.make"},
		{"ident<int>", "ident"},
		{"std::vector<int>::size", "std.vector.size"},
		{"A::B<std::pair<int, int>>::f", "A.B.f"},
		{"std::map<int, std::vector<int>>::at", "std.map.at"},
		{"Foo<(1 > 0)>::bar", "Foo.bar"},
		// Destructors and operators keep their spelling; "operator" text is
		// never mistaken for a template argument list.
		{"Foo::~Foo", "Foo.~Foo"},
		{"Str::operator+", "Str.operator+"},
		{"Str::operator<<", "Str.operator<<"},
		{"Box<T>::operator<=>", "Box.operator<=>"},
		{"operator<", "operator<"},
		{"operator()", "operator()"},
		{"Foo::operator()", "Foo.operator()"},
		{"Foo::operator  new[]", "Foo.operator new[]"},
		{"Foo::operator->", "Foo.operator->"},
		// An identifier that merely contains the word is not an operator.
		{"my_operator<int>", "my_operator"},
		// Unbalanced angle brackets are left alone rather than guessed at.
		{"weird<name", "weird<name"},
	} {
		if got := normalizeScopedName(tc.in, "::", "."); got != tc.want {
			t.Errorf("normalizeScopedName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestNormalizeScopedNameInactive: a manifest without scope_sep never has its
// names rewritten (every dialect but C++).
func TestNormalizeScopedNameInactive(t *testing.T) {
	r := &tsDialectRules{}
	if f := r.nameNormalizer(); f != nil {
		t.Fatalf("a dialect with no scope_sep must have no normalizer")
	}
	r = &tsDialectRules{scopeSep: "::", containerSep: "."}
	f := r.nameNormalizer()
	if f == nil || f("A::b") != "A.b" {
		t.Fatalf("scope_sep :: must normalize A::b to A.b")
	}
}
