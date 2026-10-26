package assignmentkit

import "github.com/geoffmilleraz/agentic-kit/kits/assignment-kit:assignmentkit"

assignment: assignmentkit.#Assignment & {
 version: 1
 id: "tiny"
 title: "Check one outcome"
 intent_refs: [{id: "reference", revision_digest: "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"}]
 policy_digest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
 subject_digest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
 goals: [{
  id: "required"
  title: "Reproducible evidence"
  kind: "required"
  depends_on: []
  profile: {
   version: 1
   id: "check"
   domain: "reference"
   criteria: [{id: "test", definition_digest: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", kind: "required", allowed_methods: ["deterministic"], allow_not_applicable: false, weight: 0}]
  }
  confidence: []
 }]
 concerns: []
 budgets: [{id: "attempts", unit: "count", limit: 2}]
 deadline: "2026-10-23T13:00:00Z"
 rules: []
}
