package intent

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/adr"
)

// audit.go — EP-7/EP-8: what the corpus CLAIMS versus what any instrument
// can show, and the list of claims nothing shows.
//
// The output is a list with remedies, never a score. A number invites
// gaming and hides which claim is false; a list is the shape that got acted
// on here (the verify gate's failing rows, doctor's remedies, plans next).
// Nothing in this file sums anything into a grade.

// SymbolOracle answers "does the index know this symbol?" — the one
// question this package cannot answer from disk. The caller injects it (the
// daemon over MCP, or a store-direct client), so the package stays pure and
// a test needs no daemon.
type SymbolOracle interface {
	// Known returns, for each requested name, whether the index holds it. A
	// name ending in "*" is a prefix query (invariants.md writes
	// "TestQuery_StalenessReport_*" for a family).
	Known(names []string) (map[string]bool, error)
}

// Claim is one statement the repository makes that no instrument proves.
// Every claim carries the remedy, because a finding without one is a
// complaint.
type Claim struct {
	Kind     string // U1…U6, U11
	Ref      string // the row, condition, or file:line the claim lives at
	Detail   string
	Remedy   string
	Evidence string // what was checked, and when it was measured
}

// Audit is one run's findings.
type Audit struct {
	Root string
	// Unproven is the list, sorted by kind then ref.
	Unproven []Claim
	// Unenforced are invariants at `stated` or `reviewed`: honest rungs, NOT
	// debt. The practice's own header says an invariant nobody checks must
	// say so; pressuring an author to upgrade or delete such a row is how
	// honest rows get quietly faked. Listed, never counted.
	Unenforced []Invariant
	// Uncomputable names every source that could not run, with the reason —
	// a source that silently contributes nothing is indistinguishable from
	// one that found nothing (law 3).
	Uncomputable []string
	// ByKind is the count per kind, for the ledger row only.
	ByKind map[string]int
	// Annotated is how many conditions carry an oracle — K0's demand signal.
	Annotated int
	Elapsed   time.Duration
}

// PlanRowState is one gated plan row's current verdict, supplied by the
// caller from the verify ledger (never re-verified here: verifying 45 plans
// takes 70 seconds, which is not a thing a doctor row or a fleet column may
// do).
type PlanRowState struct {
	Plan       string
	ID         string
	Claimed    bool
	Failing    bool
	Unknown    bool // claimed but carrying no contract
	MeasuredAt string
}

// AuditInput is everything an audit needs beyond the corpus.
type AuditInput struct {
	Corpus       Corpus
	Symbols      SymbolOracle   // nil → U4/U11's index half is uncomputable
	PlanRows     []PlanRowState // empty → U1/U2 uncomputable
	PlanNote     string         // why PlanRows is empty, when it is
	Ledger       []LedgerRow
	Now          time.Time
	IncidentDays int // U6's window; 0 → 30, the practice's own check-14 threshold
}

// RunAudit computes the findings. Deterministic: every list is sorted, the
// clock is injected, and no map iteration reaches the output.
func RunAudit(in AuditInput) Audit {
	start := time.Now()
	if in.Now.IsZero() {
		in.Now = time.Now().UTC()
	}
	if in.IncidentDays == 0 {
		in.IncidentDays = 30
	}
	a := Audit{Root: in.Corpus.Root, ByKind: map[string]int{}}

	a.auditPlanRows(in)
	a.auditInvariants(in)
	a.auditConditions(in)
	a.auditIncidents(in)
	a.auditReferences(in)

	sort.SliceStable(a.Unproven, func(i, j int) bool {
		if a.Unproven[i].Kind != a.Unproven[j].Kind {
			return a.Unproven[i].Kind < a.Unproven[j].Kind
		}
		return a.Unproven[i].Ref < a.Unproven[j].Ref
	})
	for _, c := range a.Unproven {
		a.ByKind[c.Kind]++
	}
	for _, c := range in.Corpus.Conditions {
		if c.Annotated() {
			a.Annotated++
		}
	}
	sort.Strings(a.Uncomputable)
	a.Elapsed = time.Since(start)
	return a
}

// U1/U2 — plan rows that fail, or claim done with no contract. Read from
// the verify ledger at HEAD, with the measurement's age carried through, so
// a stale reading is visible as stale rather than presented as now.
func (a *Audit) auditPlanRows(in AuditInput) {
	if len(in.PlanRows) == 0 {
		note := in.PlanNote
		if note == "" {
			note = "no verify-gate run recorded for this repo"
		}
		a.Uncomputable = append(a.Uncomputable, "U1/U2 (plan rows): "+note)
		return
	}
	for _, r := range in.PlanRows {
		switch {
		case r.Failing:
			a.Unproven = append(a.Unproven, Claim{
				Kind: "U1", Ref: r.Plan + " " + r.ID,
				Detail:   "claimed [x] but the index refutes it",
				Remedy:   "fix the code or uncheck the box",
				Evidence: "verify ledger " + r.MeasuredAt,
			})
		case r.Claimed && r.Unknown:
			a.Unproven = append(a.Unproven, Claim{
				Kind: "U2", Ref: r.Plan + " " + r.ID,
				Detail:   "claimed [x] with no acceptance contract — the gate proves nothing about it",
				Remedy:   "anchor a symbol, or mark skip_test with a skip: reason naming the real evidence",
				Evidence: "verify ledger " + r.MeasuredAt,
			})
		}
	}
}

// U3/U4 — invariants that say they are false, and invariants whose named
// test or gate step does not exist.
func (a *Audit) auditInvariants(in AuditInput) {
	var wanted []string
	for _, inv := range in.Corpus.Invariants {
		switch inv.Rung {
		case RungViolated:
			a.Unproven = append(a.Unproven, Claim{
				Kind: "U3", Ref: inv.ID,
				Detail:   trimTo(inv.Statement, 140),
				Remedy:   "fix the code or restate the invariant to what is true",
				Evidence: inv.File + ":" + itoa(inv.Line),
			})
		case RungStated, RungReviewed:
			a.Unenforced = append(a.Unenforced, inv)
		}
		wanted = append(wanted, inv.Tests...)
	}
	for _, s := range in.Corpus.Surfaces {
		wanted = append(wanted, s.Tests...)
	}
	wanted = uniqueStrings(wanted)

	// Gate steps resolve from disk — no daemon needed.
	targets := taskTargets(in.Corpus.Root)
	hooks := lefthookSteps(in.Corpus.Root)
	for _, inv := range in.Corpus.Invariants {
		for _, g := range inv.GateSteps {
			if gateStepExists(g, targets, hooks) {
				continue
			}
			a.Unproven = append(a.Unproven, Claim{
				Kind: "U4", Ref: inv.ID,
				Detail:   "names gate step `" + g + "`, which does not exist",
				Remedy:   "add the step, or correct the invariant's Enforced-by cell",
				Evidence: inv.File + ":" + itoa(inv.Line),
			})
		}
	}

	if in.Symbols == nil {
		if len(wanted) > 0 {
			a.Uncomputable = append(a.Uncomputable,
				"U4 (named tests): no index available — the daemon is unreachable or no store was given")
		}
		return
	}
	known, err := in.Symbols.Known(wanted)
	if err != nil {
		a.Uncomputable = append(a.Uncomputable, "U4 (named tests): index lookup failed: "+err.Error())
		return
	}
	for _, inv := range in.Corpus.Invariants {
		for _, tn := range inv.Tests {
			if known[tn] {
				continue
			}
			a.Unproven = append(a.Unproven, Claim{
				Kind: "U4", Ref: inv.ID,
				Detail:   "claims `" + tn + "` enforces it, but the index does not know that test",
				Remedy:   "write the test, fix the name, or restate the rung",
				Evidence: inv.File + ":" + itoa(inv.Line),
			})
		}
	}
	for _, s := range in.Corpus.Surfaces {
		for _, tn := range s.Tests {
			if known[tn] {
				continue
			}
			a.Unproven = append(a.Unproven, Claim{
				Kind: "U4", Ref: strings.Join(s.Globs, " "),
				Detail:   "the router's Done-needs names `" + tn + "`, which the index does not know",
				Remedy:   "write the test, fix the name, or correct the Done-needs cell",
				Evidence: in.Corpus.RouterPath + ":" + itoa(s.Line),
			})
		}
	}
}

// U5 — conditions that fired and were never decided. Only annotated
// conditions can fire; prose is human-judged, always.
func (a *Audit) auditConditions(in AuditInput) {
	for _, c := range in.Corpus.Conditions {
		if !c.Annotated() || c.Terminal {
			continue
		}
		row, ok := FirstSeen(in.Ledger, c.ID, "fired")
		if !ok {
			continue
		}
		a.Unproven = append(a.Unproven, Claim{
			Kind: "U5", Ref: c.ID,
			Detail:   c.Label + ": " + trimTo(c.Text, 120),
			Remedy:   "decide it: record the outcome on the record (intent-decide), or amend the condition",
			Evidence: "first fired " + row.At + " on this machine",
		})
	}
}

// U6 — open incidents whose Outcome is absent or still "pending" past the
// window. The practice's own intent-doctor uses 30 days; fathom cites that
// threshold rather than inventing one.
func (a *Audit) auditIncidents(in AuditInput) {
	rel := filepath.Join(routerRelDir, "incidents.md")
	body, err := os.ReadFile(filepath.Join(in.Corpus.Root, rel))
	if err != nil {
		return
	}
	cutoff := in.Now.AddDate(0, 0, -in.IncidentDays)
	lines := splitLines(string(body))
	inOpen := false
	cur, curLine, curDate := "", 0, time.Time{}
	outcome := ""
	flush := func() {
		if cur == "" || !inOpen {
			return
		}
		if outcome != "" && !strings.HasPrefix(strings.ToLower(outcome), "pending") {
			return
		}
		if !curDate.IsZero() && curDate.After(cutoff) {
			return
		}
		detail := "open with no outcome artifact"
		if outcome != "" {
			detail = "outcome still " + trimTo(outcome, 80)
		}
		a.Unproven = append(a.Unproven, Claim{
			Kind: "U6", Ref: cur,
			Detail:   detail,
			Remedy:   "drive it to an invariant, a test, or a decision (intent-incident)",
			Evidence: rel + ":" + itoa(curLine),
		})
	}
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "## ") {
			flush()
			cur, outcome = "", ""
			inOpen = strings.EqualFold(t, "## Open")
			continue
		}
		if m := incidentRe.FindStringSubmatch(t); m != nil {
			flush()
			cur, curLine, outcome = m[1], i+1, ""
			curDate = time.Time{}
			if d := dateRe.FindString(t); d != "" {
				if parsed, err := time.Parse("2006-01-02", d); err == nil {
					curDate = parsed
				}
			}
			continue
		}
		if strings.HasPrefix(t, "Outcome:") && cur != "" {
			outcome = strings.TrimSpace(strings.TrimPrefix(t, "Outcome:"))
		}
	}
	flush()
}

// U11 — backticked repo paths in the corpus that no longer resolve. "A
// decision that cites code that moved" is exactly the drift the practice's
// hand walk misses.
//
// Three things a naive version gets wrong, all found by running it:
//   - a pointer is relative to the CITING FILE, not the repo root (the
//     router writes `../design/product-vision.md` and `principles.md`);
//   - `~/.fathom/…` and absolute paths are not tree paths at all;
//   - a SUPERSEDED record citing a deleted file is correct by construction
//     (ADR-003 cites internal/store/duckdb.go; ADR-025 deleted it, and the
//     record must keep saying what it said).
func (a *Audit) auditReferences(in AuditInput) {
	files := map[string]bool{}
	for _, c := range in.Corpus.Conditions {
		files[c.File] = true
	}
	for _, inv := range in.Corpus.Invariants {
		files[inv.File] = true
	}
	if in.Corpus.RouterPath != "" {
		files[in.Corpus.RouterPath] = true
	}
	for _, f := range sortedKeys(files) {
		body, err := os.ReadFile(filepath.Join(in.Corpus.Root, f))
		if err != nil {
			continue
		}
		historical := historicalSpans(body)
		for i, ln := range splitLines(string(body)) {
			if historical[i+1] {
				continue
			}
			for _, m := range backtickRe.FindAllStringSubmatch(ln, -1) {
				tok := strings.TrimSpace(m[1])
				if !looksLikePath(tok) || resolvesFrom(in.Corpus.Root, f, tok) {
					continue
				}
				a.Unproven = append(a.Unproven, Claim{
					Kind: "U11", Ref: f + ":" + itoa(i+1),
					Detail:   "cites `" + tok + "`, which is not in the tree",
					Remedy:   "fix the path, or move the citation into a superseded record if it is historical",
					Evidence: f + ":" + itoa(i+1),
				})
			}
		}
	}
}

// resolvesFrom tries a pointer the way a reader would: relative to the
// citing document, then to the repo root, then — for a bare filename like
// `doctor.go` — by basename anywhere in the tree, which is how the prose
// means it.
//
// A pointer whose first segment is not a top-level directory of THIS repo
// is a sibling-repo path (`agentic-kit/docs/…`, `kits/mcp-server/…`) and is
// out of scope: this tool audits one repo's corpus, and F18 (federation) is
// refused, so it must not report another repo's tree as missing.
func resolvesFrom(root, citing, tok string) bool {
	base := filepath.Dir(filepath.Join(root, citing))
	for _, cand := range []string{filepath.Join(base, tok), filepath.Join(root, tok)} {
		clean := filepath.Clean(cand)
		if r, err := filepath.Rel(root, clean); err != nil || strings.HasPrefix(r, "..") {
			continue
		}
		if _, err := os.Stat(clean); err == nil {
			return true
		}
	}
	if !strings.Contains(tok, "/") {
		return basenameExists(root, tok)
	}
	return !firstSegmentIsLocal(root, tok)
}

// firstSegmentIsLocal reports whether tok's first path segment names a
// top-level entry of root.
func firstSegmentIsLocal(root, tok string) bool {
	seg, _, _ := strings.Cut(strings.TrimPrefix(tok, "./"), "/")
	if seg == "" || seg == ".." {
		return false
	}
	_, err := os.Stat(filepath.Join(root, seg))
	return err == nil
}

// basenameExists answers from an index built once per root, not a walk per
// token: a corpus cites dozens of bare filenames and a walk each time made
// the audit five seconds slow, well past the doctor row's budget.
func basenameExists(root, name string) bool {
	return basenameIndex(root)[name]
}

var (
	basenameMu    sync.Mutex
	basenameCache = map[string]map[string]bool{}
)

func basenameIndex(root string) map[string]bool {
	basenameMu.Lock()
	defer basenameMu.Unlock()
	if idx, ok := basenameCache[root]; ok {
		return idx
	}
	idx := map[string]bool{}
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor", "workspace", "third_party", "ref", "tmp", "dist":
				return filepath.SkipDir
			}
			return nil
		}
		idx[d.Name()] = true
		return nil
	})
	basenameCache[root] = idx
	return idx
}

// historicalSpans returns the 1-based lines belonging to a decision record
// whose status is superseded or deprecated. Such a record is a historical
// artifact: it must keep citing what it cited, and flagging it would train
// authors to edit history.
func historicalSpans(body []byte) map[int]bool {
	out := map[int]bool{}
	secs := adr.ParseSections(body)
	if len(secs) == 0 {
		return out
	}
	lines := len(splitLines(string(body)))
	for i, s := range secs {
		if s.Status != adr.StatusSuperseded && s.Status != adr.StatusDeprecated {
			continue
		}
		end := lines
		if i+1 < len(secs) {
			end = secs[i+1].Line - 1
		}
		for ln := s.Line; ln <= end; ln++ {
			out[ln] = true
		}
	}
	return out
}

var dateRe = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)

// looksLikePath keeps the reference check to things that are meant to be
// paths: a slash and a known source-ish extension, or a top-level file the
// repo actually names. A prose backtick ("`stated`") is never a path.
func looksLikePath(tok string) bool {
	if tok == "" || strings.ContainsAny(tok, " *<>|") || strings.HasSuffix(tok, "/") {
		return false
	}
	// A home-relative or absolute path is not a path in the tree.
	if strings.HasPrefix(tok, "~") || strings.HasPrefix(tok, "/") {
		return false
	}
	if strings.Contains(tok, "**") {
		return false
	}
	if !strings.Contains(tok, "/") {
		return strings.HasSuffix(tok, ".md") || strings.HasSuffix(tok, ".yml") || strings.HasSuffix(tok, ".go")
	}
	for _, ext := range []string{".go", ".md", ".cue", ".yml", ".yaml", ".sh", ".css", ".json", ".jsonl", ".sqlite", ".toml"} {
		if strings.HasSuffix(tok, ext) {
			return true
		}
	}
	return false
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func trimTo(s string, n int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

// taskTargets reads Taskfile.yml's target names.
func taskTargets(root string) map[string]bool {
	out := map[string]bool{}
	body, err := os.ReadFile(filepath.Join(root, "Taskfile.yml"))
	if err != nil {
		return out
	}
	re := regexp.MustCompile(`(?m)^  ([a-z0-9:_-]+):\s*$`)
	for _, m := range re.FindAllStringSubmatch(string(body), -1) {
		out[m[1]] = true
	}
	return out
}

// lefthookSteps reads lefthook.yml's "<hook> <step>" pairs.
func lefthookSteps(root string) map[string]bool {
	out := map[string]bool{}
	body, err := os.ReadFile(filepath.Join(root, "lefthook.yml"))
	if err != nil {
		return out
	}
	hook := ""
	for _, ln := range splitLines(string(body)) {
		if m := regexp.MustCompile(`^([a-z-]+):\s*$`).FindStringSubmatch(ln); m != nil {
			hook = m[1]
			continue
		}
		if m := regexp.MustCompile(`^    ([a-z0-9_-]+):\s*$`).FindStringSubmatch(ln); m != nil && hook != "" {
			out[hook+" "+m[1]] = true
		}
	}
	return out
}

// gateStepExists resolves "task <target>" and "lefthook <hook> <step>".
func gateStepExists(step string, targets, hooks map[string]bool) bool {
	switch {
	case strings.HasPrefix(step, "task "):
		return targets[strings.TrimSpace(strings.TrimPrefix(step, "task "))]
	case strings.HasPrefix(step, "lefthook "):
		return hooks[strings.TrimSpace(strings.TrimPrefix(step, "lefthook "))]
	}
	return true
}
