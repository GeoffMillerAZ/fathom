package main

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/testisolate"
)

// TestMain runs the package under testisolate (RT-19, §10.4): its tests
// reach the operator's intent ledger by default (internal/intent).
// Its subjects are temporary directories with no commit of their own, so a
// reading's missing subject sha is allowed here (TestCmdLangOracleRequiresSubjectSHA
// turns the refusal back on).
func TestMain(m *testing.M) {
	allowUnpinnedSubject = true
	testisolate.Main(m)
}
