package store

// doclinksync.go — one link pass's doc_links reconciliation (EW-50, wave
// ic; O362). SyncDocLinksForUnit reads the unit's open links with
// `WHERE service_id = ? AND unit_id = ? AND valid_to = 0`, which the only
// doc_links index (service_id, target) answers by scanning every link of
// the service: a pass over a service's units cost units times links. A
// DocLinkSync reads the service's open links once, at its first unit, and
// keeps that read current as it syncs each unit; the per-unit close and
// insert are SyncDocLinksForUnit's (syncDocLinksTx). It lives one pass and
// assumes it is the service's only doc_links writer meanwhile, as the
// ingest is.

import (
	"context"
	"fmt"
)

// DocLinkSync reconciles units' doc_links for one service over one pass.
type DocLinkSync struct {
	s         *SQLiteStore
	serviceID string
	open      map[string]map[docLinkKey]bool // unit -> its open links; nil until the first SyncUnit
	reads     int
}

// NewDocLinkSync starts a pass's reconciliation for serviceID.
func (s *SQLiteStore) NewDocLinkSync(serviceID string) *DocLinkSync {
	return &DocLinkSync{s: s, serviceID: serviceID}
}

// OpenReads is how many times this pass read the service's open links: one
// once a unit was synced (the linearity test's counter).
func (d *DocLinkSync) OpenReads() int { return d.reads }

// SyncUnit is SyncDocLinksForUnit(ctx, serviceID, unitID, fresh, now) with
// the unit's open links taken from the pass's one read.
func (d *DocLinkSync) SyncUnit(ctx context.Context, unitID string, fresh []DocLink, now int64) error {
	if d.open == nil {
		if err := d.load(ctx); err != nil {
			return err
		}
	}
	tx, err := d.s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("SyncDocLinksForUnit begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	open := d.open[unitID]
	if open == nil {
		open = map[docLinkKey]bool{}
	}
	if err := syncDocLinksTx(ctx, tx, d.serviceID, unitID, open, fresh, now); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	now2 := make(map[docLinkKey]bool, len(fresh))
	for _, f := range fresh {
		now2[docLinkKey{f.TargetKind, f.Target, f.Line}] = true
	}
	d.open[unitID] = now2
	return nil
}

// load reads every open doc_links row of the service, by unit.
func (d *DocLinkSync) load(ctx context.Context) error {
	d.reads++
	rows, err := d.s.db.QueryContext(ctx, `
SELECT unit_id, target_kind, target, line FROM doc_links
WHERE service_id = ? AND valid_to = 0`, d.serviceID)
	if err != nil {
		return fmt.Errorf("DocLinkSync open links: %w", err)
	}
	defer rows.Close()
	open := map[string]map[docLinkKey]bool{}
	for rows.Next() {
		var unit string
		var k docLinkKey
		if err := rows.Scan(&unit, &k.kind, &k.target, &k.line); err != nil {
			return fmt.Errorf("DocLinkSync scan open links: %w", err)
		}
		if open[unit] == nil {
			open[unit] = map[docLinkKey]bool{}
		}
		open[unit][k] = true
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("DocLinkSync open links rows: %w", err)
	}
	d.open = open
	return nil
}
