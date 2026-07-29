package guard

// A well-formed guard file (docs/design/guarded-edits.md §2.1's worked
// example): one file-scoped guard with a note and modules included, one
// command-scoped guard (§2.5), one leaning entirely on schema defaults.
withNoteAndModules: #Guards & {
	guards: [{
		scopes:          ["internal/store/**", "**/migrations/**"]
		note:            "Store schema is spec-governed; read docs/spec first."
		max_rules:       4
		include_modules: true
	}]
}

commandGuard: #Guards & {
	guards: [{
		command_scopes: ["terraform apply*", "kubectl *", "*/migrate *"]
		note: "Infra commands are guarded; check the runbook first."
	}]
}

minimalDefaults: #Guards & {
	guards: [{
		scopes: ["**/*.go"]
	}]
}
