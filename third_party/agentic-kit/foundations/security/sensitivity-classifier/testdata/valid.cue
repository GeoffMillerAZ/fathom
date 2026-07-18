// valid.cue — a generic sensitivity classification catalog.
// Conforms to #Catalog defined in schema.cue.
// All patterns are generic; no org-specific literals appear here.
package sensitivityclassifier

import "github.com/geoffmilleraz/agentic-kit/foundations/security/sensitivity-classifier:sensitivityclassifier"

GenericCatalogV1: sensitivityclassifier.#Catalog & {
	Name:    "generic-v1"
	Version: "1.0.0"
	Rules: [
		{
			ID:          "CR-001"
			Pattern:     "\\b(ssn|social.security.number)\\b"
			Level:       "restricted"
			Description: "Social Security Number patterns."
		},
		{
			ID:          "CR-002"
			Pattern:     "\\b(credit.card|card.number|cvv|ccnum)\\b"
			Level:       "restricted"
			Description: "Payment card data patterns."
		},
		{
			ID:          "CR-003"
			Pattern:     "\\b(password|passwd|passphrase|secret.key|private.key|api.key)\\b"
			Level:       "confidential"
			Description: "Credential and secret patterns."
		},
		{
			ID:          "CR-004"
			Pattern:     "\\b(internal.use.only|do.not.distribute|not.for.external)\\b"
			Level:       "internal"
			Description: "Explicit internal-use markers."
		},
		{
			ID:          "CR-005"
			Pattern:     "\\b(email|phone.number|date.of.birth|home.address)\\b"
			Level:       "confidential"
			Description: "Personally identifiable information patterns."
		},
	]
}
