package store

// sqlite_knowledge.go — SQLiteStore's knowledge cards and doc summaries.
// Split out of sqlite.go (wave g1).

import (
	"context"
	"encoding/json"
	"fmt"
)

// PutKnowledgeCards upserts a batch of knowledge-card rows. Existing rows for
// the same (service_id, symbol) are replaced.
func (s *SQLiteStore) PutKnowledgeCards(ctx context.Context, cards []KnowledgeCard) error {
	if len(cards) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("PutKnowledgeCards begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO knowledge_cards (service_id, symbol, file, summary, tags)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (service_id, symbol) DO UPDATE SET
    file    = excluded.file,
    summary = excluded.summary,
    tags    = excluded.tags
`)
	if err != nil {
		return fmt.Errorf("PutKnowledgeCards prepare: %w", err)
	}
	defer stmt.Close()

	for _, c := range cards {
		tagsJSON, err := json.Marshal(c.Tags)
		if err != nil {
			return fmt.Errorf("PutKnowledgeCards marshal tags: %w", err)
		}
		if _, err = stmt.ExecContext(ctx, c.ServiceID, c.Symbol, c.File, c.Summary, string(tagsJSON)); err != nil {
			return fmt.Errorf("PutKnowledgeCards exec: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("PutKnowledgeCards commit: %w", err)
	}
	return nil
}

// KnowledgeCards returns all knowledge cards for serviceID ordered by symbol.
func (s *SQLiteStore) knowledgeCardsRaw(ctx context.Context, serviceID string) ([]KnowledgeCard, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT service_id, symbol, file, summary, tags
FROM knowledge_cards
WHERE service_id = ?
ORDER BY symbol
`, serviceID)
	if err != nil {
		return nil, fmt.Errorf("KnowledgeCards query: %w", err)
	}
	defer rows.Close()

	var result []KnowledgeCard
	for rows.Next() {
		var c KnowledgeCard
		var tagsJSON string
		if err := rows.Scan(&c.ServiceID, &c.Symbol, &c.File, &c.Summary, &tagsJSON); err != nil {
			return nil, fmt.Errorf("KnowledgeCards scan: %w", err)
		}
		if err := json.Unmarshal([]byte(tagsJSON), &c.Tags); err != nil {
			return nil, fmt.Errorf("KnowledgeCards unmarshal tags: %w", err)
		}
		result = append(result, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("KnowledgeCards rows: %w", err)
	}
	return result, nil
}

// PutDocSummaries upserts a batch of doc-summary rows. Existing rows for the
// same (service_id, source_ref) are replaced.
func (s *SQLiteStore) PutDocSummaries(ctx context.Context, summaries []DocSummary) error {
	if len(summaries) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("PutDocSummaries begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO doc_summaries (service_id, source_ref, title, summary)
VALUES (?, ?, ?, ?)
ON CONFLICT (service_id, source_ref) DO UPDATE SET
    title   = excluded.title,
    summary = excluded.summary
`)
	if err != nil {
		return fmt.Errorf("PutDocSummaries prepare: %w", err)
	}
	defer stmt.Close()

	for _, d := range summaries {
		if _, err = stmt.ExecContext(ctx, d.ServiceID, d.SourceRef, d.Title, d.Summary); err != nil {
			return fmt.Errorf("PutDocSummaries exec: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("PutDocSummaries commit: %w", err)
	}
	return nil
}

// DocSummaries returns all doc summaries for serviceID ordered by source_ref.
func (s *SQLiteStore) docSummariesRaw(ctx context.Context, serviceID string) ([]DocSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT service_id, source_ref, title, summary
FROM doc_summaries
WHERE service_id = ?
ORDER BY source_ref
`, serviceID)
	if err != nil {
		return nil, fmt.Errorf("DocSummaries query: %w", err)
	}
	defer rows.Close()

	var result []DocSummary
	for rows.Next() {
		var d DocSummary
		if err := rows.Scan(&d.ServiceID, &d.SourceRef, &d.Title, &d.Summary); err != nil {
			return nil, fmt.Errorf("DocSummaries scan: %w", err)
		}
		result = append(result, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("DocSummaries rows: %w", err)
	}
	return result, nil
}
