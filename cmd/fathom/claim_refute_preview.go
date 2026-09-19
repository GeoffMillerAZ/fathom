package main

// claim_refute_preview.go — `fathom claim refute --dry-run`, SU-18
// (docs/plans/substrate.md §D, docs/vision/31-substrate.md §3 D8 item 2):
// "a typed disposition with an impact preview ('would filter N') before it
// becomes a scoped exception." previewRefute computes, WITHOUT WRITING
// ANYTHING, which currently served rows a refutation of claimID would
// withhold: it walks the claim's own scope glob(s) against every file the
// service has indexed (the same corpus semantic.ObligationsFor's
// fileScopeMatch would match against at serve time) and counts the files
// that match — one served row per matched file, since a dry-run previews
// exactly one claim.

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/filter"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// refuteFilesPreviewCap bounds how many matched files a preview NAMES —
// WouldWithhold is always the full, untruncated count; Files/Truncated
// describe only the named subset (docs/plans/substrate.md SU-18: "would
// withhold N served rows across M files: <first 5 files>").
const refuteFilesPreviewCap = 5

// refuteDryRunResult is `fathom claim refute --dry-run`'s JSON shape.
type refuteDryRunResult struct {
	ClaimID       string   `json:"claim_id"`
	WouldWithhold int      `json:"would_withhold"`
	Files         []string `json:"files"`
	Truncated     bool     `json:"truncated"`
}

// previewRefute computes claimID's dry-run disposition. The caller (
// cmdClaimVerdict) has already confirmed claimID exists for service; a claim
// that is not currently served (already refuted or superseded) previews as
// withholding nothing — refuting it again changes nothing further.
func previewRefute(ctx context.Context, st store.Store, service, claimID string) (refuteDryRunResult, error) {
	result := refuteDryRunResult{ClaimID: claimID, Files: []string{}}

	claims, err := st.ClaimsForService(ctx, service, "")
	if err != nil {
		return refuteDryRunResult{}, fmt.Errorf("previewRefute: load claims: %w", err)
	}
	var claim core.SemanticClaim
	found := false
	for _, cl := range claims {
		if cl.ClaimID == claimID {
			claim = cl
			found = true
			break
		}
	}
	if !found {
		return refuteDryRunResult{}, fmt.Errorf("previewRefute: claim %q not found for service %q", claimID, service)
	}
	if !claim.Status.Served() {
		return result, nil // already refuted/superseded: nothing left to withhold
	}
	if len(claim.Scope) == 0 {
		return result, nil // no scope glob: never served by file, nothing to withhold
	}

	indexedFiles, err := indexedFilesFor(ctx, st, service)
	if err != nil {
		return refuteDryRunResult{}, fmt.Errorf("previewRefute: list indexed files: %w", err)
	}

	m := filter.New(claim.Scope)
	var matched []string
	for _, f := range indexedFiles {
		if m.Match(f) {
			matched = append(matched, f)
		}
	}
	sort.Strings(matched)

	result.WouldWithhold = len(matched)
	if len(matched) > refuteFilesPreviewCap {
		result.Files = append([]string(nil), matched[:refuteFilesPreviewCap]...)
		result.Truncated = true
	} else {
		result.Files = append([]string(nil), matched...)
	}
	return result, nil
}

// indexedFilesFor returns the sorted, distinct set of a service's indexed
// source files — the same QueryObservations-derived SourceRef set
// pkg/client.Client.CompileLayer builds its own scope-verification corpus
// from (semantic.go), read here directly against store.Store since
// previewRefute is store-direct (no pkg/client.Client in scope for a CLI
// verb that mirrors cmdClaimVerdict's own discipline).
func indexedFilesFor(ctx context.Context, st store.Store, service string) ([]string, error) {
	obs, err := st.QueryObservations(ctx, service, "")
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(obs))
	for _, o := range obs {
		if o.SourceRef != "" {
			seen[o.SourceRef] = struct{}{}
		}
	}
	files := make([]string, 0, len(seen))
	for f := range seen {
		files = append(files, f)
	}
	sort.Strings(files)
	return files, nil
}

// joinTruncated renders files comma-joined, with a trailing "..." marker
// when truncated is true — the CLI text render's "<first 5 files>" tail.
func joinTruncated(files []string, truncated bool) string {
	if len(files) == 0 {
		return "(none)"
	}
	s := strings.Join(files, ", ")
	if truncated {
		s += ", ..."
	}
	return s
}
