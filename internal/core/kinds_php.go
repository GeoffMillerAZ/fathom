package core

// PHP member kinds (LB-F81, language-breadth). A property (declared or a
// promoted constructor parameter), a class/interface/trait constant and an
// enum case are definitions scip-php writes (suffix "."), so def_mapping
// counts them; they are named Owner.name, a property with its `$`.
const (
	KindPhpProperty SourceKind = "PhpProperty"
	KindPhpConst    SourceKind = "PhpConst"
	KindPhpEnumCase SourceKind = "PhpEnumCase"
)
