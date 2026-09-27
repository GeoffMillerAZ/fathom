// RunArmsConcurrently (RC-16, vision 28 §2 "cost discipline") runs one
// task's arms in parallel: PrepareSubject keys a worktree on (subject,
// base_commit, ARM), and a wired arm's own scratch daemon binds an
// independently allocated port (wireScratchDaemon) — a treatment and
// control cell of the same task already share no worktree, no daemon and
// no port, so nothing guards this concurrency; it is safe by construction.
package scenariobench

import "sync"

// RunArmsConcurrently runs each of fns concurrently and returns their
// results at the SAME index as the function that produced them — never in
// completion order — so a caller that stores or prints them keeps a
// deterministic ledger regardless of which goroutine happens to finish
// first.
func RunArmsConcurrently[T any](fns []func() (T, error)) ([]T, []error) {
	results := make([]T, len(fns))
	errs := make([]error, len(fns))
	var wg sync.WaitGroup
	wg.Add(len(fns))
	for i, fn := range fns {
		//fathom:tenancy-allow bare-go benchmark fan-out joined by a WaitGroup
		go func(i int, fn func() (T, error)) {
			defer wg.Done()
			results[i], errs[i] = fn()
		}(i, fn)
	}
	wg.Wait()
	return results, errs
}
