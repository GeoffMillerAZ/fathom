// Package endpointguard — valid test data that must unify with schema.cue.
package endpointguard

import "github.com/geoffmilleraz/agentic-kit/kits/endpoint-guard:endpointguard"

// validConfig is a well-formed endpoint-guard configuration for a production
// deployment with PoW enabled and tight budget limits.
validConfig: endpointguard.#Config & {
	Identity: {
		Salt:             "super-secret-salt"
		TrustedProxyHops: 1
	}
	KeyRate: {
		RatePerSec: 5.0
		Burst:      10
	}
	GlobalRatePerSec: 50.0
	GlobalBurst:      200
	Breaker: {
		FailureThreshold:    3
		OpenCooldownSeconds: 30
		HalfOpenProbes:      2
	}
	Budget: {
		MaxReqPerMin: 60
		MaxReqPerDay: 5000
		MaxUSDPerDay: 10.0
	}
	Strikes: {
		Threshold:        3
		BaseBlockSeconds: 300
		MaxBlockSeconds:  86400
	}
	Topic: {
		Exemplars: ["what is the weather", "tell me today's news", "show me the forecast"]
		MaxLen:    1000
		LowSim:    0.3
		HighSim:   0.7
	}
	Cache: {
		TTLSeconds:   3600
		SimThreshold: 0.95
		MaxEntries:   500
	}
	PoW: {
		// Secret is deliberately omitted: it is set at deploy time (the kit
		// requires one when Enabled; production startup refuses to run
		// without it). Committed fixtures must never carry a real secret.
		Enabled:    true
		Bits:       16
		MinBits:    16
		TTLSeconds: 120
		MaxSeen:    65536
	}
	SystemPrompt:      "You are a helpful weather assistant."
	EstCostPerCallUSD: 0.002
	// Judge calls are budget-charged at this estimate before they fire.
	TopicJudgeCostUSD: 0.001
	// HTTPHandler refuses bodies over this size with a templated 413.
	MaxBodyBytes: 65536
}

// validConfigNoPow is a configuration with PoW disabled.
validConfigNoPow: endpointguard.#Config & {
	Identity: {
		Salt:             "dev-salt"
		TrustedProxyHops: 0
	}
	KeyRate: {
		RatePerSec: 10.0
		Burst:      20
	}
	GlobalRatePerSec: 200.0
	GlobalBurst:      1000
	Breaker: {
		FailureThreshold:    5
		OpenCooldownSeconds: 60
		HalfOpenProbes:      3
	}
	Budget: {
		MaxReqPerMin: 120
		MaxReqPerDay: 10000
		MaxUSDPerDay: 50.0
	}
	Strikes: {
		Threshold:        5
		BaseBlockSeconds: 60
		MaxBlockSeconds:  3600
	}
	Topic: {
		Exemplars: ["how can I help", "what would you like to know"]
		MaxLen:    2000
		LowSim:    0.2
		HighSim:   0.8
	}
	Cache: {
		TTLSeconds:   1800
		SimThreshold: 0.9
		MaxEntries:   1000
	}
	PoW: {
		Enabled:    false
		Bits:       1
		TTLSeconds: 60
	}
	SystemPrompt:      "You are a general-purpose assistant."
	EstCostPerCallUSD: 0.001
}

// validKeyRateConfig exercises various burst values.
validKeyRateMinBurst: endpointguard.#KeyRateConfig & {
	RatePerSec: 1.0
	Burst:      1
}

// validPoWConfig exercises all PoW fields. The optional Secret is present to
// validate its shape, but holds only a placeholder marker — never commit a
// real (base64) secret value to fixtures.
validPoWConfig: endpointguard.#PoWConfig & {
	Enabled:    true
	Secret:     "set-at-deploy-time-never-in-fixtures"
	Bits:       20
	MinBits:    16
	TTLSeconds: 300
	MaxSeen:    65536
}

// validPoWConfigDefaults omits the optional fields (MinBits defaults to Bits,
// MaxSeen to 65536; the secret is supplied at deploy time, never in fixtures).
validPoWConfigDefaults: endpointguard.#PoWConfig & {
	Enabled:    true
	Bits:       18
	TTLSeconds: 300
}
