package anthropic

import "github.com/geoffmilleraz/agentic-kit/foundations/llmclient/anthropic"

effortOnly: anthropic.#OutputConfig & {effort: "high"}
effortAndSchema: anthropic.#OutputConfig & {
	effort: "medium"
	format: {type: "json_schema", schema: {type: "object"}}
}
extendedEffort: anthropic.#OutputConfig & {effort: "xhigh"}
maximumEffort: anthropic.#OutputConfig & {effort: "max"}
