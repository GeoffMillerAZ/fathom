package core

import "time"

// ReviewRun is one recorded `fathom review --mode persist --idempotency-key
// K` outcome (FL-15, docs/plans/findings-loop.md; ADR-040 decision 6): a
// repeated call with the SAME IdempotencyKey and the SAME PayloadDigest is a
// no-op that returns ResponseJSON verbatim and writes nothing else; the same
// key with a DIFFERENT PayloadDigest is a named refusal (exit 2) — an
// idempotency key must never silently replay a different request's answer.
type ReviewRun struct {
	ServiceID      string
	IdempotencyKey string
	// PayloadDigest is a content hash of the REQUEST this run answered
	// (service, mode, changed-file identity) — never the response — so a
	// caller can tell "same request, replay the answer" from "same key,
	// different request, refuse" without re-running the review.
	PayloadDigest string
	// ResponseJSON is the exact JSON this run emitted — replayed verbatim on
	// an idempotent hit.
	ResponseJSON string
	RecordedAt   time.Time
}
