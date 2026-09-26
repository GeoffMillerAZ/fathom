package intent

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	readmanifest "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/read-manifest"
	"github.com/geoffmilleraz/fathom/internal/adr"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// visions.go — DP-3 (docs/plans/custodian.md; vision 23 §2 "VISION
// reframed"): the vision promotion pipeline, computed.
//
// Visions are promises, not claims; their truth lives in the future, and
// conformance-checking their prose would be theater. What IS checkable is
// where each promise stands on the pipeline this repo practices — vision →
// decision → plan rows → proven code — so that is all this file reads:
//
//   - its status: the vision's own `Status:` line in its header, else a
//     status its index entry LEADS with ("— ACCEPTED 2026-09-13 (ADR-033)");
//   - its trigger: any condition label its author wrote (`Promote when:` …),
//     through the corpus's own condition grammar;
//   - its plans: the plans its header or index entry DECLARES as its own (a
//     `Plan:`/`Plans:` label, or a pointer its sentence calls the vision's
//     promotion), resolved in docs/plans (or docs/plans/archive — an
//     archived plan was proven), with row counts parsed by the plan
//     grammar's own parser. A plan it merely cites or builds on is resolved
//     (a dead pointer is still a broken link) but never carries its stage;
//   - its links: every relative markdown link, and every decision id its
//     header or index entry cites;
//   - its date: the newest date on its status, index entry, or header, so
//     staleness is a number of days rather than a feeling.
//
// Nothing here compiles a rule, a condition or an invariant from a vision
// (ADR-026 decision 1 as amended 2026-09-25); nothing is edited; nothing is
// scored. Findings are a list with a remedy on every line.

// VisionStage is where a promise stands, computed — never a field a human
// maintains (vision 23 §5: fathom computes status or does not show it).
type VisionStage string

const (
	// StageLanded is computed, never read: no plan that carries the vision
	// has an open row, and at least one carries claimed work. No status
	// word ("shipped", "complete") assigns it.
	StageLanded      VisionStage = "landed"
	StagePlanned     VisionStage = "planned"     // a plan that carries it still has open rows (or no rows yet)
	StageAccepted    VisionStage = "accepted"    // status says accepted/adopted; no plan carries work
	StageForReview   VisionStage = "for-review"  // status says for review / proposed / awaiting
	StageDeferred    VisionStage = "deferred"    // status says deferred
	StageSuperseded  VisionStage = "superseded"  // terminal
	StageRejected    VisionStage = "rejected"    // terminal
	StageStated      VisionStage = "stated"      // a status shown verbatim that names no stage fathom can place — including a landed claim no carrying plan evidences
	StageConditional VisionStage = "conditional" // no status, but a named trigger
	StageUnstated    VisionStage = "unstated"    // names no status, trigger, or plan that carries work
	StageUnreadable  VisionStage = "unreadable"  // the file, or a plan that carries it, exists and could not be read (ADR-027)
)

// VisionPlan is one plan a vision names, with its rows counted statically.
// Counts are CLAIMED, not proven: `fathom verify` is the proof of record,
// and this pipeline never re-verifies (a doctor row may not spend ~70 s).
type VisionPlan struct {
	Path     string `json:"path"`  // repo-relative, resolved
	Named    string `json:"named"` // the pointer as the vision wrote it
	Archived bool   `json:"archived"`
	Total    int    `json:"total"`
	Claimed  int    `json:"claimed"`
	Open     int    `json:"open"`
	Skipped  int    `json:"skipped"`
}

// Vision is one document of the series and where it stands.
type Vision struct {
	File       string      `json:"file"`
	Number     string      `json:"number"`
	Title      string      `json:"title"`
	Status     string      `json:"status,omitempty"`
	StatusFrom string      `json:"status_from,omitempty"` // "doc" | "index"
	Stage      VisionStage `json:"stage"`
	// Plans are the plans that CARRY the vision: declared as its own in its
	// header or index entry. CitedPlans are plans it names otherwise (builds
	// on, closes a trigger of) — resolved, listed, never its stage.
	Plans      []VisionPlan `json:"plans,omitempty"`
	CitedPlans []string     `json:"cited_plans,omitempty"`
	ADRs       []string     `json:"adrs,omitempty"`
	Triggers   []string     `json:"triggers,omitempty"`
	Dated      string       `json:"dated,omitempty"`
	AgeDays    int          `json:"age_days"` // -1 when undated
	IndexLine  int          `json:"index_line,omitempty"`
}

// VisionPipeline is the whole computed view.
type VisionPipeline struct {
	Home  string
	Index string
	// IndexStatus is the index's own directory-level status line, shown for
	// context and never applied per vision: a vision "names" its status
	// only when it, or its own index entry, does.
	IndexStatus string
	Visions     []Vision
	// Findings are sorted by file, then line, then kind — surfaced per
	// vision.
	Findings []Claim
	// Uncomputable names every check that could not run, with the reason.
	Uncomputable []string
	// Absent explains an empty pipeline (no vision home), so zero visions is
	// never read as a healthy one.
	Absent    string
	StaleDays int
	Coverage  readmanifest.Manifest
}

// visionStaleDays is how long a vision may sit "for review" with no plan
// before it is listed as stale: the same 30 days the practice's own
// intent-doctor check 14 uses for an open incident, and the plane's kill
// clocks use for a verb nobody runs — one threshold, not a new one.
const visionStaleDays = 30

var (
	mdLinkRe       = regexp.MustCompile(`\[[^\]]*\]\(([^)\s]+)\)`)
	planPointerRe  = regexp.MustCompile(`(?:\.\./|docs/)?plans/(?:archive/)?[A-Za-z0-9][A-Za-z0-9._-]*\.md`)
	adrIDRe        = regexp.MustCompile(`\bADR-(\d+)\b`)
	statusLineRe   = regexp.MustCompile(`(?i)^[>\s*_]*status\s*[*_]*\s*[:.]\s*[*_]*\s*(.*)$`)
	indexPrefixRe  = regexp.MustCompile(`^[\s\d.|*\-]*$`)
	addedParenRe   = regexp.MustCompile(`^\*?\(added [^)]*\)\*?`)
	leadingNumRe   = regexp.MustCompile(`^(\d+)`)
	visionStageMap = []struct {
		word  string
		stage VisionStage
	}{
		{"superseded", StageSuperseded},
		{"rejected", StageRejected}, {"declined", StageRejected}, {"withdrawn", StageRejected}, {"refused", StageRejected},
		{"deferred", StageDeferred},
		{"accepted", StageAccepted}, {"adopted", StageAccepted},
		{"for review", StageForReview}, {"awaiting", StageForReview}, {"founding doc", StageForReview},
		{"proposed", StageForReview}, {"draft", StageForReview},
	}
	// landedClaimWords say a promise landed. An index entry that LEADS with
	// one states a status (leadingStatus), but no status word is ever a
	// stage: landed is computed from the plans that carry a vision (vision
	// 23 §5 — fathom computes status or does not show it), so a status made
	// only of these is `stated`, shown verbatim.
	landedClaimWords = []string{"complete", "landed", "shipped"}

	// Carrier detection (declaresCarrier): a clause ends at sentence or list
	// punctuation or a table rule followed by whitespace; the last label
	// before a plan pointer in its clause says whether the vision declares
	// it (`Plan:`, `Plans:`, "plan `…`", "Plan rows: … in") or cites it.
	clauseEndRe    = regexp.MustCompile("[.;!?|][*_)\"'`]*(?:\\s|$)")
	carrierLabelRe = regexp.MustCompile(`(?i)\bplans?\b`)
	citeLabelRe    = regexp.MustCompile(`(?i)\b(?:builds? on|built on|successor to|supersedes|superseded by|see also|related|inputs?|depends on|prior art|cites?|sources?|closes|closed)\b`)
	promotionRe    = regexp.MustCompile(`(?i)\bpromotion\b`)
	// pathTokRe blanks paths and code spans out of a clause before labels
	// are looked for, so the "plans" inside `docs/plans/x.md` is no label.
	pathTokRe    = regexp.MustCompile("`[^`]*`|\\S*/\\S*")
	adrFileNumRe = regexp.MustCompile(`(?i)^(?:adr-?)?(\d+)`)
)

// LoadVisions computes the promotion pipeline for c's repository. now is
// injected so staleness is deterministic.
func LoadVisions(c Corpus, now time.Time) (vp VisionPipeline) {
	vp = VisionPipeline{StaleDays: visionStaleDays}
	reads := readmanifest.NewSet()
	reads.RelativeTo(c.Root)
	// Named result: every return path, early ones included, carries the
	// manifest of what was and was not read (ADR-027).
	defer func() { vp.Coverage = reads.Manifest() }()

	// The vision series is the one the router's docs map names, and only
	// that (ADR-026 amendment 2026-09-25, part 2): a docs/vision/ the router
	// does not route to is not read.
	vp.Home, vp.Index = c.VisionsHome, c.VisionsIndex
	if vp.Home == "" {
		vp.Absent = "no vision series — the router's docs map names none (a docs-map row whose question names the vision series)"
		if c.RouterPath == "" {
			vp.Absent = "no vision series — no intent router (docs/intent/README.md) was read, so none is named"
		}
		return vp
	}
	homeAbs := filepath.Join(c.Root, filepath.FromSlash(vp.Home))
	entries, err := os.ReadDir(homeAbs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
			reads.Record(homeAbs, readmanifest.Absent())
			vp.Absent = "no vision series — the router's docs map names " + vp.Home + ", which is not a directory in this tree"
			vp.Home, vp.Index = "", ""
			return vp
		}
		// ADR-027, I38: a home that exists and cannot be listed is
		// blindness on the manifest — never zero visions, never complete.
		reads.Record(homeAbs, readmanifest.Unreadable(err))
		vp.Uncomputable = append(vp.Uncomputable, "vision home "+vp.Home+" could not be listed ("+errnoOf(err)+
			") — no vision in it was seen, so every count here is a lower bound")
		return vp
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		files = append(files, filepath.Join(homeAbs, e.Name()))
	}
	sort.Strings(files)

	var found []finding
	indexLines := []string{}
	if vp.Index != "" {
		b, rerr := readRecorded(reads, filepath.Join(c.Root, filepath.FromSlash(vp.Index)))
		switch {
		case errors.Is(rerr, fs.ErrNotExist):
			vp.Uncomputable = append(vp.Uncomputable, "index "+vp.Index+", which the router's docs map names, is not in this tree — index-entry statuses and dates are not in this view")
		case rerr != nil:
			vp.Uncomputable = append(vp.Uncomputable, "index "+vp.Index+" could not be read ("+errnoOf(rerr)+") — index-entry statuses and dates are not in this view")
		default:
			indexLines = splitLines(string(b))
			vp.IndexStatus = indexStatus(indexLines)
			found = append(found, checkLinks(c.Root, vp.Index, indexLines, reads)...)
		}
	}

	decisions, width, decNote := decisionIDs(c, reads)
	if decNote != "" {
		vp.Uncomputable = append(vp.Uncomputable, "decision ids: "+decNote)
	}
	var foreign []string
	plans := planCache{root: c.Root, reads: reads, parsed: map[string]planState{}}

	for _, abs := range files {
		rel := filepath.ToSlash(relTo(c.Root, abs))
		if rel == vp.Index {
			continue
		}
		v := Vision{File: rel, AgeDays: -1}
		if m := leadingNumRe.FindStringSubmatch(filepath.Base(abs)); m != nil {
			v.Number = m[1]
		}
		idxLine, idxText := indexEntry(indexLines, filepath.Base(abs))
		v.IndexLine = idxLine

		body, rerr := readRecorded(reads, abs)
		if rerr != nil {
			v.Stage = StageUnreadable
			vp.Visions = append(vp.Visions, v)
			found = append(found, finding{file: rel, kind: "unreadable", Claim: Claim{
				Ref: rel, Detail: "could not be read (" + rerr.Error() + ") — where this promise stands is unknown, not unstated",
				Remedy:   "check the file is readable; on an external volume this is usually a wedged TCC verdict — `tccutil reset SystemPolicyRemovableVolumes <terminal bundle id>`",
				Evidence: "read " + rel,
			}})
			continue
		}
		lines := splitLines(string(body))
		pre := preamble(lines)
		for _, ln := range lines {
			if strings.HasPrefix(ln, "# ") {
				v.Title = strings.TrimSpace(strings.TrimPrefix(ln, "# "))
				break
			}
		}

		// Status: the doc's own line wins; else what its index entry leads with.
		docStatus, docLine, docPara := statusIn(pre)
		switch {
		case docStatus != "":
			v.Status, v.StatusFrom = docStatus, "doc"
		case idxText != "":
			if st := leadingStatus(idxText); st != "" {
				v.Status, v.StatusFrom = st, "index"
			}
		}
		stage := classifyStatus(v.Status)

		// Triggers: the corpus's own condition grammar over the whole doc.
		var tmp Corpus
		tmp.parseConditionsIn(rel, string(body))
		for _, cond := range tmp.Conditions {
			v.Triggers = append(v.Triggers, cond.Label+": "+trimTo(cond.Text, 100))
		}

		// Plans and decision ids: the header and the index entry only — the
		// promotion metadata, not the prose.
		srcs := []metaSource{{file: rel, first: 1, lines: pre}}
		if idxLine > 0 {
			srcs = append(srcs, metaSource{file: vp.Index, first: idxLine, lines: indexLines[idxLine-1 : idxLine]})
		}
		seenPlan := map[string]bool{}
		seenADR := map[string]bool{}
		var cited, decided []string
		var blindCarriers []finding
		for _, src := range srcs {
			for _, pm := range planMentions(src.file, src.first, src.lines) {
				vpn, blindPath, blindErr := plans.resolve(pm.tok)
				if blindErr != "" {
					// It exists (or cannot be told apart from existing) and
					// could not be read: named on the coverage manifest, never
					// "not in this tree" (ADR-027, I38). A plan that carries
					// the vision makes its stage unknowable, and says so.
					if pm.carrier && !seenPlan["blind:"+blindPath] {
						seenPlan["blind:"+blindPath] = true
						blindCarriers = append(blindCarriers, finding{file: pm.file, line: pm.line, kind: "unreadable", Claim: Claim{
							Ref:      pm.file + ":" + itoa(pm.line),
							Detail:   "names plan `" + blindPath + "`, which exists and could not be read (" + blindErr + ") — where this promise stands is unknown, not unplanned",
							Remedy:   "check the plan is readable; on an external volume this is usually a wedged TCC verdict — `tccutil reset SystemPolicyRemovableVolumes <terminal bundle id>`",
							Evidence: "read " + blindPath,
						}})
					}
					continue
				}
				if vpn == nil {
					if pm.linked {
						continue // a markdown link: checkLinks already names it
					}
					found = append(found, finding{file: pm.file, line: pm.line, kind: "broken-link", Claim: Claim{
						Ref: pm.file + ":" + itoa(pm.line), Detail: "names plan `" + pm.tok + "`, which is not in this tree",
						Remedy:   "fix the pointer; a plan in another repository is not this pipeline's evidence",
						Evidence: "resolved against docs/plans and docs/plans/archive",
					}})
					continue
				}
				if !pm.carrier {
					if !contains(cited, vpn.Path) {
						cited = append(cited, vpn.Path)
					}
					continue
				}
				if seenPlan[vpn.Path] {
					continue
				}
				seenPlan[vpn.Path] = true
				p := *vpn
				p.Named = pm.tok
				v.Plans = append(v.Plans, p)
			}
			for i, ln := range src.lines {
				line := src.first + i
				for _, m := range adrIDRe.FindAllStringSubmatch(ln, -1) {
					id := "ADR-" + m[1]
					if seenADR[id+src.file] {
						continue
					}
					seenADR[id+src.file] = true
					if !contains(v.ADRs, id) {
						v.ADRs = append(v.ADRs, id)
					}
					if decisions == nil {
						if !contains(decided, id) {
							decided = append(decided, id) // unchecked, named once above
						}
						continue
					}
					if foreignNumbering(m[1], width) {
						// "platform-engineering ADR-0098": another repository's
						// numbering (zero-padded to a different width than this
						// home's ids). Out of scope, like a sibling-repo path —
						// named below, never reported as missing here.
						if !contains(foreign, id) {
							foreign = append(foreign, id)
						}
						continue
					}
					n, _ := strconv.Atoi(m[1])
					if !decisions[n] {
						found = append(found, finding{file: src.file, line: line, kind: "broken-link", Claim: Claim{
							Ref: src.file + ":" + itoa(line), Detail: "cites " + id + ", which no decision record in " + c.decisionsHome() + " carries",
							Remedy:   "fix the id, or record the decision (intent-decide)",
							Evidence: "decision ids read from " + c.decisionsHome(),
						}})
						continue
					}
					if !contains(decided, id) {
						decided = append(decided, id)
					}
				}
			}
		}
		sort.Strings(v.ADRs)
		sort.Strings(decided)
		for _, cp := range cited {
			if !seenPlan[cp] {
				v.CitedPlans = append(v.CitedPlans, cp)
			}
		}

		// Dates: the newest on the status paragraph, the index entry, or the header.
		var dateSrc []string
		dateSrc = append(dateSrc, docPara...)
		dateSrc = append(dateSrc, pre...)
		if idxLine > 0 {
			dateSrc = append(dateSrc, indexLines[idxLine-1])
		}
		if d, ok := newestDate(dateSrc); ok {
			v.Dated = d.Format("2006-01-02")
			v.AgeDays = int(now.UTC().Truncate(24*time.Hour).Sub(d).Hours() / 24)
		}

		v.Stage = visionStage(stage, v.Plans, len(v.Triggers) > 0)
		if len(blindCarriers) > 0 {
			v.Stage = StageUnreadable
			found = append(found, blindCarriers...)
		}
		vp.Visions = append(vp.Visions, v)

		found = append(found, checkLinks(c.Root, rel, lines, reads)...)
		switch {
		case v.Stage == StageUnstated:
			ev := "header (lines 1–" + itoa(len(pre)) + ")"
			if idxLine > 0 {
				ev += " and index entry " + vp.Index + ":" + itoa(idxLine)
			} else if vp.Index != "" {
				ev += "; no entry in " + vp.Index
			}
			detail := "names no status, trigger, or plan — where this promise stands cannot be computed"
			if len(v.Plans) > 0 {
				detail = "names no status or trigger, and its plan carries no claimed or open row (every row deferred or refused) — where this promise stands cannot be computed"
			}
			found = append(found, finding{file: rel, kind: "unstated", Claim: Claim{
				Ref: rel, Detail: detail,
				Remedy:   "add a `Status:` line to its header (accepted | for review | superseded | rejected, dated), or name the plan that carries it (`Plan: docs/plans/<plan>.md`)",
				Evidence: ev + " read",
			}})
		case v.Stage == StageForReview && v.AgeDays > vp.StaleDays:
			detail := fmt.Sprintf("%q — for review %d days (since %s) with no plan carrying it and no decision; threshold %d", v.Status, v.AgeDays, v.Dated, vp.StaleDays)
			remedy := "decide it: accept (a decision record and a plan), reject, or supersede — and restate its status"
			if len(decided) > 0 {
				// It cites a decision: never told it has none (the remedy
				// would ask for a record that already exists).
				ids := strings.Join(decided, ", ")
				detail = fmt.Sprintf("%q — for review %d days (since %s) with no plan carrying it; it cites %s; threshold %d", v.Status, v.AgeDays, v.Dated, ids, vp.StaleDays)
				remedy = "if " + ids + " decided it, restate its status to match (accepted with the plan that carries it, rejected, or superseded); otherwise decide it"
			}
			found = append(found, finding{file: rel, line: docLine, kind: "stale", Claim: Claim{
				Ref:      rel + lineSuffix(docLine),
				Detail:   detail,
				Remedy:   remedy,
				Evidence: "status from " + v.StatusFrom,
			}})
		}
		if v.Dated == "" {
			found = append(found, finding{file: rel, kind: "undated", Claim: Claim{
				Ref: rel, Detail: "no date on its status, index entry, or header — its staleness cannot be computed",
				Remedy:   "date its status line (YYYY-MM-DD)",
				Evidence: "dates searched in the header and index entry",
			}})
		}
	}

	if len(foreign) > 0 {
		sort.Strings(foreign)
		vp.Uncomputable = append(vp.Uncomputable, fmt.Sprintf(
			"%d cited decision id(s) use another repository's numbering (%s) and are not checked against %s",
			len(foreign), strings.Join(foreign, ", "), c.decisionsHome()))
	}

	sort.SliceStable(found, func(i, j int) bool {
		if found[i].file != found[j].file {
			return found[i].file < found[j].file
		}
		if found[i].line != found[j].line {
			return found[i].line < found[j].line
		}
		return found[i].kind < found[j].kind
	})
	for _, f := range found {
		f.Claim.Kind = f.kind
		vp.Findings = append(vp.Findings, f.Claim)
	}
	return vp
}

// finding carries a Claim with the sort keys that make the list per-vision.
type finding struct {
	Claim
	file string
	line int
	kind string
}

// metaSource is one run of a vision's promotion metadata — its header, or
// its index entry — with the 1-based line of its first line.
type metaSource struct {
	file  string
	first int
	lines []string
}

// planMention is one plan pointer in a vision's promotion metadata.
type planMention struct {
	tok     string
	file    string
	line    int
	carrier bool // declared as the vision's own plan, not merely cited
	linked  bool // (the tail of) a markdown link target on its line
}

// planMentions finds every plan pointer in one metadata source, with the
// line it sits on and whether the vision DECLARES it as its carrier. Lines
// are joined so a label that wraps ("Plan:\n`docs/plans/x.md`") still
// governs its pointer.
func planMentions(file string, first int, lines []string) []planMention {
	joined := strings.Join(lines, "\n")
	starts := make([]int, len(lines))
	off := 0
	for i, ln := range lines {
		starts[i] = off
		off += len(ln) + 1
	}
	var out []planMention
	for _, loc := range planPointerRe.FindAllStringIndex(joined, -1) {
		li := sort.Search(len(starts), func(i int) bool { return starts[i] > loc[0] }) - 1
		tok := joined[loc[0]:loc[1]]
		out = append(out, planMention{
			tok: tok, file: file, line: first + li,
			carrier: declaresCarrier(joined, loc[0], loc[1]),
			linked:  isLinkTarget(linkTargets(lines[li]), tok),
		})
	}
	return out
}

// declaresCarrier reports whether the plan pointer at text[start:end] is
// the vision's own plan rather than one it cites. Within the pointer's
// clause, the last label before it must be a plan label (`Plan:`,
// `Plans:`, "plan `…`", "Plan rows: … in") and not a citation label
// ("Builds on:", "Successor to", "closes the trigger", "Inputs:" …); with
// no label at all, the clause after it must call it the vision's promotion
// ("… is its binding promotion"). A plan a vision builds on is evidence
// about another promise, never this one's stage.
func declaresCarrier(text string, start, end int) bool {
	from := 0
	for _, m := range clauseEndRe.FindAllStringIndex(text[:start], -1) {
		from = m[1]
	}
	before := pathTokRe.ReplaceAllString(text[from:start], " ")
	lastCarrier, lastCite := lastMatch(carrierLabelRe, before), lastMatch(citeLabelRe, before)
	if lastCarrier > lastCite {
		return true
	}
	if lastCite >= 0 {
		return false
	}
	after := text[end:]
	if m := clauseEndRe.FindStringIndex(after); m != nil {
		after = after[:m[0]]
	}
	return promotionRe.MatchString(after)
}

// lastMatch is the offset of re's last match in s, or -1.
func lastMatch(re *regexp.Regexp, s string) int {
	locs := re.FindAllStringIndex(s, -1)
	if len(locs) == 0 {
		return -1
	}
	return locs[len(locs)-1][0]
}

func lineSuffix(n int) string {
	if n <= 0 {
		return ""
	}
	return ":" + itoa(n)
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func relTo(root, p string) string {
	r, err := filepath.Rel(root, p)
	if err != nil {
		return p
	}
	return r
}

// readRecorded reads a file and records the outcome on the manifest, so an
// unreadable file is accounted for and its errno is available to name.
func readRecorded(s *readmanifest.Set, abs string) ([]byte, error) {
	b, err := os.ReadFile(abs)
	switch {
	case err == nil:
		s.Record(abs, readmanifest.Read(len(b)))
	case errors.Is(err, fs.ErrNotExist):
		s.Record(abs, readmanifest.Absent())
	default:
		s.Record(abs, readmanifest.Unreadable(err))
	}
	return b, err
}

// preamble is a vision's header: every line before its first `## `
// heading. Status, plan pointers and dates are read from here and from the
// index entry only — never from the argument below it.
func preamble(lines []string) []string {
	for i, ln := range lines {
		if strings.HasPrefix(ln, "## ") {
			return lines[:i]
		}
	}
	if len(lines) > 40 {
		return lines[:40]
	}
	return lines
}

// statusIn finds a `Status:` line in a header: its value (first line,
// emphasis stripped), its 1-based line, and its paragraph (for dates).
func statusIn(pre []string) (string, int, []string) {
	for i, ln := range pre {
		m := statusLineRe.FindStringSubmatch(ln)
		if m == nil {
			continue
		}
		val := strings.TrimSpace(strings.ReplaceAll(stripEmphasis(m[1]), "_", " "))
		para := []string{ln}
		for j := i + 1; j < len(pre) && j < i+6; j++ {
			t := strings.TrimSpace(strings.TrimLeft(pre[j], "> "))
			if t == "" {
				break
			}
			para = append(para, pre[j])
		}
		if val == "" {
			continue
		}
		return trimTo(val, 160), i + 1, para
	}
	return "", 0, nil
}

// indexStatus is the index's own directory-level status line, for context.
func indexStatus(lines []string) string {
	s, _, _ := statusIn(preamble(lines))
	return s
}

// indexEntry finds a vision's entry in the index: the first line whose
// FIRST markdown link targets the vision and whose text before that link is
// only list or table markers ("1. **", "- **", "| 33 | "). A table cell
// that merely cites the vision ("| Where? | … | [03](03-x.md) |") is a
// reference, not its entry.
func indexEntry(lines []string, base string) (int, string) {
	for i, ln := range lines {
		loc := mdLinkRe.FindStringSubmatchIndex(ln)
		if loc == nil {
			continue
		}
		target := strings.TrimPrefix(stripFragment(ln[loc[2]:loc[3]]), "./")
		if target != base {
			continue
		}
		if !indexPrefixRe.MatchString(ln[:loc[0]]) {
			continue
		}
		return i + 1, ln[loc[1]:]
	}
	return 0, ""
}

// leadingStatus returns the status an index entry LEADS with, or "". Prose
// that merely mentions a stage word ("the gate on the asks — accepted /
// reframed / rejected") is not a status; only an entry that opens with one
// states it.
func leadingStatus(rest string) string {
	t := strings.TrimSpace(rest)
	for {
		before := t
		t = strings.TrimLeft(t, "*| \t:")
		t = strings.TrimPrefix(t, "—")
		t = strings.TrimPrefix(t, "–")
		t = strings.TrimPrefix(t, "- ")
		t = strings.TrimSpace(addedParenRe.ReplaceAllString(strings.TrimSpace(t), ""))
		if t == before {
			break
		}
	}
	low := strings.ToLower(t)
	words := make([]string, 0, len(visionStageMap)+len(landedClaimWords))
	for _, sw := range visionStageMap {
		words = append(words, sw.word)
	}
	words = append(words, landedClaimWords...)
	for _, w := range words {
		if strings.HasPrefix(low, w) {
			end := len(t)
			for _, stop := range []string{" — ", " – ", " |", "|", ". "} {
				if k := strings.Index(t, stop); k >= 0 && k < end {
					end = k
				}
			}
			return strings.TrimSpace(strings.TrimRight(stripEmphasis(t[:end]), ". "))
		}
	}
	return ""
}

// classifyStatus maps a status to a stage: the EARLIEST stage word wins, and
// a landed claim ("complete", "landed", "shipped") is never a stage word —
// so "Phase F is landed; Phases G–I are a founding doc" is for-review, and a
// status of only "shipped" is `stated` until a carrying plan evidences it.
func classifyStatus(status string) VisionStage {
	if status == "" {
		return ""
	}
	low := strings.ToLower(status)
	best, bestAt := StageStated, len(low)+1
	for _, sw := range visionStageMap {
		if k := strings.Index(low, sw.word); k >= 0 && k < bestAt {
			best, bestAt = sw.stage, k
		}
	}
	return best
}

// visionStage combines the stated stage with the computed evidence. A
// terminal status stands. A plan that carries the vision outranks what its
// status says: an open row (or a live plan with no rows yet) is planned;
// when no carrying plan has an open row and at least one has claimed work,
// the promise landed — the only way `landed` is ever assigned. A plan whose
// rows are all deferred or refused carries nothing, so the stated stage
// stands. A trigger alone is a conditional promise.
func visionStage(stated VisionStage, plans []VisionPlan, hasTrigger bool) VisionStage {
	if stated == StageSuperseded || stated == StageRejected {
		return stated
	}
	claimed := false
	for _, p := range plans {
		if !p.Archived && (p.Open > 0 || p.Total == 0) {
			return StagePlanned
		}
		if p.Claimed > 0 {
			claimed = true
		}
	}
	if claimed {
		return StageLanded
	}
	if stated != "" {
		return stated
	}
	if hasTrigger {
		return StageConditional
	}
	return StageUnstated
}

// newestDate is the latest valid YYYY-MM-DD in lines.
func newestDate(lines []string) (time.Time, bool) {
	var best time.Time
	for _, ln := range lines {
		for _, s := range dateRe.FindAllString(ln, -1) {
			d, err := time.Parse("2006-01-02", s)
			if err == nil && d.After(best) {
				best = d
			}
		}
	}
	return best, !best.IsZero()
}

func stripFragment(t string) string {
	if k := strings.IndexAny(t, "#?"); k >= 0 {
		return t[:k]
	}
	return t
}

// linkTargets is the set of markdown link targets on a line.
func linkTargets(ln string) map[string]bool {
	out := map[string]bool{}
	for _, m := range mdLinkRe.FindAllStringSubmatch(ln, -1) {
		out[stripFragment(m[1])] = true
	}
	return out
}

// checkLinks reports every relative markdown link in a document that does
// not resolve. Web links, bare anchors, links inside code, and links that
// leave the repository (a sibling repo is out of scope — F18) are skipped.
// A target the reader cannot see (EACCES on a directory it cannot search)
// is recorded on reads as unreadable and is never a broken link (ADR-027).
func checkLinks(root, rel string, lines []string, reads *readmanifest.Set) []finding {
	var out []finding
	base := filepath.Dir(filepath.Join(root, filepath.FromSlash(rel)))
	fence := false
	for i, ln := range lines {
		if strings.HasPrefix(strings.TrimSpace(ln), "```") {
			fence = !fence
			continue
		}
		if fence {
			continue
		}
		for _, m := range mdLinkRe.FindAllStringSubmatchIndex(ln, -1) {
			if inBackticks(ln, m[0]) {
				continue
			}
			raw := ln[m[2]:m[3]]
			if strings.Contains(raw, "://") || strings.HasPrefix(raw, "mailto:") || strings.HasPrefix(raw, "#") {
				continue
			}
			target := stripFragment(raw)
			if target == "" {
				continue
			}
			cand := filepath.Clean(filepath.Join(base, filepath.FromSlash(target)))
			if r, err := filepath.Rel(root, cand); err != nil || strings.HasPrefix(r, "..") {
				continue
			}
			present, blind := statPresence(cand)
			if present {
				continue
			}
			if blind != nil {
				reads.Record(cand, readmanifest.Unreadable(blind))
				continue
			}
			detail := "links `" + raw + "`, which is not in the tree"
			remedy := "fix the link, or drop it"
			if strings.Contains(target, "plans/") {
				arch := filepath.Join(root, "docs", "plans", "archive", filepath.Base(target))
				if statExists(arch) {
					detail += " — the plan is archived"
					remedy = "point at docs/plans/archive/" + filepath.Base(target)
				}
			}
			out = append(out, finding{file: rel, line: i + 1, kind: "broken-link", Claim: Claim{
				Ref: rel + ":" + itoa(i+1), Detail: detail, Remedy: remedy,
				Evidence: "resolved relative to " + filepath.ToSlash(filepath.Dir(rel)),
			}})
		}
	}
	return out
}

// decisionIDs is the set of decision numbers the decisions home carries and
// the digit width its ids are written at (0 when they vary), or nil with the
// reason when it cannot be read.
func decisionIDs(c Corpus, reads *readmanifest.Set) (map[int]bool, int, string) {
	home := c.decisionsHome()
	if home == "" {
		return nil, 0, "no decisions home resolved, so cited ADR ids are not checked"
	}
	abs := filepath.Join(c.Root, filepath.FromSlash(home))
	info, err := os.Stat(abs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
			return nil, 0, "decisions home " + home + " does not exist"
		}
		reads.Record(abs, readmanifest.Unreadable(err))
		return nil, 0, "decisions home " + home + " could not be read (" + errnoOf(err) + "), so cited ADR ids are not checked"
	}
	out := map[int]bool{}
	widths := map[int]int{}
	add := func(digits string) {
		n, _ := strconv.Atoi(digits)
		out[n] = true
		widths[len(digits)]++
	}
	if info.IsDir() {
		// A directory that exists and cannot be listed leaves every cited
		// id unchecked and named — never an empty set that turns each one
		// into "no decision record carries it" (ADR-027, I38).
		entries, lerr := os.ReadDir(abs)
		if lerr != nil {
			reads.Record(abs, readmanifest.Unreadable(lerr))
			return nil, 0, "decisions home " + home + " could not be listed (" + errnoOf(lerr) + "), so cited ADR ids are not checked"
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			if m := adrFileNumRe.FindStringSubmatch(e.Name()); m != nil {
				add(m[1])
			}
		}
	} else {
		b, rerr := readRecorded(reads, abs)
		if rerr != nil {
			return nil, 0, home + " could not be read (" + rerr.Error() + ")"
		}
		for _, id := range adr.SectionIDs(adr.ParseSections(b)) {
			add(strings.TrimPrefix(id, "ADR-"))
		}
	}
	width := 0
	if len(widths) == 1 {
		for w := range widths {
			width = w
		}
	}
	return out, width, ""
}

// foreignNumbering reports whether a cited id is zero-padded to a width this
// home never uses — "ADR-0098" beside a home of "ADR-001…ADR-045" is another
// repository's record, not a missing one of ours.
func foreignNumbering(digits string, width int) bool {
	return width > 0 && len(digits) != width && strings.HasPrefix(digits, "0")
}

// planCache resolves plan pointers once per path and counts their rows with
// the plan grammar's own parser.
type planCache struct {
	root   string
	reads  *readmanifest.Set
	parsed map[string]planState // by candidate path; a genuinely absent path is not cached
}

// planState is one candidate path's outcome: its parsed plan, or blind —
// the reader's error when the path exists (or cannot be told apart from
// existing) and could not be read.
type planState struct {
	plan  *VisionPlan
	blind string
}

// resolve returns the plan a pointer names. blindPath/blindErr are set when
// a candidate could not be seen — the plan exists and was unreadable, or
// its directory could not be searched — an instrument failure recorded on
// the manifest (ADR-027), never "not in this tree".
func (pc planCache) resolve(tok string) (plan *VisionPlan, blindPath, blindErr string) {
	name := filepath.Base(tok)
	live, arch := "docs/plans/"+name, "docs/plans/archive/"+name
	cands := []string{live, arch}
	if strings.Contains(tok, "archive/") {
		cands = []string{arch, live}
	}
	for _, rel := range cands {
		st, seen := pc.parsed[rel]
		if !seen {
			var there bool
			st, there = pc.load(rel, rel == arch)
			if !there {
				continue
			}
			pc.parsed[rel] = st
		}
		if st.plan != nil {
			return st.plan, "", ""
		}
		if blindErr == "" {
			blindPath, blindErr = rel, st.blind
		}
	}
	return nil, blindPath, blindErr
}

// load reads one candidate plan path; there is false only when the path
// genuinely is not in the tree.
func (pc planCache) load(rel string, archived bool) (st planState, there bool) {
	abs := filepath.Join(pc.root, filepath.FromSlash(rel))
	present, blind := statPresence(abs)
	if blind != nil {
		pc.reads.Record(abs, readmanifest.Unreadable(blind))
		return planState{blind: errnoOf(blind)}, true
	}
	if !present {
		return planState{}, false
	}
	b, err := readRecorded(pc.reads, abs)
	if err != nil {
		return planState{blind: errnoOf(err)}, true
	}
	vpn := &VisionPlan{Path: rel, Archived: archived}
	for _, t := range verify.ParseMarkdownPlan("", b).Tasks {
		vpn.Total++
		switch {
		case t.Claimed:
			vpn.Claimed++
		case t.Skipped:
			vpn.Skipped++
		default:
			vpn.Open++
		}
	}
	return planState{plan: vpn}, true
}

// isLinkTarget reports whether tok is (the tail of) a markdown link target
// on the same line.
func isLinkTarget(links map[string]bool, tok string) bool {
	for t := range links {
		if strings.HasSuffix(t, tok) {
			return true
		}
	}
	return false
}
