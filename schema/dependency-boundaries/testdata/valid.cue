package dependencyboundaries

sample: #Rules & {
	schema_version: 1
	rules: [
		{
			id: "standalone"
			from: ["pkg/lang/**/*.go"]
			deny: ["internal/store/**", "go:net/http/**", "oci:registry.test:5000/team/*", "."]
			except: [{from: ["pkg/lang/cue/*.go"], to: ["internal/store/test[0-9]"]}]
			reason: "Keep the standalone door small."
		},
		{
			id: "local-only"
			from: ["*.go"]
			allow_only: ["lib/**", "npm:@scope/*"]
			reason: "Use the reviewed library."
		},
	]
}
