package deps

import "testing"

// EnableDockerfileCandidateForTest permits the native reading to replay the
// preregistered unmeasured candidate in a serial test. The product remains
// detect-only regardless of reader options.
func EnableDockerfileCandidateForTest(t *testing.T) {
	t.Helper()
	previous := admissions["dockerfile"]
	admissions["dockerfile"] = unmeasured
	t.Cleanup(func() { admissions["dockerfile"] = previous })
}
