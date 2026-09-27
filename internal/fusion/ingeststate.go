package fusion

// ingeststate.go — RT-109 (docs/design/responsible-tenant.md §6): watermark
// ordering. Before this, recordIngestMeta ran before extraction, so an
// ingest killed midway (SIGKILL, jetsam, the exit rung) left a watermark
// claiming HEAD over a half-written index, and nothing re-ingested it. Now
// an ingest reads the watermark it will record when it begins, writes
// ingest_state = in_progress first, and records the watermark and
// ingest_state = complete only at its final commit. A killed or halted
// ingest leaves in_progress: the next process sees a row whose pid is not
// its own and re-ingests the service (internal/mcp's ensure), unless the
// service is quarantined (RT-155).

import (
	"context"
	"os"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// ingestRun is one ingest's bookkeeping: the watermark it will record and
// the ingest_state row it replaced.
type ingestRun struct {
	service string
	target  string
	started int64 // Unix nano
	prev    store.IngestState
	hadPrev bool
}

// ingestTarget is the watermark an ingest of svc records when it
// completes: the git HEAD, or for a non-git corpus the connector's
// content-addressed watermark (ADR-017). Read before the corpus is, so a
// commit that lands during the ingest shows as drift afterwards.
func (p *Pipeline) ingestTarget(ctx context.Context, svc core.ServiceRef) string {
	commit := p.gitHeadFn(svc.Path)
	if commit == "" {
		if wm, err := p.conn.Watermark(ctx, svc); err == nil {
			commit = wm
		}
	}
	return commit
}

// beginIngestState reads the target watermark and writes ingest_state =
// in_progress (best-effort: a store without the table, or a failed write,
// never aborts the ingest — the watermark ordering alone still holds).
func (p *Pipeline) beginIngestState(ctx context.Context, svc core.ServiceRef) ingestRun {
	run := ingestRun{service: svc.ID, target: p.ingestTarget(ctx, svc), started: time.Now().UnixNano()}
	iss, ok := p.st.(store.IngestStateStore)
	if !ok {
		return run
	}
	run.prev, run.hadPrev, _ = iss.GetIngestState(ctx, svc.ID)
	_ = iss.SetIngestState(ctx, store.IngestState{
		ServiceID: svc.ID, State: store.IngestInProgress, PID: os.Getpid(),
		StartedAt: run.started, TargetCommit: run.target,
	})
	return run
}

// abandonIngestState puts back the row an ingest replaced, for an ingest
// refused before its first write (IT-15): the index is untouched, so it is
// still what its watermark says, and a refusal must not make every later
// process re-ingest.
func (p *Pipeline) abandonIngestState(ctx context.Context, run ingestRun) {
	iss, ok := p.st.(store.IngestStateStore)
	if !ok {
		return
	}
	if run.hadPrev {
		_ = iss.SetIngestState(ctx, run.prev)
		return
	}
	_ = iss.DeleteIngestState(ctx, run.service)
}

// commitIngest is an ingest's final commit: the watermark read at its start
// and ingest_state = complete.
func (p *Pipeline) commitIngest(ctx context.Context, svc core.ServiceRef, run ingestRun) {
	p.recordIngestMeta(ctx, svc, run.target)
	if iss, ok := p.st.(store.IngestStateStore); ok {
		_ = iss.SetIngestState(ctx, store.IngestState{
			ServiceID: svc.ID, State: store.IngestComplete, PID: os.Getpid(),
			StartedAt: run.started, TargetCommit: run.target,
		})
	}
}
