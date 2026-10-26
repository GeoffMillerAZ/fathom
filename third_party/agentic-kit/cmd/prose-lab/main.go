package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision/typesafe"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient/anthropic"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/internal/jsonwire"
)

type canaryStudy struct {
	Version         string          `json:"version"`
	ID              string          `json:"id"`
	DatasetKind     string          `json:"dataset_kind"`
	BudgetMicroUSD  int64           `json:"budget_micro_usd"`
	Lanes           map[string]lane `json:"lanes"`
	ExpectedCaseIDs []string        `json:"expected_case_ids"`
}

func (s canaryStudy) validate() error {
	if s.Version != "prose-lab.canary/1" || !validLabID(s.ID) || s.DatasetKind != "synthetic" || s.BudgetMicroUSD < 1 || s.BudgetMicroUSD > 10_000_000 || len(s.Lanes) < 1 || len(s.Lanes) > 4 || len(s.ExpectedCaseIDs) < 1 || len(s.ExpectedCaseIDs) > 256 {
		return errors.New("invalid synthetic canary study")
	}
	seen := map[string]bool{}
	for _, id := range s.ExpectedCaseIDs {
		if !validLabID(id) || seen[id] {
			return errors.New("invalid expected canary case roster")
		}
		seen[id] = true
	}
	for id, l := range s.Lanes {
		if !validLabID(id) || l.validate() != nil || l.reserveCost() > s.BudgetMicroUSD {
			return errors.New("invalid canary lane")
		}
	}
	return nil
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(runLab(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

func runLab(ctx context.Context, args []string, out, errOut io.Writer) int {
	if len(args) > 0 && (args[0] == "review-qualified-prepare" || args[0] == "review-qualified-capture" || args[0] == "review-qualified-report") {
		return runQualifiedReview(ctx, args, out, errOut)
	}
	if len(args) > 0 && args[0] == "review-floor-report" {
		return runReviewFloor(ctx, args[1:], out, errOut)
	}
	if len(args) > 0 && (args[0] == "review-oracle-prepare" || args[0] == "review-oracle-report") {
		return runReviewOracle(ctx, args, out, errOut)
	}
	if len(args) > 0 && (args[0] == "review-pair-prepare" || args[0] == "review-pair-capture" || args[0] == "review-pair-report") {
		return runReviewPair(ctx, args, out, errOut)
	}
	if len(args) > 0 && (args[0] == "document-review-prepare" || args[0] == "document-review-report") {
		return runDocumentReview(ctx, args, out, errOut)
	}
	if len(args) > 0 && (args[0] == "review-prepare" || args[0] == "review-capture" || args[0] == "review-report") {
		return runReview(ctx, args, out, errOut)
	}
	if len(args) > 0 && (args[0] == "document-prepare" || args[0] == "document-capture") {
		return runDocument(ctx, args, out, errOut)
	}
	return runCanary(ctx, args, out, errOut)
}

func runCanary(ctx context.Context, args []string, out, errOut io.Writer) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		if _, err := fmt.Fprintln(out, "Usage: prose-lab prepare|capture --study PATH --lane ID --case ID --request PATH [--journal PRIVATE_DIR --live]\n       prose-lab document-prepare|document-capture --study PATH --corpus DIR --cell ID --generation-lane ID --review-lane ID [--prepared-digest SHA256 --journal PRIVATE_DIR --artifacts PRIVATE_DIR --live]\n       prose-lab review-prepare|review-capture|review-report --study PATH --corpus DIR --goal-scope PATH --goal-scope-digest SHA256 --review-scope PATH --review-scope-digest SHA256 --review-lane ID [--case ID --prepared-digest SHA256 --journal PRIVATE_DIR --artifacts PRIVATE_DIR --live]; report requires --journal and --artifacts\n       prose-lab review-pair-prepare|review-pair-capture|review-pair-report --help\n       prose-lab document-review-prepare|document-review-report --help\n       prose-lab review-oracle-prepare|review-oracle-report --help\n       prose-lab review-floor-report --help\n       prose-lab review-qualified-prepare|review-qualified-capture|review-qualified-report --help\nSynthetic transport, positive document candidates, or bounded support review only. Independent acceptance and publication authority remain separate. See cmd/prose-lab/README.md."); err != nil {
			return 2
		}
		return 0
	}
	usage := func() int {
		fmt.Fprintln(errOut, "Usage: prose-lab prepare|capture --study PATH --lane ID --case ID --request PATH [--journal PRIVATE_DIR --live]")
		return 2
	}
	if len(args) < 1 || (args[0] != "prepare" && args[0] != "capture") {
		return usage()
	}
	fs := flag.NewFlagSet("prose-lab", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	studyPath := fs.String("study", "", "synthetic study JSON")
	laneID := fs.String("lane", "", "exact study lane")
	caseID := fs.String("case", "", "predeclared case ID")
	requestPath := fs.String("request", "", "synthetic typed request JSON")
	journalPath := fs.String("journal", "", "private aggregate journal directory")
	live := fs.Bool("live", false, "explicit paid capture; never implied by keys")
	parseErr := fs.Parse(args[1:])
	if errors.Is(parseErr, flag.ErrHelp) {
		return runCanary(ctx, []string{"--help"}, out, errOut)
	}
	if parseErr != nil || fs.NArg() != 0 {
		return usage()
	}
	var study canaryStudy
	raw, err := readStudyJSON(*studyPath, &study)
	if err != nil || study.validate() != nil {
		return diagnostic(errOut, "invalid_study", 2)
	}
	l, ok := study.Lanes[*laneID]
	if !ok {
		return diagnostic(errOut, "unknown_lane", 2)
	}
	found := false
	for _, id := range study.ExpectedCaseIDs {
		if id == *caseID {
			found = true
		}
	}
	if !found {
		return diagnostic(errOut, "foreign_case", 2)
	}
	rawReq, err := readLabBytes(*requestPath, 64<<10)
	if err != nil || len(rawReq) > 64<<10 {
		return diagnostic(errOut, "invalid_request", 2)
	}
	var completion llmclient.Request
	var choice decision.Request
	if l.Provider == "anthropic" {
		if decodeNativeRequest(rawReq, &completion) != nil || completion.Model != l.Model || completion.MaxTokens < 1 || int64(completion.MaxTokens) > l.MaxOutputTokens || completion.Prompt == "" || len(completion.Messages) > 0 || len(completion.Tools) > 0 {
			return diagnostic(errOut, "invalid_generation_request", 2)
		}
	} else if decodeNativeRequest(rawReq, &choice) != nil || decision.Validate(choice) != nil || len(choice.Images) > 0 {
		return diagnostic(errOut, "invalid_decision_request", 2)
	}
	if args[0] == "prepare" {
		if *live || *journalPath != "" {
			return diagnostic(errOut, "prepare_has_no_live_effects", 2)
		}
		if json.NewEncoder(out).Encode(map[string]any{"version": "prose-lab.prepared/1", "study_digest": hashBytes(raw), "request_digest": hashBytes(rawReq), "reservation_micro_usd": l.reserveCost(), "evidence": "transport-only", "live": false}) != nil {
			return 2
		}
		return 0
	}
	if !*live || *journalPath == "" {
		return diagnostic(errOut, "explicit_live_and_private_journal_required", 2)
	}
	j, err := openJournal(*journalPath, raw, study.BudgetMicroUSD, time.Now)
	if err != nil {
		return diagnostic(errOut, "journal_refused", 2)
	}
	defer j.close()
	httpClient, capture := studyHTTP(l.Provider)
	a := &attemptRunner{journal: j, lane: l, capture: capture, now: time.Now, caseID: *caseID}
	if l.Provider == "anthropic" {
		inner, err := anthropic.New(anthropic.Config{HTTPClient: httpClient, DefaultModel: l.Model})
		if err != nil {
			return diagnostic(errOut, "credential_or_client_unavailable", 2)
		}
		_, err = (&labClient{inner: inner, attempt: a}).Complete(ctx, completion)
		if err != nil {
			return diagnostic(errOut, "capture_incomplete_inspect_private_journal", 2)
		}
	} else {
		inner, err := typesafe.NewTypeSafe(typesafe.TypeSafeConfig{APIKeyEnv: "OPENROUTER_API_KEY", Transport: typesafe.TransportOpenRouter, Model: l.Model, HTTPClient: httpClient})
		if err != nil {
			return diagnostic(errOut, "credential_or_client_unavailable", 2)
		}
		_, err = (&labDecider{inner: inner, attempt: a}).Decide(ctx, choice)
		if err != nil {
			return diagnostic(errOut, "capture_incomplete_inspect_private_journal", 2)
		}
	}
	if json.NewEncoder(out).Encode(map[string]any{"version": "prose-lab.capture/1", "status": "captured", "evidence": "transport-only", "charged_micro_usd": j.charged, "publication_authority": false}) != nil {
		return 2
	}
	return 0
}

func diagnostic(w io.Writer, reason string, code int) int {
	fmt.Fprintln(w, "prose-lab:", reason)
	return code
}

// decodeNativeRequest accepts the existing adapter's optional-field contract,
// while rejecting malformed Unicode, duplicate keys, unknown fields and extras.
// Presence/semantic bounds are enforced by the selected adapter/request checks.
func decodeNativeRequest(b []byte, v any) error {
	if _, err := jsonwire.Object(b); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	return nil
}
