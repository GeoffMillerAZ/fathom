package deps

import "testing"

// TSJSUseCandidateAdmissionForTest replays the preregistered trusted
// candidate under the test binary, independently of shipped admission.
func TSJSUseCandidateAdmissionForTest(t testing.TB) {
	t.Helper()
	js, ts := admissions["javascript"], admissions["typescript"]
	delete(admissions, "javascript")
	delete(admissions, "typescript")
	t.Cleanup(func() {
		admissions["javascript"], admissions["typescript"] = js, ts
	})
}
