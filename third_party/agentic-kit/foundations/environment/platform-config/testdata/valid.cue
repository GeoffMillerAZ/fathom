// valid.cue — positive fixtures for every type in schema.cue.
// Every value here must unify cleanly on `cue vet ./...`.
package platformconfig

import "github.com/geoffmilleraz/agentic-kit/foundations/environment/platform-config:platformconfig"

// minimalConfig demonstrates the smallest valid Config (empty Settings map).
minimalConfig: platformconfig.#Config & {
	Environment: "dev"
	Settings: {}
}

// richConfig demonstrates a Config with several settings populated.
richConfig: platformconfig.#Config & {
	Environment: "production"
	Settings: {
		log_level:   "warn"
		region:      "us-east-1"
		feature_x:   "enabled"
	}
}
