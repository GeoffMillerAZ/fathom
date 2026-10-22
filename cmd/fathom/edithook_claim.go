package main

// edithook_claim.go — PC-22 (docs/vision/26-proof-credibility.md §4): the
// claim gate at the claim moment. The verify gate fires at push and the
// Stop hook fires at session end; the claim itself is the `[x]` edit on a
// docs/plans/*.md file, and the PostToolUse edit hook already sees every
// one of those. When such an edit adds at least one `[x]` relative to git
// HEAD, this runs the verify engine for THAT plan — the same daemon path
// the Stop hook uses (verifyViaHTTP, newlyClaimedTasks, claimRefusals) —
// and hands back the refusal lines for injection as additionalContext.
//
// Silence contract (I9, §5.3): a non-plan file, a plan edit that claims
// nothing new, an unindexed project, a down or slow daemon, or any other
// operational failure returns "" — the edit proceeds, nothing is printed.
// Unknown verdicts never refuse (fail-open honesty, exactly as the Stop
// hook). Every verify this path runs leaves one ledger row with
// origin: claim-moment (PC-26's pre-registered reading counts them).
//
// Cost: zero daemon traffic unless the edited file is a depth-1 plan AND
// the working tree carries a newly checked row; the local diff against
// HEAD decides that before any RPC.

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// claimMomentChannel is the beat channel this path writes: seen (a plan
// edit with a new claim), refused (with the count), clean, error.
const claimMomentChannel = "claim-moment"

// claimMomentPlanRel reports whether filePath is a depth-1 plan under root
// (docs/plans/<name>.md, never README.md, never docs/plans/archive/…) and
// returns its slash-separated repo-relative path — the same file set the
// Stop hook walks and the pre-push gate verifies.
func claimMomentPlanRel(root, filePath string) (string, bool) {
	rel, ok := relativeToRoot(root, filePath)
	if !ok {
		return "", false
	}
	rel = filepath.ToSlash(rel)
	const prefix = "docs/plans/"
	if !strings.HasPrefix(rel, prefix) {
		return "", false
	}
	name := strings.TrimPrefix(rel, prefix)
	if strings.Contains(name, "/") || !strings.HasSuffix(name, ".md") || strings.EqualFold(name, "README.md") {
		return "", false
	}
	return rel, true
}

// claimMomentVerify is the edit hook's entry point: the refusal text for
// a newly claimed plan row that the index disproves, or "" (silence).
func claimMomentVerify(url, projDir, filePath, sessionID string) string {
	return claimMomentVerifyWith(newStopHookClient(), url, projDir, filePath, sessionID)
}

// claimMomentVerifyWith is claimMomentVerify with an injected HTTP client
// (tests pin the timeout behavior through it).
func claimMomentVerifyWith(hc *http.Client, baseURL, projDir, filePath, sessionID string) string {
	// Locate the plan under a candidate root — local reads only. The
	// first candidate that contains the file names the repo root the
	// HEAD diff and the ledger row are scoped to.
	var root, rel string
	for _, cand := range serviceCandidates(projDir) {
		if r, ok := claimMomentPlanRel(cand.root, filePath); ok {
			root, rel = cand.root, r
			break
		}
	}
	if root == "" {
		return ""
	}
	newMD, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}
	oldMD := headFileVersion(root, rel) // nil when new file / no git
	// The service label does not change which rows are claimed, so the
	// "did this edit claim anything" question is answered before any
	// daemon call; an edit that only reworded a row costs nothing.
	if len(newlyClaimedTasks(oldMD, newMD, "")) == 0 {
		return ""
	}
	emitBeat(claimMomentChannel, "seen", firingMeta(map[string]string{"plan": rel}, sessionID, filePath))
	refusals, service, err := claimMomentRefusals(hc, baseURL, projDir, root, rel, oldMD, newMD, verifyOriginClaimMoment, claimMomentChannel, sessionID, filePath)
	if err != nil || service == "" {
		return "" // daemon down, unindexed or a failed verify → silent (I9)
	}
	newly := newlyClaimedTasks(oldMD, newMD, service)
	if len(refusals) == 0 {
		emitBeat(claimMomentChannel, "clean", firingMeta(map[string]string{"plan": rel, "claimed": strconv.Itoa(len(newly))}, sessionID, filePath))
		return ""
	}
	emitBeat(claimMomentChannel, "refused", firingMeta(map[string]string{"plan": rel, "claimed": strconv.Itoa(len(newly)), "refused": strconv.Itoa(len(refusals))}, sessionID, filePath))
	return "⛔ done-means-proven (claim moment): task(s) just checked [x] in " + rel + " fail verification:\n• " +
		strings.Join(refusals, "\n• ") +
		fmt.Sprintf("\nProve the claim (fix the code / contract) or uncheck it — `fathom verify --service %s --plan %s` shows the full verdict.", service, rel)
}

// claimMomentRefusals is PC-22's verify for one plan's text newMD against
// its prior oldMD: the service the daemon indexes for projDir, then the
// verify engine over newMD (a row self-skipped against the prior is
// overlaid), one ledger row with origin, and the refusal lines for the
// newly claimed rows. service is "" for an unindexed project (no error);
// err is a probe or verify failure, already recorded as an error beat on
// channel. Shared by the claim moment and the gate preflight (RO-40), so
// the two can never disagree about what verify refuses.
func claimMomentRefusals(hc *http.Client, baseURL, projDir, root, rel string, oldMD, newMD []byte, origin, channel, sessionID, filePath string) ([]string, string, error) {
	// Service identity: the first candidate the daemon has indexed —
	// the Stop hook's discipline, verbatim.
	service := ""
	for _, cand := range candidatesWithDaemon(hc, baseURL, projDir) {
		name := strings.ToLower(cand.name)
		ok, probeErr := daemonServiceIndexed(hc, baseURL, name)
		if probeErr != nil {
			emitBeat(channel, "error", firingMeta(map[string]string{"plan": rel, "stage": "probe"}, sessionID, filePath))
			return nil, "", probeErr
		}
		if ok {
			service = name
			break
		}
	}
	if service == "" {
		return nil, "", nil // unindexed project → silent
	}
	newly := newlyClaimedTasks(oldMD, newMD, service)
	localPlan := verify.ParseMarkdownPlan(service, newMD)
	// O56: this hook IS the working-tree-vs-HEAD moment — oldMD (nil for a
	// brand-new file) is always the right prior to compare against; a row
	// whose skip_test/skip is new relative to it is self-skipped, however
	// good its skip: reason reads.
	priorPlan := verify.ParseMarkdownPlan(service, oldMD)
	localPlan = verify.AnnotateSelfSkip(localPlan, &priorPlan)
	rep, err := verifyViaHTTP(hc, baseURL, service, "", newMD, localPlan, goneTreeFor(root, localPlan))
	if err != nil {
		// A timeout or a bad response is operational, never evidence
		// about the claim: silent here (the Stop hook and the push gate
		// still stand behind this edit), recorded as an error beat.
		emitBeat(channel, "error", firingMeta(map[string]string{"plan": rel, "stage": "verify"}, sessionID, filePath))
		return nil, service, err
	}
	verify.OverlaySelfSkip(&rep, localPlan)
	appendVerifyLedgerFrom(origin, root, []string{rel}, service, []verify.Report{rep},
		verifyLedgerProvenance(hc, baseURL, "", service))
	return claimRefusals(root, rel, newly, rep), service, nil
}

// withClaimMoment composes the claim-moment refusal (if any) with the
// obligations payload the edit hook already rendered. The ONE line
// cmdEditHook adds: text unchanged and ok unchanged when there is no
// refusal, so every pre-existing edit-hook golden holds byte for byte;
// a refusal leads the payload (it is the reason the edit needs attention)
// and turns a would-be-silent hook into an answering one.
func withClaimMoment(url, projDir, filePath, sessionID, text string, ok bool) (string, bool) {
	claim := claimMomentVerify(url, projDir, filePath, sessionID)
	if _, plan := claimMomentPlanRel(projDir, filePath); plan {
		noteAdviceComponent(harness.CapClaimGate, claim != "")
	}
	if claim == "" {
		return text, ok
	}
	if ok && text != "" {
		return claim + "\n\n" + text, true
	}
	return claim, true
}

// claimRefusals renders the refusal line for every newly claimed task the
// report proves FAILING — "<plan> [<id>] <title> — <criterion>: <detail>",
// sorted. Unknown never refuses: an unprovable row is a coverage gap, not a
// false claim (ADR-027). Shared by the Stop hook and the claim-moment path
// so the two gates can never disagree about what a refusal looks like.
func claimRefusals(projDir, rel string, newly []string, rep verify.Report) []string {
	verdictByID := map[string]verify.TaskVerdict{}
	for _, tv := range rep.Tasks {
		verdictByID[tv.TaskID] = tv
	}
	var out []string
	for _, id := range newly {
		tv, ok := verdictByID[id]
		if !ok || tv.Status != verify.Fail {
			continue
		}
		detail := tv.Label
		for _, c := range tv.Criteria {
			if c.Status == verify.Fail {
				detail = fmt.Sprintf("%s: %s", c.Name, c.Detail)
				break
			}
		}
		out = append(out, fmt.Sprintf("%s [%s] %s — %s", rel, tv.TaskID, tv.Title, detail))
	}
	sort.Strings(out)
	return out
}
