package readout

import (
	"reflect"
	"strings"
	"testing"
)

// claimFixtureMatcher stands in for TR-1's matchDeclaredTestCommand bound to
// a repository that declares `task test`, `task validate` and the `go test`
// runner: it returns the declared invocation a command starts with.
func claimFixtureMatcher(command string) (string, bool) {
	for _, inv := range []string{"task test", "task validate", "go test"} {
		if command == inv || strings.HasPrefix(command, inv+" ") {
			return inv, true
		}
	}
	return "", false
}

type claimWant struct {
	id, invocation string
}

func claimsOf(msg string, m claimCommandMatcher) []claimWant {
	var out []claimWant
	for _, c := range claimsInMessage(msg, m) {
		out = append(out, claimWant{c.ID, c.Invocation})
	}
	return out
}

func TestClaimsInMessageMatchesTheClosedTableAndSkipsNegations(t *testing.T) {
	matches := []struct {
		msg  string
		want []claimWant
	}{
		{"All tests pass.", []claimWant{{claimAllTestsPass, ""}}},
		{"All 412 tests passed after the fix.", []claimWant{{claimAllTestsPass, ""}}},
		{"The tests all pass now.", []claimWant{{claimAllTestsPass, ""}}},
		{"Every new test passes.", []claimWant{{claimAllTestsPass, ""}}},
		{"Tests pass.", []claimWant{{claimTestsPass, ""}}},
		{"I reran it and the new test passes now.", []claimWant{{claimTestsPass, ""}}},
		{"Unit tests are green.", []claimWant{{claimTestsPass, ""}}},
		{"**Tests:** pass", []claimWant{{claimTestsPass, ""}}},
		{"The test suite has passed.", []claimWant{{claimTestsPass, ""}}},
		{"The gate passes.", []claimWant{{claimGatePasses, ""}}},
		{"The pre-push gate passed.", []claimWant{{claimGatePasses, ""}}},
		{"Gate: green", []claimWant{{claimGatePasses, ""}}},
		{"`go test ./internal/readout/...` passes.", []claimWant{{claimCommandPasses, "go test"}}},
		{"Verified: `task validate` green", []claimWant{{claimCommandPasses, "task validate"}}},
		{"`task test` succeeded on the second run.", []claimWant{{claimCommandPasses, "task test"}}},
		{"All green.", []claimWant{{claimGreen, ""}}},
		{"Everything is green.", []claimWant{{claimGreen, ""}}},
		{"Reran the suite, all green.", []claimWant{{claimGreen, ""}}},
		{"Tests pass. The gate passes too.", []claimWant{{claimTestsPass, ""}, {claimGatePasses, ""}}},
		{"I could not reproduce the flake, but all tests pass.", []claimWant{{claimAllTestsPass, ""}}},
		{"Fixed the parser; `go test ./...` passes and the gate is green.",
			[]claimWant{{claimCommandPasses, "go test"}, {claimGatePasses, ""}}},
	}
	for _, tc := range matches {
		if got := claimsOf(tc.msg, claimFixtureMatcher); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("claimsInMessage(%q) = %v, want %v", tc.msg, got, tc.want)
		}
	}

	none := []string{
		// negations
		"I did not run the tests.",
		"Not all tests pass.",
		"The tests don't pass yet.",
		"The tests don’t pass yet.",
		"No tests pass on this branch.",
		"The tests never passed here.",
		"I can't say the tests pass.",
		"I haven't checked whether the tests pass.",
		"`go test ./...` didn't pass.",
		// hedges, conditions and purposes
		"The tests should pass.",
		"Tests should pass once the fix lands.",
		"`go test ./...` should pass now.",
		"Hopefully all tests pass.",
		"Tests pass, I think.",
		"Tests pass unless you set -race.",
		"If the gate passes, merge it.",
		"Once the tests pass, push.",
		"Run `task test` to make sure all tests pass.",
		"Do the tests pass?",
		"I'll confirm the gate passes before pushing.",
		// not a claim about a declared test command, or not a test claim at all
		"`ls -la` passes.",
		"The build passes.",
		"CI tests pass.",
		"The badges are all green.",
		"```\nok  all tests pass\n```",
		"> The user said all tests pass.",
	}
	for _, msg := range none {
		if got := claimsInMessage(msg, claimFixtureMatcher); len(got) != 0 {
			t.Errorf("claimsInMessage(%q) = %+v, want no claim", msg, got)
		}
	}

	// Without the declared commands a named command is never a claim, and
	// the prose claims still match.
	if got := claimsOf("`go test ./...` passes and all tests pass.", nil); !reflect.DeepEqual(got, []claimWant{{claimAllTestsPass, ""}}) {
		t.Errorf("nil matcher: got %v", got)
	}

	// Offsets index the original message, and the answer is deterministic.
	msg := "Done. `task validate` passes; ALL TESTS PASS."
	first := claimsInMessage(msg, claimFixtureMatcher)
	if len(first) != 2 || msg[first[0].Start:first[0].End] != "`task validate` passes" || msg[first[1].Start:first[1].End] != "ALL TESTS PASS" {
		t.Fatalf("spans: %+v", first)
	}
	for i := 0; i < 5; i++ {
		if again := claimsInMessage(msg, claimFixtureMatcher); !reflect.DeepEqual(again, first) {
			t.Fatalf("run %d differs: %+v vs %+v", i, again, first)
		}
	}

	// The table is closed: every id appears once, in a fixed order.
	var ids []string
	for _, e := range claimTable {
		ids = append(ids, e.id)
	}
	if want := []string{claimAllTestsPass, claimGatePasses, claimCommandPasses, claimTestsPass, claimGreen}; !reflect.DeepEqual(ids, want) {
		t.Errorf("claim table ids = %v, want %v", ids, want)
	}
}
