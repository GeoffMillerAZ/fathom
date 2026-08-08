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
}

#Setting: {
	// on switches the capability on or off. Absent means "this scope says
	// nothing", which is different from off.
	on?: bool
}
