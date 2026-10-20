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

// A command guard naming the token a matching command must carry.
requiresGuard: #Guards & {
	guards: [{
		command_scopes: ["go test *"]
		requires: ["-race"]
		note: "The gate runs go test -race."
	}]
}

minimalDefaults: #Guards & {
	guards: [{
		scopes: ["**/*.go"]
	}]
}

// One layer_rule per kind (VB-13) — the same worked shapes
// internal/guardspec's golden tests exercise.
layerRules: #Guards & {
	guards: []
	layer_rules: [
		{
			id:       "store-no-cgo"
			kind:     "import_direction"
			sentence: "internal/store must not import cgo-backed packages."
			from: ["store"]
			to: ["extractor"]
		},
		{
			id:       "internal-no-cycles"
			kind:     "no_cycles"
			sentence: "no import cycles under internal/."
			scope: ["internal/**"]
		},
		{
			id:       "ledger-boundary"
			kind:     "boundary"
			sentence: "only payment may import ledger."
			allow: ["payment"]
			guarded: ["ledger"]
		},
		{
			id:          "payment-not-ledger"
			kind:        "cross_service_direction"
			sentence:    "payment-service never depends on ledger-service."
			severity:    "warn"
			from_service: "payment-service"
			to_service:   "ledger-service"
		},
		{
			id:       "ledger-not-calling-payment"
			kind:     "call_direction"
			sentence: "ledger code never calls payment code."
			from: ["ledger"]
			to: ["payment"]
			allow: false
		},
	]
}
