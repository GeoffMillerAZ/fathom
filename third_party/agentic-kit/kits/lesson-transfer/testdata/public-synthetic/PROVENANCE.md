# Public synthetic fixture provenance

The reproducer and outcome remain public synthetic data. The following hashes
bind the lesson to the bounded, sanitized strict-engineering coverage defect;
they do not claim production metrics, user data, or receiver adoption.

| Field | Immutable source | SHA-256 of exact bytes |
| --- | --- | --- |
| `origin.snapshot_digest` | `87a11f4e1ef688c19c4cfa3ae82960eb19c93ae7:kits/agent-harness/engineering.go` | `9955737c1df3ea26e823325c89c67c700428f4638703a828506e310393093af5` |
| `origin.incident_digest` | Exact UTF-8 bounded summary: `Strict engineering profile pre-populated unrun coverage controls as pass, allowing unknown evidence to appear complete.` | `4255c32f60fa12116995505ec0bdfaafaf700ca557175a3479a3507a95b1391e` |
| `origin.scenario_digest` | `db2085da35e30c284c813f59d4ce8689d562a8b0:kits/agent-harness/engineering_test.go` | `405ea170f114a38bf3029db03c7470801690a50d4091617fce1700a1ec182f13` |
| `remedies[0].digest` | `db2085da35e30c284c813f59d4ce8689d562a8b0:kits/agent-harness/engineering.go` | `66248ea85be18c455a6a4440b2eade13b0a9d8bc70878f074bb26b2e1c954c4e` |
| `remedies[0].test_digest` | `db2085da35e30c284c813f59d4ce8689d562a8b0:kits/agent-harness/engineering_test.go` | `405ea170f114a38bf3029db03c7470801690a50d4091617fce1700a1ec182f13` |

Hashes establish byte linkage only. They do not authenticate a repository host,
authorize a receiver, or make a local result fresh; the receiver's own callbacks
and evidence supply those claims.
