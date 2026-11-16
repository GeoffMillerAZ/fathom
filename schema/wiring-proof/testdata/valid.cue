// Synthetic fixtures use injected times; they certify no installation.
package wiringproof

{
	"wired_preflight": {
		"session_id": "session"
		"cell_id":    "cell"
		"arm":        "assigned"
		"phase":      "preflight"
		"profile": {
			"id":               "fixture"
			"expected_service": "fixture-service"
			"product_present":  true
			"lab":              "lab-fixture"
			"bench_harness":    "replay"
			"pins": {
				"harness":                    "codex-cli"
				"harness_pin":                "synthetic-1"
				"model_pin":                  "fixture-1"
				"transcript_contract_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"product_commit":             "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit":             "dddddddddddddddddddddddddddddddddddddddd"
				"plugin_digest":              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"stanza_sha":                 "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"description_sha":            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			}
			"environment": {
				"kind":        "local"
				"host_class":  "macos"
				"harness_pin": "synthetic-1"
				"permitted_tools": [
					"read"
				]
				"startup_instructions_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"action_capture_digest":       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			}
			"channels": [
				{
					"channel":        "descriptions"
					"enabled":        false
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "hooks"
					"enabled":        true
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "maps"
					"enabled":        false
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "prime"
					"enabled":        true
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "readouts"
					"enabled":        false
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "skills"
					"enabled":        false
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "stanza"
					"enabled":        false
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "surface"
					"enabled":        true
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
			]
			"hooks": [
				{
					"id":             "hook-a"
					"enabled":        true
					"trigger_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"event":          "synthetic"
					"timeout_ns":     10
				}
			]
			"surface_tools": [
				"tool-a"
			]
			"requirements": [
				{
					"rung":       "W0"
					"applicable": true
					"reason":     "frozen applicability"
				},
				{
					"rung":       "W1"
					"applicable": true
					"reason":     "frozen applicability"
				},
				{
					"rung":       "W2"
					"applicable": true
					"reason":     "frozen applicability"
				},
				{
					"rung":       "W3"
					"applicable": true
					"reason":     "frozen applicability"
				},
				{
					"rung":       "W6"
					"applicable": true
					"reason":     "frozen applicability"
				},
			]
			"workload_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"budget": {
				"tenant_policy_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"bounds": [
					{
						"metric":  "hook_p95_ns"
						"maximum": 100
					},
					{
						"metric":  "index_age_ns"
						"maximum": 100
					},
					{
						"metric":  "payload_age_ns"
						"maximum": 100
					},
					{
						"metric":  "phys_footprint_bytes"
						"maximum": 100
					},
				]
			}
			"budget_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"source_digests": [
				{
					"id":     "delivery_parameters"
					"digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				}
			]
			"profile_digest":     "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"environment_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"frozen_at":          5
		}
		"started_at":                 10
		"finished_at":                20
		"decision_at":                21
		"agent_start_at":             30
		"startup_environment_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"startup_profile_digest":     "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"workload_digest":            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"budget_digest":              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"rungs": {
			"w0": {
				"verdict": "PASS"
				"reason":  "synthetic observation"
				"evidence_ids": [
					"b-revision"
				]
			}
			"w1": {
				"verdict": "PASS"
				"reason":  "synthetic observation"
				"evidence_ids": [
					"c-registration",
					"d-negotiation",
					"e-probe",
				]
			}
			"w2": {
				"verdict": "PASS"
				"reason":  "synthetic observation"
				"evidence_ids": [
					"f-startup"
				]
			}
			"w3": {
				"verdict": "PASS"
				"reason":  "synthetic observation"
				"evidence_ids": [
					"g-trigger",
					"h-event",
					"i-beat",
				]
			}
			"w6": {
				"verdict": "PASS"
				"reason":  "synthetic observation"
				"evidence_ids": [
					"j-resource"
				]
			}
		}
		"served_product_commit":    "cccccccccccccccccccccccccccccccccccccccc"
		"installed_product_commit": "cccccccccccccccccccccccccccccccccccccccc"
		"index_commit":             "dddddddddddddddddddddddddddddddddddddddd"
		"registration": {
			"service": "fixture-service"
			"surface_tools": [
				"tool-a"
			]
			"evidence_ids": [
				"c-registration"
			]
		}
		"primer_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"negotiated_tools": [
			"tool-a"
		]
		"answered_probe_ids": [
			"e-probe"
		]
		"hook_observations": [
			{
				"hook_id":             "hook-a"
				"trigger_evidence_id": "g-trigger"
				"event_evidence_id":   "h-event"
				"beat_evidence_id":    "i-beat"
			}
		]
		"absences": [
			{
				"channel": "descriptions"
				"evidence_ids": [
					"a-inventory"
				]
			},
			{
				"channel": "maps"
				"evidence_ids": [
					"a-inventory"
				]
			},
			{
				"channel": "readouts"
				"evidence_ids": [
					"a-inventory"
				]
			},
			{
				"channel": "skills"
				"evidence_ids": [
					"a-inventory"
				]
			},
			{
				"channel": "stanza"
				"evidence_ids": [
					"a-inventory"
				]
			},
		]
		"samples": [
			{
				"metric": "hook_p95_ns"
				"value":  1
				"evidence_ids": [
					"j-resource"
				]
			},
			{
				"metric": "index_age_ns"
				"value":  1
				"evidence_ids": [
					"j-resource"
				]
			},
			{
				"metric": "payload_age_ns"
				"value":  1
				"evidence_ids": [
					"j-resource"
				]
			},
			{
				"metric": "phys_footprint_bytes"
				"value":  1
				"evidence_ids": [
					"j-resource"
				]
			},
		]
		"evidence": [
			{
				"id":          "a-inventory"
				"service":     "fixture-service"
				"source_kind": "channel_inventory"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "b-revision"
				"service":     "fixture-service"
				"source_kind": "revision"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "c-registration"
				"service":     "fixture-service"
				"source_kind": "registration"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "d-negotiation"
				"service":     "fixture-service"
				"source_kind": "negotiation"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "e-probe"
				"service":     "fixture-service"
				"source_kind": "call_result"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "f-startup"
				"service":     "fixture-service"
				"source_kind": "startup_delivery"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "g-trigger"
				"service":     "fixture-service"
				"source_kind": "hook_trigger"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "h-event"
				"service":     "fixture-service"
				"source_kind": "hook_event"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "i-beat"
				"service":     "fixture-service"
				"source_kind": "beat"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "j-resource"
				"service":     "fixture-service"
				"source_kind": "resource_sample"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "k-scripted"
				"service":     "fixture-service"
				"source_kind": "action"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
		]
	}
	"baseline_preflight": {
		"session_id": "session"
		"cell_id":    "cell"
		"arm":        "assigned"
		"phase":      "preflight"
		"profile": {
			"id":               "fixture"
			"expected_service": "fixture-service"
			"product_present":  false
			"lab":              "lab-fixture"
			"bench_harness":    "replay"
			"pins": {
				"harness":                    "codex-cli"
				"harness_pin":                "synthetic-1"
				"model_pin":                  "fixture-1"
				"transcript_contract_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"product_commit":             "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit":             "dddddddddddddddddddddddddddddddddddddddd"
				"plugin_digest":              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"stanza_sha":                 "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"description_sha":            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			}
			"environment": {
				"kind":        "local"
				"host_class":  "macos"
				"harness_pin": "synthetic-1"
				"permitted_tools": [
					"read"
				]
				"startup_instructions_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"action_capture_digest":       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			}
			"channels": [
				{
					"channel":        "descriptions"
					"enabled":        false
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "hooks"
					"enabled":        false
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "maps"
					"enabled":        false
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "prime"
					"enabled":        false
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "readouts"
					"enabled":        false
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "skills"
					"enabled":        false
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "stanza"
					"enabled":        false
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "surface"
					"enabled":        false
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
			]
			"hooks": [
				{
					"id":             "hook-a"
					"enabled":        false
					"trigger_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"event":          "synthetic"
					"timeout_ns":     10
				}
			]
			"surface_tools": []
			"requirements": [
				{
					"rung":       "W0"
					"applicable": false
					"reason":     "frozen applicability"
				},
				{
					"rung":       "W1"
					"applicable": false
					"reason":     "frozen applicability"
				},
				{
					"rung":       "W2"
					"applicable": false
					"reason":     "frozen applicability"
				},
				{
					"rung":       "W3"
					"applicable": false
					"reason":     "frozen applicability"
				},
				{
					"rung":       "W6"
					"applicable": true
					"reason":     "frozen applicability"
				},
			]
			"workload_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"budget": {
				"tenant_policy_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"bounds": [
					{
						"metric":  "phys_footprint_bytes"
						"maximum": 100
					}
				]
			}
			"budget_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"source_digests": [
				{
					"id":     "delivery_parameters"
					"digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				}
			]
			"profile_digest":     "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"environment_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"frozen_at":          5
		}
		"started_at":                 10
		"finished_at":                20
		"decision_at":                21
		"agent_start_at":             30
		"startup_environment_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"startup_profile_digest":     "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"workload_digest":            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"budget_digest":              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"rungs": {
			"w0": {
				"verdict": "NOT-APPLICABLE"
				"reason":  "synthetic observation"
				"evidence_ids": [
					"a-inventory"
				]
			}
			"w1": {
				"verdict": "NOT-APPLICABLE"
				"reason":  "synthetic observation"
				"evidence_ids": [
					"a-inventory"
				]
			}
			"w2": {
				"verdict": "NOT-APPLICABLE"
				"reason":  "synthetic observation"
				"evidence_ids": [
					"a-inventory"
				]
			}
			"w3": {
				"verdict": "NOT-APPLICABLE"
				"reason":  "synthetic observation"
				"evidence_ids": [
					"a-inventory"
				]
			}
			"w6": {
				"verdict": "PASS"
				"reason":  "synthetic observation"
				"evidence_ids": [
					"j-resource"
				]
			}
		}
		"negotiated_tools": []
		"answered_probe_ids": []
		"hook_observations": []
		"absences": [
			{
				"channel": "descriptions"
				"evidence_ids": [
					"a-inventory"
				]
			},
			{
				"channel": "hooks"
				"evidence_ids": [
					"a-inventory"
				]
			},
			{
				"channel": "maps"
				"evidence_ids": [
					"a-inventory"
				]
			},
			{
				"channel": "prime"
				"evidence_ids": [
					"a-inventory"
				]
			},
			{
				"channel": "readouts"
				"evidence_ids": [
					"a-inventory"
				]
			},
			{
				"channel": "skills"
				"evidence_ids": [
					"a-inventory"
				]
			},
			{
				"channel": "stanza"
				"evidence_ids": [
					"a-inventory"
				]
			},
			{
				"channel": "surface"
				"evidence_ids": [
					"a-inventory"
				]
			},
		]
		"samples": [
			{
				"metric": "phys_footprint_bytes"
				"value":  1
				"evidence_ids": [
					"j-resource"
				]
			}
		]
		"evidence": [
			{
				"id":          "a-inventory"
				"service":     "fixture-service"
				"source_kind": "channel_inventory"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "b-revision"
				"service":     "fixture-service"
				"source_kind": "revision"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "c-registration"
				"service":     "fixture-service"
				"source_kind": "registration"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "d-negotiation"
				"service":     "fixture-service"
				"source_kind": "negotiation"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "e-probe"
				"service":     "fixture-service"
				"source_kind": "call_result"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "f-startup"
				"service":     "fixture-service"
				"source_kind": "startup_delivery"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "g-trigger"
				"service":     "fixture-service"
				"source_kind": "hook_trigger"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "h-event"
				"service":     "fixture-service"
				"source_kind": "hook_event"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "i-beat"
				"service":     "fixture-service"
				"source_kind": "beat"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "j-resource"
				"service":     "fixture-service"
				"source_kind": "resource_sample"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "k-scripted"
				"service":     "fixture-service"
				"source_kind": "action"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
		]
		"baseline_checks": {
			"harness_evidence_id":        "b-revision"
			"tools_evidence_id":          "a-inventory"
			"startup_evidence_id":        "f-startup"
			"action_capture_evidence_id": "k-scripted"
		}
	}
	"hook_only_preflight": {
		"session_id": "session"
		"cell_id":    "cell"
		"arm":        "assigned"
		"phase":      "preflight"
		"profile": {
			"id":               "fixture"
			"expected_service": "fixture-service"
			"product_present":  true
			"lab":              "lab-fixture"
			"bench_harness":    "replay"
			"pins": {
				"harness":                    "codex-cli"
				"harness_pin":                "synthetic-1"
				"model_pin":                  "fixture-1"
				"transcript_contract_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"product_commit":             "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit":             "dddddddddddddddddddddddddddddddddddddddd"
				"plugin_digest":              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"stanza_sha":                 "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"description_sha":            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			}
			"environment": {
				"kind":        "local"
				"host_class":  "macos"
				"harness_pin": "synthetic-1"
				"permitted_tools": [
					"read"
				]
				"startup_instructions_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"action_capture_digest":       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			}
			"channels": [
				{
					"channel":        "descriptions"
					"enabled":        false
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "hooks"
					"enabled":        true
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "maps"
					"enabled":        false
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "prime"
					"enabled":        true
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "readouts"
					"enabled":        false
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "skills"
					"enabled":        false
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "stanza"
					"enabled":        false
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "surface"
					"enabled":        false
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
			]
			"hooks": [
				{
					"id":             "hook-a"
					"enabled":        true
					"trigger_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"event":          "synthetic"
					"timeout_ns":     10
				}
			]
			"surface_tools": []
			"requirements": [
				{
					"rung":       "W0"
					"applicable": true
					"reason":     "frozen applicability"
				},
				{
					"rung":       "W1"
					"applicable": false
					"reason":     "frozen applicability"
				},
				{
					"rung":       "W2"
					"applicable": true
					"reason":     "frozen applicability"
				},
				{
					"rung":       "W3"
					"applicable": true
					"reason":     "frozen applicability"
				},
				{
					"rung":       "W6"
					"applicable": true
					"reason":     "frozen applicability"
				},
			]
			"workload_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"budget": {
				"tenant_policy_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"bounds": [
					{
						"metric":  "hook_p95_ns"
						"maximum": 100
					},
					{
						"metric":  "index_age_ns"
						"maximum": 100
					},
					{
						"metric":  "payload_age_ns"
						"maximum": 100
					},
					{
						"metric":  "phys_footprint_bytes"
						"maximum": 100
					},
				]
			}
			"budget_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"source_digests": [
				{
					"id":     "delivery_parameters"
					"digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				}
			]
			"profile_digest":     "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"environment_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"frozen_at":          5
		}
		"started_at":                 10
		"finished_at":                20
		"decision_at":                21
		"agent_start_at":             30
		"startup_environment_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"startup_profile_digest":     "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"workload_digest":            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"budget_digest":              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"rungs": {
			"w0": {
				"verdict": "PASS"
				"reason":  "synthetic observation"
				"evidence_ids": [
					"b-revision"
				]
			}
			"w1": {
				"verdict": "NOT-APPLICABLE"
				"reason":  "synthetic observation"
				"evidence_ids": [
					"a-inventory"
				]
			}
			"w2": {
				"verdict": "PASS"
				"reason":  "synthetic observation"
				"evidence_ids": [
					"f-startup"
				]
			}
			"w3": {
				"verdict": "PASS"
				"reason":  "synthetic observation"
				"evidence_ids": [
					"g-trigger",
					"h-event",
					"i-beat",
				]
			}
			"w6": {
				"verdict": "PASS"
				"reason":  "synthetic observation"
				"evidence_ids": [
					"j-resource"
				]
			}
		}
		"served_product_commit":    "cccccccccccccccccccccccccccccccccccccccc"
		"installed_product_commit": "cccccccccccccccccccccccccccccccccccccccc"
		"index_commit":             "dddddddddddddddddddddddddddddddddddddddd"
		"primer_digest":            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"negotiated_tools": []
		"answered_probe_ids": []
		"hook_observations": [
			{
				"hook_id":             "hook-a"
				"trigger_evidence_id": "g-trigger"
				"event_evidence_id":   "h-event"
				"beat_evidence_id":    "i-beat"
			}
		]
		"absences": [
			{
				"channel": "descriptions"
				"evidence_ids": [
					"a-inventory"
				]
			},
			{
				"channel": "maps"
				"evidence_ids": [
					"a-inventory"
				]
			},
			{
				"channel": "readouts"
				"evidence_ids": [
					"a-inventory"
				]
			},
			{
				"channel": "skills"
				"evidence_ids": [
					"a-inventory"
				]
			},
			{
				"channel": "stanza"
				"evidence_ids": [
					"a-inventory"
				]
			},
			{
				"channel": "surface"
				"evidence_ids": [
					"a-inventory"
				]
			},
		]
		"samples": [
			{
				"metric": "hook_p95_ns"
				"value":  1
				"evidence_ids": [
					"j-resource"
				]
			},
			{
				"metric": "index_age_ns"
				"value":  1
				"evidence_ids": [
					"j-resource"
				]
			},
			{
				"metric": "payload_age_ns"
				"value":  1
				"evidence_ids": [
					"j-resource"
				]
			},
			{
				"metric": "phys_footprint_bytes"
				"value":  1
				"evidence_ids": [
					"j-resource"
				]
			},
		]
		"evidence": [
			{
				"id":          "a-inventory"
				"service":     "fixture-service"
				"source_kind": "channel_inventory"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "b-revision"
				"service":     "fixture-service"
				"source_kind": "revision"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "c-registration"
				"service":     "fixture-service"
				"source_kind": "registration"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "d-negotiation"
				"service":     "fixture-service"
				"source_kind": "negotiation"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "e-probe"
				"service":     "fixture-service"
				"source_kind": "call_result"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "f-startup"
				"service":     "fixture-service"
				"source_kind": "startup_delivery"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "g-trigger"
				"service":     "fixture-service"
				"source_kind": "hook_trigger"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "h-event"
				"service":     "fixture-service"
				"source_kind": "hook_event"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "i-beat"
				"service":     "fixture-service"
				"source_kind": "beat"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "j-resource"
				"service":     "fixture-service"
				"source_kind": "resource_sample"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "k-scripted"
				"service":     "fixture-service"
				"source_kind": "action"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
		]
	}
	"unknown_preflight": {
		"session_id": "session"
		"cell_id":    "cell"
		"arm":        "assigned"
		"phase":      "preflight"
		"profile": {
			"id":               "fixture"
			"expected_service": "fixture-service"
			"product_present":  true
			"lab":              "lab-fixture"
			"bench_harness":    "replay"
			"pins": {
				"harness":                    "codex-cli"
				"harness_pin":                "synthetic-1"
				"model_pin":                  "fixture-1"
				"transcript_contract_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"product_commit":             "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit":             "dddddddddddddddddddddddddddddddddddddddd"
				"plugin_digest":              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"stanza_sha":                 "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"description_sha":            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			}
			"environment": {
				"kind":        "local"
				"host_class":  "macos"
				"harness_pin": "synthetic-1"
				"permitted_tools": [
					"read"
				]
				"startup_instructions_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"action_capture_digest":       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			}
			"channels": [
				{
					"channel":        "descriptions"
					"enabled":        false
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "hooks"
					"enabled":        true
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "maps"
					"enabled":        false
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "prime"
					"enabled":        true
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "readouts"
					"enabled":        false
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "skills"
					"enabled":        false
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "stanza"
					"enabled":        false
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "surface"
					"enabled":        true
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
			]
			"hooks": [
				{
					"id":             "hook-a"
					"enabled":        true
					"trigger_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"event":          "synthetic"
					"timeout_ns":     10
				}
			]
			"surface_tools": [
				"tool-a"
			]
			"requirements": [
				{
					"rung":       "W0"
					"applicable": true
					"reason":     "frozen applicability"
				},
				{
					"rung":       "W1"
					"applicable": true
					"reason":     "frozen applicability"
				},
				{
					"rung":       "W2"
					"applicable": true
					"reason":     "frozen applicability"
				},
				{
					"rung":       "W3"
					"applicable": true
					"reason":     "frozen applicability"
				},
				{
					"rung":       "W6"
					"applicable": true
					"reason":     "frozen applicability"
				},
			]
			"workload_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"budget": {
				"tenant_policy_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"bounds": [
					{
						"metric":  "hook_p95_ns"
						"maximum": 100
					},
					{
						"metric":  "index_age_ns"
						"maximum": 100
					},
					{
						"metric":  "payload_age_ns"
						"maximum": 100
					},
					{
						"metric":  "phys_footprint_bytes"
						"maximum": 100
					},
				]
			}
			"budget_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"source_digests": [
				{
					"id":     "delivery_parameters"
					"digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				}
			]
			"profile_digest":     "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"environment_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"frozen_at":          5
		}
		"started_at":                 10
		"finished_at":                20
		"decision_at":                21
		"agent_start_at":             30
		"startup_environment_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"startup_profile_digest":     "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"workload_digest":            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"budget_digest":              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"rungs": {
			"w0": {
				"verdict": "PASS"
				"reason":  "synthetic observation"
				"evidence_ids": [
					"b-revision"
				]
			}
			"w1": {
				"verdict": "UNKNOWN"
				"reason":  "synthetic observation"
				"evidence_ids": [
					"c-registration",
					"d-negotiation",
					"e-probe",
				]
			}
			"w2": {
				"verdict": "PASS"
				"reason":  "synthetic observation"
				"evidence_ids": [
					"f-startup"
				]
			}
			"w3": {
				"verdict": "PASS"
				"reason":  "synthetic observation"
				"evidence_ids": [
					"g-trigger",
					"h-event",
					"i-beat",
				]
			}
			"w6": {
				"verdict": "UNKNOWN"
				"reason":  "synthetic observation"
				"evidence_ids": []
			}
		}
		"served_product_commit":    "cccccccccccccccccccccccccccccccccccccccc"
		"installed_product_commit": "cccccccccccccccccccccccccccccccccccccccc"
		"index_commit":             "dddddddddddddddddddddddddddddddddddddddd"
		"registration": {
			"service": "fixture-service"
			"surface_tools": [
				"tool-a"
			]
			"evidence_ids": [
				"c-registration"
			]
		}
		"primer_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"negotiated_tools": [
			"tool-a"
		]
		"answered_probe_ids": [
			"e-probe"
		]
		"hook_observations": [
			{
				"hook_id":             "hook-a"
				"trigger_evidence_id": "g-trigger"
				"event_evidence_id":   "h-event"
				"beat_evidence_id":    "i-beat"
			}
		]
		"absences": [
			{
				"channel": "descriptions"
				"evidence_ids": [
					"a-inventory"
				]
			},
			{
				"channel": "maps"
				"evidence_ids": [
					"a-inventory"
				]
			},
			{
				"channel": "readouts"
				"evidence_ids": [
					"a-inventory"
				]
			},
			{
				"channel": "skills"
				"evidence_ids": [
					"a-inventory"
				]
			},
			{
				"channel": "stanza"
				"evidence_ids": [
					"a-inventory"
				]
			},
		]
		"samples": []
		"evidence": [
			{
				"id":          "a-inventory"
				"service":     "fixture-service"
				"source_kind": "channel_inventory"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "b-revision"
				"service":     "fixture-service"
				"source_kind": "revision"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "c-registration"
				"service":     "fixture-service"
				"source_kind": "registration"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "d-negotiation"
				"service":     "fixture-service"
				"source_kind": "negotiation"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "e-probe"
				"service":     "fixture-service"
				"source_kind": "call_result"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "f-startup"
				"service":     "fixture-service"
				"source_kind": "startup_delivery"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "g-trigger"
				"service":     "fixture-service"
				"source_kind": "hook_trigger"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "h-event"
				"service":     "fixture-service"
				"source_kind": "hook_event"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "i-beat"
				"service":     "fixture-service"
				"source_kind": "beat"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "j-resource"
				"service":     "fixture-service"
				"source_kind": "resource_sample"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "k-scripted"
				"service":     "fixture-service"
				"source_kind": "action"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
		]
	}
	"failed_preflight": {
		"session_id": "session"
		"cell_id":    "cell"
		"arm":        "assigned"
		"phase":      "preflight"
		"profile": {
			"id":               "fixture"
			"expected_service": "fixture-service"
			"product_present":  true
			"lab":              "lab-fixture"
			"bench_harness":    "replay"
			"pins": {
				"harness":                    "codex-cli"
				"harness_pin":                "synthetic-1"
				"model_pin":                  "fixture-1"
				"transcript_contract_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"product_commit":             "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit":             "dddddddddddddddddddddddddddddddddddddddd"
				"plugin_digest":              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"stanza_sha":                 "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"description_sha":            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			}
			"environment": {
				"kind":        "local"
				"host_class":  "macos"
				"harness_pin": "synthetic-1"
				"permitted_tools": [
					"read"
				]
				"startup_instructions_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"action_capture_digest":       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			}
			"channels": [
				{
					"channel":        "descriptions"
					"enabled":        false
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "hooks"
					"enabled":        true
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "maps"
					"enabled":        false
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "prime"
					"enabled":        true
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "readouts"
					"enabled":        false
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "skills"
					"enabled":        false
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "stanza"
					"enabled":        false
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "surface"
					"enabled":        true
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
			]
			"hooks": [
				{
					"id":             "hook-a"
					"enabled":        true
					"trigger_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"event":          "synthetic"
					"timeout_ns":     10
				}
			]
			"surface_tools": [
				"tool-a"
			]
			"requirements": [
				{
					"rung":       "W0"
					"applicable": true
					"reason":     "frozen applicability"
				},
				{
					"rung":       "W1"
					"applicable": true
					"reason":     "frozen applicability"
				},
				{
					"rung":       "W2"
					"applicable": true
					"reason":     "frozen applicability"
				},
				{
					"rung":       "W3"
					"applicable": true
					"reason":     "frozen applicability"
				},
				{
					"rung":       "W6"
					"applicable": true
					"reason":     "frozen applicability"
				},
			]
			"workload_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"budget": {
				"tenant_policy_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"bounds": [
					{
						"metric":  "hook_p95_ns"
						"maximum": 100
					},
					{
						"metric":  "index_age_ns"
						"maximum": 100
					},
					{
						"metric":  "payload_age_ns"
						"maximum": 100
					},
					{
						"metric":  "phys_footprint_bytes"
						"maximum": 100
					},
				]
			}
			"budget_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"source_digests": [
				{
					"id":     "delivery_parameters"
					"digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				}
			]
			"profile_digest":     "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"environment_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"frozen_at":          5
		}
		"started_at":                 10
		"finished_at":                20
		"decision_at":                21
		"agent_start_at":             30
		"startup_environment_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"startup_profile_digest":     "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"workload_digest":            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"budget_digest":              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"rungs": {
			"w0": {
				"verdict": "PASS"
				"reason":  "synthetic observation"
				"evidence_ids": [
					"b-revision"
				]
			}
			"w1": {
				"verdict": "PASS"
				"reason":  "synthetic observation"
				"evidence_ids": [
					"c-registration",
					"d-negotiation",
					"e-probe",
				]
			}
			"w2": {
				"verdict": "PASS"
				"reason":  "synthetic observation"
				"evidence_ids": [
					"f-startup"
				]
			}
			"w3": {
				"verdict": "PASS"
				"reason":  "synthetic observation"
				"evidence_ids": [
					"g-trigger",
					"h-event",
					"i-beat",
				]
			}
			"w6": {
				"verdict": "FAIL"
				"reason":  "synthetic observation"
				"evidence_ids": [
					"j-resource"
				]
			}
		}
		"served_product_commit":    "cccccccccccccccccccccccccccccccccccccccc"
		"installed_product_commit": "cccccccccccccccccccccccccccccccccccccccc"
		"index_commit":             "dddddddddddddddddddddddddddddddddddddddd"
		"registration": {
			"service": "fixture-service"
			"surface_tools": [
				"tool-a"
			]
			"evidence_ids": [
				"c-registration"
			]
		}
		"primer_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"negotiated_tools": [
			"tool-a"
		]
		"answered_probe_ids": [
			"e-probe"
		]
		"hook_observations": [
			{
				"hook_id":             "hook-a"
				"trigger_evidence_id": "g-trigger"
				"event_evidence_id":   "h-event"
				"beat_evidence_id":    "i-beat"
			}
		]
		"absences": [
			{
				"channel": "descriptions"
				"evidence_ids": [
					"a-inventory"
				]
			},
			{
				"channel": "maps"
				"evidence_ids": [
					"a-inventory"
				]
			},
			{
				"channel": "readouts"
				"evidence_ids": [
					"a-inventory"
				]
			},
			{
				"channel": "skills"
				"evidence_ids": [
					"a-inventory"
				]
			},
			{
				"channel": "stanza"
				"evidence_ids": [
					"a-inventory"
				]
			},
		]
		"samples": [
			{
				"metric": "hook_p95_ns"
				"value":  101
				"evidence_ids": [
					"j-resource"
				]
			},
			{
				"metric": "index_age_ns"
				"value":  1
				"evidence_ids": [
					"j-resource"
				]
			},
			{
				"metric": "payload_age_ns"
				"value":  1
				"evidence_ids": [
					"j-resource"
				]
			},
			{
				"metric": "phys_footprint_bytes"
				"value":  1
				"evidence_ids": [
					"j-resource"
				]
			},
		]
		"evidence": [
			{
				"id":          "a-inventory"
				"service":     "fixture-service"
				"source_kind": "channel_inventory"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "b-revision"
				"service":     "fixture-service"
				"source_kind": "revision"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "c-registration"
				"service":     "fixture-service"
				"source_kind": "registration"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "d-negotiation"
				"service":     "fixture-service"
				"source_kind": "negotiation"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "e-probe"
				"service":     "fixture-service"
				"source_kind": "call_result"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "f-startup"
				"service":     "fixture-service"
				"source_kind": "startup_delivery"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "g-trigger"
				"service":     "fixture-service"
				"source_kind": "hook_trigger"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "h-event"
				"service":     "fixture-service"
				"source_kind": "hook_event"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "i-beat"
				"service":     "fixture-service"
				"source_kind": "beat"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "j-resource"
				"service":     "fixture-service"
				"source_kind": "resource_sample"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "k-scripted"
				"service":     "fixture-service"
				"source_kind": "action"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
		]
	}
	"wsl_preflight": {
		"session_id": "session"
		"cell_id":    "cell"
		"arm":        "assigned"
		"phase":      "preflight"
		"profile": {
			"id":               "fixture"
			"expected_service": "fixture-service"
			"product_present":  true
			"lab":              "lab-fixture"
			"bench_harness":    "replay"
			"pins": {
				"harness":                    "codex-cli"
				"harness_pin":                "synthetic-1"
				"model_pin":                  "fixture-1"
				"transcript_contract_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"product_commit":             "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit":             "dddddddddddddddddddddddddddddddddddddddd"
				"plugin_digest":              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"stanza_sha":                 "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"description_sha":            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			}
			"environment": {
				"kind":        "wsl"
				"host_class":  "windows"
				"harness_pin": "synthetic-1"
				"permitted_tools": [
					"read"
				]
				"startup_instructions_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"action_capture_digest":       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"guest_revision":              "guest-1"
				"path_translation": [
					{
						"guest_path":   "input/file"
						"fixture_path": "fixtures/file"
					}
				]
			}
			"channels": [
				{
					"channel":        "descriptions"
					"enabled":        false
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "hooks"
					"enabled":        true
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "maps"
					"enabled":        false
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "prime"
					"enabled":        true
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "readouts"
					"enabled":        false
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "skills"
					"enabled":        false
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "stanza"
					"enabled":        false
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
				{
					"channel":        "surface"
					"enabled":        true
					"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				},
			]
			"hooks": [
				{
					"id":             "hook-a"
					"enabled":        true
					"trigger_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"event":          "synthetic"
					"timeout_ns":     10
				}
			]
			"surface_tools": [
				"tool-a"
			]
			"requirements": [
				{
					"rung":       "W0"
					"applicable": true
					"reason":     "frozen applicability"
				},
				{
					"rung":       "W1"
					"applicable": true
					"reason":     "frozen applicability"
				},
				{
					"rung":       "W2"
					"applicable": true
					"reason":     "frozen applicability"
				},
				{
					"rung":       "W3"
					"applicable": true
					"reason":     "frozen applicability"
				},
				{
					"rung":       "W6"
					"applicable": true
					"reason":     "frozen applicability"
				},
			]
			"workload_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"budget": {
				"tenant_policy_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"bounds": [
					{
						"metric":  "hook_p95_ns"
						"maximum": 100
					},
					{
						"metric":  "index_age_ns"
						"maximum": 100
					},
					{
						"metric":  "payload_age_ns"
						"maximum": 100
					},
					{
						"metric":  "resident_bytes"
						"maximum": 100
					},
				]
			}
			"budget_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"source_digests": [
				{
					"id":     "delivery_parameters"
					"digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				}
			]
			"profile_digest":     "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"environment_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"frozen_at":          5
		}
		"started_at":                 10
		"finished_at":                20
		"decision_at":                21
		"agent_start_at":             30
		"startup_environment_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"startup_profile_digest":     "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"workload_digest":            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"budget_digest":              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"rungs": {
			"w0": {
				"verdict": "PASS"
				"reason":  "synthetic observation"
				"evidence_ids": [
					"b-revision"
				]
			}
			"w1": {
				"verdict": "PASS"
				"reason":  "synthetic observation"
				"evidence_ids": [
					"c-registration",
					"d-negotiation",
					"e-probe",
				]
			}
			"w2": {
				"verdict": "PASS"
				"reason":  "synthetic observation"
				"evidence_ids": [
					"f-startup"
				]
			}
			"w3": {
				"verdict": "PASS"
				"reason":  "synthetic observation"
				"evidence_ids": [
					"g-trigger",
					"h-event",
					"i-beat",
				]
			}
			"w6": {
				"verdict": "PASS"
				"reason":  "synthetic observation"
				"evidence_ids": [
					"j-resource"
				]
			}
		}
		"served_product_commit":    "cccccccccccccccccccccccccccccccccccccccc"
		"installed_product_commit": "cccccccccccccccccccccccccccccccccccccccc"
		"index_commit":             "dddddddddddddddddddddddddddddddddddddddd"
		"registration": {
			"service": "fixture-service"
			"surface_tools": [
				"tool-a"
			]
			"evidence_ids": [
				"c-registration"
			]
		}
		"primer_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"negotiated_tools": [
			"tool-a"
		]
		"answered_probe_ids": [
			"e-probe"
		]
		"hook_observations": [
			{
				"hook_id":             "hook-a"
				"trigger_evidence_id": "g-trigger"
				"event_evidence_id":   "h-event"
				"beat_evidence_id":    "i-beat"
			}
		]
		"absences": [
			{
				"channel": "descriptions"
				"evidence_ids": [
					"a-inventory"
				]
			},
			{
				"channel": "maps"
				"evidence_ids": [
					"a-inventory"
				]
			},
			{
				"channel": "readouts"
				"evidence_ids": [
					"a-inventory"
				]
			},
			{
				"channel": "skills"
				"evidence_ids": [
					"a-inventory"
				]
			},
			{
				"channel": "stanza"
				"evidence_ids": [
					"a-inventory"
				]
			},
		]
		"samples": [
			{
				"metric": "hook_p95_ns"
				"value":  1
				"evidence_ids": [
					"j-resource"
				]
			},
			{
				"metric": "index_age_ns"
				"value":  1
				"evidence_ids": [
					"j-resource"
				]
			},
			{
				"metric": "payload_age_ns"
				"value":  1
				"evidence_ids": [
					"j-resource"
				]
			},
			{
				"metric": "resident_bytes"
				"value":  1
				"evidence_ids": [
					"j-resource"
				]
			},
		]
		"evidence": [
			{
				"id":          "a-inventory"
				"service":     "fixture-service"
				"source_kind": "channel_inventory"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "b-revision"
				"service":     "fixture-service"
				"source_kind": "revision"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "c-registration"
				"service":     "fixture-service"
				"source_kind": "registration"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "d-negotiation"
				"service":     "fixture-service"
				"source_kind": "negotiation"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "e-probe"
				"service":     "fixture-service"
				"source_kind": "call_result"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "f-startup"
				"service":     "fixture-service"
				"source_kind": "startup_delivery"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "g-trigger"
				"service":     "fixture-service"
				"source_kind": "hook_trigger"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "h-event"
				"service":     "fixture-service"
				"source_kind": "hook_event"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "i-beat"
				"service":     "fixture-service"
				"source_kind": "beat"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "j-resource"
				"service":     "fixture-service"
				"source_kind": "resource_sample"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "k-scripted"
				"service":     "fixture-service"
				"source_kind": "action"
				"phase":       "preflight"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    12
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
		]
	}
	"acted_at_k": {
		"session_id": "session"
		"cell_id":    "cell"
		"arm":        "assigned"
		"phase":      "agent"
		"pins": {
			"harness":                    "codex-cli"
			"harness_pin":                "synthetic-1"
			"model_pin":                  "fixture-1"
			"transcript_contract_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"product_commit":             "cccccccccccccccccccccccccccccccccccccccc"
			"subject_commit":             "dddddddddddddddddddddddddddddddddddddddd"
			"plugin_digest":              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"stanza_sha":                 "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"description_sha":            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}
		"profile_digest":             "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"environment_digest":         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"agent_start_at":             30
		"delivery_parameters_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"k":                          3
		"k_frozen_at":                5
		"capabilities": [
			{
				"capability": "capability-a"
				"eligible": {
					"state": "known"
					"count": 2
				}
				"successful_calls": {
					"state": "known"
					"count": 1
				}
				"acted": {
					"state": "known"
					"count": 2
				}
				"should_not_calls": {
					"state": "known"
					"count": 0
				}
				"unknown": {
					"state": "known"
					"count": 0
				}
				"denominator": {
					"eligible_opportunities": {
						"state": "known"
						"count": 2
					}
					"delivered_targets": {
						"state": "known"
						"count": 2
					}
					"eligibility": {
						"state":  "validated"
						"reason": "synthetic provenance"
						"evidence_ids": [
							"a-eligibility"
						]
						"set_digest":                "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"classifier_digest":         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"validation_reading_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					}
					"delivery": {
						"state":  "validated"
						"reason": "synthetic provenance"
						"evidence_ids": [
							"b-delivery"
						]
						"set_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					}
				}
				"action_capture": {
					"state":  "validated"
					"reason": "synthetic provenance"
					"evidence_ids": [
						"c-capture"
					]
					"set_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				}
				"w4": {
					"state":       "measured"
					"numerator":   1
					"denominator": 2
				}
				"w5": {
					"state":       "measured"
					"numerator":   2
					"denominator": 2
				}
				"bars": {
					"use_minimum":   0.5
					"acted_minimum": 0.5
				}
			}
		]
		"evidence": [
			{
				"id":          "a-eligibility"
				"service":     "fixture-service"
				"source_kind": "eligibility"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    31
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "b-delivery"
				"service":     "fixture-service"
				"source_kind": "call_result"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    32
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "c-capture"
				"service":     "fixture-service"
				"source_kind": "action_capture"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    34
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
				"capture": {
					"capability": "capability-a"
					"complete":   true
					"action_evidence_ids": [
						"d-action"
					]
				}
			},
			{
				"id":          "d-action"
				"service":     "fixture-service"
				"source_kind": "action"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    33
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
		]
	}
	"zero_opportunities": {
		"session_id": "session"
		"cell_id":    "cell"
		"arm":        "assigned"
		"phase":      "agent"
		"pins": {
			"harness":                    "codex-cli"
			"harness_pin":                "synthetic-1"
			"model_pin":                  "fixture-1"
			"transcript_contract_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"product_commit":             "cccccccccccccccccccccccccccccccccccccccc"
			"subject_commit":             "dddddddddddddddddddddddddddddddddddddddd"
			"plugin_digest":              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"stanza_sha":                 "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"description_sha":            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}
		"profile_digest":             "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"environment_digest":         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"agent_start_at":             30
		"delivery_parameters_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"k":                          3
		"k_frozen_at":                5
		"capabilities": [
			{
				"capability": "capability-a"
				"eligible": {
					"state": "known"
					"count": 0
				}
				"successful_calls": {
					"state": "known"
					"count": 0
				}
				"acted": {
					"state": "known"
					"count": 0
				}
				"should_not_calls": {
					"state": "known"
					"count": 0
				}
				"unknown": {
					"state": "known"
					"count": 0
				}
				"denominator": {
					"eligible_opportunities": {
						"state": "known"
						"count": 0
					}
					"delivered_targets": {
						"state": "known"
						"count": 0
					}
					"eligibility": {
						"state":  "validated"
						"reason": "synthetic provenance"
						"evidence_ids": [
							"a-eligibility"
						]
						"set_digest":                "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"classifier_digest":         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"validation_reading_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					}
					"delivery": {
						"state":  "validated"
						"reason": "synthetic provenance"
						"evidence_ids": [
							"b-delivery"
						]
						"set_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					}
				}
				"action_capture": {
					"state":  "validated"
					"reason": "synthetic provenance"
					"evidence_ids": [
						"c-capture"
					]
					"set_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				}
				"w4": {
					"state": "not_measured"
					"denominator": {
						"state": "known"
						"count": 0
					}
					"cause":  "zero_denominator"
					"reason": "synthetic boundary"
				}
				"w5": {
					"state": "not_measured"
					"denominator": {
						"state": "known"
						"count": 0
					}
					"cause":  "zero_denominator"
					"reason": "synthetic boundary"
				}
				"bars": {
					"use_minimum":   0.5
					"acted_minimum": 0.5
				}
			}
		]
		"evidence": [
			{
				"id":          "a-eligibility"
				"service":     "fixture-service"
				"source_kind": "eligibility"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    31
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "b-delivery"
				"service":     "fixture-service"
				"source_kind": "call_result"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    32
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "c-capture"
				"service":     "fixture-service"
				"source_kind": "action_capture"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    34
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
				"capture": {
					"capability": "capability-a"
					"complete":   true
					"action_evidence_ids": [
						"d-action"
					]
				}
			},
			{
				"id":          "d-action"
				"service":     "fixture-service"
				"source_kind": "action"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    33
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
		]
	}
	"zero_calls": {
		"session_id": "session"
		"cell_id":    "cell"
		"arm":        "assigned"
		"phase":      "agent"
		"pins": {
			"harness":                    "codex-cli"
			"harness_pin":                "synthetic-1"
			"model_pin":                  "fixture-1"
			"transcript_contract_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"product_commit":             "cccccccccccccccccccccccccccccccccccccccc"
			"subject_commit":             "dddddddddddddddddddddddddddddddddddddddd"
			"plugin_digest":              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"stanza_sha":                 "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"description_sha":            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}
		"profile_digest":             "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"environment_digest":         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"agent_start_at":             30
		"delivery_parameters_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"k":                          3
		"k_frozen_at":                5
		"capabilities": [
			{
				"capability": "capability-a"
				"eligible": {
					"state": "known"
					"count": 2
				}
				"successful_calls": {
					"state": "known"
					"count": 0
				}
				"acted": {
					"state": "known"
					"count": 2
				}
				"should_not_calls": {
					"state": "known"
					"count": 0
				}
				"unknown": {
					"state": "known"
					"count": 0
				}
				"denominator": {
					"eligible_opportunities": {
						"state": "known"
						"count": 2
					}
					"delivered_targets": {
						"state": "known"
						"count": 2
					}
					"eligibility": {
						"state":  "validated"
						"reason": "synthetic provenance"
						"evidence_ids": [
							"a-eligibility"
						]
						"set_digest":                "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"classifier_digest":         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"validation_reading_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					}
					"delivery": {
						"state":  "validated"
						"reason": "synthetic provenance"
						"evidence_ids": [
							"b-delivery"
						]
						"set_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					}
				}
				"action_capture": {
					"state":  "validated"
					"reason": "synthetic provenance"
					"evidence_ids": [
						"c-capture"
					]
					"set_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				}
				"w4": {
					"state":       "measured"
					"numerator":   0
					"denominator": 2
				}
				"w5": {
					"state":       "measured"
					"numerator":   2
					"denominator": 2
				}
				"bars": {
					"use_minimum":   0.5
					"acted_minimum": 0.5
				}
			}
		]
		"evidence": [
			{
				"id":          "a-eligibility"
				"service":     "fixture-service"
				"source_kind": "eligibility"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    31
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "b-delivery"
				"service":     "fixture-service"
				"source_kind": "call_result"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    32
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "c-capture"
				"service":     "fixture-service"
				"source_kind": "action_capture"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    34
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
				"capture": {
					"capability": "capability-a"
					"complete":   true
					"action_evidence_ids": [
						"d-action"
					]
				}
			},
			{
				"id":          "d-action"
				"service":     "fixture-service"
				"source_kind": "action"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    33
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
		]
	}
	"zero_actions": {
		"session_id": "session"
		"cell_id":    "cell"
		"arm":        "assigned"
		"phase":      "agent"
		"pins": {
			"harness":                    "codex-cli"
			"harness_pin":                "synthetic-1"
			"model_pin":                  "fixture-1"
			"transcript_contract_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"product_commit":             "cccccccccccccccccccccccccccccccccccccccc"
			"subject_commit":             "dddddddddddddddddddddddddddddddddddddddd"
			"plugin_digest":              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"stanza_sha":                 "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"description_sha":            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}
		"profile_digest":             "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"environment_digest":         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"agent_start_at":             30
		"delivery_parameters_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"k":                          3
		"k_frozen_at":                5
		"capabilities": [
			{
				"capability": "capability-a"
				"eligible": {
					"state": "known"
					"count": 2
				}
				"successful_calls": {
					"state": "known"
					"count": 1
				}
				"acted": {
					"state": "known"
					"count": 0
				}
				"should_not_calls": {
					"state": "known"
					"count": 0
				}
				"unknown": {
					"state": "known"
					"count": 0
				}
				"denominator": {
					"eligible_opportunities": {
						"state": "known"
						"count": 2
					}
					"delivered_targets": {
						"state": "known"
						"count": 2
					}
					"eligibility": {
						"state":  "validated"
						"reason": "synthetic provenance"
						"evidence_ids": [
							"a-eligibility"
						]
						"set_digest":                "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"classifier_digest":         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"validation_reading_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					}
					"delivery": {
						"state":  "validated"
						"reason": "synthetic provenance"
						"evidence_ids": [
							"b-delivery"
						]
						"set_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					}
				}
				"action_capture": {
					"state":  "validated"
					"reason": "synthetic provenance"
					"evidence_ids": [
						"c-capture"
					]
					"set_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				}
				"w4": {
					"state":       "measured"
					"numerator":   1
					"denominator": 2
				}
				"w5": {
					"state":       "measured"
					"numerator":   0
					"denominator": 2
				}
				"bars": {
					"use_minimum":   0.5
					"acted_minimum": 0.5
				}
			}
		]
		"evidence": [
			{
				"id":          "a-eligibility"
				"service":     "fixture-service"
				"source_kind": "eligibility"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    31
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "b-delivery"
				"service":     "fixture-service"
				"source_kind": "call_result"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    32
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "c-capture"
				"service":     "fixture-service"
				"source_kind": "action_capture"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    34
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
				"capture": {
					"capability": "capability-a"
					"complete":   true
					"action_evidence_ids": [
						"d-action"
					]
				}
			},
			{
				"id":          "d-action"
				"service":     "fixture-service"
				"source_kind": "action"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    33
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
		]
	}
	"two_deliveries_no_capture": {
		"session_id": "session"
		"cell_id":    "cell"
		"arm":        "assigned"
		"phase":      "agent"
		"pins": {
			"harness":                    "codex-cli"
			"harness_pin":                "synthetic-1"
			"model_pin":                  "fixture-1"
			"transcript_contract_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"product_commit":             "cccccccccccccccccccccccccccccccccccccccc"
			"subject_commit":             "dddddddddddddddddddddddddddddddddddddddd"
			"plugin_digest":              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"stanza_sha":                 "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"description_sha":            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}
		"profile_digest":             "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"environment_digest":         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"agent_start_at":             30
		"delivery_parameters_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"k":                          3
		"k_frozen_at":                5
		"capabilities": [
			{
				"capability": "capability-a"
				"eligible": {
					"state": "known"
					"count": 2
				}
				"successful_calls": {
					"state": "known"
					"count": 1
				}
				"acted": {
					"state":  "unavailable"
					"reason": "not captured"
				}
				"should_not_calls": {
					"state": "known"
					"count": 0
				}
				"unknown": {
					"state": "known"
					"count": 0
				}
				"denominator": {
					"eligible_opportunities": {
						"state": "known"
						"count": 2
					}
					"delivered_targets": {
						"state": "known"
						"count": 2
					}
					"eligibility": {
						"state":  "validated"
						"reason": "synthetic provenance"
						"evidence_ids": [
							"a-eligibility"
						]
						"set_digest":                "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"classifier_digest":         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"validation_reading_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					}
					"delivery": {
						"state":  "validated"
						"reason": "synthetic provenance"
						"evidence_ids": [
							"b-delivery"
						]
						"set_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					}
				}
				"action_capture": {
					"state":  "unavailable"
					"reason": "not captured"
					"evidence_ids": []
				}
				"w4": {
					"state":       "measured"
					"numerator":   1
					"denominator": 2
				}
				"w5": {
					"state": "not_measured"
					"denominator": {
						"state": "known"
						"count": 2
					}
					"cause":  "unavailable"
					"reason": "synthetic boundary"
				}
				"bars": {
					"use_minimum":   0.5
					"acted_minimum": 0.5
				}
			}
		]
		"evidence": [
			{
				"id":          "a-eligibility"
				"service":     "fixture-service"
				"source_kind": "eligibility"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    31
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "b-delivery"
				"service":     "fixture-service"
				"source_kind": "call_result"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    32
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
		]
	}
	"two_deliveries_complete_no_actions": {
		"session_id": "session"
		"cell_id":    "cell"
		"arm":        "assigned"
		"phase":      "agent"
		"pins": {
			"harness":                    "codex-cli"
			"harness_pin":                "synthetic-1"
			"model_pin":                  "fixture-1"
			"transcript_contract_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"product_commit":             "cccccccccccccccccccccccccccccccccccccccc"
			"subject_commit":             "dddddddddddddddddddddddddddddddddddddddd"
			"plugin_digest":              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"stanza_sha":                 "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"description_sha":            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}
		"profile_digest":             "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"environment_digest":         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"agent_start_at":             30
		"delivery_parameters_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"k":                          3
		"k_frozen_at":                5
		"capabilities": [
			{
				"capability": "capability-a"
				"eligible": {
					"state": "known"
					"count": 2
				}
				"successful_calls": {
					"state": "known"
					"count": 1
				}
				"acted": {
					"state": "known"
					"count": 0
				}
				"should_not_calls": {
					"state": "known"
					"count": 0
				}
				"unknown": {
					"state": "known"
					"count": 0
				}
				"denominator": {
					"eligible_opportunities": {
						"state": "known"
						"count": 2
					}
					"delivered_targets": {
						"state": "known"
						"count": 2
					}
					"eligibility": {
						"state":  "validated"
						"reason": "synthetic provenance"
						"evidence_ids": [
							"a-eligibility"
						]
						"set_digest":                "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"classifier_digest":         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"validation_reading_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					}
					"delivery": {
						"state":  "validated"
						"reason": "synthetic provenance"
						"evidence_ids": [
							"b-delivery"
						]
						"set_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					}
				}
				"action_capture": {
					"state":  "validated"
					"reason": "synthetic provenance"
					"evidence_ids": [
						"c-capture"
					]
					"set_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				}
				"w4": {
					"state":       "measured"
					"numerator":   1
					"denominator": 2
				}
				"w5": {
					"state":       "measured"
					"numerator":   0
					"denominator": 2
				}
				"bars": {
					"use_minimum":   0.5
					"acted_minimum": 0.5
				}
			}
		]
		"evidence": [
			{
				"id":          "a-eligibility"
				"service":     "fixture-service"
				"source_kind": "eligibility"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    31
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "b-delivery"
				"service":     "fixture-service"
				"source_kind": "call_result"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    32
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "c-capture"
				"service":     "fixture-service"
				"source_kind": "action_capture"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    34
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
				"capture": {
					"capability": "capability-a"
					"complete":   true
					"action_evidence_ids": []
				}
			},
		]
	}
	"classifier_counts_only": {
		"session_id": "session"
		"cell_id":    "cell"
		"arm":        "assigned"
		"phase":      "agent"
		"pins": {
			"harness":                    "codex-cli"
			"harness_pin":                "synthetic-1"
			"model_pin":                  "fixture-1"
			"transcript_contract_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"product_commit":             "cccccccccccccccccccccccccccccccccccccccc"
			"subject_commit":             "dddddddddddddddddddddddddddddddddddddddd"
			"plugin_digest":              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"stanza_sha":                 "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"description_sha":            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}
		"profile_digest":             "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"environment_digest":         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"agent_start_at":             30
		"delivery_parameters_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"k":                          3
		"k_frozen_at":                5
		"capabilities": [
			{
				"capability": "capability-a"
				"eligible": {
					"state": "known"
					"count": 2
				}
				"successful_calls": {
					"state": "known"
					"count": 1
				}
				"acted": {
					"state": "known"
					"count": 2
				}
				"should_not_calls": {
					"state": "known"
					"count": 0
				}
				"unknown": {
					"state": "known"
					"count": 0
				}
				"denominator": {
					"eligible_opportunities": {
						"state": "known"
						"count": 2
					}
					"delivered_targets": {
						"state": "known"
						"count": 2
					}
					"eligibility": {
						"state":  "unvalidated"
						"reason": "not validated"
						"evidence_ids": []
					}
					"delivery": {
						"state":  "validated"
						"reason": "synthetic provenance"
						"evidence_ids": [
							"b-delivery"
						]
						"set_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					}
				}
				"action_capture": {
					"state":  "validated"
					"reason": "synthetic provenance"
					"evidence_ids": [
						"c-capture"
					]
					"set_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				}
				"w4": {
					"state": "not_measured"
					"denominator": {
						"state": "known"
						"count": 2
					}
					"cause":  "unvalidated"
					"reason": "synthetic boundary"
				}
				"w5": {
					"state":       "measured"
					"numerator":   2
					"denominator": 2
				}
				"bars": {
					"use_minimum":   0.5
					"acted_minimum": 0.5
				}
			}
		]
		"evidence": [
			{
				"id":          "a-eligibility"
				"service":     "fixture-service"
				"source_kind": "eligibility"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    31
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "b-delivery"
				"service":     "fixture-service"
				"source_kind": "call_result"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    32
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "c-capture"
				"service":     "fixture-service"
				"source_kind": "action_capture"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    34
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
				"capture": {
					"capability": "capability-a"
					"complete":   true
					"action_evidence_ids": [
						"d-action"
					]
				}
			},
			{
				"id":          "d-action"
				"service":     "fixture-service"
				"source_kind": "action"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    33
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
		]
	}
	"classifier_failed_validation": {
		"session_id": "session"
		"cell_id":    "cell"
		"arm":        "assigned"
		"phase":      "agent"
		"pins": {
			"harness":                    "codex-cli"
			"harness_pin":                "synthetic-1"
			"model_pin":                  "fixture-1"
			"transcript_contract_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"product_commit":             "cccccccccccccccccccccccccccccccccccccccc"
			"subject_commit":             "dddddddddddddddddddddddddddddddddddddddd"
			"plugin_digest":              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"stanza_sha":                 "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"description_sha":            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}
		"profile_digest":             "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"environment_digest":         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"agent_start_at":             30
		"delivery_parameters_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"k":                          3
		"k_frozen_at":                5
		"capabilities": [
			{
				"capability": "capability-a"
				"eligible": {
					"state": "known"
					"count": 2
				}
				"successful_calls": {
					"state": "known"
					"count": 1
				}
				"acted": {
					"state": "known"
					"count": 2
				}
				"should_not_calls": {
					"state": "known"
					"count": 0
				}
				"unknown": {
					"state": "known"
					"count": 0
				}
				"denominator": {
					"eligible_opportunities": {
						"state": "known"
						"count": 2
					}
					"delivered_targets": {
						"state": "known"
						"count": 2
					}
					"eligibility": {
						"state":  "unvalidated"
						"reason": "synthetic provenance"
						"evidence_ids": [
							"a-eligibility"
						]
						"set_digest":                "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"classifier_digest":         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"validation_reading_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					}
					"delivery": {
						"state":  "validated"
						"reason": "synthetic provenance"
						"evidence_ids": [
							"b-delivery"
						]
						"set_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					}
				}
				"action_capture": {
					"state":  "validated"
					"reason": "synthetic provenance"
					"evidence_ids": [
						"c-capture"
					]
					"set_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				}
				"w4": {
					"state": "not_measured"
					"denominator": {
						"state": "known"
						"count": 2
					}
					"cause":  "unvalidated"
					"reason": "synthetic boundary"
				}
				"w5": {
					"state":       "measured"
					"numerator":   2
					"denominator": 2
				}
				"bars": {
					"use_minimum":   0.5
					"acted_minimum": 0.5
				}
			}
		]
		"evidence": [
			{
				"id":          "a-eligibility"
				"service":     "fixture-service"
				"source_kind": "eligibility"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    31
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "b-delivery"
				"service":     "fixture-service"
				"source_kind": "call_result"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    32
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "c-capture"
				"service":     "fixture-service"
				"source_kind": "action_capture"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    34
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
				"capture": {
					"capability": "capability-a"
					"complete":   true
					"action_evidence_ids": [
						"d-action"
					]
				}
			},
			{
				"id":          "d-action"
				"service":     "fixture-service"
				"source_kind": "action"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    33
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
		]
	}
	"unknown_denominator": {
		"session_id": "session"
		"cell_id":    "cell"
		"arm":        "assigned"
		"phase":      "agent"
		"pins": {
			"harness":                    "codex-cli"
			"harness_pin":                "synthetic-1"
			"model_pin":                  "fixture-1"
			"transcript_contract_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"product_commit":             "cccccccccccccccccccccccccccccccccccccccc"
			"subject_commit":             "dddddddddddddddddddddddddddddddddddddddd"
			"plugin_digest":              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"stanza_sha":                 "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"description_sha":            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}
		"profile_digest":             "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"environment_digest":         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"agent_start_at":             30
		"delivery_parameters_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"k":                          3
		"k_frozen_at":                5
		"capabilities": [
			{
				"capability": "capability-a"
				"eligible": {
					"state":  "unavailable"
					"reason": "not captured"
				}
				"successful_calls": {
					"state": "known"
					"count": 1
				}
				"acted": {
					"state": "known"
					"count": 2
				}
				"should_not_calls": {
					"state": "known"
					"count": 0
				}
				"unknown": {
					"state": "known"
					"count": 0
				}
				"denominator": {
					"eligible_opportunities": {
						"state":  "unavailable"
						"reason": "not captured"
					}
					"delivered_targets": {
						"state": "known"
						"count": 2
					}
					"eligibility": {
						"state":  "validated"
						"reason": "synthetic provenance"
						"evidence_ids": [
							"a-eligibility"
						]
						"set_digest":                "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"classifier_digest":         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"validation_reading_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					}
					"delivery": {
						"state":  "validated"
						"reason": "synthetic provenance"
						"evidence_ids": [
							"b-delivery"
						]
						"set_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					}
				}
				"action_capture": {
					"state":  "validated"
					"reason": "synthetic provenance"
					"evidence_ids": [
						"c-capture"
					]
					"set_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				}
				"w4": {
					"state": "not_measured"
					"denominator": {
						"state":  "unavailable"
						"reason": "not captured"
					}
					"cause":  "unavailable"
					"reason": "synthetic boundary"
				}
				"w5": {
					"state":       "measured"
					"numerator":   2
					"denominator": 2
				}
				"bars": {
					"use_minimum":   0.5
					"acted_minimum": 0.5
				}
			}
		]
		"evidence": [
			{
				"id":          "a-eligibility"
				"service":     "fixture-service"
				"source_kind": "eligibility"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    31
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "b-delivery"
				"service":     "fixture-service"
				"source_kind": "call_result"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    32
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "c-capture"
				"service":     "fixture-service"
				"source_kind": "action_capture"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    34
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
				"capture": {
					"capability": "capability-a"
					"complete":   true
					"action_evidence_ids": [
						"d-action"
					]
				}
			},
			{
				"id":          "d-action"
				"service":     "fixture-service"
				"source_kind": "action"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    33
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
		]
	}
	"one_third": {
		"session_id": "session"
		"cell_id":    "cell"
		"arm":        "assigned"
		"phase":      "agent"
		"pins": {
			"harness":                    "codex-cli"
			"harness_pin":                "synthetic-1"
			"model_pin":                  "fixture-1"
			"transcript_contract_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"product_commit":             "cccccccccccccccccccccccccccccccccccccccc"
			"subject_commit":             "dddddddddddddddddddddddddddddddddddddddd"
			"plugin_digest":              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"stanza_sha":                 "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"description_sha":            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}
		"profile_digest":             "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"environment_digest":         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"agent_start_at":             30
		"delivery_parameters_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"k":                          3
		"k_frozen_at":                5
		"capabilities": [
			{
				"capability": "capability-a"
				"eligible": {
					"state": "known"
					"count": 3
				}
				"successful_calls": {
					"state": "known"
					"count": 1
				}
				"acted": {
					"state": "known"
					"count": 2
				}
				"should_not_calls": {
					"state": "known"
					"count": 0
				}
				"unknown": {
					"state": "known"
					"count": 0
				}
				"denominator": {
					"eligible_opportunities": {
						"state": "known"
						"count": 3
					}
					"delivered_targets": {
						"state": "known"
						"count": 2
					}
					"eligibility": {
						"state":  "validated"
						"reason": "synthetic provenance"
						"evidence_ids": [
							"a-eligibility"
						]
						"set_digest":                "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"classifier_digest":         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"validation_reading_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					}
					"delivery": {
						"state":  "validated"
						"reason": "synthetic provenance"
						"evidence_ids": [
							"b-delivery"
						]
						"set_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					}
				}
				"action_capture": {
					"state":  "validated"
					"reason": "synthetic provenance"
					"evidence_ids": [
						"c-capture"
					]
					"set_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				}
				"w4": {
					"state":       "measured"
					"numerator":   1
					"denominator": 3
				}
				"w5": {
					"state":       "measured"
					"numerator":   2
					"denominator": 2
				}
				"bars": {
					"use_minimum":   0.5
					"acted_minimum": 0.5
				}
			}
		]
		"evidence": [
			{
				"id":          "a-eligibility"
				"service":     "fixture-service"
				"source_kind": "eligibility"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    31
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "b-delivery"
				"service":     "fixture-service"
				"source_kind": "call_result"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    32
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "c-capture"
				"service":     "fixture-service"
				"source_kind": "action_capture"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    34
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
				"capture": {
					"capability": "capability-a"
					"complete":   true
					"action_evidence_ids": [
						"d-action"
					]
				}
			},
			{
				"id":          "d-action"
				"service":     "fixture-service"
				"source_kind": "action"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    33
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
		]
	}
	"two_thirds": {
		"session_id": "session"
		"cell_id":    "cell"
		"arm":        "assigned"
		"phase":      "agent"
		"pins": {
			"harness":                    "codex-cli"
			"harness_pin":                "synthetic-1"
			"model_pin":                  "fixture-1"
			"transcript_contract_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"product_commit":             "cccccccccccccccccccccccccccccccccccccccc"
			"subject_commit":             "dddddddddddddddddddddddddddddddddddddddd"
			"plugin_digest":              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"stanza_sha":                 "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"description_sha":            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}
		"profile_digest":             "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"environment_digest":         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"agent_start_at":             30
		"delivery_parameters_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"k":                          3
		"k_frozen_at":                5
		"capabilities": [
			{
				"capability": "capability-a"
				"eligible": {
					"state": "known"
					"count": 3
				}
				"successful_calls": {
					"state": "known"
					"count": 2
				}
				"acted": {
					"state": "known"
					"count": 2
				}
				"should_not_calls": {
					"state": "known"
					"count": 0
				}
				"unknown": {
					"state": "known"
					"count": 0
				}
				"denominator": {
					"eligible_opportunities": {
						"state": "known"
						"count": 3
					}
					"delivered_targets": {
						"state": "known"
						"count": 2
					}
					"eligibility": {
						"state":  "validated"
						"reason": "synthetic provenance"
						"evidence_ids": [
							"a-eligibility"
						]
						"set_digest":                "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"classifier_digest":         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"validation_reading_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					}
					"delivery": {
						"state":  "validated"
						"reason": "synthetic provenance"
						"evidence_ids": [
							"b-delivery"
						]
						"set_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					}
				}
				"action_capture": {
					"state":  "validated"
					"reason": "synthetic provenance"
					"evidence_ids": [
						"c-capture"
					]
					"set_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				}
				"w4": {
					"state":       "measured"
					"numerator":   2
					"denominator": 3
				}
				"w5": {
					"state":       "measured"
					"numerator":   2
					"denominator": 2
				}
				"bars": {
					"use_minimum":   0.5
					"acted_minimum": 0.5
				}
			}
		]
		"evidence": [
			{
				"id":          "a-eligibility"
				"service":     "fixture-service"
				"source_kind": "eligibility"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    31
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "b-delivery"
				"service":     "fixture-service"
				"source_kind": "call_result"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    32
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
			{
				"id":          "c-capture"
				"service":     "fixture-service"
				"source_kind": "action_capture"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    34
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
				"capture": {
					"capability": "capability-a"
					"complete":   true
					"action_evidence_ids": [
						"d-action"
					]
				}
			},
			{
				"id":          "d-action"
				"service":     "fixture-service"
				"source_kind": "action"
				"phase":       "agent"
				"session_id":  "session"
				"call_id":     "operation"
				"range": {
					"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"start":           0
					"end":             1
				}
				"observed_at":    33
				"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
				"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"harness_pin":    "synthetic-1"
				"reason":         "synthetic evidence"
			},
		]
	}
	"preflight_only": {
		"preflight": {
			"session_id": "session"
			"cell_id":    "cell"
			"arm":        "assigned"
			"phase":      "preflight"
			"profile": {
				"id":               "fixture"
				"expected_service": "fixture-service"
				"product_present":  true
				"lab":              "lab-fixture"
				"bench_harness":    "replay"
				"pins": {
					"harness":                    "codex-cli"
					"harness_pin":                "synthetic-1"
					"model_pin":                  "fixture-1"
					"transcript_contract_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"product_commit":             "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit":             "dddddddddddddddddddddddddddddddddddddddd"
					"plugin_digest":              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"stanza_sha":                 "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"description_sha":            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				}
				"environment": {
					"kind":        "local"
					"host_class":  "macos"
					"harness_pin": "synthetic-1"
					"permitted_tools": [
						"read"
					]
					"startup_instructions_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"action_capture_digest":       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				}
				"channels": [
					{
						"channel":        "descriptions"
						"enabled":        false
						"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					},
					{
						"channel":        "hooks"
						"enabled":        true
						"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					},
					{
						"channel":        "maps"
						"enabled":        false
						"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					},
					{
						"channel":        "prime"
						"enabled":        true
						"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					},
					{
						"channel":        "readouts"
						"enabled":        false
						"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					},
					{
						"channel":        "skills"
						"enabled":        false
						"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					},
					{
						"channel":        "stanza"
						"enabled":        false
						"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					},
					{
						"channel":        "surface"
						"enabled":        true
						"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					},
				]
				"hooks": [
					{
						"id":             "hook-a"
						"enabled":        true
						"trigger_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"event":          "synthetic"
						"timeout_ns":     10
					}
				]
				"surface_tools": [
					"tool-a"
				]
				"requirements": [
					{
						"rung":       "W0"
						"applicable": true
						"reason":     "frozen applicability"
					},
					{
						"rung":       "W1"
						"applicable": true
						"reason":     "frozen applicability"
					},
					{
						"rung":       "W2"
						"applicable": true
						"reason":     "frozen applicability"
					},
					{
						"rung":       "W3"
						"applicable": true
						"reason":     "frozen applicability"
					},
					{
						"rung":       "W6"
						"applicable": true
						"reason":     "frozen applicability"
					},
				]
				"workload_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"budget": {
					"tenant_policy_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"bounds": [
						{
							"metric":  "hook_p95_ns"
							"maximum": 100
						},
						{
							"metric":  "index_age_ns"
							"maximum": 100
						},
						{
							"metric":  "payload_age_ns"
							"maximum": 100
						},
						{
							"metric":  "phys_footprint_bytes"
							"maximum": 100
						},
					]
				}
				"budget_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"source_digests": [
					{
						"id":     "delivery_parameters"
						"digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					}
				]
				"profile_digest":     "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"environment_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"frozen_at":          5
			}
			"started_at":                 10
			"finished_at":                20
			"decision_at":                21
			"agent_start_at":             30
			"startup_environment_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"startup_profile_digest":     "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"workload_digest":            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"budget_digest":              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"rungs": {
				"w0": {
					"verdict": "PASS"
					"reason":  "synthetic observation"
					"evidence_ids": [
						"b-revision"
					]
				}
				"w1": {
					"verdict": "PASS"
					"reason":  "synthetic observation"
					"evidence_ids": [
						"c-registration",
						"d-negotiation",
						"e-probe",
					]
				}
				"w2": {
					"verdict": "PASS"
					"reason":  "synthetic observation"
					"evidence_ids": [
						"f-startup"
					]
				}
				"w3": {
					"verdict": "PASS"
					"reason":  "synthetic observation"
					"evidence_ids": [
						"g-trigger",
						"h-event",
						"i-beat",
					]
				}
				"w6": {
					"verdict": "PASS"
					"reason":  "synthetic observation"
					"evidence_ids": [
						"j-resource"
					]
				}
			}
			"served_product_commit":    "cccccccccccccccccccccccccccccccccccccccc"
			"installed_product_commit": "cccccccccccccccccccccccccccccccccccccccc"
			"index_commit":             "dddddddddddddddddddddddddddddddddddddddd"
			"registration": {
				"service": "fixture-service"
				"surface_tools": [
					"tool-a"
				]
				"evidence_ids": [
					"c-registration"
				]
			}
			"primer_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"negotiated_tools": [
				"tool-a"
			]
			"answered_probe_ids": [
				"e-probe"
			]
			"hook_observations": [
				{
					"hook_id":             "hook-a"
					"trigger_evidence_id": "g-trigger"
					"event_evidence_id":   "h-event"
					"beat_evidence_id":    "i-beat"
				}
			]
			"absences": [
				{
					"channel": "descriptions"
					"evidence_ids": [
						"a-inventory"
					]
				},
				{
					"channel": "maps"
					"evidence_ids": [
						"a-inventory"
					]
				},
				{
					"channel": "readouts"
					"evidence_ids": [
						"a-inventory"
					]
				},
				{
					"channel": "skills"
					"evidence_ids": [
						"a-inventory"
					]
				},
				{
					"channel": "stanza"
					"evidence_ids": [
						"a-inventory"
					]
				},
			]
			"samples": [
				{
					"metric": "hook_p95_ns"
					"value":  1
					"evidence_ids": [
						"j-resource"
					]
				},
				{
					"metric": "index_age_ns"
					"value":  1
					"evidence_ids": [
						"j-resource"
					]
				},
				{
					"metric": "payload_age_ns"
					"value":  1
					"evidence_ids": [
						"j-resource"
					]
				},
				{
					"metric": "phys_footprint_bytes"
					"value":  1
					"evidence_ids": [
						"j-resource"
					]
				},
			]
			"evidence": [
				{
					"id":          "a-inventory"
					"service":     "fixture-service"
					"source_kind": "channel_inventory"
					"phase":       "preflight"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    12
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
				{
					"id":          "b-revision"
					"service":     "fixture-service"
					"source_kind": "revision"
					"phase":       "preflight"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    12
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
				{
					"id":          "c-registration"
					"service":     "fixture-service"
					"source_kind": "registration"
					"phase":       "preflight"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    12
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
				{
					"id":          "d-negotiation"
					"service":     "fixture-service"
					"source_kind": "negotiation"
					"phase":       "preflight"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    12
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
				{
					"id":          "e-probe"
					"service":     "fixture-service"
					"source_kind": "call_result"
					"phase":       "preflight"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    12
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
				{
					"id":          "f-startup"
					"service":     "fixture-service"
					"source_kind": "startup_delivery"
					"phase":       "preflight"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    12
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
				{
					"id":          "g-trigger"
					"service":     "fixture-service"
					"source_kind": "hook_trigger"
					"phase":       "preflight"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    12
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
				{
					"id":          "h-event"
					"service":     "fixture-service"
					"source_kind": "hook_event"
					"phase":       "preflight"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    12
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
				{
					"id":          "i-beat"
					"service":     "fixture-service"
					"source_kind": "beat"
					"phase":       "preflight"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    12
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
				{
					"id":          "j-resource"
					"service":     "fixture-service"
					"source_kind": "resource_sample"
					"phase":       "preflight"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    12
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
				{
					"id":          "k-scripted"
					"service":     "fixture-service"
					"source_kind": "action"
					"phase":       "preflight"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    12
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
			]
		}
	}
	"with_zero_calls": {
		"preflight": {
			"session_id": "session"
			"cell_id":    "cell"
			"arm":        "assigned"
			"phase":      "preflight"
			"profile": {
				"id":               "fixture"
				"expected_service": "fixture-service"
				"product_present":  true
				"lab":              "lab-fixture"
				"bench_harness":    "replay"
				"pins": {
					"harness":                    "codex-cli"
					"harness_pin":                "synthetic-1"
					"model_pin":                  "fixture-1"
					"transcript_contract_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"product_commit":             "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit":             "dddddddddddddddddddddddddddddddddddddddd"
					"plugin_digest":              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"stanza_sha":                 "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"description_sha":            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				}
				"environment": {
					"kind":        "local"
					"host_class":  "macos"
					"harness_pin": "synthetic-1"
					"permitted_tools": [
						"read"
					]
					"startup_instructions_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"action_capture_digest":       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				}
				"channels": [
					{
						"channel":        "descriptions"
						"enabled":        false
						"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					},
					{
						"channel":        "hooks"
						"enabled":        true
						"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					},
					{
						"channel":        "maps"
						"enabled":        false
						"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					},
					{
						"channel":        "prime"
						"enabled":        true
						"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					},
					{
						"channel":        "readouts"
						"enabled":        false
						"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					},
					{
						"channel":        "skills"
						"enabled":        false
						"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					},
					{
						"channel":        "stanza"
						"enabled":        false
						"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					},
					{
						"channel":        "surface"
						"enabled":        true
						"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					},
				]
				"hooks": [
					{
						"id":             "hook-a"
						"enabled":        true
						"trigger_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"event":          "synthetic"
						"timeout_ns":     10
					}
				]
				"surface_tools": [
					"tool-a"
				]
				"requirements": [
					{
						"rung":       "W0"
						"applicable": true
						"reason":     "frozen applicability"
					},
					{
						"rung":       "W1"
						"applicable": true
						"reason":     "frozen applicability"
					},
					{
						"rung":       "W2"
						"applicable": true
						"reason":     "frozen applicability"
					},
					{
						"rung":       "W3"
						"applicable": true
						"reason":     "frozen applicability"
					},
					{
						"rung":       "W6"
						"applicable": true
						"reason":     "frozen applicability"
					},
				]
				"workload_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"budget": {
					"tenant_policy_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"bounds": [
						{
							"metric":  "hook_p95_ns"
							"maximum": 100
						},
						{
							"metric":  "index_age_ns"
							"maximum": 100
						},
						{
							"metric":  "payload_age_ns"
							"maximum": 100
						},
						{
							"metric":  "phys_footprint_bytes"
							"maximum": 100
						},
					]
				}
				"budget_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"source_digests": [
					{
						"id":     "delivery_parameters"
						"digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					}
				]
				"profile_digest":     "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"environment_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"frozen_at":          5
			}
			"started_at":                 10
			"finished_at":                20
			"decision_at":                21
			"agent_start_at":             30
			"startup_environment_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"startup_profile_digest":     "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"workload_digest":            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"budget_digest":              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"rungs": {
				"w0": {
					"verdict": "PASS"
					"reason":  "synthetic observation"
					"evidence_ids": [
						"b-revision"
					]
				}
				"w1": {
					"verdict": "PASS"
					"reason":  "synthetic observation"
					"evidence_ids": [
						"c-registration",
						"d-negotiation",
						"e-probe",
					]
				}
				"w2": {
					"verdict": "PASS"
					"reason":  "synthetic observation"
					"evidence_ids": [
						"f-startup"
					]
				}
				"w3": {
					"verdict": "PASS"
					"reason":  "synthetic observation"
					"evidence_ids": [
						"g-trigger",
						"h-event",
						"i-beat",
					]
				}
				"w6": {
					"verdict": "PASS"
					"reason":  "synthetic observation"
					"evidence_ids": [
						"j-resource"
					]
				}
			}
			"served_product_commit":    "cccccccccccccccccccccccccccccccccccccccc"
			"installed_product_commit": "cccccccccccccccccccccccccccccccccccccccc"
			"index_commit":             "dddddddddddddddddddddddddddddddddddddddd"
			"registration": {
				"service": "fixture-service"
				"surface_tools": [
					"tool-a"
				]
				"evidence_ids": [
					"c-registration"
				]
			}
			"primer_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"negotiated_tools": [
				"tool-a"
			]
			"answered_probe_ids": [
				"e-probe"
			]
			"hook_observations": [
				{
					"hook_id":             "hook-a"
					"trigger_evidence_id": "g-trigger"
					"event_evidence_id":   "h-event"
					"beat_evidence_id":    "i-beat"
				}
			]
			"absences": [
				{
					"channel": "descriptions"
					"evidence_ids": [
						"a-inventory"
					]
				},
				{
					"channel": "maps"
					"evidence_ids": [
						"a-inventory"
					]
				},
				{
					"channel": "readouts"
					"evidence_ids": [
						"a-inventory"
					]
				},
				{
					"channel": "skills"
					"evidence_ids": [
						"a-inventory"
					]
				},
				{
					"channel": "stanza"
					"evidence_ids": [
						"a-inventory"
					]
				},
			]
			"samples": [
				{
					"metric": "hook_p95_ns"
					"value":  1
					"evidence_ids": [
						"j-resource"
					]
				},
				{
					"metric": "index_age_ns"
					"value":  1
					"evidence_ids": [
						"j-resource"
					]
				},
				{
					"metric": "payload_age_ns"
					"value":  1
					"evidence_ids": [
						"j-resource"
					]
				},
				{
					"metric": "phys_footprint_bytes"
					"value":  1
					"evidence_ids": [
						"j-resource"
					]
				},
			]
			"evidence": [
				{
					"id":          "a-inventory"
					"service":     "fixture-service"
					"source_kind": "channel_inventory"
					"phase":       "preflight"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    12
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
				{
					"id":          "b-revision"
					"service":     "fixture-service"
					"source_kind": "revision"
					"phase":       "preflight"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    12
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
				{
					"id":          "c-registration"
					"service":     "fixture-service"
					"source_kind": "registration"
					"phase":       "preflight"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    12
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
				{
					"id":          "d-negotiation"
					"service":     "fixture-service"
					"source_kind": "negotiation"
					"phase":       "preflight"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    12
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
				{
					"id":          "e-probe"
					"service":     "fixture-service"
					"source_kind": "call_result"
					"phase":       "preflight"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    12
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
				{
					"id":          "f-startup"
					"service":     "fixture-service"
					"source_kind": "startup_delivery"
					"phase":       "preflight"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    12
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
				{
					"id":          "g-trigger"
					"service":     "fixture-service"
					"source_kind": "hook_trigger"
					"phase":       "preflight"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    12
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
				{
					"id":          "h-event"
					"service":     "fixture-service"
					"source_kind": "hook_event"
					"phase":       "preflight"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    12
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
				{
					"id":          "i-beat"
					"service":     "fixture-service"
					"source_kind": "beat"
					"phase":       "preflight"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    12
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
				{
					"id":          "j-resource"
					"service":     "fixture-service"
					"source_kind": "resource_sample"
					"phase":       "preflight"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    12
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
				{
					"id":          "k-scripted"
					"service":     "fixture-service"
					"source_kind": "action"
					"phase":       "preflight"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    12
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
			]
		}
		"outcome": {
			"session_id": "session"
			"cell_id":    "cell"
			"arm":        "assigned"
			"phase":      "agent"
			"pins": {
				"harness":                    "codex-cli"
				"harness_pin":                "synthetic-1"
				"model_pin":                  "fixture-1"
				"transcript_contract_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"product_commit":             "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit":             "dddddddddddddddddddddddddddddddddddddddd"
				"plugin_digest":              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"stanza_sha":                 "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"description_sha":            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			}
			"profile_digest":             "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"environment_digest":         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"agent_start_at":             30
			"delivery_parameters_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"k":                          3
			"k_frozen_at":                5
			"capabilities": [
				{
					"capability": "capability-a"
					"eligible": {
						"state": "known"
						"count": 2
					}
					"successful_calls": {
						"state": "known"
						"count": 0
					}
					"acted": {
						"state": "known"
						"count": 2
					}
					"should_not_calls": {
						"state": "known"
						"count": 0
					}
					"unknown": {
						"state": "known"
						"count": 0
					}
					"denominator": {
						"eligible_opportunities": {
							"state": "known"
							"count": 2
						}
						"delivered_targets": {
							"state": "known"
							"count": 2
						}
						"eligibility": {
							"state":  "validated"
							"reason": "synthetic provenance"
							"evidence_ids": [
								"a-eligibility"
							]
							"set_digest":                "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
							"classifier_digest":         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
							"validation_reading_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						}
						"delivery": {
							"state":  "validated"
							"reason": "synthetic provenance"
							"evidence_ids": [
								"b-delivery"
							]
							"set_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						}
					}
					"action_capture": {
						"state":  "validated"
						"reason": "synthetic provenance"
						"evidence_ids": [
							"c-capture"
						]
						"set_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					}
					"w4": {
						"state":       "measured"
						"numerator":   0
						"denominator": 2
					}
					"w5": {
						"state":       "measured"
						"numerator":   2
						"denominator": 2
					}
					"bars": {
						"use_minimum":   0.5
						"acted_minimum": 0.5
					}
				}
			]
			"evidence": [
				{
					"id":          "a-eligibility"
					"service":     "fixture-service"
					"source_kind": "eligibility"
					"phase":       "agent"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    31
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
				{
					"id":          "b-delivery"
					"service":     "fixture-service"
					"source_kind": "call_result"
					"phase":       "agent"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    32
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
				{
					"id":          "c-capture"
					"service":     "fixture-service"
					"source_kind": "action_capture"
					"phase":       "agent"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    34
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
					"capture": {
						"capability": "capability-a"
						"complete":   true
						"action_evidence_ids": [
							"d-action"
						]
					}
				},
				{
					"id":          "d-action"
					"service":     "fixture-service"
					"source_kind": "action"
					"phase":       "agent"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    33
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
			]
		}
	}
	"with_zero_actions": {
		"preflight": {
			"session_id": "session"
			"cell_id":    "cell"
			"arm":        "assigned"
			"phase":      "preflight"
			"profile": {
				"id":               "fixture"
				"expected_service": "fixture-service"
				"product_present":  true
				"lab":              "lab-fixture"
				"bench_harness":    "replay"
				"pins": {
					"harness":                    "codex-cli"
					"harness_pin":                "synthetic-1"
					"model_pin":                  "fixture-1"
					"transcript_contract_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"product_commit":             "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit":             "dddddddddddddddddddddddddddddddddddddddd"
					"plugin_digest":              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"stanza_sha":                 "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"description_sha":            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				}
				"environment": {
					"kind":        "local"
					"host_class":  "macos"
					"harness_pin": "synthetic-1"
					"permitted_tools": [
						"read"
					]
					"startup_instructions_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"action_capture_digest":       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				}
				"channels": [
					{
						"channel":        "descriptions"
						"enabled":        false
						"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					},
					{
						"channel":        "hooks"
						"enabled":        true
						"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					},
					{
						"channel":        "maps"
						"enabled":        false
						"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					},
					{
						"channel":        "prime"
						"enabled":        true
						"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					},
					{
						"channel":        "readouts"
						"enabled":        false
						"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					},
					{
						"channel":        "skills"
						"enabled":        false
						"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					},
					{
						"channel":        "stanza"
						"enabled":        false
						"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					},
					{
						"channel":        "surface"
						"enabled":        true
						"content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					},
				]
				"hooks": [
					{
						"id":             "hook-a"
						"enabled":        true
						"trigger_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"event":          "synthetic"
						"timeout_ns":     10
					}
				]
				"surface_tools": [
					"tool-a"
				]
				"requirements": [
					{
						"rung":       "W0"
						"applicable": true
						"reason":     "frozen applicability"
					},
					{
						"rung":       "W1"
						"applicable": true
						"reason":     "frozen applicability"
					},
					{
						"rung":       "W2"
						"applicable": true
						"reason":     "frozen applicability"
					},
					{
						"rung":       "W3"
						"applicable": true
						"reason":     "frozen applicability"
					},
					{
						"rung":       "W6"
						"applicable": true
						"reason":     "frozen applicability"
					},
				]
				"workload_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"budget": {
					"tenant_policy_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"bounds": [
						{
							"metric":  "hook_p95_ns"
							"maximum": 100
						},
						{
							"metric":  "index_age_ns"
							"maximum": 100
						},
						{
							"metric":  "payload_age_ns"
							"maximum": 100
						},
						{
							"metric":  "phys_footprint_bytes"
							"maximum": 100
						},
					]
				}
				"budget_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"source_digests": [
					{
						"id":     "delivery_parameters"
						"digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					}
				]
				"profile_digest":     "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"environment_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"frozen_at":          5
			}
			"started_at":                 10
			"finished_at":                20
			"decision_at":                21
			"agent_start_at":             30
			"startup_environment_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"startup_profile_digest":     "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"workload_digest":            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"budget_digest":              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"rungs": {
				"w0": {
					"verdict": "PASS"
					"reason":  "synthetic observation"
					"evidence_ids": [
						"b-revision"
					]
				}
				"w1": {
					"verdict": "PASS"
					"reason":  "synthetic observation"
					"evidence_ids": [
						"c-registration",
						"d-negotiation",
						"e-probe",
					]
				}
				"w2": {
					"verdict": "PASS"
					"reason":  "synthetic observation"
					"evidence_ids": [
						"f-startup"
					]
				}
				"w3": {
					"verdict": "PASS"
					"reason":  "synthetic observation"
					"evidence_ids": [
						"g-trigger",
						"h-event",
						"i-beat",
					]
				}
				"w6": {
					"verdict": "PASS"
					"reason":  "synthetic observation"
					"evidence_ids": [
						"j-resource"
					]
				}
			}
			"served_product_commit":    "cccccccccccccccccccccccccccccccccccccccc"
			"installed_product_commit": "cccccccccccccccccccccccccccccccccccccccc"
			"index_commit":             "dddddddddddddddddddddddddddddddddddddddd"
			"registration": {
				"service": "fixture-service"
				"surface_tools": [
					"tool-a"
				]
				"evidence_ids": [
					"c-registration"
				]
			}
			"primer_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"negotiated_tools": [
				"tool-a"
			]
			"answered_probe_ids": [
				"e-probe"
			]
			"hook_observations": [
				{
					"hook_id":             "hook-a"
					"trigger_evidence_id": "g-trigger"
					"event_evidence_id":   "h-event"
					"beat_evidence_id":    "i-beat"
				}
			]
			"absences": [
				{
					"channel": "descriptions"
					"evidence_ids": [
						"a-inventory"
					]
				},
				{
					"channel": "maps"
					"evidence_ids": [
						"a-inventory"
					]
				},
				{
					"channel": "readouts"
					"evidence_ids": [
						"a-inventory"
					]
				},
				{
					"channel": "skills"
					"evidence_ids": [
						"a-inventory"
					]
				},
				{
					"channel": "stanza"
					"evidence_ids": [
						"a-inventory"
					]
				},
			]
			"samples": [
				{
					"metric": "hook_p95_ns"
					"value":  1
					"evidence_ids": [
						"j-resource"
					]
				},
				{
					"metric": "index_age_ns"
					"value":  1
					"evidence_ids": [
						"j-resource"
					]
				},
				{
					"metric": "payload_age_ns"
					"value":  1
					"evidence_ids": [
						"j-resource"
					]
				},
				{
					"metric": "phys_footprint_bytes"
					"value":  1
					"evidence_ids": [
						"j-resource"
					]
				},
			]
			"evidence": [
				{
					"id":          "a-inventory"
					"service":     "fixture-service"
					"source_kind": "channel_inventory"
					"phase":       "preflight"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    12
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
				{
					"id":          "b-revision"
					"service":     "fixture-service"
					"source_kind": "revision"
					"phase":       "preflight"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    12
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
				{
					"id":          "c-registration"
					"service":     "fixture-service"
					"source_kind": "registration"
					"phase":       "preflight"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    12
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
				{
					"id":          "d-negotiation"
					"service":     "fixture-service"
					"source_kind": "negotiation"
					"phase":       "preflight"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    12
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
				{
					"id":          "e-probe"
					"service":     "fixture-service"
					"source_kind": "call_result"
					"phase":       "preflight"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    12
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
				{
					"id":          "f-startup"
					"service":     "fixture-service"
					"source_kind": "startup_delivery"
					"phase":       "preflight"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    12
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
				{
					"id":          "g-trigger"
					"service":     "fixture-service"
					"source_kind": "hook_trigger"
					"phase":       "preflight"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    12
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
				{
					"id":          "h-event"
					"service":     "fixture-service"
					"source_kind": "hook_event"
					"phase":       "preflight"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    12
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
				{
					"id":          "i-beat"
					"service":     "fixture-service"
					"source_kind": "beat"
					"phase":       "preflight"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    12
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
				{
					"id":          "j-resource"
					"service":     "fixture-service"
					"source_kind": "resource_sample"
					"phase":       "preflight"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    12
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
				{
					"id":          "k-scripted"
					"service":     "fixture-service"
					"source_kind": "action"
					"phase":       "preflight"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    12
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
			]
		}
		"outcome": {
			"session_id": "session"
			"cell_id":    "cell"
			"arm":        "assigned"
			"phase":      "agent"
			"pins": {
				"harness":                    "codex-cli"
				"harness_pin":                "synthetic-1"
				"model_pin":                  "fixture-1"
				"transcript_contract_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"product_commit":             "cccccccccccccccccccccccccccccccccccccccc"
				"subject_commit":             "dddddddddddddddddddddddddddddddddddddddd"
				"plugin_digest":              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"stanza_sha":                 "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				"description_sha":            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			}
			"profile_digest":             "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"environment_digest":         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"agent_start_at":             30
			"delivery_parameters_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			"k":                          3
			"k_frozen_at":                5
			"capabilities": [
				{
					"capability": "capability-a"
					"eligible": {
						"state": "known"
						"count": 2
					}
					"successful_calls": {
						"state": "known"
						"count": 1
					}
					"acted": {
						"state": "known"
						"count": 0
					}
					"should_not_calls": {
						"state": "known"
						"count": 0
					}
					"unknown": {
						"state": "known"
						"count": 0
					}
					"denominator": {
						"eligible_opportunities": {
							"state": "known"
							"count": 2
						}
						"delivered_targets": {
							"state": "known"
							"count": 2
						}
						"eligibility": {
							"state":  "validated"
							"reason": "synthetic provenance"
							"evidence_ids": [
								"a-eligibility"
							]
							"set_digest":                "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
							"classifier_digest":         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
							"validation_reading_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						}
						"delivery": {
							"state":  "validated"
							"reason": "synthetic provenance"
							"evidence_ids": [
								"b-delivery"
							]
							"set_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						}
					}
					"action_capture": {
						"state":  "validated"
						"reason": "synthetic provenance"
						"evidence_ids": [
							"c-capture"
						]
						"set_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					}
					"w4": {
						"state":       "measured"
						"numerator":   1
						"denominator": 2
					}
					"w5": {
						"state":       "measured"
						"numerator":   0
						"denominator": 2
					}
					"bars": {
						"use_minimum":   0.5
						"acted_minimum": 0.5
					}
				}
			]
			"evidence": [
				{
					"id":          "a-eligibility"
					"service":     "fixture-service"
					"source_kind": "eligibility"
					"phase":       "agent"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    31
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
				{
					"id":          "b-delivery"
					"service":     "fixture-service"
					"source_kind": "call_result"
					"phase":       "agent"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    32
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
				{
					"id":          "c-capture"
					"service":     "fixture-service"
					"source_kind": "action_capture"
					"phase":       "agent"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    34
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
					"capture": {
						"capability": "capability-a"
						"complete":   true
						"action_evidence_ids": [
							"d-action"
						]
					}
				},
				{
					"id":          "d-action"
					"service":     "fixture-service"
					"source_kind": "action"
					"phase":       "agent"
					"session_id":  "session"
					"call_id":     "operation"
					"range": {
						"artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						"start":           0
						"end":             1
					}
					"observed_at":    33
					"product_commit": "cccccccccccccccccccccccccccccccccccccccc"
					"subject_commit": "dddddddddddddddddddddddddddddddddddddddd"
					"profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					"harness_pin":    "synthetic-1"
					"reason":         "synthetic evidence"
				},
			]
		}
	}
}
