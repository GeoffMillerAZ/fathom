package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient/anthropic"
)

const pairedChargeBasis = "journal-usage-priced-or-reserved-local-accounting"

type pairedReceipt struct {
	Version                   string            `json:"version"`
	ExecutionID               string            `json:"execution_id"`
	CaseID                    string            `json:"case_id"`
	Variant                   string            `json:"variant"`
	StudyDigest               string            `json:"study_digest"`
	PreparedDigest            string            `json:"prepared_digest"`
	NativeRequestDigest       string            `json:"native_request_digest"`
	Status                    string            `json:"status"`
	ChargeBasis               string            `json:"charge_basis"`
	CumulativeChargedMicroUSD int64             `json:"cumulative_charged_micro_usd"`
	CaseChargedMicroUSD       int64             `json:"case_charged_micro_usd"`
	IndependentAcceptance     bool              `json:"independent_acceptance"`
	PublicationAuthority      bool              `json:"publication_authority"`
	ModelSiteAdoption         bool              `json:"model_site_adoption"`
	ErrorClass                string            `json:"error_class,omitempty"`
	Content                   *pairedRowContent `json:"content,omitempty"`
}
type pairedRow struct {
	Position    int               `json:"position"`
	CaseID      string            `json:"case_id"`
	Variant     string            `json:"variant"`
	ExecutionID string            `json:"execution_id"`
	State       string            `json:"state"`
	Content     *pairedRowContent `json:"content,omitempty"`
}
type pairedRowContent struct {
	Assertion  string            `json:"assertion"`
	FactIDs    []string          `json:"fact_ids"`
	Facts      map[string]string `json:"facts"`
	Kind       string            `json:"kind"`
	Role       string            `json:"role,omitempty"`
	Expected   string            `json:"expected"`
	Reason     string            `json:"reason,omitempty"`
	Provenance *pairedProvenance `json:"provenance,omitempty"`
}

func pairedContent(in pairedInputs, c reviewCase, reason string) *pairedRowContent {
	v := &pairedRowContent{Assertion: c.Assertion, FactIDs: c.FactIDs, Facts: c.Facts, Kind: c.Kind, Expected: c.Expected, Reason: reason}
	for _, x := range in.scope.Extensions {
		if x.ID == c.ID {
			v.Role = x.Role
			p := x.Provenance
			v.Provenance = &p
			break
		}
	}
	return v
}

type pairedCounts struct {
	Planned          int   `json:"planned"`
	Complete         int   `json:"complete"`
	Missing          int   `json:"missing"`
	FalseSupport     int   `json:"false_support"`
	FalseHold        int   `json:"false_hold"`
	Inconclusive     int   `json:"inconclusive"`
	Pending          int   `json:"pending"`
	Failed           int   `json:"failed"`
	Unknown          int   `json:"unknown"`
	Invalid          int   `json:"invalid"`
	InputTokens      int64 `json:"input_tokens"`
	OutputTokens     int64 `json:"output_tokens"`
	CacheWriteTokens int64 `json:"cache_write_tokens"`
	CacheReadTokens  int64 `json:"cache_read_tokens"`
	ChargedMicroUSD  int64 `json:"charged_micro_usd"`
	LatencyNanos     int64 `json:"latency_nanos"`
}
type pairedReport struct {
	Version               string       `json:"version"`
	StudyDigest           string       `json:"study_digest"`
	CorpusDigest          string       `json:"corpus_digest"`
	GoalScopeDigest       string       `json:"goal_scope_digest"`
	ReviewScopeDigest     string       `json:"review_scope_digest"`
	Total                 pairedCounts `json:"total"`
	Baseline              pairedCounts `json:"baseline"`
	Candidate             pairedCounts `json:"candidate"`
	Rows                  []pairedRow  `json:"rows"`
	ChargeBasis           string       `json:"charge_basis"`
	Qualified             bool         `json:"qualified"`
	IndependentAcceptance bool         `json:"independent_acceptance"`
	PublicationAuthority  bool         `json:"publication_authority"`
	ModelSiteAdoption     bool         `json:"model_site_adoption"`
}

func runReviewPair(ctx context.Context, args []string, out, errOut io.Writer) int {
	return runReviewPairWith(ctx, args, out, errOut, reviewDeps{})
}
func runReviewPairWith(ctx context.Context, args []string, out, errOut io.Writer, deps reviewDeps) int {
	cmd := args[0]
	usageText := "Usage: prose-lab review-pair-prepare|review-pair-capture|review-pair-report --study PATH --corpus DIR --goal-scope PATH --goal-scope-digest SHA256 --base-review-scope PATH --review-scope PATH --review-scope-digest SHA256 --regression-source PATH [--case ID --variant baseline|candidate] [--prepared-digest SHA256 --journal PRIVATE_DIR --artifacts PRIVATE_DIR --live] [--include-content]"
	if len(args) == 2 && (args[1] == "--help" || args[1] == "-h" || args[1] == "help") {
		return pairedWrite(out, []byte(usageText+"\n"), 0)
	}
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	study := fs.String("study", "", "")
	corpus := fs.String("corpus", "", "")
	goal := fs.String("goal-scope", "", "")
	goalPin := fs.String("goal-scope-digest", "", "")
	base := fs.String("base-review-scope", "", "")
	scope := fs.String("review-scope", "", "")
	scopePin := fs.String("review-scope-digest", "", "")
	source := fs.String("regression-source", "", "")
	caseID := fs.String("case", "", "")
	variant := fs.String("variant", "", "")
	prepared := fs.String("prepared-digest", "", "")
	journalPath := fs.String("journal", "", "")
	artifactPath := fs.String("artifacts", "", "")
	live := fs.Bool("live", false, "")
	content := fs.Bool("include-content", false, "")
	if fs.Parse(args[1:]) != nil || fs.NArg() != 0 {
		return diagnostic(errOut, "invalid_pair_flags", 2)
	}
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	if cmd == "review-pair-prepare" && (!set["case"] || !set["variant"] || set["prepared-digest"] || set["journal"] || set["artifacts"] || set["live"]) {
		return diagnostic(errOut, "invalid_pair_prepare_flags", 2)
	}
	if cmd == "review-pair-report" && (set["case"] || set["variant"] || set["prepared-digest"] || set["live"] || *journalPath == "" || *artifactPath == "") {
		return diagnostic(errOut, "invalid_pair_report_flags", 2)
	}
	if cmd == "review-pair-capture" && (!set["case"] || !set["variant"] || !documentDigestPattern.MatchString(*prepared) || !*live || *journalPath == "" || *artifactPath == "") {
		return diagnostic(errOut, "invalid_pair_capture_flags", 2)
	}
	if cmd != "review-pair-prepare" && (filepath.Clean(*journalPath) == filepath.Clean(*artifactPath)) {
		return diagnostic(errOut, "shared_private_roots_refused", 2)
	}
	if !set["regression-source"] || *source == "" {
		return diagnostic(errOut, "regression_source_required", 2)
	}
	in, e := loadPairedInputs(*study, *corpus, *goal, *goalPin, *base, *scope, *scopePin, *source)
	if e != nil {
		return diagnostic(errOut, "paired_inputs_refused", 2)
	}
	if cmd == "review-pair-report" {
		return pairedRunReport(ctx, in, *journalPath, *artifactPath, *content, out, errOut)
	}
	c, ok := in.cases[*caseID]
	if !ok || (*variant != "baseline" && *variant != "candidate") {
		return diagnostic(errOut, "foreign_pair_execution", 2)
	}
	executionID := pairedExecutionID(*variant, *caseID)
	_, wire, e := pairedRequest(ctx, c, in.study.Review, *variant)
	if e != nil {
		return diagnostic(errOut, "paired_request_refused", 2)
	}
	digest := pairedPreparedDigest(in, executionID, wire)
	if cmd == "review-pair-prepare" {
		protocol := reviewProtocol
		if *variant == "candidate" {
			protocol = pairedCandidateProtocol
		}
		p := map[string]any{"version": "prose-lab.review-pair-prepared/1", "execution_id": executionID, "case_id": c.ID, "variant": *variant, "study_digest": in.studyDigest, "native_request_digest": hashBytes(wire), "prepared_digest": digest, "reservation_micro_usd": in.study.Review.reserveCost(), "planned": 140, "requested_review_model": in.study.Review.Model, "review_protocol": protocol, "corpus_digest": in.corpus.Digest, "goal_scope_digest": in.goalDigest, "base_review_scope_digest": in.scope.BaseScopeDigest, "review_scope_digest": in.reviewDigest, "regression_source_digest": pairedRegressionSourcePin, "budget_micro_usd": in.study.BudgetMicroUSD, "prior_direct_micro_usd": in.study.PriorDirectMicroUSD, "direct_stage_budget_micro_usd": in.study.DirectStageBudgetMicroUSD, "live": false, "independent_acceptance": false, "publication_authority": false, "model_site_adoption": false}
		if *content {
			p["content"] = pairedContent(in, c, "")
		}
		return pairedJSON(ctx, out, p, 0)
	}
	if digest != *prepared {
		return diagnostic(errOut, "paired_prepared_mismatch", 2)
	}
	return pairedRunCapture(ctx, in, c, *variant, executionID, wire, digest, *journalPath, *artifactPath, *content, out, errOut, deps)
}
func pairedWrite(out io.Writer, b []byte, code int) int {
	n, e := out.Write(b)
	if e != nil || n != len(b) {
		return 2
	}
	return code
}
func pairedJSON(ctx context.Context, out io.Writer, v any, code int) int {
	if ctx.Err() != nil {
		return 2
	}
	b, e := json.Marshal(v)
	if e != nil || len(b) > 4<<20 {
		return 2
	}
	return pairedWrite(out, append(b, '\n'), code)
}
func pairedArtifactCheck(root *os.Root, in pairedInputs, c reviewCase, variant, id string, r reservation, dStatus, dReason string, charged, cumulative int64) error {
	if pairedMarkerCheck(root, in, c, variant, id) != nil {
		return errors.New("marker mismatch")
	}
	prepared := pairedPreparedDigest(in, id, []byte(r.RequestJSON))
	var got pairedReceipt
	if readJournalJSON(root, id+".receipt.json", &got) != nil {
		return errors.New("receipt missing")
	}
	want := pairedReceipt{Version: "prose-lab.review-pair-capture/1", ExecutionID: id, CaseID: c.ID, Variant: variant, StudyDigest: in.studyDigest, PreparedDigest: prepared, NativeRequestDigest: r.RequestHash, Status: dStatus, ChargeBasis: pairedChargeBasis, CumulativeChargedMicroUSD: cumulative, CaseChargedMicroUSD: charged}
	if !reflect.DeepEqual(got, want) {
		return errors.New("receipt mismatch")
	}
	var reason struct {
		Reason string `json:"reason"`
	}
	if readJournalJSON(root, id+".reason.json", &reason) != nil || reason.Reason != dReason {
		return errors.New("reason mismatch")
	}
	return nil
}
func pairedMarkerCheck(root *os.Root, in pairedInputs, c reviewCase, variant, id string) error {
	if root == nil {
		return errors.New("artifacts unavailable")
	}
	_, wire, e := pairedRequest(context.Background(), c, in.study.Review, variant)
	if e != nil {
		return e
	}
	prepared := pairedPreparedDigest(in, id, wire)
	var marker struct {
		ExecutionID    string `json:"execution_id"`
		StudyDigest    string `json:"study_digest"`
		PreparedDigest string `json:"prepared_digest"`
	}
	if readJournalJSON(root, id+".started.json", &marker) != nil || marker.ExecutionID != id || marker.StudyDigest != in.studyDigest || marker.PreparedDigest != prepared {
		return errors.New("marker mismatch")
	}
	return nil
}
func pairedHeldReceiptCheck(root *os.Root, in pairedInputs, c reviewCase, variant, id string, r reservation, charged, cumulative int64) error {
	if pairedMarkerCheck(root, in, c, variant, id) != nil {
		return errors.New("marker mismatch")
	}
	var got pairedReceipt
	if readJournalJSON(root, id+".receipt.json", &got) != nil {
		return errors.New("held receipt missing")
	}
	want := pairedReceipt{Version: "prose-lab.review-pair-capture/1", ExecutionID: id, CaseID: c.ID, Variant: variant, StudyDigest: in.studyDigest, PreparedDigest: pairedPreparedDigest(in, id, []byte(r.RequestJSON)), NativeRequestDigest: r.RequestHash, Status: "held", ChargeBasis: pairedChargeBasis, CumulativeChargedMicroUSD: cumulative, CaseChargedMicroUSD: charged, ErrorClass: "review_incomplete"}
	if !reflect.DeepEqual(got, want) {
		return errors.New("held receipt mismatch")
	}
	return nil
}
func pairedAudit(ctx context.Context, in pairedInputs, attempts []journalAttempt, root *os.Root, content bool) (pairedReport, int, error) {
	r := pairedReport{Version: "prose-lab.review-pair-report/1", StudyDigest: in.studyDigest, CorpusDigest: in.corpus.Digest, GoalScopeDigest: in.goalDigest, ReviewScopeDigest: in.reviewDigest, Rows: make([]pairedRow, 0, 140), ChargeBasis: pairedChargeBasis}
	var cumulative int64
	stop := false
	knownCandidateMismatch := false
	for i, id := range in.ordered {
		v, cid, _ := pairedCaseID(id)
		c := in.cases[cid]
		row := pairedRow{Position: i + 1, CaseID: cid, Variant: v, ExecutionID: id, State: "missing"}
		counts := &r.Baseline
		if v == "candidate" {
			counts = &r.Candidate
		}
		counts.Planned++
		r.Total.Planned++
		if i < len(attempts) {
			a := attempts[i]
			res, o := a.Reservation, a.Outcome
			charged := res.MicroUSD
			if o != nil && o.ActualMicroUSD != nil {
				charged = *o.ActualMicroUSD
			}
			cumulative += charged
			counts.ChargedMicroUSD += charged
			r.Total.ChargedMicroUSD += charged
			if o != nil {
				if o.LatencyNanos < 0 || counts.LatencyNanos > math.MaxInt64-o.LatencyNanos || r.Total.LatencyNanos > math.MaxInt64-o.LatencyNanos {
					return r, 2, errors.New("latency accounting overflow")
				}
				counts.LatencyNanos += o.LatencyNanos
				r.Total.LatencyNanos += o.LatencyNanos
			}
			if stop || res.CaseID != id || res.Provider != "anthropic" || res.Model != in.study.Review.Model || res.Pricing != in.study.Review || res.MicroUSD != in.study.Review.reserveCost() {
				row.State = "invalid"
			} else {
				_, wire, e := pairedRequest(ctx, c, in.study.Review, v)
				if e != nil || res.RequestHash != hashBytes(wire) || res.RequestJSON != string(wire) {
					row.State = "invalid"
				} else if o == nil {
					if pairedMarkerCheck(root, in, c, v, id) != nil {
						row.State = "invalid"
					} else {
						row.State = "pending"
					}
				} else if o.Status == "failed" || o.Status == "unknown" {
					if pairedHeldReceiptCheck(root, in, c, v, id, res, charged, cumulative) != nil {
						row.State = "invalid"
					} else {
						row.State = o.Status
					}
				} else if o.Status != "completed" || o.HTTPStatus < 200 || o.HTTPStatus >= 300 || !o.UsagePresent || o.ActualMicroUSD == nil {
					row.State = "invalid"
				} else {
					d, e := pairedReplay(ctx, c, in.study.Review, v, wire, []byte(o.Response))
					if e != nil || pairedArtifactCheck(root, in, c, v, id, res, d.Status, d.Reason, charged, cumulative) != nil {
						row.State = "invalid"
					} else {
						row.State = d.Status
						if row.State == "supported" || row.State == "unsupported" {
							counts.Complete++
							r.Total.Complete++
							if row.State != c.Expected {
								if c.Expected == "supported" {
									counts.FalseHold++
									r.Total.FalseHold++
								} else {
									counts.FalseSupport++
									r.Total.FalseSupport++
								}
								if v == "candidate" {
									knownCandidateMismatch = true
								}
							}
						} else {
							counts.Inconclusive++
							r.Total.Inconclusive++
						}
						if content {
							row.Content = pairedContent(in, c, d.Reason)
						}
						_, usage, _, ue := readUsage([]byte(o.Response), in.study.Review)
						if ue == nil {
							counts.InputTokens += usage.Input
							counts.OutputTokens += usage.Output
							counts.CacheWriteTokens += usage.CacheWrite
							counts.CacheReadTokens += usage.CacheRead
							r.Total.InputTokens += usage.Input
							r.Total.OutputTokens += usage.Output
							r.Total.CacheWriteTokens += usage.CacheWrite
							r.Total.CacheReadTokens += usage.CacheRead
						}
					}
				}
			}
		}
		if content && row.Content == nil {
			row.Content = pairedContent(in, c, "")
		}
		switch row.State {
		case "missing":
			counts.Missing++
			r.Total.Missing++
		case "pending":
			counts.Pending++
			r.Total.Pending++
		case "failed":
			counts.Failed++
			r.Total.Failed++
		case "unknown":
			counts.Unknown++
			r.Total.Unknown++
		case "invalid":
			counts.Invalid++
			r.Total.Invalid++
		}
		if row.State == "pending" || row.State == "failed" || row.State == "unknown" || row.State == "invalid" || row.State == "inconclusive" || (v == "candidate" && (row.State == "supported" || row.State == "unsupported") && row.State != c.Expected) {
			stop = true
		}
		r.Rows = append(r.Rows, row)
	}
	if len(attempts) > 140 || cumulative > in.study.BudgetMicroUSD || in.study.PriorDirectMicroUSD > in.study.DirectStageBudgetMicroUSD-cumulative {
		return r, 2, errors.New("history exceeds manifest or budget")
	}
	r.Qualified = r.Total.Complete == 140 && r.Total.Inconclusive == 0 && r.Total.Invalid == 0 && r.Total.Missing == 0 && r.Total.Pending == 0 && r.Total.Failed == 0 && r.Total.Unknown == 0 && r.Candidate.FalseHold == 0 && r.Candidate.FalseSupport == 0
	if r.Total.Invalid > 0 || r.Total.Pending > 0 || r.Total.Failed > 0 || r.Total.Unknown > 0 || r.Total.Inconclusive > 0 {
		return r, 2, nil
	}
	if knownCandidateMismatch {
		return r, 1, nil
	}
	if !r.Qualified {
		return r, 2, nil
	}
	return r, 0, nil
}
func pairedRunReport(ctx context.Context, in pairedInputs, journalPath, artifactPath string, content bool, out, errOut io.Writer) int {
	attempts, charged, e := auditJournal(journalPath, in.boundStudy, in.study.BudgetMicroUSD)
	if e != nil {
		return diagnostic(errOut, "paired_journal_refused", 2)
	}
	root, e := existingReviewArtifacts(artifactPath)
	if e != nil {
		return diagnostic(errOut, "paired_artifacts_refused", 2)
	}
	defer root.Close()
	if pairedCheckArtifactNames(root, attempts) != nil {
		return diagnostic(errOut, "paired_artifacts_invalid", 2)
	}
	r, code, e := pairedAudit(ctx, in, attempts, root, content)
	if e != nil || charged != r.Total.ChargedMicroUSD {
		return diagnostic(errOut, "paired_history_refused", 2)
	}
	return pairedJSON(ctx, out, r, code)
}
func pairedCheckArtifactNames(root *os.Root, attempts []journalAttempt) error {
	f, e := root.Open(".")
	if e != nil {
		return e
	}
	defer f.Close()
	entries, e := f.ReadDir(1000)
	if e != nil && e != io.EOF {
		return e
	}
	allowed := map[string]bool{}
	for _, a := range attempts {
		id := a.Reservation.CaseID
		allowed[id+".started.json"] = true
		if a.Outcome != nil {
			allowed[id+".receipt.json"] = true
			if a.Outcome.Status == "completed" {
				allowed[id+".reason.json"] = true
			}
		}
	}
	for _, entry := range entries {
		if !allowed[entry.Name()] || !entry.Type().IsRegular() {
			return errors.New("extra artifact")
		}
	}
	return nil
}
func pairedRunCapture(ctx context.Context, in pairedInputs, c reviewCase, variant, id string, wire []byte, prepared, journalPath, artifactPath string, content bool, out, errOut io.Writer, deps reviewDeps) int {
	if deps.now == nil {
		deps.now = time.Now
	}
	if deps.captureHTTP == nil {
		deps.captureHTTP = func() (*http.Client, *captureTransport) { return studyHTTP("anthropic") }
	}
	root, e := documentArtifactRoot(artifactPath)
	if e != nil {
		return diagnostic(errOut, "paired_artifacts_refused", 2)
	}
	defer root.Close()
	j, e := openJournal(journalPath, in.boundStudy, in.study.BudgetMicroUSD, deps.now)
	if e != nil {
		return diagnostic(errOut, "paired_journal_refused", 2)
	}
	defer j.close()
	reader := &journal{root: j.root, header: j.header, next: 1}
	attempts, e := reader.readHistory(false)
	if e != nil || pairedCheckArtifactNames(root, attempts) != nil {
		return diagnostic(errOut, "paired_prior_refused", 2)
	}
	prior, _, e := pairedAudit(ctx, in, attempts, root, false)
	if e != nil || len(attempts) >= len(in.ordered) || in.ordered[len(attempts)] != id || prior.Total.Invalid > 0 || prior.Total.Pending > 0 || prior.Total.Failed > 0 || prior.Total.Unknown > 0 || prior.Total.Inconclusive > 0 || prior.Candidate.FalseHold > 0 || prior.Candidate.FalseSupport > 0 || in.study.Review.reserveCost() > in.study.BudgetMicroUSD-j.charged || in.study.PriorDirectMicroUSD > in.study.DirectStageBudgetMicroUSD-j.charged-in.study.Review.reserveCost() {
		return diagnostic(errOut, "paired_order_or_budget_refused", 2)
	}
	if e = writeExclusive(root, id+".started.json", map[string]string{"execution_id": id, "study_digest": in.studyDigest, "prepared_digest": prepared}); e != nil {
		return diagnostic(errOut, "paired_marker_refused", 2)
	}
	start := j.charged
	receipt := pairedReceipt{Version: "prose-lab.review-pair-capture/1", ExecutionID: id, CaseID: c.ID, Variant: variant, StudyDigest: in.studyDigest, PreparedDigest: prepared, NativeRequestDigest: hashBytes(wire), Status: "held", ChargeBasis: pairedChargeBasis}
	persist := func(reason string, status int) int {
		receipt.CumulativeChargedMicroUSD = j.charged
		receipt.CaseChargedMicroUSD = j.charged - start
		receipt.ErrorClass = reason
		if writeExclusive(root, id+".receipt.json", receipt) != nil {
			return diagnostic(errOut, "paired_receipt_refused", 2)
		}
		reply := receipt
		if content {
			reply.Content = pairedContent(in, c, "")
		}
		return pairedJSON(ctx, out, reply, status)
	}
	hc, cap := deps.captureHTTP()
	if hc == nil || cap == nil {
		return persist("client_unavailable", 2)
	}
	inner, e := anthropic.New(anthropic.Config{APIKey: deps.apiKey, HTTPClient: hc, DefaultModel: in.study.Review.Model})
	if e != nil {
		return persist("client_unavailable", 2)
	}
	client := &documentTextClient{inner: &labClient{inner: inner, attempt: &attemptRunner{journal: j, lane: in.study.Review, capture: cap, now: deps.now, caseID: id, expectedRequestDigest: hashBytes(wire)}}}
	d, e := pairedReviewer(c, in.study.Review, variant, client).Review(ctx, reviewClaim(c))
	if e != nil || len(j.pending) != 0 {
		return persist("review_incomplete", 2)
	}
	var last reservation
	if readJournalJSON(j.root, fmt.Sprintf("reserve-%06d.json", j.next-1), &last) != nil || last.CaseID != id || last.RequestHash != hashBytes(wire) {
		return persist("native_request_mismatch", 2)
	}
	receipt.Status = d.Status
	receipt.CumulativeChargedMicroUSD = j.charged
	receipt.CaseChargedMicroUSD = j.charged - start
	if e = writeExclusive(root, id+".reason.json", map[string]string{"reason": d.Reason}); e != nil {
		return persist("reason_write_failed", 2)
	}
	if e = writeExclusive(root, id+".receipt.json", receipt); e != nil {
		return diagnostic(errOut, "paired_receipt_refused", 2)
	}
	code := 0
	if d.Status == "inconclusive" {
		code = 2
	} else if variant == "candidate" && d.Status != c.Expected {
		code = 1
	}
	if content {
		receipt.Content = pairedContent(in, c, d.Reason)
	}
	return pairedJSON(ctx, out, receipt, code)
}
