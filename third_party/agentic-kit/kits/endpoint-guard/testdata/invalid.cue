// Package endpointguard — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
//
// The examples below would fail `cue vet` with the following errors:
//
//   invalidKeyRate_ZeroBurst:
//     Burst: 0 violates >=1 & int — Burst must be at least 1
//
//   invalidKeyRate_NegativeRate:
//     RatePerSec: -1 violates >0 & number — rate must be positive
//
//   invalidBudget_ZeroMaxUSD:
//     MaxUSDPerDay: 0 violates >0 & number — must be positive
//
//   invalidStrikes_ZeroThreshold:
//     Threshold: 0 violates >=1 & int — must be at least 1
//
//   invalidTopic_EmptyExemplars:
//     Exemplars: [] violates [string & !="", ...] — at least one exemplar required
//
//   invalidTopic_HighSimBelowLowSim:
//     HighSim: 0.2 with LowSim: 0.8 violates >=LowSim — HighSim must be >= LowSim
//
//   invalidPoW_ZeroBits:
//     Bits: 0 violates >=1 & int — at least 1 bit required
//
//   invalidPoW_EmptySecret:
//     Secret: "" violates string & !="" — either configure a real secret or
//     omit the field; an explicitly empty secret is a misconfiguration
//
//   invalidPoW_ZeroMinBits:
//     MinBits: 0 violates >=1 & int — the verification floor must be positive
//     (omit the field to default it to Bits)
//
//   invalidPoW_ZeroMaxSeen:
//     MaxSeen: 0 violates >=1 & int — the replay set must hold at least one
//     entry (omit the field to default it to 65536)
//
//   invalidIdentity_EmptySalt:
//     Salt: "" violates string & !="" — Salt must not be empty
//
//   invalidConfig_ZeroCostPerCall:
//     EstCostPerCallUSD: 0 violates >0 & number — must be positive
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package endpointguard

// // invalidKeyRate_ZeroBurst — Burst must be >= 1
// invalidKeyRate_ZeroBurst: #KeyRateConfig & {
// 	RatePerSec: 5.0
// 	Burst:      0
// }

// // invalidKeyRate_NegativeRate — RatePerSec must be > 0
// invalidKeyRate_NegativeRate: #KeyRateConfig & {
// 	RatePerSec: -1.0
// 	Burst:      10
// }

// // invalidBudget_ZeroMaxUSD — MaxUSDPerDay must be > 0
// invalidBudget_ZeroMaxUSD: #BudgetConfig & {
// 	MaxReqPerMin: 60
// 	MaxReqPerDay: 1000
// 	MaxUSDPerDay: 0
// }

// // invalidStrikes_ZeroThreshold — Threshold must be >= 1
// invalidStrikes_ZeroThreshold: #StrikesConfig & {
// 	Threshold:        0
// 	BaseBlockSeconds: 300
// 	MaxBlockSeconds:  86400
// }

// // invalidTopic_EmptyExemplars — at least one exemplar required
// invalidTopic_EmptyExemplars: #TopicConfig & {
// 	Exemplars: []
// 	MaxLen:    1000
// 	LowSim:    0.3
// 	HighSim:   0.7
// }

// // invalidTopic_HighSimBelowLowSim — HighSim must be >= LowSim
// invalidTopic_HighSimBelowLowSim: #TopicConfig & {
// 	Exemplars: ["weather"]
// 	MaxLen:    1000
// 	LowSim:    0.8
// 	HighSim:   0.2
// }

// // invalidPoW_ZeroBits — Bits must be >= 1
// invalidPoW_ZeroBits: #PoWConfig & {
// 	Enabled:    true
// 	Bits:       0
// 	TTLSeconds: 120
// }

// // invalidPoW_EmptySecret — omit Secret instead of setting it empty
// invalidPoW_EmptySecret: #PoWConfig & {
// 	Enabled:    true
// 	Secret:     ""
// 	Bits:       18
// 	TTLSeconds: 120
// }

// // invalidPoW_ZeroMinBits — MinBits must be >= 1 when present
// invalidPoW_ZeroMinBits: #PoWConfig & {
// 	Enabled:    true
// 	Bits:       18
// 	MinBits:    0
// 	TTLSeconds: 120
// }

// // invalidPoW_ZeroMaxSeen — MaxSeen must be >= 1 when present
// invalidPoW_ZeroMaxSeen: #PoWConfig & {
// 	Enabled:    true
// 	Bits:       18
// 	TTLSeconds: 120
// 	MaxSeen:    0
// }

// // invalidIdentity_EmptySalt — Salt must not be empty
// invalidIdentity_EmptySalt: #IdentityConfig & {
// 	Salt:             ""
// 	TrustedProxyHops: 0
// }

// // invalidConfig_ZeroCostPerCall — EstCostPerCallUSD must be > 0
// invalidConfig_ZeroCostPerCall: #Config & {
// 	// ... (all other fields valid)
// 	EstCostPerCallUSD: 0
// }
