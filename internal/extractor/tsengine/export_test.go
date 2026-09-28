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
