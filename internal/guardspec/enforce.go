// enforce.go — ADR-038's deterministic pre-write checks (docs/vision/29-
// verbs-not-lookups.md §2 thesis 2, wave G1): each one a small, pure
// function over a proposed edit's own repo-relative path and text, never
// an LLM call, never anything mutable. The PreToolUse hook
// (cmd/fathom/enforce.go) runs these against every enforce:true guard or
// layer rule governing the target and turns the first violation into a
// deny; `fathom rules gen-test` and this repo's own gate never see these
// functions at all — they are ADDITIONAL to, never a replacement for, the
// existing index-backed EvaluateLayerRule and the compiled-test path.
//
// Every check works on FRAGMENTS, not necessarily whole valid Go files:
// Edit's new_string/old_string, MultiEdit's concatenated edits, or Write's
// whole content. A regex-based scan (rather than go/parser) is deliberate
// — a fragment is frequently not parseable on its own, and a false
// negative here is recoverable (the advisory obligation, the compiled
// generated test, and code review all still see the final tree); a false
// positive would deny a legitimate edit, so every check stays narrow and
// says so in its own doc comment.
package guardspec

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/filter"
	"github.com/geoffmilleraz/fathom/internal/pathglob"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
)

// EnforceViolation is one deterministic check's finding: enough for the
// PreToolUse hook to render a single-line deny reason — the rule, its
// citation, and the remedy — never a wall of text (design decision, ADR-038
// item 3).
type EnforceViolation struct {
	// CheckID names what found this: a guard's own "checks" entry
	// ("org_token", "secret_literal", "workspace_output",
	// "usecase_maintained"), "forbidden_path", a layer rule's kind
	// ("import_direction", "boundary"), or "cgo_purity".
	CheckID string
	// RuleID is the layer rule id, or the guard's own Note (a guard has no
	// separate id field) — whichever names the specific rule that fired.
	RuleID string
	// Cite is the rule's own citation text (an ADR number, a sentence) —
	// served verbatim in the deny reason.
	Cite string
	// Message is the one-line violation description.
	Message string
	// Remedy is the one-line fix.
	Remedy string
}

// EvaluateGuardEnforcement runs g's enforce:true checks against one
// proposed file edit. Returns nil when g is not enforced, its scopes do
// not match repoRelPath (file checks) or match nothing (forbidden_paths),
// or every named check passes. newText is the text being written
// (Write's whole content, or Edit/MultiEdit's concatenated new_string(s));
// oldText is the text being replaced ("" for Write) — some checks
// (usecase_maintained) need both to see something being REMOVED.
func EvaluateGuardEnforcement(g Guard, repoRelPath, newText, oldText string) []EnforceViolation {
	if !g.Enforce {
		return nil
	}
	var out []EnforceViolation
	if len(g.ForbiddenPaths) > 0 && pathglob.MatchAny(g.ForbiddenPaths, repoRelPath) {
		out = append(out, EnforceViolation{
			CheckID: "forbidden_path",
			RuleID:  guardRuleID(g),
			Cite:    projectdir.Guard.Rel(),
			Message: fmt.Sprintf("%s is a forbidden_paths entry — no write to it is allowed, whatever the content", repoRelPath),
			Remedy:  firstNonEmpty(g.Note, "see the governing guard's note in .fathom/guard.cue"),
		})
	}
	if len(g.Scopes) == 0 || !filter.New(g.Scopes).Match(repoRelPath) {
		return out
	}
	for _, check := range g.Checks {
		switch check {
		case "org_token":
			out = append(out, checkOrgToken(repoRelPath, newText)...)
		case "secret_literal":
			out = append(out, checkSecretLiteral(repoRelPath, newText)...)
		case "workspace_output":
			if v := checkWorkspaceOutput(repoRelPath, newText); v != nil {
				out = append(out, *v)
			}
		case "usecase_maintained":
			if v := checkUsecaseMaintained(repoRelPath, oldText, newText); v != nil {
				out = append(out, *v)
			}
		}
	}
	return out
}

// EvaluateCommandEnforcement reports the violation for a command guard g
// that the CALLER has already matched against a command (MatchCommandGuard; for
// a command guard, matching command_scopes at all IS the violation, design
// §2.5's own shape: a command guard serves only its note). nil when g is
// not enforced.
func EvaluateCommandEnforcement(g Guard) *EnforceViolation {
	if !g.Enforce || len(g.CommandScopes) == 0 {
		return nil
	}
	cite := projectdir.Guard.Rel()
	if g.commandCheck != "" {
		cite = "git-safety"
	}
	return &EnforceViolation{
		CheckID: "command_match",
		RuleID:  guardRuleID(g),
		Cite:    cite,
		Message: "this command matches an enforced command guard",
		Remedy:  firstNonEmpty(g.Note, "see the governing guard's note in .fathom/guard.cue"),
	}
}

func guardRuleID(g Guard) string {
	if g.commandCheck != "" {
		return "git-safety/" + g.commandCheck
	}
	if g.Note != "" {
		return g.Note
	}
	if len(g.Scopes) > 0 {
		return strings.Join(g.Scopes, ",")
	}
	return strings.Join(g.CommandScopes, ",")
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// ─── checks ───────────────────────────────────────────────────────────────

// checkOrgToken runs the ADR-011 scanner (orgcheck.go) line by line over
// newText.
func checkOrgToken(repoRelPath, newText string) []EnforceViolation {
	var out []EnforceViolation
	for i, line := range strings.Split(newText, "\n") {
		hits := OrgHits(line, nil)
		if len(hits) == 0 {
			continue
		}
		out = append(out, EnforceViolation{
			CheckID: "org_token",
			RuleID:  "I19",
			Cite:    "ADR-011",
			Message: fmt.Sprintf("%s:~%d carries an org-specific token: %s", repoRelPath, i+1, strings.Join(hits, ", ")),
			Remedy:  "use a neutral placeholder (example.com, fauxtown, an RFC 2606 name) — ADR-011",
		})
	}
	return out
}

// secretPatterns is a small, deliberately narrow set of secret-SHAPED
// literals — never a general entropy scan, which would drown in false
// positives on hashes/UUIDs/test fixtures. AWS access/secret keys, a
// GitHub token, a Slack token, and a PEM private-key header.
var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),             // AWS access key id
	regexp.MustCompile(`\bASIA[0-9A-Z]{16}\b`),             // AWS STS access key id
	regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{36,}\b`),   // GitHub token (classic/fine-grained prefixes)
	regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{10,}\b`), // Slack token
	regexp.MustCompile(`-----BEGIN (?:RSA |EC |OPENSSH |DSA )?PRIVATE KEY-----`),
}

// checkSecretLiteral scans newText for a secret-shaped literal. Not tied
// to any of the eight guards-fathom.json tasks (ADR-038 item 4 names which
// tasks each check covers) — provided as a general primitive any guard's
// `checks` list can name.
func checkSecretLiteral(repoRelPath, newText string) []EnforceViolation {
	var out []EnforceViolation
	for _, re := range secretPatterns {
		if m := re.FindString(newText); m != "" {
			out = append(out, EnforceViolation{
				CheckID: "secret_literal",
				RuleID:  "I28-secrets",
				Cite:    "no secret in fixtures",
				Message: fmt.Sprintf("%s contains a secret-shaped literal", repoRelPath),
				Remedy:  "use a placeholder token, never a real-shaped credential, even in a fixture",
			})
			break // one citation is enough; never echo the secret itself
		}
	}
	return out
}

// workspaceOutputCallRe matches the three write-a-file-outside-workspace/
// calls the check watches for.
var workspaceOutputCallRe = regexp.MustCompile(`\bos\.(?:WriteFile|Create|MkdirAll)\(`)

// checkWorkspaceOutput flags newText adding a call to os.WriteFile/
// os.Create/MkdirAll while never routing through the shared workspace-path
// helpers (workspacePath(/stateLogPath() and never even mentioning
// "workspace" — I31's convention (docs/intent/invariants.md). Deliberately
// narrow (item 2's own instruction): a test file legitimately writes
// fixtures outside workspace/ via t.TempDir(), so _test.go files are
// skipped outright, and the check only fires when NEITHER helper name NOR
// the word "workspace" appears anywhere in newText — a file that already
// routes some paths through the helper is left alone even if this
// particular fragment does not repeat that call.
func checkWorkspaceOutput(repoRelPath, newText string) *EnforceViolation {
	if strings.HasSuffix(repoRelPath, "_test.go") {
		return nil
	}
	if !workspaceOutputCallRe.MatchString(newText) {
		return nil
	}
	if strings.Contains(newText, "workspacePath(") || strings.Contains(newText, "stateLogPath(") || strings.Contains(strings.ToLower(newText), "workspace") {
		return nil
	}
	return &EnforceViolation{
		CheckID: "workspace_output",
		RuleID:  "I31",
		Cite:    "docs/intent/invariants.md#L62",
		Message: fmt.Sprintf("%s writes to a hardcoded path outside workspace/ (os.WriteFile/os.Create/MkdirAll with no workspacePath()/stateLogPath() call in sight)", repoRelPath),
		Remedy:  "route the default output path through workspacePath()/stateLogPath() (cmd/fathom/defaultpaths.go) — I31",
	}
}

// checkUsecaseMaintained flags an edit to docs/use-cases/*.md that REMOVES
// the "## Maintained" line: oldText had it, newText (the same fragment's
// replacement) does not. A brand-new doc that never had one is not
// flagged here — "removes" is the literal contract (I36's own gate,
// TestMaintainedLinesParse, still catches a doc that never gained one).
func checkUsecaseMaintained(repoRelPath, oldText, newText string) *EnforceViolation {
	if path.Dir(repoRelPath) != "docs/use-cases" || !strings.HasSuffix(repoRelPath, ".md") {
		return nil
	}
	if !strings.Contains(oldText, "## Maintained") || strings.Contains(newText, "## Maintained") {
		return nil
	}
	return &EnforceViolation{
		CheckID: "usecase_maintained",
		RuleID:  "I36",
		Cite:    "docs/intent/invariants.md#L64",
		Message: fmt.Sprintf("%s: this edit removes the doc's \"## Maintained\" line", repoRelPath),
		Remedy:  "keep (or restore) the \"## Maintained\" contract naming only registered tools and dispatched verbs — I36",
	}
}

// ─── layer-rule content checks ─────────────────────────────────────────────

// importLineRe matches a bare quoted import path on its own line (the
// shape inside a parenthesized import block) or a single `import "path"`
// line — deliberately regex-based, not go/parser, so it still matches an
// Edit/MultiEdit fragment that is not a complete, parseable Go file on its
// own (see this file's package doc comment).
var importLineRe = regexp.MustCompile(`(?m)^\s*(?:import\s+)?"([A-Za-z0-9_./-]+)"\s*$`)

// cgoImportRe / cgoBuildTagRe: the two shapes that make a file cgo-backed.
var (
	cgoImportRe   = regexp.MustCompile(`(?m)^\s*(?:import\s+)?"C"\s*$`)
	cgoBuildTagRe = regexp.MustCompile(`(?m)^\s*(?://go:build.*\bcgo\b|//\s*\+build.*\bcgo\b)`)
)

// EvaluateLayerRuleEnforcement runs rule's deterministic pre-write check
// against one proposed file edit's own text: for kind import_direction or
// boundary, the import paths textually present in newText (importLineRe);
// alongside that, for ANY enforced import_direction rule, a direct
// cgo-purity scan keyed off the rule's own `from` glob (reusing the same
// field import_direction already carries — no schema addition needed):
// `import "C"` or a `//go:build cgo` / `// +build cgo` constraint appearing
// in a file under one of rule.From is a violation regardless of the rule's
// `to` list, because ANY cgo import violates a "stays pure Go" rule, not
// only one naming a specific cgo-backed package. modulePrefix (ModulePrefix)
// strips a resolved import path down to a repo-relative directory; an
// import outside the module is skipped (never repo-internal, so no From/To
// glob could ever match it). nil for a non-enforced rule or a kind other
// than import_direction/boundary (LoadLayerRules already refuses enforce:true
// on any other kind, so a caller iterating loaded rules never reaches this
// case, but the function stays total).
func EvaluateLayerRuleEnforcement(rule LayerRule, modulePrefix, repoRelPath, newText string) []EnforceViolation {
	if !rule.Enforce {
		return nil
	}
	dir := path.Dir(repoRelPath)
	var out []EnforceViolation

	if rule.Kind == ImportDirection && pathglob.MatchAny(rule.From, dir) {
		if cgoImportRe.MatchString(newText) || cgoBuildTagRe.MatchString(newText) {
			out = append(out, EnforceViolation{
				CheckID: "cgo_purity",
				RuleID:  rule.ID,
				Cite:    rule.Sentence,
				Message: fmt.Sprintf("%s adds a cgo import or build tag under %s, which %s forbids", repoRelPath, dir, rule.ID),
				Remedy:  rule.Sentence,
			})
		}
	}

	switch rule.Kind {
	case ImportDirection, Boundary:
	default:
		return out
	}
	for _, m := range importLineRe.FindAllStringSubmatch(newText, -1) {
		imp := m[1]
		if modulePrefix == "" || !strings.HasPrefix(imp, modulePrefix) {
			continue // not repo-internal — no From/To/Allow/Guarded glob is about it
		}
		toDir := strings.TrimPrefix(imp, modulePrefix)
		switch rule.Kind {
		case ImportDirection:
			if pathglob.MatchAny(rule.From, dir) && pathglob.MatchAny(rule.To, toDir) {
				out = append(out, EnforceViolation{
					CheckID: "import_direction",
					RuleID:  rule.ID,
					Cite:    rule.Sentence,
					Message: fmt.Sprintf("%s (%s) imports %s: %s", repoRelPath, dir, toDir, rule.Sentence),
					Remedy:  rule.Sentence,
				})
			}
		case Boundary:
			if !pathglob.MatchAny(rule.Allow, dir) && pathglob.MatchAny(rule.Guarded, toDir) {
				out = append(out, EnforceViolation{
					CheckID: "boundary",
					RuleID:  rule.ID,
					Cite:    rule.Sentence,
					Message: fmt.Sprintf("%s (%s) imports guarded package %s without being on the allow list: %s", repoRelPath, dir, toDir, rule.Sentence),
					Remedy:  rule.Sentence,
				})
			}
		}
	}
	return out
}

// ModulePrefix reads "<repoRoot>/go.mod"'s module line and returns its
// import path with a trailing "/" ("github.com/geoffmilleraz/fathom/") —
// the prefix EvaluateLayerRuleEnforcement strips off a resolved import
// path to get a repo-relative directory. "" (no error) when go.mod is
// absent or has no module line: a repo fathom indexes without a go.mod
// (or a non-Go repo) simply never matches any repo-internal import, which
// is the correct, safe degrade — never a false violation.
func ModulePrefix(repoRoot string) (string, error) {
	data, err := os.ReadFile(filepath.Join(repoRoot, "go.mod"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if mod, ok := strings.CutPrefix(line, "module "); ok {
			return strings.TrimSpace(mod) + "/", nil
		}
	}
	return "", nil
}
