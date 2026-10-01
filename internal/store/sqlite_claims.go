package store

// sqlite_claims.go — SQLiteStore's semantic claims: the upsert and reads,
// links, status and validity updates, the claim-event log, the shared row
// scanner and the claim-time helpers (stampClaimValidFrom is shared with
// MemoryStore). Split out of sqlite.go (wave g1).

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// PutClaims upserts claims by (service_id, claim_id) and replaces the link
// set for every distinct claim_id that appears in links.
func (s *SQLiteStore) PutClaims(ctx context.Context, claims []core.SemanticClaim, links []core.ClaimLink) error {
	if len(claims) == 0 && len(links) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("PutClaims begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if len(claims) > 0 {
		stmt, err := tx.PrepareContext(ctx, `
INSERT INTO semantic_claims
    (service_id, claim_id, layer, layer_version, kind, subject, rule, scope,
     tier, tier_profile, status, model, prompt_version, repro_key, input_digest,
     compiled_at_watermark, compiled_at, class, valid_from, valid_to, asserted_by, meta, lifecycle)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (service_id, claim_id) DO UPDATE SET
    layer                 = excluded.layer,
    layer_version         = excluded.layer_version,
    kind                  = excluded.kind,
    subject               = excluded.subject,
    rule                  = excluded.rule,
    scope                 = excluded.scope,
    tier                  = excluded.tier,
    tier_profile          = excluded.tier_profile,
    -- Refutation durability (FT-4, docs/vision/03-findings-and-experts.md
    -- §3): a recompile that re-emits an unchanged rule under a refuted
    -- ClaimID must NOT resurrect it to the compiler's fresh status —
    -- suppression is refutation with a note, and it must survive every
    -- future upsert of the same claim_id, not just the first one.
    status                = CASE WHEN semantic_claims.status = 'refuted' THEN semantic_claims.status ELSE excluded.status END,
    model                 = excluded.model,
    prompt_version        = excluded.prompt_version,
    repro_key             = excluded.repro_key,
    input_digest          = excluded.input_digest,
    compiled_at_watermark = excluded.compiled_at_watermark,
    compiled_at           = excluded.compiled_at,
    class                 = excluded.class,
    -- SU-19 (ADR-041 decision 7): valid_from is stamped ONCE, at the
    -- claim_id's first-ever mint — an unchanged-content recompile that
    -- upserts the identical content-addressed row must never drift its
    -- own history forward, so a non-empty stored value always wins over
    -- whatever this call proposes.
    valid_from            = CASE WHEN semantic_claims.valid_from != '' THEN semantic_claims.valid_from ELSE excluded.valid_from END,
    -- valid_to mirrors status' own refutation-durability CASE above: a
    -- refuted claim's closed validity window survives every future
    -- upsert exactly like its status does. GL-15 (design doc §7) extends
    -- the SAME guard to lifecycle='retired' — a retired claim's window
    -- must survive a later recompile exactly like lifecycle itself does
    -- (the CASE below), or the row ends up with the terminal lifecycle
    -- but a reopened (excluded/incoming, ordinarily zero) validity
    -- window: internally inconsistent and the exact silent-resurrection
    -- shape retirement is terminal to prevent. A NON-refuted,
    -- NON-retired row (including one this call is resurrecting from
    -- superseded back to fresh — see status' own doc comment) takes the
    -- incoming value, which reopens the window (an ordinary caller
    -- passes ValidTo's zero value/"" on a normal write) exactly when the
    -- status resurrection itself does.
    valid_to              = CASE WHEN semantic_claims.status = 'refuted' OR semantic_claims.lifecycle = 'retired' THEN semantic_claims.valid_to ELSE excluded.valid_to END,
    asserted_by           = excluded.asserted_by,
    meta                  = excluded.meta,
    -- GL-13: lifecycle mirrors status' own refutation-durability CASE
    -- above — 'retired' is this axis's terminal state, exactly like
    -- 'refuted' is status's, and a recompile that re-emits the same
    -- claim_id with the compiler's default (unset) lifecycle must not
    -- silently resurrect a retired rule back to active.
    lifecycle             = CASE WHEN semantic_claims.lifecycle = 'retired' THEN semantic_claims.lifecycle ELSE excluded.lifecycle END
`)
		if err != nil {
			return fmt.Errorf("PutClaims prepare claims: %w", err)
		}
		defer stmt.Close()

		for _, c := range claims {
			c = stampClaimValidFrom(c)
			scopeJSON, err := json.Marshal(c.Scope)
			if err != nil {
				return fmt.Errorf("PutClaims marshal scope: %w", err)
			}
			metaText := ""
			if len(c.Meta) > 0 {
				metaText = string(c.Meta)
			}
			_, err = stmt.ExecContext(
				ctx,
				c.ServiceID, c.ClaimID,
				c.Layer, c.LayerVersion, c.Kind, c.Subject, c.Rule, string(scopeJSON),
				string(c.Tier), c.TierProfile,
				string(c.Status),
				c.Model, c.PromptVersion, c.ReproKey, c.InputDigest,
				c.CompiledAtWatermark, c.CompiledAt.UnixNano(), string(c.Class),
				formatClaimTime(c.ValidFrom), formatClaimTime(c.ValidTo), c.AssertedBy, metaText,
				string(c.Lifecycle),
			)
			if err != nil {
				return fmt.Errorf("PutClaims exec claim: %w", err)
			}
		}
	}

	if len(links) > 0 {
		// Collect distinct claim_ids so we can replace their link sets.
		seen := make(map[string]struct{}, len(links))
		for _, l := range links {
			seen[l.ClaimID] = struct{}{}
		}
		for id := range seen {
			if _, err := tx.ExecContext(ctx, `DELETE FROM claim_links WHERE claim_id = ?`, id); err != nil {
				return fmt.Errorf("PutClaims delete links: %w", err)
			}
		}
		// Build service_id lookup from the claims in this call.
		svcOf := make(map[string]string, len(claims))
		for _, c := range claims {
			svcOf[c.ClaimID] = c.ServiceID
		}
		lstmt, err := tx.PrepareContext(ctx, `
INSERT INTO claim_links (service_id, claim_id, rel, target_ref, target_hash, anchor_text)
VALUES (?, ?, ?, ?, ?, ?)
`)
		if err != nil {
			return fmt.Errorf("PutClaims prepare links: %w", err)
		}
		defer lstmt.Close()
		for _, l := range links {
			if _, err := lstmt.ExecContext(ctx, svcOf[l.ClaimID], l.ClaimID, l.Rel, l.TargetRef, l.TargetHash, l.AnchorText); err != nil {
				return fmt.Errorf("PutClaims exec link: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("PutClaims commit: %w", err)
	}
	return nil
}

// ClaimsForService returns claims for serviceID ordered by (layer, subject,
// claim_id). When layer is "" all layers are returned.
func (s *SQLiteStore) ClaimsForService(ctx context.Context, serviceID, layer string) ([]core.SemanticClaim, error) {
	const cols = `service_id, claim_id, layer, layer_version, kind, subject, rule, scope,
	tier, tier_profile, status, model, prompt_version, repro_key, input_digest,
	compiled_at_watermark, compiled_at, class, valid_from, valid_to, asserted_by, meta, lifecycle`
	var (
		rows *sql.Rows
		err  error
	)
	if layer == "" {
		rows, err = s.db.QueryContext(ctx, `
SELECT `+cols+`
FROM semantic_claims
WHERE service_id = ?
ORDER BY layer, subject, claim_id
`, serviceID)
	} else {
		rows, err = s.db.QueryContext(ctx, `
SELECT `+cols+`
FROM semantic_claims
WHERE service_id = ? AND layer = ?
ORDER BY layer, subject, claim_id
`, serviceID, layer)
	}
	if err != nil {
		return nil, fmt.Errorf("ClaimsForService query: %w", err)
	}
	defer rows.Close()
	return scanClaims(rows)
}

// scopeReachAnywhereConditions are the SQL LIKE fragments matching a
// stored scope column (JSON array text, e.g. `["internal/store/**"]`)
// whose glob(s) can match ANY path regardless of scopePrefixes — see
// ClaimsForServe's doc comment on the Store interface. Checked against
// the column's literal JSON text, so the quoting mirrors how
// encoding/json renders a string element: `"**"`, `"**/…`, `"*"`.
var scopeReachAnywhereLike = []string{
	`%"**"%`, // exactly "**"
	`%"**/%`, // starts with "**/"
	`%"*"%`,  // exactly "*"
}

// ClaimsForServe is ClaimsForService's prefiltered sibling — see the Store
// interface doc comment for the exact (deliberately over-approximate)
// scope-prefix semantics.
func (s *SQLiteStore) ClaimsForServe(ctx context.Context, serviceID string, layers []string, statuses []core.ClaimStatus, scopePrefixes []string) ([]core.SemanticClaim, error) {
	const cols = `service_id, claim_id, layer, layer_version, kind, subject, rule, scope,
	tier, tier_profile, status, model, prompt_version, repro_key, input_digest,
	compiled_at_watermark, compiled_at, class, valid_from, valid_to, asserted_by, meta, lifecycle`

	where := []string{"service_id = ?"}
	args := []any{serviceID}

	if len(statuses) > 0 {
		placeholders := make([]string, len(statuses))
		for i, st := range statuses {
			placeholders[i] = "?"
			args = append(args, string(st))
		}
		where = append(where, "status IN ("+strings.Join(placeholders, ", ")+")")
	}
	if len(layers) > 0 {
		placeholders := make([]string, len(layers))
		for i, l := range layers {
			placeholders[i] = "?"
			args = append(args, l)
		}
		where = append(where, "layer IN ("+strings.Join(placeholders, ", ")+")")
	}
	if len(scopePrefixes) > 0 {
		var scopeOr []string
		for _, pat := range scopeReachAnywhereLike {
			scopeOr = append(scopeOr, "scope LIKE ?")
			args = append(args, pat)
		}
		// A bare literal scope entry (no "/" anywhere in the whole JSON
		// array) is gitignore-style "matches anywhere" — see globAnchor's
		// doc comment in internal/semantic/governingscope.go.
		scopeOr = append(scopeOr, "scope NOT LIKE ?")
		args = append(args, "%/%")
		for _, prefix := range scopePrefixes {
			scopeOr = append(scopeOr, "scope LIKE ?")
			args = append(args, "%"+prefix+"%")
		}
		where = append(where, "("+strings.Join(scopeOr, " OR ")+")")
	}

	q := `SELECT ` + cols + ` FROM semantic_claims WHERE ` + strings.Join(where, " AND ") + ` ORDER BY layer, subject, claim_id`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("ClaimsForServe query: %w", err)
	}
	defer rows.Close()
	return scanClaims(rows)
}

// ClaimLinks returns links for the given claim IDs within serviceID, ordered
// by (claim_id, rel, target_ref). Returns nil with no error for empty claimIDs.
func (s *SQLiteStore) claimLinksRaw(ctx context.Context, serviceID string, claimIDs []string) ([]core.ClaimLink, error) {
	if len(claimIDs) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(claimIDs))
	args := make([]interface{}, 0, 1+len(claimIDs))
	args = append(args, serviceID)
	for i, id := range claimIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}
	q := `
SELECT claim_id, rel, target_ref, target_hash, anchor_text
FROM claim_links
WHERE service_id = ? AND claim_id IN (` + strings.Join(placeholders, ", ") + `)
ORDER BY claim_id, rel, target_ref
`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("ClaimLinks query: %w", err)
	}
	defer rows.Close()
	var result []core.ClaimLink
	for rows.Next() {
		var l core.ClaimLink
		if err := rows.Scan(&l.ClaimID, &l.Rel, &l.TargetRef, &l.TargetHash, &l.AnchorText); err != nil {
			return nil, fmt.Errorf("ClaimLinks scan: %w", err)
		}
		result = append(result, l)
	}
	return result, rows.Err()
}

// UpdateClaimLink re-anchors a single claim_links row in place — see the
// Store interface doc comment. AnchorText is left untouched: a re-anchor only
// fires on an UNAMBIGUOUS match of the already-stored anchor text at its new
// location, so the text itself never changes, only where it points.
func (s *SQLiteStore) UpdateClaimLink(ctx context.Context, serviceID, claimID, rel, oldTargetRef, newTargetRef, newTargetHash string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE claim_links
SET target_ref = ?, target_hash = ?
WHERE service_id = ? AND claim_id = ? AND rel = ? AND target_ref = ?
`, newTargetRef, newTargetHash, serviceID, claimID, rel, oldTargetRef)
	if err != nil {
		return fmt.Errorf("UpdateClaimLink: %w", err)
	}
	return nil
}

// AppendClaimLink inserts ONE link row without touching claim_id's existing
// link set (SU-20, ADR-041 decision 7) — see the Store interface doc
// comment for why this differs from PutClaims' wholesale replace.
func (s *SQLiteStore) AppendClaimLink(ctx context.Context, serviceID string, link core.ClaimLink) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO claim_links (service_id, claim_id, rel, target_ref, target_hash, anchor_text)
VALUES (?, ?, ?, ?, ?, ?)
`, serviceID, link.ClaimID, link.Rel, link.TargetRef, link.TargetHash, link.AnchorText)
	if err != nil {
		return fmt.Errorf("AppendClaimLink: %w", err)
	}
	return nil
}

// SetClaimStatus updates the status of a single claim. No error when absent.
func (s *SQLiteStore) SetClaimStatus(ctx context.Context, serviceID, claimID string, status core.ClaimStatus) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE semantic_claims SET status = ? WHERE service_id = ? AND claim_id = ?
`, string(status), serviceID, claimID)
	if err != nil {
		return fmt.Errorf("SetClaimStatus: %w", err)
	}
	return nil
}

// SetClaimValidTo closes a single claim's bi-temporal validity window at t
// (SU-19, ADR-041 decision 7) — see the Store interface doc comment.
func (s *SQLiteStore) SetClaimValidTo(ctx context.Context, serviceID, claimID string, t time.Time) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE semantic_claims SET valid_to = ? WHERE service_id = ? AND claim_id = ?
`, formatClaimTime(t), serviceID, claimID)
	if err != nil {
		return fmt.Errorf("SetClaimValidTo: %w", err)
	}
	return nil
}

// AppendClaimEvent inserts one verdict row into the append-only event log.
func (s *SQLiteStore) AppendClaimEvent(ctx context.Context, ev core.ClaimEvent) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO claim_events (claim_id, verdict, author, note, event_at, resolution_kind, resolution_commit, asserted_by)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
`, ev.ClaimID, ev.Verdict, ev.Author, ev.Note, ev.At.UnixNano(), ev.ResolutionKind, ev.ResolutionCommit, ev.AssertedBy)
	if err != nil {
		return fmt.Errorf("AppendClaimEvent: %w", err)
	}
	return nil
}

// ClaimEvents returns all events for claimID ordered by (event_at, verdict).
func (s *SQLiteStore) ClaimEvents(ctx context.Context, claimID string) ([]core.ClaimEvent, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT claim_id, verdict, author, note, event_at, resolution_kind, resolution_commit, asserted_by
FROM claim_events
WHERE claim_id = ?
ORDER BY event_at, verdict
`, claimID)
	if err != nil {
		return nil, fmt.Errorf("ClaimEvents query: %w", err)
	}
	defer rows.Close()
	var result []core.ClaimEvent
	for rows.Next() {
		var (
			ev core.ClaimEvent
			at int64
		)
		if err := rows.Scan(&ev.ClaimID, &ev.Verdict, &ev.Author, &ev.Note, &at, &ev.ResolutionKind, &ev.ResolutionCommit, &ev.AssertedBy); err != nil {
			return nil, fmt.Errorf("ClaimEvents scan: %w", err)
		}
		ev.At = time.Unix(0, at).UTC()
		result = append(result, ev)
	}
	return result, rows.Err()
}

// ClaimEventsSince (Q-6) returns every event in [since, until] across every
// claim, ordered by (event_at, verdict) — see the Store interface's own doc
// comment for the store-wide-not-service-scoped caveat (claim_events has no
// service_id column).
func (s *SQLiteStore) ClaimEventsSince(ctx context.Context, since, until time.Time) ([]core.ClaimEvent, error) {
	return queryClaimEventsSince(ctx, s.db, since, until)
}

// queryClaimEventsSince is SQLiteStore.ClaimEventsSince's query body,
// factored into its own free function — a name unique to this file (unlike
// the ClaimEventsSince method name, which SQLiteStore and MemoryStore both
// implement) so a symbol-based tool can anchor to a single, unambiguous
// definition.
func queryClaimEventsSince(ctx context.Context, db *sql.DB, since, until time.Time) ([]core.ClaimEvent, error) {
	rows, err := db.QueryContext(ctx, `
SELECT claim_id, verdict, author, note, event_at, resolution_kind, resolution_commit, asserted_by
FROM claim_events
WHERE event_at >= ? AND event_at <= ?
ORDER BY event_at, verdict
`, since.UnixNano(), until.UnixNano())
	if err != nil {
		return nil, fmt.Errorf("ClaimEventsSince query: %w", err)
	}
	defer rows.Close()
	var result []core.ClaimEvent
	for rows.Next() {
		var (
			ev core.ClaimEvent
			at int64
		)
		if err := rows.Scan(&ev.ClaimID, &ev.Verdict, &ev.Author, &ev.Note, &at, &ev.ResolutionKind, &ev.ResolutionCommit, &ev.AssertedBy); err != nil {
			return nil, fmt.Errorf("ClaimEventsSince scan: %w", err)
		}
		ev.At = time.Unix(0, at).UTC()
		result = append(result, ev)
	}
	return result, rows.Err()
}

// scanClaims scans a full semantic_claims row-set (the shared column order
// ClaimsForService's callers rely on). Backend-agnostic — see scanObservations.
func scanClaims(rows *sql.Rows) ([]core.SemanticClaim, error) {
	var result []core.SemanticClaim
	for rows.Next() {
		var (
			c             core.SemanticClaim
			scopeJSON     string
			compiledAt    int64
			validFromText string
			validToText   string
			metaText      string
		)
		if err := rows.Scan(
			&c.ServiceID, &c.ClaimID,
			&c.Layer, &c.LayerVersion, &c.Kind, &c.Subject, &c.Rule, &scopeJSON,
			(*string)(&c.Tier), &c.TierProfile,
			(*string)(&c.Status),
			&c.Model, &c.PromptVersion, &c.ReproKey, &c.InputDigest,
			&c.CompiledAtWatermark, &compiledAt, (*string)(&c.Class),
			&validFromText, &validToText, &c.AssertedBy, &metaText,
			(*string)(&c.Lifecycle),
		); err != nil {
			return nil, fmt.Errorf("scanClaims scan: %w", err)
		}
		if err := json.Unmarshal([]byte(scopeJSON), &c.Scope); err != nil {
			return nil, fmt.Errorf("scanClaims unmarshal scope: %w", err)
		}
		c.CompiledAt = time.Unix(0, compiledAt).UTC()
		c.ValidFrom = parseClaimTime(validFromText)
		c.ValidTo = parseClaimTime(validToText)
		if metaText != "" {
			c.Meta = json.RawMessage(metaText)
		}
		result = append(result, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scanClaims rows: %w", err)
	}
	return result, nil
}

// stampClaimValidFrom is GL-16's activation of SU-19's valid_from (design
// doc governance-learning §7): every writer that mints a claim — the
// compiler, a rules-file source, a learned rule — sets CompiledAt but none
// of them ever set ValidFrom, so a compiled claim's window was open to the
// beginning of time and an as_of before its mint still served it. A claim
// whose writer left ValidFrom zero is stamped with its own CompiledAt; a
// writer's explicit ValidFrom wins, and a claim with no compile time at
// all stays zero (unconstrained — the only honest reading of a row that
// predates the column). Both backends stamp here, and both upserts keep
// the FIRST stored value, so a recompile never moves a claim's history.
func stampClaimValidFrom(c core.SemanticClaim) core.SemanticClaim {
	if c.ValidFrom.IsZero() && !c.CompiledAt.IsZero() {
		c.ValidFrom = c.CompiledAt.UTC()
	}
	return c
}

// formatClaimTime renders a claim's ValidFrom/ValidTo (SU-19) as the store's
// nullable-RFC3339-text column: the zero time.Time{} — "unset" — stores as
// "", exactly like every other additive claim column's empty-string
// default; a real time stores as RFC3339Nano (UnixNano-precision round
// trip, matching CompiledAt's own UnixNano storage).
func formatClaimTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// parseClaimTime is formatClaimTime's inverse. An empty or unparseable
// column value reads back as the zero time.Time{} — "unset", never an
// error: a legacy row predating valid_from/valid_to must read exactly like
// a row that was written with an explicitly empty value (core.SemanticClaim.
// ValidAt's own "zero means unconstrained" doc comment depends on this).
func parseClaimTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}
