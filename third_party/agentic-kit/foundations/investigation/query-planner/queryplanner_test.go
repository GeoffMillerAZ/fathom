package queryplanner_test

import (
	"context"
	"strings"
	"testing"

	queryplanner "github.com/geoffmilleraz/agentic-kit/foundations/investigation/query-planner"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

var emptyRange = queryplanner.DateRange{}

// staticRefiner returns a fixed set of refined queries regardless of input.
type staticRefiner struct {
	queries []queryplanner.RefinedQuery
}

func (r *staticRefiner) Refine(_ context.Context, _ string, _ []string) []queryplanner.RefinedQuery {
	return r.queries
}

// failRefiner always returns nil (simulates LLM error / degradation).
type failRefiner struct{}

func (failRefiner) Refine(_ context.Context, _ string, _ []string) []queryplanner.RefinedQuery {
	return nil
}

// ─── constructor ─────────────────────────────────────────────────────────────

func TestNewPlannerNilScreenAndRefinerOK(t *testing.T) {
	p := queryplanner.NewPlanner(nil, nil)
	if p == nil {
		t.Fatal("NewPlanner returned nil")
	}
}

// ─── empty brief ─────────────────────────────────────────────────────────────

func TestPlanRejectsEmptyBriefText(t *testing.T) {
	p := queryplanner.NewPlanner(nil, nil)
	_, err := p.Plan(context.Background(), queryplanner.Brief{ID: "b1", Text: ""}, nil, emptyRange)
	if err == nil {
		t.Fatal("Plan with empty brief text should return error")
	}
}

// ─── deterministic entity expansion ──────────────────────────────────────────

func TestPlanEntityEmailExpansion(t *testing.T) {
	p := queryplanner.NewPlanner(nil, nil)
	entities := []queryplanner.EntityInfo{
		{DisplayName: "Alice", Emails: []string{"alice@example.com"}},
	}
	brief := queryplanner.Brief{ID: "b1", Text: "Find lease communications."}

	queries, err := p.Plan(context.Background(), brief, entities, emptyRange)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	var gmailQueries []string
	for _, q := range queries {
		if q.Provider == "gmail" {
			gmailQueries = append(gmailQueries, q.Query)
		}
	}
	if len(gmailQueries) == 0 {
		t.Fatal("expected at least one gmail query from entity expansion")
	}
	found := false
	for _, q := range gmailQueries {
		if strings.Contains(q, "from:alice@example.com") && strings.Contains(q, "to:alice@example.com") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected from:/to: terms for alice@example.com; queries: %v", gmailQueries)
	}
}

func TestPlanEntityAliasEmailExtracted(t *testing.T) {
	p := queryplanner.NewPlanner(nil, nil)
	entities := []queryplanner.EntityInfo{
		{DisplayName: "Bob", Aliases: []string{"bob@work.com", "not-an-email"}},
	}
	brief := queryplanner.Brief{ID: "b1", Text: "Investigate Bob's activities."}

	queries, err := p.Plan(context.Background(), brief, entities, emptyRange)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	found := false
	for _, q := range queries {
		if q.Provider == "gmail" && strings.Contains(q.Query, "bob@work.com") {
			found = true
		}
	}
	if !found {
		t.Error("expected email alias bob@work.com in gmail query")
	}
}

func TestPlanEntityDriveQuery(t *testing.T) {
	p := queryplanner.NewPlanner(nil, nil)
	entities := []queryplanner.EntityInfo{
		{DisplayName: "Carol Smith", Emails: []string{"carol@example.com"}},
	}
	brief := queryplanner.Brief{ID: "b1", Text: "Find documents."}

	queries, err := p.Plan(context.Background(), brief, entities, emptyRange)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	found := false
	for _, q := range queries {
		if q.Provider == "drive" && strings.Contains(q.Query, "fullText contains 'Carol Smith'") {
			found = true
		}
	}
	if !found {
		t.Error("expected Drive fullText query for Carol Smith")
	}
}

// ─── date range ──────────────────────────────────────────────────────────────

func TestPlanNoBoundedRangeProducesNoDateClauses(t *testing.T) {
	p := queryplanner.NewPlanner(nil, nil)
	entities := []queryplanner.EntityInfo{
		{DisplayName: "Dave", Emails: []string{"dave@example.com"}},
	}
	brief := queryplanner.Brief{ID: "b1", Text: "Check Dave's emails."}

	// No date range → no after:/before: clauses.
	queries, err := p.Plan(context.Background(), brief, entities, emptyRange)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	for _, q := range queries {
		if q.Provider == "gmail" && (strings.Contains(q.Query, "after:") || strings.Contains(q.Query, "before:")) {
			t.Errorf("unbounded date range produced date clauses: %q", q.Query)
		}
	}
}

// ─── keyword extraction ───────────────────────────────────────────────────────

func TestPlanKeywordExpansion(t *testing.T) {
	p := queryplanner.NewPlanner(nil, nil)
	brief := queryplanner.Brief{ID: "b1", Text: "the security deposit itemization statement was not returned"}

	queries, err := p.Plan(context.Background(), brief, nil, emptyRange)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	var gmailKw, driveKw string
	for _, q := range queries {
		if q.Provider == "gmail" {
			gmailKw = q.Query
		}
		if q.Provider == "drive" {
			driveKw = q.Query
		}
	}
	if gmailKw == "" || driveKw == "" {
		t.Fatalf("expected keyword queries for both providers; gmail=%q drive=%q", gmailKw, driveKw)
	}
	for _, kw := range []string{"security", "deposit", "itemization", "statement", "returned"} {
		if !strings.Contains(gmailKw, kw) {
			t.Errorf("gmail keyword query missing %q: %s", kw, gmailKw)
		}
	}
	// Stopwords excluded.
	for _, sw := range []string{"the", "was", "not"} {
		if strings.Contains(gmailKw, " "+sw+" ") {
			t.Errorf("stopword %q should not appear in keyword query: %s", sw, gmailKw)
		}
	}
	if !strings.Contains(driveKw, "fullText contains 'security'") {
		t.Errorf("drive keyword query malformed: %s", driveKw)
	}
}

// ─── determinism ─────────────────────────────────────────────────────────────

func TestPlanIsDeterministic(t *testing.T) {
	p := queryplanner.NewPlanner(nil, nil)
	entities := []queryplanner.EntityInfo{
		{DisplayName: "Eve", Emails: []string{"eve@example.com"}},
	}
	brief := queryplanner.Brief{ID: "b1", Text: "Find relevant communications."}

	q1, err := p.Plan(context.Background(), brief, entities, emptyRange)
	if err != nil {
		t.Fatalf("Plan (1): %v", err)
	}
	q2, err := p.Plan(context.Background(), brief, entities, emptyRange)
	if err != nil {
		t.Fatalf("Plan (2): %v", err)
	}
	if len(q1) != len(q2) {
		t.Fatalf("Plan produced different lengths: %d vs %d", len(q1), len(q2))
	}
	for i := range q1 {
		if q1[i].Query != q2[i].Query {
			t.Errorf("query[%d] differs: %q vs %q", i, q1[i].Query, q2[i].Query)
		}
	}
}

// ─── LLM refinement ──────────────────────────────────────────────────────────

func TestPlanLLMRefinementAddsAIQueries(t *testing.T) {
	refiner := &staticRefiner{
		queries: []queryplanner.RefinedQuery{
			{Provider: "gmail", Query: "deposit agreement", Rationale: "lease terms"},
		},
	}
	p := queryplanner.NewPlanner(nil, refiner)
	brief := queryplanner.Brief{ID: "b1", Text: "Find lease documents."}

	queries, err := p.Plan(context.Background(), brief, nil, emptyRange)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	var aiQueries []queryplanner.PlannedQuery
	for _, q := range queries {
		if q.IsAI {
			aiQueries = append(aiQueries, q)
		}
	}
	if len(aiQueries) == 0 {
		t.Fatal("expected at least one AI-authored query from LLM refinement")
	}
	if aiQueries[0].Query != "deposit agreement" {
		t.Errorf("AI query = %q, want %q", aiQueries[0].Query, "deposit agreement")
	}
}

func TestPlanLLMRefinementDefaultsToGmail(t *testing.T) {
	refiner := &staticRefiner{
		queries: []queryplanner.RefinedQuery{
			{Provider: "", Query: "lease default", Rationale: "inferred"},
		},
	}
	p := queryplanner.NewPlanner(nil, refiner)
	brief := queryplanner.Brief{ID: "b1", Text: "Investigate lease default."}

	queries, err := p.Plan(context.Background(), brief, nil, emptyRange)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	for _, q := range queries {
		if q.IsAI && q.Provider != "gmail" {
			t.Errorf("AI query with empty Provider should default to gmail; got %q", q.Provider)
		}
	}
}

func TestPlanLLMRefinementDegradeOnNil(t *testing.T) {
	p := queryplanner.NewPlanner(nil, failRefiner{})
	brief := queryplanner.Brief{ID: "b1", Text: "Investigate deposit dispute."}

	queries, err := p.Plan(context.Background(), brief, nil, emptyRange)
	if err != nil {
		t.Fatalf("Plan must not return error when Refiner returns nil: %v", err)
	}
	for _, q := range queries {
		if q.IsAI {
			t.Error("expected no AI queries when Refiner returns nil")
		}
	}
}

func TestPlanLLMRefinementSkipsEmptyQuery(t *testing.T) {
	refiner := &staticRefiner{
		queries: []queryplanner.RefinedQuery{
			{Provider: "gmail", Query: "", Rationale: "empty — should be skipped"},
			{Provider: "gmail", Query: "valid query", Rationale: "good"},
		},
	}
	p := queryplanner.NewPlanner(nil, refiner)
	brief := queryplanner.Brief{ID: "b1", Text: "Find documents."}

	queries, err := p.Plan(context.Background(), brief, nil, emptyRange)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	aiCount := 0
	for _, q := range queries {
		if q.IsAI {
			aiCount++
		}
	}
	if aiCount != 1 {
		t.Errorf("expected 1 AI query (empty skipped), got %d", aiCount)
	}
}

// ─── DLP screening ───────────────────────────────────────────────────────────

func TestPlanDLPWarningsAttached(t *testing.T) {
	// Use DefaultScreen to detect SSN-like pattern in the query.
	p := queryplanner.NewPlanner(queryplanner.DefaultScreen, nil)
	entities := []queryplanner.EntityInfo{
		{DisplayName: "John 123-45-6789 Doe", Emails: []string{"john@example.com"}},
	}
	brief := queryplanner.Brief{ID: "b1", Text: "Find relevant documents."}

	queries, err := p.Plan(context.Background(), brief, entities, emptyRange)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	var driveWarnings []string
	for _, q := range queries {
		if q.Provider == "drive" {
			driveWarnings = append(driveWarnings, q.Warnings...)
		}
	}
	if len(driveWarnings) == 0 {
		t.Error("expected DLP warnings on drive query containing SSN-like pattern")
	}
	found := false
	for _, w := range driveWarnings {
		if strings.Contains(w, "SSN") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected SSN warning in: %v", driveWarnings)
	}
}

func TestDefaultScreenCleanQueryNoWarnings(t *testing.T) {
	warnings := queryplanner.DefaultScreen("(from:alice@example.com OR to:alice@example.com)")
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for clean query, got: %v", warnings)
	}
}

func TestDefaultScreenSSNWarning(t *testing.T) {
	warnings := queryplanner.DefaultScreen("subject contains 123-45-6789")
	if len(warnings) == 0 {
		t.Error("expected SSN warning")
	}
}

// ─── IsAI provenance ─────────────────────────────────────────────────────────

func TestPlanDeterministicQueriesTaggedIsAIFalse(t *testing.T) {
	p := queryplanner.NewPlanner(nil, nil)
	brief := queryplanner.Brief{ID: "b1", Text: "Find lease agreement communications."}
	entities := []queryplanner.EntityInfo{
		{DisplayName: "Alice", Emails: []string{"alice@example.com"}},
	}

	queries, err := p.Plan(context.Background(), brief, entities, emptyRange)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	for _, q := range queries {
		if q.IsAI {
			t.Errorf("deterministic query tagged IsAI=true: %+v", q)
		}
	}
}
