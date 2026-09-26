package main

// selfrestart.go — the daemon adopts a new binary on its own.
//
// Owner ask, 2026-08-12: *"I want to make it so you're empowered to do this
// sort of stuff in the future. I shouldn't be needed and let my projects
// suffer if I'm afk."*
//
// The blocker was narrow and specific. An agent can build, sign and install
// a binary; it cannot run `launchctl kickstart` (that wedges the session —
// a standing constraint in this program). So every committed daemon fix
// waited for a human, and this session proved the cost: a measurement bug
// reached main, was fixed within the hour, and then sat undeployed while
// three counterfactual sessions collected nothing. The fix existed. The
// daemon kept running the old code because one command needed hands.
//
// launchd relaunches this job on a non-zero exit (KeepAlive true under
// plist schema 1, {SuccessfulExit: false} under schema 2), so the daemon
// exiting 75 IS a restart. It therefore watches its own executable and,
// when the bytes on disk change, verifies the replacement and steps aside.
//
// Four safety properties, because a self-restarting daemon that gets this
// wrong is a crash loop that takes the whole fleet's tooling down:
//
//  1. VERIFY BEFORE YIELDING. The candidate must execute and report a
//     parseable version. A binary that cannot run is ignored and NAMED —
//     exactly the failure of 2026-08-12 (an unsigned copy,
//     OS_REASON_CODESIGNING), where exiting would have produced a restart
//     loop against a binary that could never start.
//  2. RATE LIMIT. At most one adoption per cooldown, so a flapping file
//     cannot become a restart storm.
//  3. SETTLE. The bytes must be unchanged across two consecutive polls, so
//     a half-written file is never adopted mid-copy.
//  4. LOUD. Every adoption and every refusal is logged with its reason. A
//     daemon that restarts itself silently is indistinguishable from one
//     that is crashing.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/buildinfo"
	"github.com/geoffmilleraz/fathom/internal/dlog"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

const (
	// selfRestartPoll is deliberately unhurried: a deploy that takes effect
	// within a minute is indistinguishable from instant to every consumer,
	// and the poll costs one hash of an already-cached file.
	selfRestartPoll = 20 * time.Second
	// selfRestartCooldown bounds how often the daemon may step aside.
	selfRestartCooldown = 2 * time.Minute
)

// selfRestartDisabled lets an operator switch the behaviour off without a
// rebuild. Off is a legitimate choice — some deployments want the binary
// pinned until a human says otherwise — so it is honoured and announced,
// never silently ignored.
func selfRestartDisabled() bool {
	return strings.EqualFold(os.Getenv("FATHOM_SELF_RESTART"), "off")
}

// selfRestartBuildStamp returns this process's own build stamp. A package
// var (mirroring doctor.go's doctorBinaryRevision seam) so tests can
// simulate a known build time instead of depending on the go test binary's
// own — often absent — VCS build-time stamp.
var selfRestartBuildStamp = buildinfo.Read

// selfRestartNow is the injected wall clock the adoption line's "built ...
// ago" figure is computed against (internal/CLAUDE.md's determinism
// contract: operational logs still inject `now` so assertions on emitted
// lines are not flaky). Production leaves this as time.Now; tests swap it.
var selfRestartNow = time.Now

// watchOwnBinary polls the daemon's own executable and exits when a
// verified newer binary is in place.
//
// exit is injected so the loop is testable without ending the test
// process; production passes os.Exit.
func watchOwnBinary(path string, log *dlog.Logger, poll, cooldown time.Duration, exit func(int)) {
	start, err := hashFile(path)
	if err != nil {
		// No baseline means every later comparison is a guess. Say so once
		// and stop, rather than restarting on noise.
		log.Warn(fmt.Sprintf("self-restart: disabled — cannot hash %s (%v); a deploy will need a manual restart", path, err))
		return
	}
	log.Info(fmt.Sprintf("self-restart: watching %s (%s); a verified new binary is adopted within ~%s",
		path, shortHash(start), poll*2))

	var (
		pending     string
		pendingSeen int
		last        time.Time
	)
	for {
		time.Sleep(poll)

		cur, herr := hashFile(path)
		if herr != nil || cur == start {
			pending, pendingSeen = "", 0
			continue
		}

		// (3) Settle: the same new bytes twice before acting, so a file
		// still being written is never adopted half-copied.
		if cur != pending {
			pending, pendingSeen = cur, 1
			continue
		}
		pendingSeen++
		if pendingSeen < 2 {
			continue
		}

		// (2) Rate limit.
		if !last.IsZero() && time.Since(last) < cooldown {
			continue
		}

		// (1) Verify before yielding.
		ver, verr := binaryReports(path)
		if verr != nil {
			log.Warn(fmt.Sprintf("self-restart: REFUSED — %s changed but will not run (%v). "+
				"Keeping this process, so the fleet stays served by working code. "+
				"An unsigned binary fails exactly this way; `task deploy` signs.", path, verr))
			// start is deliberately NOT updated: if the operator repairs
			// the binary in place, the next poll sees a new hash and
			// re-verifies rather than treating the bad one as the baseline.
			pending, pendingSeen = "", 0
			last = time.Now()
			continue
		}

		// Exit 75, never 0: a schema-2 plist (KeepAlive {SuccessfulExit:
		// false}) reads 0 as "stay down", and tenancyContract >= 1 — which
		// `daemon install --reconcile` gates on — promises this (§8.2).
		log.Info(fmt.Sprintf("self-restart: adopting %s (%s → %s)%s — exiting %d so launchd restarts on the new code",
			ver, shortHash(start), shortHash(cur), buildAgeSuffix(), tenancy.ExitRetryLater))
		exit(tenancy.ExitRetryLater)
		return
	}
}

// buildAgeSuffix names how long this process's own binary has been running,
// as measured from its embedded build timestamp (internal/buildinfo's
// vcs.time stamp) to now — DG-4: the "adopting" line already named the old
// and new hash but never how long the outgoing binary had been serving.
// Returns "" (no suffix at all) when the running binary carries no build
// timestamp (e.g. built without -buildvcs) or the stamp fails to parse —
// silently omitted rather than a fabricated or zero-valued age.
func buildAgeSuffix() string {
	builtAt := selfRestartBuildStamp().BuiltAt
	if builtAt == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, builtAt)
	if err != nil {
		return ""
	}
	age := selfRestartNow().Sub(t)
	if age < 0 {
		return ""
	}
	return fmt.Sprintf(", built %s ago", age.Round(time.Second))
}

// binaryReports runs the candidate's own `version` and requires a
// non-empty answer. This is what separates "a new binary is present" from
// "a new binary WORKS" — the distinction that turns a self-restart into a
// crash loop when it is skipped.
func binaryReports(path string) (string, error) {
	cmd := exec.Command(path, "version")
	done := make(chan struct{})
	var (
		outBytes []byte
		runErr   error
	)
	go func() {
		outBytes, runErr = cmd.Output()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		// A hung candidate must not hang the watcher.
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return "", fmt.Errorf("timed out reporting its version")
	}
	if runErr != nil {
		return "", runErr
	}
	v := strings.TrimSpace(string(outBytes))
	if v == "" {
		return "", fmt.Errorf("reported an empty version")
	}
	return firstLine(v), nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func shortHash(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
