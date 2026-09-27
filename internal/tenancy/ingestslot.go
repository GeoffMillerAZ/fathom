package tenancy

// ingestslot.go — the one process-wide heavy slot (design §3.5, §6, §7.1):
// the inline auto-refresh ingest (RT-149) and the derived-table backfill
// (RT-160) hold it, so an ingest and a backfill, or two of either, never
// run at once in one daemon. Wave D's refresh queue heavy slot replaces
// the inline ingest (RT-99); the backfill keeps this slot.

import "context"

var ingestSlot = make(chan struct{}, 1)

// AcquireIngestSlot waits for the slot or ctx; release is never nil.
func AcquireIngestSlot(ctx context.Context) (release func(), err error) {
	select {
	case ingestSlot <- struct{}{}:
		return func() { <-ingestSlot }, nil
	case <-ctx.Done():
		return func() {}, ctx.Err()
	}
}

// TryIngestSlot takes the slot only if it is free now: a read path that
// would otherwise wait on an ingest names the absence instead.
func TryIngestSlot() (release func(), ok bool) {
	select {
	case ingestSlot <- struct{}{}:
		return func() { <-ingestSlot }, true
	default:
		return func() {}, false
	}
}
