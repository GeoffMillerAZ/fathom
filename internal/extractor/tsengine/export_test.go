package tsengine

// SetPoolCapForTest lowers the process pool's cap so a test can drive
// eviction with fewer dialects than the real cap; the returned func restores
// it.
func SetPoolCapForTest(n int) (restore func()) {
	defaultPool.mu.Lock()
	old := defaultPool.max
	defaultPool.max = n
	defaultPool.mu.Unlock()
	return func() {
		defaultPool.mu.Lock()
		defaultPool.max = old
		defaultPool.mu.Unlock()
	}
}

// VariantsForTest is how many guard-pruned variants q holds compiled.
func VariantsForTest(q *Query) int { return len(q.variants) }

// OwnedModulesForTest is how many live modules a Parser still owns: the
// live count less abandoned modules whose reaper has not closed them yet.
func OwnedModulesForTest() int64 { return ownedModules.Load() }
