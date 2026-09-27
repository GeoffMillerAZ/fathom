package console

// jobs.go — the console's one bounded background pool (RT-117,
// docs/design/responsible-tenant.md §14). Before it, each page that
// computed in the background brought its own concurrency: the plans worker
// its two goroutines, /refactors a goroutine per range (`go
// computeRefactorReport`, its cache wiped at 64 entries). Now plans and
// conserve submit here: Workers at once, Queue waiting, one flight per key
// — a key already queued or running is joined, never run twice — and a
// queued job whose group has since asked for a newer key is dropped before
// it runs (a plan whose content moved twice needs only the newest
// verification). A full queue is refused by name (ErrJobsFull); the page
// then says the work is not yet computed, never that it is done. Workers
// start on demand and exit when the queue drains: an idle console holds no
// goroutine here.

import (
	"errors"
	"fmt"
	"sync"
)

// ErrJobsFull is the refusal when the pool's queue is full.
var ErrJobsFull = errors.New("console: background queue full — the work was not started; retry shortly")

// Job is one submitted unit of background work.
type Job struct {
	Group, Key string
	fn         func()
	done       chan struct{}
	dropped    bool
	err        error
}

// Done is closed when the job has run, failed or was dropped.
func (j *Job) Done() <-chan struct{} { return j.done }

// Dropped reports whether a newer key of the job's group superseded it
// before it ran. Meaningful once Done is closed.
func (j *Job) Dropped() bool { return j.dropped }

// Err is the job's panic, if it had one. Meaningful once Done is closed.
func (j *Job) Err() error { return j.err }

// Jobs is the bounded pool.
type Jobs struct {
	workers, max int

	mu      sync.Mutex
	queue   []*Job
	flights map[string]*Job // key → the job queued or running for it
	running int
	active  int // worker goroutines alive
}

// JobStats is the pool's instantaneous state.
type JobStats struct {
	Workers, Running, Queued int
}

// NewJobs returns a pool running at most workers jobs with at most queue
// waiting (design §14: 2 and 32).
func NewJobs(workers, queue int) *Jobs {
	if workers < 1 {
		workers = 1
	}
	if queue < 0 {
		queue = 0
	}
	return &Jobs{workers: workers, max: queue, flights: map[string]*Job{}}
}

// SubmitJob queues fn under key, in group. A key already queued or running
// returns that job (single flight); a queued job of the same group with
// another key is dropped (superseded); a full queue is ErrJobsFull.
func (j *Jobs) SubmitJob(group, key string, fn func()) (*Job, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if cur := j.flights[key]; cur != nil {
		return cur, nil
	}
	kept := j.queue[:0]
	for _, q := range j.queue {
		if group != "" && q.Group == group {
			q.dropped = true
			delete(j.flights, q.Key)
			close(q.done)
			continue
		}
		kept = append(kept, q)
	}
	for i := len(kept); i < len(j.queue); i++ {
		j.queue[i] = nil
	}
	j.queue = kept
	if len(j.queue) >= j.max {
		return nil, ErrJobsFull
	}
	job := &Job{Group: group, Key: key, fn: fn, done: make(chan struct{})}
	j.flights[key] = job
	j.queue = append(j.queue, job)
	if j.active < j.workers {
		j.active++
		go j.work()
	}
	return job, nil
}

// Stats reports the pool's state.
func (j *Jobs) Stats() JobStats {
	j.mu.Lock()
	defer j.mu.Unlock()
	return JobStats{Workers: j.active, Running: j.running, Queued: len(j.queue)}
}

// work runs queued jobs until the queue is empty, then exits.
func (j *Jobs) work() {
	for {
		j.mu.Lock()
		if len(j.queue) == 0 {
			j.active--
			j.mu.Unlock()
			return
		}
		job := j.queue[0]
		j.queue[0] = nil
		j.queue = j.queue[1:]
		j.running++
		j.mu.Unlock()

		job.err = runJob(job.fn)

		j.mu.Lock()
		j.running--
		if j.flights[job.Key] == job {
			delete(j.flights, job.Key)
		}
		close(job.done)
		j.mu.Unlock()
	}
}

// runJob runs fn, turning a panic into an error: a failed job never takes
// its worker (or the daemon) with it.
func runJob(fn func()) (err error) {
	defer func() {
		if v := recover(); v != nil {
			err = fmt.Errorf("console job panicked: %v", v)
		}
	}()
	fn()
	return nil
}
