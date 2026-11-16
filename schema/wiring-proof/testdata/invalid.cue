// Independent single mutations of validated concrete fixtures.
// cases: [
//   {
//     "name": "unknown_verdict",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "rungs/w0/verdict",
//     "diagnostic": "verdict",
//     "value": "other"
//   },
//   {
//     "name": "unknown_harness",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "profile/pins/harness",
//     "diagnostic": "harness",
//     "value": "other"
//   },
//   {
//     "name": "unknown_environment",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "profile/environment/kind",
//     "diagnostic": "kind",
//     "value": "other"
//   },
//   {
//     "name": "unknown_source",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "evidence/0/source_kind",
//     "diagnostic": "source_kind",
//     "value": "other"
//   },
//   {
//     "name": "unknown_channel",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "profile/channels/0/channel",
//     "diagnostic": "channel",
//     "value": "other"
//   },
//   {
//     "name": "unknown_metric",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "profile/budget/bounds/0/metric",
//     "diagnostic": "budget",
//     "value": "other"
//   },
//   {
//     "name": "unknown_field",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "extra",
//     "diagnostic": "extra",
//     "value": true
//   },
//   {
//     "name": "missing_pin",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "profile/pins/model_pin",
//     "diagnostic": "model_pin",
//     "remove": true
//   },
//   {
//     "name": "short_product_commit",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "profile/pins/product_commit",
//     "diagnostic": "product_commit",
//     "value": "cccccccccccc"
//   },
//   {
//     "name": "branch_subject_commit",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "profile/pins/subject_commit",
//     "diagnostic": "subject_commit",
//     "value": "main"
//   },
//   {
//     "name": "bad_digest",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "profile/pins/plugin_digest",
//     "diagnostic": "plugin_digest",
//     "value": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
//   },
//   {
//     "name": "profile_name_as_digest",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "profile/profile_digest",
//     "diagnostic": "profile_digest",
//     "value": "profile-a"
//   },
//   {
//     "name": "w4_requirement",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "profile/requirements/4/rung",
//     "diagnostic": "rung",
//     "value": "W4"
//   },
//   {
//     "name": "w5_requirement",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "profile/requirements/4/rung",
//     "diagnostic": "rung",
//     "value": "W5"
//   },
//   {
//     "name": "duplicate_requirement",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "profile/requirements/4/rung",
//     "diagnostic": "rung",
//     "value": "W3"
//   },
//   {
//     "name": "missing_requirement",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "profile/requirements",
//     "diagnostic": "requirements",
//     "value": [
//       {
//         "rung": "W0",
//         "applicable": true,
//         "reason": "frozen applicability"
//       },
//       {
//         "rung": "W1",
//         "applicable": true,
//         "reason": "frozen applicability"
//       },
//       {
//         "rung": "W2",
//         "applicable": true,
//         "reason": "frozen applicability"
//       },
//       {
//         "rung": "W3",
//         "applicable": true,
//         "reason": "frozen applicability"
//       }
//     ]
//   },
//   {
//     "name": "applicable_na",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "rungs/w0/verdict",
//     "diagnostic": "verdict",
//     "value": "NOT-APPLICABLE"
//   },
//   {
//     "name": "inapplicable_pass",
//     "fixture": "baseline_preflight",
//     "definition": "#Preflight",
//     "path": "rungs/w0/verdict",
//     "diagnostic": "verdict",
//     "value": "PASS"
//   },
//   {
//     "name": "na_without_reason",
//     "fixture": "baseline_preflight",
//     "definition": "#Preflight",
//     "path": "rungs/w0/reason",
//     "diagnostic": "reason",
//     "remove": true
//   },
//   {
//     "name": "absence_without_inventory",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "absences/0/evidence_ids/0",
//     "diagnostic": "absences",
//     "value": "b-revision"
//   },
//   {
//     "name": "missing_off_channel",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "absences",
//     "diagnostic": "absences",
//     "value": [
//       {
//         "channel": "maps",
//         "evidence_ids": [
//           "a-inventory"
//         ]
//       },
//       {
//         "channel": "readouts",
//         "evidence_ids": [
//           "a-inventory"
//         ]
//       },
//       {
//         "channel": "skills",
//         "evidence_ids": [
//           "a-inventory"
//         ]
//       },
//       {
//         "channel": "stanza",
//         "evidence_ids": [
//           "a-inventory"
//         ]
//       }
//     ]
//   },
//   {
//     "name": "enabled_absence",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "absences/0/channel",
//     "diagnostic": "absences",
//     "value": "hooks"
//   },
//   {
//     "name": "baseline_product_present_mismatch",
//     "fixture": "baseline_preflight",
//     "definition": "#Preflight",
//     "path": "profile/requirements/0/applicable",
//     "diagnostic": "requirements",
//     "value": true
//   },
//   {
//     "name": "baseline_enabled_hook",
//     "fixture": "baseline_preflight",
//     "definition": "#Preflight",
//     "path": "profile/hooks/0/enabled",
//     "diagnostic": "hooks",
//     "value": true
//   },
//   {
//     "name": "surface_off_w0_na",
//     "fixture": "hook_only_preflight",
//     "definition": "#Preflight",
//     "path": "profile/requirements/0/applicable",
//     "diagnostic": "requirements",
//     "value": false
//   },
//   {
//     "name": "surface_off_w2_na",
//     "fixture": "hook_only_preflight",
//     "definition": "#Preflight",
//     "path": "profile/requirements/2/applicable",
//     "diagnostic": "requirements",
//     "value": false
//   },
//   {
//     "name": "surface_off_w3_na",
//     "fixture": "hook_only_preflight",
//     "definition": "#Preflight",
//     "path": "profile/requirements/3/applicable",
//     "diagnostic": "requirements",
//     "value": false
//   },
//   {
//     "name": "missing_baseline_checks",
//     "fixture": "baseline_preflight",
//     "definition": "#Preflight",
//     "path": "baseline_checks",
//     "diagnostic": "baseline_checks",
//     "remove": true
//   },
//   {
//     "name": "wrong_baseline_capture",
//     "fixture": "baseline_preflight",
//     "definition": "#Preflight",
//     "path": "baseline_checks/action_capture_evidence_id",
//     "diagnostic": "baseline_checks",
//     "value": "b-revision"
//   },
//   {
//     "name": "hooks_channel_contradiction",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "profile/channels/1/enabled",
//     "diagnostic": "channels",
//     "value": false
//   },
//   {
//     "name": "hooks_channel_enabled_without_hook",
//     "fixture": "baseline_preflight",
//     "definition": "#Preflight",
//     "path": "profile/channels/1/enabled",
//     "diagnostic": "channels",
//     "value": true
//   },
//   {
//     "name": "ancestor_served",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "served_product_commit",
//     "diagnostic": "served_product_commit",
//     "value": "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
//   },
//   {
//     "name": "wrong_installed",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "installed_product_commit",
//     "diagnostic": "installed_product_commit",
//     "value": "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
//   },
//   {
//     "name": "recent_wrong_index",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "index_commit",
//     "diagnostic": "index_commit",
//     "value": "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
//   },
//   {
//     "name": "missing_served",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "served_product_commit",
//     "diagnostic": "served_product_commit",
//     "remove": true
//   },
//   {
//     "name": "wrong_primer",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "primer_digest",
//     "diagnostic": "primer_digest",
//     "value": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
//   },
//   {
//     "name": "unnegotiated_tool",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "negotiated_tools/0",
//     "diagnostic": "negotiated_tools",
//     "value": "tool-b"
//   },
//   {
//     "name": "no_answered_probe",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "answered_probe_ids",
//     "diagnostic": "answered_probe_ids",
//     "value": []
//   },
//   {
//     "name": "missing_hook",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "hook_observations",
//     "diagnostic": "hook_observations",
//     "value": []
//   },
//   {
//     "name": "unknown_hook",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "hook_observations/0/hook_id",
//     "diagnostic": "hook_observations",
//     "value": "hook-z"
//   },
//   {
//     "name": "wrong_beat_kind",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "evidence/8/source_kind",
//     "diagnostic": "evidence",
//     "value": "hook_event"
//   },
//   {
//     "name": "unjoined_beat",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "evidence/8/call_id",
//     "diagnostic": "evidence",
//     "value": "different"
//   },
//   {
//     "name": "missing_registration",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "registration",
//     "diagnostic": "registration",
//     "remove": true
//   },
//   {
//     "name": "missing_registration_evidence",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "registration/evidence_ids",
//     "diagnostic": "registration",
//     "value": []
//   },
//   {
//     "name": "wrong_registration_kind",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "registration/evidence_ids/0",
//     "diagnostic": "registration",
//     "value": "d-negotiation"
//   },
//   {
//     "name": "wrong_registration_service",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "registration/service",
//     "diagnostic": "registration",
//     "value": "other-service"
//   },
//   {
//     "name": "wrong_registration_tools",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "registration/surface_tools/0",
//     "diagnostic": "registration",
//     "value": "tool-b"
//   },
//   {
//     "name": "missing_registration_rung_citation",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "rungs/w1/evidence_ids",
//     "diagnostic": "rungs",
//     "value": [
//       "d-negotiation",
//       "e-probe"
//     ]
//   },
//   {
//     "name": "missing_startup_service",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "evidence/5/service",
//     "diagnostic": "evidence",
//     "remove": true
//   },
//   {
//     "name": "wrong_startup_service",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "evidence/5/service",
//     "diagnostic": "evidence",
//     "value": "other-service"
//   },
//   {
//     "name": "missing_sample",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "samples",
//     "diagnostic": "samples",
//     "value": [
//       {
//         "metric": "index_age_ns",
//         "value": 1,
//         "evidence_ids": [
//           "j-resource"
//         ]
//       },
//       {
//         "metric": "payload_age_ns",
//         "value": 1,
//         "evidence_ids": [
//           "j-resource"
//         ]
//       },
//       {
//         "metric": "phys_footprint_bytes",
//         "value": 1,
//         "evidence_ids": [
//           "j-resource"
//         ]
//       }
//     ]
//   },
//   {
//     "name": "over_budget",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "samples/0/value",
//     "diagnostic": "samples",
//     "value": 101
//   },
//   {
//     "name": "wrong_footprint_metric",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "profile/budget/bounds/3/metric",
//     "diagnostic": "budget",
//     "value": "resident_bytes"
//   },
//   {
//     "name": "post_agent_decision",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "decision_at",
//     "diagnostic": "decision_at",
//     "value": 31
//   },
//   {
//     "name": "decision_at_start",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "decision_at",
//     "diagnostic": "decision_at",
//     "value": 30
//   },
//   {
//     "name": "reverse_window",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "finished_at",
//     "diagnostic": "finished_at",
//     "value": 9
//   },
//   {
//     "name": "late_freeze",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "profile/frozen_at",
//     "diagnostic": "frozen_at",
//     "value": 11
//   },
//   {
//     "name": "changed_environment",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "startup_environment_digest",
//     "diagnostic": "startup_environment_digest",
//     "value": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
//   },
//   {
//     "name": "changed_profile",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "startup_profile_digest",
//     "diagnostic": "startup_profile_digest",
//     "value": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
//   },
//   {
//     "name": "changed_workload",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "workload_digest",
//     "diagnostic": "workload_digest",
//     "value": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
//   },
//   {
//     "name": "changed_budget",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "budget_digest",
//     "diagnostic": "budget_digest",
//     "value": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
//   },
//   {
//     "name": "late_evidence",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "evidence/0/observed_at",
//     "diagnostic": "observed_at",
//     "value": 25
//   },
//   {
//     "name": "wrong_session",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "evidence/0/session_id",
//     "diagnostic": "session_id",
//     "value": "other"
//   },
//   {
//     "name": "wrong_evidence_product",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "evidence/0/product_commit",
//     "diagnostic": "product_commit",
//     "value": "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
//   },
//   {
//     "name": "wrong_evidence_subject",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "evidence/0/subject_commit",
//     "diagnostic": "subject_commit",
//     "value": "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
//   },
//   {
//     "name": "wrong_evidence_profile",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "evidence/0/profile_digest",
//     "diagnostic": "profile_digest",
//     "value": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
//   },
//   {
//     "name": "wrong_evidence_harness",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "evidence/0/harness_pin",
//     "diagnostic": "harness_pin",
//     "value": "other"
//   },
//   {
//     "name": "empty_range",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "evidence/0/range/end",
//     "diagnostic": "range",
//     "value": 0
//   },
//   {
//     "name": "dangling_reference",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "rungs/w0/evidence_ids/0",
//     "diagnostic": "rungs",
//     "value": "missing"
//   },
//   {
//     "name": "duplicate_evidence",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "evidence",
//     "diagnostic": "evidence",
//     "value": [
//       {
//         "id": "a-inventory",
//         "service": "fixture-service",
//         "source_kind": "channel_inventory",
//         "phase": "preflight",
//         "session_id": "session",
//         "call_id": "operation",
//         "range": {
//           "artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//           "start": 0,
//           "end": 1
//         },
//         "observed_at": 12,
//         "product_commit": "cccccccccccccccccccccccccccccccccccccccc",
//         "subject_commit": "dddddddddddddddddddddddddddddddddddddddd",
//         "profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//         "harness_pin": "synthetic-1",
//         "reason": "synthetic evidence"
//       },
//       {
//         "id": "a-inventory",
//         "service": "fixture-service",
//         "source_kind": "channel_inventory",
//         "phase": "preflight",
//         "session_id": "session",
//         "call_id": "operation",
//         "range": {
//           "artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//           "start": 0,
//           "end": 1
//         },
//         "observed_at": 12,
//         "product_commit": "cccccccccccccccccccccccccccccccccccccccc",
//         "subject_commit": "dddddddddddddddddddddddddddddddddddddddd",
//         "profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//         "harness_pin": "synthetic-1",
//         "reason": "synthetic evidence"
//       },
//       {
//         "id": "b-revision",
//         "service": "fixture-service",
//         "source_kind": "revision",
//         "phase": "preflight",
//         "session_id": "session",
//         "call_id": "operation",
//         "range": {
//           "artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//           "start": 0,
//           "end": 1
//         },
//         "observed_at": 12,
//         "product_commit": "cccccccccccccccccccccccccccccccccccccccc",
//         "subject_commit": "dddddddddddddddddddddddddddddddddddddddd",
//         "profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//         "harness_pin": "synthetic-1",
//         "reason": "synthetic evidence"
//       },
//       {
//         "id": "c-registration",
//         "service": "fixture-service",
//         "source_kind": "registration",
//         "phase": "preflight",
//         "session_id": "session",
//         "call_id": "operation",
//         "range": {
//           "artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//           "start": 0,
//           "end": 1
//         },
//         "observed_at": 12,
//         "product_commit": "cccccccccccccccccccccccccccccccccccccccc",
//         "subject_commit": "dddddddddddddddddddddddddddddddddddddddd",
//         "profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//         "harness_pin": "synthetic-1",
//         "reason": "synthetic evidence"
//       },
//       {
//         "id": "d-negotiation",
//         "service": "fixture-service",
//         "source_kind": "negotiation",
//         "phase": "preflight",
//         "session_id": "session",
//         "call_id": "operation",
//         "range": {
//           "artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//           "start": 0,
//           "end": 1
//         },
//         "observed_at": 12,
//         "product_commit": "cccccccccccccccccccccccccccccccccccccccc",
//         "subject_commit": "dddddddddddddddddddddddddddddddddddddddd",
//         "profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//         "harness_pin": "synthetic-1",
//         "reason": "synthetic evidence"
//       },
//       {
//         "id": "e-probe",
//         "service": "fixture-service",
//         "source_kind": "call_result",
//         "phase": "preflight",
//         "session_id": "session",
//         "call_id": "operation",
//         "range": {
//           "artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//           "start": 0,
//           "end": 1
//         },
//         "observed_at": 12,
//         "product_commit": "cccccccccccccccccccccccccccccccccccccccc",
//         "subject_commit": "dddddddddddddddddddddddddddddddddddddddd",
//         "profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//         "harness_pin": "synthetic-1",
//         "reason": "synthetic evidence"
//       },
//       {
//         "id": "f-startup",
//         "service": "fixture-service",
//         "source_kind": "startup_delivery",
//         "phase": "preflight",
//         "session_id": "session",
//         "call_id": "operation",
//         "range": {
//           "artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//           "start": 0,
//           "end": 1
//         },
//         "observed_at": 12,
//         "product_commit": "cccccccccccccccccccccccccccccccccccccccc",
//         "subject_commit": "dddddddddddddddddddddddddddddddddddddddd",
//         "profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//         "harness_pin": "synthetic-1",
//         "reason": "synthetic evidence"
//       },
//       {
//         "id": "g-trigger",
//         "service": "fixture-service",
//         "source_kind": "hook_trigger",
//         "phase": "preflight",
//         "session_id": "session",
//         "call_id": "operation",
//         "range": {
//           "artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//           "start": 0,
//           "end": 1
//         },
//         "observed_at": 12,
//         "product_commit": "cccccccccccccccccccccccccccccccccccccccc",
//         "subject_commit": "dddddddddddddddddddddddddddddddddddddddd",
//         "profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//         "harness_pin": "synthetic-1",
//         "reason": "synthetic evidence"
//       },
//       {
//         "id": "h-event",
//         "service": "fixture-service",
//         "source_kind": "hook_event",
//         "phase": "preflight",
//         "session_id": "session",
//         "call_id": "operation",
//         "range": {
//           "artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//           "start": 0,
//           "end": 1
//         },
//         "observed_at": 12,
//         "product_commit": "cccccccccccccccccccccccccccccccccccccccc",
//         "subject_commit": "dddddddddddddddddddddddddddddddddddddddd",
//         "profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//         "harness_pin": "synthetic-1",
//         "reason": "synthetic evidence"
//       },
//       {
//         "id": "i-beat",
//         "service": "fixture-service",
//         "source_kind": "beat",
//         "phase": "preflight",
//         "session_id": "session",
//         "call_id": "operation",
//         "range": {
//           "artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//           "start": 0,
//           "end": 1
//         },
//         "observed_at": 12,
//         "product_commit": "cccccccccccccccccccccccccccccccccccccccc",
//         "subject_commit": "dddddddddddddddddddddddddddddddddddddddd",
//         "profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//         "harness_pin": "synthetic-1",
//         "reason": "synthetic evidence"
//       },
//       {
//         "id": "j-resource",
//         "service": "fixture-service",
//         "source_kind": "resource_sample",
//         "phase": "preflight",
//         "session_id": "session",
//         "call_id": "operation",
//         "range": {
//           "artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//           "start": 0,
//           "end": 1
//         },
//         "observed_at": 12,
//         "product_commit": "cccccccccccccccccccccccccccccccccccccccc",
//         "subject_commit": "dddddddddddddddddddddddddddddddddddddddd",
//         "profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//         "harness_pin": "synthetic-1",
//         "reason": "synthetic evidence"
//       },
//       {
//         "id": "k-scripted",
//         "service": "fixture-service",
//         "source_kind": "action",
//         "phase": "preflight",
//         "session_id": "session",
//         "call_id": "operation",
//         "range": {
//           "artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//           "start": 0,
//           "end": 1
//         },
//         "observed_at": 12,
//         "product_commit": "cccccccccccccccccccccccccccccccccccccccc",
//         "subject_commit": "dddddddddddddddddddddddddddddddddddddddd",
//         "profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//         "harness_pin": "synthetic-1",
//         "reason": "synthetic evidence"
//       }
//     ]
//   },
//   {
//     "name": "unsorted_evidence",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "evidence",
//     "diagnostic": "evidence",
//     "value": [
//       {
//         "id": "k-scripted",
//         "service": "fixture-service",
//         "source_kind": "action",
//         "phase": "preflight",
//         "session_id": "session",
//         "call_id": "operation",
//         "range": {
//           "artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//           "start": 0,
//           "end": 1
//         },
//         "observed_at": 12,
//         "product_commit": "cccccccccccccccccccccccccccccccccccccccc",
//         "subject_commit": "dddddddddddddddddddddddddddddddddddddddd",
//         "profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//         "harness_pin": "synthetic-1",
//         "reason": "synthetic evidence"
//       },
//       {
//         "id": "j-resource",
//         "service": "fixture-service",
//         "source_kind": "resource_sample",
//         "phase": "preflight",
//         "session_id": "session",
//         "call_id": "operation",
//         "range": {
//           "artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//           "start": 0,
//           "end": 1
//         },
//         "observed_at": 12,
//         "product_commit": "cccccccccccccccccccccccccccccccccccccccc",
//         "subject_commit": "dddddddddddddddddddddddddddddddddddddddd",
//         "profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//         "harness_pin": "synthetic-1",
//         "reason": "synthetic evidence"
//       },
//       {
//         "id": "i-beat",
//         "service": "fixture-service",
//         "source_kind": "beat",
//         "phase": "preflight",
//         "session_id": "session",
//         "call_id": "operation",
//         "range": {
//           "artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//           "start": 0,
//           "end": 1
//         },
//         "observed_at": 12,
//         "product_commit": "cccccccccccccccccccccccccccccccccccccccc",
//         "subject_commit": "dddddddddddddddddddddddddddddddddddddddd",
//         "profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//         "harness_pin": "synthetic-1",
//         "reason": "synthetic evidence"
//       },
//       {
//         "id": "h-event",
//         "service": "fixture-service",
//         "source_kind": "hook_event",
//         "phase": "preflight",
//         "session_id": "session",
//         "call_id": "operation",
//         "range": {
//           "artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//           "start": 0,
//           "end": 1
//         },
//         "observed_at": 12,
//         "product_commit": "cccccccccccccccccccccccccccccccccccccccc",
//         "subject_commit": "dddddddddddddddddddddddddddddddddddddddd",
//         "profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//         "harness_pin": "synthetic-1",
//         "reason": "synthetic evidence"
//       },
//       {
//         "id": "g-trigger",
//         "service": "fixture-service",
//         "source_kind": "hook_trigger",
//         "phase": "preflight",
//         "session_id": "session",
//         "call_id": "operation",
//         "range": {
//           "artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//           "start": 0,
//           "end": 1
//         },
//         "observed_at": 12,
//         "product_commit": "cccccccccccccccccccccccccccccccccccccccc",
//         "subject_commit": "dddddddddddddddddddddddddddddddddddddddd",
//         "profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//         "harness_pin": "synthetic-1",
//         "reason": "synthetic evidence"
//       },
//       {
//         "id": "f-startup",
//         "service": "fixture-service",
//         "source_kind": "startup_delivery",
//         "phase": "preflight",
//         "session_id": "session",
//         "call_id": "operation",
//         "range": {
//           "artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//           "start": 0,
//           "end": 1
//         },
//         "observed_at": 12,
//         "product_commit": "cccccccccccccccccccccccccccccccccccccccc",
//         "subject_commit": "dddddddddddddddddddddddddddddddddddddddd",
//         "profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//         "harness_pin": "synthetic-1",
//         "reason": "synthetic evidence"
//       },
//       {
//         "id": "e-probe",
//         "service": "fixture-service",
//         "source_kind": "call_result",
//         "phase": "preflight",
//         "session_id": "session",
//         "call_id": "operation",
//         "range": {
//           "artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//           "start": 0,
//           "end": 1
//         },
//         "observed_at": 12,
//         "product_commit": "cccccccccccccccccccccccccccccccccccccccc",
//         "subject_commit": "dddddddddddddddddddddddddddddddddddddddd",
//         "profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//         "harness_pin": "synthetic-1",
//         "reason": "synthetic evidence"
//       },
//       {
//         "id": "d-negotiation",
//         "service": "fixture-service",
//         "source_kind": "negotiation",
//         "phase": "preflight",
//         "session_id": "session",
//         "call_id": "operation",
//         "range": {
//           "artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//           "start": 0,
//           "end": 1
//         },
//         "observed_at": 12,
//         "product_commit": "cccccccccccccccccccccccccccccccccccccccc",
//         "subject_commit": "dddddddddddddddddddddddddddddddddddddddd",
//         "profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//         "harness_pin": "synthetic-1",
//         "reason": "synthetic evidence"
//       },
//       {
//         "id": "c-registration",
//         "service": "fixture-service",
//         "source_kind": "registration",
//         "phase": "preflight",
//         "session_id": "session",
//         "call_id": "operation",
//         "range": {
//           "artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//           "start": 0,
//           "end": 1
//         },
//         "observed_at": 12,
//         "product_commit": "cccccccccccccccccccccccccccccccccccccccc",
//         "subject_commit": "dddddddddddddddddddddddddddddddddddddddd",
//         "profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//         "harness_pin": "synthetic-1",
//         "reason": "synthetic evidence"
//       },
//       {
//         "id": "b-revision",
//         "service": "fixture-service",
//         "source_kind": "revision",
//         "phase": "preflight",
//         "session_id": "session",
//         "call_id": "operation",
//         "range": {
//           "artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//           "start": 0,
//           "end": 1
//         },
//         "observed_at": 12,
//         "product_commit": "cccccccccccccccccccccccccccccccccccccccc",
//         "subject_commit": "dddddddddddddddddddddddddddddddddddddddd",
//         "profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//         "harness_pin": "synthetic-1",
//         "reason": "synthetic evidence"
//       },
//       {
//         "id": "a-inventory",
//         "service": "fixture-service",
//         "source_kind": "channel_inventory",
//         "phase": "preflight",
//         "session_id": "session",
//         "call_id": "operation",
//         "range": {
//           "artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//           "start": 0,
//           "end": 1
//         },
//         "observed_at": 12,
//         "product_commit": "cccccccccccccccccccccccccccccccccccccccc",
//         "subject_commit": "dddddddddddddddddddddddddddddddddddddddd",
//         "profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//         "harness_pin": "synthetic-1",
//         "reason": "synthetic evidence"
//       }
//     ]
//   },
//   {
//     "name": "agent_in_preflight",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "evidence/0/phase",
//     "diagnostic": "phase",
//     "value": "agent"
//   },
//   {
//     "name": "probe_in_outcome",
//     "fixture": "zero_calls",
//     "definition": "#Outcome",
//     "path": "evidence/0/phase",
//     "diagnostic": "phase",
//     "value": "preflight"
//   },
//   {
//     "name": "null_list",
//     "fixture": "baseline_preflight",
//     "definition": "#Preflight",
//     "path": "negotiated_tools",
//     "diagnostic": "negotiated_tools",
//     "value": null
//   },
//   {
//     "name": "outcome_binding",
//     "fixture": "with_zero_calls",
//     "definition": "#Conformance",
//     "path": "outcome/cell_id",
//     "diagnostic": "cell_id",
//     "value": "other"
//   },
//   {
//     "name": "delivery_parameters_binding",
//     "fixture": "with_zero_calls",
//     "definition": "#Conformance",
//     "path": "outcome/delivery_parameters_digest",
//     "diagnostic": "delivery_parameters_digest",
//     "value": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
//   },
//   {
//     "name": "environment_pin_binding",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "profile/environment/harness_pin",
//     "diagnostic": "harness_pin",
//     "value": "other"
//   },
//   {
//     "name": "zero_measured_denominator",
//     "fixture": "acted_at_k",
//     "definition": "#Outcome",
//     "path": "capabilities/0/w4/denominator",
//     "diagnostic": "denominator",
//     "value": 0
//   },
//   {
//     "name": "negative_count",
//     "fixture": "acted_at_k",
//     "definition": "#Outcome",
//     "path": "capabilities/0/successful_calls/count",
//     "diagnostic": "count",
//     "value": -1
//   },
//   {
//     "name": "numerator_exceeds_denominator",
//     "fixture": "acted_at_k",
//     "definition": "#Outcome",
//     "path": "capabilities/0/w4/numerator",
//     "diagnostic": "numerator",
//     "value": 3
//   },
//   {
//     "name": "redundant_rate_value",
//     "fixture": "acted_at_k",
//     "definition": "#Outcome",
//     "path": "capabilities/0/w4/value",
//     "diagnostic": "value",
//     "value": 0.5
//   },
//   {
//     "name": "wrong_denominator",
//     "fixture": "acted_at_k",
//     "definition": "#Outcome",
//     "path": "capabilities/0/w4/denominator",
//     "diagnostic": "denominator",
//     "value": 3
//   },
//   {
//     "name": "zero_k",
//     "fixture": "acted_at_k",
//     "definition": "#Outcome",
//     "path": "k",
//     "diagnostic": "k",
//     "value": 0
//   },
//   {
//     "name": "late_k",
//     "fixture": "acted_at_k",
//     "definition": "#Outcome",
//     "path": "k_frozen_at",
//     "diagnostic": "k_frozen_at",
//     "value": 30
//   },
//   {
//     "name": "missing_denominator_provenance",
//     "fixture": "acted_at_k",
//     "definition": "#Outcome",
//     "path": "capabilities/0/denominator/eligibility/validation_reading_digest",
//     "diagnostic": "validation_reading_digest",
//     "remove": true
//   },
//   {
//     "name": "missing_expected_service",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "profile/expected_service",
//     "diagnostic": "expected_service",
//     "remove": true
//   },
//   {
//     "name": "missing_registration_evidence_service",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "evidence/2/service",
//     "diagnostic": "evidence",
//     "remove": true
//   },
//   {
//     "name": "wrong_registration_evidence_service",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "evidence/2/service",
//     "diagnostic": "evidence",
//     "value": "other-service"
//   },
//   {
//     "name": "missing_negotiation_service",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "evidence/3/service",
//     "diagnostic": "evidence",
//     "remove": true
//   },
//   {
//     "name": "wrong_negotiation_service",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "evidence/3/service",
//     "diagnostic": "evidence",
//     "value": "other-service"
//   },
//   {
//     "name": "missing_probe_service",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "evidence/4/service",
//     "diagnostic": "evidence",
//     "remove": true
//   },
//   {
//     "name": "wrong_probe_service",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "evidence/4/service",
//     "diagnostic": "evidence",
//     "value": "other-service"
//   },
//   {
//     "name": "missing_measured_numerator",
//     "fixture": "acted_at_k",
//     "definition": "#Outcome",
//     "path": "capabilities/0/w4/numerator",
//     "diagnostic": "numerator",
//     "remove": true
//   },
//   {
//     "name": "unavailable_numerator",
//     "fixture": "two_deliveries_no_capture",
//     "definition": "#Outcome",
//     "path": "capabilities/0/w5/numerator",
//     "diagnostic": "numerator",
//     "value": 0
//   },
//   {
//     "name": "unvalidated_numerator",
//     "fixture": "classifier_counts_only",
//     "definition": "#Outcome",
//     "path": "capabilities/0/w4/numerator",
//     "diagnostic": "numerator",
//     "value": 1
//   },
//   {
//     "name": "unavailable_measured",
//     "fixture": "acted_at_k",
//     "definition": "#Outcome",
//     "path": "capabilities/0/action_capture/state",
//     "diagnostic": "capabilities",
//     "value": "unavailable"
//   },
//   {
//     "name": "unvalidated_measured",
//     "fixture": "acted_at_k",
//     "definition": "#Outcome",
//     "path": "capabilities/0/denominator/eligibility/state",
//     "diagnostic": "capabilities",
//     "value": "unvalidated"
//   },
//   {
//     "name": "fabricated_acted_zero",
//     "fixture": "two_deliveries_no_capture",
//     "definition": "#Outcome",
//     "path": "capabilities/0/acted",
//     "diagnostic": "acted",
//     "value": {
//       "state": "known",
//       "count": 0
//     }
//   },
//   {
//     "name": "unknown_as_zero",
//     "fixture": "unknown_denominator",
//     "definition": "#Outcome",
//     "path": "capabilities/0/denominator/eligible_opportunities/count",
//     "diagnostic": "count",
//     "value": 0
//   },
//   {
//     "name": "positive_zero_denominator_cause",
//     "fixture": "two_deliveries_no_capture",
//     "definition": "#Outcome",
//     "path": "capabilities/0/w5/cause",
//     "diagnostic": "cause",
//     "value": "zero_denominator"
//   },
//   {
//     "name": "wrong_w5_numerator",
//     "fixture": "acted_at_k",
//     "definition": "#Outcome",
//     "path": "capabilities/0/w5/numerator",
//     "diagnostic": "numerator",
//     "value": 1
//   },
//   {
//     "name": "unsupported_w4_numerator",
//     "fixture": "acted_at_k",
//     "definition": "#Outcome",
//     "path": "capabilities/0/w4/numerator",
//     "diagnostic": "numerator",
//     "value": 2
//   },
//   {
//     "name": "missing_capture_summary",
//     "fixture": "two_deliveries_complete_no_actions",
//     "definition": "#Outcome",
//     "path": "evidence/2/capture",
//     "diagnostic": "capture",
//     "remove": true
//   },
//   {
//     "name": "wrong_capture_kind",
//     "fixture": "two_deliveries_complete_no_actions",
//     "definition": "#Outcome",
//     "path": "evidence/2/source_kind",
//     "diagnostic": "evidence",
//     "value": "action"
//   },
//   {
//     "name": "incomplete_capture_measured",
//     "fixture": "two_deliveries_complete_no_actions",
//     "definition": "#Outcome",
//     "path": "evidence/2/capture/complete",
//     "diagnostic": "capture",
//     "value": false
//   },
//   {
//     "name": "dangling_captured_action",
//     "fixture": "two_deliveries_complete_no_actions",
//     "definition": "#Outcome",
//     "path": "evidence/2/capture/action_evidence_ids",
//     "diagnostic": "evidence",
//     "value": [
//       "missing"
//     ]
//   },
//   {
//     "name": "empty_capture_positive_acted",
//     "fixture": "two_deliveries_complete_no_actions",
//     "definition": "#Outcome",
//     "path": "capabilities/0/acted/count",
//     "diagnostic": "capabilities",
//     "value": 1
//   },
//   {
//     "name": "capture_in_preflight",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "capture",
//     "diagnostic": "capture",
//     "value": {
//       "capability": "capability-a",
//       "complete": true,
//       "action_evidence_ids": []
//     }
//   },
//   {
//     "name": "capture_under_profile",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "profile/capture",
//     "diagnostic": "capture",
//     "value": {
//       "capability": "capability-a",
//       "complete": true,
//       "action_evidence_ids": []
//     }
//   },
//   {
//     "name": "capture_evidence_in_preflight",
//     "fixture": "wired_preflight",
//     "definition": "#Preflight",
//     "path": "evidence/0",
//     "diagnostic": "evidence",
//     "value": {
//       "id": "c-capture",
//       "service": "fixture-service",
//       "source_kind": "action_capture",
//       "phase": "agent",
//       "session_id": "session",
//       "call_id": "operation",
//       "range": {
//         "artifact_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//         "start": 0,
//         "end": 1
//       },
//       "observed_at": 34,
//       "product_commit": "cccccccccccccccccccccccccccccccccccccccc",
//       "subject_commit": "dddddddddddddddddddddddddddddddddddddddd",
//       "profile_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
//       "harness_pin": "synthetic-1",
//       "reason": "synthetic evidence",
//       "capture": {
//         "capability": "capability-a",
//         "complete": true,
//         "action_evidence_ids": []
//       }
//     }
//   },
//   {
//     "name": "registration_reference_when_unknown",
//     "fixture": "unknown_preflight",
//     "definition": "#Preflight",
//     "path": "registration/evidence_ids/0",
//     "value": "missing",
//     "diagnostic": "registration"
//   }
// ]
