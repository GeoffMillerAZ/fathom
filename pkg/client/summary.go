package client

// summary.go — RT-79 / RT-160 (docs/design/responsible-tenant.md §4.4, §6):
// list_services and the console read a service's summary row, never its
// symbols or its tree. An ingest writes the row at its final commit; a
// store written before the table is backfilled by SQL at first use, under
// the one ingest slot, reading no file. A read that finds the slot busy
// names the absence instead of waiting or falling back to a whole-table
// or whole-tree read.

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// ErrSummaryPending is a named absence: the service has no current
// summary row and it could not be derived now.
var ErrSummaryPending = errors.New("summary pending")

// SummaryAbsentError names why a service's summary is absent (I38): the
// count is unknown, never zero.
type SummaryAbsentError struct {
	Service string
	Reason  string
}

func (e *SummaryAbsentError) Error() string {
	return "summary not yet derived for " + e.Service + ": " + e.Reason
}

func (e *SummaryAbsentError) Is(target error) bool { return target == ErrSummaryPending }

// ServiceSummary returns service's current summary row. A missing or
// stale row (a store from before RT-76, or meta written by an older
// binary) is backfilled once, if the ingest slot is free right now; if it
// is not, the answer is a *SummaryAbsentError, and the next read tries
// again.
func (c *Client) ServiceSummary(ctx context.Context, service string) (store.ServiceSummary, error) {
	sum, ok, err := c.st.ServiceSummary(ctx, service)
	if err != nil {
		return store.ServiceSummary{}, fmt.Errorf("client.ServiceSummary: %w", err)
	}
	wm, wmErr := c.StoreWatermark(ctx, service)
	if wmErr != nil {
		return store.ServiceSummary{}, fmt.Errorf("client.ServiceSummary: %w", wmErr)
	}
	if ok && sum.Watermark == wm {
		return sum, nil
	}
	if wm == "" {
		return store.ServiceSummary{}, &SummaryAbsentError{Service: service, Reason: "the service has no ingest record"}
	}
	release, got := tenancy.TryIngestSlot()
	if !got {
		return store.ServiceSummary{}, &SummaryAbsentError{Service: service, Reason: "an ingest or backfill holds the ingest slot; it is derived at the next read"}
	}
	opts := store.BackfillOptions{Summary: true, Classify: connector.UnsupportedLanguageForPath}
	rep, berr := c.st.BackfillDerivedTables(ctx, service, opts)
	release()
	if berr != nil {
		// The row could not be written (a read-only mount, a busy
		// writer): derive it without writing and serve that — the count at
		// this watermark all the same, derived again at the next read.
		opts.NoWrite = true
		if ro, rerr := c.st.BackfillDerivedTables(ctx, service, opts); rerr == nil && ro.Derived != nil {
			return *ro.Derived, nil
		} else if rerr == nil && ro.IngestInProgress {
			rep = ro
		} else {
			return store.ServiceSummary{}, fmt.Errorf("client.ServiceSummary backfill: %w", berr)
		}
	}
	if rep.IngestInProgress {
		return store.ServiceSummary{}, &SummaryAbsentError{Service: service, Reason: "an ingest of this service is in progress (or was interrupted); it is derived at that ingest's final commit"}
	}
	sum, ok, err = c.st.ServiceSummary(ctx, service)
	if err != nil {
		return store.ServiceSummary{}, fmt.Errorf("client.ServiceSummary: %w", err)
	}
	if !ok {
		return store.ServiceSummary{}, &SummaryAbsentError{Service: service, Reason: "the backfill wrote no row"}
	}
	return sum, nil
}

// BackfillDerived derives service's summary and call index from the rows
// the store holds (RT-160), waiting for the ingest slot under ctx. The
// serving daemon runs it once per service after start-up; it writes
// nothing for a service already derived.
func (c *Client) BackfillDerived(ctx context.Context, service string) (store.BackfillReport, error) {
	release, err := tenancy.AcquireIngestSlot(ctx)
	if err != nil {
		return store.BackfillReport{}, err
	}
	defer release()
	return c.st.BackfillDerivedTables(ctx, service, store.BackfillOptions{Summary: true, Calls: true, Classify: connector.UnsupportedLanguageForPath})
}

// DerivedState reports which derived facts service has.
func (c *Client) DerivedState(ctx context.Context, service string) (store.DerivedState, error) {
	return c.st.DerivedState(ctx, service)
}

// CallersOfKeys is the store's indexed reverse call lookup (RT-77, RT-84):
// every observation, in a call-indexed service, whose calls carry one of
// keys or a key whose final name segment is one of keys — a candidate
// superset the caller narrows. Suppression-filtered.
func (c *Client) CallersOfKeys(ctx context.Context, keys []string) ([]store.CallerRow, error) {
	return c.st.CallersOfKeys(ctx, keys)
}

// SymbolSourcesForNames returns the (package, symbol, source_ref) rows
// declaring any of names in service — a point lookup over the (service_id,
// symbol) index, never a symbol load (RT-84). Suppression-filtered.
func (c *Client) SymbolSourcesForNames(ctx context.Context, service string, names []string) ([]store.SymbolSource, error) {
	return c.st.SymbolSourcesForNames(ctx, service, names)
}

// StoreID names the store this client reads (RT-74): the graph store and
// symbol snapshots key on it, so the MCP tools and the console — which
// share one client in the daemon — share one snapshot and one graph per
// service, and two stores never share an entry.
// The identity is a process-unique sequence number, never an address (a
// freed store's address is reused by the next one).
func (c *Client) StoreID() string {
	c.storeIDOnce.Do(func() { c.storeID = fmt.Sprintf("store-%d", storeIDSeq.Add(1)) })
	return c.storeID
}

var storeIDSeq atomic.Int64

// SuppressionToken names the suppressions currently hiding rows of service
// (the store's scope gate): the graph store and symbol snapshots key on it
// beside the watermark, so a new, removed or expired suppression is seen
// on the next read, as an unmemoized read sees it. "" when none applies
// or the store has no gate.
func (c *Client) SuppressionToken(service string) string {
	if t, ok := c.st.(interface{ SuppressionToken(string) string }); ok {
		return t.SuppressionToken(service)
	}
	return ""
}

// ServiceRefsFor is the store's ingest-extracted routes and outbound HTTP
// client calls for service (RT-89), and whether every readable file has
// been extracted. Suppression-filtered.
func (c *Client) ServiceRefsFor(ctx context.Context, service string) (store.ServiceRefs, bool, error) {
	return c.st.ServiceRefsFor(ctx, service)
}
