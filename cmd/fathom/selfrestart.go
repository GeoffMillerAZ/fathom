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
	// and the poll costs one stat (RT-12: the binary is hashed only when its
	// (size, mtime, inode) changed).
	selfRestartPoll = 20 * time.Second
	// selfRestartCooldown bounds how often the daemon may step aside,
	// across processes: the last adoption is read from the lifecycle ledger
	// (RT-12, design §8.2), so a relaunched process does not reset it.
	selfRestartCooldown = 5 * time.Minute
)

// adoptionExitCode returns the code the daemon exits with when it adopts a new
// binary (RT-11, design §8.2): 75 (EX_TEMPFAIL), never 0. Under plist
// schema 2 KeepAlive is {SuccessfulExit: false}, so exit 0 means "stay
// down" (the kill switch, a breaker stop) and an adoption must be non-zero
// to be relaunched; under schema 1 (KeepAlive true) any exit relaunches.
func adoptionExitCode() int { return tenancy.ExitRetryLater }

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

// selfRestartConfig carries the watcher's knobs. Zero values take the
// production defaults, except exit, which the caller must supply.
type selfRestartConfig struct {
	poll     time.Duration // stat interval (default selfRestartPoll)
	cooldown time.Duration // minimum spacing between adoptions (0 = none)
	// ledger is the lifecycle ledger (~/.fathom/daemon/<label>/lifecycle.jsonl)
	// holding the cross-process cooldown; "" keeps the cooldown in-process.
	ledger string
	exit   func(int)                    // os.Exit in production
	hash   func(string) (string, error) // default hashFile; tests count calls
	now    func() time.Time             // default time.Now
}

// watchOwnBinary polls the daemon's own executable and exits with
// adoptionExitCode when a verified newer binary is in place.
//
// Each poll is one stat; the binary is hashed only when its (size, mtime,
// inode) changed since the last poll (RT-12). A change is adopted once the
// stat has held still for one full poll (settle), the cooldown since the
// last adoption — this process's or, via the ledger, any predecessor's — has
// passed, and the candidate runs.
func watchOwnBinary(path string, log *dlog.Logger, cfg selfRestartConfig) {
	if cfg.poll <= 0 {
		cfg.poll = selfRestartPoll
	}
	if cfg.hash == nil {
		cfg.hash = hashFile
	}
	if cfg.now == nil {
		cfg.now = time.Now
	}
	startedAt := cfg.now() // the uptime fallback when the ledger is unreadable
	startStat, serr := statBinary(path)
	start, err := cfg.hash(path)
	if serr != nil || err != nil {
		// No baseline means every later comparison is a guess. Say so once
		// and stop, rather than restarting on noise.
		if err == nil {
			err = serr
		}
		log.Warn(fmt.Sprintf("self-restart: disabled — cannot hash %s (%v); a deploy will need a manual restart", path, err))
		return
	}
	log.Info(fmt.Sprintf("self-restart: watching %s (%s); a verified new binary is adopted within ~%s",
		path, shortHash(start), cfg.poll*2))

	var (
		seen          = startStat // the stat last observed and hashed
		pending       string      // hash of changed bytes awaiting adoption
		refusedAt     time.Time   // backoff after a refused candidate; retried after it
		deferNoticeOf string      // pending hash whose cooldown deferral was logged
		hashErrOf     binaryStat  // stat whose hash failure was logged
	)
	for {
		time.Sleep(cfg.poll)

		st, serr := statBinary(path)
		if serr != nil {
			pending = ""
			continue
		}
		if binaryStatChanged(seen, st) {
			// (3) Settle: a changed stat is hashed once and must then hold
			// still for a full poll, so a file still being written is never
			// adopted half-copied.
			cur, herr := cfg.hash(path)
			if herr != nil {
				// Not recorded as seen: the next poll hashes it again, so a
				// transient read failure never strands a deploy, and the
				// failure is named once per stat (4, I38).
				if hashErrOf != st {
					log.Warn(fmt.Sprintf("self-restart: %s changed but cannot be hashed (%v); retrying on the next poll", path, herr))
					hashErrOf = st
				}
				pending = ""
				continue
			}
			seen = st
			if cur == start {
				pending = ""
			} else {
				pending = cur
				refusedAt = time.Time{}
			}
			continue
		}
		if pending == "" {
			continue
		}

		// (2) Rate limit: after a refusal, and across processes.
		now := cfg.now()
		if !refusedAt.IsZero() && now.Sub(refusedAt) < max(cfg.cooldown, cfg.poll) {
			continue
		}
		if cfg.ledger != "" && cfg.cooldown > 0 {
			last, ok, lerr := lastAdoption(cfg.ledger)
			switch {
			case lerr != nil:
				// I38: an unreadable ledger is named, not read as "no
				// adoption". The cooldown is then unknown, and the
				// uncertainty resolves toward waiting: adopt only once this
				// process has itself run a full cooldown, which still bounds
				// adoptions to one per cooldown across processes.
				up := now.Sub(startedAt)
				if deferNoticeOf != pending {
					log.Warn(fmt.Sprintf("self-restart: cannot read the lifecycle ledger %s (%v); the cross-process cooldown is unknown, so adoption waits until this process has run %s (up %s)",
						cfg.ledger, lerr, cfg.cooldown, up.Round(time.Second)))
					deferNoticeOf = pending
				}
				if up < cfg.cooldown {
					continue
				}
			case ok && now.Sub(last) < cfg.cooldown:
				if deferNoticeOf != pending {
					log.Info(fmt.Sprintf("self-restart: %s changed (%s); adoption deferred — the last adoption was %s ago, inside the %s cooldown (deploys in the cooldown coalesce)",
						path, shortHash(pending), now.Sub(last).Round(time.Second), cfg.cooldown))
					deferNoticeOf = pending
				}
				continue
			}
		}

		// (1) Verify before yielding.
		ver, verr := binaryReports(path)
		if verr != nil {
			log.Warn(fmt.Sprintf("self-restart: REFUSED — %s changed but will not run (%v). "+
				"Keeping this process, so the fleet stays served by working code. "+
				"An unsigned binary fails exactly this way; `task deploy` signs.", path, verr))
			// start is deliberately NOT updated, and pending is kept: the
			// candidate is re-verified after the refusal backoff (the
			// cooldown), so a good binary whose first run failed
			// transiently (a version timeout under memory pressure, a
			// slow first exec) is not stranded; a repaired binary's stat
			// change re-hashes it at once.
			refusedAt = now
			continue
		}
		// (3) again: the file may have changed while the candidate ran —
		// between this poll's stat and the verify. Adopting then would
		// yield to bytes that were never hashed or settled (a copy still
		// being written) and record the wrong hash in the ledger. Re-stat;
		// a change goes back through hash and settle on the next poll.
		if after, aerr := statBinary(path); aerr != nil || after != seen {
			continue
		}

		if cfg.ledger != "" {
			if werr := appendAdoptRow(cfg.ledger, now, start, pending); werr != nil {
				log.Warn(fmt.Sprintf("self-restart: cannot record the adoption in %s (%v); the next process's cooldown will not see it", cfg.ledger, werr))
			}
		}
		log.Info(fmt.Sprintf("self-restart: adopting %s (%s → %s)%s — exiting %d so launchd restarts on the new code",
			ver, shortHash(start), shortHash(pending), buildAgeSuffix(), adoptionExitCode()))
		cfg.exit(adoptionExitCode())
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
	//fathom:tenancy-allow exec-noctx runs the new binary's --version with its own watchdog timer in the self-restart path; no request context
	cmd := exec.Command(path, "version")
	done := make(chan struct{})
	var (
		outBytes []byte
		runErr   error
	)
	//fathom:tenancy-allow bare-go runs the candidate binary's version check against a watchdog timer; buffered, never leaks
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
