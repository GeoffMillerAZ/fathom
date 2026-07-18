package queryplanner

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// Planner builds search queries from a brief, using deterministic entity
// expansion and optional LLM refinement via a caller-supplied Refiner seam.
type Planner struct {
	screen  ScreenFunc // may be nil
	refiner Refiner    // may be nil
}

// NewPlanner constructs a Planner.
//   - screen: optional DLP screen function; pass nil to skip screening.
//   - refiner: optional LLM refinement seam; pass nil to disable Phase 2.
func NewPlanner(screen ScreenFunc, refiner Refiner) *Planner {
	return &Planner{screen: screen, refiner: refiner}
}

// Plan builds planned queries from a brief, a set of known entities, and an
// optional date range. It runs in two phases:
//
//  1. Deterministic: entity email expansion → per-provider from:/to: queries;
//     keyword extraction from the brief text → content queries. Both subsets
//     always run and always produce useful output without a network call.
//  2. LLM refinement (if a Refiner was supplied): the Refiner receives the
//     brief text and a summary of existing query strings and may return
//     additional queries tagged IsAI=true. On any Refiner error, refinement
//     is skipped and the deterministic queries are returned unchanged.
//
// If screen is non-nil it is applied to each query string; any warnings are
// attached to the PlannedQuery.
//
// Plan returns an error only if the brief is invalid (empty Text). All other
// failure modes (entity list empty, Refiner error) degrade gracefully.
func (p *Planner) Plan(ctx context.Context, brief Brief, entities []EntityInfo, dr DateRange) ([]PlannedQuery, error) {
	if brief.Text == "" {
		return nil, fmt.Errorf("queryplanner: Plan: brief.Text must not be empty")
	}

	var queries []PlannedQuery

	// Phase 1a: entity email expansion.
	for _, e := range entities {
		emails := extractEmails(e)
		if len(emails) > 0 {
			gmailQ := buildGmailQuery(emails, dr)
			gq := PlannedQuery{
				Provider:  "gmail",
				Query:     gmailQ,
				Rationale: fmt.Sprintf("emails involving entity %s", e.DisplayName),
				IsAI:      false,
			}
			gq.Warnings = p.screen_(gmailQ)
			queries = append(queries, gq)
		}
		if e.DisplayName != "" {
			driveQ := fmt.Sprintf("fullText contains '%s'", strings.ReplaceAll(e.DisplayName, "'", "\\'"))
			dq := PlannedQuery{
				Provider:  "drive",
				Query:     driveQ,
				Rationale: fmt.Sprintf("documents mentioning entity %s", e.DisplayName),
				IsAI:      false,
			}
			dq.Warnings = p.screen_(driveQ)
			queries = append(queries, dq)
		}
	}

	// Phase 1b: keyword extraction from the brief text.
	if kws := extractKeywords(brief.Text, 6); len(kws) > 0 {
		gmailKw := "(" + strings.Join(kws, " OR ") + ")"
		gq := PlannedQuery{
			Provider:  "gmail",
			Query:     gmailKw,
			Rationale: "messages mentioning keywords from the brief",
			IsAI:      false,
		}
		gq.Warnings = p.screen_(gmailKw)
		queries = append(queries, gq)

		driveParts := make([]string, 0, len(kws))
		for _, kw := range kws {
			driveParts = append(driveParts, "fullText contains '"+kw+"'")
		}
		driveKw := strings.Join(driveParts, " or ")
		dq := PlannedQuery{
			Provider:  "drive",
			Query:     driveKw,
			Rationale: "documents mentioning keywords from the brief",
			IsAI:      false,
		}
		dq.Warnings = p.screen_(driveKw)
		queries = append(queries, dq)
	}

	// Phase 2: LLM refinement.
	if p.refiner != nil {
		existing := make([]string, len(queries))
		for i, q := range queries {
			existing[i] = q.Query
		}
		refined := p.refiner.Refine(ctx, brief.Text, existing)
		for _, r := range refined {
			if r.Query == "" {
				continue
			}
			provider := r.Provider
			if provider == "" {
				provider = "gmail"
			}
			pq := PlannedQuery{
				Provider:  provider,
				Query:     r.Query,
				Rationale: r.Rationale,
				IsAI:      true,
			}
			pq.Warnings = p.screen_(pq.Query)
			queries = append(queries, pq)
		}
	}

	return queries, nil
}

// screen_ applies the screen function if set, returning nil if not set.
func (p *Planner) screen_(query string) []string {
	if p.screen == nil {
		return nil
	}
	return p.screen(query)
}

// ─── deterministic helpers ───────────────────────────────────────────────────

// DefaultScreen is a ready-to-use ScreenFunc that detects SSN-like and phone
// number patterns in query strings and returns DLP warning messages.
// It can be passed to NewPlanner as the screen argument.
//
// False-positive reduction: phone number detection strips from:/to: email
// context terms first so that email addresses containing digit runs do not
// trigger spurious warnings.
func DefaultScreen(query string) []string {
	var warnings []string
	if ssnPattern.MatchString(query) {
		warnings = append(warnings, "query may contain SSN-like pattern")
	}
	stripped := emailTermRE.ReplaceAllString(query, "")
	if phonePattern.MatchString(stripped) {
		warnings = append(warnings, "query may contain phone number pattern")
	}
	return warnings
}

var (
	ssnPattern   = regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)
	phonePattern = regexp.MustCompile(`\b(\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4})\b`)
	emailTermRE  = regexp.MustCompile(`(?:from|to):[^\s)]+`)
	// keywordSplitRE splits free text into candidate tokens on non-alphanumeric runs.
	keywordSplitRE = regexp.MustCompile(`[^a-zA-Z0-9]+`)
)

// stopwords is the set of common words excluded from keyword extraction.
var stopwords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "from": true, "that": true,
	"this": true, "about": true, "between": true, "into": true, "over": true, "under": true,
	"any": true, "all": true, "are": true, "was": true, "were": true, "has": true, "have": true,
	"had": true, "his": true, "her": true, "you": true, "your": true, "our": true, "their": true,
	"them": true, "they": true, "who": true, "what": true, "when": true, "where": true,
	"which": true, "would": true, "could": true, "should": true, "will": true, "not": true,
	"evidence": true, "email": true, "emails": true, "document": true, "documents": true,
	"record": true, "records": true, "regarding": true, "related": true, "obtain": true,
}

// extractKeywords pulls significant search terms from free text: lowercased,
// at least 3 characters, not a stopword, deduped, original order preserved,
// capped at maxKeywords.
func extractKeywords(text string, maxKeywords int) []string {
	parts := keywordSplitRE.Split(strings.ToLower(text), -1)
	seen := make(map[string]bool, len(parts))
	var out []string
	for _, p := range parts {
		if len(p) < 3 || stopwords[p] || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
		if maxKeywords > 0 && len(out) >= maxKeywords {
			break
		}
	}
	return out
}

// extractEmails collects unique email addresses from an EntityInfo,
// pulling from both Emails and email-shaped Aliases.
func extractEmails(e EntityInfo) []string {
	var emails []string
	emails = append(emails, e.Emails...)
	for _, alias := range e.Aliases {
		if strings.Contains(alias, "@") {
			emails = append(emails, alias)
		}
	}
	return dedupStrings(emails)
}

// buildGmailQuery constructs a Gmail from:/to: query for the given emails,
// optionally bounded by the DateRange.
func buildGmailQuery(emails []string, dr DateRange) string {
	var parts []string
	for _, e := range emails {
		parts = append(parts, "from:"+e, "to:"+e)
	}
	q := "(" + strings.Join(parts, " OR ") + ")"
	if !dr.Start.IsZero() {
		q += " after:" + dr.Start.Format("2006/01/02")
	}
	if !dr.End.IsZero() {
		q += " before:" + dr.End.Format("2006/01/02")
	}
	return q
}

// dedupStrings returns ss with duplicates removed, preserving order.
func dedupStrings(ss []string) []string {
	seen := make(map[string]bool, len(ss))
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
