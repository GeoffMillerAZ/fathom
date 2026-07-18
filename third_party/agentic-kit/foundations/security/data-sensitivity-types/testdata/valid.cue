// valid.cue — example Classification values that satisfy #Classification.
package datasensitivitytypes

import "github.com/geoffmilleraz/agentic-kit/foundations/security/data-sensitivity-types:datasensitivitytypes"

PublicExample: datasensitivitytypes.#Classification & {
	Level:  "public"
	Reason: "No access controls required; suitable for external audiences."
}

InternalExample: datasensitivitytypes.#Classification & {
	Level:  "internal"
	Reason: "For internal use only; not for external distribution."
}

ConfidentialExample: datasensitivitytypes.#Classification & {
	Level:  "confidential"
	Reason: "Contains sensitive operational details; restricted to authorised personnel."
}

RestrictedExample: datasensitivitytypes.#Classification & {
	Level:  "restricted"
	Reason: "Highest sensitivity; need-to-know access only."
}
