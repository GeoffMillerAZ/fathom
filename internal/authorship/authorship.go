// Package authorship implements E2-2's AI-authorship provenance oracle
// (docs/plans/new-use-cases.md, Phase E §E2): a deterministic, Model-empty
// pass that parses "Co-Authored-By:" trailers out of already-ingested commit
// bodies (internal/fusion's widened parseGitLog, E2-1) into governed
// core.SemanticClaim rows at tier core.TierVerified.
//
// This is the "lexical" oracle class named by ADR-018 (docs/design/
// sdd-reconciler.md §5): truth is established by a plain, reproducible text
// match — no LLM, no human corroboration — so unlike every other v1 claim
// kind (obligations, findings, adr-rules, spec-bindings; all doc-claimed,
// compiled by internal/semantic.Compiler against an LLM), authorship claims
// are compiled here, directly, with no manifest #Layer declaration: the
// generic corpus-compile runtime (schema/semantic/schema.cue's #Layer) is
// shaped for LLM restatement of governing DOCUMENTS matched by glob — a
// citation range, a prompt file, a cost budget. A commit message is not a
// corpus document and a trailer match spends zero tokens, so forcing this
// pass through that machinery would be a shape mismatch, not a reuse: it
// would need a fabricated corpus glob and a nonexistent prompt file to
// satisfy #Layer's required fields for no actual benefit. What IS reused is
// the shared data model — core.SemanticClaim / core.ClaimLink, the same
// (layer, kind, subject) content-addressed ClaimID formula, the same
// core.TierVerified/ClaimFresh vocabulary, the same store.PutClaims sink —
// so every consumer of the claims tables (contradiction scan, the companion
// surface, `fathom report`) sees authorship claims exactly like any other
// claim kind, indistinguishable in storage from a doc-claimed one.
package authorship

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// LayerAuthorship names the claim layer this package emits into.
const LayerAuthorship = "authorship"

// KindAuthorship is the claim kind this package emits (matches
// schema/semantic/schema.cue's #Layer.kind naming convention,
// ^[a-z][a-z0-9-]*$, even though this layer has no manifest declaration —
// see the package doc comment).
const KindAuthorship = "authorship"

// authorshipLayerVersion is this oracle's own version identity, independent
// of the manifest #Layer.version convention it does not participate in.
const authorshipLayerVersion = "v1"

// authorshipPassVersion identifies the deterministic pass itself (the
// Model-empty analogue of PromptVersion) so a future rule change to
// trailerPattern is visible in stored claims' provenance without needing a
// hand-bumped constant to be remembered at every call site.
const authorshipPassVersion = "authorship/lexical-v1"

// Trailer is one parsed "Co-Authored-By:" line from a commit body.
type Trailer struct {
	Name  string // e.g. "Claude Fable 5"
	Email string // e.g. "noreply@anthropic.com"
	Line  string // the verbatim trailer line — this IS the claim evidence
}

// trailerPattern matches a git trailer of the form "Co-authored-by: Name
// <email>", case-insensitively on the key (git trailers are conventionally
// case-insensitive; real-world commits use both "Co-Authored-By" and
// "Co-authored-by"). Anchored per-line via (?m); Name is trimmed, non-greedy
// up to the angle bracket.
var trailerPattern = regexp.MustCompile(`(?im)^co-authored-by:\s*(.+?)\s*<([^<>\s]+)>\s*$`)

// ParseTrailers scans a commit body (subject + blank line + prose +
// trailers, as internal/fusion.parseGitLog now captures it — E2-1) for
// "Co-Authored-By:" trailers, returning them in the order they appear. A
// body with no trailer, or an empty body, returns nil.
func ParseTrailers(body string) []Trailer {
	if body == "" {
		return nil
	}
	var out []Trailer
	for _, line := range strings.Split(body, "\n") {
		m := trailerPattern.FindStringSubmatch(strings.TrimRight(line, "\r"))
		if m == nil {
			continue
		}
		out = append(out, Trailer{
			Name:  m[1],
			Email: m[2],
			Line:  strings.TrimSpace(line),
		})
	}
	return out
}

// CompileClaims is the deterministic, Model-empty oracle (ADR-018 lexical
// class): for every commit's DISTINCT Co-Authored-By trailer line
// (byte-identical duplicates within one commit are deduplicated — they
// would share a ClaimID and upsert into one stored row anyway, and the
// emitted count must equal the persisted count), it emits one
// core.SemanticClaim at core.TierVerified, subject = the commit SHA,
// Rule = the verbatim trailer line (the evidence — no ClaimLink citation is
// needed since the fact is fully self-contained in the trailer text). Input
// order is irrelevant to the output: commits are processed in Hash order, so
// the result (and every ClaimID within it) is identical regardless of
// whether callers pass ListCommits' newest-first order or any other. No
// links are ever produced (always returns an empty/nil second value) —
// authorship claims are not doc citations or code-scoped rules, so
// core.RelCites/RelScopes do not apply; the return shape is kept symmetric
// with internal/semantic.Compiler.Compile for callers that already know how
// to plumb (claims, links) into store.Store.PutClaims.
func CompileClaims(serviceID string, commits []store.Commit, watermark string, at time.Time) ([]core.SemanticClaim, []core.ClaimLink) {
	if len(commits) == 0 {
		return nil, nil
	}
	ordered := append([]store.Commit(nil), commits...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Hash < ordered[j].Hash })

	var claims []core.SemanticClaim
	compiledAt := at.UTC()
	for _, c := range ordered {
		trailers := ParseTrailers(c.Message)
		if len(trailers) == 0 {
			continue
		}
		// Dedupe byte-identical trailer lines within one commit (cherry-pick
		// and squash tooling routinely duplicates them): two identical lines
		// share the same reproKey and hence the same content-addressed
		// ClaimID, so the store would upsert them into ONE row — minting two
		// claims here would make the report's "claims emitted" count disagree
		// with what actually persists. Emitted must equal persisted.
		seen := map[string]bool{}
		for _, tr := range trailers {
			if seen[tr.Line] {
				continue
			}
			seen[tr.Line] = true
			reproSum := sha256.Sum256([]byte(tr.Line))
			reproKey := hex.EncodeToString(reproSum[:])
			digestSum := sha256.Sum256([]byte(c.Message))
			claims = append(claims, core.SemanticClaim{
				ClaimID:             core.ClaimID(LayerAuthorship, KindAuthorship, c.Hash, reproKey),
				ServiceID:           serviceID,
				Layer:               LayerAuthorship,
				LayerVersion:        authorshipLayerVersion,
				Kind:                KindAuthorship,
				Subject:             c.Hash,
				Rule:                tr.Line,
				Tier:                core.TierVerified,
				TierProfile:         "engineering",
				Status:              core.ClaimFresh,
				Model:               "", // Model-empty: this is a lexical pass, not an LLM restatement
				PromptVersion:       authorshipPassVersion,
				ReproKey:            reproKey,
				InputDigest:         hex.EncodeToString(digestSum[:]),
				CompiledAtWatermark: watermark,
				CompiledAt:          compiledAt,
			})
		}
	}
	return claims, nil
}
