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
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/internal/jsonwire"
)

const qualifiedPreDispatchChargeBasis = "pre-dispatch-no-native-reservation"

type qualifiedReceipt struct {
	Version                    string `json:"version"`
	Protocol                   string `json:"protocol,omitempty"`
	CaseID                     string `json:"case_id"`
	StudyDigest                string `json:"study_digest"`
	PreparedDigest             string `json:"prepared_digest"`
	NativeRequestDigest        string `json:"native_request_digest"`
	Status                     string `json:"status"`
	ChargeBasis                string `json:"charge_basis"`
	CumulativeChargedMicroUSD  int64  `json:"cumulative_charged_micro_usd"`
	CaseChargedMicroUSD        int64  `json:"case_charged_micro_usd"`
	ErrorClass                 string `json:"error_class,omitempty"`
	PriorSpendDigest           string `json:"prior_spend_digest"`
	DeclaredPriorMicroUSD      int64  `json:"declared_prior_micro_usd"`
	DeclaredStageCapMicroUSD   int64  `json:"declared_stage_cap_micro_usd"`
	CrossTrialSpendVerified    bool   `json:"cross_trial_spend_verified"`
	ProviderSpendLimitEnforced bool   `json:"provider_spend_limit_enforced"`
	SourceAuthenticated        bool   `json:"source_authenticated"`
	IdentityAuthenticated      bool   `json:"identity_authenticated"`
	IndependentAcceptance      bool   `json:"independent_acceptance"`
	PublicationAuthority       bool   `json:"publication_authority"`
	UIEntry                    bool   `json:"ui_entry"`
	ModelSiteAdoption          bool   `json:"model_site_adoption"`
}
type qualifiedRow struct {
	Position  int                `json:"position"`
	State     string             `json:"state"`
	CaseID    string             `json:"case_id,omitempty"`
	Expected  string             `json:"expected,omitempty"`
	Reason    string             `json:"reason,omitempty"`
	Decision  *qualifiedDecision `json:"decision,omitempty"`
	Assertion string             `json:"assertion,omitempty"`
	Facts     []floorFact        `json:"facts,omitempty"`
}
type qualifiedReport struct {
	Version                    string                    `json:"version"`
	Protocol                   string                    `json:"protocol,omitempty"`
	StudyDigest                string                    `json:"study_digest"`
	FloorReportDigest          string                    `json:"floor_report_digest"`
	PriorSpendDigest           string                    `json:"prior_spend_digest"`
	DeclaredPriorMicroUSD      int64                     `json:"declared_prior_micro_usd"`
	DeclaredStageCapMicroUSD   int64                     `json:"declared_stage_cap_micro_usd"`
	CrossTrialSpendVerified    bool                      `json:"cross_trial_spend_verified"`
	ProviderSpendLimitEnforced bool                      `json:"provider_spend_limit_enforced"`
	Planned                    int                       `json:"planned"`
	Completed                  int                       `json:"completed"`
	Matched                    int                       `json:"matched"`
	Mismatched                 int                       `json:"mismatched"`
	Missing                    int                       `json:"missing"`
	Pending                    int                       `json:"pending"`
	Failed                     int                       `json:"failed"`
	Unknown                    int                       `json:"unknown"`
	InvalidCount               int                       `json:"invalid_count"`
	FalseSupport               int                       `json:"false_support"`
	FalseHold                  int                       `json:"false_hold"`
	Inconclusive               int                       `json:"inconclusive"`
	InputTokens                int64                     `json:"input_tokens"`
	OutputTokens               int64                     `json:"output_tokens"`
	CacheWriteTokens           int64                     `json:"cache_write_tokens"`
	CacheReadTokens            int64                     `json:"cache_read_tokens"`
	ChargedMicroUSD            int64                     `json:"charged_micro_usd"`
	LatencyNanos               int64                     `json:"latency_nanos"`
	ConfusionMatrix            map[string]map[string]int `json:"confusion_matrix"`
	Rows                       []qualifiedRow            `json:"rows"`
	ChargeBasis                string                    `json:"charge_basis"`
	Qualified                  bool                      `json:"qualified"`
	SourceAuthenticated        bool                      `json:"source_authenticated"`
	IdentityAuthenticated      bool                      `json:"identity_authenticated"`
	IndependentAcceptance      bool                      `json:"independent_acceptance"`
	PublicationAuthority       bool                      `json:"publication_authority"`
	UIEntry                    bool                      `json:"ui_entry"`
	ModelSiteAdoption          bool                      `json:"model_site_adoption"`
}

func runQualifiedReview(ctx context.Context, args []string, out, errOut io.Writer) int {
	return runQualifiedReviewWith(ctx, args, out, errOut, reviewDeps{})
}
func runQualifiedReviewWith(ctx context.Context, args []string, out, errOut io.Writer, deps reviewDeps) int {
	if len(args) == 0 {
		return diagnostic(errOut, "qualified_command_required", 2)
	}
	cmd := args[0]
	if len(args) == 2 && (args[1] == "--help" || args[1] == "-h" || args[1] == "help") {
		return pairedWrite(out, []byte("Usage: prose-lab review-qualified-prepare|review-qualified-capture|review-qualified-report --trial PATH --prior-spend PATH --prior-spend-digest SHA256 --study PATH --corpus DIR --goal-scope PATH --goal-scope-digest SHA256 --base-review-scope PATH --review-scope PATH --review-scope-digest SHA256 --regression-source PATH --packet-digest SHA256 --assessments PATH --floor-review PATH --future-scope PATH --future-scope-digest SHA256 --fixtures DIR --review-dir DIR [--case ID] [--prepared-digest SHA256 --journal PRIVATE_DIR --artifacts PRIVATE_DIR --live] [--include-content]\n"), 0)
	}
	if cmd != "review-qualified-prepare" && cmd != "review-qualified-capture" && cmd != "review-qualified-report" {
		return diagnostic(errOut, "invalid_qualified_command", 2)
	}
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	names := []string{"study", "corpus", "goal-scope", "goal-scope-digest", "base-review-scope", "review-scope", "review-scope-digest", "regression-source", "packet-digest", "assessments", "floor-review", "future-scope", "future-scope-digest", "fixtures", "review-dir"}
	values := map[string]*string{}
	for _, n := range names {
		values[n] = fs.String(n, "", "")
	}
	trial := fs.String("trial", "", "")
	priorPath := fs.String("prior-spend", "", "")
	priorPin := fs.String("prior-spend-digest", "", "")
	caseID := fs.String("case", "", "")
	prepared := fs.String("prepared-digest", "", "")
	journalPath := fs.String("journal", "", "")
	artifactPath := fs.String("artifacts", "", "")
	live := fs.Bool("live", false, "")
	content := fs.Bool("include-content", false, "")
	if fs.Parse(args[1:]) != nil || fs.NArg() != 0 {
		return diagnostic(errOut, "invalid_qualified_flags", 2)
	}
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	for _, n := range append(names, "trial", "prior-spend", "prior-spend-digest") {
		if !set[n] {
			return diagnostic(errOut, "qualified_input_required", 2)
		}
	}
	switch cmd {
	case "review-qualified-prepare":
		if !set["case"] || set["prepared-digest"] || set["journal"] || set["artifacts"] || set["live"] {
			return diagnostic(errOut, "invalid_qualified_prepare_flags", 2)
		}
	case "review-qualified-capture":
		if !set["case"] || !documentDigestPattern.MatchString(*prepared) || !set["journal"] || !set["artifacts"] || !set["live"] || !*live {
			return diagnostic(errOut, "invalid_qualified_capture_flags", 2)
		}
	case "review-qualified-report":
		if set["case"] || set["prepared-digest"] || set["live"] || !set["journal"] || !set["artifacts"] {
			return diagnostic(errOut, "invalid_qualified_report_flags", 2)
		}
	}
	if cmd != "review-qualified-prepare" && (*journalPath == "" || *artifactPath == "" || filepath.Clean(*journalPath) == filepath.Clean(*artifactPath)) {
		return diagnostic(errOut, "shared_private_roots_refused", 2)
	}
	floorArgs := make([]string, 0, len(names)*2)
	for _, n := range names {
		floorArgs = append(floorArgs, "--"+n, *values[n])
	}
	in, e := loadQualifiedInputs(ctx, *trial, *priorPath, *priorPin, floorArgs)
	if e != nil {
		return diagnostic(errOut, "qualified_inputs_refused", 2)
	}
	if cmd == "review-qualified-report" {
		return qualifiedRunReport(ctx, in, *journalPath, *artifactPath, *content, out, errOut)
	}
	c, ok := in.cases[*caseID]
	if !ok {
		return diagnostic(errOut, "foreign_qualified_case", 2)
	}
	wire := in.wires[*caseID]
	digest := qualifiedPreparedDigest(in, *caseID, wire)
	if cmd == "review-qualified-prepare" {
		p := map[string]any{"version": "prose-lab.qualified-review-prepared/1", "study_digest": in.studyDigest, "floor_report_digest": hashBytes(jsonBytes(in.floor)), "prior_spend_digest": in.priorDigest, "declared_prior_micro_usd": in.trial.PriorDirectMicroUSD, "declared_stage_cap_micro_usd": in.trial.DirectStageBudgetMicroUSD, "cross_trial_spend_verified": false, "provider_spend_limit_enforced": false, "native_request_digest": hashBytes(wire), "prepared_digest": digest, "reservation_micro_usd": in.trial.Review.reserveCost(), "planned": 85, "protocol": in.protocol, "live": false, "source_authenticated": false, "identity_authenticated": false, "independent_acceptance": false, "publication_authority": false, "ui_entry": false, "model_site_adoption": false}
		if *content {
			p["case_id"] = *caseID
			p["assertion"] = c.Assertion
			p["facts"] = c.Facts
			p["expected"] = c.Expected
			p["requested_review_model"] = in.trial.Review.Model
		}
		return pairedJSON(ctx, out, p, 0)
	}
	if *prepared != digest {
		return diagnostic(errOut, "qualified_prepared_mismatch", 2)
	}
	return qualifiedRunCapture(ctx, in, c, wire, digest, *journalPath, *artifactPath, *content, out, errOut, deps)
}
func qualifiedMarkerCheck(root *os.Root, in qualifiedInputs, id string) error {
	if root == nil {
		return errors.New("missing artifact root")
	}
	var got struct {
		CaseID         string `json:"case_id"`
		StudyDigest    string `json:"study_digest"`
		PreparedDigest string `json:"prepared_digest"`
		Protocol       string `json:"protocol,omitempty"`
	}
	want := map[string]string{"case_id": id, "study_digest": in.studyDigest, "prepared_digest": qualifiedPreparedDigest(in, id, in.wires[id])}
	if in.protocol == qualifiedProtocolV4 {
		want["protocol"] = in.protocol
	}
	if qualifiedReadExactArtifact(root, id+".started.json", want, &got) != nil || got.CaseID != id || got.StudyDigest != in.studyDigest || got.PreparedDigest != want["prepared_digest"] || got.Protocol != qualifiedArtifactProtocol(in) {
		return errors.New("marker mismatch")
	}
	return nil
}
func qualifiedReadExactArtifact(root *os.Root, name string, shape, dst any) error {
	var raw map[string]json.RawMessage
	if err := readJournalJSON(root, name, &raw); err != nil {
		return err
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	fields, err := jsonwire.Object(jsonBytes(shape))
	if err != nil {
		return err
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	if err := exactObject(encoded, keys...); err != nil {
		return err
	}
	return decodeLabJSON(encoded, dst)
}
func qualifiedArtifactProtocol(in qualifiedInputs) string {
	if in.protocol == qualifiedProtocolV4 {
		return in.protocol
	}
	return ""
}
func qualifiedArtifactCheck(root *os.Root, in qualifiedInputs, id string, r reservation, d qualifiedDecision, charged, cumulative int64) error {
	if qualifiedMarkerCheck(root, in, id) != nil {
		return errors.New("marker mismatch")
	}
	want := qualifiedReceipt{Version: "prose-lab.qualified-review-capture/1", Protocol: qualifiedArtifactProtocol(in), CaseID: id, StudyDigest: in.studyDigest, PreparedDigest: qualifiedPreparedDigest(in, id, []byte(r.RequestJSON)), NativeRequestDigest: r.RequestHash, Status: d.Status, ChargeBasis: pairedChargeBasis, CumulativeChargedMicroUSD: cumulative, CaseChargedMicroUSD: charged, PriorSpendDigest: in.priorDigest, DeclaredPriorMicroUSD: in.trial.PriorDirectMicroUSD, DeclaredStageCapMicroUSD: in.trial.DirectStageBudgetMicroUSD}
	var got qualifiedReceipt
	if qualifiedReadExactArtifact(root, id+".receipt.json", want, &got) != nil {
		return errors.New("receipt missing")
	}
	if !reflect.DeepEqual(got, want) {
		return errors.New("receipt mismatch")
	}
	var reason qualifiedDecision
	if qualifiedReadExactArtifact(root, id+".reason.json", d, &reason) != nil || !reflect.DeepEqual(reason, d) {
		return errors.New("reason mismatch")
	}
	return nil
}
func qualifiedHeldCheck(root *os.Root, in qualifiedInputs, id string, r reservation, charged, cumulative int64) error {
	if qualifiedMarkerCheck(root, in, id) != nil {
		return errors.New("marker mismatch")
	}
	want := qualifiedReceipt{Version: "prose-lab.qualified-review-capture/1", Protocol: qualifiedArtifactProtocol(in), CaseID: id, StudyDigest: in.studyDigest, PreparedDigest: qualifiedPreparedDigest(in, id, []byte(r.RequestJSON)), NativeRequestDigest: r.RequestHash, Status: "held", ChargeBasis: pairedChargeBasis, CumulativeChargedMicroUSD: cumulative, CaseChargedMicroUSD: charged, ErrorClass: "review_incomplete", PriorSpendDigest: in.priorDigest, DeclaredPriorMicroUSD: in.trial.PriorDirectMicroUSD, DeclaredStageCapMicroUSD: in.trial.DirectStageBudgetMicroUSD}
	var got qualifiedReceipt
	if qualifiedReadExactArtifact(root, id+".receipt.json", want, &got) != nil {
		return errors.New("receipt missing")
	}
	if !reflect.DeepEqual(got, want) {
		return errors.New("held receipt mismatch")
	}
	return nil
}

// A terminal pre-dispatch failure has exactly two bound artifacts and no native
// reservation. Every other extra artifact remains an error.
func qualifiedArtifactNames(root *os.Root, in qualifiedInputs, attempts []journalAttempt, charged int64) (bool, error) {
	if root == nil || len(attempts) > len(in.ordered) {
		return false, errors.New("invalid artifact history")
	}
	f, err := root.Open(".")
	if err != nil {
		return false, err
	}
	defer f.Close()
	entries, err := f.ReadDir(1000)
	if err != nil && err != io.EOF {
		return false, err
	}
	if len(entries) >= 1000 {
		return false, errors.New("artifact entry bound")
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
	var nextID string
	if len(attempts) < len(in.ordered) {
		nextID = in.ordered[len(attempts)]
	}
	marker, receipt := false, false
	for _, entry := range entries {
		name := entry.Name()
		if !entry.Type().IsRegular() {
			return false, errors.New("nonregular artifact")
		}
		if allowed[name] {
			continue
		}
		if nextID != "" && name == nextID+".started.json" {
			marker = true
		} else if nextID != "" && name == nextID+".receipt.json" {
			receipt = true
		} else {
			return false, errors.New("unbound artifact")
		}
	}
	if marker != receipt {
		return false, errors.New("partial pre-dispatch artifact")
	}
	if !marker {
		return false, nil
	}
	if qualifiedMarkerCheck(root, in, nextID) != nil {
		return false, errors.New("pre-dispatch marker mismatch")
	}
	want := qualifiedReceipt{Version: "prose-lab.qualified-review-capture/1", Protocol: qualifiedArtifactProtocol(in), CaseID: nextID, StudyDigest: in.studyDigest, PreparedDigest: qualifiedPreparedDigest(in, nextID, in.wires[nextID]), NativeRequestDigest: hashBytes(in.wires[nextID]), Status: "held", ChargeBasis: qualifiedPreDispatchChargeBasis, CumulativeChargedMicroUSD: charged, CaseChargedMicroUSD: 0, ErrorClass: "review_incomplete", PriorSpendDigest: in.priorDigest, DeclaredPriorMicroUSD: in.trial.PriorDirectMicroUSD, DeclaredStageCapMicroUSD: in.trial.DirectStageBudgetMicroUSD}
	var got qualifiedReceipt
	if qualifiedReadExactArtifact(root, nextID+".receipt.json", want, &got) != nil {
		return false, errors.New("pre-dispatch receipt missing")
	}
	if !reflect.DeepEqual(got, want) {
		return false, errors.New("pre-dispatch receipt mismatch")
	}
	return true, nil
}
func qualifiedAudit(ctx context.Context, in qualifiedInputs, attempts []journalAttempt, root *os.Root, preDispatch bool, content bool) (qualifiedReport, int, error) {
	r := qualifiedReport{Version: "prose-lab.qualified-review-report/1", Protocol: qualifiedArtifactProtocol(in), StudyDigest: in.studyDigest, FloorReportDigest: hashBytes(jsonBytes(in.floor)), PriorSpendDigest: in.priorDigest, DeclaredPriorMicroUSD: in.trial.PriorDirectMicroUSD, DeclaredStageCapMicroUSD: in.trial.DirectStageBudgetMicroUSD, Rows: make([]qualifiedRow, 0, 85), ChargeBasis: pairedChargeBasis, ConfusionMatrix: map[string]map[string]int{}}
	for _, expected := range []string{"supported", "unsupported", "inconclusive"} {
		r.ConfusionMatrix[expected] = map[string]int{"supported": 0, "unsupported": 0, "inconclusive": 0}
	}
	var cumulative int64
	stop := false
	knownMismatch := false
	for i, id := range in.ordered {
		c := in.cases[id]
		row := qualifiedRow{Position: i + 1, State: "missing"}
		r.Planned++
		if i == len(attempts) && preDispatch {
			row.State = "failed"
		} else if i < len(attempts) {
			a := attempts[i]
			res, o := a.Reservation, a.Outcome
			charged := res.MicroUSD
			if o != nil && o.ActualMicroUSD != nil {
				charged = *o.ActualMicroUSD
			}
			cumulative += charged
			r.ChargedMicroUSD += charged
			if o != nil {
				if o.LatencyNanos < 0 || r.LatencyNanos > math.MaxInt64-o.LatencyNanos {
					return r, 2, errors.New("latency accounting overflow")
				}
				r.LatencyNanos += o.LatencyNanos
			}
			if stop || res.CaseID != id || res.Provider != "anthropic" || res.Model != in.trial.Review.Model || res.Pricing != in.trial.Review || res.MicroUSD != in.trial.Review.reserveCost() || res.RequestHash != hashBytes(in.wires[id]) || res.RequestJSON != string(in.wires[id]) {
				row.State = "invalid"
			} else if o == nil {
				if qualifiedMarkerCheck(root, in, id) != nil {
					row.State = "invalid"
				} else {
					row.State = "pending"
				}
			} else if o.Status == "failed" || o.Status == "unknown" {
				if qualifiedHeldCheck(root, in, id, res, charged, cumulative) != nil {
					row.State = "invalid"
				} else {
					row.State = o.Status
				}
			} else if o.Status != "completed" || o.HTTPStatus < 200 || o.HTTPStatus >= 300 || !o.UsagePresent || o.ActualMicroUSD == nil {
				row.State = "invalid"
			} else {
				d, e := qualifiedReplayFor(ctx, c, in.trial.Review, in.protocol, in.wires[id], []byte(o.Response))
				_, u, cost, ue := readUsage([]byte(o.Response), in.trial.Review)
				if e != nil || ue != nil || u.Input+u.CacheWrite+u.CacheRead == 0 || u.Output == 0 || cost == 0 || cost != *o.ActualMicroUSD || qualifiedArtifactCheck(root, in, id, res, d, charged, cumulative) != nil {
					row.State = "invalid"
				} else {
					row.State = d.Status
					r.ConfusionMatrix[c.Expected][d.Status]++
					r.Completed++
					r.InputTokens += u.Input
					r.OutputTokens += u.Output
					r.CacheWriteTokens += u.CacheWrite
					r.CacheReadTokens += u.CacheRead
					switch d.Status {
					case "inconclusive":
						r.Inconclusive++
					}
					if d.Status == c.Expected {
						r.Matched++
					} else {
						r.Mismatched++
						knownMismatch = true
					}
					if c.Expected != "supported" && d.Status == "supported" {
						r.FalseSupport++
					}
					if c.Expected == "supported" && d.Status != "supported" {
						r.FalseHold++
					}
					if content {
						row.Reason = d.Reason
						if in.protocol == qualifiedProtocolV4 {
							decision := d
							row.Decision = &decision
						}
					}
				}
			}
		}
		switch row.State {
		case "missing":
			r.Missing++
		case "pending":
			r.Pending++
		case "failed":
			r.Failed++
		case "unknown":
			r.Unknown++
		case "invalid":
			r.InvalidCount++
		}
		if row.State == "pending" || row.State == "failed" || row.State == "unknown" || row.State == "invalid" || knownMismatch {
			stop = true
		}
		if content {
			row.CaseID = id
			row.Expected = c.Expected
			row.Assertion = c.Assertion
			row.Facts = c.Facts
		} else if floorExpected(row.State) {
			row.State = "completed"
		}
		r.Rows = append(r.Rows, row)
	}
	if len(attempts) > 85 || cumulative > in.trial.BudgetMicroUSD || in.trial.PriorDirectMicroUSD > in.trial.DirectStageBudgetMicroUSD-cumulative {
		return r, 2, errors.New("history exceeds trial or stage budget")
	}
	r.Qualified = r.Matched == 85 && r.InvalidCount == 0 && r.Pending == 0 && r.Failed == 0 && r.Unknown == 0
	if r.InvalidCount > 0 || r.Pending > 0 || r.Failed > 0 || r.Unknown > 0 {
		return r, 2, nil
	}
	if knownMismatch {
		return r, 1, nil
	}
	if !r.Qualified {
		return r, 2, nil
	}
	return r, 0, nil
}
func qualifiedRunReport(ctx context.Context, in qualifiedInputs, journalPath, artifactPath string, content bool, out, errOut io.Writer) int {
	attempts, charged, e := auditJournal(journalPath, in.boundStudy, in.trial.BudgetMicroUSD)
	if e != nil {
		return diagnostic(errOut, "qualified_journal_refused", 2)
	}
	root, e := existingReviewArtifacts(artifactPath)
	if e != nil {
		return diagnostic(errOut, "qualified_artifacts_refused", 2)
	}
	defer root.Close()
	preDispatch, e := qualifiedArtifactNames(root, in, attempts, charged)
	if e != nil {
		return diagnostic(errOut, "qualified_artifacts_invalid", 2)
	}
	r, code, e := qualifiedAudit(ctx, in, attempts, root, preDispatch, content)
	if e != nil || charged != r.ChargedMicroUSD {
		return diagnostic(errOut, "qualified_history_refused", 2)
	}
	return pairedJSON(ctx, out, r, code)
}
func qualifiedRunCapture(ctx context.Context, in qualifiedInputs, c floorCase, wire []byte, prepared, journalPath, artifactPath string, content bool, out, errOut io.Writer, deps reviewDeps) int {
	if deps.now == nil {
		deps.now = time.Now
	}
	if deps.captureHTTP == nil {
		deps.captureHTTP = func() (*http.Client, *captureTransport) { return studyHTTP("anthropic") }
	}
	root, e := documentArtifactRoot(artifactPath)
	if e != nil {
		return diagnostic(errOut, "qualified_artifacts_refused", 2)
	}
	defer root.Close()
	j, e := openJournal(journalPath, in.boundStudy, in.trial.BudgetMicroUSD, deps.now)
	if e != nil {
		return diagnostic(errOut, "qualified_journal_refused", 2)
	}
	defer j.close()
	reader := &journal{root: j.root, header: j.header, next: 1}
	attempts, e := reader.readHistory(false)
	preDispatch, artifactErr := qualifiedArtifactNames(root, in, attempts, j.charged)
	if e != nil || artifactErr != nil {
		return diagnostic(errOut, "qualified_prior_refused", 2)
	}
	prior, _, e := qualifiedAudit(ctx, in, attempts, root, preDispatch, false)
	if e != nil || len(attempts) >= 85 || in.ordered[len(attempts)] != c.CaseID || prior.InvalidCount > 0 || prior.Pending > 0 || prior.Failed > 0 || prior.Unknown > 0 || prior.Mismatched > 0 || in.trial.Review.reserveCost() > in.trial.BudgetMicroUSD-j.charged || in.trial.PriorDirectMicroUSD > in.trial.DirectStageBudgetMicroUSD-j.charged-in.trial.Review.reserveCost() {
		return diagnostic(errOut, "qualified_order_or_budget_refused", 2)
	}
	marker := map[string]string{"case_id": c.CaseID, "study_digest": in.studyDigest, "prepared_digest": prepared}
	if in.protocol == qualifiedProtocolV4 {
		marker["protocol"] = in.protocol
	}
	if writeExclusive(root, c.CaseID+".started.json", marker) != nil {
		return diagnostic(errOut, "qualified_marker_refused", 2)
	}
	start := j.charged
	receipt := qualifiedReceipt{Version: "prose-lab.qualified-review-capture/1", Protocol: qualifiedArtifactProtocol(in), CaseID: c.CaseID, StudyDigest: in.studyDigest, PreparedDigest: prepared, NativeRequestDigest: hashBytes(wire), Status: "held", ChargeBasis: pairedChargeBasis, PriorSpendDigest: in.priorDigest, DeclaredPriorMicroUSD: in.trial.PriorDirectMicroUSD, DeclaredStageCapMicroUSD: in.trial.DirectStageBudgetMicroUSD}
	persist := func(code string) int {
		receipt.CumulativeChargedMicroUSD = j.charged
		receipt.CaseChargedMicroUSD = j.charged - start
		receipt.ErrorClass = code
		if j.next == len(attempts)+1 {
			receipt.ChargeBasis = qualifiedPreDispatchChargeBasis
		}
		if writeExclusive(root, c.CaseID+".receipt.json", receipt) != nil {
			return diagnostic(errOut, "qualified_receipt_refused", 2)
		}
		return qualifiedCaptureOutput(ctx, out, receipt, content, 2)
	}
	hc, cap := deps.captureHTTP()
	if hc == nil || cap == nil {
		return persist("review_incomplete")
	}
	a, e := anthropic.New(anthropic.Config{APIKey: deps.apiKey, HTTPClient: hc, DefaultModel: in.trial.Review.Model})
	if e != nil {
		return persist("review_incomplete")
	}
	client := &documentTextClient{inner: &labClient{inner: a, attempt: &attemptRunner{journal: j, lane: in.trial.Review, capture: cap, now: deps.now, caseID: c.CaseID, expectedRequestDigest: hashBytes(wire)}}}
	resp, e := client.Complete(ctx, qualifiedRequestFor(c, in.trial.Review, in.protocol))
	if e != nil || len(j.pending) != 0 || resp.StopReason != "end_turn" || len(resp.Content) != 1 || resp.Content[0].Type != "text" {
		return persist("review_incomplete")
	}
	d, e := qualifiedDecodeFor(resp.Text, c, in.protocol)
	if e != nil {
		return persist("review_incomplete")
	}
	var last reservation
	if readJournalJSON(j.root, fmt.Sprintf("reserve-%06d.json", j.next-1), &last) != nil || last.CaseID != c.CaseID || last.RequestHash != hashBytes(wire) {
		return persist("review_incomplete")
	}
	receipt.Status = d.Status
	receipt.CumulativeChargedMicroUSD = j.charged
	receipt.CaseChargedMicroUSD = j.charged - start
	if writeExclusive(root, c.CaseID+".reason.json", d) != nil {
		return persist("review_incomplete")
	}
	if writeExclusive(root, c.CaseID+".receipt.json", receipt) != nil {
		return diagnostic(errOut, "qualified_receipt_refused", 2)
	}
	code := 0
	if d.Status != c.Expected {
		code = 1
	}
	return qualifiedCaptureOutput(ctx, out, receipt, content, code)
}
func qualifiedCaptureOutput(ctx context.Context, out io.Writer, r qualifiedReceipt, content bool, code int) int {
	if content {
		return pairedJSON(ctx, out, r, code)
	}
	payload := map[string]any{"version": r.Version, "study_digest": r.StudyDigest, "prepared_digest": r.PreparedDigest, "native_request_digest": r.NativeRequestDigest, "status": r.Status, "charge_basis": r.ChargeBasis, "cumulative_charged_micro_usd": r.CumulativeChargedMicroUSD, "case_charged_micro_usd": r.CaseChargedMicroUSD, "error_class": r.ErrorClass, "prior_spend_digest": r.PriorSpendDigest, "declared_prior_micro_usd": r.DeclaredPriorMicroUSD, "declared_stage_cap_micro_usd": r.DeclaredStageCapMicroUSD, "cross_trial_spend_verified": false, "provider_spend_limit_enforced": false, "source_authenticated": false, "identity_authenticated": false, "independent_acceptance": false, "publication_authority": false, "ui_entry": false, "model_site_adoption": false}
	if r.Protocol != "" {
		payload["protocol"] = r.Protocol
	}
	return pairedJSON(ctx, out, payload, code)
}
