// #Capabilities — the schema for ~/.fathom/capabilities.cue (user scope)
// and <repo>/.fathom/capabilities.cue (project scope).
//
// docs/design/capability-registry.md §8. The struct is CLOSED over known
// ids so a typo is a CUE error before it is a Go error. The Go loader
// additionally refuses a known field declared OPTIONAL in an instance —
// `id?: on` in a config file would make the setting invisible while
// looking present, which is the phase-O trust-boundary inversion.

#Capabilities: {
	capabilities?: [string]: #Setting

	// budgets sizes the default answer budget per tool class, in TOKENS
	// (docs/design/budgeted-answers.md §3, BA-9). Resolved project → user
	// → built-in, the same law-13 precedence as capabilities: a user
	// scope may default a class, only the project governs it.
	budgets?: #Budgets
}

#Setting: {
	// on switches the capability on or off. Absent means "this scope says
	// nothing", which is different from off.
	on?: bool
}

// #Budgets is CLOSED over the three classes design §3 sizes; a typo is a
// CUE error before it is a Go error. A class absent here means "this
// scope says nothing", so fathom's measured built-in default stands.
//
// Each class sizes exactly the tools internal/mcp/budget_defaults.go
// (toolBudgetClasses) maps to it, named beside it — and nothing else.
// find_symbol, find_callers, symbol_context and outline are NOT budgeted
// by class: no value here changes them.
#Budgets: {
	list?:      int & >0 // query_symbols, references (built-in 1500)
	composite?: int & >0 // pack_context (built-in 4000)
	map?:       int & >0 // repo_map (built-in 1200)
}
