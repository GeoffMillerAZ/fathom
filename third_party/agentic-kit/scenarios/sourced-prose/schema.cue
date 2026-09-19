package sourcedprose

import (
	prose "github.com/geoffmilleraz/agentic-kit/kits/governed-prose-kit:governedprosekit"
	hitl "github.com/geoffmilleraz/agentic-kit/kits/hitl"
)

// The successful synthetic Run result; underlying kits own their wire shapes.
#Report: {
	Export:         prose.#ProseExport
	Invalidated:    prose.#SourcedArtifact
	Receipt:        hitl.#CommitReceipt
	Remote:         hitl.#Attempt
	LocalEffects:   1
	RemoteEffects:  1
	GeneratorCalls: 3
	Denied: ["unsupported-14", "local-replay", "stale-approval", "invalidated-export", "stale-source", "unknown-resend", "duplicate-prepare", "reconciled-resend"]
}
