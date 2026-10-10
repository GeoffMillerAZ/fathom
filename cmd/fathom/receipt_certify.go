// receipt_certify.go — core.ReceiptKindCertification's re-derivation (TC-13,
// vision 33 / ADR-043 §4, docs/plans/test-certifications.md): independent
// of internal/certify's own indexed reach computation, verifyCertificationReceipt
// walks a bare checkout with go/types alone (internal/receiptverify +
// internal/contextbench/publicloc — the same independence contract the
// other three receipt kinds use) to confirm every symbol the certification
// claims Test reaches is genuinely reachable, then — only when runTests is
// true — actually executes `go test -run '^<Test>$' <pkg>` for that one
// test. ADR-043 decision 5: "a reached test is not a passing test"; the
// run verdict is never inferred from reach alone.
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/contextbench/publicloc"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/receiptverify"
)

// certificationOutcome is verifyCertificationReceipt's result. Reach is
// the go/types reach re-derivation, reported exactly like the other three
// kinds' Outcome (OK false names the symbol the receipt claims but
// go/types does not confirm reachable). Pass is the run verdict: always
// "unverified (no run)" unless runTests was given, in which case it is
// "true" or "false" — the literal result of executing Test, never
// inferred.
type certificationOutcome struct {
	Reach receiptverify.Outcome
	Pass  string
}

// verifyCertificationReceipt is TC-13's contract symbol. It re-derives
// reach for a core.ReceiptKindCertification receipt from a bare checkout
// at r.IndexCommit, then — only when runTests is true — runs the one
// named test. A non-nil error means the check could not run at all
// (cmd/fathom's exit-2 shape, matching receiptverify.Verify's own
// contract) — never a false reach or pass verdict.
func verifyCertificationReceipt(ctx context.Context, repoRoot string, r core.Receipt, runTests bool) (certificationOutcome, error) {
	if r.Test == "" {
		return certificationOutcome{}, fmt.Errorf("receipt: certification kind has no test")
	}
	if len(r.Reached) == 0 {
		return certificationOutcome{}, fmt.Errorf("receipt: certification kind names no reached symbols")
	}

	dir, cleanup, err := receiptverify.CheckoutModule(ctx, repoRoot, r.IndexCommit)
	if err != nil {
		return certificationOutcome{}, err
	}
	defer cleanup()

	// EVX-6: offline, like the other three kinds — a module missing from
	// the cache is named and never downloaded.
	truth, err := receiptverify.LoadTruth(ctx, dir, r.IndexCommit)
	if err != nil {
		return certificationOutcome{}, err
	}

	testName := receiptverify.BareSymbolName(r.Test)
	if !declaredAt(truth, testName) {
		return certificationOutcome{Reach: receiptverify.Outcome{
			OK: false, Diff: fmt.Sprintf("%s is not declared at %s", testName, r.IndexCommit),
		}}, nil
	}

	reachable := reachableFrom(forwardEdges(truth), testName)
	var missing []string
	for _, s := range r.Reached {
		if bare := receiptverify.BareSymbolName(s); !reachable[bare] {
			missing = append(missing, s)
		}
	}
	if len(missing) > 0 {
		return certificationOutcome{Reach: receiptverify.Outcome{OK: false, Diff: fmt.Sprintf(
			"%s does not reach %v at %s (re-derived reach: %v)",
			testName, missing, r.IndexCommit, sortedKeys(reachable),
		)}}, nil
	}

	out := certificationOutcome{Reach: receiptverify.Outcome{OK: true}, Pass: "unverified (no run)"}
	if !runTests {
		return out, nil
	}
	pass, runErr := runOneGoTest(ctx, dir, testPackageDir(r.Test), testName)
	if runErr != nil {
		return certificationOutcome{}, runErr
	}
	out.Pass = fmt.Sprintf("%t", pass)
	return out, nil
}

// cmdReceiptVerifyCertification is `fathom receipt verify`'s
// certification-kind branch: prints and exit-codes verifyCertificationReceipt's
// result the same shape as the other three kinds (OK 0 / DIFFERENCE 1 /
// cannot-run 2), plus the run verdict every certification receipt carries,
// and a FAIL line (exit 1) when --run-tests actually ran the test and it
// did not pass.
func cmdReceiptVerifyCertification(repo string, r core.Receipt, runTests bool, stdout, stderr io.Writer) int {
	out, err := verifyCertificationReceipt(context.Background(), repo, r, runTests)
	if err != nil {
		fmt.Fprintf(stderr, "receipt verify: cannot run: %v\n", err)
		return 2
	}
	testName := receiptverify.BareSymbolName(r.Test)
	if !out.Reach.OK {
		fmt.Fprintf(stdout, "receipt verify: DIFFERENCE — %s\n", out.Reach.Diff)
		return 1
	}
	fmt.Fprintf(stdout, "receipt verify: OK — %s reaches %d symbol(s) at %s; pass: %s\n",
		testName, len(r.Reached), r.IndexCommit, out.Pass)
	if runTests && out.Pass != "true" {
		fmt.Fprintf(stdout, "receipt verify: FAIL — %s did not pass at %s\n", testName, r.IndexCommit)
		return 1
	}
	return 0
}

// declaredAt reports whether name is one of truth's declared function/
// method names — the check a CallersByName/UsesByName map miss cannot
// substitute for (a renamed or removed symbol has no entry in either,
// indistinguishable from a real symbol with zero callers unless
// declaration itself is checked first; receiptverify's own verifyCallers/
// verifyReferences make the same check).
func declaredAt(truth publicloc.GoTruth, name string) bool {
	for _, names := range truth.DeclsByFile {
		for _, n := range names {
			if n == name {
				return true
			}
		}
	}
	return false
}

// forwardEdges inverts publicloc.GoTruth.Edges (each a Caller/Callee pair
// by bare name) into a caller -> callees adjacency — the direction reach
// re-derivation needs and GoTruth does not carry directly (CallersByName
// runs the other way, callee -> callers).
func forwardEdges(truth publicloc.GoTruth) map[string][]string {
	forward := make(map[string][]string, len(truth.Edges))
	for _, e := range truth.Edges {
		forward[e.Caller] = append(forward[e.Caller], e.Callee)
	}
	return forward
}

// reachableFrom walks forward breadth-first from start, returning every
// bare name transitively reachable (start itself excluded).
func reachableFrom(forward map[string][]string, start string) map[string]bool {
	seen := map[string]bool{start: true}
	queue := []string{start}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, next := range forward[cur] {
			if !seen[next] {
				seen[next] = true
				queue = append(queue, next)
			}
		}
	}
	delete(seen, start)
	return seen
}

// testPackageDir extracts the module-relative package directory from a
// receipt's Test field ("dir/pkg.TestName", or a bare "TestName" for a
// package at the module root) — the same vocabulary receiptverify.
// BareSymbolName reduces from. A Test function is always package-level,
// never a method, so exactly one dot (if any) separates the directory
// from the function name — unlike BareSymbolName's general case, no
// Recv.Method ambiguity is possible here.
func testPackageDir(test string) string {
	if i := strings.LastIndex(test, "::"); i >= 0 {
		test = test[i+2:]
	}
	if i := strings.LastIndex(test, "."); i >= 0 {
		return test[:i]
	}
	return "."
}

// runOneGoTest runs `go test -run '^<name>$' <pattern>` inside dir (a bare
// checkout) and reports whether it passed. A non-nil error means the
// command itself could not be started at all — never a false pass/fail; a
// compile failure or a failing test both report (false, nil), since both
// are honest "did not pass" outcomes for the one test named.
func runOneGoTest(ctx context.Context, dir, pkgDir, name string) (bool, error) {
	pattern := pkgDir
	if pattern != "." {
		pattern = "./" + pkgDir
	}
	cmd := exec.CommandContext(ctx, "go", "test", "-run", "^"+name+"$", pattern) //nolint:gosec — name/pattern come from a receipt file the operator supplied, the tool's whole purpose
	cmd.Dir = dir
	// EVX-6: the run never downloads a module either.
	cmd.Env = receiptverify.OfflineGoEnv()
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return false, nil
		}
		return false, fmt.Errorf("receipt: go test: %w (%s)", err, out.String())
	}
	return true, nil
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
