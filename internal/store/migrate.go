package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// MigrateTableResult is the per-table outcome of a MigrateNonDerivable run:
// how many rows crossed over, and a deterministic content hash. The hash is
// computed from the SOURCE rows and then verified by re-reading the ported
// rows back from the TARGET and comparing — a mismatch fails the whole
// migration, so a printed hash always means "this is what both sides hold".
type MigrateTableResult struct {
	Table    string
	RowCount int
	SpotHash string // sha256 (hex, first 16 chars) over a deterministic JSON encoding of the rows; source==target verified
}

// MigrateReport is the full outcome of a MigrateNonDerivable run.
type MigrateReport struct {
	// Tables holds one entry per non-derivable table, in the fixed order
	// semantic_claims, claim_links, claim_events, compile_runs, service_meta,
	// commits, knowledge_cards, doc_summaries.
	Tables []MigrateTableResult
	// ServiceIDs is the sorted, distinct set of service IDs touched by any
	// migrated row.
	ServiceIDs []string
	// ServicePaths maps a migrated service ID to its recorded corpus path
	// (from service_meta.path; "" when the service had no service_meta row,
	// or the row's path was itself empty — e.g. a git-less ingest).
	ServicePaths map[string]string
	// OrphanLinksSkipped counts claim_links rows whose claim_id has no claim
	// in semantic_claims. Such rows carry service_id '' (PutClaims cannot
	// derive one) and would be unreachable by every ClaimLinks query, so
	// they are skipped, not ported — loudly, via this count.
	OrphanLinksSkipped int
}

// MigrateNonDerivable copies the eight non-derivable tables —
// semantic_claims, claim_links, claim_events, compile_runs, service_meta,
// commits, knowledge_cards, doc_summaries — from a DuckDB store at fromPath
// into a freshly created SQLite store at toPath (ADR-020). knowledge_cards
// and doc_summaries are LLM-derived enrichment that a re-ingest does NOT
// rebuild (enrichment is opt-in and unwired in production; PurgeService
// deliberately retains cards for the same reason), so treating them as
// derivable would silently lose them. Only observations and fingerprints
// are left behind: those genuinely rebuild from source, and the caller
// prints the exact re-ingest command using report.ServiceIDs/ServicePaths.
//
// The source is opened READ-ONLY (DuckDB access_mode=read_only, no schema
// DDL) and left byte-identical. After writing, every ported table is
// re-read from the target and its content hash compared against the
// source's — a mismatch fails the migration rather than printing an
// unverified hash.
//
// Refuses to write into an existing target that already has rows in ANY of
// its tables — a fresh, empty, or absent target is required; nothing here
// ever merges into or overwrites populated data.
func MigrateNonDerivable(ctx context.Context, fromPath, toPath string) (MigrateReport, error) {
	if ext := filepath.Ext(fromPath); ext != ".duckdb" {
		return MigrateReport{}, fmt.Errorf("migrate-store: --from must be a .duckdb store, got %q (ext %q)", fromPath, ext)
	}
	if ext := filepath.Ext(toPath); ext != ".sqlite" {
		return MigrateReport{}, fmt.Errorf("migrate-store: --to must be a .sqlite store, got %q (ext %q)", toPath, ext)
	}

	empty, err := targetIsEmptyOrAbsent(toPath)
	if err != nil {
		return MigrateReport{}, err
	}
	if !empty {
		base := filepath.Base(toPath)
		return MigrateReport{}, fmt.Errorf("migrate-store: target %q already has data — refusing to overwrite (remove it, and any %s-wal/%s-shm siblings, or pick a fresh path)", toPath, base, base)
	}

	from, err := openDuckDBReadOnly(fromPath)
	if err != nil {
		return MigrateReport{}, fmt.Errorf("migrate-store: opening source %q read-only: %w (if the fathom daemon is serving this store, stop it before migrating, then restart it on the new store afterwards)", fromPath, err)
	}
	defer from.Close()

	to, err := NewSQLiteStore(toPath)
	if err != nil {
		return MigrateReport{}, fmt.Errorf("migrate-store: opening target %q: %w", toPath, err)
	}
	defer to.Close()

	report := MigrateReport{ServicePaths: map[string]string{}}
	touched := map[string]struct{}{}

	// semantic_claims + claim_links: PutClaims requires both in one call so
	// its internal claim_id → service_id lookup (built from the claims slice
	// passed alongside) resolves correctly for every link — see PutClaims.
	claims, err := allSemanticClaims(ctx, from.db)
	if err != nil {
		return MigrateReport{}, fmt.Errorf("migrate-store: reading semantic_claims: %w", err)
	}
	links, err := allClaimLinks(ctx, from.db)
	if err != nil {
		return MigrateReport{}, fmt.Errorf("migrate-store: reading claim_links: %w", err)
	}
	// F4c: skip orphan links (a claim_id with no claim in semantic_claims) —
	// ported, they would land with service_id '' and be unreachable by every
	// ClaimLinks query. Filtering happens BEFORE the source hash is taken so
	// the source/target hash comparison below covers exactly the ported set.
	claimIDs := make(map[string]struct{}, len(claims))
	for _, c := range claims {
		claimIDs[c.ClaimID] = struct{}{}
	}
	keptLinks := links[:0]
	for _, l := range links {
		if _, ok := claimIDs[l.ClaimID]; ok {
			keptLinks = append(keptLinks, l)
		} else {
			report.OrphanLinksSkipped++
		}
	}
	links = keptLinks
	if err := to.PutClaims(ctx, claims, links); err != nil {
		return MigrateReport{}, fmt.Errorf("migrate-store: writing semantic_claims/claim_links: %w", err)
	}
	for _, c := range claims {
		touched[c.ServiceID] = struct{}{}
	}
	report.Tables = append(
		report.Tables,
		MigrateTableResult{Table: "semantic_claims", RowCount: len(claims), SpotHash: spotHash(claims)},
		MigrateTableResult{Table: "claim_links", RowCount: len(links), SpotHash: spotHash(links)},
	)

	// claim_events (global — no service_id column).
	events, err := allClaimEvents(ctx, from.db)
	if err != nil {
		return MigrateReport{}, fmt.Errorf("migrate-store: reading claim_events: %w", err)
	}
	for _, ev := range events {
		if err := to.AppendClaimEvent(ctx, ev); err != nil {
			return MigrateReport{}, fmt.Errorf("migrate-store: writing claim_events: %w", err)
		}
	}
	report.Tables = append(report.Tables, MigrateTableResult{Table: "claim_events", RowCount: len(events), SpotHash: spotHash(events)})

	// compile_runs.
	runs, err := allCompileRuns(ctx, from.db)
	if err != nil {
		return MigrateReport{}, fmt.Errorf("migrate-store: reading compile_runs: %w", err)
	}
	for _, r := range runs {
		if err := to.RecordCompileRun(ctx, r); err != nil {
			return MigrateReport{}, fmt.Errorf("migrate-store: writing compile_runs: %w", err)
		}
		touched[r.ServiceID] = struct{}{}
	}
	report.Tables = append(report.Tables, MigrateTableResult{Table: "compile_runs", RowCount: len(runs), SpotHash: spotHash(runs)})

	// service_meta.
	metas, err := allServiceMeta(ctx, from.db)
	if err != nil {
		return MigrateReport{}, fmt.Errorf("migrate-store: reading service_meta: %w", err)
	}
	for _, m := range metas {
		if err := to.SetServiceMeta(ctx, m); err != nil {
			return MigrateReport{}, fmt.Errorf("migrate-store: writing service_meta %q: %w", m.ServiceID, err)
		}
		touched[m.ServiceID] = struct{}{}
		report.ServicePaths[m.ServiceID] = m.Path
	}
	report.Tables = append(report.Tables, MigrateTableResult{Table: "service_meta", RowCount: len(metas), SpotHash: spotHash(metas)})

	// commits — grouped by service_id since SetCommits replaces a whole
	// service's set per call.
	commits, err := allCommits(ctx, from.db)
	if err != nil {
		return MigrateReport{}, fmt.Errorf("migrate-store: reading commits: %w", err)
	}
	byService := make(map[string][]Commit, len(commits))
	for _, c := range commits {
		byService[c.ServiceID] = append(byService[c.ServiceID], c)
		touched[c.ServiceID] = struct{}{}
	}
	svcOrder := make([]string, 0, len(byService))
	for svc := range byService {
		svcOrder = append(svcOrder, svc)
	}
	sort.Strings(svcOrder)
	for _, svc := range svcOrder {
		if err := to.SetCommits(ctx, svc, byService[svc]); err != nil {
			return MigrateReport{}, fmt.Errorf("migrate-store: writing commits for %q: %w", svc, err)
		}
	}
	report.Tables = append(report.Tables, MigrateTableResult{Table: "commits", RowCount: len(commits), SpotHash: spotHash(commits)})

	// knowledge_cards (F3: LLM-derived enrichment — re-ingest does NOT
	// rebuild these; PurgeService deliberately retains them for the same
	// reason. Losing them here would be silent data loss).
	cards, err := allKnowledgeCards(ctx, from.db)
	if err != nil {
		return MigrateReport{}, fmt.Errorf("migrate-store: reading knowledge_cards: %w", err)
	}
	if err := to.PutKnowledgeCards(ctx, cards); err != nil {
		return MigrateReport{}, fmt.Errorf("migrate-store: writing knowledge_cards: %w", err)
	}
	for _, c := range cards {
		touched[c.ServiceID] = struct{}{}
	}
	report.Tables = append(report.Tables, MigrateTableResult{Table: "knowledge_cards", RowCount: len(cards), SpotHash: spotHash(cards)})

	// doc_summaries (F3: same — enrichment, not re-ingest-derivable).
	sums, err := allDocSummaries(ctx, from.db)
	if err != nil {
		return MigrateReport{}, fmt.Errorf("migrate-store: reading doc_summaries: %w", err)
	}
	if err := to.PutDocSummaries(ctx, sums); err != nil {
		return MigrateReport{}, fmt.Errorf("migrate-store: writing doc_summaries: %w", err)
	}
	for _, d := range sums {
		touched[d.ServiceID] = struct{}{}
	}
	report.Tables = append(report.Tables, MigrateTableResult{Table: "doc_summaries", RowCount: len(sums), SpotHash: spotHash(sums)})

	report.ServiceIDs = make([]string, 0, len(touched))
	for svc := range touched {
		report.ServiceIDs = append(report.ServiceIDs, svc)
	}
	sort.Strings(report.ServiceIDs)

	// F4b: verification — re-read every ported table from the TARGET and
	// compare content hashes against the source-side hashes recorded above.
	// A printed source-only hash would be verification theater: it proves
	// what was read, not what landed.
	srcHash := make(map[string]string, len(report.Tables))
	for _, tr := range report.Tables {
		srcHash[tr.Table] = tr.SpotHash
	}
	verify := func(table string, rows any, readErr error) error {
		if readErr != nil {
			return fmt.Errorf("migrate-store: verification re-read of %s from target: %w", table, readErr)
		}
		if h := spotHash(rows); h != srcHash[table] {
			return fmt.Errorf("migrate-store: VERIFICATION FAILED for %s — source hash %s != target hash %s; the target at %q is suspect, do not use it", table, srcHash[table], h, toPath)
		}
		return nil
	}
	tgtClaims, rerr := allSemanticClaims(ctx, to.db)
	if err := verify("semantic_claims", tgtClaims, rerr); err != nil {
		return MigrateReport{}, err
	}
	tgtLinks, rerr := allClaimLinks(ctx, to.db)
	if err := verify("claim_links", tgtLinks, rerr); err != nil {
		return MigrateReport{}, err
	}
	tgtEvents, rerr := allClaimEvents(ctx, to.db)
	if err := verify("claim_events", tgtEvents, rerr); err != nil {
		return MigrateReport{}, err
	}
	tgtRuns, rerr := allCompileRuns(ctx, to.db)
	if err := verify("compile_runs", tgtRuns, rerr); err != nil {
		return MigrateReport{}, err
	}
	tgtMetas, rerr := allServiceMeta(ctx, to.db)
	if err := verify("service_meta", tgtMetas, rerr); err != nil {
		return MigrateReport{}, err
	}
	tgtCommits, rerr := allCommits(ctx, to.db)
	if err := verify("commits", tgtCommits, rerr); err != nil {
		return MigrateReport{}, err
	}
	tgtCards, rerr := allKnowledgeCards(ctx, to.db)
	if err := verify("knowledge_cards", tgtCards, rerr); err != nil {
		return MigrateReport{}, err
	}
	tgtSums, rerr := allDocSummaries(ctx, to.db)
	if err := verify("doc_summaries", tgtSums, rerr); err != nil {
		return MigrateReport{}, err
	}

	return report, nil
}

// targetIsEmptyOrAbsent reports whether path is safe to migrate into: either
// it doesn't exist yet, or it exists but every one of its tables (all ten —
// the eight migrated here plus the two derivable ones, in case a stray
// re-ingest already populated it) has zero rows.
func targetIsEmptyOrAbsent(path string) (bool, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, fmt.Errorf("migrate-store: checking target %q: %w", path, err)
	}
	s, err := NewSQLiteStore(path)
	if err != nil {
		return false, fmt.Errorf("migrate-store: opening existing target %q: %w", path, err)
	}
	defer s.Close()
	for _, table := range []string{
		"observations", "fingerprints", "knowledge_cards", "doc_summaries",
		"service_meta", "commits", "semantic_claims", "claim_links",
		"compile_runs", "claim_events",
	} {
		var n int
		if err := s.db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); err != nil {
			return false, fmt.Errorf("migrate-store: counting %s in existing target %q: %w", table, path, err)
		}
		if n > 0 {
			return false, nil
		}
	}
	return true, nil
}

// spotHash returns a short, deterministic content fingerprint for v (any of
// the plain row-slice types this file reads) — enough for an operator to
// compare two migration runs, or the same store migrated twice, without a
// full row-by-row diff. Row order is fixed by each allX query's ORDER BY, so
// the encoding (and therefore the hash) is stable across runs against
// unchanged data.
func spotHash(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		// None of the row types here carry unmarshalable fields (no chans,
		// funcs, or cycles) — this should be unreachable. Degrade to a
		// clearly-invalid marker rather than panic in an operator-facing tool.
		return "hash-error"
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])[:16]
}

// ─── raw, all-services table scans (source-side reads) ───────────────────
//
// These bypass the Store interface's per-service/per-claim scoping — there
// is no interface method to list "every row regardless of service", and
// adding one for a one-shot migration tool isn't warranted. They live here,
// inside the store package, so they can query a source *DuckStore's
// unexported db field directly.

func allSemanticClaims(ctx context.Context, db *sql.DB) ([]core.SemanticClaim, error) {
	const cols = `service_id, claim_id, layer, layer_version, kind, subject, rule, scope,
	tier, tier_profile, status, model, prompt_version, repro_key, input_digest,
	compiled_at_watermark, compiled_at`
	rows, err := db.QueryContext(ctx, `SELECT `+cols+` FROM semantic_claims ORDER BY service_id, claim_id`)
	if err != nil {
		return nil, fmt.Errorf("allSemanticClaims query: %w", err)
	}
	defer rows.Close()
	return scanClaims(rows)
}

func allClaimLinks(ctx context.Context, db *sql.DB) ([]core.ClaimLink, error) {
	// service_id is not part of core.ClaimLink (claim_id is content-addressed
	// and globally unique — see claims.go), so it is read only to order the
	// scan deterministically and then discarded; PutClaims re-derives it on
	// write from the claims passed alongside in the same call.
	rows, err := db.QueryContext(ctx, `
SELECT claim_id, rel, target_ref, target_hash
FROM claim_links
ORDER BY service_id, claim_id, rel, target_ref
`)
	if err != nil {
		return nil, fmt.Errorf("allClaimLinks query: %w", err)
	}
	defer rows.Close()
	var result []core.ClaimLink
	for rows.Next() {
		var l core.ClaimLink
		if err := rows.Scan(&l.ClaimID, &l.Rel, &l.TargetRef, &l.TargetHash); err != nil {
			return nil, fmt.Errorf("allClaimLinks scan: %w", err)
		}
		result = append(result, l)
	}
	return result, rows.Err()
}

func allClaimEvents(ctx context.Context, db *sql.DB) ([]core.ClaimEvent, error) {
	rows, err := db.QueryContext(ctx, `
SELECT claim_id, verdict, author, note, event_at
FROM claim_events
ORDER BY claim_id, event_at, verdict
`)
	if err != nil {
		return nil, fmt.Errorf("allClaimEvents query: %w", err)
	}
	defer rows.Close()
	var result []core.ClaimEvent
	for rows.Next() {
		var (
			ev core.ClaimEvent
			at int64
		)
		if err := rows.Scan(&ev.ClaimID, &ev.Verdict, &ev.Author, &ev.Note, &at); err != nil {
			return nil, fmt.Errorf("allClaimEvents scan: %w", err)
		}
		ev.At = time.Unix(0, at).UTC()
		result = append(result, ev)
	}
	return result, rows.Err()
}

func allCompileRuns(ctx context.Context, db *sql.DB) ([]core.CompileRun, error) {
	const cols = `run_id, service_id, layer, corpus_watermark, model, prompt_version,
	dry_run, tokens_in, tokens_out, cost_usd, duration_ms,
	sections_total, sections_compiled, sections_failed, claims_emitted, claims_rejected, ran_at,
	refused, refusal_reason, budget_stopped`
	rows, err := db.QueryContext(ctx, `SELECT `+cols+` FROM compile_runs ORDER BY service_id, ran_at DESC, run_id`)
	if err != nil {
		return nil, fmt.Errorf("allCompileRuns query: %w", err)
	}
	defer rows.Close()
	var result []core.CompileRun
	for rows.Next() {
		var (
			r     core.CompileRun
			ranAt int64
		)
		if err := rows.Scan(
			&r.RunID, &r.ServiceID, &r.Layer, &r.CorpusWatermark,
			&r.Model, &r.PromptVersion, &r.DryRun,
			&r.TokensIn, &r.TokensOut, &r.CostUSD, &r.DurationMS,
			&r.SectionsTotal, &r.SectionsCompiled, &r.SectionsFailed,
			&r.ClaimsEmitted, &r.ClaimsRejected, &ranAt,
			&r.Refused, &r.RefusalReason, &r.BudgetStopped,
		); err != nil {
			return nil, fmt.Errorf("allCompileRuns scan: %w", err)
		}
		r.RanAt = time.Unix(0, ranAt).UTC()
		result = append(result, r)
	}
	return result, rows.Err()
}

func allServiceMeta(ctx context.Context, db *sql.DB) ([]ServiceMeta, error) {
	rows, err := db.QueryContext(ctx, `
SELECT service_id, path, indexed_commit, indexed_at
FROM service_meta
ORDER BY service_id
`)
	if err != nil {
		return nil, fmt.Errorf("allServiceMeta query: %w", err)
	}
	defer rows.Close()
	var result []ServiceMeta
	for rows.Next() {
		var m ServiceMeta
		if err := rows.Scan(&m.ServiceID, &m.Path, &m.IndexedCommit, &m.IndexedAt); err != nil {
			return nil, fmt.Errorf("allServiceMeta scan: %w", err)
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

func allKnowledgeCards(ctx context.Context, db *sql.DB) ([]KnowledgeCard, error) {
	rows, err := db.QueryContext(ctx, `
SELECT service_id, symbol, file, summary, tags
FROM knowledge_cards
ORDER BY service_id, symbol
`)
	if err != nil {
		return nil, fmt.Errorf("allKnowledgeCards query: %w", err)
	}
	defer rows.Close()
	var result []KnowledgeCard
	for rows.Next() {
		var c KnowledgeCard
		var tagsJSON string
		if err := rows.Scan(&c.ServiceID, &c.Symbol, &c.File, &c.Summary, &tagsJSON); err != nil {
			return nil, fmt.Errorf("allKnowledgeCards scan: %w", err)
		}
		if err := json.Unmarshal([]byte(tagsJSON), &c.Tags); err != nil {
			return nil, fmt.Errorf("allKnowledgeCards unmarshal tags: %w", err)
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

func allDocSummaries(ctx context.Context, db *sql.DB) ([]DocSummary, error) {
	rows, err := db.QueryContext(ctx, `
SELECT service_id, source_ref, title, summary
FROM doc_summaries
ORDER BY service_id, source_ref
`)
	if err != nil {
		return nil, fmt.Errorf("allDocSummaries query: %w", err)
	}
	defer rows.Close()
	var result []DocSummary
	for rows.Next() {
		var d DocSummary
		if err := rows.Scan(&d.ServiceID, &d.SourceRef, &d.Title, &d.Summary); err != nil {
			return nil, fmt.Errorf("allDocSummaries scan: %w", err)
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

func allCommits(ctx context.Context, db *sql.DB) ([]Commit, error) {
	rows, err := db.QueryContext(ctx, `
SELECT service_id, hash, author, date, message, files, spec_ids
FROM commits
ORDER BY service_id, date DESC, hash
`)
	if err != nil {
		return nil, fmt.Errorf("allCommits query: %w", err)
	}
	defer rows.Close()
	var result []Commit
	for rows.Next() {
		var c Commit
		var filesJSON, specJSON string
		if err := rows.Scan(&c.ServiceID, &c.Hash, &c.Author, &c.Date, &c.Message, &filesJSON, &specJSON); err != nil {
			return nil, fmt.Errorf("allCommits scan: %w", err)
		}
		_ = json.Unmarshal([]byte(filesJSON), &c.Files)
		_ = json.Unmarshal([]byte(specJSON), &c.SpecIDs)
		result = append(result, c)
	}
	return result, rows.Err()
}
